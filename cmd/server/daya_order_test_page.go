package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"
)

type dayaLocalOrderTestResult struct {
	RequestJSON   string
	ResponseBody  string
	HTTPStatus    int
	ScenarioLabel string
}

type dayaLocalOrderTestPageData struct {
	ISIN         string
	Side         string
	Price        string
	Quantity     string
	MockResponse string
	Error        string
	Result       *dayaLocalOrderTestResult
}

var dayaLocalOrderTestPage = template.Must(template.New("daya-local-order-test").Parse(`<!doctype html>
<html lang="fa" dir="rtl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>آزمایش محلی API دایا</title>
<style>
*{box-sizing:border-box}body{margin:0;background:#f4f6fa;color:#182230;font:13px Tahoma,Arial}
main{max-width:900px;margin:22px auto;padding:0 16px}.card{background:#fff;border:1px solid #e4e7ec;border-radius:13px;padding:18px;margin:12px 0}
h1{font-size:19px;margin:0 0 8px}h2{font-size:15px;margin:0 0 12px}.muted,.foot{color:#667085;line-height:1.8}
nav{display:flex;gap:8px;flex-wrap:wrap;margin:14px 0}nav a{color:#175cd3;text-decoration:none}
.warning{padding:12px;border:1px solid #fedf89;border-radius:9px;background:#fffaeb;color:#93370d;line-height:1.9;margin:12px 0}
.grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}
label{display:block;font-weight:bold;margin:8px 0 5px}input,select{width:100%;padding:9px;border:1px solid #d0d5dd;border-radius:8px;font:inherit;min-height:38px}
button{background:#175cd3;color:white;border:0;border-radius:8px;padding:10px 16px;font:inherit;cursor:pointer;margin-top:14px}
.error{background:#fef3f2;color:#b42318;border-radius:8px;padding:10px;margin:10px 0}
.result{background:#ecfdf3;color:#05603a;border-radius:8px;padding:10px;margin:10px 0}
pre{direction:ltr;text-align:left;white-space:pre-wrap;overflow-wrap:anywhere;background:#101828;color:#f2f4f7;border-radius:9px;padding:12px;max-height:340px;overflow:auto;font:12px ui-monospace,monospace}
@media(max-width:620px){.grid{grid-template-columns:1fr}}

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

</style>
</head>
<body><main class="page-wrap">
<header class="page-header"><div><h1>دستیار دایا</h1><div class="page-subtitle">آزمایش محلی درخواست سفارش دایا · آزمون ساخت Payload با سرور ساختگی روی همین ماشین</div></div><span class="page-tag">Preview 03</span></header>
<nav class="nav page-nav">
<a href="/dashboard">داشبورد مرکزی</a>
<a href="/">سفارش‌ها</a>
<a href="/history">تاریخچه</a>
<a href="/settings">اتصال و نشست</a>
<a href="/daya-api-test" class="active">تست محلی API</a>
<a href="/daya-online-test">تست آنلاین</a>
<a href="/daya-latency">کالیبراسیون تأخیر</a>
<a class="danger-link" href="/daya-online-test/one">ارسال یک‌باره واقعی</a>
</nav>
<div class="page-content">
<div class="warning"><b>توجه مهم:</b> این آزمایش کاملاً محلی است. برنامه یک سرور ساختگی روی 127.0.0.1 ایجاد می‌کند و درخواست را فقط به همان سرور می‌فرستد. هیچ اتصالی به کارگزاری، دامنه mg.dayatrader.ir یا حساب معاملاتی برقرار نمی‌شود؛ هیچ سفارش واقعی ثبت نمی‌شود و هیچ توکن یا کوکی‌ای ارسال نمی‌شود. پاسخ سبز نیز فقط پاسخ ساختگی سرور محلی است، نه تأیید کارگزاری.</div>
<section class="card page-card">
<h2>مشخصات نمونه سفارش</h2>
<form method="post" action="/daya-api-test">
<div class="grid">
<div><label for="isin">شناسه ابزار (ISIN)</label><input id="isin" name="isin" value="{{.ISIN}}" maxlength="12" minlength="12" pattern="[A-Za-z0-9]{12}" required></div>
<div><label for="side">سمت سفارش</label><select id="side" name="side"><option value="buy" {{if eq .Side "buy"}}selected{{end}}>خرید (BUY)</option><option value="sell" {{if eq .Side "sell"}}selected{{end}}>فروش (SELL)</option></select></div>
<div><label for="price">قیمت (ریال)</label><input id="price" name="price" type="number" min="1" step="1" value="{{.Price}}" required></div>
<div><label for="quantity">تعداد</label><input id="quantity" name="quantity" type="number" min="1" step="1" value="{{.Quantity}}" required></div>
<div style="grid-column:1/-1"><label for="mock_response">پاسخ ساختگی سرور محلی</label><select id="mock_response" name="mock_response"><option value="accepted" {{if eq .MockResponse "accepted"}}selected{{end}}>201 — دریافت موفق توسط سرور ساختگی</option><option value="rejected" {{if eq .MockResponse "rejected"}}selected{{end}}>400 — رد ساختگی به علت وضعیت گروه</option></select><div class="foot">این گزینه فقط پاسخ آزمایشی را انتخاب می‌کند و وضعیت واقعی بازار یا کارگزاری را شبیه‌سازی نمی‌کند.</div></div>
</div>
<button type="submit">اجرای تست محلی (بدون ارسال واقعی)</button>
</form>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
</section>
{{if .Result}}
<section class="card page-card">
<h2>نتیجه تست محلی</h2>
<div class="result">درخواست به سرور ساختگیِ داخل همین ماشین رسید. کد HTTP ساختگی: <b>{{.Result.HTTPStatus}}</b> — {{.Result.ScenarioLabel}}</div>
<label>JSON ارسال‌شده به سرور محلی</label><pre>{{.Result.RequestJSON}}</pre>
<label>پاسخ ساختگی سرور محلی</label><pre>{{.Result.ResponseBody}}</pre>
<div class="foot">این نتیجه فقط صحت ساخت JSON، مسیر POST /orders و دریافت پاسخ HTTP از mock محلی را می‌سنجد. نتیجه هیچ ارتباطی با پذیرش سفارش توسط دایا ندارد.</div>
</section>
{{end}}
</div></main></body></html>`))

// registerDayaLocalOrderTestRoutes exposes the local mock workflow only when
// DAYA_PREVIEW_MODE=true. The handlers never access broker credentials.
func registerDayaLocalOrderTestRoutes(mux *http.ServeMux, previewMode bool) {
	mux.HandleFunc("GET /daya-api-test", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		writeDayaLocalOrderTestPage(w, dayaLocalOrderTestPageData{
			ISIN: "IRO3SIMC0001", Side: "buy", Price: "63390", Quantity: "1", MockResponse: "accepted",
		})
	})
	mux.HandleFunc("POST /daya-api-test", func(w http.ResponseWriter, r *http.Request) {
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
			writeDayaLocalOrderTestPage(w, dayaLocalOrderTestPageData{Error: "فرم معتبر نیست یا حجم آن بیش از حد مجاز است."})
			return
		}

		data := dayaLocalOrderTestPageData{
			ISIN:         strings.TrimSpace(r.FormValue("isin")),
			Side:         strings.TrimSpace(r.FormValue("side")),
			Price:        strings.TrimSpace(r.FormValue("price")),
			Quantity:     strings.TrimSpace(r.FormValue("quantity")),
			MockResponse: strings.TrimSpace(r.FormValue("mock_response")),
		}
		price, priceErr := strconv.ParseInt(data.Price, 10, 64)
		quantity, quantityErr := strconv.ParseInt(data.Quantity, 10, 64)
		if priceErr != nil || quantityErr != nil {
			data.Error = "قیمت و تعداد باید عدد صحیح مثبت باشند."
			writeDayaLocalOrderTestPage(w, data)
			return
		}
		order := Order{
			ISIN: data.ISIN, Side: data.Side, Price: price, Quantity: quantity,
			ExecutionMode: executionModeAPI,
		}
		result, err := executeDayaLocalOrderTest(r.Context(), order, data.MockResponse)
		if err != nil {
			data.Error = err.Error()
			writeDayaLocalOrderTestPage(w, data)
			return
		}
		data.Result = &result
		writeDayaLocalOrderTestPage(w, data)
	})
}

func writeDayaLocalOrderTestPage(w http.ResponseWriter, data dayaLocalOrderTestPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := dayaLocalOrderTestPage.Execute(w, data); err != nil {
		http.Error(w, "نمایش صفحه تست انجام نشد", http.StatusInternalServerError)
	}
}

// executeDayaLocalOrderTest starts a temporary mock HTTP server bound to
// loopback and exercises the same transport used by unit tests. The selected
// 201/400 response is synthetic and must never be interpreted as broker state.
func executeDayaLocalOrderTest(ctx context.Context, order Order, scenario string) (dayaLocalOrderTestResult, error) {
	var status int
	var responseMessage string
	var label string
	switch scenario {
	case "accepted":
		status = http.StatusCreated
		responseMessage = "پاسخ ساختگی: سرور محلی درخواست آزمایشی را دریافت کرد."
		label = "دریافت موفق ساختگی"
	case "rejected":
		status = http.StatusBadRequest
		responseMessage = "امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد (پاسخ ساختگی)"
		label = "رد ساختگی"
	default:
		return dayaLocalOrderTestResult{}, errors.New("نوع پاسخ آزمایشی معتبر نیست")
	}

	captured := make(chan []byte, 1)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/orders" {
			http.Error(w, "unexpected mock request", http.StatusNotFound)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, (8<<10)+1))
		if err != nil || len(body) > 8<<10 {
			http.Error(w, "invalid mock request body", http.StatusBadRequest)
			return
		}
		select {
		case captured <- body:
		default:
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		errorMessage := ""
		if status >= 400 {
			errorMessage = responseMessage
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"timestamp": time.Now().UnixMilli(),
			"path":      "/orders",
			"status":    status,
			"error":     errorMessage,
			"message":   responseMessage,
			"requestId": fmt.Sprintf("LOCAL-MOCK-%d", status),
		})
	}))
	defer mock.Close()

	httpStatus, responseBody, err := postDayaOrderToTestServer(ctx, mock.Client(), mock.URL, order)
	if err != nil {
		return dayaLocalOrderTestResult{}, err
	}
	var requestBody []byte
	select {
	case requestBody = <-captured:
	default:
		return dayaLocalOrderTestResult{}, errors.New("سرور ساختگی درخواست JSON را دریافت نکرد")
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, requestBody, "", "  "); err != nil {
		return dayaLocalOrderTestResult{}, fmt.Errorf("قالب‌بندی JSON آزمایشی: %w", err)
	}
	return dayaLocalOrderTestResult{
		RequestJSON: pretty.String(), ResponseBody: strings.TrimSpace(string(responseBody)),
		HTTPStatus: httpStatus, ScenarioLabel: label,
	}, nil
}
