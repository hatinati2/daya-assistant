package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteDayaLocalOrderTestSimulatesAcceptedResponse(t *testing.T) {
	order := Order{
		ISIN: "IRO3SIMC0001", Side: "buy", Price: 63390, Quantity: 1,
		ExecutionMode: executionModeAPI,
	}
	result, err := executeDayaLocalOrderTest(context.Background(), order, "accepted")
	if err != nil {
		t.Fatal(err)
	}
	if result.HTTPStatus != http.StatusCreated {
		t.Fatalf("HTTPStatus=%d, want %d", result.HTTPStatus, http.StatusCreated)
	}
	for _, expected := range []string{
		"\"instrumentId\": \"IRO3SIMC0001\"",
		"\"orderPrice\": 63390",
		"\"orderQuantity\": 1",
		"\"orderSide\": \"BUY\"",
		"\"orderValidityDate\": null",
		"\"orderValidityType\": \"JUST_TODAY\"",
	} {
		if !strings.Contains(result.RequestJSON, expected) {
			t.Fatalf("request JSON missing %q: %s", expected, result.RequestJSON)
		}
	}
	if !strings.Contains(result.ResponseBody, "LOCAL-MOCK-201") {
		t.Fatalf("response is not marked as local mock: %s", result.ResponseBody)
	}
	if strings.Contains(result.ResponseBody, "mg.dayatrader.ir") {
		t.Fatal("mock response must not mention or contact the live broker host")
	}
}

func TestExecuteDayaLocalOrderTestSimulatesRejectedResponse(t *testing.T) {
	order := Order{
		ISIN: "IRO3SIMC0001", Side: "sell", Price: 100, Quantity: 2,
		ExecutionMode: executionModeAPI,
	}
	result, err := executeDayaLocalOrderTest(context.Background(), order, "rejected")
	if err != nil {
		t.Fatal(err)
	}
	if result.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("HTTPStatus=%d, want %d", result.HTTPStatus, http.StatusBadRequest)
	}
	if !strings.Contains(result.ResponseBody, "LOCAL-MOCK-400") ||
		!strings.Contains(result.ResponseBody, "پاسخ ساختگی") {
		t.Fatalf("response is not clearly marked as a mock rejection: %s", result.ResponseBody)
	}
}

func TestDayaLocalOrderTestRoutesArePreviewOnly(t *testing.T) {
	mux := http.NewServeMux()
	registerDayaLocalOrderTestRoutes(mux, false)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/daya-api-test", nil)
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestDayaLocalOrderTestPageAndPostWorkInPreview(t *testing.T) {
	mux := http.NewServeMux()
	registerDayaLocalOrderTestRoutes(mux, true)

	getRecorder := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/daya-api-test", nil)
	mux.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK || !strings.Contains(getRecorder.Body.String(), "آزمایش محلی") {
		t.Fatalf("GET status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}

	form := "isin=IRO3SIMC0001&side=buy&price=63390&quantity=1&mock_response=accepted"
	postRecorder := httptest.NewRecorder()
	postRequest := httptest.NewRequest(http.MethodPost, "/daya-api-test", strings.NewReader(form))
	postRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(postRecorder, postRequest)
	if postRecorder.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", postRecorder.Code, postRecorder.Body.String())
	}
	body := postRecorder.Body.String()
	if !strings.Contains(body, "LOCAL-MOCK-201") || !strings.Contains(body, "هیچ اتصالی به کارگزاری") {
		t.Fatalf("POST did not render local-only mock result: %s", body)
	}
}
