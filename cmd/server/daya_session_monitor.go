package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	dayaOrdersURL  = dayaAPIOrigin + "/orders"
	dayaMonitorMin = time.Minute
	dayaMonitorMax = 7 * time.Minute
)

type DayaMonitorSnapshot struct {
	Running    bool      `json:"running"`
	NeedsLogin bool      `json:"needs_login"`
	LastCheck  time.Time `json:"last_check,omitempty"`
	NextCheck  time.Time `json:"next_check,omitempty"`
	LastStatus int       `json:"last_status,omitempty"`
	Checks     uint64    `json:"checks"`
	LastError  string    `json:"last_error,omitempty"`
}

type DayaSessionMonitor struct {
	mu       sync.Mutex
	http     *http.Client
	state    DayaMonitorSnapshot
	cancel   context.CancelFunc
	interval func() (time.Duration, error)
	update   func(DayaSession) error
}

func NewDayaSessionMonitor(client *http.Client) *DayaSessionMonitor {
	if client == nil {
		client = &http.Client{
			Timeout: 20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &DayaSessionMonitor{http: client, interval: dayaRandomInterval}
}

func (m *DayaSessionMonitor) SetSessionUpdater(update func(DayaSession) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.update = update
}

func dayaRandomInterval() (time.Duration, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	n := binary.BigEndian.Uint64(b[:])
	span := uint64((dayaMonitorMax-dayaMonitorMin)/time.Second) + 1
	return dayaMonitorMin + time.Duration(n%span)*time.Second, nil
}

func (m *DayaSessionMonitor) Start(parent context.Context, session DayaSession) error {
	if session.Token == "" {
		return errors.New("نشست دایا خالی است")
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(time.Now()) {
		return errors.New("نشست دایا منقضی شده است")
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.state = DayaMonitorSnapshot{Running: true}
	m.mu.Unlock()
	go m.run(ctx, session)
	return nil
}

func (m *DayaSessionMonitor) Stop(needsLogin bool, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.state.Running = false
	m.state.NeedsLogin = needsLogin
	m.state.NextCheck = time.Time{}
	m.state.LastError = message
}

func (m *DayaSessionMonitor) Snapshot() DayaMonitorSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *DayaSessionMonitor) run(ctx context.Context, session DayaSession) {
	for {
		delay, err := m.interval()
		if err != nil {
			m.Stop(false, "تولید فاصله بررسی ناموفق بود")
			return
		}
		if !session.ExpiresAt.IsZero() {
			untilExpiry := time.Until(session.ExpiresAt)
			if untilExpiry <= 0 {
				m.Stop(true, "زمان اعتبار نشست به پایان رسیده است؛ ورود دستی لازم است")
				return
			}
			if delay <= 0 || untilExpiry < delay {
				delay = untilExpiry
			}
		}
		m.mu.Lock()
		m.state.NextCheck = time.Now().Add(delay)
		m.mu.Unlock()
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if !session.ExpiresAt.IsZero() && !time.Now().Before(session.ExpiresAt) {
			m.Stop(true, "زمان اعتبار نشست به پایان رسیده است؛ ورود دستی لازم است")
			return
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, dayaOrdersURL, nil)
		if err != nil {
			m.Stop(false, "ساخت درخواست بررسی نشست ناموفق بود")
			return
		}
		req.Header.Set("Accept", "application/json")
		for _, cookie := range session.Cookies {
			if (cookie.Name == "token" || cookie.Name == "cookiesession1") && cookie.Value != "" {
				req.AddCookie(cookie.httpCookie())
			}
		}
		resp, err := m.http.Do(req)
		now := time.Now().UTC()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.mu.Lock()
			m.state.LastCheck = now
			m.state.LastError = "خطای ارتباط؛ بررسی بعدی با فاصله تصادفی انجام می‌شود"
			m.state.Checks++
			m.mu.Unlock()
			continue
		}

		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		rotated, rotateErr := dayaApplySessionCookies(&session, resp.Cookies(), now)
		m.mu.Lock()
		m.state.LastCheck = now
		m.state.LastStatus = resp.StatusCode
		m.state.Checks++
		m.state.LastError = ""
		update := m.update
		m.mu.Unlock()
		if rotateErr != nil {
			m.Stop(true, "کوکی بازگشتی از دایا معتبر نبود؛ ورود دستی لازم است")
			return
		}
		if rotated && update != nil {
			if err := update(session); err != nil {
				m.Stop(false, "ذخیره امن تغییر نشست انجام نشد؛ پایش متوقف شد")
				return
			}
		}

		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			m.Stop(true, "نشست توسط دایا پذیرفته نشد؛ ورود دستی با CAPTCHA لازم است")
			return
		case http.StatusTooManyRequests:
			m.Stop(false, "دایا محدودیت درخواست اعلام کرد؛ پایش متوقف شد")
			return
		}
		if resp.StatusCode >= 500 {
			m.mu.Lock()
			m.state.LastError = "خطای موقت سرویس دایا"
			m.mu.Unlock()
		} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			m.mu.Lock()
			m.state.LastError = "پاسخ غیرمنتظره از سرویس دایا"
			m.mu.Unlock()
		}
	}
}

func dayaApplySessionCookies(session *DayaSession, received []*http.Cookie, now time.Time) (bool, error) {
	changed := false
	for _, raw := range received {
		cookie, ok := dayaSessionCookieFromHTTP(raw)
		if !ok {
			continue
		}
		found := false
		for i := range session.Cookies {
			if session.Cookies[i].Name != cookie.Name {
				continue
			}
			found = true
			if session.Cookies[i] != cookie {
				session.Cookies[i] = cookie
				changed = true
			}
			break
		}
		if !found {
			session.Cookies = append(session.Cookies, cookie)
			changed = true
		}
		if cookie.Name == "token" {
			if cookie.Value != session.Token {
				session.Token = cookie.Value
				changed = true
			}
			if cookie.MaxAge > 0 {
				newExpiry := now.Add(time.Duration(cookie.MaxAge) * time.Second)
				if newExpiry.After(now) && !newExpiry.Equal(session.ExpiresAt) {
					session.ExpiresAt = newExpiry
					changed = true
				}
			} else if !cookie.Expires.IsZero() && cookie.Expires.After(now) && !cookie.Expires.Equal(session.ExpiresAt) {
				session.ExpiresAt = cookie.Expires
				changed = true
			} else if jwtExpiry := dayaJWTExpiry(cookie.Value); jwtExpiry.After(now) && jwtExpiry.After(session.ExpiresAt) {
				session.ExpiresAt = jwtExpiry
				changed = true
			}
		}
	}
	if session.Token == "" || session.ExpiresAt.IsZero() || !session.ExpiresAt.After(now) {
		return changed, errors.New("session token or expiry is invalid")
	}
	return changed, nil
}
