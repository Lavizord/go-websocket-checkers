package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/Lavizord/checkers-server/config"
)

var addr = flag.String("addr", ":80", "http service address")

func main() {

	config.LoadConfig()
	redisConfig := config.Cfg.Redis

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	flag.Parse()
	hub := newHub(redisConfig.Addr, redisConfig.User, redisConfig.Password)
	go hub.run()
	// we subscribe to our redis broadcast channel.
	hub.SubscribeBroadcast()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
