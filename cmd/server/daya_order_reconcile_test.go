package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDayaOrderReconciliationRouteRecordsManualConfirmation(t *testing.T) {
	store := &Store{path: filepath.Join(t.TempDir(), "orders.json"), orders: []Order{{
		ID: "local-unknown", ExecutionMode: executionModeAPI, Status: statusSubmissionUnknown,
	}}}
	mux := http.NewServeMux()
	registerDayaOrderReconciliationRoute(mux, store)

	form := url.Values{
		"id":             {"local-unknown"},
		"resolution":     {"registered"},
		"broker_order_id": {"DAYA-ORDER-42"},
		"broker_message": {"verified in official Daya panel"},
	}
	req := httptest.NewRequest(http.MethodPost, "/orders/reconcile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://example.com")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusSeeOther || recorder.Header().Get("Location") != "/history?notice=reconciled" {
		t.Fatalf("response = %d, Location=%q", recorder.Code, recorder.Header().Get("Location"))
	}
	got := store.snapshot()
	if len(got) != 1 || got[0].Status != statusSubmitted || got[0].BrokerOrderID != "DAYA-ORDER-42" {
		t.Fatalf("order was not reconciled: %#v", got)
	}
	if got[0].BrokerMessage != "verified in official Daya panel" || !got[0].ReconciledAt.After(time.Time{}) {
		t.Fatalf("reconciliation audit fields missing: %#v", got[0])
	}
}

func TestDayaOrderReconciliationRouteRejectsCrossOrigin(t *testing.T) {
	store := &Store{path: filepath.Join(t.TempDir(), "orders.json"), orders: []Order{{
		ID: "local-unknown", ExecutionMode: executionModeAPI, Status: statusSubmissionUnknown,
	}}}
	mux := http.NewServeMux()
	registerDayaOrderReconciliationRoute(mux, store)

	form := url.Values{
		"id":             {"local-unknown"},
		"resolution":     {"registered"},
		"broker_order_id": {"DAYA-ORDER-42"},
	}
	req := httptest.NewRequest(http.MethodPost, "/orders/reconcile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://attacker.example")
	req.Host = "example.com"
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusForbidden)
	}
	if store.orders[0].Status != statusSubmissionUnknown {
		t.Fatalf("cross-origin request changed order state to %q", store.orders[0].Status)
	}
}
