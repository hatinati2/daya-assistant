package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// postDayaOrderToTestServer exercises the observed POST /orders wire shape
// against a loopback-only test server. It refuses every non-loopback
// destination, so it cannot submit a live broker order.
func postDayaOrderToTestServer(ctx context.Context, client *http.Client, baseURL string, order Order) (int, []byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() == "" {
		return 0, nil, errors.New("آدرس تست باید یک HTTP loopback معتبر باشد")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return 0, nil, errors.New("ارسال سفارش فقط به سرور آزمایشی محلی مجاز است")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && parsed.Path != "/" {
		return 0, nil, errors.New("آدرس سرور آزمایشی نامعتبر است")
	}
	payload, err := MarshalDayaOrderPayload(order)
	if err != nil {
		return 0, nil, err
	}
	endpoint := strings.TrimRight(parsed.String(), "/") + "/orders"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, fmt.Errorf("ساخت درخواست آزمایشی: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if client == nil {
		client = &http.DefaultClient
	}
	// Clone the supplied client so this helper cannot mutate shared client
	// configuration, and refuse redirects to keep the loopback boundary intact.
	safeClient := *client
	safeClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if safeClient.Timeout <= 0 {
		safeClient.Timeout = 5 * time.Second
	}
	resp, err := safeClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("ارسال به سرور آزمایشی محلی: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("خواندن پاسخ سرور آزمایشی: %w", err)
	}
	if len(body) > 64<<10 {
		return resp.StatusCode, nil, errors.New("پاسخ سرور آزمایشی بیش از حد مجاز است")
	}
	return resp.StatusCode, body, nil
}
