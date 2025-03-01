package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Piece struct {
	Type     string `json:"type"`
	PlayerID string `json:"player_id"`
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
	Color     string `json:"color"`
	SessionID string `json:"session_id"`
	NumPieces int    `json:"num_pieces"`
}

type Game struct {
	ID              string       `json:"id"`
	Board           Board        `json:"board"`
	Players         []GamePlayer `json:"players"`
	CurrentPlayerID string       `json:"current_player_id"`
	Turn            int          `json:"turn"`
	Kinged          Kinged       `json:"kinged"`
	Moves           []string     `json:"moves"`
	StartTime       time.Time    `json:"start_time"`
	EndTime         time.Time    `json:"end_time"`
	Winner          string       `json:"winner"`
	BetValue        float64      `json:"bet_value"` // Bet amount for the game

}

type Kinged struct {
	W []string `json:"w"`
	B []string `json:"b"`
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
	whiteID, err := r.GetOpponentPlayerID(r.CurrentPlayerID)
	if err != nil {
		// TODO: Return an error?
	}

	game := Game{
		ID:              r.ID,
		Board:           *NewBoard(r.CurrentPlayerID, whiteID, "std-game"),
		Players:         mapPlayers(r),
		CurrentPlayerID: r.CurrentPlayerID,
		Turn:            r.Turn,
		Kinged:          Kinged{W: []string{}, B: []string{}},
		Moves:           []string{},
		StartTime:       time.Now(),
		Winner:          "",
		BetValue:        r.BetValue,
	}
	game.UpdatePlayerPieces() // Set NumPieces for each player

	//printBoard(game.Board)
	return &game
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
		fmt.Printf("Error NextPlayer getting opponent ID: %v\n", err)

	}
	g.CurrentPlayerID = nextPlayerId
	g.Turn += 1
}

func (g *Game) RemovePiece(pos string) {
	if _, exists := g.Board.Grid[pos]; exists {
		g.Board.Grid[pos] = nil
	}
}

func (g *Game) MovePiece(move Move) {

	// Validate move
	piece, exists := g.Board.Grid[move.From] // TODO: This was commented, since the FE seems to be sending the wrong ids.
	if !exists || piece == nil || piece.PieceID != move.PieceID /*|| piece.PlayerID != move.PlayerID*/ {
		// Invalid move, update and break
		return
	}

	// Move piece to new position
	g.Board.Grid[move.To] = piece
	g.Board.Grid[move.From] = nil

	// TODO: Handle kinging, find kinged piece.
	if move.IsKinged {
		//piece.Type = strings.ToUpper(piece.Type) // Convert to uppercase to indicate a king
	}

	// TODO: Review and test this. Handle capture (assumes captured piece's position is between From and To)
	if move.IsCapture {
		midRow := (move.From[0] + move.To[0]) / 2
		midCol := (move.From[1] + move.To[1]) / 2
		capturePos := fmt.Sprintf("%c%c", midRow, midCol)
		g.Board.Grid[capturePos] = nil // Remove captured piece
	}
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

func (g *Game) FinishGame() {
	g.Winner = g.CurrentPlayerID
	g.EndTime = time.Now()
}

// TODO: USE helper function for logging errors
func logError(message string, err error) {
	//fmt.Printf("[%s-%d] - (Process Game Moves) - %s: %v\n", name, pid, message, err)
}
