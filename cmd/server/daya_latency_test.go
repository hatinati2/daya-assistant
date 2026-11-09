package main

import "testing"

func TestSummarizeDayaLatencyUsesSuccessfulSamplesOnly(t *testing.T) {
	got := summarizeDayaLatency([]dayaLatencySample{
		{HTTPStatus: 403, RTTMillis: 100},
		{HTTPStatus: 401, RTTMillis: 200},
		{HTTPStatus: 0, RTTMillis: 0, Error: "timeout"},
	})
	if got.Count != 2 || got.MinRTTMillis != 100 || got.MedianRTTMillis != 150 || got.MeanRTTMillis != 150 || got.EstimatedOneWayMS != 75 {
		t.Fatalf("unexpected summary: %#v", got)
	}
	if got.P95RTTMillis != 200 {
		t.Fatalf("p95=%d, want 200", got.P95RTTMillis)
	}
}

func TestSummarizeDayaLatencyEmptySamples(t *testing.T) {
	got := summarizeDayaLatency(nil)
	if got.Count != 0 || got.EstimatedOneWayMS != 0 {
		t.Fatalf("unexpected empty summary: %#v", got)
	}
}


func TestSummarizeDayaLatencySeparatesHTTPResponsesFromAuthenticatedSuccess(t *testing.T) {
	got := summarizeDayaLatency([]dayaLatencySample{
		{Mode: "authenticated", HTTPStatus: 200, RTTMillis: 120, ResponseValid: true, Authenticated: true},
		{Mode: "authenticated", HTTPStatus: 401, RTTMillis: 180, Error: "rejected"},
		{Mode: "authenticated", HTTPStatus: 0, RTTMillis: 0, Error: "timeout"},
	})
	if got.Count != 2 || got.SuccessCount != 1 || got.RejectedCount != 1 || got.ErrorCount != 2 {
		t.Fatalf("unexpected status counts: %#v", got)
	}
	if got.MedianRTTMillis != 150 || got.EstimatedOneWayMS != 75 {
		t.Fatalf("unexpected latency summary: %#v", got)
	}
}

func TestProbeAuthenticatedDayaLatencyUsesReadOnlySessionCookie(t *testing.T) {
	var method, cookieValue string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		cookie, err := r.Cookie("token")
		if err == nil {
			cookieValue = cookie.Value
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	session := DayaSession{
		Token:     "fake-session-token",
		ExpiresAt: time.Now().Add(time.Hour),
		Cookies: []DayaSessionCookie{
			{Name: "token", Value: "fake-session-token", Path: "/"},
		},
	}
	got := probeDayaLatencyAuthenticatedAt(context.Background(), server.URL, &session, nil)
	if got.HTTPStatus != http.StatusOK || !got.ResponseValid || !got.Authenticated {
		t.Fatalf("authenticated read-only probe failed: %#v", got)
	}
	if method != http.MethodGet {
		t.Fatalf("method=%q, want GET", method)
	}
	if cookieValue != "fake-session-token" {
		t.Fatalf("token cookie was not sent")
	}
}

func TestProbeAuthenticatedDayaLatencyMarksUnauthorizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Not Authenticated"}`))
	}))
	defer server.Close()

	session := DayaSession{
		Token:     "fake-session-token",
		ExpiresAt: time.Now().Add(time.Hour),
		Cookies: []DayaSessionCookie{
			{Name: "token", Value: "fake-session-token", Path: "/"},
		},
	}
	got := probeDayaLatencyAuthenticatedAt(context.Background(), server.URL, &session, nil)
	if got.HTTPStatus != http.StatusUnauthorized || got.Authenticated || got.ResponseValid {
		t.Fatalf("401 must not count as authenticated: %#v", got)
	}
}
