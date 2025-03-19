package main

import (
	"checkers-server/config"
	"checkers-server/interfaces"
	"checkers-server/models"
	"checkers-server/postgrescli"
	"checkers-server/redisdb"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

var pid int
var postgresClient *postgrescli.PostgresCli
var redisClient *redisdb.RedisClient
var name = "restapiworker"

func init() {
	pid = os.Getpid()
	config.LoadConfig()

	redisAddr := config.Cfg.Redis.Addr
	client, err := redisdb.NewRedisClient(redisAddr)
	if err != nil {
		log.Fatalf("[%s-Redis] Error initializing Redis client: %v\n", name, err)
	}
	redisClient = client

	sqlcliente, err := postgrescli.NewPostgresCli(
		config.Cfg.Postgres.User,
		config.Cfg.Postgres.Password,
		config.Cfg.Postgres.DBName,
		config.Cfg.Postgres.Host,
		config.Cfg.Postgres.Port,
	)
	if err != nil {
		log.Fatalf("[%PostgreSQL] Error initializing POSTGRES client: %v\n", err)
	}
	postgresClient = sqlcliente
}

func gameLaunchHandler(w http.ResponseWriter, r *http.Request) {
	var req models.GameLaunchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.GameID == "" || req.OperatorName == "" {
		http.Error(w, "Game ID and Operator Name are required", http.StatusBadRequest)
		return
	}
	log.Printf("Received Game Launch Request: %+v", req)

	operator, err := postgresClient.FetchOperator(req.OperatorName, req.GameID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid operator / gameID: %v", err), http.StatusBadRequest)
		return
	}
	if !operator.Active {
		http.Error(w, fmt.Sprintf("Inactive game for operator: %s", req.OperatorName), http.StatusBadRequest)
		return
	}
	module, exists := interfaces.OperatorModules[req.OperatorName]
	if !exists {
		http.Error(w, fmt.Sprintf("Unsupported operator: %s", req.OperatorName), http.StatusBadRequest)
		return
	}

	// Delegate the request to the module
	module.HandleGameLaunch(w, r, req, *operator, redisClient, postgresClient)
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/gamelaunch", gameLaunchHandler).Methods("POST")

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
