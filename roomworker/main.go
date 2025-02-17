package main

import (
	"checkers-server/config"
	"checkers-server/messages"
	"checkers-server/models"
	"checkers-server/redisdb"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
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
	fmt.Printf("[RoomWorker-%d] - Waiting for room messages...\n", pid)
	go processRoomCreation()
	go processRoomJoin()
	go processRoomReady()
	go processRoomEnding()
	select {}
}

func processRoomCreation() {
	for {
		playerData, err := redisClient.BLPop("create_room", 0) // Block
		if err != nil {
			fmt.Printf("[RoomWorker-%d] - Error retrieving player:%v\n", pid, err)
			continue
		}
		fmt.Printf("[RoomWorker-%d] - create room!: %+v\n", pid, playerData)
		handleCreateRoom(playerData)
	}
}

func processRoomJoin() {
	for {
		playerData, err := redisClient.BLPop("join_room", 0) // Block
		if err != nil {
			fmt.Printf("[RoomWorker-%d] - Error retrieving player:%v\n", pid, err)
			continue
		}
		fmt.Printf("[RoomWorker-%d] - processing join room!: %+v\n", pid, playerData)
		handleJoinRoom(playerData)
	}
}

func processRoomReady() {
	for {
		playerData, err := redisClient.BLPop("ready_room", 0) // Block
		if err != nil {
			fmt.Printf("[RoomWorker-%d] - Error retrieving player:%v\n", pid, err)
			continue
		}
		fmt.Printf("[RoomWorker-%d] - processing ready room!: %+v\n", pid, playerData)
		// TODO: handle ready room message.
	}
}

func processRoomEnding() {
	for {
		playerData, err := redisClient.BLPop("leave_room", 0) // Block
		if err != nil {
			fmt.Printf("[RoomWorker-%d] - Error retrieving player:%v\n", pid, err)
			continue
		}
		fmt.Printf("[RoomWorker-%d] - Processing the end of room: %+v\n", pid, playerData)
		redisClient.RemoveRoom(redisdb.GenerateRoomRedisKeyById(playerData.RoomID))
		redisClient.DecrementRoomAggregate(playerData.SelectedBid)
		// TODO: We gotta notify the other player that the room has ended.
		redisClient.Client.RPush(context.Background(), "player_update", string("opponent_disconnected")).Err()
	}
}

func handleCreateRoom(player *models.Player) {
	fmt.Printf("[RoomWorker-%d] - Handling player (CREATE ROOM): %s (Session: %s, Currency: %s)\n",
		pid, player.ID, player.SessionID, player.Currency)

	room := &models.Room{
		ID:        models.GenerateUUID(),
		Player1:   player,
		StartDate: time.Now(),
		Currency:  player.Currency,
		BidAmount: player.SelectedBid,
	}
	err := redisClient.AddRoom2(room)
	if err != nil {
		fmt.Printf("[RoomWorker-%d] - Failed to add room to Redis: %v\n", pid, err)
		return
	}

	player.RoomID = room.ID
	player.Status = "waiting_oponente"
	redisClient.AddPlayer(player) // This should update out player room info.

	messageBytes, err := messages.GenerateRoomCreatedMessage(*room)
	if err != nil {
		fmt.Printf("[RoomWorker-%d] - Invalid message format: %v\n", pid, err)
		return
	}
	// Publish the validated message to Redis
	err = redisClient.PublishToPlayer(*player, string(messageBytes))
	if err != nil {
		fmt.Printf("[RoomWorker-%d] - Failed to publish message to player: %v\n", pid, err)
		return
	}
	fmt.Printf("[RoomWorker-%d] - Player successfully handled and notified, %+v\n", pid, string(messageBytes))
}

func handleJoinRoom(player *models.Player) {
	fmt.Printf("[RoomWorker-%d] - Handling player (JOIN ROOM): %s (Session: %s, Currency: %s)\n",
		pid, player.ID, player.SessionID, player.Currency)
	rooms, err := redisClient.GetEmptyRoomsByBidAmount(player.SelectedBid)
	if err != nil {
		return
	}
	colorp1 := rand.Intn(2)
	colorp2 := 1
	if colorp1 == 1 {
		colorp2 = 0
	}
	message, err := messages.GeneratePairedMessage(rooms[0].Player1, player, rooms[0].ID, colorp1)
	if err != nil {
		fmt.Printf("[RoomWorker-%d] - Error handling paired message for p1: %s\n", pid, err)
		return
	}
	message2, err2 := messages.GeneratePairedMessage(player, rooms[0].Player1, rooms[0].ID, colorp2)
	if err2 != nil {
		fmt.Printf("[RoomWorker-%d] - Error handling paired message for p2:%s\n", pid, err2)
		return
	}
	fmt.Printf("[RoomWorker-%d] - Handling player (JOIN ROOM) - Message for player1: %s\n", pid, message)
	fmt.Printf("[RoomWorker-%d] - Handling player (JOIN ROOM) - Message for player2: %s\n", pid, message2)

	redisClient.PublishPlayerEvent(rooms[0].Player1, string(message))
	redisClient.PublishPlayerEvent(player, string(message2))
}
