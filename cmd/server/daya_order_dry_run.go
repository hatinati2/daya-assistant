package main

import (
	"errors"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// orderDryRunResult is deliberately a local validation summary. It contains no
// broker endpoint, authentication material, dispatch flag, or persistence ID.
type orderDryRunResult struct {
	Mode      string `json:"mode"`
	Symbol    string `json:"symbol"`
	Side      string `json:"side"`
	Price     int64  `json:"price"`
	Quantity  int64  `json:"quantity"`
	ExecuteAt string `json:"execute_at"`
	Notional  string `json:"notional"`
}

// validateOrderDryRun reuses the normal order validation rules while forcibly
// selecting the manual path. It never stores an order and never performs I/O.
func validateOrderDryRun(r *http.Request, now time.Time) (orderDryRunResult, error) {
	if err := r.ParseForm(); err != nil {
		return orderDryRunResult{}, errors.New("فرم سفارش معتبر نیست")
	}
	// Ignore any client-selected API mode and ISIN: dry-run is never dispatchable.
	r.Form.Set("execution_mode", executionModeManual)
	r.Form.Del("isin")

	order, err := validateOrder(r, now)
	if err != nil {
		return orderDryRunResult{}, err
	}
	notional := new(big.Int).Mul(big.NewInt(order.Price), big.NewInt(order.Quantity))
	return orderDryRunResult{
		Mode:      "dry_run",
		Symbol:    strings.TrimSpace(order.Symbol),
		Side:      order.Side,
		Price:     order.Price,
		Quantity:  order.Quantity,
		ExecuteAt: formatJalaliDateTime(order.ExecuteAt),
		Notional:  notional.String(),
	}, nil
}
