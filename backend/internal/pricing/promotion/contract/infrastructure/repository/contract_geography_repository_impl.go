package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/contract/entity"
	"github.com/labuda/backend/pkg/db"
)

type ContractGeographyRepositoryImpl struct{}

func NewContractGeographyRepository() *ContractGeographyRepositoryImpl {
	return &ContractGeographyRepositoryImpl{}
}

func (r *ContractGeographyRepositoryImpl) ReplaceGeographies(ctx context.Context, tx db.Tx, contractID uuid.UUID, geos []entity.ContractGeography) error {
	if _, err := tx.Exec(ctx, `DELETE FROM promotion_contract_geographies WHERE contract_id = $1`, contractID); err != nil {
		return err
	}
	for _, g := range geos {
		if _, err := tx.Exec(ctx, `
			INSERT INTO promotion_contract_geographies (contract_id, city_id, city_name, province_id, created_at)
			VALUES ($1,$2,$3,$4, now())
		`, contractID, g.CityID, g.CityName, g.ProvinceID); err != nil {
			return err
		}
	}
	return nil
}

func (r *ContractGeographyRepositoryImpl) ListByContract(ctx context.Context, tx db.Tx, contractID uuid.UUID) ([]*entity.ContractGeography, error) {
	rows, err := tx.Query(ctx, `
		SELECT contract_id, city_id, city_name, province_id, created_at
		FROM promotion_contract_geographies
		WHERE contract_id = $1
		ORDER BY city_id ASC
	`, contractID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*entity.ContractGeography
	for rows.Next() {
		var g entity.ContractGeography
		if err := rows.Scan(&g.ContractID, &g.CityID, &g.CityName, &g.ProvinceID, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &g)
	}
	return out, rows.Err()
}

func (r *ContractGeographyRepositoryImpl) HasGeographicRestriction(ctx context.Context, tx db.Tx, contractID uuid.UUID) (bool, error) {
	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM promotion_contract_geographies WHERE contract_id = $1`, contractID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ContractGeographyRepositoryImpl) IsCityAllowed(ctx context.Context, tx db.Tx, contractID uuid.UUID, cityID string) (bool, error) {
	if cityID == "" {
		return false, nil
	}
	hasRestriction, err := r.HasGeographicRestriction(ctx, tx, contractID)
	if err != nil {
		return false, err
	}
	if !hasRestriction {
		return true, nil
	}
	var exists int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM promotion_contract_geographies WHERE contract_id = $1 AND city_id = $2`, contractID, cityID).Scan(&exists); err != nil {
		return false, err
	}
	return exists > 0, nil
}
