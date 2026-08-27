//go:build integration

package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepo "github.com/labuda/backend/internal/identity/address/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const addressesActivePrimaryUniqueIndex = "idx_addresses_user_active_primary_unique"

func setupPrimaryInvariantTest(t *testing.T) (*testdb.TestDB, *AddressService, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	svc := &AddressService{
		repo: addressRepo.NewAddressRepository(),
		log:  zap.NewNop(),
	}

	return tdb, svc, cleanup
}

func seedPrimaryInvariantUser(t *testing.T, ctx context.Context, tx db.Tx, userID uuid.UUID) {
	t.Helper()

	_, err := tx.Exec(ctx, `
		INSERT INTO users (
			id, firebase_uid, email, account_status, created_at, updated_at, role
		)
		VALUES ($1, $2, $3, 'active', NOW(), NOW(), 'user')
	`, userID, userID.String(), userID.String()+"@test.invalid")
	require.NoError(t, err)
}

func seedPrimaryInvariantAddress(
	t *testing.T,
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
	userID uuid.UUID,
	createdAt time.Time,
	isPrimary bool,
	isAvailable bool,
	nickname string,
) {
	t.Helper()

	_, err := tx.Exec(ctx, `
		INSERT INTO addresses (
			id, user_id, purpose, nickname,
			recipient_name, phone,
			province_id, province_name,
			city_id, city_name,
			district_id, district_name,
			village_id, village_name,
			street_address, postal_code,
			latitude, longitude, notes,
			is_primary, is_available_for_checkout,
			created_at, updated_at
		)
		VALUES (
			$1, $2, 'sender', $3,
			'Koikoi Farm', '08123456789',
			'33', 'Jawa Tengah',
			'3301', 'Kabupaten Demak',
			'330101', 'Mranggen',
			'3301012001', 'Rowosari',
			'Jl. Melati No. 12', '59511',
			NULL, NULL, 'seed',
			$4, $5,
			$6, $6
		)
	`, id, userID, nickname, isPrimary, isAvailable, createdAt)
	require.NoError(t, err)
}

func dropActivePrimaryUniqueIndex(t *testing.T, ctx context.Context, tx db.Tx) {
	t.Helper()

	_, err := tx.Exec(ctx, `DROP INDEX IF EXISTS public.`+addressesActivePrimaryUniqueIndex)
	require.NoError(t, err)
}

func repairPrimaryInvariant(t *testing.T, ctx context.Context, tx db.Tx) {
	t.Helper()

	_, err := tx.Exec(ctx, `
		WITH missing_primary_users AS (
			SELECT user_id
			FROM public.addresses
			WHERE is_available_for_checkout = true
			GROUP BY user_id
			HAVING COUNT(*) FILTER (WHERE is_primary = true) = 0
		),
		missing_primary_candidates AS (
			SELECT DISTINCT ON (a.user_id)
				a.id
			FROM public.addresses a
			JOIN missing_primary_users u ON u.user_id = a.user_id
			WHERE a.is_available_for_checkout = true
			ORDER BY a.user_id, a.created_at ASC, a.id ASC
		)
		UPDATE public.addresses a
		SET is_primary = true,
			updated_at = NOW()
		FROM missing_primary_candidates c
		WHERE a.id = c.id
	`)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		WITH multi_primary_users AS (
			SELECT user_id
			FROM public.addresses
			WHERE is_available_for_checkout = true
			  AND is_primary = true
			GROUP BY user_id
			HAVING COUNT(*) > 1
		),
		primary_keeper AS (
			SELECT DISTINCT ON (a.user_id)
				a.id,
				a.user_id
			FROM public.addresses a
			JOIN multi_primary_users u ON u.user_id = a.user_id
			WHERE a.is_available_for_checkout = true
			  AND a.is_primary = true
			ORDER BY a.user_id, a.created_at ASC, a.id ASC
		)
		UPDATE public.addresses a
		SET is_primary = false,
			updated_at = NOW()
		WHERE a.is_available_for_checkout = true
		  AND a.is_primary = true
		  AND a.user_id IN (SELECT user_id FROM multi_primary_users)
		  AND NOT EXISTS (
			  SELECT 1
			  FROM primary_keeper k
			  WHERE k.id = a.id
		  )
	`)
	require.NoError(t, err)
}

func activePrimaryCounts(t *testing.T, ctx context.Context, tx db.Tx, userID uuid.UUID) (int64, int64) {
	t.Helper()

	var total int64
	var primary int64
	err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE is_primary = true)
		FROM addresses
		WHERE user_id = $1
		  AND is_available_for_checkout = true
	`, userID).Scan(&total, &primary)
	require.NoError(t, err)
	return total, primary
}

func fetchPrimaryAddressID(t *testing.T, ctx context.Context, tx db.Tx, userID uuid.UUID) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM addresses
		WHERE user_id = $1
		  AND is_available_for_checkout = true
		  AND is_primary = true
	`, userID).Scan(&id)
	require.NoError(t, err)
	return id
}

func currentUserIDs(t *testing.T, ctx context.Context, tx db.Tx) map[uuid.UUID]uuid.UUID {
	t.Helper()

	rows, err := tx.Query(ctx, `SELECT id, user_id FROM addresses`)
	require.NoError(t, err)
	defer rows.Close()

	result := map[uuid.UUID]uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		var userID uuid.UUID
		require.NoError(t, rows.Scan(&id, &userID))
		result[id] = userID
	}
	require.NoError(t, rows.Err())
	return result
}

func newSenderAddressInput(userID uuid.UUID, nickname string, isPrimary bool) CreateAddressInput {
	return CreateAddressInput{
		UserID:        userID,
		Purpose:       string(addressEntity.AddressPurposeSender),
		Nickname:      nickname,
		RecipientName: "Koikoi Farm",
		Phone:         "08123456789",
		ProvinceID:    "33",
		ProvinceName:  "Jawa Tengah",
		CityID:        "3301",
		CityName:      "Kabupaten Demak",
		DistrictID:    "330101",
		DistrictName:  "Mranggen",
		VillageID:     "3301012001",
		VillageName:   "Rowosari",
		StreetAddress: "Jl. Melati No. 12",
		PostalCode:    "59511",
		Notes:         "seed",
		IsPrimary:     isPrimary,
	}
}

func assertUniqueIndexExists(t *testing.T, ctx context.Context, tx db.Tx) {
	t.Helper()

	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_indexes
			WHERE schemaname = 'public'
			  AND tablename = 'addresses'
			  AND indexname = $1
		)
	`, addressesActivePrimaryUniqueIndex).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestPrimaryAddressMigration_SafeOnCleanDatabase(t *testing.T) {
	tdb, _, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx := context.Background()
	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	assertUniqueIndexExists(t, ctx, tx)

	total, primary := activePrimaryCounts(t, ctx, tx, uuid.New())
	require.Zero(t, total)
	require.Zero(t, primary)
}

func TestPrimaryAddressMigration_SingleMissingPrimaryIsRepaired(t *testing.T) {
	tdb, _, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx := context.Background()
	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	dropActivePrimaryUniqueIndex(t, ctx, tx)

	userID := uuid.New()
	seedPrimaryInvariantUser(t, ctx, tx, userID)
	addressID := uuid.New()
	seedPrimaryInvariantAddress(
		t,
		ctx,
		tx,
		addressID,
		userID,
		time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		false,
		true,
		"Farm A",
	)

	repairPrimaryInvariant(t, ctx, tx)

	total, primary := activePrimaryCounts(t, ctx, tx, userID)
	require.EqualValues(t, 1, total)
	require.EqualValues(t, 1, primary)
	require.Equal(t, addressID, fetchPrimaryAddressID(t, ctx, tx, userID))
}

func TestPrimaryAddressMigration_MultipleMissingPrimaryIsDeterministic(t *testing.T) {
	tdb, _, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx := context.Background()
	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	dropActivePrimaryUniqueIndex(t, ctx, tx)

	userID := uuid.New()
	seedPrimaryInvariantUser(t, ctx, tx, userID)

	firstID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	secondID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	thirdID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	when := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	seedPrimaryInvariantAddress(t, ctx, tx, secondID, userID, when, false, true, "Farm B")
	seedPrimaryInvariantAddress(t, ctx, tx, thirdID, userID, when, false, true, "Farm C")
	seedPrimaryInvariantAddress(t, ctx, tx, firstID, userID, when, false, true, "Farm A")

	repairPrimaryInvariant(t, ctx, tx)

	total, primary := activePrimaryCounts(t, ctx, tx, userID)
	require.EqualValues(t, 3, total)
	require.EqualValues(t, 1, primary)
	require.Equal(t, firstID, fetchPrimaryAddressID(t, ctx, tx, userID))
}

func TestPrimaryAddressMigration_NormalizesMultiplePrimaries(t *testing.T) {
	tdb, _, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx := context.Background()
	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	dropActivePrimaryUniqueIndex(t, ctx, tx)

	userID := uuid.New()
	seedPrimaryInvariantUser(t, ctx, tx, userID)

	oldestPrimaryID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	newerPrimaryID := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	nonPrimaryID := uuid.MustParse("30000000-0000-0000-0000-000000000001")

	seedPrimaryInvariantAddress(t, ctx, tx, oldestPrimaryID, userID, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true, "Farm A")
	seedPrimaryInvariantAddress(t, ctx, tx, newerPrimaryID, userID, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), true, true, "Farm B")
	seedPrimaryInvariantAddress(t, ctx, tx, nonPrimaryID, userID, time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC), false, true, "Farm C")

	repairPrimaryInvariant(t, ctx, tx)

	total, primary := activePrimaryCounts(t, ctx, tx, userID)
	require.EqualValues(t, 3, total)
	require.EqualValues(t, 1, primary)
	require.Equal(t, oldestPrimaryID, fetchPrimaryAddressID(t, ctx, tx, userID))
}

func TestPrimaryAddressUniqueIndex_RejectsSecondActivePrimary(t *testing.T) {
	tdb, _, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New()

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		seedPrimaryInvariantUser(t, ctx, tx, userID)
		seedPrimaryInvariantAddress(t, ctx, tx, uuid.New(), userID, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true, "Farm A")
		_, insertErr := tx.Exec(ctx, `
			INSERT INTO addresses (
				id, user_id, purpose, nickname,
				recipient_name, phone,
				province_id, province_name,
				city_id, city_name,
				district_id, district_name,
				village_id, village_name,
				street_address, postal_code,
				latitude, longitude, notes,
				is_primary, is_available_for_checkout,
				created_at, updated_at
			)
			VALUES (
				$1, $2, 'sender', 'Farm B',
				'Koikoi Farm', '08123456789',
				'33', 'Jawa Tengah',
				'3301', 'Kabupaten Demak',
				'330101', 'Mranggen',
				'3301012001', 'Rowosari',
				'Jl. Melati No. 12', '59511',
				NULL, NULL, 'seed',
				true, true,
				$3, $3
			)
		`, uuid.New(), userID, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC))
		return insertErr
	})

	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr))
	require.Equal(t, "23505", pgErr.Code)
}

func TestPrimaryAddressUniqueIndex_SoftDeletedPrimaryDoesNotBlockActivePrimary(t *testing.T) {
	tdb, _, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New()

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		seedPrimaryInvariantUser(t, ctx, tx, userID)
		seedPrimaryInvariantAddress(t, ctx, tx, uuid.New(), userID, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, false, "Old Farm")
		seedPrimaryInvariantAddress(t, ctx, tx, uuid.New(), userID, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true, "Active Farm")
		return nil
	})
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, insertErr := tx.Exec(ctx, `
			INSERT INTO addresses (
				id, user_id, purpose, nickname,
				recipient_name, phone,
				province_id, province_name,
				city_id, city_name,
				district_id, district_name,
				village_id, village_name,
				street_address, postal_code,
				latitude, longitude, notes,
				is_primary, is_available_for_checkout,
				created_at, updated_at
			)
			VALUES (
				$1, $2, 'sender', 'New Primary',
				'Koikoi Farm', '08123456789',
				'33', 'Jawa Tengah',
				'3301', 'Kabupaten Demak',
				'330101', 'Mranggen',
				'3301012001', 'Rowosari',
				'Jl. Melati No. 12', '59511',
				NULL, NULL, 'seed',
				true, true,
				$3, $3
			)
		`, uuid.New(), userID, time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC))
		return insertErr
	})
	require.NoError(t, err)

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	total, primary := activePrimaryCounts(t, ctx, tx, userID)
	require.EqualValues(t, 2, total)
	require.EqualValues(t, 1, primary)
}

func TestPrimaryAddressMigration_PreservesOwnership(t *testing.T) {
	tdb, _, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx := context.Background()
	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	dropActivePrimaryUniqueIndex(t, ctx, tx)

	userA := uuid.New()
	userB := uuid.New()
	seedPrimaryInvariantUser(t, ctx, tx, userA)
	seedPrimaryInvariantUser(t, ctx, tx, userB)

	addrA1 := uuid.New()
	addrA2 := uuid.New()
	addrB1 := uuid.New()
	seedPrimaryInvariantAddress(t, ctx, tx, addrA1, userA, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), false, true, "A1")
	seedPrimaryInvariantAddress(t, ctx, tx, addrA2, userA, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true, "A2")
	seedPrimaryInvariantAddress(t, ctx, tx, addrB1, userB, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true, "B1")

	before := currentUserIDs(t, ctx, tx)
	repairPrimaryInvariant(t, ctx, tx)
	after := currentUserIDs(t, ctx, tx)

	require.Equal(t, before, after)
}

func TestPrimaryAddressService_ConcurrentFirstAddressCreation_OnePrimary(t *testing.T) {
	tdb, svc, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userID := uuid.New()
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		seedPrimaryInvariantUser(t, ctx, tx, userID)
		return nil
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	startCh := make(chan struct{})
	inputs := []CreateAddressInput{
		newSenderAddressInput(userID, "Farm A", false),
		newSenderAddressInput(userID, "Farm B", false),
	}

	for _, input := range inputs {
		input := input
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			errCh <- tdb.WithTx(ctx, func(tx db.Tx) error {
				_, err := svc.CreateAddress(ctx, tx, input)
				return err
			})
		}()
	}

	close(startCh)

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	total, primary := activePrimaryCounts(t, ctx, tx, userID)
	require.EqualValues(t, 2, total)
	require.EqualValues(t, 1, primary)
}

func TestPrimaryAddressService_ManualSetPrimary_LeavesExactlyOnePrimary(t *testing.T) {
	tdb, svc, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New()

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		seedPrimaryInvariantUser(t, ctx, tx, userID)
		seedPrimaryInvariantAddress(t, ctx, tx, uuid.MustParse("40000000-0000-0000-0000-000000000001"), userID, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true, "Farm A")
		seedPrimaryInvariantAddress(t, ctx, tx, uuid.MustParse("50000000-0000-0000-0000-000000000001"), userID, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true, "Farm B")
		return nil
	})
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return svc.SetPrimary(ctx, tx, uuid.MustParse("50000000-0000-0000-0000-000000000001"), userID)
	})
	require.NoError(t, err)

	tx, err := tdb.Pool().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	total, primary := activePrimaryCounts(t, ctx, tx, userID)
	require.EqualValues(t, 2, total)
	require.EqualValues(t, 1, primary)
	require.Equal(t, uuid.MustParse("50000000-0000-0000-0000-000000000001"), fetchPrimaryAddressID(t, ctx, tx, userID))
}

func TestPrimaryAddressService_DeletePrimary_PromotesExactlyOneRemaining(t *testing.T) {
	tdb, svc, cleanup := setupPrimaryInvariantTest(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New()
	primaryID := uuid.MustParse("60000000-0000-0000-0000-000000000001")
	oldestRemainingID := uuid.MustParse("60000000-0000-0000-0000-000000000002")
	newestRemainingID := uuid.MustParse("60000000-0000-0000-0000-000000000003")

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		seedPrimaryInvariantUser(t, ctx, tx, userID)
		seedPrimaryInvariantAddress(t, ctx, tx, primaryID, userID, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true, "Farm A")
		seedPrimaryInvariantAddress(t, ctx, tx, oldestRemainingID, userID, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true, "Farm B")
		seedPrimaryInvariantAddress(t, ctx, tx, newestRemainingID, userID, time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC), false, true, "Farm C")
		return nil
	})
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		return svc.DeleteAddress(ctx, tx, primaryID, userID)
	})
	require.NoError(t, err)

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		total, primary := activePrimaryCounts(t, ctx, tx, userID)
		require.EqualValues(t, 2, total)
		require.EqualValues(t, 1, primary)
		require.Equal(t, oldestRemainingID, fetchPrimaryAddressID(t, ctx, tx, userID))
		return nil
	})
	require.NoError(t, err)
}
