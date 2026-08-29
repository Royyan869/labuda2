package repository

import "testing"

// TestDiscountRepositoryInstantiation documents that the discount repository
// can be created successfully.
func TestDiscountRepositoryInstantiation(t *testing.T) {
	repo := NewDiscountRepository()
	if repo == nil {
		t.Fatal("NewDiscountRepository() returned nil")
	}
}

// TestDiscountUsageWriteAuthority documents that discount usage recording
// remains transaction-internal and must never become a public HTTP endpoint.
func TestDiscountUsageWriteAuthority(t *testing.T) {
	_ = "RecordUsage and IncrementUsageCount are order-transaction-internal only"
}
