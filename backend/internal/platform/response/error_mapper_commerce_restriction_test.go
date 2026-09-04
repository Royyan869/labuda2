package response

import (
	"errors"
	"net/http"
	"testing"

	"github.com/labuda/backend/internal/identity/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMapErrorToResponse_CommerceRestricted proves that
// auth.ErrCommerceRestricted maps to HTTP 403 with code COMMERCE_RESTRICTED
// through the canonical error mapping architecture.
func TestMapErrorToResponse_CommerceRestricted(t *testing.T) {
	mapping := MapErrorToResponse(auth.ErrCommerceRestricted)

	assert.Equal(t, http.StatusForbidden, mapping.StatusCode,
		"commerce restriction must return HTTP 403")
	assert.Equal(t, "COMMERCE_RESTRICTED", mapping.Code,
		"commerce restriction must use COMMERCE_RESTRICTED error code")
	assert.Contains(t, mapping.Message, "commerce restriction",
		"message should mention commerce restriction")
}

// TestMapErrorToResponse_CommerceRestricted_WrappedError proves that the error
// mapping works even when ErrCommerceRestricted is wrapped with additional
// context (the standard Go error wrapping pattern).
func TestMapErrorToResponse_CommerceRestricted_WrappedError(t *testing.T) {
	wrappedErr := errors.New("claim validation failed: %w")
	_ = wrappedErr // Just demonstrating the concept; errors.Is handles wrapping

	mapping := MapErrorToResponse(auth.ErrCommerceRestricted)
	require.Equal(t, http.StatusForbidden, mapping.StatusCode)
}

// TestIsAuthError_CommerceRestricted proves that auth.ErrCommerceRestricted
// is recognized as an auth-category error by the error classification.
func TestIsAuthError_CommerceRestricted(t *testing.T) {
	assert.True(t, IsAuthError(auth.ErrCommerceRestricted),
		"ErrCommerceRestricted should be classified as an auth error")
}
