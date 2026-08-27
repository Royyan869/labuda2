// ⚠️ FINANCIAL RULE:
// All money operations MUST go through WalletService.
// Direct balance mutation is forbidden.
//
// Dispute domain manages dispute state and resolution.
// All financial operations are delegated to WalletService.
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	walletEntity "github.com/labuda/backend/internal/core/wallet/entity"
	"github.com/labuda/backend/internal/governance/dispute/entity"
	"github.com/labuda/backend/internal/governance/dispute/infrastructure/repository"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/repository"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/internal/platform/capability"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// ResolutionType represents the type of dispute resolution.
type ResolutionType string

const (
	// ResolutionRefund means the dispute is resolved in favor of the buyer (refund).
	ResolutionRefund ResolutionType = "refund"
	// ResolutionRelease means the dispute is resolved in favor of the seller (release).
	ResolutionRelease ResolutionType = "release"
	// ResolutionPartialSplit means the dispute is resolved with partial split (buyer gets item refund, seller gets shipping).
	ResolutionPartialSplit ResolutionType = "partial_split"
)

// getCallerType returns the type of caller (buyer, seller, or system).
func getCallerType(callerID, buyerID, sellerID uuid.UUID) string {
	if callerID == buyerID {
		return "buyer"
	}
	if callerID == sellerID {
		return "seller"
	}
	return "system"
}

// OpenDisputeInput contains the input for opening a dispute.
type OpenDisputeInput struct {
	Reason      string
	Description *string
	MediaURLs   []string
	VideoURL    *string // 🔥 PHASE 7: Video evidence (required for buyer disputes)
	ReasonCode  string  // 🔥 TASK 1: Standardized reason code (required)
}

// DisputeFreezeAuthority is the finance-domain surface used by DisputeService
// to coordinate dispute freeze bookkeeping when policy requires it. Defined
// as an interface so the governance/dispute package does not import
// finance/application directly (one-way dependency). The concrete
// implementation is *FinanceService wired in cmd/core_server/dependencies_core.go.
type DisputeFreezeAuthority interface {
	// CreateDisputeFreeze records a dispute freeze for compatibility with
	// legacy dispute-freeze bookkeeping.
	// frozenAmount is the seller's economic entitlement (BuyerBase − commission).
	CreateDisputeFreeze(ctx context.Context, tx db.Tx, disputeID, sellerID, orderID uuid.UUID, frozenAmount int64) error
	// ReleaseDisputeFreeze marks the freeze as released. Idempotent.
	ReleaseDisputeFreeze(ctx context.Context, tx db.Tx, disputeID uuid.UUID) error
}

// DisputeService handles dispute lifecycle operations.
//
// All financial operations (escrow freezing, refunds, releases) are delegated
// to OrderService to maintain single responsibility principle.
//
// CRITICAL HARDENING: Uses live wallet state for escrow validation (not cached Order.EscrowStatus).
type DisputeService struct {
	disputeRepo     disputeRepo.DisputeRepository
	orderRepo       *orderRepo.OrderRepository
	orderService    *orderApp.OrderService
	walletService   *walletApp.WalletService // CRITICAL: For live escrow validation
	outboxRepo      *outboxRepo.OutboxRepository
	abuseService    *DisputeAbuseService   // 🔥 TASK 3: Abuse monitoring
	freezeAuthority DisputeFreezeAuthority // TASK 48: dispute freeze bookkeeping helper
	logger          *zap.Logger            // Optional; defaults to Nop
}

// NewDisputeService creates a new DisputeService.
func NewDisputeService(
	orderRepo *orderRepo.OrderRepository,
	orderService *orderApp.OrderService,
	walletService *walletApp.WalletService,
	outboxRepo *outboxRepo.OutboxRepository,
) *DisputeService {
	return &DisputeService{
		disputeRepo:   repository.NewDisputeRepository(),
		orderRepo:     orderRepo,
		orderService:  orderService,
		walletService: walletService, // CRITICAL: For live escrow validation
		outboxRepo:    outboxRepo,
		abuseService:  NewDisputeAbuseService(), // 🔥 TASK 3: Initialize abuse service
	}
}

// SetLogger wires the logger. Defaults to zap.NewNop() if unset.
func (s *DisputeService) SetLogger(l *zap.Logger) {
	s.logger = l
}

func (s *DisputeService) log() *zap.Logger {
	if s.logger != nil {
		return s.logger
	}
	return zap.NewNop()
}

// SetFreezeAuthority wires the finance-domain freeze surface. When unset,
// dispute freeze bookkeeping is skipped.
func (s *DisputeService) SetFreezeAuthority(a DisputeFreezeAuthority) {
	s.freezeAuthority = a
}

// OpenDispute opens a new dispute for an order.
//
// 🔥 COMPREHENSIVE UPDATE: PHASES 3-7
//
// Transaction flow:
// 1. Lock and get order for update
// 2. 🔥 PHASE 5: Authorization check (only buyer/seller)
// 3. 🔥 PHASE 4: Pre-ship dispute allowance (when overdue)
// 4. 🔥 PHASE 6: Post-ship dispute timing rules (12 hours window)
// 5. 🔥 PHASE 7: Video requirement for buyer disputes
// 6. Validate no existing dispute
// 7. Validate escrow status
// 8. Freeze escrow on order (via OrderService)
// 9. Create dispute record
// 10. Emit outbox event
//
// Caller must provide an active transaction.
func (s *DisputeService) OpenDispute(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	callerID uuid.UUID, // 🔥 PHASE 5: Authorization
	input OpenDisputeInput,
) (*entity.Dispute, error) {
	// Get order with lock
	order, err := s.orderRepo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// 🔥 PHASE 5: DISPUTE AUTHORIZATION (P0 SECURITY)
	// Only buyer or seller can open disputes for their orders.
	// System callers are reserved for internal moderation flows and bypass
	// ownership checks, but they still must satisfy the business guards below.
	if !auth.IsSystemCaller(callerID) && !order.IsUserAuthorizedForDispute(callerID) {
		return nil, &orderEntity.ErrUnauthorizedDisputeAccess{
			OrderID:  order.ID,
			UserID:   callerID,
			BuyerID:  order.BuyerID,
			SellerID: order.SellerID,
		}
	}

	// FINALITY GUARD: once the order is completed and escrow is released,
	// no caller may open a dispute. Post-release objections are handled
	// outside the app.
	if err := s.enforceAppDisputeFinality(order, callerID); err != nil {
		return nil, err
	}

	// 🔥 TASK 3: ABUSE MONITORING (P1 SECURITY)
	// Check if user has abusive dispute patterns.
	// counterpartyID is the other party in the order for repeated-dispute detection.
	counterpartyID := order.SellerID
	if callerID == order.SellerID {
		counterpartyID = order.BuyerID
	}
	if err := s.abuseService.CheckUserBeforeDispute(ctx, tx, callerID, counterpartyID); err != nil {
		return nil, fmt.Errorf("abuse check failed: %w", err)
	}

	// Check for existing dispute
	if order.HasDispute {
		return nil, ErrDisputeOpenAlreadyHasActive
	}

	// CRITICAL HARDENING: Validate escrow status using LIVE wallet state (not cached Order.EscrowStatus).
	// PRE-RELEASE  (escrow=holding)  → standard dispute path: MarkDisputeOpen + status=dispute_open.
	// POST-RELEASE (escrow=released) → blocked by finality guard above.
	walletEscrow, err := s.walletService.GetEscrowForOrder(ctx, tx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify escrow state: %w", err)
	}
	if walletEscrow == nil {
		return nil, ErrDisputeOpenNoEscrow
	}
	switch walletEscrow.Status {
	case walletEntity.EscrowStatusHolding:
		// pre-release path — normal
	case walletEntity.EscrowStatusReleased:
		return nil, ErrDisputeOpenAfterCompletion
	default:
		return nil, ErrDisputeOpenInvalidEscrowState
	}
	// 🔥 PHASE 4: PRE-SHIP DISPUTE ALLOWANCE
	// Allow disputes before shipping if order is overdue
	var allowPreShipDispute bool
	if order.Status == orderEntity.StatusPaid {
		// Can only open pre-ship dispute if order is overdue AND caller is buyer
		if callerID == order.BuyerID && order.IsBuyerEligibleForPreShipDispute() {
			allowPreShipDispute = true
		} else {
			return nil, ErrDisputeOpenPreShipNotEligible
		}
	}

	// 🔥 PHASE 6: POST-SHIP DISPUTE TIMING (12 HOURS)
	// After shipping, disputes are only allowed within 12 hours window
	// OR if order is overdue (pre-ship dispute path)
	// B4A: Dispute is available from SHIPPED status only (before buyer accepts).
	// Once buyer taps "Terima Barang" → Complete(), dispute path is closed.
	if order.Status == orderEntity.StatusShipped {
		// Check if within post-ship dispute window
		if !order.IsWithinPostShipDisputeWindow() && !allowPreShipDispute {
			return nil, ErrDisputeOpenPostShipWindowExpired
		}
	}

	// Build evidence URLs first (needed for video requirement check below).
	evidenceURLs := input.MediaURLs
	if input.VideoURL != nil {
		evidenceURLs = append(evidenceURLs, *input.VideoURL)
	}

	// 🔥 PHASE 7: VIDEO REQUIREMENT FOR BUYER DISPUTES
	// Buyer disputes require video evidence. For escalated refund disputes,
	// evidence_urls carried from the original refund satisfy this requirement.
	if callerID == order.BuyerID && input.VideoURL == nil && len(evidenceURLs) == 0 {
		return nil, &orderEntity.ErrVideoRequiredForBuyerDispute{
			OrderID: order.ID,
		}
	}

	// 🔥 TASK 1: SELLER DISPUTE VALIDATION
	// Validate reason code is provided and valid
	if input.ReasonCode == "" {
		return nil, &entity.ErrMissingReasonCode{}
	}

	// Pre-release: mark dispute open (order→dispute_open, escrow stays holding).
	if err := s.orderService.MarkDisputeOpen(ctx, tx, orderID); err != nil {
		return nil, fmt.Errorf("failed to mark dispute open: %w", err)
	}

	// 🔥 TASK 1: Create dispute entity with enhanced details
	dispute, err := entity.NewDisputeWithDetails(
		order.ID,
		order.BuyerID,
		order.SellerID,
		callerID, // Track who opened the dispute
		input.Reason,
		input.Description,
		input.ReasonCode, // Required reason code
		evidenceURLs,     // All evidence (video + media)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dispute entity: %w", err)
	}

	// 🔥 TASK 1: Additional validation for seller disputes
	if callerID == order.SellerID {
		if err := dispute.ValidateForSeller(); err != nil {
			return nil, fmt.Errorf("seller dispute validation failed: %w", err)
		}
	}

	// Persist dispute
	if err := s.disputeRepo.Create(ctx, tx, dispute); err != nil {
		return nil, fmt.Errorf("failed to create dispute: %w", err)
	}

	// Media attachments are now handled in Create() method

	// Emit outbox event
	payload, _ := json.Marshal(map[string]interface{}{
		"dispute_id":  dispute.ID,
		"order_id":    orderID,
		"buyer_id":    order.BuyerID,
		"seller_id":   order.SellerID,
		"caller_id":   callerID, // 🔥 PHASE 5: Track who opened the dispute
		"reason":      input.Reason,
		"reason_code": input.ReasonCode, // 🔥 TASK 1: Track standardized reason code
		"status":      string(dispute.Status),
		"opened_at":   dispute.OpenedAt,
		"has_video":   input.VideoURL != nil,                                  // 🔥 PHASE 7: Track video evidence
		"caller_type": getCallerType(callerID, order.BuyerID, order.SellerID), // 🔥 TASK 1: Track caller type for analytics
	})

	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"dispute.opened",
		dispute.ID,
		payload,
	); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return dispute, nil
}

// enforceAppDisputeFinality blocks normal app-initiated disputes once the
// order has reached the completed + released terminal state.
func (s *DisputeService) enforceAppDisputeFinality(order *orderEntity.Order, callerID uuid.UUID) error {
	if order.Status == orderEntity.StatusCompleted &&
		order.EscrowStatus == orderEntity.EscrowStatusReleased {
		return fmt.Errorf("cannot open dispute after order completion; handle objections outside the app")
	}
	return nil
}

// ResolveDispute resolves a dispute in favor of the buyer (refund) or seller (release).
//
// Transaction flow:
// 1. Lock and get dispute for update
// 2. Update dispute status with admin attribution and notes
// 3. If refund: delegate to OrderService.RefundFromDispute()
// 4. If release: delegate to OrderService.ReleaseFromDispute()
// 5. Emit outbox event with admin attribution
//
// Caller must provide an active transaction.
//
// All financial operations (ledger, state updates) are delegated to OrderService.
//
// The adminID and notes parameters are stored for audit trail purposes,
// ensuring every financial dispute decision is attributable.
func (s *DisputeService) ResolveDispute(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
	resolution ResolutionType,
	adminID uuid.UUID,
	notes *string,
) error {
	// 🔥 P0 SECURITY: Capability check for dispute resolution
	// System caller (workers) bypass capability check
	if !audit.IsSystemCaller(adminID) {
		if !capability.HasCapability(ctx, capability.CapFinanceDisputeResolve.String()) {
			return ErrDisputeResolutionCapabilityRequired
		}
	}

	// Get dispute with lock
	dispute, err := s.disputeRepo.GetForUpdate(ctx, tx, disputeID)
	if err != nil {
		return fmt.Errorf("dispute not found: %w", err)
	}

	// Validate dispute can be resolved
	if !dispute.CanResolve() {
		return fmt.Errorf("%w: %s", ErrDisputeResolveInvalidState, dispute.Status)
	}

	now := time.Now()

	order, err := s.orderRepo.GetForUpdate(ctx, tx, dispute.OrderID)
	if err != nil {
		return fmt.Errorf("order not found for dispute: %w", err)
	}
	if order.Status == orderEntity.StatusCompleted &&
		order.EscrowStatus == orderEntity.EscrowStatusReleased {
		return ErrDisputeResolveAfterCompletion
	}

	switch resolution {
	case ResolutionRefund:
		if err := dispute.ResolveRefund(now, adminID, notes); err != nil {
			return fmt.Errorf("failed to resolve dispute: %w", err)
		}
		if err := s.disputeRepo.Update(ctx, tx, dispute); err != nil {
			return fmt.Errorf("failed to update dispute: %w", err)
		}
		// Pre-release buyer-wins: escrow still in gateway_clearing → refund.
		if err := s.orderService.RefundFromDispute(ctx, tx, dispute.OrderID, adminID); err != nil {
			return fmt.Errorf("failed to refund: %w", err)
		}

	case ResolutionRelease:
		if err := dispute.ResolveRelease(now, adminID, notes); err != nil {
			return fmt.Errorf("failed to resolve dispute: %w", err)
		}
		if err := s.disputeRepo.Update(ctx, tx, dispute); err != nil {
			return fmt.Errorf("failed to update dispute: %w", err)
		}
		// Pre-release seller-wins: release escrow → SELLER_PAYABLE.
		if err := s.orderService.ReleaseFromDispute(ctx, tx, dispute.OrderID); err != nil {
			return fmt.Errorf("failed to release: %w", err)
		}

	case ResolutionPartialSplit:
		// Pre-release partial: item refund to buyer, shipping release to seller.
		if err := s.orderService.PartialRefundFromDispute(ctx, tx, dispute.OrderID, adminID); err != nil {
			return fmt.Errorf("failed to partial refund: %w", err)
		}
		if err := dispute.ResolvePartialSplit(now, adminID, notes); err != nil {
			return fmt.Errorf("failed to resolve dispute: %w", err)
		}
		if err := s.disputeRepo.Update(ctx, tx, dispute); err != nil {
			return fmt.Errorf("failed to update dispute: %w", err)
		}

	default:
		return fmt.Errorf("invalid resolution type: %s", resolution)
	}

	// Emit outbox event with admin attribution for audit trail
	// D1A: buyer_id/seller_id enrichment for notification handler
	payload, _ := json.Marshal(map[string]interface{}{
		"dispute_id":       dispute.ID,
		"order_id":         dispute.OrderID,
		"buyer_id":         dispute.BuyerID,
		"seller_id":        dispute.SellerID,
		"resolution":       string(resolution),
		"status":           string(dispute.Status),
		"resolved_at":      dispute.ResolvedAt,
		"resolved_by":      adminID,
		"resolution_notes": notes,
		"capability_used":  capability.CapFinanceDisputeResolve.String(),
	})

	if err := s.outboxRepo.InsertEvent(
		ctx, tx,
		"dispute.resolved",
		dispute.ID,
		payload,
	); err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// GetDispute retrieves a dispute without locking.
func (s *DisputeService) GetDispute(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) (*entity.Dispute, error) {
	return s.disputeRepo.GetForUpdate(ctx, tx, disputeID)
}

// GetDisputeByOrderID retrieves a dispute by order ID without locking.
func (s *DisputeService) GetDisputeByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*entity.Dispute, error) {
	return s.disputeRepo.GetByOrderID(ctx, tx, orderID)
}

// GetDisputeMedia retrieves all media URLs for a dispute.
func (s *DisputeService) GetDisputeMedia(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) ([]string, error) {
	return s.disputeRepo.ListMedia(ctx, tx, disputeID)
}

// ============================================================================
// ADMIN QUERY METHODS
// ============================================================================

// DisputeEntity is an alias for the dispute entity for handler convenience.
type DisputeEntity = entity.Dispute

// ListAll retrieves all disputes with optional filters for admin.
func (s *DisputeService) ListAll(
	ctx context.Context,
	tx db.Tx,
	filters disputeRepo.DisputeListFilters,
) ([]*entity.Dispute, int64, error) {
	return s.disputeRepo.ListAll(ctx, tx, filters)
}

// GetDisputeByID retrieves a dispute by ID without locking (for read-only admin view).
func (s *DisputeService) GetDisputeByID(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (*entity.Dispute, error) {
	return s.disputeRepo.GetByID(ctx, tx, id)
}

// =============================================================================
// DEADLOCK PREVENTION METHODS
// =============================================================================

// MarkOverdue marks a dispute as overdue for escalation.
// This is called by the timeout worker when a dispute exceeds the overdue threshold.
func (s *DisputeService) MarkOverdue(
	ctx context.Context,
	tx db.Tx,
	disputeID uuid.UUID,
) error {
	// Get dispute with lock
	dispute, err := s.disputeRepo.GetForUpdate(ctx, tx, disputeID)
	if err != nil {
		return fmt.Errorf("dispute not found: %w", err)
	}

	// Mark as overdue
	now := time.Now()
	if err := dispute.MarkAsOverdue(now); err != nil {
		return fmt.Errorf("failed to mark as overdue: %w", err)
	}

	// Update dispute record
	if err := s.disputeRepo.Update(ctx, tx, dispute); err != nil {
		return fmt.Errorf("failed to update dispute: %w", err)
	}

	return nil
}

// OpenDisputeFromEscalation creates a dispute linked to a refund escalation.
//
// This is a lightweight path that skips the timing window and video evidence
// requirements that apply to direct disputes — those were validated when the
// buyer originally created the refund request.
//
// Preconditions (enforced by caller / RefundService.EscalateToDispute):
//   - Refund exists and was in seller_rejected status (now escalated_to_admin)
//   - Caller is the buyer of the order
//
// This method:
//  1. Validates no existing dispute on the order
//  2. Validates escrow is still holding
//  3. Creates dispute entity with refund's reason/evidence
//  4. Marks order as dispute_open via OrderService
//  5. Persists the dispute
//  6. Emits dispute.opened outbox event
func (s *DisputeService) OpenDisputeFromEscalation(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	callerID uuid.UUID,
	reason string,
	description *string,
	reasonCode string,
	evidenceURLs []string,
) (*entity.Dispute, error) {
	// Lock order
	order, err := s.orderRepo.GetForUpdate(ctx, tx, orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	// Authorization check (caller must be buyer or seller of the order)
	if !order.IsUserAuthorizedForDispute(callerID) {
		return nil, &orderEntity.ErrUnauthorizedDisputeAccess{
			OrderID:  order.ID,
			UserID:   callerID,
			BuyerID:  order.BuyerID,
			SellerID: order.SellerID,
		}
	}

	// Duplicate dispute guard
	if order.HasDispute {
		return nil, fmt.Errorf("cannot open dispute: order already has an active dispute")
	}

	// Escrow validation — escalation only valid while escrow is holding
	walletEscrow, err := s.walletService.GetEscrowForOrder(ctx, tx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify escrow state: %w", err)
	}
	if walletEscrow == nil || walletEscrow.Status != walletEntity.EscrowStatusHolding {
		return nil, fmt.Errorf("cannot escalate: escrow not in holding state")
	}

	// Create dispute entity
	dispute, err := entity.NewDisputeWithDetails(
		order.ID,
		order.BuyerID,
		order.SellerID,
		callerID,
		reason,
		description,
		reasonCode,
		evidenceURLs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dispute entity: %w", err)
	}

	// Mark order as dispute_open (blocks auto-release)
	if err := s.orderService.MarkDisputeOpen(ctx, tx, orderID); err != nil {
		return nil, fmt.Errorf("failed to mark dispute open: %w", err)
	}

	// Persist dispute
	if err := s.disputeRepo.Create(ctx, tx, dispute); err != nil {
		return nil, fmt.Errorf("failed to create dispute: %w", err)
	}

	// Emit dispute.opened outbox event
	payload, _ := json.Marshal(map[string]interface{}{
		"dispute_id":     dispute.ID,
		"order_id":       orderID,
		"buyer_id":       order.BuyerID,
		"seller_id":      order.SellerID,
		"reason":         reason,
		"reason_code":    reasonCode,
		"status":         string(dispute.Status),
		"opened_at":      dispute.OpenedAt,
		"caller_type":    "buyer",
		"escalated_from": "refund",
	})

	if err := s.outboxRepo.InsertEvent(ctx, tx, "dispute.opened", dispute.ID, payload); err != nil {
		return nil, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return dispute, nil
}

// UpdateDispute updates a dispute entity (used by workers for state changes).
func (s *DisputeService) UpdateDispute(
	ctx context.Context,
	tx db.Tx,
	dispute *entity.Dispute,
) error {
	return s.disputeRepo.Update(ctx, tx, dispute)
}
