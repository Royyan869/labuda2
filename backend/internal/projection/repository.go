// Package projection provides read model maintenance via CQRS-style projections.
// This layer consumes outbox events and updates denormalized read tables.
//
// DESIGN PRINCIPLES:
// - No business validation (handled by domain)
// - No state transitions (handled by domain)
// - No financial recalculation (handled by domain)
// - Pure projection: copies data from write model to read model
//
// IDEMPOTENCY:
// - Single source of truth: projection_tracker table
// - Replaying same event yields same result (overwrite-based)
// - 1 row per entity: projection is last-known-state only
package projection

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/db"
)

//go:generate go run github.com/matryer/moq -out repository_mock.go . Repository

// Repository handles read model updates via upsert operations.
// All upserts are overwrite-based: last write wins.
type Repository struct {
	db *db.DB
}

// NewRepository creates a new projection repository.
func NewRepository(database *db.DB) *Repository {
	return &Repository{
		db: database,
	}
}

// ============================================================================
// ORDER SUMMARY PROJECTION
// ============================================================================

// OrderSummary represents an order in the read model.
// This is a denormalized view optimized for queries.
// 1 row per order_id. Overwritten on each order event.
type OrderSummary struct {
	// Identification (PRIMARY KEY)
	ID uuid.UUID

	// OrderNumber is the human-readable order identifier (e.g., ORD-20260528-AB12CD34).
	// Populated from the orders write model; not stored in order_summaries.
	OrderNumber *string

	// Participants
	BuyerID  uuid.UUID
	SellerID uuid.UUID
	SourceID *uuid.UUID // Can be for_sale_id, auction_id, or negotiation_id

	// Type and status
	SourceType   string // for_sale, auction, negotiation
	Status       string
	EscrowStatus string
	HasDispute   bool

	// Dispute information
	DisputeStatus     *string
	DisputeReason     *string
	DisputeOpenedAt   *time.Time
	DisputeResolvedAt *time.Time

	// Pricing snapshot (in smallest currency unit)
	// Note: Financial truth is in Ledger service, these are for display only
	Subtotal           int64
	ShippingTotal      int64
	CommissionAmount   int64
	ServiceFeeAmount   int64
	TotalPayableAmount int64

	// Escrow snapshot — stored for INSERT completeness (NOT NULL in schema).
	// Not exposed via read queries to OrderListItem; Ledger is the authority
	// for financial amounts.
	EscrowAmount   int64
	RefundedAmount int64

	// Shipping snapshot
	ShippingOptionName    string
	ShippingTransportType string

	// Timestamps
	AutoReleaseAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UpsertOrderSummary creates or updates an order summary.
// Overwrite-based: ON CONFLICT (id) updates ALL columns.
// This ensures projection is always the last-known-state.
func (r *Repository) UpsertOrderSummary(
	ctx context.Context,
	tx db.Tx,
	summary *OrderSummary,
) error {
	query := `
		INSERT INTO order_summaries (
			id, buyer_id, seller_id, source_type, source_id,
			status, escrow_status, has_dispute,
			dispute_status, dispute_reason, dispute_opened_at, dispute_resolved_at,
			subtotal, shipping_total, commission_amount,
			service_fee_amount, total_payable_amount,
			escrow_amount, refunded_amount,
			shipping_option_name, shipping_transport_type,
			auto_release_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5,
		          $6, $7, $8,
		          $9, $10, $11, $12,
		          $13, $14, $15, $16, $17,
		          $18, $19,
		          $20, $21,
		          $22, $23, $24)
		ON CONFLICT (id)
		DO UPDATE SET
			buyer_id = EXCLUDED.buyer_id,
			seller_id = EXCLUDED.seller_id,
			source_type = EXCLUDED.source_type,
			source_id = EXCLUDED.source_id,
			status = EXCLUDED.status,
			escrow_status = EXCLUDED.escrow_status,
			has_dispute = EXCLUDED.has_dispute,
			dispute_status = EXCLUDED.dispute_status,
			dispute_reason = EXCLUDED.dispute_reason,
			dispute_opened_at = EXCLUDED.dispute_opened_at,
			dispute_resolved_at = EXCLUDED.dispute_resolved_at,
			subtotal = EXCLUDED.subtotal,
			shipping_total = EXCLUDED.shipping_total,
			commission_amount = EXCLUDED.commission_amount,
			service_fee_amount = EXCLUDED.service_fee_amount,
			total_payable_amount = EXCLUDED.total_payable_amount,
			escrow_amount = EXCLUDED.escrow_amount,
			refunded_amount = EXCLUDED.refunded_amount,
			shipping_option_name = EXCLUDED.shipping_option_name,
			shipping_transport_type = EXCLUDED.shipping_transport_type,
			auto_release_at = EXCLUDED.auto_release_at,
			updated_at = EXCLUDED.updated_at
	`

	_, err := tx.Exec(ctx, query,
		summary.ID, summary.BuyerID, summary.SellerID, summary.SourceType, summary.SourceID,
		summary.Status, summary.EscrowStatus, summary.HasDispute,
		summary.DisputeStatus, summary.DisputeReason, summary.DisputeOpenedAt, summary.DisputeResolvedAt,
		summary.Subtotal, summary.ShippingTotal, summary.CommissionAmount,
		summary.ServiceFeeAmount, summary.TotalPayableAmount,
		summary.EscrowAmount, summary.RefundedAmount,
		summary.ShippingOptionName, summary.ShippingTransportType,
		summary.AutoReleaseAt, summary.CreatedAt, summary.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("upsert order summary failed: %w", err)
	}

	return nil
}

// GetOrderSummary retrieves an order summary by ID.
func (r *Repository) GetOrderSummary(
	ctx context.Context,
	orderID uuid.UUID,
) (*OrderSummary, error) {
	var summary OrderSummary

	query := `
		SELECT id, buyer_id, seller_id, source_type, source_id,
		       status, escrow_status, has_dispute,
		       dispute_status, dispute_reason, dispute_opened_at, dispute_resolved_at,
		       subtotal, shipping_total, commission_amount, service_fee_amount, total_payable_amount,
		       escrow_amount, refunded_amount,
		       shipping_option_name, shipping_transport_type,
		       auto_release_at, created_at, updated_at
		FROM order_summaries
		WHERE id = $1
	`

	err := r.db.Pool().QueryRow(ctx, query, orderID).Scan(
		&summary.ID, &summary.BuyerID, &summary.SellerID, &summary.SourceType, &summary.SourceID,
		&summary.Status, &summary.EscrowStatus, &summary.HasDispute,
		&summary.DisputeStatus, &summary.DisputeReason, &summary.DisputeOpenedAt, &summary.DisputeResolvedAt,
		&summary.Subtotal, &summary.ShippingTotal, &summary.CommissionAmount, &summary.ServiceFeeAmount, &summary.TotalPayableAmount,
		&summary.EscrowAmount, &summary.RefundedAmount,
		&summary.ShippingOptionName, &summary.ShippingTransportType,
		&summary.AutoReleaseAt, &summary.CreatedAt, &summary.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("order summary not found: %s", orderID)
		}
		return nil, fmt.Errorf("get order summary failed: %w", err)
	}

	return &summary, nil
}

// ============================================================================
// ACCOUNT BALANCE PROJECTION
// ============================================================================

// AccountBalance represents an account balance in the read model.
// 1 row per account_id. Mirrors financial_accounts.balance.
type AccountBalance struct {
	// Identification (PRIMARY KEY)
	ID uuid.UUID

	// Account details
	UserID      *uuid.UUID
	AccountType string
	Balance     int64
	Currency    string

	// Timestamps
	UpdatedAt time.Time
}

// UpsertAccountBalance creates or updates an account balance.
// Overwrite-based: ON CONFLICT (id) updates ALL columns.
func (r *Repository) UpsertAccountBalance(
	ctx context.Context,
	tx db.Tx,
	balance *AccountBalance,
) error {
	query := `
		INSERT INTO account_balances (
			id, user_id, account_type, balance, currency, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id)
		DO UPDATE SET
			user_id = EXCLUDED.user_id,
			account_type = EXCLUDED.account_type,
			balance = EXCLUDED.balance,
			currency = EXCLUDED.currency,
			updated_at = EXCLUDED.updated_at
	`

	_, err := tx.Exec(ctx, query,
		balance.ID, balance.UserID, balance.AccountType,
		balance.Balance, balance.Currency, balance.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("upsert account balance failed: %w", err)
	}

	return nil
}

// GetAccountBalance retrieves an account balance by ID.
func (r *Repository) GetAccountBalance(
	ctx context.Context,
	accountID uuid.UUID,
) (*AccountBalance, error) {
	var balance AccountBalance

	query := `
		SELECT id, user_id, account_type, balance, currency, updated_at
		FROM account_balances
		WHERE id = $1
	`

	err := r.db.Pool().QueryRow(ctx, query, accountID).Scan(
		&balance.ID, &balance.UserID, &balance.AccountType,
		&balance.Balance, &balance.Currency, &balance.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("account balance not found: %s", accountID)
		}
		return nil, fmt.Errorf("get account balance failed: %w", err)
	}

	return &balance, nil
}

// ============================================================================
// PROJECTION TRACKER (SINGLE IDEMPOTENCY LAYER)
// ============================================================================

// MarkEventProcessed records that an outbox event has been processed.
// This is the ONLY idempotency guard for projections.
func (r *Repository) MarkEventProcessed(
	ctx context.Context,
	tx db.Tx,
	outboxEventID uuid.UUID,
	eventType string,
) error {
	query := `
		INSERT INTO projection_tracker (outbox_event_id, event_type, processed_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (outbox_event_id) DO NOTHING
	`

	_, err := tx.Exec(ctx, query, outboxEventID, eventType)
	if err != nil {
		return fmt.Errorf("mark event processed failed: %w", err)
	}

	return nil
}

// IsEventProcessed checks if an event has already been processed.
func (r *Repository) IsEventProcessed(
	ctx context.Context,
	tx db.Tx,
	outboxEventID uuid.UUID,
) (bool, error) {
	var exists bool

	query := `
		SELECT EXISTS (
			SELECT 1 FROM projection_tracker
			WHERE outbox_event_id = $1
		)
	`

	err := tx.QueryRow(ctx, query, outboxEventID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check event processed failed: %w", err)
	}

	return exists, nil
}

// ============================================================================
// QUERY HELPERS (for read-side)
// ============================================================================

// ListOrderSummariesByBuyer retrieves order summaries for a buyer with cursor-based pagination.
// cursor: Unix timestamp (int64) of created_at - returns orders created before this time
// limit: Maximum number of results (capped at 50)
func (r *Repository) ListOrderSummariesByBuyer(
	ctx context.Context,
	tx db.Tx,
	buyerID uuid.UUID,
	status *string,
	limit int,
	cursor int64,
) ([]*OrderSummary, error) {
	// Cap limit at 50
	if limit > 50 {
		limit = 50
	}
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, buyer_id, seller_id, source_type, source_id,
		       status, escrow_status, has_dispute,
		       dispute_status, dispute_reason, dispute_opened_at, dispute_resolved_at,
		       subtotal, shipping_total, commission_amount,
		       escrow_amount, refunded_amount,
		       shipping_option_name, shipping_transport_type,
		       auto_release_at, created_at, updated_at
		FROM order_summaries
		WHERE buyer_id = $1
	`

	args := []interface{}{buyerID}
	argIdx := 2

	// Add cursor filter if provided
	if cursor > 0 {
		cursorTime := time.Unix(cursor, 0).UTC()
		query += fmt.Sprintf(" AND created_at < $%d", argIdx)
		args = append(args, cursorTime)
		argIdx++
	}

	// Add optional status filter
	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list order summaries by buyer failed: %w", err)
	}
	defer rows.Close()

	summaries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*OrderSummary, error) {
		var s OrderSummary
		err := row.Scan(
			&s.ID, &s.BuyerID, &s.SellerID, &s.SourceType, &s.SourceID,
			&s.Status, &s.EscrowStatus, &s.HasDispute,
			&s.DisputeStatus, &s.DisputeReason, &s.DisputeOpenedAt, &s.DisputeResolvedAt,
			&s.Subtotal, &s.ShippingTotal, &s.CommissionAmount,
			&s.EscrowAmount, &s.RefundedAmount,
			&s.ShippingOptionName, &s.ShippingTransportType,
			&s.AutoReleaseAt, &s.CreatedAt, &s.UpdatedAt,
		)
		return &s, err
	})

	if err != nil {
		return nil, fmt.Errorf("scan order summaries failed: %w", err)
	}

	return summaries, nil
}

// ListOrderSummariesBySeller retrieves order summaries for a seller with cursor-based pagination.
// cursor: Unix timestamp (int64) of created_at - returns orders created before this time
// limit: Maximum number of results (capped at 50)
func (r *Repository) ListOrderSummariesBySeller(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	status *string,
	limit int,
	cursor int64,
) ([]*OrderSummary, error) {
	// Cap limit at 50
	if limit > 50 {
		limit = 50
	}
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, buyer_id, seller_id, source_type, source_id,
		       status, escrow_status, has_dispute,
		       dispute_status, dispute_reason, dispute_opened_at, dispute_resolved_at,
		       subtotal, shipping_total, commission_amount,
		       escrow_amount, refunded_amount,
		       shipping_option_name, shipping_transport_type,
		       auto_release_at, created_at, updated_at
		FROM order_summaries
		WHERE seller_id = $1
	`

	args := []interface{}{sellerID}
	argIdx := 2

	// Add cursor filter if provided
	if cursor > 0 {
		cursorTime := time.Unix(cursor, 0).UTC()
		query += fmt.Sprintf(" AND created_at < $%d", argIdx)
		args = append(args, cursorTime)
		argIdx++
	}

	// Add optional status filter
	if status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *status)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list order summaries by seller failed: %w", err)
	}
	defer rows.Close()

	summaries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*OrderSummary, error) {
		var s OrderSummary
		err := row.Scan(
			&s.ID, &s.BuyerID, &s.SellerID, &s.SourceType, &s.SourceID,
			&s.Status, &s.EscrowStatus, &s.HasDispute,
			&s.DisputeStatus, &s.DisputeReason, &s.DisputeOpenedAt, &s.DisputeResolvedAt,
			&s.Subtotal, &s.ShippingTotal, &s.CommissionAmount,
			&s.EscrowAmount, &s.RefundedAmount,
			&s.ShippingOptionName, &s.ShippingTransportType,
			&s.AutoReleaseAt, &s.CreatedAt, &s.UpdatedAt,
		)
		return &s, err
	})

	if err != nil {
		return nil, fmt.Errorf("scan order summaries failed: %w", err)
	}

	return summaries, nil
}

// GetUserAccountBalances retrieves all account balances for a user.
func (r *Repository) GetUserAccountBalances(
	ctx context.Context,
	userID uuid.UUID,
) ([]*AccountBalance, error) {
	query := `
		SELECT id, user_id, account_type, balance, currency, updated_at
		FROM account_balances
		WHERE user_id = $1
		ORDER BY account_type
	`

	rows, err := r.db.Pool().Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list user balances failed: %w", err)
	}
	defer rows.Close()

	balances, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*AccountBalance, error) {
		var b AccountBalance
		err := row.Scan(
			&b.ID, &b.UserID, &b.AccountType,
			&b.Balance, &b.Currency, &b.UpdatedAt,
		)
		return &b, err
	})

	if err != nil {
		return nil, fmt.Errorf("scan user balances failed: %w", err)
	}

	return balances, nil
}

// GetSystemAccountBalances retrieves all system account balances.
func (r *Repository) GetSystemAccountBalances(
	ctx context.Context,
) ([]*AccountBalance, error) {
	query := `
		SELECT id, user_id, account_type, balance, currency, updated_at
		FROM account_balances
		WHERE user_id IS NULL
		ORDER BY account_type
	`

	rows, err := r.db.Pool().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list system balances failed: %w", err)
	}
	defer rows.Close()

	balances, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*AccountBalance, error) {
		var b AccountBalance
		err := row.Scan(
			&b.ID, &b.UserID, &b.AccountType,
			&b.Balance, &b.Currency, &b.UpdatedAt,
		)
		return &b, err
	})

	if err != nil {
		return nil, fmt.Errorf("scan system balances failed: %w", err)
	}

	return balances, nil
}

// CountOrderSummariesByBuyer returns the total number of order summaries for a
// buyer, optionally filtered by status. Used by OrderQueryService for
// Option-B count-comparison fallback: if this count < write-model count the
// projection is considered incomplete and the write model is used instead.
func (r *Repository) CountOrderSummariesByBuyer(
	ctx context.Context,
	tx db.Tx,
	buyerID uuid.UUID,
	status *string,
) (int64, error) {
	query := "SELECT COUNT(*) FROM order_summaries WHERE buyer_id = $1"
	args := []interface{}{buyerID}
	if status != nil {
		query += " AND status = $2"
		args = append(args, *status)
	}
	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count order summaries by buyer failed: %w", err)
	}
	return count, nil
}

// CountOrderSummariesBySeller returns the total number of order summaries for a
// seller, optionally filtered by status. Used for the same Option-B safety
// fallback as CountOrderSummariesByBuyer.
func (r *Repository) CountOrderSummariesBySeller(
	ctx context.Context,
	tx db.Tx,
	sellerID uuid.UUID,
	status *string,
) (int64, error) {
	query := "SELECT COUNT(*) FROM order_summaries WHERE seller_id = $1"
	args := []interface{}{sellerID}
	if status != nil {
		query += " AND status = $2"
		args = append(args, *status)
	}
	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count order summaries by seller failed: %w", err)
	}
	return count, nil
}

// ============================================================================
// ADMIN ORDER QUERIES
// ============================================================================

// OrderListFilters contains filters for admin order listing.
type OrderListFilters struct {
	Status     *string
	SourceType *string
	DateFrom   *time.Time
	DateTo     *time.Time
	Page       int
	PageSize   int
}

// ListOrderSummariesForAdmin retrieves all order summaries for admin with filtering and pagination.
// This is a read-only admin endpoint that returns ALL orders (not scoped to a user).
func (r *Repository) ListOrderSummariesForAdmin(
	ctx context.Context,
	tx db.Tx,
	filters OrderListFilters,
) ([]*OrderSummary, int64, error) {
	// Validate pagination
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 || filters.PageSize > 100 {
		filters.PageSize = 20
	}

	// Build base query
	baseQuery := `
		FROM order_summaries
		WHERE 1=1
	`

	args := []interface{}{}
	argIdx := 1

	// Add status filter
	if filters.Status != nil {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filters.Status)
		argIdx++
	}

	// Add source_type filter
	if filters.SourceType != nil {
		baseQuery += fmt.Sprintf(" AND source_type = $%d", argIdx)
		args = append(args, *filters.SourceType)
		argIdx++
	}

	// Add date range filters
	if filters.DateFrom != nil {
		baseQuery += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *filters.DateFrom)
		argIdx++
	}

	if filters.DateTo != nil {
		baseQuery += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *filters.DateTo)
		argIdx++
	}

	// Get total count
	countQuery := "SELECT COUNT(*) " + baseQuery
	var total int64
	err := tx.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count orders failed: %w", err)
	}

	// Get paginated results
	offset := (filters.Page - 1) * filters.PageSize
	dataQuery := `
		SELECT id, buyer_id, seller_id, source_type, source_id,
		       status, escrow_status, has_dispute,
		       dispute_status, dispute_reason, dispute_opened_at, dispute_resolved_at,
		       subtotal, shipping_total, commission_amount,
		       escrow_amount, refunded_amount,
		       shipping_option_name, shipping_transport_type,
		       auto_release_at, created_at, updated_at
		` + baseQuery + `
		ORDER BY created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)

	args = append(args, filters.PageSize, offset)

	rows, err := tx.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders failed: %w", err)
	}
	defer rows.Close()

	summaries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*OrderSummary, error) {
		var s OrderSummary
		err := row.Scan(
			&s.ID, &s.BuyerID, &s.SellerID, &s.SourceType, &s.SourceID,
			&s.Status, &s.EscrowStatus, &s.HasDispute,
			&s.DisputeStatus, &s.DisputeReason, &s.DisputeOpenedAt, &s.DisputeResolvedAt,
			&s.Subtotal, &s.ShippingTotal, &s.CommissionAmount,
			&s.EscrowAmount, &s.RefundedAmount,
			&s.ShippingOptionName, &s.ShippingTransportType,
			&s.AutoReleaseAt, &s.CreatedAt, &s.UpdatedAt,
		)
		return &s, err
	})

	if err != nil {
		return nil, 0, fmt.Errorf("scan orders failed: %w", err)
	}

	return summaries, total, nil
}
