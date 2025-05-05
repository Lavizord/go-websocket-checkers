package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Lavizord/checkers-server/internal/config"
	"github.com/Lavizord/checkers-server/internal/interfaces"
	"github.com/Lavizord/checkers-server/internal/messages"
	"github.com/Lavizord/checkers-server/internal/models"
	"github.com/Lavizord/checkers-server/internal/redisdb"

	"github.com/gorilla/websocket"
)

var redisClient *redisdb.RedisClient

var (
	players      = make(map[string]*models.Player) // Store players by ID
	playersMutex = sync.Mutex{}                    // Protects the map
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	writeWait    = 10 * time.Second
	pongWait     = 3 * time.Second
	pingInterval = 2 * time.Second // must be < pongWait
)

func init() {
	config.LoadConfig()
	redisAddr := config.Cfg.Redis.Addr
	client, err := redisdb.NewRedisClient(redisAddr)
	if err != nil {
		log.Fatalf("[Redis] Error initializing Redis client: %v", err)
	}
	redisClient = client
	go subscribeToBroadcastChannel() // This is a global channel. WSAPI will send the messages from this channel to all active ws connections
}

func HandleConnection(w http.ResponseWriter, r *http.Request) {
	//log.Printf("[wsapi] - HandleConnection: Raw query string: %s\n", r.URL.RawQuery)
	token := r.URL.Query().Get("token")
	sessionID := r.URL.Query().Get("sessionid")
	currency := r.URL.Query().Get("currency")
	//log.Printf("[wsapi] - HandleConnection: token[%v], sessionid[%v], currency[%v]\n", token, sessionID, currency)
	session, err := FetchAndValidateSession(token, sessionID, currency)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unauthorized: token[%v], sessionid[%v], currency[%v]", token, sessionID, currency), http.StatusUnauthorized)
		if session != nil {
			redisClient.RemoveSession(session.ID)
		}
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade:", err)
		conn.Close()
		return
	}
	var player *models.Player
	// We will just check if the player that connected is part of our disconnected players, this is a list of in game players that disconnected.
	// players not in game will just be deleted, and recreated when they are reconnected.
	discPlayer := redisClient.GetDisconnectedPlayerData(sessionID)
	if discPlayer != nil {
		player = &models.Player{
			ID:                 discPlayer.ID,
			Conn:               conn,
			Token:              discPlayer.Token,
			Name:               discPlayer.Name,
			SessionID:          discPlayer.SessionID,
			Currency:           currency,
			Status:             models.StatusInGame,
			GameID:             discPlayer.GameID,
			WriteChan:          make(chan []byte),
			OperatorIdentifier: session.OperatorIdentifier,
		}
	} else {
		existingPlayer, _ := redisClient.GetPlayer(sessionID)
		if existingPlayer != nil {
			log.Println("Session with active player")
			conn.Close()
			return
		}
		//playerID := models.GenerateUUID() Commented to make the player id = the session id.
		newPlayer := &models.Player{
			ID:                 sessionID,
			Conn:               conn,
			Token:              session.Token,
			Name:               session.PlayerName,
			SessionID:          session.ID,
			Currency:           currency,
			Status:             models.StatusOnline,
			WriteChan:          make(chan []byte),
			OperatorIdentifier: session.OperatorIdentifier,
		}
		player = newPlayer
		redisClient.AddPlayer(player) // Since its a new player, we add it to redis.
	}
	player.StartWriteGoroutineNew(func() {
		handlePlayerDisconnect(player)
	}, pingInterval, writeWait)

	// We add the player to our player map.
	playersMutex.Lock()
	_, exists := players[player.ID] // Check if player exists
	if !exists {
		players[player.ID] = player
	}
	playersMutex.Unlock()

	subscriptionReady := make(chan bool)
	go subscribeToPlayerChannel(player, subscriptionReady)
	<-subscriptionReady // Wait for the subscription to be ready
	module := interfaces.OperatorModules[player.OperatorIdentifier.OperatorName]
	walletBalance, err := module.HandleFetchWalletBalance(*session, redisClient)
	if err != nil {
		log.Printf("Failed to fetch wallet : %v", err)
		handlePlayerDisconnect(player)
		return
	}
	msg, err := messages.GenerateConnectedMessage(*player, walletBalance)
	if err != nil {
		log.Printf("Failed to generate connected message : %v", err)
		handlePlayerDisconnect(player)
		return
	}
	player.WriteChan <- msg

	// Now that our player has subscribbed to our stuff, we will notify the gameworker of the reconnect.
	if discPlayer != nil {
		redisClient.RPush("reconnect_game", player)
	}
	go handleMessages(player)
}

// Function to handle player channel subscription
func subscribeToPlayerChannelOld(player *models.Player, ready chan bool) {
	redisClient.SubscribePlayerChannel(*player, func(message string) {
		//log.Println("[wsapi] - Received server to PLAYER message:", message)
		player.WriteChan <- []byte(message)
	})
	ready <- true // Notify that the subscription is ready
}

func subscribeToPlayerChannel(player *models.Player, ready chan bool) {
	// Subscribe to the player's specific channel
	redisClient.SubscribePlayerChannel(*player, func(message string) {
		// Check if the message indicates a disconnect for this player
		if message == fmt.Sprintf("disconnect:%s", player.ID) {
			log.Printf("[wsapi] - Disconnecting player: %s", player.ID)
			player.Conn.Close() // Close the WebSocket connection for the player
			handlePlayerDisconnect(player)
		} else {
			// Send normal messages to the player's write channel
			player.WriteChan <- []byte(message)
		}
	})
	ready <- true // Notify that the subscription is ready
}

func subscribeToBroadcastChannel() {
	redisClient.Subscribe("game_info", func(message string) {
		//fmt.Println("[wsapi] - Received BROADCAST message:", message)
		msg, err := messages.ParseMessage([]byte(message))
		if err != nil {
			log.Println("[wsapi] - Failed to parse broadcast message:", err)
			return
		}
		finalBytes, err := json.Marshal(msg)
		if err != nil {
			log.Println("[wsapi] - Failed to marshal final broadcast message:", err)
			return
		}
		playersMutex.Lock()
		defer playersMutex.Unlock() // Ensures mutex is unlocked even if an error occurs
		for _, player := range players {
			player.WriteChan <- finalBytes // Send message to the write channel
		}
	})
}

func unsubscribeFromPlayerChannel(player *models.Player) {
	redisClient.UnsubscribePlayerChannel(*player)
}

func FetchAndValidateSession(token, sessionID, currency string) (*models.Session, error) {
	session, err := redisClient.GetSessionByID(sessionID)
	if err != nil {
		log.Printf("[FetchAndValidateSession] - Error fetching session from Redis: %v\n", err)
		return nil, fmt.Errorf("[Session] - failed to fetch session: %v", err)
	}
	//log.Printf("[FetchAndValidateSession] - Session fetched from Redis: %+v\n", session)

	if session.Currency != currency {
		log.Printf("[FetchAndValidateSession] - Currency mismatch: expected %s, got %s\n", currency, session.Currency)
		return nil, fmt.Errorf("[Session] - currency mismatch: expected %s, got %s", currency, session.Currency)
	}
	//log.Printf("[FetchAndValidateSession] - Currency validation successful\n")

	if session.Token != token {
		log.Printf("[FetchAndValidateSession] - Token mismatch: expected %s, got %s\n", token, session.Token)
		return nil, fmt.Errorf("[Session] - token mismatch")
	}
	if session.OperatorIdentifier.OperatorName == "TestOp" {
		return session, nil
	}
	//log.Printf("[FetchAndValidateSession] - Token validation successful\n")
	if session.IsTokenExpired() {
		log.Printf("[FetchAndValidateSession] - Token expired\n")
		return nil, fmt.Errorf("[Session] - token expired")
	}
	//log.Printf("[FetchAndValidateSession] - Session validation successful: %+v\n", session)
	return session, nil
}
