package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDayaTestTransportPostsObservedPayloadToLoopback(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/orders" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content-type=%q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":400,"path":"/orders","message":"simulated group-state rejection"}`))
	}))
	defer server.Close()

	order := Order{ISIN: "IRO3SIMC0001", Side: "buy", Price: 63390, Quantity: 1, ExecutionMode: executionModeAPI}
	status, body, err := postDayaOrderToTestServer(context.Background(), server.Client(), server.URL, order)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusBadRequest || !strings.Contains(string(body), "simulated group-state rejection") {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if got["instrumentId"] != "IRO3SIMC0001" || got["orderSide"] != "BUY" || got["orderQuantity"] != float64(1) {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

func TestDayaTestTransportRejectsNonLoopbackBeforeNetwork(t *testing.T) {
	order := Order{ISIN: "IRO3SIMC0001", Side: "buy", Price: 1, Quantity: 1, ExecutionMode: executionModeAPI}
	for _, address := range []string{"https://mg.dayatrader.ir", "http://example.com", "http://127.0.0.1.evil.test"} {
		if _, _, err := postDayaOrderToTestServer(context.Background(), nil, address, order); err == nil {
			t.Fatalf("expected %q to be rejected", address)
		}
	}
}
