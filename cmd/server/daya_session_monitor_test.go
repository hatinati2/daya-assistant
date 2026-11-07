package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDayaRandomIntervalBounds(t *testing.T) {
	for i:=0;i<100;i++ {
		d,err:=dayaRandomInterval()
		if err!=nil { t.Fatal(err) }
		if d<time.Minute || d>7*time.Minute { t.Fatalf("interval out of bounds: %v",d) }
	}
}

func TestDayaMonitorStopsOnUnauthorized(t *testing.T) {
	srv:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
		if r.Method!=http.MethodGet || r.URL.Path!="/orders" { t.Errorf("unexpected request %s %s",r.Method,r.URL.Path) }
		if c,err:=r.Cookie("token"); err!=nil || c.Value!="test-token" { t.Errorf("token cookie missing") }
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	client:=srv.Client()
	client.Transport=roundTripFunc(func(req *http.Request)(*http.Response,error) {
		clone:=req.Clone(req.Context()); clone.URL,_=clone.URL.Parse(srv.URL+"/orders")
		return srv.Client().Transport.RoundTrip(clone)
	})
	m:=NewDayaSessionMonitor(client)
	m.interval=func()(time.Duration,error){ return time.Millisecond,nil }
	if err:=m.Start(context.Background(),"test-token",time.Now().Add(time.Hour)); err!=nil { t.Fatal(err) }
	deadline:=time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if m.Snapshot().NeedsLogin { return }
		time.Sleep(time.Millisecond)
	}
	t.Fatal("monitor did not stop for 401")
}

type roundTripFunc func(*http.Request)(*http.Response,error)
func (f roundTripFunc) RoundTrip(r *http.Request)(*http.Response,error){ return f(r) }
