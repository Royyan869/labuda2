package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	contractRepo "github.com/labuda/backend/internal/pricing/promotion/contract/repository"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// DeliveryCandidate is the MINIMUM canonical delivery representation: the
// identity and target facts a downstream delivery/distribution consumer needs
// to hydrate and render a promoted item.
//
// ContractID is the canonical contract identity (promotion_contracts.id).
// It deliberately carries NO lifecycle fields (status, paused_at,
// planned_finish), NO funding fields, and NO operability fields: selection
// has already produced the single authority's verdict for the candidate
// window, and reconstructing any of those gates here would create a second
// authority. The hard billing authority remains Delivery Ticket issuance +
// Qualification (delivery/application), which re-validates everything inside
// its own locked transaction.
type DeliveryCandidate struct {
	ContractID uuid.UUID
	SellerID   uuid.UUID
	TargetType string
	TargetID   uuid.UUID
}

// SelectionOperabilityChecker is the canonical target/seller operability
// authority the selection boundary defers to (the shared OperabilityCheckerImpl
// also used by contract queue management and delivery qualification).
type SelectionOperabilityChecker interface {
	CheckOperability(ctx context.Context, targetType string, targetID *uuid.UUID) (bool, string, error)
}

// DeliveryHandoffService is the canonical contract-based delivery selection
// boundary: it turns "which promotion contracts should be considered for
// delivery right now?" into a concrete candidate list — and nothing more.
//
// Authority rule:
//   - Contract truth (status active, planned window) comes from
//     promotion_contracts — the single lifecycle authority.
//   - Target truth comes from the rolling queue
//     (promotion_contract_targets) resolved through the shared commerce
//     OperabilityChecker — Promotion never duplicates For Sale / Auction /
//     External Product lifecycle state.
//   - Allocation sufficiency is read from the holder-scoped
//     PROMOTION_ALLOCATION ledger account balance (financial_accounts) —
//     ledger balance is the financial truth; mutable counters never are.
//
// Selection is read-only and deterministic (created_at order). It is a
// distribution filter, NOT a billing authority: Delivery Ticket issuance and
// Qualification re-validate contract status, pacing, geography, target
// eligibility and allocation inside their own locked transactions.
type DeliveryHandoffService struct {
	db          *db.DB
	contracts   contractRepo.Repository
	targets     contractRepo.ContractTargetRepository
	operability SelectionOperabilityChecker
}

// NewDeliveryHandoffService wires the canonical contract selection boundary.
func NewDeliveryHandoffService(
	dbConn *db.DB,
	contracts contractRepo.Repository,
	targets contractRepo.ContractTargetRepository,
	operability SelectionOperabilityChecker,
) *DeliveryHandoffService {
	return &DeliveryHandoffService{
		db:          dbConn,
		contracts:   contracts,
		targets:     targets,
		operability: operability,
	}
}

// candidatePoolFactor widens the contract candidate query so target
// resolution skips (inoperable queue entries) cannot starve the requested
// limit.
const candidatePoolFactor = 3

// SelectForDelivery returns up to limit delivery candidates, each backed by
// an active contract inside its planned window with a resolved operable
// queue target and non-zero allocation. Contracts without an operable target
// are skipped (rolling queue semantics — unavailable targets never block a
// contract's other entries).
func (s *DeliveryHandoffService) SelectForDelivery(ctx context.Context, limit int) ([]DeliveryCandidate, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("delivery selection limit must be positive (got %d)", limit)
	}

	var candidates []DeliveryCandidate
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		now, err := s.contracts.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read selection time authority: %w", err)
		}

		pool := limit * candidatePoolFactor
		contracts, err := s.listCandidateContracts(ctx, tx, now, pool)
		if err != nil {
			return err
		}

		out := make([]DeliveryCandidate, 0, limit)
		for _, c := range contracts {
			if len(out) >= limit {
				break
			}
			// Allocation sufficiency: ledger account balance (financial
			// truth), never a mutable counter.
			hasFunds, err := s.allocationHasBalance(ctx, tx, c.AllocationAccountID)
			if err != nil {
				return err
			}
			if !hasFunds {
				continue
			}
			// Rolling queue resolution: first operable entry, skipping
			// inoperable targets without duplicating commerce authority.
			target, err := s.resolveEffectiveTarget(ctx, tx, c.ID)
			if err != nil {
				return err
			}
			if target == nil {
				continue
			}
			out = append(out, DeliveryCandidate{
				ContractID: c.ID,
				SellerID:   c.SellerID,
				TargetType: string(target.TargetType),
				TargetID:   target.TargetID,
			})
		}
		candidates = out
		return nil
	})
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

// listCandidateContracts returns active contracts inside their planned
// delivery window, oldest first (deterministic), bounded by pool. The pool is
// intentionally larger than the requested limit because target resolution may
// skip contracts whose effective target is inoperable.
func (s *DeliveryHandoffService) listCandidateContracts(
	ctx context.Context,
	tx db.Tx,
	now time.Time,
	pool int,
) ([]*entity.Contract, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, seller_id, kind, status, budget_rupiah, cpm_rupiah,
		       planned_start, planned_finish, allocation_account_id,
		       paused_at, finalized_at, created_at, updated_at
		FROM promotion_contracts
		WHERE status = 'active'
		  AND planned_start <= $1
		  AND planned_finish > $1
		ORDER BY created_at ASC
		LIMIT $2
	`, now, pool)
	if err != nil {
		return nil, fmt.Errorf("select promotion contract candidates: %w", err)
	}
	defer rows.Close()

	var out []*entity.Contract
	for rows.Next() {
		var c entity.Contract
		var kind, status string
		if err := rows.Scan(
			&c.ID, &c.SellerID, &kind, &status,
			&c.BudgetRupiah, &c.CPMRupiah,
			&c.PlannedStart, &c.PlannedFinish, &c.AllocationAccountID,
			&c.PausedAt, &c.FinalizedAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan promotion contract candidate: %w", err)
		}
		c.Kind = entity.Kind(kind)
		c.Status = entity.Status(status)
		out = append(out, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promotion contract candidates: %w", err)
	}
	return out, nil
}

// allocationHasBalance reports whether the holder-scoped allocation account
// still carries a positive ledger balance (PROMOTION_ALLOCATION is the
// financial truth for usable budget).
func (s *DeliveryHandoffService) allocationHasBalance(ctx context.Context, tx db.Tx, allocationAccountID uuid.UUID) (bool, error) {
	var balance int64
	err := tx.QueryRow(ctx, `
		SELECT balance FROM financial_accounts WHERE id = $1
	`, allocationAccountID).Scan(&balance)
	if err != nil {
		return false, fmt.Errorf("read promotion allocation balance: %w", err)
	}
	return balance > 0, nil
}

// resolveEffectiveTarget returns the first operable queue entry for the
// contract, or nil when every queued target is inoperable / the queue is
// empty. The checker defers to the canonical commerce OperabilityChecker.
func (s *DeliveryHandoffService) resolveEffectiveTarget(ctx context.Context, tx db.Tx, contractID uuid.UUID) (*entity.ContractTarget, error) {
	checker := func(tt promoentity.TargetType, tid *uuid.UUID) (bool, string, error) {
		if s.operability == nil {
			return true, "", nil
		}
		return s.operability.CheckOperability(ctx, string(tt), tid)
	}
	return s.targets.ResolveEffectiveTarget(ctx, tx, contractID, checker)
}