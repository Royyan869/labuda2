package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/delivery/http/dto"
	"github.com/labuda/backend/internal/commerce/order/entity"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/pkg/userdisplay"
	"github.com/labuda/backend/internal/projection"
	"github.com/labuda/backend/pkg/db"
)

// projectionLister abstracts the projection read methods used by OrderQueryService.
// Package-private interface so unit tests can inject a stub without depending on
// the concrete *projection.Repository type.
//
// The Count* methods are required for Option-B count-comparison safety fallback:
// if projCount < writeModelCount the service falls back to the write model so
// that a lagging or partial projection never silently hides orders.
type projectionLister interface {
	ListOrderSummariesByBuyer(ctx context.Context, tx db.Tx, buyerID uuid.UUID, status *string, limit int, cursor int64) ([]*projection.OrderSummary, error)
	ListOrderSummariesBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, status *string, limit int, cursor int64) ([]*projection.OrderSummary, error)
	ListOrderSummariesForAdmin(ctx context.Context, tx db.Tx, filters projection.OrderListFilters) ([]*projection.OrderSummary, int64, error)
	// Count methods for Option-B completeness validation (cursor-independent totals).
	CountOrderSummariesByBuyer(ctx context.Context, tx db.Tx, buyerID uuid.UUID, status *string) (int64, error)
	CountOrderSummariesBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, status *string) (int64, error)
}

// OrderQueryService handles read-side queries for orders.
// This is a CQRS query service that reads from the projection tables.
//
// HARDENING: Projection reads may have stale EscrowStatus if:
// - Projection worker hasn't processed latest wallet event
// - Order.EscrowStatus update failed silently
// - Event propagation delay
//
// MITIGATION: Projection should include wallet sync timestamp for staleness detection.
type OrderQueryService struct {
	projection        projectionLister
	projectionEnabled bool
}

// NewOrderQueryService creates a new OrderQueryService.
// When projectionEnabled is false, the service skips projection table reads
// entirely and queries the orders write model directly (avoiding 2 wasted
// queries against an empty order_summaries table).
func NewOrderQueryService(projection projectionLister, projectionEnabled bool) *OrderQueryService {
	return &OrderQueryService{
		projection:        projection,
		projectionEnabled: projectionEnabled,
	}
}

// OrderListItem represents a single order in a list response.
//
// ARCHITECTURAL NOTES:
// - EscrowStatus is CACHED from Wallet state (projection may be stale)
// - EscrowAmount and RefundedAmount removed - financial truth is in Ledger service
// - Contains only snapshot fields for display (Subtotal, ShippingTotal, CommissionAmount)
// - For financial amounts, query the Ledger service
//
// HARDENING: Business logic guards should use Wallet-derived EscrowStatus, not projection value.
type OrderListItem struct {
	ID           uuid.UUID `json:"id"`
	BuyerID      uuid.UUID `json:"buyer_id"`
	SellerID     uuid.UUID `json:"seller_id"`
	BuyerName    string    `json:"buyer_name"`
	BuyerAvatar  *string   `json:"buyer_avatar,omitempty"`
	SellerAvatar *string   `json:"seller_avatar,omitempty"`

	// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE (additive).
	// Strict source separation (NEVER COALESCE):
	//   - buyer_username    ← user_profiles.username
	//   - seller_username   ← user_profiles.username   (NEVER store_name)
	//   - seller_farm_name  ← seller_profiles.store_name (NEVER username)
	//   - seller_avatar_url ← user_profiles.avatar_url
	BuyerUsername      string     `json:"buyer_username"`
	SellerUsername     string     `json:"seller_username"`
	SellerFarmName     string     `json:"seller_farm_name"`
	SellerAvatarURL    string     `json:"seller_avatar_url"`
	OrderType          string     `json:"order_type"`
	Status             string     `json:"status"`
	EscrowStatus       string     `json:"escrow_status"`     // CACHED from Wallet - may be stale
	HasActiveRefund    bool       `json:"has_active_refund"` // true if order has active (non-terminal) refund
	DisputeStatus      *string    `json:"dispute_status,omitempty"`
	Subtotal           int64      `json:"subtotal"`
	ShippingTotal      int64      `json:"shipping_total"`
	CommissionAmount   int64      `json:"commission_amount"`
	ServiceFeeAmount   int64      `json:"service_fee_amount"`
	TotalPayableAmount int64      `json:"total_payable_amount"`
	ShippingOptionName string     `json:"shipping_option_name"`
	AutoReleaseAt      *int64     `json:"auto_release_at,omitempty"`
	PaymentID          *uuid.UUID `json:"payment_id,omitempty"` // V1.1 Payment Contract Refactor
	// PaymentStatus is the status of the active/latest payment for this order.
	// Priority: settlement > capture > pending > others. Nil when no payment exists.
	PaymentStatus *string `json:"payment_status,omitempty"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`

	// Computed fields for the caller
	Role              string  `json:"role"` // "buyer" or "seller"
	DisputeReason     *string `json:"dispute_reason,omitempty"`
	DisputeOpenedAt   *int64  `json:"dispute_opened_at,omitempty"`
	DisputeResolvedAt *int64  `json:"dispute_resolved_at,omitempty"`

	// Decision Contract - Backend is SINGLE SOURCE OF TRUTH for business decisions
	Decision *dto.Decision `json:"decision,omitempty"`
}

// OrderListResponse represents the paginated list response.
type OrderListResponse struct {
	Orders     []*OrderListItem `json:"orders"`
	NextCursor *int64           `json:"next_cursor,omitempty"`
	Limit      int              `json:"limit"`
}

// ListMyOrdersInput contains the parameters for ListMyOrders.
type ListMyOrdersInput struct {
	CallerID  uuid.UUID
	RoleParam string  // "buyer" or "seller"
	Status    *string // Optional status filter
	Limit     int     // Default 20, max 50
	Cursor    int64   // Unix timestamp for cursor pagination
}

// ListMyOrders retrieves orders for the authenticated user based on their role.
//
// HARDENING: This reads from CQRS projection which may have stale EscrowStatus.
// Critical business logic should validate against live Wallet state.
//
// Rules:
// - roleParam must be "buyer" or "seller"
// - If roleParam="buyer": returns orders where buyer_id=callerID
// - If roleParam="seller": returns orders where seller_id=callerID
// - Returns cursor-based pagination with next_cursor for fetching more results
// - For each order, computes the user's role and allowed actions
func (s *OrderQueryService) ListMyOrders(
	ctx context.Context,
	tx db.Tx,
	input ListMyOrdersInput,
) (*OrderListResponse, error) {
	// Validate role parameter
	if input.RoleParam != "buyer" && input.RoleParam != "seller" {
		return nil, fmt.Errorf("invalid role: %s, must be 'buyer' or 'seller'", input.RoleParam)
	}

	// Set default limit if needed
	if input.Limit <= 0 {
		input.Limit = 20
	}

	var summaries []*projection.OrderSummary
	var err error

	if !s.projectionEnabled {
		// Fast path: projection worker is disabled so order_summaries is empty.
		// Skip the 2 wasted projection queries and go straight to write model.
		if input.RoleParam == "buyer" {
			summaries, err = s.listByBuyerFromWriteModel(ctx, tx, input.CallerID, input.Status, input.Limit, input.Cursor)
		} else {
			summaries, err = s.listBySellerFromWriteModel(ctx, tx, input.CallerID, input.Status, input.Limit, input.Cursor)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list orders (write model direct): %w", err)
		}
	} else {
		// Query based on role
		if input.RoleParam == "buyer" {
			summaries, err = s.projection.ListOrderSummariesByBuyer(
				ctx, tx, input.CallerID, input.Status, input.Limit, input.Cursor,
			)
		} else {
			summaries, err = s.projection.ListOrderSummariesBySeller(
				ctx, tx, input.CallerID, input.Status, input.Limit, input.Cursor,
			)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to list orders: %w", err)
		}

		// Option B — count-comparison safety fallback.
		//
		// Projection is a read optimisation, not the authority. A lagging or
		// partially-rebuilt projection must never silently hide write-model rows.
		// Compare the cursor-independent total from the projection against the
		// write-model total; if the projection is behind, use the write model.
		//
		// Gate: projCount < writeModelCount → fallback.
		// Special case: if both are 0 the user genuinely has no orders; returning
		// an empty list from the projection is correct (no fallback needed).
		var projCount int64
		var writeModelCount int64

		if input.RoleParam == "buyer" {
			projCount, err = s.projection.CountOrderSummariesByBuyer(ctx, tx, input.CallerID, input.Status)
			if err != nil {
				return nil, fmt.Errorf("count projection orders by buyer: %w", err)
			}
			writeModelCount, err = s.countByBuyerFromWriteModel(ctx, tx, input.CallerID, input.Status)
			if err != nil {
				return nil, fmt.Errorf("count write-model orders by buyer: %w", err)
			}
		} else {
			projCount, err = s.projection.CountOrderSummariesBySeller(ctx, tx, input.CallerID, input.Status)
			if err != nil {
				return nil, fmt.Errorf("count projection orders by seller: %w", err)
			}
			writeModelCount, err = s.countBySellerFromWriteModel(ctx, tx, input.CallerID, input.Status)
			if err != nil {
				return nil, fmt.Errorf("count write-model orders by seller: %w", err)
			}
		}

		if projCount < writeModelCount {
			if input.RoleParam == "buyer" {
				summaries, err = s.listByBuyerFromWriteModel(ctx, tx, input.CallerID, input.Status, input.Limit, input.Cursor)
			} else {
				summaries, err = s.listBySellerFromWriteModel(ctx, tx, input.CallerID, input.Status, input.Limit, input.Cursor)
			}
			if err != nil {
				return nil, fmt.Errorf("failed to list orders (write model fallback): %w", err)
			}
		}
	}

	// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE:
	// Batch-hydrate buyer & seller display info for the page in two
	// queries (one for buyer side via user_profiles, one for seller
	// side via user_profiles + seller_profiles). No N+1.
	buyerIDs := make([]uuid.UUID, 0, len(summaries))
	sellerIDs := make([]uuid.UUID, 0, len(summaries))
	for _, sm := range summaries {
		buyerIDs = append(buyerIDs, sm.BuyerID)
		sellerIDs = append(sellerIDs, sm.SellerID)
	}
	buyerInfoByID, _ := userdisplay.FetchMany(ctx, tx, buyerIDs)
	sellerInfoByID, _ := sellerdisplay.FetchMany(ctx, tx, sellerIDs)

	// Convert to response items
	items := make([]*OrderListItem, len(summaries))
	var nextCursor *int64

	for i, summary := range summaries {
		items[i] = s.convertToListItem(summary, input.CallerID, input.RoleParam)

		// Hydrate Phase 5 Stage 1 additive identity fields with strict
		// source separation.
		bi := buyerInfoByID[summary.BuyerID]
		si := sellerInfoByID[summary.SellerID]
		items[i].BuyerUsername = bi.Username
		items[i].SellerUsername = si.Username
		items[i].SellerFarmName = si.FarmName
		items[i].SellerAvatarURL = si.AvatarURL

		// The next cursor is the created_at timestamp of the last item
		if i == len(summaries)-1 {
			ts := summary.CreatedAt.Unix()
			nextCursor = &ts
		}
	}

	// Batch-hydrate payment statuses in a single query (no N+1).
	// Priority: settlement(1) > capture(2) > pending(3) > others(4).
	if len(summaries) > 0 {
		orderIDs := make([]uuid.UUID, len(summaries))
		for i, sm := range summaries {
			orderIDs[i] = sm.ID
		}
		paymentStatuses, psErr := s.batchFetchPaymentStatuses(ctx, tx, orderIDs)
		if psErr == nil {
			for _, item := range items {
				if pm, ok := paymentStatuses[item.ID]; ok {
					ps := pm.Status
					item.PaymentStatus = &ps
					item.PaymentID = &pm.ID
				}
			}
		}
		// psErr non-nil: log-worthy but non-fatal; items render without payment_status
	}

	return &OrderListResponse{
		Orders:     items,
		NextCursor: nextCursor,
		Limit:      input.Limit,
	}, nil
}

// convertToListItem converts a OrderSummary to a OrderListItem with computed fields.
func (s *OrderQueryService) convertToListItem(
	summary *projection.OrderSummary,
	callerID uuid.UUID,
	roleParam string,
) *OrderListItem {
	// Build display hints
	display := s.buildDisplayHints(summary.Status, summary.AutoReleaseAt, roleParam)

	// Create decision contract
	// TODO: Build proper V3 decision with PrimaryAction and SecondaryActions
	// For now, create a simple decision state
	decision := &dto.Decision{
		State:   summary.Status,
		Display: display,
	}

	item := &OrderListItem{
		ID:                 summary.ID,
		BuyerID:            summary.BuyerID,
		SellerID:           summary.SellerID,
		BuyerName:          "",                 // TODO: Add user name to projection
		BuyerAvatar:        nil,                // TODO: Add user avatar to projection
		SellerAvatar:       nil,                // TODO: Add user avatar to projection
		OrderType:          summary.SourceType, // Use source type as order type
		Status:             summary.Status,
		EscrowStatus:       summary.EscrowStatus,
		HasActiveRefund:    false, // TODO: Query from refund service
		DisputeStatus:      summary.DisputeStatus,
		Subtotal:           summary.Subtotal,
		ShippingTotal:      summary.ShippingTotal,
		CommissionAmount:   summary.CommissionAmount,
		ServiceFeeAmount:   summary.ServiceFeeAmount,
		TotalPayableAmount: summary.TotalPayableAmount,
		ShippingOptionName: summary.ShippingOptionName,
		CreatedAt:          summary.CreatedAt.Unix(),
		UpdatedAt:          summary.UpdatedAt.Unix(),
		DisputeReason:      summary.DisputeReason,
		Role:               roleParam,
		Decision:           decision,
	}

	// Convert optional timestamp pointers
	if summary.AutoReleaseAt != nil {
		ts := summary.AutoReleaseAt.Unix()
		item.AutoReleaseAt = &ts
	}
	if summary.DisputeOpenedAt != nil {
		ts := summary.DisputeOpenedAt.Unix()
		item.DisputeOpenedAt = &ts
	}
	if summary.DisputeResolvedAt != nil {
		ts := summary.DisputeResolvedAt.Unix()
		item.DisputeResolvedAt = &ts
	}

	return item
}

// buildDisplayHints creates UI display hints based on order status, timing, and user role.
func (s *OrderQueryService) buildDisplayHints(status string, autoReleaseAt *time.Time, role string) *dto.DisplayHints {
	hints := &dto.DisplayHints{}

	switch status {
	case string(entity.StatusPending):
		// Pending payment
		badge := "Menunggu Pembayaran"
		variant := "warning"
		hints.WithBadge(badge, variant)
		hints.WithInfo("Selesaikan pembayaran sebelum waktu habis")
		if role == "seller" {
			hints.WithNextAction(dto.ActionNone, "action.wait_payment", false)
		} else {
			hints.WithNextAction(dto.ActionPay, "action.pay_now", true)
		}

	case string(entity.StatusPaid):
		// Paid, waiting for seller to ship
		badge := "Menunggu Pengiriman"
		variant := "info"
		hints.WithBadge(badge, variant)
		if role == "seller" {
			hints.WithNextAction(dto.ActionMarkShipped, "action.mark_shipped", true)
		} else {
			hints.WithNextAction(dto.ActionNone, "action.wait_shipping", false)
		}

	case string(entity.StatusShipped):
		// B4A: SHIPPED is the buyer decision screen.
		// "Terima Barang" = Complete() (final acceptance, financial action).
		badge := "Dalam Pengiriman"
		variant := "success"
		hints.WithBadge(badge, variant)
		if role == "buyer" {
			hints.WithNextAction(dto.ActionComplete, "action.confirm_receipt", true)
		} else {
			hints.WithNextAction(dto.ActionNone, "action.wait_buyer_confirm", false)
		}

		// Add time remaining if auto_release_at is set
		if autoReleaseAt != nil {
			remaining := int(time.Until(*autoReleaseAt).Seconds())
			if remaining > 0 {
				hints.WithTimeRemaining(remaining)
				hints.WithInfo("Otomatis selesai dalam 5 hari")
			}
		}

	case string(entity.StatusCompleted):
		// Order completed
		badge := "Selesai"
		variant := "success"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(dto.ActionNone, "action.order_completed", false)

	case string(entity.StatusCancelled):
		// Order cancelled
		badge := "Dibatalkan"
		variant := "error"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(dto.ActionNone, "action.order_cancelled", false)

	case string(entity.StatusExpired):
		// Payment expired
		badge := "Kadaluarsa"
		variant := "error"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(dto.ActionNone, "action.order_expired", false)

	case string(entity.StatusPartiallyRefunded):
		// Partially refunded
		badge := "Refund Sebagian"
		variant := "warning"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(dto.ActionNone, "action.partially_refunded", false)

	case string(entity.StatusRefunded):
		// Fully refunded
		badge := "Refund"
		variant := "error"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(dto.ActionNone, "action.refunded", false)

	case string(entity.StatusDisputeOpen):
		// Dispute open
		badge := "Dispute Terbuka"
		variant := "error"
		hints.WithBadge(badge, variant)
		hints.WithNextAction(dto.ActionProvideEvidence, "action.provide_evidence", true)
	}

	return hints
}

// ============================================================================
// ORDER STATS
// ============================================================================

// OrderStatsResponse represents order statistics grouped by status.
type OrderStatsResponse struct {
	TotalOrders int64 `json:"total_orders"`
	Pending     int64 `json:"pending"`
	Paid        int64 `json:"paid"`
	Shipped     int64 `json:"shipped"`
	Completed   int64 `json:"completed"`
	Cancelled   int64 `json:"cancelled"`
}

// GetOrderStats retrieves order statistics for a given user and role.
// TODO: Implement stats aggregation from order_summaries table
func (s *OrderQueryService) GetOrderStats(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	isSeller bool,
) (*OrderStatsResponse, error) {
	// TODO: Query from order_summaries table aggregated by status
	// For now, return empty stats
	return &OrderStatsResponse{
		TotalOrders: 0,
		Pending:     0,
		Paid:        0,
		Shipped:     0,
		Completed:   0,
		Cancelled:   0,
	}, nil
}

// ============================================================================
// ADMIN QUERY METHODS
// ============================================================================

// AdminOrderListFilters contains filters for admin order listing.
type AdminOrderListFilters struct {
	Status     *string
	SourceType *string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
	// Search matches order_number (exact) or order UUID prefix (case-insensitive).
	Search *string
}

// AdminOrderListResponse contains the response for admin order listing.
type AdminOrderListResponse struct {
	Orders []*AdminOrderSummary `json:"orders"`
	Total  int64                `json:"total"`
}

// AdminOrderSummary represents an order summary for admin view.
//
// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE:
//   - LEGACY buyer_name is NOT emitted on this surface.
//   - The pre-existing buyer_username / seller_username fields are
//     KEPT for compatibility but their semantics on this admin surface
//     historically resolved to COALESCE(store_name, username) — a
//     display-name shape, not a username. To uphold the strict rule
//     "seller_username NEVER contains farm name", we now ADDITIVELY
//     expose a properly-separated set of fields:
//     seller_farm_name  ← seller_profiles.store_name
//     seller_avatar_url ← user_profiles.avatar_url
//     The username sources for buyer_username / seller_username are
//     also corrected to user_profiles.username only (no COALESCE).
type AdminOrderSummary struct {
	ID                 uuid.UUID  `json:"id"`
	OrderNumber        string     `json:"order_number"`
	BuyerID            uuid.UUID  `json:"buyer_id"`
	SellerID           uuid.UUID  `json:"seller_id"`
	SourceType         string     `json:"source_type"`
	SourceID           uuid.UUID  `json:"source_id"`
	Status             string     `json:"status"`
	EscrowStatus       string     `json:"escrow_status"`
	HasDispute         bool       `json:"has_dispute"`
	DisputeStatus      *string    `json:"dispute_status,omitempty"`
	Subtotal           int64      `json:"subtotal"`
	ShippingTotal      int64      `json:"shipping_total"`
	CommissionAmount   int64      `json:"commission_amount"`
	ServiceFeeAmount   int64      `json:"service_fee_amount"`
	TotalPayableAmount int64      `json:"total_payable_amount"`
	RefundedAmount     int64      `json:"refunded_amount"`
	ShippingOption     *string    `json:"shipping_option,omitempty"`
	AutoReleaseAt      *time.Time `json:"auto_release_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// Computed fields for display.
	// buyer_username  = user_profiles.username
	// seller_username = user_profiles.username   (strict; NEVER store_name)
	BuyerUsername  *string `json:"buyer_username,omitempty"`
	BuyerAvatar    *string `json:"buyer_avatar,omitempty"`
	SellerUsername *string `json:"seller_username,omitempty"`
	SellerAvatar   *string `json:"seller_avatar,omitempty"`

	// Phase 5 Stage 1 additive fields with strict source separation.
	SellerFarmName  *string `json:"seller_farm_name,omitempty"`  // seller_profiles.store_name
	SellerAvatarURL *string `json:"seller_avatar_url,omitempty"` // user_profiles.avatar_url
}

// ListAllOrdersForAdmin lists all orders with filters for admin.
// This is a read-only endpoint that returns ALL orders (not scoped to a user).
func (s *OrderQueryService) ListAllOrdersForAdmin(
	ctx context.Context,
	tx db.Tx,
	filters AdminOrderListFilters,
) (*AdminOrderListResponse, error) {
	// Call projection repository with converted filters
	projFilters := projection.OrderListFilters{
		Status:     filters.Status,
		SourceType: filters.SourceType,
		DateFrom:   filters.DateFrom,
		DateTo:     filters.DateTo,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
	}

	var summaries []*projection.OrderSummary
	var total int64
	var err error

	if !s.projectionEnabled || filters.Search != nil {
		// Fast path: projection worker is disabled OR a search term is provided
		// (projection does not support text search; write model handles it directly).
		summaries, total, err = s.listAllFromWriteModel(ctx, tx, projFilters, filters.Search)
		if err != nil {
			return nil, fmt.Errorf("list orders for admin (write model direct): %w", err)
		}
	} else {
		summaries, total, err = s.projection.ListOrderSummariesForAdmin(ctx, tx, projFilters)
		if err != nil {
			return nil, fmt.Errorf("list orders for admin failed: %w", err)
		}

		// Option B — count-comparison safety fallback (admin path).
		// ListOrderSummariesForAdmin already returns the projection total count.
		// Compare against the write-model count; fall back if the projection is behind.
		writeModelTotal, wmErr := s.countOrdersFromWriteModel(ctx, tx, projFilters)
		if wmErr != nil {
			return nil, fmt.Errorf("count orders for admin (write model) failed: %w", wmErr)
		}

		if total < writeModelTotal {
			summaries, total, err = s.listAllFromWriteModel(ctx, tx, projFilters, nil)
			if err != nil {
				return nil, fmt.Errorf("list orders for admin (write model fallback) failed: %w", err)
			}
		}
	}

	// Convert to admin response format
	orders := make([]*AdminOrderSummary, len(summaries))
	for i, s := range summaries {
		// Handle nullable SourceID
		var sourceID uuid.UUID
		if s.SourceID != nil {
			sourceID = *s.SourceID
		}

		// Handle nullable ShippingOptionName
		var shippingOption *string
		if s.ShippingOptionName != "" {
			shippingOption = &s.ShippingOptionName
		}

		orderNum := ""
		if s.OrderNumber != nil {
			orderNum = *s.OrderNumber
		}

		orders[i] = &AdminOrderSummary{
			ID:                 s.ID,
			OrderNumber:        orderNum,
			BuyerID:            s.BuyerID,
			SellerID:           s.SellerID,
			SourceType:         s.SourceType,
			SourceID:           sourceID,
			Status:             s.Status,
			EscrowStatus:       s.EscrowStatus,
			HasDispute:         s.HasDispute,
			DisputeStatus:      s.DisputeStatus,
			Subtotal:           s.Subtotal,
			ShippingTotal:      s.ShippingTotal,
			CommissionAmount:   s.CommissionAmount,
			ServiceFeeAmount:   s.ServiceFeeAmount,
			TotalPayableAmount: s.TotalPayableAmount,
			ShippingOption:     shippingOption,
			AutoReleaseAt:      s.AutoReleaseAt,
			CreatedAt:          s.CreatedAt,
			UpdatedAt:          s.UpdatedAt,
		}

		// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE:
		// Fetch buyer & seller display fields with STRICT source
		// separation. The previous query used
		//   COALESCE(sp.store_name, up.username, '')
		// for the seller_username column, which violated the rule
		// "seller_username NEVER contains farm name". Username sources
		// are now user_profiles.username only; store_name is exposed
		// as the dedicated seller_farm_name field.
		var buyerUsername, buyerAvatar string
		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(username, ''), COALESCE(avatar_url, '')
			FROM user_profiles WHERE user_id = $1
		`, s.BuyerID).Scan(&buyerUsername, &buyerAvatar)

		var sellerUsername, sellerFarmName, sellerAvatar string
		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(up.username, '')   AS seller_username,
			       COALESCE(sp.store_name, '') AS seller_farm_name,
			       COALESCE(up.avatar_url, '') AS seller_avatar_url
			FROM users u
			LEFT JOIN user_profiles   up ON up.user_id = u.id
			LEFT JOIN seller_profiles sp ON sp.user_id = u.id
			WHERE u.id = $1
		`, s.SellerID).Scan(&sellerUsername, &sellerFarmName, &sellerAvatar)

		if buyerUsername != "" {
			orders[i].BuyerUsername = &buyerUsername
		}
		if buyerAvatar != "" {
			orders[i].BuyerAvatar = &buyerAvatar
		}
		if sellerUsername != "" {
			orders[i].SellerUsername = &sellerUsername
		}
		if sellerAvatar != "" {
			// Legacy seller_avatar mirrors seller_avatar_url on this surface.
			avatarCopy := sellerAvatar
			orders[i].SellerAvatar = &avatarCopy
			orders[i].SellerAvatarURL = &sellerAvatar
		}
		if sellerFarmName != "" {
			orders[i].SellerFarmName = &sellerFarmName
		}
	}

	return &AdminOrderListResponse{
		Orders: orders,
		Total:  total,
	}, nil
}

// ============================================================================
// WRITE MODEL COUNT HELPERS (Option B safety comparisons)
// Used exclusively to detect projection lag: if projCount < writeModelCount
// the write model is used instead of the projection for the current request.
// Count queries are cursor-independent (no pagination) so they reflect the
// true total for the user/filter combination.
// ============================================================================

// countByBuyerFromWriteModel returns the total number of orders in the write
// model for the given buyer, optionally filtered by status.
func (s *OrderQueryService) countByBuyerFromWriteModel(
	ctx context.Context,
	tx db.Tx,
	buyerID uuid.UUID,
	status *string,
) (int64, error) {
	query := "SELECT COUNT(*) FROM orders WHERE buyer_id = $1"
	args := []interface{}{buyerID}
	if status != nil {
		query += " AND status::text = $2"
		args = append(args, *status)
	}
	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count orders by buyer (write model) failed: %w", err)
	}
	return count, nil
}

// countBySellerFromWriteModel returns the total number of orders in the write
// model for the given seller, optionally filtered by status.
func (s *OrderQueryService) countBySellerFromWriteModel(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	status *string,
) (int64, error) {
	query := "SELECT COUNT(*) FROM orders WHERE seller_id = $1"
	args := []interface{}{sellerID}
	if status != nil {
		query += " AND status::text = $2"
		args = append(args, *status)
	}
	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count orders by seller (write model) failed: %w", err)
	}
	return count, nil
}

// countOrdersFromWriteModel returns the total number of orders in the write
// model matching the given admin filters. Used for admin Option-B comparison.
func (s *OrderQueryService) countOrdersFromWriteModel(
	ctx context.Context,
	tx db.Tx,
	filters projection.OrderListFilters,
) (int64, error) {
	where := " WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if filters.Status != nil {
		where += fmt.Sprintf(" AND status::text = $%d", argIdx)
		args = append(args, *filters.Status)
		argIdx++
	}
	if filters.SourceType != nil {
		where += fmt.Sprintf(" AND source_type::text = $%d", argIdx)
		args = append(args, *filters.SourceType)
		argIdx++
	}
	if filters.DateFrom != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filters.DateFrom)
		argIdx++
	}
	if filters.DateTo != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filters.DateTo)
		_ = argIdx // suppress unused increment warning
	}

	var count int64
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM orders"+where, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count orders (write model admin) failed: %w", err)
	}
	return count, nil
}

// ============================================================================
// WRITE MODEL FALLBACK
// Used when projection worker is off (PROJECTION_WORKER=false) or the
// order_summaries table has not yet been populated for newly-created orders.
// All three methods return data shaped as []*projection.OrderSummary so they
// flow through the same convertToListItem / admin-conversion path.
// Dispute columns (dispute_status, reason, opened_at, resolved_at) do not
// exist in the orders write table and will be nil in returned structs.
// ============================================================================

// listByBuyerFromWriteModel lists orders for a buyer directly from the orders table.
func (s *OrderQueryService) listByBuyerFromWriteModel(
	ctx context.Context,
	tx db.Tx,
	buyerID uuid.UUID,
	status *string,
	limit int,
	cursor int64,
) ([]*projection.OrderSummary, error) {
	query := `
		SELECT id, buyer_id, seller_id, source_type::text, source_id,
		       status::text, escrow_status::text, has_dispute,
		       subtotal, shipping_total, commission_amount, service_fee_amount, total_payable_amount,
		       COALESCE(shipping_option_name, ''), COALESCE(shipping_transport_type, ''),
		       auto_release_at, created_at, updated_at,
		       order_number
		FROM orders
		WHERE buyer_id = $1`
	args := []interface{}{buyerID}
	argIdx := 2

	if cursor > 0 {
		query += fmt.Sprintf(" AND created_at < $%d", argIdx)
		args = append(args, time.Unix(cursor, 0).UTC())
		argIdx++
	}
	if status != nil {
		query += fmt.Sprintf(" AND status::text = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders by buyer (write model) failed: %w", err)
	}
	defer rows.Close()

	return scanWriteModelOrders(rows)
}

// listBySellerFromWriteModel lists orders for a seller directly from the orders table.
func (s *OrderQueryService) listBySellerFromWriteModel(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	status *string,
	limit int,
	cursor int64,
) ([]*projection.OrderSummary, error) {
	query := `
		SELECT id, buyer_id, seller_id, source_type::text, source_id,
		       status::text, escrow_status::text, has_dispute,
		       subtotal, shipping_total, commission_amount, service_fee_amount, total_payable_amount,
		       COALESCE(shipping_option_name, ''), COALESCE(shipping_transport_type, ''),
		       auto_release_at, created_at, updated_at,
		       order_number
		FROM orders
		WHERE seller_id = $1`
	args := []interface{}{sellerID}
	argIdx := 2

	if cursor > 0 {
		query += fmt.Sprintf(" AND created_at < $%d", argIdx)
		args = append(args, time.Unix(cursor, 0).UTC())
		argIdx++
	}
	if status != nil {
		query += fmt.Sprintf(" AND status::text = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders by seller (write model) failed: %w", err)
	}
	defer rows.Close()

	return scanWriteModelOrders(rows)
}

// paymentHydration holds the payment ID and status for a single order.
type paymentHydration struct {
	ID     uuid.UUID
	Status string
}

// batchFetchPaymentStatuses returns the active/latest payment ID and status for each
// order ID in a single query (no N+1).
// Priority: settlement(1) > capture(2) > pending(3) > others(4).
// Orders without a payment record are absent from the returned map.
func (s *OrderQueryService) batchFetchPaymentStatuses(
	ctx context.Context,
	tx db.Tx,
	orderIDs []uuid.UUID,
) (map[uuid.UUID]paymentHydration, error) {
	if len(orderIDs) == 0 {
		return nil, nil
	}
	result := make(map[uuid.UUID]paymentHydration, len(orderIDs))
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT ON (reference_id) reference_id, id, status::text
		FROM payments
		WHERE reference_type = 'order'
		  AND reference_id = ANY($1::uuid[])
		ORDER BY reference_id,
		  CASE status::text
		    WHEN 'settlement' THEN 1
		    WHEN 'capture'    THEN 2
		    WHEN 'pending'    THEN 3
		    ELSE 4
		  END ASC
	`, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("batch fetch payment statuses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var orderID uuid.UUID
		var ph paymentHydration
		if err := rows.Scan(&orderID, &ph.ID, &ph.Status); err != nil {
			return nil, fmt.Errorf("scan payment status: %w", err)
		}
		result[orderID] = ph
	}
	return result, rows.Err()
}

// listAllFromWriteModel lists all orders (admin) directly from the orders table.
// search is optional: when provided, filters by order_number (exact) OR order UUID
// prefix (case-insensitive). Pass nil to skip search filtering.
func (s *OrderQueryService) listAllFromWriteModel(
	ctx context.Context,
	tx db.Tx,
	filters projection.OrderListFilters,
	search *string,
) ([]*projection.OrderSummary, int64, error) {
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 || filters.PageSize > 100 {
		filters.PageSize = 20
	}

	where := " WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if filters.Status != nil {
		where += fmt.Sprintf(" AND status::text = $%d", argIdx)
		args = append(args, *filters.Status)
		argIdx++
	}
	if filters.SourceType != nil {
		where += fmt.Sprintf(" AND source_type::text = $%d", argIdx)
		args = append(args, *filters.SourceType)
		argIdx++
	}
	if filters.DateFrom != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filters.DateFrom)
		argIdx++
	}
	if filters.DateTo != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filters.DateTo)
		argIdx++
	}
	if search != nil && *search != "" {
		// Match exact order_number OR UUID prefix (id::text ILIKE 'term%').
		where += fmt.Sprintf(" AND (order_number = $%d OR id::text ILIKE $%d)", argIdx, argIdx+1)
		args = append(args, *search, *search+"%")
		argIdx += 2
	}

	var total int64
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM orders"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders (write model) failed: %w", err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	offset := (filters.Page - 1) * filters.PageSize
	dataArgs := append(args, filters.PageSize, offset)
	dataQuery := `
		SELECT id, buyer_id, seller_id, source_type::text, source_id,
		       status::text, escrow_status::text, has_dispute,
		       subtotal, shipping_total, commission_amount, service_fee_amount, total_payable_amount,
		       COALESCE(shipping_option_name, ''), COALESCE(shipping_transport_type, ''),
		       auto_release_at, created_at, updated_at,
		       order_number
		FROM orders` + where +
		fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)

	rows, err := tx.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders (write model) failed: %w", err)
	}
	defer rows.Close()

	summaries, err := scanWriteModelOrders(rows)
	if err != nil {
		return nil, 0, err
	}
	return summaries, total, nil
}

// scanWriteModelOrders scans rows from the orders table into []*projection.OrderSummary.
// The SELECT list must be exactly (19 columns):
//
//	id, buyer_id, seller_id, source_type::text, source_id,
//	status::text, escrow_status::text, has_dispute,
//	subtotal, shipping_total, commission_amount, service_fee_amount, total_payable_amount,
//	COALESCE(shipping_option_name,''), COALESCE(shipping_transport_type,''),
//	auto_release_at, created_at, updated_at,
//	order_number
//
// Dispute columns absent from the orders write table remain nil in results.
func scanWriteModelOrders(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]*projection.OrderSummary, error) {
	var summaries []*projection.OrderSummary
	for rows.Next() {
		var s projection.OrderSummary
		var sourceID uuid.UUID
		if err := rows.Scan(
			&s.ID, &s.BuyerID, &s.SellerID, &s.SourceType, &sourceID,
			&s.Status, &s.EscrowStatus, &s.HasDispute,
			&s.Subtotal, &s.ShippingTotal, &s.CommissionAmount, &s.ServiceFeeAmount, &s.TotalPayableAmount,
			&s.ShippingOptionName, &s.ShippingTransportType,
			&s.AutoReleaseAt, &s.CreatedAt, &s.UpdatedAt,
			&s.OrderNumber,
		); err != nil {
			return nil, fmt.Errorf("scan order (write model) failed: %w", err)
		}
		s.SourceID = &sourceID
		// DisputeStatus, DisputeReason, DisputeOpenedAt, DisputeResolvedAt remain nil
		summaries = append(summaries, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders (write model) failed: %w", err)
	}
	return summaries, nil
}
