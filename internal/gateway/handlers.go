package gateway

import (
	"io"
	"net/http"
)

func (s *server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}

func (s *server) handleRUOK(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "imok")
}

func (s *server) handleSlackEvent(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "Hello Slack!")
}
