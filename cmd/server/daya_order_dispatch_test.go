package main

import (
	"context"
	"net/http"
	"net/http/httptest"
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
func TestDayaOrderBookedConfirmationRequiresFullMatch(t *testing.T) {
	expected := Order{
		ExecutionMode: executionModeAPI,
		ISIN:          "IRT1FPAF0001",
		Side:          "buy",
		Price:         9707,
		Quantity:      617,
	}
	body := []byte(`[{
		"orderFunction":"CONFIRMATION",
		"orderStatus":"BOOKED",
		"orderId":12345,
		"hostOrderNumber":6789,
		"instrumentId":"IRT1FPAF0001",
		"orderPrice":9707,
		"orderQuantity":617,
		"orderValidityType":"JUST_TODAY",
		"orderSide":"BUY",
		"orderValidityDate":null
	}]`)

	got := classifyDayaOrderResponseForOrder(http.StatusOK, body, &expected)
	if got.Status != statusSubmitted || got.BrokerOrderID != "12345" || got.BrokerHostOrderNumber != 6789 {
		t.Fatalf("matching BOOKED confirmation was not recognized: %#v", got)
	}

	// A 2xx payload must remain ambiguous when it belongs to a different order.
	wrongOrder := expected
	wrongOrder.Quantity++
	got = classifyDayaOrderResponseForOrder(http.StatusOK, body, &wrongOrder)
	if got.Status != statusSubmissionUnknown || got.BrokerOrderID != "" {
		t.Fatalf("non-matching BOOKED record was treated as this order: %#v", got)
	}

	// The same confirmation record cannot be attributed without the submitted
	// order context, preventing an arbitrary order-history row being accepted.
	got = classifyDayaOrderResponse(http.StatusOK, body)
	if got.Status != statusSubmissionUnknown || got.BrokerOrderID != "" {
		t.Fatalf("BOOKED row without matching context was confirmed: %#v", got)
	}
}

func TestDayaOrderBookedConfirmationRejectsIncompleteRecord(t *testing.T) {
	expected := Order{ExecutionMode: executionModeAPI, ISIN: "IRT1FPAF0001", Side: "buy", Price: 9707, Quantity: 617}
	body := []byte(`{"orderFunction":"CONFIRMATION","orderStatus":"BOOKED","orderId":0,"hostOrderNumber":6789,"instrumentId":"IRT1FPAF0001","orderPrice":9707,"orderQuantity":617,"orderValidityType":"JUST_TODAY","orderSide":"BUY","orderValidityDate":null}`)
	got := classifyDayaOrderResponseForOrder(http.StatusOK, body, &expected)
	if got.Status != statusSubmissionUnknown || got.BrokerOrderID != "" {
		t.Fatalf("incomplete confirmation was accepted: %#v", got)
	}
}

func TestDayaEntryAcknowledgementIsPendingUntilBooked(t *testing.T) {
	expected := Order{
		ExecutionMode: executionModeAPI,
		ISIN:          "IRT1FPAF0001",
		Side:          "buy",
		Price:         9707,
		Quantity:      617,
	}
	body := []byte(`{
		"orderFunction":"ENTRY",
		"orderStatus":"SENT_TO_TRADING_ENGINE",
		"orderId":1787906,
		"hostOrderNumber":null,
		"instrumentId":"IRT1FPAF0001",
		"orderPrice":9707,
		"orderQuantity":617,
		"orderValidityType":"JUST_TODAY",
		"orderSide":"BUY",
		"orderValidityDate":null,
		"errorCode":null,
		"errorText":null
	}`)

	got := classifyDayaOrderResponseForOrder(http.StatusOK, body, &expected)
	if got.Status != statusSubmissionPending || got.BrokerOrderID != "1787906" || got.BrokerHostOrderNumber != 0 {
		t.Fatalf("ENTRY acknowledgement must be pending, got %#v", got)
	}

	wrong := expected
	wrong.Price++
	got = classifyDayaOrderResponseForOrder(http.StatusOK, body, &wrong)
	if got.Status != statusSubmissionUnknown || got.BrokerOrderID != "" {
		t.Fatalf("ENTRY for a different order must remain unknown: %#v", got)
	}
}

func TestFetchDayaOrdersIsReadOnlyAndUsesSessionCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/orders" {
			t.Errorf("unexpected order-list request: %s %s", r.Method, r.URL.Path)
		}
		if cookie, err := r.Cookie("token"); err != nil || cookie.Value != "current-token" {
			t.Errorf("current token cookie missing")
		}
		if cookie, err := r.Cookie("cookiesession1"); err != nil || cookie.Value != "captcha-session" {
			t.Errorf("captcha session cookie missing")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	session := DayaSession{
		Token: "current-token",
		Cookies: []DayaSessionCookie{
			{Name: "token", Value: "stale-token"},
			{Name: "cookiesession1", Value: "captcha-session"},
			{Name: "unrelated", Value: "must-not-be-sent"},
		},
	}
	status, body, err := fetchDayaOrders(context.Background(), monitorTestClient(server), session)
	if err != nil || status != http.StatusOK || string(body) != "[]" {
		t.Fatalf("GET /orders result status=%d body=%q err=%v", status, body, err)
	}
}

func TestResumePendingPollingClearsAuthenticationStopAndSchedule(t *testing.T) {
	d := &DayaOrderDispatcher{
		pendingPollingStopped:    true,
		pendingPollingNeedsLogin: true,
		nextPendingPoll:          time.Now().Add(time.Hour),
	}
	d.ResumePendingPolling()
	if d.pendingPollingStopped || d.pendingPollingNeedsLogin {
		t.Fatal("fresh login must resume an authentication-stopped read-only poll")
	}
	if !d.nextPendingPoll.IsZero() {
		t.Fatalf("pending poll schedule was not reset: %v", d.nextPendingPoll)
	}
}

func TestResumePendingPollingDoesNotOverrideRateLimitStop(t *testing.T) {
	d := &DayaOrderDispatcher{
		pendingPollingStopped: true,
		nextPendingPoll:       time.Now().Add(time.Hour),
	}
	d.ResumePendingPolling()
	if !d.pendingPollingStopped {
		t.Fatal("fresh login must not override a 429 rate-limit stop")
	}
	if d.nextPendingPoll.IsZero() {
		t.Fatal("rate-limit stop must retain its stopped state")
	}
}
