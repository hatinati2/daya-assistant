package main

import (
	"errors"
	"strings"
	"time"
)

func (s *Store) claimReadyAPIOrders(now time.Time) ([]Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	original := append([]Order(nil), s.orders...)
	claimed := make([]Order, 0)
	changed := false

	for i := range s.orders {
		order := &s.orders[i]
		mode := order.ExecutionMode
		if mode == "" {
			mode = executionModeManual
		}
		if mode != executionModeAPI || order.Status != statusReady {
			continue
		}
		if order.ISIN == "" {
			order.Status = statusSubmissionFailed
			order.BrokerMessage = "شناسه ISIN برای سفارش وارد نشده است؛ سفارش به کارگزاری ارسال نشد."
			order.UpdatedAt = now
			changed = true
			continue
		}
		order.Status = statusSubmitting
		order.AttemptedAt = now
		order.BrokerMessage = "درخواست در حال ارسال به دایا است."
		order.UpdatedAt = now
		claimed = append(claimed, *order)
		changed = true
	}

	if changed {
		if err := s.saveLocked(); err != nil {
			s.orders = original
			return nil, err
		}
	}
	return claimed, nil
}

type DayaOrderOutcome struct {
	Status         string
	HTTPStatus     int
	RequestID      string
	BrokerOrderID  string
	Message        string
	ReceivedAt     time.Time
}

func (s *Store) completeAPIOrder(id string, outcome DayaOrderOutcome) error {
	if outcome.Status != statusSubmitted && outcome.Status != statusSubmissionFailed && outcome.Status != statusSubmissionUnknown {
		return errors.New("وضعیت نهایی ارسال سفارش نامعتبر است")
	}
	if outcome.Status == statusSubmitted && strings.TrimSpace(outcome.BrokerOrderID) == "" {
		return errors.New("برای تأیید ثبت سفارش، شناسه سفارش دایا الزامی است")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.orders {
		order := &s.orders[i]
		if order.ID != id {
			continue
		}
		if order.Status != statusSubmitting || order.ExecutionMode != executionModeAPI {
			return errors.New("سفارش در وضعیت ارسال API نیست")
		}
		old := *order
		order.Status = outcome.Status
		order.BrokerOrderID = strings.TrimSpace(outcome.BrokerOrderID)
		order.BrokerRequestID = strings.TrimSpace(outcome.RequestID)
		order.BrokerHTTPStatus = outcome.HTTPStatus
		order.BrokerMessage = outcome.Message
		order.BrokerResponseAt = outcome.ReceivedAt
		order.UpdatedAt = outcome.ReceivedAt
		if order.UpdatedAt.IsZero() {
			order.UpdatedAt = time.Now().In(tehran())
		}
		if err := s.saveLocked(); err != nil {
			*order = old
			return err
		}
		return nil
	}
	return errors.New("سفارش پیدا نشد")
}

func (s *Store) recoverInterruptedAPIOrders(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i := range s.orders {
		if s.orders[i].Status == statusSubmitting {
			s.orders[i].Status = statusSubmissionUnknown
			s.orders[i].BrokerMessage = "سرویس هنگام ارسال متوقف شده است؛ به دلیل احتمال ثبت در دایا، ارسال خودکار مجدد انجام نمی‌شود. وضعیت را در کارگزاری بررسی کنید."
			s.orders[i].UpdatedAt = now
			changed = true
		}
	}
	if changed {
		return s.saveLocked()
	}
	return nil
}
