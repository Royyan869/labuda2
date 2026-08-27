package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance/bankaccount/entity"
	bankAccountRepo "github.com/labuda/backend/internal/finance/bankaccount/infrastructure/repository"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// VerificationChecker is the minimal interface for checking whether a seller
// currently holds payout authority (approved KYC status) inside an existing tx.
type VerificationChecker interface {
	IsSellerVerifiedTx(ctx context.Context, tx db.Tx, sellerID uuid.UUID) (bool, error)
}

// BankAccountService handles bank account business logic.
type BankAccountService struct {
	repo *bankAccountRepo.BankAccountRepository
	log  *zap.Logger

	// Optional — wired via SetVerificationOutbox after construction.
	// When wired, mutations emit structured outbox events when the seller is
	// currently approved, so admins can be alerted to re-review (Patch E).
	verifChecker VerificationChecker
	outboxRepo   *outboxrepo.OutboxRepository
}

// NewBankAccountService creates a new BankAccountService.
func NewBankAccountService() *BankAccountService {
	return &BankAccountService{
		repo: bankAccountRepo.NewBankAccountRepository(),
		log:  zap.NewNop(),
	}
}

// SetLogger sets the logger for the service.
func (s *BankAccountService) SetLogger(log *zap.Logger) {
	s.log = log
}

// SetVerificationOutbox wires the optional verification checker and outbox repo.
// When wired, Create/SetDefault/Delete emit bank_account.*_after_verification
// events when the seller is currently KYC-approved, signalling that the
// reviewed_bank_account_ids snapshot may be stale.
func (s *BankAccountService) SetVerificationOutbox(checker VerificationChecker, outbox *outboxrepo.OutboxRepository) {
	s.verifChecker = checker
	s.outboxRepo = outbox
}

// emitIfApproved emits an outbox event (eventType) in tx when the seller is
// currently KYC-approved. Fail-open: errors are logged, never returned.
func (s *BankAccountService) emitIfApproved(ctx context.Context, tx db.Tx, sellerID uuid.UUID, bankAccountID uuid.UUID, eventType string) {
	if s.verifChecker == nil || s.outboxRepo == nil {
		return
	}
	approved, err := s.verifChecker.IsSellerVerifiedTx(ctx, tx, sellerID)
	if err != nil || !approved {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"seller_id":       sellerID.String(),
		"bank_account_id": bankAccountID.String(),
	})
	if err := s.outboxRepo.InsertEvent(ctx, tx, eventType, sellerID, payload); err != nil {
		s.log.Error("bank_account event emit failed", zap.String("event", eventType), zap.Error(err))
	}
}

// ============================================================================
// INPUT TYPES
// ============================================================================

// CreateBankAccountInput contains parameters for creating a bank account.
type CreateBankAccountInput struct {
	UserID            uuid.UUID
	BankName          string
	BankCode          string // Canonical bank code for payout rail integration
	AccountNumber     string
	AccountHolderName string
	IsDefault         bool
}

// ============================================================================
// CREATE BANK ACCOUNT
// ============================================================================

// CreateBankAccount creates a new bank account for a seller.
//
// Validation:
// - All required fields are present
// - If is_default is true, unsets existing default bank account
func (s *BankAccountService) CreateBankAccount(
	ctx context.Context,
	tx db.Tx,
	input CreateBankAccountInput,
) (*entity.BankAccount, error) {
	// Create the bank account entity
	bankAccount, err := entity.NewBankAccount(
		input.UserID,
		input.BankName,
		input.BankCode,
		input.AccountNumber,
		input.AccountHolderName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bank account entity: %w", err)
	}

	// Persist the bank account
	if err := s.repo.Create(ctx, tx, bankAccount); err != nil {
		return nil, fmt.Errorf("failed to persist bank account: %w", err)
	}

	// If setting as default, set it as default
	if input.IsDefault {
		if err := s.repo.SetDefaultBankAccount(ctx, tx, bankAccount.ID, input.UserID); err != nil {
			return nil, fmt.Errorf("failed to set default bank account: %w", err)
		}
		// Refresh to get updated IsDefault status
		bankAccount, err = s.repo.GetActiveByID(ctx, tx, bankAccount.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve updated bank account: %w", err)
		}
	}

	s.log.Info("Bank account created",
		zap.String("bank_account_id", bankAccount.ID.String()),
		zap.String("seller_id", input.UserID.String()),
		zap.String("bank_name", input.BankName),
		zap.Bool("is_default", input.IsDefault),
	)

	// Patch E: emit event if seller is currently KYC-approved, so reviewed snapshot may be stale.
	s.emitIfApproved(ctx, tx, input.UserID, bankAccount.ID, "bank_account.added_after_verification")

	return bankAccount, nil
}

// ============================================================================
// GET BANK ACCOUNT
// ============================================================================

// GetBankAccount retrieves a bank account by ID.
// Validates that the seller owns the bank account.
func (s *BankAccountService) GetBankAccount(
	ctx context.Context,
	tx db.Tx,
	bankAccountID uuid.UUID,
	sellerID uuid.UUID,
) (*entity.BankAccount, error) {
	bankAccount, err := s.repo.GetActiveByID(ctx, tx, bankAccountID)
	if err != nil {
		return nil, fmt.Errorf("bank account not found: %w", err)
	}

	// Verify ownership
	if bankAccount.UserID != sellerID {
		return nil, fmt.Errorf("bank account does not belong to seller %s", sellerID)
	}

	return bankAccount, nil
}

// ============================================================================
// LIST SELLER BANK ACCOUNTS
// ============================================================================

// ListSellerBankAccounts retrieves all active bank accounts for a seller.
func (s *BankAccountService) ListSellerBankAccounts(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
) ([]*entity.BankAccount, error) {
	bankAccounts, err := s.repo.ListActiveAccountsBySeller(ctx, tx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list seller bank accounts: %w", err)
	}

	return bankAccounts, nil
}

// ============================================================================
// SET DEFAULT BANK ACCOUNT
// ============================================================================

// SetDefaultBankAccount sets a bank account as the default for a seller.
//
// Validation:
// - Bank account exists and is owned by the seller
// - Unsets any existing default bank account
func (s *BankAccountService) SetDefaultBankAccount(
	ctx context.Context,
	tx db.Tx,
	bankAccountID uuid.UUID,
	sellerID uuid.UUID,
) error {
	// Verify ownership first
	bankAccount, err := s.repo.GetActiveByID(ctx, tx, bankAccountID)
	if err != nil {
		return fmt.Errorf("bank account not found: %w", err)
	}

	if bankAccount.UserID != sellerID {
		return fmt.Errorf("bank account does not belong to seller %s", sellerID)
	}

	// Set as default (also unsets other default accounts)
	if err := s.repo.SetDefaultBankAccount(ctx, tx, bankAccountID, sellerID); err != nil {
		return fmt.Errorf("failed to set default bank account: %w", err)
	}

	s.log.Info("Default bank account set",
		zap.String("bank_account_id", bankAccountID.String()),
		zap.String("seller_id", sellerID.String()),
	)

	// Patch E: default payout destination changed post-approval — snapshot may be stale.
	s.emitIfApproved(ctx, tx, sellerID, bankAccountID, "bank_account.default_changed_after_verification")

	return nil
}

// ============================================================================
// DELETE BANK ACCOUNT
// ============================================================================

// DeleteBankAccount soft-deletes a bank account.
//
// Validation:
// - Bank account exists and is owned by the seller
// - No active withdrawals exist for the seller
func (s *BankAccountService) DeleteBankAccount(
	ctx context.Context,
	tx db.Tx,
	bankAccountID uuid.UUID,
	sellerID uuid.UUID,
) error {
	// Soft delete (repository handles validation)
	if err := s.repo.SoftDeleteBankAccount(ctx, tx, bankAccountID, sellerID); err != nil {
		return fmt.Errorf("failed to delete bank account: %w", err)
	}

	s.log.Info("Bank account deleted",
		zap.String("bank_account_id", bankAccountID.String()),
		zap.String("seller_id", sellerID.String()),
	)

	// Patch E: bank account removed post-approval — reviewed snapshot may now reference
	// a deleted account, blocking future withdrawals until re-approved.
	s.emitIfApproved(ctx, tx, sellerID, bankAccountID, "bank_account.deleted_after_verification")

	return nil
}


