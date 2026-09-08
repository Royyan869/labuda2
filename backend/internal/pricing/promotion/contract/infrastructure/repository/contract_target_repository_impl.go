package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	contractRepo "github.com/labuda/backend/internal/pricing/promotion/contract/repository"
	promoentity "github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// ContractTargetRepositoryImpl persists the rolling target queue.
type ContractTargetRepositoryImpl struct{}

func NewContractTargetRepository() *ContractTargetRepositoryImpl {
	return &ContractTargetRepositoryImpl{}
}

var _ contractRepo.ContractTargetRepository = (*ContractTargetRepositoryImpl)(nil)

// AddTarget inserts a queue entry at the next position (append). Caller must
// have validated max 10, ownership, and target operability.
func (r *ContractTargetRepositoryImpl) AddTarget(ctx context.Context, tx db.Tx, target *entity.ContractTarget) error {
	if target == nil {
		return fmt.Errorf("contract target is nil")
	}
	if !target.IsValid() {
		return fmt.Errorf("contract target invalid: %v", target)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO promotion_contract_targets (id, contract_id, target_type, target_id, position, added_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, target.ID, target.ContractID, string(target.TargetType), target.TargetID, target.Position, target.AddedAt)
	if err != nil {
		return fmt.Errorf("insert contract target: %w", err)
	}
	return nil
}

func (r *ContractTargetRepositoryImpl) ListTargets(ctx context.Context, tx db.Tx, contractID uuid.UUID) ([]*entity.ContractTarget, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, contract_id, target_type, target_id, position, added_at
		FROM promotion_contract_targets
		WHERE contract_id = $1
		ORDER BY position ASC
	`, contractID)
	if err != nil {
		return nil, fmt.Errorf("list contract targets: %w", err)
	}
	defer rows.Close()
	var out []*entity.ContractTarget
	for rows.Next() {
		var t entity.ContractTarget
		var tt string
		if err := rows.Scan(&t.ID, &t.ContractID, &tt, &t.TargetID, &t.Position, &t.AddedAt); err != nil {
			return nil, fmt.Errorf("scan contract target: %w", err)
		}
		t.TargetType = promoentity.TargetType(tt)
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contract targets: %w", err)
	}
	return out, nil
}

func (r *ContractTargetRepositoryImpl) RemoveTarget(ctx context.Context, tx db.Tx, contractID, targetID uuid.UUID) error {
	tag, err := tx.Exec(ctx, `
		DELETE FROM promotion_contract_targets
		WHERE contract_id = $1 AND target_id = $2
	`, contractID, targetID)
	if err != nil {
		return fmt.Errorf("remove contract target: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("contract target not found: %s", targetID)
	}
	// Compact positions to keep 0..N-1 contiguous after deletion.
	// Reassign positions in order of current position.
	rows, err := tx.Query(ctx, `
		SELECT id FROM promotion_contract_targets
		WHERE contract_id = $1
		ORDER BY position ASC
	`, contractID)
	if err != nil {
		return fmt.Errorf("reorder after remove: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan reorder id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i, id := range ids {
		if _, err := tx.Exec(ctx, `UPDATE promotion_contract_targets SET position = $2 WHERE id = $1`, id, i); err != nil {
			return fmt.Errorf("compact position: %w", err)
		}
	}
	return nil
}

func (r *ContractTargetRepositoryImpl) CountTargets(ctx context.Context, tx db.Tx, contractID uuid.UUID) (int, error) {
	var n int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM promotion_contract_targets WHERE contract_id = $1`, contractID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count contract targets: %w", err)
	}
	return n, nil
}

// ResolveEffectiveTarget returns the first operable target in queue order.
// Operability is checked via the supplied checker; unavailable targets are
// skipped. Returns nil if none operable.
func (r *ContractTargetRepositoryImpl) ResolveEffectiveTarget(ctx context.Context, tx db.Tx, contractID uuid.UUID, checker func(targetType promoentity.TargetType, targetID *uuid.UUID) (bool, string, error)) (*entity.ContractTarget, error) {
	targets, err := r.ListTargets(ctx, tx, contractID)
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		ok, _, err := checker(t.TargetType, &t.TargetID)
		if err != nil {
			continue
		}
		if ok {
			return t, nil
		}
	}
	return nil, nil
}

// GetTargetForUpdate locks a single queue entry for transactional resolver.
func (r *ContractTargetRepositoryImpl) GetTargetForUpdate(ctx context.Context, tx db.Tx, contractID, targetID uuid.UUID) (*entity.ContractTarget, error) {
	var t entity.ContractTarget
	var tt string
	err := tx.QueryRow(ctx, `
		SELECT id, contract_id, target_type, target_id, position, added_at
		FROM promotion_contract_targets
		WHERE contract_id = $1 AND target_id = $2
		FOR UPDATE
	`, contractID, targetID).Scan(&t.ID, &t.ContractID, &tt, &t.TargetID, &t.Position, &t.AddedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get contract target for update: %w", err)
	}
	t.TargetType = promoentity.TargetType(tt)
	return &t, nil
}
