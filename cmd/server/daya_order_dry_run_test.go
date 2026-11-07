package main

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestValidateOrderDryRunAcceptsAndNeverSelectsAPI(t *testing.T) {
	now := time.Date(2026, time.November, 5, 8, 40, 0, 0, tehran())
	form := url.Values{
		"symbol":          {"TEST"},
		"side":            {"buy"},
		"price":           {"1000"},
		"quantity":        {"10"},
		"execute_at_date": {"۱۴۰۵/۰۸/۱۴"},
		"execute_time":    {"08:44:50"},
		"execution_mode":  {"api"},
		"isin":            {"IR0000000000"},
	}
	req := httptest.NewRequest("POST", "/api/orders/dry-run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	result, err := validateOrderDryRun(req, now)
	if err != nil {
		t.Fatalf("validateOrderDryRun: %v", err)
	}
	if result.Mode != "dry_run" || result.Symbol != "TEST" || result.Side != "buy" {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if result.Price != 1000 || result.Quantity != 10 || result.Notional != "10000" {
		t.Fatalf("unexpected amounts: %+v", result)
	}
	if result.ExecuteAt != "۱۴۰۵/۰۸/۱۴ ۰۸:۴۴:۵۰٫۰۰۰" {
		t.Fatalf("execute_at = %q", result.ExecuteAt)
	}
}

func TestValidateOrderDryRunRejectsInvalidOrder(t *testing.T) {
	now := time.Date(2026, time.November, 5, 8, 40, 0, 0, tehran())
	form := url.Values{
		"symbol":          {"TEST"},
		"side":            {"buy"},
		"price":           {"0"},
		"quantity":        {"10"},
		"execute_at_date": {"۱۴۰۵/۰۸/۱۴"},
		"execute_time":    {"08:44:50"},
	}
	req := httptest.NewRequest("POST", "/api/orders/dry-run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := validateOrderDryRun(req, now); err == nil {
		t.Fatal("expected invalid price to be rejected")
	}
}
