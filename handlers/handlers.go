package handlers

import (
	"checkers-server/core"
	"checkers-server/message"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var Mutex sync.Mutex

// This should handle our initial connection. Then handlePlayerMessages() should do most of the work.
func HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WebSocket upgrade failed:", err)
		return
	}

	player := &core.Player{Conn: conn, Money: 999, Name: r.RemoteAddr}
	
	core.AddPlayer(player)
	fmt.Println("New player connected:", player.Name)
	
	go handlePlayerMessages(player);
}


func handlePlayerMessages(player *core.Player) {
	// TODO: does this need to be sent to the player? Where do we get these values?
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
				handleLeaveQueue(player, message)

			case "join_queue":
				handleJoinQueue(player, message)

			case "move_piece":
				handleMovePiece(player, message)

			default:
				// Handle unrecognized command, or log it
				fmt.Println("Unknown command:", message.Command)
		}
	}
}

func handleLeaveQueue(player *core.Player, message *message.Message) {
	core.RemoveFromQueue(player);
	player.Conn.WriteMessage(websocket.TextMessage, []byte("You left the Queue!..."))
}

func handleJoinQueue(player *core.Player, message *message.Message) {
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
	// remove them from the Queue (!)
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

func HandleDisconnection(player *core.Player, opponent *core.Player) {
	// So when a player disconnects he can be:
	// - In a Queue, and not in a room.
	// - In a Room, and not in a Queue.
	// This means, we will need to handle it diferently.

	if player.Room == nil{
		core.RemoveFromQueue(player)	// We remove it from the Queue, it might now always be there tho. Dont think its an issue.
	} else {
		opponent.Conn.WriteMessage(websocket.TextMessage, []byte("Opponent disconnected."))		
		core.RemoveRoom(player.Room); 	// If there is a room, we need to remove it.
	}
	core.RemovePlayer(player)			// Either way, we need to remove the player from the room.
}
