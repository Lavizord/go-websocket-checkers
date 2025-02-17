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
			UpdatePlayerDataFromRedis(player)		// We are updating our player only when there are any new messages... Not sure if its the best aproach.
			handlePlayerDisconnect(player)
			break
		}
		fmt.Printf("Message from %s: %s\n", player.ID, string(msg))
		UpdatePlayerDataFromRedis(player)

		// Process the received message (expecting JSON)
		message, err := messages.ParseMessage(msg)
		if err != nil {
			player.Conn.WriteMessage(websocket.TextMessage, []byte("Invalid message format."+err.Error()))
			continue
		}
		// We update the player bidValue. This is so our RPUSH seends the player setAmount.
		if message.Command == "create_room" || message.Command == "join_room" {
			var bidValue float64
			err := json.Unmarshal(message.Value, &bidValue)
			if err != nil {
				fmt.Println("Error unmarshaling bid value:", err)
				return
			}
			player.SelectedBid = bidValue
		}
		// Now we push the command to our worker, he will determine what to do with the message.
		err = redisClient.RPush(message.Command, player)
		if err != nil {
			fmt.Printf("Error pushing message to Redis: %v\n", err)
		} else {
			// Print what we're sending to Redis
			fmt.Printf("Sending message to Redis - Command: %s, Player: %s, Bid: %f\n", message.Command, player.ID, player.SelectedBid)
		}
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
	player.SelectedBid = playerData.SelectedBid
	player.RoomID = playerData.RoomID
}