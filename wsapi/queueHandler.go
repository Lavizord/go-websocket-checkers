package wsapi

import (
	"checkers-server/messages"
	"checkers-server/models"
	"checkers-server/redisdb"
	"context"
	"encoding/json"
	"fmt"
	"log"
)

type QueueHandler struct {
	player      *models.Player
	redisClient *redisdb.RedisClient
	msg         *messages.Message[json.RawMessage]

	// Track changes for cleanup
	statusUpdated  bool
	addedToQueue   bool
	queueCountIncr bool
}

func (qh *QueueHandler) process() {
	defer qh.cleanup()

	if !qh.validateStatusTransition() {
		return
	}

	betValue, err := qh.parseBetValue()
	if err != nil {
		return
	}

	qh.updatePlayerState(betValue)
	qh.addToRedisQueue()
	qh.updateQueueCount()
	qh.sendConfirmation()
}

func (qh *QueueHandler) cleanup() {
	var sendFailedQueueConfirmation = false
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
		qh.player.WriteChan <- []byte("Invalid status transition to 'queue'")
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
		qh.player.WriteChan <- []byte("Error determining player bet value")
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
		qh.player.WriteChan <- []byte("Error adding player to queue")
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
		qh.player.WriteChan <- []byte("Error generating confirmation")
		return
	}
	qh.player.WriteChan <- m

	// Success - disable cleanup
	qh.statusUpdated = false
	qh.addedToQueue = false
	qh.queueCountIncr = false
}
