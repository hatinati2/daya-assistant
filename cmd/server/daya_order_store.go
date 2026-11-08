package main

import (
	"errors"
	"fmt"
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
	Message               string
	ReceivedAt            time.Time
}

func (s *Store) completeAPIOrder(id string, outcome DayaOrderOutcome) error {
	if outcome.Status != statusSubmitted && outcome.Status != statusSubmissionPending && outcome.Status != statusSubmissionFailed && outcome.Status != statusSubmissionUnknown {
		return errors.New("وضعیت نهایی ارسال سفارش نامعتبر است")
	}
	if (outcome.Status == statusSubmitted || outcome.Status == statusSubmissionPending) && strings.TrimSpace(outcome.BrokerOrderID) == "" {
		return errors.New("برای تأیید ثبت سفارش، شناسه سفارش دایا الزامی است")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.orders {
		order := &s.orders[i]
		if order.ID != id {
			continue
		}
		validSourceState := order.Status == statusSubmitting ||
			(order.Status == statusSubmissionPending && outcome.Status == statusSubmitted)
		if !validSourceState || order.ExecutionMode != executionModeAPI {
			return errors.New("سفارش در وضعیت معتبر برای ثبت نتیجه API نیست")
		}
		old := *order
		order.Status = outcome.Status
		order.BrokerOrderID = strings.TrimSpace(outcome.BrokerOrderID)
		order.BrokerHostOrderNumber = outcome.BrokerHostOrderNumber
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

func (s *Store) pendingAPIOrders() []Order {
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

// updateAPIOrderExecution persists broker fill counters for the exact registered
// order. The broker order ID is the primary identity; order attributes are
// checked by the caller before this method is reached.
func (s *Store) updateAPIOrderExecution(id string, record dayaOrderConfirmationRecord, now time.Time) error {
	if record.OrderID <= 0 || record.TradedQuantity == nil || *record.TradedQuantity < 0 {
		return errors.New("اطلاعات مقدار معامله‌شده از دایا معتبر نیست")
	}
	if record.MatchedQuantity != nil && *record.MatchedQuantity < 0 {
		return errors.New("مقدار تطبیق‌شده از دایا معتبر نیست")
	}
	if record.TradedPrice != nil && *record.TradedPrice < 0 {
		return errors.New("قیمت معامله‌شده از دایا معتبر نیست")
	}
	if record.RemainingQuantityOfTheOrder != nil && *record.RemainingQuantityOfTheOrder < 0 {
		return errors.New("مقدار باقی‌مانده از دایا معتبر نیست")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.orders {
		order := &s.orders[i]
		if order.ID != id {
			continue
		}
		if order.ExecutionMode != executionModeAPI ||
			(order.Status != statusSubmitted && order.Status != statusPartiallyFilled) ||
			parseBrokerOrderID(order.BrokerOrderID) != record.OrderID {
			return errors.New("رکورد معامله متعلق به همین سفارش ثبت‌شده نیست")
		}
		traded := *record.TradedQuantity
		if traded > order.Quantity {
			return errors.New("مقدار معامله‌شده از تعداد کل سفارش بیشتر است")
		}
		if order.BrokerTradedQuantity != nil && traded < *order.BrokerTradedQuantity {
			return errors.New("مقدار معامله‌شده نسبت به پایش قبلی کاهش یافته است")
		}
		remaining := order.Quantity - traded
		if record.RemainingQuantityOfTheOrder != nil {
			remaining = *record.RemainingQuantityOfTheOrder
		}
		if traded == order.Quantity && remaining == 0 {
			order.Status = statusFilled
			order.BrokerMessage = "کل تعداد سفارش معامله شد."
		} else if traded > 0 {
			order.Status = statusPartiallyFilled
			order.BrokerMessage = fmt.Sprintf("معامله جزئی انجام شده: %d از %d سهم؛ باقی‌مانده %d سهم.", traded, order.Quantity, remaining)
		} else {
			order.Status = statusSubmitted
			order.BrokerMessage = "سفارش در دفتر سفارش‌های دایا ثبت شده و منتظر تطبیق با سفارش مقابل است."
		}
		if record.MatchedQuantity != nil {
			v := *record.MatchedQuantity
			order.BrokerMatchedQuantity = &v
		}
		v := traded
		order.BrokerTradedQuantity = &v
		if record.TradedPrice != nil {
			price := *record.TradedPrice
			order.BrokerTradedPrice = &price
		}
		remainingCopy := remaining
		order.BrokerRemainingQuantity = &remainingCopy
		order.BrokerProgressAt = now
		order.UpdatedAt = now
		if err := s.saveLocked(); err != nil {
			return err
		}
		return nil
	}
	return errors.New("سفارش محلی برای ثبت پیشرفت معامله پیدا نشد")
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
