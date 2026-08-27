package publiccard

import (
	"os"
	"strings"

	"github.com/google/uuid"
)

// SellerCard is the canonical PublicCard exposure for a commerce seller.
// It wraps the public-safe UserCard identity and adds the two seller-specific
// public fields the commerce surfaces already hydrate today (store/farm name,
// avatar URL). The shape is the *foundation* for ForSaleCard.Seller and
// AuctionCard.Seller; downstream batches may add coarsened seller lifecycle
// (verified/active/suspended â†’ active/unavailable/removed) once verification
// authority is promoted past shadow mode.
//
// PUBLIC BOUNDARY GUARANTEES:
//   - email, phone, firebase_uid, account_status, KYC flags, verification
//     state, payout state, subscription enum, full_name, and any raw
//     moderation state are NEVER read or surfaced.
//   - Username falls back to publiccard.AnonymousUsername(id) via the
//     embedded UserCard when no user_profiles row is present.
//   - FarmName is the public seller-display string (seller_profiles.store_name);
//     it is NEVER coalesced with username.
//   - AvatarURL may mirror User.AvatarURL for clients that don't traverse
//     the nested user block.
//   - Lifecycle is reserved for a future coarsened public seller lifecycle
//     string and is always nil today (verification/subscription authority
//     are NOT public-card material yet).
type SellerCard struct {
	User      UserCard `json:"user"`
	FarmName  *string  `json:"farm_name"`
	AvatarURL *string  `json:"avatar_url"`
	Lifecycle *string  `json:"lifecycle"`

	// Tier â€” public seller reputation badge, omitted from the wire when nil.
	// Visible ONLY when all gates pass (see GatedSellerTier):
	//   1. ENABLE_PUBLIC_SELLER_TIER_PROFILE flag enabled
	//   2. user-identity lifecycle == "active"
	//   3. seller-trust lifecycle == "active" (active subscription)
	//   4. tier ∈ {"pro", "elite"} (Basic tier never shown publicly)
	// The AXIS BOUNDARY comment above is preserved; tier exposure is
	// additive and independent from both lifecycle axes.
	Tier *string `json:"tier,omitempty"`
}

// NewSellerCard builds a SellerCard from already-hydrated public-safe values.
// The fast path for call sites whose own batch query has already loaded
// username/avatar/store_name (e.g. sellerdisplay.FetchMany). Empty strings
// are treated as nil for the optional pointer fields.
func NewSellerCard(user UserCard, farmName string) SellerCard {
	card := SellerCard{
		User:      user,
		AvatarURL: user.AvatarURL,
	}
	if farmName != "" {
		card.FarmName = &farmName
	}
	return card
}

// NewSellerCardFromIdentity is a convenience builder when only the
// already-hydrated identity fields are known (id + username + avatar).
// The resulting SellerCard carries an empty FarmName when no store name is
// available.
func NewSellerCardFromIdentity(id uuid.UUID, username string, avatarURL *string, farmName string) SellerCard {
	return NewSellerCard(New(id, username, avatarURL), farmName)
}

// NewSellerCardWithUserLifecycle builds a SellerCard whose embedded UserCard
// carries the coarsened public user-identity lifecycle string. This is the
// canonical builder for E8.1 â€” seller-card emit sites that have hydrated raw
// user lifecycle truth (users.account_status + users.deleted_at) via
// sellerdisplay.FetchMany (or equivalent) and coarsened it via
// viewercontext.CoarsenLifecycle.
//
// The userLifecycle string MUST be one of the canonical coarsened values:
//
//   - "active"
//   - "unavailable"
//   - "removed"
//
// Empty string leaves the embedded User.Lifecycle nil (identical to
// NewSellerCard) â€” the safe default for surfaces that have not yet wired
// hydration.
//
// AXIS BOUNDARY (E8 doctrine):
//   - This helper populates SellerCard.User.Lifecycle (USER-IDENTITY axis).
//   - This helper does NOT populate the top-level SellerCard.Lifecycle.
//     The top-level field is reserved for a future SELLER-TRUST / CAPABILITY
//     coarsening (verification suspended/revoked, subscription expired) that
//     requires its own governance doctrine; until that batch lands the field
//     stays nil regardless of any user-identity state.
//
// PUBLIC BOUNDARY: This constructor never reads raw account_status enum
// values; it only accepts the already-coarsened public lifecycle string.
// Raw enum strings must NEVER reach this function.
func NewSellerCardWithUserLifecycle(
	id uuid.UUID,
	username string,
	avatarURL *string,
	farmName string,
	userLifecycle string,
) SellerCard {
	user := NewWithLifecycle(id, username, avatarURL, userLifecycle)
	return NewSellerCard(user, farmName)
}

// GatedSellerTier applies the canonical 4-gate policy before emitting a
// seller tier string on any public commerce surface (listing, auction, profile).
//
// Gates (ALL must pass):
//  1. ENABLE_PUBLIC_SELLER_TIER_PROFILE env flag is "1", "true", or "yes".
//  2. userLifecycle == "active" (user-identity axis: not banned/deleted).
//  3. sellerTrustLifecycle == "active" (seller-trust axis: active subscription).
//  4. tier ∈ {"pro", "elite"} (Basic tier is never publicly shown).
//
// Returns a pointer to the tier string when all gates pass, nil otherwise.
// Callers pass the already-coarsened lifecycle strings from
// viewercontext.CoarsenLifecycle / CoarsenSellerTrust, and the raw
// seller_profiles.tier value from sellerdisplay.Info.Tier.
func GatedSellerTier(tier, userLifecycle, sellerTrustLifecycle string) *string {
	raw := strings.TrimSpace(os.Getenv("ENABLE_PUBLIC_SELLER_TIER_PROFILE"))
	switch strings.ToLower(raw) {
	case "1", "true", "yes":
		// flag enabled â€” continue to remaining gates
	default:
		return nil
	}
	if userLifecycle != "active" {
		return nil
	}
	if sellerTrustLifecycle != "active" {
		return nil
	}
	if tier != "pro" && tier != "elite" {
		return nil
	}
	t := tier
	return &t
}

// NewSellerCardWithBothLifecycles builds a SellerCard populating BOTH the
// embedded UserCard lifecycle (user-identity axis) and the top-level
// SellerCard.Lifecycle (seller-trust axis). This is the canonical builder
// for listing/auction/search emit sites that have hydrated:
//
//   - raw user lifecycle truth (users.account_status, users.deleted_at) â†’
//     coarsened via viewercontext.CoarsenLifecycle â†’ userLifecycle param.
//   - raw seller-trust truth (latest seller_subscriptions.status) â†’
//     coarsened via viewercontext.CoarsenSellerTrust â†’ sellerTrustLifecycle.
//   - raw seller tier (seller_profiles.tier) â†’ tier param.
//
// Each lifecycle string MUST be one of "active", "unavailable", or "removed".
// Empty string leaves the respective field nil.
//
// The tier parameter is the raw seller_profiles.tier value ("basic", "pro",
// "elite", ""). Pass "" to suppress tier exposure on surfaces that do not yet
// participate in the public tier rollout (e.g. search). GatedSellerTier is
// called internally and applies the full 4-gate policy.
//
// AXIS BOUNDARY (independent axes, never collapsed):
//   - user-identity unavailable/removed = "block / redact" (E8 redaction).
//   - seller-trust unavailable          = "show + badge + disable CTAs".
//   - tier badge                        = suppressed when either axis is non-active.
//
// Mobile is expected to consume both axes independently and apply distinct
// UI policies per the owner doctrine for expired-seller visibility.
//
// PUBLIC BOUNDARY: never reads raw enum values; only accepts already-coarsened
// public lifecycle strings.
func NewSellerCardWithBothLifecycles(
	id uuid.UUID,
	username string,
	avatarURL *string,
	farmName string,
	userLifecycle string,
	sellerTrustLifecycle string,
	tier string,
) SellerCard {
	user := NewWithLifecycle(id, username, avatarURL, userLifecycle)
	card := NewSellerCard(user, farmName)
	if sellerTrustLifecycle != "" {
		v := sellerTrustLifecycle
		card.Lifecycle = &v
	}
	card.Tier = GatedSellerTier(tier, userLifecycle, sellerTrustLifecycle)
	return card
}


