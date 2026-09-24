// Package application implements the canonical Promotion Contract lifecycle.
//
// BOUNDARY:
//   - The contract is the LIFECYCLE authority (status, seller slot, pause
//     bookkeeping, planned finish). It is NOT a financial authority.
//   - All money movement delegates to FinanceService ledger operations
//     (RecordPromotionAllocation / RecordPromotionAllocationRelease).
//   - All lifecycle-sensitive time comes from the DB clock (GetDBTime).
package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/finance"
	financeapp "github.com/labuda/backend/internal/finance/application"
	ledgerRepoImpl "github.com/labuda/backend/internal/finance/infrastructure/repository"
	configapp "github.com/labuda/backend/internal/platform/config/application"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	contractRepoImpl "github.com/labuda/backend/internal/pricing/promotion/contract/infrastructure/repository"
	contractRepo "github.com/labuda/backend/internal/pricing/promotion/contract/repository"
	deliveryRepo "github.com/labuda/backend/internal/pricing/promotion/delivery/repository"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// maxContractDurationDays is the largest whole-day duration whose planned
// finish derivation (planned_start + duration_days x 24h) is exactly
// representable in a Go time.Duration (int64 nanoseconds, ~292 years).
//
// This is a TECHNICAL representation-safety bound derived from the actual
// arithmetic of this implementation — NOT a Labuda business/product duration
// policy. The only business rule is duration_days > 0 (a whole-number number
// of promotion days). Without this guard the duration x 24h multiplication
// silently wraps in int64 and could persist a contract whose planned_finish
// does not correspond to its duration; rejecting the unrepresentable range
// keeps creation fail-closed before any financial mutation. Rejections above
// this bound are overflow guards, never a product duration maximum.
const maxContractDurationDays = math.MaxInt64 / int64(24*time.Hour)

// SellerEligibilityGate is the seller governance authority the contract
// domain defers to at creation. The concrete wiring (active account, seller
// capability, subscription) is provided by the delivery/boot layer.
type SellerEligibilityGate interface {
	EnsureCanPromote(ctx context.Context, tx db.Tx, sellerID uuid.UUID) error
}

// CreatePromotionInput captures the canonical seller inputs for a new
// Promotion contract.
type CreatePromotionInput struct {
	SellerID     uuid.UUID
	Kind         entity.Kind
	BudgetRupiah int64
	DurationDays int64  // pacing boundary length in whole days
	CityIDs      []string // geographic targeting: empty = nationwide, otherwise arbitrary set of city_id
}

// FundingPreview is the canonical read-only projection of promotion funding
// sufficiency. It answers the question: "can this promotion proceed, and if
// not, how much must the seller pay?"
//
// AUTHORITY: this is a READ-ONLY projection. It acquires NO locks, creates
// NO contracts, and performs NO financial mutations. The balance is read from
// the current PROMOTE_BALANCE ledger account (non-locking snapshot). The
// actual allocation at Create time uses FOR UPDATE and may differ if another
// transaction commits between PreviewFunding and Create — the caller must
// treat this as informational guidance, not a binding reservation.
type FundingPreview struct {
	// RequiredCost is the minimum budget the seller must fund for this
	// promotion (equal to BudgetRupiah when above minimum; otherwise
	// the minimum required budget). Unit: whole Rupiah.
	RequiredCost int64 `json:"required_cost"`

	// AvailableFunding is the seller's current PROMOTE_BALANCE at the
	// time of this read. Unit: whole Rupiah.
	AvailableFunding int64 `json:"available_funding"`

	// Shortage is max(RequiredCost - AvailableFunding, 0). Unit: whole Rupiah.
	// When zero, the promotion may proceed to allocation without payment.
	Shortage int64 `json:"shortage"`

	// PaymentRequired is true when the seller must pay before allocation.
	// Equivalent to Shortage > 0.
	PaymentRequired bool `json:"payment_required"`
}

// PausePromotionInput targets an explicit seller-initiated pause.
type PausePromotionInput struct {
	SellerID   uuid.UUID
	ContractID uuid.UUID
}

// ResumePromotionInput targets an explicit seller-initiated resume.
type ResumePromotionInput struct {
	SellerID   uuid.UUID
	ContractID uuid.UUID
}

// FinalizePromotionInput triggers canonical finalization (seller stop or
// planned-finish completion both use this same boundary).
type FinalizePromotionInput struct {
	SellerID   uuid.UUID
	ContractID uuid.UUID
}

// Domain errors.
var (
	ErrPromotionKindInvalid = errors.New("promotion kind invalid")

	// ErrPromotionBudgetInvalid is returned for non-positive budgets.
	ErrPromotionBudgetInvalid = errors.New("promotion budget must be a positive Rupiah integer")

	// ErrPromotionDurationInvalid is returned for non-positive durations or
	// durations beyond the technical time.Duration representation bound
	// (fail-closed overflow guard — see maxContractDurationDays). It is NOT a
	// business/product duration cap.
	ErrPromotionDurationInvalid = errors.New("promotion duration invalid")

	ErrPromotionContractNotFound   = errors.New("promotion contract not found")
	ErrPromotionContractNotOwned   = errors.New("promotion contract not owned by seller")
	ErrPromotionPauseNotAllowed    = errors.New("promotion is not pausable in its current status")
	ErrPromotionAlreadyPaused      = errors.New("promotion is already paused")
	ErrPromotionResumeNotAllowed   = errors.New("promotion is not resumable in its current status")
	ErrPromotionAlreadyFinalized   = errors.New("promotion is already finalized")
	ErrPromotionResumeClockInvalid = errors.New("promotion resume clock invalid (pause started after resume time)")
)

// ErrPromotionBudgetBelowMinimum is returned when budget < configured
// minimum_daily_budget x duration_days, or when that multiplication itself
// overflows int64 (then Required is math.MaxInt64 — the requirement is not
// representable, so no budget can ever satisfy it; fail-closed).
type ErrPromotionBudgetBelowMinimum struct {
	Budget   int64
	Required int64
}

func (e *ErrPromotionBudgetBelowMinimum) Error() string {
	return fmt.Sprintf("promotion budget %d below minimum required budget %d", e.Budget, e.Required)
}

// ErrPromotionSellerSlotOccupied is returned when the seller already holds a
// non-finalized contract of the same kind (DB partial unique index is the
// final authority; this is the mapped application error).
type ErrPromotionSellerSlotOccupied struct {
	SellerID uuid.UUID
	Kind     entity.Kind
}

func (e *ErrPromotionSellerSlotOccupied) Error() string {
	return fmt.Sprintf("seller %s already has a non-finalized %s promotion contract", e.SellerID, e.Kind)
}

// PromotionContractService owns the canonical contract lifecycle.
type PromotionContractService struct {
	db          *db.DB
	repo        contractRepo.Repository
	targets     contractRepo.ContractTargetRepository
	geographies contractRepo.ContractGeographyRepository
	delivery    deliveryRepo.Repository
	ledger      *ledgerRepoImpl.LedgerRepository
	finance     *financeapp.FinanceService
	config      *configapp.ConfigService
	gate        SellerEligibilityGate
	operability TargetOperabilityAdapter
	log         *zap.Logger
}

// TargetOperabilityAdapter checks canonical target operability for queue management.
type TargetOperabilityAdapter interface {
	CheckOperability(ctx context.Context, targetType string, targetID *uuid.UUID) (bool, string, error)
	ValidateOwnership(ctx context.Context, sellerID uuid.UUID, targetType string, targetID *uuid.UUID) error
}

// NewPromotionContractService wires the canonical contract service.
// gate may be nil (nil gate skips the eligibility pre-check); production
// wiring must supply the real seller governance gate before any HTTP path is
// opened. deliveryRepo is used at finalization to invalidate outstanding
// delivery tickets (Phase 3 boundary) so a finalized contract can never let
// an old ticket charge.
func NewPromotionContractService(
	db *db.DB,
	financeService *financeapp.FinanceService,
	configService *configapp.ConfigService,
	gate SellerEligibilityGate,
	delivery deliveryRepo.Repository,
) *PromotionContractService {
	return &PromotionContractService{
		db:          db,
		repo:        contractRepoImpl.NewContractRepository(),
		targets:     contractRepoImpl.NewContractTargetRepository(),
		geographies: contractRepoImpl.NewContractGeographyRepository(),
		delivery:    delivery,
		ledger:      ledgerRepoImpl.NewLedgerRepository(),
		finance:     financeService,
		config:      configService,
		gate:        gate,
		log:         zap.NewNop(),
	}
}

// SetTargetOperability wires the operability adapter for queue validation.
func (s *PromotionContractService) SetTargetOperability(adapter TargetOperabilityAdapter) {
	s.operability = adapter
}

// SetLogger wires a structured logger (optional).
func (s *PromotionContractService) SetLogger(logger *zap.Logger) {
	if logger != nil {
		s.log = logger
	}
}

// ============================================================================
// CREATE — contract + allocation in ONE transaction boundary
// ============================================================================

// Create builds a promotion contract and atomically funds its allocation.
//
// Canonical sequence (single db.WithTx):
//
//  1. seller eligibility gate
//  2. kind / budget / duration validation
//  3. canonical DB time read
//  4. planned_finish = planned_start + duration_days x 24h (whole-day pacing
//     boundary; duration is validated against the technical time.Duration
//     overflow bound so the derivation cannot wrap)
//  5. minimum budget = min_daily_budget x duration_days (integer-safe)
//  6. immutable CPM snapshot read from platform config
//  7. create holder-scoped PROMOTION_ALLOCATION account
//  8. FinanceService.RecordPromotionAllocation (PROMOTE_BALANCE -> ALLOCATION)
//  9. persist contract (status active)
//
// There is NO outcome where the contract survives but allocation failed, or
// money moved but the contract is missing: every step is in the same DB
// transaction and a failure rolls everything back (contract row, allocation
// account row, ledger movement). The seller slot is enforced by partial
// unique indexes — a concurrent duplicate create fails with
// ErrPromotionSellerSlotOccupied.
func (s *PromotionContractService) Create(
	ctx context.Context,
	input CreatePromotionInput,
) (*entity.Contract, error) {
	if input.SellerID == uuid.Nil {
		return nil, fmt.Errorf("CreatePromotion: seller_id required")
	}
	if !input.Kind.IsValid() {
		return nil, ErrPromotionKindInvalid
	}
	if input.BudgetRupiah <= 0 {
		return nil, ErrPromotionBudgetInvalid
	}
	if input.DurationDays <= 0 || input.DurationDays > maxContractDurationDays {
		return nil, ErrPromotionDurationInvalid
	}

	var created *entity.Contract
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		if s.gate != nil {
			if err := s.gate.EnsureCanPromote(ctx, tx, input.SellerID); err != nil {
				return err
			}
		}

		now, err := s.repo.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read db time: %w", err)
		}

		// Minimum daily budget rule (canonical §5): budget must cover
		// min_daily_budget x duration_days, computed integer-safe.
		minDaily := s.config.GetPromotionMinDailyBudget(ctx, tx)
		required, overflow := mulNonNeg(minDaily, input.DurationDays)
		if overflow {
			// Fail closed: min_daily_budget x duration_days overflowed int64,
			// so the minimum requirement itself is not representable. Report
			// it as the largest representable requirement — it can never be
			// met, which is the only safe outcome. The transaction aborts
			// here, before any contract, allocation account, or ledger entry
			// is created.
			return &ErrPromotionBudgetBelowMinimum{
				Budget:   input.BudgetRupiah,
				Required: math.MaxInt64,
			}
		}
		if input.BudgetRupiah < required {
			return &ErrPromotionBudgetBelowMinimum{
				Budget:   input.BudgetRupiah,
				Required: required,
			}
		}

		// Immutable pricing snapshot (§9): read the ACTIVE CPM now; future
		// admin changes affect future contracts only.
		cpm := s.config.GetPromotionCPM(ctx, tx)
		if cpm <= 0 {
			return fmt.Errorf("promotion cpm config must be positive (got %d)", cpm)
		}

		// Canonical geographic targeting: dedup + snapshot validation (city_id is authority, city_name/province_id are snapshots from addresses)
		geos, err := s.canonicalizeCityIDs(ctx, tx, input.CityIDs)
		if err != nil {
			return err
		}

		contractID := uuid.New()
		plannedStart := now
		plannedFinish := now.Add(time.Duration(input.DurationDays) * 24 * time.Hour)

		// Holder-scoped allocation account (created here, in this tx).
		allocationID, err := s.ledger.GetOrCreateHolderAccount(
			ctx, tx, finance.AccountPromotionAllocation, "promotion", input.SellerID, contractID,
		)
		if err != nil {
			return fmt.Errorf("create promotion allocation account: %w", err)
		}

		// Atomic budget move. Insufficient Promote Balance surfaces as
		// financeapp.ErrPromoteBalanceInsufficient and rolls back the whole
		// transaction (no contract, no orphan allocation account).
		if _, err := s.finance.RecordPromotionAllocation(ctx, tx, contractID, input.SellerID, input.BudgetRupiah); err != nil {
			return fmt.Errorf("promotion allocation failed: %w", err)
		}

		contract := &entity.Contract{
			ID:                  contractID,
			SellerID:            input.SellerID,
			Kind:                input.Kind,
			Status:              entity.StatusActive,
			BudgetRupiah:        input.BudgetRupiah,
			CPMRupiah:           cpm,
			PlannedStart:        plannedStart,
			PlannedFinish:       plannedFinish,
			AllocationAccountID: allocationID,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := s.repo.Create(ctx, tx, contract); err != nil {
			if isSellerSlotUniqueViolation(err) {
				return &ErrPromotionSellerSlotOccupied{SellerID: input.SellerID, Kind: input.Kind}
			}
			return fmt.Errorf("create promotion contract row: %w", err)
		}

		// Persist geography rows atomically in same Tx — empty = nationwide (0 rows)
		if len(geos) > 0 {
			// assign contractID to each snapshot
			for i := range geos {
				geos[i].ContractID = contractID
			}
			if err := s.geographies.ReplaceGeographies(ctx, tx, contractID, geos); err != nil {
				return fmt.Errorf("persist promotion geographies: %w", err)
			}
		}

		created = contract
		s.log.Info("promotion_contract_created",
			zap.String("contract_id", contract.ID.String()),
			zap.String("seller_id", contract.SellerID.String()),
			zap.String("kind", string(contract.Kind)),
			zap.Int64("budget_rupiah", contract.BudgetRupiah),
			zap.Int64("cpm_rupiah", contract.CPMRupiah),
			zap.Time("planned_finish", contract.PlannedFinish),
		)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// ============================================================================
// PREVIEW FUNDING — read-only shortage projection (no mutations)
// ============================================================================

// PreviewFunding returns the canonical funding sufficiency projection for a
// proposed promotion. It applies the same input validation as Create (kind,
// budget positivity, duration positivity, minimum daily budget) but performs
// NO financial mutations, NO contract creation, and NO locking.
//
// The PROMOTE_BALANCE is read via a non-locking snapshot. If the balance
// changes between this call and the subsequent Create, the Create will
// independently re-validate and may fail — PreviewFunding is informational
// guidance, not a reservation.
//
// AUTHORITY: single canonical funding projection. There is no second
// calculation path. The shortage formula is:
//
//	shortage = max(required_cost - available_funding, 0)
func (s *PromotionContractService) PreviewFunding(
	ctx context.Context,
	input CreatePromotionInput,
) (*FundingPreview, error) {
	if input.SellerID == uuid.Nil {
		return nil, fmt.Errorf("PreviewFunding: seller_id required")
	}
	if !input.Kind.IsValid() {
		return nil, ErrPromotionKindInvalid
	}
	if input.BudgetRupiah <= 0 {
		return nil, ErrPromotionBudgetInvalid
	}
	if input.DurationDays <= 0 || input.DurationDays > maxContractDurationDays {
		return nil, ErrPromotionDurationInvalid
	}

	var preview *FundingPreview
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// Minimum daily budget rule — same authority as Create.
		minDaily := s.config.GetPromotionMinDailyBudget(ctx, tx)
		required, overflow := mulNonNeg(minDaily, input.DurationDays)
		if overflow {
			return &ErrPromotionBudgetBelowMinimum{
				Budget:   input.BudgetRupiah,
				Required: math.MaxInt64,
			}
		}
		if input.BudgetRupiah < required {
			return &ErrPromotionBudgetBelowMinimum{
				Budget:   input.BudgetRupiah,
				Required: required,
			}
		}

		// Read the seller's current PROMOTE_BALANCE — non-locking snapshot.
		promoteBalanceID, err := s.ledger.GetOrCreateUserAccount(
			ctx, tx, finance.AccountPromoteBalance, input.SellerID,
		)
		if err != nil {
			return fmt.Errorf("get promote balance account: %w", err)
		}
		balance, err := s.ledger.GetAccountBalance(ctx, tx, promoteBalanceID)
		if err != nil {
			return fmt.Errorf("read promote balance: %w", err)
		}

		requiredCost := input.BudgetRupiah
		availableFunding := balance.Int64()
		shortage := requiredCost - availableFunding
		if shortage < 0 {
			shortage = 0
		}

		preview = &FundingPreview{
			RequiredCost:     requiredCost,
			AvailableFunding: availableFunding,
			Shortage:         shortage,
			PaymentRequired:  shortage > 0,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return preview, nil
}

// ============================================================================
// PAUSE / RESUME — explicit seller pause shifts planned finish
// ============================================================================

// Pause stops ordinary delivery for an EXPLICIT seller pause. Allocation is
// retained, the seller slot stays occupied (status 'paused' is non-finalized),
// and planned_finish is NOT shifted yet — the shift is applied at resume by
// the exact measured pause duration. The pause start is recorded in DB time.
func (s *PromotionContractService) Pause(ctx context.Context, input PausePromotionInput) error {
	if input.SellerID == uuid.Nil || input.ContractID == uuid.Nil {
		return fmt.Errorf("PausePromotion: seller_id and contract_id required")
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		c, err := s.repo.GetForUpdate(ctx, tx, input.ContractID)
		if err != nil {
			return err
		}
		if c.SellerID != input.SellerID {
			return ErrPromotionContractNotOwned
		}
		if c.Status == entity.StatusPaused {
			return ErrPromotionAlreadyPaused
		}
		if c.Status != entity.StatusActive {
			return ErrPromotionPauseNotAllowed
		}

		now, err := s.repo.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read db time: %w", err)
		}
		c.Status = entity.StatusPaused
		c.PausedAt = &now
		if err := s.repo.Update(ctx, tx, c); err != nil {
			return err
		}
		s.log.Info("promotion_contract_paused",
			zap.String("contract_id", c.ID.String()),
			zap.String("seller_id", c.SellerID.String()),
			zap.Time("paused_at", now),
		)
		return nil
	})
}

// Resume restores delivery and shifts planned_finish by the exact explicit
// seller pause duration (DB-time based):
//
//	new_planned_finish = old_planned_finish + (resume_db_time - pause_db_time)
//
// Repeated pause/resume cycles accumulate because planned_finish is updated
// on every resume. No non-seller condition (low traffic, target unavailable,
// governance issue, platform disable) can shift planned_finish — no code path
// outside Resume mutates it.
func (s *PromotionContractService) Resume(ctx context.Context, input ResumePromotionInput) error {
	if input.SellerID == uuid.Nil || input.ContractID == uuid.Nil {
		return fmt.Errorf("ResumePromotion: seller_id and contract_id required")
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		c, err := s.repo.GetForUpdate(ctx, tx, input.ContractID)
		if err != nil {
			return err
		}
		if c.SellerID != input.SellerID {
			return ErrPromotionContractNotOwned
		}
		if c.Status != entity.StatusPaused {
			return ErrPromotionResumeNotAllowed
		}
		if c.PausedAt == nil {
			return fmt.Errorf("promotion %s is paused without a recorded paused_at", c.ID)
		}

		now, err := s.repo.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read db time: %w", err)
		}
		extension := now.Sub(*c.PausedAt)
		if extension < 0 {
			return ErrPromotionResumeClockInvalid
		}

		c.PlannedFinish = c.PlannedFinish.Add(extension)
		c.Status = entity.StatusActive
		c.PausedAt = nil
		if err := s.repo.Update(ctx, tx, c); err != nil {
			return err
		}
		s.log.Info("promotion_contract_resumed",
			zap.String("contract_id", c.ID.String()),
			zap.String("seller_id", c.SellerID.String()),
			zap.Duration("explicit_pause_extension", extension),
			zap.Time("new_planned_finish", c.PlannedFinish),
		)
		return nil
	})
}

// ============================================================================
// FINALIZE — seller stop / planned-finish completion boundary
// ============================================================================

// Finalize runs the canonical finalization boundary:
//
//	freeze issuance: invalidate every outstanding 'issued' Delivery Ticket
//	  (Phase 3 boundary) so a finalized contract can never let an old ticket
//	  produce a Qualified Impression or a charge
//	→ release remaining allocation exactly once (amount read from the ACTUAL
//	  allocation ledger balance, never from a mutable contract counter)
//	→ status finalized
//
// Release uses FinanceService.RecordPromotionAllocationRelease whose ledger
// idempotency key is per contract, so a duplicate/concurrent finalization can
// never release twice (the row lock serializes duplicates first).
//
// Lock order (contract row first) makes the Qualify vs Finalize race legal:
// either the qualify committed its charge before finalize observed the
// updated allocation (exact remainder released), or finalize invalidated the
// tickets first and the qualify observes a finalized contract / invalidated
// ticket and never charges. There is no third outcome.
func (s *PromotionContractService) Finalize(ctx context.Context, input FinalizePromotionInput) error {
	if input.SellerID == uuid.Nil || input.ContractID == uuid.Nil {
		return fmt.Errorf("FinalizePromotion: seller_id and contract_id required")
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		c, err := s.repo.GetForUpdate(ctx, tx, input.ContractID)
		if err != nil {
			return err
		}
		if c.SellerID != input.SellerID {
			return ErrPromotionContractNotOwned
		}
		return s.runFinalization(ctx, tx, c)
	})
}

// FinalizeBySystem is the system-triggered finalization entry used by the
// planned-finish finalization worker. It runs the SAME canonical finalization
// boundary as the seller path (one finalization authority, no duplicated
// logic) minus the seller-ownership check: the worker acts on behalf of the
// platform, not on behalf of a caller. Concurrency safety is inherited from
// the boundary itself — the contract row lock serializes concurrent triggers
// (worker vs worker, worker vs seller stop), the status check makes the
// second arrival a no-op, and the ledger release idempotency key
// (promotion_allocation_release_<contract_id>) guarantees the remaining
// allocation is released EXACTLY ONCE no matter how many processes observe
// the same contract.
func (s *PromotionContractService) FinalizeBySystem(ctx context.Context, contractID uuid.UUID) error {
	if contractID == uuid.Nil {
		return fmt.Errorf("FinalizeBySystem: contract_id required")
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		c, err := s.repo.GetForUpdate(ctx, tx, contractID)
		if err != nil {
			return err
		}
		return s.runFinalization(ctx, tx, c)
	})
}

// FinalizeDueContracts finalizes every contract whose planned delivery
// window has reached planned_finish (Owner truth: planned-finish completion
// finalizes automatically — the seller must never have to finalize manually
// just to recover unused allocation).
//
// It is an ORCHESTRATION entry, not a second finalization authority: each
// due contract is delegated to the canonical FinalizeBySystem boundary in
// its own transaction. A failure on one contract is logged and skipped so a
// single bad row cannot stall the queue; the next cycle retries it because
// the due query re-selects non-finalized past-finish contracts.
// Returns the number of contracts actually finalized this run.
func (s *PromotionContractService) FinalizeDueContracts(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("FinalizeDueContracts: limit must be positive (got %d)", limit)
	}
	var due []uuid.UUID
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		now, err := s.repo.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read db time: %w", err)
		}
		due, err = s.repo.ListDueForFinalization(ctx, tx, now, limit)
		return err
	})
	if err != nil {
		return 0, err
	}

	finalized := 0
	for _, id := range due {
		if err := s.FinalizeBySystem(ctx, id); err != nil {
			if errors.Is(err, ErrPromotionAlreadyFinalized) {
				// Concurrent trigger (another worker/process or a seller stop)
				// won the race — the contract is finalized exactly once, which
				// is the required outcome.
				continue
			}
			s.log.Warn("promotion_planned_finish_finalization_failed",
				zap.String("contract_id", id.String()),
				zap.Error(err),
			)
			continue
		}
		finalized++
		s.log.Info("promotion_planned_finish_finalized",
			zap.String("contract_id", id.String()),
		)
	}
	return finalized, nil
}

// runFinalization is the single canonical finalization core shared by the
// seller stop path (Finalize) and the planned-finish system path
// (FinalizeBySystem). Caller must have already loaded the contract row
// FOR UPDATE inside the caller's transaction.
func (s *PromotionContractService) runFinalization(ctx context.Context, tx db.Tx, c *entity.Contract) error {
	if c.Status.IsFinalized() || c.Status == entity.StatusFinalizing {
		return ErrPromotionAlreadyFinalized
	}

	// Freeze outstanding tickets BEFORE any release: an issued ticket can
	// never charge after finalization (Phase 3 boundary).
	if err := s.delivery.InvalidateIssuedForContract(ctx, tx, c.ID); err != nil {
		return fmt.Errorf("invalidate outstanding delivery tickets: %w", err)
	}

	now, err := s.repo.GetDBTime(ctx, tx)
	if err != nil {
		return fmt.Errorf("read db time: %w", err)
	}

	// Remaining allocation comes from the ledger account balance (FOR
	// UPDATE) — the only financial truth.
	allocationBal, err := s.ledger.GetAccountBalanceForUpdate(ctx, tx, c.AllocationAccountID)
	if err != nil {
		return fmt.Errorf("read allocation balance for finalization: %w", err)
	}
	remaining := allocationBal.Int64()

	if remaining > 0 {
		if err := s.finance.RecordPromotionAllocationRelease(ctx, tx, c.ID, c.SellerID, remaining); err != nil {
			return fmt.Errorf("release remaining allocation: %w", err)
		}
	}

	c.Status = entity.StatusFinalized
	c.FinalizedAt = &now
	c.PausedAt = nil
	if err := s.repo.Update(ctx, tx, c); err != nil {
		return err
	}

	s.log.Info("promotion_contract_finalized",
		zap.String("contract_id", c.ID.String()),
		zap.String("seller_id", c.SellerID.String()),
		zap.Int64("released_rupiah", remaining),
	)
	return nil
}

// Get returns a contract owned by the seller (read-only).
func (s *PromotionContractService) Get(ctx context.Context, sellerID, contractID uuid.UUID) (*entity.Contract, error) {
	if sellerID == uuid.Nil || contractID == uuid.Nil {
		return nil, fmt.Errorf("GetPromotion: seller_id and contract_id required")
	}
	var out *entity.Contract
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		c, err := s.repo.GetByID(ctx, tx, contractID)
		if err != nil {
			return err
		}
		if c.SellerID != sellerID {
			return ErrPromotionContractNotOwned
		}
		out = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// List returns every contract owned by the seller, newest first (read-only).
// Seller scoping is enforced by the query itself — a caller can only ever see
// contracts bound to the seller id they supply, so an HTTP handler passing the
// authenticated caller id can never read another seller's contracts.
func (s *PromotionContractService) List(ctx context.Context, sellerID uuid.UUID) ([]*entity.Contract, error) {
	if sellerID == uuid.Nil {
		return nil, fmt.Errorf("ListPromotions: seller_id required")
	}
	var out []*entity.Contract
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		out, err = s.repo.ListBySeller(ctx, tx, sellerID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PromotionContractService) GetGeographyCityIDs(ctx context.Context, contractID uuid.UUID) ([]string, error) {
	var out []string
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		geos, err := s.geographies.ListByContract(ctx, tx, contractID)
		if err != nil {
			return err
		}
		for _, g := range geos {
			out = append(out, g.CityID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

// ============================================================================
// TARGET QUEUE — Internal rolling queue max 10, External same authority
// ============================================================================

var (
	ErrQueueFull             = errors.New("promotion target queue is full (max 10)")
	ErrQueueDuplicate        = errors.New("target already in promotion queue")
	ErrQueueNotInternal      = errors.New("target queue only for internal promotions")
	ErrQueueExternalMismatch = errors.New("external promotion target type mismatch")
	ErrQueueContractNotFound = errors.New("promotion contract not found for queue operation")
)

type AddTargetInput struct {
	SellerID   uuid.UUID
	ContractID uuid.UUID
	TargetType string
	TargetID   uuid.UUID
}

func (s *PromotionContractService) AddTarget(ctx context.Context, input AddTargetInput) (*entity.ContractTarget, error) {
	if input.SellerID == uuid.Nil || input.ContractID == uuid.Nil || input.TargetID == uuid.Nil {
		return nil, fmt.Errorf("AddTarget: seller, contract and target required")
	}
	if input.TargetType == "" {
		return nil, fmt.Errorf("AddTarget: target_type required")
	}
	var created *entity.ContractTarget
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		c, err := s.repo.GetForUpdate(ctx, tx, input.ContractID)
		if err != nil {
			return err
		}
		if c.SellerID != input.SellerID {
			return ErrPromotionContractNotOwned
		}
		if c.Status.IsFinalized() {
			return ErrPromotionAlreadyFinalized
		}
		// Internal vs External kind check. The queue target enum
		// (promotion_target_type_enum, migration 000066) is the DB authority:
		// for_sale / auction for internal, external_product for external. The
		// external_product entity is the only external subject that exists in
		// this codebase (event/business subjects have no entity yet).
		if c.Kind == entity.KindInternal {
			if input.TargetType != "for_sale" && input.TargetType != "auction" {
				return ErrQueueExternalMismatch
			}
		} else if c.Kind == entity.KindExternal {
			if input.TargetType != "external_product" {
				return ErrQueueExternalMismatch
			}
		} else {
			return ErrPromotionKindInvalid
		}
		// Seller ownership validation.
		if s.operability != nil {
			if err := s.operability.ValidateOwnership(ctx, input.SellerID, input.TargetType, &input.TargetID); err != nil {
				return fmt.Errorf("target ownership: %w", err)
			}
			operable, reason, err := s.operability.CheckOperability(ctx, input.TargetType, &input.TargetID)
			if err != nil {
				return err
			}
			if !operable {
				return fmt.Errorf("target not operable: %s", reason)
			}
		}
		n, err := s.targets.CountTargets(ctx, tx, c.ID)
		if err != nil {
			return err
		}
		if n >= entity.MaxTargetsPerContract {
			return ErrQueueFull
		}
		// Duplicate prevention is also enforced by DB UNIQUE(contract_id, target_id).
		now, err := s.repo.GetDBTime(ctx, tx)
		if err != nil {
			return err
		}
		ct := &entity.ContractTarget{
			ID:         uuid.New(),
			ContractID: c.ID,
			TargetType: promoEntityTargetType(input.TargetType),
			TargetID:   input.TargetID,
			Position:   n,
			AddedAt:    now,
		}
		if err := s.targets.AddTarget(ctx, tx, ct); err != nil {
			if isQueueDuplicateViolation(err) {
				return ErrQueueDuplicate
			}
			return err
		}
		created = ct
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

type RemoveTargetInput struct {
	SellerID   uuid.UUID
	ContractID uuid.UUID
	TargetID   uuid.UUID
}

func (s *PromotionContractService) RemoveTarget(ctx context.Context, input RemoveTargetInput) error {
	if input.SellerID == uuid.Nil || input.ContractID == uuid.Nil || input.TargetID == uuid.Nil {
		return fmt.Errorf("RemoveTarget: ids required")
	}
	return s.db.WithTx(ctx, func(tx db.Tx) error {
		c, err := s.repo.GetForUpdate(ctx, tx, input.ContractID)
		if err != nil {
			return err
		}
		if c.SellerID != input.SellerID {
			return ErrPromotionContractNotOwned
		}
		if c.Status.IsFinalized() {
			return ErrPromotionAlreadyFinalized
		}
		return s.targets.RemoveTarget(ctx, tx, c.ID, input.TargetID)
	})
}

func (s *PromotionContractService) ListTargets(ctx context.Context, sellerID, contractID uuid.UUID) ([]*entity.ContractTarget, error) {
	if sellerID == uuid.Nil || contractID == uuid.Nil {
		return nil, fmt.Errorf("ListTargets: ids required")
	}
	var out []*entity.ContractTarget
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		c, err := s.repo.GetByID(ctx, tx, contractID)
		if err != nil {
			return err
		}
		if c.SellerID != sellerID {
			return ErrPromotionContractNotOwned
		}
		list, err := s.targets.ListTargets(ctx, tx, c.ID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ResolveEffectiveTarget returns the first operable queue entry, skipping unavailable.
func (s *PromotionContractService) ResolveEffectiveTarget(ctx context.Context, contractID uuid.UUID) (*entity.ContractTarget, error) {
	var resolved *entity.ContractTarget
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		checker := func(tt promoentity.TargetType, tid *uuid.UUID) (bool, string, error) {
			if s.operability == nil {
				return true, "", nil
			}
			return s.operability.CheckOperability(ctx, string(tt), tid)
		}
		t, err := s.targets.ResolveEffectiveTarget(ctx, tx, contractID, checker)
		if err != nil {
			return err
		}
		resolved = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

// Geography helpers — canonical targeting

func (s *PromotionContractService) ListGeographies(ctx context.Context, tx db.Tx, contractID uuid.UUID) ([]*entity.ContractGeography, error) {
	return s.geographies.ListByContract(ctx, tx, contractID)
}

func (s *PromotionContractService) HasGeographicRestriction(ctx context.Context, tx db.Tx, contractID uuid.UUID) (bool, error) {
	return s.geographies.HasGeographicRestriction(ctx, tx, contractID)
}

func (s *PromotionContractService) IsCityAllowed(ctx context.Context, tx db.Tx, contractID uuid.UUID, cityID string) (bool, error) {
	return s.geographies.IsCityAllowed(ctx, tx, contractID, cityID)
}

// canonicalizeCityIDs dedupes deterministically and resolves snapshots from canonical_geographies vocabulary
// Viewer location authority (addresses) is NOT vocabulary — vocabulary is canonical_geographies table
func (s *PromotionContractService) canonicalizeCityIDs(ctx context.Context, tx db.Tx, cityIDs []string) ([]entity.ContractGeography, error) {
	if len(cityIDs) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(cityIDs))
	var out []entity.ContractGeography
	for _, raw := range cityIDs {
		cid := strings.TrimSpace(raw)
		if cid == "" {
			continue
		}
		if _, ok := seen[cid]; ok {
			continue
		}
		seen[cid] = struct{}{}
		var cityName, provinceID string
		err := tx.QueryRow(ctx, `SELECT city_name, province_id FROM canonical_geographies WHERE city_id = $1`, cid).Scan(&cityName, &provinceID)
		if err != nil {
			return nil, fmt.Errorf("invalid city_id %s: not in canonical geography vocabulary", cid)
		}
		if cityName == "" || provinceID == "" {
			return nil, fmt.Errorf("invalid city_id %s: canonical geography missing city_name or province_id", cid)
		}
		out = append(out, entity.ContractGeography{CityID: cid, CityName: cityName, ProvinceID: provinceID})
	}
	return out, nil
}

func promoEntityTargetType(s string) promoentity.TargetType {
	return promoentity.TargetType(s)
}

func isQueueDuplicateViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if strings.Contains(pgErr.ConstraintName, "promotion_contract_targets_unique_target") {
			return true
		}
		if strings.Contains(pgErr.Message, "promotion_contract_targets_unique_target") {
			return true
		}
	}
	return strings.Contains(err.Error(), "promotion_contract_targets_unique_target")
}

// mulNonNeg computes a*b for non-negative int64s, reporting overflow instead
// of silently wrapping. Fail-closed: callers reject on overflow.
func mulNonNeg(a, b int64) (int64, bool) {
	if a <= 0 || b <= 0 {
		return 0, false
	}
	if a > math.MaxInt64/b {
		return 0, true
	}
	return a * b, false
}

// isSellerSlotUniqueViolation reports whether err is a PostgreSQL unique
// violation on one of the seller-slot partial unique indexes
// (ux_promotion_contracts_one_nonfinalized_{internal,external}_per_seller).
func isSellerSlotUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	const prefix = "ux_promotion_contracts_one_nonfinalized_"
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" && strings.HasPrefix(pgErr.ConstraintName, prefix) {
			return true
		}
		if pgErr.ConstraintName == "" && pgErr.Code == "23505" && strings.Contains(pgErr.Message, prefix) {
			return true
		}
		return false
	}
	return strings.Contains(err.Error(), prefix)
}
