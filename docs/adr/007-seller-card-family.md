# ADR-007 — SellerCard Family

## Status

Accepted

## Related Documents

- [`docs/foundation.md`](../foundation.md) — Canonical Authorities (Identity, Public Exposure)
- [`docs/architecture.md`](../architecture.md) — Identity / Trust Model, Discovery / Projection Design, Commerce Authority
- [`docs/contracts/public-card-boundary.md`](../contracts/public-card-boundary.md) — boundary contract
- ADR-001 Pricing Token Authority; ADR-002 Ledger as Authority; ADR-003 Governance Evaluator; ADR-004 Discovery / Projection Boundary
- Companion ADRs: 006 UserCard, 010 ContentCard

---

## 1. Decision

SellerCard is the canonical exposure shape for a user **acting in a seller capability**. Distinct from UserCard because it carries seller-capability-relevant fields (storefront binding, seller lifecycle, seller-capability verification indicator) and applies seller-specific redaction.

SellerCard **composes** over UserCard rather than inheriting. The two are distinct families; consumers explicitly embed one or the other.

Seller is a **capability of a user**, not a separate identity primitive. There is no `sellers` table that is the authority for seller identity independent of `users`.

## 2. Ownership

**Identity domain** owns the user-identity primitive that SellerCard composes over (canonical username, anonymous-safe fallback, lifecycle, deletion semantics).

**Commerce capability boundary** owns the seller-capability-specific extensions (storefront name / avatar / banner, seller verification indicator, seller-capability lifecycle, subscription-tier coarse exposure rule).

Boundary changes between the two ownerships require both domains' agreement.

## 3. Composition over Inheritance

- **Composition** — SellerCard is its own family with its own slot model. Slots that overlap with UserCard (canonical handle, anonymous-safe shape rules, lifecycle states) are defined consistently because both reference the same underlying user identity, but SellerCard's slots are not "UserCard slots plus extras."
- **Inheritance** — forbidden. SellerCard is not "UserCard with seller fields tacked on." Consumer code MUST NOT treat SellerCard as a subtype of UserCard or substitute one for the other.

Reasons composition is mandatory:

- Seller-capability lifecycle is not the same as user-lifecycle (a user may be active while seller capability is suspended).
- Seller verification indicator may differ from user verification indicator.
- Inheritance creates hidden type-coupling that prevents per-family amendments without breaking consumers.

## 4. Canonical Card Shape

```
SellerCard {
  id              : opaque seller reference                 (every semantic)
  card_state      : enum {full, suspended_seller, suspended_user,
                          removed_user, anonymous_fallback, redacted}

  -- Public Identity Reference
  username        : canonical public handle                (full / redacted)
                    -- never email, phone, or firebase_uid

  -- Public Display Attributes — Personal
  display_name    : public personal display name           (full only)

  -- Public Display Attributes — Storefront
  storefront_name   : public storefront / shop name        (full only when
                                                            seller-capability active)
  storefront_avatar : storefront avatar reference          (full only)
  storefront_banner : storefront banner reference          (full only; optional)

  -- Public Lifecycle State (two independent axes)
  user_lifecycle    : enum {active, unavailable, removed} | null
  seller_lifecycle  : enum {active, unavailable, removed} | null

  -- Public Capability State
  capability_tier : enum {standard, verified, premium, ...} | null
                    -- coarse only; never raw subscription state, never billing,
                       never internal entitlement bitmaps

  -- Public Verification Indicator (two independent axes)
  is_user_verified   : bool | null
  is_seller_verified : bool | null

  -- Public Relationship Indicator
  viewer_relation : enum {none, you_follow_seller,
                          seller_blocks_you, you_blocked_seller} | null
}
```

`user_lifecycle` and `seller_lifecycle` are **independent**. A user with `account_status='active'` but seller capability suspended emits `card_state = suspended_seller`, `user_lifecycle = active`, `seller_lifecycle = unavailable`. The two verification indicators are likewise independent.

## 5. Allowed Field Categories

Per the public-card boundary contract:

| Category | Slot | Rule |
|---|---|---|
| Public Identity Reference | `username` | canonical public handle ONLY |
| Public Display Attributes — Personal | `display_name` | as in UserCard |
| Public Display Attributes — Storefront | `storefront_name`, `storefront_avatar`, `storefront_banner` | seller-capability-specific |
| Public Lifecycle State | `user_lifecycle`, `seller_lifecycle` | two independent coarse indicators |
| Public Capability State | `capability_tier` | coarse only |
| Public Verification Indicator | `is_user_verified`, `is_seller_verified` | two independent coarse booleans |
| Public Relationship Indicator | `viewer_relation` | viewer-relative coarse only |
| Public Audit Reference | `id` | opaque seller reference |

SellerCard does NOT carry Public Commerce Attributes — those belong to ListingCard / AuctionCard families that embed SellerCard.

## 6. Forbidden Field Categories

Inherits the boundary contract's forbidden categories, with seller-capability-specific reinforcements:

- **Auth Identity** — email, phone, firebase_uid, auth provider identifiers, OTP material, session tokens. Never any slot under any semantic. Never as a fallback for missing canonical username, missing storefront_name, or missing display_name. A `COALESCE(storefront_name, email)`-style fallback chain is forbidden.
- **Financial Authority Fields** — ledger balances, gateway payloads, payout state, seller payable, escrow state, settlement state, withdrawal state. SellerCard does not carry financial state; ledger is the authority (ADR-002).
- **Pricing Authority Fields** — raw pricing tokens, pricing snapshot internals.
- **Subscription Internals** — raw subscription state, billing state, provider payloads, cycle dates, invoice references, raw provider tier names. `capability_tier` carries only the coarse boundary-permitted enum.
- **Internal Moderation Metadata**
- **Inventory Internals** — SellerCard does not aggregate inventory; that is ListingCard's concern.
- **Capability Internals** — raw entitlement bitmaps, role-binding internals.
- **Relationship Graph Internals** — counterparty's seller follower lists.
- **Verification Internals** — uploaded documents, provider payloads, raw verifier identity.
- **Realtime Transport Internals**.

The **Auth Identity** and **Financial Authority** prohibitions are the load-bearing invariants. The first prevents email leakage through the storefront-name fallback chain; the second prevents financial / commerce internals from crossing the public boundary.

## 7. Lifecycle Rendering

Two independent axes produce a 2D state space:

| User lifecycle \ Seller lifecycle | active | unavailable | removed |
|---|---|---|---|
| **active** | `full` | `suspended_seller` | `removed_seller` |
| **unavailable** | `suspended_user` | `suspended_user` | `suspended_user` |
| **removed** | `removed_user` | `removed_user` | `removed_user` |

User-axis suspension / removal dominates the seller-axis state.

| State | username | display_name | storefront_* | capability_tier | verification |
|---|---|---|---|---|---|
| `full` | canonical handle | full | full | full | both full |
| `suspended_seller` | canonical handle (or anonymous-safe per evaluator) | redacted | redacted (storefront fields hidden) | coarsened or absent | personal preserved; seller absent / false |
| `suspended_user` | anonymous-safe identifier | absent | absent | absent | absent |
| `removed_user` | slot omission default | n/a | n/a | n/a | n/a |
| `removed_seller` | per UserCard rules | n/a | absent | absent | personal preserved; seller absent |
| `redacted` | per evaluator | redacted | redacted | coarsened | redacted |
| `anonymous_fallback` | anonymous-safe identifier | absent | absent | absent | absent |

SUSPENDED is reversible. REMOVED on the user axis is terminal. REMOVED on the seller axis is per Commerce domain — typically reversible.

## 8. Anonymous-Safe Fallback

Consistent with UserCard:

- deterministic, non-leaking, stable across the lifecycle window;
- shape: `user_<short_hash>` — identity-rooted, not capability-rooted (the underlying identifier is the user UUID);
- never email-fallback under any semantic.

A suspended seller does NOT fall back to their personal display — they emit redacted storefront markers. Storefront fields are not substitutable with personal fields.

## 9. Tombstone

SellerCard does not carry a tombstone shape. Sellers are not content-bearing. A removed seller emits `removed_user` or `removed_seller`.

## 10. Embedded-Card Rules

SellerCard is a **leaf** — it does not embed other cards.

Consumer families that embed SellerCard:

| Consumer family | Embedding rule |
|---|---|
| ContentCard | embeds SellerCard for seller-acting author contexts (ADR-010) |
| ListingCard | embeds SellerCard as the canonical seller reference (ADR-008) |
| AuctionCard | embeds SellerCard (ADR-009) |

Any commerce-card family that emits a seller reference MUST embed SellerCard, never UserCard.

## 11. Cross-Surface Convergence

SellerCard is a single family. The same family applies across:

- Storefront page (`/sellers/:id`, `/shops/:slug`)
- Embedded in ListingCard / AuctionCard
- Embedded in ContentCard for seller-acting authors
- `/search/users` for the seller subset — predicate filters on canonical seller-display attributes; never aliases email
- Reference resolution

No per-surface variants.

## 12. Forbidden Patterns

- **Email fallback in `storefront_name`** — `COALESCE(storefront_name, email)` and analogues.
- **Auth Identity in any SellerCard slot**.
- **Endpoint-specific SellerCard variants** (StorefrontSellerCard, ListingSellerCard, AuctionSellerCard, SearchSellerCard).
- **Repository-built seller cards** — repositories return raw entities + metadata.
- **Handler-built seller cards** — no inline `gin.H` seller-card maps.
- **Frontend-rendered seller exposure** — clients render what the boundary emits; never re-decide capability_tier, verification, or storefront-name fallback.
- **Silent fallback to raw `seller_profiles` row exposure**.
- **Financial-field promotion** — no slot sourced from `seller_payable`, `escrow_balance`, `gateway_payload`, `payout_state`, or any billing column. Recursive and unconditional.
- **Subscription-tier raw exposure** — `capability_tier` is a coarse enum; raw provider tier names (`"premium_yearly_v3"`) are forbidden.
- **Seller-as-separate-identity treatment** — a separate `sellers` table treated as authority for seller identity independent of `users` is forbidden.
- **Inheritance-style coupling with UserCard** — substitution / "SellerCard slots ⊇ UserCard slots" assumptions.
- **Conflating `is_user_verified` and `is_seller_verified`** — single `is_verified` slot is forbidden.
