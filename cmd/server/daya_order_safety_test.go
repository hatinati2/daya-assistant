package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDayaOrderAPIStaysDisabledUntilDispatchContractIsVerified(t *testing.T) {
	if dayaOrderAPICapable() {
		t.Fatal("live Daya order API must remain disabled until a separately reviewed dispatch contract exists")
	}
	if dayaOrderEndpoint != "https://mg.dayatrader.ir/orders" {
		t.Fatalf("observed endpoint changed unexpectedly: %q", dayaOrderEndpoint)
	}
}

func TestMarshalDayaOrderPayloadHasExactlyObservedFields(t *testing.T) {
	order := Order{
		ISIN: " iro3simc0001 ", Side: "buy", Price: 63390, Quantity: 300,
		ExecutionMode: executionModeAPI,
	}
	raw, err := MarshalDayaOrderPayload(order)
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	want := []string{
		"instrumentId", "orderPrice", "orderQuantity",
		"orderSide", "orderValidityDate", "orderValidityType",
	}
	if len(fields) != len(want) {
		t.Fatalf("got %d JSON fields, want exactly %d: %s", len(fields), len(want), raw)
	}
	for _, key := range want {
		if _, ok := fields[key]; !ok {
			t.Errorf("missing contract field %q in %s", key, raw)
		}
	}
	if string(fields["orderValidityDate"]) != "null" {
		t.Errorf("orderValidityDate=%s, want null", fields["orderValidityDate"])
	}
}

func TestDecodeDayaOrderErrorRejectsMalformedAndOutOfRangeStatus(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{name: "empty", body: nil},
		{name: "malformed JSON", body: []byte(`{"status":400`)},
		{name: "missing status", body: []byte(`{"path":"/orders","message":"rejected"}`)},
		{name: "success status", body: []byte(`{"status":201}`)},
		{name: "status below HTTP error range", body: []byte(`{"status":399}`)},
		{name: "status above HTTP range", body: []byte(`{"status":600}`)},
		{name: "oversized body", body: make([]byte, (64<<10)+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeDayaOrderError(tc.body); err == nil {
				t.Fatal("expected invalid error response to be rejected")
			}
		})
	}
}

func TestDecodeDayaOrderErrorUsesErrorWhenMessageIsMissing(t *testing.T) {
	got, err := DecodeDayaOrderError([]byte(`{"timestamp":1794085962787,"path":"/orders","status":400,"error":"group is closed","requestId":"local-test"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Message != "group is closed" {
		t.Fatalf("message=%q, want fallback from error field", got.Message)
	}
	if strings.TrimSpace(got.RequestID) == "" {
		t.Fatal("request ID should be preserved for support correlation")
	}
}
