// ⚠️ FINANCIAL RULE:
// All money operations MUST go through WalletService.
// Direct balance mutation is forbidden.
//
// Order domain is a PRICING SNAPSHOT only.
// Wallet domain is the SINGLE SOURCE OF TRUTH for all money operations.
package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	// FINANCIAL AUTHORITY: WalletService is the ONLY authority for money movement
	// All escrow operations MUST go through WalletService
	"github.com/labuda/backend/internal/commerce/order/entity"
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	walletEntity "github.com/labuda/backend/internal/core/wallet/entity"
	coinsentity "github.com/labuda/backend/internal/incentive/coins/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// OrderPaymentService handles payment operations for orders.
//
// CRITICAL ARCHITECTURAL CHANGE:
// - Order is now a PRICING SNAPSHOT ONLY (immutable after creation)
// - Order does NOT store: EscrowAmount, RefundedAmount, CoinsDiscount, Discount fields
// - WalletService is the SINGLE SOURCE OF TRUTH for all money operations
// - This service calculates amounts dynamically from Order snapshots when needed
//
// FinanceReleaseRecorder is the minimal finance-domain dependency required by
// the gateway-aware release path. FinanceService satisfies this interface;
// the indirection keeps OrderPaymentService free of a hard import on the
// finance application package.
type FinanceReleaseRecorder interface {
	RecordOrderRelease(
		ctx context.Context,
		tx db.Tx,
		orderID uuid.UUID,
		sellerID uuid.UUID,
		gross int64,
		commission int64,
		sellerNet int64,
	) error
}

// SystemRefundReason values for platform-initiated gateway refunds. Mirrors
// the refund-entity reason enum without importing the entity package directly.
const (
	SystemRefundReasonDispute        = "other"
	SystemRefundReasonTimeout        = "other"
	SystemRefundReasonPaymentExpired = "other"
	SystemRefundReasonAdminManual    = "other"
)

// GatewayRefundInitiator is the minimal RefundService surface required by
// OrderPaymentService to dispatch a canonical gateway-aware system refund.
// Implemented by *refund/application.RefundService.CreateAndDispatchSystemRefund;
// the indirection keeps OrderPaymentService free of a hard import on the
// finance/refund package.
//
// The contract:
//   - synchronous: dispatches the gateway refund inside the caller's tx
//   - idempotent on idempotencyKey: replays return the existing refund row
//   - on dispatch failure: returns a non-nil error AND leaves the refund row
//     at gateway_status=failed so retries are visible
//   - on success: refund row is at gateway_status=pending; canonical ledger
//     reversal + escrow flip happen later via the webhook ack pipeline
type GatewayRefundInitiator interface {
	CreateAndDispatchSystemRefundFlat(
		ctx context.Context,
		tx db.Tx,
		orderID uuid.UUID,
		buyerID uuid.UUID,
		sellerID uuid.UUID,
		adminID uuid.UUID,
		productAmount int64,
		shippingAmount int64,
		pd int64,
		s int64,
		c int64,
		k int64,
		reason string,
		idempotencyKey string,
	) error
}

// FINANCIAL AUTHORITY: WalletService is the ONLY authority for money movement
// All escrow operations MUST go through WalletService
type OrderPaymentService struct {
	walletService   *walletApp.WalletService

	// financeRecorder is set after construction via SetFinanceReleaseRecorder.
	// It is required by ReleaseGatewayEscrowToSeller and only that method;
	// other methods on this service work without it.
	financeRecorder FinanceReleaseRecorder

	// gatewayRefundInitiator dispatches platform-initiated gateway refunds.
	// Wired via SetGatewayRefundInitiator. Required by every PAID refund
	// path: when unset, InitiateGatewayRefundForOrder fails closed instead
	// of pretending a refund was issued.
	gatewayRefundInitiator GatewayRefundInitiator

	// coinsSpendReader resolves the canonical coins spent (K) for an order
	// from the coins domain (coins_transactions), NOT from orders.coins_used
	// (dead, never persisted). Wired via SetCoinsSpendReader; nil-safe (K=0).
	coinsSpendReader CoinsSpendReader
}

// CoinsSpendReader is the minimal coins-domain surface needed to resolve K
// (coins actually redeemed for an order). The canonical implementation is
// coinsRepo.FindSpendByReference against coins_transactions
// (reference_type='order_spend', reference_id=order_id), matching the worker's
// CoinsRefundRequiredHandler.findSpendTransaction. K is a coin-subsystem
// concern and must never be read from orders.coins_used.
type CoinsSpendReader interface {
	FindSpendByReference(ctx context.Context, tx db.Tx, userID uuid.UUID, referenceID uuid.UUID) (*coinsentity.CoinsTransaction, error)
}

// SetCoinsSpendReader wires the canonical K reader. Optional: when unset,
// K resolves to 0 (no coins redeemed), which is correct for orders paid
// without coins.
func (s *OrderPaymentService) SetCoinsSpendReader(r CoinsSpendReader) {
	s.coinsSpendReader = r
}

// coinsSpendForOrder returns the canonical coins redeemed (K) for an order by
// reading the coins-domain spend transaction. Returns 0 when the reader is
// unset or no spend exists. Errors are returned so a real DB failure is not
// silently treated as "no coins".
func (s *OrderPaymentService) coinsSpendForOrder(ctx context.Context, tx db.Tx, userID, orderID uuid.UUID) (int64, error) {
	if s.coinsSpendReader == nil {
		return 0, nil
	}
	spend, err := s.coinsSpendReader.FindSpendByReference(ctx, tx, userID, orderID)
	if err != nil {
		return 0, err
	}
	if spend == nil {
		return 0, nil
	}
	return spend.Amount, nil
}

// NewOrderPaymentService creates a new OrderPaymentService.
func NewOrderPaymentService(walletService *walletApp.WalletService) *OrderPaymentService {
	return &OrderPaymentService{
		walletService:   walletService,
	}
}

// SetFinanceReleaseRecorder installs the finance-domain ledger recorder used
// by ReleaseGatewayEscrowToSeller. Call this once during dependency wiring.
// Safe to leave unset only if the gateway-aware release path is not used.
func (s *OrderPaymentService) SetFinanceReleaseRecorder(r FinanceReleaseRecorder) {
	s.financeRecorder = r
}

// SetGatewayRefundInitiator wires the RefundService dependency used to
// dispatch platform-initiated gateway refunds. Required by every PAID
// refund path (dispute / timeout / manual / expire-with-escrow). When
// unset, InitiateGatewayRefundForOrder fails closed.
func (s *OrderPaymentService) SetGatewayRefundInitiator(g GatewayRefundInitiator) {
	s.gatewayRefundInitiator = g
}

// ErrGatewayRefundInitiatorNotConfigured is returned by
// InitiateGatewayRefundForOrder when the dependency has not been wired.
var ErrGatewayRefundInitiatorNotConfigured = fmt.Errorf("gateway refund initiator not configured")

// InitiateGatewayRefundForOrder dispatches a canonical gateway refund for a
// PAID order whose escrow is still in holding state. It MUST be called by
// every platform-initiated refund decision (dispute resolution, timeout,
// expire-with-escrow, manual admin) BEFORE any wallet primitive flips
// escrow.status, so the gateway-side reversal is in flight when the local
// state moves.
//
// The refund row is created (or located by idempotency key) and the gateway
// API is called synchronously. On dispatch failure the surrounding tx MUST
// be aborted by the caller (return the error up); the local state never
// pretends a refund happened when the gateway refused.
//
// systemUserID is the platform identity recorded as the admin on the refund
// row (typically auth.SystemCallerID). idempotencyKey MUST be deterministic
// per (orderID, trigger) so replays converge to a single refund row.
func (s *OrderPaymentService) InitiateGatewayRefundForOrder(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
	systemUserID uuid.UUID,
	refundAmount int64,
	reason string,
	idempotencyKey string,
) error {
	if s.gatewayRefundInitiator == nil {
		return ErrGatewayRefundInitiatorNotConfigured
	}
	// REFUND ECONOMICS REBASE: buyer cash refund excludes commission (C) and
	// payment fee (F). Pass the pricing components (PD, S, C, K) so the
	// gateway refund pipeline can compute proportional commission reversal
	// and coin restoration at ack time.
	// TODO(refund-economics): legacy single-amount callers pass the whole
	// refundAmount as productAmount with shippingAmount=0; they will be
	// migrated to explicit product/shipping split amounts.
	// CANONICAL SNAPSHOT DERIVATION: PD = TotalBeforeCoins - S where
	// TotalBeforeCoins = (P-D)+S is the persisted, token-validated buyer
	// funding base. orders.discount_amount is NOT authoritative (never
	// persisted). K is resolved from the coins domain, never from
	// orders.coins_used.
	pd := order.TotalBeforeCoinsAmount.Int64() - order.ShippingTotal.Int64()
	if pd <= 0 {
		pd = order.Subtotal.Int64()
	}
	sVal := order.ShippingTotal.Int64()
	cVal := order.CommissionAmount.Int64()
	kVal, err := s.coinsSpendForOrder(ctx, tx, order.BuyerID, order.ID)
	if err != nil {
		return fmt.Errorf("resolve coins spent: %w", err)
	}
	return s.gatewayRefundInitiator.CreateAndDispatchSystemRefundFlat(
		ctx, tx,
		order.ID,
		order.BuyerID,
		order.SellerID,
		systemUserID,
		refundAmount, // productAmount (legacy single-amount callers)
		0,            // shippingAmount (legacy single-amount callers)
		pd,
		sVal,
		cVal,
		kVal,
		reason,
		idempotencyKey,
	)
}

// ============================================================================
// ESCROW OPERATIONS - WALLET SERVICE REDIRECT
// ============================================================================
// All escrow operations are now handled by WalletService, which is the
// SINGLE SOURCE OF TRUTH for all money operations.
//
// FINANCIAL AUTHORITY (gateway-funded model):
// - Order domain = pricing snapshot only
// - Wallet domain owns escrow row state (holding / released / refunded)
// - Finance ledger owns money state (GATEWAY_CLEARING, SELLER_PAYABLE, etc.)
// - Buyer/seller wallet balances are NOT touched by escrow lifecycle — money
//   physically lives at the platform clearing account.
// ============================================================================

// RefundToBuyer flips the order's escrow to "refunded" without any wallet
// balance mutation. Gateway-side refund issuance (Midtrans) and the matching
// ledger reversal are orchestrated separately via the refund pipeline
// (RefundService.InitiateGatewayRefund + HandleGatewayRefundAck →
// FinanceService.RecordRefundReversal).
//
// Idempotent: if the escrow is already refunded, returns success.
func (s *OrderPaymentService) RefundToBuyer(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) error {
	_, _, err := s.walletService.RefundGatewayEscrow(ctx, tx, order.ID)
	return err
}

// PartialRefundLedger flips the order's escrow to "released" (terminal)
// when a portion is refunded to the buyer and the remainder released to the
// seller (e.g., partial dispute resolution). No wallet balance mutation;
// ledger entries are written separately via the refund pipeline + order
// release ledger.
//
// NOT idempotent against re-execution that changes the split: callers must
// commit a single partial-refund decision.
func (s *OrderPaymentService) PartialRefundLedger(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
	refundAmount money.Money,
) error {
	_, _, err := s.walletService.PartialRefundGatewayEscrow(ctx, tx, order.ID, refundAmount.Int64())
	return err
}

// ReleaseSummary describes the amounts moved by a gateway-aware release.
type ReleaseSummary struct {
	OrderID       uuid.UUID
	SellerID      uuid.UUID
	Gross         int64
	Commission    int64
	SellerNet     int64
	NewlyReleased bool // false if the escrow was already released (idempotent)
}

// ReleaseGatewayEscrowToSeller performs the canonical gateway-aware release.
//
// Flow (single tx, caller-owned):
//  1. Compute gross/commission/sellerNet from the order pricing snapshot.
//  2. Lock + validate + flip escrow.status via WalletService.ReleaseGatewayEscrow.
//  3. Book finance ledger via FinanceService.RecordOrderRelease (idempotent
//     via UNIQUE idempotency_key="order_release_<order_id>").
//
// IDEMPOTENCY: both the wallet escrow flip and the ledger write are
// idempotent; replays are no-ops.
//
// Wallet balances (buyer.* and seller.*) are NOT touched. The seller's
// withdrawable surface is financial_accounts[SELLER_PAYABLE].
func (s *OrderPaymentService) ReleaseGatewayEscrowToSeller(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) (*ReleaseSummary, error) {
	if s.financeRecorder == nil {
		return nil, fmt.Errorf("ReleaseGatewayEscrowToSeller: finance recorder not configured")
	}

	// CANONICAL ESCROW/RELEASE GROSS: gross = total_before_coins_amount = PD + S.
	// Commission C is a seller/platform-side allocation carved out of the
	// buyer-funded pool: sellerNet = gross - C, and the ledger invariant
	// sellerNet + commission == gross holds. The rejected model (gross =
	// P+S+C, treating commission as buyer-funded cash) must not fund the
	// release. The escrow row was created with this same canonical amount at
	// finalization (CreateEscrowFromGatewaySettlement), so ReleaseGatewayEscrow's
	// amount-match guard stays consistent.
	gross := order.TotalBeforeCoinsAmount.Int64()
	if gross <= 0 {
		// Legacy rows / test fixtures without a persisted buyer base: fall
		// back to the previous product+shipping+commission gross so the
		// release remains functional for pre-convergence rows.
		gross = order.Subtotal.Int64() + order.ShippingTotal.Int64() + order.CommissionAmount.Int64()
	}
	commission := order.CommissionAmount.Int64()
	if commission > gross {
		return nil, fmt.Errorf("ReleaseGatewayEscrowToSeller: commission %d exceeds funded gross %d", commission, gross)
	}
	sellerNet := gross - commission

	// Wallet half: lock escrow, validate state/amount, flip status.
	_, newly, err := s.walletService.ReleaseGatewayEscrow(ctx, tx, order.ID, gross)
	if err != nil {
		return nil, err
	}

	// Finance half: write ledger transaction. Idempotent via UNIQUE key,
	// so calling on an already-released escrow is a safe no-op.
	if err := s.financeRecorder.RecordOrderRelease(
		ctx, tx, order.ID, order.SellerID, gross, commission, sellerNet,
	); err != nil {
		return nil, err
	}

	return &ReleaseSummary{
		OrderID:       order.ID,
		SellerID:      order.SellerID,
		Gross:         gross,
		Commission:    commission,
		SellerNet:     sellerNet,
		NewlyReleased: newly,
	}, nil
}

// PartialRefundEscrow flips the escrow to "released" (terminal) for partial
// split dispute resolution: buyer is refunded item price, seller is released
// shipping. No wallet balance mutation; ledger entries are written elsewhere.
func (s *OrderPaymentService) PartialRefundEscrow(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	refundAmount int64,
) (*walletEntity.Escrow, error) {
	escrow, _, err := s.walletService.PartialRefundGatewayEscrow(ctx, tx, orderID, refundAmount)
	return escrow, err
}
