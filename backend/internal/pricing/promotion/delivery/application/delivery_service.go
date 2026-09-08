// Package application implements the canonical Promotion delivery boundary:
// Delivery Ticket issuance (Model A — NO money movement) and server-side
// Qualification producing an immutable Qualified Impression whose exact
// charge flows through FinanceService (PROMOTION_ALLOCATION -> PLATFORM_REVENUE).
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/finance"
	financeapp "github.com/labuda/backend/internal/finance/application"
	configapp "github.com/labuda/backend/internal/platform/config/application"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	contractRepo "github.com/labuda/backend/internal/pricing/promotion/contract/repository"
	deliveryentity "github.com/labuda/backend/internal/pricing/promotion/delivery/entity"
	deliveryRepo "github.com/labuda/backend/internal/pricing/promotion/delivery/repository"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ============================================================================
// CANONICAL LOCK ORDER (single consistent order for every promotion path)
// ============================================================================
//
//	1. promotion_contracts row            (FOR UPDATE)
//	2. promotion_delivery_tickets row     (FOR UPDATE)
//	3. financial_accounts allocation row  (FOR UPDATE, inside FinanceService)
//	4. financial_accounts promote balance (FOR UPDATE, release path only)
//
// Every promotion state transition (create, pause, resume, finalize, issue,
// qualify) acquires the CONTRACT row lock first, so all transitions of one
// contract serialize on that row. Ticket rows are always locked after their
// contract; finance rows are always touched after the contract lock.
// Therefore no lock cycle exists between promotion paths, and the Qualify vs
// Finalize race resolves to exactly two legal outcomes:
//
//	Outcome A: qualify wins the contract lock -> charge commits -> finalize
//	           later reads the updated allocation and releases the exact
//	           remainder.
//	Outcome B: finalize wins the contract lock -> tickets invalidated +
//	           allocation released + status finalized -> qualify observes a
//	           non-active contract (or invalidated ticket) and never charges.
//
// There is NO outcome where a ticket charges after the allocation was
// released, because release and charge both require the contract lock first.
// ============================================================================

// DefaultTicketTTL is the server-derived expiry window for an issued ticket.
// Expiry is checked against the DB clock at qualification time; client
// timestamps are never trusted.
const DefaultTicketTTL = 15 * time.Minute

// Domain errors.
var (
	// ErrContractNotActive is returned when a ticket is issued or qualified
	// against a contract that is not in the active state (finalized,
	// finalizing, paused). Only active contracts may issue/qualify.
	ErrContractNotActive = errors.New("promotion contract is not active")

	// ErrTicketNotFound is returned when the ticket id does not exist.
	ErrTicketNotFound = errors.New("promotion delivery ticket not found")

	// ErrTicketNotIssued is returned when a ticket is not in the 'issued'
	// state (already consumed or invalidated). A consumed ticket is the
	// idempotent signal that its one Qualified Impression already exists.
	ErrTicketNotIssued = errors.New("promotion delivery ticket is not in issued state")

	// ErrTicketExpired is returned when a ticket's server-derived expiry has
	// passed at qualification time (DB clock authority).
	ErrTicketExpired = errors.New("promotion delivery ticket expired")

	// ErrTicketSelfDelivery is returned when the ticket's bound viewer is the
	// contract's seller: seller self-delivery is never billable.
	ErrTicketSelfDelivery = errors.New("seller self-delivery is not billable")

	// ErrDeliveryDisabled is returned when the platform-wide canonical config
	// gate disables promotion delivery.
	ErrDeliveryDisabled = errors.New("promotion delivery is disabled")

	// ErrPacingThrottled is returned when pacing envelope throttles issuance.
	ErrPacingThrottled = errors.New("promotion pacing throttled: over envelope")

	// ErrGeoIneligible is returned when viewer's canonical city is not in contract's allowed set.
	ErrGeoIneligible = errors.New("viewer geography ineligible for promotion contract")
)

// ErrTargetIneligible is returned when the canonical commerce authority
// reports the ticket's target as no longer operable at qualification time.
type ErrTargetIneligible struct {
	Reason string
}

func (e *ErrTargetIneligible) Error() string {
	return fmt.Sprintf("promotion target ineligible: %s", e.Reason)
}

// IssueTicketInput captures the canonical ticket issuance inputs. Money rule
// (Model A): issuance moves NO money — no ledger transaction, no allocation
// reservation, no balance mutation.
type IssueTicketInput struct {
	ContractID uuid.UUID
	TargetType promoentity.TargetType
	TargetID   uuid.UUID
	// ViewerID binds the audience identity. It is NOT NULL: seller
	// self-delivery must be detectable at qualification.
	ViewerID uuid.UUID
	// Viewer geography — canonical primary address city_id, empty when viewer has no primary.
	ViewerCityID     string
	ViewerHasPrimary bool
	// TTL is the server-derived ticket lifetime (expires_at = DB now + TTL).
	// Zero uses DefaultTicketTTL.
	TTL time.Duration
}

// QualifyTicketInput targets one issued ticket for server-side qualification.
type QualifyTicketInput struct {
	TicketID uuid.UUID
}

// DeliveryService owns the canonical ticket issuance and qualification
// boundary.
type DeliveryService struct {
	db          *db.DB
	contracts   contractRepo.Repository
	tickets     deliveryRepo.Repository
	geographies contractRepo.ContractGeographyRepository
	finance     *financeapp.FinanceService
	config      *configapp.ConfigService
	eligibility TargetEligibility
	log         *zap.Logger
}

// NewDeliveryService wires the canonical delivery service.
func NewDeliveryService(
	dbConn *db.DB,
	contracts contractRepo.Repository,
	tickets deliveryRepo.Repository,
	financeService *financeapp.FinanceService,
	configService *configapp.ConfigService,
	eligibility TargetEligibility,
) *DeliveryService {
	return &DeliveryService{
		db:          dbConn,
		contracts:   contracts,
		tickets:     tickets,
		geographies: nil,
		finance:     financeService,
		config:      configService,
		eligibility: eligibility,
		log:         zap.NewNop(),
	}
}

// SetGeographyRepository wires the geography repository for geo eligibility.
func (s *DeliveryService) SetGeographyRepository(r contractRepo.ContractGeographyRepository) {
	s.geographies = r
}

// SetLogger wires a structured logger (optional).
func (s *DeliveryService) SetLogger(logger *zap.Logger) {
	if logger != nil {
		s.log = logger
	}
}

func (s *DeliveryService) promotionDeliveryEnabled(ctx context.Context, tx db.Tx) (bool, error) {
	if s.config == nil {
		return false, errors.New("delivery service config authority is missing")
	}
	return s.config.IsPromotionDeliveryEnabledResult(ctx, tx)
}

func (s *DeliveryService) isGeographicallyEligible(ctx context.Context, tx db.Tx, contractID uuid.UUID, viewerCityID string, viewerHasPrimary bool) (bool, error) {
	if s.geographies == nil {
		return true, nil
	}
	hasRestriction, err := s.geographies.HasGeographicRestriction(ctx, tx, contractID)
	if err != nil {
		return false, err
	}
	if !hasRestriction {
		return true, nil
	}
	if !viewerHasPrimary || viewerCityID == "" {
		return false, nil
	}
	return s.geographies.IsCityAllowed(ctx, tx, contractID, viewerCityID)
}

func (s *DeliveryService) resolveViewerCityTx(ctx context.Context, tx db.Tx, viewerID uuid.UUID) (string, bool, error) {
	if viewerID == uuid.Nil {
		return "", false, nil
	}
	var cityID string
	err := tx.QueryRow(ctx, `SELECT city_id FROM addresses WHERE user_id = $1 AND is_primary = true AND is_available_for_checkout = true LIMIT 1`, viewerID).Scan(&cityID)
	if err != nil {
		return "", false, nil
	}
	if cityID == "" {
		return "", false, nil
	}
	return cityID, true, nil
}

// enforcePacingEnvelope implements V1 pacing: time-based envelope with tolerance,
// bounded catch-up, traffic-aware throttling. Returns ErrPacingThrottled if over envelope.
func (s *DeliveryService) enforcePacingEnvelope(ctx context.Context, tx db.Tx, c *entity.Contract, now time.Time) error {
	totalDuration := c.PlannedFinish.Sub(c.PlannedStart)
	if totalDuration <= 0 {
		return nil
	}
	elapsed := now.Sub(c.PlannedStart)
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > totalDuration {
		// Past planned finish — ordinary issuance should stop.
		return ErrContractNotActive
	}
	// Estimate spent via QI count * CPM cumulative (fail-open if count unavailable).
	n, err := s.tickets.CountQualifiedImpressions(ctx, tx, c.ID)
	if err != nil {
		return nil
	}
	spentApprox := n * c.CPMRupiah / 1000
	expected := int64(float64(c.BudgetRupiah) * float64(elapsed) / float64(totalDuration))
	tolerance := expected + expected/3 // 1.33x envelope
	if spentApprox > tolerance && float64(elapsed)/float64(totalDuration) < 0.8 {
		return ErrPacingThrottled
	}
	return nil
}

// ============================================================================
// ISSUE — authorization only, NO money movement (Model A)
// ============================================================================

// IssueTicket creates a server-side delivery authorization for one potential
// delivery. It locks the contract row (canonical lock order), requires an
// active contract, rejects seller self-delivery at issuance, and derives
// issued_at / expires_at from the DB clock.
//
// Money boundary: this method performs NO ledger transaction, NO allocation
// reservation, NO balance mutation. The ticket is authorization/state only.
func (s *DeliveryService) IssueTicket(ctx context.Context, input IssueTicketInput) (*deliveryentity.DeliveryTicket, error) {
	if input.ContractID == uuid.Nil {
		return nil, fmt.Errorf("IssueTicket: contract_id required")
	}
	if input.TargetID == uuid.Nil {
		return nil, fmt.Errorf("IssueTicket: target_id required")
	}
	if input.ViewerID == uuid.Nil {
		return nil, fmt.Errorf("IssueTicket: viewer_id required (audience binding)")
	}
	if !input.TargetType.IsValid() {
		return nil, fmt.Errorf("IssueTicket: invalid target type %q", input.TargetType)
	}
	ttl := input.TTL
	if ttl <= 0 {
		ttl = DefaultTicketTTL
	}

	var created *deliveryentity.DeliveryTicket
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		// CANONICAL DELIVERY GATE
		enabled, err := s.promotionDeliveryEnabled(ctx, tx)
		if err != nil {
			return err
		}
		if !enabled {
			return ErrDeliveryDisabled
		}

		// Lock order step 1: contract row first.
		c, err := s.contracts.GetForUpdate(ctx, tx, input.ContractID)
		if err != nil {
			return err
		}
		now, err := s.contracts.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read db time: %w", err)
		}
		if c.Status != entity.StatusActive {
			return ErrContractNotActive
		}
		if c.SellerID == input.ViewerID {
			return ErrTicketSelfDelivery
		}
		// Geographic eligibility — empty geography = nationwide, otherwise viewer city must be allowed
		if eligible, err := s.isGeographicallyEligible(ctx, tx, c.ID, input.ViewerCityID, input.ViewerHasPrimary); err != nil {
			return err
		} else if !eligible {
			return ErrGeoIneligible
		}
		// Pacing envelope: time-based envelope + bounded catch-up + traffic-aware throttling.
		// High early traffic must not exhaust budget. We check expected spend vs actual.
		if err := s.enforcePacingEnvelope(ctx, tx, c, now); err != nil {
			return err
		}

		ticket := &deliveryentity.DeliveryTicket{
			ID:         uuid.New(),
			ContractID: c.ID,
			TargetType: input.TargetType,
			TargetID:   input.TargetID,
			ViewerID:   input.ViewerID,
			Status:     deliveryentity.TicketStatusIssued,
			IssuedAt:   now,
			ExpiresAt:  now.Add(ttl),
			CreatedAt:  now,
		}
		if err := s.tickets.CreateTicket(ctx, tx, ticket); err != nil {
			return err
		}
		created = ticket
		s.log.Info("promotion_delivery_ticket_issued",
			zap.String("ticket_id", ticket.ID.String()),
			zap.String("contract_id", ticket.ContractID.String()),
			zap.String("target_type", string(ticket.TargetType)),
			zap.String("target_id", ticket.TargetID.String()),
			zap.String("viewer_id", ticket.ViewerID.String()),
			zap.Time("expires_at", ticket.ExpiresAt),
		)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// ============================================================================
// QUALIFY — the ONLY path that moves promotion money
// ============================================================================// QualifyTicket is the canonical server-side qualification boundary. It
// produces exactly one Qualified Impression for the ticket and books the
// exact charge through FinanceService.RecordQualifiedImpression
// (PROMOTION_ALLOCATION -> PLATFORM_REVENUE).
//
// Pre-flight (outside the tx): the canonical target-eligibility gate re-reads
// commerce state through the OperabilityChecker (ticket snapshot is never
// trusted for billing). This deliberately runs BEFORE the qualification
// transaction: the checker reads via its own pool connection, and acquiring a
// second pool connection while a transaction holds row locks can exhaust a
// small pool and deadlock concurrent qualifications. Target eligibility is a
// business gate, not a money-integrity gate — money safety is guaranteed
// inside the tx by the contract/ticket/allocation locks.
//
// Transaction (single db.WithTx, lock order contract -> ticket -> target):
//
//  1. read ticket (no lock) to discover the contract id
//  2. lock contract FOR UPDATE          (serializes with finalize/issue)
//  3. lock ticket FOR UPDATE
//  4. validate ticket: issued, unexpired (DB clock), viewer != seller
//  5. validate contract: status == active
//  6. revalidate CANONICAL target eligibility on the same tx connection
//     (target row FOR UPDATE — closes the pre-flight TOCTOU window: a
//     target that lost purchase availability after the pre-flight gate is
//     observed here and can never bill; no second pool acquisition, so no
//     connection-pool deadlock)
//  7. N = COUNT(QI for contract) + 1    (stable under the contract lock)
//  8. charge = finance.PromotionCharge(N, contract CPM snapshot)
//  9. create the immutable Qualified Impression row
//  10. FinanceService.RecordQualifiedImpression (allocation sufficiency +
//     ledger charge; zero-charge is a documented no-op)
//  11. mark ticket consumed
//  12. commit
//
// The whole qualification is one DB transaction: any validation failure rolls
// back the ticket consume, the QI row, and the ledger charge together.
// N and charge are deterministic: concurrent qualifications of the same
// contract serialize on the contract row, so N is unique and the cumulative
// CPM charge is exact (UNIQUE(contract_id, sequence_n) is the DB backstop).
func (s *DeliveryService) QualifyTicket(ctx context.Context, input QualifyTicketInput) (*deliveryentity.QualifiedImpression, error) {
	if input.TicketID == uuid.Nil {
		return nil, fmt.Errorf("QualifyTicket: ticket_id required")
	}

	// Pre-flight: canonical target eligibility re-check (pool read, outside
	// the tx — see the doc comment above for why).
	probe, err := s.tickets.ReadTicket(ctx, input.TicketID)
	if err != nil {
		return nil, err
	}
	operable, reason, err := s.eligibility.CheckOperability(ctx, probe.TargetType, &probe.TargetID)
	if err != nil {
		return nil, fmt.Errorf("target eligibility check failed: %w", err)
	}
	if !operable {
		return nil, &ErrTargetIneligible{Reason: reason}
	}

	var qualified *deliveryentity.QualifiedImpression
	err = s.db.WithTx(ctx, func(tx db.Tx) error {
		// CANONICAL DELIVERY GATE (independent check for in-flight tickets)
		enabled, err := s.promotionDeliveryEnabled(ctx, tx)
		if err != nil {
			return err
		}
		if !enabled {
			return ErrDeliveryDisabled
		}

		// Step 1: un-locked read only to discover the contract id. Ticket
		// rows are immutable in contract_id, so this read is stable.
		probe, err := s.tickets.GetByID(ctx, tx, input.TicketID)
		if err != nil {
			return err
		}

		// Lock order step 1: contract row first.
		c, err := s.contracts.GetForUpdate(ctx, tx, probe.ContractID)
		if err != nil {
			return err
		}
		now, err := s.contracts.GetDBTime(ctx, tx)
		if err != nil {
			return fmt.Errorf("read db time: %w", err)
		}

		// Lock order step 2: ticket row. All validation reads the locked row.
		ticket, err := s.tickets.GetForUpdate(ctx, tx, input.TicketID)
		if err != nil {
			return err
		}

		// 4. Ticket validity.
		if !ticket.Status.CanQualify() {
			return ErrTicketNotIssued
		}
		if !ticket.ExpiresAt.After(now) {
			return ErrTicketExpired
		}
		if ticket.ViewerID == c.SellerID {
			return ErrTicketSelfDelivery
		}

		// 5. Contract validity. The locked re-read reflects the latest
		// committed state, so a finalize that won the contract lock first is
		// observed here (Outcome B).
		if c.Status != entity.StatusActive {
			return ErrContractNotActive
		}

		// 6. Transaction-boundary target eligibility revalidation (canonical
		// commerce authority, same tx connection — no second pool
		// acquisition, no connection-pool deadlock). The target row is read
		// FOR UPDATE so the eligibility judgment serializes against
		// concurrent target-state mutations: a target that lost purchase
		// availability between the pre-flight gate and this transaction is
		// observed here and can never produce a billable QI.
		operable, reason, err := s.eligibility.CheckOperabilityTx(ctx, tx, now, ticket.TargetType, &ticket.TargetID)
		if err != nil {
			return fmt.Errorf("target eligibility tx revalidation failed: %w", err)
		}
		if !operable {
			return &ErrTargetIneligible{Reason: reason}
		}

		// 6b. Geographic revalidation — same Tx, same lock order, no second pool
		if s.geographies != nil {
			viewerCityID, viewerHasPrimary, _ := s.resolveViewerCityTx(ctx, tx, ticket.ViewerID)
			eligible, err := s.isGeographicallyEligible(ctx, tx, c.ID, viewerCityID, viewerHasPrimary)
			if err != nil {
				return err
			}
			if !eligible {
				return ErrGeoIneligible
			}
		}

		// 7. Canonical sequence N (stable under the contract lock).
		n, err := s.tickets.CountQualifiedImpressions(ctx, tx, c.ID)
		if err != nil {
			return fmt.Errorf("count qualified impressions: %w", err)
		}
		n++

		// 8. Exact cumulative CPM charge from the contract's immutable
		// pricing snapshot (Phase 1 authority — no second formula).
		charge, err := finance.PromotionCharge(n, c.CPMRupiah)
		if err != nil {
			return fmt.Errorf("compute promotion charge: %w", err)
		}

		// 9. Immutable billable fact (UNIQUE(ticket_id) is the DB authority).
		qi := &deliveryentity.QualifiedImpression{
			ID:                  uuid.New(),
			TicketID:            ticket.ID,
			ContractID:          c.ID,
			AllocationAccountID: c.AllocationAccountID,
			TargetType:          ticket.TargetType,
			TargetID:            ticket.TargetID,
			SequenceN:           n,
			ChargeRupiah:        charge,
			ServerOccurredAt:    now,
			CreatedAt:           now,
		}
		if err := s.tickets.CreateQualifiedImpression(ctx, tx, qi); err != nil {
			return fmt.Errorf("create qualified impression: %w", err)
		}

		// 10. FinanceService charge (allocation sufficiency + ledger; a
		// zero-charge QI performs no money movement, matching Phase 1).
		if err := s.finance.RecordQualifiedImpression(ctx, tx, qi.ID, c.ID, c.SellerID, charge); err != nil {
			return fmt.Errorf("promotion qualified impression charge failed: %w", err)
		}

		// 11. Consume the ticket in the same transaction.
		if err := s.tickets.MarkConsumed(ctx, tx, ticket.ID, now); err != nil {
			return err
		}

		qualified = qi
		s.log.Info("promotion_qualified_impression_recorded",
			zap.String("qi_id", qi.ID.String()),
			zap.String("ticket_id", qi.TicketID.String()),
			zap.String("contract_id", qi.ContractID.String()),
			zap.Int64("sequence_n", qi.SequenceN),
			zap.Int64("charge_rupiah", qi.ChargeRupiah),
		)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return qualified, nil
}
