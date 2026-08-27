package shared

import (
	"reflect"
	"testing"
)

func TestNormalizeCertificatesCanonicalOrderAndDedupes(t *testing.T) {
	got, err := NormalizeCertificates([]string{
		" Health ",
		"breeder",
		"contest",
		"BREEDER",
		"",
		"ownership",
	})
	if err != nil {
		t.Fatalf("NormalizeCertificates returned error: %v", err)
	}

	want := []string{"breeder", "contest", "ownership", "health"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeCertificates() = %#v, want %#v", got, want)
	}
}

func TestNormalizeCertificatesRejectsUnknownValue(t *testing.T) {
	got, err := NormalizeCertificates([]string{"breeder", "bonus"})
	if err == nil {
		t.Fatalf("NormalizeCertificates() error = nil, want invalid certificate error; got %#v", got)
	}
	if err.Error() != "invalid certificate: bonus" {
		t.Fatalf("NormalizeCertificates() error = %v, want invalid certificate: bonus", err)
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
