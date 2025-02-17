package redisdb

import (
	"checkers-server/models"
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func (r *RedisClient) AddRoom(key string, room *models.Room) error {
	data, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("[RedisClient] (Room) - failed to serialize room: %v", err)
	}
	exists, err := r.CheckRoomAggregateExists(room.BidAmount)
	if err == nil {
		if exists {
			r.IncrementRoomAggregate(room.BidAmount)
		} else {
			r.CreateRoomAggregate(room.BidAmount)
		}
	}
	return r.Client.HSet(context.Background(), key, room.ID, data).Err()
}

func (r *RedisClient) AddRoom2(room *models.Room) error {
	ctx := context.Background()

	// Serialize the full room object
	data, err := json.Marshal(room)
	if err != nil {
		return fmt.Errorf("[RedisClient] (Room) - failed to serialize room: %v", err)
	}

	// Store full room data in a hash
	roomKey := fmt.Sprintf("room:%s", room.ID)
	err = r.Client.HSet(ctx, roomKey, map[string]interface{}{
		"id":        room.ID,
		"bidAmount": room.BidAmount,
		"data":      string(data), // Store full room JSON
	}).Err()
	if err != nil {
		return fmt.Errorf("[RedisClient] (Room) - failed to store room data: %v", err)
	}

	// Add room ID to sorted set, indexed by bid amount, this will help us get the rooms by bid amount
	zsetKey := "rooms_by_bid"
	err = r.Client.ZAdd(ctx, zsetKey, redis.Z{
		Score:  room.BidAmount,
		Member: room.ID,
	}).Err()
	if err != nil {
		return fmt.Errorf("[RedisClient] (Room) - failed to update room index: %v", err)
	}

	// Manage Room Aggregate
	exists, err := r.CheckRoomAggregateExists(room.BidAmount)
	if err == nil {
		if exists {
			r.IncrementRoomAggregate(room.BidAmount)
		} else {
			r.CreateRoomAggregate(room.BidAmount)
		}
	}

	return nil
}

func (r *RedisClient) RemoveRoom(key string) error {
	return r.Client.HDel(context.Background(), key).Err()
}

func (r *RedisClient) GetRoomsByBidAmount(bidAmount float64) ([]models.Room, error) {
	ctx := context.Background()
	zsetKey := "rooms_by_bid"

	// Get room IDs in the given bid amount range
	roomIDs, err := r.Client.ZRangeByScore(ctx, zsetKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", bidAmount),
		Max: fmt.Sprintf("%f", bidAmount),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to retrieve rooms: %v", err)
	}

	// Retrieve full room data from Hash
	var rooms []models.Room
	for _, roomID := range roomIDs {
		roomKey := fmt.Sprintf("room:%s", roomID)
		data, err := r.Client.HGet(ctx, roomKey, "data").Result()
		if err != nil {
			continue // Skip if the room data is missing
		}

		var room models.Room
		if err := json.Unmarshal([]byte(data), &room); err == nil {
			rooms = append(rooms, room)
		}
	}

	return rooms, nil
}

func (r *RedisClient) GetEmptyRoomsByBidAmount(bidAmount float64) ([]models.Room, error) {
	ctx := context.Background()
	zsetKey := "rooms_by_bid"

	// Get room IDs in the given bid amount range
	// TODO: This is bugged, its not procuding the riight results.
	roomIDs, err := r.Client.ZRangeByScore(ctx, zsetKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", bidAmount),
		Max: fmt.Sprintf("%f", bidAmount),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to retrieve rooms: %v", err)
	}

	// Retrieve full room data from Hash
	var rooms []models.Room
	for _, roomID := range roomIDs {
		roomKey := fmt.Sprintf("room:%s", roomID)
		data, err := r.Client.HGet(ctx, roomKey, "data").Result()
		if err != nil {
			continue // Skip if the room data is missing
		}

		var room models.Room
		if err := json.Unmarshal([]byte(data), &room); err == nil {
			if room.Player2 == nil{
				rooms = append(rooms, room)
			}
		}
	}

	return rooms, nil
}
