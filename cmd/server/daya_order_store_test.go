package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClaimReadyAPIOrdersClaimsOnlyEligibleOrders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orders.json")
	store, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 7, 10, 0, 0, 0, tehran())
	orders := []Order{
		{ID: "api-ready", ExecutionMode: executionModeAPI, ISIN: "IR0000000001", Status: statusReady},
		{ID: "api-without-isin", ExecutionMode: executionModeAPI, Status: statusReady},
		{ID: "manual-ready", ExecutionMode: executionModeManual, Status: statusReady},
		{ID: "api-scheduled", ExecutionMode: executionModeAPI, ISIN: "IR0000000002", Status: statusScheduled},
	}
	if err := store.addMany(orders); err != nil {
		t.Fatal(err)
	}

	claimed, err := store.claimReadyAPIOrders(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].ID != "api-ready" {
		t.Fatalf("claimed = %#v, want only api-ready", claimed)
	}
	if claimed[0].Status != statusSubmitting || !claimed[0].AttemptedAt.Equal(now) {
		t.Fatalf("claimed order state = %#v, want submitting at supplied time", claimed[0])
	}

	got := make(map[string]Order)
	for _, order := range store.snapshot() {
		got[order.ID] = order
	}
	if got["api-ready"].Status != statusSubmitting {
		t.Errorf("eligible API order status = %q, want %q", got["api-ready"].Status, statusSubmitting)
	}
	if got["api-without-isin"].Status != statusSubmissionFailed {
		t.Errorf("API order without ISIN status = %q, want %q", got["api-without-isin"].Status, statusSubmissionFailed)
	}
	if got["manual-ready"].Status != statusReady || got["api-scheduled"].Status != statusScheduled {
		t.Errorf("ineligible orders were changed: manual=%q scheduled=%q", got["manual-ready"].Status, got["api-scheduled"].Status)
	}
}

func TestCompleteAPIOrderPersistsFinalBrokerResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orders.json")
	store, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 7, 10, 1, 0, 0, tehran())
	if err := store.add(Order{ID: "api-1", ExecutionMode: executionModeAPI, Status: statusSubmitting}); err != nil {
		t.Fatal(err)
	}

	if err := store.completeAPIOrder("api-1", DayaOrderOutcome{
		Status: statusSubmitted, BrokerOrderID: "broker-123", RequestID: "request-123",
		HTTPStatus: 201, Message: "ثبت شد", ReceivedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.snapshot()
	if len(got) != 1 {
		t.Fatalf("reloaded order count = %d, want 1", len(got))
	}
	if got[0].Status != statusSubmitted || got[0].BrokerOrderID != "broker-123" || got[0].BrokerRequestID != "request-123" || got[0].BrokerHTTPStatus != 201 || got[0].BrokerMessage != "ثبت شد" || !got[0].BrokerResponseAt.Equal(now) || !got[0].UpdatedAt.Equal(now) {
		t.Fatalf("persisted completion = %#v", got[0])
	}
	if err := store.completeAPIOrder("api-1", DayaOrderOutcome{
		Status: statusSubmitted, BrokerOrderID: "broker-123", RequestID: "request-123",
		HTTPStatus: 201, Message: "ثبت شد", ReceivedAt: now,
	}); err == nil {
		t.Fatal("expected a second completion to be rejected")
	}
}

func TestCompleteAPIOrderRejectsInvalidFinalStatus(t *testing.T) {
	store := &Store{orders: []Order{{ID: "api-1", Status: statusSubmitting}}}
	err := store.completeAPIOrder("api-1", DayaOrderOutcome{Status: statusSubmitting, ReceivedAt: time.Now().In(tehran())})
	if err == nil {
		t.Fatal("expected non-final status to be rejected")
	}
	if store.orders[0].Status != statusSubmitting {
		t.Fatalf("invalid completion changed order state to %q", store.orders[0].Status)
	}
}

func TestRecoverInterruptedAPIOrdersMarksUnknownAndNeverReclaims(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orders.json")
	store, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.add(Order{ID: "interrupted", ExecutionMode: executionModeAPI, ISIN: "IR0000000003", Status: statusSubmitting}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 7, 10, 2, 0, 0, tehran())
	if err := store.recoverInterruptedAPIOrders(now); err != nil {
		t.Fatal(err)
	}

	reloaded, err := newStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.snapshot()
	if len(got) != 1 || got[0].Status != statusSubmissionUnknown || !got[0].UpdatedAt.Equal(now) {
		t.Fatalf("recovered state = %#v, want submission_unknown at supplied time", got)
	}
	claimed, err := reloaded.claimReadyAPIOrders(now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatalf("ambiguous order was reclaimed: %#v", claimed)
	}
}

func TestClaimReadyAPIOrdersRollsBackWhenPersistenceFails(t *testing.T) {
	blockedParent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	store := &Store{
		path: filepath.Join(blockedParent, "orders.json"),
		orders: []Order{{
			ID: "api-1", ExecutionMode: executionModeAPI, ISIN: "IR0000000004",
			Status: statusReady,
		}},
	}
	before := store.snapshot()
	_, err := store.claimReadyAPIOrders(time.Now().In(tehran()))
	if err == nil {
		t.Fatal("expected persistence error")
	}
	after := store.snapshot()
	if len(after) != len(before) || after[0].Status != before[0].Status || !after[0].UpdatedAt.Equal(before[0].UpdatedAt) {
		t.Fatalf("failed claim was not rolled back: before=%#v after=%#v", before, after)
	}
}

func TestReconcileAPIOrderRequiresBrokerIDToConfirmRegistration(t *testing.T) {
	store := &Store{orders: []Order{{ID: "unknown", ExecutionMode: executionModeAPI, Status: statusSubmissionUnknown}}}
	now := time.Now().In(tehran())
	if err := store.reconcileAPIOrder("unknown", "registered", "", "", now); err == nil {
		t.Fatal("expected broker order ID to be required")
	}
	if store.orders[0].Status != statusSubmissionUnknown {
		t.Fatalf("failed reconciliation changed state to %q", store.orders[0].Status)
	}
	if err := store.reconcileAPIOrder("unknown", "registered", "BRK-123", "verified in broker panel", now); err != nil {
		t.Fatal(err)
	}
	if store.orders[0].Status != statusSubmitted || store.orders[0].BrokerOrderID != "BRK-123" || !store.orders[0].ReconciledAt.Equal(now) {
		t.Fatalf("registered reconciliation not persisted: %#v", store.orders[0])
	}
}

func TestReconcileAPIOrderCanRecordManualRejection(t *testing.T) {
	store := &Store{orders: []Order{{ID: "unknown", ExecutionMode: executionModeAPI, Status: statusSubmissionUnknown}}}
	now := time.Now().In(tehran())
	if err := store.reconcileAPIOrder("unknown", "rejected", "", "ثبت نشد", now); err != nil {
		t.Fatal(err)
	}
	if store.orders[0].Status != statusSubmissionFailed || store.orders[0].BrokerMessage != "ثبت نشد" {
		t.Fatalf("rejected reconciliation not persisted: %#v", store.orders[0])
	}
}

func TestReconcileAPIOrderCannotChangeManualOrDefiniteOrders(t *testing.T) {
	store := &Store{orders: []Order{
		{ID: "manual", ExecutionMode: executionModeManual, Status: statusSubmissionUnknown},
		{ID: "failed", ExecutionMode: executionModeAPI, Status: statusSubmissionFailed},
	}}
	now := time.Now().In(tehran())
	for _, id := range []string{"manual", "failed"} {
		if err := store.reconcileAPIOrder(id, "registered", "BRK-1", "", now); err == nil {
			t.Errorf("expected reconciliation for %s to be rejected", id)
		}
	}
}
