package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// DayaOrderDispatcher is the server-side worker for API-mode orders. The worker
// is wired into the Preview/Production process, but the live capability gate
// remains fail-closed in daya_order.go until the broker contract is verified.
type DayaOrderDispatcher struct {
	store       *Store
	credentials *DayaCredentialStore
	http        *http.Client
	liveEnabled func() bool
	now         func() time.Time
}

func NewDayaOrderDispatcher(store *Store, credentials *DayaCredentialStore) *DayaOrderDispatcher {
	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			// Never forward an authenticated order request to a redirect target.
			return http.ErrUseLastResponse
		},
	}
	return &DayaOrderDispatcher{
		store: store, credentials: credentials, http: client,
		liveEnabled: dayaOrderAPICapable,
		now:         func() time.Time { return time.Now().In(tehran()) },
	}
}

// Run checks for due API orders. It does not poll the broker while live dispatch
// is disabled; no broker POST can occur through this worker with the current
// dayaOrderAPICapable implementation.
func (d *DayaOrderDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.dispatchReady(ctx); err != nil {
				log.Printf("Daya order dispatcher: %v", err)
			}
		}
	}
}

func (d *DayaOrderDispatcher) dispatchReady(ctx context.Context) error {
	if d == nil || d.liveEnabled == nil || !d.liveEnabled() {
		return nil
	}
	if d.store == nil || d.credentials == nil {
		return fmt.Errorf("Daya order dispatcher is missing its store or credential store")
	}

	session, _, err := d.credentials.LoadAccessSession()
	if err != nil {
		return fmt.Errorf("could not load the encrypted Daya session")
	}
	if session.Token == "" {
		return fmt.Errorf("no authenticated Daya session is available")
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(d.now()) {
		return fmt.Errorf("the Daya session has expired; manual login is required")
	}

	orders, err := d.store.claimReadyAPIOrders(d.now())
	if err != nil {
		return fmt.Errorf("could not claim due API orders: %w", err)
	}
	for _, order := range orders {
		status, body, submitErr := postDayaOrder(ctx, d.http, session, order)
		outcome := DayaOrderOutcome{Status: statusSubmissionUnknown, Message: "نتیجه ارسال مشخص نیست؛ وضعیت سفارش را در دایا بررسی کنید و ارسال مجدد خودکار انجام نمی‌شود.", ReceivedAt: d.now()}
		if submitErr != nil {
			// A transport error can happen after the broker received the request.
			// Treat it as ambiguous and never retry automatically.
			outcome.Message = "ارتباط هنگام ارسال قطع شد؛ ممکن است سفارش در دایا ثبت شده باشد. قبل از هر اقدام، وضعیت را در کارگزاری بررسی کنید."
		} else {
			outcome = classifyDayaOrderResponseForOrder(status, body, &order)
			outcome.ReceivedAt = d.now()
		}
		if err := d.store.completeAPIOrder(order.ID, outcome); err != nil {
			log.Printf("Daya order state update failed for local order %q: %v", order.ID, err)
		}
	}
	return nil
}

func newDayaOrderRequest(ctx context.Context, session DayaSession, order Order) (*http.Request, error) {
	if session.Token == "" {
		return nil, fmt.Errorf("نشست معتبر دایا موجود نیست")
	}
	payload, err := MarshalDayaOrderPayload(order)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dayaOrderEndpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("ساخت درخواست سفارش دایا ناموفق بود")
	}

	cookieValues := make([]string, 0, 2)
	seen := make(map[string]bool, 2)
	for _, cookie := range session.Cookies {
		if cookie.Name != "token" && cookie.Name != "cookiesession1" {
			continue
		}
		if cookie.Name == "token" {
			cookie.Value = session.Token
		}
		if cookie.Value == "" || seen[cookie.Name] {
			continue
		}
		seen[cookie.Name] = true
		cookieValues = append(cookieValues, (&http.Cookie{Name: cookie.Name, Value: cookie.Value}).String())
	}
	if !seen["token"] {
		cookieValues = append(cookieValues, (&http.Cookie{Name: "token", Value: session.Token}).String())
	}
	req.Header.Set("Cookie", strings.Join(cookieValues, "; "))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", dayaWebOrigin)
	req.Header.Set("Referer", dayaWebOrigin+"/")
	req.Header.Set("x-client-type", "WEB_CLIENT")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	return req, nil
}

func postDayaOrder(ctx context.Context, client *http.Client, session DayaSession, order Order) (int, []byte, error) {
	req, err := newDayaOrderRequest(ctx, session, order)
	if err != nil {
		return 0, nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	safeClient := *client
	safeClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := safeClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("درخواست سفارش دایا به نتیجه قطعی نرسید")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 {
		return resp.StatusCode, nil, fmt.Errorf("خواندن پاسخ سفارش دایا کامل نشد")
	}
	return resp.StatusCode, body, nil
}

// dayaOrderConfirmationRecord is the observed order-history/confirmation record.
// It may be returned directly or as an array. It is not assumed to be the
// immediate POST response unless it matches the exact submitted order.
type dayaOrderConfirmationRecord struct {
	OrderFunction      string  `json:"orderFunction"`
	OrderStatus        string  `json:"orderStatus"`
	OrderID            int64   `json:"orderId"`
	HostOrderNumber    int64   `json:"hostOrderNumber"`
	InstrumentID       string  `json:"instrumentId"`
	OrderPrice         int64   `json:"orderPrice"`
	OrderQuantity      int64   `json:"orderQuantity"`
	OrderSide          string  `json:"orderSide"`
	OrderValidityType  string  `json:"orderValidityType"`
	OrderValidityDate  *string `json:"orderValidityDate"`
}

// decodeDayaConfirmationRecords accepts only the observed direct object or
// array shape; unknown envelopes remain ambiguous rather than being guessed.
func decodeDayaConfirmationRecords(body []byte) []dayaOrderConfirmationRecord {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var records []dayaOrderConfirmationRecord
		if json.Unmarshal(body, &records) == nil {
			return records
		}
		return nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var record dayaOrderConfirmationRecord
		if json.Unmarshal(body, &record) == nil {
			return []dayaOrderConfirmationRecord{record}
		}
	}
	return nil
}

func dayaConfirmationMatches(record dayaOrderConfirmationRecord, expected *Order) bool {
	if expected == nil || record.OrderFunction != "CONFIRMATION" ||
		record.OrderStatus != "BOOKED" || record.OrderID <= 0 ||
		record.HostOrderNumber <= 0 {
		return false
	}

	instrumentID := strings.ToUpper(strings.TrimSpace(expected.ISIN))
	if record.InstrumentID != instrumentID || record.OrderPrice != expected.Price ||
		record.OrderQuantity != expected.Quantity || record.OrderValidityType != dayaOrderValidityToday ||
		record.OrderValidityDate != nil {
		return false
	}

	wantSide := ""
	switch strings.ToLower(strings.TrimSpace(expected.Side)) {
	case "buy":
		wantSide = dayaOrderSideBuy
	case "sell":
		wantSide = dayaOrderSideSell
	default:
		return false
	}
	return record.OrderSide == wantSide
}

// classifyDayaOrderResponse keeps the legacy no-context call fail-closed.
// A success record cannot be attributed to an order without matching the
// submitted instrument, side, price, quantity, and validity.
func classifyDayaOrderResponse(status int, body []byte) DayaOrderOutcome {
	return classifyDayaOrderResponseForOrder(status, body, nil)
}

func classifyDayaOrderResponseForOrder(status int, body []byte, expected *Order) DayaOrderOutcome {
	var metadata struct {
		RequestID string `json:"requestId"`
		Message   string `json:"message"`
		Error     string `json:"error"`
	}
	_ = json.Unmarshal(body, &metadata)
	message := strings.TrimSpace(metadata.Message)
	if message == "" {
		message = strings.TrimSpace(metadata.Error)
	}
	outcome := DayaOrderOutcome{HTTPStatus: status, RequestID: strings.TrimSpace(metadata.RequestID), ReceivedAt: time.Now().In(tehran())}
	switch {
	case status >= 200 && status < 300:
		for _, record := range decodeDayaConfirmationRecords(body) {
			if dayaConfirmationMatches(record, expected) {
				outcome.Status = statusSubmitted
				outcome.BrokerOrderID = fmt.Sprintf("%d", record.OrderID)
				outcome.BrokerHostOrderNumber = record.HostOrderNumber
				outcome.Message = "سفارش با وضعیت BOOKED و تطبیق کامل مشخصات سفارش در پاسخ دایا تأیید شد."
				return outcome
			}
		}
		outcome.Status = statusSubmissionUnknown
		if message == "" {
			message = fmt.Sprintf("دایا HTTP %d برگرداند، اما رکورد CONFIRMATION/BOOKED منطبق با سفارش ارسالی پیدا نشد.", status)
		}
		outcome.Message = message + " نتیجه را در پنل دایا بررسی کنید؛ ارسال مجدد خودکار انجام نمی‌شود."
	case status >= 400 && status < 500:
		outcome.Status = statusSubmissionFailed
		if strings.Contains(string(body), "امكان ثبت سفارش با توجه به وضعيت كروه وجود ندارد") ||
			strings.Contains(string(body), "امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد") {
			message = "امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد"
		}
		if message == "" {
			message = fmt.Sprintf("دایا درخواست را با HTTP %d رد کرد.", status)
		}
		outcome.Message = message
	default:
		outcome.Status = statusSubmissionUnknown
		if message == "" {
			message = fmt.Sprintf("پاسخ دایا HTTP %d قطعی نیست.", status)
		}
		outcome.Message = message + " قبل از هر ارسال مجدد، وضعیت سفارش را در کارگزاری بررسی کنید."
	}
	return outcome
}
