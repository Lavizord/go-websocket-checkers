package models

type GameLaunchRequest struct {
	Currency     string `json:"currency"`
	OperatorName string `json:"operator_name"`
	GameID       string `json:"gameid"`
	Language     string `json:"language"`
	Token        string `json:"token"`
}
