package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	orderapp "github.com/labuda/backend/internal/commerce/order/application"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	financeApp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// CoinSpendConsumer completes the canonical coin lifecycle
// RESERVE → CONSUME (order_spend) when an order payment settles with coins
// redeemed (K > 0). Implemented by coinsApp.CoinsService.ConsumeAndSpendForOrder.
// K remains authoritative in the coins domain; this interface only carries the
// consume+spend into the settlement transaction so it is atomic with the
// payment settlement, escrow creation, and platform K funding.
type CoinSpendConsumer interface {
	ConsumeAndSpendForOrder(ctx context.Context, tx db.Tx, userID uuid.UUID, orderID uuid.UUID, paymentID uuid.UUID, amount int64) error
}

// CanonicalFinalizationService owns the canonical order-payment finalization
// sequence after webhook ingestion has already authenticated and validated the
// incoming gateway notification.
//
// It contains only the business-finalization steps:
// payment settlement, coin consume/spend, finance settlement, platform K
// funding, escrow creation, order paid, and the outbox-emitting order
// transition.
type CanonicalFinalizationService struct {
	settlementService *repository.PaymentSettlementService
	financeService    *financeApp.FinanceService
	walletService     *walletApp.WalletService
	orderService      *orderapp.OrderService
	orderRepo         *orderRepoImpl.OrderRepository
	coinSpendConsumer CoinSpendConsumer
	log               *zap.Logger
}

// NewCanonicalFinalizationService creates the canonical finalization service
// using the canonical order, wallet, and finance services.
func NewCanonicalFinalizationService(
	financeService *financeApp.FinanceService,
	orderService *orderapp.OrderService,
	walletService *walletApp.WalletService,
	log *zap.Logger,
) *CanonicalFinalizationService {
	return &CanonicalFinalizationService{
		settlementService: repository.NewPaymentSettlementService(),
		financeService:    financeService,
		walletService:     walletService,
		orderService:      orderService,
		orderRepo:         orderRepoImpl.NewOrderRepository(),
		log:               log,
	}
}

// SetCoinSpendConsumer wires the coins-domain consume+spend surface used to
// complete the RESERVE → CONSUME lifecycle at settlement. Optional: when
// unset, coin redemption is skipped (K is treated as 0), which is only safe
// for orders paid without coins. Production wiring always sets it.
func (s *CanonicalFinalizationService) SetCoinSpendConsumer(c CoinSpendConsumer) {
	s.coinSpendConsumer = c
}

// SetAuditService wires the audit service into the internal settlement
// service so payment.settled audit behavior remains identical.
func (s *CanonicalFinalizationService) SetAuditService(auditService interface {
	PaymentSettled(ctx context.Context, tx db.Tx, paymentID uuid.UUID, amount int64)
	PaymentFailed(ctx context.Context, tx db.Tx, paymentID uuid.UUID, reason string)
	PaymentCreated(ctx context.Context, tx db.Tx, paymentID, userID uuid.UUID, amount int64)
}) {
	s.settlementService.SetAuditService(auditService)
}

// FinalizeOrderPayment runs the canonical order-payment finalization chain.
// It assumes webhook ingestion has already validated signature, deduped the
// event, resolved the payment row, and validated the amount.
func (s *CanonicalFinalizationService) FinalizeOrderPayment(
	ctx context.Context,
	tx db.Tx,
	payment *repository.Payment,
	transactionID string,
	paymentType string,
) error {
	if payment == nil {
		return fmt.Errorf("payment cannot be nil")
	}
	if payment.ReferenceType != repository.ReferenceTypeOrder {
		return fmt.Errorf("canonical finalization only supports order payments: %s", payment.ReferenceType)
	}
	if payment.ReferenceID == nil || *payment.ReferenceID == uuid.Nil {
		return fmt.Errorf("payment has no valid reference_id")
	}

	if err := s.settlementService.SettlePaymentByID(ctx, tx, payment.ID, transactionID, paymentType); err != nil {
		return fmt.Errorf("failed to settle payment: %w", err)
	}

	if s.log != nil {
		s.log.Info("Payment settled successfully",
			zap.String("payment_id", payment.ID.String()),
		)
	}

	// CANONICAL COIN CONSUME+SPEND (RESERVE → CONSUME): if the buyer redeemed
	// coins (K = payment.CoinsToUse > 0), complete the coin lifecycle in the
	// same transaction as settlement: consume the reservation exactly once,
	// write the canonical order_spend transaction, and deduct K from the
	// user's coin balance exactly once. K is authoritative in the coins
	// domain; no finance ledger entry is created here (the platform funding
	// of K is booked separately by FinanceService.RecordCoinFunding).
	if payment.CoinsToUse > 0 {
		if s.coinSpendConsumer == nil {
			return fmt.Errorf("coin spend consumer not configured for payment with coins_to_use=%d", payment.CoinsToUse)
		}
		if err := s.coinSpendConsumer.ConsumeAndSpendForOrder(
			ctx, tx, payment.UserID, *payment.ReferenceID, payment.ID, int64(payment.CoinsToUse),
		); err != nil {
			return fmt.Errorf("failed to consume and spend coins at settlement: %w", err)
		}
	}

	orderID := *payment.ReferenceID
	order, err := s.orderRepo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return fmt.Errorf("failed to load order for escrow: %w", err)
	}

	// Canonical funding ledger. Fail closed if not wired.
	if s.financeService == nil {
		return fmt.Errorf("finance service not wired for settlement funding")
	}
	if err := s.financeService.RecordGatewayPaymentSettlement(
		ctx, tx, payment.ID, orderID, transactionID, payment.GrossAmount.Int64(),
	); err != nil {
		return fmt.Errorf("failed to record settlement funding: %w", err)
	}

	// PASS_18V: immediately carve the buyer payment method fee out of
	// GATEWAY_CLEARING into PLATFORM_REVENUE. Everything after this point
	// (escrow creation, release, refund) operates on the seller-side
	// escrow amount only — the fee never touches those paths.
	if err := s.financeService.RecordBuyerPaymentFeeRevenue(
		ctx, tx, payment.ID, orderID, payment.ServiceFeeAmount.Int64(),
	); err != nil {
		return fmt.Errorf("failed to record buyer payment fee revenue: %w", err)
	}

	// CANONICAL PLATFORM FUNDING OF K: after settlement + fee sweep,
	// GATEWAY_CLEARING = BuyerBase - K. The seller's economic entitlement is
	// BuyerBase = PD + S, so the platform funds K into GATEWAY_CLEARING
	// (DR PLATFORM_BANK / CR GATEWAY_CLEARING) before release. This makes the
	// clearing account hold exactly BuyerBase — fully funding the seller
	// release without ever overdrawing. K never becomes platform revenue.
	if payment.CoinsToUse > 0 {
		if err := s.financeService.RecordCoinFunding(
			ctx, tx, payment.ID, orderID, int64(payment.CoinsToUse),
		); err != nil {
			return fmt.Errorf("failed to record platform coin funding: %w", err)
		}
	}

	// CANONICAL ESCROW AMOUNT: EscrowAmount = PD + S = total_before_coins_amount.
	// The buyer-funded escrow is the persisted, token-validated buyer base —
	// commission C is a seller/platform-side allocation and is NOT added to
	// buyer-funded cash. CalculateGrossEscrowFromSnapshot (P+S+C) is the
	// rejected model and must not fund the escrow row.
	escrowAmount := order.TotalBeforeCoinsAmount
	if _, err := s.walletService.CreateEscrowFromGatewaySettlement(ctx, tx, walletApp.CreateEscrowFromGatewaySettlementInput{
		OrderID:   orderID,
		BuyerID:   order.BuyerID,
		SellerID:  order.SellerID,
		Amount:    escrowAmount.Int64(),
		PaymentID: payment.ID,
	}); err != nil {
		return fmt.Errorf("failed to create gateway escrow: %w", err)
	}

	if err := s.orderService.MarkPaid(ctx, tx, orderID); err != nil {
		return fmt.Errorf("failed to mark order as paid: %w", err)
	}

	if s.log != nil {
		s.log.Info("Order marked as paid",
			zap.String("payment_id", payment.ID.String()),
			zap.String("order_id", orderID.String()),
		)
	}

	return nil
}
