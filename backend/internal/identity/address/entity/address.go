package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Address represents a user's shipping or sender address.
//
// IMPORTANT DESIGN NOTES:
// - Address is stored as a separate entity
// - Order must store address snapshot, NOT address_id (for immutability)
// - Address is only used during checkout flow
// - User can have multiple addresses with one marked as primary
//
// ADDRESS PURPOSES:
// - "shipping": Buyer receives items (any user can create)
// - "sender": Seller ships items from (ONLY sellers can create)
//
// FUTURE CONSIDERATIONS:
// - Duplicate detection: Users may create multiple identical addresses
//   Consider adding de-duplication logic or UI hints for existing addresses
type Address struct {
	ID        uuid.UUID
	UserID    uuid.UUID

	// Purpose defines the address usage: "shipping" or "sender"
	Purpose AddressPurpose

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

// AddressPurpose defines the usage type of an address.
type AddressPurpose string

const (
	// AddressPurposeShipping is for delivery addresses (buyer receives items)
	AddressPurposeShipping AddressPurpose = "shipping"

	// AddressPurposeSender is for sender addresses (seller ships items)
	AddressPurposeSender AddressPurpose = "sender"
)

// AllAddressPurposes lists all valid address purposes.
var AllAddressPurposes = []AddressPurpose{
	AddressPurposeShipping,
	AddressPurposeSender,
}

// IsValidPurpose checks if a purpose is valid.
func IsValidPurpose(purpose string) bool {
	switch AddressPurpose(purpose) {
	case AddressPurposeShipping, AddressPurposeSender:
		return true
	default:
		return false
	}
}

// ============================================================================
// BUSINESS ERRORS
// ============================================================================

// InvalidPurposeError is returned when an invalid purpose is provided.
type InvalidPurposeError struct {
	Purpose string
}

func (e *InvalidPurposeError) Error() string {
	return fmt.Sprintf("invalid address purpose: %s", e.Purpose)
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
// - Purpose must be valid ("shipping" or "sender")
// - RecipientName is required
// - Phone is required and must be valid
// - At least ProvinceID/CityID are required
// - StreetAddress is required
func NewAddress(
	userID uuid.UUID,
	purpose AddressPurpose,
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
	// Validate purpose
	if !IsValidPurpose(string(purpose)) {
		return nil, &InvalidPurposeError{Purpose: string(purpose)}
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
		Purpose:                 purpose,
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


