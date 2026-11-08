package main

import (
	"net/http"
	"strings"
	"time"
)

// registerDayaOrderReconciliationRoute lets an authenticated panel operator
// resolve an ambiguous API result after checking the official Daya panel.
// It never contacts the broker and never triggers another submission.
func registerDayaOrderReconciliationRoute(mux *http.ServeMux, store *Store) {
	mux.HandleFunc("POST /orders/reconcile", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			http.Error(w, "درخواست نامعتبر", http.StatusForbidden)
			return
		}
		if store == nil {
			http.Error(w, "ذخیره سفارش‌ها در دسترس نیست", http.StatusInternalServerError)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "فرم نتیجه بررسی معتبر نیست", http.StatusBadRequest)
			return
		}
		id := strings.TrimSpace(r.FormValue("id"))
		resolution := strings.TrimSpace(r.FormValue("resolution"))
		brokerOrderID := strings.TrimSpace(r.FormValue("broker_order_id"))
		message := strings.TrimSpace(r.FormValue("broker_message"))
		if err := store.reconcileAPIOrder(id, resolution, brokerOrderID, message, time.Now().In(tehran())); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/history?notice=reconciled", http.StatusSeeOther)
	})
}
