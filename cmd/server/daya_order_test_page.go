package main

import (
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
	ISIN          string
	Side          string
	Price         string
	Quantity      string
	MockResponse  string
	Error         string
	Result        *dayaLocalOrderTestResult
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
</style>
</head>
<body><main>
<h1>آزمایش محلی درخواست سفارش دایا</h1>
<div class="muted">این صفحه فقط در نسخه Preview نمایش داده می‌شود.</div>
<nav><a href="/">بازگشت به سفارش‌ها</a><a href="/settings">اتصال به دایا</a></nav>
<div class="warning"><b>توجه مهم:</b> این آزمایش کاملاً محلی است. برنامه یک سرور ساختگی روی 127.0.0.1 ایجاد می‌کند و درخواست را فقط به همان سرور می‌فرستد. هیچ اتصالی به کارگزاری، دامنه mg.dayatrader.ir یا حساب معاملاتی برقرار نمی‌شود؛ هیچ سفارش واقعی ثبت نمی‌شود و هیچ توکن یا کوکی‌ای ارسال نمی‌شود. پاسخ سبز نیز فقط پاسخ ساختگی سرور محلی است، نه تأیید کارگزاری.</div>
<section class="card">
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
<section class="card">
<h2>نتیجه تست محلی</h2>
<div class="result">درخواست به سرور ساختگیِ داخل همین ماشین رسید. کد HTTP ساختگی: <b>{{.Result.HTTPStatus}}</b> — {{.Result.ScenarioLabel}}</div>
<label>JSON ارسال‌شده به سرور محلی</label><pre>{{.Result.RequestJSON}}</pre>
<label>پاسخ ساختگی سرور محلی</label><pre>{{.Result.ResponseBody}}</pre>
<div class="foot">این نتیجه فقط صحت ساخت JSON، مسیر POST /orders و دریافت پاسخ HTTP از mock محلی را می‌سنجد. نتیجه هیچ ارتباطی با پذیرش سفارش توسط دایا ندارد.</div>
</section>
{{end}}
</main></body></html>`))

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
			ISIN: strings.TrimSpace(r.FormValue("isin")),
			Side: strings.TrimSpace(r.FormValue("side")),
			Price: strings.TrimSpace(r.FormValue("price")),
			Quantity: strings.TrimSpace(r.FormValue("quantity")),
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

	var captured []byte
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
		captured = body
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"timestamp": time.Now().UnixMilli(),
			"path": "/orders",
			"status": status,
			"error": func() string { if status >= 400 { return responseMessage }; return "" }(),
			"message": responseMessage,
			"requestId": fmt.Sprintf("LOCAL-MOCK-%d", status),
		})
	}))
	defer mock.Close()

	httpStatus, responseBody, err := postDayaOrderToTestServer(ctx, mock.Client(), mock.URL, order)
	if err != nil {
		return dayaLocalOrderTestResult{}, err
	}
	if len(captured) == 0 {
		return dayaLocalOrderTestResult{}, errors.New("سرور ساختگی درخواست JSON را دریافت نکرد")
	}
	var pretty strings.Builder
	var indented []byte
	if err := json.Indent(&pretty, captured, "", "  "); err != nil {
		return dayaLocalOrderTestResult{}, fmt.Errorf("قالب‌بندی JSON آزمایشی: %w", err)
	}
	indented = []byte(pretty.String())
	return dayaLocalOrderTestResult{
		RequestJSON: string(indented), ResponseBody: strings.TrimSpace(string(responseBody)),
		HTTPStatus: httpStatus, ScenarioLabel: label,
	}, nil
}
