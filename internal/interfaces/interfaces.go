package interfaces

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Lavizord/checkers-server/internal/models"
	"github.com/Lavizord/checkers-server/internal/postgrescli"
	"github.com/Lavizord/checkers-server/internal/redisdb"
	"github.com/Lavizord/checkers-server/internal/walletrequests"
)

// OperatorModule defines the interface for operator-specific code
type OperatorInterface interface {
	HandleGameLaunch(w http.ResponseWriter, r *http.Request, req models.GameLaunchRequest, op models.Operator, rc *redisdb.RedisClient, pgs *postgrescli.PostgresCli)
	HandleFetchWalletBalance(s models.Session, rc *redisdb.RedisClient) (int64, error)
	HandlePostBet(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, betValue int64, gameID string) (int64, error)
	HandlePostWin(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, betValue int64, gameID string) (int64, int64, error)
}

// OperatorModules maps operator names to their respective modules
var OperatorModules = map[string]OperatorInterface{
	"SokkerDuel": &SokkerDuelModule{},
	"TestOp":     &TestModule{},
	//"AnotherOperator": &AnotherOperatorModule{},
	// Add more operators as needed
}

// SokkerDuelModule handles requests for the SokkerDuel operator
type SokkerDuelModule struct{}

func (m *SokkerDuelModule) HandleGameLaunch(w http.ResponseWriter, r *http.Request, req models.GameLaunchRequest, op models.Operator, rc *redisdb.RedisClient, pgs *postgrescli.PostgresCli) {
	// Fetch wallet information
	logInResponse, err := walletrequests.SokkerDuelGetWallet(op.OperatorWalletBaseUrl, req.Token)
	if err != nil {
		respondWithError(w, "Failed to fetch wallet", err)
		return
	}
	if logInResponse.Status != "success" {
		respondWithError(w, "Wallet request != success", fmt.Errorf("api err: %v", logInResponse.Data))
		return
	}
	session, err := checkExistingSession(req.Token, rc)
	if err != nil || session == nil {
		session, _ = checkPreviousPlayerSession(req.OperatorName, logInResponse.Data.Username, req.Currency, rc)
		if session != nil {
			rc.DisconnectPlayer(session.ID) // We send a message to disconnect the previous websocket connection.
			rc.RemoveSession(session.ID)    // If the session exists, from a previous token, we remove the session
			session.Token = req.Token       // We just update the token.
			rc.AddSession(session)          // we update the session in redis.
		} else {
			session, err = generatePlayerSession( // then we generate a new session.
				op,
				req.Token,
				logInResponse.Data.Username,
				logInResponse.Data.Currency,
				rc,
			)
			if err != nil {
				respondWithError(w, "Failed to generate session", err)
				return
			}
			err = pgs.SaveSession(*session)
			if err != nil {
				respondWithError(w, "Failed to save session", err)
				return
			}
		}
	}
	gameURL, err := generateGameURL(op.GameBaseUrl, req.Token, session.ID, logInResponse.Data.Currency)
	if err != nil {
		respondWithError(w, "Failed to generate game URL", err)
		return
	}
	// Final response
	response := models.SokkerDuelGamelaunchResponse{
		Token: req.Token,
		Url:   gameURL,
	}
	respondWithJSON(w, http.StatusOK, response)
}

func (m *SokkerDuelModule) HandleFetchWalletBalance(s models.Session, rc *redisdb.RedisClient) (int64, error) {
	logInResponse, err := walletrequests.SokkerDuelGetWallet(s.OperatorBaseUrl, s.Token)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch wallet: %v", err)
	}
	return int64(logInResponse.Data.Balance), nil
}

func (m *SokkerDuelModule) HandlePostBet(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, betValue int64, gameID string) (int64, error) {
	// Validate input parameters
	if betValue <= 0 {
		return -1, fmt.Errorf("invalid bet value: %d", betValue)
	}
	if gameID == "" {
		return -1, fmt.Errorf("empty game ID")
	}
	if session.ID == "" {
		return -1, fmt.Errorf("invalid session")
	}

	betData := models.SokkerDuelBet{
		OperatorGameName: session.OperatorIdentifier.GameName,
		Currency:         session.Currency,
		Amount:           betValue,
		TransactionID:    models.GenerateUUID(),
		RoundID:          gameID,
	}
	// Make API call - now we know it either returns success response or error
	betResponse, err := walletrequests.SokkerDuelPostBet(session, betData)
	if err != nil {
		if saveErr := saveFailedBetTransaction(pgs, session, betData, err, gameID); saveErr != nil {
			return -1, fmt.Errorf("API error: %v | Transaction save error: %v", err, saveErr)
		}
		return -1, err // Return original API error
	}

	// At this point, we're guaranteed betResponse is valid and status="success"
	// Prepare and save transaction
	trans := models.Transaction{
		ID:          betData.TransactionID,
		SessionID:   session.ID,
		Type:        "bet",
		Amount:      betValue,
		Currency:    session.Currency,
		Platform:    "sokkerpro",
		Operator:    "SokkerDuel",
		Client:      session.PlayerName,
		Game:        session.OperatorIdentifier.GameName,
		RoundID:     gameID,
		Timestamp:   time.Now(),
		Status:      betResponse.Status,
		Description: string(mustMarshal(betResponse)), // Safe because we know Data exists
	}
	if err := pgs.SaveTransaction(trans); err != nil {
		return -1, fmt.Errorf("failed to save transaction: %v", err)
	}
	// Update session
	session.ExtractID = betResponse.Data.ExtractID
	if err := rc.AddSession(&session); err != nil {
		return -1, fmt.Errorf("failed to save session: %v", err)
	}
	// Parse balance (we know it exists from API contract)
	fbalance, err := strconv.ParseFloat(betResponse.Data.Balance, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse balance: %v", err)
	}
	return int64(fbalance * 100), nil
}

func (m *SokkerDuelModule) HandlePostWin(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, winValue int64, gameID string) (int64, int64, error) {
	// Validate input parameters
	if winValue <= 0 {
		return -1, -1, fmt.Errorf("invalid win value: %d", winValue)
	}
	if gameID == "" {
		return -1, -1, fmt.Errorf("empty game ID")
	}
	if session.ID == "" {
		return -1, -1, fmt.Errorf("invalid session")
	}
	var winnings = CalculateWinAmount(winValue, session.OperatorIdentifier.WinFactor)
	winData := models.SokkerDuelWin{
		OperatorGameName: session.OperatorIdentifier.GameName,
		Currency:         session.Currency,
		Amount:           winnings,
		TransactionID:    models.GenerateUUID(),
		RoundID:          gameID,
		ExtractID:        session.ExtractID,
	}
	// Make API call - guaranteed to return either success response or error
	winResponse, err := walletrequests.SokkerDuelPostWin(session, winData)
	if err != nil {
		// Save failed transaction before returning
		if saveErr := saveFailedWinTransaction(pgs, session, winData, err, gameID); saveErr != nil {
			return -1, winnings, fmt.Errorf("API error: %v | Transaction save error: %v", err, saveErr)
		}
		return -1, -1, err // Return original API error
	}
	// At this point, we're guaranteed winResponse is valid and status="success"
	trans := models.Transaction{
		ID:          winData.TransactionID,
		SessionID:   session.ID,
		Type:        "win",
		Amount:      winnings,
		Currency:    session.Currency,
		Platform:    "sokkerpro",
		Operator:    "SokkerDuel",
		Client:      session.PlayerName,
		Game:        session.OperatorIdentifier.GameName,
		RoundID:     gameID,
		Timestamp:   time.Now(),
		Status:      winResponse.Status,
		Description: string(mustMarshal(winResponse)), // Safe because we know Data exists
	}
	if err := pgs.SaveTransaction(trans); err != nil {
		return -1, -1, fmt.Errorf("failed to save transaction: %v", err)
	}
	// Reset ExtractID in session
	session.ExtractID = 0
	if err := rc.AddSession(&session); err != nil {
		return -1, -1, fmt.Errorf("failed to save session: %v", err)
	}
	// Parse balance (we know it exists from API contract)
	fbalance, err := strconv.ParseFloat(winResponse.Data.Balance, 64)
	if err != nil {
		return -1, -1, fmt.Errorf("failed to parse balance: %v", err)
	}
	return int64(fbalance * 100), winnings, nil
}

// Helper function to save failed transactions
func saveFailedBetTransaction(pgs *postgrescli.PostgresCli, session models.Session, betData models.SokkerDuelBet, apiError error, gameID string) error {
	trans := models.Transaction{
		ID:          betData.TransactionID,
		SessionID:   session.ID,
		Type:        "bet",
		Amount:      betData.Amount,
		Currency:    session.Currency,
		Platform:    "sokkerpro",
		Operator:    "SokkerDuel",
		Client:      session.PlayerName,
		Game:        session.OperatorIdentifier.GameName,
		RoundID:     gameID,
		Timestamp:   time.Now(),
		Status:      "error",
		Description: apiError.Error(),
	}
	return pgs.SaveTransaction(trans)
}

func saveFailedWinTransaction(pgs *postgrescli.PostgresCli, session models.Session, winData models.SokkerDuelWin, apiError error, gameID string) error {
	trans := models.Transaction{
		ID:          winData.TransactionID,
		SessionID:   session.ID,
		Type:        "win",
		Amount:      winData.Amount,
		Currency:    session.Currency,
		Platform:    "sokkerpro",
		Operator:    "SokkerDuel",
		Client:      session.PlayerName,
		Game:        session.OperatorIdentifier.GameName,
		RoundID:     gameID,
		Timestamp:   time.Now(),
		Status:      "error",
		Description: apiError.Error(),
	}
	return pgs.SaveTransaction(trans)
}

func generateGameURL(baseURL, token, sessionID, currency string) (string, error) {
	// Parse the base URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %v", err)
	}

	// Add query parameters
	query := url.Values{}
	query.Add("token", token)
	query.Add("sessionid", sessionID)
	query.Add("currency", currency)
	parsedURL.RawQuery = query.Encode()

	// Return the full URL as a string
	return parsedURL.String(), nil
}

func generatePlayerSession(op models.Operator, token, username, currency string, rc *redisdb.RedisClient) (*models.Session, error) {
	session := models.Session{
		ID:         models.GenerateUUID(),
		Token:      token,
		PlayerName: username,
		Currency:   currency,
		OperatorIdentifier: models.OperatorIdentifier{
			OperatorName:     op.OperatorName,
			OperatorGameName: op.OperatorGameName,
			GameName:         op.GameName,
			WinFactor:        op.WinFactor,
		},
		OperatorBaseUrl: op.OperatorWalletBaseUrl,
		CreatedAt:       time.Now(),
	}
	err := rc.AddSession(&session)
	return &session, err
}

func checkExistingSession(token string, rc *redisdb.RedisClient) (*models.Session, error) {
	// First, check Redis for an active session
	session, err := rc.GetSessionByToken(token)
	if err == nil && session != nil {
		return session, nil // Session exists
	}
	return nil, fmt.Errorf("session not found")
}

func checkPreviousPlayerSession(operator string, playerName string, currency string, rc *redisdb.RedisClient) (*models.Session, error) {
	//fmt.Printf("Checking previous Player session: %v, %v, %v.", operator, playerName, currency)
	session, err := rc.GetSessionByOperatorPlayerCurrency(operator, playerName, currency)
	if err == nil && session != nil {
		fmt.Printf("Session found!: ID:%v", session.ID)
		return session, nil // Session exists
	}
	//fmt.Printf("Session not found.")
	return nil, fmt.Errorf("session not found")
}

// Helper function to send JSON errors
func respondWithError(w http.ResponseWriter, message string, err error) {
	respondWithJSON(w, http.StatusInternalServerError, map[string]string{
		"error":   message,
		"details": err.Error(),
	})
}

// Helper function to send JSON responses
func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(payload)
}

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("failed to marshal API response: %v", err)
		return []byte("")
	}
	return b
}

func CalculateWinAmount(betValue int64, winFactor float64) int64 {
	// Multiply by 2 then by 0.9 (equivalent to multiplying by 1.8)
	// Using float64 for precise multiplication then converting back to int
	winAmount := float64(betValue*2) * winFactor
	//log.Printf("Calculating win amount:[betValue: %v], [winFactor: %v], final value: [%v]", betValue, winFactor, winAmount)
	return int64(winAmount) // Truncates decimal places
}

type TestModule struct{}

func (m *TestModule) HandleGameLaunch(w http.ResponseWriter, r *http.Request, req models.GameLaunchRequest, op models.Operator, rc *redisdb.RedisClient, pgs *postgrescli.PostgresCli) {

	session, err := checkExistingSession(req.Token, rc)
	if err != nil || session == nil {
		session, _ = checkPreviousPlayerSession(req.OperatorName, "TESTUSER", req.Currency, rc)
		if session != nil {
			rc.RemoveSession(session.ID) // If the session exists, from a previous token, we remove the session
		}
		session, err = generatePlayerSession( // then we generate a new session.
			op,
			req.Token,
			models.GenerateUUID(),
			req.Currency,
			rc,
		)
		if err != nil {
			respondWithError(w, "Failed to generate session", err)
			return
		}
		err = pgs.SaveSession(*session)
		if err != nil {
			respondWithError(w, "Failed to save session", err)
			return
		}
	}
	gameURL, err := generateGameURL(op.GameBaseUrl, req.Token, session.ID, req.Currency)
	if err != nil {
		respondWithError(w, "Failed to generate game URL", err)
		return
	}
	// Final response
	response := models.SokkerDuelGamelaunchResponse{
		Token: req.Token,
		Url:   gameURL,
	}
	respondWithJSON(w, http.StatusOK, response)
}

func (m *TestModule) HandleFetchWalletBalance(s models.Session, rc *redisdb.RedisClient) (int64, error) {
	return int64(10000), nil
}

func (m *TestModule) HandlePostBet(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, betValue int64, gameID string) (int64, error) {
	return 100, nil
}
func (m *TestModule) HandlePostWin(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, winValue int64, gameID string) (int64, int64, error) {
	return 199, 99, nil
}
