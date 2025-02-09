package redisdb

import (
	"checkers-server/core"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

func (rc *RedisClient) AddPlayer(ctx context.Context, player *core.Player) error {
	data, err := json.Marshal(player)
	if err != nil {
		return fmt.Errorf("[Redis.Player] - %w", err)
	}
	return rc.Client.Set(ctx, "player:"+strconv.Itoa(player.Id), data, 0).Err()
}

func (rc *RedisClient) CountPlayers() int {
	keys, err := rc.Client.Keys(context.Background(), "player:*").Result()
	if err != nil {
		fmt.Errorf("[Redis.Player.Count] - %w", err)
		return -1
	}
	return len(keys)
}

func (rc *RedisClient) RemovePlayer(player *core.Player) error {
	rc.RemoveFromQueue(player)	// This also removes it from a queue.
	err := rc.Client.Del(context.Background(), "player:"+strconv.Itoa(player.Id)).Err()
	if err != nil {
		return fmt.Errorf("failed to remove player from queue: %w", err)
	}
	return nil
}