package redisdb

import (
	"checkers-server/models"
	"context"
	"encoding/json"
	"fmt"
)

func (r *RedisClient) AddPlayerDeprecated(key string, player *models.Player) error {
	data, err := json.Marshal(player)
	if err != nil {
		return fmt.Errorf("[RedisClient] - failed to serialize player: %v", err)
	}

	return r.Client.HSet(context.Background(), key, player.ID, data).Err()
}

func (r *RedisClient) AddPlayer(player *models.Player) error {
	data, err := json.Marshal(player)
	if err != nil {
		return fmt.Errorf("[RedisClient] - failed to serialize player: %v", err)
	}
	// Uses a shared key ("players") , store the player data under their ID
	return r.Client.HSet(context.Background(), "players", player.ID, data).Err()
}


func (r *RedisClient) GetPlayerDeprecated(key string, playerID string) (*models.Player, error) {
	data, err := r.Client.HGet(context.Background(), key, playerID).Result()
	if err != nil {
		return nil, err
	}

	var player models.Player
	err = json.Unmarshal([]byte(data), &player)
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to deserialize player: %v", err)
	}

	return &player, nil
}

func (r *RedisClient) GetPlayer(playerID string) (*models.Player, error) {
	data, err := r.Client.HGet(context.Background(), "players", playerID).Result()
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to get player: %v", err)
	}

	var player models.Player
	err = json.Unmarshal([]byte(data), &player)
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to deserialize player: %v", err)
	}
	return &player, nil
}

func (r *RedisClient) RemovePlayer(playerID string) error {
	exists, err := r.Client.HExists(context.Background(), "players", playerID).Result()
	if err != nil {
		return fmt.Errorf("[RedisClient] - failed to check if player exists: %v", err)
	}

	if !exists {
		return fmt.Errorf("[RedisClient] - atempting to delete player that does not exist: %s", playerID)
	}
	return r.Client.HDel(context.Background(), "players", playerID).Err()
}

