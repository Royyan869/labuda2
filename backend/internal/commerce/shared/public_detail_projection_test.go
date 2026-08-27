package shared

import (
	"testing"

	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
)

func TestBuildPublicOriginSummary_UsesCityAndProvinceOnly(t *testing.T) {
	address := &addressEntity.Address{
		StreetAddress: "Jl. Rahasia 1",
		DistrictName:  "Kecamatan Borobudur",
		CityName:      "Magelang",
		ProvinceName:  "Jawa Tengah",
		PostalCode:    "56553",
	}

	got := BuildPublicOriginSummary(address)
	if got != "Magelang, Jawa Tengah" {
		t.Fatalf("BuildPublicOriginSummary() = %q, want %q", got, "Magelang, Jawa Tengah")
	}
}

func TestBuildPublicOriginSummary_HandlesMissingValuesSafely(t *testing.T) {
	cases := []struct {
		name    string
		address *addressEntity.Address
		want    string
	}{
		{name: "nil", address: nil, want: ""},
		{
			name: "city only",
			address: &addressEntity.Address{
				CityName: "Magelang",
			},
			want: "Magelang",
		},
		{
			name: "province only",
			address: &addressEntity.Address{
				ProvinceName: "Jawa Tengah",
			},
			want: "Jawa Tengah",
		},
		{
			name: "district only is ignored",
			address: &addressEntity.Address{
				DistrictName: "Kecamatan Borobudur",
			},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildPublicOriginSummary(tc.address); got != tc.want {
				t.Fatalf("BuildPublicOriginSummary() = %q, want %q", got, tc.want)
			}
		})
	}
}
