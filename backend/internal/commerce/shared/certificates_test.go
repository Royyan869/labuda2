package shared

import (
	"reflect"
	"testing"

	productEntity "github.com/hishumi/backend/internal/commerce/product/entity"
)

func TestNormalizeCertificatesCanonicalOrderAndDedupes(t *testing.T) {
	got, err := NormalizeCertificates([]string{
		" Health ",
		"breeder",
		"contest",
		"BREEDER",
		"",
		"import",
	})
	if err != nil {
		t.Fatalf("NormalizeCertificates returned error: %v", err)
	}

	want := []string{"breeder", "contest", "import", "health"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeCertificates() = %#v, want %#v", got, want)
	}
}

func TestNormalizeCertificatesRejectsUnknownValue(t *testing.T) {
	got, err := NormalizeCertificates([]string{"breeder", "bonus"})
	if err == nil {
		t.Fatalf("NormalizeCertificates() error = nil, want invalid certificate error; got %#v", got)
	}
	if err.Error() != `invalid certificate value "bonus": allowed breeder, contest, import, health` {
		t.Fatalf("NormalizeCertificates() error = %v, want invalid certificate value \"bonus\" with the canonical allowed list", err)
	}
}

func TestNormalizeCertificatesEmptyInput(t *testing.T) {
	got, err := NormalizeCertificates(nil)
	if err != nil {
		t.Fatalf("NormalizeCertificates returned error: %v", err)
	}
	if got == nil {
		t.Fatalf("NormalizeCertificates() returned nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("NormalizeCertificates() length = %d, want 0", len(got))
	}
}

// Negative contract: `ownership` was retired when the owner established the
// canonical vocabulary (breeder, contest, import, health). It must never be
// accepted again by any producer path.
func TestNormalizeCertificatesRejectsRetiredOwnershipValue(t *testing.T) {
	if _, err := NormalizeCertificates([]string{"ownership"}); err == nil {
		t.Fatal("NormalizeCertificates accepted retired certificate value 'ownership'")
	}
}

// Positive lock: the commerce wire normalizer and the product entity validate
// exactly the same vocabulary, so the two paths cannot diverge again.
func TestCertificateVocabularyHasSingleAuthority(t *testing.T) {
	for _, value := range productEntity.CanonicalCertificateOrder {
		if _, err := NormalizeCertificates([]string{value}); err != nil {
			t.Fatalf("canonical certificate %q rejected by NormalizeCertificates: %v", value, err)
		}
	}

	ordered := append([]string{}, productEntity.CanonicalCertificateOrder...)
	if err := productEntity.ValidateCertificates(&ordered); err != nil {
		t.Fatalf("product entity rejected its own canonical certificate order: %v", err)
	}

	normalized, err := NormalizeCertificates(productEntity.CanonicalCertificateOrder)
	if err != nil {
		t.Fatalf("NormalizeCertificates rejected the canonical order: %v", err)
	}
	if !reflect.DeepEqual(normalized, productEntity.CanonicalCertificateOrder) {
		t.Fatalf(
			"NormalizeCertificates(%v) = %v, want the entity order unchanged",
			productEntity.CanonicalCertificateOrder,
			normalized,
		)
	}
}
