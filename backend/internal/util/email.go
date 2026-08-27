package util

import "strings"

// NormalizeEmail normalizes email by trimming whitespace and converting to lowercase
// Returns nil if input is nil or empty after trimming
func NormalizeEmail(email *string) *string {
	if email == nil {
		return nil
	}

	e := strings.TrimSpace(*email)
	if e == "" {
		return nil
	}

	e = strings.ToLower(e)
	return &e
}


