package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"example.com/marketplace-chat/internal/marketplace"
)

type server struct{ realtime *marketplace.Client }

type openRoomRequest struct {
	OrderID   string `json:"order_id"`
	ClientID  string `json:"client_id"`
	RequestID string `json:"request_id"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	s := &server{realtime: marketplace.NewClient(key)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders/open", s.openRoom)
	mux.HandleFunc("POST /orders/update", s.publishUpdate)
	log.Printf("marketplace chat listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) openRoom(w http.ResponseWriter, r *http.Request) {
	var input openRoomRequest
	if !decode(w, r, &input) {
		return
	}
	if input.OrderID == "" || input.ClientID == "" || input.RequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order_id, client_id, and request_id are required"})
		return
	}
	channel := marketplace.ChannelFor(input.OrderID)
	if err := s.realtime.CreateOrderChannel(r.Context(), channel, input.RequestID+"-channel"); err != nil {
		writeServiceError(w, err)
		return
	}
	token, err := s.realtime.IssueRoomToken(r.Context(), input.ClientID, channel, input.RequestID+"-token")
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"channel": channel, "token": json.RawMessage(token)})
}

func (s *server) publishUpdate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RequestID string                  `json:"request_id"`
		Update    marketplace.OrderUpdate `json:"update"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.RequestID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request_id is required"})
		return
	}
	if err := input.Update.Validate(); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	if err := s.realtime.PublishOrderUpdate(r.Context(), input.Update, input.RequestID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"channel": marketplace.ChannelFor(input.Update.OrderID), "event": string(input.Update.Kind), "status": "published"})
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

func writeServiceError(w http.ResponseWriter, err error) {
	var apiErr *marketplace.APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		writeJSON(w, apiErr.Status, map[string]any{"error": apiErr.Code, "detail": apiErr.Detail})
		return
	}
	log.Printf("realtime operation failed: %v", err)
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "realtime operation failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
