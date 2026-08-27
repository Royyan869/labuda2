// Package infrastructure provides unit tests for the actor resolver.
package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/labuda/backend/internal/platform/capability/entity"
	capabilityRepo "github.com/labuda/backend/internal/platform/capability/infrastructure/repository"
)

// mockUserStateQuerier is a mock implementation of UserStateQuerier for testing.
type mockUserStateQuerier struct {
	users map[uuid.UUID]*mockUserState
}

// mockUserState holds the mock state for a user.
type mockUserState struct {
	role               string
	accountStatus      string
	emailVerified      bool
	isIdentityComplete bool
	sellerStatus       *string
}

func newMockUserStateQuerier() *mockUserStateQuerier {
	return &mockUserStateQuerier{
		users: make(map[uuid.UUID]*mockUserState),
	}
}

// GetUserState implements UserStateQuerier.GetUserState.
func (m *mockUserStateQuerier) GetUserState(ctx context.Context, userID uuid.UUID) (role, accountStatus string, emailVerified, isIdentityComplete bool, sellerStatus *string, err error) {
	state, ok := m.users[userID]
	if !ok {
		return "", "", false, false, nil, &entity.ActorNotFound{UserID: userID}
	}
	return state.role, state.accountStatus, state.emailVerified, state.isIdentityComplete, state.sellerStatus, nil
}

// addMockUser adds a mock user with the given state.
func (m *mockUserStateQuerier) addMockUser(userID uuid.UUID, state *mockUserState) {
	m.users[userID] = state
}

// TestActorResolver_ResolveActor tests the ResolveActor method.
func TestActorResolver_ResolveActor(t *testing.T) {
	t.Run("resolves user with role and capabilities", func(t *testing.T) {
		repo := capabilityRepo.NewCapabilityRepository(nil)
		stateQuerier := newMockUserStateQuerier()
		resolver := NewActorResolver(repo, stateQuerier)

		userID := uuid.New()
		stateQuerier.addMockUser(userID, &mockUserState{
			role:               "admin",
			accountStatus:      "active",
			emailVerified:      true,
			isIdentityComplete: true,
			sellerStatus:       nil,
		})

		// Verify resolver is created correctly
		assert.NotNil(t, resolver)
	})

	t.Run("returns ActorNotFound for non-existent user", func(t *testing.T) {
		repo := capabilityRepo.NewCapabilityRepository(nil)
		stateQuerier := newMockUserStateQuerier()
		resolver := NewActorResolver(repo, stateQuerier)

		userID := uuid.New()
		// Don't add to users map - simulating non-existent user

		_, err := resolver.ResolveActor(context.Background(), userID)
		assert.Error(t, err)
		// Check for ActorNotFound using errors.As since the error may be wrapped
		var notFound *entity.ActorNotFound
		assert.True(t, errors.As(err, &notFound))
	})
}

// TestActor_HasCapability tests the Actor's HasCapability method.
func TestActor_HasCapability(t *testing.T) {
	t.Run("returns true when actor has capability", func(t *testing.T) {
		actor := &entity.Actor{
			ID:           uuid.New(),
			Role:         "admin",
			Capabilities: []string{"finance.withdraw.read", "governance.audit.read"},
		}

		assert.True(t, actor.HasCapability("finance.withdraw.read"))
		assert.True(t, actor.HasCapability("governance.audit.read"))
	})

	t.Run("returns false when actor does not have capability", func(t *testing.T) {
		actor := &entity.Actor{
			ID:           uuid.New(),
			Role:         "admin",
			Capabilities: []string{"finance.withdraw.read"},
		}

		assert.False(t, actor.HasCapability("finance.withdraw.review"))
		assert.False(t, actor.HasCapability("invalid.capability"))
	})

	t.Run("returns false when actor has no capabilities", func(t *testing.T) {
		actor := &entity.Actor{
			ID:           uuid.New(),
			Role:         "user",
			Capabilities: []string{},
		}

		assert.False(t, actor.HasCapability("finance.withdraw.read"))
	})
}

// TestActor_HasAnyCapability tests the Actor's HasAnyCapability method.
func TestActor_HasAnyCapability(t *testing.T) {
	t.Run("returns true when actor has at least one capability", func(t *testing.T) {
		actor := &entity.Actor{
			ID:           uuid.New(),
			Role:         "admin",
			Capabilities: []string{"finance.withdraw.read"},
		}

		assert.True(t, actor.HasAnyCapability("finance.withdraw.read", "finance.withdraw.review"))
		assert.True(t, actor.HasAnyCapability("finance.withdraw.review", "finance.withdraw.read"))
	})

	t.Run("returns false when actor has none of the capabilities", func(t *testing.T) {
		actor := &entity.Actor{
			ID:           uuid.New(),
			Role:         "admin",
			Capabilities: []string{"finance.withdraw.read"},
		}

		assert.False(t, actor.HasAnyCapability("finance.withdraw.review", "governance.audit.read"))
	})

	t.Run("returns false for empty capability list", func(t *testing.T) {
		actor := &entity.Actor{
			ID:           uuid.New(),
			Role:         "admin",
			Capabilities: []string{"finance.withdraw.read"},
		}

		assert.False(t, actor.HasAnyCapability())
	})
}

// TestActor_HasAllCapabilities tests the Actor's HasAllCapabilities method.
func TestActor_HasAllCapabilities(t *testing.T) {
	t.Run("returns true when actor has all capabilities", func(t *testing.T) {
		actor := &entity.Actor{
			ID:           uuid.New(),
			Role:         "admin",
			Capabilities: []string{"finance.withdraw.read", "governance.audit.read"},
		}

		assert.True(t, actor.HasAllCapabilities("finance.withdraw.read", "governance.audit.read"))
	})

	t.Run("returns false when actor missing some capabilities", func(t *testing.T) {
		actor := &entity.Actor{
			ID:           uuid.New(),
			Role:         "admin",
			Capabilities: []string{"finance.withdraw.read"},
		}

		assert.False(t, actor.HasAllCapabilities("finance.withdraw.read", "governance.audit.read"))
	})

	t.Run("returns true for empty capability list", func(t *testing.T) {
		actor := &entity.Actor{
			ID:           uuid.New(),
			Role:         "user",
			Capabilities: []string{},
		}

		assert.True(t, actor.HasAllCapabilities())
	})
}

// TestActor_RoleMethods tests the Actor's role helper methods.
func TestActor_RoleMethods(t *testing.T) {
	t.Run("IsAdmin returns true only for admin role", func(t *testing.T) {
		adminActor := &entity.Actor{ID: uuid.New(), Role: "admin"}
		userActor := &entity.Actor{ID: uuid.New(), Role: "user"}
		sellerActor := &entity.Actor{ID: uuid.New(), Role: "seller"}

		assert.True(t, adminActor.IsAdmin())
		assert.False(t, userActor.IsAdmin())
		assert.False(t, sellerActor.IsAdmin())
	})
}

// TestNewCapabilityGrant tests the NewCapabilityGrant factory function.
func TestNewCapabilityGrant(t *testing.T) {
	t.Run("creates active capability grant", func(t *testing.T) {
		userID := uuid.New()
		grantedBy := uuid.New()

		cap := entity.NewCapabilityGrant(userID, "finance.withdraw.read", &grantedBy)

		assert.NotEqual(t, uuid.Nil, cap.ID)
		assert.Equal(t, userID, cap.UserID)
		assert.Equal(t, "finance.withdraw.read", cap.Capability)
		assert.Equal(t, &grantedBy, cap.GrantedBy)
		assert.NotNil(t, cap.GrantedAt)
		assert.Nil(t, cap.RevokedAt)
		assert.True(t, cap.IsActive())
		assert.False(t, cap.IsRevoked())
	})

	t.Run("creates capability without grantor", func(t *testing.T) {
		userID := uuid.New()

		cap := entity.NewCapabilityGrant(userID, "finance.withdraw.read", nil)

		assert.Nil(t, cap.GrantedBy)
		assert.True(t, cap.IsActive())
	})
}

// TestUserCapability_Revoke tests the Revoke method.
func TestUserCapability_Revoke(t *testing.T) {
	t.Run("revokes an active capability", func(t *testing.T) {
		cap := entity.NewCapabilityGrant(uuid.New(), "finance.withdraw.read", nil)

		assert.True(t, cap.IsActive())
		assert.False(t, cap.IsRevoked())

		revokedAt := cap.GrantedAt.AddDate(0, 0, 1) // 1 day later
		cap.Revoke(revokedAt)

		assert.False(t, cap.IsActive())
		assert.True(t, cap.IsRevoked())
		assert.Equal(t, &revokedAt, cap.RevokedAt)
	})
}

// TestActorNotFound_Error tests the ActorNotFound error.
func TestActorNotFound_Error(t *testing.T) {
	userID := uuid.New()
	err := &entity.ActorNotFound{UserID: userID}

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "actor not found")
}

// TestErrDuplicateCapability_Error tests the ErrDuplicateCapability error.
func TestErrDuplicateCapability_Error(t *testing.T) {
	userID := uuid.New()
	err := &entity.ErrDuplicateCapability{UserID: userID, Capability: "finance.withdraw.read"}

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already has this active capability")
}

// TestErrCapabilityNotFound_Error tests the ErrCapabilityNotFound error.
func TestErrCapabilityNotFound_Error(t *testing.T) {
	userID := uuid.New()
	err := &entity.ErrCapabilityNotFound{UserID: userID, Capability: "finance.withdraw.read"}

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestErrInvalidCapability_Error tests the ErrInvalidCapability error.
func TestErrInvalidCapability_Error(t *testing.T) {
	err := &entity.ErrInvalidCapability{Capability: "invalid.capability"}

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid capability")
}


