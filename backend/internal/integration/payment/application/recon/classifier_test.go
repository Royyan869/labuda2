package recon

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

var (
	fixedNow      = time.Date(2026, time.May, 11, 12, 0, 0, 0, time.UTC)
	orderUUID     = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	paymentUUID   = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userUUID      = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	escrowUUID    = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	refundUUID    = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	refundUUIDAlt = uuid.MustParse("66666666-6666-6666-6666-666666666666")
)

const (
	midtransOID    = "ORDER-LABUDA-ABC-123"
	gatewayRefKey  = "system_refund_dispute_11111111-1111-1111-1111-111111111111"
	gatewayRefID   = "REFUND-MT-ABC-456"
	gatewayRefIDB  = "REFUND-MT-DEF-789"
	gatewayRefKeyB = "system_refund_partial_dispute_11111111-1111-1111-1111-111111111111"
)

func defaultThresholds() Thresholds {
	return Thresholds{
		PendingPaymentGrace:       3 * time.Minute,
		OrphanRecoveryGrace:       2 * time.Minute,
		StuckRefundGrace:          5 * time.Minute,
		PendingPaymentExpiryGrace: 1 * time.Minute,
	}
}

// cleanSnapshot returns a fully converged, drift-free snapshot for a paid,
// completed, fully-released order. Each test mutates a copy of this baseline
// to fabricate exactly one targeted drift signature.
func cleanSnapshot() Snapshot {
	paidAt := fixedNow.Add(-2 * time.Hour)
	releasedAt := fixedNow.Add(-30 * time.Minute)
	return Snapshot{
		Now: fixedNow,
		Gateway: GatewaySnapshot{
			MidtransOrderID:   midtransOID,
			Available:         true,
			TransactionStatus: GatewayStatusSettlement,
			TransactionID:     "MT-TX-001",
			GrossAmount:       100_000,
			TransactionTime:   fixedNow.Add(-2 * time.Hour),
			SettlementTime:    ptrTime(paidAt),
			QueriedAt:         fixedNow,
		},
		Payment: &PaymentRow{
			ID:              paymentUUID,
			UserID:          userUUID,
			MidtransOrderID: midtransOID,
			TransactionID:   "MT-TX-001",
			GrossAmount:     100_000,
			Status:          LocalPaymentStatusPaid,
			ReferenceType:   "order",
			ReferenceID:     orderUUID,
			PaidAt:          ptrTime(paidAt),
			ExpiredAt:       fixedNow.Add(24 * time.Hour),
			CreatedAt:       fixedNow.Add(-3 * time.Hour),
		},
		Order: &OrderRow{
			ID:           orderUUID,
			Status:       OrderStatusCompleted,
			EscrowStatus: OrderEscrowStatusReleased,
			GrossAmount:  100_000,
			HasDispute:   false,
			CreatedAt:    fixedNow.Add(-3 * time.Hour),
			UpdatedAt:    fixedNow.Add(-30 * time.Minute),
		},
		Escrow: &EscrowRow{
			ID:         escrowUUID,
			OrderID:    orderUUID,
			Status:     EscrowStatusReleased,
			Amount:     100_000,
			CreatedAt:  fixedNow.Add(-2 * time.Hour),
			ReleasedAt: ptrTime(releasedAt),
		},
		Webhooks: []WebhookEventRef{
			{
				EventID:           "WH-001",
				MidtransOrderID:   midtransOID,
				Status:            WebhookStatusSucceeded,
				TransactionStatus: GatewayStatusSettlement,
				TransactionID:     "MT-TX-001",
				ReceivedAt:        fixedNow.Add(-2 * time.Hour),
				ProcessedAt:       ptrTime(paidAt),
			},
		},
		Ledger: LedgerLookup{
			BuyerSettlementExists:          true,
			OrderReleaseExists:             true,
			OrderReleaseAmount:             100_000,
			RefundReversalExistsByRefundID: map[uuid.UUID]bool{},
		},
		Outbox: OutboxLookup{
			MoneyReleasedAliveByOrderID:         map[uuid.UUID]bool{orderUUID: true},
			MoneyRefundSucceededAliveByRefundID: map[uuid.UUID]bool{},
			CoinsRefundRequiredAliveByOrderID:   map[uuid.UUID]bool{},
		},
		Thresholds: defaultThresholds(),
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

// ---------------------------------------------------------------------------
// Table-driven matrix
// ---------------------------------------------------------------------------

type caseSpec struct {
	name      string
	mutate    func(*Snapshot)
	expectAll []DriftClass // set equality (order-independent)
}

func TestClassify_Matrix(t *testing.T) {
	tests := []caseSpec{
		{
			name:      "no-drift_clean_baseline",
			mutate:    func(_ *Snapshot) {},
			expectAll: nil,
		},

		// -------- D1 --------
		{
			name: "D1_gateway_settled_local_pending_past_grace_no_orphan",
			mutate: func(s *Snapshot) {
				s.Payment.Status = LocalPaymentStatusPending
				s.Payment.PaidAt = nil
				s.Payment.CreatedAt = fixedNow.Add(-10 * time.Minute)
				s.Webhooks = nil
				s.Ledger.BuyerSettlementExists = false
				s.Ledger.OrderReleaseExists = false
				s.Order.Status = OrderStatusPendingPayment
				s.Order.EscrowStatus = OrderEscrowStatusNone
				s.Escrow = nil
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			},
			// D6 fires too because gateway terminal + no webhook.
			expectAll: []DriftClass{DriftD1GatewaySettledLocalUnpaid, DriftD6MissingWebhookDelivery},
		},
		{
			name: "D1_suppressed_within_grace",
			mutate: func(s *Snapshot) {
				s.Payment.Status = LocalPaymentStatusPending
				s.Payment.PaidAt = nil
				s.Payment.CreatedAt = fixedNow.Add(-1 * time.Minute) // inside 3min grace
				s.Webhooks = nil
				s.Ledger.BuyerSettlementExists = false
				s.Ledger.OrderReleaseExists = false
				s.Order.Status = OrderStatusPendingPayment
				s.Order.EscrowStatus = OrderEscrowStatusNone
				s.Escrow = nil
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			},
			// Inside grace: D1 suppressed. D6 still fires (gateway terminal + no webhook).
			expectAll: []DriftClass{DriftD6MissingWebhookDelivery},
		},
		{
			name: "D1_suppressed_by_orphan_recovery_in_flight",
			mutate: func(s *Snapshot) {
				s.Payment.Status = LocalPaymentStatusPending
				s.Payment.PaidAt = nil
				s.Payment.CreatedAt = fixedNow.Add(-10 * time.Minute)
				s.Webhooks = []WebhookEventRef{
					{
						EventID:         "WH-orphan",
						MidtransOrderID: midtransOID,
						Status:          WebhookStatusOrphaned,
						ReceivedAt:      fixedNow.Add(-30 * time.Second), // within 2min grace
					},
				}
				s.Ledger.BuyerSettlementExists = false
				s.Ledger.OrderReleaseExists = false
				s.Order.Status = OrderStatusPendingPayment
				s.Order.EscrowStatus = OrderEscrowStatusNone
				s.Escrow = nil
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			},
			// Both D1 and D6 suppressed by orphan recovery grace.
			expectAll: nil,
		},

		// -------- D2 --------
		{
			name: "D2_local_paid_gateway_expired",
			mutate: func(s *Snapshot) {
				s.Gateway.TransactionStatus = GatewayStatusExpire
			},
			expectAll: []DriftClass{DriftD2LocalPaidGatewayTerminalFailure},
		},
		{
			name: "D2_local_paid_gateway_deny",
			mutate: func(s *Snapshot) {
				s.Gateway.TransactionStatus = GatewayStatusDeny
			},
			expectAll: []DriftClass{DriftD2LocalPaidGatewayTerminalFailure},
		},

		// -------- D3 --------
		{
			name: "D3_gateway_full_refund_local_escrow_holding",
			mutate: func(s *Snapshot) {
				s.Escrow.Status = EscrowStatusHolding
				s.Escrow.ReleasedAt = nil
				s.Order.Status = OrderStatusDelivered
				s.Order.EscrowStatus = OrderEscrowStatusHolding
				s.Ledger.OrderReleaseExists = false
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
				s.Gateway.RefundChargebackHistory = []GatewayRefundEntry{
					{
						RefundKey: gatewayRefKey,
						RefundID:  gatewayRefID,
						Amount:    100_000,
						Status:    GatewayRefundHistorySuccess,
						CreatedAt: fixedNow.Add(-10 * time.Minute),
					},
				}
			},
			// D4 also fires because local has no successful refund row.
			expectAll: []DriftClass{DriftD3GatewayRefundedLocalHolding, DriftD4PartialRefundMismatch},
		},

		// -------- D4 --------
		{
			name: "D4_partial_refund_mismatch_gateway_more",
			mutate: func(s *Snapshot) {
				s.Gateway.RefundChargebackHistory = []GatewayRefundEntry{
					{
						RefundKey: gatewayRefKey,
						RefundID:  gatewayRefID,
						Amount:    40_000,
						Status:    GatewayRefundHistorySuccess,
						CreatedAt: fixedNow.Add(-1 * time.Minute),
					},
				}
				s.Refunds = []RefundRow{
					{
						ID:                    refundUUID,
						OrderID:               orderUUID,
						RequestedAmount:       40_000,
						Status:                "admin_refunded",
						GatewayStatus:         GatewayRefundStatusPending,
						GatewayRefundID:       gatewayRefID,
						GatewayIdempotencyKey: gatewayRefKey,
						GatewayRequestedAt:    ptrTime(fixedNow.Add(-1 * time.Minute)), // within 5min grace → D11 suppressed
					},
				}
			},
			expectAll: []DriftClass{DriftD4PartialRefundMismatch},
		},
		{
			name: "D4_partial_refund_mismatch_local_more",
			mutate: func(s *Snapshot) {
				s.Refunds = []RefundRow{
					{
						ID:                    refundUUID,
						OrderID:               orderUUID,
						RequestedAmount:       30_000,
						Status:                "admin_refunded",
						GatewayStatus:         GatewayRefundStatusSucceeded,
						GatewayRefundID:       gatewayRefID,
						GatewayIdempotencyKey: gatewayRefKey,
						GatewayRequestedAt:    ptrTime(fixedNow.Add(-10 * time.Minute)),
						GatewayAcknowledgedAt: ptrTime(fixedNow.Add(-8 * time.Minute)),
					},
				}
			},
			expectAll: []DriftClass{DriftD4PartialRefundMismatch},
		},

		// -------- D5 --------
		{
			// Two distinct gateway transactions both attesting settlement
			// for the same order → real duplicate-settlement signature.
			name: "D5_duplicate_settlement_two_distinct_tx_ids",
			mutate: func(s *Snapshot) {
				s.Webhooks = []WebhookEventRef{
					{
						EventID:           "WH-001",
						MidtransOrderID:   midtransOID,
						Status:            WebhookStatusSucceeded,
						TransactionStatus: GatewayStatusSettlement,
						TransactionID:     "MT-TX-001",
						ReceivedAt:        fixedNow.Add(-2 * time.Hour),
					},
					{
						EventID:           "WH-002",
						MidtransOrderID:   midtransOID,
						Status:            WebhookStatusSucceeded,
						TransactionStatus: GatewayStatusSettlement,
						TransactionID:     "MT-TX-002",
						ReceivedAt:        fixedNow.Add(-1 * time.Hour),
					},
				}
			},
			expectAll: []DriftClass{DriftD5DuplicateSettlement},
		},
		{
			// Legitimate Midtrans retry: same transaction_id delivered
			// twice. Must NOT trigger D5.
			name: "D5_suppressed_same_transaction_id_retry",
			mutate: func(s *Snapshot) {
				s.Webhooks = []WebhookEventRef{
					{
						EventID:           "WH-001",
						MidtransOrderID:   midtransOID,
						Status:            WebhookStatusSucceeded,
						TransactionStatus: GatewayStatusSettlement,
						TransactionID:     "MT-TX-001",
						ReceivedAt:        fixedNow.Add(-2 * time.Hour),
					},
					{
						EventID:           "WH-002",
						MidtransOrderID:   midtransOID,
						Status:            WebhookStatusSucceeded,
						TransactionStatus: GatewayStatusSettlement,
						TransactionID:     "MT-TX-001",
						ReceivedAt:        fixedNow.Add(-1 * time.Hour),
					},
				}
			},
			expectAll: nil,
		},
		{
			// Lifecycle progression pending → settlement → settlement.
			// Only the settled rows count, and they share a transaction_id.
			// Must NOT trigger D5.
			name: "D5_suppressed_lifecycle_progression",
			mutate: func(s *Snapshot) {
				s.Webhooks = []WebhookEventRef{
					{
						EventID:           "WH-PENDING",
						MidtransOrderID:   midtransOID,
						Status:            WebhookStatusSucceeded,
						TransactionStatus: GatewayStatusPending,
						TransactionID:     "MT-TX-001",
						ReceivedAt:        fixedNow.Add(-3 * time.Hour),
					},
					{
						EventID:           "WH-SETTLE-1",
						MidtransOrderID:   midtransOID,
						Status:            WebhookStatusSucceeded,
						TransactionStatus: GatewayStatusSettlement,
						TransactionID:     "MT-TX-001",
						ReceivedAt:        fixedNow.Add(-2 * time.Hour),
					},
					{
						EventID:           "WH-SETTLE-2",
						MidtransOrderID:   midtransOID,
						Status:            WebhookStatusSucceeded,
						TransactionStatus: GatewayStatusSettlement,
						TransactionID:     "MT-TX-001",
						ReceivedAt:        fixedNow.Add(-1 * time.Hour),
					},
				}
			},
			expectAll: nil,
		},

		// -------- D6 --------
		{
			name: "D6_no_webhook_gateway_terminal",
			mutate: func(s *Snapshot) {
				s.Webhooks = nil
			},
			expectAll: []DriftClass{DriftD6MissingWebhookDelivery},
		},
		{
			name: "D6_suppressed_orphan_within_grace",
			mutate: func(s *Snapshot) {
				s.Webhooks = []WebhookEventRef{
					{
						EventID:         "WH-orphan",
						MidtransOrderID: midtransOID,
						Status:          WebhookStatusOrphaned,
						ReceivedAt:      fixedNow.Add(-30 * time.Second),
					},
				}
			},
			expectAll: nil,
		},
		{
			name: "D6_fires_orphan_past_grace",
			mutate: func(s *Snapshot) {
				s.Webhooks = []WebhookEventRef{
					{
						EventID:         "WH-orphan",
						MidtransOrderID: midtransOID,
						Status:          WebhookStatusOrphaned,
						ReceivedAt:      fixedNow.Add(-10 * time.Minute), // beyond 2min grace
					},
				}
			},
			expectAll: []DriftClass{DriftD6MissingWebhookDelivery},
		},

		// -------- D7 --------
		{
			name: "D7_webhook_succeeded_settlement_ledger_missing",
			mutate: func(s *Snapshot) {
				s.Ledger.BuyerSettlementExists = false
			},
			expectAll: []DriftClass{DriftD7WebhookProcessedLedgerAbsent},
		},

		// -------- D8 --------
		{
			name: "D8_escrow_released_projection_holding",
			mutate: func(s *Snapshot) {
				s.Order.EscrowStatus = OrderEscrowStatusHolding
			},
			expectAll: []DriftClass{DriftD8EscrowStateMismatch},
		},
		{
			name: "D8_escrow_refunded_projection_released",
			mutate: func(s *Snapshot) {
				s.Escrow.Status = EscrowStatusRefunded
				s.Escrow.RefundedAt = ptrTime(fixedNow.Add(-30 * time.Minute))
				s.Escrow.ReleasedAt = nil
				// Projection still says released → mismatch.
			},
			expectAll: []DriftClass{DriftD8EscrowStateMismatch},
		},

		// -------- D9 --------
		{
			name: "D9_full_refund_no_coins_refund_event",
			mutate: func(s *Snapshot) {
				s.Escrow.Status = EscrowStatusRefunded
				s.Escrow.ReleasedAt = nil
				s.Escrow.RefundedAt = ptrTime(fixedNow.Add(-30 * time.Minute))
				s.Order.Status = OrderStatusRefunded
				s.Order.EscrowStatus = OrderEscrowStatusRefunded
				s.Ledger.OrderReleaseExists = false
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
				s.Refunds = []RefundRow{
					{
						ID:                    refundUUID,
						OrderID:               orderUUID,
						RequestedAmount:       100_000,
						Status:                "admin_refunded",
						GatewayStatus:         GatewayRefundStatusSucceeded,
						GatewayRefundID:       gatewayRefID,
						GatewayIdempotencyKey: gatewayRefKey,
						GatewayRequestedAt:    ptrTime(fixedNow.Add(-1 * time.Hour)),
						GatewayAcknowledgedAt: ptrTime(fixedNow.Add(-50 * time.Minute)),
					},
				}
				s.Gateway.RefundChargebackHistory = []GatewayRefundEntry{
					{
						RefundKey: gatewayRefKey,
						RefundID:  gatewayRefID,
						Amount:    100_000,
						Status:    GatewayRefundHistorySuccess,
						CreatedAt: fixedNow.Add(-1 * time.Hour),
					},
				}
				s.Ledger.RefundReversalExistsByRefundID = map[uuid.UUID]bool{refundUUID: true}
				s.Outbox.MoneyRefundSucceededAliveByRefundID = map[uuid.UUID]bool{refundUUID: true}
				s.Outbox.CoinsRefundRequiredAliveByOrderID = map[uuid.UUID]bool{} // missing
			},
			expectAll: []DriftClass{DriftD9RefundFullCoinsNotRefunded},
		},

		// -------- D10 --------
		{
			name: "D10_order_completed_release_ledger_absent",
			mutate: func(s *Snapshot) {
				s.Ledger.OrderReleaseExists = false
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{} // outbox also missing
			},
			// D14 also fires for the missing money.released outbox? No: D14 requires
			// LedgerLookup.OrderReleaseExists=true. If both are missing, only D10 fires.
			expectAll: []DriftClass{DriftD10OrderCompletedReleaseAbsent},
		},

		// -------- D11 --------
		{
			name: "D11_stuck_pending_refund_past_grace",
			mutate: func(s *Snapshot) {
				s.Refunds = []RefundRow{
					{
						ID:                    refundUUID,
						OrderID:               orderUUID,
						RequestedAmount:       40_000,
						Status:                "admin_refunded",
						GatewayStatus:         GatewayRefundStatusPending,
						GatewayRefundID:       gatewayRefID,
						GatewayIdempotencyKey: gatewayRefKey,
						GatewayRequestedAt:    ptrTime(fixedNow.Add(-10 * time.Minute)),
					},
				}
				s.Gateway.RefundChargebackHistory = []GatewayRefundEntry{
					{
						RefundKey: gatewayRefKey,
						RefundID:  gatewayRefID,
						Amount:    40_000,
						Status:    GatewayRefundHistorySuccess, // gateway already succeeded; local ack lost
						CreatedAt: fixedNow.Add(-9 * time.Minute),
					},
				}
			},
			// D4 also fires (gateway has 40k successful, local has 0 successful).
			expectAll: []DriftClass{DriftD4PartialRefundMismatch, DriftD11StuckPendingRefund},
		},
		{
			name: "D11_suppressed_within_grace",
			mutate: func(s *Snapshot) {
				s.Refunds = []RefundRow{
					{
						ID:                    refundUUID,
						OrderID:               orderUUID,
						RequestedAmount:       40_000,
						Status:                "admin_refunded",
						GatewayStatus:         GatewayRefundStatusPending,
						GatewayRefundID:       gatewayRefID,
						GatewayIdempotencyKey: gatewayRefKey,
						GatewayRequestedAt:    ptrTime(fixedNow.Add(-1 * time.Minute)), // inside 5min grace
					},
				}
			},
			expectAll: nil,
		},

		// -------- D12 --------
		{
			name: "D12_pending_payment_past_expiry_no_webhook",
			mutate: func(s *Snapshot) {
				s.Payment.Status = LocalPaymentStatusPending
				s.Payment.PaidAt = nil
				s.Payment.ExpiredAt = fixedNow.Add(-10 * time.Minute)
				s.Webhooks = nil
				s.Gateway.TransactionStatus = GatewayStatusPending
				s.Gateway.SettlementTime = nil
				s.Ledger.BuyerSettlementExists = false
				s.Ledger.OrderReleaseExists = false
				s.Order.Status = OrderStatusPendingPayment
				s.Order.EscrowStatus = OrderEscrowStatusNone
				s.Escrow = nil
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			},
			expectAll: []DriftClass{DriftD12PendingPaymentPastExpiry},
		},

		// -------- D13 --------
		{
			name: "D13_orders_escrow_none_but_escrow_row_exists",
			mutate: func(s *Snapshot) {
				s.Order.EscrowStatus = OrderEscrowStatusNone
			},
			expectAll: []DriftClass{DriftD13ProjectionNoneEscrowExists},
		},

		// -------- D14 --------
		{
			name: "D14_order_release_ledger_present_outbox_dead",
			mutate: func(s *Snapshot) {
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{} // dead/missing
			},
			expectAll: []DriftClass{DriftD14LedgerEntryOutboxMissing},
		},
		{
			name: "D14_refund_reversal_present_outbox_missing",
			mutate: func(s *Snapshot) {
				s.Ledger.RefundReversalExistsByRefundID = map[uuid.UUID]bool{refundUUID: true}
				s.Outbox.MoneyRefundSucceededAliveByRefundID = map[uuid.UUID]bool{} // missing
			},
			expectAll: []DriftClass{DriftD14LedgerEntryOutboxMissing},
		},
		{
			name: "D14_both_sub_cases_simultaneous",
			mutate: func(s *Snapshot) {
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
				s.Ledger.RefundReversalExistsByRefundID = map[uuid.UUID]bool{
					refundUUID:    true,
					refundUUIDAlt: true,
				}
				s.Outbox.MoneyRefundSucceededAliveByRefundID = map[uuid.UUID]bool{
					refundUUIDAlt: true, // only this one is alive
				}
			},
			// 2 D14 findings (release missing + refundUUID's reversal outbox missing).
			expectAll: []DriftClass{DriftD14LedgerEntryOutboxMissing, DriftD14LedgerEntryOutboxMissing},
		},

		// -------- Cross-firing / conflicting state --------
		{
			name: "conflicting_D8_and_D9_full_refund_path_with_stale_projection",
			mutate: func(s *Snapshot) {
				s.Escrow.Status = EscrowStatusRefunded
				s.Escrow.RefundedAt = ptrTime(fixedNow.Add(-30 * time.Minute))
				s.Escrow.ReleasedAt = nil
				s.Order.Status = OrderStatusRefunded
				s.Order.EscrowStatus = OrderEscrowStatusReleased // stale projection
				s.Ledger.OrderReleaseExists = false
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
				s.Refunds = []RefundRow{
					{
						ID:                    refundUUID,
						OrderID:               orderUUID,
						RequestedAmount:       100_000,
						Status:                "admin_refunded",
						GatewayStatus:         GatewayRefundStatusSucceeded,
						GatewayRefundID:       gatewayRefID,
						GatewayIdempotencyKey: gatewayRefKey,
						GatewayRequestedAt:    ptrTime(fixedNow.Add(-1 * time.Hour)),
						GatewayAcknowledgedAt: ptrTime(fixedNow.Add(-50 * time.Minute)),
					},
				}
				s.Gateway.RefundChargebackHistory = []GatewayRefundEntry{
					{
						RefundKey: gatewayRefKey,
						RefundID:  gatewayRefID,
						Amount:    100_000,
						Status:    GatewayRefundHistorySuccess,
						CreatedAt: fixedNow.Add(-1 * time.Hour),
					},
				}
				s.Ledger.RefundReversalExistsByRefundID = map[uuid.UUID]bool{refundUUID: true}
				s.Outbox.MoneyRefundSucceededAliveByRefundID = map[uuid.UUID]bool{refundUUID: true}
			},
			expectAll: []DriftClass{
				DriftD8EscrowStateMismatch,
				DriftD9RefundFullCoinsNotRefunded,
			},
		},
		{
			name: "conflicting_D1_and_D6_and_D12_all_fire_for_lost_settlement",
			mutate: func(s *Snapshot) {
				// Pending payment, past expiry, no webhook, gateway says settled.
				// (Phantom case — gateway settled but payment never marked paid AND
				// past expiry; tests that all three drift classes co-fire.)
				s.Payment.Status = LocalPaymentStatusPending
				s.Payment.PaidAt = nil
				s.Payment.CreatedAt = fixedNow.Add(-1 * time.Hour)
				s.Payment.ExpiredAt = fixedNow.Add(-10 * time.Minute)
				s.Webhooks = nil
				s.Order.Status = OrderStatusPendingPayment
				s.Order.EscrowStatus = OrderEscrowStatusNone
				s.Escrow = nil
				s.Ledger.BuyerSettlementExists = false
				s.Ledger.OrderReleaseExists = false
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			},
			expectAll: []DriftClass{
				DriftD1GatewaySettledLocalUnpaid,
				DriftD6MissingWebhookDelivery,
				DriftD12PendingPaymentPastExpiry,
			},
		},

		// -------- Gateway unavailable --------
		{
			name: "gateway_unavailable_suppresses_gateway_required_classes",
			mutate: func(s *Snapshot) {
				s.Gateway.Available = false
				s.Gateway.TransactionStatus = ""
				s.Gateway.RefundChargebackHistory = nil
				// Force D8 + D10 + D13 + D14 candidates simultaneously; only
				// local-only drift classes should fire.
				s.Escrow.Status = EscrowStatusRefunded
				s.Escrow.RefundedAt = ptrTime(fixedNow.Add(-30 * time.Minute))
				s.Escrow.ReleasedAt = nil
				s.Order.EscrowStatus = OrderEscrowStatusNone
				s.Ledger.OrderReleaseExists = true
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{} // D14
			},
			// D13 fires (projection none + escrow exists). D14 fires (release ledger
			// exists but outbox missing). D8 suppressed because D13 takes precedence
			// for the 'none' case (D8 guards EscrowStatus != none).
			expectAll: []DriftClass{
				DriftD13ProjectionNoneEscrowExists,
				DriftD14LedgerEntryOutboxMissing,
			},
		},

		// -------- D8 vs D13 disjointness --------
		{
			name: "D8_and_D13_are_mutually_exclusive",
			mutate: func(s *Snapshot) {
				s.Escrow.Status = EscrowStatusHolding
				s.Order.EscrowStatus = OrderEscrowStatusNone
			},
			expectAll: []DriftClass{DriftD13ProjectionNoneEscrowExists},
		},

		// -------- D15 --------
		{
			// Escrow row exists for an order with no payment row — the
			// canonical case D15 was introduced to catch. Gateway is
			// unavailable because in reality no payment row means the
			// resolver skips the gateway query (midtrans_order_id is empty).
			name: "D15_escrow_row_present_payment_absent",
			mutate: func(s *Snapshot) {
				s.Payment = nil
				s.Gateway.Available = false
				s.Gateway.TransactionStatus = ""
				s.Order.Status = OrderStatusPendingPayment
				s.Order.EscrowStatus = OrderEscrowStatusHolding
				s.Order.GrossAmount = 125_000
				s.Escrow.Status = EscrowStatusHolding
				s.Escrow.ReleasedAt = nil
				s.Webhooks = nil
				s.Ledger.OrderReleaseExists = false
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			},
			expectAll: []DriftClass{DriftD15EscrowPresentPaymentAbsent},
		},
		{
			// orders.escrow_amount > 0 with no escrow row and no payment row.
			// Same drift signal, surfaced from the projection column alone.
			name: "D15_projection_amount_without_payment",
			mutate: func(s *Snapshot) {
				s.Payment = nil
				s.Escrow = nil
				s.Gateway.Available = false
				s.Gateway.TransactionStatus = ""
				s.Order.Status = OrderStatusPendingPayment
				s.Order.EscrowStatus = OrderEscrowStatusNone
				s.Order.GrossAmount = 125_000
				s.Webhooks = nil
				s.Ledger.OrderReleaseExists = false
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			},
			expectAll: []DriftClass{DriftD15EscrowPresentPaymentAbsent},
		},
		{
			// Order with no payment AND no escrow surface — D15 must NOT
			// fire (this is the legitimate "fresh order, payment not yet
			// created" state).
			name: "D15_suppressed_no_money_surface_yet",
			mutate: func(s *Snapshot) {
				s.Payment = nil
				s.Escrow = nil
				s.Gateway.Available = false
				s.Gateway.TransactionStatus = ""
				s.Order.Status = OrderStatusPendingPayment
				s.Order.EscrowStatus = OrderEscrowStatusNone
				s.Order.GrossAmount = 0
				s.Webhooks = nil
				s.Ledger.OrderReleaseExists = false
				s.Ledger.BuyerSettlementExists = false
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			},
			expectAll: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := cleanSnapshot()
			tc.mutate(&s)

			got := Classify(s)

			gotClasses := extractDriftClasses(got)
			wantClasses := append([]DriftClass(nil), tc.expectAll...)
			sortDriftClasses(gotClasses)
			sortDriftClasses(wantClasses)

			if !equalDriftSlices(gotClasses, wantClasses) {
				t.Fatalf("drift class mismatch:\n  got:  %v\n  want: %v\n  full findings: %+v",
					gotClasses, wantClasses, got)
			}

			for _, f := range got {
				if err := validateFindingShape(f); err != nil {
					t.Errorf("finding shape invalid: %v\n  finding: %+v", err, f)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Replay-safety / idempotency proof
// ---------------------------------------------------------------------------

func TestClassify_DeterministicAcrossInvocations(t *testing.T) {
	specs := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{
			"all_drift_co_firing",
			func(s *Snapshot) {
				s.Payment.Status = LocalPaymentStatusPending
				s.Payment.PaidAt = nil
				s.Payment.CreatedAt = fixedNow.Add(-1 * time.Hour)
				s.Payment.ExpiredAt = fixedNow.Add(-30 * time.Minute)
				s.Webhooks = nil
				s.Order.Status = OrderStatusPendingPayment
				s.Order.EscrowStatus = OrderEscrowStatusNone
				s.Escrow = nil
				s.Ledger.BuyerSettlementExists = false
				s.Ledger.OrderReleaseExists = false
				s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			},
		},
		{
			"D14_two_refunds_unordered_map",
			func(s *Snapshot) {
				s.Ledger.RefundReversalExistsByRefundID = map[uuid.UUID]bool{
					refundUUID:    true,
					refundUUIDAlt: true,
				}
				s.Outbox.MoneyRefundSucceededAliveByRefundID = map[uuid.UUID]bool{}
			},
		},
	}

	for _, sp := range specs {
		sp := sp
		t.Run(sp.name, func(t *testing.T) {
			s := cleanSnapshot()
			sp.mutate(&s)

			first := Classify(s)
			for i := 0; i < 10; i++ {
				again := Classify(s)
				if !reflect.DeepEqual(first, again) {
					t.Fatalf("classify is non-deterministic on iteration %d\n  first: %+v\n  again: %+v", i, first, again)
				}
			}
		})
	}
}

func TestClassify_IdempotencyKeysAreUniquePerFindingDimension(t *testing.T) {
	s := cleanSnapshot()
	// Trigger D14 twice (one per refund) plus the order-release D14.
	s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
	s.Ledger.RefundReversalExistsByRefundID = map[uuid.UUID]bool{
		refundUUID:    true,
		refundUUIDAlt: true,
	}
	s.Outbox.MoneyRefundSucceededAliveByRefundID = map[uuid.UUID]bool{}

	got := Classify(s)
	seen := make(map[string]struct{})
	for _, f := range got {
		if _, dup := seen[f.IdempotencyKey]; dup {
			t.Errorf("duplicate idempotency key produced by classifier: %s", f.IdempotencyKey)
		}
		seen[f.IdempotencyKey] = struct{}{}
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 D14 findings (1 release + 2 refunds), got %d: %+v", len(got), got)
	}
}

func TestClassify_IdempotencyKeyFormat(t *testing.T) {
	s := cleanSnapshot()
	s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{} // trigger D14 order release

	got := Classify(s)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d", len(got))
	}
	want := "recon|" + string(DriftD14LedgerEntryOutboxMissing) + "|order=" + orderUUID.String() + "|d=20260511"
	if got[0].IdempotencyKey != want {
		t.Errorf("idempotency key format drift\n  got:  %s\n  want: %s", got[0].IdempotencyKey, want)
	}
}

func TestClassify_DailyBucketingChangesKey(t *testing.T) {
	s1 := cleanSnapshot()
	s1.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
	s2 := s1
	s2.Now = s1.Now.Add(48 * time.Hour) // +2 days

	f1 := Classify(s1)
	f2 := Classify(s2)
	if len(f1) != 1 || len(f2) != 1 {
		t.Fatalf("expected one finding each; got len(f1)=%d len(f2)=%d", len(f1), len(f2))
	}
	if f1[0].IdempotencyKey == f2[0].IdempotencyKey {
		t.Errorf("expected idempotency keys to differ across day buckets:\n  f1: %s\n  f2: %s",
			f1[0].IdempotencyKey, f2[0].IdempotencyKey)
	}
}

// ---------------------------------------------------------------------------
// Coverage proof — assert every D1..D14 is exercised by at least one positive
// case in the matrix.
// ---------------------------------------------------------------------------

func TestClassify_MatrixCoversAllDriftClasses(t *testing.T) {
	required := []DriftClass{
		DriftD1GatewaySettledLocalUnpaid,
		DriftD2LocalPaidGatewayTerminalFailure,
		DriftD3GatewayRefundedLocalHolding,
		DriftD4PartialRefundMismatch,
		DriftD5DuplicateSettlement,
		DriftD6MissingWebhookDelivery,
		DriftD7WebhookProcessedLedgerAbsent,
		DriftD8EscrowStateMismatch,
		DriftD9RefundFullCoinsNotRefunded,
		DriftD10OrderCompletedReleaseAbsent,
		DriftD11StuckPendingRefund,
		DriftD12PendingPaymentPastExpiry,
		DriftD13ProjectionNoneEscrowExists,
		DriftD14LedgerEntryOutboxMissing,
		DriftD15EscrowPresentPaymentAbsent,
	}

	// Run the entire matrix and collect every drift class observed.
	produced := make(map[DriftClass]bool)
	// Re-declare a compact equivalent to TestClassify_Matrix's spec list.
	// We just iterate and run Classify, accumulating drift classes seen.
	// (We don't reuse the variable from TestClassify_Matrix to keep tests
	// independently runnable.)

	// To avoid duplicating the giant matrix here, we exercise each class
	// directly with the minimal positive trigger pattern proven above.
	mutators := []func(*Snapshot){
		// D1
		func(s *Snapshot) {
			s.Payment.Status = LocalPaymentStatusPending
			s.Payment.PaidAt = nil
			s.Payment.CreatedAt = fixedNow.Add(-10 * time.Minute)
			s.Webhooks = nil
			s.Ledger.BuyerSettlementExists = false
			s.Ledger.OrderReleaseExists = false
			s.Order.Status = OrderStatusPendingPayment
			s.Order.EscrowStatus = OrderEscrowStatusNone
			s.Escrow = nil
			s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
		},
		// D2
		func(s *Snapshot) { s.Gateway.TransactionStatus = GatewayStatusExpire },
		// D3
		func(s *Snapshot) {
			s.Escrow.Status = EscrowStatusHolding
			s.Escrow.ReleasedAt = nil
			s.Order.Status = OrderStatusDelivered
			s.Order.EscrowStatus = OrderEscrowStatusHolding
			s.Ledger.OrderReleaseExists = false
			s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
			s.Gateway.RefundChargebackHistory = []GatewayRefundEntry{
				{RefundKey: gatewayRefKey, RefundID: gatewayRefID, Amount: 100_000, Status: GatewayRefundHistorySuccess, CreatedAt: fixedNow.Add(-10 * time.Minute)},
			}
		},
		// D4
		func(s *Snapshot) {
			s.Gateway.RefundChargebackHistory = []GatewayRefundEntry{
				{RefundKey: gatewayRefKey, RefundID: gatewayRefID, Amount: 40_000, Status: GatewayRefundHistorySuccess, CreatedAt: fixedNow.Add(-10 * time.Minute)},
			}
		},
		// D5 — two settled webhooks with distinct gateway transaction_ids.
		func(s *Snapshot) {
			s.Webhooks = []WebhookEventRef{
				{
					EventID: "WH-001", MidtransOrderID: midtransOID,
					Status:            WebhookStatusSucceeded,
					TransactionStatus: GatewayStatusSettlement, TransactionID: "MT-TX-001",
					ReceivedAt: fixedNow.Add(-2 * time.Hour),
				},
				{
					EventID: "WH-002", MidtransOrderID: midtransOID,
					Status:            WebhookStatusSucceeded,
					TransactionStatus: GatewayStatusSettlement, TransactionID: "MT-TX-002",
					ReceivedAt: fixedNow.Add(-1 * time.Hour),
				},
			}
		},
		// D6
		func(s *Snapshot) { s.Webhooks = nil },
		// D7
		func(s *Snapshot) { s.Ledger.BuyerSettlementExists = false },
		// D8
		func(s *Snapshot) { s.Order.EscrowStatus = OrderEscrowStatusHolding },
		// D9
		func(s *Snapshot) {
			s.Refunds = []RefundRow{{
				ID: refundUUID, OrderID: orderUUID, RequestedAmount: 100_000,
				Status: "admin_refunded", GatewayStatus: GatewayRefundStatusSucceeded,
				GatewayRefundID: gatewayRefID, GatewayIdempotencyKey: gatewayRefKey,
				GatewayRequestedAt:    ptrTime(fixedNow.Add(-1 * time.Hour)),
				GatewayAcknowledgedAt: ptrTime(fixedNow.Add(-50 * time.Minute)),
			}}
			s.Gateway.RefundChargebackHistory = []GatewayRefundEntry{
				{RefundKey: gatewayRefKey, RefundID: gatewayRefID, Amount: 100_000, Status: GatewayRefundHistorySuccess, CreatedAt: fixedNow.Add(-1 * time.Hour)},
			}
		},
		// D10
		func(s *Snapshot) {
			s.Ledger.OrderReleaseExists = false
			s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
		},
		// D11
		func(s *Snapshot) {
			s.Refunds = []RefundRow{{
				ID: refundUUID, OrderID: orderUUID, RequestedAmount: 40_000,
				Status: "admin_refunded", GatewayStatus: GatewayRefundStatusPending,
				GatewayRefundID: gatewayRefID, GatewayIdempotencyKey: gatewayRefKey,
				GatewayRequestedAt: ptrTime(fixedNow.Add(-10 * time.Minute)),
			}}
		},
		// D12
		func(s *Snapshot) {
			s.Payment.Status = LocalPaymentStatusPending
			s.Payment.PaidAt = nil
			s.Payment.ExpiredAt = fixedNow.Add(-10 * time.Minute)
			s.Webhooks = nil
			s.Gateway.TransactionStatus = GatewayStatusPending
			s.Gateway.SettlementTime = nil
			s.Ledger.BuyerSettlementExists = false
			s.Ledger.OrderReleaseExists = false
			s.Order.Status = OrderStatusPendingPayment
			s.Order.EscrowStatus = OrderEscrowStatusNone
			s.Escrow = nil
			s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
		},
		// D13
		func(s *Snapshot) { s.Order.EscrowStatus = OrderEscrowStatusNone },
		// D14
		func(s *Snapshot) { s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{} },
		// D15
		func(s *Snapshot) {
			s.Payment = nil
			s.Gateway.Available = false
			s.Gateway.TransactionStatus = ""
			s.Webhooks = nil
			s.Order.Status = OrderStatusPendingPayment
			s.Order.EscrowStatus = OrderEscrowStatusHolding
			s.Escrow.Status = EscrowStatusHolding
			s.Escrow.ReleasedAt = nil
			s.Ledger.OrderReleaseExists = false
			s.Outbox.MoneyReleasedAliveByOrderID = map[uuid.UUID]bool{}
		},
	}

	for _, m := range mutators {
		s := cleanSnapshot()
		m(&s)
		for _, f := range Classify(s) {
			produced[f.DriftClass] = true
		}
	}

	for _, dc := range required {
		if !produced[dc] {
			t.Errorf("matrix coverage gap: drift class %s is never produced", dc)
		}
	}
}

// ---------------------------------------------------------------------------
// Owner-decision regression tests (post Phase 1A approval, 2026-05-12)
// ---------------------------------------------------------------------------

// D2 must route through SeverityCriticalSecurity, not ordinary critical.
func TestClassify_D2_RoutesAsCriticalSecurity(t *testing.T) {
	s := cleanSnapshot()
	s.Gateway.TransactionStatus = GatewayStatusExpire

	got := Classify(s)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].DriftClass != DriftD2LocalPaidGatewayTerminalFailure {
		t.Fatalf("expected D2, got %s", got[0].DriftClass)
	}
	if got[0].Severity != SeverityCriticalSecurity {
		t.Errorf("D2 severity must be %q, got %q", SeverityCriticalSecurity, got[0].Severity)
	}
	if got[0].Severity == SeverityCritical {
		t.Errorf("D2 severity must NOT equal operational %q — security oncall routing diverges",
			SeverityCritical)
	}
}

// D11 must suppress when the refund has neither gateway_idempotency_key nor
// gateway_refund_id populated — not-yet-dispatch-ready ≠ stuck.
func TestClassify_D11_SuppressedWhenNoGatewayIdentifiers(t *testing.T) {
	s := cleanSnapshot()
	s.Refunds = []RefundRow{
		{
			ID:                    refundUUID,
			OrderID:               orderUUID,
			RequestedAmount:       40_000,
			Status:                "admin_refunded",
			GatewayStatus:         GatewayRefundStatusPending,
			GatewayRefundID:       "", // no identifiers yet
			GatewayIdempotencyKey: "",
			GatewayRequestedAt:    ptrTime(fixedNow.Add(-1 * time.Hour)), // well past 5min grace
		},
	}

	got := Classify(s)
	for _, f := range got {
		if f.DriftClass == DriftD11StuckPendingRefund {
			t.Fatalf("D11 must be suppressed when refund has no gateway identifiers; got %+v", f)
		}
	}
}

// D11 still fires when at least one gateway identifier is populated, even if
// the other is empty — boundary case for the suppression rule.
func TestClassify_D11_FiresWithEitherGatewayIdentifier(t *testing.T) {
	specs := []struct {
		name string
		refundKeyOnly bool
	}{
		{"refund_key_only", true},
		{"refund_id_only", false},
	}
	for _, sp := range specs {
		sp := sp
		t.Run(sp.name, func(t *testing.T) {
			r := RefundRow{
				ID:                 refundUUID,
				OrderID:            orderUUID,
				RequestedAmount:    40_000,
				Status:             "admin_refunded",
				GatewayStatus:      GatewayRefundStatusPending,
				GatewayRequestedAt: ptrTime(fixedNow.Add(-1 * time.Hour)),
			}
			if sp.refundKeyOnly {
				r.GatewayIdempotencyKey = gatewayRefKey
			} else {
				r.GatewayRefundID = gatewayRefID
			}
			s := cleanSnapshot()
			s.Refunds = []RefundRow{r}

			got := Classify(s)
			found := false
			for _, f := range got {
				if f.DriftClass == DriftD11StuckPendingRefund {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("D11 must fire when one of (gateway_idempotency_key, gateway_refund_id) is populated; got %+v", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Permutation-stability suite (owner-required)
//
// Future DB planner / query ordering changes must NOT alter reconciliation
// findings. Shuffling every slice input to Classify and comparing byte-for-
// byte against the baseline output proves the classifier is order-invariant
// over its slice inputs. Map inputs (LedgerLookup, OutboxLookup) are tested
// for iteration-order robustness by TestClassify_DeterministicAcrossInvocations
// (Go map iteration is randomized; classifier output is stable).
// ---------------------------------------------------------------------------

// permutationFixture builds a maximally heterogeneous snapshot where
// multiple drift classes co-fire over multi-row inputs. Used as the input to
// the permutation-stability suite.
func permutationFixture() Snapshot {
	s := cleanSnapshot()

	// Multiple refunds — two succeeded + one stuck-pending (past grace).
	s.Refunds = []RefundRow{
		{
			ID: uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001"),
			OrderID: orderUUID, RequestedAmount: 30_000,
			Status: "admin_refunded", GatewayStatus: GatewayRefundStatusSucceeded,
			GatewayRefundID: "MT-RF-001", GatewayIdempotencyKey: "key-001",
			GatewayRequestedAt:    ptrTime(fixedNow.Add(-2 * time.Hour)),
			GatewayAcknowledgedAt: ptrTime(fixedNow.Add(-1*time.Hour - 30*time.Minute)),
		},
		{
			ID: uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000002"),
			OrderID: orderUUID, RequestedAmount: 20_000,
			Status: "admin_refunded", GatewayStatus: GatewayRefundStatusSucceeded,
			GatewayRefundID: "MT-RF-002", GatewayIdempotencyKey: "key-002",
			GatewayRequestedAt:    ptrTime(fixedNow.Add(-1 * time.Hour)),
			GatewayAcknowledgedAt: ptrTime(fixedNow.Add(-50 * time.Minute)),
		},
		{
			ID: uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000003"),
			OrderID: orderUUID, RequestedAmount: 10_000,
			Status: "admin_refunded", GatewayStatus: GatewayRefundStatusPending,
			GatewayRefundID: "MT-RF-003", GatewayIdempotencyKey: "key-003",
			GatewayRequestedAt: ptrTime(fixedNow.Add(-30 * time.Minute)), // past 5min grace
		},
	}

	// Gateway refund history mirrors the two succeeded entries.
	s.Gateway.RefundChargebackHistory = []GatewayRefundEntry{
		{RefundKey: "key-001", RefundID: "MT-RF-001", Amount: 30_000, Status: GatewayRefundHistorySuccess, CreatedAt: fixedNow.Add(-2 * time.Hour)},
		{RefundKey: "key-002", RefundID: "MT-RF-002", Amount: 20_000, Status: GatewayRefundHistorySuccess, CreatedAt: fixedNow.Add(-1 * time.Hour)},
		{RefundKey: "key-003", RefundID: "MT-RF-003", Amount: 10_000, Status: GatewayRefundHistorySuccess, CreatedAt: fixedNow.Add(-30 * time.Minute)}, // ack lost → drives D4 + D11
	}

	// Multiple webhooks — base settlement + an in-flight processing row with
	// a distinct gateway transaction_id (drives D5 collision below).
	s.Webhooks = []WebhookEventRef{
		{
			EventID: "WH-base", MidtransOrderID: midtransOID,
			Status:            WebhookStatusSucceeded,
			TransactionStatus: GatewayStatusSettlement, TransactionID: "MT-TX-BASE",
			ReceivedAt:  fixedNow.Add(-3 * time.Hour),
			ProcessedAt: ptrTime(fixedNow.Add(-2*time.Hour - 50*time.Minute)),
		},
		{
			EventID: "WH-aux", MidtransOrderID: midtransOID,
			Status:            WebhookStatusProcessing,
			TransactionStatus: GatewayStatusSettlement, TransactionID: "MT-TX-AUX",
			ReceivedAt: fixedNow.Add(-1 * time.Hour),
		},
	}

	// Ledger map — release entry exists, two refund reversals exist.
	s.Ledger = LedgerLookup{
		BuyerSettlementExists: true,
		OrderReleaseExists:    true,
		OrderReleaseAmount:    100_000,
		RefundReversalExistsByRefundID: map[uuid.UUID]bool{
			s.Refunds[0].ID: true,
			s.Refunds[1].ID: true,
		},
	}

	// Outbox map — release alive; refund #2's money.refund_succeeded dead;
	// coins.refund_required missing (50k cumulative < 100k gross, so D9
	// doesn't fire — but we still exercise the map). Two D14 findings
	// expected: refund #0 missing money.refund_succeeded? Actually we set
	// alive for #0, dead for #1. Plan: refund #0 alive, refund #1 dead.
	s.Outbox = OutboxLookup{
		MoneyReleasedAliveByOrderID: map[uuid.UUID]bool{orderUUID: true},
		MoneyRefundSucceededAliveByRefundID: map[uuid.UUID]bool{
			s.Refunds[0].ID: true,
			// s.Refunds[1].ID intentionally absent → dead → triggers D14
		},
		CoinsRefundRequiredAliveByOrderID: map[uuid.UUID]bool{},
	}

	// Force a D5 collision (third distinct settled transaction_id) and a D8
	// mismatch on top of the refund signal.
	s.Webhooks = append(s.Webhooks, WebhookEventRef{
		EventID: "WH-dup", MidtransOrderID: midtransOID,
		Status:            WebhookStatusSucceeded,
		TransactionStatus: GatewayStatusSettlement, TransactionID: "MT-TX-DUP",
		ReceivedAt: fixedNow.Add(-30 * time.Minute),
	})
	s.Order.EscrowStatus = OrderEscrowStatusHolding // disagrees with escrow.status=released → D8

	return s
}

func TestClassify_PermutationStability_AllSlices(t *testing.T) {
	baseline := permutationFixture()
	baselineOut := Classify(baseline)
	if len(baselineOut) == 0 {
		t.Fatal("permutation fixture must produce at least one finding; check setup")
	}

	// Sanity: baseline must contain the multi-row drift classes we care about.
	{
		seen := map[DriftClass]bool{}
		for _, f := range baselineOut {
			seen[f.DriftClass] = true
		}
		for _, required := range []DriftClass{
			DriftD4PartialRefundMismatch,
			DriftD5DuplicateSettlement,
			DriftD8EscrowStateMismatch,
			DriftD11StuckPendingRefund,
			DriftD14LedgerEntryOutboxMissing,
		} {
			if !seen[required] {
				t.Fatalf("permutation fixture is missing required drift class %s; baseline=%+v", required, baselineOut)
			}
		}
	}

	// Reverse permutation — strongest single perturbation.
	t.Run("reverse_all_slices", func(t *testing.T) {
		perm := clonePermutation(baseline)
		reverseRefunds(perm.Refunds)
		reverseGatewayRefunds(perm.Gateway.RefundChargebackHistory)
		reverseWebhooks(perm.Webhooks)

		got := Classify(perm)
		assertFindingsByteEqual(t, "reverse", baselineOut, got)
	})

	// Randomised permutations across seeded RNG. Each seed produces a fresh
	// permutation independent of every other; classifier must produce
	// byte-equivalent output every time.
	for seed := int64(1); seed <= 64; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("shuffle_seed_%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			perm := clonePermutation(baseline)
			rng.Shuffle(len(perm.Refunds), func(i, j int) {
				perm.Refunds[i], perm.Refunds[j] = perm.Refunds[j], perm.Refunds[i]
			})
			rng.Shuffle(len(perm.Gateway.RefundChargebackHistory), func(i, j int) {
				perm.Gateway.RefundChargebackHistory[i], perm.Gateway.RefundChargebackHistory[j] =
					perm.Gateway.RefundChargebackHistory[j], perm.Gateway.RefundChargebackHistory[i]
			})
			rng.Shuffle(len(perm.Webhooks), func(i, j int) {
				perm.Webhooks[i], perm.Webhooks[j] = perm.Webhooks[j], perm.Webhooks[i]
			})

			got := Classify(perm)
			assertFindingsByteEqual(t, fmt.Sprintf("seed=%d", seed), baselineOut, got)
		})
	}
}

// Map inputs are tested for stability via repeated classify invocations
// against an unchanged snapshot — Go's randomized map iteration provides
// the implicit permutation, and asserting byte-equal output across many
// invocations proves order-invariance.
func TestClassify_PermutationStability_MapsViaRepeatedInvocation(t *testing.T) {
	s := permutationFixture()
	first := Classify(s)
	for i := 0; i < 128; i++ {
		again := Classify(s)
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("map-iteration sensitivity at invocation %d\n  first: %+v\n  again: %+v", i, first, again)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func extractDriftClasses(findings []Finding) []DriftClass {
	out := make([]DriftClass, len(findings))
	for i, f := range findings {
		out[i] = f.DriftClass
	}
	return out
}

func sortDriftClasses(s []DriftClass) {
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
}

// equalDriftSlices treats nil and empty as equal; reflect.DeepEqual does not.
func equalDriftSlices(a, b []DriftClass) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// clonePermutation deep-copies slice and map containers so a shuffle in one
// permutation cannot leak into the baseline or sibling permutations. The
// underlying row structs are value types (no embedded pointers we mutate),
// so a slice copy is sufficient. Time pointers are shared but never written
// by the classifier.
func clonePermutation(s Snapshot) Snapshot {
	out := s
	out.Refunds = append([]RefundRow(nil), s.Refunds...)
	out.Gateway.RefundChargebackHistory = append([]GatewayRefundEntry(nil), s.Gateway.RefundChargebackHistory...)
	out.Webhooks = append([]WebhookEventRef(nil), s.Webhooks...)
	if s.Ledger.RefundReversalExistsByRefundID != nil {
		m := make(map[uuid.UUID]bool, len(s.Ledger.RefundReversalExistsByRefundID))
		for k, v := range s.Ledger.RefundReversalExistsByRefundID {
			m[k] = v
		}
		out.Ledger.RefundReversalExistsByRefundID = m
	}
	if s.Outbox.MoneyReleasedAliveByOrderID != nil {
		m := make(map[uuid.UUID]bool, len(s.Outbox.MoneyReleasedAliveByOrderID))
		for k, v := range s.Outbox.MoneyReleasedAliveByOrderID {
			m[k] = v
		}
		out.Outbox.MoneyReleasedAliveByOrderID = m
	}
	if s.Outbox.MoneyRefundSucceededAliveByRefundID != nil {
		m := make(map[uuid.UUID]bool, len(s.Outbox.MoneyRefundSucceededAliveByRefundID))
		for k, v := range s.Outbox.MoneyRefundSucceededAliveByRefundID {
			m[k] = v
		}
		out.Outbox.MoneyRefundSucceededAliveByRefundID = m
	}
	if s.Outbox.CoinsRefundRequiredAliveByOrderID != nil {
		m := make(map[uuid.UUID]bool, len(s.Outbox.CoinsRefundRequiredAliveByOrderID))
		for k, v := range s.Outbox.CoinsRefundRequiredAliveByOrderID {
			m[k] = v
		}
		out.Outbox.CoinsRefundRequiredAliveByOrderID = m
	}
	return out
}

func reverseRefunds(rs []RefundRow) {
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}
}

func reverseGatewayRefunds(es []GatewayRefundEntry) {
	for i, j := 0, len(es)-1; i < j; i, j = i+1, j-1 {
		es[i], es[j] = es[j], es[i]
	}
}

func reverseWebhooks(ws []WebhookEventRef) {
	for i, j := 0, len(ws)-1; i < j; i, j = i+1, j-1 {
		ws[i], ws[j] = ws[j], ws[i]
	}
}

// assertFindingsByteEqual fails the test if the two finding slices are not
// DeepEqual. "Byte-equal" here means: same length, same DriftClass /
// Severity / IdempotencyKey / SuggestedAction / Notes / amounts / OrderID /
// RefundID / DetectedAt at the same indices.
func assertFindingsByteEqual(t *testing.T, label string, want, got []Finding) {
	t.Helper()
	if reflect.DeepEqual(want, got) {
		return
	}
	t.Fatalf("permutation %s broke byte-equivalence:\n  want (len=%d): %+v\n  got  (len=%d): %+v",
		label, len(want), want, len(got), got)
}

func validateFindingShape(f Finding) error {
	if f.DriftClass == "" {
		return fmt.Errorf("DriftClass empty")
	}
	if f.Severity == "" {
		return fmt.Errorf("Severity empty")
	}
	if f.IdempotencyKey == "" {
		return fmt.Errorf("IdempotencyKey empty")
	}
	if !strings.HasPrefix(f.IdempotencyKey, "recon|") {
		return fmt.Errorf("IdempotencyKey missing 'recon|' prefix: %s", f.IdempotencyKey)
	}
	if f.DetectedAt.IsZero() {
		return fmt.Errorf("DetectedAt zero")
	}
	if f.SuggestedAction == "" {
		return fmt.Errorf("SuggestedAction empty (operator guidance required)")
	}
	return nil
}


