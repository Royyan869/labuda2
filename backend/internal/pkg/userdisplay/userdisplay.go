// Package userdisplay provides batch lookup of public user display
// fields (username, avatar URL) used by Phase 5 Stage 1 contract
// convergence on auction bid history and order endpoints.
//
// Source-of-truth bindings:
//   - username   ← user_profiles.username
//   - avatar_url ← user_profiles.avatar_url
//
// users.email is NEVER projected as a public identity per
// viewer-context-contract.md §4.1.
package userdisplay

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// Info holds the public user display fields for a single user.
type Info struct {
	Username  string
	AvatarURL string
}

// FetchMany batch-loads display info for the given user IDs.
//
// Returns a map keyed by user_id. IDs without a user_profiles row are
// still represented in the map with empty-string fields, so callers can
// treat absence as missing-display and emit empty values.
func FetchMany(
	ctx context.Context,
	tx db.Tx,
	ids []uuid.UUID,
) (map[uuid.UUID]Info, error) {
	out := make(map[uuid.UUID]Info, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	deduped := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		deduped = append(deduped, id)
		out[id] = Info{}
	}
	if len(deduped) == 0 {
		return out, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT u.id,
		       COALESCE(up.username, '')   AS username,
		       COALESCE(up.avatar_url, '') AS avatar_url
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		WHERE u.id = ANY($1)
	`, deduped)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id        uuid.UUID
			username  string
			avatarURL string
		)
		if err := rows.Scan(&id, &username, &avatarURL); err != nil {
			return out, err
		}
		out[id] = Info{
			Username:  username,
			AvatarURL: avatarURL,
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}

// FetchOne is a convenience wrapper for a single user_id.
// Returns a zero-valued Info when no profile row is present.
func FetchOne(
	ctx context.Context,
	tx db.Tx,
	id uuid.UUID,
) (Info, error) {
	m, err := FetchMany(ctx, tx, []uuid.UUID{id})
	if err != nil {
		return Info{}, err
	}
	if v, ok := m[id]; ok {
		return v, nil
	}
	return Info{}, nil
}


