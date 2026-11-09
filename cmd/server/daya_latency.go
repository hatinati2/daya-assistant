package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"time"
)

const dayaLatencyProbeURL = "https://mg.dayatrader.ir/orders"

type dayaLatencySample struct {
	Mode             string `json:"mode"`
	HTTPStatus       int    `json:"http_status"`
	RTTMillis        int64  `json:"rtt_ms"`
	StartedAt        string `json:"started_at"`
	EndedAt          string `json:"ended_at"`
	ResponseValid    bool   `json:"response_valid"`
	Authenticated    bool   `json:"authenticated"`
	SessionRotated   bool   `json:"session_rotated"`
	SessionPersisted bool   `json:"session_persisted"`
	Error            string `json:"error,omitempty"`
	StopBatch        bool   `json:"-"`
}

type dayaLatencySummary struct {
	Count             int     `json:"count"`
	SuccessCount      int     `json:"success_count"`
	RejectedCount     int     `json:"rejected_count"`
	ErrorCount        int     `json:"error_count"`
	MinRTTMillis      int64   `json:"min_rtt_ms"`
	MedianRTTMillis   float64 `json:"median_rtt_ms"`
	MeanRTTMillis     float64 `json:"mean_rtt_ms"`
	P95RTTMillis      int64   `json:"p95_rtt_ms"`
	EstimatedOneWayMS float64 `json:"estimated_one_way_ms"`
	Note              string  `json:"note"`
}

type dayaLatencyResponse struct {
	Mode    string              `json:"mode"`
	Samples []dayaLatencySample `json:"samples"`
	Summary dayaLatencySummary  `json:"summary"`
}

type dayaLatencyPageData struct {
	HasSession bool
}

var dayaLatencyPage = template.Must(template.New("daya-latency").Parse(`
<!doctype html>
<html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>کالیبره‌کردن تأخیر دایا</title><style>
*{box-sizing:border-box}body{margin:0;background:#f4f6fa;color:#182230;font:14px Tahoma,Arial}
main{max-width:980px;margin:26px auto;padding:0 16px}.card{background:#fff;border:1px solid #e4e7ec;border-radius:13px;padding:18px;margin:14px 0}
h1{font-size:21px}.muted{color:#667085;line-height:2}.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:10px}
.stat{background:#f8fafc;border-radius:9px;padding:12px}.value{font-size:21px;font-weight:bold;direction:ltr;text-align:right}
button{background:#175cd3;color:#fff;border:0;border-radius:8px;padding:11px 16px;font:inherit;cursor:pointer;margin:4px 0}
button.secondary{background:#344054}button:disabled{opacity:.5;cursor:not-allowed}
.warning{padding:12px;background:#fffaeb;border:1px solid #fedf89;border-radius:9px;line-height:2}
.nav{display:flex;gap:8px;flex-wrap:wrap;margin:12px 0}.nav a{color:#175cd3;text-decoration:none}
table{width:100%;border-collapse:collapse}td,th{padding:8px;border-bottom:1px solid #eaecf0;text-align:right;font-size:12px}
@media(max-width:640px){.stats{grid-template-columns:repeat(2,1fr)}}

/* Keep all Preview 03 pages on the same visual system as the main workspace */
body{background:#f4f6fa;color:#182230;font:12.5px Tahoma,Arial}
main,.wrap{max-width:1400px;margin:18px auto;padding:0 16px}
.page-header,.head{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;margin-bottom:12px}
.page-header h1,.head h1{font-size:19px;margin:0}
.page-subtitle{color:#667085;font-size:12px;line-height:1.8;margin-top:6px}
.page-tag{display:inline-block;border-radius:18px;padding:7px 10px;background:#eef4ff;color:#1849a9;font-size:11px}
.page-content{max-width:1080px;margin:0 auto}
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


/* Wider content like the main order workspace; denser forms on desktop */
main,.wrap{width:calc(100% - 32px);max-width:1400px}
.page-content{width:100%;max-width:1360px;margin:0 auto}
.page-card{width:100%;max-width:1360px;margin:12px auto;padding:20px}
.page-content .grid{grid-template-columns:repeat(4,minmax(0,1fr));gap:12px}
.page-content .grid>*{min-width:0}
@media(max-width:980px){
 .page-content .grid{grid-template-columns:repeat(2,minmax(0,1fr))}
}
@media(max-width:600px){
 main,.wrap{width:100%;padding-left:12px;padding-right:12px}
 .page-content .grid{grid-template-columns:1fr}
 .page-card{padding:14px}
}

</style></head><body><main class="page-wrap">
<header class="page-header"><div><h1>دستیار دایا</h1><div class="page-subtitle">کالیبره‌کردن تأخیر ارتباط با دایا · دو حالت بدون نشست و با نشست ذخیره‌شده</div></div><span class="page-tag">Preview 03</span></header>
<nav class="nav page-nav">
<a href="/dashboard">داشبورد مرکزی</a>
<a href="/">سفارش‌ها</a>
<a href="/history">تاریخچه</a>
<a href="/settings">اتصال و نشست</a>
<a href="/daya-api-test">تست محلی API</a>
<a href="/daya-online-test">تست آنلاین</a>
<a href="/daya-latency" class="active">کالیبراسیون تأخیر</a>
<a class="danger-link" href="/daya-online-test/one">ارسال یک‌باره واقعی</a>
</nav>
<div class="page-content">
<p class="muted">هر دو حالت فقط درخواست GET خواندنی به مسیر سفارش‌ها می‌فرستند. هیچ POST، سفارش‌گذاری، لغو یا تغییری در اطلاعات معاملاتی انجام نمی‌شود.</p>
<section class="card page-card">
<div class="warning">RTT زمان رفت‌وبرگشت درخواست تا دریافت سرآیند پاسخ و پردازش مقصد است. نصف RTT فقط تخمین تقریبی یک‌طرفه است و زمان رسیدن یا ثبت سفارش POST را پیش‌بینی یا تضمین نمی‌کند. در حالت بدون نشست، HTTP 401/403 فقط دریافت پاسخ است و دسترسی موفق محسوب نمی‌شود.</div>
<p>
<button id="run-anonymous">اجرای ۱۰ نمونه بدون نشست</button>
<button id="run-authenticated" class="secondary" {{if not .HasSession}}disabled title="ابتدا از صفحه اتصال، دستی وارد دایا شو"{{end}}>اجرای ۱۰ نمونه با نشست ذخیره‌شده</button>
<span id="state" class="muted" aria-live="polite"></span>
</p>
<p id="mode-note" class="muted">{{if .HasSession}}نشست رمزنگاری‌شده روی سرور موجود است؛ مقدار توکن و کوکی به مرورگر ارسال نمی‌شود.{{else}}نشست معتبر ذخیره‌شده پیدا نشد؛ برای حالت دوم ابتدا از صفحه اتصال وارد دایا شو.{{end}}</p>
<div id="summary" class="stats"></div>
<p id="note" class="muted"></p>
</section>
<section class="card"><h2>نمونه‌ها</h2><div style="overflow:auto"><table><thead><tr><th>#</th><th>HTTP</th><th>RTT (ms)</th><th>پاسخ معتبر</th><th>چرخش نشست</th><th>زمان پایان (تهران)</th><th>توضیح</th></tr></thead><tbody id="rows"></tbody></table></div></section>
<script>
const anonymousButton=document.getElementById("run-anonymous");
const authenticatedButton=document.getElementById("run-authenticated");
const state=document.getElementById("state");
const initialAuthDisabled=authenticatedButton.disabled;
function escapeHTML(value){return String(value??"").replace(/[&<>"']/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));}
async function runCalibration(mode,button){
 const authenticated=mode==="authenticated";
 const other=authenticated?anonymousButton:authenticatedButton;
 button.disabled=true;other.disabled=true;state.textContent="در حال اندازه‌گیری…";
 document.getElementById("summary").replaceChildren();
 document.getElementById("rows").replaceChildren();
 try{
  const url=authenticated?"/api/daya/latency-authenticated?count=10":"/api/daya/latency-samples?count=10";
  const response=await fetch(url,{method:authenticated?"POST":"GET",cache:"no-store",headers:{"Accept":"application/json"}});
  const data=await response.json().catch(()=>({}));
  if(!response.ok)throw new Error(data.error||"خطا در اندازه‌گیری (HTTP "+response.status+")");
  const s=data.summary;
  const cards=[
   ["تعداد پاسخ‌های HTTP",s.count],
   ["پاسخ معتبر ۲۰۰",s.success_count],
   ["پاسخ ۴۰۱/۴۰۳",s.rejected_count],
   ["میانه RTT",Number(s.median_rtt_ms||0).toFixed(1)+" ms"],
   ["میانگین RTT",Number(s.mean_rtt_ms||0).toFixed(1)+" ms"],
   ["صدک ۹۵ RTT",(s.p95_rtt_ms||0)+" ms"],
   ["کمترین RTT",(s.min_rtt_ms||0)+" ms"],
   ["تخمین یک‌طرفه",Number(s.estimated_one_way_ms||0).toFixed(1)+" ms"],
   ["خطاهای ارتباط/پاسخ",s.error_count]
  ];
  document.getElementById("summary").innerHTML=cards.map(x=>'<div class="stat"><div class="muted">'+escapeHTML(x[0])+'</div><div class="value">'+escapeHTML(x[1])+'</div></div>').join("");
  document.getElementById("note").textContent=s.note||"";
  document.getElementById("rows").innerHTML=(data.samples||[]).map((x,i)=>'<tr><td>'+(i+1)+'</td><td>'+escapeHTML(x.http_status||"—")+'</td><td>'+escapeHTML(x.rtt_ms)+'</td><td>'+(x.response_valid?"بله":"خیر")+'</td><td>'+(x.session_rotated?(x.session_persisted?"ذخیره شد":"ذخیره نشد"):"—")+'</td><td>'+escapeHTML(x.ended_at||"—")+'</td><td>'+escapeHTML(x.error||"—")+'</td></tr>').join("");
  state.textContent=authenticated?"اندازه‌گیری با نشست ذخیره‌شده تمام شد":"اندازه‌گیری بدون نشست تمام شد";
 }catch(error){state.textContent=error.message||"اندازه‌گیری ناموفق بود"}
 finally{button.disabled=false;anonymousButton.disabled=false;authenticatedButton.disabled=initialAuthDisabled;}
}
anonymousButton.addEventListener("click",()=>runCalibration("anonymous",anonymousButton));
authenticatedButton.addEventListener("click",()=>runCalibration("authenticated",authenticatedButton));
</script></main></body></html>
`))

func registerDayaLatencyRoutes(mux *http.ServeMux, previewMode bool, credentials *DayaCredentialStore, monitor *DayaSessionMonitor) {
	mux.HandleFunc("GET /daya-latency", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		hasSession := false
		if credentials != nil {
			session, _, err := credentials.LoadAccessSession()
			hasSession = err == nil && session.Token != "" && (session.ExpiresAt.IsZero() || session.ExpiresAt.After(time.Now()))
			if monitor != nil && monitor.Snapshot().NeedsLogin {
				hasSession = false
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = dayaLatencyPage.Execute(w, dayaLatencyPageData{HasSession: hasSession})
	})
	mux.HandleFunc("GET /api/daya/latency-samples", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		count, err := parseDayaLatencyCount(r.URL.Query().Get("count"))
		if err != nil {
			writeDayaLatencyError(w, http.StatusBadRequest, err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 110*time.Second)
		defer cancel()
		result := dayaLatencyResponse{Mode: "anonymous", Samples: make([]dayaLatencySample, 0, count)}
		for i := 0; i < count; i++ {
			result.Samples = append(result.Samples, probeDayaLatency(ctx))
			if i+1 < count {
				time.Sleep(250 * time.Millisecond)
			}
		}
		result.Summary = summarizeDayaLatency(result.Samples)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	})
	mux.HandleFunc("POST /api/daya/latency-authenticated", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		if !sameOrigin(r) {
			writeDayaLatencyError(w, http.StatusForbidden, "درخواست از مبدأ پنل پذیرفته نشد")
			return
		}
		if credentials == nil {
			writeDayaLatencyError(w, http.StatusServiceUnavailable, "مخزن نشست دایا در دسترس نیست")
			return
		}
		if monitor != nil && monitor.Snapshot().NeedsLogin {
			writeDayaLatencyError(w, http.StatusConflict, "پایش دایا نیاز به ورود مجدد را ثبت کرده است؛ ابتدا با CAPTCHA دوباره وارد شو")
			return
		}
		count, err := parseDayaLatencyCount(r.URL.Query().Get("count"))
		if err != nil {
			writeDayaLatencyError(w, http.StatusBadRequest, err.Error())
			return
		}
		session, _, err := credentials.LoadAccessSession()
		if err != nil || session.Token == "" {
			writeDayaLatencyError(w, http.StatusConflict, "نشست ذخیره‌شده دایا موجود نیست؛ ابتدا از صفحه اتصال با CAPTCHA وارد شو")
			return
		}
		if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(time.Now()) {
			writeDayaLatencyError(w, http.StatusConflict, "نشست دایا منقضی شده است؛ ورود مجدد با CAPTCHA لازم است")
			return
		}

		monitorWasRunning := monitor != nil && monitor.Snapshot().Running
		if monitorWasRunning {
			monitor.Stop(false, "پایش خودکار هنگام کالیبراسیون دستی موقتاً متوقف شد")
		}

		ctx, cancel := context.WithTimeout(r.Context(), 110*time.Second)
		defer cancel()
		result := dayaLatencyResponse{Mode: "authenticated", Samples: make([]dayaLatencySample, 0, count)}
		needsLogin := false
		rateLimited := false
		sessionUpdateFailed := false

		for i := 0; i < count; i++ {
			sample := probeDayaLatencyAuthenticatedAt(ctx, dayaLatencyProbeURL, &session, credentials)
			result.Samples = append(result.Samples, sample)
			switch sample.HTTPStatus {
			case http.StatusUnauthorized, http.StatusForbidden:
				needsLogin = true
				break
			case http.StatusTooManyRequests:
				rateLimited = true
				break
			}
			if sample.StopBatch || needsLogin || rateLimited || ctx.Err() != nil {
				sessionUpdateFailed = sample.StopBatch
				break
			}
			if i+1 < count {
				time.Sleep(250 * time.Millisecond)
			}
		}
		result.Summary = summarizeDayaLatency(result.Samples)
		if needsLogin {
			result.Summary.Note += " دایا نشست ذخیره‌شده را نپذیرفت؛ ادامه آزمایش متوقف شد و ورود دستی لازم است."
		} else if rateLimited {
			result.Summary.Note += " دایا محدودیت درخواست اعلام کرد؛ ادامه آزمایش متوقف شد."
		} else if sessionUpdateFailed {
			result.Summary.Note += " پردازش یا ذخیره امن چرخش نشست ناموفق بود؛ پایش خودکار ازسرگرفته نشد."
		}

		if monitor != nil {
			switch {
			case needsLogin:
				monitor.Stop(true, "نشست هنگام کالیبراسیون احراز هویت‌شده پذیرفته نشد؛ ورود دستی با CAPTCHA لازم است")
			case rateLimited:
				monitor.Stop(false, "دایا هنگام کالیبراسیون محدودیت درخواست اعلام کرد؛ پایش متوقف شد")
			case sessionUpdateFailed:
				monitor.Stop(false, "به‌روزرسانی امن نشست هنگام کالیبراسیون ناموفق بود؛ بررسی دستی لازم است")
			case monitorWasRunning:
				if err := monitor.Start(context.Background(), session); err != nil {
					monitor.Stop(false, "راه‌اندازی دوباره پایش نشست پس از کالیبراسیون ناموفق بود")
				}
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	})
}

func parseDayaLatencyCount(raw string) (int, error) {
	count := 10
	if raw == "" {
		return count, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 3 || n > 20 {
		return 0, fmt.Errorf("تعداد نمونه باید بین ۳ تا ۲۰ باشد")
	}
	return n, nil
}

func writeDayaLatencyError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func probeDayaLatency(parent context.Context) dayaLatencySample {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	start := time.Now()
	sample := dayaLatencySample{Mode: "anonymous", StartedAt: start.In(tehran()).Format(time.RFC3339Nano)}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dayaLatencyProbeURL, nil)
	if err != nil {
		sample.Error = "ساخت درخواست ناموفق بود"
		return sample
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "DayaAssistant-LatencyProbe/1.0")
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	end := time.Now()
	sample.RTTMillis = end.Sub(start).Milliseconds()
	sample.EndedAt = end.In(tehran()).Format(time.RFC3339Nano)
	if err != nil {
		sample.Error = "ارتباط با endpoint کامل نشد"
		return sample
	}
	sample.HTTPStatus = resp.StatusCode
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	_ = resp.Body.Close()
	if readErr != nil || len(body) > 1<<20 {
		sample.Error = "خواندن پاسخ خواندنی کامل نشد یا پاسخ بیش از حد مجاز بود"
		return sample
	}
	if resp.StatusCode == http.StatusOK {
		if _, validationErr := summarizeDayaOrderListResponse(resp.StatusCode, body); validationErr == nil {
			sample.ResponseValid = true
		} else {
			sample.Error = "پاسخ HTTP 200 دریافت شد اما ساختار JSON سفارش‌ها معتبر نبود"
		}
	}
	return sample
}

func probeDayaLatencyAuthenticatedAt(parent context.Context, endpoint string, session *DayaSession, credentials *DayaCredentialStore) dayaLatencySample {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	start := time.Now()
	sample := dayaLatencySample{Mode: "authenticated", StartedAt: start.In(tehran()).Format(time.RFC3339Nano)}
	if session == nil || session.Token == "" {
		sample.Error = "نشست معتبر دایا موجود نیست"
		sample.StopBatch = true
		return sample
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(start) {
		sample.Error = "نشست دایا منقضی شده است"
		sample.StopBatch = true
		return sample
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		sample.Error = "ساخت درخواست خواندنی ناموفق بود"
		return sample
	}
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
		req.AddCookie(cookie.httpCookie())
		seen[cookie.Name] = true
	}
	if !seen["token"] {
		req.AddCookie(&http.Cookie{Name: "token", Value: session.Token, Path: "/"})
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", dayaWebOrigin)
	req.Header.Set("Referer", dayaWebOrigin+"/")
	req.Header.Set("x-client-type", "WEB_CLIENT")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36")

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	end := time.Now()
	sample.RTTMillis = end.Sub(start).Milliseconds()
	sample.EndedAt = end.In(tehran()).Format(time.RFC3339Nano)
	if err != nil {
		sample.Error = "ارتباط احراز هویت‌شده با endpoint کامل نشد"
		return sample
	}
	defer resp.Body.Close()
	sample.HTTPStatus = resp.StatusCode

	if len(resp.Cookies()) > 0 {
		updated := *session
		updated.Cookies = append([]DayaSessionCookie(nil), session.Cookies...)
		rotated, applyErr := dayaApplySessionCookies(&updated, resp.Cookies(), time.Now().UTC())
		if applyErr != nil {
			sample.Error = "کوکی بازگشتی قابل اعتبارسنجی نبود"
			sample.StopBatch = true
			return sample
		}
		if rotated {
			sample.SessionRotated = true
			if credentials != nil {
				if saveErr := credentials.SaveAccessSession(updated); saveErr != nil {
					sample.Error = "چرخش نشست دریافت شد اما ذخیره امن آن ناموفق بود"
					sample.StopBatch = true
					return sample
				}
				sample.SessionPersisted = true
			}
			*session = updated
		}
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if readErr != nil || len(body) > 1<<20 {
		sample.Error = "خواندن پاسخ خواندنی کامل نشد یا پاسخ بیش از حد مجاز بود"
		return sample
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		sample.Error = "نشست از سوی دایا پذیرفته نشد؛ ادامه آزمایش متوقف شد"
	case http.StatusTooManyRequests:
		sample.Error = "دایا محدودیت درخواست اعلام کرد؛ ادامه آزمایش متوقف شد"
	case http.StatusOK:
		if _, validationErr := summarizeDayaOrderListResponse(resp.StatusCode, body); validationErr == nil {
			sample.ResponseValid = true
			sample.Authenticated = true
		} else {
			sample.Error = "پاسخ HTTP 200 دریافت شد اما ساختار JSON سفارش‌ها معتبر نبود"
		}
	default:
		sample.Error = fmt.Sprintf("پاسخ غیرمنتظره از دایا: HTTP %d", resp.StatusCode)
	}
	return sample
}

func summarizeDayaLatency(samples []dayaLatencySample) dayaLatencySummary {
	values := make([]int64, 0, len(samples))
	var total int64
	summary := dayaLatencySummary{
		Note: "RTT تا دریافت سرآیندهای پاسخ اندازه‌گیری شده است. نصف میانه فقط تخمین یک‌طرفه است و زمان POST یا ثبت سفارش را نشان نمی‌دهد.",
	}
	if len(samples) > 0 && samples[0].Mode == "authenticated" {
		summary.Note = "اندازه‌گیری با نشست رمزنگاری‌شده ذخیره‌شده انجام شده است. فقط پاسخ HTTP 200 با JSON معتبر، پاسخ معتبر و احراز هویت موفق شمرده می‌شود؛ هیچ POST سفارش ارسال نمی‌شود."
	} else {
		summary.Note += " پاسخ 401/403 به معنی دریافت پاسخ HTTP است، نه دسترسی موفق یا احراز هویت."
	}
	for _, sample := range samples {
		if sample.HTTPStatus > 0 {
			values = append(values, sample.RTTMillis)
			total += sample.RTTMillis
			if sample.ResponseValid {
				summary.SuccessCount++
			}
			if sample.HTTPStatus == http.StatusUnauthorized || sample.HTTPStatus == http.StatusForbidden {
				summary.RejectedCount++
			}
		}
		if sample.Error != "" {
			summary.ErrorCount++
		}
	}
	summary.Count = len(values)
	if len(values) == 0 {
		return summary
	}
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
	summary.MinRTTMillis = values[0]
	summary.MeanRTTMillis = float64(total) / float64(len(values))
	mid := len(values) / 2
	if len(values)%2 == 0 {
		summary.MedianRTTMillis = float64(values[mid-1]+values[mid]) / 2
	} else {
		summary.MedianRTTMillis = float64(values[mid])
	}
	p95 := int(float64(len(values))*0.95 + 0.999999)
	if p95 < 1 {
		p95 = 1
	}
	if p95 > len(values) {
		p95 = len(values)
	}
	summary.P95RTTMillis = values[p95-1]
	summary.EstimatedOneWayMS = summary.MedianRTTMillis / 2
	return summary
}
