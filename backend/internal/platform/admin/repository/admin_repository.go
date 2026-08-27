// Package repository provides the admin repository interface.
package repository

import (
	"context"

	"github.com/google/uuid"
)

// UserListFilters represents filters for listing users.
type UserListFilters struct {
	SearchQuery string
	Status      string
	Role        string
	IsAdmin     *bool
	IsSuspended *bool
	SortBy      string
	SortOrder   string
	Page        int
	PageSize    int
}

// UserSummary represents a simplified user for list views.
type UserSummary struct {
	ID            uuid.UUID
	FirebaseUID   string
	Email         string
	PhoneNumber   *string
	EmailVerified bool
	PhoneVerified bool
	AccountStatus string
	Role          string
	Username      *string
	IsVerified    bool
	CoinBalance   *int64
	CreatedAt     interface{}
	UpdatedAt     interface{}
}

// UserDetails represents a complete user with all information.
type UserDetails struct {
	ID             uuid.UUID
	FirebaseUID    string
	Email          string
	PhoneNumber    *string
	EmailVerified  bool
	PhoneVerified  bool
	AccountStatus  string
	Role           string
	Username       *string
	Bio            *string
	AvatarURL      *string
	IsVerified     bool
	FollowersCount *int
	FollowingCount *int
	City           *string
	Province       *string
	CoinBalance    *int64
	CreatedAt      interface{}
	UpdatedAt      interface{}

	// Seller authority fields (Batch 52: canonical seller state visibility)
	HasSellerProfile   bool    // seller_profiles row exists
	SubscriptionStatus *string // seller_subscriptions.status (active/expired)
	VerificationStatus *string // seller_verifications.status (8 states)
	SellerPayable      *int64  // financial_accounts balance where account_type='SELLER_PAYABLE'
	SellerTier         *string // seller_reputation_state.current_tier (basic/pro/elite); nil if no row

	// RecoverableSubscriptionPaymentID is the UUID of a settled subscription payment
	// that has no corresponding seller_subscriptions row (webhook miss scenario).
	// Non-nil only when such a payment exists and recovery is possible.
	RecoverableSubscriptionPaymentID *uuid.UUID

	// Warning counts (governance visibility, read-only).
	// WarningCount is the total number of warnings ever issued.
	// ActiveWarningCount is warnings where is_active=true AND (expires_at IS NULL OR expires_at > now()).
	// SevereWarningCount is active warnings with level='severe'.
	WarningCount       int
	ActiveWarningCount int
	SevereWarningCount int
}

// DashboardMetrics represents platform metrics.
type DashboardMetrics struct {
	TotalUsers       int64
	ActiveUsersToday int64
	ActiveSellers    int64
	TotalOrders      int64
	OrdersToday      int64
	PendingReports   int64
	TotalRevenue     int64
}

// AuditLogEntry represents a single audit log entry.
type AuditLogEntry struct {
	ID         uuid.UUID
	ActorID    uuid.UUID
	ActionType string
	TargetType string
	TargetID   uuid.UUID
	Metadata   map[string]interface{}
	CreatedAt  interface{}
}

// AuditLogFilters represents filters for listing audit logs.
type AuditLogFilters struct {
	AdminID    *uuid.UUID
	Action     string
	TargetType string
	TargetID   *uuid.UUID
	Page       int
	PageSize   int
}

// AdminRepository defines the interface for admin data operations.
//
// DESIGN PRINCIPLES:
// - All database operations are abstracted behind this interface
// - Repository does NOT contain business logic
// - Transactions use interface{} tx parameter for flexibility
type AdminRepository interface {
	// ListUsers returns a paginated list of users with filters applied.
	ListUsers(ctx context.Context, tx interface{}, filters UserListFilters) ([]UserSummary, error)

	// CountUsers returns the total count of users matching filters.
	CountUsers(ctx context.Context, tx interface{}, filters UserListFilters) (int, error)

	// GetUserDetails returns detailed information about a specific user.
	GetUserDetails(ctx context.Context, tx interface{}, userID uuid.UUID) (*UserDetails, error)

	// GetUserStatus returns the current account_status of a user.
	GetUserStatus(ctx context.Context, tx interface{}, userID uuid.UUID) (string, error)

	// UpdateUserStatus updates the account_status of a user.
	UpdateUserStatus(ctx context.Context, tx interface{}, userID uuid.UUID, status string) error

	// GetDashboardMetrics returns platform metrics.
	GetDashboardMetrics(ctx context.Context, tx interface{}) (*DashboardMetrics, error)

	// ListAuditLogs returns a paginated list of audit logs with filters applied.
	ListAuditLogs(ctx context.Context, tx interface{}, filters AuditLogFilters) ([]AuditLogEntry, error)

	// CountAuditLogs returns the total count of audit logs matching filters.
	CountAuditLogs(ctx context.Context, tx interface{}, filters AuditLogFilters) (int, error)
}
