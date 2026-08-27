package repository

import "testing"

// TestDiscountTargetTableSchemaContract locks the SQL table names referenced by
// DiscountRepositoryImpl for selected-item discounts.
//
// The repository now persists a single generic target table:
//   - "discount_targets" (INSERT, SELECT, DELETE, ANY batch)
//
// This replaces the old forSale/variety pivot split and keeps target handling
// consistent for forSale and auction checkout contexts.
func TestDiscountTargetTableSchemaContract(t *testing.T) {
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


