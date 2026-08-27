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

func (r *fakeAddressRepository) GetByUserIDFiltered(
	_ context.Context,
	_ db.Tx,
	userID uuid.UUID,
	purpose string,
) ([]*addressEntity.Address, error) {
	return r.activeAddresses(userID, purpose), nil
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

func (r *fakeAddressRepository) GetPrimaryByUserIDFiltered(
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
		switch address.Purpose {
		case addressEntity.AddressPurposeShipping:
			count.ShippingCount++
		case addressEntity.AddressPurposeSender:
			count.SenderCount++
		}
	}
	return count, nil
}

func (r *fakeAddressRepository) activeAddresses(userID uuid.UUID, purpose string) []*addressEntity.Address {
	addresses := make([]*addressEntity.Address, 0, len(r.addresses))
	for _, address := range r.addresses {
		if address.UserID != userID || !address.IsAvailableForCheckout {
			continue
		}
		if purpose != "" && string(address.Purpose) != purpose {
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
		Purpose:       string(addressEntity.AddressPurposeSender),
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
	purpose addressEntity.AddressPurpose,
	createdAt time.Time,
	isPrimary bool,
	isAvailable bool,
) *addressEntity.Address {
	return &addressEntity.Address{
		ID:                     id,
		UserID:                 userID,
		Purpose:                purpose,
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

func TestCreateAddress_FirstAddressBecomesPrimary(t *testing.T) {
	userID := uuid.New()
	repo := newFakeAddressRepository()
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	input := validAddressInput()
	input.UserID = userID
	input.IsPrimary = false

	address, err := svc.CreateAddress(context.Background(), addressServiceTx{}, CreateAddressInput{
		UserID:        input.UserID,
		Purpose:       input.Purpose,
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

func TestCreateAddress_RepairsMissingPrimaryToOldestActiveAddress(t *testing.T) {
	userID := uuid.New()
	oldest := makeAddress(userID, uuid.New(), addressEntity.AddressPurposeSender, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), false, true)
	newer := makeAddress(userID, uuid.New(), addressEntity.AddressPurposeSender, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(oldest, newer)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	input := validAddressInput()
	input.UserID = userID

	address, err := svc.CreateAddress(context.Background(), addressServiceTx{}, CreateAddressInput{
		UserID:        input.UserID,
		Purpose:       input.Purpose,
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
	require.True(t, repo.addresses[oldest.ID].IsPrimary)
	require.False(t, repo.addresses[newer.ID].IsPrimary)
	require.False(t, repo.addresses[address.ID].IsPrimary)
}

func TestDeleteAddress_PromotesOldestRemainingActiveAddress(t *testing.T) {
	userID := uuid.New()
	primary := makeAddress(userID, uuid.New(), addressEntity.AddressPurposeSender, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	oldestRemaining := makeAddress(userID, uuid.New(), addressEntity.AddressPurposeSender, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	newestRemaining := makeAddress(userID, uuid.New(), addressEntity.AddressPurposeSender, time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(primary, oldestRemaining, newestRemaining)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	err := svc.DeleteAddress(context.Background(), addressServiceTx{}, primary.ID, userID)

	require.NoError(t, err)
	require.False(t, repo.addresses[primary.ID].IsAvailableForCheckout)
	require.True(t, repo.addresses[oldestRemaining.ID].IsPrimary)
	require.False(t, repo.addresses[newestRemaining.ID].IsPrimary)
}

func TestGetPrimaryFiltered_UsesCanonicalPrimaryRegardlessOfPurpose(t *testing.T) {
	userID := uuid.New()
	sender := makeAddress(userID, uuid.New(), addressEntity.AddressPurposeSender, time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), true, true)
	shipping := makeAddress(userID, uuid.New(), addressEntity.AddressPurposeShipping, time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC), false, true)
	repo := newFakeAddressRepository(sender, shipping)
	svc := &AddressService{repo: repo, log: zap.NewNop()}

	address, err := svc.GetPrimaryFiltered(context.Background(), addressServiceTx{}, userID, string(addressEntity.AddressPurposeShipping))

	require.NoError(t, err)
	require.NotNil(t, address)
	require.Equal(t, sender.ID, address.ID)
	require.Equal(t, addressEntity.AddressPurposeSender, address.Purpose)
}
