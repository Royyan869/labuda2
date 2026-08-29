package entity

import (
	"time"

	"github.com/google/uuid"
)

// SellingSurface represents the exclusive selling surface attached to a Product.
// A Product may belong to exactly one surface: for_sale OR auction, never both.
type SellingSurface string

const (
	SellingSurfaceNone    SellingSurface = ""      // Unattached: Product has no selling surface
	SellingSurfaceForSale SellingSurface = "for_sale" // Attached to a ForSale
	SellingSurfaceAuction SellingSurface = "auction"  // Attached to an Auction
)

// Product is the internal physical item authority.
// selling_surface tracks exclusive surface ownership (for_sale | auction | null).
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
	SellingSurface  SellingSurface // Exclusive surface ownership: NULL | 'for_sale' | 'auction'
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
