package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hatinati2/daya-assistant/internal/config"
)

func main() {
	cfg := config.Load()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"daya-assistant"}`))
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>دستیار دیا</title><style>body{font-family:Tahoma,sans-serif;margin:0;background:#f5f7fb;color:#182230}.wrap{max-width:760px;margin:10vh auto;padding:24px}.card{background:white;border:1px solid #e5eaf1;border-radius:16px;padding:28px;box-shadow:0 8px 30px #1822300b}h1{margin-top:0}.muted{color:#667085}</style></head><body><main class="wrap"><section class="card"><h1>دستیار دیا</h1><p>زیرساخت اولیه با موفقیت اجرا شده است.</p><p class="muted">اتصال کارگزاری و مدیریت سفارش‌ها هنوز پیاده‌سازی نشده‌اند.</p></section></main></body></html>`))
	})

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Daya Assistant listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		_ = server.Close()
	}
	_ = time.Second // keep time imported for explicit duration semantics in this scaffold
}
