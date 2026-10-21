package marketplace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublishRetriesWithSameIdempotencyKey(t *testing.T) {
	var calls int
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{},"metadata":{}}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["event"] != "buyer_update" {
			t.Errorf("event = %v", body["event"])
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"published":true},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL
	client.http = server.Client()
	client.sleep = func(context.Context, time.Duration) error { return nil }
	update := OrderUpdate{OrderID: "ord-7", AccountID: "buyer-4", State: OrderShipped, Kind: BuyerUpdate, Message: "Arriving Friday"}
	if err := client.PublishOrderUpdate(context.Background(), update, "req-22"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if keys[0] != "req-22" || keys[1] != "req-22" {
		t.Fatalf("idempotency keys = %v", keys)
	}
}
