package viewercontext

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// geographyQuerier is satisfied by *pgxpool.Pool and db.Tx
type geographyQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ResolveViewerGeography resolves the canonical viewer geography from primary address.
// Uses addresses.is_primary authority, not user_profiles or contents.
func ResolveViewerGeography(ctx context.Context, q geographyQuerier, viewerID uuid.UUID) GeographyOverlay {
	if viewerID == uuid.Nil || q == nil {
		return NewGeographyOverlay("", "", "", false, true)
	}
	var provinceID, cityID, cityName string
	err := q.QueryRow(ctx, `
		SELECT province_id, city_id, city_name
		FROM addresses
		WHERE user_id = $1 AND is_primary = true AND is_available_for_checkout = true
		LIMIT 1
	`, viewerID).Scan(&provinceID, &cityID, &cityName)
	if err != nil {
		return NewGeographyOverlay("", "", "", false, true)
	}
	if cityID == "" {
		return NewGeographyOverlay("", "", "", false, true)
	}
	return NewGeographyOverlay(provinceID, cityID, cityName, true, true)
}
