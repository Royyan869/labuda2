package generator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func validSource() *Source {
	return &Source{
		Provinces: []Province{{ID: "12", Name: "Sumatera Utara"}},
		Regencies: []Regency{{ID: "12.04", Name: "Kabupaten Nias", ProvinceID: "12"}},
		Districts: []District{{ID: "12.04.28", Name: "Ma'u", CityID: "12.04"}},
		Villages:  []Village{{ID: "12.04.28.2001", Name: "Balodano", DistrictID: "12.04.28"}},
		Postal:    []Postal{{VillageID: "1204282001", PostalCode: "22855"}},
	}
}

func TestBuild_CompleteHierarchy(t *testing.T) {
	rows, err := Build(validSource())
	require.NoError(t, err)
	require.Len(t, rows, 4)
	require.Equal(t, Row{Code: "12", Level: LevelProvince, Name: "Sumatera Utara"}, rows[0])
	require.Equal(t, Row{Code: "1204", Level: LevelRegency, Name: "Kabupaten Nias", ParentCode: "12"}, rows[1])
	require.Equal(t, Row{Code: "120428", Level: LevelDistrict, Name: "Ma'u", ParentCode: "1204"}, rows[2])
	require.Equal(t, Row{Code: "1204282001", Level: LevelVillage, Name: "Balodano", ParentCode: "120428", PostalCode: "22855"}, rows[3])
}

// TestBuild_MissingDistrictParent_Fails is the negative proof: a village whose
// parent district is absent from the source must FAIL generation. The record
// must NOT be silently dropped to produce a deceptively complete master.
func TestBuild_MissingDistrictParent_Fails(t *testing.T) {
	src := validSource()
	// Remove the district the village depends on.
	src.Districts = nil

	rows, err := Build(src)
	require.Error(t, err)
	require.Nil(t, rows)
	require.Contains(t, err.Error(), "MISSING PARENT")
	require.Contains(t, err.Error(), "village 1204282001")
	require.Contains(t, err.Error(), "120428")
}

func TestBuild_MissingRegencyParent_Fails(t *testing.T) {
	src := validSource()
	src.Provinces = nil

	rows, err := Build(src)
	require.Error(t, err)
	require.Nil(t, rows)
	require.Contains(t, err.Error(), "MISSING PARENT")
	require.Contains(t, err.Error(), "regency 1204")
	require.Contains(t, err.Error(), "12")
}

func TestBuild_MissingProvinceOfRegency_Fails(t *testing.T) {
	src := validSource()
	src.Provinces = []Province{{ID: "31", Name: "DKI Jakarta"}} // wrong province

	rows, err := Build(src)
	require.Error(t, err)
	require.Nil(t, rows)
	require.Contains(t, err.Error(), "MISSING PARENT")
	require.Contains(t, err.Error(), "regency 1204")
}

func TestBuild_DuplicateCode_Fails(t *testing.T) {
	src := validSource()
	src.Districts = append(src.Districts, District{ID: "12.04.28", Name: "Duplicate", CityID: "12.04"})

	_, err := Build(src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate geography code")
}

func TestBuild_BadCodeLength_Fails(t *testing.T) {
	src := validSource()
	src.Districts = []District{{ID: "12.04.2", Name: "Bad", CityID: "12.04"}} // 5 digits after norm

	_, err := Build(src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "length")
}

func TestBuild_EmptyName_Fails(t *testing.T) {
	src := validSource()
	src.Districts = []District{{ID: "12.04.28", Name: "   ", CityID: "12.04"}}

	_, err := Build(src)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "empty name")
}
