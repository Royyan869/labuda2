// ⚠️ FINANCIAL RULE:
// All money operations MUST go through WalletService.
// Direct balance mutation is forbidden.
//
// Order domain is a PRICING SNAPSHOT only.
// Wallet domain is the SINGLE SOURCE OF TRUTH for all money operations.
package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/entity"
	orderRepoImpl "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingRepoImpl "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	auditApp "github.com/labuda/backend/internal/governance/audit/application"
	disputerepo "github.com/labuda/backend/internal/governance/dispute/repository"
	"github.com/labuda/backend/internal/identity/auth"
	coinsApp "github.com/labuda/backend/internal/incentive/coins/application"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"go.uber.org/zap"
)

// OrderService is a facade that delegates to specialized order services.
// It keeps the orchestration layer thin while separating concerns.
type OrderService struct {
	repo              orderrepository.OrderRepository
	creationService   *OrderCreationService
	paymentService    *OrderPaymentService
	completionService *OrderCompletionService
}

// PaymentService returns the underlying OrderPaymentService for late dependency
// wiring (e.g. SetFinanceReleaseRecorder during dependency injection bootstrap).
// Domain logic should NOT use this — call methods on OrderService instead.
func (s *OrderService) PaymentService() *OrderPaymentService {
	return s.paymentService
}

// NewOrderService creates a new OrderService facade with all dependencies.
// Constructor signature remains stable for existing callers.
func NewOrderService(
	accountStatusChecker auth.AccountStatusChecker,
	shippingService *shippingApp.ShippingService,
	outboxRepo *outboxRepo.OutboxRepository,
	configService *platformconfigApp.ConfigService,
	coinsService *coinsApp.CoinsService,
	roleChecker auth.RoleChecker,
	actorResolver capabilityEntity.ActorResolver, // SERVICE LAYER ENFORCEMENT
	auditService *auditApp.AuditService, // OBSERVABILITY: Audit service
	productShippingRepo shippingRepoImpl.ProductShippingSetupRepository, // DI: Product shipping options
	walletService *walletApp.WalletService, // WALLET PHASE 1: Escrow hold on order creation
	shippingQuoteService ShippingQuoteService, // HARD FIX: Shipping quote reactivation
) *OrderService {
	// Create payment service first (needed by other services)
	paymentService := NewOrderPaymentService(walletService)

	// Create specialized services
	creationService := NewOrderCreationService(
		accountStatusChecker,
		shippingService,
		outboxRepo,
		configService,
		roleChecker,
		actorResolver,       // SERVICE LAYER ENFORCEMENT
		auditService,        // Pass audit service to creation service
		productShippingRepo, // DI: Product shipping options
		nil,                 // auctionStatusChecker - optional
		walletService,       // WALLET PHASE 1: Escrow hold on order creation
	)

	completionService := NewOrderCompletionService(
		accountStatusChecker,
		outboxRepo,
		paymentService,
		coinsService,
		shippingQuoteService, // HARD FIX: Shipping quote reactivation
		nil,                  // disputeRepo - will be set later
		walletService,        // Used to derive Order.EscrowStatus from Wallet state
		zap.NewNop(),         // Logger is required but not used in this facade
	)

	return &OrderService{
		repo:              orderRepoImpl.NewOrderRepository(),
		creationService:   creationService,
		paymentService:    paymentService,
		completionService: completionService,
	}
}

// GetCreationService returns the OrderCreationService for external access.
// Used by chat handler for chat-centric order creation.
func (s *OrderService) GetCreationService() *OrderCreationService {
	return s.creationService
}

// SetCommerceGovRepository wires the canonical commerce restriction repository
// into the order creation path so buyer restriction is enforced inside the
// same transaction as the order mutation.
func (s *OrderService) SetCommerceGovRepository(repo commercegov.Repository) {
	s.creationService.SetCommerceGovRepository(repo)
}

// SetShippingQuoteService sets the shipping quote service for shipping quote reactivation.
// This allows the service to be wired up after OrderService creation to avoid circular dependencies.
func (s *OrderService) SetShippingQuoteService(shippingQuoteService ShippingQuoteService) {
	s.completionService.shippingQuoteService = shippingQuoteService
}

// SetCommerceViolationRepo wires the canonical commerce violation/restriction
// repository into the completion path (auction settlement-failure rollback).
func (s *OrderService) SetCommerceViolationRepo(repo commercegov.Repository) {
	s.completionService.SetCommerceViolationRepo(repo)
}

// SetDisputeRepository sets the dispute repository for entry point guards.
// This allows the repository to be wired up after OrderService creation to avoid circular dependencies.
func (s *OrderService) SetDisputeRepository(disputeRepo disputerepo.DisputeRepository) {
	s.completionService.disputeRepo = disputeRepo
}

// SetCoinsService wires the coins service into the completion path.
//
// CoinsService is built after OrderService in the bootstrap, so this setter
// is the post-construction injection point. Loyalty points granted at order
// completion are a NON-CANONICAL side effect — the completion service guards
// for a nil coinsService and skips the reward if it is not wired, so missing
// this setter call only loses loyalty points, never blocks completion or
// money movement.
func (s *OrderService) SetCoinsService(coinsService *coinsApp.CoinsService) {
	s.completionService.coinsService = coinsService
}

// SetActiveRefundChecker wires the active refund checker into the completion path.
//
// H2-F2a MONEY-SAFETY: Blocks auto-completion when a refund is being
// negotiated or settled, preventing escrow release while refund is in flight.
// The completion service has a defensive nil guard — if this setter is never
// called, the refund check is skipped (open-by-default for backward compat).
func (s *OrderService) SetActiveRefundChecker(checker ActiveRefundChecker) {
	s.completionService.activeRefundChecker = checker
}

// GetOrder retrieves an order by ID.
func (s *OrderService) GetOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*entity.Order, error) {
	return s.repo.GetByID(ctx, tx, orderID)
}

// GetOrderForValidation retrieves an order's buyer and seller IDs for ownership validation.
// Used by support domain to verify ticket creation is linked to a valid order.
func (s *OrderService) GetOrderForValidation(
	ctx context.Context,
	orderID uuid.UUID,
) (buyerID, sellerID uuid.UUID, err error) {
	order, err := s.repo.GetByID(ctx, nil, orderID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return order.BuyerID, order.SellerID, nil
}

// ============================================================================
// ORDER CREATION METHODS (delegated to OrderCreationService)
// ============================================================================

// CreateFromAuction creates an order from a completed auction.
func (s *OrderService) CreateFromAuction(
	ctx context.Context,
	tx db.Tx,
	input CreateFromAuctionInput,
) (*entity.Order, error) {
	return s.creationService.CreateFromAuction(ctx, tx, input)
}

// CreateFromSaleSurface creates a direct purchase order from a sale surface.
// Supports optional negotiation context via Input.NegotiationID field.
func (s *OrderService) CreateFromSaleSurface(
	ctx context.Context,
	tx db.Tx,
	input CreateFromSaleSurfaceInput,
) (*entity.Order, error) {
	return s.creationService.CreateFromSaleSurface(ctx, tx, input)
}

// RefundToBuyer flips the order's escrow to "refunded" (gateway-funded model).
// No wallet balance mutation; ledger reversal flows through the refund pipeline.
func (s *OrderService) RefundToBuyer(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
) error {
	return s.paymentService.RefundToBuyer(ctx, tx, order)
}

// PartialRefundLedger flips the order's escrow to "released" for partial
// dispute resolution (gateway-funded model).
func (s *OrderService) PartialRefundLedger(
	ctx context.Context,
	tx db.Tx,
	order *entity.Order,
	refundAmount money.Money,
) error {
	return s.paymentService.PartialRefundLedger(ctx, tx, order, refundAmount)
}

// ============================================================================
// ORDER COMPLETION METHODS (delegated to OrderCompletionService)
// ============================================================================

// MarkPaid transitions an order from pending to paid.
func (s *OrderService) MarkPaid(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	return s.completionService.MarkPaid(ctx, tx, orderID)
}

// MarkShipped transitions an order from paid to shipped.
//
// SHIPPING PROOF REQUIREMENTS (STRICT - NO FAKE SHIPMENT):
// - proofType: REQUIRED - "tracking" | "phone" | "manual"
// - shippingReference: REQUIRED for tracking/phone types
// - shippingProofMedia: REQUIRED for manual type
// - note: Optional shipping note
func (s *OrderService) MarkShipped(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	orderID uuid.UUID,
	idempotencyKey string,
	proofType *string,
	shippingReference *string,
	shippingProofMedia *string,
	note *string,
) error {
	return s.completionService.MarkShipped(ctx, tx, callerID, orderID, idempotencyKey, proofType, shippingReference, shippingProofMedia, note)
}

// Complete completes an order and releases escrow to seller.
// B4A: This is the canonical "Terima Barang" path — buyer's single-click final acceptance.
func (s *OrderService) Complete(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	orderID uuid.UUID,
	idempotencyKey string,
) error {
	return s.completionService.Complete(ctx, tx, callerID, orderID, idempotencyKey)
}

// ExtendConfirmation extends the confirmation window for a delivered order.
func (s *OrderService) ExtendConfirmation(
	ctx context.Context,
	tx db.Tx,
	callerID uuid.UUID,
	orderID uuid.UUID,
	idempotencyKey string,
) error {
	return s.completionService.ExtendConfirmation(ctx, tx, callerID, orderID, idempotencyKey)
}

// Cancel cancels an order and refunds escrow to buyer.
func (s *OrderService) Cancel(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	idempotencyKey string,
	callerID uuid.UUID,
) error {
	return s.completionService.Cancel(ctx, tx, orderID, idempotencyKey, callerID)
}

// CancelOverdue allows buyer to cancel an order that is overdue for shipment.
// 🔥 PHASE 3: BUYER FORCE ACTION
// This method allows buyers to cancel orders when seller has not shipped
// within the ReadyToShipBy + grace period deadline.
func (s *OrderService) CancelOverdue(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	idempotencyKey string,
	callerID uuid.UUID,
) error {
	return s.completionService.CancelOverdue(ctx, tx, orderID, idempotencyKey, callerID)
}

// Expire expires an order when payment times out.
func (s *OrderService) Expire(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	return s.completionService.Expire(ctx, tx, orderID)
}

// MarkDisputeOpen marks an order as being in dispute.
func (s *OrderService) MarkDisputeOpen(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	return s.completionService.MarkDisputeOpen(ctx, tx, orderID)
}

// MarkHasDisputePostRelease is retained as a guard for finalized dispute flow.
// The owner finality rule no longer allows completed+released dispute
// creation in the live runtime.
func (s *OrderService) MarkHasDisputePostRelease(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	return s.completionService.MarkHasDisputePostRelease(ctx, tx, orderID)
}

// RefundOrder refunds an order to buyer.
func (s *OrderService) RefundOrder(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	return s.completionService.RefundOrder(ctx, tx, orderID)
}

// RefundFromDispute refunds an order from a dispute resolution.
// adminID is the authenticated admin who authorized the resolution; it is stored
// as refunds.reviewed_by so the refund row carries the real actor identity.
func (s *OrderService) RefundFromDispute(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
) error {
	return s.completionService.RefundFromDispute(ctx, tx, orderID, adminID)
}

// ReleaseFromDispute releases an order from a dispute resolution.
func (s *OrderService) ReleaseFromDispute(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) error {
	return s.completionService.ReleaseFromDispute(ctx, tx, orderID)
}

// PartialRefundFromDispute resolves a dispute with partial split:
// - Buyer gets refund for item price (subtotal)
// - Seller gets release for shipping fee (shipping_total)
// adminID is the authenticated admin who authorized the resolution; it is stored
// as refunds.reviewed_by so the refund row carries the real actor identity.
func (s *OrderService) PartialRefundFromDispute(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
) error {
	return s.completionService.PartialRefundFromDispute(ctx, tx, orderID, adminID)
}

// RefundFromDisputePostRelease is parked under the owner finality rule.
// Post-release buyer objections are handled outside the app and the
// completion service now returns an explicit error instead of dispatching.
func (s *OrderService) RefundFromDisputePostRelease(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
) error {
	return s.completionService.RefundFromDisputePostRelease(ctx, tx, orderID, adminID)
}

// PartialRefundFromDisputePostRelease is parked under the owner finality rule.
// Post-release buyer objections are handled outside the app and the
// completion service now returns an explicit error instead of dispatching.
func (s *OrderService) PartialRefundFromDisputePostRelease(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	adminID uuid.UUID,
) error {
	return s.completionService.PartialRefundFromDisputePostRelease(ctx, tx, orderID, adminID)
}

// SyncRefundSettlementFromGatewayAck is the order-domain authority hook used
// by refund webhook ack handling to sync order.status and order.escrow_status.
// refundID is accepted for trace parity with caller context.
func (s *OrderService) SyncRefundSettlementFromGatewayAck(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	_ uuid.UUID,
	fullyRefunded bool,
	occurredAt time.Time,
) error {
	return s.completionService.SyncRefundSettlementFromGatewayAck(
		ctx, tx, orderID, fullyRefunded, occurredAt,
	)
}
