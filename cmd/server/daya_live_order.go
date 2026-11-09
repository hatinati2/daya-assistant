package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type dayaLiveOrderTestPageData struct {
	Nonce  string
	Error  string
	Result *Order
}

var dayaLiveOrderTestPage = template.Must(template.New("daya-live-order-test").Parse(`<!doctype html>
<html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>ارسال یک سفارش واقعی دایا</title>
<style>
*{box-sizing:border-box}body{margin:0;background:#f4f6fa;color:#182230;font:14px Tahoma,Arial}
main{max-width:760px;margin:28px auto;padding:0 16px}.card{background:#fff;border:1px solid #e4e7ec;border-radius:13px;padding:20px;margin:14px 0}
h1{font-size:20px}label{display:block;margin:12px 0 6px;font-weight:bold}input,select{width:100%;padding:10px;border:1px solid #d0d5dd;border-radius:8px;font:inherit;min-height:40px}
.grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.warning{padding:13px;border:1px solid #fedf89;background:#fffaeb;border-radius:9px;color:#93370d;line-height:1.9}
button{background:#b42318;color:#fff;border:0;border-radius:8px;padding:12px 18px;font:inherit;cursor:pointer;margin-top:12px}.error{padding:12px;background:#fef3f2;color:#b42318;border-radius:9px}.result{padding:12px;background:#ecfdf3;color:#05603a;border-radius:9px;line-height:1.9}
a{color:#175cd3;text-decoration:none}.muted{color:#667085;line-height:1.9}input[type=checkbox]{width:auto;min-height:0}
@media(max-width:600px){.grid{grid-template-columns:1fr}}
</style></head><body><main>
<h1>ارسال دستی یک سفارش واقعی به دایا</h1>
<p class="muted"><a href="/dashboard">داشبورد مرکزی</a> · <a href="/daya-online-test">بازگشت به تست آنلاین</a> · فقط نسخه Preview</p>
<div class="warning"><b>این فرم درخواست واقعی POST /orders می‌فرستد.</b> سفارش وارد زمان‌بندی نمی‌شود. فقط یک بار ارسال می‌شود و در صورت نامشخص‌بودن پاسخ، هیچ ارسال مجدد خودکاری انجام نمی‌شود. سفارش ممکن است در کارگزاری ثبت شود.</div>
<section class="card">
{{if not .Result}}
<form method="post" action="/daya-online-test/one/send" onsubmit="return confirm('تأیید نهایی: همین یک سفارش واقعی با مشخصات واردشده به دایا ارسال شود؟')">
<input type="hidden" name="submission_nonce" value="{{.Nonce}}">
<div class="grid">
<div><label>ISIN</label><input name="isin" maxlength="12" minlength="12" pattern="[A-Za-z0-9]{12}" required></div>
<div><label>سمت</label><select name="side"><option value="buy">خرید BUY</option><option value="sell">فروش SELL</option></select></div>
<div><label>قیمت به ریال</label><input name="price" type="number" min="1" step="1" required></div>
<div><label>تعداد</label><input name="quantity" type="number" min="1" step="1" required></div>
</div>
<label style="display:flex;gap:9px;align-items:flex-start;font-weight:normal"><input type="checkbox" name="confirm_live_order" value="yes" required><span>ارسال همین سفارش واقعی را با مشخصات بالا تأیید می‌کنم.</span></label>
<button type="submit">ارسال یک سفارش واقعی</button>
</form>
{{end}}
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
{{if .Result}}
<div class="result">
<h2>نتیجه سفارش ارسالی</h2>
وضعیت: <b>{{.Result.BrokerMessage}}</b><br>
HTTP: {{.Result.BrokerHTTPStatus}}<br>
ISIN: {{.Result.ISIN}} · سمت: {{.Result.Side}}<br>
قیمت: {{.Result.Price}} · تعداد: {{.Result.Quantity}}<br>
{{if .Result.BrokerOrderID}}شناسه سفارش دایا: {{.Result.BrokerOrderID}}<br>{{end}}
{{if .Result.BrokerHostOrderNumber}}شماره میزبان: {{.Result.BrokerHostOrderNumber}}<br>{{end}}
<div class="muted">این نتیجه ذخیره شده است. در وضعیت نامشخص، سفارش را دوباره نفرست.</div>
</div>
{{end}}
</section></main></body></html>`))

var dayaLiveOrderTestNonceStore = struct {
	sync.Mutex
	values map[string]time.Time
}{values: make(map[string]time.Time)}

func registerDayaLiveOrderTestRoutes(mux *http.ServeMux, previewMode bool, credentials *DayaCredentialStore, store *Store) {
	mux.HandleFunc("GET /daya-online-test/one", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		data := dayaLiveOrderTestPageData{}
		nonce, err := issueDayaLiveOrderTestNonce(time.Now())
		if err != nil {
			http.Error(w, "ساخت فرم امن ممکن نشد", http.StatusInternalServerError)
			return
		}
		data.Nonce = nonce
		if id := r.URL.Query().Get("order_id"); id != "" && store != nil {
			for _, order := range store.snapshot() {
				if order.ID == id && strings.HasPrefix(id, "online-") {
					data.Result = &order
					break
				}
			}
		}
		writeDayaLiveOrderTestPage(w, data)
	})
	mux.HandleFunc("POST /daya-online-test/one/send", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		if !sameOrigin(r) {
			http.Error(w, "درخواست نامعتبر", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
		if err := r.ParseForm(); err != nil {
			renderDayaLiveOrderTestError(w, "فرم معتبر نیست یا حجم آن بیش از حد مجاز است.")
			return
		}
		if r.FormValue("confirm_live_order") != "yes" {
			renderDayaLiveOrderTestError(w, "تأیید صریح ارسال سفارش واقعی لازم است.")
			return
		}
		if !consumeDayaLiveOrderTestNonce(r.FormValue("submission_nonce"), time.Now()) {
			renderDayaLiveOrderTestError(w, "این فرم قبلاً استفاده شده یا منقضی شده است؛ صفحه را تازه کن.")
			return
		}
		order, err := parseDayaLiveOrderTestForm(r)
		if err != nil {
			renderDayaLiveOrderTestError(w, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		id, err := submitOneDayaOnlineTestOrder(ctx, credentials, store, order)
		if id != "" {
			// PRG prevents browser refresh from replaying a live POST. Even if
			// result persistence failed, show the stored in-flight record and
			// never present the submission form again on this result page.
			http.Redirect(w, r, "/daya-online-test/one?order_id="+id, http.StatusSeeOther)
			return
		}
		if err != nil {
			renderDayaLiveOrderTestError(w, err.Error())
			return
		}
		renderDayaLiveOrderTestError(w, "نتیجه ارسال در دسترس نیست؛ قبل از هر اقدام، پنل دایا را بررسی کن.")
	})
}

func issueDayaLiveOrderTestNonce(now time.Time) (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	nonce := base64.RawURLEncoding.EncodeToString(raw[:])
	dayaLiveOrderTestNonceStore.Lock()
	defer dayaLiveOrderTestNonceStore.Unlock()
	for key, expiry := range dayaLiveOrderTestNonceStore.values {
		if !expiry.After(now) {
			delete(dayaLiveOrderTestNonceStore.values, key)
		}
	}
	dayaLiveOrderTestNonceStore.values[nonce] = now.Add(30 * time.Minute)
	return nonce, nil
}

func consumeDayaLiveOrderTestNonce(nonce string, now time.Time) bool {
	if nonce == "" || len(nonce) > 128 {
		return false
	}
	dayaLiveOrderTestNonceStore.Lock()
	defer dayaLiveOrderTestNonceStore.Unlock()
	expiry, ok := dayaLiveOrderTestNonceStore.values[nonce]
	delete(dayaLiveOrderTestNonceStore.values, nonce)
	return ok && expiry.After(now)
}

func parseDayaLiveOrderTestForm(r *http.Request) (Order, error) {
	isin := strings.ToUpper(strings.TrimSpace(r.FormValue("isin")))
	side := strings.TrimSpace(r.FormValue("side"))
	price, priceErr := strconv.ParseInt(strings.TrimSpace(r.FormValue("price")), 10, 64)
	quantity, quantityErr := strconv.ParseInt(strings.TrimSpace(r.FormValue("quantity")), 10, 64)
	if priceErr != nil || quantityErr != nil {
		return Order{}, errors.New("قیمت و تعداد باید عدد صحیح مثبت باشند.")
	}
	order := Order{
		ISIN: isin, Symbol: isin, Side: side, Price: price, Quantity: quantity,
		ExecutionMode: executionModeAPI,
	}
	if _, err := MarshalDayaOrderPayload(order); err != nil {
		return Order{}, err
	}
	return order, nil
}

func submitOneDayaOnlineTestOrder(ctx context.Context, credentials *DayaCredentialStore, store *Store, order Order) (string, error) {
	if credentials == nil || store == nil {
		return "", errors.New("مخزن نشست یا سفارش Preview در دسترس نیست.")
	}
	session, _, err := credentials.LoadAccessSession()
	if err != nil || session.Token == "" {
		return "", errors.New("نشست رمزنگاری‌شده دایا موجود نیست؛ ابتدا از صفحه اتصال وارد دایا شو.")
	}
	now := time.Now().In(tehran())
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(now) {
		return "", errors.New("نشست دایا منقضی شده است؛ ورود مجدد با CAPTCHA لازم است.")
	}
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return "", errors.New("ساخت شناسه یکتای تست ناموفق بود.")
	}
	localID := "online-" + hex.EncodeToString(idBytes[:])
	order.ID = localID
	order.Status = statusSubmitting
	order.ExecuteAt = now
	order.CreatedAt = now
	order.UpdatedAt = now
	order.AttemptedAt = now
	order.BrokerMessage = "ارسال دستی یک‌باره از صفحه تست آنلاین Preview؛ ارسال مجدد خودکار ممنوع است."
	if err := store.add(order); err != nil {
		return "", errors.New("ذخیره سفارش آزمایشی در Preview ناموفق بود؛ درخواست به دایا ارسال نشد.")
	}

	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	status, body, postErr := postDayaOrder(ctx, client, session, order)
	outcome := DayaOrderOutcome{
		Status: statusSubmissionUnknown, HTTPStatus: status,
		Message:    "نتیجه POST قطعی نیست؛ وضعیت را در پنل دایا بررسی کن و این سفارش را دوباره نفرست.",
		ReceivedAt: time.Now().In(tehran()),
	}
	if postErr != nil {
		outcome.Message = "ارتباط هنگام POST قطع شد؛ ممکن است دایا درخواست را دریافت کرده باشد. سفارش را دوباره نفرست و پنل دایا را بررسی کن."
	} else {
		outcome = classifyDayaOrderResponseForOrder(status, body, &order)
		outcome.ReceivedAt = time.Now().In(tehran())
	}
	if err := store.completeAPIOrder(localID, outcome); err != nil {
		return localID, errors.New("درخواست ارسال شد اما ذخیره نتیجه محلی کامل نشد؛ برای جلوگیری از سفارش تکراری دوباره ارسال نکن و تاریخچه Preview را بررسی کن.")
	}

	// One immediate read-only GET may resolve ENTRY to BOOKED. It never
	// resends POST and never checks execution/fill progress.
	if outcome.Status == statusSubmissionPending && outcome.BrokerOrderID != "" {
		getStatus, getBody, getErr := fetchDayaOrders(ctx, client, session)
		if getErr == nil && getStatus == http.StatusOK {
			for _, record := range decodeDayaConfirmationRecords(getBody) {
				if record.OrderID != parseBrokerOrderID(outcome.BrokerOrderID) || !dayaBookedConfirmationMatches(record, &order) {
					continue
				}
				booked := DayaOrderOutcome{
					Status: statusSubmitted, BrokerOrderID: outcome.BrokerOrderID,
					BrokerHostOrderNumber: record.HostOrderNumber, HTTPStatus: getStatus,
					Message:    "ثبت سفارش در دایا با وضعیت BOOKED تأیید شد.",
					ReceivedAt: time.Now().In(tehran()),
				}
				if err := store.completeAPIOrder(localID, booked); err != nil {
					return localID, errors.New("پاسخ BOOKED دریافت شد اما ذخیره نهایی محلی ناموفق بود؛ ارسال مجدد نکن.")
				}
				break
			}
		}
	}
	return localID, nil
}

func writeDayaLiveOrderTestPage(w http.ResponseWriter, data dayaLiveOrderTestPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := dayaLiveOrderTestPage.Execute(w, data); err != nil {
		http.Error(w, "نمایش صفحه تست آنلاین سفارش انجام نشد", http.StatusInternalServerError)
	}
}

func renderDayaLiveOrderTestError(w http.ResponseWriter, message string) {
	nonce, err := issueDayaLiveOrderTestNonce(time.Now())
	data := dayaLiveOrderTestPageData{Error: message}
	if err == nil {
		data.Nonce = nonce
	}
	writeDayaLiveOrderTestPage(w, data)
}
