package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

const dayaLatencyProbeURL = "https://mg.dayatrader.ir/orders"

type dayaLatencySample struct {
	HTTPStatus int    `json:"http_status"`
	RTTMillis  int64  `json:"rtt_ms"`
	StartedAt  string `json:"started_at"`
	EndedAt    string `json:"ended_at"`
	Error      string `json:"error,omitempty"`
}

type dayaLatencySummary struct {
	Count             int     `json:"count"`
	MinRTTMillis      int64   `json:"min_rtt_ms"`
	MedianRTTMillis   float64 `json:"median_rtt_ms"`
	MeanRTTMillis     float64 `json:"mean_rtt_ms"`
	P95RTTMillis      int64   `json:"p95_rtt_ms"`
	EstimatedOneWayMS float64 `json:"estimated_one_way_ms"`
	Note              string  `json:"note"`
}

type dayaLatencyResponse struct {
	Samples []dayaLatencySample `json:"samples"`
	Summary dayaLatencySummary  `json:"summary"`
}

var dayaLatencyPage = template.Must(template.New("daya-latency").Parse(`<!doctype html>
<html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>کالیبره‌کردن تأخیر دایا</title><style>
*{box-sizing:border-box}body{margin:0;background:#f4f6fa;color:#182230;font:14px Tahoma,Arial}
main{max-width:820px;margin:28px auto;padding:0 16px}.card{background:#fff;border:1px solid #e4e7ec;border-radius:13px;padding:20px;margin:14px 0}
h1{font-size:20px}.muted{color:#667085;line-height:2}.stats{display:grid;grid-template-columns:repeat(2,1fr);gap:10px}
.stat{background:#f8fafc;border-radius:9px;padding:12px}.value{font-size:22px;font-weight:bold;direction:ltr;text-align:right}
button{background:#175cd3;color:#fff;border:0;border-radius:8px;padding:12px 18px;font:inherit;cursor:pointer}
button:disabled{opacity:.5}.warning{padding:12px;background:#fffaeb;border:1px solid #fedf89;border-radius:9px;line-height:2}
table{width:100%;border-collapse:collapse}td,th{padding:8px;border-bottom:1px solid #eaecf0;text-align:right}
@media(max-width:560px){.stats{grid-template-columns:1fr}}
</style></head><body><main>
<h1>کالیبره‌کردن تأخیر ارتباط با دایا</h1>
<p class="muted">این ابزار فقط درخواست GET خواندنی به مسیر سفارش‌ها می‌فرستد و زمان رفت‌وبرگشت را روی همین سرور اندازه می‌گیرد. هیچ سفارشی ارسال، ثبت یا تغییری داده نمی‌شود.</p>
<section class="card">
<div class="warning">زمان RTT شامل رفت‌وبرگشت شبکه و پردازش مقصد است؛ نصف‌کردن آن فقط تخمین تقریبی تأخیر یک‌طرفه است. این آزمایش با POST سفارش یکسان نیست و زمان رسیدن سفارش را تضمین نمی‌کند.</div>
<p><button id="run">اجرای ۱۰ نمونه</button> <span id="state" class="muted"></span></p>
<div id="summary" class="stats"></div>
<p id="note" class="muted"></p>
</section>
<section class="card"><h2>نمونه‌ها</h2><table><thead><tr><th>#</th><th>HTTP</th><th>RTT (ms)</th><th>زمان پایان (تهران)</th></tr></thead><tbody id="rows"></tbody></table></section>
<script>
const run=document.getElementById('run'),state=document.getElementById('state');
run.addEventListener('click',async()=>{run.disabled=true;state.textContent='در حال اندازه‌گیری…';
try{const r=await fetch('/api/daya/latency-samples?count=10',{cache:'no-store'});const d=await r.json();if(!r.ok)throw new Error(d.error||'خطا در اندازه‌گیری');
const s=d.summary;const cards=[['تعداد نمونه',s.count],['میانه RTT',s.median_rtt_ms.toFixed(1)+' ms'],['میانگین RTT',s.mean_rtt_ms.toFixed(1)+' ms'],['صدک ۹۵ RTT',s.p95_rtt_ms+' ms'],['کمترین RTT',s.min_rtt_ms+' ms'],['تخمین یک‌طرفه',s.estimated_one_way_ms.toFixed(1)+' ms']];
document.getElementById('summary').innerHTML=cards.map(x=>'<div class="stat"><div class="muted">'+x[0]+'</div><div class="value">'+x[1]+'</div></div>').join('');
document.getElementById('note').textContent=s.note;
document.getElementById('rows').innerHTML=d.samples.map((x,i)=>'<tr><td>'+(i+1)+'</td><td>'+x.http_status+'</td><td>'+x.rtt_ms+'</td><td>'+x.ended_at+'</td></tr>').join('');
state.textContent='اندازه‌گیری تمام شد';
}catch(e){state.textContent=e.message}finally{run.disabled=false}});
</script></main></body></html>`))

func registerDayaLatencyRoutes(mux *http.ServeMux, previewMode bool) {
	mux.HandleFunc("GET /daya-latency", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode { http.NotFound(w, r); return }
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = dayaLatencyPage.Execute(w, nil)
	})
	mux.HandleFunc("GET /api/daya/latency-samples", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode { http.NotFound(w, r); return }
		count := 10
		if raw := r.URL.Query().Get("count"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 3 || n > 20 {
				http.Error(w, `{"error":"تعداد نمونه باید بین ۳ تا ۲۰ باشد"}`, http.StatusBadRequest)
				return
			}
			count = n
		}
		result := dayaLatencyResponse{Samples: make([]dayaLatencySample, 0, count)}
		for i := 0; i < count; i++ {
			result.Samples = append(result.Samples, probeDayaLatency(r.Context()))
			if i+1 < count { time.Sleep(250 * time.Millisecond) }
		}
		result.Summary = summarizeDayaLatency(result.Samples)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	})
}

func probeDayaLatency(parent context.Context) dayaLatencySample {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	start := time.Now()
	sample := dayaLatencySample{StartedAt: start.In(tehran()).Format(time.RFC3339Nano)}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dayaLatencyProbeURL, nil)
	if err != nil { sample.Error = "ساخت درخواست ناموفق بود"; return sample }
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "DayaAssistant-LatencyProbe/1.0")
	client := &http.Client{Timeout: 5*time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	end := time.Now()
	sample.RTTMillis = end.Sub(start).Milliseconds()
	sample.EndedAt = end.In(tehran()).Format(time.RFC3339Nano)
	if err != nil { sample.Error = "ارتباط با endpoint کامل نشد"; return sample }
	sample.HTTPStatus = resp.StatusCode
	_ = resp.Body.Close()
	return sample
}

func summarizeDayaLatency(samples []dayaLatencySample) dayaLatencySummary {
	values := make([]int64, 0, len(samples))
	var total int64
	for _, sample := range samples {
		if sample.Error == "" && sample.HTTPStatus > 0 {
			values = append(values, sample.RTTMillis)
			total += sample.RTTMillis
		}
	}
	summary := dayaLatencySummary{Count: len(values), Note: "تخمین یک‌طرفه برابر نصف میانه RTT است و فقط شاخص تقریبی شبکه است؛ برای زمان‌بندی ارسال سفارش واقعی باید اختلاف زمان شروع POST تا ثبت کارگزاری جداگانه اندازه‌گیری شود."}
	if len(values) == 0 { return summary }
	// Insertion sort keeps this small, bounded sample set dependency-free.
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- { values[j], values[j-1] = values[j-1], values[j] }
	}
	summary.MinRTTMillis = values[0]
	summary.MeanRTTMillis = float64(total) / float64(len(values))
	mid := len(values)/2
	if len(values)%2 == 0 { summary.MedianRTTMillis = float64(values[mid-1]+values[mid])/2 } else { summary.MedianRTTMillis = float64(values[mid]) }
	p95 := int(float64(len(values))*0.95 + 0.999999)
	if p95 < 1 { p95 = 1 }; if p95 > len(values) { p95 = len(values) }
	summary.P95RTTMillis = values[p95-1]
	summary.EstimatedOneWayMS = summary.MedianRTTMillis / 2
	if summary.Count == 0 { summary.Note = fmt.Sprintf("هیچ نمونه موفقی ثبت نشد؛ خطاهای شبکه را بررسی کنید.") }
	return summary
}
