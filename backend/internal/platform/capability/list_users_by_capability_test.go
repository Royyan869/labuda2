package capability

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/platform/capability/entity"
)

// TestListUsersByCapability_ActiveCapabilityReturned verifies that users with
// an active (non-revoked) capability are returned.
func TestListUsersByCapability_ActiveCapabilityReturned(t *testing.T) {
	repo := newMockCapabilityRepository()
	checker := NewChecker(repo)

	userA := uuid.New()
	userB := uuid.New()

	// Grant finance.withdraw.review to both users
	capA := entity.NewCapabilityGrant(userA, CapFinanceWithdrawReview.String(), nil)
	capB := entity.NewCapabilityGrant(userB, CapFinanceWithdrawReview.String(), nil)
	require.NoError(t, repo.Create(context.Background(), nil, capA))
	require.NoError(t, repo.Create(context.Background(), nil, capB))

	// Also grant a different capability to userA (should not appear in result)
	capExtra := entity.NewCapabilityGrant(userA, CapGovernanceAuditRead.String(), nil)
	require.NoError(t, repo.Create(context.Background(), nil, capExtra))

	ids, err := checker.repo.ListUsersByCapability(context.Background(), nil, CapFinanceWithdrawReview.String())
	require.NoError(t, err)
	assert.Len(t, ids, 2)
	assert.Contains(t, ids, userA)
	assert.Contains(t, ids, userB)
}

// TestListUsersByCapability_RevokedExcluded verifies that users whose
// capability has been revoked are NOT returned.
func TestListUsersByCapability_RevokedExcluded(t *testing.T) {
	repo := newMockCapabilityRepository()

	userActive := uuid.New()
	userRevoked := uuid.New()

	capActive := entity.NewCapabilityGrant(userActive, CapFinanceWithdrawReview.String(), nil)
	capRevoked := entity.NewCapabilityGrant(userRevoked, CapFinanceWithdrawReview.String(), nil)
	require.NoError(t, repo.Create(context.Background(), nil, capActive))
	require.NoError(t, repo.Create(context.Background(), nil, capRevoked))

	// Revoke userRevoked's capability
	now := time.Now()
	capRevoked.RevokedAt = &now
	repo.revokedCapabilities[userRevoked] = append(repo.revokedCapabilities[userRevoked], CapFinanceWithdrawReview.String())

	ids, err := repo.ListUsersByCapability(context.Background(), nil, CapFinanceWithdrawReview.String())
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Contains(t, ids, userActive)
	assert.NotContains(t, ids, userRevoked)
}

// TestListUsersByCapability_DeduplicatedRows verifies that a user holding the
// same capability string from multiple grants is returned only once.
func TestListUsersByCapability_DeduplicatedRows(t *testing.T) {
	repo := newMockCapabilityRepository()

	userID := uuid.New()

	// Simulate two grants of the same capability (possible if resource_id differs)
	cap1 := entity.NewCapabilityGrant(userID, CapFinanceWithdrawReview.String(), nil)
	cap2 := entity.NewCapabilityGrant(userID, CapFinanceWithdrawReview.String(), nil)
	cap2.ID = uuid.New() // Different grant ID

	// Bypass duplicate detection in mock by inserting directly
	repo.capabilities[userID] = append(repo.capabilities[userID], cap1, cap2)

	ids, err := repo.ListUsersByCapability(context.Background(), nil, CapFinanceWithdrawReview.String())
	require.NoError(t, err)
	assert.Len(t, ids, 1, "same user should appear only once even with multiple grants")
	assert.Equal(t, userID, ids[0])
}

// TestListUsersByCapability_NoMatchReturnsEmpty verifies that an empty slice
// is returned when no users hold the queried capability.
func TestListUsersByCapability_NoMatchReturnsEmpty(t *testing.T) {
	repo := newMockCapabilityRepository()

	userID := uuid.New()
	cap := entity.NewCapabilityGrant(userID, CapGovernanceAuditRead.String(), nil)
	require.NoError(t, repo.Create(context.Background(), nil, cap))

	ids, err := repo.ListUsersByCapability(context.Background(), nil, CapFinanceWithdrawReview.String())
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// TestCapabilityService_ListUsersByCapability_InvalidCapability verifies
// that the service layer rejects invalid capability strings.
func TestCapabilityService_ListUsersByCapability_InvalidCapability(t *testing.T) {
	repo := newMockCapabilityRepository()
	svc := &testCapabilityService{repo: repo}

	ids, err := svc.ListUsersByCapability(context.Background(), "invalid.capability.string")
	assert.Error(t, err)
	assert.Nil(t, ids)
}

// TestCapabilityService_ListUsersByCapability_ValidCapability verifies
// the service passthrough for a valid capability.
func TestCapabilityService_ListUsersByCapability_ValidCapability(t *testing.T) {
	repo := newMockCapabilityRepository()

	userID := uuid.New()
	cap := entity.NewCapabilityGrant(userID, CapSellerVerificationReview.String(), nil)
	require.NoError(t, repo.Create(context.Background(), nil, cap))

	svc := &testCapabilityService{repo: repo}

	ids, err := svc.ListUsersByCapability(context.Background(), CapSellerVerificationReview.String())
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Equal(t, userID, ids[0])
}

// testCapabilityService mirrors the service-layer validation without
// requiring an audit logger (which is not relevant to this test).
type testCapabilityService struct {
	repo *mockCapabilityRepository
}

func (s *testCapabilityService) ListUsersByCapability(ctx context.Context, capabilityStr string) ([]uuid.UUID, error) {
	if !IsValid(capabilityStr) {
		return nil, &entity.ErrInvalidCapability{Capability: capabilityStr}
	}
	return s.repo.ListUsersByCapability(ctx, nil, capabilityStr)
}


