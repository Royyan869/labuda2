// Package sellerdisplay provides batch lookup of public seller display
// fields (username, farm name, avatar URL) used by Phase 5 Stage 1
// seller/farm contract convergence on listing, auction, and order
// surfaces.
//
// Source-of-truth bindings (STRICT — NO COALESCE CORRUPTION):
//   - seller_username  ← user_profiles.username (NEVER store_name)
//   - seller_farm_name ← seller_profiles.store_name (NEVER username)
//   - seller_avatar_url ← user_profiles.avatar_url
//
// These fields are ADDITIVE alongside any pre-existing legacy display
// fields (e.g. seller_name) and MUST keep cleanly-separated semantics:
// seller_username NEVER contains farm name, seller_farm_name NEVER
// contains username.
package sellerdisplay

import (
	"context"

	"github.com/google/uuid"
	"github.com/labuda/backend/pkg/db"
)

// Info holds the public seller display fields for a single seller user.
//
// E8.1 — AccountStatus + IsDeleted carry raw user-identity lifecycle truth
// INSIDE the service boundary so viewercontext.CoarsenLifecycle can coarsen
// at the single canonical mapping site before the value reaches the wire.
// These fields are NEVER serialized directly; only the coarsened public
// lifecycle string ({active, unavailable, removed}) crosses the public-card
// boundary via publiccard.NewSellerCardWithUserLifecycle.
//
// Slot-persistence is preserved (no `users.deleted_at IS NULL` filter inside
// FetchMany) so a tombstoned seller surfaces Lifecycle="removed" rather than
// falling through to the anonymous-safe nil-lifecycle path. Same carve-out
// as chat (E4.2) and content-detail author (E6).
type Info struct {
	Username      string
	FarmName      string
	AvatarURL        string
	StoreImageURL    string
	PublicOriginLine string
	AccountStatus    string // raw users.account_status — service-internal only
	IsDeleted     bool   // (users.deleted_at IS NOT NULL) — service-internal only

	// Seller-trust axis (subscription) — populated from latest
	// seller_subscriptions row for this user. Empty string when the user
	// has no subscription record (treated as "unavailable" by the
	// seller-trust coarsener). Never serialized directly; callers MUST
	// coarsen to the 3-state public lifecycle via CoarsenSellerTrust before
	// emitting to the wire.
	SubscriptionStatus string

	// Tier — raw seller_profiles.tier value ("basic", "pro", "elite", "").
	// Empty string when the user has no seller_profiles row. Service-internal
	// only; callers MUST apply the ENABLE_PUBLIC_SELLER_TIER_PROFILE gate +
	// lifecycle gates before emitting to the wire via SellerCard.Tier.
	Tier string
}

// FetchMany batch-loads display info for the given seller user IDs.
//
// Returns a map keyed by seller user_id. IDs that have neither a
// user_profiles row nor a seller_profiles row are still represented
// in the map with empty-string fields, so callers can treat absence
// as missing-display and emit empty values.
//
// Single LEFT JOIN over user_profiles (canonical public identity) and
// seller_profiles (business display) keyed by user_id. No N+1.
func FetchMany(
	ctx context.Context,
	tx db.Tx,
	ids []uuid.UUID,
) (map[uuid.UUID]Info, error) {
	out := make(map[uuid.UUID]Info, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	// Deduplicate and seed map with zero values so callers always
	// find the requested key.
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

	// E8.1 — Additive projection of raw user-identity lifecycle truth
	// (u.account_status, u.deleted_at). Slot-persistence preserved (NO
	// `u.deleted_at IS NULL` filter) so tombstoned sellers surface as
	// Lifecycle="removed" downstream via viewercontext.CoarsenLifecycle.
	// Raw account_status never crosses the service boundary; callers
	// MUST coarsen before emitting to the wire.
	rows, err := tx.Query(ctx, `
		SELECT u.id,
		       COALESCE(up.username, '')   AS seller_username,
		       COALESCE(sp.store_name, '') AS seller_farm_name,
		       COALESCE(up.avatar_url, '') AS seller_avatar_url,
		       '' AS store_image_url,
		       '' AS public_origin_line,
		       u.account_status,
		       (u.deleted_at IS NOT NULL)  AS is_deleted,
		       COALESCE(ss.status::text, '') AS subscription_status,
		       COALESCE(sp.tier::text, '')  AS seller_tier
		FROM users u
		LEFT JOIN user_profiles   up ON up.user_id = u.id
		LEFT JOIN seller_profiles sp ON sp.user_id = u.id
		LEFT JOIN LATERAL (
		    SELECT status
		    FROM seller_subscriptions
		    WHERE user_id = u.id
		    ORDER BY created_at DESC
		    LIMIT 1
		) ss ON true
		WHERE u.id = ANY($1)
	`, deduped)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                 uuid.UUID
			username           string
			farmName           string
			avatarURL          string
			storeImageURL      string
			publicOriginLine   string
			accountStatus      string
			isDeleted          bool
			subscriptionStatus string
			tier               string
		)
		if err := rows.Scan(&id, &username, &farmName, &avatarURL, &storeImageURL, &publicOriginLine, &accountStatus, &isDeleted, &subscriptionStatus, &tier); err != nil {
			return out, err
		}
		out[id] = Info{
			Username:         username,
			FarmName:         farmName,
			StoreImageURL:    storeImageURL,
			PublicOriginLine: publicOriginLine,
			AvatarURL:          avatarURL,
			AccountStatus:      accountStatus,
			IsDeleted:          isDeleted,
			SubscriptionStatus: subscriptionStatus,
			Tier:               tier,
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}

// FetchOne is a convenience wrapper for a single seller user_id.
// Returns a zero-valued Info when no profile rows are present.
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


