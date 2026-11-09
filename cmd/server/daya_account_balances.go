package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const dayaFinancialBodyLimit = 1 << 20

type DayaAccountBalanceMetric struct {
	Status  string `json:"status"`
	Value   string `json:"value,omitempty"`
	Unit    string `json:"unit,omitempty"`
	Field   string `json:"field,omitempty"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message,omitempty"`
}

type DayaAccountBalanceEndpoint struct {
	Path       string   `json:"path"`
	StatusCode int      `json:"status_code"`
	Error      string   `json:"error,omitempty"`
	Fields     []string `json:"fields,omitempty"`
	Schema     []string `json:"schema,omitempty"`
}

type DayaAccountBalanceReport struct {
	RetrievedAt  time.Time                    `json:"retrieved_at"`
	NeedsLogin   bool                         `json:"needs_login"`
	BuyingPower  DayaAccountBalanceMetric     `json:"buying_power"`
	CashBalance  DayaAccountBalanceMetric     `json:"cash_balance"`
	Withdrawable DayaAccountBalanceMetric     `json:"withdrawable"`
	Endpoints    []DayaAccountBalanceEndpoint `json:"endpoints"`
}

type dayaBalanceEndpointData struct {
	path   string
	status int
	body   any
	err    string
}

type dayaBalanceCandidate struct {
	value string
	field string
	path  string
}

func dayaNormalizeFinancialKey(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func dayaFinancialAmount(value any) (string, bool) {
	var raw string
	switch v := value.(type) {
	case json.Number:
		raw = v.String()
	case string:
		raw = strings.TrimSpace(persianDigitReplacer.Replace(v))
	case float64:
		raw = strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		raw = strconv.FormatInt(v, 10)
	case int:
		raw = strconv.Itoa(v)
	default:
		return "", false
	}
	raw = strings.ReplaceAll(raw, ",", "")
	raw = strings.ReplaceAll(raw, "٬", "")
	if raw == "" {
		return "", false
	}
	if _, err := strconv.ParseFloat(raw, 64); err != nil {
		return "", false
	}
	return raw, true
}

func dayaMakeAliasSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[dayaNormalizeFinancialKey(value)] = struct{}{}
	}
	return result
}

func dayaCollectBalanceCandidates(value any, path string, aliases map[string]struct{}, out *[]dayaBalanceCandidate, depth int) {
	if depth > 16 {
		return
	}
	switch node := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(node))
		for key := range node {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := node[key]
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			if _, ok := aliases[dayaNormalizeFinancialKey(key)]; ok {
				if amount, valid := dayaFinancialAmount(child); valid {
					*out = append(*out, dayaBalanceCandidate{value: amount, field: key, path: childPath})
					continue
				}
			}
			dayaCollectBalanceCandidates(child, childPath, aliases, out, depth+1)
		}
	case []any:
		for i, child := range node {
			dayaCollectBalanceCandidates(child, fmt.Sprintf("%s[%d]", path, i), aliases, out, depth+1)
		}
	}
}

// dayaSafeFinancialFieldName accepts simple JSON property names only.
func dayaSafeFinancialFieldName(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for i, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9') || (i > 0 && r == '_')) {
			return false
		}
	}
	return true
}

// dayaCollectFinancialFieldNames returns field names only, never values or raw payloads.
// Sensitive identifiers and session-related names are deliberately excluded.
func dayaCollectFinancialFieldNames(value any, out map[string]struct{}, depth int) {
	if depth > 12 || len(out) >= 40 {
		return
	}
	switch node := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(node))
		for key := range node {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if len(out) >= 40 {
				return
			}
			child := node[key]
			normalized := dayaNormalizeFinancialKey(key)
			if strings.Contains(normalized, "account") || strings.Contains(normalized, "customer") ||
				strings.Contains(normalized, "userid") || strings.Contains(normalized, "token") ||
				strings.Contains(normalized, "cookie") || strings.Contains(normalized, "password") ||
				strings.Contains(normalized, "phone") || strings.Contains(normalized, "mobile") ||
				strings.Contains(normalized, "national") || strings.Contains(normalized, "identity") ||
				strings.Contains(normalized, "isin") || strings.Contains(normalized, "symbol") ||
				strings.Contains(normalized, "orderid") || normalized == "id" {
				continue
			}
			if _, ok := dayaFinancialAmount(child); ok {
				if dayaSafeFinancialFieldName(key) {
					out[key] = struct{}{}
				}
				continue
			}
			dayaCollectFinancialFieldNames(child, out, depth+1)
		}
	case []any:
		for _, child := range node {
			dayaCollectFinancialFieldNames(child, out, depth+1)
		}
	}
}

// dayaFinancialSchemaFieldAllowed omits identifiers and personal or session data.
func dayaFinancialSchemaFieldAllowed(key string) bool {
	if !dayaSafeFinancialFieldName(key) {
		return false
	}
	normalized := dayaNormalizeFinancialKey(key)
	for _, blocked := range []string{
		"account", "customer", "userid", "username", "token", "cookie",
		"password", "phone", "mobile", "email", "address", "national",
		"identity", "birth", "firstname", "lastname", "fullname",
		"isin", "symbol", "orderid", "brokerid", "clientid", "id",
	} {
		if strings.Contains(normalized, blocked) {
			return false
		}
	}
	return true
}

// dayaCollectFinancialSchema records safe property paths and JSON types only.
// It never includes values, and samples at most two array elements per path.
func dayaCollectFinancialSchema(value any, path string, out *[]string, depth int) {
	if depth > 10 || len(*out) >= 100 {
		return
	}
	appendEntry := func(entry string) {
		if len(*out) < 100 {
			*out = append(*out, entry)
		}
	}
	switch node := value.(type) {
	case map[string]any:
		if path != "" {
			appendEntry(path + ":object")
		}
		keys := make([]string, 0, len(node))
		for key := range node {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if len(*out) >= 100 {
				return
			}
			if !dayaFinancialSchemaFieldAllowed(key) {
				continue
			}
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			switch child := node[key].(type) {
			case map[string]any:
				dayaCollectFinancialSchema(child, childPath, out, depth+1)
			case []any:
				appendEntry(childPath + ":array")
				limit := len(child)
				if limit > 2 {
					limit = 2
				}
				for i := 0; i < limit; i++ {
					dayaCollectFinancialSchema(child[i], childPath+"[]", out, depth+1)
				}
			case json.Number:
				appendEntry(childPath + ":number")
			case string:
				appendEntry(childPath + ":string")
			case bool:
				appendEntry(childPath + ":boolean")
			case nil:
				appendEntry(childPath + ":null")
			default:
				appendEntry(childPath + ":other")
			}
		}
	case []any:
		limit := len(node)
		if limit > 2 {
			limit = 2
		}
		for i := 0; i < limit; i++ {
			dayaCollectFinancialSchema(node[i], path+"[]", out, depth+1)
		}
	}
}

func dayaCollectUnit(value any, depth int) string {
	if depth > 12 {
		return ""
	}
	switch node := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(node))
		for key := range node {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			normalized := dayaNormalizeFinancialKey(key)
			if normalized == "currency" || normalized == "currencycode" || normalized == "amountunit" || normalized == "moneyunit" || normalized == "unit" {
				if text, ok := node[key].(string); ok {
					value := strings.ToLower(strings.TrimSpace(text))
					switch {
					case strings.Contains(value, "irr"), strings.Contains(value, "rial"), strings.Contains(value, "ریال"):
						return "ریال"
					case strings.Contains(value, "irt"), strings.Contains(value, "toman"), strings.Contains(value, "تومان"):
						return "تومان"
					}
				}
			}
			if found := dayaCollectUnit(node[key], depth+1); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range node {
			if found := dayaCollectUnit(child, depth+1); found != "" {
				return found
			}
		}
	}
	return ""
}

func dayaSelectBalanceMetric(preferred, fallback *dayaBalanceEndpointData, aliases map[string]struct{}, label string) DayaAccountBalanceMetric {
	collect := func(endpoint *dayaBalanceEndpointData) []dayaBalanceCandidate {
		if endpoint == nil || endpoint.err != "" || endpoint.status != http.StatusOK || endpoint.body == nil {
			return nil
		}
		var candidates []dayaBalanceCandidate
		dayaCollectBalanceCandidates(endpoint.body, "", aliases, &candidates, 0)
		for i := range candidates {
			candidates[i].path = endpoint.path + ":" + candidates[i].path
		}
		return candidates
	}

	candidates := collect(preferred)
	source := preferred
	if len(candidates) == 0 {
		candidates = collect(fallback)
		source = fallback
	}
	if len(candidates) == 0 {
		message := "فیلد معتبر این مبلغ در پاسخ فعلی کارگزاری شناسایی نشد."
		if preferred != nil && preferred.err != "" && (fallback == nil || fallback.err != "") {
			message = "دریافت اطلاعات مالی از کارگزاری ناموفق بود."
		}
		return DayaAccountBalanceMetric{Status: "not_found", Message: message}
	}

	depth := func(path string) int { return strings.Count(path, ".") + strings.Count(path, "[") }
	sort.Slice(candidates, func(i, j int) bool {
		if depth(candidates[i].path) != depth(candidates[j].path) {
			return depth(candidates[i].path) < depth(candidates[j].path)
		}
		return candidates[i].path < candidates[j].path
	})
	minDepth := depth(candidates[0].path)
	selected := candidates[:0]
	for _, candidate := range candidates {
		if depth(candidate.path) > minDepth {
			break
		}
		selected = append(selected, candidate)
	}
	value := selected[0].value
	for _, candidate := range selected[1:] {
		if candidate.value != value {
			return DayaAccountBalanceMetric{
				Status:  "ambiguous",
				Message: "چند فیلد مالی با مقادیر متفاوت پیدا شد؛ برای جلوگیری از نمایش عدد اشتباه، مبلغ پنهان شده است.",
			}
		}
	}
	unit := dayaCollectUnit(source.body, 0)
	return DayaAccountBalanceMetric{
		Status: "ok", Value: value, Unit: unit,
		Field: selected[0].field, Source: source.path, Message: label,
	}
}

func dayaBalanceEndpointReport(endpoint dayaBalanceEndpointData) DayaAccountBalanceEndpoint {
	report := DayaAccountBalanceEndpoint{Path: endpoint.path, StatusCode: endpoint.status, Error: endpoint.err}
	if endpoint.body != nil {
		fields := make(map[string]struct{})
		dayaCollectFinancialFieldNames(endpoint.body, fields, 0)
		for field := range fields {
			report.Fields = append(report.Fields, field)
		}
		sort.Strings(report.Fields)
		dayaCollectFinancialSchema(endpoint.body, "", &report.Schema, 0)
		sort.Strings(report.Schema)
	}
	return report
}

func fetchDayaFinancialEndpoint(ctx context.Context, client *http.Client, endpoint string, session *DayaSession, credentials *DayaCredentialStore) dayaBalanceEndpointData {
	result := dayaBalanceEndpointData{path: endpoint}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dayaAPIOrigin+endpoint, nil)
	if err != nil {
		result.err = "ساخت درخواست خواندنی کارگزاری ناموفق بود."
		return result
	}
	req.Header.Set("Accept", "application/json")
	for _, cookie := range session.Cookies {
		if (cookie.Name == "token" || cookie.Name == "cookiesession1") && cookie.Value != "" {
			req.AddCookie(cookie.httpCookie())
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		result.err = "ارتباط با مسیر مالی کارگزاری برقرار نشد."
		return result
	}
	result.status = resp.StatusCode
	defer resp.Body.Close()

	if received := resp.Cookies(); len(received) > 0 {
		candidate := *session
		candidate.Cookies = append([]DayaSessionCookie(nil), session.Cookies...)
		rotated, applyErr := dayaApplySessionCookies(&candidate, received, time.Now().UTC())
		if applyErr == nil && rotated && credentials != nil {
			if saveErr := credentials.SaveAccessSession(candidate); saveErr == nil {
				*session = candidate
			} else {
				result.err = "ذخیره امن تغییر نشست کارگزاری ناموفق بود."
				return result
			}
		}
	}
	switch result.status {
	case http.StatusUnauthorized, http.StatusForbidden:
		result.err = "نشست کارگزاری پذیرفته نشد؛ ورود مجدد لازم است."
		return result
	case http.StatusTooManyRequests:
		result.err = "کارگزاری محدودیت درخواست اعمال کرد؛ دریافت متوقف شد."
		return result
	}
	if result.status != http.StatusOK {
		result.err = fmt.Sprintf("پاسخ کارگزاری HTTP %d بود.", result.status)
		return result
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, dayaFinancialBodyLimit+1))
	if err != nil || len(body) == 0 || len(body) > dayaFinancialBodyLimit {
		result.err = "پاسخ مالی کارگزاری خالی، بیش از حد مجاز یا ناخوانا بود."
		return result
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		result.err = "ساختار پاسخ مالی کارگزاری JSON معتبر نیست."
		return result
	}
	result.body = payload
	return result
}

// ReadDayaAccountBalances uses only observed read-only GET endpoints. It never
// returns raw broker payloads, account identifiers, holdings, or session cookies.
func ReadDayaAccountBalances(ctx context.Context, credentials *DayaCredentialStore) DayaAccountBalanceReport {
	missing := DayaAccountBalanceMetric{Status: "not_found", Message: "مبلغ هنوز دریافت نشده است."}
	report := DayaAccountBalanceReport{
		RetrievedAt: time.Now().UTC(), BuyingPower: missing,
		CashBalance: missing, Withdrawable: missing,
		Endpoints: make([]DayaAccountBalanceEndpoint, 0, 2),
	}
	if credentials == nil {
		report.Endpoints = append(report.Endpoints, DayaAccountBalanceEndpoint{Path: "/financials/buying-power", Error: "مخزن نشست کارگزاری در دسترس نیست."})
		return report
	}
	session, _, err := credentials.LoadAccessSession()
	if err != nil || session.Token == "" {
		report.NeedsLogin = true
		report.Endpoints = append(report.Endpoints,
			DayaAccountBalanceEndpoint{Path: "/financials/buying-power", Error: "نشست معتبر کارگزاری موجود نیست."},
			DayaAccountBalanceEndpoint{Path: "/financials/portfolio/instant", Error: "نشست معتبر کارگزاری موجود نیست."},
		)
		return report
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(time.Now()) {
		report.NeedsLogin = true
		report.Endpoints = append(report.Endpoints,
			DayaAccountBalanceEndpoint{Path: "/financials/buying-power", Error: "نشست کارگزاری منقضی شده است."},
			DayaAccountBalanceEndpoint{Path: "/financials/portfolio/instant", Error: "نشست کارگزاری منقضی شده است."},
		)
		return report
	}

	client := &http.Client{
		Timeout:       8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	client.Jar = nil
	buying := fetchDayaFinancialEndpoint(ctx, client, "/financials/buying-power", &session, credentials)
	report.Endpoints = append(report.Endpoints, dayaBalanceEndpointReport(buying))
	if buying.status == http.StatusUnauthorized || buying.status == http.StatusForbidden || buying.status == http.StatusTooManyRequests {
		report.NeedsLogin = buying.status == http.StatusUnauthorized || buying.status == http.StatusForbidden
		report.Endpoints = append(report.Endpoints, DayaAccountBalanceEndpoint{Path: "/financials/portfolio/instant", Error: "به‌دلیل پاسخ مسیر اول، درخواست دوم اجرا نشد."})
		return report
	}

	portfolio := fetchDayaFinancialEndpoint(ctx, client, "/financials/portfolio/instant", &session, credentials)
	report.Endpoints = append(report.Endpoints, dayaBalanceEndpointReport(portfolio))
	if portfolio.status == http.StatusUnauthorized || portfolio.status == http.StatusForbidden {
		report.NeedsLogin = true
	}

	report.BuyingPower = dayaSelectBalanceMetric(&buying, &portfolio,
		dayaMakeAliasSet("buyingPower", "availableBuyingPower", "cashBuyingPower", "buyingPowerAmount", "buyingPowerAvailable", "availableToPurchase", "buyingPowerValue", "credit"),
		"قدرت خرید")
	report.CashBalance = dayaSelectBalanceMetric(&portfolio, &buying,
		dayaMakeAliasSet("cashBalance", "totalCashBalance", "accountCashBalance", "cashAmount", "cashBalanceAmount", "totalCash", "cash"),
		"موجودی نقدی")
	report.Withdrawable = dayaSelectBalanceMetric(&portfolio, &buying,
		dayaMakeAliasSet("withdrawableCash", "cashWithdrawable", "withdrawableBalance", "cashAvailableToWithdraw", "availableToWithdraw", "withdrawableAmount", "availableWithdrawalAmount", "withdrawalAvailableAmount"),
		"موجودی قابل برداشت")
	return report
}

func registerDayaAccountBalanceRoutes(mux *http.ServeMux, previewMode bool, credentials *DayaCredentialStore) {
	mux.HandleFunc("GET /api/daya/account-balances", func(w http.ResponseWriter, r *http.Request) {
		if !previewMode {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
		defer cancel()
		report := ReadDayaAccountBalances(ctx, credentials)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		_ = json.NewEncoder(w).Encode(report)
	})
}
