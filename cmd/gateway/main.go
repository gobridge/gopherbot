package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

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

	serverAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)

	fmt.Println(http.ListenAndServe(serverAddr, mux))
}
