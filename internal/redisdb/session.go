package redisdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Lavizord/checkers-server/internal/models"
)

func (r *RedisClient) AddSession(session *models.Session) error {
	ctx := context.Background()

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
	// Use pipeline for atomic operations
	pipe := r.Client.TxPipeline()
	// 1. Store session data
	sessionKey := fmt.Sprintf("session:%s", session.ID)
	pipe.HSet(ctx, sessionKey, map[string]interface{}{
		"id":                  session.ID,
		"token":               session.Token,
		"player_name":         session.PlayerName,
		"currency":            session.Currency,
		"operator_identifier": string(operatorIdentifierData),
		"operator_base_url":   session.OperatorBaseUrl,
		"created_at":          session.CreatedAt.Format(time.RFC3339),
		"data":                string(data),
	})
	// 2. Create token->ID mapping to help fetch session by token
	tokenKey := fmt.Sprintf("session_token:%s", session.Token)
	pipe.Set(ctx, tokenKey, session.ID, 0)
	// 3. Create operator index to fetch by operator data.
	indexKey := fmt.Sprintf("session_index:%s:%s:%s",
		session.OperatorIdentifier.OperatorName,
		session.PlayerName,
		session.Currency)
	pipe.Set(ctx, indexKey, session.ID, 0)
	// Execute all operations atomically
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("[RedisClient] (Session) - failed to store session: %v", err)
	}
	return nil
}

func (r *RedisClient) RemoveSession(sessionID string) error {
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)

	// First retrieve all necessary metadata
	fields, err := r.Client.HMGet(ctx, sessionKey,
		"operator_identifier",
		"player_name",
		"currency",
		"token",
	).Result()

	if err != nil || fields[0] == nil || fields[1] == nil || fields[2] == nil || fields[3] == nil {
		return fmt.Errorf("[RedisClient] - failed to retrieve session metadata: %v", err)
	}

	// Unmarshal operator identifier
	var opIdentifier models.OperatorIdentifier
	if err := json.Unmarshal([]byte(fields[0].(string)), &opIdentifier); err != nil {
		return fmt.Errorf("[RedisClient] - failed to unmarshal operator identifier: %v", err)
	}

	// Prepare pipeline for atomic deletion
	pipe := r.Client.TxPipeline()
	// 1. Delete main session hash
	pipe.Del(ctx, sessionKey)
	// 2. Delete token index (NEW)
	tokenKey := fmt.Sprintf("session_token:%s", fields[3].(string))
	pipe.Del(ctx, tokenKey)
	// 3. Delete operator index
	indexKey := fmt.Sprintf("session_index:%s:%s:%s",
		opIdentifier.OperatorName,
		fields[1].(string),
		fields[2].(string))
	pipe.Del(ctx, indexKey)
	// Execute all deletions
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("[RedisClient] - failed to delete session data: %v", err)
	}
	return nil
}

func (r *RedisClient) GetSessionByID(sessionID string) (*models.Session, error) {
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)

	// Retrieve session fields (including 'data' and any other necessary fields)
	fields, err := r.Client.HMGet(ctx, sessionKey, "data").Result()
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to retrieve session %s: %v", sessionID, err)
	}
	if fields[0] == nil {
		return nil, fmt.Errorf("[RedisClient] - session data not found for %s", sessionID)
	}

	// Unmarshal 'data' field into the session object
	var session models.Session
	data, ok := fields[0].(string) // Ensure correct type assertion
	if !ok {
		return nil, fmt.Errorf("[RedisClient] - session data is not a valid string")
	}

	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("[RedisClient] - failed to unmarshal session data: %v", err)
	}

	return &session, nil
}

func (r *RedisClient) GetSessionByToken(token string) (*models.Session, error) {
	ctx := context.Background()
	tokenKey := fmt.Sprintf("session_token:%s", token)
	// 1. Get ID from token index
	sessionID, err := r.Client.Get(ctx, tokenKey).Result()
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - session not found for token: %s", token)
	}
	return r.GetSessionByID(sessionID)
}

func (r *RedisClient) GetSessionByOperatorPlayerCurrency(operator, playerName, currency string) (*models.Session, error) {
	ctx := context.Background()
	indexKey := fmt.Sprintf("session_index:%s:%s:%s", operator, playerName, currency)

	// Fetch session ID using index
	sessionID, err := r.Client.Get(ctx, indexKey).Result()
	if err != nil {
		return nil, fmt.Errorf("[RedisClient] - session not found for operator: %s, player: %s, currency: %s", operator, playerName, currency)
	}

	// Fetch full session data
	return r.GetSessionByID(sessionID)
}
