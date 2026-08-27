// Scenario D11 — stuck pending refund (canonical detector).
//
// CANONICAL HYPOTHESIS:
//
//	Gateway accepted a refund request (refunds.gateway_status='pending',
//	gateway_refund_id non-empty, gateway_requested_at set) BUT the local
//	acknowledgement webhook for that refund never reached the canonical
//	handler. Once the refund row's age exceeds Thresholds.StuckRefundGrace,
//	the recon classifier emits DriftD11StuckPendingRefund.
//
// FORENSIC INTENT:
//
//	This scenario produces D11 ON PURPOSE using only canonical primitives:
//	  * Refund dispatch goes through OrderPaymentService.InitiateGatewayRefundForOrder.
//	  * Webhook loss is simulated by the dev-only webhook_drop middleware
//	    hot-armed AFTER the original settlement callback has been observed.
//	  * No fake rows. No direct SQL mutation of escrow/refund state. No
//	    threshold shrinking. The grace wait is real wall-clock.
//
// THE OPERATOR MUST HAVE ALREADY:
//
//	1. Created an order via the canonical flow (mobile app / API).
//	2. Paid that order via the Midtrans sandbox UI; settlement webhook has
//	   reached core_server and flipped payments.status='paid',
//	   escrow.status='holding', orders.status='paid'.
//
// THEN INVOKE:
//
//	./corpus_driver --mode=scenario-d11 \
//	    --order-id=<uuid> \
//	    --scenario-tag=<short tag, e.g. D11_2026_05_12_A> \
//	    [--refund-amount=<int; default = order gross>] \
//	    [--core-server-url=http://localhost:8080] \
//	    [--stuck-refund-grace=5m] \
//	    [--observation-window=30s] \
//	    [--output-dir=audit_runs]
//
// OUTPUT:
//
//	* Six SCENARIO_D11_* structured log lines on stdout/stderr (boundary markers).
//	* JSON timeline file at <output-dir>/scenario_d11_<tag>_<ts>.json.
//	* Recon audit report split into PRIMARY EXPECTED FINDING vs
//	  AMBIENT OPERATIONAL NOISE (no merging, no suppression).
//	* Final D11 verdict line: DETECTED | NOT_DETECTED + diagnostic notes.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/integration/payment/application/recon"
	"github.com/labuda/backend/internal/integration/payment/application/recon/audit"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/serverboot"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/db"
)

type scenarioD11Flags struct {
	orderID           string
	scenarioTag       string
	refundAmount      int64
	reason            string
	coreServerURL     string
	stuckRefundGrace  time.Duration
	observationWindow time.Duration
	dispatchTimeout   time.Duration
	outputDir         string
}

// parseScenarioD11Flags reads scenario flags from the global flag set. It is
// called after the parent flag set has already been parsed.
func parseScenarioD11Flags() *scenarioD11Flags {
	f := &scenarioD11Flags{}
	fs := flag.NewFlagSet("scenario-d11", flag.ContinueOnError)
	fs.StringVar(&f.orderID, "order-id", "", "UUID of the already-paid order to refund (REQUIRED)")
	fs.StringVar(&f.scenarioTag, "scenario-tag", "", "short tag for this scenario run (REQUIRED)")
	fs.Int64Var(&f.refundAmount, "refund-amount", 0, "refund amount in minor units; 0 = full order gross")
	fs.StringVar(&f.reason, "reason", "scenario_d11_forensic", "human-readable reason recorded on the WEBHOOK_DROP_ARMED log")
	fs.StringVar(&f.coreServerURL, "core-server-url", "http://localhost:8080", "core_server base URL for the hot-arm endpoint")
	fs.DurationVar(&f.stuckRefundGrace, "stuck-refund-grace", 5*time.Minute, "D11 grace window passed to recon_audit; real wall-clock wait")
	fs.DurationVar(&f.observationWindow, "observation-window", 30*time.Second, "post-dispatch window before declaring the webhook dropped via no-ack")
	fs.DurationVar(&f.dispatchTimeout, "dispatch-timeout", 30*time.Second, "max wait for the refund row to reach gateway_status='pending'")
	fs.StringVar(&f.outputDir, "output-dir", "audit_runs", "directory where the timeline JSON is written")

	// flag.CommandLine has already been parsed in main(); pull remaining
	// args from there.
	_ = fs.Parse(flag.CommandLine.Args())
	return f
}

// scenarioD11Timeline is the canonical evidence record persisted at the end
// of the run. Field semantics MUST stay stable across runs so a forensic
// replay tool can correlate events.
type scenarioD11Timeline struct {
	ScenarioTag         string    `json:"scenario_tag"`
	OrderID             string    `json:"order_id"`
	MidtransOrderID     string    `json:"midtrans_order_id"`
	RefundID            string    `json:"refund_id,omitempty"`
	GatewayRefundID     string    `json:"gateway_refund_id,omitempty"`
	RefundAmount        int64     `json:"refund_amount"`
	StartedAt           time.Time `json:"started_at"`
	PaymentSettledAt    time.Time `json:"payment_settled_at"`
	WebhookDropArmedAt  time.Time `json:"webhook_drop_armed_at"`
	RefundDispatchedAt  time.Time `json:"refund_dispatched_at"`
	WebhookDropAt       time.Time `json:"webhook_drop_at"`
	GraceExpiredAt      time.Time `json:"grace_expired_at"`
	ReconAuditStartedAt time.Time `json:"recon_audit_started_at"`
	FinishedAt          time.Time `json:"finished_at"`

	StuckRefundGrace  time.Duration `json:"stuck_refund_grace"`
	ObservationWindow time.Duration `json:"observation_window"`

	Verdict      string   `json:"verdict"` // DETECTED | NOT_DETECTED | INCONCLUSIVE
	VerdictNotes []string `json:"verdict_notes,omitempty"`

	PrimaryFinding      *recon.Finding  `json:"primary_finding,omitempty"`
	AmbientFindings     []recon.Finding `json:"ambient_findings,omitempty"`
	AmbientFindingCount int             `json:"ambient_finding_count"`
}

func runScenarioD11(
	deps *serverboot.Dependencies,
	dbWrap *database.DB,
	cfg *config.Config,
	log *logger.Logger,
) error {
	f := parseScenarioD11Flags()
	if f.orderID == "" || f.scenarioTag == "" {
		return errors.New("scenario-d11 requires --order-id and --scenario-tag")
	}
	if deps.OrderService == nil {
		return errors.New("OrderService not wired on Dependencies; scenario cannot proceed")
	}
	orderID, err := uuid.Parse(f.orderID)
	if err != nil {
		return fmt.Errorf("parse --order-id: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tl := &scenarioD11Timeline{
		ScenarioTag:       f.scenarioTag,
		OrderID:           orderID.String(),
		StuckRefundGrace:  f.stuckRefundGrace,
		ObservationWindow: f.observationWindow,
		StartedAt:         time.Now().UTC(),
	}

	log.Info("SCENARIO_D11_START",
		zap.String("scenario_tag", f.scenarioTag),
		zap.String("order_id", orderID.String()),
		zap.Duration("stuck_refund_grace", f.stuckRefundGrace),
		zap.Duration("observation_window", f.observationWindow),
		zap.String("core_server_url", f.coreServerURL),
	)

	// ──────────────────────────────────────────────────────────────
	// STEP 1 — Prereq: order must be paid + escrow holding. We do NOT
	// drive the settlement here; the operator paid the order manually via
	// the Midtrans sandbox UI. Read-only inspection.
	// ──────────────────────────────────────────────────────────────
	paymentMeta, err := loadPaymentMeta(ctx, dbWrap, orderID)
	if err != nil {
		return fmt.Errorf("load payment meta: %w", err)
	}
	if paymentMeta.paymentStatus != "paid" {
		return fmt.Errorf("order %s is not paid yet (payments.status=%q); pay it via the sandbox first", orderID, paymentMeta.paymentStatus)
	}
	if paymentMeta.paidAt == nil {
		return fmt.Errorf("order %s: payments.paid_at is NULL despite status='paid'; investigate state inconsistency before re-running", orderID)
	}
	tl.MidtransOrderID = paymentMeta.midtransOrderID
	tl.PaymentSettledAt = paymentMeta.paidAt.UTC()
	log.Info("SCENARIO_D11_SETTLED",
		zap.String("midtrans_order_id", paymentMeta.midtransOrderID),
		zap.Time("payment_settled_at", tl.PaymentSettledAt),
		zap.Int64("payment_gross", paymentMeta.gross),
	)

	// ──────────────────────────────────────────────────────────────
	// STEP 2 — Hot-arm webhook drop for this midtrans_order_id. Done
	// AFTER settlement was already observed locally; this guarantees the
	// settlement callback was NOT dropped. Only the subsequent refund
	// callback will be suppressed.
	// ──────────────────────────────────────────────────────────────
	armedAt, err := armWebhookDrop(ctx, f.coreServerURL, paymentMeta.midtransOrderID, f.scenarioTag, f.reason)
	if err != nil {
		return fmt.Errorf("hot-arm webhook drop: %w", err)
	}
	tl.WebhookDropArmedAt = armedAt.UTC()
	log.Info("scenario_d11_webhook_drop_armed_locally",
		zap.String("midtrans_order_id", paymentMeta.midtransOrderID),
		zap.Time("armed_at", tl.WebhookDropArmedAt),
	)

	// ──────────────────────────────────────────────────────────────
	// STEP 3 — Dispatch the canonical gateway refund. Reuses the same
	// OrderPaymentService method that production callers use (e.g.
	// buyer-overdue auto-cancel and admin manual refund).
	// ──────────────────────────────────────────────────────────────
	refundAmt := f.refundAmount
	if refundAmt <= 0 {
		refundAmt = paymentMeta.gross
	}
	tl.RefundAmount = refundAmt

	idempotencyKey := fmt.Sprintf("scenario_d11_%s_%s", f.scenarioTag, orderID.String())
	dispatchErr := dbWrap.WithTx(ctx, func(tx db.Tx) error {
		order, err := deps.OrderService.GetOrder(ctx, tx, orderID)
		if err != nil {
			return fmt.Errorf("load order entity: %w", err)
		}
		return deps.OrderService.PaymentService().InitiateGatewayRefundForOrder(
			ctx, tx, order, auth.SystemCallerID,
			refundAmt, "other", idempotencyKey,
		)
	})
	if dispatchErr != nil {
		// Per OrderPaymentService contract, this means the gateway refused
		// synchronously (no in-flight refund). Surface to operator —
		// this is a VALID runtime discovery, not a scenario bug.
		log.Error("scenario_d11_refund_dispatch_rejected",
			zap.Error(dispatchErr),
			zap.String("order_id", orderID.String()),
		)
		tl.Verdict = "INCONCLUSIVE"
		tl.VerdictNotes = append(tl.VerdictNotes,
			fmt.Sprintf("refund dispatch failed synchronously: %v", dispatchErr),
			"this is a sandbox/gateway discovery, NOT a D11 condition; D11 requires gateway accept",
		)
		tl.FinishedAt = time.Now().UTC()
		_ = writeTimeline(f.outputDir, tl, log)
		return dispatchErr
	}

	refundSnap, err := pollRefundDispatched(ctx, dbWrap, orderID, idempotencyKey, f.dispatchTimeout)
	if err != nil {
		return fmt.Errorf("poll refund dispatched: %w", err)
	}
	tl.RefundID = refundSnap.id.String()
	tl.GatewayRefundID = refundSnap.gatewayRefundID
	tl.RefundDispatchedAt = refundSnap.gatewayRequestedAt.UTC()
	log.Info("SCENARIO_D11_REFUND_DISPATCHED",
		zap.String("refund_id", tl.RefundID),
		zap.String("gateway_refund_id", refundSnap.gatewayRefundID),
		zap.String("gateway_status", refundSnap.gatewayStatus),
		zap.Time("refund_dispatched_at", tl.RefundDispatchedAt),
		zap.Int64("refund_amount", refundAmt),
	)

	// ──────────────────────────────────────────────────────────────
	// STEP 4 — Observe webhook drop. We do NOT have direct evidence of
	// "the webhook arrived and was suppressed by the middleware" from
	// the corpus_driver process, because that log lives in core_server's
	// stdout. The local detector is: after the observation window,
	// refunds.gateway_acknowledged_at remains NULL AND the row's
	// gateway_status is still 'pending'. That is the canonical
	// "no local acknowledgement" condition D11 is meant to detect.
	// ──────────────────────────────────────────────────────────────
	deadline := time.Now().UTC().Add(f.observationWindow)
	select {
	case <-time.After(time.Until(deadline)):
	case <-ctx.Done():
		return ctx.Err()
	}
	postObs, err := loadRefund(ctx, dbWrap, refundSnap.id)
	if err != nil {
		return fmt.Errorf("re-read refund after observation window: %w", err)
	}
	tl.WebhookDropAt = time.Now().UTC()
	dropConfirmed := postObs.gatewayAcknowledgedAt == nil && postObs.gatewayStatus == "pending"
	log.Info("SCENARIO_D11_WEBHOOK_DROPPED",
		zap.Time("webhook_drop_at", tl.WebhookDropAt),
		zap.Bool("no_ack_confirmed", dropConfirmed),
		zap.String("gateway_status_after_window", postObs.gatewayStatus),
		zap.Bool("acknowledged_at_null", postObs.gatewayAcknowledgedAt == nil),
	)
	if !dropConfirmed {
		tl.VerdictNotes = append(tl.VerdictNotes,
			fmt.Sprintf("after %s the refund row showed gateway_status=%q acknowledged_at_null=%v — webhook was NOT dropped; this is a sandbox semantics discovery",
				f.observationWindow, postObs.gatewayStatus, postObs.gatewayAcknowledgedAt == nil),
		)
	}

	// ──────────────────────────────────────────────────────────────
	// STEP 5 — Wait until the recon grace expires. Real wall-clock; no
	// shrinking. Grace anchor is refund.gateway_requested_at + grace.
	// ──────────────────────────────────────────────────────────────
	tl.GraceExpiredAt = tl.RefundDispatchedAt.Add(f.stuckRefundGrace)
	wait := time.Until(tl.GraceExpiredAt)
	if wait > 0 {
		log.Info("scenario_d11_waiting_for_grace_expiry",
			zap.Time("grace_expired_at", tl.GraceExpiredAt),
			zap.Duration("wait", wait),
		)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	log.Info("SCENARIO_D11_GRACE_EXPIRED",
		zap.Time("grace_expired_at", tl.GraceExpiredAt),
	)

	// ──────────────────────────────────────────────────────────────
	// STEP 6 — Run recon_audit against this single order, in-process.
	// We reuse the same audit lib that the recon_audit binary uses; that
	// keeps the classifier path identical to the production tool.
	// ──────────────────────────────────────────────────────────────
	tl.ReconAuditStartedAt = time.Now().UTC()
	log.Info("SCENARIO_D11_AUDIT_RUN",
		zap.Time("recon_audit_started_at", tl.ReconAuditStartedAt),
		zap.String("order_id", orderID.String()),
	)
	report, err := runScenarioAudit(ctx, cfg, dbWrap, orderID, f.stuckRefundGrace, log)
	if err != nil {
		return fmt.Errorf("recon audit: %w", err)
	}

	// ──────────────────────────────────────────────────────────────
	// STEP 7 — Distinguish PRIMARY (D11 for our order) from AMBIENT
	// (everything else). No merging, no suppression — operator sees both.
	// ──────────────────────────────────────────────────────────────
	var primary *recon.Finding
	for i := range report.Findings {
		fnd := &report.Findings[i]
		if fnd.DriftClass == recon.DriftD11StuckPendingRefund &&
			fnd.OrderID != nil && *fnd.OrderID == orderID {
			primary = fnd
			break
		}
	}
	tl.PrimaryFinding = primary
	for i := range report.Findings {
		fnd := report.Findings[i]
		if primary != nil &&
			fnd.DriftClass == primary.DriftClass &&
			fnd.OrderID != nil && primary.OrderID != nil &&
			*fnd.OrderID == *primary.OrderID {
			continue
		}
		tl.AmbientFindings = append(tl.AmbientFindings, fnd)
	}
	tl.AmbientFindingCount = len(tl.AmbientFindings)

	switch {
	case primary != nil:
		tl.Verdict = "DETECTED"
	case dropConfirmed:
		tl.Verdict = "NOT_DETECTED"
		tl.VerdictNotes = append(tl.VerdictNotes,
			"webhook drop was confirmed (no ack) AND grace elapsed, yet recon classifier did NOT raise D11 — classifier regression suspected",
		)
	default:
		tl.Verdict = "INCONCLUSIVE"
		tl.VerdictNotes = append(tl.VerdictNotes,
			"webhook drop was NOT confirmed AND classifier did not raise D11 — sandbox returned terminal ack synchronously or webhook was not actually suppressed",
		)
	}
	tl.FinishedAt = time.Now().UTC()

	// ──────────────────────────────────────────────────────────────
	// STEP 8 — Persist timeline + print report split.
	// ──────────────────────────────────────────────────────────────
	if err := writeTimeline(f.outputDir, tl, log); err != nil {
		log.Warn("scenario_d11_timeline_write_failed", zap.Error(err))
	}
	printScenarioD11Report(tl, report, primary)
	return nil
}

// ════════════════════════════════════════════════════════════════════════
// DB readers (read-only — corpus_driver MUST NOT mutate canonical state
// outside the canonical service call path).
// ════════════════════════════════════════════════════════════════════════

type paymentMeta struct {
	paymentID       uuid.UUID
	midtransOrderID string
	paymentStatus   string
	paidAt          *time.Time
	gross           int64
}

func loadPaymentMeta(ctx context.Context, dbWrap *database.DB, orderID uuid.UUID) (*paymentMeta, error) {
	row := dbWrap.Pool().QueryRow(ctx, `
		SELECT id, midtrans_order_id, status, paid_at, gross_amount
		FROM payments
		WHERE reference_type = 'order' AND reference_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, orderID)
	pm := &paymentMeta{}
	if err := row.Scan(&pm.paymentID, &pm.midtransOrderID, &pm.paymentStatus, &pm.paidAt, &pm.gross); err != nil {
		return nil, err
	}
	return pm, nil
}

type refundSnapshot struct {
	id                    uuid.UUID
	gatewayStatus         string
	gatewayRefundID       string
	gatewayRequestedAt    time.Time
	gatewayAcknowledgedAt *time.Time
}

func loadRefund(ctx context.Context, dbWrap *database.DB, refundID uuid.UUID) (*refundSnapshot, error) {
	row := dbWrap.Pool().QueryRow(ctx, `
		SELECT id,
		       gateway_status,
		       COALESCE(gateway_refund_id, ''),
		       gateway_requested_at,
		       gateway_acknowledged_at
		FROM refunds
		WHERE id = $1
	`, refundID)
	rs := &refundSnapshot{}
	var reqAt *time.Time
	if err := row.Scan(&rs.id, &rs.gatewayStatus, &rs.gatewayRefundID, &reqAt, &rs.gatewayAcknowledgedAt); err != nil {
		return nil, err
	}
	if reqAt != nil {
		rs.gatewayRequestedAt = *reqAt
	}
	return rs, nil
}

// pollRefundDispatched waits for the refund row matching the scenario's
// idempotency key to reach gateway_status='pending' with a non-empty
// gateway_refund_id and a populated gateway_requested_at. That is the
// canonical "gateway accepted" state we need before we can assert the
// webhook-drop-after-accept invariant. Polls every 500ms.
func pollRefundDispatched(ctx context.Context, dbWrap *database.DB, orderID uuid.UUID, idempotencyKey string, timeout time.Duration) (*refundSnapshot, error) {
	deadline := time.Now().Add(timeout)
	for {
		row := dbWrap.Pool().QueryRow(ctx, `
			SELECT id,
			       gateway_status,
			       COALESCE(gateway_refund_id, ''),
			       gateway_requested_at,
			       gateway_acknowledged_at
			FROM refunds
			WHERE order_id = $1 AND gateway_idempotency_key = $2
			ORDER BY created_at DESC
			LIMIT 1
		`, orderID, idempotencyKey)
		rs := &refundSnapshot{}
		var reqAt *time.Time
		err := row.Scan(&rs.id, &rs.gatewayStatus, &rs.gatewayRefundID, &reqAt, &rs.gatewayAcknowledgedAt)
		if err == nil && rs.gatewayStatus == "pending" && rs.gatewayRefundID != "" && reqAt != nil {
			rs.gatewayRequestedAt = *reqAt
			return rs, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("refund did not reach gateway_status='pending' with non-empty gateway_refund_id within %s (last_status=%q gateway_refund_id_empty=%v scan_err=%v)",
				timeout, rs.gatewayStatus, rs.gatewayRefundID == "", err)
		}
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// ════════════════════════════════════════════════════════════════════════
// Hot-arm client.
// ════════════════════════════════════════════════════════════════════════

func armWebhookDrop(ctx context.Context, baseURL, midtransOrderID, scenarioTag, reason string) (time.Time, error) {
	url := strings.TrimRight(baseURL, "/") + "/dev/webhook-drop/arm"
	body, _ := json.Marshal(map[string]string{
		"midtrans_order_id": midtransOrderID,
		"scenario_tag":      scenarioTag,
		"reason":            reason,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return time.Time{}, fmt.Errorf("hot-arm returned %d: %s", resp.StatusCode, strings.TrimSpace(buf.String()))
	}
	return time.Now().UTC(), nil
}

// ════════════════════════════════════════════════════════════════════════
// In-process recon audit.
// ════════════════════════════════════════════════════════════════════════

func runScenarioAudit(
	ctx context.Context,
	cfg *config.Config,
	dbWrap *database.DB,
	orderID uuid.UUID,
	stuckRefundGrace time.Duration,
	log *logger.Logger,
) (*audit.Report, error) {
	gw := audit.NewGatewayClient(
		cfg.Midtrans.ServerKey,
		cfg.Midtrans.Environment == "production",
		15*time.Second,
	)
	resolver := audit.NewResolver(audit.ResolverConfig{
		Pool:    dbWrap.Pool(),
		Gateway: gw,
		Thresholds: recon.Thresholds{
			PendingPaymentGrace:       3 * time.Minute,
			OrphanRecoveryGrace:       2 * time.Minute,
			StuckRefundGrace:          stuckRefundGrace,
			PendingPaymentExpiryGrace: 1 * time.Minute,
		},
		SkipGateway:   false,
		GatewayBudget: 50,
	})
	orch := audit.NewOrchestrator(resolver, func() time.Time { return time.Now().UTC() })
	report, err := orch.Run(ctx, audit.RunSpec{
		OrderIDs: []uuid.UUID{orderID},
		Since:    time.Now().UTC().Add(-24 * time.Hour),
		Limit:    1,
	})
	if err != nil {
		log.Warn("scenario_d11_audit_run_error_nonfatal", zap.Error(err))
	}
	return report, nil
}

// ════════════════════════════════════════════════════════════════════════
// Output.
// ════════════════════════════════════════════════════════════════════════

func writeTimeline(outputDir string, tl *scenarioD11Timeline, log *logger.Logger) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	ts := tl.StartedAt.Format("20060102T150405Z")
	safeTag := strings.NewReplacer("/", "_", " ", "_").Replace(tl.ScenarioTag)
	path := filepath.Join(outputDir, fmt.Sprintf("scenario_d11_%s_%s.json", safeTag, ts))
	data, err := json.MarshalIndent(tl, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	log.Info("scenario_d11_timeline_written", zap.String("path", path))
	return nil
}

func printScenarioD11Report(tl *scenarioD11Timeline, report *audit.Report, primary *recon.Finding) {
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println("SCENARIO D11 REPORT")
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Printf("Scenario tag:        %s\n", tl.ScenarioTag)
	fmt.Printf("Order ID:            %s\n", tl.OrderID)
	fmt.Printf("Midtrans order ID:   %s\n", tl.MidtransOrderID)
	fmt.Printf("Refund ID:           %s\n", tl.RefundID)
	fmt.Printf("Gateway refund ID:   %s\n", tl.GatewayRefundID)
	fmt.Printf("Refund amount:       %d\n", tl.RefundAmount)
	fmt.Println()
	fmt.Println("─── CANONICAL TIMELINE ─────────────────────────────────────")
	fmt.Printf("  payment_settled_at      : %s\n", tl.PaymentSettledAt.Format(time.RFC3339))
	fmt.Printf("  webhook_drop_armed_at   : %s\n", tl.WebhookDropArmedAt.Format(time.RFC3339))
	fmt.Printf("  refund_dispatched_at    : %s\n", tl.RefundDispatchedAt.Format(time.RFC3339))
	fmt.Printf("  webhook_drop_at         : %s  (post-observation no-ack confirmation)\n", tl.WebhookDropAt.Format(time.RFC3339))
	fmt.Printf("  grace_expired_at        : %s  (refund_dispatched_at + %s)\n", tl.GraceExpiredAt.Format(time.RFC3339), tl.StuckRefundGrace)
	fmt.Printf("  recon_audit_started_at  : %s\n", tl.ReconAuditStartedAt.Format(time.RFC3339))
	fmt.Println()

	fmt.Println("─── PRIMARY EXPECTED FINDING ───────────────────────────────")
	if primary != nil {
		fmt.Printf("  drift_class      : %s\n", primary.DriftClass)
		fmt.Printf("  severity         : %s\n", primary.Severity)
		fmt.Printf("  order_id         : %s\n", uuidStr(primary.OrderID))
		fmt.Printf("  refund_id        : %s\n", uuidStr(primary.RefundID))
		fmt.Printf("  detected_at      : %s\n", primary.DetectedAt.Format(time.RFC3339))
		fmt.Printf("  suggested_action : %s\n", primary.SuggestedAction)
		fmt.Printf("  notes            : %s\n", primary.Notes)
	} else {
		fmt.Println("  (none — D11 was NOT raised for this order)")
	}
	fmt.Println()

	fmt.Println("─── AMBIENT OPERATIONAL NOISE ──────────────────────────────")
	fmt.Printf("  count: %d\n", tl.AmbientFindingCount)
	if tl.AmbientFindingCount == 0 {
		fmt.Println("  (none)")
	} else {
		for i, fnd := range tl.AmbientFindings {
			fmt.Printf("  [%d] drift_class=%s severity=%s order_id=%s notes=%s\n",
				i+1, fnd.DriftClass, fnd.Severity, uuidStr(fnd.OrderID), fnd.Notes)
		}
	}
	fmt.Println()

	fmt.Printf("─── VERDICT: D11 %s ───────────────────────────────────────\n", tl.Verdict)
	for _, note := range tl.VerdictNotes {
		fmt.Printf("  note: %s\n", note)
	}
	if report != nil {
		fmt.Printf("  audit duration: %.2fs, snapshots resolved: %d, errors: %d\n",
			report.DurationSeconds, report.SnapshotsResolved, len(report.Errors))
	}
	fmt.Println("════════════════════════════════════════════════════════════")
}

func uuidStr(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
