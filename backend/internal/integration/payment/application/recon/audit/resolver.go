package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/labuda/backend/internal/integration/payment/application/recon"
)

// Resolver builds a recon.Snapshot for a single order_id by issuing a fixed
// set of read-only DB queries plus one gateway status query. It is the
// translation layer between operational truth (DB rows + Midtrans API) and
// the pure classifier's value-only Snapshot.
//
// Per Phase 1B constitution this struct holds NO mutation surface — only
// pgxpool.Pool (read) and gatewayClient (HTTP GET).
type Resolver struct {
	pool          *pgxpool.Pool
	gateway       GatewayQuery
	thresholds    recon.Thresholds
	skipGateway   bool
	gatewayBudget int // max successful gateway calls per audit run; <= 0 means unlimited
	gatewayUsed   int
}

// ResolverConfig configures the resolver.
type ResolverConfig struct {
	Pool          *pgxpool.Pool
	Gateway       GatewayQuery
	Thresholds    recon.Thresholds
	SkipGateway   bool // local-only mode; classifier sees Available=false
	GatewayBudget int  // 0 = unlimited; >0 = cap on actual HTTP calls
}

// NewResolver constructs a Resolver.
func NewResolver(cfg ResolverConfig) *Resolver {
	return &Resolver{
		pool:          cfg.Pool,
		gateway:       cfg.Gateway,
		thresholds:    cfg.Thresholds,
		skipGateway:   cfg.SkipGateway,
		gatewayBudget: cfg.GatewayBudget,
	}
}

// ResolveError carries the order context for a failed resolve so the
// orchestrator can record it in run statistics without aborting the run.
type ResolveError struct {
	OrderID uuid.UUID
	Stage   string
	Err     error
}

func (e *ResolveError) Error() string {
	return fmt.Sprintf("resolve order=%s stage=%s: %v", e.OrderID, e.Stage, e.Err)
}

func (e *ResolveError) Unwrap() error { return e.Err }

// ResolveOrder fetches every input the classifier needs for one order and
// returns a fully-populated Snapshot keyed to `now`.
//
// On gateway transport failure the returned Snapshot has Gateway.Available=
// false; the classifier suppresses gateway-required drift classes and
// surfaces local-only signals.
func (r *Resolver) ResolveOrder(ctx context.Context, orderID uuid.UUID, now time.Time) (recon.Snapshot, error) {
	snap := recon.Snapshot{
		Now:        now,
		Thresholds: r.thresholds,
		Ledger: recon.LedgerLookup{
			RefundReversalExistsByRefundID: map[uuid.UUID]bool{},
		},
		Outbox: recon.OutboxLookup{
			MoneyReleasedAliveByOrderID:         map[uuid.UUID]bool{},
			MoneyRefundSucceededAliveByRefundID: map[uuid.UUID]bool{},
			CoinsRefundRequiredAliveByOrderID:   map[uuid.UUID]bool{},
		},
	}

	order, err := r.fetchOrder(ctx, orderID)
	if err != nil {
		return snap, &ResolveError{OrderID: orderID, Stage: "order", Err: err}
	}
	snap.Order = order

	payment, err := r.fetchPaymentByOrderID(ctx, orderID)
	if err != nil {
		return snap, &ResolveError{OrderID: orderID, Stage: "payment", Err: err}
	}
	snap.Payment = payment

	escrow, err := r.fetchEscrow(ctx, orderID)
	if err != nil {
		return snap, &ResolveError{OrderID: orderID, Stage: "escrow", Err: err}
	}
	snap.Escrow = escrow

	refunds, err := r.fetchRefunds(ctx, orderID)
	if err != nil {
		return snap, &ResolveError{OrderID: orderID, Stage: "refunds", Err: err}
	}
	snap.Refunds = refunds

	midtransOID := ""
	if payment != nil {
		midtransOID = payment.MidtransOrderID
	}
	if midtransOID != "" {
		webhooks, err := r.fetchWebhooks(ctx, midtransOID)
		if err != nil {
			return snap, &ResolveError{OrderID: orderID, Stage: "webhooks", Err: err}
		}
		snap.Webhooks = webhooks
	}

	if err := r.fetchLedgerLookups(ctx, &snap, orderID, refunds); err != nil {
		return snap, &ResolveError{OrderID: orderID, Stage: "ledger", Err: err}
	}
	if err := r.fetchOutboxLookups(ctx, &snap, orderID, refunds); err != nil {
		return snap, &ResolveError{OrderID: orderID, Stage: "outbox", Err: err}
	}

	if r.skipGateway || r.gatewayQuotaExhausted() || midtransOID == "" {
		snap.Gateway = recon.GatewaySnapshot{
			MidtransOrderID: midtransOID,
			Available:       false,
			QueriedAt:       now,
		}
	} else {
		gw, err := r.gateway.FetchStatus(ctx, midtransOID)
		if err != nil {
			// Failure is NOT fatal; classifier handles Available=false.
			snap.Gateway = recon.GatewaySnapshot{
				MidtransOrderID: midtransOID,
				Available:       false,
				QueriedAt:       now,
			}
			return snap, &ResolveError{OrderID: orderID, Stage: "gateway", Err: err}
		}
		r.gatewayUsed++
		snap.Gateway = gw
	}

	return snap, nil
}

// GatewayCallsUsed reports how many gateway HTTP calls succeeded so far in
// this resolver's lifetime (one per audit run).
func (r *Resolver) GatewayCallsUsed() int { return r.gatewayUsed }

func (r *Resolver) gatewayQuotaExhausted() bool {
	return r.gatewayBudget > 0 && r.gatewayUsed >= r.gatewayBudget
}

// CandidateScan returns up to `limit` orders created at or after `since`,
// most-recent first. Phase 1B uses this single broad scan rather than the
// drift-specific candidate query designed in the audit doc; broad scope is
// the point of the validation phase.
func (r *Resolver) CandidateScan(ctx context.Context, since time.Time, limit int) ([]uuid.UUID, error) {
	const q = `
		SELECT id
		FROM orders
		WHERE created_at >= $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, q, since, limit)
	if err != nil {
		return nil, fmt.Errorf("candidate scan: %w", err)
	}
	defer rows.Close()

	out := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan candidate row: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Per-table fetchers — all SELECT-only.
// ---------------------------------------------------------------------------

func (r *Resolver) fetchOrder(ctx context.Context, orderID uuid.UUID) (*recon.OrderRow, error) {
	// CANONICAL SOURCE: total_before_coins_amount = PD + S is the buyer-funded
	// escrow base. orders.escrow_amount is NOT authoritative (never persisted).
	const q = `
		SELECT id, status::text, escrow_status::text, total_before_coins_amount,
		       has_dispute, created_at, updated_at
		FROM orders
		WHERE id = $1
	`
	var out recon.OrderRow
	err := r.pool.QueryRow(ctx, q, orderID).Scan(
		&out.ID, &out.Status, &out.EscrowStatus, &out.GrossAmount,
		&out.HasDispute, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("order not found: %s", orderID)
		}
		return nil, err
	}
	return &out, nil
}

func (r *Resolver) fetchPaymentByOrderID(ctx context.Context, orderID uuid.UUID) (*recon.PaymentRow, error) {
	// At most one ACTIVE payment per order (idx_active_payment_per_order)
	// but there can be historical rows for the same order_id (e.g. one
	// expired + one paid). We pick the freshest by created_at — that is
	// the payment the order is currently tied to.
	const q = `
		SELECT id, user_id, midtrans_order_id, COALESCE(transaction_id, ''),
		       gross_amount, status::text, reference_type, reference_id,
		       paid_at, expired_at, created_at
		FROM payments
		WHERE reference_type = 'order' AND reference_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	var out recon.PaymentRow
	var paidAt sql.NullTime
	err := r.pool.QueryRow(ctx, q, orderID).Scan(
		&out.ID, &out.UserID, &out.MidtransOrderID, &out.TransactionID,
		&out.GrossAmount, &out.Status, &out.ReferenceType, &out.ReferenceID,
		&paidAt, &out.ExpiredAt, &out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if paidAt.Valid {
		v := paidAt.Time
		out.PaidAt = &v
	}
	return &out, nil
}

func (r *Resolver) fetchEscrow(ctx context.Context, orderID uuid.UUID) (*recon.EscrowRow, error) {
	const q = `
		SELECT id, order_id, status::text, amount, created_at, released_at, refunded_at
		FROM escrows
		WHERE order_id = $1
	`
	var out recon.EscrowRow
	var releasedAt, refundedAt sql.NullTime
	err := r.pool.QueryRow(ctx, q, orderID).Scan(
		&out.ID, &out.OrderID, &out.Status, &out.Amount,
		&out.CreatedAt, &releasedAt, &refundedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if releasedAt.Valid {
		v := releasedAt.Time
		out.ReleasedAt = &v
	}
	if refundedAt.Valid {
		v := refundedAt.Time
		out.RefundedAt = &v
	}
	return &out, nil
}

func (r *Resolver) fetchRefunds(ctx context.Context, orderID uuid.UUID) ([]recon.RefundRow, error) {
	const q = `
		SELECT id, order_id, requested_amount, status::text,
		       gateway_status,
		       COALESCE(gateway_refund_id, ''),
		       COALESCE(gateway_idempotency_key, ''),
		       gateway_requested_at, gateway_acknowledged_at
		FROM refunds
		WHERE order_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]recon.RefundRow, 0)
	for rows.Next() {
		var r recon.RefundRow
		var req, ack sql.NullTime
		if err := rows.Scan(
			&r.ID, &r.OrderID, &r.RequestedAmount, &r.Status,
			&r.GatewayStatus,
			&r.GatewayRefundID,
			&r.GatewayIdempotencyKey,
			&req, &ack,
		); err != nil {
			return nil, err
		}
		if req.Valid {
			v := req.Time
			r.GatewayRequestedAt = &v
		}
		if ack.Valid {
			v := ack.Time
			r.GatewayAcknowledgedAt = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (r *Resolver) fetchWebhooks(ctx context.Context, midtransOID string) ([]recon.WebhookEventRef, error) {
	const q = `
		SELECT COALESCE(event_id, ''),
		       COALESCE(midtrans_order_id, ''),
		       status::text,
		       COALESCE(payload->>'transaction_status', ''),
		       COALESCE(payload->>'transaction_id', ''),
		       received_at,
		       processed_at
		FROM payment_webhook_events
		WHERE midtrans_order_id = $1
		ORDER BY received_at ASC
	`
	rows, err := r.pool.Query(ctx, q, midtransOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]recon.WebhookEventRef, 0)
	for rows.Next() {
		var w recon.WebhookEventRef
		var processedAt sql.NullTime
		if err := rows.Scan(
			&w.EventID, &w.MidtransOrderID, &w.Status,
			&w.TransactionStatus, &w.TransactionID,
			&w.ReceivedAt, &processedAt,
		); err != nil {
			return nil, err
		}
		if processedAt.Valid {
			v := processedAt.Time
			w.ProcessedAt = &v
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Canonical ledger idempotency keys (owner-locked, verified against
// runtime writers in 2026-05-12 Phase 1B validation):
//
//	payment_settlement_<provider_transaction_id>
//	    written by FinanceService.RecordGatewayPaymentSettlement —
//	    finance_service.go:552. The substituted value is the Midtrans-side
//	    transaction_id (payments.transaction_id), NOT the order UUID.
//	order_release_<order_id>
//	    written by FinanceService.RecordOrderRelease — finance_service.go:428.
//	refund_reversal_<refund_id>
//	    written by FinanceService refund reversal path.
//
// These strings are stable per owner decision and live here rather than in
// the classifier because the classifier abstracts over LedgerLookup.
const (
	ledgerKeyBuyerSettleFmt    = "payment_settlement_%s"
	ledgerKeyOrderReleaseFmt   = "order_release_%s"
	ledgerKeyRefundReversalFmt = "refund_reversal_%s"
)

func (r *Resolver) fetchLedgerLookups(ctx context.Context, snap *recon.Snapshot, orderID uuid.UUID, refunds []recon.RefundRow) error {
	releaseKey := fmt.Sprintf(ledgerKeyOrderReleaseFmt, orderID)

	// One query per key keeps the SQL trivially correct and replay-stable.
	const exists = `SELECT EXISTS(SELECT 1 FROM ledger_transactions WHERE idempotency_key = $1)`
	const releaseAmt = `SELECT COALESCE((SELECT total_debit FROM ledger_transactions WHERE idempotency_key = $1), 0)`

	// Buyer-settle key substitutes payments.transaction_id (the gateway-
	// assigned provider_transaction_id), not the order UUID. When no payment
	// row resolved, or the payment hasn't yet received a settlement webhook
	// (transaction_id empty), the canonical entry cannot exist; record false
	// and let the classifier reason about it via D7/D15.
	if snap.Payment != nil && snap.Payment.TransactionID != "" {
		settleKey := fmt.Sprintf(ledgerKeyBuyerSettleFmt, snap.Payment.TransactionID)
		if err := r.pool.QueryRow(ctx, exists, settleKey).Scan(&snap.Ledger.BuyerSettlementExists); err != nil {
			return fmt.Errorf("settle exists: %w", err)
		}
	}
	if err := r.pool.QueryRow(ctx, exists, releaseKey).Scan(&snap.Ledger.OrderReleaseExists); err != nil {
		return fmt.Errorf("release exists: %w", err)
	}
	if snap.Ledger.OrderReleaseExists {
		if err := r.pool.QueryRow(ctx, releaseAmt, releaseKey).Scan(&snap.Ledger.OrderReleaseAmount); err != nil {
			return fmt.Errorf("release amount: %w", err)
		}
	}

	for _, rf := range refunds {
		key := fmt.Sprintf(ledgerKeyRefundReversalFmt, rf.ID)
		var present bool
		if err := r.pool.QueryRow(ctx, exists, key).Scan(&present); err != nil {
			return fmt.Errorf("refund reversal exists for %s: %w", rf.ID, err)
		}
		if present {
			snap.Ledger.RefundReversalExistsByRefundID[rf.ID] = true
		}
	}
	return nil
}

// Canonical outbox idempotency keys (format `{eventType}.{entityID}` per
// existing convention — see refund_gateway.go:770 emitGatewayOutbox helper).
//
//	money.released.<order_id>
//	money.refund_succeeded.<refund_id>
//	coins.refund_required.<order_id>
//
// Liveness (owner-locked Phase 1A decision):
//
//	status ∈ {pending, processing, succeeded, failed} → ALIVE
//	status = dead_letter                              → DEAD
//	row absent                                        → DEAD
const (
	outboxKeyMoneyReleasedFmt        = "money.released.%s"
	outboxKeyMoneyRefundSucceededFmt = "money.refund_succeeded.%s"
	outboxKeyCoinsRefundRequiredFmt  = "coins.refund_required.%s"
)

func (r *Resolver) fetchOutboxLookups(ctx context.Context, snap *recon.Snapshot, orderID uuid.UUID, refunds []recon.RefundRow) error {
	const aliveQuery = `
		SELECT EXISTS(
			SELECT 1 FROM outbox
			WHERE idempotency_key = $1
			  AND status <> 'dead_letter'
		)
	`

	releaseKey := fmt.Sprintf(outboxKeyMoneyReleasedFmt, orderID)
	coinsKey := fmt.Sprintf(outboxKeyCoinsRefundRequiredFmt, orderID)

	var releaseAlive, coinsAlive bool
	if err := r.pool.QueryRow(ctx, aliveQuery, releaseKey).Scan(&releaseAlive); err != nil {
		return fmt.Errorf("money.released alive: %w", err)
	}
	if err := r.pool.QueryRow(ctx, aliveQuery, coinsKey).Scan(&coinsAlive); err != nil {
		return fmt.Errorf("coins.refund_required alive: %w", err)
	}
	if releaseAlive {
		snap.Outbox.MoneyReleasedAliveByOrderID[orderID] = true
	}
	if coinsAlive {
		snap.Outbox.CoinsRefundRequiredAliveByOrderID[orderID] = true
	}

	for _, rf := range refunds {
		key := fmt.Sprintf(outboxKeyMoneyRefundSucceededFmt, rf.ID)
		var alive bool
		if err := r.pool.QueryRow(ctx, aliveQuery, key).Scan(&alive); err != nil {
			return fmt.Errorf("money.refund_succeeded alive for %s: %w", rf.ID, err)
		}
		if alive {
			snap.Outbox.MoneyRefundSucceededAliveByRefundID[rf.ID] = true
		}
	}
	return nil
}


