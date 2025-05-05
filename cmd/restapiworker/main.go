package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Lavizord/checkers-server/internal/config"
	"github.com/Lavizord/checkers-server/internal/interfaces"
	"github.com/Lavizord/checkers-server/internal/models"
	"github.com/Lavizord/checkers-server/internal/postgrescli"
	"github.com/Lavizord/checkers-server/internal/redisdb"

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
		respondWithJSON(w, http.StatusBadRequest, models.GameLaunchResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if req.GameID == "" || req.OperatorName == "" {
		respondWithJSON(w, http.StatusBadRequest, models.GameLaunchResponse{
			Success: false,
			Message: "Game ID and Operator Name are required",
		})
		return
	}

	//log.Printf("Received Game Launch Request: %+v", req)

	operator, err := postgresClient.FetchOperator(req.OperatorName, req.GameID)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, models.GameLaunchResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid operator / gameID: %v", err),
		})
		return
	}

	if !operator.Active {
		respondWithJSON(w, http.StatusBadRequest, models.GameLaunchResponse{
			Success: false,
			Message: fmt.Sprintf("Inactive game for operator: %s", req.OperatorName),
		})
		return
	}

	module, exists := interfaces.OperatorModules[req.OperatorName]
	if !exists {
		respondWithJSON(w, http.StatusBadRequest, models.GameLaunchResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported operator: %s", req.OperatorName),
		})
		return
	}

	// Delegate the request to the module
	module.HandleGameLaunch(w, r, req, *operator, redisClient, postgresClient)
}

// Utility function to respond with JSON
func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/gamelaunch", gameLaunchHandler).Methods("POST")
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")
	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
