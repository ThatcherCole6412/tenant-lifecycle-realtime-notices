package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublishSendsIdempotencyKeyInRequestBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Idempotency-Key"); got != "" {
			t.Errorf("Idempotency-Key header = %q, want empty", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if got := body["idempotency_key"]; got != "evt-202" {
			t.Errorf("idempotency_key = %v, want evt-202", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":null}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL
	if err := client.Publish(context.Background(), "evt-202", "tenant-acme", "account.ready", "acct-7", map[string]any{"severity": "info"}); err != nil {
		t.Fatal(err)
	}
}
