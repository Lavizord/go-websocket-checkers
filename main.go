package main

import (
	"checkers-server/config"
	"checkers-server/redisdb"
	"checkers-server/wsapi"
	"fmt"
	"log"
	"net/http"
	"os"
)

func init() {
	config.LoadConfig()
	redisConData := config.Cfg.Redis
	client, err := redisdb.NewRedisClient(redisConData.Addr, redisConData.User, redisConData.Password)
	if err != nil {
		log.Panicf("[Redis] Error initializing Redis client: %v", err)
	}
	wsapi.RedisClient = client

}

func hasLocalCerts() (exists bool, cert string, key string) {
	certPath := os.Getenv("SSL_CERT_PATH")
	keyPath := os.Getenv("SSL_KEY_PATH")
	if certPath == "" || keyPath == "" {
		return false, "", ""
	}
	return true, certPath, keyPath
}

func main() {
	defer func() {
		if wsapi.RedisClient != nil {
			wsapi.RedisClient.CloseRedisClient()
		}
	}()

	port := config.FirstPortFromConfig("wsapi")
	addrs := fmt.Sprintf(":%d", port)

	http.HandleFunc("/ws/health", func(w http.ResponseWriter, r *http.Request) {
		if err := wsapi.RedisClient.Client.Ping(r.Context()).Err(); err != nil {
			http.Error(w, "Redis unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	go wsapi.SubscribeToBroadcastChannel() // This is a global channel. WSAPI will send the messages from this channel to all active ws connections
	has, certPath, keyPath := hasLocalCerts()
	if has {
		http.HandleFunc("/ws/checkers", wsapi.HandleConnection)
		log.Println("[wsapi] - SSL certificate paths set, listening on HTTPS .")
		log.Printf("[wsapi] - WebSocket server started on %s\n", addrs)
		log.Fatal(http.ListenAndServeTLS(addrs, certPath, keyPath, nil))
	} else {
		http.HandleFunc("/ws/checkers", wsapi.HandleConnection)
		log.Println("[wsapi] - SSL certificate paths not set, defaulting to listen on HTTP.")
		log.Printf("[wsapi] - WebSocket server started on %s\n", addrs)
		log.Fatal(http.ListenAndServe(addrs, nil))
	}
}
