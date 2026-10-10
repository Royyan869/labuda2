package shared

import (
	"strings"

	"github.com/hishumi/backend/internal/commerce/shipping/entity"
	addressEntity "github.com/hishumi/backend/internal/identity/address/entity"
)

// PublicShippingSetupSummary is the buyer-facing shipping option shape.
// It intentionally omits seller-only/internal fields.
type PublicShippingSetupSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	TransportType string `json:"transport_type"`
}

// BuildPublicShippingSetupSummaries converts shipping option entities into a
// buyer-facing summary payload.
func BuildPublicShippingSetupSummaries(options []*entity.ShippingSetup) []PublicShippingSetupSummary {
	if len(options) == 0 {
		return []PublicShippingSetupSummary{}
	}

	result := make([]PublicShippingSetupSummary, 0, len(options))
	for _, option := range options {
		if option == nil {
			continue
		}
		result = append(result, PublicShippingSetupSummary{
			ID:            option.ID.String(),
			Name:          strings.TrimSpace(option.Name),
			TransportType: strings.TrimSpace(string(option.TransportType)),
		})
	}
	return result
}

// BuildPublicOriginSummary returns a safe public string summary for a seller
// sender address. It excludes street, district, recipient, phone, and
// coordinates.
//
// DELEGATION: the redaction rule itself lives ONCE, on the address entity
// (identity/address/entity), so commerce and the public profile cannot drift.
func BuildPublicOriginSummary(address *addressEntity.Address) string {
	return addressEntity.BuildPublicOriginSummary(address)
}
