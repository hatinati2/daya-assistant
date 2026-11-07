package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	dayaWebOrigin = "https://dayatrader.ir"
	dayaAPIOrigin = "https://mg.dayatrader.ir"
)

type DayaCaptchaChallenge struct {
	CaptchaID    string `json:"captcha_id"`
	ImageDataURL string `json:"image_data_url"`
}

type DayaLoginResult struct {
	Token string
	ExpiresAt time.Time
	Cookies []DayaSessionCookie
}

type dayaCaptchaState struct {
	Cookie string
	Session DayaSessionCookie
	ExpiresAt time.Time
}

type DayaLoginClient struct {
	HTTP    *http.Client
	BaseURL string

	mu         sync.Mutex
	challenges map[string]dayaCaptchaState
}

func (c *DayaLoginClient) baseURL() string {
	if strings.TrimSpace(c.BaseURL) != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return dayaAPIOrigin
}

func (c *DayaLoginClient) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (c *DayaLoginClient) FetchCaptcha(ctx context.Context) (DayaCaptchaChallenge, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL()+"/captcha", nil)
	if err != nil {
		return DayaCaptchaChallenge{}, errors.New("ساخت درخواست CAPTCHA دایا ناموفق بود")
	}
	req.Header.Set("Accept", "image/png,image/*;q=0.8,*/*;q=0.5")
	req.Header.Set("Origin", dayaWebOrigin)
	req.Header.Set("Referer", dayaWebOrigin+"/login")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return DayaCaptchaChallenge{}, errors.New("ارتباط با سرویس CAPTCHA دایا برقرار نشد")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return DayaCaptchaChallenge{}, fmt.Errorf("دریافت CAPTCHA دایا با HTTP %d ناموفق بود", resp.StatusCode)
	}
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "image/png") {
		return DayaCaptchaChallenge{}, errors.New("پاسخ CAPTCHA دایا تصویر PNG نبود")
	}
	captchaID := strings.TrimSpace(resp.Header.Get("captchaId"))
	if captchaID == "" || len(captchaID) > 128 {
		return DayaCaptchaChallenge{}, errors.New("شناسه CAPTCHA در پاسخ دایا وجود ندارد")
	}
	imageBytes, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return DayaCaptchaChallenge{}, errors.New("خواندن تصویر CAPTCHA دایا ناموفق بود")
	}
	if len(imageBytes) == 0 || len(imageBytes) >= 256<<10 {
		return DayaCaptchaChallenge{}, errors.New("اندازه تصویر CAPTCHA دایا نامعتبر است")
	}
	var sessionCookie *http.Cookie
	for _, cookie := range resp.Cookies() {
		if strings.EqualFold(cookie.Name, "cookiesession1") {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		return DayaCaptchaChallenge{}, errors.New("کوکی نشست CAPTCHA در پاسخ دایا وجود ندارد")
	}
	sessionCookieData, ok := dayaSessionCookieFromHTTP(sessionCookie)
	if !ok { return DayaCaptchaChallenge{}, errors.New("کوکی CAPTCHA دایا معتبر نیست") }
	cookieHeader := sessionCookie.Name + "=" + sessionCookie.Value
	c.mu.Lock()
	if c.challenges == nil {
		c.challenges = make(map[string]dayaCaptchaState)
	}
	now := time.Now()
	for id, state := range c.challenges {
		if !state.ExpiresAt.After(now) {
			delete(c.challenges, id)
		}
	}
	if len(c.challenges) >= 32 {
		for id := range c.challenges {
			delete(c.challenges, id)
			break
		}
	}
	c.challenges[captchaID] = dayaCaptchaState{Cookie: cookieHeader, Session: sessionCookieData, ExpiresAt: now.Add(4 * time.Minute)}
	c.mu.Unlock()
	return DayaCaptchaChallenge{
		CaptchaID:    captchaID,
		ImageDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes),
	}, nil
}

func (c *DayaLoginClient) Login(ctx context.Context, username, password, captchaAnswer, captchaID string) (DayaLoginResult, error) {
	username = strings.TrimSpace(username)
	captchaAnswer = strings.TrimSpace(captchaAnswer)
	captchaID = strings.TrimSpace(captchaID)
	if username == "" || password == "" || captchaAnswer == "" || captchaID == "" {
		return DayaLoginResult{}, errors.New("نام کاربری، رمز عبور، شناسه و پاسخ CAPTCHA الزامی است")
	}
	if len(username) > 200 || len(password) > 1024 || len(captchaAnswer) > 12 || len(captchaID) > 128 {
		return DayaLoginResult{}, errors.New("اطلاعات ورود دایا معتبر نیست")
	}

	c.mu.Lock()
	state, ok := c.challenges[captchaID]
	if ok {
		delete(c.challenges, captchaID)
	}
	c.mu.Unlock()
	if !ok || !state.ExpiresAt.After(time.Now()) {
		return DayaLoginResult{}, errors.New("تصویر CAPTCHA منقضی یا قبلاً مصرف شده است؛ تصویر تازه دریافت کن")
	}

	payload, err := json.Marshal(struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		CaptchaAnswer string `json:"captchaAnswer"`
		CaptchaID     string `json:"captchaId"`
	}{username, password, captchaAnswer, captchaID})
	if err != nil {
		return DayaLoginResult{}, errors.New("ساخت درخواست ورود دایا ناموفق بود")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+"/oms-login", bytes.NewReader(payload))
	if err != nil {
		return DayaLoginResult{}, errors.New("ساخت درخواست ورود دایا ناموفق بود")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", dayaWebOrigin)
	req.Header.Set("Referer", dayaWebOrigin+"/login")
	req.Header.Set("x-client-type", "WEB_CLIENT")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Cookie", state.Cookie)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return DayaLoginResult{}, errors.New("ارتباط با درگاه ورود دایا برقرار نشد")
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return DayaLoginResult{}, errors.New("خواندن پاسخ ورود دایا ناموفق بود")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DayaLoginResult{}, dayaLoginResponseError(resp.StatusCode, body)
	}
	var tokenCookie *http.Cookie
	cookies := []DayaSessionCookie{state.Session}
	for _, cookie := range resp.Cookies() {
		safeCookie, valid := dayaSessionCookieFromHTTP(cookie)
		if !valid { continue }
		switch safeCookie.Name {
		case "cookiesession1":
			replaced := false
			for i := range cookies { if cookies[i].Name == "cookiesession1" { cookies[i] = safeCookie; replaced = true } }
			if !replaced { cookies = append(cookies, safeCookie) }
		case "token":
			tokenCookie = cookie
			replaced := false
			for i := range cookies { if cookies[i].Name == "token" { cookies[i] = safeCookie; replaced = true } }
			if !replaced { cookies = append(cookies, safeCookie) }
		}
	}
	if tokenCookie == nil {
		return DayaLoginResult{}, errors.New("پاسخ ورود موفق بود اما کوکی نشست token دریافت نشد")
	}
	expiresAt := time.Time{}
	switch {
	case tokenCookie.MaxAge > 0:
		expiresAt = time.Now().Add(time.Duration(tokenCookie.MaxAge) * time.Second)
	case !tokenCookie.Expires.IsZero():
		expiresAt = tokenCookie.Expires
	default:
		expiresAt = dayaJWTExpiry(tokenCookie.Value)
	}
	if expiresAt.IsZero() || !expiresAt.After(time.Now()) {
		return DayaLoginResult{}, errors.New("زمان اعتبار کوکی نشست دایا قابل تشخیص نیست")
	}
	return DayaLoginResult{Token: tokenCookie.Value, ExpiresAt: expiresAt, Cookies: cookies}, nil
}

func dayaLoginResponseError(status int, body []byte) error {
	var payload map[string]json.RawMessage
	message := ""
	if json.Unmarshal(body, &payload) == nil {
		for _, key := range []string{"message", "msg", "error"} {
			if raw := payload[key]; len(raw) != 0 {
				var value string
				if json.Unmarshal(raw, &value) == nil && value != "" {
					message = value
					break
				}
			}
		}
	}
	if message != "" {
		return fmt.Errorf("دایا ورود را نپذیرفت (HTTP %d): %s", status, message)
	}
	if status == http.StatusUnauthorized || status == http.StatusBadRequest {
		return fmt.Errorf("دایا ورود را نپذیرفت (HTTP %d)؛ نام کاربری، رمز عبور و CAPTCHA را بررسی کن", status)
	}
	return fmt.Errorf("پاسخ درگاه ورود دایا ناموفق بود (HTTP %d)", status)
}

func dayaJWTExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		ExpiresAt int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.ExpiresAt <= 0 {
		return time.Time{}
	}
	return time.Unix(claims.ExpiresAt, 0)
}
