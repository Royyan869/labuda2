package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepo "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/pkg/db"
)

// stubTx satisfies db.Tx without a live connection: the fake never touches
// the transaction, it only proves which rows the rule was allowed to see.
type stubTx struct{ db.Tx }

type fakeAddressRepo struct {
	addressRepo.AddressRepository

	rows      []*addressEntity.Address
	lookupErr error
}

func (f *fakeAddressRepo) GetByUserIDForDisplay(
	_ context.Context,
	_ db.Tx,
	_ uuid.UUID,
) ([]*addressEntity.Address, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	return f.rows, nil
}

// CANONICAL TRUTH (owner-locked): buyer-visible origin is "City, Province" —
// never street, district, recipient, phone or coordinates.
//
// RESOLUTION ORDER (owner rule, identical on the public profile): the
// seller's PRIMARY sender address wins; the product's own sender address is
// the fallback when no primary flag exists. Both surfaces delegate to
// identity/address/repository.ResolvePublicOrigin, so a listing card and the
// seller's profile can never show different origins.
func TestPublicListingOrigin_PrefersPrimaryThenProductAddress(t *testing.T) {
	farmAddressID := uuid.New()
	product := &productEntity.Product{
		ID:            uuid.New(),
		SellerID:      uuid.New(),
		FarmAddressID: &farmAddressID,
	}

	primary := &addressEntity.Address{
		CityName:      "Magelang",
		ProvinceName:  "Jawa Tengah",
		StreetAddress: "Jl. Kantor 7",
		IsPrimary:     true,
		Tags:          []addressEntity.AddressTag{addressEntity.TagSender},
	}
	productAddress := &addressEntity.Address{
		ID:            farmAddressID,
		StreetAddress: "Jl. Rahasia 1",
		DistrictName:  "Kecamatan Borobudur",
		CityName:      "Sleman",
		ProvinceName:  "Jawa Tengah",
		RecipientName: "Budi",
		Phone:         "081200000000",
		Tags:          []addressEntity.AddressTag{addressEntity.TagSender},
	}

	repo := &fakeAddressRepo{rows: []*addressEntity.Address{productAddress, primary}}

	if got := PublicListingOrigin(context.Background(), stubTx{}, repo, product); got != "Magelang, Jawa Tengah" {
		t.Fatalf("PublicListingOrigin() = %q, want the PRIMARY sender origin %q", got, "Magelang, Jawa Tengah")
	}

	// Same product, account without a primary flag: the product's own sender
	// address takes over — still city + province only.
	repo.rows = []*addressEntity.Address{productAddress}
	if got := PublicListingOrigin(context.Background(), stubTx{}, repo, product); got != "Sleman, Jawa Tengah" {
		t.Fatalf("PublicListingOrigin() = %q, want the product sender origin %q", got, "Sleman, Jawa Tengah")
	}
}

func TestPublicListingOrigin_FallsBackToSellerPrimarySenderAddress(t *testing.T) {
	product := &productEntity.Product{ID: uuid.New(), SellerID: uuid.New()}

	repo := &fakeAddressRepo{
		rows: []*addressEntity.Address{
			{
				CityName:     "Bandung",
				ProvinceName: "Jawa Barat",
				IsPrimary:    true,
				Tags:          []addressEntity.AddressTag{addressEntity.TagSender},
			},
		},
	}

	got := PublicListingOrigin(context.Background(), stubTx{}, repo, product)
	if got != "Bandung, Jawa Barat" {
		t.Fatalf("PublicListingOrigin() = %q, want %q", got, "Bandung, Jawa Barat")
	}
}

// Missing truth HIDES the line: an absent product or an unresolvable address
// yields an empty origin, never a fabricated one, and never an error that
// would fail the detail read.
func TestPublicListingOrigin_HidesWhenTruthIsMissing(t *testing.T) {
	productWithMissingAddress := &productEntity.Product{
		ID:            uuid.New(),
		SellerID:      uuid.New(),
		FarmAddressID: func() *uuid.UUID { id := uuid.New(); return &id }(),
	}

	cases := []struct {
		name    string
		product *productEntity.Product
		repo    addressRepo.AddressRepository
	}{
		{name: "nil product", product: nil, repo: &fakeAddressRepo{}},
		{name: "no address rows", product: productWithMissingAddress, repo: &fakeAddressRepo{}},
		{
			name:    "lookup failure",
			product: productWithMissingAddress,
			repo:    &fakeAddressRepo{lookupErr: errors.New("db down")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PublicListingOrigin(context.Background(), stubTx{}, tc.repo, tc.product); got != "" {
				t.Fatalf("PublicListingOrigin() = %q, want empty (hide, never fabricate)", got)
			}
		})
	}
}
