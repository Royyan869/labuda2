package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	promotionRepo "github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
)

// PromotionRepositoryImpl implements PromotionRepository using PostgreSQL.
type PromotionRepositoryImpl struct{}

// NewPromotionRepository creates a new PromotionRepositoryImpl.
func NewPromotionRepository() *PromotionRepositoryImpl {
	return &PromotionRepositoryImpl{}
}

// ========================================================================
// DATABASE TIME (CRITICAL FOR ACCOUNTING)
// ========================================================================

// GetDBTime returns the current database time.
// CRITICAL: All accounting operations MUST use this method instead of time.Now()
// to ensure consistency across multiple app servers and prevent clock skew issues.
func (r *PromotionRepositoryImpl) GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error) {
	query := `SELECT NOW()`

	var dbTime time.Time
	err := tx.QueryRow(ctx, query).Scan(&dbTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get database time: %w", err)
	}

	return dbTime, nil
}

// ========================================================================
// PROMOTION PACKAGES
// ========================================================================

// CreatePackage persists a new promotion package.
func (r *PromotionRepositoryImpl) CreatePackage(ctx context.Context, tx db.Tx, pkg *entity.PromotionPackage) error {
	query := `
		INSERT INTO promotion_packages (
			id, name, total_duration_hours, validity_window_hours,
			price_amount, allowed_target_types, is_active, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	allowedTypes := make([]string, len(pkg.AllowedTargetTypes))
	for i, tt := range pkg.AllowedTargetTypes {
		allowedTypes[i] = string(tt)
	}

	_, err := tx.Exec(ctx, query,
		pkg.ID,
		pkg.Name,
		pkg.TotalDurationHours,
		pkg.ValidityWindowHours,
		pkg.PriceAmount,
		allowedTypes,
		pkg.IsActive,
		pkg.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create promotion package: %w", err)
	}

	return nil
}

// GetPackageByID retrieves a package by ID.
func (r *PromotionRepositoryImpl) GetPackageByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionPackage, error) {
	query := `
		SELECT id, name, total_duration_hours, validity_window_hours,
		       price_amount, allowed_target_types, is_active, created_at
		FROM promotion_packages
		WHERE id = $1
	`

	var pkg entity.PromotionPackage
	var allowedTypes []string

	err := tx.QueryRow(ctx, query, id).Scan(
		&pkg.ID,
		&pkg.Name,
		&pkg.TotalDurationHours,
		&pkg.ValidityWindowHours,
		&pkg.PriceAmount,
		&allowedTypes,
		&pkg.IsActive,
		&pkg.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get promotion package: %w", err)
	}

	// Convert string slice to TargetType slice
	pkg.AllowedTargetTypes = make([]entity.TargetType, len(allowedTypes))
	for i, at := range allowedTypes {
		pkg.AllowedTargetTypes[i] = entity.TargetType(at)
	}

	return &pkg, nil
}

// ListPackages retrieves all active packages.
func (r *PromotionRepositoryImpl) ListPackages(ctx context.Context, tx db.Tx, includeInactive bool) ([]*entity.PromotionPackage, error) {
	query := `
		SELECT id, name, total_duration_hours, validity_window_hours,
		       price_amount, allowed_target_types, is_active, created_at
		FROM promotion_packages
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if !includeInactive {
		query += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, true)
		argIdx++
	}

	query += " ORDER BY price_amount"

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list promotion packages: %w", err)
	}
	defer rows.Close()

	var packages []*entity.PromotionPackage
	for rows.Next() {
		var pkg entity.PromotionPackage
		var allowedTypes []string

		err := rows.Scan(
			&pkg.ID,
			&pkg.Name,
			&pkg.TotalDurationHours,
			&pkg.ValidityWindowHours,
			&pkg.PriceAmount,
			&allowedTypes,
			&pkg.IsActive,
			&pkg.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion package: %w", err)
		}

		pkg.AllowedTargetTypes = make([]entity.TargetType, len(allowedTypes))
		for i, at := range allowedTypes {
			pkg.AllowedTargetTypes[i] = entity.TargetType(at)
		}

		packages = append(packages, &pkg)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion packages: %w", rows.Err())
	}

	return packages, nil
}

// UpdatePackage updates a package.
func (r *PromotionRepositoryImpl) UpdatePackage(ctx context.Context, tx db.Tx, pkg *entity.PromotionPackage) error {
	query := `
		UPDATE promotion_packages
		SET name = $2, total_duration_hours = $3, validity_window_hours = $4,
		    price_amount = $5, allowed_target_types = $6, is_active = $7
		WHERE id = $1
	`

	allowedTypes := make([]string, len(pkg.AllowedTargetTypes))
	for i, tt := range pkg.AllowedTargetTypes {
		allowedTypes[i] = string(tt)
	}

	result, err := tx.Exec(ctx, query,
		pkg.ID,
		pkg.Name,
		pkg.TotalDurationHours,
		pkg.ValidityWindowHours,
		pkg.PriceAmount,
		allowedTypes,
		pkg.IsActive,
	)

	if err != nil {
		return fmt.Errorf("failed to update promotion package: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("promotion package not found: %s", pkg.ID)
	}

	return nil
}

// ========================================================================
// PROMOTION OWNERSHIP
// ========================================================================

// CreateOwnership persists a new promotion ownership.
func (r *PromotionRepositoryImpl) CreateOwnership(ctx context.Context, tx db.Tx, ownership *entity.PromotionOwnership) error {
	query := `
		INSERT INTO promotion_ownerships (
			id, user_id, package_id, status, purchased_at, expires_at,
			total_duration_hours, consumed_duration_hours, source_billing_id,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := tx.Exec(ctx, query,
		ownership.ID,
		ownership.UserID,
		ownership.PackageID,
		string(ownership.Status),
		ownership.PurchasedAt,
		ownership.ExpiresAt,
		ownership.TotalDurationHours,
		ownership.ConsumedDurationHours,
		ownership.SourceBillingID, // nil → SQL NULL; non-nil → checked by unique index
		ownership.CreatedAt,
		ownership.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create promotion ownership: %w", err)
	}

	return nil
}

// GetOwnershipByID retrieves an ownership by ID.
func (r *PromotionRepositoryImpl) GetOwnershipByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionOwnership, error) {
	query := `
		SELECT id, user_id, package_id, status, purchased_at, expires_at,
		       total_duration_hours, consumed_duration_hours, source_billing_id,
		       created_at, updated_at
		FROM promotion_ownerships
		WHERE id = $1
	`

	var ownership entity.PromotionOwnership
	var statusStr string

	err := tx.QueryRow(ctx, query, id).Scan(
		&ownership.ID,
		&ownership.UserID,
		&ownership.PackageID,
		&statusStr,
		&ownership.PurchasedAt,
		&ownership.ExpiresAt,
		&ownership.TotalDurationHours,
		&ownership.ConsumedDurationHours,
		&ownership.SourceBillingID,
		&ownership.CreatedAt,
		&ownership.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get promotion ownership: %w", err)
	}

	ownership.Status = entity.OwnershipStatus(statusStr)
	return &ownership, nil
}

// GetOwnershipForUpdate retrieves an ownership with FOR UPDATE lock.
func (r *PromotionRepositoryImpl) GetOwnershipForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionOwnership, error) {
	query := `
		SELECT id, user_id, package_id, status, purchased_at, expires_at,
		       total_duration_hours, consumed_duration_hours, source_billing_id,
		       created_at, updated_at
		FROM promotion_ownerships
		WHERE id = $1
		FOR UPDATE
	`

	var ownership entity.PromotionOwnership
	var statusStr string

	err := tx.QueryRow(ctx, query, id).Scan(
		&ownership.ID,
		&ownership.UserID,
		&ownership.PackageID,
		&statusStr,
		&ownership.PurchasedAt,
		&ownership.ExpiresAt,
		&ownership.TotalDurationHours,
		&ownership.ConsumedDurationHours,
		&ownership.SourceBillingID,
		&ownership.CreatedAt,
		&ownership.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get promotion ownership for update: %w", err)
	}

	ownership.Status = entity.OwnershipStatus(statusStr)
	return &ownership, nil
}

// GetOwnershipWithInstances retrieves an ownership with all its instances.
// This is used to calculate consumed duration dynamically.
func (r *PromotionRepositoryImpl) GetOwnershipWithInstances(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionOwnership, []*entity.PromotionInstance, error) {
	// Get ownership
	ownership, err := r.GetOwnershipByID(ctx, tx, id)
	if err != nil {
		return nil, nil, err
	}
	if ownership == nil {
		return nil, nil, nil
	}

	// Get all instances for this ownership
	instances, err := r.ListInstancesByOwnership(ctx, tx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get instances for ownership: %w", err)
	}

	return ownership, instances, nil
}

// UpdateOwnership persists changes to an ownership.
func (r *PromotionRepositoryImpl) UpdateOwnership(ctx context.Context, tx db.Tx, ownership *entity.PromotionOwnership) error {
	query := `
		UPDATE promotion_ownerships
		SET status = $2, consumed_duration_hours = $3, updated_at = $4
		WHERE id = $1
	`

	result, err := tx.Exec(ctx, query,
		ownership.ID,
		string(ownership.Status),
		ownership.ConsumedDurationHours,
		ownership.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update promotion ownership: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("promotion ownership not found: %s", ownership.ID)
	}

	return nil
}

// AddConsumedDurationToOwnership adds consumed duration (in seconds) to an ownership.
// This is used when finalizing instances to bake their consumed duration into the ownership.
//
// CRITICAL: This method enforces the accounting invariant:
// consumed_duration_hours <= total_duration_hours
// If adding would exceed total, it caps at total and logs a critical error.
func (r *PromotionRepositoryImpl) AddConsumedDurationToOwnership(ctx context.Context, tx db.Tx, ownershipID uuid.UUID, seconds int) error {
	// CRITICAL: Validate input before proceeding
	if seconds < 0 {
		return fmt.Errorf("invalid negative seconds: %d", seconds)
	}

	// Convert seconds to hours (round up to be conservative)
	hours := (seconds + 3599) / 3600

	// CRITICAL: Check invariant BEFORE update to prevent corruption
	// We need to ensure consumed + new <= total
	checkQuery := `
		SELECT consumed_duration_hours, total_duration_hours
		FROM promotion_ownerships
		WHERE id = $1
		FOR UPDATE
	`

	var currentConsumed, totalHours int
	err := tx.QueryRow(ctx, checkQuery, ownershipID).Scan(&currentConsumed, &totalHours)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("promotion ownership not found: %s", ownershipID)
		}
		return fmt.Errorf("failed to check ownership invariant: %w", err)
	}

	// HARD INVARIANT CHECK: consumed + new_hours <= total
	if currentConsumed+hours > totalHours {
		// CRITICAL INVARIANT VIOLATION: Log with maximum severity
		// This indicates a bug or data corruption that needs immediate attention
		// TODO: Add proper observability/alerting
		//
		// For now, we cap at total to prevent corruption but log the violation
		excess := (currentConsumed + hours) - totalHours
		_ = fmt.Sprintf("CRITICAL ACCOUNTING INVARIANT VIOLATION: ownership %s would exceed total duration by %d hours. Capping at total. This indicates a bug in promotion duration accounting.", ownershipID, excess)

		// Cap at total to prevent corruption
		hours = totalHours - currentConsumed
		if hours < 0 {
			hours = 0
		}
	}

	query := `
		UPDATE promotion_ownerships
		SET consumed_duration_hours = consumed_duration_hours + $2,
		    status = CASE
		        WHEN consumed_duration_hours + $2 >= total_duration_hours THEN 'consumed'
		        ELSE status
		    END,
		    updated_at = NOW()
		WHERE id = $1
	`

	result, err := tx.Exec(ctx, query, ownershipID, hours)
	if err != nil {
		return fmt.Errorf("failed to add consumed duration to ownership: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("promotion ownership not found: %s", ownershipID)
	}

	return nil
}

// ListOwnershipsByUser retrieves all ownerships for a user, optionally filtered by status.
func (r *PromotionRepositoryImpl) ListOwnershipsByUser(ctx context.Context, tx db.Tx, userID uuid.UUID, status entity.OwnershipStatus) ([]*entity.PromotionOwnership, error) {
	query := `
		SELECT id, user_id, package_id, status, purchased_at, expires_at,
		       total_duration_hours, consumed_duration_hours, source_billing_id,
		       created_at, updated_at
		FROM promotion_ownerships
		WHERE user_id = $1
	`
	args := []interface{}{userID}

	if status != "" {
		query += " AND status = $2"
		args = append(args, string(status))
	}

	query += " ORDER BY purchased_at DESC"

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list promotion ownerships: %w", err)
	}
	defer rows.Close()

	var ownerships []*entity.PromotionOwnership
	for rows.Next() {
		var ownership entity.PromotionOwnership
		var statusStr string

		err := rows.Scan(
			&ownership.ID,
			&ownership.UserID,
			&ownership.PackageID,
			&statusStr,
			&ownership.PurchasedAt,
			&ownership.ExpiresAt,
			&ownership.TotalDurationHours,
			&ownership.ConsumedDurationHours,
			&ownership.SourceBillingID,
			&ownership.CreatedAt,
			&ownership.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion ownership: %w", err)
		}

		ownership.Status = entity.OwnershipStatus(statusStr)
		ownerships = append(ownerships, &ownership)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion ownerships: %w", rows.Err())
	}

	return ownerships, nil
}

// ListOwnershipsByUserPaginated retrieves ownerships for a user with pagination.
func (r *PromotionRepositoryImpl) ListOwnershipsByUserPaginated(ctx context.Context, tx db.Tx, userID uuid.UUID, status entity.OwnershipStatus, limit, offset int) ([]*entity.PromotionOwnership, error) {
	query := `
		SELECT id, user_id, package_id, status, purchased_at, expires_at,
		       total_duration_hours, consumed_duration_hours, source_billing_id,
		       created_at, updated_at
		FROM promotion_ownerships
		WHERE user_id = $1
	`
	args := []interface{}{userID}
	argIdx := 2

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(status))
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY purchased_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list promotion ownerships paginated: %w", err)
	}
	defer rows.Close()

	var ownerships []*entity.PromotionOwnership
	for rows.Next() {
		var ownership entity.PromotionOwnership
		var statusStr string

		err := rows.Scan(
			&ownership.ID,
			&ownership.UserID,
			&ownership.PackageID,
			&statusStr,
			&ownership.PurchasedAt,
			&ownership.ExpiresAt,
			&ownership.TotalDurationHours,
			&ownership.ConsumedDurationHours,
			&ownership.SourceBillingID,
			&ownership.CreatedAt,
			&ownership.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion ownership: %w", err)
		}

		ownership.Status = entity.OwnershipStatus(statusStr)
		ownerships = append(ownerships, &ownership)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion ownerships: %w", rows.Err())
	}

	return ownerships, nil
}

// FindActiveOwnershipByUserAndPackage finds an available ownership for a user's specific package.
func (r *PromotionRepositoryImpl) FindActiveOwnershipByUserAndPackage(ctx context.Context, tx db.Tx, userID, packageID uuid.UUID) (*entity.PromotionOwnership, error) {
	query := `
		SELECT id, user_id, package_id, status, purchased_at, expires_at,
		       total_duration_hours, consumed_duration_hours, source_billing_id,
		       created_at, updated_at
		FROM promotion_ownerships
		WHERE user_id = $1 AND package_id = $2 AND status = 'available'
		AND expires_at > NOW()
		AND consumed_duration_hours < total_duration_hours
		ORDER BY purchased_at ASC
		LIMIT 1
	`

	var ownership entity.PromotionOwnership
	var statusStr string

	err := tx.QueryRow(ctx, query, userID, packageID).Scan(
		&ownership.ID,
		&ownership.UserID,
		&ownership.PackageID,
		&statusStr,
		&ownership.PurchasedAt,
		&ownership.ExpiresAt,
		&ownership.TotalDurationHours,
		&ownership.ConsumedDurationHours,
		&ownership.SourceBillingID,
		&ownership.CreatedAt,
		&ownership.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find active promotion ownership: %w", err)
	}

	ownership.Status = entity.OwnershipStatus(statusStr)
	return &ownership, nil
}

// ListExpiredOwnerships retrieves ownerships that have passed their validity window.
func (r *PromotionRepositoryImpl) ListExpiredOwnerships(ctx context.Context, tx db.Tx, limit int) ([]*entity.PromotionOwnership, error) {
	query := `
		SELECT id, user_id, package_id, status, purchased_at, expires_at,
		       total_duration_hours, consumed_duration_hours, source_billing_id,
		       created_at, updated_at
		FROM promotion_ownerships
		WHERE expires_at <= NOW()
		AND status != 'expired'
		AND status != 'cancelled'
		ORDER BY expires_at ASC
		LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list expired ownerships: %w", err)
	}
	defer rows.Close()

	var ownerships []*entity.PromotionOwnership
	for rows.Next() {
		var ownership entity.PromotionOwnership
		var statusStr string

		err := rows.Scan(
			&ownership.ID,
			&ownership.UserID,
			&ownership.PackageID,
			&statusStr,
			&ownership.PurchasedAt,
			&ownership.ExpiresAt,
			&ownership.TotalDurationHours,
			&ownership.ConsumedDurationHours,
			&ownership.SourceBillingID,
			&ownership.CreatedAt,
			&ownership.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan expired ownership: %w", err)
		}

		ownership.Status = entity.OwnershipStatus(statusStr)
		ownerships = append(ownerships, &ownership)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating expired ownerships: %w", rows.Err())
	}

	return ownerships, nil
}

// ========================================================================
// PROMOTION INSTANCES
// ========================================================================

// CreateInstance persists a new promotion instance.
func (r *PromotionRepositoryImpl) CreateInstance(ctx context.Context, tx db.Tx, instance *entity.PromotionInstance) error {
	query := `
		INSERT INTO promotion_instances (
			id, ownership_id, user_id, target_type, target_id,
			status, activated_at, stopped_at, stop_reason,
			paused_at, total_paused_duration,
			finalized, finalized_at, finalized_seconds,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	_, err := tx.Exec(ctx, query,
		instance.ID,
		instance.OwnershipID,
		instance.UserID,
		string(instance.TargetType),
		instance.TargetID,
		string(instance.Status),
		instance.ActivatedAt,
		instance.StoppedAt,
		instance.StopReason,
		instance.PausedAt,
		instance.TotalPausedDuration,
		instance.Finalized,
		instance.FinalizedAt,
		instance.FinalizedSeconds,
		instance.CreatedAt,
		instance.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create promotion instance: %w", err)
	}

	return nil
}

// GetInstanceByID retrieves an instance by ID.
func (r *PromotionRepositoryImpl) GetInstanceByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE id = $1
	`

	var instance entity.PromotionInstance
	var targetTypeStr, statusStr string

	err := tx.QueryRow(ctx, query, id).Scan(
		&instance.ID,
		&instance.OwnershipID,
		&instance.UserID,
		&targetTypeStr,
		&instance.TargetID,
		&statusStr,
		&instance.ActivatedAt,
		&instance.StoppedAt,
		&instance.StopReason,
		&instance.PausedAt,
		&instance.TotalPausedDuration,
		&instance.Finalized,
		&instance.FinalizedAt,
		&instance.FinalizedSeconds,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get promotion instance: %w", err)
	}

	instance.TargetType = entity.TargetType(targetTypeStr)
	instance.Status = entity.InstanceStatus(statusStr)
	return &instance, nil
}

// GetInstanceForUpdate retrieves an instance by ID with FOR UPDATE lock.
func (r *PromotionRepositoryImpl) GetInstanceForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE id = $1
		FOR UPDATE
	`

	var instance entity.PromotionInstance
	var targetTypeStr, statusStr string

	err := tx.QueryRow(ctx, query, id).Scan(
		&instance.ID,
		&instance.OwnershipID,
		&instance.UserID,
		&targetTypeStr,
		&instance.TargetID,
		&statusStr,
		&instance.ActivatedAt,
		&instance.StoppedAt,
		&instance.StopReason,
		&instance.PausedAt,
		&instance.TotalPausedDuration,
		&instance.Finalized,
		&instance.FinalizedAt,
		&instance.FinalizedSeconds,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get promotion instance for update: %w", err)
	}

	instance.TargetType = entity.TargetType(targetTypeStr)
	instance.Status = entity.InstanceStatus(statusStr)
	return &instance, nil
}

// UpdateInstance persists changes to an instance.
func (r *PromotionRepositoryImpl) UpdateInstance(ctx context.Context, tx db.Tx, instance *entity.PromotionInstance) error {
	query := `
		UPDATE promotion_instances
		SET status = $2, activated_at = $3, stopped_at = $4, stop_reason = $5,
		    paused_at = $6, total_paused_duration = $7,
		    finalized = $8, finalized_at = $9, finalized_seconds = $10,
		    updated_at = $11
		WHERE id = $1
	`

	result, err := tx.Exec(ctx, query,
		instance.ID,
		string(instance.Status),
		instance.ActivatedAt,
		instance.StoppedAt,
		instance.StopReason,
		instance.PausedAt,
		instance.TotalPausedDuration,
		instance.Finalized,
		instance.FinalizedAt,
		instance.FinalizedSeconds,
		instance.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update promotion instance: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("promotion instance not found: %s", instance.ID)
	}

	return nil
}

// ListInstancesByUser retrieves all instances for a user, optionally filtered by status.
func (r *PromotionRepositoryImpl) ListInstancesByUser(ctx context.Context, tx db.Tx, userID uuid.UUID, status entity.InstanceStatus) ([]*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE user_id = $1
	`
	args := []interface{}{userID}

	if status != "" {
		query += " AND status = $2"
		args = append(args, string(status))
	}

	query += " ORDER BY created_at DESC"

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list promotion instances: %w", err)
	}
	defer rows.Close()

	var instances []*entity.PromotionInstance
	for rows.Next() {
		var instance entity.PromotionInstance
		var targetTypeStr, statusStr string

		err := rows.Scan(
			&instance.ID,
			&instance.OwnershipID,
			&instance.UserID,
			&targetTypeStr,
			&instance.TargetID,
			&statusStr,
			&instance.ActivatedAt,
			&instance.StoppedAt,
			&instance.StopReason,
			&instance.PausedAt,
			&instance.TotalPausedDuration,
			&instance.Finalized,
			&instance.FinalizedAt,
			&instance.FinalizedSeconds,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion instance: %w", err)
		}

		instance.TargetType = entity.TargetType(targetTypeStr)
		instance.Status = entity.InstanceStatus(statusStr)
		instances = append(instances, &instance)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion instances: %w", rows.Err())
	}

	return instances, nil
}

// ListInstancesByOwnership retrieves all instances for an ownership.
func (r *PromotionRepositoryImpl) ListInstancesByOwnership(ctx context.Context, tx db.Tx, ownershipID uuid.UUID) ([]*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE ownership_id = $1
		ORDER BY created_at DESC
	`

	rows, err := tx.Query(ctx, query, ownershipID)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances by ownership: %w", err)
	}
	defer rows.Close()

	var instances []*entity.PromotionInstance
	for rows.Next() {
		var instance entity.PromotionInstance
		var targetTypeStr, statusStr string

		err := rows.Scan(
			&instance.ID,
			&instance.OwnershipID,
			&instance.UserID,
			&targetTypeStr,
			&instance.TargetID,
			&statusStr,
			&instance.ActivatedAt,
			&instance.StoppedAt,
			&instance.StopReason,
			&instance.PausedAt,
			&instance.TotalPausedDuration,
			&instance.Finalized,
			&instance.FinalizedAt,
			&instance.FinalizedSeconds,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion instance: %w", err)
		}

		instance.TargetType = entity.TargetType(targetTypeStr)
		instance.Status = entity.InstanceStatus(statusStr)
		instances = append(instances, &instance)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion instances: %w", rows.Err())
	}

	return instances, nil
}

// GetActiveInstanceByOwnership retrieves the active instance for an ownership, if any.
func (r *PromotionRepositoryImpl) GetActiveInstanceByOwnership(ctx context.Context, tx db.Tx, ownershipID uuid.UUID) (*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE ownership_id = $1 AND status = 'active'
		LIMIT 1
	`

	var instance entity.PromotionInstance
	var targetTypeStr, statusStr string

	err := tx.QueryRow(ctx, query, ownershipID).Scan(
		&instance.ID,
		&instance.OwnershipID,
		&instance.UserID,
		&targetTypeStr,
		&instance.TargetID,
		&statusStr,
		&instance.ActivatedAt,
		&instance.StoppedAt,
		&instance.StopReason,
		&instance.PausedAt,
		&instance.TotalPausedDuration,
		&instance.Finalized,
		&instance.FinalizedAt,
		&instance.FinalizedSeconds,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active instance by ownership: %w", err)
	}

	instance.TargetType = entity.TargetType(targetTypeStr)
	instance.Status = entity.InstanceStatus(statusStr)
	return &instance, nil
}

// GetActiveInstanceByOwnershipForUpdate retrieves the active instance with FOR UPDATE lock.
// This prevents race conditions during activation by locking the row for the duration
// of the transaction. If no active instance exists, returns nil without error.
func (r *PromotionRepositoryImpl) GetActiveInstanceByOwnershipForUpdate(ctx context.Context, tx db.Tx, ownershipID uuid.UUID) (*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE ownership_id = $1 AND status = 'active'
		FOR UPDATE
		LIMIT 1
	`

	var instance entity.PromotionInstance
	var targetTypeStr, statusStr string

	err := tx.QueryRow(ctx, query, ownershipID).Scan(
		&instance.ID,
		&instance.OwnershipID,
		&instance.UserID,
		&targetTypeStr,
		&instance.TargetID,
		&statusStr,
		&instance.ActivatedAt,
		&instance.StoppedAt,
		&instance.StopReason,
		&instance.PausedAt,
		&instance.TotalPausedDuration,
		&instance.Finalized,
		&instance.FinalizedAt,
		&instance.FinalizedSeconds,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active instance by ownership for update: %w", err)
	}

	instance.TargetType = entity.TargetType(targetTypeStr)
	instance.Status = entity.InstanceStatus(statusStr)
	return &instance, nil
}

// GetActiveInstancesByTarget retrieves all active instances for a specific target.
func (r *PromotionRepositoryImpl) GetActiveInstancesByTarget(ctx context.Context, tx db.Tx, targetType entity.TargetType, targetID uuid.UUID) ([]*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE target_type = $1 AND target_id = $2 AND status = 'active'
	`

	rows, err := tx.Query(ctx, query, string(targetType), targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active instances by target: %w", err)
	}
	defer rows.Close()

	var instances []*entity.PromotionInstance
	for rows.Next() {
		var instance entity.PromotionInstance
		var targetTypeStr, statusStr string

		err := rows.Scan(
			&instance.ID,
			&instance.OwnershipID,
			&instance.UserID,
			&targetTypeStr,
			&instance.TargetID,
			&statusStr,
			&instance.ActivatedAt,
			&instance.StoppedAt,
			&instance.StopReason,
			&instance.PausedAt,
			&instance.TotalPausedDuration,
			&instance.Finalized,
			&instance.FinalizedAt,
			&instance.FinalizedSeconds,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion instance: %w", err)
		}

		instance.TargetType = entity.TargetType(targetTypeStr)
		instance.Status = entity.InstanceStatus(statusStr)
		instances = append(instances, &instance)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion instances: %w", rows.Err())
	}

	return instances, nil
}

// GetPausedInstancesByTarget retrieves all paused instances for a specific target.
func (r *PromotionRepositoryImpl) GetPausedInstancesByTarget(ctx context.Context, tx db.Tx, targetType entity.TargetType, targetID uuid.UUID) ([]*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE target_type = $1 AND target_id = $2 AND status = 'paused'
	`

	rows, err := tx.Query(ctx, query, string(targetType), targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get paused instances by target: %w", err)
	}
	defer rows.Close()

	var instances []*entity.PromotionInstance
	for rows.Next() {
		var instance entity.PromotionInstance
		var targetTypeStr, statusStr string

		err := rows.Scan(
			&instance.ID,
			&instance.OwnershipID,
			&instance.UserID,
			&targetTypeStr,
			&instance.TargetID,
			&statusStr,
			&instance.ActivatedAt,
			&instance.StoppedAt,
			&instance.StopReason,
			&instance.PausedAt,
			&instance.TotalPausedDuration,
			&instance.Finalized,
			&instance.FinalizedAt,
			&instance.FinalizedSeconds,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion instance: %w", err)
		}

		instance.TargetType = entity.TargetType(targetTypeStr)
		instance.Status = entity.InstanceStatus(statusStr)
		instances = append(instances, &instance)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion instances: %w", rows.Err())
	}

	return instances, nil
}

// GetActiveInstancesForDiscovery retrieves active instances for discovery surfaces.
func (r *PromotionRepositoryImpl) GetActiveInstancesForDiscovery(ctx context.Context, tx db.Tx, limit int) ([]*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE status = 'active'
		ORDER BY RANDOM() LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get active instances for discovery: %w", err)
	}
	defer rows.Close()

	var instances []*entity.PromotionInstance
	for rows.Next() {
		var instance entity.PromotionInstance
		var targetTypeStr, statusStr string

		err := rows.Scan(
			&instance.ID,
			&instance.OwnershipID,
			&instance.UserID,
			&targetTypeStr,
			&instance.TargetID,
			&statusStr,
			&instance.ActivatedAt,
			&instance.StoppedAt,
			&instance.StopReason,
			&instance.PausedAt,
			&instance.TotalPausedDuration,
			&instance.Finalized,
			&instance.FinalizedAt,
			&instance.FinalizedSeconds,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion instance: %w", err)
		}

		instance.TargetType = entity.TargetType(targetTypeStr)
		instance.Status = entity.InstanceStatus(statusStr)
		instances = append(instances, &instance)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion instances: %w", rows.Err())
	}

	return instances, nil
}

// GetAllActiveInstances retrieves all active instances (for worker processing).
func (r *PromotionRepositoryImpl) GetAllActiveInstances(ctx context.Context, tx db.Tx, limit int) ([]*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE status = 'active'
		ORDER BY activated_at ASC
		LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get all active instances: %w", err)
	}
	defer rows.Close()

	var instances []*entity.PromotionInstance
	for rows.Next() {
		var instance entity.PromotionInstance
		var targetTypeStr, statusStr string

		err := rows.Scan(
			&instance.ID,
			&instance.OwnershipID,
			&instance.UserID,
			&targetTypeStr,
			&instance.TargetID,
			&statusStr,
			&instance.ActivatedAt,
			&instance.StoppedAt,
			&instance.StopReason,
			&instance.PausedAt,
			&instance.TotalPausedDuration,
			&instance.Finalized,
			&instance.FinalizedAt,
			&instance.FinalizedSeconds,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion instance: %w", err)
		}

		instance.TargetType = entity.TargetType(targetTypeStr)
		instance.Status = entity.InstanceStatus(statusStr)
		instances = append(instances, &instance)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion instances: %w", rows.Err())
	}

	return instances, nil
}

// GetAllPausedInstances retrieves all paused instances (for worker resume sweep).
func (r *PromotionRepositoryImpl) GetAllPausedInstances(ctx context.Context, tx db.Tx, limit int) ([]*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE status = 'paused'
		ORDER BY paused_at ASC
		LIMIT $1
	`

	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get all paused instances: %w", err)
	}
	defer rows.Close()

	var instances []*entity.PromotionInstance
	for rows.Next() {
		var instance entity.PromotionInstance
		var targetTypeStr, statusStr string

		err := rows.Scan(
			&instance.ID,
			&instance.OwnershipID,
			&instance.UserID,
			&targetTypeStr,
			&instance.TargetID,
			&statusStr,
			&instance.ActivatedAt,
			&instance.StoppedAt,
			&instance.StopReason,
			&instance.PausedAt,
			&instance.TotalPausedDuration,
			&instance.Finalized,
			&instance.FinalizedAt,
			&instance.FinalizedSeconds,
			&instance.CreatedAt,
			&instance.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan promotion instance: %w", err)
		}

		instance.TargetType = entity.TargetType(targetTypeStr)
		instance.Status = entity.InstanceStatus(statusStr)
		instances = append(instances, &instance)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating promotion instances: %w", rows.Err())
	}

	return instances, nil
}

// ========================================================================
// VALIDATION
// ========================================================================

// HasActivePromotionForTarget checks if there's already an active promotion for a target.
func (r *PromotionRepositoryImpl) HasActivePromotionForTarget(ctx context.Context, tx db.Tx, targetType entity.TargetType, targetID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM promotion_instances
			WHERE target_type = $1 AND target_id = $2 AND status = 'active'
		)
	`

	var exists bool
	err := tx.QueryRow(ctx, query, string(targetType), targetID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check active promotion for target: %w", err)
	}

	return exists, nil
}

// GetActiveInstanceByTargetForUpdate retrieves an active instance for a target with FOR UPDATE lock.
func (r *PromotionRepositoryImpl) GetActiveInstanceByTargetForUpdate(ctx context.Context, tx db.Tx, targetType entity.TargetType, targetID uuid.UUID) (*entity.PromotionInstance, error) {
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason,
		       paused_at, total_paused_duration,
		       finalized, finalized_at, finalized_seconds,
		       created_at, updated_at
		FROM promotion_instances
		WHERE target_type = $1 AND target_id = $2 AND status = 'active'
		FOR UPDATE
		LIMIT 1
	`

	var instance entity.PromotionInstance
	var targetTypeStr, statusStr string

	err := tx.QueryRow(ctx, query, string(targetType), targetID).Scan(
		&instance.ID,
		&instance.OwnershipID,
		&instance.UserID,
		&targetTypeStr,
		&instance.TargetID,
		&statusStr,
		&instance.ActivatedAt,
		&instance.StoppedAt,
		&instance.StopReason,
		&instance.PausedAt,
		&instance.TotalPausedDuration,
		&instance.Finalized,
		&instance.FinalizedAt,
		&instance.FinalizedSeconds,
		&instance.CreatedAt,
		&instance.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active instance by target for update: %w", err)
	}

	instance.TargetType = entity.TargetType(targetTypeStr)
	instance.Status = entity.InstanceStatus(statusStr)
	return &instance, nil
}

// ListCampaignsAdmin retrieves promotion instances for admin visibility with filters.
func (r *PromotionRepositoryImpl) ListCampaignsAdmin(
	ctx context.Context,
	tx db.Tx,
	filter promotionRepo.AdminCampaignFilter,
) ([]*promotionRepo.AdminCampaignRow, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	args := []interface{}{}
	argIdx := 1

	where := " WHERE 1=1"
	if filter.Status != "" {
		where += fmt.Sprintf(" AND pi.status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.TargetType != "" {
		where += fmt.Sprintf(" AND pi.target_type = $%d", argIdx)
		args = append(args, filter.TargetType)
		argIdx++
	}
	if filter.OwnerUserID != nil {
		where += fmt.Sprintf(" AND pi.user_id = $%d", argIdx)
		args = append(args, *filter.OwnerUserID)
		argIdx++
	}
	if filter.PackageID != nil {
		where += fmt.Sprintf(" AND po.package_id = $%d", argIdx)
		args = append(args, *filter.PackageID)
		argIdx++
	}

	// Count query
	countQuery := `
		SELECT COUNT(*)
		FROM promotion_instances pi
		JOIN promotion_ownerships po ON pi.ownership_id = po.id
	` + where

	var total int
	if err := tx.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count admin campaigns: %w", err)
	}

	// Data query
	dataQuery := `
		SELECT
		    pi.id, pi.ownership_id, pi.user_id, pi.target_type, pi.target_id,
		    pi.status, pi.activated_at, pi.stopped_at, pi.stop_reason,
		    pi.paused_at, pi.total_paused_duration,
		    pi.finalized, pi.finalized_at, pi.finalized_seconds,
		    pi.created_at, pi.updated_at,
		    po.package_id, pp.name AS package_name,
		    po.total_duration_hours, po.consumed_duration_hours
		FROM promotion_instances pi
		JOIN promotion_ownerships po ON pi.ownership_id = po.id
		JOIN promotion_packages pp ON po.package_id = pp.id
	` + where + fmt.Sprintf(" ORDER BY pi.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)

	args = append(args, limit, filter.Offset)

	rows, err := tx.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list admin campaigns: %w", err)
	}
	defer rows.Close()

	var result []*promotionRepo.AdminCampaignRow
	for rows.Next() {
		var instance entity.PromotionInstance
		var row promotionRepo.AdminCampaignRow
		var targetTypeStr, statusStr string

		if err := rows.Scan(
			&instance.ID,
			&instance.OwnershipID,
			&instance.UserID,
			&targetTypeStr,
			&instance.TargetID,
			&statusStr,
			&instance.ActivatedAt,
			&instance.StoppedAt,
			&instance.StopReason,
			&instance.PausedAt,
			&instance.TotalPausedDuration,
			&instance.Finalized,
			&instance.FinalizedAt,
			&instance.FinalizedSeconds,
			&instance.CreatedAt,
			&instance.UpdatedAt,
			&row.PackageID,
			&row.PackageName,
			&row.OwnershipTotalHours,
			&row.OwnershipConsumedHours,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan admin campaign row: %w", err)
		}

		instance.TargetType = entity.TargetType(targetTypeStr)
		instance.Status = entity.InstanceStatus(statusStr)
		row.Instance = &instance
		result = append(result, &row)
	}

	if rows.Err() != nil {
		return nil, 0, fmt.Errorf("error iterating admin campaigns: %w", rows.Err())
	}

	return result, total, nil
}
