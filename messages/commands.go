package messages

type CommandType string

const (
	ServerCommand    CommandType = "server"
	ClientCommand    CommandType = "client"
	BroadcastCommand CommandType = "broadcast"
)

type CommandInfo struct {
	Type CommandType
}

var validCommands = map[string]CommandInfo{
	"create_room":  {Type: ClientCommand},
	"room_created": {Type: ServerCommand},
	"paired":       {Type: ServerCommand},
	"ready_room":   {Type: ClientCommand},
	"join_room":    {Type: ClientCommand},
	"leave_room":   {Type: ClientCommand},
	"game_info":    {Type: BroadcastCommand},
}
