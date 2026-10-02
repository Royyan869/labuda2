package entity

import "strings"

// BuildPublicOriginSummary returns the public-safe origin summary of an
// address: "City, Province".
//
// ONE IMPLEMENTATION of the public-address redaction rule — it deliberately
// excludes street, district, village, postal code, recipient, phone and
// coordinates. Every surface that shows an address publicly (commerce detail
// seller cards, public profile) goes through this function, so the redaction
// boundary cannot drift per caller.
//
// A nil address yields "" (hide rather than fabricate).
func BuildPublicOriginSummary(address *Address) string {
	if address == nil {
		return ""
	}

	parts := make([]string, 0, 2)
	if city := strings.TrimSpace(address.CityName); city != "" {
		parts = append(parts, city)
	}
	if province := strings.TrimSpace(address.ProvinceName); province != "" {
		parts = append(parts, province)
	}

	return strings.Join(parts, ", ")
}
