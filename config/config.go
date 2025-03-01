package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

/*
	{
	"redis": {
		"addr": "localhost:6379",
		"db": 0
	},
	"services": {
		"wsapi": { "ports": [8080, 8081, 8082] },
		"pstatusworker": {},
		"roomworker": {}
	}
	}
*/
type Config struct {
	Redis struct {
		Addr string `json:"addr"`
		DB   int    `json:"db"`
	} `json:"redis"`
	Services map[string]struct {
		Ports []int `json:"ports,omitempty"`
		Timer int   `json:"timer,omitempty"`
	} `json:"services"`
}
// Global config instance
var Cfg Config

func LoadConfig() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("[config.go] - CONFIG_PATH not set")
	} 
	fmt.Println("[config.go] - Config path is:", configPath)
	
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("[config.go] -Error opening config file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&Cfg); err != nil {
		log.Fatalf("[config.go] -Error decoding JSON: %v", err)
	}
	log.Printf("Config loaded: %+v", Cfg)
}