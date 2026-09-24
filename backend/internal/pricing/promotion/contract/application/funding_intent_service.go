// Package application implements the canonical Promotion Contract lifecycle.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	paymentmethodentity "github.com/labuda/backend/internal/commerce/paymentmethod/entity"
	"github.com/labuda/backend/internal/finance/billing/application"
	billingentity "github.com/labuda/backend/internal/finance/billing/entity"
	contractentity "github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	contractRepoImpl "github.com/labuda/backend/internal/pricing/promotion/contract/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// FundingIntentResult is the response from CreateFundingIntent.
// It represents a snapshot funding calculation and payment obligation —
// NOT a promotion contract, NOT a reservation, NOT a binding.
type FundingIntentResult struct {
	// PaymentRequired is true when the seller must pay before allocation.
	PaymentRequired bool `json:"payment_required"`

	// Shortage is the exact amount the seller must pay. Zero when
	// PaymentRequired is false. The payment amount is immutable after creation.
	Shortage int64 `json:"shortage"`

	// IntentID is the canonical funding intent identifier. Present only
	// when PaymentRequired is true. The seller references this when paying.
	IntentID string `json:"intent_id,omitempty"`

	// BillingID is the billing transaction identifier for payment via
	// POST /payments/billing. Present only when PaymentRequired is true.
	BillingID string `json:"billing_id,omitempty"`

	// RequiredCost is the total promotion cost (informational snapshot).
	RequiredCost int64 `json:"required_cost"`

	// AvailableFunding is the seller's current PROMOTE_BALANCE (informational snapshot).
	AvailableFunding int64 `json:"available_funding"`
}

// FundingIntentService creates exact-shortage payment intents for promotion
// funding. It wraps PromotionContractService (for shortage calculation) and
// BillingService (for billing transaction creation).
//
// AUTHORITY: single canonical funding intent service. There is no second
// calculation or payment path.
type FundingIntentService struct {
	contractSvc *PromotionContractService
	billingSvc  *application.BillingService
	intentRepo  *contractRepoImpl.FundingIntentRepository
	log         *zap.Logger

	// methodReader is the canonical enabled-payment-method authority used by
	// the read-only disclosure surface. Wired at boot via
	// SetPaymentMethodReader; nil → the disclosure fails closed rather than
	// inventing a method list or a fee.
	methodReader PaymentMethodReader
}

// PaymentMethodReader is the canonical enabled payment method lookup authority
// (implemented by the payment method repository). The disclosure surface never
// invents a method and never computes a fee itself.
type PaymentMethodReader interface {
	ListEnabled(ctx context.Context, tx db.Tx) ([]paymentmethodentity.Method, error)
}

// NewFundingIntentService wires the canonical funding intent service.
func NewFundingIntentService(
	contractSvc *PromotionContractService,
	billingSvc *application.BillingService,
	log *zap.Logger,
) *FundingIntentService {
	if log == nil {
		log = zap.NewNop()
	}
	return &FundingIntentService{
		contractSvc: contractSvc,
		billingSvc:  billingSvc,
		intentRepo:  contractRepoImpl.NewFundingIntentRepository(),
		log:         log,
	}
}

// SetPaymentMethodReader wires the canonical payment method authority into the
// promotion funding disclosure surface. Must be called at boot; nil → the
// disclosure endpoint fails closed (no invented methods, no invented fees).
func (s *FundingIntentService) SetPaymentMethodReader(r PaymentMethodReader) {
	s.methodReader = r
}

// CreateFundingIntent computes the exact shortage for a proposed promotion and,
// when shortage > 0, creates a billing transaction for exactly that amount.
//
// The promotion parameters are a snapshot used for shortage calculation.
// They do NOT bind the payment to a specific future promotion. After settlement,
// the resulting PROMOTE_BALANCE is fully reusable — the seller may create the
// original promotion, a modified promotion, another promotion, or wait.
//
// RACE-SAFE IDEMPOTENCY:
//
// The method uses INSERT ... ON CONFLICT DO NOTHING RETURNING to atomically
// handle concurrent duplicate requests. Two concurrent calls with identical
// params will produce exactly one intent + one billing obligation:
//
//   - Call A: INSERT succeeds → creates billing → links billing
//   - Call B: INSERT conflicts (unique index) → reads existing → returns it
//
// The unique index ux_promotion_funding_intents_seller_params on
// (seller_id, kind, budget_rupiah, duration_days) is the DB-level authority
// for preventing duplicate concurrent intents.
//
// No ledger mutation occurs. PROMOTE_BALANCE is only credited after
// payment settlement (webhook → MarkPaid → RecordPromoteBalanceFunding).
func (s *FundingIntentService) CreateFundingIntent(
	ctx context.Context,
	input CreatePromotionInput,
) (*FundingIntentResult, error) {
	// Step 1: Compute shortage (read-only, same validation as Create)
	preview, err := s.contractSvc.PreviewFunding(ctx, input)
	if err != nil {
		return nil, err
	}

	// Step 2: No shortage → no payment needed
	if preview.Shortage == 0 {
		return &FundingIntentResult{
			PaymentRequired:  false,
			Shortage:         0,
			RequiredCost:     preview.RequiredCost,
			AvailableFunding: preview.AvailableFunding,
		}, nil
	}

	// Step 3: Shortage > 0 → atomic upsert + billing
	var result *FundingIntentResult
	err = s.contractSvc.db.WithTx(ctx, func(tx db.Tx) error {
		// Atomic upsert: INSERT ... ON CONFLICT DO NOTHING RETURNING id
		// This is the canonical race-safe idempotency mechanism.
		intentID := uuid.New()
		intent := &contractentity.FundingIntent{
			ID:             intentID,
			SellerID:       input.SellerID,
			Kind:           string(input.Kind),
			BudgetRupiah:   input.BudgetRupiah,
			DurationDays:   input.DurationDays,
			CityIDs:        input.CityIDs,
			ShortageAmount: preview.Shortage,
			CreatedAt:      time.Now(),
		}
		insertedID, err := s.intentRepo.UpsertIntent(ctx, tx, intent)
		if err != nil {
			return fmt.Errorf("upsert funding intent: %w", err)
		}

		if insertedID == uuid.Nil {
			// Conflict — intent already exists with same params.
			// Read the existing intent and return it.
			existing, err := s.intentRepo.LatestIntentForSeller(ctx, tx, input.SellerID)
			if err != nil {
				return fmt.Errorf("read existing intent: %w", err)
			}
			if existing == nil {
				return fmt.Errorf("upsert conflict but no existing intent found for seller %s", input.SellerID)
			}

			if existing.BillingTransactionID != nil {
				// Billing already linked — check its status
				billing, bErr := s.billingSvc.GetBillingTransaction(ctx, tx, *existing.BillingTransactionID)
				if bErr != nil {
					return fmt.Errorf("get existing billing: %w", bErr)
				}
				// Return existing intent if billing is pending or paid
				result = &FundingIntentResult{
					PaymentRequired:  true,
					Shortage:         existing.ShortageAmount,
					IntentID:         existing.ID.String(),
					BillingID:        billing.ID.String(),
					RequiredCost:     preview.RequiredCost,
					AvailableFunding: preview.AvailableFunding,
				}
				return nil
			}
			// No billing linked yet — return existing (idempotent)
			result = &FundingIntentResult{
				PaymentRequired:  true,
				Shortage:         existing.ShortageAmount,
				IntentID:         existing.ID.String(),
				BillingID:        "",
				RequiredCost:     preview.RequiredCost,
				AvailableFunding: preview.AvailableFunding,
			}
			return nil
		}

		// Insert succeeded — create billing transaction for exact shortage
		billing, err := s.billingSvc.CreateBillingTransaction(
			ctx,
			tx,
			input.SellerID,     // caller
			input.SellerID,     // payer
			insertedID,         // target_id = intent ID (binding)
			billingentity.TypePromoteBalanceTopUp,
			money.New(preview.Shortage),
			0, // platform fee MUST be 0 — funding is not revenue
		)
		if err != nil {
			return fmt.Errorf("create billing transaction: %w", err)
		}

		// Link billing to intent
		if err := s.intentRepo.LinkBilling(ctx, tx, insertedID, billing.ID); err != nil {
			return fmt.Errorf("link billing to intent: %w", err)
		}

		result = &FundingIntentResult{
			PaymentRequired:  true,
			Shortage:         preview.Shortage,
			IntentID:         insertedID.String(),
			BillingID:        billing.ID.String(),
			RequiredCost:     preview.RequiredCost,
			AvailableFunding: preview.AvailableFunding,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Payment initiation error sentinels.
var (
	ErrFundingIntentNotFound          = errors.New("funding intent not found")
	ErrFundingIntentNotOwnedByCaller  = errors.New("funding intent not owned by caller")
	ErrFundingIntentNoBilling         = errors.New("funding intent has no linked billing transaction")
	ErrFundingIntentBillingAmountMismatch = errors.New("billing amount does not match intent shortage")
	ErrFundingIntentBillingNotPending = errors.New("billing transaction is not pending")
)

// InitiatePaymentInput captures the parameters for payment initiation.
type InitiatePaymentInput struct {
	IntentID          uuid.UUID
	CallerID          uuid.UUID
	PaymentMethodCode string
}

// InitiatePaymentResult holds the verified billing data for payment initiation.
type InitiatePaymentResult struct {
	BillingID uuid.UUID
	Shortage  int64
}

// InitiatePayment loads a FundingIntent, verifies the linked billing transaction's
// amount matches the immutable shortage, and returns the billing ID for payment
// initiation through the canonical billing→payment engine.
//
// This method performs ONLY validation. It does NOT create payments, call Midtrans,
// or mutate any financial state. The actual payment initiation is delegated to
// CorePaymentHandler.InitiateBillingPayment.
//
// Invariants enforced:
//   - Intent exists and is owned by the caller
//   - Intent has a linked billing transaction
//   - billing.gross_amount == intent.shortage_amount (immutable)
//   - billing.status == pending
func (s *FundingIntentService) InitiatePayment(
	ctx context.Context,
	input InitiatePaymentInput,
) (*InitiatePaymentResult, error) {
	var result *InitiatePaymentResult
	err := s.contractSvc.db.WithTx(ctx, func(tx db.Tx) error {
		// Load intent
		intent, err := s.intentRepo.GetByID(ctx, tx, input.IntentID)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrFundingIntentNotFound, err)
		}

		// Verify ownership
		if intent.SellerID != input.CallerID {
			return ErrFundingIntentNotOwnedByCaller
		}

		// Verify billing linked
		if intent.BillingTransactionID == nil {
			return ErrFundingIntentNoBilling
		}

		// Load billing transaction
		billing, err := s.billingSvc.GetBillingTransaction(ctx, tx, *intent.BillingTransactionID)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrFundingIntentNoBilling, err)
		}

		// Verify billing amount == intent shortage (immutable)
		if billing.GrossAmount.Int64() != intent.ShortageAmount {
			return fmt.Errorf("%w: billing=%d intent=%d",
				ErrFundingIntentBillingAmountMismatch,
				billing.GrossAmount.Int64(),
				intent.ShortageAmount)
		}

		// Verify billing is pending
		if billing.Status != billingentity.StatusPending {
			return ErrFundingIntentBillingNotPending
		}

		result = &InitiatePaymentResult{
			BillingID: billing.ID,
			Shortage:  intent.ShortageAmount,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ============================================================================
// PAYMENT METHOD DISCLOSURE (READ-ONLY)
// ============================================================================

// FundingPaymentMethodOption is one selectable payment method for the current
// promotion funding obligation. Vocabulary mirrors the payment snapshot columns
// (service_fee_amount, gross_amount) so the client renders exactly what
// InitiateBillingPayment will snapshot at initiation.
type FundingPaymentMethodOption struct {
	MethodCode       string `json:"method_code"`
	DisplayName      string `json:"display_name"`
	ServiceFeeAmount int64  `json:"service_fee_amount"`
	GrossAmount      int64  `json:"gross_amount"`
}

// FundingPaymentMethodsResult is the read-only disclosure payload for the
// promotion funding method picker: the exact shortage obligation plus every
// enabled method with F and shortage+F.
type FundingPaymentMethodsResult struct {
	ShortageAmount int64                        `json:"shortage_amount"`
	Currency       string                       `json:"currency"`
	Methods        []FundingPaymentMethodOption `json:"methods"`
}

// PaymentMethodsInput identifies the funding obligation being disclosed.
type PaymentMethodsInput struct {
	IntentID uuid.UUID
	CallerID uuid.UUID
}

// buildFundingPaymentMethodOptions computes, per enabled method, the canonical
// fee F = CalculateFee(shortage, method) and the resulting gross shortage+F for
// the promotion funding obligation. Methods with an invalid fee formula are
// skipped (onInvalid receives the code and cause) rather than failing the whole
// list, matching the canonical subscription and order disclosures.
func buildFundingPaymentMethodOptions(
	shortage money.Money,
	methods []paymentmethodentity.Method,
	onInvalid func(code string, err error),
) []FundingPaymentMethodOption {
	options := make([]FundingPaymentMethodOption, 0, len(methods))
	for _, m := range methods {
		fee, err := paymentmethodentity.CalculateFee(shortage, m)
		if err != nil {
			if onInvalid != nil {
				onInvalid(m.Code, err)
			}
			continue
		}
		options = append(options, FundingPaymentMethodOption{
			MethodCode:       m.Code,
			DisplayName:      m.DisplayName,
			ServiceFeeAmount: fee.Int64(),
			GrossAmount:      shortage.Add(fee).Int64(),
		})
	}
	return options
}

// PaymentMethods returns the enabled payment methods available for a valid,
// pending, caller-owned funding intent, with the canonical fee and gross total
// computed from the intent's exact shortage.
//
// READ-ONLY: no payment row is created, no Midtrans call is made, no ledger row
// is written, and no FundingIntent is created or mutated.
//
// FEE PARITY: the fee base is billing.GrossAmount — the exact immutable shortage
// obligation, and the very same base CorePaymentHandler.InitiateBillingPayment
// charges its fee on — so the disclosed gross is exactly what initiation will
// snapshot. The fee itself is always paymentmethodentity.CalculateFee and the
// method list is always the canonical ListEnabled authority.
//
// Rejections mirror InitiatePayment exactly: unknown intent → not found, other
// seller's intent → not owned, unlinked billing → no billing, amount drift →
// mismatch, settled/failed billing → not pending.
func (s *FundingIntentService) PaymentMethods(
	ctx context.Context,
	input PaymentMethodsInput,
) (*FundingPaymentMethodsResult, error) {
	if s.methodReader == nil {
		return nil, errors.New("promotion funding payment methods: payment method reader not configured")
	}

	var result *FundingPaymentMethodsResult
	err := s.contractSvc.db.WithTx(ctx, func(tx db.Tx) error {
		intent, err := s.intentRepo.GetByID(ctx, tx, input.IntentID)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrFundingIntentNotFound, err)
		}

		if intent.SellerID != input.CallerID {
			return ErrFundingIntentNotOwnedByCaller
		}

		if intent.BillingTransactionID == nil {
			return ErrFundingIntentNoBilling
		}

		billing, err := s.billingSvc.GetBillingTransaction(ctx, tx, *intent.BillingTransactionID)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrFundingIntentNoBilling, err)
		}

		if billing.Status != billingentity.StatusPending {
			return ErrFundingIntentBillingNotPending
		}

		// The obligation, unchanged and immutable: the billing amount is the
		// exact shortage the seller must pay today, and the fee base.
		shortage := billing.GrossAmount
		if shortage.Int64() != intent.ShortageAmount {
			return fmt.Errorf("%w: billing=%d intent=%d",
				ErrFundingIntentBillingAmountMismatch,
				shortage.Int64(), intent.ShortageAmount)
		}

		methods, err := s.methodReader.ListEnabled(ctx, tx)
		if err != nil {
			return fmt.Errorf("list enabled payment methods: %w", err)
		}

		result = &FundingPaymentMethodsResult{
			ShortageAmount: shortage.Int64(),
			Currency:       "IDR",
			Methods: buildFundingPaymentMethodOptions(shortage, methods, func(code string, err error) {
				s.log.Warn("Skipping payment method with invalid fee formula",
					zap.String("method_code", code),
					zap.Error(err),
				)
			}),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
