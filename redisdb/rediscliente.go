package redisdb

import (
	"checkers-server/models"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Client        *redis.Client
	Ctx           context.Context
	Subscriptions map[string]*redis.PubSub // Stores active subscriptions per channel
	mu            sync.Mutex
}

func NewRedisClient(addr string) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// check connection
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to connect to Redis at %s: %w", addr, err)
	}
	return &RedisClient{
		Client:        client,
		Subscriptions: make(map[string]*redis.PubSub),
	}, nil
}

// RPush - Push serialized player to Redis
func (r *RedisClient) RPush(queue string, player *models.Player) error {
	data, err := json.Marshal(player)
	if err != nil {
		return err
	}
	return r.Client.RPush(context.Background(), queue, string(data)).Err()
}

// BLPop - Retrieve a player from Redis queue
func (r *RedisClient) BLPop(queue string, timeout int) (*models.Player, error) {
	result, err := r.Client.BLPop(context.Background(), 0, queue).Result()
	if err != nil {
		return nil, err
	}

	if len(result) > 1 {
		var player models.Player
		err = json.Unmarshal([]byte(result[1]), &player)
		if err != nil {
			return nil, err
		}
		return &player, nil
	}

	return nil, fmt.Errorf("no player found in queue")
}

func (rc *RedisClient) PublishPlayerEvent(player *models.Player, message string) error {

	//data, err := json.Marshal(message)
	//if err != nil {
	//	return fmt.Errorf("[pkg/redisdb/cliente] - failed to marshal event data: %w", err)
	//}

	err := rc.Client.Publish(context.Background(), GetPlayerPubSubChannel(*player), message).Err()
	if err != nil {
		return fmt.Errorf("[pkg/redisdb/cliente] - failed to publish player event: %w", err)
	}
	return nil
}

func (rc *RedisClient) Publish(channel string, message []byte) error {
	err := rc.Client.Publish(context.Background(), channel, message).Err()
	if err != nil {
		return fmt.Errorf("[RedisClient] - failed to publish message: %w", err)
	}
	return nil
}

func (r *RedisClient) SubscribePlayerChannel(player models.Player, messageHandler func(string)) {
	channel := GetPlayerPubSubChannel(player) // Function to get player-specific channel
	r.mu.Lock()
	if _, exists := r.Subscriptions[channel]; exists {
		r.mu.Unlock()
		fmt.Println("Already subscribed to", channel)
		return
	}
	pubsub := r.Client.Subscribe(context.Background(), channel)
	r.Subscriptions[channel] = pubsub
	r.mu.Unlock()

	go func() {
		for msg := range pubsub.Channel() {
			messageHandler(msg.Payload)
		}
	}()
}

func (r *RedisClient) Subscribe(channel string, messageHandler func(string)) {
	r.mu.Lock()
	if _, exists := r.Subscriptions[channel]; exists {
		r.mu.Unlock()
		fmt.Println("Already subscribed to", channel)
		return
	}
	pubsub := r.Client.Subscribe(context.Background(), channel)
	r.Subscriptions[channel] = pubsub
	r.mu.Unlock()

	go func() {
		for msg := range pubsub.Channel() {
			messageHandler(msg.Payload)
		}
	}()
}

func (r *RedisClient) UnsubscribePlayerChannel(player models.Player) {
	channel := GetPlayerPubSubChannel(player)

	r.mu.Lock()
	pubsub, exists := r.Subscriptions[channel]
	if !exists {
		r.mu.Unlock()
		fmt.Println("Not subscribed to", channel)
		return
	}
	delete(r.Subscriptions, channel)
	fmt.Printf("[RedisClii] (UnsubscribePlayerChannel) - Deleted subscription of [%d] and [%v]\n", player.Name, channel)
	r.mu.Unlock()

	if err := pubsub.Unsubscribe(context.Background(), channel); err != nil {
		fmt.Println("Error unsubscribing from", channel, ":", err)
	} else {
		fmt.Printf("[RedisClii] (UnsubscribePlayerChannel) - Unsubscribe of [%d] and [%v]\n", player.Name, channel)
	}
}

func (r *RedisClient) Unsubscribe(channel string) {
	r.mu.Lock()
	pubsub, exists := r.Subscriptions[channel]
	if !exists {
		r.mu.Unlock()
		fmt.Println("Not subscribed to", channel)
		return
	}
	delete(r.Subscriptions, channel)
	r.mu.Unlock()

	if err := pubsub.Unsubscribe(context.Background(), channel); err != nil {
		fmt.Println("Error unsubscribing from", channel, ":", err)
	}
}

func (r *RedisClient) PublishToPlayer(player models.Player, message string) error {
	return r.Client.Publish(context.Background(), GetPlayerPubSubChannel(player), message).Err()
}
