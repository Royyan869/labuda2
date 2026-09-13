// Package auth tests for the canonical RoleCheckerDB role-write authority.
//
// SCOPE: these tests pin the CURRENT production contract of
// RoleCheckerDB.SetRole (internal/identity/auth/role_checker_db.go). Production
// signals rejection with fmt.Errorf message contracts, not sentinel error
// types, so the assertions below verify the exact messages the production
// method returns.
//
// SCOPE BOUNDARY: RoleCheckerDB.SetRole does NOT itself check the
// governance.role.assign capability. That capability gate lives at the
// route/handler boundary (RequireCapability + CoreUserHandler.SetRole). These
// tests therefore exercise only the two role-write guards that really live
// inside the production method — role-vocabulary validation and the
// self-escalation guard — both of which return before any database access.
package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time contract: the canonical role authority must satisfy the
// RoleChecker interface consumed by the middleware and server wiring.
var _ RoleChecker = (*RoleCheckerDB)(nil)

// TestRoleCheckerDB_SetRole_InvalidRoleRejected verifies that SetRole rejects
// every role outside the canonical vocabulary {"user", "admin"} with the
// production "invalid role: <role>" error.
//
// The zero-value checker is sufficient and deliberate: the invalid-role branch
// returns before rc.db is dereferenced, so this test exercises the real
// production guard without a database.
func TestRoleCheckerDB_SetRole_InvalidRoleRejected(t *testing.T) {
	checker := &RoleCheckerDB{}
	callerID := uuid.New()
	targetID := uuid.New()

	for _, invalidRole := range []string{
		"superadmin",  // legacy super-admin vocabulary
		"super_admin", // uppercase-separated variant
		"root_admin",  // legacy root-admin vocabulary
		"seller",      // seller authority is a profile, never a role
		"ADMIN",       // role vocabulary is exact and case-sensitive
		"",            // empty string is not a role
	} {
		name := invalidRole
		if name == "" {
			name = "empty"
		}
		t.Run("role_"+name, func(t *testing.T) {
			err := checker.SetRole(context.Background(), callerID, targetID, invalidRole)
			require.Error(t, err, "a role outside {user, admin} must be rejected")
			assert.Equal(t, "invalid role: "+invalidRole, err.Error())
		})
	}
}

// TestRoleCheckerDB_SetRole_SelfEscalationBlocked verifies the production
// service-level self-escalation guard: a caller cannot assign the canonical
// admin role to themselves, even if handler-level checks are bypassed.
//
// The guard returns before any database access, so the zero-value checker is
// used deliberately.
func TestRoleCheckerDB_SetRole_SelfEscalationBlocked(t *testing.T) {
	checker := &RoleCheckerDB{}
	selfID := uuid.New()

	err := checker.SetRole(context.Background(), selfID, selfID, capabilityEntity.AdminRole)
	require.Error(t, err, "assigning the admin role to self must be blocked")
	assert.Equal(t, "self-escalation blocked: cannot assign elevated role to self", err.Error())
}

// TestRoleCheckerDB_RoleVocabulary_MatchesCanonicalAdminRole pins the coupling
// between SetRole's hardcoded role vocabulary and the single canonical admin
// role value. If one side changes, this fails instead of silently allowing a
// mismatch between the write path and the authority constant.
func TestRoleCheckerDB_RoleVocabulary_MatchesCanonicalAdminRole(t *testing.T) {
	assert.Equal(t, "admin", capabilityEntity.AdminRole)
}
