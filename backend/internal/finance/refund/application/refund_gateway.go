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
	walletEntity "github.com/labuda/backend/internal/core/wallet/entity"
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
type DisputeFreezeReleaser interface {
	ReleaseDisputeFreezeByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) error
}
type OrderRefundStatusSyncer interface {
	SyncRefundSettlementFromGatewayAck(ctx context.Context, tx db.Tx, orderID, refundID uuid.UUID, fullyRefunded bool, occurredAt time.Time) error
}

func (s *RefundService) SetGatewayClient(client GatewayRefundClient, logger *zap.Logger) {
	s.gatewayClient = client
	s.gatewayLogger = logger
}
func (s *RefundService) SetFinanceReverser(reverser FinanceReverser) { s.financeReverser = reverser }
func (s *RefundService) SetDisputeFreezeReleaser(releaser DisputeFreezeReleaser) {
	s.freezeReleaser = releaser
}
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

	refund, err := s.refundRepo.GetForUpdate(ctx, tx, input.RefundID)
	if err != nil {
		return nil, fmt.Errorf("refund not found: %w", err)
	}
	if refund.GatewayStatus == entity.GatewayRefundSucceeded {
		return nil, ErrRefundAlreadySettledByGateway
	}

	escrow, err := s.walletService.GetEscrowForOrder(ctx, tx, refund.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to load escrow: %w", err)
	}
	if escrow == nil {
		return nil, fmt.Errorf("cannot refund: no escrow for order")
	}
	if escrow.Status != walletEntity.EscrowStatusHolding {
		return nil, fmt.Errorf("gateway refund requires escrow in holding state, got %q", escrow.Status)
	}

	order, err := s.orderRepo.GetForUpdate(ctx, tx, refund.OrderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}
	// Cap: PD + S (excludes C and F). K subtraction handled by cashRefund caller.
	orderCap := order.Subtotal.Int64() + order.ShippingTotal.Int64()
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

	refund, err = s.refundRepo.GetForUpdate(ctx, tx, refund.ID)
	if err != nil {
		return fmt.Errorf("re-lock refund: %w", err)
	}

	now := time.Now()
	success := isWebhookRefundSuccess(notification)

	if success && refund.GatewayStatus == entity.GatewayRefundSucceeded {
		return nil
	}
	if !success && refund.GatewayStatus == entity.GatewayRefundFailed {
		return nil
	}

	if success {
		refund.MarkGatewayAckSucceeded(notification.RefundChargeID, now)

		if s.financeReverser != nil {
			order, err := s.orderRepo.GetForUpdate(ctx, tx, refund.OrderID)
			if err != nil {
				return fmt.Errorf("refund reversal: lock order: %w", err)
			}
			escrow, err := s.walletService.GetEscrowForOrder(ctx, tx, refund.OrderID)
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
			// authoritative (never persisted). Fall back to Subtotal only when
			// the buyer base is absent (legacy rows / test fixtures).
			pd := order.TotalBeforeCoinsAmount.Int64() - order.ShippingTotal.Int64()
			if pd <= 0 {
				pd = order.Subtotal.Int64()
			}
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

			afterRelease := escrow.Status == walletEntity.EscrowStatusReleased
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

			escrowAlreadyTerminal := escrow.Status != walletEntity.EscrowStatusHolding
			cumCash := breakdown.CumProductRefundAfter + breakdown.CumShippingRefundAfter - breakdown.CumCoinsRestoredAfter
			fullyRefunded := !summary.Duplicate && !escrowAlreadyTerminal && cumCash >= pd+sVal-kVal
			partiallyRefunded := !summary.Duplicate && !escrowAlreadyTerminal && cumCash > 0 && cumCash < pd+sVal-kVal

			if fullyRefunded {
				if _, _, err := s.walletService.RefundGatewayEscrow(ctx, tx, refund.OrderID); err != nil {
					return fmt.Errorf("refund reversal: flip escrow: %w", err)
				}
			} else if partiallyRefunded {
				if _, _, err := s.walletService.PartialRefundGatewayEscrow(ctx, tx, refund.OrderID, breakdown.CashRefund); err != nil {
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

func isWebhookRefundSuccess(n *midtrans.NotificationPayload) bool {
	if n.TransactionStatus != string(midtrans.StatusRefund) && n.TransactionStatus != string(midtrans.StatusPartialRefund) {
		return false
	}
	return n.StatusCode == "" || n.StatusCode == "200"
}
