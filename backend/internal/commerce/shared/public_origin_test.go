package shared

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	productEntity "github.com/hishumi/backend/internal/commerce/product/entity"
	addressEntity "github.com/hishumi/backend/internal/identity/address/entity"
	addressRepo "github.com/hishumi/backend/internal/identity/address/repository"
	"github.com/hishumi/backend/pkg/db"
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
// never street, district, recipient, phone or coordinates — and it comes from
// the account's primary address. There is no product-level origin.
func TestPublicListingOrigin_UsesAccountPrimary(t *testing.T) {
	product := &productEntity.Product{ID: uuid.New(), SellerID: uuid.New()}

	primary := &addressEntity.Address{
		CityName:      "Magelang",
		ProvinceName:  "Jawa Tengah",
		StreetAddress: "Jl. Kantor 7",
		IsPrimary:     true,
		CreatedAt:     time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	}
	other := &addressEntity.Address{
		CityName:     "Sleman",
		ProvinceName: "Jawa Tengah",
		IsPrimary:    false,
		CreatedAt:    time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}

	repo := &fakeAddressRepo{rows: []*addressEntity.Address{other, primary}}

	if got := PublicListingOrigin(context.Background(), stubTx{}, repo, product); got != "Magelang, Jawa Tengah" {
		t.Fatalf("PublicListingOrigin() = %q, want the PRIMARY origin %q", got, "Magelang, Jawa Tengah")
	}
}

// Missing truth HIDES the line: an absent product or an unresolvable address
// yields an empty origin, never a fabricated one, and never an error that
// would fail the detail read.
func TestPublicListingOrigin_HidesWhenTruthIsMissing(t *testing.T) {
	product := &productEntity.Product{ID: uuid.New(), SellerID: uuid.New()}

	cases := []struct {
		name    string
		product *productEntity.Product
		repo    addressRepo.AddressRepository
	}{
		{name: "nil product", product: nil, repo: &fakeAddressRepo{}},
		{name: "no address rows", product: product, repo: &fakeAddressRepo{}},
		{
			name:    "lookup failure",
			product: product,
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
