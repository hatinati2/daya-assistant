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
		// Defense in depth: never claim an API order before its scheduled time,
		// even if another code path accidentally marks it ready early.
		if order.ExecuteAt.After(now) {
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
	Status                string
	HTTPStatus            int
	RequestID             string
	BrokerOrderID         string
	BrokerHostOrderNumber int64
	MatchedQuantity       *int64
	TradedQuantity        *int64
	TradedPrice           *int64
	RemainingQuantity     *int64
	Message               string
	ReceivedAt            time.Time
	ExecutionAt           time.Time
}

func (o DayaOrderOutcome) hasExecutionProgress() bool {
	return o.MatchedQuantity != nil || o.TradedQuantity != nil ||
		o.TradedPrice != nil || o.RemainingQuantity != nil
}

func (s *Store) completeAPIOrder(id string, outcome DayaOrderOutcome) error {
	switch outcome.Status {
	case statusSubmitted, statusSubmissionPending, statusPartiallyFilled, statusFilled,
		statusSubmissionFailed, statusSubmissionUnknown:
	default:
		return errors.New("وضعیت نهایی ارسال سفارش نامعتبر است")
	}
	if (outcome.Status == statusSubmitted || outcome.Status == statusSubmissionPending ||
		outcome.Status == statusPartiallyFilled || outcome.Status == statusFilled) &&
		strings.TrimSpace(outcome.BrokerOrderID) == "" {
		return errors.New("برای تأیید ثبت سفارش، شناسه سفارش دایا الزامی است")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.orders {
		order := &s.orders[i]
		if order.ID != id {
			continue
		}
		if order.ExecutionMode != executionModeAPI {
			return errors.New("سفارش در وضعیت معتبر برای ثبت نتیجه API نیست")
		}
		if order.Status == statusFilled {
			return nil
		}
		for _, value := range []*int64{
			outcome.MatchedQuantity, outcome.TradedQuantity, outcome.RemainingQuantity, outcome.TradedPrice,
		} {
			if value != nil && *value < 0 {
				return errors.New("مقادیر اجرای سفارش نمی‌توانند منفی باشند")
			}
		}
		for _, value := range []*int64{
			outcome.MatchedQuantity, outcome.TradedQuantity, outcome.RemainingQuantity,
		} {
			if value != nil && *value > order.Quantity {
				return errors.New("مقدار اجرای سفارش از تعداد کل سفارش بیشتر است")
			}
		}
		if outcome.Status == statusPartiallyFilled &&
			(outcome.TradedQuantity == nil || *outcome.TradedQuantity <= 0 || *outcome.TradedQuantity >= order.Quantity) {
			return errors.New("برای وضعیت معامله جزئی، مقدار معامله‌شده باید بین صفر و تعداد کل سفارش باشد")
		}
		if outcome.Status == statusFilled &&
			(outcome.TradedQuantity == nil || *outcome.TradedQuantity != order.Quantity) {
			return errors.New("تکمیل سفارش فقط با معامله‌شدن کل تعداد سفارش تأیید می‌شود")
		}

		initialResult := order.Status == statusSubmitting
		bookedResult := order.Status == statusSubmissionPending &&
			(outcome.Status == statusSubmitted || outcome.Status == statusPartiallyFilled || outcome.Status == statusFilled)
		executionUpdate := (order.Status == statusSubmitted || order.Status == statusPartiallyFilled) &&
			outcome.hasExecutionProgress() &&
			(outcome.Status == statusSubmitted || outcome.Status == statusPartiallyFilled || outcome.Status == statusFilled)
		if !initialResult && !bookedResult && !executionUpdate {
			return errors.New("سفارش در وضعیت معتبر برای ثبت نتیجه API نیست")
		}

		// Broker counters are cumulative. Ignore an older snapshot rather than
		// rolling back a partial fill already persisted locally.
		if order.BrokerExecutionKnown && outcome.TradedQuantity != nil &&
			* outcome.TradedQuantity < order.BrokerTradedQuantity {
			return nil
		}

		old := *order
		if order.Status == statusPartiallyFilled && outcome.Status == statusSubmitted {
			// A delayed/stale BOOKED snapshot must not hide an earlier partial fill.
			outcome.Status = statusPartiallyFilled
		}
		order.Status = outcome.Status
		if brokerID := strings.TrimSpace(outcome.BrokerOrderID); brokerID != "" {
			order.BrokerOrderID = brokerID
		}
		if outcome.BrokerHostOrderNumber > 0 {
			order.BrokerHostOrderNumber = outcome.BrokerHostOrderNumber
		}
		order.BrokerRequestID = strings.TrimSpace(outcome.RequestID)
		order.BrokerHTTPStatus = outcome.HTTPStatus
		order.BrokerMessage = outcome.Message
		order.BrokerResponseAt = outcome.ReceivedAt
		if outcome.MatchedQuantity != nil {
			order.BrokerMatchedQuantity = *outcome.MatchedQuantity
			order.BrokerExecutionKnown = true
		}
		if outcome.TradedQuantity != nil {
			order.BrokerTradedQuantity = *outcome.TradedQuantity
			order.BrokerExecutionKnown = true
		}
		if outcome.TradedPrice != nil {
			order.BrokerTradedPrice = *outcome.TradedPrice
			order.BrokerExecutionKnown = true
		}
		if outcome.RemainingQuantity != nil {
			order.BrokerRemainingQuantity = *outcome.RemainingQuantity
			order.BrokerExecutionKnown = true
		}
		if order.BrokerExecutionKnown {
			order.BrokerExecutionAt = outcome.ExecutionAt
			if order.BrokerExecutionAt.IsZero() {
				order.BrokerExecutionAt = outcome.ReceivedAt
			}
		}
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

func (s *Store) pendingAPIOrders() []Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Order, 0)
	for _, order := range s.orders {
		if order.ExecutionMode == executionModeAPI && order.Status == statusSubmissionPending && order.BrokerOrderID != "" {
			out = append(out, order)
		}
	}
	return out
}

// monitoredAPIOrders includes ENTRY acknowledgements and broker-BOOKED orders
// that are still waiting for a counterparty or have only partially traded.
func (s *Store) monitoredAPIOrders() []Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Order, 0)
	for _, order := range s.orders {
		if order.ExecutionMode != executionModeAPI || strings.TrimSpace(order.BrokerOrderID) == "" {
			continue
		}
		switch order.Status {
		case statusSubmissionPending, statusSubmitted, statusPartiallyFilled:
			out = append(out, order)
		}
	}
	return out
}

func (s *Store) reconcileAPIOrder(id, resolution, brokerOrderID, message string, now time.Time) error {
	id = strings.TrimSpace(id)
	brokerOrderID = strings.TrimSpace(brokerOrderID)
	message = strings.TrimSpace(message)
	if id == "" {
		return errors.New("شناسه سفارش محلی الزامی است")
	}
	if len(brokerOrderID) > 128 || len(message) > 1000 {
		return errors.New("شناسه یا توضیح نتیجه بررسی بیش از حد مجاز است")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.orders {
		order := &s.orders[i]
		if order.ID != id {
			continue
		}
		if order.ExecutionMode != executionModeAPI ||
			(order.Status != statusSubmissionUnknown && order.Status != statusSubmissionPending) {
			return errors.New("فقط سفارش API در انتظار تأیید یا با نتیجه نامشخص قابل تطبیق دستی است")
		}
		old := *order
		switch resolution {
		case "registered":
			if brokerOrderID == "" {
				return errors.New("برای تأیید ثبت سفارش، شناسه سفارش دایا را وارد کنید")
			}
			order.Status = statusSubmitted
			order.BrokerOrderID = brokerOrderID
			if message == "" {
				message = "ثبت سفارش پس از بررسی دستی در پنل دایا تأیید شد."
			}
		case "rejected":
			if brokerOrderID != "" {
				return errors.New("برای نتیجه ردشده نباید شناسه سفارش ثبت‌شده وارد شود")
			}
			order.Status = statusSubmissionFailed
			if message == "" {
				message = "پس از بررسی دستی در پنل دایا، ثبت سفارش تأیید نشد."
			}
		default:
			return errors.New("نتیجه بررسی دستی معتبر نیست")
		}
		order.BrokerMessage = message
		order.ReconciledAt = now
		order.UpdatedAt = now
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
