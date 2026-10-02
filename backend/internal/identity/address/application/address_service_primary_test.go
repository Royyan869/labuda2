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
	return r.activeAddresses(userID, ""), nil
}

// GetByUserIDForDisplay is the public-display read (no checkout filter);
// the fake mirrors GetByUserID so the display contract and the list contract
// stay aligned inside these tests.
func (r *fakeAddressRepository) GetByUserIDForDisplay(ctx context.Context, tx db.Tx, userID uuid.UUID) ([]*addressEntity.Address, error) {
	return r.GetByUserID(ctx, tx, userID)
}

func (r *fakeAddressRepository) GetByUserIDFiltered(
	_ context.Context,
	_ db.Tx,
	userID uuid.UUID,
	tag string,
) ([]*addressEntity.Address, error) {
	return r.activeAddresses(userID, tag), nil
}

func (r *fakeAddressRepository) GetPrimaryByUserID(
	_ context.Context,
	_ db.Tx,
	userID uuid.UUID,
) (*addressEntity.Address, error) {
	for _, address := range r.activeAddresses(userID, "") {
		if address.IsPrimary {
			return address, nil
		}
	}
	return nil, nil
}

func (r *fakeAddressRepository) GetPrimaryByTag(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
	_ string,
) (*addressEntity.Address, error) {
	return r.GetPrimaryByUserID(ctx, tx, userID)
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
	count := &addressRepoInterface.AddressCount{}
	for _, address := range r.addresses {
		if address.UserID != userID || !address.IsAvailableForCheckout {
			continue
		}
		count.Total++
		if address.HasTag(addressEntity.TagShipping) {
			count.ShippingCount++
		}
		if address.HasTag(addressEntity.TagSender) {
			count.SenderCount++
		}
	}
	return count, nil
}

// activeAddresses narrows a user's active rows to those carrying the tag
// (an empty tag means "no narrowing").
func (r *fakeAddressRepository) activeAddresses(userID uuid.UUID, tag string) []*addressEntity.Address {
	addresses := make([]*addressEntity.Address, 0, len(r.addresses))
	for _, address := range r.addresses {
		if address.UserID != userID || !address.IsAvailableForCheckout {
			continue
		}
		if tag != "" && !address.HasTag(addressEntity.AddressTag(tag)) {
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
		Tags:         []string{string(addressEntity.TagSender)},
		Nickname:      "Farm",
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
	tags []addressEntity.AddressTag,
	createdAt time.Time,
	isPrimary bool,
	isAvailable bool,
) *addressEntity.Address {
	return &addressEntity.Address{
		ID:                     id,
		UserID:                 userID,
		Tags:                   tags,
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

// TestCreateAddress_WithIsPrimaryTrue_PersistsAsPrimary verifies the current
// service contract: when CreateAddress is called with IsPrimary=true the new
// address is stored as primary (and existing primaries are unset).
func TestCreateAddress_WithIsPrimaryTrue_PersistsAsPrimary(t *testing.T) {
	userID := uuid.New()
	repo := newFakeAddressRepository()
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	input := validAddressInput()
	input.UserID = userID
	input.IsPrimary = true

	address, err := svc.CreateAddress(context.Background(), addressServiceTx{},		CreateAddressInput{
			UserID:        input.UserID,
			Tags:          input.Tags,
			Nickname:      input.Nickname,
		RecipientName: input.RecipientName,
		Phone:         input.Phone,
		ProvinceID:    input.ProvinceID,
		ProvinceName:  input.ProvinceName,
		CityID:        input.CityID,
		CityName:      input.CityName,
		DistrictID:    input.DistrictID,
		DistrictName:  input.DistrictName,
		VillageID:     input.VillageID,
		VillageName:   input.VillageName,
		StreetAddress: input.StreetAddress,
		PostalCode:    input.PostalCode,
		Notes:         input.Notes,
		IsPrimary:     input.IsPrimary,
	})

	require.NoError(t, err)
	require.NotNil(t, address)
	require.True(t, address.IsPrimary)
	require.True(t, repo.addresses[address.ID].IsPrimary)
}

// TestCreateAddress_FirstAddressBecomesPrimaryWithBothTags locks the A2
// write law: when the account owns no other active address, the row being
// created becomes its everything — primary flag plus both role tags — no
// matter what the caller supplied.
func TestCreateAddress_FirstAddressBecomesPrimaryWithBothTags(t *testing.T) {
	userID := uuid.New()
	repo := newFakeAddressRepository()
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	input := validAddressInput()
	input.UserID = userID
	input.IsPrimary = false

	address, err := svc.CreateAddress(context.Background(), addressServiceTx{},		CreateAddressInput{
			UserID:        input.UserID,
			Tags:          input.Tags,
			Nickname:      input.Nickname,
		RecipientName: input.RecipientName,
		Phone:         input.Phone,
		ProvinceID:    input.ProvinceID,
		ProvinceName:  input.ProvinceName,
		CityID:        input.CityID,
		CityName:      input.CityName,
		DistrictID:    input.DistrictID,
		DistrictName:  input.DistrictName,
		VillageID:     input.VillageID,
		VillageName:   input.VillageName,
		StreetAddress: input.StreetAddress,
		PostalCode:    input.PostalCode,
		Notes:         input.Notes,
		IsPrimary:     input.IsPrimary,
	})

	require.NoError(t, err)
	require.NotNil(t, address)
	require.True(t, address.IsPrimary, "the first address of an account is auto-primary")
	require.True(t, repo.addresses[address.ID].IsPrimary)
	require.True(t, address.HasTag(addressEntity.TagShipping), "the sole address carries both roles")
	require.True(t, address.HasTag(addressEntity.TagSender))
}

// TestCreateAddress_SecondAddressKeepsCallerChoice locks the other half of
// A2: once the account owns another active address, tags and the primary
// flag stay exactly what the caller chose — no forcing.
func TestCreateAddress_SecondAddressKeepsCallerChoice(t *testing.T) {
	userID := uuid.New()
	existing := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagShipping, addressEntity.TagSender}, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	repo := newFakeAddressRepository(existing)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	input := validAddressInput()
	input.UserID = userID
	input.IsPrimary = false

	address, err := svc.CreateAddress(context.Background(), addressServiceTx{}, input)

	require.NoError(t, err)
	require.False(t, address.IsPrimary, "a non-first address does not steal the primary")
	require.False(t, repo.addresses[address.ID].IsPrimary)
	require.Equal(t, []addressEntity.AddressTag{addressEntity.TagSender}, address.Tags, "tags stay the caller's choice at ≥2")
	require.True(t, repo.addresses[existing.ID].IsPrimary)
}

// TestCreateAddress_WithIsPrimaryTrue_UnsetsExistingPrimary verifies the
// current service contract: creating a new primary unsets the user's existing
// primary before persisting.
func TestCreateAddress_WithIsPrimaryTrue_UnsetsExistingPrimary(t *testing.T) {
	userID := uuid.New()
	existing := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagSender}, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	repo := newFakeAddressRepository(existing)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	input := validAddressInput()
	input.UserID = userID
	input.IsPrimary = true

	address, err := svc.CreateAddress(context.Background(), addressServiceTx{},		CreateAddressInput{
			UserID:        input.UserID,
			Tags:          input.Tags,
			Nickname:      input.Nickname,
		RecipientName: input.RecipientName,
		Phone:         input.Phone,
		ProvinceID:    input.ProvinceID,
		ProvinceName:  input.ProvinceName,
		CityID:        input.CityID,
		CityName:      input.CityName,
		DistrictID:    input.DistrictID,
		DistrictName:  input.DistrictName,
		VillageID:     input.VillageID,
		VillageName:   input.VillageName,
		StreetAddress: input.StreetAddress,
		PostalCode:    input.PostalCode,
		Notes:         input.Notes,
		IsPrimary:     input.IsPrimary,
	})

	require.NoError(t, err)
	require.True(t, address.IsPrimary)
	require.False(t, repo.addresses[existing.ID].IsPrimary, "previous primary must be unset")
	require.True(t, repo.addresses[address.ID].IsPrimary)
}

// TestDeleteAddress_PromotesOldestRemainingPrimary locks A2 after a delete:
// deleting the primary must leave exactly one primary behind — the oldest
// remaining active address inherits it.
func TestDeleteAddress_PromotesOldestRemainingPrimary(t *testing.T) {
	userID := uuid.New()
	primary := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagSender}, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	oldestRemaining := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagSender}, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	newestRemaining := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagSender}, time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC), false, true)
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
	only := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagShipping, addressEntity.TagSender}, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	repo := newFakeAddressRepository(only)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	err := svc.DeleteAddress(context.Background(), addressServiceTx{}, only.ID, userID)

	require.NoError(t, err)
	require.False(t, repo.addresses[only.ID].IsAvailableForCheckout)
	active, getErr := repo.GetByUserID(context.Background(), addressServiceTx{}, userID)
	require.NoError(t, getErr)
	require.Empty(t, active, "an empty address book is legal")
}

// TestReconcile_SoleRemainingAddressBecomesEverything: when a delete leaves
// exactly one active address behind, that address inherits BOTH roles and
// the primary flag — no matter how it was tagged before.
func TestReconcile_SoleRemainingAddressBecomesEverything(t *testing.T) {
	userID := uuid.New()
	primary := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagSender}, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	shippingOnly := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagShipping}, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(primary, shippingOnly)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	err := svc.DeleteAddress(context.Background(), addressServiceTx{}, primary.ID, userID)

	require.NoError(t, err)
	remaining := repo.addresses[shippingOnly.ID]
	require.True(t, remaining.IsPrimary, "the sole survivor becomes the primary")
	require.True(t, remaining.HasTag(addressEntity.TagShipping))
	require.True(t, remaining.HasTag(addressEntity.TagSender), "the sole survivor carries both roles")
}

// TestReconcile_PromotesOldestWhenNoPrimary: pre-reconciler data with ≥2
// active addresses and no primary gets exactly one — the oldest row.
func TestReconcile_PromotesOldestWhenNoPrimary(t *testing.T) {
	userID := uuid.New()
	older := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagShipping}, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), false, true)
	newer := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagSender}, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(newer, older)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	err := svc.reconcile(context.Background(), addressServiceTx{}, userID)

	require.NoError(t, err)
	require.True(t, repo.addresses[older.ID].IsPrimary, "oldest active address is promoted")
	require.False(t, repo.addresses[newer.ID].IsPrimary)
	// ≥2 keeps the user's tags untouched.
	require.Equal(t, []addressEntity.AddressTag{addressEntity.TagShipping}, repo.addresses[older.ID].Tags)
}

// The primary flag is a single account-wide resource. Narrowing the read to
// the "shipping" tag must NOT invent a second primary: the account's one
// primary (here, tagged sender) is what comes back.
func TestGetPrimaryFiltered_UsesCanonicalPrimaryRegardlessOfTag(t *testing.T) {
	userID := uuid.New()
	sender := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagSender}, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	shipping := makeAddress(userID, uuid.New(), []addressEntity.AddressTag{addressEntity.TagShipping}, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(sender, shipping)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	address, err := svc.GetPrimaryFiltered(context.Background(), addressServiceTx{}, userID, string(addressEntity.TagShipping))

	require.NoError(t, err)
	require.NotNil(t, address)
	require.Equal(t, sender.ID, address.ID)
	require.Equal(t, []addressEntity.AddressTag{addressEntity.TagSender}, address.Tags)
}

// An address may serve both roles at once: tags are a set, not a choice.
func TestAddress_CarriesShippingAndSenderTagsTogether(t *testing.T) {
	userID := uuid.New()
	dual := makeAddress(
		userID,
		uuid.New(),
		[]addressEntity.AddressTag{addressEntity.TagShipping, addressEntity.TagSender},
		time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		true,
		true,
	)

	require.True(t, dual.HasTag(addressEntity.TagShipping))
	require.True(t, dual.HasTag(addressEntity.TagSender))

	// Both tag-narrowed reads resolve the same row.
	repo := newFakeAddressRepository(dual)
	require.Len(t, repo.activeAddresses(userID, "shipping"), 1)
	require.Len(t, repo.activeAddresses(userID, "sender"), 1)

	// And both counts pick it up while Total still counts one address.
	count, err := repo.CountByUserID(context.Background(), addressServiceTx{}, userID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count.Total)
	require.Equal(t, int64(1), count.ShippingCount)
	require.Equal(t, int64(1), count.SenderCount)
}
