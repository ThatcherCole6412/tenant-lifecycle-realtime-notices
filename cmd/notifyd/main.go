package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"example.com/tenant-notify-cutover/notify"
)

type server struct {
	client    *notify.Client
	lifecycle *notify.LifecycleService
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := notify.NewClient(key)
	s := &server{client: client, lifecycle: notify.NewLifecycleService(client)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", s.events)
	mux.HandleFunc("POST /sessions", s.sessions)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("notifyd listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	var input notify.LifecycleEvent
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.lifecycle.Notify(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *server) sessions(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ClientID string `json:"client_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := decodeJSON(r, &input); err != nil || input.ClientID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid client_id and tenant_id are required"})
		return
	}
	channel, err := notify.TenantChannel(input.TenantID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	token, err := s.client.IssueToken(r.Context(), input.ClientID, []string{channel})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, token)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var apiErr *notify.APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		writeJSON(w, apiErr.Status, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
		return
	}
	log.Printf("request failed: %v", err)
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "notification delivery failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
