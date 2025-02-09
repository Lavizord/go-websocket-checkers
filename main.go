package main

import (
	"checkers-server/broadcast"
	"checkers-server/core"
	"checkers-server/handlers"
	"checkers-server/redisdb"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// ! Not working right.
// clearConsole clears the console screen.
func clearConsole() {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows": 
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		fmt.Println("\n\n\n\n\n\n\n\n\n\n")
	}
}

func monitorGameRooms() {
	var checkersDb = redisdb.NewRedisClient("localhost:6379", "", 0)
	for {
		time.Sleep(5 * time.Second)
		clearConsole()
		
		//onlinePlayers := len(core.ConnectedPlayers)
		onlinePlayers := checkersDb.CountPlayers()
		queuePlayers, _ := checkersDb.GetTotalQueuedPlayers()
		activeRooms := len(core.Rooms)
		
		// Count stuff to print it grouped later.
		selectedBidCount := make(map[float64]int)
		for _, player := range core.WaitingQueue {
			selectedBidCount[player.SelectedBid]++
		}
		bidAmountCount := make(map[float64]int)
		for _, room := range core.Rooms {
			bidAmountCount[room.BidAmount]++
		}

		fmt.Printf("Online Players: [%d] | In Queue: [%d] | Active Rooms: [%d]\n",
			onlinePlayers, queuePlayers, activeRooms)
		// Print counted stuff grouped.
		for selectedBid, count := range selectedBidCount {
			fmt.Printf("[%d] Queued Players of SelectedBid [%.2f]\n", count, selectedBid)
		}
		for bidAmount, count := range bidAmountCount {
			fmt.Printf("[%d] - Rooms of BidAmount [%.2f]\n", count, bidAmount)
		}
		fmt.Println("----------------------------------------")
	}
}


func main() {
	
	go monitorGameRooms()
	go broadcast.PlayersInQueue()
	http.HandleFunc("/ws", handlers.HandleConnection)
	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
