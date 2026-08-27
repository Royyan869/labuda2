package username

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	MinLength = 3
	MaxLength = 30
)

var (
	pattern = regexp.MustCompile(`^[a-z0-9_]+$`)
	reserved = map[string]struct{}{
		"admin":     {},
		"api":       {},
		"root":      {},
		"system":    {},
		"www":       {},
		"mail":      {},
		"ftp":       {},
		"localhost": {},
		"test":      {},
		"demo":      {},
		"support":   {},
		"help":      {},
		"info":      {},
		"billing":   {},
		"sales":     {},
		"marketing": {},
		"labuda":    {},
		"shona":     {},
	}
)

func Normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func IsReserved(value string) bool {
	_, exists := reserved[Normalize(value)]
	return exists
}

func ValidateFormat(value string) error {
	username := Normalize(value)
	if len(username) < MinLength || len(username) > MaxLength {
		return fmt.Errorf("username must be between %d and %d characters", MinLength, MaxLength)
	}
	if !pattern.MatchString(username) {
		return fmt.Errorf("username must contain only lowercase letters, numbers, and underscores")
	}
	return nil
}


