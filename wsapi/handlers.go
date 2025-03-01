package wsapi

import (
	"checkers-server/messages"
	"checkers-server/models"
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
)

func handleMessages(player *models.Player) {
	defer player.Conn.Close()
	for {
		_, msg, err := player.Conn.ReadMessage()
		if err != nil {
			UpdatePlayerDataFromRedis(player)
			handlePlayerDisconnect(player)
			break
		}
		fmt.Printf("Message from %s: %s\n", player.ID, string(msg))
		UpdatePlayerDataFromRedis(player)

		// Process the received message (expecting JSON), this will read the command but leave the value.
		message, err := messages.ParseMessage(msg)
		if err != nil {
			player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid message format."+err.Error()))
			continue
		}

		// Directly route to the right handler based on the command
		switch message.Command {
		case "queue":
			handleQueue(message, player)

		case "leave_queue":
			handleLeaveQueue(message, player)

		case "ready_queue":
			handleReadyQueue(message, player)

		case "leave_room":
			if player.Status != models.StatusInRoom {
				player.Conn.WriteMessage(websocket.TextMessage, []byte("Can't issue a leave room when not in a room."))
				continue
			}
			handleLeaveRoom(player)
		
		case "move_piece":
			handleMovePiece(message, player)
		}
		
		
	}
}

func handleQueue(msg *messages.Message[json.RawMessage], player *models.Player) {
	if player.UpdatePlayerStatus(models.StatusInQueue) != nil {
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid status transition to 'queue'"))
		return
	}
	// update the player bet and push it to Redis
	var betValue float64
	err := json.Unmarshal(msg.Value, &betValue)
	if err != nil {
		fmt.Printf("Error determinng player bet value: %v\n", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error determinng player bet value"))
		return
	}

	player.SelectedBet = betValue
	player.Status = models.StatusInQueue

	// Pushing the player to the "queue" Redis list
	queueName := fmt.Sprintf("queue:%f", betValue) 
	err = redisClient.RPush(queueName, player) //
	if err != nil {
		fmt.Printf("Error pushing player to Redis queue: %v\n", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player to queue"))
		return
	}
	redisClient.UpdatePlayersInQueueSet(player.ID, models.StatusInQueue)
	// we update out player status.
	redisClient.AddPlayer(player)

	// send a confirmation message back to the player
	m, err := messages.GenerateQueueConfirmationMessage(true)
	if err != nil {
		fmt.Println("Error generating queue confirmation:", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error generating confirmation"))
		return
	}
	player.Conn.WriteMessage(websocket.TextMessage, m)
}

func handleLeaveQueue(msg *messages.Message[json.RawMessage], player *models.Player) {
	if player.UpdatePlayerStatus(models.StatusOnline) != nil {
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid status transition to 'Online'"))
		return
	}
	redisClient.UpdatePlayersInQueueSet(player.ID, models.StatusOnline)
	redisClient.AddPlayer(player)	// This is important, we will only re-add players to a queue that are in queue.
	queueName := fmt.Sprintf("queue:%f", player.SelectedBet) 
	err := redisClient.RemovePlayerFromQueue(queueName, player) 
	if err != nil {
		fmt.Printf("Error removing player from Redis queue: %v\n", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error removing player to queue"))
		return
	}
	// we update out player status.
	// send a confirmation message back to the player
	m, err := messages.GenerateQueueConfirmationMessage(false)
	if err != nil {
		fmt.Println("Error generating queue confirmation false:", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error generating confirmation"))
		return
	}
	player.Conn.WriteMessage(websocket.TextMessage, m)
}

func handleReadyQueue(msg *messages.Message[json.RawMessage], player *models.Player) {
	// We have to check if the message for readyqueue true or false.
	var value bool
	json.Unmarshal(msg.Value, &value)
	if value {
		// update the player status to ready / awaiting opponent.
		if player.UpdatePlayerStatus(models.StatusInRoomReady) != nil {
			player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid status transition to 'ready_queue true'"))
			return
		}
	} else {
		// update the player status, to unready / waiting ready.
		if player.UpdatePlayerStatus(models.StatusInRoom) != nil {
			player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid status transition to 'ready_queue false'"))
			return
		}
	}
	player.Conn.WriteMessage(websocket.TextMessage, []byte("processing 'ready_queue'"))
	// we update our player to redis.
	redisClient.UpdatePlayersInQueueSet(player.ID, player.Status)
	err := redisClient.AddPlayer(player)
	if err != nil {
		fmt.Printf("Error adding player to Redis: %v\n", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player"))
		return
	}
	err = redisClient.RPush("ready_queue", player) // now we tell roomworker to process this player ready.
	if err != nil {
		fmt.Printf("Error pushing player to Redis ready queue: %v\n", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player to queue"))
		return
	}

}

func handleLeaveRoom(player *models.Player) {
	// update the player status
	if player.UpdatePlayerStatus(models.StatusOnline) != nil {
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid status transition to 'leave_room'"))
		return
	}
	player.Conn.WriteMessage(websocket.TextMessage, []byte("processing 'leave_room'"))
	// we update our player to redis.
	err := redisClient.AddPlayer(player)
	if err != nil {
		fmt.Printf("Error adding player to Redis: %v\n", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player"))
		return
	}
	err = redisClient.RPush("leave_room", player) 
	if err != nil {
		fmt.Printf("Error pushing player to Redis leave_room queue: %v\n", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player to queue"))
		return
	}

}

func handleMovePiece(message *messages.Message[json.RawMessage], player *models.Player) {
	// TODO: Validate we can move. 
	// Currently the movement message is just being sent to the game worker
	err := redisClient.RPushGeneric("move_piece", message.Value) 
	if err != nil {
		fmt.Printf("Error pushing move to Redis handleMovePiece queue: %v\n", err)
		player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player to queue"))
		return
	}
}

func handlePlayerDisconnect(player *models.Player) {
	fmt.Println("Player disconnected:", player.ID)
	playersMutex.Lock()
	delete(players, player.ID)
	playersMutex.Unlock()
	// Unsubscribe from Redis channels
	unsubscribeFromPlayerChannel(player)
	// Notify worker of disconnection
	redisClient.UpdatePlayersInQueueSet(player.ID, models.StatusOffline)
	redisClient.RPush("player_offline", player)
}

func UpdatePlayerDataFromRedis(player *models.Player) {
	playerData, err := redisClient.GetPlayer(string(player.ID))
	if err != nil {
		fmt.Printf("[Handlers] - Failed to update player data from redis!: Player: %s", player.ID)
		return
	}
	player.Currency = playerData.Currency
	player.CurrencyAmount = playerData.CurrencyAmount
	player.Status = playerData.Status
	player.SelectedBet = playerData.SelectedBet
	player.RoomID = playerData.RoomID
}
