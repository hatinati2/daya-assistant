package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func apiTestOrder() Order {
	return Order{ID: "local-1", ISIN: "IRO3SIMC0001", Side: "buy", Price: 63390, Quantity: 1, ExecutionMode: executionModeAPI}
}

func TestNewDayaOrderRequestUsesObservedEndpointAndSessionCookies(t *testing.T) {
	session := DayaSession{
		Token: "test-token",
		Cookies: []DayaSessionCookie{
			{Name: "cookiesession1", Value: "captcha-session"},
			{Name: "token", Value: "stale-token"},
			{Name: "unrelated", Value: "must-not-be-sent"},
		},
	}
	req, err := newDayaOrderRequest(context.Background(), session, apiTestOrder())
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != http.MethodPost || req.URL.String() != dayaOrderEndpoint {
		t.Fatalf("request target = %s %s", req.Method, req.URL)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type=%q", got)
	}
	cookie := req.Header.Get("Cookie")
	if !strings.Contains(cookie, "token=test-token") || !strings.Contains(cookie, "cookiesession1=captcha-session") {
		t.Fatalf("required session cookies missing: %q", cookie)
	}
	if strings.Contains(cookie, "unrelated") || strings.Contains(cookie, "stale-token") {
		t.Fatalf("unexpected cookie was included: %q", cookie)
	}
}

func TestDayaOrderResponseClassificationFailsClosed(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantStatus string
	}{
		{name: "2xx without verified success schema is unknown", status: http.StatusCreated, wantStatus: statusSubmissionUnknown},
		{name: "4xx is a definite rejection", status: http.StatusBadRequest, wantStatus: statusSubmissionFailed},
		{name: "5xx is ambiguous", status: http.StatusBadGateway, wantStatus: statusSubmissionUnknown},
		{name: "redirect is ambiguous", status: http.StatusFound, wantStatus: statusSubmissionUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := classifyDayaOrderResponse(test.status, []byte(`{"status":400,"message":"rejected","requestId":"req-test"}`))
			if got.Status != test.wantStatus {
				t.Fatalf("status=%q, want %q (message: %s)", got.Status, test.wantStatus, got.Message)
			}
			if got.HTTPStatus != test.status || got.RequestID != "req-test" {
				t.Fatalf("response metadata not preserved: %#v", got)
			}
		})
	}
}

func TestDayaOrderResponseRecognizesRealGroupStateRejection(t *testing.T) {
	body := []byte("امكان ثبت سفارش با توجه به وضعيت كروه وجود ندارد")
	outcome := classifyDayaOrderResponse(http.StatusBadRequest, body)
	if outcome.Status != statusSubmissionFailed {
		t.Fatalf("status=%q, want definite submission failure", outcome.Status)
	}
	if outcome.BrokerOrderID != "" {
		t.Fatalf("broker order ID=%q, want empty", outcome.BrokerOrderID)
	}
	if outcome.Message != "امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد" {
		t.Fatalf("message=%q", outcome.Message)
	}
}

func TestDayaDispatcherDoesNothingWhileLiveGateIsClosed(t *testing.T) {
	dispatcher := &DayaOrderDispatcher{
		liveEnabled: func() bool { return false },
		now:         func() time.Time { return time.Now() },
	}
	if err := dispatcher.dispatchReady(context.Background()); err != nil {
		t.Fatal(err)
	}
}
