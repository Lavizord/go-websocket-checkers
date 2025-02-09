package redisdb

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis client
type RedisClient struct {
	Client *redis.Client
}

func NewRedisClient(addr, password string, db int) *RedisClient {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("[Redis] - Failed to connect to Redis: %v", err)
	}
	fmt.Println("[Redis] - Connected to Redis successfully!")

	return &RedisClient{Client: client}
}

func (rc *RedisClient) Close() error {
	return rc.Client.Close()
}
