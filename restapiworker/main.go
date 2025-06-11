package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Lavizord/checkers-server/config"
	"github.com/Lavizord/checkers-server/interfaces"
	"github.com/Lavizord/checkers-server/models"
	"github.com/Lavizord/checkers-server/postgrescli"
	"github.com/Lavizord/checkers-server/redisdb"

	"github.com/gorilla/mux"
)

var postgresClient *postgrescli.PostgresCli
var redisClient *redisdb.RedisClient
var name = "restapi"

func init() {
	config.LoadConfig()

	redisConData := config.Cfg.Redis
	client, err := redisdb.NewRedisClient(redisConData.Addr, redisConData.User, redisConData.Password, redisConData.Tls)
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
		config.Cfg.Postgres.Ssl,
	)
	if err != nil {
		log.Fatalf("[PostgreSQL] Error initializing POSTGRES client: %v\n", err)
	}
	postgresClient = sqlcliente
}

func gameLaunchHandler(w http.ResponseWriter, r *http.Request) {
	go handleGameLaunchHandler(w, r)
}

func handleGameLaunchHandler(w http.ResponseWriter, r *http.Request) {
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
	// 1ª Procurar no cache, o operator tem ttl definido.
	operator, err := redisClient.GetOperator(req.OperatorName, req.GameID)
	if err != nil {
		// se não encontrar no cache procurar no postgress.
		log.Printf("[GameLaunchHandler] - error fetching operator from redis: %v", err)
		operator, err = postgresClient.FetchOperator(req.OperatorName, req.GameID)
		if err != nil {
			log.Printf("[GameLaunchHandler] - error fetching the operator from sql: %v", err)
			respondWithJSON(w, http.StatusBadRequest, models.GameLaunchResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid operator / gameID: %v", err),
			})
			return
		}
		// Se encontrar no postgress, guardar no cache.
		redisClient.AddOperator(operator)
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

func gameMovesHandler(w http.ResponseWriter, r *http.Request) {
	var req models.GameMovesRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.GameID == "" {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid or missing game_id",
		})
		return
	}

	log.Printf("Fetching moves for gameID: [%s]", req.GameID)
	moves, err := postgresClient.FetchGameMoves(req.GameID)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Failed to fetch moves :" + err.Error(),
		})
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"moves":   moves,
	})
}

// Utility function to respond with JSON
func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func registerRoutes(r *mux.Router) {
	r.HandleFunc("/api/gamelaunch", gameLaunchHandler).Methods("POST")
	r.HandleFunc("/api/game/moves", gameMovesHandler).Methods("POST")

	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/api/health", healthHandler).Methods("GET")
}

func main() {
	defer func() {
		if redisClient != nil {
			redisClient.CloseRedisClient()
		}
		if postgresClient != nil {
			postgresClient.Close()
		}
	}()

	router := mux.NewRouter()
	registerRoutes(router)

	port := config.FirstPortFromConfig(name)
	addrs := fmt.Sprintf(":%d", port)

	log.Printf("[API] - HTTP server starting on %d...", port)
	log.Fatal(http.ListenAndServe(addrs, router))
}
