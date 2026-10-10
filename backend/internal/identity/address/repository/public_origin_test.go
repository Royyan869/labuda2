package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	addressEntity "github.com/hishumi/backend/internal/identity/address/entity"
	"github.com/hishumi/backend/pkg/db"
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

func address(id uuid.UUID, city, province string, primary bool, createdAt time.Time) *addressEntity.Address {
	return &addressEntity.Address{
		ID:            id,
		CityName:      city,
		ProvinceName:  province,
		StreetAddress: "Jl. Rahasia 1",
		DistrictName:  "Kecamatan Borobudur",
		IsPrimary:     primary,
		CreatedAt:     createdAt,
	}
}

// CANONICAL RULE: the account's PRIMARY address resolves the public origin;
// when no primary is flagged, the oldest active address answers. There is no
// role (shipping/sender) narrowing and no product-level address.
func TestResolvePublicOrigin_PrimaryThenOldest(t *testing.T) {
	primaryID := uuid.New()
	olderID := uuid.New()
	newerID := uuid.New()

	cases := []struct {
		name string
		rows []*addressEntity.Address
		want string
	}{
		{
			name: "primary wins over every other address",
			rows: []*addressEntity.Address{
				address(newerID, "Sleman", "DIY", false, time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)),
				address(primaryID, "Magelang", "Jawa Tengah", true, time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)),
			},
			want: "Magelang, Jawa Tengah",
		},
		{
			name: "no primary: oldest address resolves",
			rows: []*addressEntity.Address{
				address(newerID, "Sleman", "DIY", false, time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)),
				address(olderID, "Bandung", "Jawa Barat", false, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)),
			},
			want: "Bandung, Jawa Barat",
		},
		{
			name: "single non-primary address resolves",
			rows: []*addressEntity.Address{
				address(olderID, "Depok", "Jawa Barat", false, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)),
			},
			want: "Depok, Jawa Barat",
		},
		{name: "no address hides the line", rows: nil, want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeAddressRepo{rows: tc.rows}

			got := ResolvePublicOrigin(context.Background(), stubTx{}, repo, uuid.New())
			if got != tc.want {
				t.Fatalf("ResolvePublicOrigin() = %q, want %q", got, tc.want)
			}
			// Street-level data must never reach a public surface.
			for _, leaked := range []string{"Jl. Rahasia 1", "Kecamatan Borobudur"} {
				if got == leaked {
					t.Fatalf("origin %q leaks street/district data", got)
				}
			}
		})
	}
}

// TestResolvePublicOrigin_LookupFailureHides proves an unresolvable address
// never fails the read that carries it.
func TestResolvePublicOrigin_LookupFailureHides(t *testing.T) {
	repo := &fakeAddressRepo{lookupErr: errors.New("db down")}

	got := ResolvePublicOrigin(context.Background(), stubTx{}, repo, uuid.New())
	if got != "" {
		t.Fatalf("ResolvePublicOrigin() = %q, want empty on lookup failure", got)
	}
}
