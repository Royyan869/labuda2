//go:build integration

package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	addressRepo "github.com/labuda/backend/internal/identity/address/infrastructure/repository"
	geography "github.com/labuda/backend/internal/platform/geography"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestAddressService_GeographyGate proves the address book cannot persist a
// geographic identity outside the canonical Geography Master.
func TestAddressService_GeographyGate(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()

	svc := &AddressService{
		repo: addressRepo.NewAddressRepository(),
		geo:  geography.NewValidator(),
		log:  zap.NewNop(),
	}

	userID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		seedPrimaryInvariantUser(t, ctx, tx, userID)
		return nil
	}))

	// Unknown city: rejected before the row is written.
	invalid := newSenderAddressInput(userID, "Bad", false)
	invalid.CityID = "9999"
	invalid.CityName = "Nowhere"
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := svc.CreateAddress(ctx, tx, invalid)
		return e
	})
	require.ErrorIs(t, err, geography.ErrInvalidGeography)

	// City whose province parent does not match: rejected.
	mismatch := newSenderAddressInput(userID, "Mismatch", false)
	mismatch.ProvinceID = "31"
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := svc.CreateAddress(ctx, tx, mismatch)
		return e
	})
	require.ErrorIs(t, err, geography.ErrInvalidGeography)

	// Valid canonical chain: accepted.
	valid := newSenderAddressInput(userID, "Good", false)
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := svc.CreateAddress(ctx, tx, valid)
		return e
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM addresses WHERE user_id = $1`, userID).Scan(&count))
	require.Equal(t, 1, count, "only the valid address is persisted")
}
