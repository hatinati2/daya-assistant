package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"
)

type dayaDashboardSummary struct {
	CurrentTime       string              `json:"current_time"`
	ScheduledOrders   int                 `json:"scheduled_orders"`
	ReadyOrders       int                 `json:"ready_orders"`
	HistoryOrders     int                 `json:"history_orders"`
	TotalOrders       int                 `json:"total_orders"`
	CredentialsSaved  bool                `json:"credentials_saved"`
	SessionAvailable  bool                `json:"session_available"`
	SessionExpiresAt  string              `json:"session_expires_at,omitempty"`
	Monitor           DayaMonitorSnapshot `json:"monitor"`
	APIDispatchActive bool                `json:"api_dispatch_active"`
}

type dayaDashboardPageData struct {
	Summary dayaDashboardSummary
}

var dayaDashboardPage = template.Must(template.New("daya-dashboard").Parse(`
<!doctype html>
<html lang="fa" dir="rtl">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>داشبورد مرکزی دایا · Preview 03</title>
<style>
*{box-sizing:border-box}body{margin:0;background:#f4f6fa;color:#182230;font:14px Tahoma,Arial}
main{max-width:1180px;margin:24px auto;padding:0 16px}.head{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap}
h1{font-size:22px;margin:0}.muted,.foot{color:#667085;line-height:1.8}.nav{display:flex;gap:7px;flex-wrap:wrap;margin:14px 0}
.nav a,.link{display:inline-block;padding:8px 11px;border:1px solid #d0d5dd;border-radius:9px;color:#344054;background:#fff;text-decoration:none}
.nav a.active{background:#175cd3;color:#fff;border-color:#175cd3}
.grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin:14px 0}
.stat,.card{background:#fff;border:1px solid #e4e7ec;border-radius:13px;padding:15px;min-width:0}
.stat-label{color:#667085;font-size:12px}.stat-value{font-size:25px;font-weight:bold;margin-top:7px;direction:ltr;text-align:right;font-variant-numeric:tabular-nums}
.sections{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}
.card h2{font-size:15px;margin:0 0 8px}.card p{color:#667085;line-height:1.85;margin:5px 0 12px}
.card a{color:#175cd3;text-decoration:none;font-weight:bold}.card .foot{font-size:11px}
.status{padding:10px 12px;border-radius:9px;background:#eff8ff;color:#1849a9;line-height:1.8;margin:12px 0}
.warn{padding:12px;border:1px solid #fedf89;background:#fffaeb;color:#93370d;border-radius:9px;line-height:1.9;margin:12px 0}
.danger{border-color:#fecdca}.danger a{color:#b42318}
@media(max-width:850px){.grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:560px){.sections{grid-template-columns:1fr}.grid{grid-template-columns:repeat(2,minmax(0,1fr))}.stat-value{font-size:21px}}
</style>
</head>
<body><main>
<header class="head">
 <div><h1>داشبورد مرکزی دایا</h1><div class="muted">Preview 03 · پنل متمرکز ابزارها و وضعیت سامانه</div></div>
 <div class="muted">ساعت سرور تهران: <b id="current-time">{{.Summary.CurrentTime}}</b></div>
</header>
<nav class="nav">
 <a class="active" href="/dashboard">داشبورد مرکزی</a>
 <a href="/">مدیریت سفارش‌ها</a>
 <a href="/history">تاریخچه</a>
 <a href="/settings">اتصال و نشست دایا</a>
 <a href="/daya-online-test">تست آنلاین خواندنی</a>
 <a href="/daya-latency">کالیبراسیون تأخیر</a>
</nav>
<div class="grid">
 <div class="stat"><div class="stat-label">سفارش‌های زمان‌بندی‌شده</div><div class="stat-value" id="scheduled-orders">{{.Summary.ScheduledOrders}}</div></div>
 <div class="stat"><div class="stat-label">سفارش‌های آماده / غیرزمان‌بندی‌شده</div><div class="stat-value" id="ready-orders">{{.Summary.ReadyOrders}}</div></div>
 <div class="stat"><div class="stat-label">کل رکوردهای سفارش</div><div class="stat-value" id="total-orders">{{.Summary.TotalOrders}}</div></div>
 <div class="stat"><div class="stat-label">نشست ذخیره‌شده دایا</div><div class="stat-value" id="session-state">{{if .Summary.SessionAvailable}}موجود{{else}}نیازمند ورود{{end}}</div></div>
</div>
<div class="status" id="session-monitor-state">
{{if .Summary.Monitor.Running}}پایش نشست فعال است.{{else if .Summary.Monitor.NeedsLogin}}نشست معتبر نیست؛ ورود دستی با CAPTCHA لازم است.{{else}}پایش نشست در حال حاضر فعال نیست.{{end}}
آخرین HTTP: <b id="monitor-http">{{if .Summary.Monitor.LastStatus}}{{.Summary.Monitor.LastStatus}}{{else}}—{{end}}</b>
· تعداد بررسی‌ها: <b id="monitor-checks">{{.Summary.Monitor.Checks}}</b>
</div>
<div class="foot" id="session-expiry">انقضای ثبت‌شده نشست: {{if .Summary.SessionExpiresAt}}{{.Summary.SessionExpiresAt}}{{else}}نامشخص / نشست موجود نیست{{end}}</div>
<div class="warn">این داشبورد امکانات را یک‌جا گردآوری می‌کند. تست آنلاین و کالیبراسیون نشست فقط GET خواندنی اجرا می‌کنند. ارسال خودکار سفارش از API همچنان غیرفعال است. بخش «ارسال یک سفارش واقعی» جداگانه و فقط با تأیید صریح عمل می‌کند.</div>
<div class="sections">
 <section class="card">
  <h2>مدیریت سفارش و زمان‌بندی</h2>
  <p>تعریف سفارش، انتخاب نماد و ISIN، تقویم شمسی، ساعت هماهنگ سرور، برنامه‌ریز چندنوبتی، لغو و مدیریت سفارش‌های فعال.</p>
  <a href="/">ورود به فضای مدیریت سفارش‌ها ←</a>
 </section>
 <section class="card">
  <h2>تاریخچه و تطبیق سفارش</h2>
  <p>مشاهده نتایج قبلی، سفارش‌های آماده، بررسی وضعیت‌های مبهم و ثبت نتیجه بررسی دستی.</p>
  <a href="/history">مشاهده تاریخچه ←</a>
 </section>
 <section class="card">
  <h2>اتصال، ورود و پایش نشست</h2>
  <p>مدیریت اطلاعات ورود رمزنگاری‌شده، دریافت CAPTCHA، ورود دستی، وضعیت نشست و بررسی دو endpoint فقط‌خواندنی.</p>
  <a href="/settings">مدیریت اتصال دایا ←</a>
 </section>
 <section class="card">
  <h2>تست آنلاین خواندنی</h2>
  <p>استفاده از نشست رمزنگاری‌شده موجود برای GET /orders، اعتبارسنجی پاسخ JSON و نمایش شمارش کلی رکوردها بدون نمایش داده خام.</p>
  <a href="/daya-online-test">اجرای تست آنلاین ←</a>
 </section>
 <section class="card">
  <h2>کالیبراسیون ارتباط</h2>
  <p>مقایسه RTT درخواست بدون نشست با RTT درخواست احراز هویت‌شده با نشست ذخیره‌شده؛ تفکیک پاسخ HTTP از پاسخ معتبر ۲۰۰.</p>
  <a href="/daya-latency">باز کردن کالیبراتور دوحالته ←</a>
 </section>
 <section class="card">
  <h2>آزمایش محلی API</h2>
  <p>ساخت و بررسی Payload در برابر سرور ساختگی روی همین ماشین؛ بدون اتصال به کارگزاری و بدون استفاده از نشست.</p>
  <a href="/daya-api-test">اجرای تست ساختگی محلی ←</a>
 </section>
 <section class="card danger">
  <h2>ارسال دستی یک سفارش واقعی</h2>
  <p><b>عملیات پرریسک:</b> این مسیر یک POST واقعی به دایا می‌فرستد. سفارش ممکن است ثبت شود. از آن فقط در صورت نیاز و پس از بررسی تمام مشخصات استفاده کن.</p>
  <a href="/daya-online-test/one">رفتن به فرم ارسال یک‌باره ←</a>
 </section>
 <section class="card">
  <h2>وضعیت ارسال خودکار API</h2>
  <p id="api-dispatch-state">{{if .Summary.APIDispatchActive}}قابلیت ارسال API فعال است.{{else}}ارسال خودکار از مسیر API غیرفعال است و این داشبورد آن را فعال نمی‌کند.{{end}}</p>
  <div class="foot">داشبورد فقط دسترسی به امکانات موجود را متمرکز می‌کند و تنظیمات معاملاتی یا رفتار dispatcher را تغییر نمی‌دهد.</div>
 </section>
</div>
<script>
(async()=>{
 const ids={scheduled:document.getElementById("scheduled-orders"),ready:document.getElementById("ready-orders"),total:document.getElementById("total-orders"),session:document.getElementById("session-state"),monitor:document.getElementById("session-monitor-state"),http:document.getElementById("monitor-http"),checks:document.getElementById("monitor-checks"),expiry:document.getElementById("session-expiry"),clock:document.getElementById("current-time")};
 async function refresh(){
  try{
   const r=await fetch("/api/dashboard/summary",{cache:"no-store",headers:{"Accept":"application/json"}});
   if(!r.ok)return;
   const d=await r.json();
   ids.scheduled.textContent=String(d.scheduled_orders);
   ids.ready.textContent=String(d.ready_orders);
   ids.total.textContent=String(d.total_orders);
   ids.session.textContent=d.session_available?"موجود":"نیازمند ورود";
   ids.http.textContent=d.monitor.last_status?String(d.monitor.last_status):"—";
   ids.checks.textContent=String(d.monitor.checks||0);
   ids.monitor.textContent=(d.monitor.running?"پایش نشست فعال است.":(d.monitor.needs_login?"نشست معتبر نیست؛ ورود دستی با CAPTCHA لازم است.":"پایش نشست فعال نیست."))+" آخرین HTTP: ";
   ids.http.textContent=d.monitor.last_status?String(d.monitor.last_status):"—";
   ids.monitor.append(ids.http);
   ids.monitor.append(document.createTextNode(" · تعداد بررسی‌ها: "));
   ids.monitor.append(ids.checks);
   ids.expiry.textContent="انقضای ثبت‌شده نشست: "+(d.session_expires_at||"نامشخص / نشست موجود نیست");
   ids.clock.textContent=d.current_time;
  }catch(_){}
 }
 refresh();window.setInterval(refresh,15000);
})();
</script>
</main></body></html>
`))

func buildDayaDashboardSummary(store *Store, credentials *DayaCredentialStore, monitor *DayaSessionMonitor, now time.Time) dayaDashboardSummary {
	summary := dayaDashboardSummary{
		CurrentTime:       formatJalaliDateTime(now.In(tehran())),
		APIDispatchActive: dayaOrderAPICapable(),
	}
	if store != nil {
		orders := store.snapshot()
		summary.TotalOrders = len(orders)
		for _, order := range orders {
			if order.Status == statusScheduled {
				summary.ScheduledOrders++
			} else if order.Status == statusReady {
				summary.ReadyOrders++
			} else {
				summary.HistoryOrders++
			}
		}
	}
	if credentials != nil {
		_, saved, err := credentials.Status()
		summary.CredentialsSaved = err == nil && saved
		session, _, loadErr := credentials.LoadAccessSession()
		if loadErr == nil && session.Token != "" && (session.ExpiresAt.IsZero() || session.ExpiresAt.After(now)) {
			summary.SessionAvailable = true
			if !session.ExpiresAt.IsZero() {
				summary.SessionExpiresAt = session.ExpiresAt.In(tehran()).Format("2006-01-02 15:04:05 MST")
			}
		}
	}
	if monitor != nil {
		summary.Monitor = monitor.Snapshot()
	}
	return summary
}

func registerDayaDashboardRoutes(mux *http.ServeMux, previewMode bool, store *Store, credentials *DayaCredentialStore, monitor *DayaSessionMonitor) {
	mux.HandleFunc("GET /dashboard", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		summary := buildDayaDashboardSummary(store, credentials, monitor, time.Now().In(tehran()))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = dayaDashboardPage.Execute(w, dayaDashboardPageData{Summary: summary})
	})
	mux.HandleFunc("GET /api/dashboard/summary", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		summary := buildDayaDashboardSummary(store, credentials, monitor, time.Now().In(tehran()))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(summary)
	})
}
