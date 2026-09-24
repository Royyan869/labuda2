package http

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	orderrepoimpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	ratingApp "github.com/labuda/backend/internal/commerce/order/rating/application"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	paymentmethodentity "github.com/labuda/backend/internal/commerce/paymentmethod/entity"
	paymentmethodrepo "github.com/labuda/backend/internal/commerce/paymentmethod/infrastructure/repository"
	"github.com/labuda/backend/internal/commerce/seller/entity"
	sellerRepo "github.com/labuda/backend/internal/commerce/seller/repository"
	subscriptionApp "github.com/labuda/backend/internal/commerce/subscription/application"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	subscriptionRepo "github.com/labuda/backend/internal/commerce/subscription/repository"
	financeapp "github.com/labuda/backend/internal/finance/application"
	financeRepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
	userRepo "github.com/labuda/backend/internal/identity/user/repository"
	paymentRepository "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// SellerHandler handles HTTP requests for seller profile and subscription read operations.
//
// LEDGER LOCKDOWN: seller domain CANNOT access ledger directly
// All ledger operations MUST go through FinanceService
//
// RATING DOMAIN BOUNDARY: seller domain CANNOT access order_ratings table directly
// All rating operations MUST go through RatingReader interface (read-only access)
//
// SINGLE SOURCE OF TRUTH: Uses shared SellerOnboardingService for validation
type SellerHandler struct {
	subscriptionService *subscriptionApp.SubscriptionService
	db                  *db.DB
	log                 *zap.Logger
	financeService      *financeapp.FinanceService
	orderRepo           orderrepository.OrderRepository
	withdrawRepo        *financeRepo.WithdrawRepository
	ratingReader        ratingApp.RatingReader // Interface-based access (read-only)
	userRepo            userRepo.UserRepository
	sellerRepo          sellerRepo.SellerRepository
	onboardingService   *subscriptionApp.SellerOnboardingService // ← SINGLE SOURCE OF TRUTH

	// Subscription payment initiation deps
	paymentRepo       subscriptionPaymentRepository
	paymentMethodRepo paymentMethodRepository
	midtransClient    snapTransactionClient
	subRepo           subscriptionRepo.SellerSubscriptionRepository
	frontendURL       string

	// withdrawalFeeProvider returns the canonical configured seller withdrawal
	// fee (admin-configurable, Rp0 allowed) for the seller earnings display
	// surface. Wired post-construction via SetWithdrawalFeeProvider.
	withdrawalFeeProvider financeapp.WithdrawalFeeProvider
}

type subscriptionPaymentRepository interface {
	FindPendingSubscriptionPayment(ctx context.Context, tx db.Tx, userID uuid.UUID) (*paymentRepository.Payment, error)
	CreatePayment(ctx context.Context, tx db.Tx, input paymentRepository.CreatePaymentInput) (*paymentRepository.Payment, error)
	UpdatePaymentURL(ctx context.Context, tx db.Tx, paymentID uuid.UUID, paymentURL string) error
}

// paymentMethodRepository defines the canonical payment method lookups
// required by subscription payment creation (PMF-02).
type paymentMethodRepository interface {
	GetByCode(ctx context.Context, tx db.Tx, code string) (*paymentmethodentity.Method, error)
	ListEnabled(ctx context.Context, tx db.Tx) ([]paymentmethodentity.Method, error)
}

type snapTransactionClient interface {
	CreateSnapTransaction(req *midtrans.SnapRequest) (*midtrans.SnapResponse, error)
}

// NewSellerHandler creates a new SellerHandler.
func NewSellerHandler(
	subscriptionService *subscriptionApp.SubscriptionService,
	db *db.DB,
	log *zap.Logger,
	userRepo userRepo.UserRepository,
	sellerRepo sellerRepo.SellerRepository,
	onboardingService *subscriptionApp.SellerOnboardingService,
	paymentRepo *paymentRepository.PaymentRepository,
	midtransClient *midtrans.Client,
	subRepo subscriptionRepo.SellerSubscriptionRepository,
	frontendURL string,
) *SellerHandler {
	if log == nil {
		log = zap.NewNop()
	}

	// RATING DOMAIN BOUNDARY: Use factory to get rating reader interface
	ratingFactory := ratingApp.NewRatingDomainFactory()

	return &SellerHandler{
		subscriptionService:        subscriptionService,
		db:                         db,
		log:                        log,
		financeService:             financeapp.NewFinanceService(),
		orderRepo:                  orderrepoimpl.NewOrderRepository(),
		withdrawRepo:               financeRepo.NewWithdrawRepository(),
		ratingReader:               ratingFactory.GetReader(),
		userRepo:                   userRepo,
		sellerRepo:                 sellerRepo,
		onboardingService:          onboardingService,
		paymentRepo:                paymentRepo,
		midtransClient:             midtransClient,
		subRepo:                    subRepo,
		frontendURL:                frontendURL,
	}
}

// SetPaymentMethodRepository wires the canonical payment method authority
// into the subscription payment initiation path (PMF-02). Must be called
// at boot; nil → subscription initiation rejects all requests.
func (h *SellerHandler) SetPaymentMethodRepository(repo paymentMethodRepository) {
	h.paymentMethodRepo = repo
}

// SetWithdrawalFeeProvider wires the canonical configured seller withdrawal
// fee authority into the seller earnings surface (same authority as
// WithdrawService). Must be called at boot; nil → earnings fail-closes.
func (h *SellerHandler) SetWithdrawalFeeProvider(p financeapp.WithdrawalFeeProvider) {
	h.withdrawalFeeProvider = p
}

// SellerProfileResponse represents the seller profile response.
type SellerProfileResponse struct {
	ID                   uuid.UUID   `json:"id"`
	UserID               uuid.UUID   `json:"user_id"`
	StoreName            string      `json:"store_name"`
	StoreImageURL        *string     `json:"store_image_url"`
	StoreImageUpdatedAt  *string     `json:"store_image_updated_at,omitempty"`
	Tier                 entity.Tier `json:"tier"`
	CreatedAt            string      `json:"created_at"`
	UpdatedAt            string      `json:"updated_at"`
}

// SubscriptionResponse represents the seller subscription response.
type SubscriptionResponse struct {
	ID           uuid.UUID                 `json:"id"`
	UserID       uuid.UUID                 `json:"user_id"`
	Status       subscriptionEntity.Status `json:"status"`
	StartedAt    string                    `json:"started_at"`
	ExpiresAt    string                    `json:"expires_at"`
	DurationDays int                       `json:"duration_days"`
	AmountPaid   int64                     `json:"amount_paid"`
	Currency     string                    `json:"currency"`
	PaymentID    uuid.UUID                 `json:"payment_id"`
	CreatedAt    string                    `json:"created_at"`
	UpdatedAt    string                    `json:"updated_at"`
}

// SubscriptionConfigResponse represents the seller subscription pricing config.
type SubscriptionConfigResponse struct {
	ID                  uuid.UUID `json:"id"`
	YearlyFeeRupiah     int64     `json:"yearly_fee_rupiah"`
	DurationDays        int       `json:"duration_days"`
	RenewalReminderDays int       `json:"renewal_reminder_days"`
	Enabled             bool      `json:"enabled"`
	CreatedAt           string    `json:"created_at"`
}

func subscriptionConfigToResponse(
	config *subscriptionEntity.SellerSubscriptionConfig,
) *SubscriptionConfigResponse {
	return &SubscriptionConfigResponse{
		ID:                  config.ID,
		YearlyFeeRupiah:     config.YearlyFeeRupiah,
		DurationDays:        config.DurationDays,
		RenewalReminderDays: config.RenewalReminderDays,
		Enabled:             config.Enabled,
		CreatedAt:           config.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// OnboardingRequest represents the seller onboarding request.
type OnboardingRequest struct {
	StoreName     string  `json:"store_name" binding:"required"`
	StoreImageURL *string `json:"store_image_url"`
}

// OnboardingResponse represents the seller onboarding response.
type OnboardingResponse struct {
	ProfileID            uuid.UUID   `json:"profile_id"`
	UserID               uuid.UUID   `json:"user_id"`
	StoreName            string      `json:"store_name"`
	Tier                 entity.Tier `json:"tier"`
	RequiresVerification []string    `json:"requires_verification"`
}

// GetProfile handles GET /api/v1/seller/profile
//
// Returns the seller profile for the authenticated user.
// If the user has no seller profile, returns 404.
func (h *SellerHandler) GetProfile(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Get seller profile using SubscriptionService
	profile, err := h.subscriptionService.GetSellerProfile(ctx, userID)
	if err != nil {
		h.log.Error("Failed to get seller profile",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve seller profile")
		return
	}

	if profile == nil {
		response.NotFound(c, "Seller profile not found")
		return
	}

	// Map to response DTO
	var imageUpdatedAt *string
	if profile.StoreImageUpdatedAt != nil {
		s := profile.StoreImageUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		imageUpdatedAt = &s
	}
	resp := SellerProfileResponse{
		ID:                  profile.ID,
		UserID:              profile.UserID,
		StoreName:           profile.StoreName,
		StoreImageURL:       profile.StoreImageURL,
		StoreImageUpdatedAt: imageUpdatedAt,
		Tier:                profile.Tier,
		CreatedAt:           profile.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           profile.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	response.Success(c, resp)
}

// validateSellerStoreImageStorageKey validates the canonical store image storage key.
// Canonical form is exactly "images/stores/{userID}.jpg" owned by the caller.
// Absolute URLs (https://...), path traversal, or cross-user keys are rejected.
func validateSellerStoreImageStorageKey(raw string, userID uuid.UUID) error {
	expected := "images/stores/" + userID.String() + ".jpg"
	if raw != expected {
		return fmt.Errorf("store_image_url must be %s", expected)
	}
	// Defensive: reject traversal even though exact match already covers it.
	if len(raw) >= 2 {
		for i := 0; i+1 < len(raw); i++ {
			if raw[i] == '.' && raw[i+1] == '.' {
				return fmt.Errorf("store_image_url must be %s", expected)
			}
		}
	}
	return nil
}

// UpdateProfileRequest is the PATCH /seller/profile request body.
type UpdateProfileRequest struct {
	StoreName     *string `json:"store_name"`
	StoreImageURL *string `json:"store_image_url"`
}

// UpdateProfile handles PATCH /api/v1/seller/profile
//
// Canonical seller/store identity writer. Updates store_name and/or store_image_url atomically.
// Requires an existing seller profile (must have onboarded). Validates canonical storage key
// "images/stores/{userID}.jpg" for image (not absolute URL).
// On success bumps store_image_updated_at when image changes.
func (h *SellerHandler) UpdateProfile(c *gin.Context) {
	ctx := c.Request.Context()
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if req.StoreName == nil && req.StoreImageURL == nil {
		response.BadRequest(c, "store_name or store_image_url is required")
		return
	}
	if req.StoreName != nil && *req.StoreName == "" {
		response.BadRequest(c, "store_name is required")
		return
	}
	if req.StoreImageURL != nil && *req.StoreImageURL != "" {
		if err := validateSellerStoreImageStorageKey(*req.StoreImageURL, userID); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}
	var updated *entity.SellerProfile
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		p, err := h.sellerRepo.UpdateSellerProfileTx(ctx, tx, userID, req.StoreName, req.StoreImageURL)
		if err != nil {
			return err
		}
		updated = p
		return nil
	})
	if err != nil {
		if err.Error() == "seller profile not found" {
			response.NotFound(c, "Seller profile not found")
			return
		}
		if err.Error() == "store_name is required" || strings.HasPrefix(err.Error(), "store_image_url must") {
			response.BadRequest(c, err.Error())
			return
		}
		h.log.Error("Failed to update seller profile", zap.String("user_id", userID.String()), zap.Error(err))
		response.InternalServerError(c, "Failed to update seller profile")
		return
	}
	var imageUpdatedAt *string
	if updated.StoreImageUpdatedAt != nil {
		s := updated.StoreImageUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		imageUpdatedAt = &s
	}
	resp := SellerProfileResponse{
		ID:                  updated.ID,
		UserID:              updated.UserID,
		StoreName:           updated.StoreName,
		StoreImageURL:       updated.StoreImageURL,
		StoreImageUpdatedAt: imageUpdatedAt,
		Tier:                updated.Tier,
		CreatedAt:           updated.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           updated.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	response.Success(c, resp)
}

// Onboarding handles POST /api/v1/seller/onboarding
//
// Creates or retrieves a seller profile for the authenticated user.
//
// STRICT MODE IDEMPOTENCY:
// - If profile exists: Return existing profile (IGNORE new store_name)
// - If profile doesn't exist: Create new profile after validation
// - Multiple concurrent requests with different store_name: First one wins
//
// Validation Requirements:
// - verified email
// - username exists
// - phone_number exists: User must have a phone number set
// - address exists: User must have a location set in their profile
//
// Returns:
// - 200: Profile created or already exists
// - 400: Validation failed with missing requirements
// - 401: User not authenticated
// - 500: Server error
//
// SINGLE SOURCE OF TRUTH: Uses shared SellerOnboardingService for validation
// ROW LOCKING: Uses GetByUserIDForUpdate to prevent race conditions
func (h *SellerHandler) Onboarding(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body
	var req OnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	// Validate store_name
	if req.StoreName == "" {
		response.BadRequest(c, "store_name is required")
		return
	}
	// Validate store_image_url if provided (optional field, null is valid)
	// Canonical: storage key "images/stores/{userID}.jpg", not absolute URL.
	if req.StoreImageURL != nil && *req.StoreImageURL != "" {
		if err := validateSellerStoreImageStorageKey(*req.StoreImageURL, userID); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}

	var existingProfile *entity.SellerProfile
	var missingRequirements []string

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		// STRICT MODE: Lock seller profile row to prevent concurrent onboarding
		// Step 1: Check if seller profile already exists (IDEMPOTENCY FIRST)
		profile, err := h.sellerRepo.GetByUserIDForUpdate(ctx, tx, userID)
		if err != nil {
			return err
		}
		existingProfile = profile

		// Step 2: If no existing profile, validate onboarding requirements
		// SINGLE SOURCE OF TRUTH: Uses shared SellerOnboardingService (already locks user row)
		if existingProfile == nil {
			missingRequirements = h.onboardingService.ValidateOnboardingWithoutProfile(ctx, tx, userID)

			// Step 3: If validation passed, create or fetch the seller profile
			// atomically. ON CONFLICT DO NOTHING prevents a race from turning
			// into a duplicate-key 500 when multiple requests validate at once.
			if len(missingRequirements) == 0 {
				profile, err := h.sellerRepo.EnsureProfileExistsTxWithImage(ctx, tx, userID, req.StoreName, req.StoreImageURL)
				if err != nil {
					return err
				}

				// Step 4: Seed the seller_verifications lifecycle row at
				// status=not_submitted. Become Seller opens selling authority
				// (subscription gate); payout authority requires a separate
				// verification approval. Seeding here guarantees the
				// canonical lifecycle row exists exactly once per seller and
				// keeps the WithdrawService gate well-defined (it reads
				// status=approved). ON CONFLICT DO NOTHING makes the seed
				// idempotent against any future path that may also seed.
				if _, err := tx.Exec(ctx, `
					INSERT INTO seller_verifications (seller_id, status)
					VALUES ($1, 'not_submitted')
					ON CONFLICT (seller_id) DO NOTHING
				`, userID); err != nil {
					return err
				}

				existingProfile = profile
			}
		}

		return nil
	})

	if err != nil {
		h.log.Error("Failed to process seller onboarding",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to process onboarding")
		return
	}

	// If there are missing requirements, return them
	if len(missingRequirements) > 0 {
		resp := OnboardingResponse{
			RequiresVerification: missingRequirements,
		}
		response.ErrorWithDetails(c, 400, "MISSING_REQUIREMENTS", "Missing verification requirements", resp)
		return
	}

	// Return success with profile details
	resp := OnboardingResponse{
		ProfileID:            existingProfile.ID,
		UserID:               existingProfile.UserID,
		StoreName:            existingProfile.StoreName,
		Tier:                 existingProfile.Tier,
		RequiresVerification: []string{},
	}

	response.Success(c, resp)
}

// GetSubscription handles GET /api/v1/seller/subscription
//
// Returns the active subscription for the authenticated user.
// If the user has no active subscription, returns 404.
func (h *SellerHandler) GetSubscription(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Get active subscription using SubscriptionService
	subscription, err := h.subscriptionService.GetActiveSubscription(ctx, userID)
	if err != nil {
		h.log.Error("Failed to get seller subscription",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve seller subscription")
		return
	}

	if subscription == nil {
		response.NotFound(c, "No active subscription found")
		return
	}

	// Map to response DTO
	resp := SubscriptionResponse{
		ID:           subscription.ID,
		UserID:       subscription.UserID,
		Status:       subscription.Status,
		StartedAt:    subscription.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
		ExpiresAt:    subscription.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		DurationDays: subscription.DurationDays,
		AmountPaid:   subscription.AmountPaid.Int64(),
		Currency:     subscription.Currency,
		PaymentID:    subscription.PaymentID,
		CreatedAt:    subscription.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    subscription.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	response.Success(c, resp)
}

// GetSubscriptionConfig handles GET /api/v1/seller/subscription/config
//
// Returns the currently active seller subscription config for authenticated users.
// This powers the seller onboarding package disclosure step.
func (h *SellerHandler) GetSubscriptionConfig(c *gin.Context) {
	ctx := c.Request.Context()

	var config *SubscriptionConfigResponse
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		entity, err := h.subRepo.GetActiveConfig(ctx, tx)
		if err != nil {
			return err
		}
		if entity != nil {
			config = subscriptionConfigToResponse(entity)
		}
		return nil
	})
	if err != nil {
		h.log.Error("Failed to get seller subscription config", zap.Error(err))
		response.InternalServerError(c, "Failed to retrieve subscription config")
		return
	}
	if config == nil {
		response.NotFound(c, "No active subscription config found")
		return
	}

	response.Success(c, gin.H{"config": config})
}

// ============================================================================
// SUBSCRIPTION PAYMENT INITIATION
// ============================================================================

// InitiateSubscriptionPaymentRequest is the request body for subscription
// payment initiation. payment_method_code is REQUIRED — there is no silent
// default, no first-enabled-method fallback, and no backend auto-selection.
type InitiateSubscriptionPaymentRequest struct {
	PaymentMethodCode string `json:"payment_method_code" binding:"required"`
}

// InitiateSubscriptionPaymentResponse represents the response for initiating
// a subscription payment via Midtrans Snap.
type InitiateSubscriptionPaymentResponse struct {
	PaymentID   uuid.UUID `json:"payment_id"`
	PaymentURL  string    `json:"payment_url"`
	GrossAmount int64     `json:"gross_amount"`
	ExpiredAt   string    `json:"expired_at"`
}

func buildInitiateSubscriptionPaymentResponse(payment *paymentRepository.Payment, paymentURL string) *InitiateSubscriptionPaymentResponse {
	return &InitiateSubscriptionPaymentResponse{
		PaymentID:   payment.ID,
		PaymentURL:  paymentURL,
		GrossAmount: payment.GrossAmount.Int64(),
		ExpiredAt:   payment.ExpiredAt.Format(time.RFC3339),
	}
}

// writeSubscriptionInitiationError maps canonical subscription-initiation
// errors to the correct HTTP status. It returns true when it has written a
// response, and false when the error is not a recognized client-input error
// (the caller then logs and returns 500).
//
// PMF-02: an unknown or disabled payment method is client input and must be
// 400, mirroring the canonical CreatePayment (PASS_18V) and billing top-up
// handlers. Before this mapping both cases collapsed into 500 with the
// sentinel discarded by a non-wrapping error.
func writeSubscriptionInitiationError(c *gin.Context, err error, paymentMethodCode string) bool {
	switch {
	case errors.Is(err, paymentmethodrepo.ErrMethodNotFound):
		response.BadRequest(c, fmt.Sprintf("Unknown payment method: %s", paymentMethodCode))
		return true
	case errors.Is(err, paymentmethodentity.ErrMethodDisabled):
		response.BadRequest(c, fmt.Sprintf("Payment method is disabled: %s", paymentMethodCode))
		return true
	default:
		return false
	}
}

func (h *SellerHandler) initiateSubscriptionPaymentTx(c *gin.Context, ctx context.Context, tx db.Tx, userID uuid.UUID, paymentMethodCode string) (*InitiateSubscriptionPaymentResponse, error) {
	// Step 1: Validate onboarding — seller_profile must exist.
	// Mobile calls POST /seller/onboarding first (identity before authority).
	if err := h.onboardingService.ValidateOnboarding(ctx, tx, userID); err != nil {
		return nil, err
	}

	// Step 2: Load active subscription config (admin-seeded principal)
	config, err := h.subRepo.GetActiveConfig(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("get active config: %w", err)
	}
	if config == nil {
		response.Error(c, 503, "NO_ACTIVE_CONFIG",
			"Konfigurasi langganan tidak tersedia")
		return nil, nil
	}

	// Step 2b: Load and validate payment method (PMF-02).
	// payment_method_code is REQUIRED — no silent default, no fallback.
	// Failures are wrapped with the canonical sentinels so the HTTP layer can
	// map unknown/disabled methods to 400 (the PMF-01 and PASS_18V pattern)
	// instead of collapsing every failure into a 500.
	if h.paymentMethodRepo == nil {
		return nil, fmt.Errorf("payment method repository not configured")
	}
	method, err := h.paymentMethodRepo.GetByCode(ctx, tx, paymentMethodCode)
	if err != nil {
		if errors.Is(err, paymentmethodrepo.ErrMethodNotFound) {
			return nil, fmt.Errorf("%w: %s", paymentmethodrepo.ErrMethodNotFound, paymentMethodCode)
		}
		return nil, fmt.Errorf("load payment method: %w", err)
	}
	if !method.Enabled {
		return nil, fmt.Errorf("%w: %s", paymentmethodentity.ErrMethodDisabled, method.Code)
	}

	// Step 2c: Calculate payment-method fee from canonical authority.
	// F = CalculateFee(A, method), where A = subscription principal.
	principal := config.YearlyFeeRupiah
	fee, err := paymentmethodentity.CalculateFee(money.New(principal), *method)
	if err != nil {
		return nil, fmt.Errorf("calculate payment fee: %w", err)
	}

	// Step 3: Idempotency — return existing pending payment if found.
	existingPayment, err := h.paymentRepo.FindPendingSubscriptionPayment(ctx, tx, userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find pending subscription payment: %w", err)
	}

	if existingPayment != nil {
		if existingPayment.PaymentURL != nil && *existingPayment.PaymentURL != "" {
			return buildInitiateSubscriptionPaymentResponse(existingPayment, *existingPayment.PaymentURL), nil
		}

		// Existing pending payment — reuse its immutable snapshot for gateway.
		amountIDR := float64(existingPayment.GrossAmount.Int64())
		expiryMinutes := int(time.Until(existingPayment.ExpiredAt).Minutes())
		if expiryMinutes < 1 {
			expiryMinutes = 1
		}
		if expiryMinutes > 1440 {
			expiryMinutes = 1440
		}

		snapReq := &midtrans.SnapRequest{
			TransactionDetails: midtrans.TransactionDetails{
				OrderID:     existingPayment.MidtransOrderID,
				GrossAmount: amountIDR,
			},
			ItemDetails: []midtrans.ItemDetail{
				{
					ID:       "seller_subscription",
					Price:    amountIDR,
					Quantity: 1,
					Name:     "Langganan Penjual 1 Tahun",
				},
			},
			Expiry: &midtrans.Expiry{
				Unit:     "minute",
				Duration: expiryMinutes,
			},
			// Snap channel restriction comes ONLY from the canonical method row.
			// Without this the reused payment's Snap page would offer every
			// merchant-enabled channel, not the bucket of the method the seller
			// selected (see CorePaymentHandler.createMidtransTransaction).
			EnabledPayments: method.MidtransChannels,
		}
		if h.frontendURL != "" {
			snapReq.Callbacks = &midtrans.Callbacks{
				Finish: h.frontendURL + "/payment/finish",
			}
		}

		snapResp, err := h.midtransClient.CreateSnapTransaction(snapReq)
		if err != nil {
			return nil, fmt.Errorf("midtrans snap: %w", err)
		}

		if err := h.paymentRepo.UpdatePaymentURL(ctx, tx, existingPayment.ID, snapResp.RedirectURL); err != nil {
			return nil, fmt.Errorf("update payment URL: %w", err)
		}

		return buildInitiateSubscriptionPaymentResponse(existingPayment, snapResp.RedirectURL), nil
	}

	// Step 4: Create payment row with immutable snapshot (PMF-02).
	// gross_amount = A + F, service_fee_amount = F, payment_method_code = M.
	paymentNumber := fmt.Sprintf("PAY-SUB-%d", time.Now().UnixNano())
	midtransOrderID := fmt.Sprintf("LAB-SUB-%s", uuid.New().String())
	grossAmount := money.New(principal).Add(fee) // A + F
	expiredAt := time.Now().Add(24 * time.Hour) // 24h payment window

	refID := userID // reference_id = userID for subscriptions
	payment, err := h.paymentRepo.CreatePayment(ctx, tx, paymentRepository.CreatePaymentInput{
		UserID:           userID,
		PaymentNumber:    paymentNumber,
		MidtransOrderID:  midtransOrderID,
		GrossAmount:      grossAmount,
		ServiceFeeAmount: fee,
		CoinsToUse:       0,
		ReferenceType:    paymentRepository.ReferenceTypeSubscription,
		ReferenceID:      &refID,
		ExpiredAt:        expiredAt,
		PaymentMethodCode: &paymentMethodCode,
	})
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	// Step 5: Build Midtrans Snap request from payment snapshot (PMF-02).
	// Gateway gross MUST equal payment.GrossAmount — NOT config directly.
	amountIDR := float64(payment.GrossAmount.Int64())
	expiryMinutes := int(time.Until(expiredAt).Minutes())
	if expiryMinutes < 1 {
		expiryMinutes = 1
	}
	if expiryMinutes > 1440 {
		expiryMinutes = 1440
	}

	snapReq := &midtrans.SnapRequest{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:     midtransOrderID,
			GrossAmount: amountIDR,
		},
		ItemDetails: []midtrans.ItemDetail{
			{
				ID:       "seller_subscription",
				Price:    amountIDR,
				Quantity: 1,
				Name:     "Langganan Penjual 1 Tahun",
			},
		},
		Expiry: &midtrans.Expiry{
			Unit:     "minute",
			Duration: expiryMinutes,
		},
		// Snap channel restriction comes ONLY from the canonical method row:
		// the Snap page must offer exactly the MidtransChannels bucket of the
		// method the seller selected, never every merchant-enabled channel
		// (mirrors CorePaymentHandler.createMidtransTransaction).
		EnabledPayments: method.MidtransChannels,
	}
	if h.frontendURL != "" {
		snapReq.Callbacks = &midtrans.Callbacks{
			Finish: h.frontendURL + "/payment/finish",
		}
	}

	snapResp, err := h.midtransClient.CreateSnapTransaction(snapReq)
	if err != nil {
		return nil, fmt.Errorf("midtrans snap: %w", err)
	}

	// Step 6: Store redirect URL on payment row
	if err := h.paymentRepo.UpdatePaymentURL(ctx, tx, payment.ID, snapResp.RedirectURL); err != nil {
		return nil, fmt.Errorf("update payment URL: %w", err)
	}

	return buildInitiateSubscriptionPaymentResponse(payment, snapResp.RedirectURL), nil
}

// InitiateSubscriptionPayment handles POST /api/v1/seller/subscription/initiate
//
// Creates a Midtrans Snap payment for a seller subscription purchase or renewal.
//
// Flow:
// 1. Validates onboarding (seller_profile must already exist via POST /seller/onboarding)
// 2. Loads active subscription config (admin-seeded principal A)
// 3. Loads and validates the REQUIRED payment_method_code (unknown/disabled → 400)
// 4. Calculates the canonical payment-method fee F = CalculateFee(A, method)
// 5. Returns the existing pending payment if one exists (idempotent reuse of its
//    immutable M1/F1 snapshot — a newly selected method does NOT supersede it)
// 6. Creates payment row + Midtrans Snap token with the A+F snapshot
// 7. Returns payment_url for client redirect
//
// Renewal is intentionally not window-gated here: the canonical renewal
// stacking logic lives in SubscriptionPaymentService, which appends the new
// entitlement interval at the chain end.
//
// The actual subscription activation happens asynchronously via Midtrans webhook
// → PaymentWebhookService STEP 8f → SubscriptionPaymentService.ProcessSuccessfulPayment.
func (h *SellerHandler) InitiateSubscriptionPayment(c *gin.Context) {
	ctx := c.Request.Context()

	// Auth: extract userID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse request body — payment_method_code is REQUIRED (PMF-02).
	var req InitiateSubscriptionPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "payment_method_code is required")
		return
	}

	var result *InitiateSubscriptionPaymentResponse

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		initResult, err := h.initiateSubscriptionPaymentTx(c, ctx, tx, userID, req.PaymentMethodCode)
		if err != nil {
			var onboardingErr *subscriptionApp.ErrOnboardingIncomplete
			if errors.As(err, &onboardingErr) {
				response.ErrorWithDetails(c, 400, "MISSING_REQUIREMENTS",
					"Missing verification requirements",
					map[string]interface{}{"requires_verification": onboardingErr.MissingRequirements})
				return nil
			}
			return err
		}

		result = initResult
		return nil
	})

	if err != nil {
		// Canonical input-validation mapping: an unknown or disabled payment
		// method is a client error, not a server fault. The sentinels are the
		// same ones used by the canonical CreatePayment / top-up handlers.
		if writeSubscriptionInitiationError(c, err, req.PaymentMethodCode) {
			return
		}
		h.log.Error("Failed to initiate subscription payment",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to initiate subscription payment")
		return
	}

	// If result is nil, the response was already sent inside the tx (error responses)
	if result != nil {
		response.Success(c, result)
	}
}

// ============================================================================
// SUBSCRIPTION PAYMENT METHOD DISCLOSURE (PMF-02)
// ============================================================================

// SubscriptionPaymentMethodOption is one selectable payment method for a
// subscription payment, carrying the backend-calculated fee and gross amount
// for the CURRENT active subscription principal A. Vocabulary mirrors the
// payment snapshot columns (service_fee_amount, gross_amount) so the client
// renders exactly what will be snapshotted at initiation.
type SubscriptionPaymentMethodOption struct {
	MethodCode       string `json:"method_code"`
	DisplayName      string `json:"display_name"`
	ServiceFeeAmount int64  `json:"service_fee_amount"`
	GrossAmount      int64  `json:"gross_amount"`
}

// SubscriptionPaymentMethodsResponse is the disclosure payload for the seller
// method picker: the principal A plus every enabled method with F and A+F.
type SubscriptionPaymentMethodsResponse struct {
	PrincipalAmount int64                             `json:"principal_amount"`
	Currency        string                            `json:"currency"`
	Methods         []SubscriptionPaymentMethodOption `json:"methods"`
}

// buildSubscriptionPaymentMethodOptions computes, per enabled method, the
// canonical fee F = CalculateFee(A, method) and the resulting gross A+F for the
// subscription principal A. Methods with an invalid fee formula are skipped
// (onInvalid receives the code and cause) rather than failing the whole list,
// matching GET /payments/methods. Pure function, so the fee math is provable
// without a database.
func buildSubscriptionPaymentMethodOptions(
	principal money.Money,
	methods []paymentmethodentity.Method,
	onInvalid func(code string, err error),
) []SubscriptionPaymentMethodOption {
	options := make([]SubscriptionPaymentMethodOption, 0, len(methods))
	for _, m := range methods {
		fee, err := paymentmethodentity.CalculateFee(principal, m)
		if err != nil {
			if onInvalid != nil {
				onInvalid(m.Code, err)
			}
			continue
		}
		options = append(options, SubscriptionPaymentMethodOption{
			MethodCode:       m.Code,
			DisplayName:      m.DisplayName,
			ServiceFeeAmount: fee.Int64(),
			GrossAmount:      principal.Add(fee).Int64(),
		})
	}
	return options
}

// GetSubscriptionPaymentMethods handles GET /api/v1/seller/subscription/payment-methods
//
// This is the subscription-scoped sibling of GET /payments/methods (which is
// order-scoped and computes the fee on an order's buyer base). The seller must
// explicitly choose a method before POST /seller/subscription/initiate, and the
// client never computes a fee — the backend is the sole fee authority.
//
// Fee authority: F = CalculateFee(A, method) where A = the active
// seller_subscription_configs.yearly_fee_rupiah. Disabled methods are excluded
// by ListEnabled, so a method returned here is always acceptable to initiation.
//
// Requires an authenticated account only (no seller authority, no onboarding)
// so the upgrade wizard can render the picker before onboarding completes.
func (h *SellerHandler) GetSubscriptionPaymentMethods(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var result *SubscriptionPaymentMethodsResponse

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		config, err := h.subRepo.GetActiveConfig(ctx, tx)
		if err != nil {
			return fmt.Errorf("get active config: %w", err)
		}
		if config == nil {
			response.Error(c, 503, "NO_ACTIVE_CONFIG",
				"Konfigurasi langganan tidak tersedia")
			return nil
		}

		if h.paymentMethodRepo == nil {
			return fmt.Errorf("payment method repository not configured")
		}
		methods, err := h.paymentMethodRepo.ListEnabled(ctx, tx)
		if err != nil {
			return fmt.Errorf("list enabled payment methods: %w", err)
		}

		principal := money.New(config.YearlyFeeRupiah)
		options := buildSubscriptionPaymentMethodOptions(principal, methods, func(code string, err error) {
			h.log.Warn("Skipping payment method with invalid fee formula",
				zap.String("method_code", code),
				zap.Error(err),
			)
		})

		result = &SubscriptionPaymentMethodsResponse{
			PrincipalAmount: principal.Int64(),
			Currency:        "IDR",
			Methods:         options,
		}
		return nil
	})

	if err != nil {
		h.log.Error("Failed to list subscription payment methods",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to load payment methods")
		return
	}

	if result != nil {
		response.Success(c, result)
	}
}

// SellerEarningsResponse represents the seller earnings response.
type SellerEarningsResponse struct {
	AvailableBalance    int64 `json:"available_balance"`
	PendingBalance      int64 `json:"pending_balance"`
	TotalWithdrawn      int64 `json:"total_withdrawn"`
	TotalEarned         int64 `json:"total_earned"`
	WithdrawalFeeAmount int64 `json:"withdrawal_fee_amount"`

	// Balance breakdown.
	// Source of truth: FinanceService.GetSellerWithdrawable (SellerWithdrawableSummary).
	GrossPayable        int64 `json:"gross_payable"`
	WithdrawableBalance int64 `json:"withdrawable_balance"`
}

// GetEarnings handles GET /api/v1/seller/earnings
//
// Returns the seller's earnings breakdown:
// - available_balance: Dispute-aware withdrawable balance (SELLER_PAYABLE ledger)
// - pending_balance: Always 0 — no maturity hold period; funds are withdrawable on release
// - total_withdrawn: Sum of all COMPLETED withdrawal amounts
// - total_earned: Total credits ever received to SELLER_PAYABLE account
//
// Authority: FinanceService.GetSellerWithdrawable (canonical SELLER_PAYABLE ledger read).
// payable_maturity table is NOT used — it is never written to at runtime.
func (h *SellerHandler) GetEarnings(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Query all data within a single transaction for consistency
	var availableBalance, pendingBalance, totalWithdrawn, totalEarned int64
	var grossPayable int64
	var withdrawalFee int64

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		// 0. Canonical configured seller withdrawal fee (same authority as
		// WithdrawService; admin-configurable, Rp0 allowed).
		if h.withdrawalFeeProvider == nil {
			return fmt.Errorf("seller earnings: withdrawal fee provider not configured")
		}
		withdrawalFee = h.withdrawalFeeProvider.GetSellerWithdrawalFee(ctx, tx)

		// 1. Available balance: withdrawable from the SELLER_PAYABLE ledger.
		// This is the same authority used by AssertSellerWithdrawalAllowed at withdrawal time.
		withdrawable, err := h.financeService.GetSellerWithdrawable(ctx, tx, userID)
		if err != nil {
			return err
		}
		availableBalance = withdrawable.Withdrawable
		grossPayable = withdrawable.PayableBalance

		// 2. Pending balance: no maturity hold period exists in this system.
		// Funds become withdrawable immediately on gateway escrow release.
		pendingBalance = 0

		// 3. Total withdrawn: Sum of COMPLETED withdrawal amounts
		withdrawn, err := h.withdrawRepo.GetWithdrawnTotal(ctx, tx, userID)
		if err != nil {
			return err
		}
		totalWithdrawn = withdrawn

		// 4. Total earned: Total credits ever received to SELLER_PAYABLE account
		// FINANCE REDIRECT: All ledger operations go through FinanceService
		earned, err := h.financeService.GetSellerTotalEarnings(ctx, tx, userID)
		if err != nil {
			return err
		}
		totalEarned = earned

		return nil
	})

	if err != nil {
		h.log.Error("Failed to get seller earnings",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve seller earnings")
		return
	}

	resp := SellerEarningsResponse{
		AvailableBalance:    availableBalance,
		PendingBalance:      pendingBalance,
		TotalWithdrawn:      totalWithdrawn,
		TotalEarned:         totalEarned,
		WithdrawalFeeAmount: withdrawalFee,

		// Balance breakdown from SellerWithdrawableSummary.
		GrossPayable:        grossPayable,
		WithdrawableBalance: availableBalance, // Invariant: withdrawable_balance == available_balance
	}

	response.Success(c, resp)
}

// ============================================================================
// SELLER DASHBOARD ENDPOINTS
// ============================================================================

// SellerDashboardResponse represents the seller dashboard response.
type SellerDashboardResponse struct {
	TotalForSales  int64 `json:"total_for_sales"`
	ActiveForSales int64 `json:"active_for_sales"`
	SoldItems      int64 `json:"sold_items"`
	TotalRevenue   int64 `json:"total_revenue"`
	PendingOrders  int64 `json:"pending_orders"`
}

// GetDashboard handles GET /api/v1/seller/dashboard
//
// Returns the seller dashboard statistics:
// - total_for_sales: Total count of forSales for this seller
// - active_for_sales: Count of active forSales (status = 'active')
// - sold_items: Count of completed orders where user is seller
// - total_revenue: Sum of seller's share from completed orders (subtotal - commission)
// - pending_orders: Count of orders awaiting action (pending, paid)
func (h *SellerHandler) GetDashboard(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var stats SellerDashboardResponse

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		// 1. Total forSales — counts real for_sales rows (canonical
		// fixed-price sale table; the legacy `forSales` table is write-dead).
		err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM for_sales WHERE seller_id = $1
		`, userID).Scan(&stats.TotalForSales)
		if err != nil {
			return err
		}

		// 2. Active forSales
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM for_sales WHERE seller_id = $1 AND status = 'active'
		`, userID).Scan(&stats.ActiveForSales)
		if err != nil {
			return err
		}

		// 3. Sold items (completed orders)
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM orders WHERE seller_id = $1 AND status = 'completed'
		`, userID).Scan(&stats.SoldItems)
		if err != nil {
			return err
		}

		// 4. Total revenue (subtotal - commission from completed orders)
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(subtotal - commission_amount), 0)
			FROM orders
			WHERE seller_id = $1 AND status = 'completed'
		`, userID).Scan(&stats.TotalRevenue)
		if err != nil {
			return err
		}

		// 5. Pending orders (awaiting seller action)
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM orders
			WHERE seller_id = $1 AND status IN ('pending_payment', 'paid')
		`, userID).Scan(&stats.PendingOrders)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		h.log.Error("Failed to get seller dashboard",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve seller dashboard")
		return
	}

	response.Success(c, stats)
}

// SellerAnalyticsResponse represents the seller analytics response.
type SellerAnalyticsResponse struct {
	Views          int64   `json:"views"`
	ConversionRate float64 `json:"conversion_rate"`
}

// GetAnalytics handles GET /api/v1/seller/analytics
//
// Returns the seller analytics:
// - views: Total views across all seller's forSales
// - conversion_rate: (sold_items / views) * 100, limited to 2 decimal places
func (h *SellerHandler) GetAnalytics(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var stats SellerAnalyticsResponse

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		// 1. Total views across all forSales.
		// NOTE: view tracking was only ever wired to the legacy `forSales`
		// table (write-dead since the for_sales/products split) and
		// was never re-implemented against for_sales. There is no
		// canonical view-count source today, so this reports 0 rather than
		// querying a table nothing writes to. Real view tracking for
		// for_sales is tracked as PASS_21C follow-up debt.
		stats.Views = 0

		// 2. Conversion rate: (sold_items / views) * 100
		// Get sold items count
		var soldItems int64
		err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM orders WHERE seller_id = $1 AND status = 'completed'
		`, userID).Scan(&soldItems)
		if err != nil {
			return err
		}

		if stats.Views > 0 {
			stats.ConversionRate = (float64(soldItems) / float64(stats.Views)) * 100
			// Limit to 2 decimal places
			stats.ConversionRate = float64(int(stats.ConversionRate*100)) / 100
		}

		return nil
	})

	if err != nil {
		h.log.Error("Failed to get seller analytics",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve seller analytics")
		return
	}

	response.Success(c, stats)
}

// SellerPerformanceResponse represents the seller performance response.
type SellerPerformanceResponse struct {
	Rating          float64 `json:"rating"`
	CompletedOrders int64   `json:"completed_orders"`
	CancelRate      float64 `json:"cancel_rate"`
	ResponseTime    string  `json:"response_time"`
}

// GetPerformance handles GET /api/v1/seller/performance
//
// Returns the seller performance metrics:
// - rating: Average rating received (0-5 scale)
// - completed_orders: Total count of completed orders
// - cancel_rate: (cancelled_orders / total_orders) * 100
// - response_time: Average response time in hours (placeholder for now)
func (h *SellerHandler) GetPerformance(c *gin.Context) {
	ctx := c.Request.Context()

	// Get user ID from context (set by auth middleware)
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var stats SellerPerformanceResponse

	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		// 1. Average rating (from rating service)
		// RATING DOMAIN BOUNDARY: Use RatingReader interface (read-only access)
		ratingSummary, err := h.ratingReader.GetRatingSummary(ctx, tx, userID)
		if err != nil {
			return err
		}
		stats.Rating = ratingSummary.AverageRating

		// 2. Completed orders count
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM orders WHERE seller_id = $1 AND status = 'completed'
		`, userID).Scan(&stats.CompletedOrders)
		if err != nil {
			return err
		}

		// 3. Cancel rate: (cancelled_orders / total_orders) * 100
		var totalOrders int64
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM orders WHERE seller_id = $1
		`, userID).Scan(&totalOrders)
		if err != nil {
			return err
		}

		var cancelledOrders int64
		err = tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM orders WHERE seller_id = $1 AND status = 'cancelled'
		`, userID).Scan(&cancelledOrders)
		if err != nil {
			return err
		}

		if totalOrders > 0 {
			stats.CancelRate = (float64(cancelledOrders) / float64(totalOrders)) * 100
			// Limit to 2 decimal places
			stats.CancelRate = float64(int(stats.CancelRate*100)) / 100
		}

		// 4. Response time - placeholder, would need to track message response times
		// For now, return a default value
		stats.ResponseTime = "2h"

		return nil
	})

	if err != nil {
		h.log.Error("Failed to get seller performance",
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to retrieve seller performance")
		return
	}

	response.Success(c, stats)
}

