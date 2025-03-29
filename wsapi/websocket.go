package wsapi

import (
	"checkers-server/config"
	"checkers-server/interfaces"
	"checkers-server/messages"
	"checkers-server/models"
	"checkers-server/redisdb"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

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
	log.Printf("[wsapi] - HandleConnection: token[%v], sessionid[%v], currency[%v]\n", token, sessionID, currency)
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
		fmt.Println("Failed to upgrade:", err)
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
		playerID := models.GenerateUUID()
		newPlayer := &models.Player{
			ID:                 playerID,
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
	player.StartWriteGoroutine() // Start the write goroutine

	// We add the player to our player map.
	playersMutex.Lock()
	players[player.ID] = player
	playersMutex.Unlock()

	subscriptionReady := make(chan bool)
	go subscribeToPlayerChannel(player, subscriptionReady)
	<-subscriptionReady // Wait for the subscription to be ready
	module, _ := interfaces.OperatorModules[player.OperatorIdentifier.OperatorName]
	walletBalance, err := module.HandleFetchWalletBalance(*session, redisClient)
	if err != nil {
		log.Printf("Failed to fetch wallet : %v", err)
		return
	}
	msg, err := messages.GenerateConnectedMessage(*player, walletBalance)
	if err != nil {
		log.Printf("Failed to fetchgenerate connected message : %v", err)
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
func subscribeToPlayerChannel(player *models.Player, ready chan bool) {
	redisClient.SubscribePlayerChannel(*player, func(message string) {
		fmt.Println("[wsapi] - Received server to PLAYER message:", message)
		// Send the received message to the player's WebSocket connection
		player.WriteChan <- []byte(message)
	})
	ready <- true // Notify that the subscription is ready
}

func subscribeToBroadcastChannel() {
	redisClient.Subscribe("game_info", func(message string) {
		fmt.Println("[wsapi] - Received BROADCAST message:", message)
		// Step 1: Parse the message using messages.ParseMessage
		msg, err := messages.ParseMessage([]byte(message))
		if err != nil {
			fmt.Println("[wsapi] - Failed to parse message:", err)
			return
		}
		// Step 2: Marshal the message back to JSON
		finalBytes, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("[wsapi] - Failed to marshal final message:", err)
			return
		}
		// Step 3: Send the message to all connected players
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

func unsubscribeFromBroadcastChannel(player *models.Player) {
	redisClient.Unsubscribe("game_info")
}

// Mock user validation
func IsUserValid(token string, sessionID string) (bool, models.Player) {
	player, exists := redisdb.MockPlayers[token]
	if exists && player.SessionID == sessionID {
		return true, player
	}
	return false, models.Player{} // Invalid user
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
	//log.Printf("[FetchAndValidateSession] - Token validation successful\n")
	if session.IsTokenExpired() {
		log.Printf("[FetchAndValidateSession] - Token expired\n")
		return nil, fmt.Errorf("[Session] - token expired")
	}
	//log.Printf("[FetchAndValidateSession] - Session validation successful: %+v\n", session)
	return session, nil
}
