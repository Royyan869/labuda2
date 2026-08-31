package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// CaseRepositoryImpl is the canonical Case persistence using pgx.
type CaseRepositoryImpl struct{}

// NewCaseRepository creates the canonical Case repository.
func NewCaseRepository() CaseRepository {
	return &CaseRepositoryImpl{}
}

// caseColumns is the canonical column list for cases queries.
// Keep SELECT and Scan in sync.
const caseColumns = `id, subject_type, subject_id, status, created_at, closed_at, updated_at`

// FindOrCreateOpenCase finds an existing open Case for the subject,
// or creates a new one if none exists.
//
// Race safety: Uses INSERT ... ON CONFLICT ... DO NOTHING to avoid
// transaction abort on 23505. Under concurrent requests:
//   - First request: INSERT succeeds (RowsAffected=1)
//   - Concurrent request: INSERT DO NOTHING (RowsAffected=0) → SELECT finds existing
//
// The partial unique index uniq_active_case_per_subject is the final guard.
func (r *CaseRepositoryImpl) FindOrCreateOpenCase(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID) (*entity.CanonicalCase, error) {
	caseID := uuid.New()
	now := time.Now().UTC()

	// Atomic: try to insert. If a row already exists (same subject, open status),
	// the unique index rejects the INSERT but ON CONFLICT DO NOTHING keeps
	// the transaction valid (no 23505 abort).
	result, err := tx.Exec(ctx, `
		INSERT INTO cases (id, subject_type, subject_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'open', $4, $5)
		ON CONFLICT (subject_type, subject_id) WHERE status = 'open'
		DO NOTHING
	`, caseID, string(subjectType), subjectID, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert or skip case failed: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 1 {
		// We created the Case — return it directly
		return &entity.CanonicalCase{
			ID:          caseID,
			SubjectType: subjectType,
			SubjectID:   subjectID,
			Status:      entity.CaseStatusOpen,
			CreatedAt:   now,
			UpdatedAt:   now,
		}, nil
	}

	// RowsAffected == 0: another transaction created the Case.
	// SELECT to retrieve it (transaction is still valid — no abort).
	kase, err := r.findOpenCase(ctx, tx, subjectType, subjectID)
	if err != nil {
		return nil, fmt.Errorf("find existing case after conflict failed: %w", err)
	}
	if kase == nil {
		return nil, fmt.Errorf("case not found after conflict resolution: subject_type=%s subject_id=%s", subjectType, subjectID)
	}
	return kase, nil
}

// findOpenCase retrieves the open Case for a subject, or nil if none exists.
func (r *CaseRepositoryImpl) findOpenCase(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID) (*entity.CanonicalCase, error) {
	query := `SELECT ` + caseColumns + `
		FROM cases
		WHERE subject_type = $1 AND subject_id = $2 AND status = 'open'
		LIMIT 1`

	row := tx.QueryRow(ctx, query, string(subjectType), subjectID)
	kase, err := scanCase(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find open case query failed: %w", err)
	}
	return kase, nil
}


// GetByID retrieves a Case by its ID. Returns (nil, nil) when not found.
func (r *CaseRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, caseID uuid.UUID) (*entity.CanonicalCase, error) {
	query := `SELECT ` + caseColumns + `
		FROM cases
		WHERE id = $1`

	row := tx.QueryRow(ctx, query, caseID)
	kase, err := scanCase(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get case failed: %w", err)
	}
	return kase, nil
}

// ListBySubject retrieves all Cases for a subject, ordered by created_at DESC.
func (r *CaseRepositoryImpl) ListBySubject(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID, limit, offset int) ([]*entity.CanonicalCase, error) {
	query := `SELECT ` + caseColumns + `
		FROM cases
		WHERE subject_type = $1 AND subject_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := tx.Query(ctx, query, string(subjectType), subjectID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list cases by subject failed: %w", err)
	}
	defer rows.Close()

	var cases []*entity.CanonicalCase
	for rows.Next() {
		kase, err := scanCaseRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan case row failed: %w", err)
		}
		cases = append(cases, kase)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate case rows failed: %w", rows.Err())
	}
	return cases, nil
}

// ResolveCase marks a Case as resolved.
func (r *CaseRepositoryImpl) ResolveCase(ctx context.Context, tx db.Tx, caseID uuid.UUID) error {
	now := time.Now().UTC()
	result, err := tx.Exec(ctx, `
		UPDATE cases
		SET status = 'resolved', closed_at = $1, updated_at = $2
		WHERE id = $3 AND status = 'open'
	`, now, now, caseID)
	if err != nil {
		return fmt.Errorf("resolve case failed: %w", err)
	}
	if result.RowsAffected() == 0 {
		return &entity.ErrCaseAlreadyResolved{CaseID: caseID, Status: entity.CaseStatusResolved}
	}
	return nil
}

// ListAll retrieves all Cases for admin governance view, ordered by created_at DESC.
func (r *CaseRepositoryImpl) ListAll(ctx context.Context, tx db.Tx, statusFilter *entity.CaseStatus, limit, offset int) ([]*entity.CanonicalCase, error) {
	var query string
	var args []interface{}

	if statusFilter != nil {
		query = `SELECT ` + caseColumns + `
			FROM cases
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3`
		args = []interface{}{string(*statusFilter), limit, offset}
	} else {
		query = `SELECT ` + caseColumns + `
			FROM cases
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all cases failed: %w", err)
	}
	defer rows.Close()

	var cases []*entity.CanonicalCase
	for rows.Next() {
		kase, err := scanCaseRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan case row failed: %w", err)
		}
		cases = append(cases, kase)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate case rows failed: %w", rows.Err())
	}
	return cases, nil
}

// CountAll returns the total number of Cases for admin pagination.
func (r *CaseRepositoryImpl) CountAll(ctx context.Context, tx db.Tx, statusFilter *entity.CaseStatus) (int, error) {
	var query string
	var args []interface{}

	if statusFilter != nil {
		query = `SELECT COUNT(*) FROM cases WHERE status = $1`
		args = []interface{}{string(*statusFilter)}
	} else {
		query = `SELECT COUNT(*) FROM cases`
		args = []interface{}{}
	}

	var count int
	err := tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count all cases failed: %w", err)
	}
	return count, nil
}

// caseScanner is satisfied by pgx.Row and pgx.Rows.
type caseScanner interface {
	Scan(dest ...any) error
}

// scanCase scans one canonical Case row.
// Column order must match caseColumns.
func scanCase(row caseScanner) (*entity.CanonicalCase, error) {
	var id, subjectID uuid.UUID
	var subjectType, status string
	var createdAt, updatedAt time.Time
	var closedAt *time.Time

	err := row.Scan(
		&id, &subjectType, &subjectID, &status,
		&createdAt, &closedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &entity.CanonicalCase{
		ID:          id,
		SubjectType: entity.ReportTargetType(subjectType),
		SubjectID:   subjectID,
		Status:      entity.CaseStatus(status),
		CreatedAt:   createdAt,
		ClosedAt:    closedAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// scanCaseRow scans one canonical Case row from pgx.Rows.
func scanCaseRow(rows pgx.Rows) (*entity.CanonicalCase, error) {
	var id, subjectID uuid.UUID
	var subjectType, status string
	var createdAt, updatedAt time.Time
	var closedAt *time.Time

	err := rows.Scan(
		&id, &subjectType, &subjectID, &status,
		&createdAt, &closedAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &entity.CanonicalCase{
		ID:          id,
		SubjectType: entity.ReportTargetType(subjectType),
		SubjectID:   subjectID,
		Status:      entity.CaseStatus(status),
		CreatedAt:   createdAt,
		ClosedAt:    closedAt,
		UpdatedAt:   updatedAt,
	}, nil
}
