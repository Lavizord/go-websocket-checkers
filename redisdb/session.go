package redisdb

import (
	"checkers-server/models"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (r *RedisClient) AddSession(session *models.Session) error {
	ctx := context.Background()

	// Ensure CreatedAt is set if not already
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("[RedisClient] (Session) - failed to serialize session: %v", err)
	}
	operatorIdentifierData, err := json.Marshal(session.OperatorIdentifier)
	if err != nil {
		return fmt.Errorf("[RedisClient] (Session) - failed to serialize operator identifier: %v", err)
	}
	// Store session data in a hash
	sessionKey := fmt.Sprintf("session:%s", session.ID)
	err = r.Client.HSet(ctx, sessionKey, map[string]interface{}{
		"id":                  session.ID,
		"token":               session.Token,
		"player_name":         session.PlayerName,
		"balance":             session.Balance,
		"currency":            session.Currency,
		"operator_identifier": string(operatorIdentifierData),
		"operator_base_url":   session.OperatorBaseUrl,
		"created_at":          session.CreatedAt.Format(time.RFC3339), // Store timestamp as string
		"data":                string(data),                           // Store full session JSON
	}).Err()
	if err != nil {
		return fmt.Errorf("[RedisClient] (Session) - failed to store session data: %v", err)
	}
	return nil
}

func (r *RedisClient) RemoveSession(sessionID string) error {
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)

	exists, err := r.Client.HExists(ctx, sessionKey, "id").Result()
	if err != nil {
		return fmt.Errorf("[RedisClient] - failed to check if session exists: %v", err)
	}
	if !exists {
		return fmt.Errorf("[RedisClient] - attempting to delete session that does not exist: %s", sessionID)
	}
	if err := r.Client.Del(ctx, sessionKey).Err(); err != nil {
		return fmt.Errorf("[RedisClient] - failed to delete session: %v", err)
	}
	return nil
}

func (r *RedisClient) GetSessionByID(sessionID string) (*models.Session, error) {
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)

	data, err := r.Client.HGet(ctx, sessionKey, "data").Result()
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to retrieve session %s: %v", sessionID, err)
	}
	var session models.Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to unmarshal session data: %v", err)
	}
	return &session, nil
}
