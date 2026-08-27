package response

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/labuda/backend/internal/identity/auth"
)

// TestMapErrorToResponse_AccountStatusErrors is the PASS_17B regression suite:
// every canonical account-state rejection must map to a stable 401/403
// status/code, never the generic 500 default. Service layers wrap these
// sentinels with %w (e.g. "buyer account not active: %w"), so wrapped forms
// are asserted too.
func TestMapErrorToResponse_AccountStatusErrors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "suspended",
			err:        auth.ErrAccountSuspended,
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_SUSPENDED",
		},
		{
			name:       "banned",
			err:        auth.ErrAccountBanned,
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_BANNED",
		},
		{
			name:       "inactive",
			err:        auth.ErrAccountInactive,
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_INACTIVE",
		},
		{
			name:       "removed",
			err:        auth.ErrAccountRemoved,
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_REMOVED",
		},
		{
			name:       "invalid caller",
			err:        auth.ErrInvalidCaller,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_CALLER",
		},
		{
			name:       "removed wrapped by service layer",
			err:        fmt.Errorf("buyer account not active: %w", auth.ErrAccountRemoved),
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_REMOVED",
		},
		{
			name:       "suspended wrapped by service layer",
			err:        fmt.Errorf("seller account not active: %w", auth.ErrAccountSuspended),
			wantStatus: http.StatusForbidden,
			wantCode:   "ACCOUNT_SUSPENDED",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapping := MapErrorToResponse(tc.err)
			if mapping.StatusCode != tc.wantStatus {
				t.Errorf("StatusCode = %d, want %d", mapping.StatusCode, tc.wantStatus)
			}
			if mapping.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", mapping.Code, tc.wantCode)
			}
			if mapping.StatusCode == http.StatusInternalServerError {
				t.Errorf("%s: fell through to generic 500 — PASS_17B regression", tc.name)
			}
		})
	}
}

// TestMapErrorToResponse_UnknownErrorRemains500 proves genuine internal
// failures are not reclassified by the account-status hardening.
func TestMapErrorToResponse_UnknownErrorRemains500(t *testing.T) {
	mapping := MapErrorToResponse(errors.New("connection refused"))
	if mapping.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", mapping.StatusCode)
	}
	if mapping.Code != "INTERNAL_ERROR" {
		t.Errorf("Code = %q, want INTERNAL_ERROR", mapping.Code)
	}
}

// TestIsAuthError_CoversAllAccountStates proves IsAuthError recognizes every
// canonical account-state sentinel, including removed (PASS_17B), and does not
// misclassify unrelated errors.
func TestIsAuthError_CoversAllAccountStates(t *testing.T) {
	for _, err := range []error{
		auth.ErrAccountSuspended,
		auth.ErrAccountBanned,
		auth.ErrAccountInactive,
		auth.ErrAccountRemoved,
		auth.ErrInvalidCaller,
	} {
		if !IsAuthError(err) {
			t.Errorf("IsAuthError(%v) = false, want true", err)
		}
		wrapped := fmt.Errorf("guard failed: %w", err)
		if !IsAuthError(wrapped) {
			t.Errorf("IsAuthError(wrapped %v) = false, want true", err)
		}
	}

	if IsAuthError(errors.New("connection refused")) {
		t.Error("IsAuthError(unrelated error) = true, want false")
	}
}
