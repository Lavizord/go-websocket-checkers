package models

import (
	"fmt"
	"time"
)

// Room represents the data for a game room, containing two players and game details
type Room struct {
	ID              string     `json:"id"`
	Player1         *Player    `json:"player_1"`
	Player2         *Player    `json:"player_2"`
	StartDate       time.Time  `json:"start_date"`
	Currency        string     `json:"currency"`  // Currency for the room
	BetValue        float64    `json:"bet_value"` // Bet amount for the game
	CurrentPlayerID string     `json:"current_player_id"`
	IsRoomOpen      bool       `json:"is_room_open"`
	OperatorIdentifier OperatorIdentifier `json:"operator_identifier"`
}

func (r *Room) GetOpponentPlayerID(playerID string) (string, error) {
	if r.Player1 != nil && r.Player1.ID == playerID {
		if r.Player2 != nil {
			return r.Player2.ID, nil
		}
	} else if r.Player2 != nil && r.Player2.ID == playerID {
		if r.Player1 != nil {
			return r.Player1.ID, nil
		}
	}
	return "", fmt.Errorf("opponent not found for player ID: %s", playerID)
}
