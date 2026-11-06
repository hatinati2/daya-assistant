package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDayaLoginUsesCaptchaCookieAndObservedPayload(t *testing.T) {
	const captchaID = "test-captcha-id"
	var loginCookieSeen, payloadSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/captcha":
			if r.Method != http.MethodGet || r.Header.Get("Origin") != dayaWebOrigin {
				t.Fatalf("unexpected CAPTCHA request: %s %s", r.Method, r.URL.Path)
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("captchaId", captchaID)
			http.SetCookie(w, &http.Cookie{Name: "cookiesession1", Value: "captcha-session", Path: "/", HttpOnly: true})
			_, _ = w.Write([]byte("\x89PNG"))
		case "/oms-login":
			loginCookieSeen = strings.Contains(r.Header.Get("Cookie"), "cookiesession1=captcha-session")
			if r.Header.Get("x-client-type") != "WEB_CLIENT" {
				t.Errorf("x-client-type = %q", r.Header.Get("x-client-type"))
			}
			var got map[string]string
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode login payload: %v", err)
			}
			payloadSeen = got["username"] == "user" && got["password"] == "pass" &&
				got["captchaAnswer"] == "12345" && got["captchaId"] == captchaID
			http.SetCookie(w, &http.Cookie{Name: "token", Value: "header.payload.signature", Path: "/", HttpOnly: true, MaxAge: 3600})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{}"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := &DayaLoginClient{HTTP: server.Client(), BaseURL: server.URL}
	challenge, err := client.FetchCaptcha(context.Background())
	if err != nil {
		t.Fatalf("FetchCaptcha: %v", err)
	}
	if challenge.CaptchaID != captchaID || !strings.HasPrefix(challenge.ImageDataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected challenge: %#v", challenge)
	}
	result, err := client.Login(context.Background(), "user", "pass", "12345", captchaID)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.Token != "header.payload.signature" || result.ExpiresAt.Before(time.Now().Add(50*time.Minute)) {
		t.Fatalf("unexpected session result: %#v", result)
	}
	if !loginCookieSeen || !payloadSeen {
		t.Fatalf("captcha cookie=%v payload=%v", loginCookieSeen, payloadSeen)
	}
}

func TestDayaLoginRejectsReusedCaptcha(t *testing.T) {
	client := &DayaLoginClient{}
	_, err := client.Login(context.Background(), "user", "pass", "12345", "unknown")
	if err == nil {
		t.Fatal("expected unknown CAPTCHA challenge to be rejected")
	}
}
