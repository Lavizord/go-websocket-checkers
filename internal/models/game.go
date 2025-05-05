package models

import (
	"fmt"
	"log"
	"time"

	"github.com/Lavizord/checkers-server/internal/config"

	"github.com/google/uuid"
)

type Piece struct {
	Type     string `json:"type"`
	PlayerID string `json:"player_id"`
	IsKinged bool   `json:"is_kinged"`
	PieceID  string `json:"piece_id"`
}

func getPieceUUID() *string {
	id := uuid.New().String()
	return &id
}

type GamePlayer struct {
	ID        string `json:"id"`
	Token     string `json:"token"`
	Name      string `json:"name"`
	Timer     int    `json:"timer"`
	Color     string `json:"color"`
	SessionID string `json:"session_id"`
	NumPieces int    `json:"num_pieces"`
}

type Game struct {
	ID                 string             `json:"id"`
	Board              Board              `json:"board"`
	Players            []GamePlayer       `json:"players"`
	CurrentPlayerID    string             `json:"current_player_id"`
	Turn               int                `json:"turn"`
	Moves              []Move             `json:"moves"`
	StartTime          time.Time          `json:"start_time"`
	EndTime            time.Time          `json:"end_time"`
	Winner             string             `json:"winner"`
	BetValue           float64            `json:"bet_value"` // Bet amount for the game
	TimerSetting       string             `json:"timer_settings"`
	OperatorIdentifier OperatorIdentifier `json:"operator_identifier"`
}

// Move represents a single move in the game
type Move struct {
	PlayerID  string `json:"player_id"`  // The player making the move
	PieceID   string `json:"piece_id"`   // Will be given to clientes by the server.
	From      string `json:"from"`       // e.g., "A1"
	To        string `json:"to"`         // e.g., "B2"
	IsCapture bool   `json:"is_capture"` // Whether the move captured an opponent's piece
	IsKinged  bool   `json:"is_kinged"`  // Whether the piece was kinged after the move
}

func MapPlayerToGamePlayer(player Player) GamePlayer {
	return GamePlayer{
		ID:        player.ID,
		Name:      player.Name,
		Token:     player.Token,
		SessionID: player.SessionID,
		Timer:     0,
	}
}

func mapPlayers(r *Room) []GamePlayer {
	players := []GamePlayer{}

	if r.Player1 != nil {
		players = append(players, MapPlayerToGamePlayer(*r.Player1))
	}
	if r.Player2 != nil {
		players = append(players, MapPlayerToGamePlayer(*r.Player2))
	}

	return players
}

func (r *Room) NewGame() *Game {
	whiteID, _ := r.GetOpponentPlayerID(r.CurrentPlayerID)

	game := Game{
		ID:    r.ID,
		Board: *NewBoard(r.CurrentPlayerID, whiteID, "std-game"),
		//Board:           *NewBoard(r.CurrentPlayerID, whiteID, "two-pieces-endgame"),
		//Board:           *NewBoard(r.CurrentPlayerID, whiteID, "multiple-capture"),
		Players:            mapPlayers(r),
		CurrentPlayerID:    r.CurrentPlayerID,
		Turn:               0,
		Moves:              []Move{},
		StartTime:          time.Now(),
		Winner:             "",
		BetValue:           r.BetValue,
		TimerSetting:       config.Cfg.Services["gameworker"].TimerSetting,
		OperatorIdentifier: r.OperatorIdentifier,
	}

	if game.Players[0].ID == whiteID {
		game.Players[0].Color = "w"
		game.Players[1].Color = "b"
	} else {
		game.Players[0].Color = "b"
		game.Players[1].Color = "w"
	}
	game.SetUpPlayerTimers()
	game.UpdatePlayerPieces() // Set NumPieces for each player
	return &game
}

func (g *Game) SetUpPlayerTimers() {

	switch g.TimerSetting {
	case "reset":
		g.Players[0].Timer = config.Cfg.Services["gameworker"].Timer
		g.Players[1].Timer = g.Players[0].Timer

	case "cumulative":
		calculatedTimer := config.Cfg.Services["gameworker"].Timer * config.Cfg.Services["gameworker"].PiecesInMatch
		g.Players[0].Timer = calculatedTimer + 1
		g.Players[1].Timer = g.Players[0].Timer

	}
}

func (g *Game) CountPlayerPieces(playerID string) int {
	count := 0
	for _, piece := range g.Board.Grid {
		if piece != nil && piece.PlayerID == playerID {
			count++
		}
	}
	return count
}

func (g *Game) UpdatePlayerPieces() {
	for i := range g.Players {
		g.Players[i].NumPieces = g.CountPlayerPieces(g.Players[i].ID)
	}
}

func (g *Game) GetOpponentPlayerID(playerID string) (string, error) {
	if len(g.Players) != 2 {
		return "", fmt.Errorf("invalid number of players in game")
	}

	for _, player := range g.Players {
		if player.ID != playerID {
			return player.ID, nil
		}
	}
	return "", fmt.Errorf("opponent not found for player ID: %s", playerID)
}

func (g *Game) GetOpponentGamePlayer(playerID string) (*GamePlayer, error) {
	if len(g.Players) != 2 {
		return nil, fmt.Errorf("invalid number of players in game")
	}

	for _, player := range g.Players {
		if player.ID != playerID {
			return &player, nil
		}
	}

	return nil, fmt.Errorf("opponent not found for player ID: %s", playerID)
}

func (g *Game) GetGamePlayer(playerID string) (*GamePlayer, error) {
	if len(g.Players) != 2 {
		return nil, fmt.Errorf("invalid number of players in game")
	}

	for _, player := range g.Players {
		if player.ID == playerID {
			return &player, nil
		}
	}

	return nil, fmt.Errorf("player not found for player ID: %s", playerID)
}

// Updates player id and turn count.
func (g *Game) NextPlayer() {
	nextPlayerId, err := g.GetOpponentPlayerID(g.CurrentPlayerID)
	if err != nil {
		log.Printf("Error NextPlayer getting opponent ID: %v\n", err)
	}
	g.CurrentPlayerID = nextPlayerId
	g.Turn += 1
}

func (g *Game) RemovePiece(pos string) {
	if _, exists := g.Board.Grid[pos]; exists {
		g.Board.Grid[pos] = nil
	}
}

func (g *Game) MovePiece(move Move) bool {

	// Validate move
	piece, exists := g.Board.Grid[move.From]
	if !exists || piece == nil || piece.PieceID != move.PieceID || piece.PlayerID != move.PlayerID {
		// Invalid move, update and break
		return false
	}

	// Move piece to new position
	g.Board.Grid[move.To] = piece
	g.Board.Grid[move.From] = nil

	//if move.IsCapture && !piece.IsKinged {
	//	midRow := (move.From[0] + move.To[0]) / 2
	//	midCol := (move.From[1] + move.To[1]) / 2
	//	capturePos := fmt.Sprintf("%c%c", midRow, midCol)
	//	g.Board.Grid[capturePos] = nil // Remove captured piece
	//}
	// Handle capture
	if move.IsCapture {
		var capturePos string
		if piece.IsKinged {
			// For kinged pieces, the captured piece is the last square before the landing position
			fromRow, fromCol := move.From[0], move.From[1]
			toRow, toCol := move.To[0], move.To[1]

			// Calculate the direction of movement
			rowStep := 1
			if toRow < fromRow {
				rowStep = -1
			}
			colStep := 1
			if toCol < fromCol {
				colStep = -1
			}

			// Calculate the position of the captured piece
			captureRow := toRow - byte(rowStep)
			captureCol := toCol - byte(colStep)
			capturePos = fmt.Sprintf("%c%c", captureRow, captureCol)
		} else {
			// For regular pieces, the captured piece is in the middle of the from and to positions
			midRow := (move.From[0] + move.To[0]) / 2
			midCol := (move.From[1] + move.To[1]) / 2
			capturePos = fmt.Sprintf("%c%c", midRow, midCol)
		}

		// Remove the captured piece
		g.Board.Grid[capturePos] = nil
	}
	return true
}

func (g *Game) CheckGameOver() bool {
	// Check each player for pieces
	for _, player := range g.Players {
		if player.NumPieces == 0 {
			return true // Game over if any player's pieces are zero
		}
	}
	return false // Game continues if both players have pieces
}

func (g *Game) FinishGame(winnerID string) {
	g.Winner = winnerID
	g.EndTime = time.Now()
}
