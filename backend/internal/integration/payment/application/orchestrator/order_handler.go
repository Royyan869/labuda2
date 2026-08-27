// ⚠️ INTEGRATION LAYER:
// This module is an external payment adapter.
// It does NOT contain business logic or money mutation.
//
// ⚠️ Payment domain does NOT handle money.
// It only handles gateway status and webhook processing.
package orchestrator

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/identity/auth"
	coinsapp "github.com/labuda/backend/internal/incentive/coins/application"
	coinsRepo "github.com/labuda/backend/internal/incentive/coins/infrastructure/repository"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	"github.com/labuda/backend/internal/commerce/order/application"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	orderrepository "github.com/labuda/backend/internal/commerce/order/repository"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	platformconfigRepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	shippingapp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingentity "github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/pkg/db"
)

// systemAccountStatusChecker is a no-op AccountStatusChecker for system operations.
// Orchestrator uses SystemCallerID, which bypasses account status checks.
type systemAccountStatusChecker struct{}

func (s *systemAccountStatusChecker) EnsureActive(ctx context.Context, userID uuid.UUID) error {
	return nil // System operations bypass account status checks
}

func (s *systemAccountStatusChecker) GetStatus(ctx context.Context, userID uuid.UUID) (string, error) {
	return "active", nil // System operations bypass account status checks
}

func (s *systemAccountStatusChecker) IsBanned(ctx context.Context, userID uuid.UUID) (bool, error) {
	return false, nil // System operations bypass account status checks
}

// systemRoleChecker is a minimal RoleChecker implementation for system operations.
type systemRoleChecker struct{}

func (s *systemRoleChecker) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return auth.IsSystemCaller(userID), nil
}

func (s *systemRoleChecker) IsSeller(ctx context.Context, userID uuid.UUID) (bool, error) {
	return auth.IsSystemCaller(userID), nil
}

func (s *systemRoleChecker) HasActiveSellerCapability(ctx context.Context, userID uuid.UUID) (bool, error) {
	return auth.IsSystemCaller(userID), nil
}

func (s *systemRoleChecker) HasSellerProfile(ctx context.Context, userID uuid.UUID) (bool, error) {
	return auth.IsSystemCaller(userID), nil
}

// Stub shipping repositories for orchestrator processing
type stubShippingOptionRepository struct{}

func (r *stubShippingOptionRepository) Create(ctx context.Context, tx db.Tx, option *shippingentity.ShippingOption) error {
	return nil
}
func (r *stubShippingOptionRepository) Update(ctx context.Context, tx db.Tx, option *shippingentity.ShippingOption) error {
	return nil
}
func (r *stubShippingOptionRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingentity.ShippingOption, error) {
	return nil, nil
}
func (r *stubShippingOptionRepository) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingentity.ShippingOption, error) {
	return nil, nil
}
func (r *stubShippingOptionRepository) GetBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, onlyActive bool) ([]*shippingentity.ShippingOption, error) {
	return nil, nil
}
func (r *stubShippingOptionRepository) GetByName(ctx context.Context, tx db.Tx, sellerID uuid.UUID, name string) (*shippingentity.ShippingOption, error) {
	return nil, nil
}
func (r *stubShippingOptionRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}

type stubShippingCoverageRepository struct{}

func (r *stubShippingCoverageRepository) Create(ctx context.Context, tx db.Tx, coverage *shippingentity.ShippingCoverage) error {
	return nil
}
func (r *stubShippingCoverageRepository) Update(ctx context.Context, tx db.Tx, coverage *shippingentity.ShippingCoverage) error {
	return nil
}
func (r *stubShippingCoverageRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingentity.ShippingCoverage, error) {
	return nil, nil
}
func (r *stubShippingCoverageRepository) GetByShippingOption(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID) ([]*shippingentity.ShippingCoverage, error) {
	return nil, nil
}
func (r *stubShippingCoverageRepository) GetByOptionAndProvince(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID, provinceCode string) (*shippingentity.ShippingCoverage, error) {
	return nil, nil
}
func (r *stubShippingCoverageRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}
func (r *stubShippingCoverageRepository) DeleteByShippingOption(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID) error {
	return nil
}

type stubCityOverrideRepository struct{}

func (r *stubCityOverrideRepository) Create(ctx context.Context, tx db.Tx, override *shippingentity.CityOverride) error {
	return nil
}
func (r *stubCityOverrideRepository) Update(ctx context.Context, tx db.Tx, override *shippingentity.CityOverride) error {
	return nil
}
func (r *stubCityOverrideRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingentity.CityOverride, error) {
	return nil, nil
}
func (r *stubCityOverrideRepository) GetByCoverage(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID) ([]*shippingentity.CityOverride, error) {
	return nil, nil
}
func (r *stubCityOverrideRepository) GetByCoverageAndCity(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID, cityCode string) (*shippingentity.CityOverride, error) {
	return nil, nil
}
func (r *stubCityOverrideRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}
func (r *stubCityOverrideRepository) DeleteByCoverage(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID) error {
	return nil
}

type stubListingShippingOptionRepository struct{}

func (r *stubListingShippingOptionRepository) Create(ctx context.Context, tx db.Tx, listingID uuid.UUID, shippingOptionID uuid.UUID, sortOrder int) error {
	return nil
}
func (r *stubListingShippingOptionRepository) Delete(ctx context.Context, tx db.Tx, listingID uuid.UUID, shippingOptionID uuid.UUID) error {
	return nil
}
func (r *stubListingShippingOptionRepository) GetByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*shippingentity.ShippingOption, error) {
	return nil, nil
}
func (r *stubListingShippingOptionRepository) GetAvailableByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*shippingentity.ShippingOption, error) {
	return nil, nil
}
func (r *stubListingShippingOptionRepository) DeleteByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) error {
	return nil
}
func (r *stubListingShippingOptionRepository) DeleteByShippingOption(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID) error {
	return nil
}
func (r *stubListingShippingOptionRepository) CreateBulk(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingOptionIDs []uuid.UUID) error {
	return nil
}
func (r *stubListingShippingOptionRepository) CountByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) (int64, error) {
	return 0, nil
}

// newStubShippingService creates a stub shipping service for orchestrator processing
func newStubShippingService() *shippingapp.ShippingService {
	return shippingapp.NewShippingService(
		&stubShippingOptionRepository{},
		&stubShippingCoverageRepository{},
		&stubCityOverrideRepository{},
		&stubListingShippingOptionRepository{},
	)
}

// OrderPaymentHandler handles payment finalization for orders.
//
// This is the NEW handler for order-based payments.
// The Order entity is the canonical transaction engine.
type OrderPaymentHandler struct {
	orderService *application.OrderService
	db           *db.DB
}

// NewOrderPaymentHandler creates a new order payment handler.
// Commission is now read from PlatformConfigService (V2.2).
func NewOrderPaymentHandler(db *db.DB) *OrderPaymentHandler {
	accountStatusChecker := &systemAccountStatusChecker{}
	roleChecker := &systemRoleChecker{}
	outboxRepository := outboxRepo.NewOutboxRepository(db)
	platformConfigRepo := platformconfigRepo.NewPlatformConfigRepository()
	configService := platformconfigApp.NewConfigService(platformConfigRepo)

	// Create required services for OrderService
	shippingService := newStubShippingService()
	coinsRepository := coinsRepo.NewCoinsRepository()
	coinsService := coinsapp.NewCoinsService(coinsRepository, db)

	return &OrderPaymentHandler{
		// shippingQuoteService=nil: Not available in payment integration context.
		// Order expiry/refund will not reactivate shipping quotes (acceptable limitation for this integration layer).
		// TODO: If handler is used in production, wire shippingQuoteService via SetShippingQuoteService().
		orderService: application.NewOrderService(accountStatusChecker, shippingService, outboxRepository, configService, coinsService, roleChecker, nil, nil, &stubListingShippingOptionRepository{}, nil, nil),
		db:           db,
	}
}

// ReferenceType returns "order" as this handler's reference type.
func (h *OrderPaymentHandler) ReferenceType() string {
	return repository.ReferenceTypeOrder
}

// HandlePaymentCompleted handles successful payment for orders.
// This transitions the order from pending to paid status.
func (h *OrderPaymentHandler) HandlePaymentCompleted(
	ctx context.Context,
	paymentID uuid.UUID,
) error {
	return h.db.WithTx(ctx, func(tx db.Tx) error {
		// Get payment to retrieve order_id from reference_id
		payment, err := h.getPaymentForUpdate(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("failed to get payment: %w", err)
		}

		// Validate this is an order payment
		if payment.ReferenceType != repository.ReferenceTypeOrder {
			return fmt.Errorf("invalid reference_type for order handler: %s", payment.ReferenceType)
		}

		if payment.ReferenceID == nil || *payment.ReferenceID == uuid.Nil {
			return fmt.Errorf("payment has no valid reference_id")
		}

		orderID := *payment.ReferenceID

		// Mark order as paid (idempotent via order status check)
		if err := h.orderService.MarkPaid(ctx, tx, orderID); err != nil {
			return fmt.Errorf("failed to mark order as paid: %w", err)
		}

		return nil
	})
}

// HandlePaymentExpired handles expired payment for orders.
// This expires the order if still in pending status.
func (h *OrderPaymentHandler) HandlePaymentExpired(
	ctx context.Context,
	paymentID uuid.UUID,
) error {
	return h.db.WithTx(ctx, func(tx db.Tx) error {
		// Get payment to retrieve order_id from reference_id
		payment, err := h.getPaymentForUpdate(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("failed to get payment: %w", err)
		}

		// Validate this is an order payment
		if payment.ReferenceType != repository.ReferenceTypeOrder {
			return fmt.Errorf("invalid reference_type for order handler: %s", payment.ReferenceType)
		}

		if payment.ReferenceID == nil || *payment.ReferenceID == uuid.Nil {
			return fmt.Errorf("payment has no valid reference_id")
		}

		orderID := *payment.ReferenceID

		// Expire the order (idempotent via order status check)
		if err := h.orderService.Expire(ctx, tx, orderID); err != nil {
			// If order is not in pending status, it's already been processed
			// This is not a critical error - the payment expiry is already handled
			return fmt.Errorf("failed to expire order: %w", err)
		}

		return nil
	})
}

// HandlePaymentRefunded handles refunded payment for orders.
// This transitions the order to refunded status and reverses escrow.
func (h *OrderPaymentHandler) HandlePaymentRefunded(
	ctx context.Context,
	paymentID uuid.UUID,
) error {
	return h.db.WithTx(ctx, func(tx db.Tx) error {
		// Get payment to retrieve order_id from reference_id
		payment, err := h.getPaymentForUpdate(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("failed to get payment: %w", err)
		}

		// Validate this is an order payment
		if payment.ReferenceType != repository.ReferenceTypeOrder {
			return fmt.Errorf("invalid reference_type for order handler: %s", payment.ReferenceType)
		}

		if payment.ReferenceID == nil || *payment.ReferenceID == uuid.Nil {
			return fmt.Errorf("payment has no valid reference_id")
		}

		orderID := *payment.ReferenceID

		// Refund the order (reverses escrow and updates order status)
		// This uses the ledger reversal transaction internally
		if err := h.orderService.RefundOrder(ctx, tx, orderID); err != nil {
			// If order is not in holding status, it may have already been processed
			// This could be a legitimate scenario (e.g., already shipped)
			return fmt.Errorf("failed to refund order: %w", err)
		}

		return nil
	})
}

// getPaymentForUpdate retrieves a payment by ID with row locking.
func (h *OrderPaymentHandler) getPaymentForUpdate(
	ctx context.Context,
	tx db.Tx,
	paymentID uuid.UUID,
) (*repository.Payment, error) {
	paymentRepo := repository.NewPaymentRepository()
	return paymentRepo.GetByIDForUpdate(ctx, tx, paymentID)
}

// MarkPaid marks an order as paid based on payment.
// This will be used in Phase 2.
func (h *OrderPaymentHandler) MarkPaid(
	ctx context.Context,
	orderID uuid.UUID,
) error {
	return h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.orderService.MarkPaid(ctx, tx, orderID)
	})
}

// CancelOrder cancels an order based on expired payment.
// This will be used in Phase 2.
// Uses uuid.Nil as callerID to bypass buyer ownership check (system operation).
func (h *OrderPaymentHandler) CancelOrder(
	ctx context.Context,
	orderID uuid.UUID,
) error {
	return h.db.WithTx(ctx, func(tx db.Tx) error {
		// Generate idempotency key for payment expiry cancellation
		idempotencyKey := fmt.Sprintf("payment_expiry_%s", orderID.String())
		return h.orderService.Cancel(ctx, tx, orderID, idempotencyKey, uuid.Nil)
	})
}

// GetOrderRepository returns the underlying order repository.
// Useful for testing and direct access when needed.
func (h *OrderPaymentHandler) GetOrderRepository() orderrepository.OrderRepository {
	return orderRepo.NewOrderRepository()
}


