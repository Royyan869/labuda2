// DOMAIN: COMMERCE
// NOTE: Manual shipping cost quotes provided by sellers

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
)

// QuoteStatus represents the lifecycle status of a shipping quote.
type QuoteStatus string

const (
	QuoteStatusActive  QuoteStatus = "ACTIVE"  // Quote is available for use
	QuoteStatusUsed    QuoteStatus = "USED"    // Quote has been used in an order
	QuoteStatusExpired QuoteStatus = "EXPIRED" // Quote has expired
	QuoteStatusInvalid QuoteStatus = "INVALID" // Quote is invalid (for_sale unavailable)
)

// IsValid returns true if the status is a valid QuoteStatus.
func (s QuoteStatus) IsValid() bool {
	switch s {
	case QuoteStatusActive, QuoteStatusUsed, QuoteStatusExpired, QuoteStatusInvalid:
		return true
	}
	return false
}

// CanBeUsed returns true if the quote can be used for checkout.
func (s QuoteStatus) CanBeUsed() bool {
	return s == QuoteStatusActive
}

// ShippingQuote represents a manual shipping cost quote provided by seller via chat.
//
// VALIDATION RULES (TASK A-G):
// - Quote scope: tied to chat_id, buyer_id, and product/sale-surface binding
// - One current unsuperseded quote per canonical context
// - Lifecycle: ACTIVE -> USED/EXPIRED
// - Address lock: destination_city_id, destination_province_id
// - Quote price overrides all shipping options when used
// - Order snapshot stores quote_id, quote_price, destination, origin
//
// SECURITY:
// - Validates buyer_id match, product_id or sale-surface match, status ACTIVE
// - Address locked at quote creation, validated at checkout
// - Reactivation limit enforced (reactivation_count < max_reuse)
type ShippingQuote struct {
	ID                    uuid.UUID
	ChatID                uuid.UUID
	ProductID             uuid.UUID
	SourceType            *string
	SourceID              *uuid.UUID
	SellerID              uuid.UUID
	BuyerID               uuid.UUID
	Cost                  money.Money
	Note                  *string
	Status                QuoteStatus // ACTIVE, USED, EXPIRED (TASK C)
	SupersededAt          *time.Time
	SupersededByID        *uuid.UUID
	DestinationCityID     *string    // Destination city lock for validation (TASK D)
	DestinationProvinceID *string    // Destination province lock for validation (TASK D)
	UsedAt                *time.Time // Timestamp when quote was used (TASK C)
	ExpiresAt             *time.Time // Expiration timestamp. New records must always set this.
	CreatedAt             time.Time
	ReactivationCount     int // Number of times quote has been reactivated
	MaxReuse              int // Maximum allowed reactivations (default: 2)
}

// NewShippingQuote creates a new ShippingQuote entity with default ACTIVE status.
func NewShippingQuote(
	chatID, productID uuid.UUID,
	sourceType string,
	sourceID uuid.UUID,
	sellerID, buyerID uuid.UUID,
	cost money.Money,
	note *string,
	destinationCityID, destinationProvinceID *string,
	expiresAt time.Time,
) *ShippingQuote {
	return &ShippingQuote{
		ID:                    uuid.New(),
		ChatID:                chatID,
		ProductID:             productID,
		SourceType:            &sourceType,
		SourceID:              &sourceID,
		SellerID:              sellerID,
		BuyerID:               buyerID,
		Cost:                  cost,
		Note:                  note,
		Status:                QuoteStatusActive, // Default to ACTIVE (TASK C)
		ExpiresAt:             &expiresAt,
		DestinationCityID:     destinationCityID,     // Address lock (TASK D)
		DestinationProvinceID: destinationProvinceID, // Address lock (TASK D)
		CreatedAt:             time.Now(),
	}
}


// IsActive returns true if the quote is in ACTIVE status.
func (q *ShippingQuote) IsActive() bool {
	return q.Status == QuoteStatusActive
}

// IsSuperseded returns true when a newer quote revision has replaced this one.
func (q *ShippingQuote) IsSuperseded() bool {
	return q.SupersededAt != nil
}

// IsCurrent returns true when the quote is the current unsuperseded revision.
func (q *ShippingQuote) IsCurrent() bool {
	return q.IsActive() && !q.IsSuperseded()
}

// IsBuyerUsableAt returns true when the quote is structurally usable by a buyer
// before seller/source/operator checks are applied.
func (q *ShippingQuote) IsBuyerUsableAt(now time.Time) bool {
	return q.IsCurrent() &&
		q.ExpiresAt != nil &&
		q.UsedAt == nil &&
		!q.IsExpiredAt(now)
}

// MarkUsed marks the quote as USED and sets the used_at timestamp.
// Returns error if quote is not structurally buyer-usable at the provided time.
func (q *ShippingQuote) MarkUsed(now time.Time) error {
	if !q.IsBuyerUsableAt(now) {
		return &InvalidQuoteStatusError{CurrentStatus: q.Status, RequiredStatus: QuoteStatusActive}
	}
	q.Status = QuoteStatusUsed
	q.UsedAt = &now
	return nil
}

// MarkExpired marks the quote as EXPIRED.
// Returns error if quote is already used.
func (q *ShippingQuote) MarkExpired() error {
	if q.Status == QuoteStatusUsed {
		return &InvalidQuoteStatusError{CurrentStatus: q.Status, RequiredStatus: QuoteStatusActive}
	}
	q.Status = QuoteStatusExpired
	return nil
}

// MarkInvalid marks the quote as INVALID.
// Used when the associated for_sale becomes unavailable (sold, withdrawn, deleted).
func (q *ShippingQuote) MarkInvalid() error {
	if q.Status == QuoteStatusUsed {
		return &InvalidQuoteStatusError{CurrentStatus: q.Status, RequiredStatus: QuoteStatusActive}
	}
	q.Status = QuoteStatusInvalid
	return nil
}

// Reactivate transitions a USED quote back to ACTIVE for reuse.
// Returns error if the quote is not reactivateable.
func (q *ShippingQuote) Reactivate(expiresAt time.Time) error {
	if q.Status != QuoteStatusUsed {
		return &InvalidQuoteStatusError{CurrentStatus: q.Status, RequiredStatus: QuoteStatusUsed}
	}
	if q.IsSuperseded() {
		return fmt.Errorf("cannot reactivate superseded quote %s", q.ID)
	}
	if q.ReactivationCount >= q.MaxReuse {
		return fmt.Errorf("cannot reactivate quote %s: reuse limit reached", q.ID)
	}
	if expiresAt.IsZero() {
		return fmt.Errorf("cannot reactivate quote %s: invalid expiry", q.ID)
	}
	q.Status = QuoteStatusActive
	q.UsedAt = nil
	q.ExpiresAt = &expiresAt
	q.ReactivationCount++
	return nil
}

// IsExpiredAt returns true when the provided time is at or past expires_at.
func (q *ShippingQuote) IsExpiredAt(now time.Time) bool {
	if q.ExpiresAt == nil {
		return true
	}
	return !now.Before(*q.ExpiresAt)
}

// CanBeReactivated returns true if the quote can be reactivated.
// A quote can be reactivated if:
// - It is in USED status
// - Reactivation count is less than max_reuse
func (q *ShippingQuote) CanBeReactivated() bool {
	return q.Status == QuoteStatusUsed && !q.IsSuperseded() && q.ReactivationCount < q.MaxReuse
}

// IncrementReactivationCount increments the reactivation counter.
func (q *ShippingQuote) IncrementReactivationCount() {
	q.ReactivationCount++
}

// ValidateDestinationAddress validates that the provided address matches
// the locked destination on the quote (TASK D - Address Lock).
func (q *ShippingQuote) ValidateDestinationAddress(provinceID, cityID string) error {
	// If quote has no locked destination, any address is valid
	if q.DestinationProvinceID == nil && q.DestinationCityID == nil {
		return nil
	}

	// Validate province match if locked
	if q.DestinationProvinceID != nil && *q.DestinationProvinceID != provinceID {
		return &DestinationMismatchError{
			Field:    "province_id",
			Expected: *q.DestinationProvinceID,
			Provided: provinceID,
			QuoteID:  q.ID,
		}
	}

	// Validate city match if locked
	if q.DestinationCityID != nil && *q.DestinationCityID != cityID {
		return &DestinationMismatchError{
			Field:    "city_id",
			Expected: *q.DestinationCityID,
			Provided: cityID,
			QuoteID:  q.ID,
		}
	}

	return nil
}

// GetItemReference returns the source ID and whether the quote belongs to an auction.
// Returns (itemID, isAuction).
func (q *ShippingQuote) GetItemReference() (uuid.UUID, bool) {
	if q.SourceType != nil && q.SourceID != nil && *q.SourceID != uuid.Nil {
		return *q.SourceID, *q.SourceType == "auction"
	}
	return uuid.Nil, false
}

// InvalidQuoteStatusError is returned when attempting an invalid status transition.
type InvalidQuoteStatusError struct {
	CurrentStatus  QuoteStatus
	RequiredStatus QuoteStatus
}

func (e *InvalidQuoteStatusError) Error() string {
	return fmt.Sprintf("invalid quote status: current=%s, required=%s", e.CurrentStatus, e.RequiredStatus)
}

// DestinationMismatchError is returned when the checkout address doesn't match
// the locked destination on the quote.
type DestinationMismatchError struct {
	Field    string
	Expected string
	Provided string
	QuoteID  uuid.UUID
}

func (e *DestinationMismatchError) Error() string {
	return fmt.Sprintf("destination %s mismatch for quote %s: expected=%s, provided=%s",
		e.Field, e.QuoteID, e.Expected, e.Provided)
}
