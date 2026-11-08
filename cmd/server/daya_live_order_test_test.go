package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseDayaLiveOrderTestFormBuildsOnlyValidatedOneShotPayload(t *testing.T) {
	form := url.Values{
		"isin":     {"IR0000000001"},
		"side":     {"buy"},
		"price":    {"12345"},
		"quantity": {"2"},
	}
	req := httptest.NewRequest(http.MethodPost, "/daya-online-test/one/send", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	order, err := parseDayaLiveOrderTestForm(req)
	if err != nil {
		t.Fatal(err)
	}
	if order.ISIN != "IR0000000001" || order.Side != "buy" || order.Price != 12345 || order.Quantity != 2 || order.ExecutionMode != executionModeAPI {
		t.Fatalf("parsed one-shot order = %#v", order)
	}
	if order.Status != "" || !order.ExecuteAt.IsZero() {
		t.Fatalf("parser must not create or schedule an order: %#v", order)
	}
}

func TestParseDayaLiveOrderTestFormRejectsInvalidSideAndNumbers(t *testing.T) {
	tests := []url.Values{
		{"isin": {"IR0000000001"}, "side": {"hold"}, "price": {"10"}, "quantity": {"1"}},
		{"isin": {"IR0000000001"}, "side": {"buy"}, "price": {"0"}, "quantity": {"1"}},
		{"isin": {"IR0000000001"}, "side": {"buy"}, "price": {"10"}, "quantity": {"nope"}},
		{"isin": {"short"}, "side": {"buy"}, "price": {"10"}, "quantity": {"1"}},
	}
	for i, form := range tests {
		req := httptest.NewRequest(http.MethodPost, "/daya-online-test/one/send", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if _, err := parseDayaLiveOrderTestForm(req); err == nil {
			t.Fatalf("invalid test order %d was accepted", i)
		}
	}
}

func TestDayaLiveOrderTestNonceIsSingleUseAndExpires(t *testing.T) {
	now := time.Now()
	nonce, err := issueDayaLiveOrderTestNonce(now)
	if err != nil {
		t.Fatal(err)
	}
	if !consumeDayaLiveOrderTestNonce(nonce, now.Add(time.Second)) {
		t.Fatal("fresh one-time nonce was rejected")
	}
	if consumeDayaLiveOrderTestNonce(nonce, now.Add(2*time.Second)) {
		t.Fatal("one-time nonce was accepted twice")
	}
	expired, err := issueDayaLiveOrderTestNonce(now)
	if err != nil {
		t.Fatal(err)
	}
	if consumeDayaLiveOrderTestNonce(expired, now.Add(31*time.Minute)) {
		t.Fatal("expired nonce was accepted")
	}
}

func TestDayaLiveOrderTestRouteIsPreviewOnly(t *testing.T) {
	mux := http.NewServeMux()
	registerDayaLiveOrderTestRoutes(mux, false, nil, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/daya-online-test/one", nil)
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("non-Preview live-order test route status = %d, want 404", recorder.Code)
	}
}
