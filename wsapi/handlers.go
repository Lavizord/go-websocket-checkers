package wsapi

import (
	"checkers-server/messages"
	"checkers-server/models"
	"encoding/json"
	"fmt"
	"log"
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
		log.Printf("Message from %s: %s\n", player.ID, string(msg))
		UpdatePlayerDataFromRedis(player)

		// Process the received message (expecting JSON), this will read the command but leave the value.
		message, err := messages.ParseMessage(msg)
		if err != nil {
			player.WriteChan <- []byte("Invalid message format." + err.Error())
			continue
		}

		// Directly route to the right handler based on the command
		switch message.Command {
		case "queue":
			handleQueue(message, player)

		case "leave_queue":
			handleLeaveQueue(player)

		case "ready_queue":
			handleReadyQueue(message, player)

		case "leave_room":
			if player.Status != models.StatusInRoom {
				player.WriteChan <- []byte("Can't issue a leave room when not in a Room.")
				continue
			}
			handleLeaveRoom(player)

		case "leave_game":
			if player.Status != models.StatusInGame {
				player.WriteChan <- []byte("Can't issue a leave game when not in a game.")
				continue
			}
			handleLeaveGame(player)

		case "move_piece":
			if player.Status != models.StatusInGame {
				player.WriteChan <- []byte("Can't issue a move when not in a Game.")
				continue
			}
			handleMovePiece(message, player)
		}
	}
}

func handleQueue(msg *messages.Message[json.RawMessage], player *models.Player) {
	qh := &QueueHandler{
		player:      player,
		redisClient: redisClient,
		msg:         msg,
	}
	qh.process()
}

func handleLeaveQueue(player *models.Player) {
	if player.UpdatePlayerStatus(models.StatusOnline) != nil {
		player.WriteChan <- []byte("Invalid status transition to 'Online'")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid status transition to 'Online'"))
		return
	}
	redisClient.UpdatePlayersInQueueSet(player.ID, models.StatusOnline)
	redisClient.UpdatePlayer(player) // This is important, we will only re-add players to a queue that are in queue.
	queueName := fmt.Sprintf("queue:%f", player.SelectedBet)
	err := redisClient.RemovePlayerFromQueue(queueName, player)
	if err != nil {
		log.Printf("Error removing player from Redis queue: %v\n", err)
		player.WriteChan <- []byte("Error removing player to queue")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("Error removing player to queue"))
		return
	}
	// we update out player status.
	// send a confirmation message back to the player
	m, err := messages.GenerateQueueConfirmationMessage(false)
	if err != nil {
		fmt.Println("Error generating queue confirmation false:", err)
		player.WriteChan <- []byte("Error generating confirmation")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("Error generating confirmation"))
		return
	}
	player.WriteChan <- m
	//player.Conn.WriteMessage(websocket.TextMessage, m)
}

func handleReadyQueue(msg *messages.Message[json.RawMessage], player *models.Player) {
	// We have to check if the message for readyqueue true or false.
	var value bool
	json.Unmarshal(msg.Value, &value)
	if value {
		// update the player status to ready / awaiting opponent.
		if player.UpdatePlayerStatus(models.StatusInRoomReady) != nil {
			player.WriteChan <- []byte("Invalid status transition to 'ready_queue true'")
			//player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid status transition to 'ready_queue true'"))
			return
		}
	} else {
		// update the player status, to unready / waiting ready.
		if player.UpdatePlayerStatus(models.StatusInRoom) != nil {
			player.WriteChan <- []byte("Invalid status transition to 'ready_queue false'")
			//player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid status transition to 'ready_queue false'"))
			return
		}
	}
	player.WriteChan <- []byte("processing 'ready_queue'")
	//player.Conn.WriteMessage(websocket.TextMessage, []byte("processing 'ready_queue'"))
	// we update our player to redis.
	redisClient.UpdatePlayersInQueueSet(player.ID, player.Status)
	err := redisClient.UpdatePlayer(player)
	if err != nil {
		log.Printf("Error adding player to Redis: %v\n", err)
		player.WriteChan <- []byte("Error adding player")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player"))
		return
	}
	err = redisClient.RPush("ready_queue", player) // now we tell roomworker to process this player ready.
	if err != nil {
		log.Printf("Error pushing player to Redis ready queue: %v\n", err)
		player.WriteChan <- []byte("Error adding player to queue")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player to queue"))
		return
	}

}

func handleLeaveRoom(player *models.Player) {
	// update the player status
	if player.UpdatePlayerStatus(models.StatusOnline) != nil {
		player.WriteChan <- []byte("Invalid status transition to 'leave_room'")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid status transition to 'leave_room'"))
		return
	}
	player.WriteChan <- []byte("processing 'leave_room'")
	//player.Conn.WriteMessage(websocket.TextMessage, []byte("processing 'leave_room'"))
	// we update our player to redis.
	err := redisClient.UpdatePlayer(player)
	if err != nil {
		log.Printf("Error adding player to Redis: %v\n", err)
		player.WriteChan <- []byte("Error adding player")
		// player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player"))
		return
	}
	err = redisClient.RPush("leave_room", player)
	if err != nil {
		log.Printf("Error pushing player to Redis leave_room queue: %v\n", err)
		player.WriteChan <- []byte("Error adding player to queue")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player to queue"))
		return
	}

}

func handleLeaveGame(player *models.Player) {
	// update the player status
	//if player.UpdatePlayerStatus(models.StatusOnline) != nil {
	//	player.WriteChan <- []byte("Invalid status transition to 'leave_game'")
	//	return
	//}
	player.WriteChan <- []byte("processing 'leave_game'")

	//err := redisClient.UpdatePlayer(player)
	//if err != nil {
	//	log.Printf("Error adding player to Redis: %v\n", err)
	//	player.WriteChan <- []byte("Error adding player")
	//	return
	//}
	err := redisClient.RPush("leave_game", player)
	if err != nil {
		log.Printf("Error pushing player to Redis leave_game queue: %v\n", err)
		player.WriteChan <- []byte("Error adding player to leave_game")
		return
	}

}

func handleMovePiece(message *messages.Message[json.RawMessage], player *models.Player) {
	var move models.Move
	err := json.Unmarshal([]byte(message.Value), &move)
	if err != nil {
		log.Printf("[Handlers] - Handle Move Piece - JSON Unmarshal Error: %v\n", err)
		player.WriteChan <- []byte("[Handlers] - Handle Move Piece - JSON Unmarshal Error")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("[Handlers] - Handle Move Piece - JSON Unmarshal Error"))
		return
	}
	if move.PlayerID != player.ID {
		log.Printf("[Handlers] - Handle Move Piece - move.PlayerID != player.ID\n")
		player.WriteChan <- []byte("[Handlers] - Handle Move Piece - move.PlayerID != player.ID")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("[Handlers] - Handle Move Piece - move.PlayerID != player.ID"))
		return
	}
	// movement message is sent to the game worker
	err = redisClient.RPushGeneric("move_piece", message.Value)
	if err != nil {
		log.Printf("Error pushing move to Redis handleMovePiece queue: %v\n", err)
		player.WriteChan <- []byte("Error adding player to queue")
		//player.Conn.WriteMessage(websocket.TextMessage, []byte("Error adding player to queue"))
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
		log.Printf("[Handlers] - Failed to update player data from redis!: Player: %s", player.ID)
		return
	}
	player.Currency = playerData.Currency
	player.Status = playerData.Status
	player.SelectedBet = playerData.SelectedBet
	player.RoomID = playerData.RoomID
}
