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
		return fmt.Errorf("[Redis.Player] - ")
	}
	return rc.Client.Set(ctx, "player:"+strconv.Itoa(player.Id), data, 0).Err()
}

func (rc *RedisClient) CountPlayers() int {
	keys, err := rc.Client.Keys(context.Background(), "player:*").Result()
	if err != nil {
		fmt.Errorf("[Redis.Player.Count] - ")
		return -1
	}
	return len(keys)
}

func (rc *RedisClient) RemovePlayer(player *core.Player) error {
	return rc.Client.Del(context.Background(), "player:"+strconv.Itoa(player.Id)).Err()
}

