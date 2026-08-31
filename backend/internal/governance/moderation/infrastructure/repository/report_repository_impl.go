package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/governance/moderation/entity"
	"github.com/labuda/backend/pkg/db"
)

// ReportRepositoryImpl is the canonical Report persistence using pgx.
type ReportRepositoryImpl struct{}

// NewReportRepository creates the canonical Report repository.
func NewReportRepository() ReportRepository {
	return &ReportRepositoryImpl{}
}

// reportColumns is the canonical column list for reports queries.
// Keep SELECT and Scan in sync.
const reportColumns = `id, reporter_id, subject_type, subject_id, reason_code,
		       reason_note, evidence_snapshot, case_id, created_at`

// Create inserts a new immutable Report.
//
// Duplicate protection: the unique index uniq_reports_one_per_reporter_subject
// is the race-safe final guard. Under concurrent inserts for the same
// (reporter_id, subject_type, subject_id), exactly one INSERT succeeds; the
// other fails with 23505 and is translated to ErrDuplicateReport. This is
// stronger than SELECT-then-INSERT, which is not race-safe.
func (r *ReportRepositoryImpl) Create(ctx context.Context, tx db.Tx, report *entity.Report) error {
	var evidenceBytes []byte
	if report.EvidenceSnapshot != nil {
		b, err := report.EvidenceSnapshot.MarshalJSON()
		if err != nil {
			return fmt.Errorf("marshal report evidence snapshot failed: %w", err)
		}
		evidenceBytes = b
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO reports (
			id, reporter_id, subject_type, subject_id,
			reason_code, reason_note, evidence_snapshot, case_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		report.ID,
		report.ReporterID,
		string(report.SubjectType),
		report.SubjectID,
		string(report.ReasonCode),
		report.ReasonNote,
		evidenceBytes,
		report.CaseID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return &ErrDuplicateReport{
				ReporterID:  report.ReporterID,
				SubjectType: report.SubjectType,
				SubjectID:   report.SubjectID,
			}
		}
		return fmt.Errorf("create report failed: %w", err)
	}
	return nil
}

// ValidateTarget validates that a subject exists in its canonical target
// domain table AND returns the minimal immutable evidence snapshot of the
// subject at report time (Business Truth §23: snapshot metadata so governance
// history does not depend on the live object).
//
// Target lifecycle is NOT modified by Report intake (canonical spec §9).
// Lifecycle-aware guards mirror the legacy ResourceExists behavior:
//   - content / comment / user: soft-delete pattern — a soft-deleted subject
//     is not reportable (guarded by deleted_at IS NULL).
//   - for_sale / auction: status-based lifecycle (no deleted_at). A withdrawn,
//     sold, or ended listing remains reportable as governance history
//     (Business Truth §26: reports remain valid after state changes).
func (r *ReportRepositoryImpl) ValidateTarget(ctx context.Context, tx db.Tx, subjectType entity.ReportTargetType, subjectID uuid.UUID) (*entity.EvidenceSnapshot, error) {
	switch subjectType {
	case entity.ReportTargetContent:
		return r.validateContent(ctx, tx, subjectID)
	case entity.ReportTargetComment:
		return r.validateComment(ctx, tx, subjectID)
	case entity.ReportTargetForSale:
		return r.validateForSale(ctx, tx, subjectID)
	case entity.ReportTargetAuction:
		return r.validateAuction(ctx, tx, subjectID)
	case entity.ReportTargetUser:
		return r.validateUser(ctx, tx, subjectID)
	default:
		return nil, &ErrReportTargetNotFound{SubjectType: subjectType, SubjectID: subjectID}
	}
}

// validateContent validates a contents row and builds its evidence snapshot.
func (r *ReportRepositoryImpl) validateContent(ctx context.Context, tx db.Tx, contentID uuid.UUID) (*entity.EvidenceSnapshot, error) {
	query := `
		SELECT
			c.author_id::text,
			COALESCE(p.username, ''),
			COALESCE(c.caption, ''),
			CASE WHEN c.original_author_id IS NOT NULL THEN 'repost' ELSE 'post' END,
			c.visibility::text,
			c.deleted_at IS NOT NULL
		FROM contents c
		LEFT JOIN user_profiles p ON p.user_id = c.author_id
		WHERE c.id = $1 AND c.deleted_at IS NULL
		LIMIT 1
	`
	return scanEvidence(ctx, query, tx, contentID)
}

// validateComment validates a comments row and builds its evidence snapshot.
func (r *ReportRepositoryImpl) validateComment(ctx context.Context, tx db.Tx, commentID uuid.UUID) (*entity.EvidenceSnapshot, error) {
	query := `
		SELECT
			c.author_id::text,
			COALESCE(p.username, ''),
			COALESCE(c.body, ''),
			COALESCE(c.body, ''),
			CASE WHEN ccr.comment_id IS NULL THEN 'normal' ELSE 'commerce_reference' END,
			c.deleted_at IS NOT NULL
		FROM comments c
		LEFT JOIN user_profiles p ON p.user_id = c.author_id
		LEFT JOIN comment_commerce_references ccr ON ccr.comment_id = c.id
		WHERE c.id = $1 AND c.deleted_at IS NULL
		LIMIT 1
	`
	return scanEvidence(ctx, query, tx, commentID)
}

// validateForSale validates a for_sales row and builds its evidence snapshot.
// Status-based lifecycle: no deleted_at guard. A withdrawn/sold listing
// remains reportable (governance history).
func (r *ReportRepositoryImpl) validateForSale(ctx context.Context, tx db.Tx, forSaleID uuid.UUID) (*entity.EvidenceSnapshot, error) {
	query := `
		SELECT
			fps.seller_id::text,
			COALESCE(p.username, ''),
			COALESCE(prod.title, ''),
			COALESCE(prod.description, ''),
			fps.status::text,
			false
		FROM for_sales fps
		JOIN products prod ON prod.id = fps.product_id
		LEFT JOIN user_profiles p ON p.user_id = fps.seller_id
		WHERE fps.id = $1
		LIMIT 1
	`
	return scanEvidence(ctx, query, tx, forSaleID)
}

// validateAuction validates an auctions row and builds its evidence snapshot.
// Status-based lifecycle: no deleted_at guard.
func (r *ReportRepositoryImpl) validateAuction(ctx context.Context, tx db.Tx, auctionID uuid.UUID) (*entity.EvidenceSnapshot, error) {
	query := `
		SELECT
			a.seller_id::text,
			COALESCE(p.username, ''),
			COALESCE(prod.title, ''),
			COALESCE(prod.description, ''),
			a.status::text,
			false
		FROM auctions a
		JOIN products prod ON prod.id = a.product_id
		LEFT JOIN user_profiles p ON p.user_id = a.seller_id
		WHERE a.id = $1
		LIMIT 1
	`
	return scanEvidence(ctx, query, tx, auctionID)
}

// validateUser validates a users row and builds its evidence snapshot.
func (r *ReportRepositoryImpl) validateUser(ctx context.Context, tx db.Tx, userID uuid.UUID) (*entity.EvidenceSnapshot, error) {
	query := `
		SELECT
			u.id::text,
			COALESCE(p.username, ''),
			'',
			'',
			u.account_status::text,
			u.deleted_at IS NOT NULL
		FROM users u
		LEFT JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
		LIMIT 1
	`
	return scanEvidence(ctx, query, tx, userID)
}

// scanEvidence scans a target evidence row:
// (author_id, author_username, title, text, status, is_deleted).
func scanEvidence(ctx context.Context, query string, tx db.Tx, subjectID uuid.UUID) (*entity.EvidenceSnapshot, error) {
	var authorID, authorUsername, title, text, status string
	var isDeleted bool

	err := tx.QueryRow(ctx, query, subjectID).Scan(
		&authorID, &authorUsername, &title, &text, &status, &isDeleted,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &ErrReportTargetNotFound{SubjectID: subjectID}
		}
		return nil, fmt.Errorf("validate report target failed: %w", err)
	}

	// Truncate long text to keep the snapshot minimal.
	if len(text) > 500 {
		text = text[:500]
	}

	return &entity.EvidenceSnapshot{
		AuthorID:       authorID,
		AuthorUsername: authorUsername,
		Title:          title,
		Text:           text,
		Status:         status,
		IsDeleted:      isDeleted,
	}, nil
}


// GetByID retrieves a Report. Returns nil when not found.
func (r *ReportRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, reportID uuid.UUID) (*entity.Report, error) {
	query := `SELECT ` + reportColumns + `
		FROM reports
		WHERE id = $1`

	row := tx.QueryRow(ctx, query, reportID)
	report, err := scanReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get report failed: %w", err)
	}
	return report, nil
}

// ListByReporter retrieves reports created by a reporter, newest first.
func (r *ReportRepositoryImpl) ListByReporter(ctx context.Context, tx db.Tx, reporterID uuid.UUID, limit, offset int) ([]*entity.Report, error) {
	query := `SELECT ` + reportColumns + `
		FROM reports
		WHERE reporter_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := tx.Query(ctx, query, reporterID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list reports by reporter failed: %w", err)
	}
	defer rows.Close()

	var reports []*entity.Report
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, fmt.Errorf("scan report row failed: %w", err)
		}
		reports = append(reports, report)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate report rows failed: %w", rows.Err())
	}
	return reports, nil
}

// HasUserReported returns true if the reporter already reported the subject.
func (r *ReportRepositoryImpl) HasUserReported(ctx context.Context, tx db.Tx, reporterID uuid.UUID, subjectType entity.ReportTargetType, subjectID uuid.UUID) (bool, error) {
	var exists int
	err := tx.QueryRow(ctx, `
		SELECT 1
		FROM reports
		WHERE reporter_id = $1
		  AND subject_type = $2
		  AND subject_id = $3
		LIMIT 1
	`, reporterID, string(subjectType), subjectID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check existing report failed: %w", err)
	}
	return true, nil
}

// reportScanner is satisfied by pgx.Row and pgx.Rows.
type reportScanner interface {
	Scan(dest ...any) error
}

// scanReport scans one canonical Report row.
// Column order must match reportColumns.
func scanReport(row reportScanner) (*entity.Report, error) {
	var id, reporterID, subjectID uuid.UUID
	var subjectType, reasonCode string
	var reasonNote *string
	var evidenceBytes []byte
	var caseID *uuid.UUID
	var createdAt time.Time

	err := row.Scan(
		&id, &reporterID, &subjectType, &subjectID, &reasonCode,
		&reasonNote, &evidenceBytes, &caseID, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	report := &entity.Report{
		ID:          id,
		ReporterID:  reporterID,
		SubjectType: entity.ReportTargetType(subjectType),
		SubjectID:   subjectID,
		ReasonCode:  entity.ReportReasonCode(reasonCode),
		ReasonNote:  reasonNote,
		CaseID:      caseID,
		CreatedAt:   createdAt,
	}

	if len(evidenceBytes) > 0 {
		var snapshot entity.EvidenceSnapshot
		if err := snapshot.UnmarshalJSON(evidenceBytes); err != nil {
			return nil, fmt.Errorf("decode report evidence snapshot failed: %w", err)
		}
		report.EvidenceSnapshot = &snapshot
	}

	return report, nil
}
