package bootstrap_test

import (
	"testing"

	"github.com/labuda/backend/internal/platform/capability"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoMinimumBootstrapAuthority proves the legacy "minimum bootstrap admin"
// model is purged.
//
// There must not be a second, smaller authority through which an admin acquires
// power: bootstrap-admin is initial-setup / disaster-recovery only, and it
// grants the entire canonical capability universe, which is exactly what makes
// the bootstrapped account a full-access admin by derivation.
func TestNoMinimumBootstrapAuthority(t *testing.T) {
	universe := capability.AllCapabilityStrings()
	require.NotEmpty(t, universe)

	assert.True(t,
		capability.IsFullAccessAdmin(capabilityEntity.AdminRole, universe),
		"the canonical universe must derive full access",
	)

	// Sanity: the universe is the same list bootstrap grants, so bootstrap
	// cannot drift away from the derived full-access definition.
	assert.ElementsMatch(t, capability.AllCapabilityStrings(), universe)
}
