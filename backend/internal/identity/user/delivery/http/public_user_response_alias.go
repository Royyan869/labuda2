package http

import "github.com/google/uuid"

// PublicUserResponse mirrors the canonical dto.PublicUserResponse shape.
// KYC verification flags are intentionally absent — see the doctrine note on
// dto.PublicUserResponse for the rationale.
type PublicUserResponse struct {
	UserID         uuid.UUID
	Username       string
	Bio            *string
	AvatarURL      *string
	Location       *string
	FollowersCount int
	FollowingCount int
	IsSeller       bool
	Roles          []string
	CreatedAt      string
}


