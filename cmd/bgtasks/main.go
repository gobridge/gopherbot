package main

import (
	"log"

	_ "github.com/heroku/x/hmetrics/onload"

	"github.com/gobridge/gopherbot/config"
)

func main() {
	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := config.DefaultLogger(cfg)

	if err := runServer(cfg, logger); err != nil {
		log.Fatalf("failed to run new bgtasks server: %v", err.Error())
	}
}
