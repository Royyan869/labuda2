package entity

import (
	"encoding/json"
	"testing"
)

func TestAddressSnapshot_MarshalJSON_SnakeCaseKeys(t *testing.T) {
	snap := AddressSnapshot{
		RecipientName: "Ali Budiman",
		Phone:         "081234567890",
		ProvinceID:    "32",
		ProvinceName:  "Jawa Barat",
		CityID:        "3204",
		CityName:      "Bandung",
		DistrictID:    "320401",
		DistrictName:  "Coblong",
		VillageID:     "3204011001",
		VillageName:   "Dago",
		StreetAddress: "Jl. Ir. H. Juanda No. 100",
		PostalCode:    "40135",
	}

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify snake_case keys are present
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	requiredKeys := []string{
		"recipient_name", "phone",
		"province_id", "province_name",
		"city_id", "city_name",
		"district_id", "district_name",
		"village_id", "village_name",
		"street_address", "postal_code",
	}

	for _, key := range requiredKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected snake_case key %q in JSON output, got keys: %v", key, keys(raw))
		}
	}

	// Verify PascalCase keys are NOT present
	pascalKeys := []string{
		"RecipientName", "Phone", "ProvinceID", "ProvinceName",
		"CityID", "CityName", "DistrictID", "DistrictName",
		"VillageID", "VillageName", "StreetAddress", "PostalCode",
	}

	for _, key := range pascalKeys {
		if _, ok := raw[key]; ok {
			t.Errorf("unexpected PascalCase key %q in JSON output — tags not applied", key)
		}
	}

	// Verify omitempty on latitude/longitude (nil → absent)
	if _, ok := raw["latitude"]; ok {
		t.Error("nil latitude should be omitted from JSON")
	}
	if _, ok := raw["longitude"]; ok {
		t.Error("nil longitude should be omitted from JSON")
	}
}

func TestAddressSnapshot_RoundTrip(t *testing.T) {
	lat := 6.9175
	lon := 107.6191

	original := AddressSnapshot{
		RecipientName: "Siti Nurhaliza",
		Phone:         "089876543210",
		ProvinceID:    "32",
		ProvinceName:  "Jawa Barat",
		CityID:        "3204",
		CityName:      "Bandung",
		DistrictID:    "320401",
		DistrictName:  "Coblong",
		VillageID:     "3204011001",
		VillageName:   "Dago",
		StreetAddress: "Jl. Ganeca No. 10",
		PostalCode:    "40132",
		Latitude:      &lat,
		Longitude:     &lon,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded AddressSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.RecipientName != original.RecipientName {
		t.Errorf("RecipientName: got %q, want %q", decoded.RecipientName, original.RecipientName)
	}
	if decoded.ProvinceName != original.ProvinceName {
		t.Errorf("ProvinceName: got %q, want %q", decoded.ProvinceName, original.ProvinceName)
	}
	if decoded.CityName != original.CityName {
		t.Errorf("CityName: got %q, want %q", decoded.CityName, original.CityName)
	}
	if decoded.StreetAddress != original.StreetAddress {
		t.Errorf("StreetAddress: got %q, want %q", decoded.StreetAddress, original.StreetAddress)
	}
	if decoded.PostalCode != original.PostalCode {
		t.Errorf("PostalCode: got %q, want %q", decoded.PostalCode, original.PostalCode)
	}
	if decoded.Latitude == nil || *decoded.Latitude != lat {
		t.Errorf("Latitude: got %v, want %v", decoded.Latitude, lat)
	}
	if decoded.Longitude == nil || *decoded.Longitude != lon {
		t.Errorf("Longitude: got %v, want %v", decoded.Longitude, lon)
	}
}

func TestAddressSnapshot_AdminUnmarshalCompatibility(t *testing.T) {
	// Simulate what admin_order_handler does: unmarshal JSONB into a struct
	// with the same snake_case tags. This must produce non-empty values.
	snap := AddressSnapshot{
		RecipientName: "Ali",
		Phone:         "08123",
		ProvinceName:  "Jawa Barat",
		CityName:      "Bandung",
		DistrictName:  "Coblong",
		VillageName:   "Dago",
		StreetAddress: "Jl. Juanda 100",
		PostalCode:    "40135",
	}

	// Marshal as the repository does
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Unmarshal as admin_order_handler does (snake_case tags)
	var adminSnap struct {
		RecipientName string `json:"recipient_name"`
		Phone         string `json:"phone"`
		ProvinceName  string `json:"province_name"`
		CityName      string `json:"city_name"`
		DistrictName  string `json:"district_name"`
		VillageName   string `json:"village_name"`
		StreetAddress string `json:"street_address"`
		PostalCode    string `json:"postal_code"`
	}

	if err := json.Unmarshal(data, &adminSnap); err != nil {
		t.Fatalf("admin unmarshal failed: %v", err)
	}

	if adminSnap.RecipientName == "" {
		t.Error("admin unmarshal: RecipientName is empty — tag mismatch")
	}
	if adminSnap.ProvinceName == "" {
		t.Error("admin unmarshal: ProvinceName is empty — tag mismatch")
	}
	if adminSnap.CityName == "" {
		t.Error("admin unmarshal: CityName is empty — tag mismatch")
	}
	if adminSnap.StreetAddress == "" {
		t.Error("admin unmarshal: StreetAddress is empty — tag mismatch")
	}
	if adminSnap.PostalCode == "" {
		t.Error("admin unmarshal: PostalCode is empty — tag mismatch")
	}
}

func keys(m map[string]interface{}) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}


