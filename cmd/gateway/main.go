package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	_ "github.com/heroku/x/hmetrics/onload"
)

func main() {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		panic("PORT environment variable must be set")
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

	fmt.Println(http.ListenAndServe("0.0.0.0:"+port, mux))
}
