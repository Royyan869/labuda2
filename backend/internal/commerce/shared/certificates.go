package shared

import (
	"strings"

	productEntity "github.com/hishumi/backend/internal/commerce/product/entity"
)

// NormalizeCertificates validates commerce certificate values, removes
// duplicates, and returns them in canonical order.
//
// Empty inputs return an empty slice. Unknown values are rejected so the
// backend never persists non-canonical certificate strings.
//
// The vocabulary and its order live in ONE place —
// productEntity.CanonicalCertificateOrder — and validation is delegated to
// productEntity.ValidateCertificates. This file deliberately keeps no copy of
// the list: the previous duplicate vocabulary is how `ownership` managed to
// survive here after the entity was updated, and how the two validation paths
// could accept different values.
func NormalizeCertificates(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}

	requested := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value != "" {
			requested = append(requested, value)
		}
	}

	if err := productEntity.ValidateCertificates(&requested); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(requested))
	for _, value := range requested {
		seen[value] = struct{}{}
	}

	result := make([]string, 0, len(seen))
	for _, canonical := range productEntity.CanonicalCertificateOrder {
		if _, ok := seen[canonical]; ok {
			result = append(result, canonical)
		}
	}
	return result, nil
}
