package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/pkg/db"
)

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

// WhitelistAuditRepository is the persistence contract for whitelist audit logs.
// All methods are read or append-only; no update/delete paths exist.
type WhitelistAuditRepository interface {
	// Append writes a single audit record. Fail-closed: returns error on DB failure.
	Append(ctx context.Context, rec WhitelistAuditRecord) error
	// List returns paginated audit records ordered by created_at DESC.
	List(ctx context.Context, limit, offset int) ([]WhitelistAuditRecord, error)
	// ListBySeller returns audit records for a specific seller, ordered by created_at DESC.
	ListBySeller(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]WhitelistAuditRecord, error)
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

func (r *WhitelistAuditRepositoryImpl) List(ctx context.Context, limit, offset int) ([]WhitelistAuditRecord, error) {
	const q = `
		SELECT id, seller_id, action, actor_id, reason, source, metadata, created_at
		FROM payout_whitelist_audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Pool().Query(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

func (r *WhitelistAuditRepositoryImpl) ListBySeller(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]WhitelistAuditRecord, error) {
	const q = `
		SELECT id, seller_id, action, actor_id, reason, source, metadata, created_at
		FROM payout_whitelist_audit_logs
		WHERE seller_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool().Query(ctx, q, sellerID, limit, offset)
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


