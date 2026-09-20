// Package application: gateway-aware refund orchestration (TASK 33, S2C2 rebase).
//
// CANONICAL S2C2 REFUND ECONOMICS:
//
//	CashRefund = Rpd + Rs - CoinDelta   (gateway cash, excludes C and F)
//	CoinDelta  = floor(K * cumProductAfter / PD) - floor(K * cumProductBefore / PD)
//	CommissionDelta = floor(C * cumProductAfter / PD) - floor(C * cumProductBefore / PD)
//	SellerComponent = Rpd + Rs - CommissionDelta
//	Max gateway cash = PD + S - K
//	F is non-refundable. C is never in buyer refund.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	escrowEntity "github.com/labuda/backend/internal/core/escrow/entity"
	financeapp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/internal/finance/refund/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"go.uber.org/zap"
)

type GatewayRefundClient interface {
	RefundWithKey(ctx context.Context, orderID, refundKey string, amount int64, reason string) (*midtrans.RefundResponse, error)
}
type FinanceReverser interface {
	RecordRefundReversal(ctx context.Context, tx db.Tx, input financeapp.RecordRefundReversalInput) (*financeapp.RecordRefundReversalSummary, error)
	RecordPartialRefundRelease(ctx context.Context, tx db.Tx, input financeapp.RecordPartialRefundReleaseInput) (bool, error)
	// RecordCoinFundingReversal reverses the platform funding of K for the
	// coins restored to the buyer on this refund event (CoinDelta). Called in
	// the same tx as RecordRefundReversal so GATEWAY_CLEARING is never left
	// holding platform funding for a refunded entitlement.
	RecordCoinFundingReversal(ctx context.Context, tx db.Tx, refundID uuid.UUID, orderID uuid.UUID, amount int64) error
}
type OrderRefundStatusSyncer interface {
	SyncRefundSettlementFromGatewayAck(ctx context.Context, tx db.Tx, orderID, refundID uuid.UUID, fullyRefunded bool, occurredAt time.Time) error
}

func (s *RefundService) SetGatewayClient(client GatewayRefundClient, logger *zap.Logger) {
	s.gatewayClient = client
	s.gatewayLogger = logger
}
func (s *RefundService) SetFinanceReverser(reverser FinanceReverser) { s.financeReverser = reverser }
func (s *RefundService) SetOrderRefundStatusSyncer(syncer OrderRefundStatusSyncer) {
	s.orderRefundStatusSyncer = syncer
}

func (s *RefundService) gatewayLog() *zap.Logger {
	if s.gatewayLogger != nil {
		return s.gatewayLogger
	}
	return zap.NewNop()
}

var ErrGatewayClientNotConfigured = errors.New("gateway refund client not configured")
var ErrRefundAlreadySettledByGateway = errors.New("refund already settled at gateway")

// SystemRefundInput — S2C2: ProductAmount (Rpd) + ShippingAmount (Rs) + order/payment snapshots.
//
// AdminID is the human admin who decided the refund, persisted as
// refunds.reviewed_by. Automatic platform refunds have no human reviewer and
// MUST pass uuid.Nil (persisted as NULL).
type SystemRefundInput struct {
	OrderID, BuyerID, SellerID, AdminID uuid.UUID
	ProductAmount                       int64 // Rpd
	ShippingAmount                      int64 // Rs
	PD, S, C, K                         int64 // order/payment snapshots
	Reason                              entity.RefundReason
	Description                         *string
	IdempotencyKey                      string
}

func (s *RefundService) CreateAndDispatchSystemRefund(ctx context.Context, tx db.Tx, input SystemRefundInput) (*entity.Refund, error) {
	if s.gatewayClient == nil {
		return nil, ErrGatewayClientNotConfigured
	}
	if input.OrderID == uuid.Nil {
		return nil, fmt.Errorf("system refund: order_id required")
	}
	if input.ProductAmount < 0 || input.ProductAmount > input.PD {
		return nil, fmt.Errorf("system refund: Rpd %d out of range [0, PD=%d]", input.ProductAmount, input.PD)
	}
	if input.ShippingAmount < 0 || input.ShippingAmount > input.S {
		return nil, fmt.Errorf("system refund: Rs %d out of range [0, S=%d]", input.ShippingAmount, input.S)
	}
	if input.PD <= 0 {
		return nil, fmt.Errorf("system refund: PD must be positive")
	}
	if input.IdempotencyKey == "" {
		return nil, fmt.Errorf("system refund: idempotency key required")
	}

	// Compute coin delta for this event so we know the gateway cash amount.
	coinsBefore := proportionalFloor(int64(0), input.K, input.PD) // cumProductBefore=0 for new refund
	coinsAfter := proportionalFloor(input.ProductAmount, input.K, input.PD)
	coinDelta := coinsAfter - coinsBefore
	cashRefund := input.ProductAmount + input.ShippingAmount - coinDelta
	if cashRefund <= 0 {
		return nil, fmt.Errorf("system refund: cashRefund must be positive (Rpd=%d Rs=%d coinDelta=%d)", input.ProductAmount, input.ShippingAmount, coinDelta)
	}
	if cashRefund > input.PD+input.S-input.K {
		return nil, fmt.Errorf("system refund: cashRefund %d exceeds cap PD+S-K=%d", cashRefund, input.PD+input.S-input.K)
	}

	existing, err := s.refundRepo.GetByGatewayIdempotencyKey(ctx, tx, input.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("system refund: idempotency lookup: %w", err)
	}
	if existing != nil && (existing.GatewayStatus == entity.GatewayRefundSucceeded || existing.GatewayStatus == entity.GatewayRefundPending) {
		return existing, nil
	}

	var refund *entity.Refund
	if existing != nil {
		refund = existing
	} else {
		refund = entity.NewSystemRefund(input.OrderID, input.BuyerID, input.SellerID, input.AdminID, input.Reason, input.ProductAmount, input.ShippingAmount, input.Description)
		if err := s.refundRepo.Create(ctx, tx, refund); err != nil {
			return nil, fmt.Errorf("system refund: create: %w", err)
		}
	}

	_, dispatchErr := s.InitiateGatewayRefund(ctx, tx, InitiateGatewayRefundInput{
		RefundID: refund.ID, Amount: cashRefund, Reason: string(input.Reason),
		IdempotencyKey: input.IdempotencyKey, CallerID: auth.SystemCallerID, CallerType: GatewayRefundCallerTypeSystem,
	})
	if dispatchErr != nil {
		return nil, fmt.Errorf("system refund: dispatch: %w", dispatchErr)
	}
	refreshed, err := s.refundRepo.GetByID(ctx, tx, refund.ID)
	if err != nil {
		return nil, fmt.Errorf("system refund: reload: %w", err)
	}
	if refreshed != nil {
		refund = refreshed
	}
	return refund, nil
}

func (s *RefundService) CreateAndDispatchSystemRefundFlat(ctx context.Context, tx db.Tx,
	orderID, buyerID, sellerID, adminID uuid.UUID,
	productAmount, shippingAmount, pd, sVal, c, k int64, reason, idempotencyKey string,
) error {
	_, err := s.CreateAndDispatchSystemRefund(ctx, tx, SystemRefundInput{
		OrderID: orderID, BuyerID: buyerID, SellerID: sellerID, AdminID: adminID,
		ProductAmount: productAmount, ShippingAmount: shippingAmount,
		PD: pd, S: sVal, C: c, K: k,
		Reason: entity.RefundReason(reason), IdempotencyKey: idempotencyKey,
	})
	return err
}

type InitiateGatewayRefundInput struct {
	RefundID       uuid.UUID
	Amount         int64
	Reason         string
	IdempotencyKey string
	CallerID       uuid.UUID
	CallerType     GatewayRefundCallerType
}
type GatewayRefundCallerType string

const (
	GatewayRefundCallerTypeAdmin  GatewayRefundCallerType = "admin"
	GatewayRefundCallerTypeSystem GatewayRefundCallerType = "system"
)

type ErrGatewayRefundCallerProvenanceRequired struct{ Reason string }

func (e *ErrGatewayRefundCallerProvenanceRequired) Error() string {
	return "gateway refund caller provenance required: " + e.Reason
}

func (s *RefundService) InitiateGatewayRefund(ctx context.Context, tx db.Tx, input InitiateGatewayRefundInput) (*entity.Refund, error) {
	if s.gatewayClient == nil {
		return nil, ErrGatewayClientNotConfigured
	}
	if input.CallerID == uuid.Nil {
		return nil, &ErrGatewayRefundCallerProvenanceRequired{Reason: "caller_id required"}
	}
	switch input.CallerType {
	case GatewayRefundCallerTypeAdmin:
		if auth.IsSystemCaller(input.CallerID) {
			return nil, &ErrGatewayRefundCallerProvenanceRequired{Reason: "admin caller cannot be system caller"}
		}
	case GatewayRefundCallerTypeSystem:
		if !auth.IsSystemCaller(input.CallerID) {
			return nil, &ErrGatewayRefundCallerProvenanceRequired{Reason: "system caller must use system caller id"}
		}
	default:
		return nil, &ErrGatewayRefundCallerProvenanceRequired{Reason: "caller_type required"}
	}
	if input.IdempotencyKey == "" {
		return nil, fmt.Errorf("gateway refund idempotency key required")
	}
	if input.Amount <= 0 {
		return nil, fmt.Errorf("gateway refund amount must be positive")
	}

	if existing, err := s.refundRepo.GetByGatewayIdempotencyKey(ctx, tx, input.IdempotencyKey); err != nil {
		return nil, fmt.Errorf("idempotency lookup: %w", err)
	} else if existing != nil && existing.ID == input.RefundID && (existing.GatewayStatus == entity.GatewayRefundPending || existing.GatewayStatus == entity.GatewayRefundSucceeded) {
		return existing, nil
	}

	// CANONICAL LOCK ORDER: ORDER → REFUND (never REFUND → ORDER).
	// Read refund without lock first to obtain orderID for canonical lock ordering.
	refundSnapshot, err := s.refundRepo.GetByID(ctx, tx, input.RefundID)
	if err != nil {
		return nil, fmt.Errorf("refund not found: %w", err)
	}
	if refundSnapshot == nil {
		return nil, fmt.Errorf("refund not found: %w", err)
	}

	// Lock order first (canonical ORDER → REFUND order)
	order, err := s.orderRepo.GetForUpdate(ctx, tx, refundSnapshot.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// Now lock refund for state transition (after order lock acquired)
	refund, err := s.refundRepo.GetForUpdate(ctx, tx, input.RefundID)
	if err != nil {
		return nil, fmt.Errorf("refund not found: %w", err)
	}
	if refund.GatewayStatus == entity.GatewayRefundSucceeded {
		return nil, ErrRefundAlreadySettledByGateway
	}

	escrow, err := s.escrowService.GetEscrowForOrder(ctx, tx, refund.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to load escrow: %w", err)
	}
	if escrow == nil {
		return nil, fmt.Errorf("cannot refund: no escrow for order")
	}
	if escrow.Status != escrowEntity.EscrowStatusHolding {
		return nil, fmt.Errorf("gateway refund requires escrow in holding state, got %q", escrow.Status)
	}
	// Cap: PD + S = total_before_coins_amount (excludes C and F).
	// FAIL CLOSED: no fallback to the undiscounted P + S cap.
	if !order.HasCanonicalMoneyBase() {
		return nil, fmt.Errorf("cannot dispatch gateway refund: canonical buyer-funded base invalid (total_before_coins=%d shipping=%d)", order.TotalBeforeCoinsAmount.Int64(), order.ShippingTotal.Int64())
	}
	orderCap := order.TotalBeforeCoinsAmount.Int64()
	if input.Amount > orderCap {
		return nil, fmt.Errorf("gateway refund amount %d exceeds PD+S=%d", input.Amount, orderCap)
	}

	resp, dispatchErr := s.gatewayClient.RefundWithKey(ctx, refund.OrderID.String(), input.IdempotencyKey, input.Amount, input.Reason)
	now := time.Now()
	if dispatchErr != nil {
		errMsg := dispatchErr.Error()
		refund.MarkGatewayRequestFailed(errMsg, now)
		s.refundRepo.Update(ctx, tx, refund)
		s.emitGatewayOutbox(ctx, tx, refund, "money.refund_failed", input.Amount, &errMsg)
		s.gatewayLog().Warn("gateway_refund_failed", zap.String("refund_id", refund.ID.String()), zap.String("error", errMsg))
		return refund, dispatchErr
	}
	var gwID *string
	if resp != nil {
		switch {
		case resp.RefundChargeID != "":
			id := resp.RefundChargeID
			gwID = &id
		case resp.TransactionID != "":
			id := resp.TransactionID
			gwID = &id
		}
	}
	refund.MarkGatewayDispatched(input.IdempotencyKey, gwID, now)
	s.refundRepo.Update(ctx, tx, refund)
	s.emitGatewayOutbox(ctx, tx, refund, "money.refund_pending", input.Amount, nil)
	return refund, nil
}

// HandleGatewayRefundAck — S2C2 rebase: uses product/shipping split, CashRefund = Rpd+Rs-CoinDelta.
func (s *RefundService) HandleGatewayRefundAck(ctx context.Context, tx db.Tx, notification *midtrans.NotificationPayload) error {
	if notification == nil {
		return fmt.Errorf("nil refund notification")
	}

	var refund *entity.Refund
	var err error
	if notification.RefundChargeID != "" {
		refund, err = s.refundRepo.GetByGatewayRefundID(ctx, tx, notification.RefundChargeID)
		if err != nil {
			return fmt.Errorf("refund lookup by charge id: %w", err)
		}
	}
	if refund == nil && notification.RefundKey != "" {
		refund, err = s.refundRepo.GetByGatewayIdempotencyKey(ctx, tx, notification.RefundKey)
		if err != nil {
			return fmt.Errorf("refund lookup by refund key: %w", err)
		}
	}
	if refund == nil {
		return nil
	}

	// DUPLICATE CHECK (no locks needed): when the refund is already terminal
	// at gateway level, return immediately without acquiring any row locks.
	now := time.Now()
	success := isWebhookRefundSuccess(notification)
	if success && refund.GatewayStatus == entity.GatewayRefundSucceeded {
		return nil
	}
	if !success && refund.GatewayStatus == entity.GatewayRefundFailed {
		return nil
	}

	// CANONICAL LOCK ORDER: ORDER → REFUND (never REFUND → ORDER).
	// The refund row was already looked up without a lock above.
	// When financeReverser is wired, we need both ORDER and REFUND locks.
	// Lock ORDER first (canonical), then REFUND for state transition.
	// When financeReverser is nil, only REFUND is locked.
	var order *orderEntity.Order
	if s.financeReverser != nil {
		// Lock order first (canonical ORDER → REFUND order)
		order, err = s.orderRepo.GetForUpdate(ctx, tx, refund.OrderID)
		if err != nil {
			return fmt.Errorf("refund reversal: lock order: %w", err)
		}
	}

	// Now lock refund for state transition (after order lock acquired)
	refund, err = s.refundRepo.GetForUpdate(ctx, tx, refund.ID)
	if err != nil {
		return fmt.Errorf("re-lock refund: %w", err)
	}

	// Defense-in-depth: re-check idempotency after acquiring locks
	if success && refund.GatewayStatus == entity.GatewayRefundSucceeded {
		return nil
	}
	if !success && refund.GatewayStatus == entity.GatewayRefundFailed {
		return nil
	}

	if success {
		refund.MarkGatewayAckSucceeded(notification.RefundChargeID, now)

		// REC-6 SLICE 3: gateway_captured_after_order_invalid refunds are
		// confirmation-only — no escrow was created, no ledger was mutated,
		// no seller payable exists. The financial reversal block below requires
		// escrow in Holding state and would fail ("no escrow for order"). Skip
		// it entirely for this canonical reason; the refund state transition
		// (pending → succeeded) and the outbox event still apply.
		if refund.Reason == entity.RefundReasonGatewayCapturedAfterOrderInvalid {
			s.gatewayLog().Info("rec6_ack_confirmation_only",
				zap.String("refund_id", refund.ID.String()),
				zap.String("reason", string(refund.Reason)),
			)
		} else if s.financeReverser != nil {
			// Order already locked above (canonical ORDER → REFUND order)
			escrow, err := s.escrowService.GetEscrowForOrder(ctx, tx, refund.OrderID)
			if err != nil {
				return fmt.Errorf("refund reversal: load escrow: %w", err)
			}
			if escrow == nil {
				return fmt.Errorf("refund reversal: no escrow for order %s", refund.OrderID)
			}

			gatewayCash, err := parseMidtransRefundAmount(notification.RefundAmount)
			if err != nil {
				return fmt.Errorf("refund reversal: parse refund_amount %q: %w", notification.RefundAmount, err)
			}

			// CANONICAL PD DERIVATION (S2C2): PD = BuyerBase - S where
			// BuyerBase = total_before_coins_amount = (P-D)+S is the persisted,
			// token-validated buyer funding base. This is the canonical source
			// for the discounted product value; orders.discount_amount is NOT
			// authoritative (never persisted). FAIL CLOSED when the base is absent;
			// there is NO Subtotal (P) fallback.
			if !order.HasCanonicalMoneyBase() {
				return fmt.Errorf("refund reversal: canonical money base invalid (order_id=%s total_before_coins=%d shipping=%d)", refund.OrderID, order.TotalBeforeCoinsAmount.Int64(), order.ShippingTotal.Int64())
			}
			pd := order.DiscountedProductAmount().Int64()
			sVal := order.ShippingTotal.Int64()
			cVal := order.CommissionAmount.Int64()
			kVal, err := s.coinsSpendForOrder(ctx, tx, refund.BuyerID, refund.OrderID)
			if err != nil {
				return fmt.Errorf("refund reversal: resolve coins spent: %w", err)
			}

			// Derive Rpd/Rs from refund row (set at creation/dispatch) or fall back.
			rpd := int64(0)
			rs := int64(0)
			if refund.RefundedProductAmount != nil {
				rpd = *refund.RefundedProductAmount
			}
			if refund.RefundedShippingAmount != nil {
				rs = *refund.RefundedShippingAmount
			}
			if rpd == 0 && rs == 0 && gatewayCash > 0 {
				// Legacy fallback: split from product first, then shipping.
				rpd = gatewayCash
				if rpd > pd {
					rpd = pd
				}
				rs = gatewayCash - rpd
				if rs > sVal {
					rs = sVal
				}
				if rs < 0 {
					rs = 0
				}
			}

			cumProductBefore, err := s.refundRepo.GetCumulativeProductRefundByOrder(ctx, tx, refund.OrderID, &refund.ID)
			if err != nil {
				return fmt.Errorf("refund reversal: cumulative product before: %w", err)
			}
			cumShippingBefore, err := s.refundRepo.GetCumulativeShippingRefundByOrder(ctx, tx, refund.OrderID, &refund.ID)
			if err != nil {
				return fmt.Errorf("refund reversal: cumulative shipping before: %w", err)
			}
			cumCoinsBefore, err := s.refundRepo.GetCumulativeCoinsRefundedByOrder(ctx, tx, refund.OrderID, &refund.ID)
			if err != nil {
				return fmt.Errorf("refund reversal: cumulative coins before: %w", err)
			}
			cumCommissionBefore := proportionalFloor(cumProductBefore, cVal, pd)

			breakdown, err := CalculateProportionalRefundBreakdown(
				pd, sVal, cVal, kVal, rpd, rs,
				cumProductBefore, cumShippingBefore, cumCoinsBefore, cumCommissionBefore,
			)
			if err != nil {
				return fmt.Errorf("refund reversal: compute breakdown: %w", err)
			}

			// Validate gateway cash against expected CashRefund.
			if gatewayCash != breakdown.CashRefund {
				s.gatewayLog().Warn("refund_cash_mismatch",
					zap.String("refund_id", refund.ID.String()),
					zap.Int64("gateway_cash", gatewayCash),
					zap.Int64("expected_cash", breakdown.CashRefund),
				)
			}

			afterRelease := escrow.Status == escrowEntity.EscrowStatusReleased
			if afterRelease {
				return fmt.Errorf("post-release refund acknowledgements are disabled")
			}

			cumCashTotal := breakdown.CumProductRefundAfter + breakdown.CumShippingRefundAfter - breakdown.CumCoinsRestoredAfter
			summary, err := s.financeReverser.RecordRefundReversal(ctx, tx, financeapp.RecordRefundReversalInput{
				RefundID: refund.ID, OrderID: refund.OrderID, BuyerID: refund.BuyerID, SellerID: refund.SellerID,
				RefundAmount: breakdown.CashRefund, SellerComponent: breakdown.SellerComponent,
				CommissionComponent: breakdown.CommissionDelta,
				OrderGross:          pd + sVal, CumulativeRefunded: cumCashTotal,
				RoundingAdjustment: breakdown.RoundingAdjustment, AfterRelease: afterRelease,
			})
			if err != nil {
				return fmt.Errorf("refund reversal: record ledger: %w", err)
			}

			// CANONICAL PLATFORM FUNDING REVERSAL: the buyer's coins restored on
			// this event (CoinDelta) correspond to platform funding of K that is
			// no longer funding the seller's (refunded) entitlement. Return that
			// portion to PLATFORM_BANK so the platform funding is not stranded in
			// GATEWAY_CLEARING and the clearing account drains exactly to 0.
			if breakdown.CoinDelta > 0 {
				if err := s.financeReverser.RecordCoinFundingReversal(ctx, tx, refund.ID, refund.OrderID, breakdown.CoinDelta); err != nil {
					return fmt.Errorf("refund reversal: reverse platform coin funding: %w", err)
				}
			}

			refund.FinalRefundAmount = &breakdown.CashRefund
			refund.RefundedProductAmount = &rpd
			refund.RefundedShippingAmount = &rs
			refund.CoinsRefundedAmount = &breakdown.CoinDelta

			if !summary.Duplicate && refund.RefundedAt == nil {
				refund.RefundedAt = &now
				refund.UpdatedAt = now
			}

			escrowAlreadyTerminal := escrow.Status != escrowEntity.EscrowStatusHolding
			cumCash := breakdown.CumProductRefundAfter + breakdown.CumShippingRefundAfter - breakdown.CumCoinsRestoredAfter
			fullyRefunded := !summary.Duplicate && !escrowAlreadyTerminal && cumCash >= pd+sVal-kVal
			partiallyRefunded := !summary.Duplicate && !escrowAlreadyTerminal && cumCash > 0 && cumCash < pd+sVal-kVal

			if fullyRefunded {
				if _, _, err := s.escrowService.RefundGatewayEscrow(ctx, tx, refund.OrderID); err != nil {
					return fmt.Errorf("refund reversal: flip escrow: %w", err)
				}
			} else if partiallyRefunded {
				if _, _, err := s.escrowService.PartialRefundGatewayEscrow(ctx, tx, refund.OrderID, breakdown.CashRefund); err != nil {
					return fmt.Errorf("refund reversal: flip escrow: %w", err)
				}
				// CANONICAL REMAINDER: the seller's remaining economic entitlement
				// after this refund event is BuyerBase - (Rpd + Rs), i.e.
				// (PD + S) - (cumProductRefundAfter + cumShippingRefundAfter).
				// The platform funding of K for the refunded portion was already
				// reversed (RecordCoinFundingReversal with CoinDelta), so the
				// remainder is fully cash-backed by GATEWAY_CLEARING. The prior
				// formula (pd+s-k) - cumCash conflated coin restoration with
				// seller funding and understated the remainder by the restored
				// coin portion.
				remainder := (pd + sVal) - (breakdown.CumProductRefundAfter + breakdown.CumShippingRefundAfter)
				if remainder > 0 {
					remCommission := int64(0)
					if pd > 0 && cVal > 0 {
						remProduct := pd - breakdown.CumProductRefundAfter
						if remProduct > 0 {
							remCommission = proportionalFloor(remProduct, cVal, pd)
						}
					}
					s.financeReverser.RecordPartialRefundRelease(ctx, tx, financeapp.RecordPartialRefundReleaseInput{
						RefundID: refund.ID, OrderID: refund.OrderID, SellerID: refund.SellerID,
						Remainder: remainder, SellerNet: remainder - remCommission, Commission: remCommission,
					})
				}
			}

			if fullyRefunded || partiallyRefunded {
				if s.orderRefundStatusSyncer == nil {
					return fmt.Errorf("orderRefundStatusSyncer not configured")
				}
				if err := s.orderRefundStatusSyncer.SyncRefundSettlementFromGatewayAck(ctx, tx, refund.OrderID, refund.ID, fullyRefunded, now); err != nil {
					return fmt.Errorf("refund reversal: sync order status+escrow: %w", err)
				}
			}

			// Emit coins.refund_required with coin_delta for BOTH full and partial.
			coinsPayload := map[string]interface{}{
				"order_id": refund.OrderID.String(), "user_id": refund.BuyerID.String(),
				"coin_delta": breakdown.CoinDelta, "reason": "money_refund_succeeded", "source": "gateway_refund_ack",
			}
			coinsBytes, _ := json.Marshal(coinsPayload)
			s.outboxRepo.InsertEvent(ctx, tx, "coins.refund_required", refund.OrderID, coinsBytes)
		} else {
			s.gatewayLog().Warn("refund_reversal_unwired", zap.String("refund_id", refund.ID.String()))
		}

		s.refundRepo.Update(ctx, tx, refund)
		successAmount := int64(0)
		if refund.FinalRefundAmount != nil {
			successAmount = *refund.FinalRefundAmount
		}
		s.emitGatewayOutbox(ctx, tx, refund, "money.refund_succeeded", successAmount, nil)
		return nil
	}

	errMsg := fmt.Sprintf("gateway refund failed: status_code=%s message=%s", notification.StatusCode, notification.StatusMessage)
	refund.MarkGatewayAckFailed(errMsg, now)
	s.refundRepo.Update(ctx, tx, refund)
	s.emitGatewayOutbox(ctx, tx, refund, "money.refund_failed", 0, &errMsg)
	return nil
}

func (s *RefundService) emitGatewayOutbox(ctx context.Context, tx db.Tx, refund *entity.Refund, eventType string, amount int64, errMsg *string) error {
	payload := map[string]interface{}{
		"refund_id": refund.ID, "order_id": refund.OrderID, "gateway_status": string(refund.GatewayStatus),
		"gateway_attempts": refund.GatewayAttempts, "gateway_refund_id": refund.GatewayRefundID,
		"gateway_idempotency_key": refund.GatewayIdempotencyKey, "amount": amount,
	}
	if errMsg != nil {
		payload["error"] = *errMsg
	}
	bytes, _ := json.Marshal(payload)
	return s.outboxRepo.InsertEvent(ctx, tx, eventType, refund.ID, bytes)
}

// ===========================================================================
// REC-6 SLICE 1: CANONICAL REFUND-INTENT AUTHORITY
// ===========================================================================
//
// CreateRefundIntentForInvalidOrder creates a durable, idempotent refund intent
// for a gateway-success payment whose order entered a terminal state (expired,
// cancelled, etc.) that prevents normal payment finalization.
//
// This is THE canonical entry point for REC-6 refund-intent creation.
// All three producers (webhook, discovery, orphan recovery) must converge here.
//
// INVARIANTS:
//   - Creates a Refund row with status=system_refunded, gateway_status=unsubmitted
//   - Amount = payment.gross_amount (full captured amount, no escrow formula)
//   - No escrow created, no ledger mutation, no seller payable, no platform revenue
//   - No gateway HTTP dispatch (that's a later slice)
//   - Idempotent: repeated calls with the same paymentID return existing refund
//   - gateway_idempotency_key = "rec6:payment:<payment_id>" (deterministic)
//
// The refund row is the durable evidence that a refund is owed. The actual
// gateway dispatch and financial reversal are handled by later slices.
type Rec6RefundIntentInput struct {
	PaymentID             uuid.UUID
	OrderID               uuid.UUID
	BuyerID               uuid.UUID
	SellerID              uuid.UUID
	GrossAmount           int64 // payment.gross_amount — full captured amount
	PaymentMidtransOrderID string
}

// Rec6IdempotencyKey returns the deterministic gateway_idempotency_key for
// a REC-6 refund intent. The key is based on payment_id, which uniquely
// identifies the gateway capture event that triggered this refund.
func Rec6IdempotencyKey(paymentID uuid.UUID) string {
	return fmt.Sprintf("rec6:payment:%s", paymentID.String())
}

func (s *RefundService) CreateRefundIntentForInvalidOrder(
	ctx context.Context,
	tx db.Tx,
	input Rec6RefundIntentInput,
) (*entity.Refund, error) {
	if input.OrderID == uuid.Nil {
		return nil, fmt.Errorf("rec6 refund intent: order_id required")
	}
	if input.PaymentID == uuid.Nil {
		return nil, fmt.Errorf("rec6 refund intent: payment_id required")
	}
	if input.GrossAmount <= 0 {
		return nil, fmt.Errorf("rec6 refund intent: gross_amount must be positive, got %d", input.GrossAmount)
	}

	idempotencyKey := Rec6IdempotencyKey(input.PaymentID)

	// Idempotent: if a refund with this key already exists, return it.
	existing, err := s.refundRepo.GetByGatewayIdempotencyKey(ctx, tx, idempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("rec6 refund intent: idempotency lookup: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	// Create refund entity using the existing system-refund constructor.
	// productAmount = GrossAmount, shippingAmount = 0: the entire gateway
	// capture is treated as product amount because there is no escrow to
	// decompose. This is semantically correct: the full captured amount
	// needs to be refunded.
	//
	// ATTRIBUTION: this is an automatic system refund with no human reviewer,
	// so reviewed_by is recorded as NULL (uuid.Nil). auth.SystemCallerID is an
	// authorization/audit sentinel and MUST NOT be persisted as a users.id.
	refund := entity.NewSystemRefund(
		input.OrderID,
		input.BuyerID,
		input.SellerID,
		uuid.Nil, // no human reviewer — reviewed_by persists as NULL
		entity.RefundReasonGatewayCapturedAfterOrderInvalid,
		input.GrossAmount, // productAmount = full gross (no escrow split)
		0,                // shippingAmount = 0 (no escrow split)
		nil,              // description
	)

	// Set the deterministic idempotency key BEFORE persist.
	refund.GatewayIdempotencyKey = &idempotencyKey

	// Persist the refund row.
	if err := s.refundRepo.Create(ctx, tx, refund); err != nil {
		return nil, fmt.Errorf("rec6 refund intent: create: %w", err)
	}

	return refund, nil
}

func isWebhookRefundSuccess(n *midtrans.NotificationPayload) bool {
	if n.TransactionStatus != string(midtrans.StatusRefund) && n.TransactionStatus != string(midtrans.StatusPartialRefund) {
		return false
	}
	return n.StatusCode == "" || n.StatusCode == "200"
}
