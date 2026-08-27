# ADR-006 — UserCard Family

## Status

Accepted

## Related Documents

- [`docs/foundation.md`](../foundation.md) — Canonical Authorities (Identity, Public Exposure)
- [`docs/architecture.md`](../architecture.md) — Identity / Trust Model, Discovery / Projection Design
- [`docs/contracts/public-card-boundary.md`](../contracts/public-card-boundary.md) — boundary contract; categories, semantics, forbidden patterns
- [`docs/contracts/viewer-context.md`](../contracts/viewer-context.md) — viewer input contract
- ADR-003 Governance Evaluator; ADR-004 Discovery / Projection Boundary
- Companion ADRs: 007 SellerCard, 010 ContentCard

---

## 1. Decision

UserCard is the canonical exposure shape for a user identity outside any commerce or content context. All user-rendering public surfaces (profile, follower / following lists, reference resolution, embedded-author rendering) MUST flow through the UserCard family builder.

UserCard is a **leaf** card family — it does not embed other cards. It is **composed over** by SellerCard (ADR-007) and **embedded by** ContentCard (ADR-010), CommentAuthorCard, NotificationActorCard, and ChatParticipantCard.

## 2. Ownership

**Identity domain** owns UserCard.

The Identity domain defines: allowed field categories, anonymous-safe fallback shape, suspended / removed shapes, avatar / display-name rules, verification indicator rules.

Ownership does not grant the right to violate the forbidden field categories defined in the public-card boundary contract.

## 3. Canonical Card Shape

```
UserCard {
  id              : opaque user reference                  (every semantic)
  card_state      : enum {full, suspended, removed,
                          anonymous_fallback, redacted}    (every semantic)

  username        : canonical public handle                (full / redacted)
                    -- never email, phone, or firebase_uid
                    -- replaced with anonymous-safe fallback in suspended /
                       anonymous_fallback / removed-with-reference

  display_name    : public display name | null             (full only)
  avatar_url      : canonical avatar reference | null      (full only;
                                                            absent in anonymous_fallback)
  lifecycle_state : enum {active, unavailable, removed} | null
                                                           (where surface requires
                                                            coarse indicator)
  is_verified     : bool | null                            (full / redacted)
                                                           -- never provider name,
                                                              never document refs
  capability      : enum {none, seller, ...} | null        (full only — drives
                                                            UserCard → SellerCard
                                                            embedding decision)
  viewer_relation : enum {none, you_follow, blocks_you,
                          you_blocked} | null              (only where consumer
                                                            family explicitly permits)
}
```

## 4. Allowed Field Categories

Per the public-card boundary contract, UserCard selects from:

| Category | Slot | Rule |
|---|---|---|
| Public Identity Reference | `username` | canonical public handle ONLY |
| Public Display Attributes | `display_name`, `avatar_url` | full on `full`; redacted-shape on `redacted` / `suspended` |
| Public Lifecycle State | `lifecycle_state` | coarse public-facing lifecycle only |
| Public Capability State | `capability` | coarse capability flag only |
| Public Verification Indicator | `is_verified` | coarse boolean only |
| Public Relationship Indicator | `viewer_relation` | viewer-relative coarse indicator only where consumer permits |
| Public Audit Reference | `id` | opaque user reference |

## 5. Forbidden Field Categories

UserCard MUST NOT carry:

- **Auth Identity** — email, phone, firebase_uid, auth provider identifiers, OTP material, session tokens. This is the load-bearing invariant of this ADR. A slot value sourced (directly or via fallback chain) from any Auth Identity column is itself an Auth Identity emission, regardless of slot label. SQL aliasing (`SELECT email AS username`) is forbidden.
- Internal Moderation Metadata
- Financial Authority Fields
- Pricing Authority Fields
- Inventory Internals
- Capability Internals (raw subscription state, raw entitlement bitmaps)
- Relationship Graph Internals (counterparty's relationship sets)
- Verification Internals (uploaded documents, provider payloads)
- Realtime Transport Internals

## 6. Lifecycle Rendering

| User state | Card state | `username` | `display_name` / `avatar_url` | `is_verified` |
|---|---|---|---|---|
| active, hydrated | `full` | canonical handle | full | full |
| `account_status='suspended'` | `suspended` | anonymous-safe identifier | redacted-shape | absent or false |
| `deleted_at IS NOT NULL` | `removed` | anonymous-safe identifier OR slot omission | absent | absent |
| canonical handle unavailable | `anonymous_fallback` | anonymous-safe identifier | absent | absent |
| evaluator REDACT | `redacted` | canonical handle OR anonymous-safe (per evaluator decision) | redacted-shape | redacted-shape |

SUSPENDED is reversible. The boundary does NOT collapse SUSPENDED into DELETED.

## 7. Anonymous-Safe Fallback

When canonical Public Identity Reference is unavailable, redacted, suspended, or deleted but the surface still requires a stable reference:

- **deterministic** — same input → same output, stable across renderings within the lifecycle window.
- **non-leaking** — does not embed email, phone, firebase_uid, or any internal identifier.
- **shape**: `user_<short_hash>` style (specific construction is implementation-defined under the Identity domain's hash function authority).

Email fallback is forbidden under all five exposure semantics. There is no exception path.

## 8. Tombstone

UserCard does **not** carry a tombstone shape. Tombstone applies to content-bearing entities; users are not content-bearing. A user who is both deleted AND whose slot must persist for reference integrity emits `card_state = removed`.

## 9. Embedded-Card Rules

UserCard is a **leaf** — it does not embed other cards.

Consumer families that embed UserCard:

| Consumer family | Embedding rule | Ownership |
|---|---|---|
| ContentCard | embeds UserCard for typical-user authors (ADR-010) | Social domain consumes Identity-domain UserCard |
| CommentAuthorCard | reduced UserCard variant — own family, not "UserCard with fewer fields" | Identity domain |
| NotificationActorCard | reduced UserCard / SellerCard variant — distinct because of stricter staleness and redaction semantics | Identity domain |
| ChatParticipantCard | distinct family — chat-specific relationship-overlay-relative redaction | Chat domain consumes Identity-domain UserCard primitives |

## 10. Cross-Surface Convergence

UserCard is a single family. Per public-card boundary, no endpoint-specific variants are allowed. The same family applies across:

- Profile rendering (`/users/:id`)
- Follower / following lists
- `/search/users` — predicate filters on canonical username only; never aliasing email or any other Auth Identity column into the result's identity slot
- Reference resolution (any endpoint returning a user identity for cross-reference)
- Embedded in ContentCard / commerce card families

Surface-specific variation is expressed only through which categories the family makes available to that surface and through evaluator decisions driven by surface context.

## 11. Forbidden Patterns

- **Email fallback** in any UserCard slot.
- **Auth Identity in Public Identity Reference** — including SQL aliasing.
- **Endpoint-specific UserCard variants** (ProfileUserCard, FollowerUserCard, SearchUserCard, MentionUserCard).
- **Repository-built user cards** — repositories return raw entities + metadata, not cards.
- **Handler-built user cards** — no inline `gin.H` user-card maps.
- **Frontend-rendered user identity** — exposure is a backend-boundary authority; frontend renders what the boundary emits and never re-decides.
- **Silent fallback to raw user-row exposure** — fail-loud: anonymous-safe fallback OR explicit error.
- **`omitempty` as redaction** — serialization optimization, not an exposure decision.
- **Raw verification metadata exposure** — provider name, document references, verifier identity. The `is_verified` slot is a coarse boolean only.
- **Counterparty relationship-set exposure** — followers / blockers / mutes lists. The `viewer_relation` slot is a viewer-relative coarse indicator only.
