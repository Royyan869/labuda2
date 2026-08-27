package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	"github.com/labuda/backend/internal/finance/infrastructure/repository"
	ledgerintf "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// webhookOutboxEmitter is a minimal interface for emitting outbox events.
// Defined locally so tests can inject a spy without importing the full
// outbox infrastructure package.
type webhookOutboxEmitter interface {
	InsertEvent(ctx context.Context, tx db.Tx, eventType string, entityID uuid.UUID, payload []byte) error
}

// WebhookStatus represents the status reported by the gateway in a webhook callback.
type WebhookStatus string

const (
	WebhookStatusSuccess  WebhookStatus = "SUCCESS"  // Payout completed successfully
	WebhookStatusPending  WebhookStatus = "PENDING"  // Payout is pending/settling
	WebhookStatusFailed   WebhookStatus = "FAILED"   // Payout failed
	WebhookStatusRejected WebhookStatus = "REJECTED" // Payout rejected by bank
	WebhookStatusUnknown  WebhookStatus = "UNKNOWN"  // Unknown status (log and ignore)
)

// WebhookCallback represents a callback from the payment gateway.
type WebhookCallback struct {
	// ExternalReferenceID is our reference (sent during submission)
	ExternalReferenceID string

	// GatewayReferenceID is the gateway's internal reference
	GatewayReferenceID string

	// Status is the final status from the gateway
	Status WebhookStatus

	// Message is an optional message from the gateway
	Message string

	// Timestamp is when the gateway processed this (Unix)
	Timestamp int64

	// RawPayload is the full callback for audit purposes
	RawPayload string
}

// WebhookHandler handles payment gateway webhook callbacks.
//
// SAFETY GUARDS:
// - Idempotent: duplicate callbacks are handled safely
// - State validation: only allows valid state transitions
// - Final state protection: cannot override already-settled statuses
// - Audit logging: all callbacks are logged
// - FINALITY: Completes ledger movement atomically on settlement
// - OUTBOX: Emits withdrawal.completed event on gateway success (same tx)
type WebhookHandler struct {
	withdrawRepo *repository.WithdrawRepository
	ledgerRepo   *repository.LedgerRepository
	outboxRepo   webhookOutboxEmitter // optional; nil-safe
	log          *zap.Logger
}

// NewWebhookHandler creates a new webhook handler.
// ledgerRepo is required for finality accounting - completing ledger movement on settlement.
// outboxRepo is optional (nil-safe); when provided, gateway-settled withdrawals emit
// a withdrawal.completed outbox event in the same DB transaction.
func NewWebhookHandler(
	withdrawRepo *repository.WithdrawRepository,
	ledgerRepo *repository.LedgerRepository,
	outboxRepo webhookOutboxEmitter,
	log *zap.Logger,
) *WebhookHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &WebhookHandler{
		withdrawRepo: withdrawRepo,
		ledgerRepo:   ledgerRepo,
		outboxRepo:   outboxRepo,
		log:          log,
	}
}

// HandleCallback processes a webhook callback from the payment gateway.
//
// Returns:
// - nil: callback was processed successfully
// - ErrDuplicateCallback: callback was a duplicate (not an error, idempotent)
// - ErrInvalidStateTransition: callback would cause invalid state transition
// - ErrWithdrawalNotFound: withdrawal not found
// - error: other processing error
func (h *WebhookHandler) HandleCallback(ctx context.Context, tx db.Tx, callback WebhookCallback) error {
	// Find withdrawal by external reference
	withdrawal, err := h.withdrawRepo.GetByExternalReference(ctx, tx, callback.ExternalReferenceID)
	if err != nil {
		return fmt.Errorf("find withdrawal by external reference: %w", err)
	}

	h.log.Info("Processing webhook callback",
		zap.String("withdrawal_id", withdrawal.ID.String()),
		zap.String("external_ref", callback.ExternalReferenceID),
		zap.String("current_status", string(withdrawal.Status)),
		zap.String("callback_status", string(callback.Status)),
		zap.String("gateway_ref", callback.GatewayReferenceID),
	)

	// Check if withdrawal is already in a final state
	if withdrawal.Status.IsFinal() {
		h.log.Info("Withdrawal already in final state, ignoring callback",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.String("status", string(withdrawal.Status)),
		)
		return ErrDuplicateCallback
	}

	// Process based on callback status
	switch callback.Status {
	case WebhookStatusSuccess:
		return h.handleSuccessCallback(ctx, tx, withdrawal, callback)

	case WebhookStatusPending:
		return h.handlePendingCallback(ctx, tx, withdrawal, callback)

	case WebhookStatusFailed, WebhookStatusRejected:
		return h.handleFailedCallback(ctx, tx, withdrawal, callback)

	default:
		h.log.Warn("Unknown webhook status, ignoring",
			zap.String("status", string(callback.Status)),
			zap.String("withdrawal_id", withdrawal.ID.String()),
		)
		return nil
	}
}

// handleSuccessCallback processes a SUCCESS webhook.
// Transitions: SUBMITTED/SETTLING -> SETTLED
//
// WITHDRAWAL CONSISTENCY FIX:
// This method completes the ledger movement atomically with status update.
// Funds are moved from WITHDRAWAL_COMMITTED (where approved withdrawals sit)
// to PLATFORM_BANK to reflect the actual outflow.
//
// FINALITY ACCOUNTING:
// When a payout is truly settled by the gateway, funds must move from
// WITHDRAWAL_COMMITTED to PLATFORM_BANK to reflect the actual outflow.
//
// Ledger movement:
// - DR WITHDRAWAL_COMMITTED (decrease committed balance)
// - CR PLATFORM_BANK (increase bank outflow - funds left the system)
//
// This is idempotent - if the ledger transaction already exists, it will be
// skipped due to the unique idempotency key constraint.
func (h *WebhookHandler) handleSuccessCallback(
	ctx context.Context,
	tx db.Tx,
	withdrawal *repository.Withdrawal,
	callback WebhookCallback,
) error {
	// Validate state transition
	currentStatus := withdrawal.Status
	if currentStatus != repository.WithdrawalStatusSubmitted &&
		currentStatus != repository.WithdrawalStatusSettling {
		h.log.Warn("Invalid state for success callback",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.String("current_status", string(currentStatus)),
		)
		return &InvalidStateTransitionError{
			From: string(currentStatus),
			To:   string(repository.WithdrawalStatusSettled),
		}
	}

	// CRITICAL: Complete ledger movement BEFORE marking settled
	// This ensures accounting truth is updated atomically with status
	//
	// MONEY MODEL (PASS_18H, owner-confirmed): net_payout = amount - feeAmount.
	// The fee is deducted FROM the reserved amount, never added on top of it.
	//
	// Movement: DR WITHDRAWAL_COMMITTED (full amount), CR PLATFORM_BANK (net
	// payout) / CR PLATFORM_REVENUE (fee).
	amountMoney := money.New(withdrawal.Amount)
	feeMoney := money.New(withdrawal.FeeAmount)
	netPayoutMoney := amountMoney.Sub(feeMoney)
	idempotencyKey := fmt.Sprintf("withdraw_settle_%s", withdrawal.ID.String())

	// Get system account IDs
	// WITHDRAWAL_COMMITTED is where approved withdrawals sit
	withdrawalCommittedID, err := h.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountWithdrawalCommitted)
	if err != nil {
		return fmt.Errorf("get withdrawal committed account: %w", err)
	}

	platformBankID, err := h.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformBank)
	if err != nil {
		return fmt.Errorf("get platform bank account: %w", err)
	}
	platformRevenueID, err := h.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountPlatformRevenue)
	if err != nil {
		return fmt.Errorf("get platform revenue account: %w", err)
	}

	// Create ledger transaction entries
	// DR WITHDRAWAL_COMMITTED (decrease committed by the full amount)
	// CR PLATFORM_BANK (net payout — what actually leaves to the seller's bank)
	// CR PLATFORM_REVENUE (withdrawal fee)
	entries := []ledgerintf.Entry{
		{AccountID: withdrawalCommittedID, Amount: amountMoney.Neg()}, // Credit (decrease committed)
		{AccountID: platformBankID, Amount: netPayoutMoney},           // Net payout to seller bank
		{AccountID: platformRevenueID, Amount: feeMoney},              // Withdrawal fee revenue
	}

	err = h.ledgerRepo.CreateTransaction(
		ctx,
		tx,
		idempotencyKey,
		"WITHDRAWAL_SETTLE",
		withdrawal.ID,
		nil, // orderID
		nil, // paymentID
		entries,
	)
	if err != nil {
		return fmt.Errorf("complete settlement ledger movement: %w", err)
	}

	h.log.Info("Settlement ledger movement completed",
		zap.String("withdrawal_id", withdrawal.ID.String()),
		zap.Int64("amount", withdrawal.Amount),
		zap.Int64("fee_amount", withdrawal.FeeAmount),
		zap.Int64("net_payout", netPayoutMoney.Int64()),
		zap.String("idempotency_key", idempotencyKey),
	)

	// Mark as settled AFTER ledger movement succeeds
	err = h.withdrawRepo.MarkSettled(ctx, tx, withdrawal.ID, callback.RawPayload)
	if err != nil {
		return fmt.Errorf("mark settled: %w", err)
	}

	h.log.Info("Withdrawal marked as settled via webhook with completed ledger movement",
		zap.String("withdrawal_id", withdrawal.ID.String()),
	)

	// OUTBOX ATOMIC — emit withdrawal.completed in the same transaction.
	// The existing notification handler (withdrawal.completed) is already registered
	// in SetupNotificationHandlers. Using the same event name means the seller
	// receives a push notification for gateway-settled payouts, closing the P0
	// visibility gap where manual completion notified but gateway path did not.
	// source=gateway_webhook distinguishes this emission from the manual path.
	// InsertEvent failure rolls back the entire transaction.
	if h.outboxRepo != nil {
		payload := map[string]interface{}{
			"withdrawal_id": withdrawal.ID.String(),
			"seller_id":     withdrawal.SellerID.String(),
			"amount":        withdrawal.Amount,
			"fee_amount":    withdrawal.FeeAmount,
			"status":        string(repository.WithdrawalStatusSettled),
			"source":        "gateway_webhook",
		}
		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return fmt.Errorf("marshal withdrawal.completed payload: %w", marshalErr)
		}
		if err := h.outboxRepo.InsertEvent(ctx, tx, "withdrawal.completed", withdrawal.ID, payloadBytes); err != nil {
			return fmt.Errorf("outbox withdrawal.completed: %w", err)
		}
	}

	return nil
}

// handlePendingCallback processes a PENDING webhook.
// Transitions: SUBMITTED -> SETTLING
// No-op if already SETTLING
func (h *WebhookHandler) handlePendingCallback(
	ctx context.Context,
	tx db.Tx,
	withdrawal *repository.Withdrawal,
	callback WebhookCallback,
) error {
	currentStatus := withdrawal.Status

	// If already SETTLING, this is a duplicate pending callback
	if currentStatus == repository.WithdrawalStatusSettling {
		h.log.Debug("Already in SETTLING state, ignoring pending callback",
			zap.String("withdrawal_id", withdrawal.ID.String()),
		)
		return ErrDuplicateCallback
	}

	// Can only transition from SUBMITTED to SETTLING
	if currentStatus != repository.WithdrawalStatusSubmitted {
		h.log.Warn("Invalid state for pending callback",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.String("current_status", string(currentStatus)),
		)
		return &InvalidStateTransitionError{
			From: string(currentStatus),
			To:   string(repository.WithdrawalStatusSettling),
		}
	}

	// Transition to SETTLING
	err := h.withdrawRepo.MarkSettling(ctx, tx, withdrawal.ID, callback.RawPayload)
	if err != nil {
		return fmt.Errorf("mark settling: %w", err)
	}

	h.log.Info("Withdrawal marked as settling via webhook",
		zap.String("withdrawal_id", withdrawal.ID.String()),
	)

	return nil
}

// handleFailedCallback processes a FAILED/REJECTED webhook.
// Transitions: SUBMITTED/SETTLING -> FAILED_FINAL
//
// WITHDRAWAL CONSISTENCY FIX:
// When a payout fails permanently (e.g., bank rejection), funds MUST be returned
// to the seller's payable account to prevent accounting ambiguity.
//
// FINALITY ACCOUNTING FOR FAILED PAYOUTS:
// Funds are returned from WITHDRAWAL_COMMITTED (where approved withdrawals sit)
// to the seller's SELLER_PAYABLE account.
//
// Ledger movement for failed payout:
// - DR WITHDRAWAL_COMMITTED (decrease committed)
// - CR user's SELLER_PAYABLE (return funds to seller)
//
// This is idempotent - if the ledger transaction already exists, it will be
// skipped due to the unique idempotency key constraint.
func (h *WebhookHandler) handleFailedCallback(
	ctx context.Context,
	tx db.Tx,
	withdrawal *repository.Withdrawal,
	callback WebhookCallback,
) error {
	currentStatus := withdrawal.Status

	// Can only fail from SUBMITTED or SETTLING states
	if currentStatus != repository.WithdrawalStatusSubmitted &&
		currentStatus != repository.WithdrawalStatusSettling {
		h.log.Warn("Invalid state for failed callback",
			zap.String("withdrawal_id", withdrawal.ID.String()),
			zap.String("current_status", string(currentStatus)),
		)
		return &InvalidStateTransitionError{
			From: string(currentStatus),
			To:   string(repository.WithdrawalStatusFailedFinal),
		}
	}

	// CRITICAL: Return funds to seller BEFORE marking as failed
	// This prevents accounting ambiguity where funds are stuck in WITHDRAWAL_COMMITTED
	// but the withdrawal is marked as FAILED_FINAL
	//
	// MONEY MODEL (PASS_18H): the full reserved `amount` (not amount+fee) is
	// what was committed and is what returns to SELLER_PAYABLE on failure —
	// the fee is only ever split off at successful settlement.
	amountMoney := money.New(withdrawal.Amount)
	idempotencyKey := fmt.Sprintf(gatewayRestoreKeyFmt, withdrawal.ID.String())

	// Get system account IDs
	// WITHDRAWAL_COMMITTED is where approved withdrawals sit
	withdrawalCommittedID, err := h.ledgerRepo.GetSystemAccountID(ctx, tx, finance.AccountWithdrawalCommitted)
	if err != nil {
		return fmt.Errorf("get withdrawal committed account: %w", err)
	}

	// Get seller's payable account - funds are returned to the seller
	sellerPayableID, err := h.ledgerRepo.GetOrCreateUserAccount(ctx, tx, finance.AccountSellerPayable, withdrawal.SellerID)
	if err != nil {
		return fmt.Errorf("get seller payable account: %w", err)
	}

	// Create ledger transaction entries to return funds
	// DR WITHDRAWAL_COMMITTED (decrease committed - credit/negative)
	// CR user's SELLER_PAYABLE (increase seller's payable - debit/positive)
	entries := []ledgerintf.Entry{
		{AccountID: withdrawalCommittedID, Amount: amountMoney.Neg()}, // Credit (decrease committed)
		{AccountID: sellerPayableID, Amount: amountMoney},             // Debit (restore full amount)
	}

	err = h.ledgerRepo.CreateTransaction(
		ctx,
		tx,
		idempotencyKey,
		"WITHDRAWAL_FAIL_RETURN",
		withdrawal.ID,
		nil, // orderID
		nil, // paymentID
		entries,
	)
	if err != nil {
		return fmt.Errorf("return funds to seller: %w", err)
	}

	h.log.Info("Failed payout funds returned to seller",
		zap.String("withdrawal_id", withdrawal.ID.String()),
		zap.String("seller_id", withdrawal.SellerID.String()),
		zap.Int64("amount", withdrawal.Amount),
		zap.Int64("fee_amount", withdrawal.FeeAmount),
		zap.String("idempotency_key", idempotencyKey),
	)

	// Mark as failed (final - gateway failures are not retryable from callback flow)
	reason := callback.Message
	if reason == "" {
		reason = "Gateway reported failure via webhook"
	}

	err = h.withdrawRepo.MarkFailed(ctx, tx, withdrawal.ID, repository.WithdrawalStatusFailedFinal, reason, callback.RawPayload)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}

	// OUTBOX ATOMIC - emit withdrawal.failed in the same transaction so the
	// seller receives a failure notification with the terminal payout state.
	if h.outboxRepo != nil {
		payload := map[string]interface{}{
			"withdrawal_id": withdrawal.ID.String(),
			"seller_id":     withdrawal.SellerID.String(),
			"amount":        withdrawal.Amount,
			"fee_amount":    withdrawal.FeeAmount,
			"status":        string(repository.WithdrawalStatusFailedFinal),
			"source":        "gateway_webhook",
			"reason":        reason,
		}
		payloadBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return fmt.Errorf("marshal withdrawal.failed payload: %w", marshalErr)
		}
		if err := h.outboxRepo.InsertEvent(ctx, tx, "withdrawal.failed", withdrawal.ID, payloadBytes); err != nil {
			return fmt.Errorf("outbox withdrawal.failed: %w", err)
		}
	}

	h.log.Info("Withdrawal marked as failed via webhook with funds returned",
		zap.String("withdrawal_id", withdrawal.ID.String()),
		zap.String("reason", reason),
	)

	return nil
}

// Error definitions
var (
	// ErrDuplicateCallback is returned when a duplicate callback is received (idempotent)
	ErrDuplicateCallback = fmt.Errorf("duplicate callback ignored")

	// ErrWithdrawalNotFound is returned when withdrawal cannot be found
	ErrWithdrawalNotFound = fmt.Errorf("withdrawal not found")
)

// InvalidStateTransitionError represents an invalid state transition attempt.
type InvalidStateTransitionError struct {
	From string
	To   string
}

func (e *InvalidStateTransitionError) Error() string {
	return fmt.Sprintf("invalid state transition: %s -> %s", e.From, e.To)
}

func (e *InvalidStateTransitionError) Is(target error) bool {
	_, ok := target.(*InvalidStateTransitionError)
	return ok
}
