package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Daya's observed order contract. This file deliberately contains no HTTP
// transport or dispatch function: building/decoding a contract must never
// submit an order. Live submission remains disabled until separately reviewed.
const (
	dayaOrderEndpoint       = dayaAPIOrigin + "/orders"
	dayaOrderSideBuy        = "BUY"
	dayaOrderSideSell       = "SELL"
	dayaOrderValidityToday = "JUST_TODAY"
)

var dayaInstrumentIDPattern = regexp.MustCompile(`^[A-Z0-9]{12}$`)

type DayaOrderPayload struct {
	InstrumentID      string  `json:"instrumentId"`
	OrderPrice        int64   `json:"orderPrice"`
	OrderQuantity     int64   `json:"orderQuantity"`
	OrderSide         string  `json:"orderSide"`
	OrderValidityDate *string `json:"orderValidityDate"`
	OrderValidityType string  `json:"orderValidityType"`
}

// BuildDayaOrderPayload maps a locally scheduled order to the observed Daya
// JSON shape. It performs local validation only and never makes a network call.
func BuildDayaOrderPayload(order Order) (DayaOrderPayload, error) {
	if order.ExecutionMode != executionModeAPI {
		return DayaOrderPayload{}, errors.New("ساخت Payload دایا فقط برای سفارش‌های مسیر API مجاز است")
	}
	instrumentID := strings.ToUpper(strings.TrimSpace(order.ISIN))
	if !dayaInstrumentIDPattern.MatchString(instrumentID) {
		return DayaOrderPayload{}, errors.New("شناسه ابزار دایا باید دقیقاً ۱۲ نویسه حرفی یا عددی باشد")
	}
	if order.Price <= 0 || order.Quantity <= 0 {
		return DayaOrderPayload{}, errors.New("قیمت و تعداد سفارش باید مثبت باشند")
	}

	side := ""
	switch strings.ToLower(strings.TrimSpace(order.Side)) {
	case "buy":
		side = dayaOrderSideBuy
	case "sell":
		side = dayaOrderSideSell
	default:
		return DayaOrderPayload{}, errors.New("سمت سفارش دایا نامعتبر است")
	}

	return DayaOrderPayload{
		InstrumentID:      instrumentID,
		OrderPrice:        order.Price,
		OrderQuantity:     order.Quantity,
		OrderSide:         side,
		OrderValidityDate: nil,
		OrderValidityType: dayaOrderValidityToday,
	}, nil
}

// MarshalDayaOrderPayload serializes the observed contract locally only.
func MarshalDayaOrderPayload(order Order) ([]byte, error) {
	payload, err := BuildDayaOrderPayload(order)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return nil, fmt.Errorf("ساخت JSON سفارش دایا: %w", err)
	}
	return bytes.TrimSpace(out.Bytes()), nil
}

type DayaOrderError struct {
	Timestamp int64  `json:"timestamp"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	RequestID string `json:"requestId"`
	Message   string `json:"message"`
}

// DecodeDayaOrderError parses a broker error response without logging or
// retaining credentials, request headers, or cookie values.
func DecodeDayaOrderError(data []byte) (DayaOrderError, error) {
	var result DayaOrderError
	if len(data) == 0 || len(data) > 64<<10 {
		return result, errors.New("اندازه پاسخ خطای دایا نامعتبر است")
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("خواندن پاسخ خطای دایا: %w", err)
	}
	if result.Status < 400 || result.Status > 599 {
		return result, errors.New("پاسخ، خطای HTTP معتبر دایا نیست")
	}
	if result.Message == "" {
		result.Message = result.Error
	}
	return result, nil
}
