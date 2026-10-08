package geography

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is the read surface the geography repository needs. Both
// *pgxpool.Pool and pkg/db.Tx satisfy it.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repository is the canonical read surface of the Geography Master.
type Repository interface {
	ListProvinces(ctx context.Context, q Querier) ([]Entity, error)
	ListChildren(ctx context.Context, q Querier, parentCode string, level Level) ([]Entity, error)
	GetByCode(ctx context.Context, q Querier, code string) (*Entity, error)
}

// RepositoryImpl is the Postgres-backed canonical geography repository.
type RepositoryImpl struct {
	pool *pgxpool.Pool
}

// NewRepository constructs the canonical geography repository.
func NewRepository(pool *pgxpool.Pool) *RepositoryImpl {
	return &RepositoryImpl{pool: pool}
}

const listColumns = `code, level, name, parent_code, postal_code`

// ListProvinces returns every province ordered by code.
func (r *RepositoryImpl) ListProvinces(ctx context.Context, q Querier) ([]Entity, error) {
	if q == nil {
		q = r.pool
	}
	rows, err := q.Query(ctx,
		`SELECT `+listColumns+` FROM canonical_geographies WHERE level = 'province' ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("list provinces: %w", err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ListChildren returns the direct children of parentCode at level, ordered by name.
func (r *RepositoryImpl) ListChildren(ctx context.Context, q Querier, parentCode string, level Level) ([]Entity, error) {
	if q == nil {
		q = r.pool
	}
	rows, err := q.Query(ctx,
		`SELECT `+listColumns+` FROM canonical_geographies WHERE parent_code = $1 AND level = $2 ORDER BY name`,
		parentCode, string(level))
	if err != nil {
		return nil, fmt.Errorf("list %s children: %w", level, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// GetByCode returns a single master row, or ErrGeographyNotFound.
func (r *RepositoryImpl) GetByCode(ctx context.Context, q Querier, code string) (*Entity, error) {
	if q == nil {
		q = r.pool
	}
	var e Entity
	var level, name string
	var parent, postal *string
	err := q.QueryRow(ctx,
		`SELECT `+listColumns+` FROM canonical_geographies WHERE code = $1`, code,
	).Scan(&e.Code, &level, &name, &parent, &postal)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGeographyNotFound
		}
		return nil, fmt.Errorf("get geography: %w", err)
	}
	e.Level = Level(level)
	e.Name = name
	e.ParentCode = parent
	e.PostalCode = postal
	return &e, nil
}

func scanEntities(rows pgx.Rows) ([]Entity, error) {
	out := make([]Entity, 0)
	for rows.Next() {
		var e Entity
		var level, name string
		var parent, postal *string
		if err := rows.Scan(&e.Code, &level, &name, &parent, &postal); err != nil {
			return nil, fmt.Errorf("scan geography: %w", err)
		}
		e.Level = Level(level)
		e.Name = name
		e.ParentCode = parent
		e.PostalCode = postal
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate geography: %w", err)
	}
	return out, nil
}
