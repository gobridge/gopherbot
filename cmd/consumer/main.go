package main

import (
	"log"

	_ "github.com/heroku/x/hmetrics/onload"

	"github.com/theckman/gopher2/config"
	"github.com/theckman/gopher2/internal/consumer"
)

func main() {
	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := consumer.RunServer(cfg); err != nil {
		log.Fatalf("failed to run new gateway server: %v", err.Error())
	}
}
