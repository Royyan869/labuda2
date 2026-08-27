package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/money"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// SHIPPING PROOF VALIDATION TESTS
// ============================================================================

// TestMarkShipped_RequiresProofType tests that proof_type is required
func TestMarkShipped_RequiresProofType(t *testing.T) {
	order := createTestOrderPaid()

	// Try to mark shipped without proof_type
	err := order.MarkShipped(nil, strPtr("REF123"), nil, nil)

	// Should fail with InvalidShippingProofError
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proof_type is required")
	assert.Equal(t, StatusPaid, order.Status, "Status should remain paid")
	assert.Nil(t, order.AutoReleaseAt, "Timer should NOT start")
}

// TestMarkShipped_RejectsInvalidProofType tests that invalid proof_type is rejected
func TestMarkShipped_RejectsInvalidProofType(t *testing.T) {
	order := createTestOrderPaid()

	invalidType := "invalid"
	err := order.MarkShipped(&invalidType, strPtr("REF123"), nil, nil)

	// Should fail with InvalidShippingProofError
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proof_type must be one of")
	assert.Equal(t, StatusPaid, order.Status, "Status should remain paid")
	assert.Nil(t, order.AutoReleaseAt, "Timer should NOT start")
}

// TestMarkShipped_TrackingRequiresReference tests tracking type requires shipping_reference
func TestMarkShipped_TrackingRequiresReference(t *testing.T) {
	order := createTestOrderPaid()

	// proof_type = tracking without shipping_reference
	proofType := ProofTypeTracking
	err := order.MarkShipped(&proofType, nil, nil, nil)

	// Should fail with InvalidShippingProofError
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid shipping proof: tracking_number is required for proof_type=tracking")
	assert.Equal(t, StatusPaid, order.Status, "Status should remain paid")
	assert.Nil(t, order.AutoReleaseAt, "Timer should NOT start")
}

// TestMarkShipped_PhoneRequiresReference tests phone type requires shipping_reference
func TestMarkShipped_PhoneRequiresReference(t *testing.T) {
	order := createTestOrderPaid()

	// proof_type = phone without shipping_reference
	proofType := ProofTypePhone
	err := order.MarkShipped(&proofType, nil, nil, nil)

	// Should fail with InvalidShippingProofError
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid shipping proof: tracking_number is required for proof_type=phone")
	assert.Equal(t, StatusPaid, order.Status, "Status should remain paid")
	assert.Nil(t, order.AutoReleaseAt, "Timer should NOT start")
}

// TestMarkShipped_ManualRequiresProofMedia tests manual type requires shipping_proof_media
func TestMarkShipped_ManualRequiresProofMedia(t *testing.T) {
	order := createTestOrderPaid()

	// proof_type = manual without shipping_proof_media
	proofType := ProofTypeManual
	err := order.MarkShipped(&proofType, nil, nil, nil)

	// Should fail with InvalidShippingProofError
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "shipping_proof_media is required for proof_type=manual")
	assert.Equal(t, StatusPaid, order.Status, "Status should remain paid")
	assert.Nil(t, order.AutoReleaseAt, "Timer should NOT start")
}

// TestMarkShipped_PhoneValidatesFormat tests phone format validation
func TestMarkShipped_PhoneValidatesFormat(t *testing.T) {
	testCases := []struct {
		name      string
		phone     string
		wantError bool
	}{
		{"Valid Indonesian 08 prefix", "08123456789", false},
		{"Valid Indonesian +62 prefix", "+628123456789", false},
		{"Valid Indonesian 628 prefix", "628123456789", false},
		{"Invalid - too short", "08123", true},
		{"Invalid - wrong prefix", "09123456789", true},
		{"Invalid - non-numeric", "08abcdefghi", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			order := createTestOrderPaid()
			proofType := ProofTypePhone

			err := order.MarkShipped(&proofType, &tc.phone, nil, nil)

			if tc.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid shipping proof: phone number must be at least 10 digits (Indonesian format: 08xxxxxxxxxx, +628xxxxxxxxxx, or 628xxxxxxxxxx)")
				assert.Equal(t, StatusPaid, order.Status, "Status should remain paid")
				assert.Nil(t, order.AutoReleaseAt, "Timer should NOT start")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, StatusShipped, order.Status, "Status should be shipped")
				assert.NotNil(t, order.AutoReleaseAt, "Timer should start")
				assert.Equal(t, &tc.phone, order.TrackingNumber)
				assert.Equal(t, &proofType, order.ProofType)
			}
		})
	}
}

// TestMarkShipped_TrackingSuccess tests successful marking with tracking proof
func TestMarkShipped_TrackingSuccess(t *testing.T) {
	order := createTestOrderPaid()

	proofType := ProofTypeTracking
	reference := "JNE123456789"
	note := "Barang dikirim hari ini"

	err := order.MarkShipped(&proofType, &reference, nil, &note)

	// Should succeed
	assert.NoError(t, err)
	assert.Equal(t, StatusShipped, order.Status, "Status should be shipped")
	assert.NotNil(t, order.AutoReleaseAt, "Timer should start")
	assert.Equal(t, &proofType, order.ProofType)
	assert.Equal(t, &reference, order.TrackingNumber)
	assert.Nil(t, order.ShippingProofMedia)
	assert.Equal(t, &note, order.ShippingNote)

	// Verify timer is set to ~5 days from now
	expectedTimer := time.Now().Add(5 * 24 * time.Hour)
	timeDiff := order.AutoReleaseAt.Sub(expectedTimer)
	assert.Less(t, timeDiff.Abs(), time.Second, "Timer should be ~5 days from now")
}

// TestMarkShipped_PhoneSuccess tests successful marking with phone proof
func TestMarkShipped_PhoneSuccess(t *testing.T) {
	order := createTestOrderPaid()

	proofType := ProofTypePhone
	reference := "08123456789"

	err := order.MarkShipped(&proofType, &reference, nil, nil)

	// Should succeed
	assert.NoError(t, err)
	assert.Equal(t, StatusShipped, order.Status, "Status should be shipped")
	assert.NotNil(t, order.AutoReleaseAt, "Timer should start")
	assert.Equal(t, &proofType, order.ProofType)
	assert.Equal(t, &reference, order.TrackingNumber)
}

// TestMarkShipped_ManualSuccess tests successful marking with manual proof
func TestMarkShipped_ManualSuccess(t *testing.T) {
	order := createTestOrderPaid()

	proofType := ProofTypeManual
	mediaURL := "https://storage.example.com/proof/abc123.jpg"
	note := "Bukti foto barang diambil kurir"

	err := order.MarkShipped(&proofType, nil, &mediaURL, &note)

	// Should succeed
	assert.NoError(t, err)
	assert.Equal(t, StatusShipped, order.Status, "Status should be shipped")
	assert.NotNil(t, order.AutoReleaseAt, "Timer should start")
	assert.Equal(t, &proofType, order.ProofType)
	assert.Nil(t, order.TrackingNumber)
	assert.Equal(t, &mediaURL, order.ShippingProofMedia)
	assert.Equal(t, &note, order.ShippingNote)
}

// TestMarkShipped_OnlyStartsTimerAfterProofValidation tests timer only starts after valid proof
func TestMarkShipped_OnlyStartsTimerAfterProofValidation(t *testing.T) {
	// This test verifies the FAIL CONDITION: "timer still seller-controlled"
	// Timer should ONLY start after proof validation passes

	t.Run("Invalid proof - no timer", func(t *testing.T) {
		order := createTestOrderPaid()
		proofType := ProofTypeTracking // tracking type but no reference

		err := order.MarkShipped(&proofType, nil, nil, nil)

		assert.Error(t, err)
		assert.Nil(t, order.AutoReleaseAt, "Timer should NOT start with invalid proof")
	})

	t.Run("Valid proof - timer starts", func(t *testing.T) {
		order := createTestOrderPaid()
		proofType := ProofTypeTracking
		reference := "JNE123456789"

		err := order.MarkShipped(&proofType, &reference, nil, nil)

		assert.NoError(t, err)
		assert.NotNil(t, order.AutoReleaseAt, "Timer SHOULD start with valid proof")
	})
}

// =============================================================================
// OVERDUE SLA ENFORCEMENT TESTS
// ============================================================================

// TestIsEligibleForBuyerCancelDueToOverdue tests buyer can cancel overdue orders
func TestIsEligibleForBuyerCancelDueToOverdue(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name              string
		readyToShipBy     *time.Time
		status            Status
		wantEligible      bool
		overdueTier       OverdueTier
	}{
		{
			name:          "Not overdue - not eligible",
			readyToShipBy: ptrTime(now.Add(24 * time.Hour)),
			status:        StatusPaid,
			wantEligible:  false,
			overdueTier:   OverdueNone,
		},
		{
			name:          "Tier 1 overdue (0-2 days) - not eligible",
			readyToShipBy: ptrTime(now.Add(-2 * 24 * time.Hour)),
			status:        StatusPaid,
			wantEligible:  false,
			overdueTier:   OverdueTier1,
		},
		{
			name:          "Tier 2 overdue (3+ days) - eligible",
			readyToShipBy: ptrTime(now.Add(-4 * 24 * time.Hour)),
			status:        StatusPaid,
			wantEligible:  true,
			overdueTier:   OverdueTier2,
		},
		{
			name:          "Tier 3 overdue (7+ days) - eligible",
			readyToShipBy: ptrTime(now.Add(-10 * 24 * time.Hour)),
			status:        StatusPaid,
			wantEligible:  true,
			overdueTier:   OverdueTier3,
		},
		{
			name:          "Already shipped - not eligible",
			readyToShipBy: ptrTime(now.Add(-10 * 24 * time.Hour)),
			status:        StatusShipped,
			wantEligible:  false,
			overdueTier:   OverdueNone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			order := &Order{
				ID:              uuid.New(),
				BuyerID:         uuid.New(),
				SellerID:        uuid.New(),
				SourceType:      OrderSourceForSale,
				SourceID:        uuid.New(),
				Quantity:        1,
				UnitPrice:       money.New(100000),
				Subtotal:        money.New(100000),
				ShippingTotal:   money.New(10000),
				CommissionPercent: 5,
				CommissionAmount: money.New(5500),
				ShippingOptionID:     uuidPtr(),
				ShippingOptionName:   "JNE Regular",
				ShippingTransportType: "land",
				Status:          tc.status,
				EscrowStatus:     EscrowStatusHolding,
				ReadyToShipBy:    tc.readyToShipBy,
				CreatedAt:        now,
				UpdatedAt:        now,
			}

			isEligible := order.IsEligibleForBuyerCancelDueToOverdue()
			assert.Equal(t, tc.wantEligible, isEligible)

			// Verify overdue tier calculation
			overdueInfo := order.CalculateOverdueInfo()
			assert.Equal(t, tc.overdueTier, overdueInfo.Tier)
		})
	}
}

// =============================================================================
// HELPER FUNCTIONS
// ============================================================================

func createTestOrderPaid() *Order {
	now := time.Now()
	return &Order{
		ID:                   uuid.New(),
		BuyerID:              uuid.New(),
		SellerID:             uuid.New(),
		SourceType:           OrderSourceForSale,
		SourceID:             uuid.New(),
		Quantity:             1,
		UnitPrice:            money.New(100000),
		Subtotal:             money.New(100000),
		ShippingTotal:        money.New(10000),
		CommissionPercent:    5,
		CommissionAmount:     money.New(5500),
		ShippingOptionID:     uuidPtr(),
		ShippingOptionName:   "JNE Regular",
		ShippingTransportType: "land",
		Status:               StatusPaid,
		EscrowStatus:         EscrowStatusHolding,
		ReadyToShipBy:        ptrTime(now.Add(3 * 24 * time.Hour)),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func strPtr(s string) *string {
	return &s
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func uuidPtr() *uuid.UUID {
	id := uuid.New()
	return &id
}


