package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	sellerRepo "github.com/labuda/backend/internal/commerce/seller/repository"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	subscriptionRepo "github.com/labuda/backend/internal/commerce/subscription/repository"
	financeApp "github.com/labuda/backend/internal/finance/application"
	userRepo "github.com/labuda/backend/internal/identity/user/repository"
	paymentRepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
)

var (
	// ErrNoActiveConfig is returned when no active subscription config exists.
	ErrNoActiveConfig = errors.New("no active subscription configuration found")
	// ErrMissingSettlementTimestamp is returned when a settled payment lacks PaidAt.
	ErrMissingSettlementTimestamp = errors.New("settled payment is missing settlement timestamp")
)

// OutboxRepository defines the interface for outbox event operations.
type OutboxRepository interface {
	InsertEvent(
		ctx context.Context,
		tx db.Tx,
		eventType string,
		entityID uuid.UUID,
		payload []byte,
	) error
}

// ConfigRepository defines the interface for subscription config operations.
type ConfigRepository interface {
	GetActiveConfig(ctx context.Context, tx db.Tx) (*subscriptionEntity.SellerSubscriptionConfig, error)
}

// PaymentRepository defines the payment lookups required by the subscription
// activation flow.
type PaymentRepository interface {
	GetByIDForUpdate(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*paymentRepo.Payment, error)
}

// SellerSubscriptionPaymentService handles seller subscription payment activation.
//
// This service implements a ledger-first, idempotent payment activation flow:
// 1. Idempotency check via payment_id
// 2. Onboarding validation (STRICT MODE - no bypass)
// 3. Seller-level lock
// 4. Settled payment validation and activation timestamp lookup
// 5. Chain-end lookup and interval stacking
// 6. Ledger entry creation via FinanceService (DR GATEWAY_CLEARING, CR PLATFORM_REVENUE)
// 7. Subscription row insertion
// 8. Seller profile creation (business identity only)
// 9. Outbox event emission
//
// All operations are atomic within a single transaction.
//
// FINANCIAL BOUNDARY: This service does NOT access ledger directly.
// All ledger operations are routed through FinanceService.
//
// STRICT MODE: Onboarding validation is MANDATORY - no payment without complete onboarding
// SINGLE SOURCE OF TRUTH: Uses shared SellerOnboardingService for validation
type SellerSubscriptionPaymentService struct {
	db                Transactor
	paymentRepo       PaymentRepository
	subRepo           subscriptionRepo.SellerSubscriptionRepository
	sellerRepo        sellerRepo.SellerRepository
	userRepo          userRepo.UserRepository
	onboardingService *SellerOnboardingService
	financeService    *financeApp.FinanceService
	outboxRepo        OutboxRepository
	configRepo        ConfigRepository
}

// NewSellerSubscriptionPaymentService creates a new SellerSubscriptionPaymentService.
func NewSellerSubscriptionPaymentService(
	transactor Transactor,
	paymentRepo PaymentRepository,
	subRepo subscriptionRepo.SellerSubscriptionRepository,
	sellerRepo sellerRepo.SellerRepository,
	userRepo userRepo.UserRepository,
	onboardingService *SellerOnboardingService,
	financeService *financeApp.FinanceService,
	outboxRepo OutboxRepository,
	configRepo ConfigRepository,
) *SellerSubscriptionPaymentService {
	return &SellerSubscriptionPaymentService{
		db:                transactor,
		paymentRepo:       paymentRepo,
		subRepo:           subRepo,
		sellerRepo:        sellerRepo,
		userRepo:          userRepo,
		onboardingService: onboardingService,
		financeService:    financeService,
		outboxRepo:        outboxRepo,
		configRepo:        configRepo,
	}
}

// ProcessSuccessfulPayment processes a successful subscription payment.
//
// This is the public wrapper that opens its own transaction. Use
// ProcessSuccessfulPaymentTx when you are already inside a caller-owned tx
// and need to avoid nested locking.
func (s *SellerSubscriptionPaymentService) ProcessSuccessfulPayment(
	ctx context.Context,
	paymentID uuid.UUID,
	userID uuid.UUID,
	providerEventID string,
) error {
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		return s.ProcessSuccessfulPaymentTx(ctx, tx, paymentID, userID, providerEventID)
	})
}

// ProcessSuccessfulPaymentTx performs the canonical subscription activation
// work inside an existing transaction.
func (s *SellerSubscriptionPaymentService) ProcessSuccessfulPaymentTx(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
	userID uuid.UUID,
	providerEventID string,
) error {
	// Step 0: Lock payment row for the duration of this transaction.
	// Serializes concurrent ProcessSuccessfulPayment calls for the same
	// paymentID (e.g., two reconciliation worker pods racing on the same
	// orphaned payment). The second caller blocks here until the first
	// transaction commits; the idempotency check at step 1 then sees
	// existingCount > 0 and returns nil.
	var lockedPaymentID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT id FROM payments WHERE id = $1 FOR UPDATE
	`, paymentID).Scan(&lockedPaymentID); err != nil {
		return fmt.Errorf("lock payment row: %w", err)
	}

	// Step 1: Idempotency check - check if payment_id already used.
	// Safe to read without additional lock: payment row is locked above.
	var existingCount int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM seller_subscriptions WHERE payment_id = $1
	`, paymentID).Scan(&existingCount)
	if err != nil {
		return fmt.Errorf("idempotency check failed: %w", err)
	}
	if existingCount > 0 {
		// Idempotent success - payment already processed
		return nil
	}

	// Step 2: ONBOARDING VALIDATION - Mandatory gate before subscription
	// STRICT MODE: FAIL HARD - No payment without complete onboarding
	// SINGLE SOURCE OF TRUTH: Uses shared SellerOnboardingService
	if err := s.onboardingService.ValidateOnboarding(ctx, tx, userID); err != nil {
		return err
	}

	// Step 3: Acquire seller-level row lock to serialize distinct renewal
	// payments for the same seller. This is the seller authority mutex.
	sellerProfile, err := s.sellerRepo.GetByUserIDForUpdate(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("lock seller profile failed: %w", err)
	}
	if sellerProfile == nil {
		return fmt.Errorf("seller profile not found for user %s", userID)
	}

	// Step 4: Re-check idempotency after the seller-level lock.
	// This closes the gap where another payment row could be processed
	// between the first count and the seller authority lock.
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM seller_subscriptions WHERE payment_id = $1
	`, paymentID).Scan(&existingCount)
	if err != nil {
		return fmt.Errorf("idempotency re-check failed: %w", err)
	}
	if existingCount > 0 {
		return nil
	}

	// Step 5: Load the settled payment row and anchor the new interval to
	// the payment activation timestamp.
	if s.paymentRepo == nil {
		return fmt.Errorf("payment repository not configured")
	}
	payment, err := s.paymentRepo.GetByIDForUpdate(ctx, tx, paymentID)
	if err != nil {
		return fmt.Errorf("load payment failed: %w", err)
	}
	if payment == nil {
		return fmt.Errorf("payment not found: %s", paymentID)
	}
	if !payment.IsSettled() {
		return fmt.Errorf("payment %s is not settled", paymentID)
	}
	if payment.PaidAt == nil {
		return ErrMissingSettlementTimestamp
	}

	activationAt := *payment.PaidAt
	now := time.Now()

	// Step 6: Get active config for pricing snapshot (yearly fee only)
	config, err := s.configRepo.GetActiveConfig(ctx, tx)
	if err != nil {
		return fmt.Errorf("get active config failed: %w", err)
	}
	if config == nil {
		return ErrNoActiveConfig
	}

	// Step 7: Compute the new entitlement interval from the current chain end.
	var newStartedAt, newExpiresAt time.Time
	duration := time.Duration(config.DurationDays) * 24 * time.Hour

	chainEnd, err := s.subRepo.GetLatestByUserIDForUpdate(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("load entitlement chain end failed: %w", err)
	}

	newStartedAt = activationAt
	if chainEnd != nil && chainEnd.ExpiresAt.After(newStartedAt) {
		newStartedAt = chainEnd.ExpiresAt
	}
	newExpiresAt = newStartedAt.Add(duration)

	// Step 8: Create ledger entry FIRST via FinanceService
	// DR GATEWAY_CLEARING, CR PLATFORM_REVENUE
	// FINANCIAL BOUNDARY: Subscription domain does NOT build ledger entries directly
	if err := s.financeService.RecordSubscriptionRevenue(ctx, tx, paymentID, config.YearlyFeeRupiah, providerEventID); err != nil {
		return fmt.Errorf("create ledger entry failed: %w", err)
	}

	// Step 9: Insert subscription row with canonical duration.
	subscription := &subscriptionEntity.SellerSubscription{
		ID:           uuid.New(),
		UserID:       userID,
		Status:       subscriptionEntity.StatusActive,
		StartedAt:    newStartedAt,
		ExpiresAt:    newExpiresAt,
		DurationDays: config.DurationDays,
		AmountPaid:   money.New(config.YearlyFeeRupiah),
		Currency:     "IDR",
		PaymentID:    paymentID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.subRepo.InsertTx(ctx, tx, subscription); err != nil {
		return fmt.Errorf("insert subscription failed: %w", err)
	}

	// Step 10: Ensure seller profile exists (for business identity)
	_, err = s.sellerRepo.EnsureProfileExistsTx(ctx, tx, userID, "")
	if err != nil {
		return fmt.Errorf("ensure seller profile failed: %w", err)
	}

	// Step 11: Emit outbox event
	if err := s.emitActivationEvent(ctx, tx, subscription); err != nil {
		return fmt.Errorf("emit outbox event failed: %w", err)
	}

	return nil
}

// emitActivationEvent emits an outbox event for subscription activation.
func (s *SellerSubscriptionPaymentService) emitActivationEvent(
	ctx context.Context,
	tx db.Tx,
	subscription *subscriptionEntity.SellerSubscription,
) error {
	payload := map[string]interface{}{
		"subscription_id": subscription.ID,
		"user_id":         subscription.UserID,
		"payment_id":      subscription.PaymentID,
		"started_at":      subscription.StartedAt.Format(time.RFC3339),
		"expires_at":      subscription.ExpiresAt.Format(time.RFC3339),
		"amount_paid":     subscription.AmountPaid.Int64(),
		"currency":        subscription.Currency,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload failed: %w", err)
	}

	// Static event type — subscription_id is in the payload.
	// Idempotency: InsertEvent builds key as "seller.subscription.activated.<subscription.ID>"
	return s.outboxRepo.InsertEvent(ctx, tx, "seller.subscription.activated", subscription.ID, payloadBytes)
}

// validateOnboarding validates that the user has completed seller onboarding.
//
// STRICT MODE: FAIL HARD - No payment without complete onboarding
// This prevents fake seller entry by ensuring:
// - Email is verified
// - Username exists
// - Phone number exists (phone_number IS NOT NULL)
// - Address exists (user_profiles.location IS NOT NULL)
// - Seller profile exists (seller_profile must be created first)
//
// MARKET AUTHORITY ENFORCEMENT:
// - Seller profile must exist before subscription payment
// - This ensures seller identity is created before market authority is granted
//
// SEPARATION OF CONCERNS:
// - Identity creation (seller_profile) happens during onboarding
// - Market authority (seller_subscription) happens during payment
// This ensures deterministic behavior and no hidden side effects.
//
