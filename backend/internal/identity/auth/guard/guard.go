package guard

import (
	"errors"
	"time"
)

// Authorization errors for user state validation
var (
	ErrUserDeleted  = errors.New("user deleted")
	ErrUserInactive = errors.New("account not active")
)

// UserLike defines the interface for user state validation
// Any user entity can implement this interface to use the guard
type UserLike interface {
	GetDeletedAt() *time.Time
	GetAccountStatus() string
}

// EnsureActiveUser validates that a user is in an active state
// This is a global invariant guard for all user operations
//
// Checks:
// 1. User is not deleted (deleted_at IS NULL)
// 2. User account status is active
//
// Usage:
//   if err := guard.EnsureActiveUser(user); err != nil {
//       return err
//   }
func EnsureActiveUser(u UserLike) error {
	if u.GetDeletedAt() != nil {
		return ErrUserDeleted
	}
	if u.GetAccountStatus() != "active" {
		return ErrUserInactive
	}
	return nil
}


