package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Lavizord/checkers-server/internal/messages"
	"github.com/Lavizord/checkers-server/internal/models"
)

func handleMessages(player *models.Player) {
	defer player.Conn.Close()

	player.Conn.SetReadDeadline(time.Now().Add(pongWait))
	player.Conn.SetPongHandler(func(string) error {
		//log.Println("[wsapi] - Pong received from client")
		player.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		if player.Conn == nil {
			UpdatePlayerDataFromRedis(player)
			handlePlayerDisconnect(player)
			break
		}
		_, msg, err := player.Conn.ReadMessage()
		if err != nil {
			UpdatePlayerDataFromRedis(player)
			handlePlayerDisconnect(player)
			break
		}
		UpdatePlayerDataFromRedis(player)

		message, err := messages.ParseMessage(msg)
		if err != nil {
			msg, _ = messages.GenerateGenericMessage("error", "Invalid message format."+err.Error())
			player.WriteChan <- msg
			continue
		}
		if message.Command == "ping" {
			msg := messages.MessageSimple{
				Command: "pong",
			}
			msgBytes, _ := json.Marshal(msg)
			player.WriteChan <- msgBytes
		}

		routeMessages(message, player)
	}
}

func routeMessages(message *messages.Message[json.RawMessage], player *models.Player) {
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
			msg, _ := messages.GenerateGenericMessage("invalid", "Can't issue a leave room when not in a Room.")
			player.WriteChan <- msg
			return
		}
		handleLeaveRoom(player)

	case "leave_game":
		if player.Status != models.StatusInGame {
			msg, _ := messages.GenerateGenericMessage("invalid", "Can't issue a leave game when not in a game.")
			player.WriteChan <- msg
			return
		}
		handleLeaveGame(player)

	case "move_piece":
		if player.Status != models.StatusInGame {
			msg, _ := messages.GenerateGenericMessage("invalid", "Can't issue a move when not in a Game.")
			player.WriteChan <- msg
			return
		}
		handleMovePiece(message, player)
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
		msg, _ := messages.GenerateGenericMessage("invalid", "Invalid status transition to 'Online'")
		player.WriteChan <- msg
		return
	}
	player.SetStatusOnline()
	redisClient.UpdatePlayer(player) // This is important, we will only re-add players to a queue that are in queue.
	redisClient.UpdatePlayersInQueueSet(player.ID, models.StatusOnline)
	queueName := fmt.Sprintf("queue:%f", player.SelectedBet)
	err := redisClient.RemovePlayerFromQueue(queueName, player)
	if err != nil {
		//log.Printf("Error removing player from Redis queue: %v\n", err)	//! This was commented, it fails when there is only 1 player in queue.
		//player.WriteChan <- []byte("Error removing player to queue")
		return
	}
	// we update out player status., send a confirmation message back to the player
	m, err := messages.GenerateQueueConfirmationMessage(false)
	if err != nil {
		msg, _ := messages.GenerateGenericMessage("error", "Error generating queue confirmation false:"+err.Error())
		log.Println("Error generating queue confirmation false:", err)
		player.WriteChan <- msg
		return
	}
	player.WriteChan <- m
}

func handleReadyQueue(msg *messages.Message[json.RawMessage], player *models.Player) {
	// We have to check if the message for readyqueue true or false.
	var value bool
	json.Unmarshal(msg.Value, &value)
	if value {
		// update the player status to ready / awaiting opponent.
		if player.UpdatePlayerStatus(models.StatusInRoomReady) != nil {
			msg, _ := messages.GenerateGenericMessage("invalid", "Invalid status transition to 'ready_queue true'")
			player.WriteChan <- msg
			return
		}
	} else {
		// update the player status, to unready / waiting ready.
		if player.UpdatePlayerStatus(models.StatusInRoom) != nil {
			msg, _ := messages.GenerateGenericMessage("invalid", "Invalid status transition to 'ready_queue false'")
			player.WriteChan <- msg
			return
		}
	}
	msgBytes, _ := messages.GenerateGenericMessage("info", "Processing 'ready_queue'")
	player.WriteChan <- msgBytes
	// we update our player to redis.
	redisClient.UpdatePlayersInQueueSet(player.ID, player.Status)
	err := redisClient.UpdatePlayer(player)
	if err != nil {
		log.Printf("Error updating player to Redis: %v\n", err)
		msgBytes, _ := messages.GenerateGenericMessage("error", "Error Updating player: "+err.Error())
		player.WriteChan <- msgBytes
		return
	}
	err = redisClient.RPush("ready_queue", player) // now we tell roomworker to process this player ready.
	if err != nil {
		log.Printf("Error pushing player to Redis ready queue: %v\n", err)
		msgBytes, _ := messages.GenerateGenericMessage("error", "Pushing player to queue: "+err.Error())
		player.WriteChan <- msgBytes
		return
	}
}

func handleLeaveRoom(player *models.Player) {
	// update the player status
	if player.UpdatePlayerStatus(models.StatusOnline) != nil {
		msgBytes, _ := messages.GenerateGenericMessage("invalid", "Invalid status transition to 'leave_room'")
		player.WriteChan <- msgBytes
		return
	}
	msgBytes, _ := messages.GenerateGenericMessage("info", "Processing 'leave_room'")
	player.WriteChan <- msgBytes
	// we update our player to redis.
	err := redisClient.UpdatePlayer(player)
	if err != nil {
		log.Printf("Error updagint player to Redis on HandleLeaveRoom: %v\n", err)
		msg, _ := messages.GenerateGenericMessage("error", "error updating player.")
		player.WriteChan <- msg
		return
	}
	err = redisClient.RPush("leave_room", player)
	if err != nil {
		log.Printf("Error pushing player to Redis leave_room queue: %v\n", err)
		msg, _ := messages.GenerateGenericMessage("error", "error leaving room.")
		player.WriteChan <- msg
		return
	}
}

func handleLeaveGame(player *models.Player) {
	msgBytes, _ := messages.GenerateGenericMessage("invalid", "Processing 'leave_game'")
	player.WriteChan <- msgBytes
	err := redisClient.RPush("leave_game", player)
	if err != nil {
		log.Printf("Error pushing player to Redis leave_game queue: %v\n", err)
		msgBytes, _ := messages.GenerateGenericMessage("error", "Error adding player to leave_game")
		player.WriteChan <- msgBytes
		return
	}
}

func handleMovePiece(message *messages.Message[json.RawMessage], player *models.Player) {
	var move models.Move
	err := json.Unmarshal([]byte(message.Value), &move)
	if err != nil {
		log.Printf("[Handlers] - Handle Move Piece - JSON Unmarshal Error: %v\n", err)
		msg, _ := messages.GenerateGenericMessage("invalid", "Handle Move Piece - JSON Unmarshal Error.")
		player.WriteChan <- msg
		return
	}
	if move.PlayerID != player.ID {
		log.Printf("[Handlers] - Handle Move Piece - move.PlayerID != player.ID\n")
		msg, _ := messages.GenerateGenericMessage("invalid", "Handle Move Piece - move.PlayerID != player.ID.")
		player.WriteChan <- msg
		return
	}
	// movement message is sent to the game worker
	err = redisClient.RPushGeneric("move_piece", message.Value)
	if err != nil {
		log.Printf("Error pushing move to Redis handleMovePiece queue: %v\n", err)
		msg, _ := messages.GenerateGenericMessage("error", "error pushing move to gameworker.")
		player.WriteChan <- msg
		return
	}
}

func handlePlayerDisconnect(player *models.Player) {
	//log.Println("Player disconnected:", player.ID)
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
