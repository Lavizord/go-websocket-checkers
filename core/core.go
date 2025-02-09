package core

/*
	Handle the core of the memory data of the server.
	If we need to lock / unlock any data it should be done here.
*/

import (
	"sync"

	"github.com/gorilla/websocket"
)

var Mutex sync.Mutex

// Room represents a game room containing two players.
type Room struct {
	Player1  *Player
	Player2  *Player
	BidAmount float64
}

// Player represents a single player in the game.
// The selected bid only need to be passed to the server when the player looks for a match.
type Player struct {
	Id int
	Name string
	Money float64
	Conn *websocket.Conn
	Room *Room
	Color int	// 1 = black / 0 = white -- This is defined on room start.
	SelectedBid float64
}

var ConnectedPlayers []*Player 	// All players connected.
var WaitingQueue []*Player		// Those waiting for a game.
var Rooms []*Room				// Rooms of ongoing games 


// * This is cooool
func (r *Room) GetOpponent(player *Player) (*Player){
	// Just in case there is no room 
	if r == nil{
		return nil
	}

	if r.Player1 == player {
		return r.Player2
	} else if r.Player2 == player {
		return r.Player1
	}
	return nil;
}

// RemovePlayer removes a player from the ConnectedPlayers list and queue
func RemovePlayer(player *Player) {
	Mutex.Lock()
	defer Mutex.Unlock()

	for i, p := range ConnectedPlayers {
		if p == player {
			ConnectedPlayers = append(ConnectedPlayers[:i], ConnectedPlayers[i+1:]...)
			break
		}
	}		
	for i, p := range WaitingQueue {
		if p == player {
			WaitingQueue = append(WaitingQueue[:i], WaitingQueue[i+1:]...)
			break
		}
	}
}

func AddToQueue(player *Player) {
	Mutex.Lock()
	defer Mutex.Unlock()

	WaitingQueue = append(WaitingQueue, player)
}

func FilterWaitingQueue(queue []*Player, predicate func(*Player) bool) []*Player {
	var filtered []*Player
	for _, player := range queue {
		if predicate(player) {
			filtered = append(filtered, player)
		}
	}
	return filtered
}

func IsPlayerInQueue(player *Player) bool {
    for _, p := range WaitingQueue {
        if p == player { 
            return true
        }
    }
    return false
}

func IsPlayerInGame(player *Player) bool {
    for _, r := range Rooms {
        if r.Player1 == player || r.Player2 == player  { 
            return true
        }
    }
    return false
}

func FindPlayerByName(playerName string) *Player {
	for _, p := range ConnectedPlayers {
		if p.Name == playerName { // Match based on player name
			return p
		}
	}
	return nil // Return nil if no player is found
}

func RemoveFromQueue(player *Player) {
	Mutex.Lock()
	defer Mutex.Unlock()

	for i, p := range WaitingQueue {
		if p == player {
			WaitingQueue = append(WaitingQueue[:i], WaitingQueue[i+1:]...)
			break
		}
	}
}

func CreateRoom(p1, p2 *Player) *Room {
	Mutex.Lock()
	defer Mutex.Unlock()

	room := &Room{Player1: p1, Player2: p2, BidAmount: p1.SelectedBid}
	Rooms = append(Rooms, room)
	p1.Room = room
	p2.Room = room

	return room
}

func RemoveRoom(room *Room) {
	Mutex.Lock()
	defer Mutex.Unlock()

	for i, r := range Rooms {
		if r == room {
			Rooms = append(Rooms[:i], Rooms[i+1:]...)
			break
		}
	}
}
