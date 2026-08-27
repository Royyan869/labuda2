// Package publiccard hosts the canonical PublicCard exposure types per
// docs/contracts/public-card-boundary.md.
//
// PHASE 2A SCOPE — three card families:
//   - CommentAuthorCard
//   - ChatParticipantCard
//   - NotificationActorCard
//
// All three share a single minimal shape today (UserCard) and use the same
// public-safe hydration path. Per-family Build functions are exposed so call
// sites are doctrinally attributed even though the type shape is currently
// shared; a future divergence (e.g. ChatParticipantCard gaining a
// relationship overlay) can change the family type without touching call
// site call shapes.
//
// PUBLIC BOUNDARY GUARANTEES:
//   - email, phone, firebase_uid, account_status, KYC flags, full_name are
//     never read by this package.
//   - Username falls back to a deterministic anonymous-safe form
//     ("user_<first-8-hex>") when no user_profiles row is present. The
//     fallback never reads or surfaces any auth identifier.
//   - Lifecycle is reserved for a future coarsened public lifecycle string
//     and is always nil today.
package publiccard

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/userdisplay"
	"github.com/labuda/backend/pkg/db"
)

// UserCard is the minimal canonical public-safe user identity shape that
// underlies CommentAuthorCard / ChatParticipantCard / NotificationActorCard
// in Phase 2A. JSON keys match the existing authorref.AuthorRef shape so
// this type is a drop-in replacement on surfaces that already emit a typed
// author block.
type UserCard struct {
	ID             uuid.UUID `json:"id"`
	Username       string    `json:"username"`
	AvatarURL      *string   `json:"avatar_url"`
	Lifecycle      *string   `json:"lifecycle"`
	FollowersCount int       `json:"followers_count,omitempty"`
	FollowingCount int       `json:"following_count,omitempty"`
}

// New builds a UserCard from already-hydrated identity values. This is the
// fast path for call sites whose own batch query has already loaded
// username/avatar (e.g. the comment SQL projection); they should NOT do a
// second DB hit through BuildMany.
//
// `avatarURL` may be nil; an empty string is treated the same as nil.
//
// LIFECYCLE: This constructor leaves Lifecycle nil. Call sites that have
// hydrated the coarsened public user lifecycle from canonical truth
// (users.account_status, users.deleted_at) MUST use NewWithLifecycle
// instead — never set Lifecycle on the returned struct after the fact.
func New(id uuid.UUID, username string, avatarURL *string) UserCard {
	card := UserCard{ID: id}
	if username != "" {
		card.Username = username
	} else {
		card.Username = AnonymousUsername(id)
	}
	if avatarURL != nil && *avatarURL != "" {
		card.AvatarURL = avatarURL
	}
	return card
}

// NewWithLifecycle is the additive lifecycle-carrying constructor. It is
// the canonical builder for call sites that have already projected the raw
// lifecycle truth (users.account_status + users.deleted_at) and coarsened
// it via viewercontext.CoarsenLifecycle. The lifecycle string MUST be one
// of the canonical coarsened values:
//
//   - "active"
//   - "unavailable"
//   - "removed"
//
// Empty string leaves the Lifecycle field nil (identical to New) — this is
// the safe default for surfaces that have not yet been wired through the
// hydration seam.
//
// PUBLIC BOUNDARY: This constructor never reads raw account_status enum
// values; it only accepts the already-coarsened public lifecycle string.
// Raw enum strings must NEVER reach this function.
func NewWithLifecycle(id uuid.UUID, username string, avatarURL *string, lifecycle string) UserCard {
	card := New(id, username, avatarURL)
	if lifecycle != "" {
		v := lifecycle
		card.Lifecycle = &v
	}
	return card
}

// BuildMany batch-hydrates UserCards for the given user IDs via
// userdisplay.FetchMany. The returned map has an entry for every non-nil ID
// in the input; missing user_profiles rows yield an anonymous-safe card.
//
// PUBLIC BOUNDARY: userdisplay.FetchMany selects only username and avatar_url
// from user_profiles + users.deleted_at filter; no email / phone /
// firebase_uid / account_status column is ever read.
func BuildMany(ctx context.Context, tx db.Tx, ids []uuid.UUID) (map[uuid.UUID]UserCard, error) {
	out := make(map[uuid.UUID]UserCard, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	infos, err := userdisplay.FetchMany(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		out[id] = cardFromInfo(id, infos[id])
	}
	return out, nil
}

// BuildOne is the single-ID convenience wrapper.
func BuildOne(ctx context.Context, tx db.Tx, id uuid.UUID) (UserCard, error) {
	if id == uuid.Nil {
		return UserCard{}, nil
	}
	info, err := userdisplay.FetchOne(ctx, tx, id)
	if err != nil {
		return UserCard{}, err
	}
	return cardFromInfo(id, info), nil
}

// Anonymous returns a fully synthetic public-safe card for the given id.
// Used when the caller cannot or does not want to hit the DB.
func Anonymous(id uuid.UUID) UserCard {
	return UserCard{
		ID:       id,
		Username: AnonymousUsername(id),
	}
}

// AnonymousUsername returns the deterministic anonymous-safe username for
// the given UUID. Exposed so callers can compute the fallback without
// allocating a full card (e.g. for log lines).
func AnonymousUsername(id uuid.UUID) string {
	s := id.String()
	if len(s) >= 8 {
		return fmt.Sprintf("user_%s", s[:8])
	}
	return fmt.Sprintf("user_%s", s)
}

func cardFromInfo(id uuid.UUID, info userdisplay.Info) UserCard {
	var avatar *string
	if info.AvatarURL != "" {
		v := info.AvatarURL
		avatar = &v
	}
	return New(id, info.Username, avatar)
}


