package main

import (
	"context"
	"log"

	"github.com/Lavizord/checkers-server/models"
	"github.com/Lavizord/checkers-server/redisdb"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	redis *redisdb.RedisClient
}

func newHub(addr string, username string, password string) *Hub {
	redisclient, err := redisdb.NewRedisClient(addr, username, password)
	if err != nil {
		log.Fatalf("[Redis] Error initializing Redis client: %v", err)
	}
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		redis:      redisclient,
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

		case client := <-h.unregister:
			log.Println("[HUB.Run] - Unregister")
			if _, ok := h.clients[client]; ok {
				client.UpdatePlayerDataFromRedis()
				h.redis.UpdatePlayersInQueueSet(client.player.ID, models.StatusOffline)
				if client.player.RoomID != "" || client.player.Status == models.StatusInRoom || client.player.Status == models.StatusInRoomReady {
					log.Printf("[Hub.Run] - Removed player is in a Room, sending notification to room worker!: %v\n", client.player)
					h.redis.RPush("leave_room", client.player)
				}
				if client.player.GameID != "" || client.player.Status == models.StatusInGame {
					log.Printf("[Hub.Run] - Removed player is in a Game, sending notification to Game worker!: %v\n", client.player)
					h.redis.RPush("disconnect_game", client.player)
				}
				h.redis.RemovePlayer(client.player.ID)
				delete(h.clients, client)
				close(client.send)
			}

		case message := <-h.broadcast:
			//msg, _ := json.Marshal(message)
			//log.Printf("[HUB.Run] - Broadcast: %v", msg)

			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					h.redis.UpdatePlayersInQueueSet(client.player.ID, models.StatusOffline)
					if client.player.RoomID != "" || client.player.Status == models.StatusInRoom || client.player.Status == models.StatusInRoomReady {
						log.Printf("[Hub.Run] - Removed player is in a Room, sending notification to room worker!: %v\n", client.player)
						h.redis.RPush("leave_room", client.player)
					}
					if client.player.GameID != "" || client.player.Status == models.StatusInGame {
						log.Printf("[Hub.Run] - Removed player is in a Game, sending notification to Game worker!: %v\n", client.player)
						h.redis.RPush("disconnect_game", client.player)
					}
					h.redis.RemovePlayer(client.player.ID)
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) SubscribeBroadcast() {
	pubsub := h.redis.Client.Subscribe(context.Background(), "game_info")
	ch := pubsub.Channel()

	go func() {
		for msg := range ch {
			//log.Printf("[HUB.SubscribeBroadcast]: %v", msg.Payload)
			h.broadcast <- []byte(msg.Payload) // Send to all connected clients
		}
	}()
}
