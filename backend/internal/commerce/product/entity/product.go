package entity

import (
	"time"

	"github.com/google/uuid"
)

// Product is the internal physical item authority.
// It is intentionally sale-surface agnostic.
type Product struct {
	ID              uuid.UUID
	SellerID        uuid.UUID
	Title           string
	Description     string
	MediaURLs       []string
	Variety         string
	SizeCm          *int
	AgeMonths       *int
	Gender          *string
	Breeder         *string
	Bloodline       *string
	Certificates    []string
	FarmAddressID   *uuid.UUID
	PreparationTime string
	PreparationNote *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
