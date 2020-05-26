package main

import (
	"log"

	_ "github.com/heroku/x/hmetrics/onload"

	"github.com/gobridge/gopherbot/config"
	"github.com/gobridge/gopherbot/internal/gateway"
)

func main() {
	c, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	l := config.DefaultLogger(c)

	if err := gateway.RunServer(c, l); err != nil {
		l.Fatal().
			Err(err).
			Msg("failed to run gateway server")
	}
}
