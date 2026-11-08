package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSummarizeDayaOrderListResponseCountsOnlySafeStatuses(t *testing.T) {
	body := []byte(`[
		{"orderFunction":"ENTRY","orderStatus":"SENT_TO_TRADING_ENGINE","orderId":123456,"instrumentId":"IRT1FPAF0001","orderPrice":9999,"orderQuantity":12},
		{"orderFunction":"CONFIRMATION","orderStatus":"BOOKED","orderId":123456,"hostOrderNumber":987654,"instrumentId":"IRT1FPAF0001","orderPrice":9999,"orderQuantity":12},
		{"orderFunction":"OTHER","orderStatus":"CANCELLED","accountNumber":"do-not-expose"}
	]`)
	got, err := summarizeDayaOrderListResponse(http.StatusOK, body)
	if err != nil {
		t.Fatal(err)
	}
	if got.HTTPStatus != http.StatusOK || got.RecordCount != 3 || got.EntryCount != 1 || got.BookedCount != 1 || got.OtherCount != 1 {
		t.Fatalf("unexpected safe summary: %#v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"123456", "987654", "IRT1FPAF0001", "9999", "accountNumber", "do-not-expose"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("summary leaked broker data %q: %s", secret, encoded)
		}
	}
}

func TestSummarizeDayaOrderListResponseAcceptsEmptyArray(t *testing.T) {
	got, err := summarizeDayaOrderListResponse(http.StatusOK, []byte("[]"))
	if err != nil {
		t.Fatal(err)
	}
	if got.RecordCount != 0 || got.EntryCount != 0 || got.BookedCount != 0 {
		t.Fatalf("empty response summary = %#v", got)
	}
}

func TestSummarizeDayaOrderListResponseStopsOnAuthenticationAndRateLimit(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: "401/403"},
		{name: "forbidden", status: http.StatusForbidden, want: "401/403"},
		{name: "rate limited", status: http.StatusTooManyRequests, want: "429"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := summarizeDayaOrderListResponse(test.status, []byte(`{"message":"sensitive broker message"}`))
			if err == nil || got != nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("summary=%#v err=%v, want safe stop message %q", got, err, test.want)
			}
			if strings.Contains(err.Error(), "sensitive broker message") {
				t.Fatal("raw broker message leaked into error")
			}
		})
	}
}

func TestSummarizeDayaOrderListResponseRejectsInvalidJSON(t *testing.T) {
	for _, body := range [][]byte{nil, []byte("not-json"), []byte("null"), []byte(strings.Repeat("x", (1<<20)+1))} {
		if _, err := summarizeDayaOrderListResponse(http.StatusOK, body); err == nil {
			t.Fatalf("invalid response accepted: size=%d", len(body))
		}
	}
}

func TestDayaOnlineTestRoutesArePreviewOnly(t *testing.T) {
	mux := http.NewServeMux()
	registerDayaOnlineTestRoutes(mux, false, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/daya-online-test", nil)
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("non-Preview route status = %d, want 404", recorder.Code)
	}
}
