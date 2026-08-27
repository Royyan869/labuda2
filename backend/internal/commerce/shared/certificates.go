package shared

import (
	"fmt"
	"strings"
)

var canonicalCertificateOrder = []string{
	"breeder",
	"contest",
	"ownership",
	"health",
}

var canonicalCertificateSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(canonicalCertificateOrder))
	for _, value := range canonicalCertificateOrder {
		set[value] = struct{}{}
	}
	return set
}()

// NormalizeCertificates validates commerce certificate values, removes
// duplicates, and returns them in canonical order.
//
// Empty inputs return an empty slice. Unknown values are rejected so the
// backend never persists non-canonical certificate strings.
func NormalizeCertificates(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}

	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if _, ok := canonicalCertificateSet[value]; !ok {
			return nil, fmt.Errorf("invalid certificate: %s", raw)
		}
		seen[value] = struct{}{}
	}

	result := make([]string, 0, len(seen))
	for _, canonical := range canonicalCertificateOrder {
		if _, ok := seen[canonical]; ok {
			result = append(result, canonical)
		}
	}
	return result, nil
}
