package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/db"
)

const whitelistAuditColumns = `
		id, seller_id, action, actor_id, reason, source, metadata, created_at
	FROM payout_whitelist_audit_logs`

// WhitelistAuditRecord mirrors a row in payout_whitelist_audit_logs.
type WhitelistAuditRecord struct {
	ID        uuid.UUID  `json:"id"`
	SellerID  *uuid.UUID `json:"seller_id,omitempty"` // nil for WHITELIST_INITIALIZED
	Action    string     `json:"action"`
	ActorID   string     `json:"actor_id"`
	Reason    string     `json:"reason"`
	Source    string     `json:"source"`
	Metadata  []byte     `json:"metadata,omitempty"` // raw JSONB bytes
	CreatedAt time.Time  `json:"created_at"`
}

// WhitelistAuditCursor is the keyset position over the append-only whitelist
// audit log. The log is an immutable, append-only compliance trail ordered by
// (created_at DESC, id DESC); pagination is keyset (NO OFFSET) so newly
// appended rows at the head never shift the window under the reviewer.
type WhitelistAuditCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// WhitelistAuditRepository is the persistence contract for whitelist audit logs.
// All methods are read or append-only; no update/delete paths exist.
type WhitelistAuditRepository interface {
	// Append writes a single audit record. Fail-closed: returns error on DB failure.
	Append(ctx context.Context, rec WhitelistAuditRecord) error
	// List returns a keyset page of audit records ordered by created_at DESC, id DESC.
	// A nil cursor returns the newest page.
	List(ctx context.Context, limit int, cursor *WhitelistAuditCursor) ([]WhitelistAuditRecord, error)
	// ListBySeller returns a keyset page for a specific seller, ordered by created_at DESC, id DESC.
	ListBySeller(ctx context.Context, sellerID uuid.UUID, limit int, cursor *WhitelistAuditCursor) ([]WhitelistAuditRecord, error)
}

// WhitelistAuditRepositoryImpl is the PostgreSQL implementation.
type WhitelistAuditRepositoryImpl struct {
	db *db.DB
}

// NewWhitelistAuditRepository creates a repository backed by the given DB pool.
func NewWhitelistAuditRepository(database *db.DB) WhitelistAuditRepository {
	return &WhitelistAuditRepositoryImpl{db: database}
}

func (r *WhitelistAuditRepositoryImpl) Append(ctx context.Context, rec WhitelistAuditRecord) error {
	const q = `
		INSERT INTO payout_whitelist_audit_logs
			(seller_id, action, actor_id, reason, source, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	var metadata interface{}
	if len(rec.Metadata) > 0 {
		metadata = rec.Metadata
	}

	_, err := r.db.Pool().Exec(ctx, q,
		rec.SellerID,
		rec.Action,
		rec.ActorID,
		rec.Reason,
		rec.Source,
		metadata,
		rec.CreatedAt,
	)
	return err
}

func (r *WhitelistAuditRepositoryImpl) List(ctx context.Context, limit int, cursor *WhitelistAuditCursor) ([]WhitelistAuditRecord, error) {
	query := `SELECT` + whitelistAuditColumns
	var args []interface{}
	if cursor != nil {
		query += ` WHERE (created_at, id) < ($1, $2)`
		args = append(args, cursor.CreatedAt, cursor.ID)
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

func (r *WhitelistAuditRepositoryImpl) ListBySeller(ctx context.Context, sellerID uuid.UUID, limit int, cursor *WhitelistAuditCursor) ([]WhitelistAuditRecord, error) {
	query := `SELECT` + whitelistAuditColumns + ` WHERE seller_id = $1`
	args := []interface{}{sellerID}
	if cursor != nil {
		query += fmt.Sprintf(` AND (created_at, id) < ($%d, $%d)`, len(args)+1, len(args)+2)
		args = append(args, cursor.CreatedAt, cursor.ID)
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := r.db.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

func scanAuditRows(rows pgx.Rows) ([]WhitelistAuditRecord, error) {
	var out []WhitelistAuditRecord
	for rows.Next() {
		var rec WhitelistAuditRecord
		var metadata []byte
		if err := rows.Scan(
			&rec.ID,
			&rec.SellerID,
			&rec.Action,
			&rec.ActorID,
			&rec.Reason,
			&rec.Source,
			&metadata,
			&rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		rec.Metadata = metadata
		out = append(out, rec)
	}
	return out, rows.Err()
}


