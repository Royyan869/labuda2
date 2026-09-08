package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	contractRepo "github.com/labuda/backend/internal/pricing/promotion/contract/repository"
	"github.com/labuda/backend/pkg/db"
)

// ContractRepositoryImpl persists canonical promotion contract rows.
type ContractRepositoryImpl struct{}

// NewContractRepository creates a new ContractRepositoryImpl.
func NewContractRepository() *ContractRepositoryImpl {
	return &ContractRepositoryImpl{}
}

var _ contractRepo.Repository = (*ContractRepositoryImpl)(nil)

// GetDBTime returns the current database time (DB clock authority).
func (r *ContractRepositoryImpl) GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error) {
	var dbTime time.Time
	if err := tx.QueryRow(ctx, `SELECT NOW()`).Scan(&dbTime); err != nil {
		return time.Time{}, fmt.Errorf("get database time: %w", err)
	}
	return dbTime, nil
}

const contractColumns = `
	id, seller_id, kind, status, budget_rupiah, cpm_rupiah,
	planned_start, planned_finish, allocation_account_id,
	paused_at, finalized_at, created_at, updated_at`

func (r *ContractRepositoryImpl) Create(ctx context.Context, tx db.Tx, c *entity.Contract) error {
	if c == nil {
		return fmt.Errorf("promotion contract is nil")
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO promotion_contracts (
			id, seller_id, kind, status, budget_rupiah, cpm_rupiah,
			planned_start, planned_finish, allocation_account_id,
			paused_at, finalized_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		c.ID,
		c.SellerID,
		string(c.Kind),
		string(c.Status),
		c.BudgetRupiah,
		c.CPMRupiah,
		c.PlannedStart,
		c.PlannedFinish,
		c.AllocationAccountID,
		c.PausedAt,
		c.FinalizedAt,
		c.CreatedAt,
		c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create promotion contract failed: %w", err)
	}
	return nil
}

func (r *ContractRepositoryImpl) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Contract, error) {
	return r.scanContract(ctx, tx, id, false)
}

func (r *ContractRepositoryImpl) ListBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID) ([]*entity.Contract, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+contractColumns+`
		FROM promotion_contracts
		WHERE seller_id = $1
		ORDER BY created_at DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("list promotion contracts failed: %w", err)
	}
	defer rows.Close()

	var out []*entity.Contract
	for rows.Next() {
		var c entity.Contract
		var kind, status string
		if err := rows.Scan(
			&c.ID, &c.SellerID, &kind, &status,
			&c.BudgetRupiah, &c.CPMRupiah,
			&c.PlannedStart, &c.PlannedFinish, &c.AllocationAccountID,
			&c.PausedAt, &c.FinalizedAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan promotion contract failed: %w", err)
		}
		c.Kind = entity.Kind(kind)
		c.Status = entity.Status(status)
		out = append(out, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promotion contracts failed: %w", err)
	}
	return out, nil
}

func (r *ContractRepositoryImpl) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*entity.Contract, error) {
	return r.scanContract(ctx, tx, id, true)
}

func (r *ContractRepositoryImpl) scanContract(ctx context.Context, tx db.Tx, id uuid.UUID, forUpdate bool) (*entity.Contract, error) {
	query := `
		SELECT ` + contractColumns + `
		FROM promotion_contracts
		WHERE id = $1
	`
	if forUpdate {
		query += ` FOR UPDATE`
	}

	var c entity.Contract
	var kind, status string
	err := tx.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.SellerID, &kind, &status,
		&c.BudgetRupiah, &c.CPMRupiah,
		&c.PlannedStart, &c.PlannedFinish, &c.AllocationAccountID,
		&c.PausedAt, &c.FinalizedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows || err.Error() == "no rows in result set" {
			return nil, contractRepo.ErrContractNotFound
		}
		return nil, fmt.Errorf("get promotion contract failed: %w", err)
	}

	c.Kind = entity.Kind(kind)
	c.Status = entity.Status(status)
	return &c, nil
}

// Update persists ONLY the mutable lifecycle fields. budget_rupiah,
// cpm_rupiah, planned_start, kind and allocation_account_id are immutable and
// deliberately absent from the SET clause — a pricing snapshot can never be
// silently repriced.
func (r *ContractRepositoryImpl) Update(ctx context.Context, tx db.Tx, c *entity.Contract) error {
	if c == nil {
		return fmt.Errorf("promotion contract is nil")
	}
	_, err := tx.Exec(ctx, `
		UPDATE promotion_contracts
		SET status = $2,
		    planned_finish = $3,
		    paused_at = $4,
		    finalized_at = $5,
		    updated_at = NOW()
		WHERE id = $1
	`,
		c.ID,
		string(c.Status),
		c.PlannedFinish,
		c.PausedAt,
		c.FinalizedAt,
	)
	if err != nil {
		return fmt.Errorf("update promotion contract failed: %w", err)
	}
	return nil
}
