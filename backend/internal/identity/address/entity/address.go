package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Address is the canonical saved-address record for an account.
//
// CANONICAL TRUTH (one authority, one address book):
// - Address belongs to the ACCOUNT, never to a role. An account owns one
//   address book; seller/buyer are labels drawn from Tags, not separate books.
// - Tags describe how an address is USED. One address may carry several tags
//   ("shipping" AND "sender" at the same time).
// - Exactly ONE address per account is primary (enforced by
//   idx_addresses_user_active_primary_unique). There is no per-purpose primary.
// - Order stores an address snapshot, NOT address_id (for immutability).
// - Promotion scope will attach additional tags later; no schema change needed.
//
// FORBIDDEN DESIGN (killed, do not reintroduce):
// - A separate address book / primary per purpose.
// - A single-purpose "purpose" column that forces one role per address.
// - Any profile-side copy of an address (origin line is derived, never stored).
type Address struct {
	ID        uuid.UUID
	UserID    uuid.UUID

	// Tags declares how this address may be used: "shipping", "sender".
	// An address used both as a destination and as an origin carries both.
	Tags []AddressTag

	// Nickname is an optional user-defined name for this address (e.g., "Home", "Office")
	Nickname string

	// Recipient information
	RecipientName string
	Phone         string

	// Location information (Indonesia administrative regions)
	ProvinceID   string
	ProvinceName string
	CityID       string
	CityName     string
	DistrictID   string
	DistrictName string
	VillageID    string
	VillageName  string

	// Street address details
	StreetAddress string
	PostalCode    string

	// Optional coordinates for delivery verification
	Latitude  *float64
	Longitude *float64

	// Optional notes for delivery instructions
	Notes string

	// Flags
	IsPrimary               bool
	IsAvailableForCheckout bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// AddressTag declares a usage of an address.
type AddressTag string

const (
	// TagShipping marks an address a buyer may be delivered to.
	TagShipping AddressTag = "shipping"

	// TagSender marks an address goods may be shipped from.
	TagSender AddressTag = "sender"
)

// AllAddressTags lists every valid address tag.
var AllAddressTags = []AddressTag{
	TagShipping,
	TagSender,
}

// IsValidTag reports whether a wire-level tag value is known.
func IsValidTag(tag string) bool {
	switch AddressTag(tag) {
	case TagShipping, TagSender:
		return true
	default:
		return false
	}
}

// NormalizeTags validates, de-duplicates and canonically orders a wire-level
// tag list. It returns InvalidTagsError when the list is empty or unknown.
func NormalizeTags(tags []string) ([]AddressTag, error) {
	if len(tags) == 0 {
		return nil, &InvalidTagsError{Tags: tags}
	}

	seen := make(map[AddressTag]bool, len(tags))
	result := make([]AddressTag, 0, len(tags))
	for _, raw := range tags {
		if !IsValidTag(raw) {
			return nil, &InvalidTagsError{Tags: tags}
		}
		tag := AddressTag(raw)
		if seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
	}

	// Stable canonical order so equal tag sets compare equal on the wire.
	ordered := make([]AddressTag, 0, len(result))
	for _, known := range AllAddressTags {
		if seen[known] {
			ordered = append(ordered, known)
		}
	}
	return ordered, nil
}

// HasTag reports whether the address carries the given tag.
func (a *Address) HasTag(tag AddressTag) bool {
	for _, t := range a.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// TagStrings renders the canonical tag list as wire values.
func (a *Address) TagStrings() []string {
	out := make([]string, 0, len(a.Tags))
	for _, t := range a.Tags {
		out = append(out, string(t))
	}
	return out
}

// TagsFrom converts a persisted text[] into the canonical tag list.
func TagsFrom(values []string) []AddressTag {
	tags := make([]AddressTag, 0, len(values))
	for _, v := range values {
		tags = append(tags, AddressTag(v))
	}
	return tags
}

// ============================================================================
// BUSINESS ERRORS
// ============================================================================

// InvalidTagsError is returned when a tag list is empty or holds unknown tags.
type InvalidTagsError struct {
	Tags []string
}

func (e *InvalidTagsError) Error() string {
	return fmt.Sprintf("invalid address tags: must be a non-empty subset of %q, got %v", AllAddressTags, e.Tags)
}

// MissingRequiredFieldError is returned when a required field is missing.
type MissingRequiredFieldError struct {
	Field string
}

func (e *MissingRequiredFieldError) Error() string {
	return fmt.Sprintf("missing required field: %s", e.Field)
}

// InvalidPhoneError is returned when phone number is invalid.
type InvalidPhoneError struct {
	Phone string
}

func (e *InvalidPhoneError) Error() string {
	return fmt.Sprintf("invalid phone number: %s", e.Phone)
}

// AddressNotFoundError is returned when an address is not found.
type AddressNotFoundError struct {
	ID uuid.UUID
}

func (e *AddressNotFoundError) Error() string {
	return fmt.Sprintf("address not found: %s", e.ID)
}

// AddressNotOwnedError is returned when user tries to access another user's address.
type AddressNotOwnedError struct {
	AddressID uuid.UUID
	UserID    uuid.UUID
}

func (e *AddressNotOwnedError) Error() string {
	return fmt.Sprintf("address not owned by user: address_id=%s, user_id=%s", e.AddressID, e.UserID)
}

// AddressUnavailableError is returned when address is not available for checkout.
type AddressUnavailableError struct {
	ID uuid.UUID
}

func (e *AddressUnavailableError) Error() string {
	return fmt.Sprintf("address not available for checkout: %s", e.ID)
}

// PrimaryAddressAlreadyExistsError is returned when user already has a primary address.
type PrimaryAddressAlreadyExistsError struct {
	UserID uuid.UUID
}

func (e *PrimaryAddressAlreadyExistsError) Error() string {
	return fmt.Sprintf("user already has a primary address: user_id=%s", e.UserID)
}

// SenderAddressRequiresSellerAuthorityError is returned when a non-authorized
// user tries to create a sender address.
// Sender addresses are shipping origin addresses for sellers.
type SenderAddressRequiresSellerAuthorityError struct {
	UserID uuid.UUID
}

func (e *SenderAddressRequiresSellerAuthorityError) Error() string {
	return fmt.Sprintf("sender address requires seller authority: user_id=%s", e.UserID)
}

// ============================================================================
// ENTITY METHODS
// ============================================================================

// CanBeUsedForCheckout checks if the address can be used for checkout.
func (a *Address) CanBeUsedForCheckout() bool {
	return a.IsAvailableForCheckout
}

// SetAsPrimary marks this address as primary.
// This method should be called within a transaction that also unsets other primary addresses.
func (a *Address) SetAsPrimary() {
	a.IsPrimary = true
	a.UpdatedAt = time.Now()
}

// UnsetAsPrimary removes the primary flag from this address.
func (a *Address) UnsetAsPrimary() {
	a.IsPrimary = false
	a.UpdatedAt = time.Now()
}

// MakeUnavailableForCheckout marks this address as unavailable for checkout.
func (a *Address) MakeUnavailableForCheckout() {
	a.IsAvailableForCheckout = false
	a.UpdatedAt = time.Now()
}

// MakeAvailableForCheckout marks this address as available for checkout.
func (a *Address) MakeAvailableForCheckout() {
	a.IsAvailableForCheckout = true
	a.UpdatedAt = time.Now()
}

// ============================================================================
// SNAPSHOT FOR ORDER
// ============================================================================

// AddressSnapshot represents the address data that should be stored in an order.
// This ensures order immutability even if the original address is modified or deleted.
type AddressSnapshot struct {
	RecipientName string `json:"recipient_name"`
	Phone         string `json:"phone"`

	ProvinceID   string `json:"province_id"`
	ProvinceName string `json:"province_name"`
	CityID       string `json:"city_id"`
	CityName     string `json:"city_name"`
	DistrictID   string `json:"district_id"`
	DistrictName string `json:"district_name"`
	VillageID    string `json:"village_id"`
	VillageName  string `json:"village_name"`

	StreetAddress string `json:"street_address"`
	PostalCode    string `json:"postal_code"`

	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// ToSnapshot creates an immutable snapshot of this address for order storage.
func (a *Address) ToSnapshot() AddressSnapshot {
	var lat, lon *float64
	if a.Latitude != nil {
		lat = a.Latitude
	}
	if a.Longitude != nil {
		lon = a.Longitude
	}

	return AddressSnapshot{
		RecipientName: a.RecipientName,
		Phone:         a.Phone,

		ProvinceID:   a.ProvinceID,
		ProvinceName: a.ProvinceName,
		CityID:       a.CityID,
		CityName:     a.CityName,
		DistrictID:   a.DistrictID,
		DistrictName: a.DistrictName,
		VillageID:    a.VillageID,
		VillageName:  a.VillageName,

		StreetAddress: a.StreetAddress,
		PostalCode:    a.PostalCode,

		Latitude:  lat,
		Longitude: lon,
	}
}

// ============================================================================
// FACTORY
// ============================================================================

// NewAddress creates a new address.
//
// Validation:
// - Tags must be a non-empty subset of AllAddressTags
// - RecipientName is required
// - Phone is required and must be valid
// - At least ProvinceID/CityID are required
// - StreetAddress is required
func NewAddress(
	userID uuid.UUID,
	tags []AddressTag,
	nickname string,
	recipientName string,
	phone string,
	provinceID string,
	provinceName string,
	cityID string,
	cityName string,
	districtID string,
	districtName string,
	villageID string,
	villageName string,
	streetAddress string,
	postalCode string,
	latitude *float64,
	longitude *float64,
	notes string,
	isPrimary bool,
) (*Address, error) {
	// Validate tags: at least one, no unknown values.
	if len(tags) == 0 {
		return nil, &InvalidTagsError{}
	}
	for _, tag := range tags {
		if !IsValidTag(string(tag)) {
			return nil, &InvalidTagsError{Tags: []string{string(tag)}}
		}
	}

	// Validate required fields
	if recipientName == "" {
		return nil, &MissingRequiredFieldError{Field: "recipient_name"}
	}

	if phone == "" {
		return nil, &MissingRequiredFieldError{Field: "phone"}
	}

	// Basic phone validation (Indonesia: starts with 0 or 62, 10-15 digits)
	// This is a simple validation - can be enhanced with regex
	if len(phone) < 10 || len(phone) > 15 {
		return nil, &InvalidPhoneError{Phone: phone}
	}

	if provinceID == "" {
		return nil, &MissingRequiredFieldError{Field: "province_id"}
	}

	if cityID == "" {
		return nil, &MissingRequiredFieldError{Field: "city_id"}
	}

	if streetAddress == "" {
		return nil, &MissingRequiredFieldError{Field: "street_address"}
	}

	now := time.Now()

	return &Address{
		ID:                      uuid.New(),
		UserID:                  userID,
		Tags:                    tags,
		Nickname:                nickname,
		RecipientName:           recipientName,
		Phone:                   phone,
		ProvinceID:              provinceID,
		ProvinceName:            provinceName,
		CityID:                  cityID,
		CityName:                cityName,
		DistrictID:              districtID,
		DistrictName:            districtName,
		VillageID:               villageID,
		VillageName:             villageName,
		StreetAddress:           streetAddress,
		PostalCode:              postalCode,
		Latitude:                latitude,
		Longitude:               longitude,
		Notes:                   notes,
		IsPrimary:               isPrimary,
		IsAvailableForCheckout:  true,
		CreatedAt:               now,
		UpdatedAt:               now,
	}, nil
}


