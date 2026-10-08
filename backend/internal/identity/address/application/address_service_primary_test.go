package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepoInterface "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type addressServiceTx struct{}

func (addressServiceTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (addressServiceTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (addressServiceTx) QueryRow(context.Context, string, ...any) pgx.Row        { return pgx.Row(nil) }
func (addressServiceTx) Commit(context.Context) error                            { return nil }
func (addressServiceTx) Rollback(context.Context) error                          { return nil }

var _ db.Tx = (*addressServiceTx)(nil)

type fakeAddressRepository struct {
	addresses map[uuid.UUID]*addressEntity.Address
}

func newFakeAddressRepository(addresses ...*addressEntity.Address) *fakeAddressRepository {
	repo := &fakeAddressRepository{
		addresses: make(map[uuid.UUID]*addressEntity.Address, len(addresses)),
	}
	for _, address := range addresses {
		repo.addresses[address.ID] = cloneAddress(address)
	}
	return repo
}

func (r *fakeAddressRepository) Create(_ context.Context, _ db.Tx, address *addressEntity.Address) error {
	r.addresses[address.ID] = cloneAddress(address)
	return nil
}

func (r *fakeAddressRepository) GetByID(_ context.Context, _ db.Tx, id uuid.UUID) (*addressEntity.Address, error) {
	address, ok := r.addresses[id]
	if !ok {
		return nil, &addressEntity.AddressNotFoundError{ID: id}
	}
	return cloneAddress(address), nil
}

func (r *fakeAddressRepository) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*addressEntity.Address, error) {
	return r.GetByID(ctx, tx, id)
}

func (r *fakeAddressRepository) Update(_ context.Context, _ db.Tx, address *addressEntity.Address) error {
	r.addresses[address.ID] = cloneAddress(address)
	return nil
}

func (r *fakeAddressRepository) Delete(_ context.Context, _ db.Tx, id uuid.UUID) error {
	address, ok := r.addresses[id]
	if !ok {
		return &addressEntity.AddressNotFoundError{ID: id}
	}
	address.IsAvailableForCheckout = false
	address.UpdatedAt = time.Now()
	return nil
}

func (r *fakeAddressRepository) GetByUserID(_ context.Context, _ db.Tx, userID uuid.UUID) ([]*addressEntity.Address, error) {
	return r.activeAddresses(userID), nil
}

// GetByUserIDForDisplay is the public-display read (no checkout filter);
// the fake mirrors GetByUserID so the display contract and the list contract
// stay aligned inside these tests.
func (r *fakeAddressRepository) GetByUserIDForDisplay(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]*addressEntity.Address, error) {
	return r.GetByUserID(ctx, tx, userID)
}

func (r *fakeAddressRepository) GetPrimaryByUserID(
	_ context.Context,
	_ db.Tx,
	userID uuid.UUID,
) (*addressEntity.Address, error) {
	for _, address := range r.activeAddresses(userID) {
		if address.IsPrimary {
			return address, nil
		}
	}
	return nil, nil
}

func (r *fakeAddressRepository) SetPrimary(_ context.Context, _ db.Tx, addressID uuid.UUID) error {
	target, ok := r.addresses[addressID]
	if !ok {
		return &addressEntity.AddressNotFoundError{ID: addressID}
	}
	for _, address := range r.addresses {
		if address.UserID == target.UserID {
			address.IsPrimary = false
			address.UpdatedAt = time.Now()
		}
	}
	target.IsPrimary = true
	target.UpdatedAt = time.Now()
	return nil
}

func (r *fakeAddressRepository) UnsetAllPrimary(_ context.Context, _ db.Tx, userID uuid.UUID) error {
	for _, address := range r.addresses {
		if address.UserID == userID {
			address.IsPrimary = false
			address.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (r *fakeAddressRepository) CountByUserID(
	_ context.Context,
	_ db.Tx,
	userID uuid.UUID,
) (*addressRepoInterface.AddressCount, error) {
	var total int64
	total = int64(len(r.activeAddresses(userID)))
	return &addressRepoInterface.AddressCount{Total: total}, nil
}

// activeAddresses returns a user's active rows.
func (r *fakeAddressRepository) activeAddresses(userID uuid.UUID) []*addressEntity.Address {
	addresses := make([]*addressEntity.Address, 0, len(r.addresses))
	for _, address := range r.addresses {
		if address.UserID != userID || !address.IsAvailableForCheckout {
			continue
		}
		addresses = append(addresses, cloneAddress(address))
	}
	return addresses
}

func cloneAddress(address *addressEntity.Address) *addressEntity.Address {
	if address == nil {
		return nil
	}
	clone := *address
	return &clone
}

func validAddressInput() CreateAddressInput {
	return CreateAddressInput{
		Nickname:      "Rumah",
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
		Notes:         "Rear gate",
	}
}

func makeAddress(
	userID uuid.UUID,
	id uuid.UUID,
	createdAt time.Time,
	isPrimary bool,
	isAvailable bool,
) *addressEntity.Address {
	return &addressEntity.Address{
		ID:                     id,
		UserID:                 userID,
		RecipientName:          "Koikoi Farm",
		Phone:                  "08123456789",
		ProvinceID:             "33",
		ProvinceName:           "Jawa Tengah",
		CityID:                 "3301",
		CityName:               "Kabupaten Demak",
		DistrictID:             "330101",
		DistrictName:           "Mranggen",
		VillageID:              "3301012001",
		VillageName:            "Rowosari",
		StreetAddress:          "Jl. Melati No. 12",
		PostalCode:             "59511",
		Notes:                  "Rear gate",
		IsPrimary:              isPrimary,
		IsAvailableForCheckout: isAvailable,
		CreatedAt:              createdAt,
		UpdatedAt:              createdAt,
	}
}

// TestCreateAddress_FirstAddressBecomesPrimary locks the lifecycle law: when
// the account owns no other active address, the row being created becomes the
// primary no matter what the caller supplied.
func TestCreateAddress_FirstAddressBecomesPrimary(t *testing.T) {
	userID := uuid.New()
	repo := newFakeAddressRepository()
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	input := validAddressInput()
	input.UserID = userID
	input.IsPrimary = false

	address, err := svc.CreateAddress(context.Background(), addressServiceTx{}, input)

	require.NoError(t, err)
	require.NotNil(t, address)
	require.True(t, address.IsPrimary, "the first address of an account is auto-primary")
	require.True(t, repo.addresses[address.ID].IsPrimary)
}

// TestCreateAddress_SecondAddressKeepsPrimary locks the other half: once the
// account owns another active address, the new address does not steal the
// existing primary.
func TestCreateAddress_SecondAddressKeepsPrimary(t *testing.T) {
	userID := uuid.New()
	existing := makeAddress(userID, uuid.New(), time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	repo := newFakeAddressRepository(existing)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	input := validAddressInput()
	input.UserID = userID
	input.IsPrimary = false

	address, err := svc.CreateAddress(context.Background(), addressServiceTx{}, input)

	require.NoError(t, err)
	require.False(t, address.IsPrimary, "a non-first address does not steal the primary")
	require.False(t, repo.addresses[address.ID].IsPrimary)
	require.True(t, repo.addresses[existing.ID].IsPrimary)
}

// TestSetPrimary_ChangesPrimary locks the explicit "Jadikan alamat utama"
// lifecycle: the previous primary is unset and the chosen address becomes the
// only primary.
func TestSetPrimary_ChangesPrimary(t *testing.T) {
	userID := uuid.New()
	first := makeAddress(userID, uuid.New(), time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	second := makeAddress(userID, uuid.New(), time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(first, second)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	err := svc.SetPrimary(context.Background(), addressServiceTx{}, second.ID, userID)

	require.NoError(t, err)
	require.False(t, repo.addresses[first.ID].IsPrimary)
	require.True(t, repo.addresses[second.ID].IsPrimary)
}

// TestDeleteNonPrimary_PreservesPrimary: deleting a non-primary leaves the
// current primary untouched.
func TestDeleteNonPrimary_PreservesPrimary(t *testing.T) {
	userID := uuid.New()
	primary := makeAddress(userID, uuid.New(), time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	other := makeAddress(userID, uuid.New(), time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(primary, other)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	err := svc.DeleteAddress(context.Background(), addressServiceTx{}, other.ID, userID)

	require.NoError(t, err)
	require.True(t, repo.addresses[primary.ID].IsPrimary)
	require.False(t, repo.addresses[other.ID].IsAvailableForCheckout)
}

// TestDeleteAddress_PromotesOldestRemainingPrimary locks the surviving-primary
// selection rule: deleting the primary promotes the oldest remaining active
// address (created_at ASC, id ASC).
func TestDeleteAddress_PromotesOldestRemainingPrimary(t *testing.T) {
	userID := uuid.New()
	primary := makeAddress(userID, uuid.New(), time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	oldestRemaining := makeAddress(userID, uuid.New(), time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	newestRemaining := makeAddress(userID, uuid.New(), time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(primary, oldestRemaining, newestRemaining)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	err := svc.DeleteAddress(context.Background(), addressServiceTx{}, primary.ID, userID)

	require.NoError(t, err)
	require.False(t, repo.addresses[primary.ID].IsAvailableForCheckout)
	require.True(t, repo.addresses[oldestRemaining.ID].IsPrimary, "oldest remaining inherits the primary")
	require.False(t, repo.addresses[newestRemaining.ID].IsPrimary)
}

// TestDeleteAddress_LastAddressDeletedLeavesBookEmpty: 0 active addresses is
// a legal state — the reconciler forces nothing onto an empty book.
func TestDeleteAddress_LastAddressDeletedLeavesBookEmpty(t *testing.T) {
	userID := uuid.New()
	only := makeAddress(userID, uuid.New(), time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	repo := newFakeAddressRepository(only)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	err := svc.DeleteAddress(context.Background(), addressServiceTx{}, only.ID, userID)

	require.NoError(t, err)
	require.False(t, repo.addresses[only.ID].IsAvailableForCheckout)
	active, getErr := repo.GetByUserID(context.Background(), addressServiceTx{}, userID)
	require.NoError(t, getErr)
	require.Empty(t, active, "an empty address book is legal")
}

// TestReconcile_PromotesOldestWhenNoPrimary: pre-reconciler data with ≥2
// active addresses and no primary gets exactly one — the oldest row.
func TestReconcile_PromotesOldestWhenNoPrimary(t *testing.T) {
	userID := uuid.New()
	older := makeAddress(userID, uuid.New(), time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), false, true)
	newer := makeAddress(userID, uuid.New(), time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(newer, older)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	err := svc.reconcile(context.Background(), addressServiceTx{}, userID)

	require.NoError(t, err)
	require.True(t, repo.addresses[older.ID].IsPrimary, "oldest active address is promoted")
	require.False(t, repo.addresses[newer.ID].IsPrimary)
}

// TestGetPrimaryFiltered_ReturnsAccountPrimary locks that the primary read is
// account-wide with no role narrowing.
func TestGetPrimaryFiltered_ReturnsAccountPrimary(t *testing.T) {
	userID := uuid.New()
	primary := makeAddress(userID, uuid.New(), time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	other := makeAddress(userID, uuid.New(), time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(primary, other)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	address, err := svc.GetPrimaryFiltered(context.Background(), addressServiceTx{}, userID)

	require.NoError(t, err)
	require.NotNil(t, address)
	require.Equal(t, primary.ID, address.ID)
}

// TestCountByUserID_CountsActiveAddresses proves the count is a plain active
// address count.
func TestCountByUserID_CountsActiveAddresses(t *testing.T) {
	userID := uuid.New()
	repo := newFakeAddressRepository(
		makeAddress(userID, uuid.New(), time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true),
		makeAddress(userID, uuid.New(), time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true),
	)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	count, err := svc.CountByUserID(context.Background(), addressServiceTx{}, userID)

	require.NoError(t, err)
	require.Equal(t, int64(2), count.Total)
}
