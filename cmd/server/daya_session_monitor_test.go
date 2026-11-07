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
	m.windowCheck = func(time.Time) (bool, time.Duration) { return true, 0 }
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
	m.windowCheck = func(time.Time) (bool, time.Duration) { return true, 0 }
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
	m.windowCheck = func(time.Time) (bool, time.Duration) { return true, 0 }
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
	m.windowCheck = func(time.Time) (bool, time.Duration) { return true, 0 }
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

func TestDayaMonitorWindowUsesTehranTime(t *testing.T) {
	location := tehran()
	tests := []struct {
		name   string
		hour   int
		minute int
		open   bool
	}{
		{name: "before closing boundary", hour: 1, minute: 59, open: true},
		{name: "closing boundary", hour: 2, minute: 0, open: false},
		{name: "before opening boundary", hour: 15, minute: 29, open: false},
		{name: "opening boundary", hour: 15, minute: 30, open: true},
		{name: "evening", hour: 22, minute: 0, open: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Date(2026, time.November, 7, test.hour, test.minute, 0, 0, location)
			open, wait := dayaMonitorWindow(now)
			if open != test.open {
				t.Fatalf("window open=%v, want %v", open, test.open)
			}
			if !open && wait <= 0 {
				t.Fatalf("closed window must return positive wait, got %v", wait)
			}
		})
	}
}

func TestDayaMonitorWaitsForWindowThenChecks(t *testing.T) {
	requests := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/orders" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		requests <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m := NewDayaSessionMonitor(monitorTestClient(server))
	var windowChecks int
	m.windowCheck = func(time.Time) (bool, time.Duration) {
		windowChecks++
		if windowChecks == 1 {
			return false, 60 * time.Millisecond
		}
		return true, 0
	}
	m.interval = func() (time.Duration, error) {
		return time.Millisecond, nil
	}
	if err := m.Start(context.Background(), monitorSession()); err != nil {
		t.Fatal(err)
	}
	defer m.Stop(false, "test done")

	select {
	case <-requests:
		t.Fatal("monitor must not call Daya while the monitoring window is closed")
	case <-time.After(20 * time.Millisecond):
	}

	select {
	case <-requests:
		snapshot := waitForMonitor(t, m, func(s DayaMonitorSnapshot) bool {
			return s.Checks > 0
		})
		if snapshot.LastStatus != http.StatusOK {
			t.Fatalf("unexpected response status: %d", snapshot.LastStatus)
		}
	case <-time.After(time.Second):
		t.Fatal("monitor did not resume checks after the window opened")
	}
}
