package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/pkg/db"
)

// stubTx satisfies db.Tx without a live connection: the fake never touches
// the transaction, it only proves which rows the rule was allowed to see.
type stubTx struct{ db.Tx }

type fakeAddressRepo struct {
	AddressRepository

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

func senderAddress(id uuid.UUID, city string, primary bool) *addressEntity.Address {
	return &addressEntity.Address{
		ID:            id,
		Tags:          []addressEntity.AddressTag{addressEntity.TagSender},
		CityName:      city,
		ProvinceName:  "Jawa Tengah",
		StreetAddress: "Jl. Rahasia 1",
		DistrictName:  "Kecamatan Borobudur",
		IsPrimary:     primary,
	}
}

func shippingAddress(city string, primary bool) *addressEntity.Address {
	return &addressEntity.Address{
		Tags:         []addressEntity.AddressTag{addressEntity.TagShipping},
		CityName:     city,
		ProvinceName: "Jawa Barat",
		IsPrimary:    primary,
	}
}

// OWNER RULE (both surfaces): primary first, sender as fallback — and a user
// who is not yet a seller still gets their address shown.
func TestResolvePublicOrigin_PrimaryFirstSenderFallback(t *testing.T) {
	productAddressID := uuid.New()
	otherID := uuid.New()

	primarySender := senderAddress(otherID, "Magelang", true)
	sender := senderAddress(productAddressID, "Sleman", false)

	cases := []struct {
		name    string
		rows    []*addressEntity.Address
		want    string
		product *uuid.UUID
	}{
		{
			name: "primary sender wins over the product address",
			rows: []*addressEntity.Address{sender, primarySender},
			product: func() *uuid.UUID {
				return &productAddressID
			}(),
			want: "Magelang, Jawa Tengah",
		},
		{
			name: "product sender address used when no primary exists",
			rows: []*addressEntity.Address{sender},
			product: func() *uuid.UUID {
				return &productAddressID
			}(),
			want: "Sleman, Jawa Tengah",
		},
		{
			name: "profile with one non-primary sender resolves",
			rows: []*addressEntity.Address{sender},
			want: "Sleman, Jawa Tengah",
		},
		{
			name:    "not a seller: primary shipping address wins",
			rows:    []*addressEntity.Address{shippingAddress("Bandung", true), shippingAddress("Depok", false)},
			want:    "Bandung, Jawa Barat",
		},
		{
			name: "not a seller: any shipping address still resolves",
			rows: []*addressEntity.Address{shippingAddress("Depok", false)},
			want: "Depok, Jawa Barat",
		},
		{
			name: "not a seller: any address at all, primary first",
			rows: []*addressEntity.Address{
				{
					Tags:         []addressEntity.AddressTag{addressEntity.TagShipping},
					CityName:     "Depok",
					ProvinceName: "Jawa Barat",
				},
			},
			want: "Depok, Jawa Barat",
		},
		{name: "no address hides the line", rows: nil, want: ""},
		{name: "lookup failure hides the line", rows: nil, want: "", product: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeAddressRepo{rows: tc.rows}
			if tc.name == "lookup failure hides the line" {
				repo.lookupErr = errors.New("db down")
			}

			got := ResolvePublicOrigin(context.Background(), stubTx{}, repo, uuid.New(), tc.product)
			if got != tc.want {
				t.Fatalf("ResolvePublicOrigin() = %q, want %q", got, tc.want)
			}
			// Street-level data must never reach a public surface, whatever
			// path resolved the address.
			for _, leaked := range []string{"Jl. Rahasia 1", "Kecamatan Borobudur"} {
				if got != "" && got == leaked {
					t.Fatalf("origin %q leaks street/district data", got)
				}
			}
		})
	}
}
