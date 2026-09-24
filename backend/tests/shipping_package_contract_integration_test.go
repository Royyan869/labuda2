//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	shippingInfraRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// ONE-PACKAGE SHIPPING CONTRACT (Owner-locked business truth)
//
// Positive proof: the canonical one-package create/update/delete/retire flows
// behave as designed (full package persisted, content editable while linked,
// unlinked option deletable, retire via active toggle).
//
// Negative proof (kill-once locks):
//   - a bare option (zero destinations) can never be persisted;
//   - an option linked to any listing can never be hard-deleted.
// ============================================================================

func int64Ptr(v int64) *int64 { return &v }
func boolPtr(v bool) *bool    { return &v }

func newSellerShippingService() *shippingApp.SellerShippingService {
	optionRepo := shippingInfraRepo.NewShippingSetupRepository()
	coverageRepo := shippingInfraRepo.NewShippingCoverageRepository()
	cityOverrideRepo := shippingInfraRepo.NewCityOverrideRepository()
	productShippingRepo := shippingInfraRepo.NewProductShippingSetupRepository(optionRepo)
	return shippingApp.NewSellerShippingService(optionRepo, coverageRepo, cityOverrideRepo, productShippingRepo)
}

func samplePackageInput(sellerID uuid.UUID, name string) shippingApp.ShippingPackageInput {
	return shippingApp.ShippingPackageInput{
		SellerID:        sellerID,
		Name:            name,
		TransportType:   shippingEntity.TransportBus,
		InternalPurpose: "kantong besar, untuk 10 ekor",
		Destinations: []shippingApp.ProvinceDestinationInput{
			{
				ProvinceCode: "31",
				ProvinceName: "DKI Jakarta",
				Rate:         150_000, // all-in: shipping + packing
				IsAvailable:  true,
				CityQualifications: []shippingApp.CityQualificationInput{
					{CityCode: "3171", CityName: "Jakarta Pusat", Rate: int64Ptr(165_000)},
				},
			},
			{
				ProvinceCode: "32",
				ProvinceName: "Jawa Barat",
				Rate:         120_000,
				IsAvailable:  true,
				CityQualifications: []shippingApp.CityQualificationInput{
					{CityCode: "3273", CityName: "Bandung", IsAvailable: boolPtr(false)},
				},
			},
		},
	}
}

func TestShippingPackage_Create_PersistsFullPackage(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	seller := seedLiveUser(t, ctx, tdb)
	svc := newSellerShippingService()

	var option *shippingEntity.ShippingSetup
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		option, err = svc.CreateShippingPackage(ctx, tx, samplePackageInput(seller, "Bus Handoyo"))
		return err
	}))

	require.NotEqual(t, uuid.Nil, option.ID)
	require.True(t, option.IsActive)
	require.Equal(t, "kantong besar, untuk 10 ekor", option.InternalPurpose)

	// Option row carries the seller-private note.
	var internalPurpose string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT internal_purpose FROM shipping_options WHERE id=$1`, option.ID).Scan(&internalPurpose))
	require.Equal(t, "kantong besar, untuk 10 ekor", internalPurpose)

	// Both provinces persisted with rates.
	var rate31, rate32 int64
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT province_rate FROM shipping_coverages WHERE shipping_option_id=$1 AND province_code='31'`, option.ID).Scan(&rate31))
	require.Equal(t, int64(150_000), rate31)
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT province_rate FROM shipping_coverages WHERE shipping_option_id=$1 AND province_code='32'`, option.ID).Scan(&rate32))
	require.Equal(t, int64(120_000), rate32)

	// City qualification: Jakarta Pusat has a rate override.
	var overrideRate *int64
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT o.rate FROM shipping_city_overrides o
		 JOIN shipping_coverages c ON c.id = o.shipping_coverage_id
		 WHERE c.shipping_option_id=$1 AND o.city_code='3171'`, option.ID).Scan(&overrideRate))
	require.NotNil(t, overrideRate)
	require.Equal(t, int64(165_000), *overrideRate)

	// City qualification: Bandung is disabled (availability override, inherited rate).
	var overrideRateBandung *int64
	var overrideAvail *bool
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT o.rate, o.is_available FROM shipping_city_overrides o
		 JOIN shipping_coverages c ON c.id = o.shipping_coverage_id
		 WHERE c.shipping_option_id=$1 AND o.city_code='3273'`, option.ID).Scan(&overrideRateBandung, &overrideAvail))
	require.Nil(t, overrideRateBandung)
	require.NotNil(t, overrideAvail)
	require.False(t, *overrideAvail)
}

func TestShippingPackage_Create_WithoutDestinations_Rejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	seller := seedLiveUser(t, ctx, tdb)
	svc := newSellerShippingService()

	input := samplePackageInput(seller, "Opsi Telanjang")
	input.Destinations = nil

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.CreateShippingPackage(ctx, tx, input)
		return err
	})
	require.ErrorIs(t, err, shippingApp.ErrShippingPackageIncomplete)

	// Negative proof: no bare option row exists.
	var cnt int64
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM shipping_options WHERE seller_id=$1 AND name='Opsi Telanjang'`, seller).Scan(&cnt))
	require.Equal(t, int64(0), cnt)
}

// seedActiveForSaleLinkedTo inserts an active for_sale listing and links the
// given shipping option to it (order-history-relevant state).
func seedActiveForSaleLinkedTo(t *testing.T, ctx context.Context, tdb *testdb.TestDB, seller, optionID uuid.UUID) uuid.UUID {
	t.Helper()
	productID := uuid.New()
	forSaleID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO products (id, seller_id, title, description, media_urls, variety, preparation_time, selling_surface, created_at, updated_at) VALUES ($1,$2,'live listing','desc','[]','Kohaku','immediate','for_sale',NOW(),NOW())`, productID, seller)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, quantity_available, created_at, updated_at) VALUES ($1,$2,$3,100000,false,'active',1,NOW(),NOW())`, forSaleID, productID, seller)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO product_shipping_options (product_id, shipping_option_id, sort_order, created_at) VALUES ($1,$2,0,NOW())`, productID, optionID)
		return err
	}))
	return forSaleID
}

func TestShippingPackage_Update_WhileLinkedToActiveListing_Succeeds(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	seller := seedLiveUser(t, ctx, tdb)
	svc := newSellerShippingService()

	var option *shippingEntity.ShippingSetup
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		option, err = svc.CreateShippingPackage(ctx, tx, samplePackageInput(seller, "Bus Ramayana"))
		return err
	}))

	// Link to an ACTIVE listing — content must remain editable (orders keep
	// their checkout snapshot; order creation re-validates coverage).
	seedActiveForSaleLinkedTo(t, ctx, tdb, seller, option.ID)

	input := samplePackageInput(seller, "Bus Ramayana")
	input.InternalPurpose = "kantong kecil, untuk 1 ekor"
	input.Destinations[0].Rate = 175_000 // seller's subscription tariff changed

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.UpdateShippingPackage(ctx, tx, option.ID, input)
		return err
	}))

	var rate31 int64
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT province_rate FROM shipping_coverages WHERE shipping_option_id=$1 AND province_code='31'`, option.ID).Scan(&rate31))
	require.Equal(t, int64(175_000), rate31)

	var note string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT internal_purpose FROM shipping_options WHERE id=$1`, option.ID).Scan(&note))
	require.Equal(t, "kantong kecil, untuk 1 ekor", note)

	// Link intact.
	var linkCnt int64
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM product_shipping_options WHERE shipping_option_id=$1`, option.ID).Scan(&linkCnt))
	require.Equal(t, int64(1), linkCnt)
}

func TestShippingPackage_Delete_LinkedOption_Rejected(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	seller := seedLiveUser(t, ctx, tdb)
	svc := newSellerShippingService()

	var option *shippingEntity.ShippingSetup
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		option, err = svc.CreateShippingPackage(ctx, tx, samplePackageInput(seller, "Bus Terlink"))
		return err
	}))
	seedActiveForSaleLinkedTo(t, ctx, tdb, seller, option.ID)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return svc.DeleteShippingSetup(ctx, tx, option.ID, seller)
	})
	require.ErrorIs(t, err, shippingApp.ErrShippingLinkedOptionUndeletable)

	// Option still exists (must be retired via deactivate, not deleted).
	var cnt int64
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM shipping_options WHERE id=$1`, option.ID).Scan(&cnt))
	require.Equal(t, int64(1), cnt)
}

func TestShippingPackage_Delete_Unlinked_Succeeds(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	seller := seedLiveUser(t, ctx, tdb)
	svc := newSellerShippingService()

	var option *shippingEntity.ShippingSetup
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		option, err = svc.CreateShippingPackage(ctx, tx, samplePackageInput(seller, "Bus Boleh Hapus"))
		return err
	}))

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return svc.DeleteShippingSetup(ctx, tx, option.ID, seller)
	}))

	var optCnt, covCnt int64
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM shipping_options WHERE id=$1`, option.ID).Scan(&optCnt))
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM shipping_coverages WHERE shipping_option_id=$1`, option.ID).Scan(&covCnt))
	require.Equal(t, int64(0), optCnt)
	require.Equal(t, int64(0), covCnt)
}

func TestShippingPackage_SetActive_RetireAndRestore(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	seller := seedLiveUser(t, ctx, tdb)
	svc := newSellerShippingService()

	var option *shippingEntity.ShippingSetup
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		option, err = svc.CreateShippingPackage(ctx, tx, samplePackageInput(seller, "Bus Pensiun"))
		return err
	}))

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.SetShippingSetupActive(ctx, tx, option.ID, seller, false)
		return err
	}))
	var isActive bool
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT is_active FROM shipping_options WHERE id=$1`, option.ID).Scan(&isActive))
	require.False(t, isActive)

	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := svc.SetShippingSetupActive(ctx, tx, option.ID, seller, true)
		return err
	}))
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT is_active FROM shipping_options WHERE id=$1`, option.ID).Scan(&isActive))
	require.True(t, isActive)
}
