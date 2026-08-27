// Package capability provides unit tests for the capability checker.
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

// mockCapabilityRepository is a mock implementation of CapabilityRepository for testing.
type mockCapabilityRepository struct {
	capabilities        map[uuid.UUID][]*entity.UserCapability // userID -> capabilities
	duplicateAttempt    map[uuid.UUID]string                   // userID -> capability (for duplicate error simulation)
	revokedCapabilities map[uuid.UUID][]string                 // userID -> revoked capabilities
}

func newMockCapabilityRepository() *mockCapabilityRepository {
	return &mockCapabilityRepository{
		capabilities:        make(map[uuid.UUID][]*entity.UserCapability),
		duplicateAttempt:    make(map[uuid.UUID]string),
		revokedCapabilities: make(map[uuid.UUID][]string),
	}
}

// Create implements CapabilityRepository.
func (m *mockCapabilityRepository) Create(ctx context.Context, tx interface{}, cap *entity.UserCapability) error {
	// Check for duplicate
	if dupCap, ok := m.duplicateAttempt[cap.UserID]; ok && dupCap == cap.Capability {
		return &entity.ErrDuplicateCapability{UserID: cap.UserID, Capability: cap.Capability}
	}

	m.capabilities[cap.UserID] = append(m.capabilities[cap.UserID], cap)
	return nil
}

// GetByID implements CapabilityRepository.
func (m *mockCapabilityRepository) GetByID(ctx context.Context, tx interface{}, id uuid.UUID) (*entity.UserCapability, error) {
	for _, caps := range m.capabilities {
		for _, cap := range caps {
			if cap.ID == id {
				return cap, nil
			}
		}
	}
	return nil, nil
}

// GetActiveCapability implements CapabilityRepository.
func (m *mockCapabilityRepository) GetActiveCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capability string) (*entity.UserCapability, error) {
	// Check if it was revoked
	if revoked, ok := m.revokedCapabilities[userID]; ok {
		for _, r := range revoked {
			if r == capability {
				return nil, nil // Not found (revoked)
			}
		}
	}

	caps, ok := m.capabilities[userID]
	if !ok {
		return nil, nil
	}
	for _, cap := range caps {
		if cap.Capability == capability && cap.IsActive() {
			return cap, nil
		}
	}
	return nil, nil
}

// ListActiveCapabilities implements CapabilityRepository.
func (m *mockCapabilityRepository) ListActiveCapabilities(ctx context.Context, tx interface{}, userID uuid.UUID) ([]*entity.UserCapability, error) {
	caps, ok := m.capabilities[userID]
	if !ok {
		return nil, nil
	}

	var active []*entity.UserCapability
	for _, cap := range caps {
		if cap.IsActive() {
			// Check if not in revoked list
			if revoked, ok := m.revokedCapabilities[userID]; ok {
				isRevoked := false
				for _, r := range revoked {
					if r == cap.Capability {
						isRevoked = true
						break
					}
				}
				if !isRevoked {
					active = append(active, cap)
				}
			} else {
				active = append(active, cap)
			}
		}
	}
	return active, nil
}

// Revoke implements CapabilityRepository.
func (m *mockCapabilityRepository) Revoke(ctx context.Context, tx interface{}, id uuid.UUID, revokedAt *interface{}) error {
	for userID, caps := range m.capabilities {
		for _, cap := range caps {
			if cap.ID == id {
				now := time.Now()
				cap.RevokedAt = &now
				if m.revokedCapabilities[userID] == nil {
					m.revokedCapabilities[userID] = []string{}
				}
				m.revokedCapabilities[userID] = append(m.revokedCapabilities[userID], cap.Capability)
				return nil
			}
		}
	}
	return &entity.ErrCapabilityNotFound{}
}

// HasCapability implements CapabilityRepository.
func (m *mockCapabilityRepository) HasCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capability string) (bool, error) {
	caps, ok := m.capabilities[userID]
	if !ok {
		return false, nil
	}

	for _, cap := range caps {
		if cap.Capability == capability && cap.IsActive() {
			// Check if not revoked
			if revoked, ok := m.revokedCapabilities[userID]; ok {
				for _, r := range revoked {
					if r == capability {
						return false, nil
					}
				}
			}
			return true, nil
		}
	}
	return false, nil
}

// HasAnyCapability implements CapabilityRepository.
func (m *mockCapabilityRepository) HasAnyCapability(ctx context.Context, tx interface{}, userID uuid.UUID, capabilities []string) (bool, error) {
	for _, cap := range capabilities {
		has, err := m.HasCapability(ctx, tx, userID, cap)
		if err != nil {
			return false, err
		}
		if has {
			return true, nil
		}
	}
	return false, nil
}

// CountActiveCapabilities implements CapabilityRepository.
func (m *mockCapabilityRepository) CountActiveCapabilities(ctx context.Context, tx interface{}, userID uuid.UUID) (int, error) {
	caps, ok := m.capabilities[userID]
	if !ok {
		return 0, nil
	}

	count := 0
	for _, cap := range caps {
		if cap.IsActive() {
			if revoked, ok := m.revokedCapabilities[userID]; ok {
				isRevoked := false
				for _, r := range revoked {
					if r == cap.Capability {
						isRevoked = true
						break
					}
				}
				if !isRevoked {
					count++
				}
			} else {
				count++
			}
		}
	}
	return count, nil
}

// ListUsersByCapability implements CapabilityRepository.
func (m *mockCapabilityRepository) ListUsersByCapability(ctx context.Context, tx interface{}, capability string) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]bool)
	var result []uuid.UUID
	for userID, caps := range m.capabilities {
		for _, cap := range caps {
			if cap.Capability == capability && cap.IsActive() {
				if revoked, ok := m.revokedCapabilities[userID]; ok {
					isRevoked := false
					for _, r := range revoked {
						if r == capability {
							isRevoked = true
							break
						}
					}
					if isRevoked {
						continue
					}
				}
				if !seen[userID] {
					seen[userID] = true
					result = append(result, userID)
				}
			}
		}
	}
	return result, nil
}

// TestChecker_HasCapability tests the HasCapability method.
func TestChecker_HasCapability(t *testing.T) {
	t.Run("user has active capability", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()
		cap := entity.NewCapabilityGrant(userID, CapFinanceWithdrawRead.String(), nil)
		repo.Create(context.Background(), nil, cap)

		has, err := checker.HasCapability(context.Background(), userID, CapFinanceWithdrawRead)
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("user does not have capability", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()

		has, err := checker.HasCapability(context.Background(), userID, CapFinanceWithdrawRead)
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("capability is revoked", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()
		cap := entity.NewCapabilityGrant(userID, CapFinanceWithdrawRead.String(), nil)
		repo.Create(context.Background(), nil, cap)

		// Revoke the capability
		now := time.Now()
		cap.RevokedAt = &now
		repo.revokedCapabilities[userID] = append(repo.revokedCapabilities[userID], cap.Capability)

		has, err := checker.HasCapability(context.Background(), userID, CapFinanceWithdrawRead)
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("invalid capability string", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()

		_, err := checker.HasCapability(context.Background(), userID, Capability("invalid.capability"))
		assert.Error(t, err)
		assert.IsType(t, &entity.ErrInvalidCapability{}, err)
	})
}

// TestChecker_HasAnyCapability tests the HasAnyCapability method.
func TestChecker_HasAnyCapability(t *testing.T) {
	t.Run("user has one of the capabilities", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()
		cap := entity.NewCapabilityGrant(userID, CapFinanceWithdrawRead.String(), nil)
		repo.Create(context.Background(), nil, cap)

		has, err := checker.HasAnyCapability(context.Background(), userID, []Capability{
			CapFinanceWithdrawReview,
			CapFinanceWithdrawRead,
		})
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("user has none of the capabilities", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()

		has, err := checker.HasAnyCapability(context.Background(), userID, []Capability{
			CapFinanceWithdrawReview,
			CapFinanceDisputeResolve,
		})
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("empty capability list returns false", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()

		has, err := checker.HasAnyCapability(context.Background(), userID, []Capability{})
		require.NoError(t, err)
		assert.False(t, has)
	})
}

// TestChecker_HasAllCapabilities tests the HasAllCapabilities method.
func TestChecker_HasAllCapabilities(t *testing.T) {
	t.Run("user has all capabilities", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()
		cap1 := entity.NewCapabilityGrant(userID, CapFinanceWithdrawRead.String(), nil)
		cap2 := entity.NewCapabilityGrant(userID, CapFinanceWithdrawReview.String(), nil)
		repo.Create(context.Background(), nil, cap1)
		repo.Create(context.Background(), nil, cap2)

		has, err := checker.HasAllCapabilities(context.Background(), userID, []Capability{
			CapFinanceWithdrawRead,
			CapFinanceWithdrawReview,
		})
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("user has only some capabilities", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()
		cap := entity.NewCapabilityGrant(userID, CapFinanceWithdrawRead.String(), nil)
		repo.Create(context.Background(), nil, cap)

		has, err := checker.HasAllCapabilities(context.Background(), userID, []Capability{
			CapFinanceWithdrawRead,
			CapFinanceWithdrawReview,
		})
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("empty list returns true", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()

		has, err := checker.HasAllCapabilities(context.Background(), userID, []Capability{})
		require.NoError(t, err)
		assert.True(t, has)
	})
}

// TestChecker_ListActiveCapabilities tests the ListActiveCapabilities method.
func TestChecker_ListActiveCapabilities(t *testing.T) {
	t.Run("returns only active capabilities", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()
		cap1 := entity.NewCapabilityGrant(userID, CapFinanceWithdrawRead.String(), nil)
		cap2 := entity.NewCapabilityGrant(userID, CapFinanceWithdrawReview.String(), nil)
		repo.Create(context.Background(), nil, cap1)
		repo.Create(context.Background(), nil, cap2)

		// Revoke one capability
		now := time.Now()
		cap2.RevokedAt = &now
		repo.revokedCapabilities[userID] = append(repo.revokedCapabilities[userID], cap2.Capability)

		caps, err := checker.ListActiveCapabilities(context.Background(), userID)
		require.NoError(t, err)
		assert.Len(t, caps, 1)
		assert.Equal(t, CapFinanceWithdrawRead, caps[0])
	})

	t.Run("returns empty list for user with no capabilities", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()

		caps, err := checker.ListActiveCapabilities(context.Background(), userID)
		require.NoError(t, err)
		assert.Empty(t, caps)
	})
}

// TestChecker_CountActiveCapabilities tests the CountActiveCapabilities method.
func TestChecker_CountActiveCapabilities(t *testing.T) {
	t.Run("counts only active capabilities", func(t *testing.T) {
		repo := newMockCapabilityRepository()
		checker := NewChecker(repo)

		userID := uuid.New()
		cap1 := entity.NewCapabilityGrant(userID, CapFinanceWithdrawRead.String(), nil)
		cap2 := entity.NewCapabilityGrant(userID, CapFinanceWithdrawReview.String(), nil)
		cap3 := entity.NewCapabilityGrant(userID, CapFinanceDisputeResolve.String(), nil)
		repo.Create(context.Background(), nil, cap1)
		repo.Create(context.Background(), nil, cap2)
		repo.Create(context.Background(), nil, cap3)

		// Revoke one capability
		now := time.Now()
		cap2.RevokedAt = &now
		repo.revokedCapabilities[userID] = append(repo.revokedCapabilities[userID], cap2.Capability)

		count, err := checker.CountActiveCapabilities(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

// TestCapability_IsValid tests the IsValid function.
func TestCapability_IsValid(t *testing.T) {
	t.Run("valid capabilities return true", func(t *testing.T) {
		assert.True(t, IsValid(CapFinanceWithdrawRead.String()))
		assert.True(t, IsValid(CapFinanceWithdrawReview.String()))
		assert.True(t, IsValid(CapFinanceDisputeResolve.String()))
		assert.True(t, IsValid(CapGovernanceAuditRead.String()))
		assert.True(t, IsValid(CapPromotionExternalProductReview.String()))
	})

	t.Run("invalid capability returns false", func(t *testing.T) {
		assert.False(t, IsValid("invalid.capability"))
		assert.False(t, IsValid(""))
		assert.False(t, IsValid("finance.withdraw.invalid"))
	})
}

// TestCapability_AllCapabilities tests the AllCapabilities function.
func TestCapability_AllCapabilities(t *testing.T) {
	all := AllCapabilities()

	// Should contain all defined capabilities
	assert.Contains(t, all, CapFinanceWithdrawRead)
	assert.Contains(t, all, CapFinanceWithdrawReview)
	assert.Contains(t, all, CapFinanceDisputeResolve)
	assert.Contains(t, all, CapGovernanceAuditRead)
	assert.Contains(t, all, CapPromotionExternalProductReview)

	// All should be valid
	for _, cap := range all {
		assert.True(t, IsValid(cap.String()), "Capability %s should be valid", cap)
	}
}


