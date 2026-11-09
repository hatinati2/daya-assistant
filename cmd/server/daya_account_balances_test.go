package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDayaNormalizeFinancialKey(t *testing.T) {
	for input, want := range map[string]string{
		"buyingPower":       "buyingpower",
		"cash_balance":      "cashbalance",
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
			"cashBalance":      json.Number("200000000"),
			"withdrawableCash": "150000000",
			"accountNumber":    "must-not-be-returned",
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
			"cashBalance":      json.Number("100"),
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

func TestDayaBuyingPowerRecognizesCreditField(t *testing.T) {
	buying := &dayaBalanceEndpointData{
		path:   "/financials/buying-power",
		status: 200,
		body: map[string]any{
			"credit":            json.Number("125000000"),
			"inProgressCashOut": json.Number("5000000"),
			"tPlus1Amount":      json.Number("7000000"),
			"tPlus2Amount":      json.Number("9000000"),
		},
	}
	got := dayaSelectBalanceMetric(
		buying, nil,
		dayaMakeAliasSet("buyingPower", "credit"),
		"قدرت خرید",
	)
	if got.Status != "ok" || got.Value != "125000000" || got.Field != "credit" {
		t.Fatalf("credit should be recognized as buying power: %#v", got)
	}

	// Do not reinterpret in-progress cash-out or settlement fields as
	// cash balance or withdrawable funds without verified semantics.
	cash := dayaSelectBalanceMetric(
		buying, nil,
		dayaMakeAliasSet("cashBalance", "totalCashBalance"),
		"موجودی نقدی",
	)
	withdrawable := dayaSelectBalanceMetric(
		buying, nil,
		dayaMakeAliasSet("withdrawableCash", "cashAvailableToWithdraw", "availableToWithdraw"),
		"موجودی قابل برداشت",
	)
	if cash.Status != "not_found" || withdrawable.Status != "not_found" {
		t.Fatalf("unverified fields must not be mapped: cash=%#v withdrawable=%#v", cash, withdrawable)
	}
}


func TestDayaCollectFinancialSchemaShowsSafePathsAndTypesOnly(t *testing.T) {
	payload := map[string]any{
		"data": []any{
			map[string]any{
				"cashAvailable": "6000000",
				"portfolio": map[string]any{"withdrawable": json.Number("4500000")},
				"accountNumber": "123456789",
				"customerName": "Sensitive Name",
			},
		},
	}
	var schema []string
	dayaCollectFinancialSchema(payload, "", &schema, 0)
	joined := strings.Join(schema, "\n")
	for _, expected := range []string{
		"data:array",
		"data[].cashAvailable:string",
		"data[].portfolio.withdrawable:number",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("schema missing %q in %q", expected, joined)
		}
	}
	for _, forbidden := range []string{"accountNumber", "customerName", "123456789", "Sensitive Name"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("sensitive information leaked: %q in %q", forbidden, joined)
		}
	}
}
