package main

import (
	"checkers-server/config"
	"checkers-server/messages"
	"checkers-server/models"
	"checkers-server/redisdb"
	"fmt"
	"log"
	"os"
)

var pid int
var redisClient *redisdb.RedisClient

func init() {
	pid = os.Getpid()
	config.LoadConfig()
	redisAddr := config.Cfg.Redis.Addr
	client, err := redisdb.NewRedisClient(redisAddr)
	if err != nil {
		log.Fatalf("[Redis] Error initializing Redis client: %v\n", err)
	}
	redisClient = client
}

func main() {
	fmt.Printf("[PStatus Worker-%d] - Waiting for player connections...\n", pid)
	go processPlayerOnline()
	go processPlayerOffline()

	select {}
}

func processPlayerOnline(){
	for {
		playerData, err := redisClient.BLPop("player_online", 0) // Block
		if err != nil {
			fmt.Printf("[PStatus Worker-%d] - Error retrieving player:%v\n", pid, err)
			continue
		}
		fmt.Printf("[PStatus Worker-%d] - Player connected: %+v\n", pid, playerData)
		handleNewPlayer(playerData)
	}
}


func processPlayerOffline(){
	for {
		playerData, err := redisClient.BLPop("player_offline", 0) // Block
		if err != nil {
			fmt.Printf("[PStatus Worker-%d] - Error retrieving player: %d\n", pid, err)
			continue
		}
		fmt.Printf("[PStatus Worker-%d] - Player disconnected: %+v\n", pid, playerData)
		playerData, err = redisClient.GetPlayer(playerData.ID)
		if err == nil {
			handleRemovePlayer(playerData)	
		}
	}
}

func handleRemovePlayer(player *models.Player) {
	fmt.Printf("[PStatus Worker-%d] - Removing player: %s (Session: %s, Currency: %s, RoomID: %s)\n",
		pid, player.ID, player.SessionID, player.Currency, player.RoomID)
	
	// We dont need to issue a command to leave the queue, since the queue fetched the player-
	if(player.RoomID != "" || player.Status == models.StatusInRoom || player.Status == models.StatusInRoomReady){
		fmt.Printf("[PStatus Worker-%d] - Removed player is in a Room, sending notification to room worker!: %v\n", pid, player)
		redisClient.RPush("leave_room", player)
	}
	if(player.GameID != "" || player.Status == models.StatusInGame){
		fmt.Printf("[PStatus Worker-%d] - Removed player is in a Game, sending notification to Game worker!: %v\n", pid, player)
		redisClient.RPush("disconnect_game", player)
	}

	err := redisClient.RemovePlayer(string(player.ID))
	if err != nil {
		fmt.Printf("[PStatus Worker-%d] - Failed to remove player: %v\n", pid, err)
		return
	}
	fmt.Printf("[PStatus Worker-%d] - Player successfully removed.\n", pid)
}

func handleNewPlayer(player *models.Player) {
	fmt.Printf("[PStatus Worker-%d] - Handling player: %s (Session: %s, Currency: %s)\n",
		pid, player.ID, player.SessionID, player.Currency)

	err := redisClient.AddPlayer(player)
	if err != nil {
		fmt.Printf("[PStatus Worker-%d] - Failed to add player: %v\n", pid, err)
		return
	}
	jsonMessage, err := messages.GenerateConnectedMessage(*player)
	err = redisClient.PublishToPlayer(*player, string(jsonMessage))
	if err != nil {
		fmt.Printf("[PStatus Worker-%d] - Failed to publish message to player: %v\n", pid, err)
		return
	}
	fmt.Printf("[PStatus Worker-%d] - Player successfully handled and notified.\n", pid)
}

func updatePlayerToRedis(player *models.Player) {
	
	err := redisClient.UpdatePlayer(player)
	if err != nil {
		fmt.Printf("[PStatus Worker-%d] - Failed to add player: %v\n", pid, err)
		return
	}
	jsonMessage, err := messages.GenerateConnectedMessage(*player)
	err = redisClient.PublishToPlayer(*player, string(jsonMessage))
	if err != nil {
		fmt.Printf("[PStatus Worker-%d] - Failed to publish message to player: %v\n", pid, err)
		return
	}
	fmt.Printf("[PStatus Worker-%d] - Player successfully handled and notified.\n", pid)
}