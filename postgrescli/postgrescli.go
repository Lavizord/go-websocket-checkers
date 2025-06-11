package postgrescli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Lavizord/checkers-server/models"
	"github.com/google/uuid"

	_ "github.com/lib/pq"
)

type PostgresCli struct {
	DB *sql.DB
}

func NewPostgresCli(user, password, dbname, host, port string, ssl bool) (*PostgresCli, error) {
	connStr := fmt.Sprintf(
		"user=%s password=%s dbname=%s host=%s port=%s binary_parameters=no",
		user, password, dbname, host, port)

	if ssl {
		connStr += " sslmode=require"
	} else {
		connStr += " sslmode=disable"
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetConnMaxLifetime(time.Minute * 5)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresCli{DB: db}, nil
}

func (pc *PostgresCli) Close() {
	pc.DB.Close()
}

func (pc *PostgresCli) SaveSession(session models.Session) error {
	query := `
		INSERT INTO sessions (
			SessionId, Token, PlayerName, Currency, OperatorBaseUrl, CreatedAt, OperatorName, OperatorGameName, GameName
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	log.Printf("Saving session: %+v\n", session)

	_, err := pc.DB.Exec(
		query,
		session.ID,
		session.Token,
		session.PlayerName,
		session.Currency,
		session.OperatorBaseUrl,
		session.CreatedAt,
		session.OperatorIdentifier.OperatorName,
		session.OperatorIdentifier.OperatorGameName,
		session.OperatorIdentifier.GameName,
	)
	if err != nil {
		return fmt.Errorf("error inserting session: %w", err)
	}

	//log.Printf("Session saved with ID: %s\n", session.ID)
	return nil
}

func (pc *PostgresCli) SaveTransaction(transaction models.Transaction) error {
	query := `
		INSERT INTO transactions (
			TransactionID, SessionID, Type, Amount, Currency, Platform, Operator, Client, Game, Status, Description, RoundID, Timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING TransactionID
	`

	var transactionID string
	err := pc.DB.QueryRow(
		query,
		transaction.ID,
		transaction.SessionID,
		transaction.Type,
		transaction.Amount,
		transaction.Currency,
		transaction.Platform,
		transaction.Operator,
		transaction.Client,
		transaction.Game,
		transaction.Status,
		transaction.Description,
		transaction.RoundID,
		transaction.Timestamp,
	).Scan(&transactionID)

	if err != nil {
		return fmt.Errorf("error inserting transaction: %w", err)
	}

	return nil
}

// SaveGame method to save a game to the database
func (pc *PostgresCli) SaveGame(game models.Game, reason string) error {
	// Convert moves to JSONB
	movesJSON, err := json.Marshal(game.Moves)
	if err != nil {
		return fmt.Errorf("error marshalling moves: %w", err)
	}

	// Convert game_players to JSONB
	playersJSON, err := json.Marshal(game.Players)
	if err != nil {
		return fmt.Errorf("error marshalling players: %w", err)
	}

	// SQL query to insert the game data
	query := `
		INSERT INTO games (
			ID, OperatorName, OperatorGameName, GameName, StartDate, EndDate, Moves, BetAmount, Winner, GamePlayers, WinFactor, NumMoves, GameOverReason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id
	`
	var gameID string
	err = pc.DB.QueryRow(
		query,
		game.ID,
		game.OperatorIdentifier.OperatorName,
		game.OperatorIdentifier.OperatorGameName,
		game.OperatorIdentifier.GameName,
		game.StartTime,
		game.EndTime,
		movesJSON,
		game.BetValue,
		game.Winner,
		playersJSON,
		game.OperatorIdentifier.WinFactor,
		len(game.Moves),
		reason,
	).Scan(&gameID)

	if err != nil {
		log.Printf("error inserting game: %v", err)
		return fmt.Errorf("error inserting game: %w", err)
	}
	//log.Printf("Game saved with ID: %d\n", gameID)
	return nil
}

func (pc *PostgresCli) FetchGameMoves(gameID string) ([]models.Move, error) {
	query := `SELECT moves FROM games WHERE id = $1`
	log.Printf("Fetching moves for gameID: %s", gameID) // not [%s]
	parsedUUID, err := uuid.Parse(gameID)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}
	log.Printf("Fetching moves for gameID: %s", gameID) // not [%s]

	var movesJSON []byte
	err = pc.DB.QueryRow(query, parsedUUID).Scan(&movesJSON)
	if err != nil {
		return nil, fmt.Errorf("error fetching moves for game %s: %w", gameID, err)
	}

	var moves []models.Move
	if err := json.Unmarshal(movesJSON, &moves); err != nil {
		return nil, fmt.Errorf("error unmarshalling moves JSON: %w", err)
	}

	return moves, nil
}

// FetchOperator fetches an operator from the database using OperatorName and OperatorGameName
func (pc *PostgresCli) FetchOperator(operatorName, operatorGameName string) (*models.Operator, error) {
	query := `
		SELECT ID, OperatorName, OperatorGameName, GameName, Active, GameBaseUrl, OperatorWalletBaseUrl, WinFactor
		FROM operators
		WHERE OperatorName = $1 AND OperatorGameName = $2
	`
	row := pc.DB.QueryRow(query, operatorName, operatorGameName)

	var operator models.Operator
	err := row.Scan(
		&operator.ID,
		&operator.OperatorName,
		&operator.OperatorGameName,
		&operator.GameName,
		&operator.Active,
		&operator.GameBaseUrl,
		&operator.OperatorWalletBaseUrl,
		&operator.WinFactor,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("operator not found with OperatorName=%s and OperatorGameName=%s", operatorName, operatorGameName)
		}
		return nil, fmt.Errorf("error fetching operator: %w", err)
	}

	return &operator, nil
}
