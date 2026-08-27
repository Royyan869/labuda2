// Package http provides HTTP handlers for admin order operations.
package http

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/order/application"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// AdminOrderHandler handles HTTP requests for admin order operations.
type AdminOrderHandler struct {
	queryService *application.OrderQueryService
	db           *db.DB
	log          *zap.Logger
}

// NewAdminOrderHandler creates a new AdminOrderHandler.
func NewAdminOrderHandler(
	queryService *application.OrderQueryService,
	db *db.DB,
	log *zap.Logger,
) *AdminOrderHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &AdminOrderHandler{
		queryService: queryService,
		db:           db,
		log:          log,
	}
}

// ============================================================================
// ADMIN ORDER LIST ENDPOINT
// ============================================================================

// ListOrders handles GET /api/v1/admin/orders
//
// Returns ALL orders with optional filtering and pagination.
// This is a read-only admin endpoint that returns ALL orders (not scoped to a user).
//
// Query parameters:
//   - status: Filter by order status (pending, paid, shipped, delivered, completed, cancelled, expired, refunded)
//   - source: Filter by source type (for_sale, auction, negotiation)
//   - date_from: Filter by creation date (RFC3339 format)
//   - date_to: Filter by creation date (RFC3339 format)
//   - page: Page number (default: 1)
//   - page_size: Items per page (default: 20, max: 100)
//
// Response includes:
//   - order id
//   - buyer info (id, username, avatar)
//   - seller info (id, username, avatar)
//   - status
//   - pricing (subtotal, shipping, commission)
//   - created_at
//
// Authorization: Admin only
func (h *AdminOrderHandler) ListOrders(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context for audit logging
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	source := c.Query("source")
	dateFromStr := c.Query("date_from")
	dateToStr := c.Query("date_to")
	search := c.Query("search")

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Build filters
	filters := application.AdminOrderListFilters{
		Page:     page,
		PageSize: pageSize,
	}

	if status != "" {
		filters.Status = &status
	}

	if source != "" {
		filters.SourceType = &source
	}

	if search != "" {
		filters.Search = &search
	}

	if dateFromStr != "" {
		if t, err := time.Parse(time.RFC3339, dateFromStr); err == nil {
			filters.DateFrom = &t
		}
	}

	if dateToStr != "" {
		if t, err := time.Parse(time.RFC3339, dateToStr); err == nil {
			filters.DateTo = &t
		}
	}

	// Execute query within transaction
	var result *application.AdminOrderListResponse
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		result, err = h.queryService.ListAllOrdersForAdmin(ctx, tx, filters)
		return err
	})

	if err != nil {
		h.log.Error("Failed to list orders for admin",
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to fetch orders")
		return
	}

	response.SuccessWithMeta(c, gin.H{
		"orders": result.Orders,
	}, &response.Meta{
		Page:       page,
		PerPage:    pageSize,
		Total:      int(result.Total),
		TotalPages: int((result.Total + int64(pageSize) - 1) / int64(pageSize)),
	})
}

// ============================================================================
// ADMIN ORDER DETAIL ENDPOINT
// ============================================================================

// OrderDetailResponse contains the full order details for admin.
type OrderDetailResponse struct {
	ID          uuid.UUID `json:"id"`
	OrderNumber string    `json:"order_number"`
	BuyerID     uuid.UUID `json:"buyer_id"`
	SellerID    uuid.UUID `json:"seller_id"`
	SourceType  string    `json:"source_type"`
	SourceID    uuid.UUID `json:"source_id"`
	// SourceStatus is the current governance status of the underlying fixed-price sale or
	// auction at query time (read-only operator visibility). Nil for negotiation
	// orders or when the source row is not found.
	SourceStatus       *string    `json:"source_status,omitempty"`
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
	TrackingNumber     *string    `json:"tracking_number,omitempty"`
	AutoReleaseAt      *time.Time `json:"auto_release_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// User info.
	//
	// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE:
	// buyer_username  = user_profiles.username
	// seller_username = user_profiles.username   (strict; NEVER store_name)
	// seller_farm_name  = seller_profiles.store_name (additive)
	// seller_avatar_url = user_profiles.avatar_url   (additive alias)
	BuyerUsername  *string `json:"buyer_username,omitempty"`
	BuyerAvatar    *string `json:"buyer_avatar,omitempty"`
	SellerUsername *string `json:"seller_username,omitempty"`
	SellerAvatar   *string `json:"seller_avatar,omitempty"`

	// Phase 5 Stage 1 additive fields with strict source separation.
	SellerFarmName  *string `json:"seller_farm_name,omitempty"`
	SellerAvatarURL *string `json:"seller_avatar_url,omitempty"`

	// Items
	Items []OrderItemDetail `json:"items,omitempty"`

	// Shipping source + origin (I1-C1: where shipping cost originated + seller origin)
	ShippingSource *string               `json:"shipping_source,omitempty"`
	ShippingOrigin *ShippingOriginDetail `json:"shipping_origin,omitempty"`

	// Shipping address
	ShippingAddress *ShippingAddressDetail `json:"shipping_address,omitempty"`

	// Dispute info (if exists)
	Dispute *DisputeSummary `json:"dispute,omitempty"`

	// Refund info (if exists) — surfaces refund_id for gateway retry
	Refund *RefundSummary `json:"refund,omitempty"`

	// Timeline events
	Timeline []TimelineEvent `json:"timeline,omitempty"`
}

// OrderItemDetail represents an order item for admin view.
type OrderItemDetail struct {
	ProductID        uuid.UUID `json:"product_id"`
	ProductTitle     string    `json:"product_title"`
	Quantity         int       `json:"quantity"`
	UnitPrice        int64     `json:"unit_price"`
	Subtotal         int64     `json:"subtotal"`
	SnapshotImageURL *string   `json:"snapshot_image_url,omitempty"`
}

// ShippingAddressDetail represents shipping address for admin view.
type ShippingAddressDetail struct {
	ID            uuid.UUID `json:"id"`
	RecipientName string    `json:"recipient_name"`
	Phone         string    `json:"phone"`
	Province      string    `json:"province"`
	City          string    `json:"city"`
	Address       string    `json:"address"`
	PostalCode    string    `json:"postal_code"`
}

// ShippingOriginDetail represents seller's farm/warehouse address for admin view.
type ShippingOriginDetail struct {
	RecipientName string `json:"recipient_name"`
	Phone         string `json:"phone"`
	Province      string `json:"province"`
	City          string `json:"city"`
	District      string `json:"district,omitempty"`
	Village       string `json:"village,omitempty"`
	Address       string `json:"address"`
	PostalCode    string `json:"postal_code"`
}

// DisputeSummary represents a dispute summary for order context.
type DisputeSummary struct {
	ID          uuid.UUID  `json:"id"`
	Reason      string     `json:"reason"`
	Description *string    `json:"description,omitempty"`
	Status      string     `json:"status"`
	OpenedAt    time.Time  `json:"opened_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// RefundSummary represents a refund linked to an order for admin view.
// Surfaces refund_id so the admin can trigger gateway retry via
// POST /admin/refunds/:refund_id/gateway/initiate.
type RefundSummary struct {
	ID               uuid.UUID `json:"id"`
	Status           string    `json:"status"`
	Reason           string    `json:"reason"`
	GatewayStatus    string    `json:"gateway_status"`
	GatewayAttempts  int       `json:"gateway_attempts"`
	GatewayRefundID  *string   `json:"gateway_refund_id,omitempty"`
	LastGatewayError *string   `json:"last_gateway_error,omitempty"`
	RequestedAmount  int64     `json:"requested_amount"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TimelineEvent represents a timeline event in the order lifecycle.
type TimelineEvent struct {
	Event     string                 `json:"event"`
	Timestamp time.Time              `json:"timestamp"`
	ActorID   *uuid.UUID             `json:"actor_id,omitempty"`
	ActorName *string                `json:"actor_name,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// GetOrderDetail handles GET /api/v1/admin/orders/:id
//
// Returns FULL order detail including:
//   - items
//   - pricing breakdown
//   - shipping info
//   - timeline (events)
//   - dispute (if exists)
//
// Authorization: Admin only
func (h *AdminOrderHandler) GetOrderDetail(c *gin.Context) {
	ctx := c.Request.Context()

	// Get admin ID from context
	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	// Parse order ID
	orderID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	// Execute query within transaction
	var detail *OrderDetailResponse
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Get order from orders table (not projection, for full detail)
		//
		// WARNING: refunded_amount from orders table is NON-AUTHORITATIVE.
		// This is a legacy column for display only.
		// For financial truth, query the Ledger service.
		//
		// Payment = transaction record (what happened)
		// Ledger = financial source of truth (current state)
		var id, buyerID, sellerID, sourceID uuid.UUID
		var sourceType, status, escrowStatus string
		var hasDispute bool
		var disputeStatus *string
		var subtotal, shippingTotal, commissionAmount, serviceFeeAmount, totalPayableAmount, refundedAmount int64
		var shippingOptionName, shippingReference *string
		var shippingSource *string
		var originSnapshotJSON []byte
		var autoReleaseAt *time.Time
		var createdAt, updatedAt time.Time
		var orderNumber *string

		err := tx.QueryRow(ctx, `
			SELECT id, buyer_id, seller_id, source_type, source_id,
		       status, escrow_status, has_dispute, dispute_status,
		       subtotal, shipping_total, commission_amount, service_fee_amount, total_payable_amount,
		       refunded_amount,
			       shipping_option_name, tracking_number,
			       shipping_source, shipping_origin_snapshot,
			       auto_release_at, created_at, updated_at,
			       order_number
			FROM orders WHERE id = $1
		`, orderID).Scan(
			&id, &buyerID, &sellerID, &sourceType, &sourceID,
			&status, &escrowStatus, &hasDispute, &disputeStatus,
			&subtotal, &shippingTotal, &commissionAmount, &serviceFeeAmount, &totalPayableAmount,
			&refundedAmount,
			&shippingOptionName, &shippingReference,
			&shippingSource, &originSnapshotJSON,
			&autoReleaseAt, &createdAt, &updatedAt,
			&orderNumber,
		)

		if err != nil {
			return err
		}

		// Unmarshal shipping origin snapshot from JSONB
		var shippingOriginDetail *ShippingOriginDetail
		if originSnapshotJSON != nil {
			var snap struct {
				RecipientName string `json:"recipient_name"`
				Phone         string `json:"phone"`
				ProvinceName  string `json:"province_name"`
				CityName      string `json:"city_name"`
				DistrictName  string `json:"district_name"`
				VillageName   string `json:"village_name"`
				StreetAddress string `json:"street_address"`
				PostalCode    string `json:"postal_code"`
			}
			if err := json.Unmarshal(originSnapshotJSON, &snap); err == nil {
				shippingOriginDetail = &ShippingOriginDetail{
					RecipientName: snap.RecipientName,
					Phone:         snap.Phone,
					Province:      snap.ProvinceName,
					City:          snap.CityName,
					District:      snap.DistrictName,
					Village:       snap.VillageName,
					Address:       snap.StreetAddress,
					PostalCode:    snap.PostalCode,
				}
			}
		}

		orderNumStr := ""
		if orderNumber != nil {
			orderNumStr = *orderNumber
		}

		detail = &OrderDetailResponse{
			ID:                 id,
			OrderNumber:        orderNumStr,
			BuyerID:            buyerID,
			SellerID:           sellerID,
			SourceType:         sourceType,
			SourceID:           sourceID,
			Status:             status,
			EscrowStatus:       escrowStatus,
			HasDispute:         hasDispute,
			DisputeStatus:      disputeStatus,
			Subtotal:           subtotal,
			ShippingTotal:      shippingTotal,
			CommissionAmount:   commissionAmount,
			ServiceFeeAmount:   serviceFeeAmount,
			TotalPayableAmount: totalPayableAmount,
			RefundedAmount:     refundedAmount,
			ShippingOption:     shippingOptionName,
			TrackingNumber:     shippingReference,
			ShippingSource:     shippingSource,
			ShippingOrigin:     shippingOriginDetail,
			AutoReleaseAt:      autoReleaseAt,
			CreatedAt:          createdAt,
			UpdatedAt:          updatedAt,
		}

		// Fetch source governance status (read-only operator visibility).
		// for_sale → for_sales.status; auction → auctions.status; negotiation → nil.
		var sourceStatus string
		switch sourceType {
		case "for_sale":
			_ = tx.QueryRow(ctx, `SELECT status FROM for_sales WHERE id = $1`, sourceID).Scan(&sourceStatus)
		case "auction":
			_ = tx.QueryRow(ctx, `SELECT status FROM auctions WHERE id = $1`, sourceID).Scan(&sourceStatus)
		}
		if sourceStatus != "" {
			detail.SourceStatus = &sourceStatus
		}

		// Phase 5 Stage 1 — SELLER/FARM CONTRACT CONVERGENCE:
		// Strict source separation. Username sources are
		// user_profiles.username only; the dedicated seller_farm_name
		// field exposes seller_profiles.store_name.
		var buyerUsername, buyerAvatar string
		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(username, ''), COALESCE(avatar_url, '')
			FROM user_profiles WHERE user_id = $1
		`, buyerID).Scan(&buyerUsername, &buyerAvatar)

		var sellerUsername, sellerFarmName, sellerAvatar string
		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(up.username, '')   AS seller_username,
			       COALESCE(sp.store_name, '') AS seller_farm_name,
			       COALESCE(up.avatar_url, '') AS seller_avatar_url
			FROM users u
			LEFT JOIN user_profiles   up ON up.user_id = u.id
			LEFT JOIN seller_profiles sp ON sp.user_id = u.id
			WHERE u.id = $1
		`, sellerID).Scan(&sellerUsername, &sellerFarmName, &sellerAvatar)

		if buyerUsername != "" {
			detail.BuyerUsername = &buyerUsername
		}
		if buyerAvatar != "" {
			detail.BuyerAvatar = &buyerAvatar
		}
		if sellerUsername != "" {
			detail.SellerUsername = &sellerUsername
		}
		if sellerAvatar != "" {
			avatarCopy := sellerAvatar
			detail.SellerAvatar = &avatarCopy
			detail.SellerAvatarURL = &sellerAvatar
		}
		if sellerFarmName != "" {
			detail.SellerFarmName = &sellerFarmName
		}

		// Fetch order items
		rows, err := tx.Query(ctx, `
			SELECT product_id, name, quantity, unit_price, subtotal, snapshot_image_url
			FROM order_items WHERE order_id = $1
		`, orderID)
		if err == nil {
			defer rows.Close()
			items := []OrderItemDetail{}
			for rows.Next() {
				var item OrderItemDetail
				if err := rows.Scan(
					&item.ProductID, &item.ProductTitle, &item.Quantity,
					&item.UnitPrice, &item.Subtotal, &item.SnapshotImageURL,
				); err == nil {
					items = append(items, item)
				}
			}
			detail.Items = items
		}

		// Fetch shipping address
		var addressID uuid.UUID
		var recipientName, phone, province, city, address, postalCode string
		err = tx.QueryRow(ctx, `
			SELECT sa.id, sa.recipient_name, sa.phone, sa.province,
			       sa.city, sa.address, sa.postal_code
			FROM shipping_addresses sa
			INNER JOIN orders o ON o.shipping_address_id = sa.id
			WHERE o.id = $1
		`, orderID).Scan(
			&addressID, &recipientName, &phone, &province,
			&city, &address, &postalCode,
		)
		if err == nil {
			detail.ShippingAddress = &ShippingAddressDetail{
				ID:            addressID,
				RecipientName: recipientName,
				Phone:         phone,
				Province:      province,
				City:          city,
				Address:       address,
				PostalCode:    postalCode,
			}
		}

		// Fetch dispute if exists
		if hasDispute {
			var disputeID uuid.UUID
			var reason, disputeStatus string
			var description *string
			var openedAt time.Time
			var resolvedAt *time.Time

			err := tx.QueryRow(ctx, `
				SELECT id, reason, description, status, opened_at, resolved_at
				FROM disputes WHERE order_id = $1
			`, orderID).Scan(
				&disputeID, &reason, &description, &disputeStatus, &openedAt, &resolvedAt,
			)
			if err == nil {
				detail.Dispute = &DisputeSummary{
					ID:          disputeID,
					Reason:      reason,
					Description: description,
					Status:      disputeStatus,
					OpenedAt:    openedAt,
					ResolvedAt:  resolvedAt,
				}
			}
		}

		// Fetch refund for this order (if exists) — surfaces refund_id for gateway retry.
		// Uses LIMIT 1 since an order can have at most one active refund row.
		{
			var rID uuid.UUID
			var rStatus, rReason, rGatewayStatus string
			var rGatewayAttempts int
			var rGatewayRefundID, rLastGatewayError *string
			var rRequestedAmount int64
			var rCreatedAt, rUpdatedAt time.Time
			rErr := tx.QueryRow(ctx, `
				SELECT id, status, reason, gateway_status, gateway_attempts,
				       gateway_refund_id, last_gateway_error, requested_amount,
				       created_at, updated_at
				FROM refunds WHERE order_id = $1 LIMIT 1
			`, orderID).Scan(
				&rID, &rStatus, &rReason, &rGatewayStatus, &rGatewayAttempts,
				&rGatewayRefundID, &rLastGatewayError, &rRequestedAmount,
				&rCreatedAt, &rUpdatedAt,
			)
			if rErr == nil {
				detail.Refund = &RefundSummary{
					ID:               rID,
					Status:           rStatus,
					Reason:           rReason,
					GatewayStatus:    rGatewayStatus,
					GatewayAttempts:  rGatewayAttempts,
					GatewayRefundID:  rGatewayRefundID,
					LastGatewayError: rLastGatewayError,
					RequestedAmount:  rRequestedAmount,
					CreatedAt:        rCreatedAt,
					UpdatedAt:        rUpdatedAt,
				}
			}
		}

		// Build basic timeline
		detail.Timeline = []TimelineEvent{
			{Event: "order_created", Timestamp: createdAt},
		}
		if shippingReference != nil {
			detail.Timeline = append(detail.Timeline, TimelineEvent{
				Event:     "order_shipped",
				Timestamp: updatedAt, // Approximation
			})
		}

		return nil
	})

	if err != nil {
		h.log.Error("Failed to get order detail for admin",
			zap.String("order_id", orderID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to fetch order")
		return
	}

	response.Success(c, detail)
}

// ============================================================================
// ADMIN ORDER TIMELINE ENDPOINT
// ============================================================================

// GetOrderTimeline handles GET /api/v1/admin/orders/:id/timeline
//
// Returns timeline events for the order sourced from the outbox event log.
// Both the active outbox and the archive are queried so events are visible
// regardless of processing state. Dispute events are included when a dispute
// exists for the order.
//
// Response: []TimelineEvent sorted by timestamp ascending.
//
// Authorization: Admin only (order.read)
func (h *AdminOrderHandler) GetOrderTimeline(c *gin.Context) {
	ctx := c.Request.Context()

	adminID, ok := middleware.MustGetUserIDFromContext(c)
	if !ok {
		return
	}

	orderID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	var events []TimelineEvent
	notFound := false

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		// Verify order exists.
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM orders WHERE id = $1)`, orderID,
		).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			notFound = true
			return nil
		}

		// Collect aggregate IDs: always the order, plus dispute if one exists.
		aggregateIDs := []uuid.UUID{orderID}
		var disputeID uuid.UUID
		if err := tx.QueryRow(ctx,
			`SELECT id FROM disputes WHERE order_id = $1 LIMIT 1`, orderID,
		).Scan(&disputeID); err == nil {
			aggregateIDs = append(aggregateIDs, disputeID)
		}

		events = make([]TimelineEvent, 0)
		for _, aggID := range aggregateIDs {
			if err := func() error {
				rows, err := tx.Query(ctx, `
					SELECT event_type, created_at, payload
					FROM (
						SELECT event_type, created_at, payload
						FROM outbox
						WHERE aggregate_id = $1
						UNION ALL
						SELECT event_type, created_at, payload
						FROM outbox_archive
						WHERE aggregate_id = $1
					) combined
				`, aggID)
				if err != nil {
					return fmt.Errorf("failed to query timeline events: %w", err)
				}
				defer rows.Close()

				for rows.Next() {
					var eventType string
					var ts time.Time
					var payload []byte
					if err := rows.Scan(&eventType, &ts, &payload); err != nil {
						return fmt.Errorf("failed to scan timeline event: %w", err)
					}
					evt := TimelineEvent{
						Event:     eventType,
						Timestamp: ts,
					}
					if len(payload) > 0 {
						var meta map[string]interface{}
						if json.Unmarshal(payload, &meta) == nil && len(meta) > 0 {
							evt.Metadata = meta
						}
					}
					events = append(events, evt)
				}
				return rows.Err()
			}(); err != nil {
				return err
			}
		}

		// Sort merged events by timestamp ascending.
		sort.Slice(events, func(i, j int) bool {
			return events[i].Timestamp.Before(events[j].Timestamp)
		})

		return nil
	})

	if err != nil {
		h.log.Error("Failed to get order timeline",
			zap.String("order_id", orderID.String()),
			zap.String("admin_id", adminID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to fetch order timeline")
		return
	}

	if notFound {
		response.NotFound(c, "Order not found")
		return
	}

	response.Success(c, events)
}


