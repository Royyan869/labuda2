// Package repository provides the admin repository implementation.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labuda/backend/internal/platform/admin/repository"
	"github.com/labuda/backend/pkg/db"
)

// AdminRepositoryImpl handles admin data persistence using pgx-based DB layer.
type AdminRepositoryImpl struct{}

// NewAdminRepository creates a new AdminRepository.
func NewAdminRepository() repository.AdminRepository {
	return &AdminRepositoryImpl{}
}

// ListUsers returns a paginated list of users with filters applied.
func (r *AdminRepositoryImpl) ListUsers(
	ctx context.Context,
	tx interface{},
	filters repository.UserListFilters,
) ([]repository.UserSummary, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	// Build base query
	baseQuery := `
		SELECT
			u.id, u.firebase_uid, u.email, u.phone_number,
			(u.email_verified_at IS NOT NULL) AS email_verified, u.phone_verified, u.account_status,
			u.role, u.created_at, u.updated_at,
			up.username, up.is_verified,
			cb.balance as coin_balance
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN user_coin_balance cb ON cb.user_id = u.id
		WHERE u.deleted_at IS NULL
	`

	args := []interface{}{}
	argNum := 1

	// Build filters
	whereClauses := r.buildWhereClauses(filters, &argNum, &args)

	// Apply WHERE clause
	if len(whereClauses) > 0 {
		whereSQL := ""
		for i, clause := range whereClauses {
			if i > 0 {
				whereSQL += " AND "
			}
			whereSQL += clause
		}
		baseQuery += " AND " + whereSQL
	}

	// Add sorting and pagination
	sortBy := r.validateSortField(filters.SortBy)
	sortOrder := r.validateSortOrder(filters.SortOrder)
	orderClause := fmt.Sprintf(" ORDER BY u.%s %s", sortBy, sortOrder)
	baseQuery += orderClause + fmt.Sprintf(" LIMIT $%d OFFSET $%d", argNum, argNum+1)
	args = append(args, filters.PageSize, (filters.Page-1)*filters.PageSize)

	// Execute query
	rows, err := dbTx.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	users := []repository.UserSummary{}
	for rows.Next() {
		u, err := r.scanUserSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		users = append(users, *u)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list users scan failed: %w", rows.Err())
	}

	return users, nil
}

// CountUsers returns the total count of users matching filters.
func (r *AdminRepositoryImpl) CountUsers(
	ctx context.Context,
	tx interface{},
	filters repository.UserListFilters,
) (int, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return 0, fmt.Errorf("invalid transaction type")
	}

	countQuery := `
		SELECT COUNT(*)
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		WHERE u.deleted_at IS NULL
	`

	args := []interface{}{}
	argNum := 1

	// Build filters
	whereClauses := r.buildWhereClauses(filters, &argNum, &args)

	// Apply WHERE clause
	if len(whereClauses) > 0 {
		whereSQL := ""
		for i, clause := range whereClauses {
			if i > 0 {
				whereSQL += " AND "
			}
			whereSQL += clause
		}
		countQuery += " AND " + whereSQL
	}

	var total int
	err := dbTx.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return total, nil
}

// GetUserDetails returns detailed information about a specific user.
func (r *AdminRepositoryImpl) GetUserDetails(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
) (*repository.UserDetails, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	query := `
		SELECT
			u.id, u.firebase_uid, u.email, u.phone_number,
			(u.email_verified_at IS NOT NULL) AS email_verified, u.phone_verified, u.account_status,
			u.role, u.created_at, u.updated_at,
			up.username, up.bio, up.avatar_url,
			up.is_verified,
			COALESCE((
				SELECT COUNT(*)
				FROM user_follows uf
				WHERE uf.following_id = u.id
			), 0) AS followers_count,
			COALESCE((
				SELECT COUNT(*)
				FROM user_follows uf
				WHERE uf.follower_id = u.id
			), 0) AS following_count,
			up.city, up.province,
			cb.balance AS coin_balance,
			(sp.id IS NOT NULL) AS has_seller_profile,
			ss.status AS subscription_status,
			sv.status AS verification_status,
			fa.balance AS seller_payable,
			(SELECT p.id
			 FROM payments p
			 WHERE p.user_id = u.id
			   AND p.reference_type = 'subscription'
			   AND p.status IN ('settlement', 'capture')
			   AND NOT EXISTS (SELECT 1 FROM seller_subscriptions ss2 WHERE ss2.payment_id = p.id)
			 ORDER BY p.created_at DESC LIMIT 1
			) AS recoverable_subscription_payment_id,
			srs.current_tier AS seller_tier,
			(SELECT COUNT(*) FROM user_warnings uw WHERE uw.user_id = u.id) AS warning_count,
			(SELECT COUNT(*) FROM user_warnings uw
			 WHERE uw.user_id = u.id
			   AND uw.is_active = true
			   AND (uw.expires_at IS NULL OR uw.expires_at > now())
			) AS active_warning_count,
			(SELECT COUNT(*) FROM user_warnings uw
			 WHERE uw.user_id = u.id
			   AND uw.is_active = true
			   AND (uw.expires_at IS NULL OR uw.expires_at > now())
			   AND uw.level = 'severe'
			) AS severe_warning_count
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN user_coin_balance cb ON cb.user_id = u.id
		LEFT JOIN seller_profiles sp ON sp.user_id = u.id
		LEFT JOIN seller_subscriptions ss ON ss.user_id = u.id
		LEFT JOIN seller_verifications sv ON sv.seller_id = u.id
		LEFT JOIN financial_accounts fa ON fa.user_id = u.id AND fa.account_type = 'SELLER_PAYABLE'
		LEFT JOIN seller_reputation_state srs ON srs.seller_id = u.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`

	var userDetails repository.UserDetails
	var phoneNumber, username, bio, avatarURL, city, province pgtype.Text
	var subscriptionStatus, verificationStatus pgtype.Text
	var coinBalance, followersCount, followingCount, sellerPayable pgtype.Int8
	var recoverablePaymentID pgtype.Text
	var sellerTier pgtype.Text
	// up.is_verified is NOT NULL on user_profiles itself, but this is a
	// LEFT JOIN — a user with no profile row yields NULL for every up.*
	// column, so it must be scanned as nullable.
	var isVerified pgtype.Bool

	err := dbTx.QueryRow(ctx, query, userID).Scan(
		&userDetails.ID, &userDetails.FirebaseUID, &userDetails.Email, &phoneNumber,
		&userDetails.EmailVerified, &userDetails.PhoneVerified, &userDetails.AccountStatus,
		&userDetails.Role, &userDetails.CreatedAt, &userDetails.UpdatedAt,
		&username, &bio, &avatarURL,
		&isVerified,
		&followersCount, &followingCount,
		&city, &province,
		&coinBalance,
		&userDetails.HasSellerProfile,
		&subscriptionStatus,
		&verificationStatus,
		&sellerPayable,
		&recoverablePaymentID,
		&sellerTier,
		&userDetails.WarningCount,
		&userDetails.ActiveWarningCount,
		&userDetails.SevereWarningCount,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found: %s", userID)
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// Populate nullable fields
	userDetails.IsVerified = isVerified.Valid && isVerified.Bool
	userDetails.PhoneNumber = stringPtrFromPgtypeText(phoneNumber)
	userDetails.Username = stringPtrFromPgtypeText(username)
	userDetails.Bio = stringPtrFromPgtypeText(bio)
	userDetails.AvatarURL = stringPtrFromPgtypeText(avatarURL)
	userDetails.CoinBalance = int64PtrFromPgtypeInt8(coinBalance)
	userDetails.FollowersCount = intPtrFromPgtypeInt8(followersCount)
	userDetails.FollowingCount = intPtrFromPgtypeInt8(followingCount)
	userDetails.City = stringPtrFromPgtypeText(city)
	userDetails.Province = stringPtrFromPgtypeText(province)
	userDetails.SubscriptionStatus = stringPtrFromPgtypeText(subscriptionStatus)
	userDetails.VerificationStatus = stringPtrFromPgtypeText(verificationStatus)
	userDetails.SellerPayable = int64PtrFromPgtypeInt8(sellerPayable)
	userDetails.SellerTier = stringPtrFromPgtypeText(sellerTier)
	if recoverablePaymentID.Valid {
		id, err := uuid.Parse(recoverablePaymentID.String)
		if err == nil {
			userDetails.RecoverableSubscriptionPaymentID = &id
		}
	}

	return &userDetails, nil
}

// GetUserStatus returns the current account_status of a user.
func (r *AdminRepositoryImpl) GetUserStatus(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
) (string, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return "", fmt.Errorf("invalid transaction type")
	}

	var status string
	err := dbTx.QueryRow(ctx, "SELECT account_status FROM users WHERE id = $1", userID).Scan(&status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("user not found: %s", userID)
		}
		return "", fmt.Errorf("failed to query user status: %w", err)
	}

	return status, nil
}

// UpdateUserStatus updates the account_status of a user.
func (r *AdminRepositoryImpl) UpdateUserStatus(
	ctx context.Context,
	tx interface{},
	userID uuid.UUID,
	status string,
) error {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return fmt.Errorf("invalid transaction type")
	}

	query := `UPDATE users SET account_status = $1, updated_at = NOW() WHERE id = $2`
	result, err := dbTx.Exec(ctx, query, status, userID)
	if err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	return nil
}

// GetDashboardMetrics returns platform metrics.
func (r *AdminRepositoryImpl) GetDashboardMetrics(
	ctx context.Context,
	tx interface{},
) (*repository.DashboardMetrics, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	var metrics repository.DashboardMetrics

	// Query all metrics
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// Total users
	err := dbTx.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE deleted_at IS NULL").Scan(&metrics.TotalUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to query total users: %w", err)
	}

	// Active users today
	err = dbTx.QueryRow(ctx,
		"SELECT COUNT(*) FROM users WHERE created_at >= $1 AND deleted_at IS NULL",
		today,
	).Scan(&metrics.ActiveUsersToday)
	if err != nil {
		return nil, fmt.Errorf("failed to query active users today: %w", err)
	}

	// Active sellers
	err = dbTx.QueryRow(ctx,
		`SELECT COUNT(*)
		FROM users u
		WHERE u.account_status = 'active'
		  AND u.deleted_at IS NULL
		  AND EXISTS (
			  SELECT 1
			  FROM seller_profiles sp
			  WHERE sp.user_id = u.id
		  )
		  AND EXISTS (
			  SELECT 1
			  FROM seller_subscriptions ss
			  WHERE ss.user_id = u.id
			    AND ss.status = 'active'
			    AND ss.started_at <= CURRENT_TIMESTAMP
			    AND CURRENT_TIMESTAMP < ss.expires_at
		  )`,
	).Scan(&metrics.ActiveSellers)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sellers: %w", err)
	}

	// Total orders
	err = dbTx.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&metrics.TotalOrders)
	if err != nil {
		return nil, fmt.Errorf("failed to query total orders: %w", err)
	}

	// Orders today
	err = dbTx.QueryRow(ctx,
		"SELECT COUNT(*) FROM orders WHERE created_at >= $1",
		today,
	).Scan(&metrics.OrdersToday)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders today: %w", err)
	}

	// Pending reports (canonical moderation: cases with status 'open').
	// The legacy moderation_cases table was dropped in migration 000056;
	// the canonical replacement is cases.status = 'open'.
	err = dbTx.QueryRow(ctx,
		"SELECT COUNT(*) FROM cases WHERE status = 'open'",
	).Scan(&metrics.PendingReports)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending reports: %w", err)
	}

	// Total revenue
	err = dbTx.QueryRow(ctx,
		"SELECT COALESCE(SUM(commission_amount), 0) FROM orders WHERE status = 'completed'",
	).Scan(&metrics.TotalRevenue)
	if err != nil {
		return nil, fmt.Errorf("failed to query total revenue: %w", err)
	}

	return &metrics, nil
}

// ListAuditLogs returns a paginated list of audit logs with filters applied.
func (r *AdminRepositoryImpl) ListAuditLogs(
	ctx context.Context,
	tx interface{},
	filters repository.AuditLogFilters,
) ([]repository.AuditLogEntry, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}

	baseQuery := `
		SELECT id, actor_id, action_type, target_type, target_id, metadata, created_at
		FROM admin_audit_logs
		WHERE 1=1
	`

	args := []interface{}{}
	argNum := 1

	// Build filters
	if filters.AdminID != nil {
		baseQuery += fmt.Sprintf(" AND actor_id = $%d", argNum)
		args = append(args, *filters.AdminID)
		argNum++
	}

	if filters.Action != "" {
		baseQuery += fmt.Sprintf(" AND action_type = $%d", argNum)
		args = append(args, filters.Action)
		argNum++
	}

	if filters.TargetType != "" {
		baseQuery += fmt.Sprintf(" AND target_type = $%d", argNum)
		args = append(args, filters.TargetType)
		argNum++
	}

	if filters.TargetID != nil {
		baseQuery += fmt.Sprintf(" AND target_id = $%d", argNum)
		args = append(args, *filters.TargetID)
		argNum++
	}

	// Add sorting and pagination (always sort by created_at DESC)
	baseQuery += " ORDER BY created_at DESC"
	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argNum, argNum+1)
	args = append(args, filters.PageSize, (filters.Page-1)*filters.PageSize)

	// Execute query
	rows, err := dbTx.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	logs := []repository.AuditLogEntry{}
	for rows.Next() {
		log, err := r.scanAuditLogEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log row: %w", err)
		}
		logs = append(logs, *log)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("list audit logs scan failed: %w", rows.Err())
	}

	return logs, nil
}

// CountAuditLogs returns the total count of audit logs matching filters.
func (r *AdminRepositoryImpl) CountAuditLogs(
	ctx context.Context,
	tx interface{},
	filters repository.AuditLogFilters,
) (int, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return 0, fmt.Errorf("invalid transaction type")
	}

	countQuery := `SELECT COUNT(*) FROM admin_audit_logs WHERE 1=1`

	args := []interface{}{}
	argNum := 1

	// Build filters
	if filters.AdminID != nil {
		countQuery += fmt.Sprintf(" AND actor_id = $%d", argNum)
		args = append(args, *filters.AdminID)
		argNum++
	}

	if filters.Action != "" {
		countQuery += fmt.Sprintf(" AND action_type = $%d", argNum)
		args = append(args, filters.Action)
		argNum++
	}

	if filters.TargetType != "" {
		countQuery += fmt.Sprintf(" AND target_type = $%d", argNum)
		args = append(args, filters.TargetType)
		argNum++
	}

	if filters.TargetID != nil {
		countQuery += fmt.Sprintf(" AND target_id = $%d", argNum)
		args = append(args, *filters.TargetID)
		argNum++
	}

	var total int
	err := dbTx.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	return total, nil
}

// buildWhereClauses constructs WHERE clauses for user queries.
func (r *AdminRepositoryImpl) buildWhereClauses(
	filters repository.UserListFilters,
	argNum *int,
	args *[]interface{},
) []string {
	whereClauses := []string{}

	if filters.SearchQuery != "" {
		whereClauses = append(whereClauses, fmt.Sprintf(
			"(u.email ILIKE $%d OR up.username ILIKE $%d)",
			*argNum, *argNum+1,
		))
		searchPattern := "%" + filters.SearchQuery + "%"
		*args = append(*args, searchPattern, searchPattern)
		*argNum += 2
	}

	if filters.Status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("u.account_status = $%d", *argNum))
		*args = append(*args, filters.Status)
		*argNum++
	}

	if filters.Role != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("u.role = $%d", *argNum))
		*args = append(*args, filters.Role)
		*argNum++
	}

	if filters.IsAdmin != nil {
		if *filters.IsAdmin {
			whereClauses = append(whereClauses, "u.role = 'admin'")
		} else {
			whereClauses = append(whereClauses, "u.role != 'admin'")
		}
	}

	if filters.IsSuspended != nil {
		if *filters.IsSuspended {
			whereClauses = append(whereClauses, "u.account_status = 'suspended'")
		} else {
			whereClauses = append(whereClauses, "u.account_status != 'suspended'")
		}
	}

	return whereClauses
}

// validateSortField validates the sort field.
func (r *AdminRepositoryImpl) validateSortField(sortBy string) string {
	validSortFields := map[string]bool{
		"created_at": true, "updated_at": true, "email": true,
		"username": true, "account_status": true,
	}
	if !validSortFields[sortBy] {
		return "created_at"
	}
	return sortBy
}

// validateSortOrder validates the sort order.
func (r *AdminRepositoryImpl) validateSortOrder(sortOrder string) string {
	if sortOrder != "asc" && sortOrder != "desc" {
		return "desc"
	}
	return sortOrder
}

// scanUserSummary scans a UserSummary from a database row.
func (r *AdminRepositoryImpl) scanUserSummary(rows pgx.Rows) (*repository.UserSummary, error) {
	var u repository.UserSummary
	var username pgtype.Text
	var phoneNumber pgtype.Text
	var isVerified pgtype.Bool
	var coinBalance pgtype.Int8

	// up.is_verified is NOT NULL on user_profiles itself, but this is a
	// LEFT JOIN — a user with no profile row at all (e.g. a directly
	// seeded/bootstrapped admin account) yields NULL for every up.*
	// column, so it must be scanned as nullable like username/phoneNumber.
	err := rows.Scan(
		&u.ID, &u.FirebaseUID, &u.Email, &phoneNumber,
		&u.EmailVerified, &u.PhoneVerified, &u.AccountStatus,
		&u.Role, &u.CreatedAt, &u.UpdatedAt,
		&username, &isVerified,
		&coinBalance,
	)

	if err != nil {
		return nil, err
	}

	u.Username = stringPtrFromPgtypeText(username)
	u.PhoneNumber = stringPtrFromPgtypeText(phoneNumber)
	u.IsVerified = isVerified.Valid && isVerified.Bool
	u.CoinBalance = int64PtrFromPgtypeInt8(coinBalance)

	return &u, nil
}

// scanAuditLogEntry scans an AuditLogEntry from a database row.
func (r *AdminRepositoryImpl) scanAuditLogEntry(rows pgx.Rows) (*repository.AuditLogEntry, error) {
	var log repository.AuditLogEntry
	var metadata []byte

	err := rows.Scan(
		&log.ID, &log.ActorID, &log.ActionType, &log.TargetType, &log.TargetID,
		&metadata, &log.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Parse JSONB metadata - for now leave as nil
	// The handler/service layer will handle JSON unmarshaling if needed
	log.Metadata = nil

	return &log, nil
}

// Helper functions for null handling

func stringPtrFromPgtypeText(t pgtype.Text) *string {
	if t.Valid {
		return &t.String
	}
	return nil
}

func intPtrFromPgtypeInt8(i pgtype.Int8) *int {
	if i.Valid {
		val := int(i.Int64)
		return &val
	}
	return nil
}

func int64PtrFromPgtypeInt8(i pgtype.Int8) *int64 {
	if i.Valid {
		return &i.Int64
	}
	return nil
}
