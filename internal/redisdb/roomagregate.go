package redisdb

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/Lavizord/checkers-server/internal/models"
)

func (r *RedisClient) CreateQueueCount(aggregateValue float64) {
	key := fmt.Sprintf("queue_count:%f", aggregateValue)
	_, err := r.Client.SetNX(context.Background(), key, 1, 0).Result()
	if err != nil {
		log.Printf("Error setting room aggregate: %v", err)
	}
}

func (r *RedisClient) IncrementQueueCount(aggregateValue float64) {
	key := fmt.Sprintf("queue_count:%f", aggregateValue)
	_, err := r.Client.Incr(context.Background(), key).Result()
	if err != nil {
		log.Printf("Error incrementing room aggregate: %v", err)
	}
}

func (r *RedisClient) DecrementQueueCount(aggregateValue float64) {
	key := fmt.Sprintf("queue_count:%f", aggregateValue)
	_, err := r.Client.Decr(context.Background(), key).Result()
	if err != nil {
		log.Printf("Error decrementing room aggregate: %v", err)
	}
}

func (r *RedisClient) CheckQueueCountExists(aggregateValue float64) (bool, error) {
	key := fmt.Sprintf("queue_count:%f", aggregateValue)

	// Check if the key exists in Redis
	exists, err := r.Client.Exists(context.Background(), key).Result()
	if err != nil {
		return false, fmt.Errorf("Error checking if room aggregate exists: %v", err)
	}
	return exists == 1, nil
}

func (r *RedisClient) GetQueueNumberResponse() (*models.QueueNumbersResponse, error) {
	var cursor uint64
	var keys []string
	var err error
	for {
		var partialKeys []string
		partialKeys, cursor, err = r.Client.Scan(context.Background(), cursor, "queue_count:*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("error scanning for room aggregates: %v", err)
		}
		keys = append(keys, partialKeys...)
		if cursor == 0 {
			break
		}
	}
	playerCount := make([]models.PlayerCountPerBetValue, 0, len(keys))
	// Use MGET for better performance when fetching multiple values
	if len(keys) > 0 {
		values, err := r.Client.MGet(context.Background(), keys...).Result()
		if err != nil {
			return nil, fmt.Errorf("error getting aggregate values: %v", err)
		}

		for i, key := range keys {
			parts := strings.Split(key, ":")
			if len(parts) < 2 {
				continue // Skip malformed keys
			}
			aggregateValue, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				continue // Skip keys with invalid numeric parts
			}
			// Handle the MGet result which could be nil
			if values[i] == nil {
				continue
			}
			count, err := strconv.ParseInt(values[i].(string), 10, 64)
			if err != nil {
				continue // Skip invalid counts
			}
			games, _ := r.CountGamesByBetValue(aggregateValue) // Now we fetch our games.
			playerCount = append(playerCount, models.PlayerCountPerBetValue{
				BetValue:    aggregateValue,
				PlayerCount: count + (games * 2),
			})
		}
	}

	// Sort by Count in descending order (most players first)
	sort.Slice(playerCount, func(i, j int) bool {
		return playerCount[i].PlayerCount > playerCount[j].PlayerCount
	})
	return &models.QueueNumbersResponse{
		QueuNumbers: playerCount,
	}, nil
}
