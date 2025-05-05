package main

import (
	"log"
	"os"
	"time"

	"github.com/Lavizord/checkers-server/internal/config"
	"github.com/Lavizord/checkers-server/internal/models"
	"github.com/Lavizord/checkers-server/internal/redisdb"
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
	log.Printf("[PStatus Worker-%d] - Waiting for player connections...\n", pid)
	//go processPlayerOnline()
	go processPlayerOffline()
	//go redisClient.StartSessionCleanup(time.Minute * 60) // TODO: This might need to be reviwed... What if there are multiple pstatus workers? Maybe I need to make a separate worker to clean up the sessions.
	select {}
}

func processPlayerOffline() {
	for {
		playerData, err := redisClient.BLPop("player_offline", 0) // Block
		if err != nil {
			log.Printf("[PStatus Worker-%d] - Error retrieving player: %d\n", pid, err)
			continue
		}
		//log.Printf("[PStatus Worker-%d] - Player disconnected: %+v\n", pid, playerData)
		playerData, err = redisClient.GetPlayer(playerData.ID)
		if err == nil {
			//handleRemovePlayer(playerData)
			playerData.DisconnectedAt = time.Now().Unix()
			handleDisconnectPlayer(playerData)
		}
	}
}

func handleRemovePlayer(player *models.Player) {
	//log.Printf("[PStatus Worker-%d] - Removing player: %s (Session: %s, Currency: %s, RoomID: %s)\n",
	//	pid, player.ID, player.SessionID, player.Currency, player.RoomID)

	// We dont need to issue a command to leave the queue, since the queue fetched the player-
	if player.RoomID != "" || player.Status == models.StatusInRoom || player.Status == models.StatusInRoomReady {
		//log.Printf("[PStatus Worker-%d] - Removed player is in a Room, sending notification to room worker!: %v\n", pid, player)
		redisClient.RPush("leave_room", player)
	}
	if player.GameID != "" || player.Status == models.StatusInGame {
		//log.Printf("[PStatus Worker-%d] - Removed player is in a Game, sending notification to Game worker!: %v\n", pid, player)
		redisClient.RPush("disconnect_game", player)
	}

	err := redisClient.RemovePlayer(string(player.ID))
	if err != nil {
		log.Printf("[PStatus Worker-%d] - Failed to remove player: %v\n", pid, err)
		return
	}
	//log.Printf("[PStatus Worker-%d] - Player successfully removed.\n", pid)
}

func handleDisconnectPlayer(player *models.Player) {
	//log.Printf("[PStatus Worker-%d] - Disconnecting player: %s (Session: %s, Currency: %s, RoomID: %s)\n",
	//	pid, player.ID, player.SessionID, player.Currency, player.RoomID)

	// We dont need to issue a command to leave the queue, since the queue fetches the player and checks status.

	// If its in a room we push a leave room command.
	if player.RoomID != "" || player.Status == models.StatusInRoom || player.Status == models.StatusInRoomReady {
		//log.Printf("[PStatus Worker-%d] - Removed player is in a Room, sending notification to room worker!: %v\n", pid, player)
		redisClient.RPush("leave_room", player)
	}

	// If its in a game we push a disconnected game command.
	if player.GameID != "" || player.Status == models.StatusInGame {
		//log.Printf("[PStatus Worker-%d] - Removed player is in a Game, sending notification to Game worker!: %v\n", pid, player)
		redisClient.RPush("disconnect_game", player)
	} else {
		// If the player is not in game we will remove it.
		// If it is in game we will need to keep it in the redis memory.
		err := redisClient.RemovePlayer(string(player.ID))
		if err != nil {
			log.Printf("[PStatus Worker-%d] - Failed to remove player: %v\n", pid, err)
			return
		}
		//log.Printf("[PStatus Worker-%d] - Player successfully removed.\n", pid)
	}

}
