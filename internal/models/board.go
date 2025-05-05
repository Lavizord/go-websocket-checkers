package models

import (
	"fmt"
	"log"

	"github.com/google/uuid"
)

type Board struct {
	Grid map[string]*Piece
}

func NewBoard(blackID, whiteID, boardtype string) *Board {
	board := &Board{Grid: make(map[string]*Piece)}
	switch boardtype {
	case "std-game":
		board.GenerateInitialBoard(blackID, whiteID) // Automatically initialize board state
	case "two-pieces-endgame":
		board.GenerateEndGameTestBoard(blackID, whiteID) // Automatically initialize board state
	case "multiple-capture":
		board.GenerateMultipleCaptureTestBoard(blackID, whiteID) // Automatically initialize board state
	}
	return board
}

// GenerateInitialBoard initializes the board with starting pieces
func (b *Board) GenerateInitialBoard(blackID, whiteID string) {
	rows := []string{"A", "B", "C", "D", "E", "F", "G", "H"}

	for i, row := range rows {
		for col := 1; col <= 8; col++ {
			pos := fmt.Sprintf("%s%d", row, col)
			b.Grid[pos] = nil
			// Only place pieces on dark squares
			if (i+col)%2 == 1 {
				if i < 3 { // Top 3 rows for black pieces
					b.Grid[pos] = &Piece{Type: "b", PieceID: uuid.New().String(), PlayerID: blackID}
				} else if i > 4 { // Bottom 3 rows for white pieces
					b.Grid[pos] = &Piece{Type: "w", PieceID: uuid.New().String(), PlayerID: whiteID}
				} else {
					b.Grid[pos] = nil // Empty middle rows
				}
			}
		}
	}
}

// GenerateEndGameTestBoard initializes the board with a test configuration
func (b *Board) GenerateEndGameTestBoard(blackID, whiteID string) {
	// Set positions for testing
	testPositions := map[string]*Piece{
		"A2": {Type: "b", PieceID: uuid.New().String(), PlayerID: blackID}, // Black piece
		"B3": {Type: "w", PieceID: uuid.New().String(), PlayerID: whiteID}, // White piece
	}

	// Initialize board and place test pieces
	for row := 'A'; row <= 'H'; row++ {
		for col := 1; col <= 8; col++ {
			pos := fmt.Sprintf("%c%d", row, col)
			if piece, exists := testPositions[pos]; exists {
				b.Grid[pos] = piece // Place test pieces
			} else {
				b.Grid[pos] = nil // Empty squares
			}
		}
	}
}

// Test config for multiple capture
func (b *Board) GenerateMultipleCaptureTestBoard(blackID, whiteID string) {
	// Set positions for testing
	testPositions := map[string]*Piece{
		"A1": {Type: "b", PieceID: uuid.New().String(), PlayerID: blackID},
		"A3": {Type: "b", PieceID: uuid.New().String(), PlayerID: blackID},
		"A7": {Type: "b", PieceID: uuid.New().String(), PlayerID: blackID},
		"B2": {Type: "b", PieceID: uuid.New().String(), PlayerID: blackID},
		"B4": {Type: "b", PieceID: uuid.New().String(), PlayerID: blackID},
		"C5": {Type: "b", PieceID: uuid.New().String(), PlayerID: blackID},
		"D6": {Type: "w", PieceID: uuid.New().String(), PlayerID: whiteID},
		"E7": {Type: "w", PieceID: uuid.New().String(), PlayerID: whiteID},
	}

	// Initialize board and place test pieces
	for row := 'A'; row <= 'H'; row++ {
		for col := 1; col <= 8; col++ {
			pos := fmt.Sprintf("%c%d", row, col)
			if piece, exists := testPositions[pos]; exists {
				b.Grid[pos] = piece // Place test pieces
			} else {
				b.Grid[pos] = nil // Empty squares
			}
		}
	}
}

func (b *Board) GetPieceByID(pieceID string) *Piece {
	for _, piece := range b.Grid {
		if piece == nil {
			continue
		}
		if piece.PieceID == pieceID {
			return piece
		}
	}
	return nil
}

func (b *Board) CanPieceCaptureNEW(pos string) bool {
	piece, exists := b.Grid[pos]
	if !exists || piece == nil {
		log.Printf("(CanPieceCapture) - Piece doesn't exist on the board\n")
		return false // No piece at this position
	}

	// Define the possible capture directions
	var directions []struct{ rowDelta, colDelta int }

	if piece.IsKinged {
		// Kings can move in all four diagonal directions
		directions = []struct{ rowDelta, colDelta int }{
			{1, 1},   // Diagonal right (down)
			{1, -1},  // Diagonal left (down)
			{-1, 1},  // Diagonal right (up)
			{-1, -1}, // Diagonal left (up)
		}
		//log.Printf("(CanPieceCapture) - Piece is king, multiple directions.")
	} else {
		//log.Printf("(CanPieceCapture) - Piece is not king, single direction.")
		// Normal pieces can only move forward
		var direction = GetPieceDirection(*piece)
		directions = []struct{ rowDelta, colDelta int }{
			{direction, 1},  // Diagonal right
			{direction, -1}, // Diagonal left
		}
	}

	// Convert position (e.g., "A3" → row 'A', col 3)
	fromRow, fromCol, err := parsePosition(pos)
	if err != nil {
		log.Println(err)
		return false
	}

	for _, dir := range directions {
		// Compute middle position (opponent's piece)
		midRow := fromRow + rune(dir.rowDelta)
		midCol := fromCol + dir.colDelta
		midPos := fmt.Sprintf("%c%d", midRow, midCol)

		// Compute landing position
		landRow := fromRow + rune(2*dir.rowDelta)
		landCol := fromCol + 2*dir.colDelta
		landPos := fmt.Sprintf("%c%d", landRow, landCol)
		//log.Printf("(CanPieceCapture) - Checking landPos [%s]\n", landPos)
		// Ensure middle square has an opponent piece
		midPiece, midExists := b.Grid[midPos]
		if !midExists || midPiece == nil || midPiece.PlayerID == piece.PlayerID {
			//log.Printf("(CanPieceCapture) - Middle piece doesn't exist or is not an opponent!\n")
			continue // No opponent to jump over
		}
		//log.Printf("(CanPieceCapture) - Middle piece exists and is an opponent!\n")
		// Ensure landing square is empty
		if destPiece, destExists := b.Grid[landPos]; destExists && destPiece == nil {
			//log.Printf("(CanPieceCapture) - Valid capture move found\n")
			return true // Valid capture move found!
		}
	}
	//log.Printf("(CanPieceCapture) - No captures available\n")
	return false // No captures available
}

func (b *Board) WasPieceKinged(pos string, piece Piece) bool {
	if piece.IsKinged { // If its alerady a king we just return false.
		return false
	}

	if len(pos) == 0 {
		return false
	}
	firstChar := pos[0]
	if piece.Type == "b" && firstChar == 'H' {
		log.Printf("(WasPieceKinged) - Black piece was kinged!")
		return true
	}
	if piece.Type == "w" && firstChar == 'A' {
		log.Printf("(WasPieceKinged) - White piece was kinged!")
		return true
	}
	return false
}

func GetPieceDirection(piece Piece) int {
	if piece.Type == "w" {
		return -1 // White pieces move "up" (decreasing row)
	} else {
		return 1 // Black pieces move "down" (increasing row)
	}
}

func (b *Board) PiecesThatCanCapture(playerID string) []Piece {
	var capturers []Piece
	for pos, piece := range b.Grid {
		if piece == nil {
			continue
		}
		if piece.PlayerID != playerID {
			continue
		}
		if b.CanPieceCaptureNEW(pos) {
			capturers = append(capturers, *piece)
		}
	}
	return capturers
}

// parsePosition converts a position string (e.g., "A3") into row (rune) and column (int).
// Returns an error if the position is invalid.
func parsePosition(pos string) (rune, int, error) {
	if len(pos) != 2 {
		return 0, 0, fmt.Errorf("(Parse Position) - invalid position format: must be 2 characters (e.g., 'A3')")
	}
	row := rune(pos[0])      // Convert the first character to a rune (e.g., 'A')
	col := int(pos[1] - '0') // Convert the second character to an integer (e.g., '3' → 3)

	// Validate the row and column
	if row < 'A' || row > 'H' || col < 1 || col > 8 {
		return 0, 0, fmt.Errorf("(Parse Position) - position is out of bounds: must be between A1 and H8")
	}
	return row, col, nil
}

func (b *Board) IsValidMove(move Move) (bool, error) {
	piece, exists := b.Grid[move.From]
	if !exists || piece == nil {
		return false, fmt.Errorf("(isValidMove) - piece does not exist at the source square")
	}
	// Check if the piece belongs to the player making the move
	if piece.PlayerID != move.PlayerID {
		return false, fmt.Errorf("(isValidMove) - piece does not belong to the player")
	}
	fromRow, fromCol, err := parsePosition(move.From)
	if err != nil {
		return false, fmt.Errorf("(isValidMove) - invalid source position: %v", err)
	}
	toRow, toCol, err := parsePosition(move.To)
	if err != nil {
		return false, fmt.Errorf("(isValidMove) - invalid destination position: %v", err)
	}
	// Check if the destination square is empty
	_, exists = b.Grid[move.To]
	if !exists {
		return false, fmt.Errorf("(isValidMove) - destination square doesn't exist")
	}
	if b.Grid[move.To] != nil {
		return false, fmt.Errorf("(isValidMove) - destination square is not empty")
	}
	// Calculate the difference in rows and columns
	deltaRow := int(toRow - fromRow)
	deltaCol := toCol - fromCol
	// Skip direction validation if the piece is kinged
	if !piece.IsKinged {
		direction := GetPieceDirection(*piece)
		if deltaRow*direction <= 0 { // Check if the move is in the correct direction
			return false, fmt.Errorf("(isValidMove) - move is not in the correct direction for the piece type")
		}
	}
	// Check if the move is diagonal
	if abs(deltaCol) != 1 || abs(deltaRow) != 1 {
		// If it's not a single diagonal move, check if it's a capture
		if !move.IsCapture || abs(deltaCol) != 2 || abs(deltaRow) != 2 {
			return false, fmt.Errorf("(isValidMove) - move is not diagonal or a valid capture")
		}

		// For a capture, check if the intermediate square has an opponent's piece
		captureRow := fromRow + rune(deltaRow/2)
		captureCol := fromCol + deltaCol/2
		captureSquare := string(captureRow) + string('0'+captureCol)
		capturedPiece, exists := b.Grid[captureSquare]
		if !exists || capturedPiece == nil || capturedPiece.PlayerID == move.PlayerID {
			return false, fmt.Errorf("(isValidMove) - invalid capture: no opponent's piece to capture")
		}
	}
	// If all checks pass, the move is valid
	return true, nil
}

func (b *Board) IsValidMoveKing(move Move) (bool, error) {
	piece, exists := b.Grid[move.From]
	if !exists || piece == nil {
		return false, fmt.Errorf("(IsValidMoveKing) - piece does not exist at source")
	}
	if piece.PlayerID != move.PlayerID {
		return false, fmt.Errorf("(IsValidMoveKing) - piece does not belong to player")
	}
	if !piece.IsKinged {
		return false, fmt.Errorf("(IsValidMoveKing) - piece is not kinged")
	}

	fromRow, fromCol, err := parsePosition(move.From)
	if err != nil {
		return false, fmt.Errorf("(IsValidMoveKing) - invalid source: %v", err)
	}
	toRow, toCol, err := parsePosition(move.To)
	if err != nil {
		return false, fmt.Errorf("(IsValidMoveKing) - invalid destination: %v", err)
	}

	if _, ok := b.Grid[move.To]; !ok {
		return false, fmt.Errorf("(IsValidMoveKing) - destination does not exist")
	}
	if b.Grid[move.To] != nil {
		return false, fmt.Errorf("(IsValidMoveKing) - destination not empty")
	}

	deltaRow := int(toRow - fromRow)
	deltaCol := int(toCol - fromCol)

	if abs(deltaRow) != abs(deltaCol) {
		return false, fmt.Errorf("(IsValidMoveKing) - move not diagonal")
	}

	stepRow := 1
	if deltaRow < 0 {
		stepRow = -1
	}
	stepCol := 1
	if deltaCol < 0 {
		stepCol = -1
	}

	enemySeen := false
	for r, c := fromRow+rune(stepRow), fromCol+stepCol; r != toRow && c != toCol; r, c = r+rune(stepRow), c+stepCol {
		square := string(r) + string('0'+c)
		p, exists := b.Grid[square]
		if !exists {
			return false, fmt.Errorf("(IsValidMoveKing) - square %v does not exist", square)
		}
		if p == nil {
			continue
		}
		if p.PlayerID == move.PlayerID {
			return false, fmt.Errorf("(IsValidMoveKing) - path blocked by own piece at %v", square)
		}
		if enemySeen {
			return false, fmt.Errorf("(IsValidMoveKing) - multiple captures not supported in one move")
		}
		enemySeen = true
	}

	if enemySeen && !move.IsCapture {
		return false, fmt.Errorf("(IsValidMoveKing) - move is a capture but not flagged as capture")
	}
	if !enemySeen && move.IsCapture {
		return false, fmt.Errorf("(IsValidMoveKing) - flagged as capture but no enemy on path")
	}

	return true, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
