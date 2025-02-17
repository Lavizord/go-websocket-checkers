package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Player struct {
	ID             string          `json:"id"`
	Token          string          `json:"token"`
	RoomID		   string		   `json:"room_id"`
	SessionID      string          `json:"session_id"`
	Currency       string          `json:"currency"`
	CurrencyAmount int             `json:"currency_amount"`
	Status         string          `json:"status"` // IN GAME! ONLINE! OFFLNE
	SelectedBid    float64         `json:"selected_bid"`
	Name           string          `json:"name"`
	Conn           *websocket.Conn `json:"-"` // Exclude Conn from JSON
}

// Room represents the data for a game room, containing two players and game details
type Room struct {
	ID                string     `json:"id"`
	Player1           *Player    `json:"player_1"`
	Player2           *Player    `json:"player_2"`
	StartDate         time.Time  `json:"start_date"`
	Currency          string     `json:"currency"`  // Currency for the room
	BidAmount         float64    `json:"bid_amount"` // Bet amount for the game
	Turn              int        `json:"turn"`
	CurrentTurnPlayer *Player    `json:"current_turn_player"`
	GameBoard         string     `json:"game_board"`  // TODO: Representation of the game board (could be a string of board state?)
	GameStatus        string     `json:"game_status"` // Status of the game (e.g., "waiting", "in_progress", "finished")
	GameEndDate       *time.Time `json:"game_end_date,omitempty"`
	Winner            *Player    `json:"winner,omitempty"` // Player who won (if game is over)
	IsRoomOpen        bool       `json:"is_room_open"`
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
	ID          string  `json:"id"`
	Player      string  `json:"name"`
	Currency    string  `json:"currency"`
	SelectedBid float64 `json:"selected_bid"`
}

type CreateRoomMessage struct {
	Command string    `json:"command"`
	Value   RoomValue `json:"value"`
}

type PairedValue struct {
	Color    int    `json:"color"`
	Opponent string `json:"opponent"`
	RoomID	 string `json:"room_id"`
}

func GenerateUUID() string {
	return uuid.New().String() // Generates a new UUID and returns it as a string
}
