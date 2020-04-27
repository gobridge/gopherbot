package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	_ "github.com/heroku/x/hmetrics/onload"

	"github.com/theckman/gopher2/config"
)

func main() {
	cfg, err := config.LoadEnv()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, "no")
			return
		}
		_, _ = io.WriteString(w, "Hello World!")
	})

	redisOpts := &redis.Options{
		Network:      "tcp",
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  11 * time.Second,
		WriteTimeout: 5 * time.Second,
		PoolSize:     20,
		MinIdleConns: 5,
		PoolTimeout:  5 * time.Second,
	}

	c := redis.NewClient(redisOpts)
	key := fmt.Sprintf("heartbeat:%s:%s", cfg.Heroku.AppName, cfg.Heroku.DynoID)
	res := c.Set(key, time.Now().Unix(), 0)
	if res.Err() != nil {
		log.Printf("redis error: %v", res.Err())
	}

	serverAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)

	fmt.Println(http.ListenAndServe(serverAddr, mux))
}
