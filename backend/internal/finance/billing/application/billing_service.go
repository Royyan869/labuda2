package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/finance/billing/entity"
	billingrepo "github.com/labuda/backend/internal/finance/billing/infrastructure/repository"
	financeapp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

// BillingService handles billing transaction state transitions.
// It processes non-order payments: promotion package purchase.
//
// LEDGER LOCKDOWN: billing domain CANNOT access ledger directly
// All ledger operations MUST go through FinanceService
type BillingService struct {
	billingRepo          *billingrepo.BillingRepository
	financeService       *financeapp.FinanceService
	roleChecker          auth.RoleChecker
	accountStatusChecker auth.AccountStatusChecker
	ownership            *auth.OwnershipValidator
}

// NewBillingService creates a new BillingService.
func NewBillingService(roleChecker auth.RoleChecker, accountStatusChecker auth.AccountStatusChecker) *BillingService {
	return &BillingService{
		billingRepo:          billingrepo.NewBillingRepository(),
		financeService:       financeapp.NewFinanceService(),
		roleChecker:          roleChecker,
		accountStatusChecker: accountStatusChecker,
		ownership:            auth.NewOwnershipValidator(),
	}
}

// CreateBillingTransaction creates a new pending billing transaction.
// AUTHORIZATION: Only the payer can create a billing transaction for themselves.
func (s *BillingService) CreateBillingTransaction(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	payerID uuid.UUID,
	targetID uuid.UUID,
	billingType entity.Type,
	grossAmount money.Money,
	platformFeePercent int64,
) (*entity.BillingTransaction, error) {
	// Validate caller
	if err := auth.ValidateCaller(callerID); err != nil {
		return nil, err
	}

	// ACCOUNT STATUS: Check if payer's account is active (system caller bypasses)
	if !auth.IsSystemCaller(callerID) {
		if err := s.accountStatusChecker.EnsureActive(ctx, callerID); err != nil {
			return nil, err
		}
	}

	// AUTHORIZATION: PayerID must match CallerID (users can only create billing for themselves)
	// System caller can bypass this check
	if !auth.IsSystemCaller(callerID) && callerID != payerID {
		return nil, auth.ErrOwnerRequired
	}

	billing, err := entity.NewBillingTransaction(payerID, targetID, billingType, grossAmount, platformFeePercent)
	if err != nil {
		return nil, err
	}

	if err := s.billingRepo.CreateBillingTransaction(ctx, tx, billing); err != nil {
		return nil, err
	}

	return billing, nil
}

// MarkPaid processes a successful billing payment.
// This creates ledger entries based on the billing type:
//
// UNIFIED SETTLEMENT MODEL V2:
//
// For promotion_package:
//   - platform_fee + net_amount → PLATFORM_REVENUE (full amount as revenue)
//   - Status becomes paid (promotion ownership created immediately)
//
// Returns (newlyMarkedPaid bool, err error).
// newlyMarkedPaid=false means the billing was already paid before this call;
// callers MUST short-circuit and skip any post-payment side-effects when false.
func (s *BillingService) MarkPaid(
	ctx context.Context,
	tx db.Tx,
	billingID uuid.UUID,
) (bool, error) {
	// Lock the billing transaction for update
	billing, err := s.billingRepo.GetForUpdate(ctx, tx, billingID)
	if err != nil {
		return false, err
	}

	// Check idempotency - already paid?
	// Return false so callers know to skip post-payment side-effects.
	if billing.Status == entity.StatusPaid {
		return false, nil // Already processed
	}

	// Validate status transition
	if err := billing.MarkPaid(); err != nil {
		return false, fmt.Errorf("invalid billing status: %w", err)
	}

	// Build idempotency key for ledger transaction
	idempotencyKey := fmt.Sprintf("billing-%s", billingID)

	// Process based on billing type
	switch billing.Type {
	case entity.TypePromotionPackage:
		// Promotion package: full amount goes to platform revenue
		// These are one-time service purchases with no escrow holding
		if err := s.processServicePurchase(ctx, tx, billing, idempotencyKey); err != nil {
			return false, err
		}

	default:
		return false, fmt.Errorf("unsupported billing type: %s", billing.Type)
	}

	// Persist status change
	return true, s.billingRepo.UpdateStatus(ctx, tx, billing)
}

// processServicePurchase handles promotion package billing payments.
// Full amount (platform fee + net amount) goes to platform revenue.
// These are one-time service purchases with no escrow holding.
//
// FINANCE REDIRECT: Delegates to FinanceService.RecordBillingServiceRevenue
func (s *BillingService) processServicePurchase(
	ctx context.Context,
	tx db.Tx,
	billing *entity.BillingTransaction,
	idempotencyKey string,
) error {
	// FINANCE REDIRECT: All ledger operations go through FinanceService
	return s.financeService.RecordBillingServiceRevenue(ctx, tx, billing)
}

// MarkFailed marks a billing transaction as failed.
func (s *BillingService) MarkFailed(
	ctx context.Context,
	tx db.Tx,
	billingID uuid.UUID,
) error {
	billing, err := s.billingRepo.GetForUpdate(ctx, tx, billingID)
	if err != nil {
		return err
	}

	// Check idempotency - already failed?
	if billing.Status == entity.StatusFailed {
		return nil // Already processed
	}

	if err := billing.MarkFailed(); err != nil {
		return err
	}

	return s.billingRepo.UpdateStatus(ctx, tx, billing)
}

// GetBillingTransaction retrieves a billing transaction without locking.
func (s *BillingService) GetBillingTransaction(
	ctx context.Context,
	tx db.Tx,
	billingID uuid.UUID,
) (*entity.BillingTransaction, error) {
	return s.billingRepo.GetByID(ctx, tx, billingID)
}

// GetBillingTransactionsByPayer retrieves all billing transactions for a payer.
func (s *BillingService) GetBillingTransactionsByPayer(
	ctx context.Context,
	tx db.Tx,
	payerID uuid.UUID,
	limit int,
) ([]*entity.BillingTransaction, error) {
	return s.billingRepo.GetByPayerID(ctx, tx, payerID, limit)
}

// GetBillingTransactionsByTarget retrieves billing transactions by target ID.
func (s *BillingService) GetBillingTransactionsByTarget(
	ctx context.Context,
	tx db.Tx,
	targetID uuid.UUID,
) ([]*entity.BillingTransaction, error) {
	return s.billingRepo.GetByTargetID(ctx, tx, targetID)
}


