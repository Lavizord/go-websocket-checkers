package models

type Operator struct {
	ID                    int     `json:"id"`
	OperatorName          string  `json:"operator_name"`
	OperatorGameName      string  `json:"operator_game_name"`
	GameName              string  `json:"game_name"`
	Active                bool    `json:"active"`
	GameBaseUrl           string  `json:"game_base_url"`
	OperatorWalletBaseUrl string  `json:"operator_wallet_base_url"`
	WinFactor             float64 `json:"win_factor"`
}

type WalletResponse struct {
	Status string `json:"status"`
	Data   struct {
		Username string  `json:"username"`
		Balance  float64 `json:"balance"`
		Currency string  `json:"currency"`
	} `json:"data"`
}

type SokkerDuelGamelaunchResponse struct {
	Token string `json:"token"`
	Url   string `json:"url"`
}

type SokkerDuelBet struct {
	OperatorGameName string `json:"game_id"`
	Currency         string `json:"currency"`
	Amount           int64  `json:"amount"`
	TransactionID    string `json:"transaction_id"`
	RoundID          string `json:"round_id"`
}

type SokkerDuelWin struct {
	OperatorGameName string `json:"game_id"`
	Currency         string `json:"currency"`
	Amount           int64  `json:"amount"`
	TransactionID    string `json:"transaction_id"`
	ExtractID        int64  `json:"extractSokkerDuelId"`
	RoundID          string `json:"round_id"`
}

type SokkerDuelBetResponse struct {
	Status string `json:"status"`
	Data   struct {
		GameID        string `json:"game_id"`
		Currency      string `json:"currency"`
		Amount        int64  `json:"amount"`
		Balance       string `json:"balance"`
		TransactionID string `json:"transaction_id"`
		ExtractID     int64  `json:"extractSokkerDuelId"`
	} `json:"data"`
}
type SokkerDuelWinResponse struct {
	Status string `json:"status"`
	Data   struct {
		GameID        string `json:"game_id"`
		Currency      string `json:"currency"`
		Amount        int64  `json:"amount"`
		Balance       string `json:"balance"`
		TransactionID string `json:"transaction_id"`
		ExtractID     int64  `json:"extractSokkerDuelId"`
	} `json:"data"`
}

type SokkerDuelErrorResponse struct {
	Status string `json:"status"`
	Auth   bool   `json:"auth"`
	Resp   string `json:"resp"`
}
