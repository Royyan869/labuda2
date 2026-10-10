package username

import "testing"

func TestIsReserved_ProductBrandNames(t *testing.T) {
	reserved := []string{"labuda", "hishumi"}
	for _, name := range reserved {
		if !IsReserved(name) {
			t.Errorf("IsReserved(%q) = false, want true (product brand must be reserved)", name)
		}
		if !IsReserved(Normalize(name)) {
			t.Errorf("IsReserved(Normalize(%q)) = false, want true", name)
		}
	}
}

func TestIsReserved_CaseInsensitive(t *testing.T) {
	for _, name := range []string{"Labuda", "LABUDA", "HiShumi", "HISHUMI"} {
		if !IsReserved(name) {
			t.Errorf("IsReserved(%q) = false, want true (reservation must be case-insensitive)", name)
		}
	}
}

func TestIsReserved_NonReservedStillAvailable(t *testing.T) {
	for _, name := range []string{"budi", "koi_farm", "seller123"} {
		if IsReserved(name) {
			t.Errorf("IsReserved(%q) = true, want false", name)
		}
	}
}
