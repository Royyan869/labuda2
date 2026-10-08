package geography

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service is the canonical read surface of the Geography Master.
type Service struct {
	repo Repository
	pool *pgxpool.Pool
}

// NewService constructs the canonical geography service.
func NewService(repo Repository, pool *pgxpool.Pool) *Service {
	return &Service{repo: repo, pool: pool}
}

// Provinces returns every province ordered by code.
func (s *Service) Provinces(ctx context.Context) ([]Entity, error) {
	return s.repo.ListProvinces(ctx, s.pool)
}

// Regencies returns the regencies/cities of a province.
func (s *Service) Regencies(ctx context.Context, provinceCode string) ([]Entity, error) {
	return s.repo.ListChildren(ctx, s.pool, provinceCode, LevelRegency)
}

// Districts returns the districts of a regency/city.
func (s *Service) Districts(ctx context.Context, regencyCode string) ([]Entity, error) {
	return s.repo.ListChildren(ctx, s.pool, regencyCode, LevelDistrict)
}

// Villages returns the villages of a district (with postal codes).
func (s *Service) Villages(ctx context.Context, districtCode string) ([]Entity, error) {
	return s.repo.ListChildren(ctx, s.pool, districtCode, LevelVillage)
}

// ByCode returns a single master row.
func (s *Service) ByCode(ctx context.Context, code string) (*Entity, error) {
	return s.repo.GetByCode(ctx, s.pool, code)
}
