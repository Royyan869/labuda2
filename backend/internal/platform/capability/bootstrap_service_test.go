package capability_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/platform/capability"
	"github.com/labuda/backend/internal/platform/capability/entity"
)

// mockCapabilityRepository is a mock for testing.
type mockCapabilityRepository struct {
	// activeCapabilities tracks which user has which capabilities active
	activeCapabilities map[uuid.UUID]map[string]bool

	// createError forces Create to return an error
	createError error

	// hasError forces HasCapability to return an error
	hasError error

	// createdRecords tracks all Create calls
	createdRecords []*entity.UserCapability
}

func newMockCapabilityRepository() *mockCapabilityRepository {
	return &mockCapabilityRepository{
		activeCapabilities: make(map[uuid.UUID]map[string]bool),
	}
}

func (m *mockCapabilityRepository) Create(ctx context.Context, tx interface{}, cap *entity.UserCapability) error {
	if m.createError != nil {
		return m.createError
	}

	// Simulate unique constraint check
	userCaps, ok := m.activeCapabilities[cap.UserID]
	if !ok {
		userCaps = make(map[string]bool)
		m.activeCapabilities[cap.UserID] = userCaps
	}

	if userCaps[cap.Capability] {
		return &entity.ErrDuplicateCapability{
			UserID:     cap.UserID,
			Capability: cap.Capability,
		}
	}

	// Mark as active
	userCaps[cap.Capability] = true
	m.createdRecords = append(m.createdRecords, cap)
	return nil
}

func (m *mockCapabilityRepository) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.UserCapability, error) {
	return nil, errors.New("not implemented")
}

func (m *mockCapabilityRepository) GetActiveCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capability string) (*entity.UserCapability, error) {
	return nil, errors.New("not implemented")
}

func (m *mockCapabilityRepository) ListActiveCapabilities(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*entity.UserCapability, error) {
	return nil, errors.New("not implemented")
}

func (m *mockCapabilityRepository) Revoke(ctx context.Context, tx interface{}, id uuid.UUID, revokedAt *interface{}) error {
	return errors.New("not implemented")
}

func (m *mockCapabilityRepository) HasCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capability string) (bool, error) {
	if m.hasError != nil {
		return false, m.hasError
	}

	userCaps, ok := m.activeCapabilities[userID]
	if !ok {
		return false, nil
	}
	return userCaps[capability], nil
}

func (m *mockCapabilityRepository) HasAnyCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capabilities []string) (bool, error) {
	return false, errors.New("not implemented")
}

func (m *mockCapabilityRepository) CountActiveCapabilities(ctx context.Context, tx interface{}, userID uuid.UUID) (int, error) {
	return 0, errors.New("not implemented")
}

func (m *mockCapabilityRepository) ListUsersByCapability(ctx context.Context, tx interface{}, capability string) ([]uuid.UUID, error) {
	var result []uuid.UUID
	for userID, caps := range m.activeCapabilities {
		if caps[capability] {
			result = append(result, userID)
		}
	}
	return result, nil
}

// setActiveCapability is a test helper to pre-set an active capability.
func (m *mockCapabilityRepository) setActiveCapability(userID uuid.UUID, cap string) {
	userCaps, ok := m.activeCapabilities[userID]
	if !ok {
		userCaps = make(map[string]bool)
		m.activeCapabilities[userID] = userCaps
	}
	userCaps[cap] = true
}

// Test AssignInitialCapabilities

func TestBootstrapService_AssignInitialCapabilities_CreateNew(t *testing.T) {
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()
	grantedBy := uuid.New()

	caps := []string{
		capability.CapFinanceWithdrawRead.String(),
		capability.CapFinanceWithdrawReview.String(),
	}

	// EXECUTE
	result, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, &grantedBy)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 2, result.Created)
	assert.Equal(t, 0, result.SkippedExisting)
	assert.Equal(t, 0, result.Invalid)
	assert.Empty(t, result.Errors)

	// Verify records were created
	assert.Len(t, repo.createdRecords, 2)
	for _, rec := range repo.createdRecords {
		assert.Equal(t, userID, rec.UserID)
		assert.Equal(t, &grantedBy, rec.GrantedBy)
		assert.True(t, rec.IsActive())
	}
}

func TestBootstrapService_AssignInitialCapabilities_SkipDuplicate(t *testing.T) {
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	// Pre-set one capability as active
	existingCap := capability.CapFinanceWithdrawRead.String()
	repo.setActiveCapability(userID, existingCap)

	caps := []string{
		existingCap, // Should be skipped
		capability.CapFinanceWithdrawReview.String(), // Should be created
	}

	// EXECUTE
	result, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 1, result.SkippedExisting)
	assert.Equal(t, 0, result.Invalid)
	assert.Empty(t, result.Errors)
}

func TestBootstrapService_AssignInitialCapabilities_RejectInvalid(t *testing.T) {
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	caps := []string{
		capability.CapFinanceWithdrawRead.String(),   // Valid
		"invalid.capability.name",                    // Invalid
		"finance.withdraw.approve",                   // Invalid (not a defined constant)
		capability.CapFinanceWithdrawReview.String(), // Valid
	}

	// EXECUTE
	result, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 2, result.Created) // Only valid ones created
	assert.Equal(t, 0, result.SkippedExisting)
	assert.Equal(t, 2, result.Invalid) // Two invalid ones
	assert.Len(t, result.Errors, 2)

	// Verify error messages
	for _, e := range result.Errors {
		assert.Equal(t, "invalid capability string", e.Reason)
	}
}

func TestBootstrapService_AssignInitialCapabilities_MixedValidInvalidDuplicate(t *testing.T) {
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	// Pre-set one capability as active
	existingCap := capability.CapFinanceWithdrawRead.String()
	repo.setActiveCapability(userID, existingCap)

	caps := []string{
		existingCap,       // Duplicate - should skip
		"invalid.bad.cap", // Invalid
		capability.CapFinanceWithdrawReview.String(), // Valid - should create
		"another.invalid", // Invalid
	}

	// EXECUTE
	result, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 1, result.SkippedExisting)
	assert.Equal(t, 2, result.Invalid)
	assert.Len(t, result.Errors, 2)
}

func TestBootstrapService_AssignInitialCapabilities_IdempotentRerun(t *testing.T) {
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	caps := []string{
		capability.CapFinanceWithdrawRead.String(),
		capability.CapFinanceWithdrawReview.String(),
	}

	// EXECUTE FIRST RUN
	result1, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, result1.Created)
	assert.Equal(t, 0, result1.SkippedExisting)

	// EXECUTE SECOND RUN (should skip all)
	result2, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result2.Created)
	assert.Equal(t, 2, result2.SkippedExisting)

	// Verify no duplicate records
	totalCreated := len(repo.createdRecords)
	assert.Equal(t, 2, totalCreated, "Total created should still be 2")
}

func TestBootstrapService_AssignInitialCapabilities_CreateErrors(t *testing.T) {
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	// Force Create to fail
	repo.createError = errors.New("database connection failed")

	caps := []string{
		capability.CapFinanceWithdrawRead.String(),
	}

	// EXECUTE
	result, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)

	// ASSERT
	require.NoError(t, err) // Service doesn't fail, returns errors in result
	assert.Equal(t, 0, result.Created)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Reason, "grant failed")
}

// Test HasCapability Errors

func TestBootstrapService_AssignInitialCapabilities_HasCapabilityError(t *testing.T) {
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	// Force HasCapability to fail
	repo.hasError = errors.New("database error")

	caps := []string{
		capability.CapFinanceWithdrawRead.String(),
	}

	// EXECUTE
	result, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 0, result.Created)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Reason, "check existing failed")
}

// Test Presets

func TestGetPresetCapabilities_ValidPresets(t *testing.T) {
	tests := []struct {
		name      string
		preset    string
		wantCount int
		wantCaps  []string
	}{
		{
			name:      "finance_reviewer",
			preset:    capability.PresetFinanceReviewer,
			wantCount: 3,
			wantCaps: []string{
				capability.CapFinanceWithdrawRead.String(),
				capability.CapFinanceWithdrawReview.String(),
				capability.CapFinanceDisputeResolve.String(),
			},
		},
		{
			name:      "governance_basic",
			preset:    capability.PresetGovernanceBasic,
			wantCount: 3,
		},
		{
			name:      "moderation_basic",
			preset:    capability.PresetModerationBasic,
			wantCount: 3,
		},
		{
			name:      "seller_verification",
			preset:    capability.PresetSellerVerification,
			wantCount: 1,
		},
		{
			name:      "config_manager",
			preset:    capability.PresetConfigManager,
			wantCount: 3,
		},
		{
			name:      "support_admin",
			preset:    capability.PresetSupportAdmin,
			wantCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps, err := capability.GetPresetCapabilities(tt.preset)
			require.NoError(t, err)
			assert.Len(t, caps, tt.wantCount)

			if tt.wantCaps != nil {
				assert.ElementsMatch(t, tt.wantCaps, caps)
			}
		})
	}
}

func TestGetPresetCapabilities_UnknownPreset(t *testing.T) {
	_, err := capability.GetPresetCapabilities("unknown_preset")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown preset")
}

func TestBootstrapService_AssignInitialCapabilitiesFromPreset(t *testing.T) {
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	// EXECUTE
	result, err := service.AssignInitialCapabilitiesFromPreset(
		context.Background(),
		nil,
		userID,
		capability.PresetFinanceReviewer,
		nil,
	)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 3, result.Created)
	assert.Equal(t, 0, result.SkippedExisting)
	assert.Equal(t, 0, result.Invalid)
	assert.Empty(t, result.Errors)
}

func TestBootstrapService_AssignInitialCapabilitiesFromPreset_UnknownPreset(t *testing.T) {
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	// EXECUTE
	_, err := service.AssignInitialCapabilitiesFromPreset(
		context.Background(),
		nil,
		userID,
		"unknown_preset",
		nil,
	)

	// ASSERT
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown preset")
}

// Test ValidateCapabilities

func TestValidateCapabilities_AllValid(t *testing.T) {
	caps := []string{
		capability.CapFinanceWithdrawRead.String(),
		capability.CapFinanceWithdrawReview.String(),
		capability.CapGovernanceAuditRead.String(),
	}

	valid, invalid := capability.ValidateCapabilities(caps)

	assert.Len(t, valid, 3)
	assert.Len(t, invalid, 0)
}

func TestValidateCapabilities_SomeInvalid(t *testing.T) {
	caps := []string{
		capability.CapFinanceWithdrawRead.String(),
		"invalid.capability",
		capability.CapFinanceWithdrawReview.String(),
		"another.bad",
	}

	valid, invalid := capability.ValidateCapabilities(caps)

	assert.Len(t, valid, 2)
	assert.Len(t, invalid, 2)
	assert.ElementsMatch(t, []string{"invalid.capability", "another.bad"}, invalid)
}

func TestValidateCapabilities_AllInvalid(t *testing.T) {
	caps := []string{
		"invalid.one",
		"invalid.two",
	}

	valid, invalid := capability.ValidateCapabilities(caps)

	assert.Len(t, valid, 0)
	assert.Len(t, invalid, 2)
}

func TestValidateCapabilities_Empty(t *testing.T) {
	valid, invalid := capability.ValidateCapabilities([]string{})

	assert.Len(t, valid, 0)
	assert.Len(t, invalid, 0)
}

func TestValidateCapabilities_KnownPromotionExternalProductReview(t *testing.T) {
	caps := []string{
		capability.CapPromotionExternalProductReview.String(),
		"invalid.capability",
	}

	valid, invalid := capability.ValidateCapabilities(caps)

	assert.Contains(t, valid, capability.CapPromotionExternalProductReview.String())
	assert.Contains(t, invalid, "invalid.capability")
}

func TestBootstrapService_AssignInitialCapabilities_AllowsPromotionExternalProductReview(t *testing.T) {
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	result, err := service.AssignInitialCapabilities(
		context.Background(),
		nil,
		userID,
		[]string{capability.CapPromotionExternalProductReview.String()},
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 0, result.SkippedExisting)
	assert.Equal(t, 0, result.Invalid)
	assert.Empty(t, result.Errors)

	hasCap, err := repo.HasCapability(context.Background(), nil, userID, capability.CapPromotionExternalProductReview.String())
	require.NoError(t, err)
	assert.True(t, hasCap)
}

// Safety Tests

func TestBootstrapService_Safety_NeverRevokesExisting(t *testing.T) {
	// This test ensures that running bootstrap NEVER revokes existing capabilities
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	// Pre-set a capability that is NOT in the list we're about to assign
	otherCap := capability.CapGovernanceAuditRead.String()
	repo.setActiveCapability(userID, otherCap)

	// Assign only finance capabilities
	caps := []string{
		capability.CapFinanceWithdrawRead.String(),
	}

	// EXECUTE
	result, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)

	// Verify the other capability is still active
	hasCap, _ := repo.HasCapability(context.Background(), nil, userID, otherCap)
	assert.True(t, hasCap, "Other capability should still be active")
}

func TestBootstrapService_Safety_NoDuplicateActiveGrants(t *testing.T) {
	// This test ensures that the unique constraint is respected
	// SETUP
	repo := newMockCapabilityRepository()
	service := capability.NewBootstrapService(repo)
	userID := uuid.New()

	caps := []string{
		capability.CapFinanceWithdrawRead.String(),
	}

	// EXECUTE FIRST TIME
	result1, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result1.Created)

	// EXECUTE SECOND TIME - should detect duplicate
	result2, err := service.AssignInitialCapabilities(context.Background(), nil, userID, caps, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result2.Created)
	assert.Equal(t, 1, result2.SkippedExisting)

	// Verify only one record created
	assert.Len(t, repo.createdRecords, 1)
}


