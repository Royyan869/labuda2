// Package geography owns the ONE canonical Geography Master for Labuda.
//
// Business truth (owner locked): Geography does not belong to Address,
// Shipping, Recipient, Promotion, Mobile or Admin — those are consumers.
// This package owns the master table (canonical_geographies), the seed of
// the full Indonesian hierarchy, the read API, and the write-side
// validation every consumer must pass before it may persist geographic
// identity.
//
// The canonical identity is the normalized BPS code with dots removed:
//
//	province  2 digits   e.g. "32"
//	regency   4 digits   e.g. "3204"
//	district  6 digits   e.g. "320401"
//	village  10 digits   e.g. "3204012001"
//
// Parent/child integrity is enforced by a self-referencing FK on the master
// and re-checked by the Validator on every consumer write.
package geography

import (
	"context"
	"errors"
	"fmt"

	"github.com/hishumi/backend/pkg/db"
)

// Level is the canonical geographic level vocabulary.
type Level string

const (
	LevelProvince Level = "province"
	LevelRegency  Level = "regency"
	LevelDistrict Level = "district"
	LevelVillage  Level = "village"
)

// Entity is one canonical geography row.
type Entity struct {
	Code       string
	Level      Level
	Name       string
	ParentCode *string
	PostalCode *string
}

// Errors returned by the master and its validator.
var (
	// ErrInvalidGeography is returned when a consumer submits a code that is
	// absent from the master, whose level is wrong, or whose parent link does
	// not match the submitted hierarchy.
	ErrInvalidGeography = errors.New("invalid canonical geography")
	// ErrGeographyNotFound is returned by a single-code read for an unknown code.
	ErrGeographyNotFound = errors.New("canonical geography not found")
)

// Validator is the single write-side gate for consumer geography identity.
//
// It is stateless; callers pass the transaction the consumer write will use so
// the check and the write share one view of the master. A nil *Validator is
// never wired in production; consumers that hold an optional validator skip
// the check only when it is nil (test-only construction).
type Validator struct{}

// NewValidator constructs the canonical geography validator.
func NewValidator() *Validator { return &Validator{} }

// ValidateAddressScope verifies a Province -> Regency -> District -> Village
// chain against the master. Province and regency are mandatory (the address
// wire contract requires both); district and village are checked when present
// and must attach to the submitted parent.
func (v *Validator) ValidateAddressScope(
	ctx context.Context,
	tx db.Tx,
	provinceID, cityID, districtID, villageID string,
) error {
	if provinceID == "" || cityID == "" {
		return fmt.Errorf("%w: province_id and city_id are required", ErrInvalidGeography)
	}
	if err := requireLevel(ctx, tx, provinceID, LevelProvince); err != nil {
		return err
	}
	if err := requireParent(ctx, tx, cityID, LevelRegency, provinceID); err != nil {
		return err
	}
	if districtID != "" {
		if err := requireParent(ctx, tx, districtID, LevelDistrict, cityID); err != nil {
			return err
		}
	}
	if villageID != "" {
		if districtID == "" {
			return fmt.Errorf("%w: district_id is required when village_id is set", ErrInvalidGeography)
		}
		if err := requireParent(ctx, tx, villageID, LevelVillage, districtID); err != nil {
			return err
		}
	}
	return nil
}

// ValidateProvinceCity verifies that provinceCode exists and, when cityCode is
// set, that the regency exists and its parent is provinceCode. Shipping
// coverage selects a province (required) with optional city-level overrides.
func (v *Validator) ValidateProvinceCity(
	ctx context.Context,
	tx db.Tx,
	provinceCode, cityCode string,
) error {
	if provinceCode == "" {
		return fmt.Errorf("%w: province_code is required", ErrInvalidGeography)
	}
	if err := requireLevel(ctx, tx, provinceCode, LevelProvince); err != nil {
		return err
	}
	if cityCode != "" {
		if err := requireParent(ctx, tx, cityCode, LevelRegency, provinceCode); err != nil {
			return err
		}
	}
	return nil
}

// requireLevel checks that code exists in the master at the expected level.
func requireLevel(ctx context.Context, tx db.Tx, code string, level Level) error {
	var found string
	err := tx.QueryRow(ctx,
		`SELECT code FROM canonical_geographies WHERE code = $1 AND level = $2`,
		code, string(level),
	).Scan(&found)
	if err != nil {
		return fmt.Errorf("%w: %s %q is not in the canonical geography master", ErrInvalidGeography, level, code)
	}
	return nil
}

// requireParent checks that code exists at the expected level AND that its
// parent_code equals parent.
func requireParent(ctx context.Context, tx db.Tx, code string, level Level, parent string) error {
	var got *string
	err := tx.QueryRow(ctx,
		`SELECT parent_code FROM canonical_geographies WHERE code = $1 AND level = $2`,
		code, string(level),
	).Scan(&got)
	if err != nil {
		return fmt.Errorf("%w: %s %q is not in the canonical geography master", ErrInvalidGeography, level, code)
	}
	if got == nil || *got != parent {
		return fmt.Errorf("%w: %s %q is not a child of %q", ErrInvalidGeography, level, code, parent)
	}
	return nil
}
