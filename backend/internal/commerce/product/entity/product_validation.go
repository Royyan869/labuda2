package entity

import (
	"fmt"
	"strings"
)

// Canonical allowed certificate values.
//
// This entity is the ONE authority for the certificate vocabulary.
// [CanonicalCertificateOrder] is what `shared.NormalizeCertificates` reads for
// validation and ordering, so product validation and the commerce wire layer
// can never drift apart again.
//
// Business truth (owner): a certificate is a plain statement the seller makes
// about the fish — contest, breeder, import, or health. It is product content
// like variety/breeder/bloodline, never a document upload.
const (
	CertificateBreeder = "breeder"
	CertificateContest = "contest"
	CertificateImport  = "import"
	CertificateHealth  = "health"
)

// CanonicalCertificateOrder lists the accepted certificate values in the order
// they are persisted and rendered to buyers.
var CanonicalCertificateOrder = []string{
	CertificateBreeder,
	CertificateContest,
	CertificateImport,
	CertificateHealth,
}

var allowedCertificates = func() map[string]struct{} {
	set := make(map[string]struct{}, len(CanonicalCertificateOrder))
	for _, value := range CanonicalCertificateOrder {
		set[value] = struct{}{}
	}
	return set
}()

// ValidateTitle validates a product title pointer for update flows.
// Nil means absent (no change). Non-nil is trimmed and must be 1..200.
func ValidateTitle(title *string) error {
	if title == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*title)
	if trimmed == "" {
		return fmt.Errorf("title is required")
	}
	if len(trimmed) < 1 || len(trimmed) > 200 {
		return fmt.Errorf("title must be between 1 and 200 characters")
	}
	return nil
}

// ValidateDescription validates description pointer (max 5000).
func ValidateDescription(desc *string) error {
	if desc == nil {
		return nil
	}
	if len(*desc) > 5000 {
		return fmt.Errorf("description must be at most 5000 characters")
	}
	return nil
}

// ValidateCertificates validates certificates slice pointer.
// Nil means absent. Empty slice means clear. Each value must be allowed.
func ValidateCertificates(certs *[]string) error {
	if certs == nil {
		return nil
	}
	for _, c := range *certs {
		if _, ok := allowedCertificates[c]; !ok {
			return fmt.Errorf(
				"invalid certificate value %q: allowed %s",
				c,
				strings.Join(CanonicalCertificateOrder, ", "),
			)
		}
	}
	return nil
}

// ValidatePreparationTime validates preparation time pointer via canonical enum.
func ValidatePreparationTime(pt *string) error {
	if pt == nil {
		return nil
	}
	// Use string check against canonical values to avoid import cycle with forsale entity.
	switch *pt {
	case "1_3_days", "4_7_days", "8_15_days":
		return nil
	default:
		return fmt.Errorf("invalid preparation_time %q: allowed 1_3_days, 4_7_days, 8_15_days", *pt)
	}
}

// ProductContentPatch holds editable Product content for draft update.
// Nil means absent (preserve), non-nil means set (including empty slice for clear).
// FarmAddressID, SellerID, ID, SellingSurface are intentionally absent (creation-only/immutable).
type ProductContentPatch struct {
	Title           *string
	Description     *string
	MediaURLs       *[]ProductMedia
	Variety         *string
	SizeCM          *int
	AgeMonths       *int
	Gender          *string
	Breeder         *string
	Bloodline       *string
	Certificates    *[]string
	PreparationTime *string
}

// Validate validates the patch using canonical rules.
func (p *ProductContentPatch) Validate() error {
	if err := ValidateTitle(p.Title); err != nil {
		return err
	}
	if err := ValidateDescription(p.Description); err != nil {
		return err
	}
	if err := ValidateCertificates(p.Certificates); err != nil {
		return err
	}
	if err := ValidatePreparationTime(p.PreparationTime); err != nil {
		return err
	}
	return nil
}

// ApplyTo applies patch to product (mutates in place). Caller must have validated.
func (p *ProductContentPatch) ApplyTo(product *Product) {
	if p.Title != nil {
		product.Title = strings.TrimSpace(*p.Title)
	}
	if p.Description != nil {
		product.Description = *p.Description
	}
	if p.MediaURLs != nil {
		product.MediaURLs = *p.MediaURLs
	}
	if p.Variety != nil {
		product.Variety = *p.Variety
	}
	if p.SizeCM != nil {
		product.SizeCm = p.SizeCM
	}
	if p.AgeMonths != nil {
		product.AgeMonths = p.AgeMonths
	}
	if p.Gender != nil {
		product.Gender = p.Gender
	}
	if p.Breeder != nil {
		product.Breeder = p.Breeder
	}
	if p.Bloodline != nil {
		product.Bloodline = p.Bloodline
	}
	if p.Certificates != nil {
		product.Certificates = *p.Certificates
	}
	if p.PreparationTime != nil {
		product.PreparationTime = *p.PreparationTime
	}
}
