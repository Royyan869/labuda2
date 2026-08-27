package recon

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Classify is the pure deterministic entry point of Phase 1A. Given a
// resolved Snapshot it returns zero or more Findings, sorted deterministically
// by (DriftClass, IdempotencyKey). The same Snapshot value MUST produce the
// same Finding slice on every invocation; this is the replay-safety contract.
//
// Classify performs NO DB writes, NO gateway calls, NO logging, NO time
// reads. Anything that would break determinism is a bug.
func Classify(s Snapshot) []Finding {
	findings := make([]Finding, 0, 8)

	findings = appendAll(findings, detectD1(s))
	findings = appendAll(findings, detectD2(s))
	findings = appendAll(findings, detectD3(s))
	findings = appendAll(findings, detectD4(s))
	findings = appendAll(findings, detectD5(s))
	findings = appendAll(findings, detectD6(s))
	findings = appendAll(findings, detectD7(s))
	findings = appendAll(findings, detectD8(s))
	findings = appendAll(findings, detectD9(s))
	findings = appendAll(findings, detectD10(s))
	findings = appendAll(findings, detectD11(s))
	findings = appendAll(findings, detectD12(s))
	findings = appendAll(findings, detectD13(s))
	findings = appendAll(findings, detectD14(s))
	findings = appendAll(findings, detectD15(s))

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].DriftClass != findings[j].DriftClass {
			return findings[i].DriftClass < findings[j].DriftClass
		}
		return findings[i].IdempotencyKey < findings[j].IdempotencyKey
	})
	return findings
}

func appendAll(dst []Finding, src []Finding) []Finding {
	if len(src) == 0 {
		return dst
	}
	return append(dst, src...)
}

// ---------------------------------------------------------------------------
// Detectors
// ---------------------------------------------------------------------------

// D1: gateway settled / captured, local payment still pending past the grace
// window, with no orphaned-webhook recovery in flight.
func detectD1(s Snapshot) []Finding {
	if !s.Gateway.Available {
		return nil
	}
	if s.Payment == nil {
		return nil
	}
	if s.Payment.Status != LocalPaymentStatusPending {
		return nil
	}
	if !gatewayIsSettled(s.Gateway.TransactionStatus) {
		return nil
	}
	if s.Payment.CreatedAt.Add(s.Thresholds.PendingPaymentGrace).After(s.Now) {
		return nil
	}
	if orphanRecoveryInFlight(s) {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD1GatewaySettledLocalUnpaid, SeverityCritical,
			"replay webhook via admin /admin/payments/{order_id}/resync-from-gateway",
			fmt.Sprintf("gateway tx=%s but payment row still pending", s.Gateway.TransactionStatus),
			s.Gateway.GrossAmount, s.Payment.GrossAmount, nil),
	}
}

// D2: local payment marked paid (paid_at set) but gateway returned a terminal
// failure status. Phantom settlement signature; never auto-correct.
func detectD2(s Snapshot) []Finding {
	if !s.Gateway.Available {
		return nil
	}
	if s.Payment == nil {
		return nil
	}
	if s.Payment.Status != LocalPaymentStatusPaid || s.Payment.PaidAt == nil {
		return nil
	}
	if !gatewayIsTerminalFailure(s.Gateway.TransactionStatus) {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD2LocalPaidGatewayTerminalFailure, SeverityCriticalSecurity,
			"route to security oncall — phantom settlement implies webhook forgery or replay; never auto-correct",
			fmt.Sprintf("local paid_at=%s, gateway status=%s", s.Payment.PaidAt.UTC().Format(time.RFC3339), s.Gateway.TransactionStatus),
			s.Gateway.GrossAmount, s.Payment.GrossAmount, nil),
	}
}

// D3: gateway reports at least one successful refund whose cumulative total
// equals or exceeds the order gross, but escrow.status is still holding.
// Partial-only refund mismatches surface in D4.
func detectD3(s Snapshot) []Finding {
	if !s.Gateway.Available {
		return nil
	}
	if s.Escrow == nil || s.Escrow.Status != EscrowStatusHolding {
		return nil
	}
	if s.Order == nil {
		return nil
	}
	gatewaySuccessTotal := sumGatewaySuccessfulRefunds(s.Gateway.RefundChargebackHistory)
	if gatewaySuccessTotal <= 0 {
		return nil
	}
	if gatewaySuccessTotal < s.Order.GrossAmount {
		// Partial; D4 handles aggregation mismatches.
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD3GatewayRefundedLocalHolding, SeverityCritical,
			"replay RefundService.HandleGatewayRefundAck via operator endpoint",
			"gateway cumulative refund >= order gross while escrow still holding",
			gatewaySuccessTotal, s.Escrow.Amount, nil),
	}
}

// D4: cumulative refund total disagrees between gateway and local refunds
// (any successful gateway entry not mirrored locally as gateway_status=
// succeeded, or vice versa). Excludes the D3 / fully-converged case.
func detectD4(s Snapshot) []Finding {
	if !s.Gateway.Available {
		return nil
	}
	gatewaySuccessTotal := sumGatewaySuccessfulRefunds(s.Gateway.RefundChargebackHistory)
	localSuccessTotal := sumLocalSuccessfulRefunds(s.Refunds)
	if gatewaySuccessTotal == localSuccessTotal {
		return nil
	}
	if gatewaySuccessTotal == 0 && localSuccessTotal == 0 {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD4PartialRefundMismatch, SeverityHigh,
			"reconcile refund rows via operator /admin/refunds/{id}/resync-from-gateway",
			fmt.Sprintf("gateway successful refund total=%d, local successful refund total=%d", gatewaySuccessTotal, localSuccessTotal),
			gatewaySuccessTotal, localSuccessTotal, nil),
	}
}

// D5: two or more distinct gateway transactions attest a settled state for
// the same midtrans_order_id. Lifecycle progression (pending → settlement
// → capture) does NOT trigger D5: only settled-class transaction_status
// rows are counted, and we deduplicate by payload transaction_id so a
// Midtrans-side retry of the same transaction (legitimate re-delivery)
// collapses to one. Distinct transaction_ids attesting settlement for the
// same order is the duplicate-settlement signature.
func detectD5(s Snapshot) []Finding {
	if len(s.Webhooks) < 2 {
		return nil
	}
	distinctTxIDs := make(map[string]struct{})
	for _, w := range s.Webhooks {
		// Skip rows we never trusted (failed/orphaned/manual-review/quarantine/
		// terminal-review processing): their payload claim is meaningless because
		// we haven't reconciled it.
		if w.Status == WebhookStatusFailed ||
			w.Status == WebhookStatusOrphaned ||
			w.Status == WebhookStatusManualReview ||
			w.Status == WebhookStatusQuarantined ||
			w.Status == WebhookStatusTerminalReview {
			continue
		}
		if !gatewayIsSettled(w.TransactionStatus) {
			continue
		}
		// A settled webhook missing a payload transaction_id cannot be
		// deduplicated against legitimate retries — fall back to event_id
		// so we don't silently miss a real attacker-injected duplicate.
		key := w.TransactionID
		if key == "" {
			key = "event:" + w.EventID
		}
		distinctTxIDs[key] = struct{}{}
	}
	if len(distinctTxIDs) < 2 {
		return nil
	}
	mtID := s.Gateway.MidtransOrderID
	if mtID == "" && s.Payment != nil {
		mtID = s.Payment.MidtransOrderID
	}
	return []Finding{
		buildFinding(s, DriftD5DuplicateSettlement, SeverityHigh,
			"manual investigation — verify event_id UNIQUE constraint and webhook signature integrity",
			fmt.Sprintf("%d distinct gateway transaction_id values attest settlement for midtrans_order_id=%s", len(distinctTxIDs), mtID),
			0, 0, nil),
	}
}

// D6: gateway reports a terminal state for the order but no webhook row
// exists at all (or all rows are failed terminal). Orphan-aware: suppressed
// while an orphaned webhook is within recovery grace.
func detectD6(s Snapshot) []Finding {
	if !s.Gateway.Available {
		return nil
	}
	if !gatewayIsTerminal(s.Gateway.TransactionStatus) {
		return nil
	}
	if orphanRecoveryInFlight(s) {
		return nil
	}
	if hasNonFailedWebhook(s.Webhooks) {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD6MissingWebhookDelivery, SeverityHigh,
			"replay webhook via /admin/payments/{order_id}/resync-from-gateway",
			fmt.Sprintf("gateway terminal status=%s, no live webhook event for midtrans_order_id=%s", s.Gateway.TransactionStatus, s.Gateway.MidtransOrderID),
			s.Gateway.GrossAmount, 0, nil),
	}
}

// D7: a webhook event reached the terminal succeeded state for this order but
// the corresponding ledger settlement entry is absent.
func detectD7(s Snapshot) []Finding {
	if s.Payment == nil || s.Order == nil {
		return nil
	}
	if !hasSucceededWebhook(s.Webhooks) {
		return nil
	}
	if !isLocalPaymentSettled(s.Payment.Status) {
		return nil
	}
	if s.Ledger.BuyerSettlementExists {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD7WebhookProcessedLedgerAbsent, SeverityCritical,
			"investigate FinanceService.RecordOrderSettlement; replay via operator if intent is clear",
			"webhook succeeded but no settlement ledger entry exists",
			0, s.Payment.GrossAmount, nil),
	}
}

// D8: escrow row exists, orders.escrow_status projection is non-'none' but
// disagrees with escrows.status. The 'none' projection case is D13.
func detectD8(s Snapshot) []Finding {
	if s.Escrow == nil || s.Order == nil {
		return nil
	}
	if s.Order.EscrowStatus == OrderEscrowStatusNone {
		return nil
	}
	if s.Escrow.Status == s.Order.EscrowStatus {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD8EscrowStateMismatch, SeverityHigh,
			"investigate the HandleGatewayRefundAck projection sync; consider /admin/orders/{order_id}/repair-projection (projection only)",
			fmt.Sprintf("escrows.status=%s vs orders.escrow_status=%s", s.Escrow.Status, s.Order.EscrowStatus),
			0, 0, nil),
	}
}

// D9: cumulative successful refund equals or exceeds order gross, but the
// coins.refund_required outbox event for this order is absent or dead.
func detectD9(s Snapshot) []Finding {
	if s.Order == nil {
		return nil
	}
	localSuccessTotal := sumLocalSuccessfulRefunds(s.Refunds)
	if localSuccessTotal < s.Order.GrossAmount || localSuccessTotal == 0 {
		return nil
	}
	if s.Outbox.CoinsRefundRequiredAliveByOrderID[s.Order.ID] {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD9RefundFullCoinsNotRefunded, SeverityHigh,
			"replay coins.refund_required emission via operator endpoint",
			"order fully refunded but coins.refund_required outbox is absent or dead",
			0, localSuccessTotal, nil),
	}
}

// D10: order.status='completed' but the release ledger entry is absent.
func detectD10(s Snapshot) []Finding {
	if s.Order == nil {
		return nil
	}
	if s.Order.Status != OrderStatusCompleted {
		return nil
	}
	if s.Ledger.OrderReleaseExists {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD10OrderCompletedReleaseAbsent, SeverityCritical,
			"investigate OrderCompletionService.Complete path; canonical replay only via operator",
			"order.status=completed without order_release_<order_id> ledger entry",
			0, s.Order.GrossAmount, nil),
	}
}

// D11: a refund pinned at gateway_status='pending' has aged past the stuck
// grace window with no acknowledgement.
func detectD11(s Snapshot) []Finding {
	if !s.Gateway.Available {
		return nil
	}
	out := make([]Finding, 0)
	for i := range s.Refunds {
		r := s.Refunds[i]
		if r.GatewayStatus != GatewayRefundStatusPending {
			continue
		}
		if r.GatewayAcknowledgedAt != nil {
			continue
		}
		if r.GatewayRequestedAt == nil {
			continue
		}
		if r.GatewayRequestedAt.Add(s.Thresholds.StuckRefundGrace).After(s.Now) {
			continue
		}
		// Owner-locked: a refund with no gateway identifiers is not yet
		// dispatch-ready, not stuck. Suppress D11 until at least one of
		// gateway_idempotency_key / gateway_refund_id is populated.
		if r.GatewayIdempotencyKey == "" && r.GatewayRefundID == "" {
			continue
		}
		gatewayHit := findGatewayRefundEntry(s.Gateway.RefundChargebackHistory, r)
		notes := "refund stuck at gateway_status=pending past grace window; gateway entry not located"
		var gwAmount int64
		if gatewayHit != nil {
			gwAmount = gatewayHit.Amount
			notes = fmt.Sprintf("refund stuck pending; gateway entry status=%s amount=%d", gatewayHit.Status, gatewayHit.Amount)
		}
		refundID := r.ID
		f := buildFinding(s, DriftD11StuckPendingRefund, SeverityHigh,
			"if gateway entry is success replay HandleGatewayRefundAck; if failed mark refund failed via operator",
			notes, gwAmount, r.RequestedAmount, &refundID)
		out = append(out, f)
	}
	return out
}

// D12: payment past its expiry window with no live webhook event covering the
// transition. Local timeout worker should have flipped it.
func detectD12(s Snapshot) []Finding {
	if s.Payment == nil {
		return nil
	}
	if s.Payment.Status != LocalPaymentStatusPending {
		return nil
	}
	if s.Payment.ExpiredAt.Add(s.Thresholds.PendingPaymentExpiryGrace).After(s.Now) {
		return nil
	}
	if hasNonFailedWebhook(s.Webhooks) {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD12PendingPaymentPastExpiry, SeverityMedium,
			"investigate payment-expiry worker; manual cancel via operator if confirmed expired at gateway",
			fmt.Sprintf("payment.expired_at=%s, now=%s, no live webhook delivered", s.Payment.ExpiredAt.UTC().Format(time.RFC3339), s.Now.UTC().Format(time.RFC3339)),
			0, s.Payment.GrossAmount, nil),
	}
}

// D13: orders.escrow_status='none' projection while an escrow row exists for
// the same order.
func detectD13(s Snapshot) []Finding {
	if s.Escrow == nil || s.Order == nil {
		return nil
	}
	if s.Order.EscrowStatus != OrderEscrowStatusNone {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD13ProjectionNoneEscrowExists, SeverityHigh,
			"re-run projection sync via /admin/orders/{order_id}/repair-projection (projection column only)",
			fmt.Sprintf("escrow exists with status=%s but orders.escrow_status='none'", s.Escrow.Status),
			0, s.Escrow.Amount, nil),
	}
}

// D14: a canonical ledger entry exists but the required outbox observability
// event is absent or dead-letter. Two sub-cases:
//  1. order release ledger booked but money.released outbox missing.
//  2. refund reversal ledger booked but money.refund_succeeded outbox
//     missing (one finding per affected refund).
func detectD14(s Snapshot) []Finding {
	out := make([]Finding, 0, 2)

	if s.Order != nil && s.Ledger.OrderReleaseExists &&
		!s.Outbox.MoneyReleasedAliveByOrderID[s.Order.ID] {
		out = append(out, buildFinding(s, DriftD14LedgerEntryOutboxMissing, SeverityHigh,
			"replay money.released outbox emission via operator endpoint",
			"order_release_<order_id> exists but money.released outbox is absent or dead-letter",
			0, s.Ledger.OrderReleaseAmount, nil))
	}

	refundIDs := sortedRefundIDsWithReversal(s.Ledger.RefundReversalExistsByRefundID)
	for _, rid := range refundIDs {
		if s.Outbox.MoneyRefundSucceededAliveByRefundID[rid] {
			continue
		}
		refundID := rid
		out = append(out, buildFinding(s, DriftD14LedgerEntryOutboxMissing, SeverityHigh,
			"replay money.refund_succeeded outbox emission via operator endpoint",
			"refund_reversal_<refund_id> exists but money.refund_succeeded outbox is absent or dead-letter",
			0, 0, &refundID))
	}
	return out
}

// D15: order has downstream money state (an escrow row exists, OR
// orders.total_before_coins_amount > 0 (canonical buyer-funded escrow base),
// OR orders.escrow_status is not 'none') but the canonical payment authority
// — the payments row — is missing. Every other D-class gates on s.Payment !=
// nil and would silently ignore this state; D15 closes that blind spot.
func detectD15(s Snapshot) []Finding {
	if s.Order == nil {
		return nil
	}
	if s.Payment != nil {
		return nil
	}
	escrowRowPresent := s.Escrow != nil
	nonZeroEscrowAmount := s.Order.GrossAmount > 0
	nonNoneProjection := s.Order.EscrowStatus != "" &&
		s.Order.EscrowStatus != OrderEscrowStatusNone
	if !escrowRowPresent && !nonZeroEscrowAmount && !nonNoneProjection {
		return nil
	}
	return []Finding{
		buildFinding(s, DriftD15EscrowPresentPaymentAbsent, SeverityHigh,
			"investigate order creation path; payment row should be co-created with the order",
			fmt.Sprintf("payment row absent but escrow_row_present=%v escrow_amount=%d escrow_status=%s",
				escrowRowPresent, s.Order.GrossAmount, s.Order.EscrowStatus),
			0, s.Order.GrossAmount, nil),
	}
}

// ---------------------------------------------------------------------------
// Helpers — all pure
// ---------------------------------------------------------------------------

func gatewayIsSettled(status string) bool {
	return status == GatewayStatusSettlement || status == GatewayStatusCapture
}

func gatewayIsTerminalFailure(status string) bool {
	return status == GatewayStatusExpire || status == GatewayStatusDeny || status == GatewayStatusCancel
}

func gatewayIsTerminal(status string) bool {
	return gatewayIsSettled(status) || gatewayIsTerminalFailure(status)
}

func isLocalPaymentSettled(status string) bool {
	return status == LocalPaymentStatusPaid ||
		status == LocalPaymentStatusSettlement ||
		status == LocalPaymentStatusCapture
}

func sumGatewaySuccessfulRefunds(entries []GatewayRefundEntry) int64 {
	var total int64
	for _, e := range entries {
		if e.Status == GatewayRefundHistorySuccess {
			total += e.Amount
		}
	}
	return total
}

func sumLocalSuccessfulRefunds(refunds []RefundRow) int64 {
	var total int64
	for _, r := range refunds {
		if r.GatewayStatus == GatewayRefundStatusSucceeded {
			total += r.RequestedAmount
		}
	}
	return total
}

func findGatewayRefundEntry(entries []GatewayRefundEntry, r RefundRow) *GatewayRefundEntry {
	if r.GatewayIdempotencyKey == "" && r.GatewayRefundID == "" {
		return nil
	}
	for i := range entries {
		e := entries[i]
		if r.GatewayIdempotencyKey != "" && e.RefundKey == r.GatewayIdempotencyKey {
			return &entries[i]
		}
		if r.GatewayRefundID != "" && e.RefundID == r.GatewayRefundID {
			return &entries[i]
		}
	}
	return nil
}

func orphanRecoveryInFlight(s Snapshot) bool {
	grace := s.Thresholds.OrphanRecoveryGrace
	if grace <= 0 {
		return false
	}
	for _, w := range s.Webhooks {
		if w.Status != WebhookStatusOrphaned {
			continue
		}
		if w.ReceivedAt.Add(grace).After(s.Now) {
			return true
		}
	}
	return false
}

func hasNonFailedWebhook(webhooks []WebhookEventRef) bool {
	for _, w := range webhooks {
		if w.Status == WebhookStatusSucceeded ||
			w.Status == WebhookStatusProcessing ||
			w.Status == WebhookStatusPending ||
			w.Status == WebhookStatusManualReview ||
			w.Status == WebhookStatusQuarantined ||
			w.Status == WebhookStatusTerminalReview {
			return true
		}
	}
	return false
}

func hasSucceededWebhook(webhooks []WebhookEventRef) bool {
	for _, w := range webhooks {
		if w.Status == WebhookStatusSucceeded {
			return true
		}
	}
	return false
}

func sortedRefundIDsWithReversal(m map[uuid.UUID]bool) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// buildFinding centralises Finding construction so idempotency key formation
// and detected_at stamping stay deterministic and consistent.
func buildFinding(
	s Snapshot,
	class DriftClass,
	severity Severity,
	suggested string,
	notes string,
	gatewayAmount int64,
	localAmount int64,
	refundID *uuid.UUID,
) Finding {
	var orderID *uuid.UUID
	if s.Order != nil {
		oid := s.Order.ID
		orderID = &oid
	}
	mtID := s.Gateway.MidtransOrderID
	if mtID == "" && s.Payment != nil {
		mtID = s.Payment.MidtransOrderID
	}
	return Finding{
		DriftClass:            class,
		Severity:              severity,
		OrderID:               orderID,
		MidtransOrderID:       mtID,
		RefundID:              refundID,
		SuggestedAction:       suggested,
		DetectedAt:            s.Now,
		IdempotencyKey:        buildIdempotencyKey(class, s.Now, orderID, refundID, mtID),
		GatewayObservedAmount: gatewayAmount,
		LocalObservedAmount:   localAmount,
		Notes:                 notes,
	}
}

// buildIdempotencyKey produces a deterministic, daily-bucketed key. Refund-
// scoped findings include the refund UUID; otherwise order UUID is used; if
// no local order yet the gateway's midtrans_order_id anchors the key.
//
// Format: `recon|{class}|{scope}={id}|d={YYYYMMDD}`
func buildIdempotencyKey(class DriftClass, now time.Time, orderID, refundID *uuid.UUID, midtransOrderID string) string {
	bucket := now.UTC().Format("20060102")
	switch {
	case refundID != nil && orderID != nil:
		return fmt.Sprintf("recon|%s|order=%s|refund=%s|d=%s", class, orderID.String(), refundID.String(), bucket)
	case refundID != nil:
		return fmt.Sprintf("recon|%s|refund=%s|d=%s", class, refundID.String(), bucket)
	case orderID != nil:
		return fmt.Sprintf("recon|%s|order=%s|d=%s", class, orderID.String(), bucket)
	default:
		return fmt.Sprintf("recon|%s|mt=%s|d=%s", class, midtransOrderID, bucket)
	}
}


