//go:build integration

package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	geography "github.com/labuda/backend/internal/platform/geography"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

// TestSellerShippingService_GeographyGate proves a seller may select canonical
// provinces/cities but may NOT define an independent geographic identity.
func TestSellerShippingService_GeographyGate(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	svc := NewSellerShippingService(
		shippingRepo.NewShippingSetupRepository(),
		shippingRepo.NewShippingCoverageRepository(),
		shippingRepo.NewCityOverrideRepository(),
		shippingRepo.NewProductShippingSetupRepository(shippingRepo.NewShippingSetupRepository()),
	)
	svc.SetGeographyValidator(geography.NewValidator())

	seller := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := tx.Exec(ctx, `
			INSERT INTO users (id, firebase_uid, email, account_status, created_at, updated_at, role)
			VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
		`, seller, seller.String(), seller.String()+"@test.invalid")
		return e
	}))

	// Unknown province: rejected before any coverage is written.
	invalid := ShippingPackageInput{
		SellerID:      seller,
		Name:          "Bad",
		TransportType: shippingEntity.TransportCustom,
		Destinations: []ProvinceDestinationInput{
			{ProvinceCode: "9999", ProvinceName: "Nowhere", Rate: 1000, IsAvailable: true},
		},
	}
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := svc.CreateShippingPackage(ctx, tx, invalid)
		return e
	})
	require.ErrorIs(t, err, geography.ErrInvalidGeography)

	var optionCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM shipping_options WHERE seller_id = $1`, seller).Scan(&optionCount))
	require.Equal(t, 0, optionCount, "invalid geography leaves no shipping option")

	// Canonical province + valid city override: accepted.
	valid := ShippingPackageInput{
		SellerID:      seller,
		Name:          "Good",
		TransportType: shippingEntity.TransportCustom,
		Destinations: []ProvinceDestinationInput{
			{
				ProvinceCode: "32",
				ProvinceName: "Jawa Barat",
				Rate:         10000,
				IsAvailable:  true,
				CityQualifications: []CityQualificationInput{
					{CityCode: "3204", CityName: "Kabupaten Bandung"},
				},
			},
		},
	}
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := svc.CreateShippingPackage(ctx, tx, valid)
		return e
	})
	require.NoError(t, err)

	var coverageCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT COUNT(*) FROM shipping_coverages c
		JOIN shipping_options o ON o.id = c.shipping_option_id
		WHERE o.seller_id = $1
	`, seller).Scan(&coverageCount))
	require.Equal(t, 1, coverageCount)
}
