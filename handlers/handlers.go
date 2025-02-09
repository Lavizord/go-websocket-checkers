package handlers

import (
	"checkers-server/core"
	"checkers-server/message"
	"checkers-server/redisdb"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var Mutex sync.Mutex

var checkersDb = redisdb.NewRedisClient("localhost:6379", "", 0)

// This should handle our initial connection. Then handlePlayerMessages() should do most of the work.
func HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Get playerID from query params
	playerId := r.URL.Query().Get("playerId")
	if playerId == "" {
		fmt.Println("[CLI] Missing playerId on connection.")
		http.Error(w, "Missing playerId", http.StatusBadRequest)
		return
	}
	playerIdInt, err := strconv.Atoi(playerId)
	if err != nil {
		fmt.Println("[CLI] Invalid playerId:", playerId)
		http.Error(w, "Invalid playerId", http.StatusBadRequest)
		return
	}
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WebSocket upgrade failed:", err)
		return
	}
	
	player := &core.Player{Id: playerIdInt, Conn: conn, Money: 999, Name: r.RemoteAddr}
	//if !handlePossibleReconnection(player) {
		checkersDb.AddPlayer(r.Context(), player)
	//}
	fmt.Println("New player connected:", player.Id)
	go handlePlayerMessages(player, r);
}


func handlePlayerMessages(player *core.Player, r *http.Request) {
	conn := player.Conn
	jsonMessage, err := message.GenerateConnectedMessage(player)
	if err != nil {
		fmt.Println("Error generating message:", err)
		return
	}
	conn.WriteMessage(websocket.TextMessage, []byte(jsonMessage))

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			HandleDisconnection(player, player.Room.GetOpponent(player))
			return
		}
		message, err := message.ParseMessage(msg, conn)
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte("Invalid message format."))
			continue
		}

		// Since we have a valid message, process it based on the command
		switch message.Command {
			case "leave_queue":
				handleLeaveQueue(player)

			case "join_queue":
				handleJoinQueue(player, message, r)

			case "move_piece":
				handleMovePiece(player, message)

			default:
				// Handle unrecognized command, or log it
				fmt.Println("Unknown command:", message.Command)
		}
	}
}

func handleLeaveQueue(player *core.Player) {
	checkersDb.RemoveFromQueue(player);
	core.RemoveFromQueue(player);
	player.Conn.WriteMessage(websocket.TextMessage, []byte("You left the Queue!..."))
}

func handleJoinQueue(player *core.Player, message *message.Message, r *http.Request) {
	var selectedBid float64
	if err := json.Unmarshal(message.Value, &selectedBid); err != nil {
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid bid value."))
		return
	}

	if core.IsPlayerInQueue(player) {
		player.Conn.WriteMessage(websocket.TextMessage, []byte("You are already in a Queue!..."))
		return
	} 

	player.SelectedBid = selectedBid
	checkersDb.AddToQueue(player)
	core.AddToQueue(player)

	// not enough players to check for a match.
	if len(core.WaitingQueue) < 2 {
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Waiting for an opponent..."))
		return
	}

	// Lets filter the queue to try to find a match
	filteredQueue := core.FilterWaitingQueue(core.WaitingQueue, func(player *core.Player) bool {
		return player.SelectedBid == selectedBid
	})

	// No two players with the same bet waiting...
	if len(filteredQueue) < 2 {
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Waiting for an opponent..."))
		return
	}
	// if there are we move on to creating our room for the match.
	handleRoomCreation(filteredQueue)
}

func handleRoomCreation(filteredQueue []*core.Player) {
	// Created room withh the first two players of the queue.
	room := core.CreateRoom(filteredQueue[0], filteredQueue[1]);
	// remove them from the Queue (!) TODO: Remove old method.
	checkersDb.RemoveFromQueue(room.Player1);
	checkersDb.RemoveFromQueue(room.Player2);
	core.RemoveFromQueue(room.Player1);
	core.RemoveFromQueue(room.Player2);
	room.Player1.Conn.WriteMessage(websocket.TextMessage, []byte(message.GeneratePairedMessage(room.Player1, room.Player2, 1)))
	room.Player1.Color = 1
	room.Player2.Conn.WriteMessage(websocket.TextMessage, []byte(message.GeneratePairedMessage(room.Player2, room.Player1, 0)))
	room.Player2.Color = 0

}

// For now we just send the update to the oponent.
func handleMovePiece(player *core.Player, message *message.Message) {
	if player.Room != nil {
		player.Room.GetOpponent(player).Conn.WriteMessage(websocket.TextMessage, []byte(message.Value))
		return
	} 
	player.Conn.WriteMessage(websocket.TextMessage, []byte("You are not in a Game!..."))
}

// Also monitors for a disconnect
func HandleDisconnection(player *core.Player, opponent *core.Player) {
	// The player identity should be checked and preserved for reconnection.
	//reconnected := make(chan bool, 1) // Make the channel buffered to prevent blocking.

	// Start the goroutine to monitor the reconnection.
	//go waitForReconnection(player, reconnected)

	// Wait for either reconnection or timeout (20 seconds).
	//select {
	//case <-reconnected:
	//	// Player reconnected successfully, nothing more to do.
	//	fmt.Println("Player reconnected successfully.")
	//	return

	//case <-time.After(20 * time.Second):
		// Timeout reached, player did not reconnect.
		// Shut down the room if no reconnection.
		if player.Room == nil {
			core.RemoveFromQueue(player) // Remove from queue if not in room.
		} else {
			opponent.Conn.WriteMessage(websocket.TextMessage, []byte("Opponent disconnected."))
			core.RemoveRoom(player.Room) // Remove the room.
		}
		core.RemovePlayer(player) // Remove player from the system.
		checkersDb.RemovePlayer(player)
	//}
}


// Wait for the player to reconnect, checking every 5 seconds.
func waitForReconnection(player *core.Player, reconnected chan bool) {
	// Check every 5 seconds if the player has reconnected.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if the player has reconnected.
			if player.Conn != nil && player.Room != nil {
				// Successfully reconnected, notify and resume game.
				player.Conn.WriteMessage(websocket.TextMessage, []byte("Reconnected successfully!"))
				reconnected <- true // Send signal that player has reconnected.
				return
			}
		}
	}
}


// On player reconnect, identify the player based on their name and restore state. Returns true if it was a reconnect.
func handlePossibleReconnection(player *core.Player) (bool) {
	existingPlayer := core.FindPlayerByName(player.Name)
	if existingPlayer == nil {
		return false
	} 
	// Player found, restore their previous state (queue or room)
	if existingPlayer.Room != nil {
		// Rejoin the same room
		player.Room = existingPlayer.Room
		// TODO: I need to assgin this player to the right room in the right order...
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Reconnected to the same room!"))
		return true
	}
	return false
}
