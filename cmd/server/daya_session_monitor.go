package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	dayaOrdersURL = dayaAPIOrigin + "/orders"
	dayaMonitorMin = time.Minute
	dayaMonitorMax = 7 * time.Minute
)

type DayaMonitorSnapshot struct {
	Running bool `json:"running"`
	NeedsLogin bool `json:"needs_login"`
	LastCheck time.Time `json:"last_check,omitempty"`
	NextCheck time.Time `json:"next_check,omitempty"`
	LastStatus int `json:"last_status,omitempty"`
	Checks uint64 `json:"checks"`
	LastError string `json:"last_error,omitempty"`
}

type DayaSessionMonitor struct {
	mu sync.Mutex
	http *http.Client
	state DayaMonitorSnapshot
	cancel context.CancelFunc
	interval func() (time.Duration,error)
}

func NewDayaSessionMonitor(client *http.Client) *DayaSessionMonitor {
	if client == nil {
		client = &http.Client{Timeout:20*time.Second, CheckRedirect:func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &DayaSessionMonitor{http:client, interval:dayaRandomInterval}
}

func dayaRandomInterval() (time.Duration,error) {
	var b [8]byte
	if _,err:=rand.Read(b[:]); err!=nil { return 0,err }
	n:=binary.BigEndian.Uint64(b[:])
	span:=uint64((dayaMonitorMax-dayaMonitorMin)/time.Second)+1
	return dayaMonitorMin+time.Duration(n%span)*time.Second,nil
}

func (m *DayaSessionMonitor) Start(parent context.Context, token string, expires time.Time) error {
	if token=="" { return errors.New("نشست دایا خالی است") }
	if !expires.IsZero() && !expires.After(time.Now()) { return errors.New("نشست دایا منقضی شده است") }
	m.mu.Lock()
	if m.cancel!=nil { m.cancel() }
	ctx,cancel:=context.WithCancel(parent)
	m.cancel=cancel
	m.state=DayaMonitorSnapshot{Running:true}
	m.mu.Unlock()
	go m.run(ctx,token,expires)
	return nil
}

func (m *DayaSessionMonitor) Stop(needsLogin bool,message string) {
	m.mu.Lock(); defer m.mu.Unlock()
	if m.cancel!=nil { m.cancel(); m.cancel=nil }
	m.state.Running=false
	m.state.NeedsLogin=needsLogin
	m.state.NextCheck=time.Time{}
	m.state.LastError=message
}

func (m *DayaSessionMonitor) Snapshot() DayaMonitorSnapshot {
	m.mu.Lock(); defer m.mu.Unlock()
	return m.state
}

func (m *DayaSessionMonitor) run(ctx context.Context,token string,expires time.Time) {
	for {
		delay,err:=m.interval()
		if err!=nil { m.Stop(false,"تولید فاصله بررسی ناموفق بود"); return }
		m.mu.Lock(); m.state.NextCheck=time.Now().Add(delay); m.mu.Unlock()
		timer:=time.NewTimer(delay)
		select { case <-ctx.Done(): timer.Stop(); return; case <-timer.C: }
		if !expires.IsZero() && !time.Now().Before(expires) { m.Stop(true,"زمان اعتبار نشست به پایان رسیده است؛ ورود دستی لازم است"); return }
		req,err:=http.NewRequestWithContext(ctx,http.MethodGet,dayaOrdersURL,nil)
		if err!=nil { m.Stop(false,"ساخت درخواست بررسی نشست ناموفق بود"); return }
		req.Header.Set("Accept","application/json")
		req.AddCookie(&http.Cookie{Name:"token",Value:token})
		resp,err:=m.http.Do(req)
		now:=time.Now().UTC()
		if err!=nil {
			if ctx.Err()!=nil { return }
			m.mu.Lock(); m.state.LastCheck=now; m.state.LastError="خطای ارتباط؛ بررسی بعدی با فاصله تصادفی انجام می‌شود"; m.state.Checks++; m.mu.Unlock()
			continue
		}
		_,_=io.Copy(io.Discard,io.LimitReader(resp.Body,2<<20))
		_ = resp.Body.Close()
		m.mu.Lock(); m.state.LastCheck=now; m.state.LastStatus=resp.StatusCode; m.state.Checks++; m.state.LastError=""; m.mu.Unlock()
		switch resp.StatusCode {
		case http.StatusUnauthorized,http.StatusForbidden:
			m.Stop(true,"نشست توسط دایا پذیرفته نشد؛ ورود دستی با CAPTCHA لازم است"); return
		case http.StatusTooManyRequests:
			m.Stop(false,"دایا محدودیت درخواست اعلام کرد؛ پایش متوقف شد"); return
		}
		if resp.StatusCode>=500 { m.mu.Lock(); m.state.LastError="خطای موقت سرویس دایا"; m.mu.Unlock() }
	}
}
