package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDayaNormalizeFinancialKey(t *testing.T) {
	for input, want := range map[string]string{
		"buyingPower": "buyingpower",
		"cash_balance": "cashbalance",
		"withdrawable-cash": "withdrawablecash",
	} {
		if got := dayaNormalizeFinancialKey(input); got != want {
			t.Fatalf("normalize %q = %q, want %q", input, got, want)
		}
	}
}

func TestDayaSelectBalanceMetricFindsNestedKnownFields(t *testing.T) {
	buying := &dayaBalanceEndpointData{
		path: "/financials/buying-power", status: 200,
		body: map[string]any{"data": map[string]any{"buyingPower": json.Number("123456789")}},
	}
	portfolio := &dayaBalanceEndpointData{
		path: "/financials/portfolio/instant", status: 200,
		body: map[string]any{"data": map[string]any{
			"cashBalance": json.Number("200000000"),
			"withdrawableCash": "150000000",
			"accountNumber": "must-not-be-returned",
		}},
	}
	gotBuying := dayaSelectBalanceMetric(buying, portfolio, dayaMakeAliasSet("buyingPower"), "قدرت خرید")
	gotCash := dayaSelectBalanceMetric(portfolio, buying, dayaMakeAliasSet("cashBalance"), "موجودی نقدی")
	gotWithdrawable := dayaSelectBalanceMetric(portfolio, buying, dayaMakeAliasSet("withdrawableCash"), "موجودی قابل برداشت")
	if gotBuying.Value != "123456789" || gotCash.Value != "200000000" || gotWithdrawable.Value != "150000000" {
		t.Fatalf("unexpected metrics: buying=%#v cash=%#v withdrawable=%#v", gotBuying, gotCash, gotWithdrawable)
	}
	encoded, err := json.Marshal([]DayaAccountBalanceMetric{gotBuying, gotCash, gotWithdrawable})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"accountNumber", "must-not-be-returned"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("sensitive payload leaked: %q in %s", forbidden, encoded)
		}
	}
}

func TestDayaSelectBalanceMetricHidesConflictingValues(t *testing.T) {
	endpoint := &dayaBalanceEndpointData{
		path: "/financials/portfolio/instant", status: 200,
		body: map[string]any{"data": map[string]any{
			"cashBalance": json.Number("100"),
			"totalCashBalance": json.Number("200"),
		}},
	}
	got := dayaSelectBalanceMetric(endpoint, nil, dayaMakeAliasSet("cashBalance", "totalCashBalance"), "موجودی نقدی")
	if got.Status != "ambiguous" || got.Value != "" {
		t.Fatalf("conflicting money fields must not be displayed: %#v", got)
	}
}

func TestDayaFinancialAmountAcceptsNumericValues(t *testing.T) {
	for _, input := range []any{json.Number("1234.50"), "1,234.50", "۱۲۳۴", int64(1234)} {
		if _, ok := dayaFinancialAmount(input); !ok {
			t.Fatalf("expected valid amount for %#v", input)
		}
	}
	for _, input := range []any{"abc", true, nil, map[string]any{}} {
		if _, ok := dayaFinancialAmount(input); ok {
			t.Fatalf("unexpected valid amount for %#v", input)
		}
	}
}

func TestDayaCollectUnit(t *testing.T) {
	got := dayaCollectUnit(map[string]any{"result": map[string]any{"currencyCode": "IRR"}}, 0)
	if got != "ریال" {
		t.Fatalf("unit = %q, want ریال", got)
	}
}
