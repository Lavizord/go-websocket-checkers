package interfaces

import (
	"checkers-server/models"
	"checkers-server/postgrescli"
	"checkers-server/redisdb"
	"checkers-server/walletrequests"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// OperatorModule defines the interface for operator-specific code
type OperatorInterface interface {
	HandleGameLaunch(w http.ResponseWriter, r *http.Request, req models.GameLaunchRequest, op models.Operator, rc *redisdb.RedisClient, pgs *postgrescli.PostgresCli)
	HandlePostBet(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, betValue int, gameID string) error
	HandlePostWin(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, betValue int, gameID string) error
}

// OperatorModules maps operator names to their respective modules
var OperatorModules = map[string]OperatorInterface{
	"SokkerDuel": &SokkerDuelModule{},
	//"AnotherOperator": &AnotherOperatorModule{},
	// Add more operators as needed
}

// SokkerDuelModule handles requests for the SokkerDuel operator
type SokkerDuelModule struct{}

func (m *SokkerDuelModule) HandleGameLaunch(w http.ResponseWriter, r *http.Request, req models.GameLaunchRequest, op models.Operator, rc *redisdb.RedisClient, pgs *postgrescli.PostgresCli) {
	// Fetch wallet information
	logInResponse, err := walletrequests.SokkerDuelGetWallet(op, req.Token)
	if err != nil || logInResponse.Status != "success" {
		http.Error(w, fmt.Sprintf("Failed to fetch wallet: %v, api err:%v", err, logInResponse.Data), http.StatusInternalServerError)
		return
	}

	session, err := generatePlayerSession(
		op,
		req.Token,
		logInResponse.Data.Username,
		logInResponse.Data.Currency,
		logInResponse.Data.Balance,
		rc,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate session: %v", err), http.StatusInternalServerError)
		return
	}
	err = pgs.SaveSession(session)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save session to postgres session: %v", err), http.StatusInternalServerError)
		return
	}
	gameURL, err := generateGameURL(op.GameBaseUrl, req.Token, session.ID, logInResponse.Data.Currency)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate game URL: %v", err), http.StatusInternalServerError)
		return
	}

	response := models.SokkerDuelGamelaunchResponse{
		Token: req.Token,
		Url:   gameURL,
	}

	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	// Encode the response
	if err := encoder.Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

func (m *SokkerDuelModule) HandlePostBet(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, betValue int, gameID string) error {

	betData := models.SokkerDuelBet{
		OperatorGameName: session.OperatorIdentifier.GameName,
		Currency:         session.Currency,
		Amount:           betValue,
		TransactionID:    models.GenerateUUID(),
	}
	betResponse, err := walletrequests.SokkerDuelPostBet(session, betData)
	// Prepare the transaction record
	trans := models.Transaction{
		ID:        betData.TransactionID,
		SessionID: session.ID,
		Type:      "bet",
		Amount:    betValue,
		Currency:  session.Currency,
		Platform:  "sokkerpro",
		Operator:  "sokkerduel",
		Client:    session.PlayerName,
		Game:      session.OperatorIdentifier.GameName,
		RoundID:   gameID,
		Timestamp: time.Now(),
	}
	if err != nil {
		// If there's an error, set the status to "error" and store the error message
		trans.Status = "error"
		trans.Description = err.Error()
	} else {
		// If the API call is successful, set the status from the response
		trans.Status = betResponse.Status
		// Marshal the response data and store it in the transaction
		marshalResponseData, _ := json.Marshal(betResponse.Data)
		trans.Description = string(marshalResponseData)
	}
	if err := pgs.SaveTransaction(trans); err != nil {
		// If saving the transaction fails, return the error
		return fmt.Errorf("failed to save transaction: %v", err)
	}
	session.ExtractID = betResponse.Data.ExtractID
	rc.AddSession(&session) // we save our session with the extract ID.
	// Return the API error if there was one
	if err != nil {
		return fmt.Errorf("API call failed: %v", err)
	}

	// If everything is successful, return nil
	return nil
}

func (m *SokkerDuelModule) HandlePostWin(pgs *postgrescli.PostgresCli, rc *redisdb.RedisClient, session models.Session, betValue int, gameID string) error {
	winData := models.SokkerDuelWin{
		OperatorGameName: session.OperatorIdentifier.GameName,
		Currency:         session.Currency,
		Amount:           betValue,
		TransactionID:    models.GenerateUUID(),
		ExtractID:        session.ExtractID,
	}
	betResponse, err := walletrequests.SokkerDuelPostWin(session, winData)

	// Prepare the transaction record
	trans := models.Transaction{
		ID:        winData.TransactionID,
		SessionID: session.ID,
		Type:      "win",
		Amount:    betValue,
		Currency:  session.Currency,
		Platform:  "sokkerpro",
		Operator:  "sokkerduel",
		Client:    session.PlayerName,
		Game:      session.OperatorIdentifier.GameName,
		RoundID:   gameID,
		Timestamp: time.Now(),
	}
	if err != nil {
		// If there's an error, set the status to "error" and store the error message
		trans.Status = "error"
		trans.Description = err.Error()
	} else {
		// If the API call is successful, set the status from the response
		trans.Status = betResponse.Status
		// Marshal the response data and store it in the transaction
		marshalResponseData, _ := json.Marshal(betResponse.Data)
		trans.Description = string(marshalResponseData)
	}
	if err := pgs.SaveTransaction(trans); err != nil {
		// If saving the transaction fails, return the error
		return fmt.Errorf("failed to save transaction: %v", err)
	}
	session.ExtractID = ""
	rc.AddSession(&session) // we save our session with the extract ID.
	// Return the API error if there was one
	if err != nil {
		return fmt.Errorf("API call failed: %v", err)
	}
	return nil
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
	query.Add("sessionId", sessionID)
	query.Add("currency", currency)
	parsedURL.RawQuery = query.Encode()

	// Return the full URL as a string
	return parsedURL.String(), nil
}

func generatePlayerSession(op models.Operator, token, username, currency string, balance int64, rc *redisdb.RedisClient) (models.Session, error) {
	session := models.Session{
		ID:         models.GenerateUUID(),
		Token:      token,
		PlayerName: username,
		Balance:    balance,
		Currency:   currency,
		OperatorIdentifier: models.OperatorIdentifier{
			OperatorName:     op.OperatorName,
			OperatorGameName: op.OperatorGameName,
			GameName:         op.GameName,
		},
		OperatorBaseUrl: op.OperatorWalletBaseUrl,
		CreatedAt:       time.Now(),
	}
	err := rc.AddSession(&session)
	return session, err
}
