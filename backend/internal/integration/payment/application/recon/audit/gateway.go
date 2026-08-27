// Package audit implements the Phase 1B one-shot reconciliation audit tool.
//
// CONSTITUTIONAL ROLE:
//   - READ-ONLY across DB and gateway.
//   - NO mutations, NO alerts, NO replay, NO auto-fix.
//   - NO scheduler / worker / goroutine fan-out.
//   - Calls the pure recon.Classify on a fully-resolved Snapshot per order.
//
// The audit package is intentionally separate from internal/worker so the
// CLI cannot accidentally bring up any worker side-effects.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labuda/backend/internal/integration/payment/application/recon"
)

// GatewayQuery is the read-only gateway-status surface the Resolver depends
// on. Any implementation that returns a recon.GatewaySnapshot keyed to a
// midtrans_order_id satisfies it; the audit package ships gatewayClient as
// the production implementation. Tests can substitute a stub.
type GatewayQuery interface {
	FetchStatus(ctx context.Context, midtransOrderID string) (recon.GatewaySnapshot, error)
}

// gatewayClient is the Phase 1B read-only Midtrans status fetcher. It does
// not share circuit-breaker state with pkg/midtrans.Client because we want
// the audit tool to be completely separable from production runtime — a
// broken audit must never trip a production circuit breaker, and a tripped
// production circuit must not block the audit.
//
// Auth is server-key Basic, same as pkg/midtrans.Client.GetTransactionStatus.
type gatewayClient struct {
	serverKey string
	baseURL   string
	http      *http.Client
}

// NewGatewayClient constructs the read-only Midtrans status fetcher.
//
// production=true selects the production core API base URL; false uses the
// sandbox base URL. perRequestTimeout caps each HTTP call independently of
// the context deadline.
func NewGatewayClient(serverKey string, production bool, perRequestTimeout time.Duration) GatewayQuery {
	base := "https://api.sandbox.midtrans.com/v2"
	if production {
		base = "https://api.midtrans.com/v2"
	}
	if perRequestTimeout <= 0 {
		perRequestTimeout = 15 * time.Second
	}
	return &gatewayClient{
		serverKey: serverKey,
		baseURL:   base,
		http:      &http.Client{Timeout: perRequestTimeout},
	}
}

// gatewayStatusResponse mirrors the Midtrans GET /v2/{order_id}/status
// response body with the extended fields needed by the classifier
// (specifically `refund_chargeback_history`, which pkg/midtrans's
// NotificationPayload does not model).
type gatewayStatusResponse struct {
	TransactionTime         string                    `json:"transaction_time"`
	SettlementTime          string                    `json:"settlement_time"`
	TransactionStatus       string                    `json:"transaction_status"`
	TransactionID           string                    `json:"transaction_id"`
	StatusCode              string                    `json:"status_code"`
	PaymentType             string                    `json:"payment_type"`
	OrderID                 string                    `json:"order_id"`
	GrossAmount             string                    `json:"gross_amount"`
	Currency                string                    `json:"currency"`
	RefundChargebackHistory []gatewayRefundHistoryRow `json:"refund_chargeback_history"`
}

type gatewayRefundHistoryRow struct {
	RefundKey         string `json:"refund_key"`
	RefundChargebackID string `json:"refund_chargeback_id"`
	RefundAmount      string `json:"refund_amount"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
}

// fetchStatus calls GET /v2/{order_id}/status and translates the response
// into a recon.GatewaySnapshot. It returns a snapshot with Available=false
// (and a non-nil error) on transport failure; the caller decides whether to
// emit a partial-coverage finding or skip.
func (g *gatewayClient) FetchStatus(ctx context.Context, midtransOrderID string) (recon.GatewaySnapshot, error) {
	queriedAt := time.Now().UTC()
	empty := recon.GatewaySnapshot{
		MidtransOrderID: midtransOrderID,
		Available:       false,
		QueriedAt:       queriedAt,
	}

	url := fmt.Sprintf("%s/%s/status", g.baseURL, midtransOrderID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return empty, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(g.serverKey, "")

	resp, err := g.http.Do(req)
	if err != nil {
		return empty, fmt.Errorf("midtrans status http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		// 404 = gateway has no record. Distinct from transport failure: the
		// gateway IS reachable, it just doesn't know this order. Returned as
		// Available=true with empty TransactionStatus so the classifier can
		// reason about "gateway says nothing here".
		return recon.GatewaySnapshot{
			MidtransOrderID: midtransOrderID,
			Available:       true,
			TransactionStatus: "",
			QueriedAt:         queriedAt,
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return empty, fmt.Errorf("midtrans non-200: status=%d body=%q", resp.StatusCode, truncate(string(body), 256))
	}

	var raw gatewayStatusResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return empty, fmt.Errorf("unmarshal status body: %w", err)
	}

	return translateGatewayResponse(raw, queriedAt), nil
}

// translateGatewayResponse converts the gateway's wire format to the pure
// classifier's GatewaySnapshot. Extracted for unit testing without an HTTP
// round-trip.
func translateGatewayResponse(raw gatewayStatusResponse, queriedAt time.Time) recon.GatewaySnapshot {
	gross := parseGatewayAmount(raw.GrossAmount)
	tt := parseGatewayTime(raw.TransactionTime)
	var st *time.Time
	if parsed := parseGatewayTime(raw.SettlementTime); !parsed.IsZero() {
		v := parsed
		st = &v
	}

	hist := make([]recon.GatewayRefundEntry, 0, len(raw.RefundChargebackHistory))
	for _, r := range raw.RefundChargebackHistory {
		hist = append(hist, recon.GatewayRefundEntry{
			RefundKey: r.RefundKey,
			RefundID:  r.RefundChargebackID,
			Amount:    parseGatewayAmount(r.RefundAmount),
			Status:    strings.ToLower(strings.TrimSpace(r.Status)),
			CreatedAt: parseGatewayTime(r.CreatedAt),
		})
	}

	return recon.GatewaySnapshot{
		MidtransOrderID:         raw.OrderID,
		Available:               true,
		TransactionStatus:       strings.ToLower(strings.TrimSpace(raw.TransactionStatus)),
		StatusCode:              raw.StatusCode,
		TransactionID:           raw.TransactionID,
		PaymentType:             raw.PaymentType,
		GrossAmount:             gross,
		TransactionTime:         tt,
		SettlementTime:          st,
		RefundChargebackHistory: hist,
		QueriedAt:               queriedAt,
	}
}

// parseGatewayAmount converts Midtrans's stringified rupiah amount (which
// can carry decimals like "100000.00") to integer cents. Returns 0 on parse
// failure; the classifier handles 0 amounts gracefully.
func parseGatewayAmount(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Drop fractional part if present.
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseGatewayTime parses Midtrans's "YYYY-MM-DD HH:MM:SS" format (Jakarta
// time, no zone). Returns the zero time on any failure.
func parseGatewayTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	const layout = "2006-01-02 15:04:05"
	// Midtrans timestamps are nominally WIB (UTC+7) but the API does not
	// emit a zone. For audit-time comparisons we treat the value as UTC; a
	// 7-hour skew is not material to drift classification thresholds
	// expressed in minutes-to-hours.
	if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
		return t
	}
	return time.Time{}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}


