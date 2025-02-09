package redisdb

import (
	"checkers-server/core"
	"context"
	"fmt"
	"strconv"
)

func (rc *RedisClient) AddToQueue(ctx context.Context, player *core.Player) error {
	key := "queue:" + strconv.FormatFloat(player.SelectedBid, 'f', 2, 64)
	return rc.Client.RPush(ctx, key, player.Id).Err()
}

func (rc *RedisClient) RemoveFromQueue(ctx context.Context, player *core.Player) error {
	key := "queue:" + strconv.FormatFloat(player.SelectedBid, 'f', 2, 64)
	return rc.Client.LRem(ctx, key, 0, player.Id).Err()
}

func (rc *RedisClient) GetQueueSizeOf(ctx context.Context, bid float64) (int, error) {
	key := "queue:" + strconv.FormatFloat(bid, 'f', 2, 64)
	size, err := rc.Client.LLen(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("[Redis.Queue] - failed to get queue size for %s: %w", key, err)
	}
	return int(size), nil
}

func (rc *RedisClient) GetTotalQueuedPlayers() (int, error) {
	keys, err := rc.Client.Keys(context.Background(), "queue:*").Result()
	if err != nil {
		return 0, fmt.Errorf("[Redis.Queue.TotalQueuedPlayers] - failed to fetch queue keys: %w", err)
	}
	totalPlayers := 0
	for _, key := range keys {
		size, err := rc.Client.LLen(context.Background(), key).Result()
		if err != nil {
			return 0, fmt.Errorf("[Redis.Queue.TotalQueuedPlayers] - failed to get queue size for %s: %w", key, err)
		}
		totalPlayers += int(size)
	}
	return totalPlayers, nil
}

func (rc *RedisClient) GetNextPlayer(ctx context.Context, bid float64) (int, error) {
	key := "queue:" + strconv.FormatFloat(bid, 'f', 2, 64)
	playerID, err := rc.Client.LPop(ctx, key).Int()
	if err != nil {
		
		return 0, fmt.Errorf("[Redis.Queue.GetNextPlayer] - failed to get next player") 
	}
	return playerID, nil
}