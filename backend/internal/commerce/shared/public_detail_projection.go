package shared

import (
	"strings"

	"github.com/labuda/backend/internal/commerce/shipping/entity"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
)

// PublicShippingOptionSummary is the buyer-facing shipping option shape.
// It intentionally omits seller-only/internal fields.
type PublicShippingOptionSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	TransportType string `json:"transport_type"`
}

// BuildPublicShippingOptionSummaries converts shipping option entities into a
// buyer-facing summary payload.
func BuildPublicShippingOptionSummaries(options []*entity.ShippingOption) []PublicShippingOptionSummary {
	if len(options) == 0 {
		return []PublicShippingOptionSummary{}
	}

	result := make([]PublicShippingOptionSummary, 0, len(options))
	for _, option := range options {
		if option == nil {
			continue
		}
		result = append(result, PublicShippingOptionSummary{
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
func BuildPublicOriginSummary(address *addressEntity.Address) string {
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
