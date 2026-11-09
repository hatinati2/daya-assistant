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

func TestDayaReportMapsOnlyTheFiveObservedBrokerFields(t *testing.T) {
	buying := &dayaBalanceEndpointData{
		path:   "/financials/buying-power",
		status: 200,
		body: map[string]any{
			"accountBalance":    json.Number("6000000"),
			"credit":            json.Number("0"),
			"inProgressCashOut": json.Number("0"),
			"tPlus1Amount":      json.Number("6000000"),
			"tPlus2Amount":      json.Number("6000000"),
		},
	}

	accountBalance := dayaSelectBalanceMetric(buying, nil, dayaMakeAliasSet("accountBalance"), "موجودی حساب")
	credit := dayaSelectBalanceMetric(buying, nil, dayaMakeAliasSet("credit"), "اعتبار")
	inProgressCashOut := dayaSelectBalanceMetric(buying, nil, dayaMakeAliasSet("inProgressCashOut"), "برداشت در حال انجام")
	tPlus1 := dayaSelectBalanceMetric(buying, nil, dayaMakeAliasSet("tPlus1Amount"), "T+1")
	tPlus2 := dayaSelectBalanceMetric(buying, nil, dayaMakeAliasSet("tPlus2Amount"), "T+2")

	for _, got := range []struct {
		name   string
		metric DayaAccountBalanceMetric
		want   string
		field  string
	}{
		{"accountBalance", accountBalance, "6000000", "accountBalance"},
		{"credit", credit, "0", "credit"},
		{"inProgressCashOut", inProgressCashOut, "0", "inProgressCashOut"},
		{"tPlus1Amount", tPlus1, "6000000", "tPlus1Amount"},
		{"tPlus2Amount", tPlus2, "6000000", "tPlus2Amount"},
	} {
		if got.metric.Status != "ok" || got.metric.Value != got.want || got.metric.Field != got.field {
			t.Fatalf("%s must pass through unchanged: %#v", got.name, got.metric)
		}
	}
}

func TestDayaCollectFinancialSchemaShowsSafePathsAndTypesOnly(t *testing.T) {
	payload := map[string]any{
		"data": []any{
			map[string]any{
				"accountBalance":    json.Number("6000000"),
				"credit":            json.Number("0"),
				"inProgressCashOut": json.Number("0"),
				"tPlus1Amount":      json.Number("6000000"),
				"tPlus2Amount":      json.Number("6000000"),
				"cashAvailable":     "6000000",
				"portfolio":         map[string]any{"withdrawable": json.Number("4500000")},
				"accountNumber":     "123456789",
				"customerName":      "Sensitive Name",
			},
		},
	}
	var schema []string
	dayaCollectFinancialSchema(payload, "", &schema, 0)
	joined := strings.Join(schema, "\n")
	for _, expected := range []string{
		"data:array",
		"data[].accountBalance:number",
		"data[].credit:number",
		"data[].inProgressCashOut:number",
		"data[].tPlus1Amount:number",
		"data[].tPlus2Amount:number",
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
