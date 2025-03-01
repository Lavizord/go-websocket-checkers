package main

import (
	"checkers-server/config"
	"checkers-server/messages"
	"checkers-server/redisdb"
	"fmt"
	"log"
	"os"
	"time"
)

var pid int
var name = "BroadCastWorker"
var redisClient *redisdb.RedisClient

func init() {
	config.LoadConfig()
	pid = os.Getpid()
	redisAddr := config.Cfg.Redis.Addr
	client, err := redisdb.NewRedisClient(redisAddr)
	if err != nil {
		log.Fatalf("[%s][BroadcastWorker-%d][Redis] Error initializing Redis client: %v", name, pid, err)
	}
	redisClient = client
}

func main() {
	fmt.Printf("[BroadcastWorker-%d] - Broadcasting room stats...\n", pid)
	ticker := time.NewTicker(time.Duration(config.Cfg.Services["broadcastworker"].Timer) * time.Second) // Adjust interval as needed
	defer ticker.Stop()

	for range ticker.C {
		aggregates, err := redisClient.GetRoomAggregates()
		if err != nil {
			log.Printf("[BroadcastWorker-%d] Error fetching room aggregates: %v\n", pid, err)
			continue
		}
		// Create a message with the game_info
		messageBytes, err := messages.NewMessage("game_info", aggregates)
		if err != nil {
			log.Printf("[BroadcastWorker-%d] Error creating message: %v\n", pid, err)
			continue
		}
		// Publish the message
		err = redisClient.Publish("game_info", messageBytes)
		if err != nil {
			log.Printf("[BroadcastWorker-%d] Error publishing message: %v\n", pid, err)
		} else {
			fmt.Printf("[BroadcastWorker-%d] Published room aggregates\n", pid)
		}
	}
}
