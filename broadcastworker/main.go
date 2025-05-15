package main

import (
	"log"
	"os"
	"time"

	"github.com/Lavizord/checkers-server/config"
	"github.com/Lavizord/checkers-server/messages"
	"github.com/Lavizord/checkers-server/redisdb"
)

var pid int
var name = "BroadCastWorker"
var redisClient *redisdb.RedisClient

func init() {
	config.LoadConfig()
	pid = os.Getpid()
	redisConData := config.Cfg.Redis
	client, err := redisdb.NewRedisClient(redisConData.Addr, redisConData.User, redisConData.Password)
	if err != nil {
		log.Fatalf("[%s][BroadcastWorker-%d][Redis] Error initializing Redis client: %v", name, pid, err)
	}
	redisClient = client
}

func main() {
	log.Printf("[BroadcastWorker-%d] - Broadcasting room stats...\n", pid)
	ticker := time.NewTicker(time.Duration(config.Cfg.Services["broadcastworker"].Timer) * time.Second)
	defer func() {
		ticker.Stop()
		if redisClient != nil {
			redisClient.CloseRedisClient()
		}
	}()
	for range ticker.C {
		msg, _ := messages.GenerateGameInfoMessageBytes(redisClient)
		// Publish the message
		err := redisClient.Publish("game_info", msg)
		if err != nil {
			log.Printf("[BroadcastWorker-%d] Error publishing message: %v\n", pid, err)
		} else {
			//log.Printf("[BroadcastWorker-%d] Published room aggregates\n", pid)
		}
	}
}
