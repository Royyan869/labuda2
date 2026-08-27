// Package audit provides admin audit logging functionality.
// All sensitive admin operations (withdrawals, disputes, role changes, etc.)
// are logged here for accountability and compliance.
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/pkg/db"
)

// SystemCallerID is a special UUID used for system-initiated operations.
// Operations performed by the system caller (workers, cron jobs, internal processes)
// should NOT generate admin audit logs.
// This is defined here to avoid circular import with auth package.
var SystemCallerID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// IsSystemCaller returns true if the callerID is the system caller.
func IsSystemCaller(callerID uuid.UUID) bool {
	return callerID == SystemCallerID
}

// AdminAuditLogger defines interface for logging admin actions.
type AdminAuditLogger interface {
	// Log records an admin action. Logging failure should not break the main operation.
	Log(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error

	// LogSafe records an admin action and ignores any errors.
	// Use this when you want best-effort logging that never fails the calling operation.
	LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{})

	// LogTx records an admin action within a transaction.
	// If logging fails, the transaction will be rolled back.
	// Use this for atomic audit logging where the audit trail must succeed for the operation to succeed.
	LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error
}

// AdminAuditLoggerDB implements AdminAuditLogger using PostgreSQL.
type AdminAuditLoggerDB struct {
	pool *pgxpool.Pool
}

// NewAdminAuditLoggerDB creates a new database-backed admin audit logger.
func NewAdminAuditLoggerDB(pool *pgxpool.Pool) *AdminAuditLoggerDB {
	return &AdminAuditLoggerDB{pool: pool}
}

// Log records an admin action to the database.
// This is a best-effort operation: failures are logged but do not return errors.
// The audit log should not block critical business operations.
func (l *AdminAuditLoggerDB) Log(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	// Skip logging for system caller - system operations don't generate admin audit logs
	if IsSystemCaller(actorID) {
		return nil
	}

	// Convert metadata to JSONB
	var metadataJSON []byte
	if metadata != nil && len(metadata) > 0 {
		var err error
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			// If metadata serialization fails, log without metadata
			metadataJSON = nil
		}
	}

	query := `
		INSERT INTO admin_audit_logs (actor_id, action_type, target_type, target_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := l.pool.Exec(ctx, query, actorID, actionType, targetType, targetID, metadataJSON)
	if err != nil {
		// Return error wrapped but don't panic - caller can decide whether to log it
		return fmt.Errorf("failed to write admin audit log: %w", err)
	}

	return nil
}

// LogSafe is a convenience method that logs an admin action and ignores any errors.
// Use this when you want best-effort logging that never fails the calling operation.
func (l *AdminAuditLoggerDB) LogSafe(ctx context.Context, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) {
	_ = l.Log(ctx, actorID, actionType, targetType, targetID, metadata)
}

// LogTx records an admin action within a transaction.
// If logging fails, the transaction will be rolled back.
// This ensures atomic audit logging - the audit trail is guaranteed to exist for completed operations.
func (l *AdminAuditLoggerDB) LogTx(ctx context.Context, tx db.Tx, actorID uuid.UUID, actionType string, targetType string, targetID uuid.UUID, metadata map[string]interface{}) error {
	// Skip logging for system caller - system operations don't generate admin audit logs
	if IsSystemCaller(actorID) {
		return nil
	}

	// Convert metadata to JSONB
	var metadataJSON []byte
	if metadata != nil && len(metadata) > 0 {
		var err error
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			// If metadata serialization fails, log without metadata
			metadataJSON = nil
		}
	}

	query := `
		INSERT INTO admin_audit_logs (actor_id, action_type, target_type, target_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := tx.Exec(ctx, query, actorID, actionType, targetType, targetID, metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to write admin audit log: %w", err)
	}

	return nil
}

// Action types for admin audit logs
const (
	// Withdrawal actions
	ActionWithdrawApproved     = "withdraw_approved"
	ActionWithdrawRejected     = "withdraw_rejected"
	ActionWithdrawProcessed    = "withdraw_processed"

	// Dispute actions
	ActionDisputeResolvedApproved     = "dispute_resolved_approved"
	ActionDisputeResolvedRejected     = "dispute_resolved_rejected"
	ActionDisputeResolvedPartialSplit = "dispute_resolved_partial_split"

	// Role actions
	ActionRoleChanged = "role_changed"

	// Account status actions
	ActionAccountStatusChanged = "account_status_changed"

	// Refund actions
	ActionRefundGatewayInitiated = "refund_gateway_initiated"

	// Auction actions
	ActionAuctionAdminCancelled = "auction_admin_cancelled"
)

// Target types for admin audit logs
const (
	TargetTypeUser       = "user"
	TargetTypeWithdrawal = "withdrawal"
	TargetTypeDispute    = "dispute"
	TargetTypeRefund     = "refund"
	TargetTypeAuction    = "auction"
)


