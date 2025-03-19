package models

type Operator struct {
	ID                    int    `json:"id"`
	OperatorName          string `json:"operator_name"`
	OperatorGameName      string `json:"operator_game_name"`
	GameName              string `json:"game_name"`
	Active                bool   `json:"active"`
	GameBaseUrl           string `json:"game_base_url"`
	OperatorWalletBaseUrl string `json:"operator_wallet_base_url"`
}

type WalletResponse struct {
	Status string `json:"status"`
	Data   struct {
		Username string `json:"username"`
		Balance  int64  `json:"balance"`
		Currency string `json:"currency"`
	} `json:"data"`
}

type SokkerDuelGamelaunchResponse struct {
	Token string `json:"token"`
	Url   string `json:"url"`
}

type SokkerDuelBet struct {
	OperatorGameName string `json:"game_id"`
	Currency         string `json:"currency"`
	Amount           int    `json:"amount"`
	TransactionID    string `json:"transaction_id"`
}

type SokkerDuelWin struct {
	OperatorGameName string `json:"game_id"`
	Currency         string `json:"currency"`
	Amount           int    `json:"amount"`
	TransactionID    string `json:"transaction_id"`
	ExtractID        string `json:"extract_id"`
}

type SokkerDuelBetAndWinResponse struct {
	Status string `json:"status"`
	Data   struct {
		GameID        string `json:"game_id"`
		Currency      string `json:"currency"`
		Amount        int    `json:"amount"`
		Balance       int64  `json:"balance"`
		TransactionID string `json:"transaction_id"`
		ExtractID     string `json:"extract_id"`
	} `json:"data"`
}
