package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func monitorTestClient(server *httptest.Server) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = "http"
		clone.URL.Host = strings.TrimPrefix(server.URL, "http://")
		return http.DefaultTransport.RoundTrip(clone)
	})}
}

func waitForMonitor(t *testing.T, m *DayaSessionMonitor, predicate func(DayaMonitorSnapshot) bool) DayaMonitorSnapshot {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot := m.Snapshot()
		if predicate(snapshot) {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("monitor condition not met; snapshot=%#v", m.Snapshot())
	return DayaMonitorSnapshot{}
}

func monitorSession() DayaSession {
	return DayaSession{
		Token:     "test-token",
		ExpiresAt: time.Now().Add(time.Hour),
		Cookies: []DayaSessionCookie{
			{Name: "token", Value: "test-token"},
			{Name: "cookiesession1", Value: "captcha-session"},
		},
	}
}

func TestDayaRandomIntervalBounds(t *testing.T) {
	for i := 0; i < 100; i++ {
		d, err := dayaRandomInterval()
		if err != nil {
			t.Fatal(err)
		}
		if d < time.Minute || d > 7*time.Minute {
			t.Fatalf("interval out of bounds: %v", d)
		}
	}
}

func TestDayaMonitorSendsBothCookiesAndStopsOnUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/orders" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if c, err := r.Cookie("token"); err != nil || c.Value != "test-token" {
			t.Error("token cookie missing")
		}
		if c, err := r.Cookie("cookiesession1"); err != nil || c.Value != "captcha-session" {
			t.Error("captcha session cookie missing")
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	m := NewDayaSessionMonitor(monitorTestClient(server))
	m.interval = func() (time.Duration, error) {
		return time.Millisecond, nil
	}
	if err := m.Start(context.Background(), monitorSession()); err != nil {
		t.Fatal(err)
	}
	snapshot := waitForMonitor(t, m, func(s DayaMonitorSnapshot) bool {
		return s.NeedsLogin
	})
	if snapshot.LastStatus != http.StatusUnauthorized || snapshot.Running {
		t.Fatalf("unexpected 401 state: %#v", snapshot)
	}
}

func TestDayaMonitorPersistsRotatedToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name: "token", Value: "rotated-token", Path: "/", MaxAge: 1800, HttpOnly: true,
		})
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	saved := make(chan DayaSession, 1)
	m := NewDayaSessionMonitor(monitorTestClient(server))
	m.interval = func() (time.Duration, error) {
		return time.Millisecond, nil
	}
	m.SetSessionUpdater(func(session DayaSession) error {
		saved <- session
		return nil
	})
	if err := m.Start(context.Background(), monitorSession()); err != nil {
		t.Fatal(err)
	}
	select {
	case session := <-saved:
		m.Stop(false, "test done")
		if session.Token != "rotated-token" ||
			session.ExpiresAt.Before(time.Now().Add(29*time.Minute)) ||
			session.ExpiresAt.After(time.Now().Add(31*time.Minute)) {
			t.Fatalf("rotated session not saved: token=%q expiry=%v", session.Token, session.ExpiresAt)
		}
	case <-time.After(time.Second):
		t.Fatal("session rotation was not persisted")
	}
}

func TestDayaMonitorDoesNotExtendExpiryWithoutCookieRotation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	saved := make(chan DayaSession, 1)
	m := NewDayaSessionMonitor(monitorTestClient(server))
	m.interval = func() (time.Duration, error) {
		return time.Millisecond, nil
	}
	m.SetSessionUpdater(func(session DayaSession) error {
		saved <- session
		return nil
	})
	if err := m.Start(context.Background(), monitorSession()); err != nil {
		t.Fatal(err)
	}
	waitForMonitor(t, m, func(s DayaMonitorSnapshot) bool {
		return s.Checks > 0
	})
	select {
	case <-saved:
		t.Fatal("unchanged session must not be persisted or extended")
	default:
	}
	m.Stop(false, "test done")
}

func TestDayaMonitorStopsOnRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	m := NewDayaSessionMonitor(monitorTestClient(server))
	m.interval = func() (time.Duration, error) {
		return time.Millisecond, nil
	}
	if err := m.Start(context.Background(), monitorSession()); err != nil {
		t.Fatal(err)
	}
	snapshot := waitForMonitor(t, m, func(s DayaMonitorSnapshot) bool {
		return !s.Running && s.LastStatus == http.StatusTooManyRequests
	})
	if snapshot.NeedsLogin {
		t.Fatal("429 should stop monitor without declaring expired login")
	}
}

func TestDayaApplySessionCookiesIgnoresUnrelatedCookies(t *testing.T) {
	session := monitorSession()
	changed, err := dayaApplySessionCookies(
		&session,
		[]*http.Cookie{{Name: "unrelated", Value: "private"}},
		time.Now(),
	)
	if err != nil || changed || len(session.Cookies) != 2 {
		t.Fatalf("unexpected unrelated cookie handling: changed=%v err=%v", changed, err)
	}
}

func TestDayaMonitorPollsRepeatedlyWithoutTimeOfDayGate(t *testing.T) {
	requests := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/orders" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		requests <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m := NewDayaSessionMonitor(monitorTestClient(server))
	m.interval = func() (time.Duration, error) {
		return 5 * time.Millisecond, nil
	}
	if err := m.Start(context.Background(), monitorSession()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(false, "test done")

	for i := 0; i < 2; i++ {
		select {
		case <-requests:
		case <-time.After(time.Second):
			t.Fatalf("monitor did not perform polling request %d", i+1)
		}
	}
	snapshot := waitForMonitor(t, m, func(s DayaMonitorSnapshot) bool {
		return s.Checks >= 2
	})
	if snapshot.LastStatus != http.StatusOK || !snapshot.Running {
		t.Fatalf("unexpected monitor state after repeated polling: %#v", snapshot)
	}
}

func TestVerifyCurrentEndpointsCallsEachVerifiedGETOnceAndPersistsRotation(t *testing.T) {
	requestCounts := map[string]int{}
	saved := make(chan DayaSession, 2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCounts[r.URL.Path]++
		if r.Method != http.MethodGet {
			t.Errorf("%s method = %s, want GET", r.URL.Path, r.Method)
		}
		token, _ := r.Cookie("token")
		if token == nil || token.Value == "" {
			t.Errorf("%s request did not include token cookie", r.URL.Path)
		}
		switch r.URL.Path {
		case "/orders":
			if token != nil && token.Value != "test-token" {
				t.Errorf("/orders token = %q, want initial token", token.Value)
			}
			http.SetCookie(w, &http.Cookie{
				Name: "token", Value: "rotated-token", Path: "/", MaxAge: 1800, HttpOnly: true,
			})
			w.WriteHeader(http.StatusOK)
		case "/financials/portfolio/instant":
			if token != nil && token.Value != "rotated-token" {
				t.Errorf("portfolio token = %q, want rotated token from first response", token.Value)
			}
			http.SetCookie(w, &http.Cookie{
				Name: "cookiesession1", Value: "rotated-session-cookie", Path: "/", MaxAge: 3600,
			})
			w.WriteHeader(http.StatusPartialContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	m := NewDayaSessionMonitor(monitorTestClient(server))
	m.SetSessionUpdater(func(session DayaSession) error {
		saved <- session
		return nil
	})
	updated, report := m.VerifyCurrentEndpoints(context.Background(), monitorSession())

	if len(report.Results) != 2 {
		t.Fatalf("result count = %d, want exactly 2: %#v", len(report.Results), report.Results)
	}
	if requestCounts["/orders"] != 1 || requestCounts["/financials/portfolio/instant"] != 1 {
		t.Fatalf("request counts = %#v, want exactly one GET per endpoint", requestCounts)
	}
	wantPaths := []string{"/orders", "/financials/portfolio/instant"}
	wantStatuses := []int{http.StatusOK, http.StatusPartialContent}
	for i, result := range report.Results {
		if result.Method != http.MethodGet || result.Path != wantPaths[i] {
			t.Errorf("result[%d] = %#v, want GET %s", i, result, wantPaths[i])
		}
		if result.StatusCode != wantStatuses[i] {
			t.Errorf("result[%d] status = %d, want %d", i, result.StatusCode, wantStatuses[i])
		}
		if result.LatencyMS < 0 {
			t.Errorf("result[%d] latency_ms = %f, want non-negative", i, result.LatencyMS)
		}
		if !result.SetCookiePresent || result.SetCookieCount != 1 {
			t.Errorf("result[%d] Set-Cookie telemetry = %#v, want one header", i, result)
		}
		if !result.SessionRotated || !result.SessionPersisted {
			t.Errorf("result[%d] rotation should be validated and persisted: %#v", i, result)
		}
	}
	if updated.Token != "rotated-token" {
		t.Fatalf("updated token = %q, want rotated token", updated.Token)
	}
	foundSessionCookie := false
	for _, cookie := range updated.Cookies {
		if cookie.Name == "cookiesession1" && cookie.Value == "rotated-session-cookie" {
			foundSessionCookie = true
		}
	}
	if !foundSessionCookie {
		t.Fatalf("rotated cookiesession1 not present in updated session: %#v", updated.Cookies)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-saved:
		case <-time.After(time.Second):
			t.Fatalf("session update %d was not persisted", i+1)
		}
	}
}

func TestVerifyCurrentEndpointsAttemptsBothPathsAfterFirstEndpointError(t *testing.T) {
	requestCounts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCounts[r.URL.Path]++
		if r.URL.Path == "/orders" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if r.URL.Path == "/financials/portfolio/instant" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	m := NewDayaSessionMonitor(monitorTestClient(server))
	_, report := m.VerifyCurrentEndpoints(context.Background(), monitorSession())
	if requestCounts["/orders"] != 1 || requestCounts["/financials/portfolio/instant"] != 1 {
		t.Fatalf("request counts = %#v, want one request to both endpoints", requestCounts)
	}
	if len(report.Results) != 2 {
		t.Fatalf("result count = %d, want 2", len(report.Results))
	}
	if report.Results[0].StatusCode != http.StatusBadGateway || report.Results[1].StatusCode != http.StatusOK {
		t.Fatalf("statuses = %d, %d; want 502, 200", report.Results[0].StatusCode, report.Results[1].StatusCode)
	}
}

func TestVerifyCurrentEndpointsDoesNotFollowRedirects(t *testing.T) {
	requestCounts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCounts[r.URL.Path]++
		if r.URL.Path == "/orders" {
			http.Redirect(w, r, "/orders-redirect-target", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m := NewDayaSessionMonitor(monitorTestClient(server))
	_, report := m.VerifyCurrentEndpoints(context.Background(), monitorSession())
	if requestCounts["/orders"] != 1 || requestCounts["/orders-redirect-target"] != 0 {
		t.Fatalf("redirect request counts = %#v, want no redirect follow", requestCounts)
	}
	if len(report.Results) != 2 || report.Results[0].StatusCode != http.StatusFound {
		t.Fatalf("redirect result = %#v, want HTTP 302 returned as-is", report.Results)
	}
}

func TestVerifyCurrentEndpointsDoesNotPersistWhenNoSetCookieIsReturned(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	saved := false
	m := NewDayaSessionMonitor(monitorTestClient(server))
	m.SetSessionUpdater(func(DayaSession) error {
		saved = true
		return nil
	})
	_, report := m.VerifyCurrentEndpoints(context.Background(), monitorSession())

	if requests != 2 {
		t.Fatalf("request count = %d, want exactly 2", requests)
	}
	if len(report.Results) != 2 {
		t.Fatalf("result count = %d, want 2", len(report.Results))
	}
	for _, result := range report.Results {
		if result.StatusCode != http.StatusOK {
			t.Errorf("%s status = %d, want 200", result.Path, result.StatusCode)
		}
		if result.SetCookiePresent || result.SetCookieCount != 0 {
			t.Errorf("%s unexpectedly reports Set-Cookie: %#v", result.Path, result)
		}
		if result.SessionRotated || result.SessionPersisted {
			t.Errorf("%s unexpectedly reports session rotation: %#v", result.Path, result)
		}
	}
	if saved {
		t.Fatal("session updater must not be called when no cookie was rotated")
	}
}
