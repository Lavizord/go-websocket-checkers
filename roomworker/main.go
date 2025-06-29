package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Lavizord/checkers-server/config"
	"github.com/Lavizord/checkers-server/interfaces"
	"github.com/Lavizord/checkers-server/messages"
	"github.com/Lavizord/checkers-server/models"
	"github.com/Lavizord/checkers-server/postgrescli"
	"github.com/Lavizord/checkers-server/redisdb"
)

var pid int
var redisClient *redisdb.RedisClient
var postgresClient *postgrescli.PostgresCli
var name = "roomworker"

func init() {
	pid = os.Getpid()
	config.LoadConfig()
	redisConData := config.Cfg.Redis
	client, err := redisdb.NewRedisClient(redisConData.Addr, redisConData.User, redisConData.Password, redisConData.Tls)
	if err != nil {
		log.Fatalf("[Redis] Error initializing Redis client: %v\n", err)
	}
	redisClient = client

	sqlcliente, err := postgrescli.NewPostgresCli(
		config.Cfg.Postgres.User,
		config.Cfg.Postgres.Password,
		config.Cfg.Postgres.DBName,
		config.Cfg.Postgres.Host,
		config.Cfg.Postgres.Port,
		config.Cfg.Postgres.Ssl,
	)
	if err != nil {
		log.Fatalf("[%s-PostgreSQL] Error initializing POSTGRES client: %v\n", name, err)
	}
	postgresClient = sqlcliente
}

func main() {
	log.Printf("[RoomWorker-%d] - Waiting for room messages...\n", pid)

	defer func() {
		if redisClient != nil {
			redisClient.CloseRedisClient()
		}
	}()

	go processReadyQueue()
	go processQueue()

	select {}
}

func spawnWorkers(n int, workerFunc func()) {
	for i := 0; i < n; i++ {
		go workerFunc()
	}
}

func processQueue() {
	// Launch a goroutine for each bet queue
	for _, bet := range models.DamasValidBetAmounts {
		go processQueueForBet(bet)
	}
	// Block forever or wait on a channel (to prevent the main goroutine from exiting)
	select {}
}

func processQueueForBet(bet float64) {
	queueName := fmt.Sprintf("queue:%f", bet)
	for {
		// Block indefinitely for player1 (this goroutine is dedicated to this queue)
		player1, err := redisClient.BLPop(queueName, 0)
		if err != nil {
			log.Printf("[RoomWorker-%d] - Error retrieving player 1 from %s: %v\n", pid, queueName, err)
			continue
		}
		var p1disc, p2disc bool

		// log.Printf("[RoomWorker-%d] - Retrieved player 1 from %s: %v\n", pid, queueName, player1)
		player1Details, err := redisClient.GetPlayer(player1.ID)
		if err != nil {
			// We will make a check if the player is one of the disconnected players.
			player1Details = redisClient.GetDisconnectedInQueuePlayerData(player1.ID)
			if player1Details == nil {
				log.Printf("[RoomWorker-%v] - Error retrieving player 1 details, player removed from queue: %v\n", pid, err)
				redisClient.DecrementQueueCount(bet)
				continue
			}
			p1disc = true
			log.Printf("[RoomWorker-%v] -Player 1 details retrieved from offline queue list.", pid)
		}

		// We check to see if the player is eligible to be processed.
		if !player1Details.IsEligibleForQueue(bet) {
			log.Printf("[RoomWorker] - player1 with status %v not eligible to be processed by the queue, player removed from queue: %v\n", player1Details.Status, queueName)
			redisClient.DecrementQueueCount(bet)
			continue
		}

		// Try fetching the second player with a timeout
		player2, err := redisClient.BLPop(queueName, config.Cfg.Services["roomworker"].Timer)
		if err != nil {
			log.Printf("[RoomWorker-%d] - No second player found in %s, re-queueing player 1.\n", pid, queueName)
			// Since we failed to get the player2, we will requeue the player1.
			time.Sleep(time.Second * 1)
			redisClient.RPush(queueName, player1)
			continue
		}

		// log.Printf("[RoomWorker-%d] - Retrieved player 2 from %s: %v\n", pid, queueName, player2)
		player2Details, err := redisClient.GetPlayer(player2.ID)
		if err != nil {
			// We will make a check if the player is one of the disconnected players.
			player2Details = redisClient.GetDisconnectedInQueuePlayerData(player2.ID)
			if player2Details == nil {
				log.Printf("[RoomWorker-%v] - Error retrieving player 2 details, player removed from queue: %v\n", pid, err)
				redisClient.DecrementQueueCount(bet)
				continue
			}
			p2disc = true
			log.Printf("[RoomWorker-%v] -Player 2 details retrieved from offline queue list.", pid)
		}

		if player1Details.ID == player2Details.ID {
			log.Printf("[RoomWorker-%d] - player1Details.ID == player2Details.ID, player2 removed from queue: %v\n", pid, queueName)
			redisClient.RPush(queueName, player1)
			redisClient.DecrementQueueCount(bet)
			continue
		}

		// Before we handle the paired, we will do a final check to make sure the players2 is still online / valid.
		if !player2Details.IsEligibleForQueue(bet) {
			// If it is not valid, we will add player 1 back to the queue.
			log.Printf("[RoomWorker-%d] - player2 not eligible to be processed by the queue, player removed from queue: %v\n", pid, queueName)
			redisClient.RPush(queueName, player1)
			redisClient.DecrementQueueCount(bet)
			continue
		}

		// Process both players
		// log.Printf("[RoomWorker-%d] - Pairing players: %s and %s from %s\n", pid, player1, player2, queueName)
		handleQueuePaired(player1, player2, p1disc, p2disc)
	}
}

func processReadyQueue() {
	for {
		playerData, err := redisClient.BLPop("ready_queue", 0) // Block
		if err != nil {
			log.Printf("[RoomWorker-%d] - Error retrieving player:%v\n", pid, err)
			continue
		}
		log.Printf("[RoomWorker-%d] - processing ready room!: %+v\n", pid, playerData)
		// Aqui ou damos handle do ready queue ou handle do unreadyqueue
		if playerData.Status == models.StatusInRoomReady {
			redisClient.PublishToRoomPubSub(playerData.RoomID, "player_ready:"+playerData.ID)
			//handleReadyRoom(playerData)
			continue
		}
		if playerData.Status == models.StatusInRoom {
			redisClient.PublishToRoomPubSub(playerData.RoomID, "player_unready:"+playerData.ID)
			//handleUnReadyRoom(playerData)
			continue
		}
		log.Printf("Player is neither InRoomReady neither InRoom?!")
	}
}

func handleQueuePaired(player1, player2 *models.Player, p1disc, p2disc bool) {
	room := &models.Room{
		ID:                 models.GenerateUUID(),
		Player1:            player1,
		Player2:            player2,
		StartDate:          time.Now(),
		Currency:           player1.Currency,
		BetValue:           player1.SelectedBet,
		OperatorIdentifier: player1.OperatorIdentifier,
	}

	player1.RoomID = room.ID
	player2.RoomID = room.ID
	player1.Status = models.StatusInRoom
	player2.Status = models.StatusInRoom

	if p1disc == true {
		redisClient.SaveDisconnectInQueuePlayerData(player1)
	} else {
		redisClient.UpdatePlayer(player1)
	}
	if p2disc == true {
		redisClient.SaveDisconnectInQueuePlayerData(player2)
	} else {
		redisClient.UpdatePlayer(player2)
	}

	cleanup := true
	defer func() {
		if cleanup {
			player1.RoomID = ""
			player2.RoomID = ""
			player1.Status = models.StatusOnline
			player2.Status = models.StatusOnline
			if p1disc == true {
				redisClient.SaveDisconnectInQueuePlayerData(player1)
			} else {
				redisClient.UpdatePlayer(player1)
			}
			if p2disc == true {
				redisClient.SaveDisconnectInQueuePlayerData(player2)
			} else {
				redisClient.UpdatePlayer(player2)
			}
			redisClient.RemoveRoom(redisdb.GenerateRoomRedisKeyById(room.ID))
			redisClient.DecrementQueueCount(player1.SelectedBet)
			redisClient.DecrementQueueCount(player2.SelectedBet)
			msg, _ := messages.GenerateGenericMessage("error", "failed to handle queue paired.")
			redisClient.PublishToPlayer(*player1, string(msg))
			redisClient.PublishToPlayer(*player2, string(msg))
		}
	}()

	colorp1 := rand.Intn(2)
	colorp2 := 1
	if colorp1 == 1 {
		room.CurrentPlayerID = player1.ID
		colorp2 = 0
	} else {
		room.CurrentPlayerID = player2.ID
	}

	err := redisClient.AddRoom(room)
	if err != nil {
		log.Printf("Failed to add room to Redis: %v\n", err)
		return
	}

	message1, err := messages.GeneratePairedMessage(player1, player2, room.ID, colorp1, interfaces.CalculateWinAmount(int64(room.BetValue*100), room.OperatorIdentifier.WinFactor), 30)
	if err != nil {
		log.Printf("Error generating message for player1: %v\n", err)
		return
	}

	message2, err := messages.GeneratePairedMessage(player2, player1, room.ID, colorp2, interfaces.CalculateWinAmount(int64(room.BetValue*100), room.OperatorIdentifier.WinFactor), 30)
	if err != nil {
		log.Printf("Error generating message for player2: %v\n", err)
		return
	}

	err = redisClient.PublishToPlayer(*player1, string(message1))
	if err != nil {
		log.Printf("Failed to publish message to player1: %v\n", err)
		return
	}
	err = redisClient.PublishToPlayer(*player2, string(message2))
	if err != nil {
		log.Printf("Failed to publish message to player2: %v\n", err)
		return
	}

	// This will start a pubsub tied to a timer.
	listenRoom(context.Background(), redisClient, room)

	cleanup = false
	redisClient.DecrementQueueCount(player1.SelectedBet)
	redisClient.DecrementQueueCount(player2.SelectedBet)
}

func handleReadyRoomNew(player *models.Player, proom *models.Room) {
	//log.Printf("[RoomWorker-%d] - Handling player (READY QUEUE): %s (Session: %s, Currency: %s)\n",
	//	pid, player.ID, player.SessionID, player.Currency)

	var errorWithOpponent bool
	errorWithOpponent = false
	defer func() {
		if errorWithOpponent {
			addPlayerToQueue(player, true, true)
		}
	}()
	player2, err := proom.GetOpponentPlayer(player.ID)
	// We will always notify the opponent the we are ready.
	msg, err := messages.GenerateOpponentReadyMessage(true)
	if err != nil {
		log.Printf("[RoomWorker-%d] - Error handleReadyRoom getting GenerateOpponentReadyMessage(true) for opponent:%s\n", pid, err)
		return
	}
	redisClient.PublishPlayerEvent(player2, string(msg))
	// now we tell our player that is ready if the opponent is ready or not.
	if player2.Status != models.StatusInRoomReady {
		//log.Printf("[RoomWorker-%d] - handleReadyRoom Opponent aint ready yet!:%s\n", pid, err)
		msg, err := messages.GenerateOpponentReadyMessage(false)
		if err != nil {
			log.Printf("[RoomWorker-%d] - Error handleReadyRoom getting GenerateOpponentReadyMessage(false) for player:%s\n", pid, err)
		}
		redisClient.PublishPlayerEvent(player, string(msg))
		return
	}
	// Now! If both players are ready...!!
	// Before we start the game, we will need to post to the wallet api of the bet, we will use our api interface for that.
	module, exists := interfaces.OperatorModules[proom.OperatorIdentifier.OperatorName]
	if !exists {
		log.Printf("[RoomWorker-%d] - Error handleReadyRoom getting getting interfaces.OperatorModules[%v]:%s\n", pid, proom.OperatorIdentifier.OperatorName, err)
		return
	}

	session1, err := redisClient.GetSessionByID(player.SessionID)
	if err != nil {
		log.Printf("[RoomWorker-%d] - Error handleReadyRoom fetching player1 sessionID:%s\n", pid, err)
		return
	}
	session2, err := redisClient.GetSessionByID(player2.SessionID)
	if err != nil {
		log.Printf("[RoomWorker-%d] - Error handleReadyRoom fetching player2 sessionID:%s\n", pid, err)
		return
	}

	newBalance1, err := module.HandlePostBet(postgresClient, redisClient, *session1, int64(proom.BetValue*100), proom.ID)
	if err != nil {
		log.Printf("[RoomWorker-%d] - Error HandlePostBet failed to bet:%s for sessionid:[%s]\n", pid, err, session1.ID)
		player.SetStatusOnline()
		redisClient.UpdatePlayer(player)
		msg, _ := messages.GenerateGenericMessage("error", err.Error())
		redisClient.PublishPlayerEvent(player, string(msg))
		redisClient.RemoveRoom(redisdb.GenerateRoomRedisKeyById(proom.ID))

		msg, _ = messages.NewMessage("opponent_left_room", true)
		redisClient.PublishPlayerEvent(player2, string(msg))

		// since the first player failed the api check, we will queue up the second plyer.
		addPlayerToQueue(player2, true, true)
		// TODO: CREDITAR VALOR A JOGADOR.
		return
	}
	newBalance2, err := module.HandlePostBet(postgresClient, redisClient, *session2, int64(proom.BetValue*100), proom.ID)
	if err != nil {
		log.Printf("[RoomWorker-%d] - Error HandlePostBet failed to bet:%s for sessionid:[%s]\n", pid, err, session1.ID)
		player2.SetStatusOnline()
		redisClient.UpdatePlayer(player2)
		msg, _ := messages.GenerateGenericMessage("error", err.Error())
		redisClient.PublishPlayerEvent(player2, string(msg))
		redisClient.RemoveRoom(redisdb.GenerateRoomRedisKeyById(proom.ID))

		// since the second player failed the api check, we will queue up the first player.
		addPlayerToQueue(player2, true, true)
		// TODO: CREDITAR VALOR A JOGADOR.
		return
	}
	// Now that everything is OK, we will start up the game
	msgP1, _ := messages.NewMessage("balance_update", float64(newBalance1)/100)
	msgP2, _ := messages.NewMessage("balance_update", float64(newBalance2)/100)

	// then notify player and store it in redis.
	redisClient.UpdatePlayer(player)
	redisClient.UpdatePlayer(player2)

	redisClient.PublishPlayerEvent(player, string(msgP1))
	redisClient.PublishPlayerEvent(player2, string(msgP2))

	redisClient.PublishToRoomPubSub(proom.ID, "game_start")

	// Then we start a match
	roomdata, _ := json.Marshal(proom)
	err = redisClient.RPushGeneric("create_game", roomdata)
	if err != nil {
		log.Printf("[RoomWorker-%d] - Error handleReadyRoom Creating Game RPushGeneric:%s\n", pid, err)
	}

}

func handleUnReadyRoomNew(player *models.Player, proom *models.Room) {
	//log.Printf("[RoomWorker-%d] - Handling player (UN-READY QUEUE): %s (Session: %s, Currency: %s)\n",
	//	pid, player.ID, player.SessionID, player.Currency)

	var errorWithOpponent bool
	errorWithOpponent = false
	defer func() {
		if errorWithOpponent {
			addPlayerToQueue(player, true, true)
		}
	}()
	player2, err := proom.GetOpponentPlayer(player.ID)
	if err != nil {
		errorWithOpponent = true
		log.Printf("[RoomWorker-%d] - Error handleUnReadyRoom getting opponent player:%s\n", pid, err)
		return
	}
	// We will always notify the opponent the we are no longer ready.
	msg, err := messages.GenerateOpponentReadyMessage(false)
	if err != nil {
		log.Printf("[RoomWorker-%d] - Error handleUnReadyRoom getting GenerateOpponentReadyMessage(false) for opponent:%s\n", pid, err)
		return
	}
	redisClient.PublishPlayerEvent(player2, string(msg))

	// now we tell our player that is ready if the opponent is ready or not.
	if player2.Status != models.StatusInRoomReady {
		//log.Printf("[RoomWorker-%d] - handleUnReadyRoom Opponent aint ready yet!:%s\n", pid, err)
		msg, err := messages.GenerateOpponentReadyMessage(false)
		if err != nil {
			log.Printf("[RoomWorker-%d] - Error handleUnReadyRoom getting GenerateOpponentReadyMessage(false) for player:%s\n", pid, err)
		}
		redisClient.PublishPlayerEvent(player, string(msg))
		return
	}
}

func handleEndRoom(rdb *redisdb.RedisClient, room *models.Room) {
	p1, _ := rdb.GetPlayer(room.Player1.ID)
	p2, _ := rdb.GetPlayer(room.Player2.ID)

	key := fmt.Sprintf("%t_%t", p1 != nil, p2 != nil)
	switch key {
	case "false_false":
		// both nil, no players, we will just remove the room, and remove both players from possible offline lists.
		//log.Printf("[RoomWorker-%d] - handleEndRoom - false_false: %v\n", pid)
		p1 = rdb.GetDisconnectedInQueuePlayerData(room.Player1.ID)
		if p1 != nil {
			if p1.Status == models.StatusInRoomReady {
				addPlayerToQueue(p1, true, true)
			} else {
				rdb.DeleteDisconnectedInQueuePlayerData(p1.ID)
			}
		}
		p2 = rdb.GetDisconnectedInQueuePlayerData(room.Player2.ID)
		if p2 != nil {
			if p2.Status == models.StatusInRoomReady {
				addPlayerToQueue(p2, true, true)
			} else {
				rdb.DeleteDisconnectedInQueuePlayerData(p2.ID)
			}
		}
		err := rdb.RemoveRoom(redisdb.GenerateRoomRedisKeyById(room.ID))
		if err != nil {
			log.Printf("[RoomWorker-%d] - handleEndRoom - Error removing room: %v\n", pid, err)
			return
		}

	case "true_false":
		// only p1, we will handle the removal of the P2, and requeue the p1.
		// log.Printf("[RoomWorker-%d] - handleEndRoom - false_true: %v\n", pid)
		// Since this guy is offline, we will check if its state was in room ready.
		msg, _ := messages.NewMessage("room_failed_ready_check", true)
		p2 = rdb.GetDisconnectedInQueuePlayerData(room.Player2.ID)
		if p2 != nil {
			if p2.Status == models.StatusInRoomReady {
				addPlayerToQueue(p2, true, true)
			} else {
				rdb.DeleteDisconnectedInQueuePlayerData(p2.ID)
			}
		}
		if p1.Status == models.StatusInRoomReady {
			addPlayerToQueue(p1, true, true)
		} else {
			p1.SetStatusOnline()
			rdb.PublishToPlayerID(p1.ID, string(msg))
			rdb.UpdatePlayer(p1)
		}
		err := rdb.RemoveRoom(redisdb.GenerateRoomRedisKeyById(room.ID))
		if err != nil {
			log.Printf("[RoomWorker-%d] - handleEndRoom - Error removing room: %v\n", pid, err)
			return
		}
		return

	case "false_true":
		// only p1, we will handle the removal of the P2, and requeue the p1.
		//log.Printf("[RoomWorker-%d] - handleEndRoom - false_true:\n", pid)
		msg, _ := messages.NewMessage("room_failed_ready_check", true)
		// Since this guy is offline, we will check if its state was in room ready.
		p1 = rdb.GetDisconnectedInQueuePlayerData(room.Player1.ID)
		if p1 != nil {
			if p1.Status == models.StatusInRoomReady {
				addPlayerToQueue(p1, true, true)
			} else {
				rdb.DeleteDisconnectedInQueuePlayerData(p1.ID)
			}
		}
		if p2.Status == models.StatusInRoomReady {
			addPlayerToQueue(p2, true, true)
		} else {
			p2.SetStatusOnline()
			rdb.PublishToPlayerID(p2.ID, string(msg))
			rdb.UpdatePlayer(p2)
		}
		err := rdb.RemoveRoom(redisdb.GenerateRoomRedisKeyById(room.ID))
		if err != nil {
			log.Printf("[RoomWorker-%d] - handleEndRoom - Error removing room:", pid)
			return
		}
		return

	case "true_true":
		// both, this means that the timer ran out...
		//log.Printf("[RoomWorker-%d] - handleEndRoom - true_true:\n", pid)
		msg, _ := messages.NewMessage("room_failed_ready_check", true)
		if p1.Status == models.StatusInRoomReady {
			addPlayerToQueue(p1, true, true)
		} else {
			p1.SetStatusOnline()
			rdb.PublishToPlayerID(p1.ID, string(msg))
			rdb.UpdatePlayer(p1)
		}
		if p2.Status == models.StatusInRoomReady {
			addPlayerToQueue(p2, true, true)
		} else {
			p2.SetStatusOnline()
			rdb.PublishToPlayerID(p2.ID, string(msg))
			rdb.UpdatePlayer(p2)
		}
		err := rdb.RemoveRoom(redisdb.GenerateRoomRedisKeyById(room.ID))
		if err != nil {
			log.Printf("[RoomWorker-%d] - handleEndRoom - Error removing room: %v\n", pid, err)
			return
		}
		return
	}
}

func addPlayerToQueue(player *models.Player, incrementQueueCount, notify bool) {
	// Reset both player data.
	player.RoomID = ""
	player.GameID = ""
	player.Status = models.StatusInQueue
	err := redisClient.UpdatePlayer(player)
	if err != nil {
		redisClient.SaveDisconnectInQueuePlayerData(player)
	}

	// Pushing the player to the "queue" Redis list
	queueName := fmt.Sprintf("queue:%f", player.SelectedBet)
	err2 := redisClient.RPush(queueName, player)
	if err2 != nil {
		log.Printf("[RoomWorker-%d] - Error adding player to queue:%v\n", pid, err2)
		return
	}
	if incrementQueueCount {
		redisClient.IncrementQueueCount(player.SelectedBet)
	}
	if notify {
		queueMsg, _ := messages.GenerateQueueConfirmationMessage(true)
		redisClient.PublishPlayerEvent(player, string(queueMsg))
	}
}

// Method to listen to room messages.
//
// Initially created to manage the room timer, but was  expanded to be used by a few
// other messages.
func listenRoom(ctx context.Context, rdb *redisdb.RedisClient, room *models.Room) {
	pubsub := rdb.Client.Subscribe(ctx, "roompubsub:"+room.ID)
	ch := pubsub.Channel()

	go func() {
		defer pubsub.Close()

		countdown := 30
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Main loop of the listen, will check the channel and the tiker channel for messages.
		for {
			select {
			case msg := <-ch:
				//log.Println("Received:", msg.Payload)
				// Main switch to route the channel specific messages.
				switch {
				case msg.Payload == "room_end":
					//log.Println("Timer canceled by message room_end")
					handleEndRoom(rdb, room)
					return

				case msg.Payload == "game_start":
					//log.Println("Timer canceled by message game_start")
					return

				case strings.HasPrefix(msg.Payload, "leave_room:"):
					playerID := strings.TrimPrefix(msg.Payload, "leave_room:")
					// First we will handle the player who left.
					playerWhoLeft, _ := room.GetPlayer(playerID)
					playerWhoLeft.SetStatusOnline()
					rdb.UpdatePlayer(playerWhoLeft)
					// now we handle the player that was in the room, he can be online, offline, ready or unready.
					opponentId, _ := room.GetOpponentPlayerID(playerID)
					// We check if its offline.
					opponentPlayer := rdb.GetDisconnectedInQueuePlayerData(opponentId)
					if opponentPlayer == nil {
						// This means the player should be online:
						msg, _ := messages.NewMessage("opponent_left_room", true)
						opponentPlayer, _ = room.GetPlayer(opponentId)
						redisClient.PublishToPlayer(*opponentPlayer, string(msg))
						addPlayerToQueue(opponentPlayer, true, true)
					} else {
						addPlayerToQueue(opponentPlayer, true, true)
					}
					err := rdb.RemoveRoom(redisdb.GenerateRoomRedisKeyById(room.ID))
					if err != nil {
						log.Printf("[RoomWorker-%d] - processRoomEnding - Error removing room: %v\n", pid, err)
					}
					return

				case strings.HasPrefix(msg.Payload, "player_ready:"):
					playerID := strings.TrimPrefix(msg.Payload, "player_ready:")
					room.SetPlayerReady(playerID)
					player, _ := room.GetPlayer(playerID)
					handleReadyRoomNew(player, room)

				case strings.HasPrefix(msg.Payload, "player_unready:"):
					playerID := strings.TrimPrefix(msg.Payload, "player_unready:")
					room.SetPlayerUnReady(playerID)
					player, _ := room.GetPlayer(playerID)
					handleUnReadyRoomNew(player, room)

				case strings.HasPrefix(msg.Payload, "player_reconnect:"):
					playerID := strings.TrimPrefix(msg.Payload, "player_reconnect:")
					opponent, _ := room.GetOpponentPlayer(playerID)
					player, _ := room.GetOpponentPlayer(opponent.ID)
					outBoundMsg, _ := messages.GeneratePairedMessage(player, opponent, room.ID, room.DeducePlayerColor(playerID), interfaces.CalculateWinAmount(int64(room.BetValue*100), room.OperatorIdentifier.WinFactor), countdown)
					rdb.PublishToPlayerID(playerID, string(outBoundMsg))
				}

			case <-ticker.C:
				countdown--
				if countdown == 27 || countdown == 10 || countdown == 3 {
					timerMsg, _ := messages.NewMessage("room_timer", strconv.Itoa(countdown))
					rdb.PublishToPlayerID(room.Player1.ID, string(timerMsg))
					rdb.PublishToPlayerID(room.Player2.ID, string(timerMsg))
				}
				//log.Println("Countdown:", countdown)
				if countdown <= 0 {
					//log.Println("Room timed out")
					handleEndRoom(rdb, room)
					return
				}

			case <-ctx.Done():
				return
			}
		}

	}()
}
