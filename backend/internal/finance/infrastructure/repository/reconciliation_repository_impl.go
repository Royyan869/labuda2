package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/finance/entity"
	"github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/pkg/db"
)

// reconciliationResultRow represents a database row for reconciliation_results.
type reconciliationResultRow struct {
	ID                   uuid.UUID          `db:"id"`
	CheckedAt            time.Time          `db:"checked_at"`
	TotalAccounts        int                `db:"total_accounts"`
	MismatchedAccounts   int                `db:"mismatched_accounts"`
	Severity             string             `db:"severity"`
	Details              json.RawMessage    `db:"details"`
	ActionTaken          string             `db:"action_taken"`
	AutoRepaired         bool               `db:"auto_repaired"`
	DoubleCheckPassed    bool               `db:"double_check_passed"`
	CreatedAt            time.Time          `db:"created_at"`
}

// ReconciliationRepositoryImpl implements ReconciliationRepository for PostgreSQL.
type ReconciliationRepositoryImpl struct {
	db db.DB
}

// NewReconciliationRepository creates a new ReconciliationRepositoryImpl.
func NewReconciliationRepository(db db.DB) repository.ReconciliationRepository {
	return &ReconciliationRepositoryImpl{db: db}
}

// Create persists a new reconciliation result.
func (r *ReconciliationRepositoryImpl) Create(ctx context.Context, tx interface{}, result *entity.ReconciliationResult) error {
	query := `
		INSERT INTO reconciliation_results (
			id, checked_at, total_accounts, mismatched_accounts,
			severity, details, action_taken, auto_repaired, double_check_passed, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
	`

	dbTx, err := r.unwrapTx(tx)
	if err != nil {
		return err
	}

	detailsJSON := result.Details.ToJSON()

	_, err = dbTx.Exec(ctx, query,
		result.ID,
		result.CheckedAt,
		result.TotalAccounts,
		result.MismatchedAccounts,
		string(result.Severity),
		detailsJSON,
		string(result.ActionTaken),
		result.AutoRepaired,
		result.DoubleCheckPassed,
		result.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create reconciliation result: %w", err)
	}

	return nil
}

// GetByID retrieves a reconciliation result by ID.
func (r *ReconciliationRepositoryImpl) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.ReconciliationResult, error) {
	query := `
		SELECT id, checked_at, total_accounts, mismatched_accounts,
		       severity, details, action_taken, auto_repaired, double_check_passed, created_at
		FROM reconciliation_results
		WHERE id = $1
	`

	dbTx, err := r.unwrapTx(tx)
	if err != nil {
		return nil, err
	}

	var row reconciliationResultRow
	err = dbTx.QueryRow(ctx, query, id).Scan(
		&row.ID,
		&row.CheckedAt,
		&row.TotalAccounts,
		&row.MismatchedAccounts,
		&row.Severity,
		&row.Details,
		&row.ActionTaken,
		&row.AutoRepaired,
		&row.DoubleCheckPassed,
		&row.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("reconciliation result not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get reconciliation result: %w", err)
	}

	return r.rowToEntity(&row)
}

// List retrieves reconciliation results with filtering and pagination.
func (r *ReconciliationRepositoryImpl) List(ctx context.Context, tx interface{}, filters repository.ReconciliationFilters) ([]*entity.ReconciliationResult, error) {
	query := `
		SELECT id, checked_at, total_accounts, mismatched_accounts,
		       severity, details, action_taken, auto_repaired, double_check_passed, created_at
		FROM reconciliation_results
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filters.Severity != nil {
		query += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, string(*filters.Severity))
		argIdx++
	}

	if filters.ActionTaken != nil {
		query += fmt.Sprintf(" AND action_taken = $%d", argIdx)
		args = append(args, string(*filters.ActionTaken))
		argIdx++
	}

	if filters.AutoRepaired != nil {
		query += fmt.Sprintf(" AND auto_repaired = $%d", argIdx)
		args = append(args, *filters.AutoRepaired)
		argIdx++
	}

	if filters.DateFrom != nil {
		query += fmt.Sprintf(" AND checked_at >= $%d", argIdx)
		args = append(args, *filters.DateFrom)
		argIdx++
	}

	if filters.DateTo != nil {
		query += fmt.Sprintf(" AND checked_at <= $%d", argIdx)
		args = append(args, *filters.DateTo)
		argIdx++
	}

	query += " ORDER BY checked_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filters.Limit)
	}
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filters.Offset)
	}

	dbTx, err := r.unwrapTx(tx)
	if err != nil {
		return nil, err
	}

	rows, err := dbTx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list reconciliation results: %w", err)
	}
	defer rows.Close()

	var results []*entity.ReconciliationResult
	for rows.Next() {
		var row reconciliationResultRow
		err := rows.Scan(
			&row.ID,
			&row.CheckedAt,
			&row.TotalAccounts,
			&row.MismatchedAccounts,
			&row.Severity,
			&row.Details,
			&row.ActionTaken,
			&row.AutoRepaired,
			&row.DoubleCheckPassed,
			&row.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan reconciliation result row: %w", err)
		}

		result, err := r.rowToEntity(&row)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reconciliation results: %w", err)
	}

	return results, nil
}

// Count returns the total count of reconciliation results matching filters.
func (r *ReconciliationRepositoryImpl) Count(ctx context.Context, tx interface{}, filters repository.ReconciliationFilters) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM reconciliation_results
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filters.Severity != nil {
		query += fmt.Sprintf(" AND severity = $%d", argIdx)
		args = append(args, string(*filters.Severity))
		argIdx++
	}

	if filters.ActionTaken != nil {
		query += fmt.Sprintf(" AND action_taken = $%d", argIdx)
		args = append(args, string(*filters.ActionTaken))
		argIdx++
	}

	if filters.AutoRepaired != nil {
		query += fmt.Sprintf(" AND auto_repaired = $%d", argIdx)
		args = append(args, *filters.AutoRepaired)
		argIdx++
	}

	if filters.DateFrom != nil {
		query += fmt.Sprintf(" AND checked_at >= $%d", argIdx)
		args = append(args, *filters.DateFrom)
		argIdx++
	}

	if filters.DateTo != nil {
		query += fmt.Sprintf(" AND checked_at <= $%d", argIdx)
		args = append(args, *filters.DateTo)
		argIdx++
	}

	dbTx, err := r.unwrapTx(tx)
	if err != nil {
		return 0, err
	}

	var count int64
	err = dbTx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count reconciliation results: %w", err)
	}

	return count, nil
}

// GetLatest returns the most recent reconciliation result.
func (r *ReconciliationRepositoryImpl) GetLatest(ctx context.Context, tx interface{}) (*entity.ReconciliationResult, error) {
	query := `
		SELECT id, checked_at, total_accounts, mismatched_accounts,
		       severity, details, action_taken, auto_repaired, double_check_passed, created_at
		FROM reconciliation_results
		ORDER BY checked_at DESC
		LIMIT 1
	`

	dbTx, err := r.unwrapTx(tx)
	if err != nil {
		return nil, err
	}

	var row reconciliationResultRow
	err = dbTx.QueryRow(ctx, query).Scan(
		&row.ID,
		&row.CheckedAt,
		&row.TotalAccounts,
		&row.MismatchedAccounts,
		&row.Severity,
		&row.Details,
		&row.ActionTaken,
		&row.AutoRepaired,
		&row.DoubleCheckPassed,
		&row.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No results yet
		}
		return nil, fmt.Errorf("failed to get latest reconciliation result: %w", err)
	}

	return r.rowToEntity(&row)
}

// GetLatestBySeverity returns the most recent result with a given severity.
func (r *ReconciliationRepositoryImpl) GetLatestBySeverity(ctx context.Context, tx interface{}, severity entity.ReconcileSeverity) (*entity.ReconciliationResult, error) {
	query := `
		SELECT id, checked_at, total_accounts, mismatched_accounts,
		       severity, details, action_taken, auto_repaired, double_check_passed, created_at
		FROM reconciliation_results
		WHERE severity = $1
		ORDER BY checked_at DESC
		LIMIT 1
	`

	dbTx, err := r.unwrapTx(tx)
	if err != nil {
		return nil, err
	}

	var row reconciliationResultRow
	err = dbTx.QueryRow(ctx, query, string(severity)).Scan(
		&row.ID,
		&row.CheckedAt,
		&row.TotalAccounts,
		&row.MismatchedAccounts,
		&row.Severity,
		&row.Details,
		&row.ActionTaken,
		&row.AutoRepaired,
		&row.DoubleCheckPassed,
		&row.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No results with this severity
		}
		return nil, fmt.Errorf("failed to get latest reconciliation result by severity: %w", err)
	}

	return r.rowToEntity(&row)
}

// DeleteOld deletes reconciliation results older than the given duration.
func (r *ReconciliationRepositoryImpl) DeleteOld(ctx context.Context, tx interface{}, olderThan time.Duration) (int, error) {
	query := `
		DELETE FROM reconciliation_results
		WHERE created_at < $1
		RETURNING id
	`

	dbTx, err := r.unwrapTx(tx)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)

	rows, err := dbTx.Query(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old reconciliation results: %w", err)
	}
	defer rows.Close()

	var deleted int
	for rows.Next() {
		deleted++
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("error iterating deleted reconciliation results: %w", err)
	}

	return deleted, nil
}

// unwrapTx unwraps the transaction interface to a db.Tx.
func (r *ReconciliationRepositoryImpl) unwrapTx(tx interface{}) (db.Tx, error) {
	dbTx, ok := tx.(db.Tx)
	if !ok {
		return nil, fmt.Errorf("invalid transaction type")
	}
	return dbTx, nil
}

// rowToEntity converts a database row to a ReconciliationResult entity.
func (r *ReconciliationRepositoryImpl) rowToEntity(row *reconciliationResultRow) (*entity.ReconciliationResult, error) {
	details := entity.ReconcileDetailsFromJSON(row.Details)

	return &entity.ReconciliationResult{
		ID:                 row.ID,
		CheckedAt:          row.CheckedAt,
		TotalAccounts:      row.TotalAccounts,
		MismatchedAccounts: row.MismatchedAccounts,
		Severity:           entity.ReconcileSeverity(row.Severity),
		Details:            details,
		ActionTaken:        entity.ReconcileAction(row.ActionTaken),
		AutoRepaired:       row.AutoRepaired,
		DoubleCheckPassed:  row.DoubleCheckPassed,
		CreatedAt:          row.CreatedAt,
	}, nil
}


