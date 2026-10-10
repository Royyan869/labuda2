//go:build integration

package geography_test

import (
	"context"
	"testing"

	geography "github.com/hishumi/backend/internal/platform/geography"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
)

// TestCanonicalMaster_SeededAndHierarchyWorks proves the master is populated
// by the test bootstrap and the Province -> Regency -> District -> Village
// relationships resolve.
func TestCanonicalMaster_SeededAndHierarchyWorks(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	n, err := geography.Count(ctx, tdb.Pool())
	require.NoError(t, err)
	require.Equal(t, int64(91143), n, "master must hold the exact complete Indonesian hierarchy")

	repo := geography.NewRepository(tdb.Pool())
	svc := geography.NewService(repo, tdb.Pool())

	provinces, err := svc.Provinces(ctx)
	require.NoError(t, err)
	require.Equal(t, 38, len(provinces))

	regencies, err := svc.Regencies(ctx, "32")
	require.NoError(t, err)
	require.NotEmpty(t, regencies)

	districts, err := svc.Districts(ctx, "3204")
	require.NoError(t, err)
	require.NotEmpty(t, districts)

	villages, err := svc.Villages(ctx, districts[0].Code)
	require.NoError(t, err)
	require.NotEmpty(t, villages)
}

// TestCanonicalMaster_RestoredDistrictsAndCriticalChain proves the restored
// districts/villages resolve through the canonical service against the live
// master, including the exact critical chain 12 -> 1204 -> 120428 -> 1204282001.
func TestCanonicalMaster_RestoredDistrictsAndCriticalChain(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := geography.NewRepository(tdb.Pool())
	svc := geography.NewService(repo, tdb.Pool())

	// Critical chain: province 12 -> regency 1204 -> district 120428 -> village.
	regencies, err := svc.Regencies(ctx, "12")
	require.NoError(t, err)
	require.True(t, containsCode(regencies, "1204"), "regency 1204 present")

	districts, err := svc.Districts(ctx, "1204")
	require.NoError(t, err)
	maU, ok := findCode(districts, "120428")
	require.True(t, ok, "district 120428 (Ma'u) present")
	require.Equal(t, "Ma'u", maU.Name)

	villages, err := svc.Villages(ctx, "120428")
	require.NoError(t, err)
	balodano, ok := findCode(villages, "1204282001")
	require.True(t, ok, "village 1204282001 present")
	require.Equal(t, "Balodano", balodano.Name)
	require.NotNil(t, balodano.PostalCode)
	require.Equal(t, "22855", *balodano.PostalCode)

	// Single-row read resolves the village + postal.
	v, err := svc.ByCode(ctx, "1204282001")
	require.NoError(t, err)
	require.Equal(t, "Balodano", v.Name)
	require.Equal(t, "120428", *v.ParentCode)
	require.Equal(t, "22855", *v.PostalCode)

	// All 19 restored districts exist.
	restored := map[string]string{
		"120428": "1204", "120435": "1204", "121421": "1214", "121423": "1214",
		"121425": "1214", "122504": "1225", "122508": "1225", "127805": "1278",
		"352922": "3529", "520503": "5205", "530210": "5302", "710410": "7104",
		"731010": "7310", "731813": "7318", "731833": "7318", "731834": "7318",
		"731835": "7318", "732612": "7326", "940516": "9405",
	}
	for code, parent := range restored {
		d, err := svc.ByCode(ctx, code)
		require.NoErrorf(t, err, "restored district %s", code)
		require.Equal(t, geography.LevelDistrict, d.Level)
		require.Equal(t, parent, *d.ParentCode)
	}

	// The validator accepts the restored chain.
	val := geography.NewValidator()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return val.ValidateAddressScope(ctx, tx, "12", "1204", "120428", "1204282001")
	}))
}

func containsCode(entities []geography.Entity, code string) bool {
	_, ok := findCode(entities, code)
	return ok
}

func findCode(entities []geography.Entity, code string) (geography.Entity, bool) {
	for _, e := range entities {
		if e.Code == code {
			return e, true
		}
	}
	return geography.Entity{}, false
}

// TestValidator_AcceptsValidRejectsInvalid proves consumers cannot create
// geographic truth outside the master.
func TestValidator_AcceptsValidRejectsInvalid(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	v := geography.NewValidator()

	// Valid full chain: 32 -> 3204 -> a real district -> a real village.
	repo := geography.NewRepository(tdb.Pool())
	svc := geography.NewService(repo, tdb.Pool())
	districts, err := svc.Districts(ctx, "3204")
	require.NoError(t, err)
	require.NotEmpty(t, districts)
	villages, err := svc.Villages(ctx, districts[0].Code)
	require.NoError(t, err)
	require.NotEmpty(t, villages)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return v.ValidateAddressScope(ctx, tx, "32", "3204", districts[0].Code, villages[0].Code)
	})
	require.NoError(t, err)

	// Unknown city is rejected.
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return v.ValidateAddressScope(ctx, tx, "32", "9999", "", "")
	})
	require.ErrorIs(t, err, geography.ErrInvalidGeography)

	// City whose parent is a different province is rejected (3204 belongs to 32, not 31).
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return v.ValidateAddressScope(ctx, tx, "31", "3204", "", "")
	})
	require.ErrorIs(t, err, geography.ErrInvalidGeography)

	// Shipping province/city gate.
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return v.ValidateProvinceCity(ctx, tx, "32", "3204")
	})
	require.NoError(t, err)
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return v.ValidateProvinceCity(ctx, tx, "32", "3171")
	})
	require.ErrorIs(t, err, geography.ErrInvalidGeography)
}
