package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildDayaOrderPayloadMatchesObservedContract(t *testing.T) {
	order := Order{
		ISIN:          "IRO3SIMC0001",
		Side:          "buy",
		Price:         63390,
		Quantity:      9999,
		ExecutionMode: executionModeAPI,
	}
	payload, err := BuildDayaOrderPayload(order)
	if err != nil {
		t.Fatal(err)
	}
	if payload.InstrumentID != "IRO3SIMC0001" ||
		payload.OrderPrice != 63390 ||
		payload.OrderQuantity != 9999 ||
		payload.OrderSide != "BUY" ||
		payload.OrderValidityDate != nil ||
		payload.OrderValidityType != "JUST_TODAY" {
		t.Fatalf("unexpected Daya payload: %#v", payload)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, field := range []string{
		`"instrumentId":"IRO3SIMC0001"`,
		`"orderPrice":63390`,
		`"orderQuantity":9999`,
		`"orderSide":"BUY"`,
		`"orderValidityDate":null`,
		`"orderValidityType":"JUST_TODAY"`,
	} {
		if !strings.Contains(text, field) {
			t.Fatalf("serialized payload missing %s: %s", field, text)
		}
	}
}

func TestBuildDayaOrderPayloadSell(t *testing.T) {
	order := Order{
		ISIN:          "IRO3SIMC0001",
		Side:          "sell",
		Price:         10,
		Quantity:      2,
		ExecutionMode: executionModeAPI,
	}
	payload, err := BuildDayaOrderPayload(order)
	if err != nil {
		t.Fatal(err)
	}
	if payload.OrderSide != "SELL" {
		t.Fatalf("side=%q, want SELL", payload.OrderSide)
	}
}

func TestBuildDayaOrderPayloadRejectsInvalidInputs(t *testing.T) {
	base := Order{
		ISIN:          "IRO3SIMC0001",
		Side:          "buy",
		Price:         10,
		Quantity:      2,
		ExecutionMode: executionModeAPI,
	}
	tests := []struct {
		name string
		edit func(*Order)
	}{
		{name: "manual mode", edit: func(o *Order) { o.ExecutionMode = executionModeManual }},
		{name: "invalid instrument", edit: func(o *Order) { o.ISIN = "bad" }},
		{name: "zero price", edit: func(o *Order) { o.Price = 0 }},
		{name: "zero quantity", edit: func(o *Order) { o.Quantity = 0 }},
		{name: "invalid side", edit: func(o *Order) { o.Side = "hold" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			order := base
			test.edit(&order)
			if _, err := BuildDayaOrderPayload(order); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDecodeDayaOrderGroupStateError(t *testing.T) {
	raw := []byte(`{"timestamp":1794085962787,"path":"/orders","status":400,"error":"امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد","requestId":"sample-request","message":"امکان ثبت سفارش با توجه به وضعیت گروه وجود ندارد"}`)
	got, err := DecodeDayaOrderError(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != 400 || got.Path != "/orders" ||
		got.RequestID != "sample-request" ||
		!strings.Contains(got.Message, "وضعیت گروه") {
		t.Fatalf("unexpected decoded error: %#v", got)
	}
}

func TestDecodeDayaOrderErrorRejectsSuccessAndOversizedBody(t *testing.T) {
	if _, err := DecodeDayaOrderError([]byte(`{"status":200}`)); err == nil {
		t.Fatal("expected non-error status to be rejected")
	}
	if _, err := DecodeDayaOrderError(make([]byte, (64<<10)+1)); err == nil {
		t.Fatal("expected oversized body to be rejected")
	}
}
