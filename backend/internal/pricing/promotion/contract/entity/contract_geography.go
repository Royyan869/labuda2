package entity

import (
	"time"

	"github.com/google/uuid"
)

// ContractGeography is a canonical geographic targeting row for a promotion contract.
// city_id is the canonical membership authority (normalized addresses.city_id).
// city_name is immutable display snapshot, province_id is contextual snapshot.
type ContractGeography struct {
	ContractID uuid.UUID
	CityID     string
	CityName   string
	ProvinceID string
	CreatedAt  time.Time
}
