// âš ï¸ FINANCIAL RULE:
// All money operations MUST go through WalletService.
// Direct balance mutation is forbidden.
//
// âš ï¸ Finance domain is NOT financial authority.
// It is ONLY for billing ledger and reporting.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	billingentity "github.com/labuda/backend/internal/finance/billing/entity"
	ledgerrepoimpl "github.com/labuda/backend/internal/finance/infrastructure/repository"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// FinanceService is the SINGLE ENTRY POINT for all ledger operations.
//
// ============================================================================
// FINANCIAL AUTHORITY RESTORATION
// ============================================================================
// After LOCKDOWN phase, only finance domain may access ledger.
// All other domains MUST route their financial operations through this service.
//
// ARCHITECTURAL RULE:
// - FinanceService owns ALL ledger operations
// - No other domain may call ledgerRepo.CreateTransaction() directly
// - All money flows: DOMAIN â†’ FinanceService â†’ Ledger
//
// Financial operations handled:
// - Escrow creation (payment settlement)
// - Escrow release to seller (order completion)
// - Refunds to buyer
// - Billing revenue recording
// - Seller earnings queries
// ============================================================================
type FinanceService struct {
	ledgerRepo        ledgerepo.LedgerRepository
	disputeFreezeRepo DisputeFreezeWriter
	logger            *zap.Logger
}

// NewFinanceService creates a new FinanceService.
func NewFinanceService() *FinanceService {
	return &FinanceService{
		ledgerRepo: ledgerrepoimpl.NewLedgerRepository(),
		logger:     zap.NewNop(),
	}
}

// DisputeFreezeWriter is the minimal dispute_freeze persistence surface used by
// FinanceService to track dispute freeze bookkeeping. Defined as an interface
// so tests can inject a fake without a real DB.
//
// The concrete implementation is
// internal/finance/infrastructure/repository.DisputeFreezeRepository.
type DisputeFreezeWriter interface {
	Create(ctx context.Context, tx db.Tx, freeze *ledgerepo.DisputeFreeze) error
	Release(ctx context.Context, tx db.Tx, disputeID uuid.UUID) error
	ReleaseByOrderID(ctx context.Context, tx db.Tx, orderID uuid.UUID) error
	GetTotalActiveBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (int64, error)
}

// SetDisputeFreezeRepo wires the dispute_freeze repository used by
// CreateDisputeFreeze, ReleaseDisputeFreeze, and AssertSellerWithdrawalAllowed.
// When unset the dispute-freeze surface is a no-op (safe for old deployments
// before migration 000131 is applied).
func (s *FinanceService) SetDisputeFreezeRepo(repo DisputeFreezeWriter) {
	s.disputeFreezeRepo = repo
}

// SetLogger wires a structured logger for finance observability events.
// Optional: callers that omit it get a no-op logger and lose only the
// structured-log surface (correctness is unchanged).
func (s *FinanceService) SetLogger(logger *zap.Logger) {
	if logger != nil {
		s.logger = logger
	}
}

// ============================================================================
// BILLING - SERVICE REVENUE
// ============================================================================

// RecordBillingServiceRevenue creates ledger entries for service billing payments.
//
// Called by: BillingService.MarkPaid() for promotion packages
//
// For promotion packages: full amount goes to platform revenue immediately.
// These are one-time service purchases with no escrow holding.
//
// Billing/subscription payments do NOT flow through RecordGatewayPaymentSettlement
// (which funds GATEWAY_CLEARING). Instead, the combined settlement + 100% commission
// effect is recorded as a single transfer from BANK_SETTLEMENT to PLATFORM_REVENUE.
//
// Ledger entries (Î£ entries = 0 invariant):
// - Debit:  PLATFORM_REVENUE (+gross) â€” platform keeps full amount
// - Credit: BANK_SETTLEMENT  (-gross) â€” reserve drains (mirrors gateway settlement)
func (s *FinanceService) RecordBillingServiceRevenue(
	ctx context.Context,
	tx db.Tx,
	billing *billingentity.BillingTransaction,
) error {
	// Get system account IDs
	platformRevenueAccount, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, ledgerepo.AccountPlatformRevenue)
	if err != nil {
		return fmt.Errorf("get platform revenue account: %w", err)
	}

	bankSettlementAccount, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, ledgerepo.AccountBankSettlement)
	if err != nil {
		return fmt.Errorf("get bank settlement account: %w", err)
	}

	// Build idempotency key
	idempotencyKey := fmt.Sprintf("billing-%s", billing.ID)

	// Build ledger entries: DR PLATFORM_REVENUE / CR BANK_SETTLEMENT
	// Mirrors RecordSubscriptionRevenue â€” full amount to platform, no escrow
	entries := []ledgerepo.Entry{
		{AccountID: platformRevenueAccount, Amount: billing.GrossAmount},      // DR +gross (revenue increases)
		{AccountID: bankSettlementAccount, Amount: billing.GrossAmount.Neg()}, // CR -gross (reserve drains)
	}

	if err := s.ledgerRepo.CreateTransaction(ctx, tx, idempotencyKey, "billing", billing.ID, nil, nil, entries); err != nil {
		return fmt.Errorf("create ledger transaction: %w", err)
	}

	return nil
}

// ============================================================================
// SELLER EARNINGS QUERY
// ============================================================================

// GetSellerTotalEarnings returns the total amount ever credited to seller's payable account.
//
// Called by: SellerHandler.GetEarnings()
//
// This includes all completed orders where funds were released to the seller.
func (s *FinanceService) GetSellerTotalEarnings(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (int64, error) {
	total, err := s.ledgerRepo.GetTotalCreditToUserAccount(ctx, tx, ledgerepo.AccountSellerPayable, sellerID)
	if err != nil {
		return 0, fmt.Errorf("get seller earnings: %w", err)
	}
	return total, nil
}

// ============================================================================
// ORDER RELEASE - GATEWAY-FUNDED ESCROW
// ============================================================================

// RecordOrderRelease books the finance ledger transaction for a gateway-funded
// escrow release. This is the canonical money mutation when an order moves from
// "paid + holding" to "completed + released".
//
// Caller responsibilities:
//   - escrow row already locked FOR UPDATE and validated (status=holding)
//     by WalletService.ReleaseGatewayEscrow
//   - all amounts are Rupiah integers (PASS_18H canonical unit, no cents/sen);
//     sellerNet + commission MUST equal gross
//   - tx is the same DB transaction used to update escrow.status / order.status
//
// Ledger movements (Î£ entries = 0 invariant):
//   - GATEWAY_CLEARING balance -= gross   (entry amount = -gross)
//   - SELLER_PAYABLE[seller]   += sellerNet (entry amount = +sellerNet)
//   - PLATFORM_REVENUE         += commission (entry amount = +commission)
//
// Buyer accounts and any wallet.* balances are NOT touched. Seller's
// withdrawable surface is financial_accounts[SELLER_PAYABLE], not wallet.
//
// IDEMPOTENCY: idempotency_key = "order_release_<order_id>". Repeat calls
// (worker retry, admin retry, race with auto-complete) are no-ops at the
// ledger layer regardless of whether the caller's outer guard fired.
func (s *FinanceService) RecordOrderRelease(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	sellerID uuid.UUID,
	gross int64,
	commission int64,
	sellerNet int64,
) error {
	if gross < 0 || commission < 0 || sellerNet < 0 {
		return fmt.Errorf("RecordOrderRelease: amounts must be non-negative (gross=%d commission=%d sellerNet=%d)", gross, commission, sellerNet)
	}
	if sellerNet+commission != gross {
		return fmt.Errorf("RecordOrderRelease: sellerNet (%d) + commission (%d) must equal gross (%d)", sellerNet, commission, gross)
	}

	gatewayClearingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, ledgerepo.AccountGatewayClearing)
	if err != nil {
		return fmt.Errorf("get gateway clearing account: %w", err)
	}
	platformRevenueID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, ledgerepo.AccountPlatformRevenue)
	if err != nil {
		return fmt.Errorf("get platform revenue account: %w", err)
	}
	sellerPayableID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, ledgerepo.AccountSellerPayable, sellerID)
	if err != nil {
		return fmt.Errorf("get/create seller payable account: %w", err)
	}

	// Sign convention (verified against finance/infrastructure/repository/ledger_repository.go:160):
	// newBalance = oldBalance + entry.Amount.
	// â†’ positive amount increases balance, negative amount decreases.
	// At release, GATEWAY_CLEARING is drained into SELLER_PAYABLE + PLATFORM_REVENUE.
	entries := []ledgerepo.Entry{
		{AccountID: gatewayClearingID, Amount: money.New(-gross)},
		{AccountID: sellerPayableID, Amount: money.New(sellerNet)},
		{AccountID: platformRevenueID, Amount: money.New(commission)},
	}

	idempotencyKey := fmt.Sprintf("order_release_%s", orderID.String())
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "order_release", orderID, &orderID, nil, entries,
	); err != nil {
		return fmt.Errorf("record order release ledger: %w", err)
	}

	return nil
}

// ============================================================================
// SUBSCRIPTION REVENUE
// ============================================================================

// RecordSubscriptionRevenue creates ledger entries for seller subscription payments.
//
// Called by: SubscriptionPaymentService.ProcessSuccessfulPayment()
//
// Full subscription fee goes to platform revenue immediately.
// These are upfront payments with no escrow holding.
//
// Subscription payments do NOT flow through RecordGatewayPaymentSettlement.
// See RecordBillingServiceRevenue for rationale.
//
// Ledger entries (Î£ entries = 0 invariant):
// - Debit:  PLATFORM_REVENUE (+amount) â€” platform keeps full amount
// - Credit: BANK_SETTLEMENT  (-amount) â€” reserve drains
func (s *FinanceService) RecordSubscriptionRevenue(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	amount int64,
	providerEventID string,
) error {
	// Get system account IDs
	platformRevenueID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, ledgerepo.AccountPlatformRevenue)
	if err != nil {
		return fmt.Errorf("get platform revenue account: %w", err)
	}

	bankSettlementID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, ledgerepo.AccountBankSettlement)
	if err != nil {
		return fmt.Errorf("get bank settlement account: %w", err)
	}

	// Build idempotency key
	idempotencyKey := fmt.Sprintf("seller_subscription_payment_%s", providerEventID)

	// Build ledger entries
	// DR PLATFORM_REVENUE (positive = debit, revenue increases)
	// CR BANK_SETTLEMENT  (negative = credit, reserve drains)
	entries := []ledgerepo.Entry{
		{AccountID: platformRevenueID, Amount: money.New(amount)}, // DR +amount
		{AccountID: bankSettlementID, Amount: money.New(-amount)}, // CR -amount
	}

	if err := s.ledgerRepo.CreateTransaction(ctx, tx, idempotencyKey, "seller_subscription_payment", paymentID, nil, nil, entries); err != nil {
		return fmt.Errorf("create subscription revenue transaction: %w", err)
	}

	return nil
}

// ============================================================================
// PAYMENT SETTLEMENT FUNDING (TASK 39E)
// ============================================================================

// RecordGatewayPaymentSettlement books the canonical double-entry that funds
// GATEWAY_CLEARING when an external gateway (Midtrans) confirms a payment
// settlement. This is the missing inflow side that pairs with
// RecordOrderRelease's outflow at order completion.
//
// Caller responsibilities:
//   - payment row already locked / status transitioned via
//     PaymentSettlementService.SettlePaymentByID in the SAME db.Tx
//   - escrow row not yet created (this MUST run before
//     WalletService.CreateEscrowFromGatewaySettlement)
//   - gross is the payment.GrossAmount as a Rupiah integer (>=0), Labuda's
//     canonical money unit (PASS_18H) — no cents/sen subunit
//   - providerTransactionID is the gateway-issued transaction id (Midtrans
//     `transaction_id` from the webhook notification â€” used for idempotency)
//
// Ledger movements (Î£ entries = 0 invariant):
//   - GATEWAY_CLEARING balance += gross   (entry amount = +gross, debit)
//   - BANK_SETTLEMENT  balance -= gross   (entry amount = -gross, credit)
//
// BANK_SETTLEMENT was seeded with a large reserve float at bootstrap so its
// post-credit balance stays non-negative. No wallet balances are touched;
// no buyer/seller financial_accounts rows are touched.
//
// IDEMPOTENCY:
//   - idempotency_key = "payment_settlement_<provider_transaction_id>"
//   - Duplicate webhook for the same gateway transaction â†’ ledger CreateTransaction
//     no-ops (returns nil) and we emit finance_payment_settlement_duplicate_ignored.
//   - Replay safety: the upstream "bulletproof guard" in payment_webhook.go
//     also prevents re-entry once payment.status moved off pending; this method
//     is the second line of defense at the ledger layer.
func (s *FinanceService) RecordGatewayPaymentSettlement(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	orderID uuid.UUID,
	providerTransactionID string,
	gross int64,
) error {
	if paymentID == uuid.Nil {
		return fmt.Errorf("RecordGatewayPaymentSettlement: payment_id required")
	}
	if orderID == uuid.Nil {
		return fmt.Errorf("RecordGatewayPaymentSettlement: order_id required")
	}
	if providerTransactionID == "" {
		return fmt.Errorf("RecordGatewayPaymentSettlement: provider_transaction_id required")
	}
	if gross <= 0 {
		return fmt.Errorf("RecordGatewayPaymentSettlement: gross must be positive (got %d)", gross)
	}

	// Use canonical uppercase account-type strings (the same values the
	// system_account_bootstrap inserts into financial_accounts.account_type).
	gatewayClearingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountGatewayClearing)
	if err != nil {
		return fmt.Errorf("get gateway clearing account: %w", err)
	}
	bankSettlementID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountBankSettlement)
	if err != nil {
		return fmt.Errorf("get bank settlement account: %w", err)
	}

	idempotencyKey := fmt.Sprintf("payment_settlement_%s", providerTransactionID)

	// Pre-check for duplicate so we can emit the right structured log.
	// Functionally optional â€” CreateTransaction is itself idempotent â€” but
	// distinguishes the two observability cases the task spec requires.
	alreadyRecorded, err := s.ledgerRepo.CountTransactionsByEntityID(ctx, tx, paymentID)
	if err != nil {
		return fmt.Errorf("ledger duplicate-check failed: %w", err)
	}

	entries := []ledgerepo.Entry{
		{AccountID: gatewayClearingID, Amount: money.New(gross)}, // DR +gross
		{AccountID: bankSettlementID, Amount: money.New(-gross)}, // CR -gross
	}

	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "payment_settlement", paymentID, &orderID, &paymentID, entries,
	); err != nil {
		return fmt.Errorf("record gateway payment settlement ledger: %w", err)
	}

	// CountTransactionsByEntityID returned a count for THIS payment_id used as
	// the reference_id. If a settlement row already existed before this call,
	// CreateTransaction was a no-op and we treat this as a duplicate.
	if alreadyRecorded > 0 {
		s.logger.Info("finance_payment_settlement_duplicate_ignored",
			zap.String("payment_id", paymentID.String()),
			zap.String("order_id", orderID.String()),
			zap.String("provider_transaction_id", providerTransactionID),
			zap.Int64("gross_amount", gross),
			zap.String("idempotency_key", idempotencyKey),
		)
		return nil
	}

	s.logger.Info("finance_payment_settlement_recorded",
		zap.String("payment_id", paymentID.String()),
		zap.String("order_id", orderID.String()),
		zap.String("provider_transaction_id", providerTransactionID),
		zap.Int64("gross_amount", gross),
		zap.String("idempotency_key", idempotencyKey),
	)
	return nil
}

// RecordBuyerPaymentFeeRevenue realizes the buyer payment method fee
// (PASS_18V) as platform revenue at the moment the gateway payment settles.
//
// Context: RecordGatewayPaymentSettlement funds GATEWAY_CLEARING with the
// FULL gross amount (seller-side escrow + buyer payment fee), but only the
// escrow-equivalent portion is ever drained back out later, at release
// (ReleaseGatewayEscrowToSeller/RecordOrderRelease) or refund. Without this
// step the buyer fee portion would sit in GATEWAY_CLEARING forever —
// unaccounted, and never recognized as revenue.
//
// This call MUST run in the same tx as RecordGatewayPaymentSettlement, for
// every order payment, immediately after it. The buyer payment fee is
// non-refundable platform revenue regardless of what later happens to the
// order (refund/dispute/completion) — see PASS_18V report for the explicit
// refund-policy rationale — so recognizing it at settlement (rather than at
// order release, like commission) is correct: it is earned the moment the
// buyer successfully pays via the chosen method, not when the order ships.
//
// Ledger movements (Σ entries = 0 invariant):
//   - GATEWAY_CLEARING balance -= buyerPaymentFee (entry amount = -fee, credit)
//   - PLATFORM_REVENUE  balance += buyerPaymentFee (entry amount = +fee, debit)
//
// A zero fee (e.g. a hypothetical free method) is a no-op, not an error.
//
// IDEMPOTENCY: idempotency_key = "payment_fee_revenue_<payment_id>". Safe to
// call on webhook replay — ledgerRepo.CreateTransaction no-ops on duplicate.
func (s *FinanceService) RecordBuyerPaymentFeeRevenue(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	orderID uuid.UUID,
	buyerPaymentFee int64,
) error {
	if paymentID == uuid.Nil {
		return fmt.Errorf("RecordBuyerPaymentFeeRevenue: payment_id required")
	}
	if orderID == uuid.Nil {
		return fmt.Errorf("RecordBuyerPaymentFeeRevenue: order_id required")
	}
	if buyerPaymentFee < 0 {
		return fmt.Errorf("RecordBuyerPaymentFeeRevenue: buyer payment fee must not be negative (got %d)", buyerPaymentFee)
	}
	if buyerPaymentFee == 0 {
		return nil
	}

	gatewayClearingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountGatewayClearing)
	if err != nil {
		return fmt.Errorf("get gateway clearing account: %w", err)
	}
	platformRevenueID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformRevenue)
	if err != nil {
		return fmt.Errorf("get platform revenue account: %w", err)
	}

	idempotencyKey := fmt.Sprintf("payment_fee_revenue_%s", paymentID.String())

	entries := []ledgerepo.Entry{
		{AccountID: gatewayClearingID, Amount: money.New(-buyerPaymentFee)}, // CR -fee
		{AccountID: platformRevenueID, Amount: money.New(buyerPaymentFee)},  // DR +fee
	}

	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "payment_fee_revenue", paymentID, &orderID, &paymentID, entries,
	); err != nil {
		return fmt.Errorf("record buyer payment fee revenue ledger: %w", err)
	}

	s.logger.Info("finance_buyer_payment_fee_revenue_recorded",
		zap.String("payment_id", paymentID.String()),
		zap.String("order_id", orderID.String()),
		zap.Int64("buyer_payment_fee", buyerPaymentFee),
		zap.String("idempotency_key", idempotencyKey),
	)
	return nil
}

// ============================================================================
// COIN FUNDING — PLATFORM-FUNDED BUYER BENEFIT (K)
// ============================================================================
//
// RecordCoinFunding books the canonical platform funding of K (coins redeemed
// by the buyer) into GATEWAY_CLEARING so the seller's economic entitlement
// (BuyerBase = PD + S) can be released without overdrawing the clearing
// account.
//
// LOCKED BUSINESS CONTRACT:
//   - Buyer cash payment = BuyerBase - K + F (the buyer paid K in coins).
//   - GATEWAY_CLEARING after settlement + fee sweep = BuyerBase - K.
//   - Seller economic entitlement = BuyerBase = PD + S.
//   - K is a PLATFORM-FUNDED BUYER BENEFIT — the platform absorbs K.
//   - K is NOT gateway cash and NEVER becomes PLATFORM_REVENUE.
//
// Ledger movement (Σ entries = 0 invariant):
//   - DR PLATFORM_BANK    -K   (platform's own bank holdings fund the benefit)
//   - CR GATEWAY_CLEARING +K   (clearing now holds BuyerBase, fully funding the
//                               seller entitlement at release)
//
// PLATFORM_BANK is the established account for the platform's own money
// leaving the platform (see withdrawal payouts: WITHDRAWAL_COMMITTED →
// PLATFORM_BANK). Funding a buyer benefit is the same economic direction:
// platform money funding a real obligation. K is never recorded as revenue.
//
// IDEMPOTENCY: idempotency_key = "coin_funding_<payment_id>". Replays
// (duplicate settlement webhook, retry) are no-ops at the ledger layer.
//
// CALLER: CanonicalFinalizationService.FinalizeOrderPayment, in the same tx
// as payment settlement, coin consume/spend, escrow creation, and order paid.
func (s *FinanceService) RecordCoinFunding(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	orderID uuid.UUID,
	amount int64,
) error {
	if paymentID == uuid.Nil {
		return fmt.Errorf("RecordCoinFunding: payment_id required")
	}
	if orderID == uuid.Nil {
		return fmt.Errorf("RecordCoinFunding: order_id required")
	}
	if amount <= 0 {
		return fmt.Errorf("RecordCoinFunding: amount must be positive (got %d)", amount)
	}

	platformBankID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformBank)
	if err != nil {
		return fmt.Errorf("RecordCoinFunding: get platform bank account: %w", err)
	}
	gatewayClearingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountGatewayClearing)
	if err != nil {
		return fmt.Errorf("RecordCoinFunding: get gateway clearing account: %w", err)
	}

	// Sign convention (matches RecordOrderRelease):
	// newBalance = oldBalance + entry.Amount.
	//   - PLATFORM_BANK -K: platform money funds the benefit (bank holdings decrease)
	//   - GATEWAY_CLEARING +K: clearing gains the funded amount
	entries := []ledgerepo.Entry{
		{AccountID: platformBankID, Amount: money.New(-amount)},
		{AccountID: gatewayClearingID, Amount: money.New(amount)},
	}

	idempotencyKey := fmt.Sprintf("coin_funding_%s", paymentID.String())
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "coin_funding", paymentID, &orderID, &paymentID, entries,
	); err != nil {
		return fmt.Errorf("record coin funding ledger: %w", err)
	}

	s.logger.Info("finance_coin_funding_recorded",
		zap.String("payment_id", paymentID.String()),
		zap.String("order_id", orderID.String()),
		zap.Int64("amount", amount),
		zap.String("idempotency_key", idempotencyKey),
	)
	return nil
}

// RecordCoinFundingReversal reverses platform funding of K for the portion of
// coins restored to the buyer on refund.
//
// When an order that redeemed coins (K) is refunded, the buyer's coins are
// restored via the coins domain (coins.refund_required → refund_earn). The
// corresponding platform funding of K in GATEWAY_CLEARING is no longer needed
// to fund the seller's (refunded) entitlement, so it is returned to
// PLATFORM_BANK.
//
// Ledger movement (Σ entries = 0 invariant):
//   - DR GATEWAY_CLEARING -CoinDelta (funding for the refunded entitlement leaves clearing)
//   - CR PLATFORM_BANK    +CoinDelta (platform recovers the funding)
//
// CoinDelta is the product-proportional coins restored for this refund event
// (refund_math.go: proportionalFloor(K, cumProductAfter, PD) -
// proportionalFloor(K, cumProductBefore, PD)). Reversing exactly CoinDelta
// keeps the ledger in balance:
//
//	full refund:   GATEWAY_CLEARING = B - (B-K) - K = 0; PLATFORM_BANK = -K + K = 0
//	partial refund: GATEWAY_CLEARING = B - CashRefund - CoinDelta (drained by
//	                the remainder release to 0); PLATFORM_BANK = -(K - CoinDelta)
//
// where B = BuyerBase = PD + S.
//
// IDEMPOTENCY: idempotency_key = "coin_funding_reversal_<refund_id>". Replays
// (duplicate refund webhook ack) are no-ops at the ledger layer.
//
// CALLER: RefundService.HandleGatewayRefundAck, in the same tx as
// RecordRefundReversal, before the partial-refund remainder release.
func (s *FinanceService) RecordCoinFundingReversal(
	ctx context.Context,
	tx db.Tx,
	refundID uuid.UUID,
	orderID uuid.UUID,
	amount int64,
) error {
	if refundID == uuid.Nil {
		return fmt.Errorf("RecordCoinFundingReversal: refund_id required")
	}
	if orderID == uuid.Nil {
		return fmt.Errorf("RecordCoinFundingReversal: order_id required")
	}
	if amount <= 0 {
		return nil // No coins restored for this refund event — nothing to reverse.
	}

	gatewayClearingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountGatewayClearing)
	if err != nil {
		return fmt.Errorf("RecordCoinFundingReversal: get gateway clearing account: %w", err)
	}
	platformBankID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformBank)
	if err != nil {
		return fmt.Errorf("RecordCoinFundingReversal: get platform bank account: %w", err)
	}

	entries := []ledgerepo.Entry{
		{AccountID: gatewayClearingID, Amount: money.New(-amount)},
		{AccountID: platformBankID, Amount: money.New(amount)},
	}

	idempotencyKey := fmt.Sprintf("coin_funding_reversal_%s", refundID.String())
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "coin_funding_reversal", refundID, &orderID, nil, entries,
	); err != nil {
		return fmt.Errorf("record coin funding reversal ledger: %w", err)
	}

	s.logger.Info("finance_coin_funding_reversal_recorded",
		zap.String("refund_id", refundID.String()),
		zap.String("order_id", orderID.String()),
		zap.Int64("amount", amount),
		zap.String("idempotency_key", idempotencyKey),
	)
	return nil
}

// ============================================================================
// REFUND REVERSAL â€” PHASE 2B (TASK 41)
// ============================================================================
//
// RecordRefundReversal books the canonical double-entry that reverses a
// settled order on a successful gateway refund webhook (refund.success).
//
// Phase 2B scope: FULL refund only. Two branches:
//
//	BEFORE RELEASE (escrow still in 'holding'):
//	  Money never moved past GATEWAY_CLEARING. Drain it back to BUYER_REFUNDABLE.
//	    DR BUYER_REFUNDABLE[buyer]  +gross
//	    CR GATEWAY_CLEARING         -gross
//	  Seller payable and platform revenue are untouched (they were never
//	  credited at this point).
//
//	AFTER RELEASE with sufficient seller payable:
//	  Money was already drained from GATEWAY_CLEARING into SELLER_PAYABLE +
//	  PLATFORM_REVENUE at order completion. Reverse those two specifically.
//	    DR BUYER_REFUNDABLE[buyer]    +gross
//	    CR SELLER_PAYABLE[seller]     -sellerNet
//	    CR PLATFORM_REVENUE           -commission
//	  gross = sellerNet + commission. Platform forfeits commission on a full
//	  refund. The CHECK constraint financial_accounts.balance >= 0 is the
//	  fail-closed backstop; a pre-check on SELLER_PAYABLE balance surfaces
//	  insufficient-funds as a typed error before the constraint fires.
//
// BUYER_REFUNDABLE semantics:
//
//	This is an ACCOUNTING / AUDIT account. It is NOT a redeemable wallet
//	balance and the buyer cannot spend from it. The buyer's actual money
//	reversal happens at the payment gateway (Midtrans refunds the original
//	instrument). This ledger entry only records that reversal in our books.
//
// IDEMPOTENCY:
//   - idempotency_key = "refund_reversal_<refund_id>"
//   - Replay (Midtrans webhook retry, manual replay) â†’ CreateTransaction
//     no-ops (returns nil) and we emit finance_refund_reversal_duplicate_ignored.
//
// CALLER RESPONSIBILITIES:
//   - refund row already locked FOR UPDATE
//   - order row already locked FOR UPDATE (caller reads pricing snapshot)
//   - escrow already loaded; AfterRelease=true iff escrow.status='released'
//   - sellerNet + commission MUST equal gross
//   - tx is the same DB transaction used by the webhook handler
//
// PHASE 2B INVARIANTS (do NOT cross):
//   - no wallet balance mutation
//   - no escrow status mutation
//   - no order status mutation
//   - legacy seller offset path removed
//   - withdrawal freeze logic untouched
//   - automatic dispute â†’ refund trigger NOT introduced
//
// DEFERRED (out of scope for Phase 2B):
//   - partial refund
//   - shipping refund split
//
// ErrSellerPayableInsufficient is returned for the after-release branch when
// the seller's payable balance cannot fully fund the refund. Caller should
// NOT mutate refund/escrow/order state when this error is returned.
var ErrSellerPayableInsufficient = errors.New("seller payable insufficient for refund reversal")

// RecordRefundReversalInput captures the parameters for booking a refund
// reversal ledger transaction. All amounts are Rupiah integers (PASS_18J) —
// Labuda's canonical money unit, no cents/sen subunit.
type RecordRefundReversalInput struct {
	RefundID            uuid.UUID
	OrderID             uuid.UUID
	BuyerID             uuid.UUID
	SellerID            uuid.UUID
	RefundAmount        int64
	SellerComponent     int64
	CommissionComponent int64
	OrderGross          int64
	CumulativeRefunded  int64
	RoundingAdjustment  int64
	AfterRelease        bool
}

// RecordRefundReversalSummary describes the outcome of RecordRefundReversal.
type RecordRefundReversalSummary struct {
	Phase               string // "before_release" | "after_release"
	Duplicate           bool   // true if this call was a no-op (duplicate webhook)
	RefundAmount        int64
	SellerComponent     int64
	CommissionComponent int64
	CumulativeRefunded  int64
	RoundingAdjustment  int64
	IdempotencyKey      string
}

// RecordRefundReversal â€” see package doc-block above for full semantics.
func (s *FinanceService) RecordRefundReversal(
	ctx context.Context,
	tx db.Tx,
	input RecordRefundReversalInput,
) (*RecordRefundReversalSummary, error) {
	if input.RefundID == uuid.Nil {
		return nil, fmt.Errorf("RecordRefundReversal: refund_id required")
	}
	if input.OrderID == uuid.Nil {
		return nil, fmt.Errorf("RecordRefundReversal: order_id required")
	}
	if input.BuyerID == uuid.Nil {
		return nil, fmt.Errorf("RecordRefundReversal: buyer_id required")
	}
	if input.SellerID == uuid.Nil {
		return nil, fmt.Errorf("RecordRefundReversal: seller_id required")
	}
	if input.RefundAmount <= 0 || input.CommissionComponent < 0 || input.SellerComponent < 0 {
		return nil, fmt.Errorf("RecordRefundReversal: amounts invalid (refund_amount=%d commission_component=%d seller_component=%d)",
			input.RefundAmount, input.CommissionComponent, input.SellerComponent)
	}
	if input.SellerComponent+input.CommissionComponent != input.RefundAmount {
		return nil, fmt.Errorf("RecordRefundReversal: seller_component (%d) + commission_component (%d) must equal refund_amount (%d)",
			input.SellerComponent, input.CommissionComponent, input.RefundAmount)
	}
	if input.OrderGross <= 0 || input.CumulativeRefunded <= 0 || input.CumulativeRefunded > input.OrderGross {
		return nil, fmt.Errorf("RecordRefundReversal: invalid cumulative/order gross (cumulative=%d order_gross=%d)",
			input.CumulativeRefunded, input.OrderGross)
	}

	idempotencyKey := fmt.Sprintf("refund_reversal_%s", input.RefundID.String())
	phase := "before_release"
	if input.AfterRelease {
		phase = "after_release"
	}

	// Pre-check for duplicate so we can emit the right structured log.
	// CreateTransaction is itself idempotent (UNIQUE on idempotency_key),
	// so this is observability-only.
	alreadyRecorded, err := s.ledgerRepo.CountTransactionsByEntityID(ctx, tx, input.RefundID)
	if err != nil {
		return nil, fmt.Errorf("ledger duplicate-check failed: %w", err)
	}

	gatewayClearingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountGatewayClearing)
	if err != nil {
		return nil, fmt.Errorf("get gateway clearing account: %w", err)
	}
	buyerRefundableID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountBuyerRefundable, input.BuyerID)
	if err != nil {
		return nil, fmt.Errorf("get/create buyer refundable account: %w", err)
	}

	var entries []ledgerepo.Entry
	if input.AfterRelease {
		sellerPayableID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountSellerPayable, input.SellerID)
		if err != nil {
			return nil, fmt.Errorf("get seller payable account: %w", err)
		}
		platformRevenueID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformRevenue)
		if err != nil {
			return nil, fmt.Errorf("get platform revenue account: %w", err)
		}

		// Lock seller payable balance and decide branch:
		//   - balance >= sellerNet  â†’ settle the reversal normally
		//   - balance <  sellerNet  â†’ fail closed
		//
		// On replay (alreadyRecorded > 0) the balance has already been net'd
		// down by the prior reversal â€” the original branch decision is still
		// recorded by the existing ledger row, so we don't need to recompute.
		if alreadyRecorded == 0 {
			sellerPayableBal, err := s.ledgerRepo.GetAccountBalanceForUpdate(ctx, tx, sellerPayableID)
			if err != nil {
				return nil, fmt.Errorf("read seller payable balance: %w", err)
			}
			available := sellerPayableBal.Int64()
			if available >= input.SellerComponent {
				// Phase 2B branch â€” payable fully covers reversal.
				entries = []ledgerepo.Entry{
					{AccountID: buyerRefundableID, Amount: money.New(input.RefundAmount)},         // DR +refund
					{AccountID: sellerPayableID, Amount: money.New(-input.SellerComponent)},       // CR -seller
					{AccountID: platformRevenueID, Amount: money.New(-input.CommissionComponent)}, // CR -commission
				}
			} else {
				s.logger.Warn("finance_refund_reversal_seller_payable_insufficient",
					zap.String("refund_id", input.RefundID.String()),
					zap.String("order_id", input.OrderID.String()),
					zap.String("seller_id", input.SellerID.String()),
					zap.Int64("seller_payable_balance", available),
					zap.Int64("required_seller_component", input.SellerComponent),
					zap.String("reason", "insufficient_seller_payable"),
				)
				return nil, ErrSellerPayableInsufficient
			}
		} else {
			// Replay path: re-issue the same shape we (might have) written
			// before. CreateTransaction is idempotent on idempotency_key, so
			// the entry list never reaches the DB â€” we just need a balanced
			// shape so the validator does not panic. Use the Phase 2B shape
			// which is balanced regardless of seller payable state.
			entries = []ledgerepo.Entry{
				{AccountID: buyerRefundableID, Amount: money.New(input.RefundAmount)},
				{AccountID: sellerPayableID, Amount: money.New(-input.SellerComponent)},
				{AccountID: platformRevenueID, Amount: money.New(-input.CommissionComponent)},
			}
		}
		_ = gatewayClearingID
	} else {
		entries = []ledgerepo.Entry{
			{AccountID: buyerRefundableID, Amount: money.New(input.RefundAmount)},  // DR +refund
			{AccountID: gatewayClearingID, Amount: money.New(-input.RefundAmount)}, // CR -refund
		}
	}

	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "refund_reversal", input.RefundID, &input.OrderID, nil, entries,
	); err != nil {
		return nil, fmt.Errorf("record refund reversal ledger: %w", err)
	}

	if alreadyRecorded > 0 {
		s.logger.Info("finance_refund_reversal_duplicate_ignored",
			zap.String("refund_id", input.RefundID.String()),
			zap.String("order_id", input.OrderID.String()),
			zap.String("phase", phase),
			zap.Int64("refund_amount", input.RefundAmount),
			zap.String("idempotency_key", idempotencyKey),
		)
		return &RecordRefundReversalSummary{
			Phase:               phase,
			Duplicate:           true,
			RefundAmount:        input.RefundAmount,
			SellerComponent:     input.SellerComponent,
			CommissionComponent: input.CommissionComponent,
			CumulativeRefunded:  input.CumulativeRefunded,
			RoundingAdjustment:  input.RoundingAdjustment,
			IdempotencyKey:      idempotencyKey,
		}, nil
	}

	if input.AfterRelease {
		s.logger.Info("finance_refund_after_release",
			zap.String("refund_id", input.RefundID.String()),
			zap.String("order_id", input.OrderID.String()),
			zap.String("buyer_id", input.BuyerID.String()),
			zap.String("seller_id", input.SellerID.String()),
			zap.Int64("refund_amount", input.RefundAmount),
			zap.Int64("seller_component", input.SellerComponent),
			zap.Int64("commission_component", input.CommissionComponent),
			zap.Int64("cumulative_refunded", input.CumulativeRefunded),
			zap.Int64("order_gross", input.OrderGross),
			zap.Int64("rounding_adjustment", input.RoundingAdjustment),
			zap.String("phase", phase),
			zap.String("idempotency_key", idempotencyKey),
		)
	} else {
		s.logger.Info("finance_refund_before_release",
			zap.String("refund_id", input.RefundID.String()),
			zap.String("order_id", input.OrderID.String()),
			zap.String("buyer_id", input.BuyerID.String()),
			zap.Int64("refund_amount", input.RefundAmount),
			zap.Int64("cumulative_refunded", input.CumulativeRefunded),
			zap.Int64("order_gross", input.OrderGross),
			zap.Int64("rounding_adjustment", input.RoundingAdjustment),
			zap.String("idempotency_key", idempotencyKey),
		)
	}

	s.logger.Info("finance_refund_reversal_recorded",
		zap.String("refund_id", input.RefundID.String()),
		zap.String("order_id", input.OrderID.String()),
		zap.String("phase", phase),
		zap.Int64("refund_amount", input.RefundAmount),
		zap.Int64("seller_component", input.SellerComponent),
		zap.Int64("commission_component", input.CommissionComponent),
		zap.Int64("cumulative_refunded", input.CumulativeRefunded),
		zap.Int64("order_gross", input.OrderGross),
		zap.Int64("rounding_adjustment", input.RoundingAdjustment),
		zap.String("idempotency_key", idempotencyKey),
	)
	return &RecordRefundReversalSummary{
		Phase:               phase,
		Duplicate:           false,
		RefundAmount:        input.RefundAmount,
		SellerComponent:     input.SellerComponent,
		CommissionComponent: input.CommissionComponent,
		CumulativeRefunded:  input.CumulativeRefunded,
		RoundingAdjustment:  input.RoundingAdjustment,
		IdempotencyKey:      idempotencyKey,
	}, nil
}

// ============================================================================
// PARTIAL REFUND REMAINDER RELEASE (H2-A3)
// ============================================================================

// RecordPartialRefundReleaseInput captures the parameters for booking the
// remainder release after a partial (product-only) refund. The remainder is
// the portion of the order gross that was NOT refunded to the buyer â€” this
// amount must flow from GATEWAY_CLEARING to SELLER_PAYABLE + PLATFORM_REVENUE.
type RecordPartialRefundReleaseInput struct {
	RefundID   uuid.UUID
	OrderID    uuid.UUID
	SellerID   uuid.UUID
	Remainder  int64 // orderGross - cumulativeRefunded
	SellerNet  int64 // remainder - commission on remainder
	Commission int64 // proportional commission on remainder
}

// RecordPartialRefundRelease books the remainder release ledger entries after
// a partial refund. This is the seller's shipping fee (minus proportional
// commission) being released from GATEWAY_CLEARING.
//
// Ledger entries:
//   - GATEWAY_CLEARING  -= remainder  (drain remaining clearing balance)
//   - SELLER_PAYABLE    += sellerNet  (seller receives shipping minus commission)
//   - PLATFORM_REVENUE  += commission (platform keeps commission on shipping)
//
// IDEMPOTENCY: idempotency_key = "partial_release_<refund_id>". Replays are
// no-ops at the ledger layer.
//
// PRECONDITION: This must be called AFTER RecordRefundReversal and AFTER the
// escrow flip to RELEASED, in the SAME transaction.
func (s *FinanceService) RecordPartialRefundRelease(
	ctx context.Context,
	tx db.Tx,
	input RecordPartialRefundReleaseInput,
) (bool, error) {
	if input.RefundID == uuid.Nil {
		return false, fmt.Errorf("RecordPartialRefundRelease: refund_id required")
	}
	if input.OrderID == uuid.Nil {
		return false, fmt.Errorf("RecordPartialRefundRelease: order_id required")
	}
	if input.SellerID == uuid.Nil {
		return false, fmt.Errorf("RecordPartialRefundRelease: seller_id required")
	}
	if input.Remainder <= 0 {
		return false, fmt.Errorf("RecordPartialRefundRelease: remainder must be positive, got %d", input.Remainder)
	}
	if input.SellerNet < 0 || input.Commission < 0 {
		return false, fmt.Errorf("RecordPartialRefundRelease: amounts must be non-negative (seller_net=%d commission=%d)", input.SellerNet, input.Commission)
	}
	if input.SellerNet+input.Commission != input.Remainder {
		return false, fmt.Errorf("RecordPartialRefundRelease: seller_net (%d) + commission (%d) must equal remainder (%d)", input.SellerNet, input.Commission, input.Remainder)
	}

	idempotencyKey := fmt.Sprintf("partial_release_%s", input.RefundID.String())

	// Duplicate check (observability only â€” CreateTransaction is itself idempotent).
	alreadyRecorded, err := s.ledgerRepo.CountTransactionsByEntityID(ctx, tx, input.RefundID)
	if err != nil {
		return false, fmt.Errorf("partial release duplicate-check: %w", err)
	}
	// The refund_reversal tx for this refundID was already recorded (count >= 1).
	// The partial_release tx would make it >= 2. We check via idempotency_key
	// at the DB level, but log for observability.

	gatewayClearingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountGatewayClearing)
	if err != nil {
		return false, fmt.Errorf("partial release: get gateway clearing: %w", err)
	}
	sellerPayableID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountSellerPayable, input.SellerID)
	if err != nil {
		return false, fmt.Errorf("partial release: get seller payable: %w", err)
	}
	platformRevenueID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformRevenue)
	if err != nil {
		return false, fmt.Errorf("partial release: get platform revenue: %w", err)
	}

	entries := []ledgerepo.Entry{
		{AccountID: gatewayClearingID, Amount: money.New(-input.Remainder)},
		{AccountID: sellerPayableID, Amount: money.New(input.SellerNet)},
		{AccountID: platformRevenueID, Amount: money.New(input.Commission)},
	}

	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idempotencyKey, "partial_refund_release", input.RefundID, &input.OrderID, nil, entries,
	); err != nil {
		return false, fmt.Errorf("record partial release ledger: %w", err)
	}

	isDuplicate := alreadyRecorded >= 2 // refund_reversal + partial_release already existed
	s.logger.Info("finance_partial_refund_release_recorded",
		zap.String("refund_id", input.RefundID.String()),
		zap.String("order_id", input.OrderID.String()),
		zap.String("seller_id", input.SellerID.String()),
		zap.Int64("remainder", input.Remainder),
		zap.Int64("seller_net", input.SellerNet),
		zap.Int64("commission", input.Commission),
		zap.String("idempotency_key", idempotencyKey),
		zap.Bool("duplicate", isDuplicate),
	)
	return isDuplicate, nil
}

// ============================================================================
// SELLER WITHDRAWABLE — DISPUTE-AWARE FREEZE
// ============================================================================

// ErrWithdrawalBlockedByWithdrawableBalance is returned by AssertSellerWithdrawalAllowed
// when the requested withdrawal exceeds the dispute-aware withdrawable balance.
type ErrWithdrawalBlockedByWithdrawableBalance struct {
	SellerID            uuid.UUID
	RequestedAmount     int64
	PayableBalance      int64
	ActiveDisputeFreeze int64
	Withdrawable        int64
}

func (e *ErrWithdrawalBlockedByWithdrawableBalance) Error() string {
	return fmt.Sprintf(
		"withdrawal blocked by withdrawable balance: seller=%s requested=%d payable=%d freeze=%d withdrawable=%d",
		e.SellerID, e.RequestedAmount, e.PayableBalance, e.ActiveDisputeFreeze, e.Withdrawable,
	)
}

// SellerWithdrawableSummary is the dispute-aware snapshot of a
// seller's payout surface used by both the withdraw guard and observability.
type SellerWithdrawableSummary struct {
	PayableBalance      int64
	ActiveDisputeFreeze int64 // sum of active dispute_freeze rows
	Withdrawable        int64 // max(payable - dispute_freeze, 0)
}

// GetSellerWithdrawable returns the seller's dispute-aware withdrawable amount.
// Pure read — locks nothing — safe for non-mutating callers (admin UI,
// observability). For withdrawal-time enforcement, use
// AssertSellerWithdrawalAllowed which performs FOR UPDATE locking.
func (s *FinanceService) GetSellerWithdrawable(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) (*SellerWithdrawableSummary, error) {
	if sellerID == uuid.Nil {
		return nil, fmt.Errorf("GetSellerWithdrawable: seller_id required")
	}
	payableID, err := s.ledgerRepo.GetUserAccountID(ctx, tx, finance.AccountSellerPayable, sellerID)
	var payableBalance int64
	if err != nil {
		// No payable account yet — treated as zero balance.
		payableBalance = 0
	} else {
		bal, err := s.ledgerRepo.GetAccountBalance(ctx, tx, payableID)
		if err != nil {
			return nil, fmt.Errorf("read seller payable balance: %w", err)
		}
		payableBalance = bal.Int64()
	}
	var freezeTotal int64
	if s.disputeFreezeRepo != nil {
		freezeTotal, err = s.disputeFreezeRepo.GetTotalActiveBySeller(ctx, tx, sellerID)
		if err != nil {
			return nil, fmt.Errorf("read dispute freeze total: %w", err)
		}
	}
	withdrawable := payableBalance - freezeTotal
	if withdrawable < 0 {
		withdrawable = 0
	}
	return &SellerWithdrawableSummary{
		PayableBalance:      payableBalance,
		ActiveDisputeFreeze: freezeTotal,
		Withdrawable:        withdrawable,
	}, nil
}

// AssertSellerWithdrawalAllowed is the canonical withdrawal-time guard for
// the dispute-aware freeze. It MUST be called inside the same db.Tx as the
// downstream withdrawal mutation, so the FOR UPDATE lock on SELLER_PAYABLE
// holds across the decision and the wallet/finance write.
//
// Returns ErrWithdrawalBlockedByWithdrawableBalance when amount > withdrawable.
//
// Lock order (matches release/refund paths to avoid deadlock):
//  1. SELLER_PAYABLE balance (FOR UPDATE)
//  2. dispute freeze rows are read (no per-row lock; the SUM is the source of
//     truth and a concurrent freeze insert in another tx serializes via the
//     surrounding dispute workflow tx, which locks the order row first).
func (s *FinanceService) AssertSellerWithdrawalAllowed(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	amount int64,
) (*SellerWithdrawableSummary, error) {
	if sellerID == uuid.Nil {
		return nil, fmt.Errorf("AssertSellerWithdrawalAllowed: seller_id required")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("AssertSellerWithdrawalAllowed: amount must be positive (got %d)", amount)
	}

	payableID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountSellerPayable, sellerID)
	if err != nil {
		return nil, fmt.Errorf("get/create seller payable account: %w", err)
	}
	payableBal, err := s.ledgerRepo.GetAccountBalanceForUpdate(ctx, tx, payableID)
	if err != nil {
		return nil, fmt.Errorf("lock seller payable balance: %w", err)
	}
	var freezeTotal int64
	if s.disputeFreezeRepo != nil {
		freezeTotal, err = s.disputeFreezeRepo.GetTotalActiveBySeller(ctx, tx, sellerID)
		if err != nil {
			return nil, fmt.Errorf("read dispute freeze total: %w", err)
		}
	}
	// TASK 48: subtract active dispute freezes so disputed funds cannot be
	// withdrawn while an active dispute remains unresolved.
	withdrawable := payableBal.Int64() - freezeTotal
	if withdrawable < 0 {
		withdrawable = 0
	}
	summary := &SellerWithdrawableSummary{
		PayableBalance:      payableBal.Int64(),
		ActiveDisputeFreeze: freezeTotal,
		Withdrawable:        withdrawable,
	}
	if amount > withdrawable {
		s.logger.Warn("withdrawal_blocked_by_balance_ceiling",
			zap.String("seller_id", sellerID.String()),
			zap.Int64("requested_amount", amount),
			zap.Int64("payable_balance", payableBal.Int64()),
			zap.Int64("active_dispute_freeze", freezeTotal),
			zap.Int64("withdrawable", withdrawable),
		)
		return summary, &ErrWithdrawalBlockedByWithdrawableBalance{
			SellerID:            sellerID,
			RequestedAmount:     amount,
			PayableBalance:      payableBal.Int64(),
			ActiveDisputeFreeze: freezeTotal,
			Withdrawable:        withdrawable,
		}
	}
	return summary, nil
}

// CreateDisputeFreeze records a dispute freeze against the seller's
// SELLER_PAYABLE. This helper remains as compatibility scaffolding, but the
// live runtime no longer opens completed+released disputes.
//
// The freeze reduces the seller's withdrawable until the dispute resolves:
//   - Seller wins  â†’ ReleaseDisputeFreeze (no ledger change, freeze deleted)
//   - Buyer  wins  â†’ ReleaseDisputeFreeze for the frozen amount
//
// FIX-4: Acquires SELLER_PAYABLE FOR UPDATE before inserting the freeze row.
// This serializes CreateDisputeFreeze against concurrent calls to
// AssertSellerWithdrawalAllowed (which also locks SELLER_PAYABLE FOR UPDATE),
// closing the TOCTOU window where a withdrawal TX could read freeze_total=0
// before this TX commits the new dispute_freezes row.
func (s *FinanceService) CreateDisputeFreeze(
	ctx context.Context,
	tx db.Tx,
	disputeID, sellerID, orderID uuid.UUID,
	frozenAmount int64,
) error {
	if s.disputeFreezeRepo == nil {
		return fmt.Errorf("dispute freeze repository not wired")
	}
	// FIX-4: lock SELLER_PAYABLE to serialize against concurrent withdrawal auth.
	payableID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountSellerPayable, sellerID)
	if err != nil {
		return fmt.Errorf("CreateDisputeFreeze: get seller payable account: %w", err)
	}
	if _, err = s.ledgerRepo.GetAccountBalanceForUpdate(ctx, tx, payableID); err != nil {
		return fmt.Errorf("CreateDisputeFreeze: lock seller payable: %w", err)
	}
	freeze := &ledgerepo.DisputeFreeze{
		ID:           uuid.New(),
		DisputeID:    disputeID,
		OrderID:      orderID,
		SellerID:     sellerID,
		FrozenAmount: frozenAmount,
		Status:       "active",
		CreatedAt:    time.Now().UnixMilli(),
		UpdatedAt:    time.Now().UnixMilli(),
	}
	if err := s.disputeFreezeRepo.Create(ctx, tx, freeze); err != nil {
		return fmt.Errorf("CreateDisputeFreeze: %w", err)
	}
	s.logger.Info("dispute_financial_freeze_created",
		zap.String("dispute_id", disputeID.String()),
		zap.String("order_id", orderID.String()),
		zap.String("seller_id", sellerID.String()),
		zap.Int64("frozen_amount", frozenAmount),
	)
	return nil
}

// ReleaseDisputeFreeze marks a dispute freeze as released.
// Idempotent â€” safe to call even if no freeze exists (legacy compatibility).
func (s *FinanceService) ReleaseDisputeFreeze(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) error {
	if s.disputeFreezeRepo == nil {
		return nil // no-op when repo not wired (pre-migration)
	}
	if err := s.disputeFreezeRepo.Release(ctx, tx, disputeID); err != nil {
		return fmt.Errorf("ReleaseDisputeFreeze: %w", err)
	}
	s.logger.Info("dispute_financial_freeze_released",
		zap.String("dispute_id", disputeID.String()),
	)
	return nil
}

// ReleaseDisputeFreezeByOrderID marks any active freeze for the given order
// as released. Idempotent: no-op if no active freeze exists. This remains as
// compatibility scaffolding; current runtime does not reach it from the
// completed+released refund/dispute flow.
func (s *FinanceService) ReleaseDisputeFreezeByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	if s.disputeFreezeRepo == nil {
		return nil // no-op when repo not wired
	}
	if err := s.disputeFreezeRepo.ReleaseByOrderID(ctx, tx, orderID); err != nil {
		return fmt.Errorf("ReleaseDisputeFreezeByOrderID: %w", err)
	}
	s.logger.Info("dispute_financial_freeze_released_by_order",
		zap.String("order_id", orderID.String()),
	)
	return nil
}

// RecordWithdrawalRequest books the canonical request-time finance ledger
// transaction for a seller withdrawal. This is the SINGLE point at which
// money leaves SELLER_PAYABLE and is reserved into WITHDRAWAL_PENDING.
//
// Ledger movements (Î£ entries = 0 invariant):
//   - SELLER_PAYABLE[seller]   -amount  (decrease â€” sign convention: balance += entry.Amount)
//   - WITHDRAWAL_PENDING       +amount  (increase â€” system account)
//
// MUST run inside the same db.Tx as AssertSellerWithdrawalAllowed so the
// SELLER_PAYABLE FOR UPDATE lock taken there is still held when this
// transaction posts. The wallet domain is NOT touched.
//
// IDEMPOTENCY: idempotency_key = "withdrawal_request_<withdrawal_id>".
// A duplicate call (HTTP retry, replay) is a no-op at the ledger layer:
// the unique-key constraint on financial_transactions returns success
// without re-applying entries. Caller must therefore use the same
// withdrawalID across retries â€” this is satisfied because withdrawal IDs
// are persisted in wallet.withdrawals before this method runs and the
// single-pending guard prevents two pending rows per seller.
func (s *FinanceService) RecordWithdrawalRequest(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	amount int64,
	feeAmount int64,
	withdrawalID uuid.UUID,
) error {
	if sellerID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalRequest: seller_id required")
	}
	if withdrawalID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalRequest: withdrawal_id required")
	}
	if amount <= 0 {
		return fmt.Errorf("RecordWithdrawalRequest: amount must be positive (got %d)", amount)
	}
	if feeAmount < 0 {
		return fmt.Errorf("RecordWithdrawalRequest: fee_amount must be non-negative (got %d)", feeAmount)
	}

	sellerPayableID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountSellerPayable, sellerID)
	if err != nil {
		return fmt.Errorf("get/create seller payable account: %w", err)
	}
	pendingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountWithdrawalPending)
	if err != nil {
		return fmt.Errorf("get withdrawal pending account: %w", err)
	}

	// MONEY MODEL (PASS_18H): reserve exactly `amount` — the fee is deducted
	// FROM it at final settlement, never added on top. feeAmount is accepted
	// here only for logging/audit symmetry with the other lifecycle methods.
	entries := []ledgerepo.Entry{
		{AccountID: sellerPayableID, Amount: money.New(-amount)},
		{AccountID: pendingID, Amount: money.New(amount)},
	}
	idem := fmt.Sprintf("withdrawal_request_%s", withdrawalID.String())
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idem, "withdrawal_request", withdrawalID, nil, nil, entries,
	); err != nil {
		return fmt.Errorf("record withdrawal request ledger: %w", err)
	}
	s.logger.Info("withdrawal_request_ledger_posted",
		zap.String("seller_id", sellerID.String()),
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.Int64("amount", amount),
		zap.Int64("fee_amount", feeAmount),
		zap.String("seller_payable_account", sellerPayableID.String()),
		zap.String("withdrawal_pending_account", pendingID.String()),
		zap.String("idempotency_key", idem),
	)
	return nil
}

// RecordWithdrawalCommit books the canonical approval-time finance ledger
// transaction for a seller withdrawal. This is the SINGLE point at which
// reserved money moves from WITHDRAWAL_PENDING into WITHDRAWAL_COMMITTED,
// signalling that the platform has committed to the payout (admin approved
// or webhook acknowledged).
//
// Ledger movements (Î£ entries = 0 invariant):
//   - WITHDRAWAL_PENDING    -amount  (decrease â€” release reservation)
//   - WITHDRAWAL_COMMITTED  +amount  (increase â€” committed liability)
//
// MUST run inside the same db.Tx as the withdrawal status flip so the
// state machine and ledger move in lockstep. The wallet domain is NOT
// touched. SELLER_PAYABLE is NOT touched (the seller already gave it up
// at request time).
//
// IDEMPOTENCY: idempotency_key = "withdrawal_commit_<withdrawal_id>".
// A duplicate call (admin double-click, retry) is a no-op at the ledger
// layer via the unique-key constraint. Caller MUST gate the surrounding
// status flip on the SAME idempotency surface (i.e. check status before
// calling, treat already-approved as success without re-posting).
func (s *FinanceService) RecordWithdrawalCommit(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	amount int64,
	feeAmount int64,
	withdrawalID uuid.UUID,
) error {
	if sellerID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalCommit: seller_id required")
	}
	if withdrawalID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalCommit: withdrawal_id required")
	}
	if amount <= 0 {
		return fmt.Errorf("RecordWithdrawalCommit: amount must be positive (got %d)", amount)
	}
	if feeAmount < 0 {
		return fmt.Errorf("RecordWithdrawalCommit: fee_amount must be non-negative (got %d)", feeAmount)
	}

	pendingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountWithdrawalPending)
	if err != nil {
		return fmt.Errorf("get withdrawal pending account: %w", err)
	}
	committedID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountWithdrawalCommitted)
	if err != nil {
		return fmt.Errorf("get withdrawal committed account: %w", err)
	}

	// MONEY MODEL (PASS_18H): move the same reserved `amount` — no fee added.
	entries := []ledgerepo.Entry{
		{AccountID: pendingID, Amount: money.New(-amount)},
		{AccountID: committedID, Amount: money.New(amount)},
	}
	idem := fmt.Sprintf("withdrawal_commit_%s", withdrawalID.String())
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idem, "withdrawal_commit", withdrawalID, nil, nil, entries,
	); err != nil {
		return fmt.Errorf("record withdrawal commit ledger: %w", err)
	}
	s.logger.Info("withdrawal_commit_ledger_posted",
		zap.String("seller_id", sellerID.String()),
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.Int64("amount", amount),
		zap.Int64("fee_amount", feeAmount),
		zap.String("withdrawal_pending_account", pendingID.String()),
		zap.String("withdrawal_committed_account", committedID.String()),
		zap.String("idempotency_key", idem),
	)
	return nil
}

// RecordWithdrawalReject restores reserved money to SELLER_PAYABLE when an
// admin rejects a still-pending withdrawal. This is the SINGLE point at
// which money returns from WITHDRAWAL_PENDING back to the seller's
// payable surface.
//
// Ledger movements (Î£ entries = 0 invariant):
//   - WITHDRAWAL_PENDING       -amount  (decrease â€” release reservation)
//   - SELLER_PAYABLE[seller]   +amount  (increase â€” restore payable)
//
// IDEMPOTENCY: idempotency_key = "withdrawal_reject_<withdrawal_id>".
// The unique key collapses retried/double-clicked rejections into one
// ledger transaction. The caller MUST gate the status flip on the same
// surface â€” already-rejected withdrawals are a no-op.
//
// PRECONDITION: the withdrawal must be in the "pending" state (still
// holding funds in WITHDRAWAL_PENDING). Rejecting a withdrawal that has
// already been committed (PROCESSING) requires RecordWithdrawalRestore
// instead, which sources from WITHDRAWAL_COMMITTED.
func (s *FinanceService) RecordWithdrawalReject(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	amount int64,
	feeAmount int64,
	withdrawalID uuid.UUID,
) error {
	if sellerID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalReject: seller_id required")
	}
	if withdrawalID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalReject: withdrawal_id required")
	}
	if amount <= 0 {
		return fmt.Errorf("RecordWithdrawalReject: amount must be positive (got %d)", amount)
	}
	if feeAmount < 0 {
		return fmt.Errorf("RecordWithdrawalReject: fee_amount must be non-negative (got %d)", feeAmount)
	}

	pendingID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountWithdrawalPending)
	if err != nil {
		return fmt.Errorf("get withdrawal pending account: %w", err)
	}
	sellerPayableID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountSellerPayable, sellerID)
	if err != nil {
		return fmt.Errorf("get/create seller payable account: %w", err)
	}

	// MONEY MODEL (PASS_18H): restore the same reserved `amount` — no fee added.
	entries := []ledgerepo.Entry{
		{AccountID: pendingID, Amount: money.New(-amount)},
		{AccountID: sellerPayableID, Amount: money.New(amount)},
	}
	idem := fmt.Sprintf("withdrawal_reject_%s", withdrawalID.String())
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idem, "withdrawal_reject", withdrawalID, nil, nil, entries,
	); err != nil {
		return fmt.Errorf("record withdrawal reject ledger: %w", err)
	}
	s.logger.Info("withdrawal_reject_ledger_posted",
		zap.String("seller_id", sellerID.String()),
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.Int64("amount", amount),
		zap.Int64("fee_amount", feeAmount),
		zap.String("withdrawal_pending_account", pendingID.String()),
		zap.String("seller_payable_account", sellerPayableID.String()),
		zap.String("idempotency_key", idem),
	)
	return nil
}

// RecordWithdrawalRestore restores committed payout money back to the
// seller payable surface when an already-approved manual payout fails or
// is explicitly rejected before final completion.
//
// Ledger movements:
//   - WITHDRAWAL_COMMITTED     -amount
//   - SELLER_PAYABLE[seller]   +amount
//
// IDEMPOTENCY: withdrawal_restore_<withdrawal_id>
func (s *FinanceService) RecordWithdrawalRestore(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	amount int64,
	feeAmount int64,
	withdrawalID uuid.UUID,
) error {
	if sellerID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalRestore: seller_id required")
	}
	if withdrawalID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalRestore: withdrawal_id required")
	}
	if amount <= 0 {
		return fmt.Errorf("RecordWithdrawalRestore: amount must be positive (got %d)", amount)
	}
	if feeAmount < 0 {
		return fmt.Errorf("RecordWithdrawalRestore: fee_amount must be non-negative (got %d)", feeAmount)
	}

	committedID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountWithdrawalCommitted)
	if err != nil {
		return fmt.Errorf("get withdrawal committed account: %w", err)
	}
	sellerPayableID, err := s.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountSellerPayable, sellerID)
	if err != nil {
		return fmt.Errorf("get/create seller payable account: %w", err)
	}

	// MONEY MODEL (PASS_18H): restore the same reserved `amount` — no fee added.
	entries := []ledgerepo.Entry{
		{AccountID: committedID, Amount: money.New(-amount)},
		{AccountID: sellerPayableID, Amount: money.New(amount)},
	}
	idem := fmt.Sprintf("withdrawal_restore_%s", withdrawalID.String())
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idem, "withdrawal_restore", withdrawalID, nil, nil, entries,
	); err != nil {
		return fmt.Errorf("record withdrawal restore ledger: %w", err)
	}
	s.logger.Info("withdrawal_restore_ledger_posted",
		zap.String("seller_id", sellerID.String()),
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.Int64("amount", amount),
		zap.Int64("fee_amount", feeAmount),
		zap.String("withdrawal_committed_account", committedID.String()),
		zap.String("seller_payable_account", sellerPayableID.String()),
		zap.String("idempotency_key", idem),
	)
	return nil
}

// RecordWithdrawalComplete finalizes a manually executed payout.
//
// MONEY MODEL (PASS_18H, owner-confirmed): net_payout = amount - feeAmount.
// The fee is deducted FROM the reserved amount, never added on top of it.
//
// Ledger movements (Σ entries = 0 invariant):
//   - WITHDRAWAL_COMMITTED  -amount              (release the full reservation)
//   - PLATFORM_BANK         +(amount-feeAmount)  (net payout — what actually leaves to the seller's bank)
//   - PLATFORM_REVENUE      +feeAmount            (withdrawal fee revenue)
//
// IDEMPOTENCY: withdrawal_complete_<withdrawal_id>
func (s *FinanceService) RecordWithdrawalComplete(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	amount int64,
	feeAmount int64,
	withdrawalID uuid.UUID,
) error {
	if sellerID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalComplete: seller_id required")
	}
	if withdrawalID == uuid.Nil {
		return fmt.Errorf("RecordWithdrawalComplete: withdrawal_id required")
	}
	if amount <= 0 {
		return fmt.Errorf("RecordWithdrawalComplete: amount must be positive (got %d)", amount)
	}
	if feeAmount < 0 {
		return fmt.Errorf("RecordWithdrawalComplete: fee_amount must be non-negative (got %d)", feeAmount)
	}
	if feeAmount >= amount {
		return fmt.Errorf("RecordWithdrawalComplete: fee_amount (%d) must be less than amount (%d)", feeAmount, amount)
	}

	committedID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountWithdrawalCommitted)
	if err != nil {
		return fmt.Errorf("get withdrawal committed account: %w", err)
	}
	platformBankID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformBank)
	if err != nil {
		return fmt.Errorf("get platform bank account: %w", err)
	}
	platformRevenueID, err := s.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformRevenue)
	if err != nil {
		return fmt.Errorf("get platform revenue account: %w", err)
	}
	netPayout := amount - feeAmount

	entries := []ledgerepo.Entry{
		{AccountID: committedID, Amount: money.New(-amount)},
		{AccountID: platformBankID, Amount: money.New(netPayout)},
		{AccountID: platformRevenueID, Amount: money.New(feeAmount)},
	}
	idem := fmt.Sprintf("withdrawal_complete_%s", withdrawalID.String())
	if err := s.ledgerRepo.CreateTransaction(
		ctx, tx, idem, "withdrawal_complete", withdrawalID, nil, nil, entries,
	); err != nil {
		return fmt.Errorf("record withdrawal complete ledger: %w", err)
	}
	s.logger.Info("withdrawal_complete_ledger_posted",
		zap.String("seller_id", sellerID.String()),
		zap.String("withdrawal_id", withdrawalID.String()),
		zap.Int64("amount", amount),
		zap.Int64("fee_amount", feeAmount),
		zap.Int64("net_payout", netPayout),
		zap.String("withdrawal_committed_account", committedID.String()),
		zap.String("platform_bank_account", platformBankID.String()),
		zap.String("platform_revenue_account", platformRevenueID.String()),
		zap.String("idempotency_key", idem),
	)
	return nil
}
