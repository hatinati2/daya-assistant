package main

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"
	"time"
)

type dayaOnlineTestResult struct {
	HTTPStatus    int
	RecordCount   int
	EntryCount    int
	BookedCount   int
	OtherCount    int
	ResponseBytes int
}

type dayaOnlineTestPageData struct {
	Error  string
	Result *dayaOnlineTestResult
}

var dayaOnlineTestPage = template.Must(template.New("daya-online-test").Parse(`<!doctype html>
<html lang="fa" dir="rtl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>تست آنلاین اتصال دایا</title>
<style>
*{box-sizing:border-box}body{margin:0;background:#f4f6fa;color:#182230;font:14px Tahoma,Arial}
main{max-width:760px;margin:28px auto;padding:0 16px}.card{background:#fff;border:1px solid #e4e7ec;border-radius:13px;padding:20px;margin:14px 0}
h1{font-size:20px;margin:0 0 10px}.muted,.foot{color:#667085;line-height:1.9}
nav{display:flex;gap:12px;flex-wrap:wrap;margin:16px 0}a{color:#175cd3;text-decoration:none}
.note{padding:13px;border:1px solid #b2ddff;background:#eff8ff;border-radius:9px;color:#1849a9;line-height:1.9}
.warning{padding:13px;border:1px solid #fedf89;background:#fffaeb;border-radius:9px;color:#93370d;line-height:1.9;margin:12px 0}
button{background:#175cd3;color:#fff;border:0;border-radius:8px;padding:12px 18px;font:inherit;cursor:pointer;margin-top:10px}
.error{padding:12px;border-radius:9px;background:#fef3f2;color:#b42318;line-height:1.9;margin-top:12px}
.success{padding:12px;border-radius:9px;background:#ecfdf3;color:#05603a;line-height:1.9;margin-top:12px}
table{border-collapse:collapse;width:100%;margin-top:14px}td{padding:9px;border-bottom:1px solid #eaecf0}td:first-child{color:#475467}

/* Preview 03 shared page chrome */
body{background:#f4f6fa;color:#182230;font:12.5px Tahoma,Arial}
main,.wrap{max-width:1400px;margin:18px auto;padding:0 16px}
.page-header,.head{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;margin-bottom:12px}
.page-header h1,.head h1{font-size:19px;margin:0}
.page-subtitle{color:#667085;font-size:12px;line-height:1.8;margin-top:6px}
.page-tag{display:inline-block;border-radius:18px;padding:7px 10px;background:#eef4ff;color:#1849a9;font-size:11px}
.nav{display:flex;gap:7px;flex-wrap:wrap;margin:0 0 12px}
.nav a{display:inline-block;padding:7px 10px;border:1px solid #d0d5dd;border-radius:9px;color:#344054;text-decoration:none;background:#fff;font-size:12px}
.nav a.active{background:#175cd3;color:#fff;border-color:#175cd3}
.nav a.danger-link{color:#b42318;border-color:#fecdca}
.card,.page-card{background:#fff;border:1px solid #e4e7ec;border-radius:13px;padding:18px;margin:12px 0;min-width:0}
.page-card{max-width:1080px;margin-left:auto;margin-right:auto}
h1{font-size:19px;margin:0 0 8px}h2{font-size:14px;margin:0 0 10px}
.muted,.foot{color:#667085;line-height:1.8}
label{display:block;margin:10px 0 5px;font-weight:bold}
input,select,textarea{width:100%;padding:8px 10px;border:1px solid #d0d5dd;border-radius:8px;background:#fff;color:#182230;font:inherit;min-height:36px}
input:focus,select:focus,textarea:focus{outline:2px solid #b2ddff;outline-offset:1px;border-color:#84caff}
button{border:0;border-radius:8px;background:#175cd3;color:#fff;padding:8px 12px;font:inherit;cursor:pointer}
button:disabled{opacity:.5;cursor:not-allowed}
.warning,.note{padding:11px 13px;border:1px solid #fedf89;border-radius:9px;background:#fffaeb;color:#93370d;line-height:1.8;margin:10px 0}
.error,.err{padding:11px 13px;border-radius:9px;background:#fef3f2;color:#b42318;line-height:1.8;margin:10px 0}
.success,.result,.msg{padding:11px 13px;border-radius:9px;background:#ecfdf3;color:#05603a;line-height:1.8;margin:10px 0}
table{border-collapse:collapse;width:100%;margin-top:12px}
th,td{padding:8px 9px;border-bottom:1px solid #eaecf0;text-align:right}
th{font-size:11px;color:#667085}
pre{border-radius:10px}
@media(max-width:640px){main,.wrap{padding-left:12px;padding-right:12px}.page-card{padding:14px}.nav{gap:6px}.nav a{padding:7px 8px;font-size:11px}}

</style></head>
<body><main class="page-wrap">
<header class="page-header"><div><h1>دستیار دایا</h1><div class="page-subtitle">تست آنلاین اتصال به کارگزاری دایا · بررسی خواندنی ارتباط و دسترسی با نشست رمزنگاری‌شده</div></div><span class="page-tag">Preview 03</span></header>
<nav class="nav page-nav">
<a href="/dashboard">داشبورد مرکزی</a>
<a href="/">سفارش‌ها</a>
<a href="/history">تاریخچه</a>
<a href="/settings">اتصال و نشست</a>
<a href="/daya-api-test">تست محلی API</a>
<a href="/daya-online-test" class="active">تست آنلاین</a>
<a href="/daya-latency">کالیبراسیون تأخیر</a>
<a class="danger-link" href="/daya-online-test/one">ارسال یک‌باره واقعی</a>
</nav>
<div class="page-content">
<div class="warning"><b>مرز تست:</b> با فشردن دکمه فقط درخواست واقعی و خواندنی GET /orders به سرور دایا فرستاده می‌شود. هیچ POST /orders، خرید، فروش، لغو یا تغییری در سفارش‌ها انجام نمی‌شود. پاسخ خام، شناسه‌ها، قیمت‌ها، تعدادها و کوکی‌ها در این صفحه نمایش داده نمی‌شوند.</div>
<section class="card page-card">
<h2>بررسی ارتباط و نشست</h2>
<div class="note">این آزمایش بررسی می‌کند که Preview با نشست ذخیره‌شده می‌تواند به endpoint واقعی سفارش‌های دایا وصل شود و پاسخ JSON معتبر دریافت کند.</div>
<form method="post" action="/daya-online-test/check">
<button type="submit">اجرای تست آنلاین واقعی (فقط خواندنی)</button>
</form>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
{{if .Result}}
<div class="success">درخواست واقعی به دایا پاسخ داد و پاسخ JSON معتبر دریافت شد.</div>
<table>
<tr><td>HTTP Status</td><td><b>{{.Result.HTTPStatus}}</b></td></tr>
<tr><td>تعداد رکوردهای پاسخ</td><td>{{.Result.RecordCount}}</td></tr>
<tr><td>تعداد ENTRY / SENT_TO_TRADING_ENGINE</td><td>{{.Result.EntryCount}}</td></tr>
<tr><td>تعداد CONFIRMATION / BOOKED</td><td>{{.Result.BookedCount}}</td></tr>
<tr><td>سایر رکوردها</td><td>{{.Result.OtherCount}}</td></tr>
<tr><td>اندازه پاسخ</td><td>{{.Result.ResponseBytes}} بایت</td></tr>
</table>
<div class="foot">این نتیجه اتصال واقعی و دسترسی خواندنی به فهرست سفارش‌ها را تأیید می‌کند؛ به معنی آزمایش POST یا ثبت سفارش جدید نیست.</div>
{{end}}
</section></div></main></body></html>`))

// registerDayaOnlineTestRoutes exposes a manual, read-only broker connectivity
// probe only in Preview. It never submits, changes, cancels, or schedules orders.
func registerDayaOnlineTestRoutes(mux *http.ServeMux, previewMode bool, credentials *DayaCredentialStore) {
	mux.HandleFunc("GET /daya-online-test", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		writeDayaOnlineTestPage(w, dayaOnlineTestPageData{})
	})
	mux.HandleFunc("POST /daya-online-test/check", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		if !sameOrigin(r) {
			http.Error(w, "درخواست نامعتبر", http.StatusForbidden)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		result, err := runDayaOnlineReadOnlyCheck(ctx, credentials)
		data := dayaOnlineTestPageData{Result: result}
		if err != nil {
			data.Result = nil
			data.Error = err.Error()
		}
		writeDayaOnlineTestPage(w, data)
	})
}

func writeDayaOnlineTestPage(w http.ResponseWriter, data dayaOnlineTestPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := dayaOnlineTestPage.Execute(w, data); err != nil {
		http.Error(w, "نمایش نتیجهٔ تست دایا انجام نشد", http.StatusInternalServerError)
	}
}

func runDayaOnlineReadOnlyCheck(ctx context.Context, credentials *DayaCredentialStore) (*dayaOnlineTestResult, error) {
	if credentials == nil {
		return nil, errors.New("مخزن نشست دایا در دسترس نیست")
	}
	session, _, err := credentials.LoadAccessSession()
	if err != nil {
		return nil, errors.New("خواندن نشست رمزنگاری‌شدهٔ دایا ناموفق بود؛ از صفحه اتصال، دوباره وارد شو")
	}
	if session.Token == "" {
		return nil, errors.New("نشست دایا ذخیره نشده است؛ ابتدا از صفحه اتصال وارد دایا شو")
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(time.Now()) {
		return nil, errors.New("نشست دایا منقضی شده است؛ ابتدا ورود مجدد با CAPTCHA لازم است")
	}

	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	status, body, err := fetchDayaOrders(ctx, client, session)
	if err != nil {
		return nil, errors.New("ارتباط خواندنی با دایا برقرار نشد؛ هیچ سفارشی ارسال نشده است")
	}
	result, err := summarizeDayaOrderListResponse(status, body)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// summarizeDayaOrderListResponse deliberately returns counts only. Broker
// order IDs, account details, instruments, prices, quantities and raw messages
// are never exposed by the online-test page.
func summarizeDayaOrderListResponse(status int, body []byte) (*dayaOnlineTestResult, error) {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, errors.New("دایا نشست را نپذیرفت (401/403)؛ بررسی متوقف شد و ورود مجدد با CAPTCHA لازم است")
	case http.StatusTooManyRequests:
		return nil, errors.New("دایا محدودیت درخواست اعمال کرد (429)؛ تست متوقف شد و تکرار خودکار انجام نمی‌شود")
	}
	if status != http.StatusOK {
		return nil, errors.New("دایا پاسخ HTTP " + http.StatusText(status) + " نداد؛ هیچ سفارش جدیدی ارسال نشده است")
	}
	if len(body) == 0 || len(body) > 1<<20 || !json.Valid(body) {
		return nil, errors.New("دایا پاسخ خالی، بیش از حد بزرگ یا غیر JSON برگرداند")
	}

	trimmed := strings.TrimSpace(string(body))
	var rawRecords []json.RawMessage
	switch {
	case strings.HasPrefix(trimmed, "["):
		if err := json.Unmarshal(body, &rawRecords); err != nil {
			return nil, errors.New("ساختار فهرست سفارش‌های دایا قابل خواندن نیست")
		}
	case strings.HasPrefix(trimmed, "{"):
		rawRecords = []json.RawMessage{json.RawMessage(body)}
	default:
		return nil, errors.New("ساختار پاسخ فهرست سفارش‌های دایا پشتیبانی نمی‌شود")
	}

	result := &dayaOnlineTestResult{
		HTTPStatus:    status,
		RecordCount:   len(rawRecords),
		ResponseBytes: len(body),
	}
	for _, raw := range rawRecords {
		var record struct {
			OrderFunction string `json:"orderFunction"`
			OrderStatus   string `json:"orderStatus"`
		}
		if err := json.Unmarshal(raw, &record); err != nil {
			result.OtherCount++
			continue
		}
		switch {
		case record.OrderFunction == "ENTRY" && record.OrderStatus == "SENT_TO_TRADING_ENGINE":
			result.EntryCount++
		case record.OrderFunction == "CONFIRMATION" && record.OrderStatus == "BOOKED":
			result.BookedCount++
		default:
			result.OtherCount++
		}
	}
	return result, nil
}
