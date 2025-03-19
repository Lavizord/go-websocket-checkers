package models

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID                 string             `json:"session_id"`
	Token              string             `json:"token"`
	PlayerName         string             `json:"player_name"`
	Balance            int64              `json:"balance"`
	Currency           string             `json:"currency"`
	OperatorBaseUrl    string             `json:"operator_base_url"`
	CreatedAt          time.Time          `json:"created_at"`
	ExtractID          string             `json:"extract_id"` // This was created to store the extract ID of a bet, so that we can later use it in the win post...
	OperatorIdentifier OperatorIdentifier `json:"operator_identifier"`
}

type Transaction struct {
	ID          string    `json:"transaction_id"` // Unique ID for each transaction (auto-incremented by the database)
	SessionID   string    `json:"session_id"`     // Session ID for the player
	Type        string    `json:"type"`           // Type of transaction: 'bet' or 'win'
	Amount      int       `json:"amount"`         // Amount in cents
	Currency    string    `json:"currency"`       // Currency code (e.g., EUR, USD)
	Platform    string    `json:"platform"`       // Platform name
	Operator    string    `json:"operator"`       // Operator name (e.g., SokkerDuel)
	Client      string    `json:"client"`         // Client ID (player ID)
	Game        string    `json:"game"`           // Internal game name
	Status      string    `json:"status"`         // HTTP status code
	Description string    `json:"description"`    // Description (e.g., "Insufficient Funds" or "OK")
	RoundID     string    `json:"round_id"`       // Foreign key to the round / game
	Timestamp   time.Time `json:"timestamp"`      // Timestamp in UTC
}

type OperatorIdentifier struct {
	OperatorName     string `json:"operator_name"`
	OperatorGameName string `json:"operator_game_name"`
	GameName         string `json:"game_name"`
}

type RoomAggregate struct {
	AggregateValue float64 `json:"aggregate_value"`
	Count          int64   `json:"count"`
}

type RoomAggregateResponse struct {
	PlayersWaiting int             `json:"players_waiting"`
	RoomAggregate  []RoomAggregate `json:"room_aggregate"`
}

type RoomValue struct {
	ID       string  `json:"id"`
	Player   string  `json:"name"`
	Currency string  `json:"currency"`
	BetValue float64 `json:"bet_value"`
}

type CreateRoomMessage struct {
	Command string    `json:"command"`
	Value   RoomValue `json:"value"`
}

type PairedValue struct {
	Color    int    `json:"color"`
	Opponent string `json:"opponent"`
	RoomID   string `json:"room_id"`
}

type QueueConfirmation struct {
	IsConfirmed bool `json:"is_confirmed"`
}

func GenerateUUID() string {
	return uuid.New().String() // Generates a new UUID and returns it as a string
}
