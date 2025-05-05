package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Lavizord/checkers-server/internal/messages"
	"github.com/Lavizord/checkers-server/internal/models"
	"github.com/Lavizord/checkers-server/internal/redisdb"
)

type QueueHandler struct {
	player      *models.Player
	redisClient *redisdb.RedisClient
	msg         *messages.Message[json.RawMessage]

	// Track changes for cleanup
	initialValidationsFailed bool
	statusUpdated            bool
	addedToQueue             bool
	queueCountIncr           bool
}

func (qh *QueueHandler) process() {
	defer qh.cleanup()

	if !qh.validateStatusTransition() {
		qh.initialValidationsFailed = true
		log.Print("QueueHandler process invalid status transition.")
		return
	}

	betValue, err := qh.parseBetValue()
	if err != nil {
		qh.initialValidationsFailed = true
		log.Print("QueueHandler failed to parse bet.")
		return
	}

	found := false
	for _, v := range models.DamasValidBetAmounts {
		if v == betValue {
			found = true
			break
		}
	}
	if !found {
		qh.initialValidationsFailed = true
		log.Print("Bet is not valid for the configured ValidBetAmounts")
		return
	}

	qh.updatePlayerState(betValue)
	qh.addToRedisQueue()
	qh.updateQueueCount()
	qh.sendConfirmation()
}

func (qh *QueueHandler) cleanup() {
	var sendFailedQueueConfirmation = false

	if qh.initialValidationsFailed {
		qh.player.UpdatePlayerStatus(models.StatusOnline)
		qh.redisClient.UpdatePlayer(qh.player)
		sendFailedQueueConfirmation = true
	}

	if qh.statusUpdated {
		qh.player.UpdatePlayerStatus(models.StatusOnline)
		qh.redisClient.UpdatePlayer(qh.player)
		sendFailedQueueConfirmation = true
	}

	if qh.addedToQueue {
		queueName := fmt.Sprintf("queue:%f", qh.player.SelectedBet)
		qh.redisClient.Client.LRem(context.Background(), queueName, 1, qh.player)
		sendFailedQueueConfirmation = true
	}

	if qh.queueCountIncr {
		qh.redisClient.DecrementQueueCount(qh.player.SelectedBet)
		sendFailedQueueConfirmation = true
	}

	if sendFailedQueueConfirmation {
		msg, _ := messages.GenerateQueueConfirmationMessage(false)
		qh.player.WriteChan <- msg
	}
}

func (qh *QueueHandler) validateStatusTransition() bool {
	if qh.player.UpdatePlayerStatus(models.StatusInQueue) != nil {
		msgBytes, _ := messages.GenerateGenericMessage("invalid", "Invalid status transition to 'queue'")
		qh.player.WriteChan <- msgBytes
		return false
	}
	qh.statusUpdated = true
	return true
}

func (qh *QueueHandler) parseBetValue() (float64, error) {
	var betValue float64
	err := json.Unmarshal(qh.msg.Value, &betValue)
	if err != nil {
		log.Printf("Error determining player bet value: %v\n", err)
		msgBytes, _ := messages.GenerateGenericMessage("error", "Error determining player bet value")
		qh.player.WriteChan <- msgBytes
		return 0, err
	}
	return betValue, nil
}

func (qh *QueueHandler) updatePlayerState(betValue float64) {
	qh.player.SelectedBet = betValue
	qh.player.Status = models.StatusInQueue
	qh.redisClient.UpdatePlayersInQueueSet(qh.player.ID, models.StatusInQueue)
	qh.redisClient.UpdatePlayer(qh.player)
}

func (qh *QueueHandler) addToRedisQueue() error {
	queueName := fmt.Sprintf("queue:%f", qh.player.SelectedBet)
	err := qh.redisClient.RPush(queueName, qh.player)
	if err != nil {
		log.Printf("Error pushing player to Redis queue: %v\n", err)
		msgBytes, _ := messages.GenerateGenericMessage("error", "error adding player to queue")
		qh.player.WriteChan <- msgBytes
		return err
	}
	qh.addedToQueue = true
	return nil
}

func (qh *QueueHandler) updateQueueCount() {
	exists, err := qh.redisClient.CheckQueueCountExists(qh.player.SelectedBet)
	if err == nil {
		if !exists {
			qh.redisClient.CreateQueueCount(qh.player.SelectedBet)
		} else {
			qh.redisClient.IncrementQueueCount(qh.player.SelectedBet)
		}
		qh.queueCountIncr = true
	}
}

func (qh *QueueHandler) sendConfirmation() {
	m, err := messages.GenerateQueueConfirmationMessage(true)
	if err != nil {
		fmt.Println("Error generating queue confirmation:", err)
		msgBytes, _ := messages.GenerateGenericMessage("error", "error generating confirmation")
		qh.player.WriteChan <- msgBytes
		return
	}
	qh.player.WriteChan <- m

	// Success - disable cleanup
	qh.statusUpdated = false
	qh.addedToQueue = false
	qh.queueCountIncr = false
}
