package main

import (
	"checkers-server/config"
	"checkers-server/messages"
	"checkers-server/models"
	"checkers-server/redisdb"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

var pid int
var redisClient *redisdb.RedisClient
var name = "GameWorker"

func init() {
	pid = os.Getpid()
	config.LoadConfig()
	redisAddr := config.Cfg.Redis.Addr
	client, err := redisdb.NewRedisClient(redisAddr)
	if err != nil {
		log.Fatalf("[%s-Redis] Error initializing Redis client: %v\n", name, err)
	}
	redisClient = client
}

func main() {
	fmt.Printf("[%s-%d] - Waiting for Game messages...\n", name, pid)
	go processGameCreation()
	go processGameMoves()
	go processGameOverQueue()
	select {}
}

func processGameCreation() {
	for {
		roomData, err := redisClient.BLPopGeneric("create_game", 0) // Block
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Creation) - Error retrieving room data: %v\n", name, pid, err)
			continue
		}

		if len(roomData) < 2 {
			fmt.Printf("[%s-%d] - (Process Game Creation) - Unexpected BLPop result: %+v\n", name, pid, roomData)
			continue
		}
		fmt.Printf("[%s-%d] - (Process Game Creation) - create game!: %+v\n", name, pid, roomData)

		var room models.Room
		err = json.Unmarshal([]byte(roomData[1]), &room) // Extract second element
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Creation) - JSON Unmarshal Error: %v\n", name, pid, err)
			continue
		}

		player1, err := redisClient.GetPlayer(room.Player1.ID)
		player2, err := redisClient.GetPlayer(room.Player2.ID)
		game := room.NewGame()
		// we need to update our players with a game ID.
		player1.GameID = game.ID
		player2.GameID = game.ID
		err = redisClient.AddPlayer(player1)
		err = redisClient.AddPlayer(player2)
		err = redisClient.AddGame(game)
		msg, err := messages.GenerateGameStartMessage(*game)
		fmt.Printf("[%s-%d] - (Process Game Creation) - Message to publish: %v\n", name, pid, msg)
		redisClient.PublishToGamePlayer(game.Players[0], string(msg))
		redisClient.PublishToGamePlayer(game.Players[1], string(msg))
		go startTurnTimer(game) // Start turn timer
	}
}

func processGameMoves() {
	for {
		moveData, err := redisClient.BLPopGeneric("move_piece", 0) // Block
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Moves) - Error retrieving move data: %v\n", name, pid, err)
			continue
		}
		fmt.Printf("[%s-%d] - (Process Game Moves) - processing move DATA!: %+v\n", name, pid, moveData)

		// We start by getting our move data, player, game and opponentPlayer.
		var move models.Move
		err = json.Unmarshal([]byte(moveData[1]), &move) // Extract second element
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Moves) - JSON Unmarshal Error: %v\n", name, pid, err)
			continue
		}
		player, err := redisClient.GetPlayer(move.PlayerID)
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Moves) - Failed to get player!: %v\n", name, pid, err)
			continue
		}
		game, err := redisClient.GetGame(player.GameID)
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Moves) - Failed to get game!: %v\n", name, pid, err)
			continue
		}
		opponent, err := game.GetOpponentGamePlayer(move.PlayerID)

		// We move our piece.
		game.MovePiece(move)
		game.UpdatePlayerPieces()
		// We send the message to the opponent player.
		msg, err := messages.GenerateMoveMessage(move)
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Moves) - Failed to generate message: %v\n", name, pid, string(msg))
		}
		fmt.Printf("[%s-%d] - (Process Game Moves) - Message to publish: %v\n", name, pid, string(msg))
		redisClient.PublishToGamePlayer(*opponent, string(msg))

		// We check for game Over
		if game.CheckGameOver() {
			redisClient.Client.RPush(context.Background(), "game_over_queue", game.ID)
			redisClient.AddGame(game) // we update our game
			continue
		}
		// we check for a turn change.
		if move.IsCapture && !game.Board.CanPieceCapture(move.To) {
			handleTurnChange(game)
		}
		redisClient.AddGame(game) // we update our game at the end.
	}
}

func processGameOverQueue() {
	for {
		// Block until there is a game over message
		gameOverData, err := redisClient.Client.BLPop(context.Background(), 0, "game_over_queue").Result()
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Over) - Error retrieving game over data: %v\n", name, pid, err)
			continue
		}

		if len(gameOverData) < 2 {
			fmt.Printf("[%s-%d] - (Process Game Over) - Unexpected BLPop result", name, pid)
			continue
		}

		gameOverMessage := gameOverData[1]                                                                   // Get the message
		fmt.Printf("[%s-%d] - (Process Game Over) - Processing game over: %s\n", name, pid, gameOverMessage) // this should be a game ID.

		// Now we handle game logic.
		game, err := redisClient.GetGame(gameOverMessage)
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Moves) - Failed to get game!: %v\n", name, pid, err)
			continue
		}
		game.FinishGame()         // This should handle our data side of things.
		redisClient.AddGame(game) // we update our game at the end.
		msg, err := messages.GenerateGameOverMessage("winner", *game)
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Over) - Failed to get game!: %v\n", name, pid, err)
			continue
		}
		redisClient.PublishToGamePlayer(*&game.Players[0], string(msg))
		redisClient.PublishToGamePlayer(*&game.Players[1], string(msg))

		// Now we will process palyer balance for the winner.
		winnerPlayer, err := redisClient.GetPlayer(game.Winner)
		if err != nil {
			fmt.Printf("[%s-%d] - (Process Game Over) - Failed to get winner player!: %v\n", name, pid, err)
			continue
		}
		winnerPlayer.UpdateBalance(game.BetValue)
		msgP1, err := messages.NewMessage("balance_update", winnerPlayer.CurrencyAmount)
		redisClient.PublishPlayerEvent(winnerPlayer, string(msgP1))
		redisClient.AddPlayer(winnerPlayer)
	}
}

func handleTurnChange(game *models.Game) {
	stopChannel := fmt.Sprintf("game:%s:stop_timer", game.ID)
	redisClient.Client.Publish(context.Background(), stopChannel, "STOP") // Stop the old timer
	game.NextPlayer()
	msg , err := messages.NewMessage("turn_switch", game.CurrentPlayerID)
	if err != nil {
		fmt.Printf("[%s-%d] - (Handle Turn Change) - Failed to generate for turn change: %v\n", name, pid, msg)
	}
	BroadCastToGamePlayers(msg, *game)
	go startTurnTimer(game) // Start a fresh timer
}

func startTurnTimer(game *models.Game) {
	ctx := context.Background()
	stopChannel := fmt.Sprintf("game:%s:stop_timer", game.ID)
	pubsub := redisClient.Client.Subscribe(ctx, stopChannel)
	defer pubsub.Close()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// fetch the timer from the config.
	timer := config.Cfg.Services["gameworker"].Timer
	for timer > 0 {
		select {
		case <-ticker.C:
			msg, _ := messages.GenerateGameTimerMessage(*game, timer)
			redisClient.PublishToGamePlayer(game.Players[0], string(msg))
			redisClient.PublishToGamePlayer(game.Players[1], string(msg))
			timer--

		case msg := <-pubsub.Channel():
			if msg.Payload == "STOP" {
				fmt.Printf("Timer stopped for game %s\n", game.ID)
				return // Exit the function, stopping the timer
			}
		}
	}
	handleTurnChange(game)
	redisClient.AddGame(game)
	fmt.Printf("Turn timer expired for game %s\n", game.ID)
}


func BroadCastToGamePlayers(msg []byte, game models.Game) {
	redisClient.PublishToGamePlayer(game.Players[0], string(msg))
	redisClient.PublishToGamePlayer(game.Players[1], string(msg))
}
