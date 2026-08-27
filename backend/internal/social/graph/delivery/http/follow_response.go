package http

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
)

// FollowUserCardResponse is the follow-owned HTTP response DTO for
// followers/following lists. It replaces publiccard.UserCard on the follow
// response path so that the follow domain controls its own identity contract.
//
// C2E2A contract — every emitted card MUST contain:
//
//	id                — stable user ID
//	username          — raw username (empty when degraded or absent)
//	avatar_url        — optional avatar URL (nil when degraded or absent)
//	lifecycle         — "active" | "unavailable" | "removed" (never nil)
//	followers_count   — live count
//	following_count   — live count
//
// Wire keys match the publiccard.UserCard shape so the mobile contract is
// unchanged.
type FollowUserCardResponse struct {
	ID             uuid.UUID `json:"id"`
	Username       string    `json:"username"`
	AvatarURL      *string   `json:"avatar_url"`
	Lifecycle      string    `json:"lifecycle"`
	FollowersCount int       `json:"followers_count"`
	FollowingCount int       `json:"following_count"`
}

// hydrateFollowUserCards resolves a slice of user IDs into sanitized,
// lifecycle-aware FollowUserCardResponse values via a single bulk SQL query.
//
// Identity contract (C2E2A):
//   - active + profile present   → username, avatar, lifecycle "active"
//   - active + no profile        → username "", avatar nil, lifecycle "active"
//   - unavailable (suspended/banned) → username "", avatar nil, lifecycle "unavailable"
//   - removed (soft-deleted)     → username "", avatar nil, lifecycle "removed"
//   - hard-missing user (no row) → silently dropped from output
//
// Never synthesizes identity from ID. Never calls publiccard helpers.
//
// The output slice preserves input ID order. Duplicate input IDs produce one
// output entry per occurrence that resolved to a DB row.
//
// Database errors are returned to the caller — the handler must emit a
// canonical server error, not HTTP 200 with an empty list.
func (h *FollowHandler) hydrateFollowUserCards(ctx context.Context, ids []uuid.UUID) ([]FollowUserCardResponse, error) {
	if len(ids) == 0 {
		return []FollowUserCardResponse{}, nil
	}

	rows, err := h.db.Pool().Query(ctx, `
		SELECT
			u.id,
			COALESCE(p.username, '') AS username,
			p.avatar_url,
			u.account_status,
			(u.deleted_at IS NOT NULL) AS is_deleted,
			COALESCE((
				SELECT COUNT(*)
				FROM user_follows uf
				WHERE uf.following_id = u.id
			), 0) AS followers_count,
			COALESCE((
				SELECT COUNT(*)
				FROM user_follows uf
				WHERE uf.follower_id = u.id
			), 0) AS following_count
		FROM users u
		LEFT JOIN user_profiles p ON u.id = p.user_id
		WHERE u.id = ANY($1)
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("hydrate follow user cards: query: %w", err)
	}
	defer rows.Close()

	cardsByID := make(map[uuid.UUID]FollowUserCardResponse, len(ids))
	for rows.Next() {
		var (
			userID         uuid.UUID
			username       string
			avatarURL      *string
			accountStatus  string
			isDeleted      bool
			followersCount int64
			followingCount int64
		)
		if err := rows.Scan(&userID, &username, &avatarURL, &accountStatus, &isDeleted, &followersCount, &followingCount); err != nil {
			return nil, fmt.Errorf("hydrate follow user cards: scan: %w", err)
		}

		lifecycle := string(viewercontext.CoarsenLifecycle(accountStatus, isDeleted))
		card := sanitizeFollowCard(userID, username, avatarURL, lifecycle, int(followersCount), int(followingCount))
		cardsByID[userID] = card
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("hydrate follow user cards: rows: %w", rows.Err())
	}

	// Reconstruct in original input order. Missing users (no DB row) are
	// dropped. Each input position that resolved to a DB row produces one
	// output entry.
	result := make([]FollowUserCardResponse, 0, len(cardsByID))
	for _, id := range ids {
		if card, ok := cardsByID[id]; ok {
			result = append(result, card)
		}
	}
	return result, nil
}

// sanitizeFollowCard applies the C2E2A identity contract to a single hydrated
// row. Username and avatar are cleared for degraded lifecycles so the HTTP
// response never leaks live identity of unavailable or removed users.
func sanitizeFollowCard(
	id uuid.UUID,
	rawUsername string,
	rawAvatarURL *string,
	lifecycle string,
	followersCount int,
	followingCount int,
) FollowUserCardResponse {
	card := FollowUserCardResponse{
		ID:             id,
		Lifecycle:      lifecycle,
		FollowersCount: followersCount,
		FollowingCount: followingCount,
	}

	switch lifecycle {
	case string(viewercontext.PublicLifecycleStateActive):
		// Active users keep their profile identity (or empty defaults when
		// the profile is absent).
		if rawUsername != "" {
			card.Username = rawUsername
		}
		if rawAvatarURL != nil && *rawAvatarURL != "" {
			card.AvatarURL = rawAvatarURL
		}

	default:
		// Degraded lifecycles (unavailable, removed): redact username and
		// avatar. Never synthesize a replacement identity.
		card.Username = ""
		card.AvatarURL = nil
	}

	return card
}
