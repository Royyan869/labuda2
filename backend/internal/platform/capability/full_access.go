package capability

import (
	"github.com/labuda/backend/internal/platform/capability/entity"
)

// ============================================================
// FULL ACCESS — DERIVED AUTHORITY STATE
// ============================================================
//
// Full access is NOT a role, NOT a database column, NOT a stored flag and NOT
// a capability. It is a *derived* property of two canonical facts:
//
//	users.role == "admin"
//	  AND
//	the admin's active capability set covers the entire canonical universe
//
// Consequences that follow from deriving it:
//   - A new capability added to the canonical universe immediately raises the
//     bar for full access; nobody is "grandfathered" into it.
//   - Duplicate capability rows cannot fake full access, because coverage is
//     computed over a *set* — never over a row count.
//   - A user with every capability but role != "admin" is not full access.
//
// This file is the ONE implementation of that set comparison.

// IsFullAccessAdmin reports whether the given role + active capabilities
// constitute canonical full access.
//
// The admin role value comes from the single canonical definition in the
// entity layer (entity.AdminRole); this package does not declare a second one.
func IsFullAccessAdmin(role string, activeCapabilities []string) bool {
	if role != entity.AdminRole {
		return false
	}
	return len(MissingCapabilities(activeCapabilities)) == 0
}

// MissingCapabilities returns the canonical capabilities an active set does not
// cover, in canonical universe order. Empty means full coverage.
//
// Duplicates in activeCapabilities are harmless: coverage is set-based.
func MissingCapabilities(activeCapabilities []string) []Capability {
	held := make(map[string]struct{}, len(activeCapabilities))
	for _, c := range activeCapabilities {
		held[c] = struct{}{}
	}

	var missing []Capability
	for _, c := range canonicalCapabilities {
		if _, ok := held[c.String()]; !ok {
			missing = append(missing, c)
		}
	}
	return missing
}

// MissingCapabilityStrings is MissingCapabilities as strings.
func MissingCapabilityStrings(activeCapabilities []string) []string {
	missing := MissingCapabilities(activeCapabilities)
	out := make([]string, 0, len(missing))
	for _, c := range missing {
		out = append(out, c.String())
	}
	return out
}
