package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildDayaDashboardSummaryCountsOrders(t *testing.T) {
	store := &Store{orders: []Order{
		{ID: "scheduled-1", Status: statusScheduled},
		{ID: "scheduled-2", Status: statusScheduled},
		{ID: "ready-1", Status: statusReady},
		{ID: "cancelled-1", Status: statusCancelled},
		{ID: "submitted-1", Status: statusSubmitted},
	}}
	got := buildDayaDashboardSummary(store, nil, nil, time.Now().In(tehran()))
	if got.ScheduledOrders != 2 || got.ReadyOrders != 1 || got.HistoryOrders != 2 || got.TotalOrders != 5 {
		t.Fatalf("unexpected dashboard order counts: %#v", got)
	}
	if got.CurrentTime == "" {
		t.Fatal("dashboard server time is empty")
	}
}

func TestDayaDashboardRouteIsPreviewOnly(t *testing.T) {
	mux := http.NewServeMux()
	registerDayaDashboardRoutes(mux, false, nil, nil, nil)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("dashboard in non-preview mode returned %d, want 404", recorder.Code)
	}
}
