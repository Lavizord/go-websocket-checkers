package messages

import (
	"checkers-server/models"
	"encoding/json"
	"fmt"
)

type Message[T any] struct {
	Command string `json:"command"`
	Value   T      `json:"value,omitempty"`
}

type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type MovePieceValue struct {
	PreviousPosition Position   `json:"previous_position"`
	NewPosition      Position   `json:"new_position"`
	KilledPieces     []Position `json:"killed_pieces"`
}

func EncodeMessage[T any](command string, value T) ([]byte, error) {
	msg := Message[T]{Command: command, Value: value}
	return json.Marshal(msg)
}

func DecodeRawMessage(data []byte) (*Message[json.RawMessage], error) {
	var msg Message[json.RawMessage]
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("[Message Parser - DecodeRawMessage] invalid message format: %w", err)
	}
	return &msg, nil
}

// Decode a Fully Typed Message
func DecodeTypedMessage[T any](data []byte) (*Message[T], error) {
	var msg Message[T]
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("[Message Parser - DecodeTypedMessage] invalid message format: %w", err)
	}
	return &msg, nil
}

func NewMessage[T any](command string, value T) ([]byte, error) {
	if _, ok := validCommands[command]; !ok {
		return nil, fmt.Errorf("[Message Parser - New Message] invalid command: %s", command)
	}
	message := Message[T]{
		Command: command,
		Value:   value,
	}
	return json.Marshal(message)
}

func ParseMessage(msgBytes []byte) (*Message[json.RawMessage], error) {
	msg, err := DecodeRawMessage(msgBytes)
	if err != nil {
		return nil, err
	}
	// Check if the command is in our map
	if _, ok := validCommands[msg.Command]; !ok {
		return nil, fmt.Errorf("[Message Parser] invalid command: %s", msg.Command)
	}

	switch msg.Command {
	case "create_room", "join_room":
		var value float64
		if err := json.Unmarshal(msg.Value, &value); err != nil {
			return nil, fmt.Errorf("[Message Parser] invalid value format for %s: %w", msg.Command, err)
		}

	case "leave_room":
		return msg, nil

	case "ready_room":
		return msg, nil

	case "move_piece":
		var value MovePieceValue
		if err := json.Unmarshal(msg.Value, &value); err != nil {
			return nil, fmt.Errorf("[Message Parser] invalid value format for move_piece: %w", err)
		}

	case "custom_command":
		if !json.Valid(msg.Value) {
			return nil, fmt.Errorf("[Message Parser] invalid JSON format for custom_command")
		}

	case "game_info":
		var roomAggregateResponse models.RoomAggregateResponse
		if err := json.Unmarshal(msg.Value, &roomAggregateResponse); err != nil {
			return nil, fmt.Errorf("invalid value format for game_info: %w", err)
		}
		fmt.Printf("[Message Parser] Parsed game_info: %+v\n", roomAggregateResponse)
	}

	return msg, nil
}

// TODO:: Make this use the new message foramt / parser
func GenerateConnectedMessage(player *models.Player) (string, error) {
	msg, err := EncodeMessage("connected", struct {
		PlayerName string  `json:"player_name"`
		Money      float64 `json:"money"`
	}{
		PlayerName: "Player_" + player.Name,
		Money:      float64(player.CurrencyAmount),
	})

	if err != nil {
		return "", err
	}
	return string(msg), nil
}

func GeneratePairedMessage(player1, player2 *models.Player, roomID string, color int) ([]byte, error) {
	pairedValue := models.PairedValue{
		Color:    color,
		Opponent: player2.Name,
		RoomID:   roomID,
	}
	return NewMessage("paired", pairedValue)
}

func GenerateRoomCreatedMessage(room models.Room) ([]byte, error) {
	roomValue := models.RoomValue{
		ID:          room.ID,
		Player:      room.Player1.Name,
		Currency:    room.Currency,
		SelectedBid: room.BidAmount,
	}
	return NewMessage("room_created", roomValue)
}

// Helper function to marshal a value and ignore errors
func MustMarshal(v interface{}) json.RawMessage {
	bytes, _ := json.Marshal(v)
	return bytes
}
