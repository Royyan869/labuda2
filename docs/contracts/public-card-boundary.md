# Public Card Boundary Contract

Related Documents:
- docs/foundation.md (Canonical Authorities — Public Exposure, Identity, Discovery)
- docs/architecture.md (Discovery / Projection Design, Identity / Trust Model, Evaluator Authority Design)
- docs/adr/ (ADR-003 Governance Evaluator, ADR-004 Discovery / Projection Boundary)
- docs/contracts/viewer-context.md (ViewerContext Contract)
- docs/contracts/content-detail-visibility-doctrine.md (Endpoint-scoped doctrine for `GET /api/v1/contents/:id`)

---

## 1. Purpose

The Public Card Boundary is the **single canonical exposure authority** in the platform. It is the only place where raw entity state is transformed into the bytes that cross a public surface.

### 1.1 Exposure authority

Every public-facing response — discovery card, search card, listing/auction/content detail card, profile card, comment-author card, notification-actor card, chat-participant card — must pass through the public-card boundary. No endpoint, handler, repository, or DTO assembler is permitted to invent its own field exposure rules.

### 1.2 Separation from evaluator

The evaluator (ADR-003) decides **whether** an entity may be exposed at all (ALLOW / DENY / TOMBSTONE / REDACT). The public-card boundary decides **what bytes** are emitted given that decision. The two are sequential, not interchangeable.

- Evaluator returns DENY → boundary emits nothing for that slot.
- Evaluator returns TOMBSTONE → boundary emits the canonical tombstone shape for that card family.
- Evaluator returns REDACT → boundary applies the canonical redaction shape per field category.
- Evaluator returns ALLOW → boundary emits the full public field set defined by the card family.

The boundary never re-decides allow/deny. The evaluator never serializes fields.

### 1.3 Separation from repository

Repositories return raw entities and metadata. Repositories do not return cards. Repositories do not apply exposure rules. Repositories that emit "public-shaped" rows have created a hidden exposure authority and are doctrine violations under ADR-004.

### 1.4 Separation from ranking

Ranking orders allowed cards. Ranking does not decide visibility, does not decide exposure, does not redact. Ranking signals (engagement scores, freshness metadata) must not be embedded in public card field categories unless explicitly defined for that card family.

---

## 2. Core Rules

### 2.1 Repositories return raw entities + metadata

Repositories produce:
- the canonical write-model row (or projection row, marked as such),
- lifecycle metadata (deletion markers, terminal-state markers),
- moderation metadata (status flags, decision references),
- relationship metadata where it is the repository's authority,
- projection-staleness metadata where applicable.

Repositories must not produce: redacted strings, anonymized handles, tombstone placeholders, public field subsets, or per-viewer variants. All of these are boundary responsibilities.

### 2.2 Evaluator returns decisions

Evaluator (ADR-003) consumes ViewerContext (per the ViewerContext Contract) plus repository-supplied target context, and returns a structured decision: ALLOW / DENY / TOMBSTONE / REDACT, with reason codes, precedence path, and decision metadata. Evaluator does not return rendered fields.

### 2.3 Card-builder applies exposure semantics

The card-builder is the **only** layer that:
- selects fields to expose by card family (Section 3),
- applies redaction shapes,
- applies tombstone shapes,
- applies anonymous-fallback shapes,
- enforces the forbidden field categories (Section 4.2).

Card-builder is a pure function of (raw entity + metadata, evaluator decision, viewer context relevance). It never re-enters the evaluator and never re-queries repositories.

### 2.4 Email never fallback

Email is auth / contact identity (Foundation — Identity). It is **never** a public field, never a fallback for missing username, never present in any card family.

When canonical public name is absent, the boundary emits a deterministic anonymous-safe fallback (Section 5.5). It does not fall back to email, phone, firebase_uid, or any internal identifier.

### 2.5 No DTO bypass

A DTO that serializes a raw entity directly to the wire — bypassing the card-builder — is a forbidden pattern (Section 7.2). All public emissions for entities that have a defined card family must flow through that family's card-builder, regardless of which surface initiates the response.

---

## 3. Card Family Taxonomy

Card families are the canonical exposure shapes. New families require an ADR. Endpoint-specific variants of these families are forbidden (Section 8.2).

### 3.1 UserCard

Canonical public exposure for a user identity outside any commerce or content context. Used by profile rendering, follower/following lists, reference resolution.

### 3.2 SellerCard

Canonical public exposure for a user acting in a seller capability. Distinct from UserCard because it carries seller-capability-relevant fields (e.g., shop binding, seller lifecycle) and applies seller-specific redaction (e.g., suspended seller).

### 3.3 ListingCard

Canonical public exposure for a listing entity in discovery, search, storefront, and reference contexts. Carries listing-relevant exposure fields and embeds a SellerCard reference for the listing's seller.

### 3.4 AuctionCard

Canonical public exposure for an auction entity. Distinct from ListingCard because of auction-specific lifecycle (pending, live, settled, cancelled) and auction-specific exposure semantics. Embeds a SellerCard reference.

### 3.5 ContentCard

Canonical public exposure for social content (posts, articles). Embeds a UserCard or SellerCard reference for the author depending on author capability context, and embeds ListingCard / AuctionCard references for any commerce shares.

### 3.6 CommentAuthorCard

Canonical public exposure for the author of a comment. A reduced UserCard variant that carries only the field categories appropriate for inline comment rendering. CommentAuthorCard is its own family — it is not "UserCard with fewer fields chosen at the call site."

### 3.7 NotificationActorCard

Canonical public exposure for the actor referenced in a notification payload. A reduced UserCard / SellerCard variant scoped to notification-payload exposure semantics. Distinct because notification payloads have stricter staleness and redaction semantics (delivery-time governance per ADR-005).

### 3.8 ChatParticipantCard

Canonical public exposure for a chat participant within a chat room context. Distinct because chat carries relationship-overlay-relative redaction (mute/block applied at participant level) and because chat is a commerce entry point (Foundation — Chat) requiring stable participant identity.

---

## 4. Field Category Model

The boundary contract defines **field categories**, not fields. Concrete fields are bound at implementation time to a category; the contract governs categories.

### 4.1 Allowed field categories

Each card family selects from this enumeration. Inclusion of a category in a family is governed by the family's defining ADR or this contract; ad-hoc inclusion is forbidden.

- **Public Identity Reference** — canonical public handle reference; never email, phone, or auth identity.
- **Public Display Attributes** — public name, public avatar reference, public bio reference where the family permits.
- **Public Lifecycle State** — coarse public-facing lifecycle (active / unavailable / removed); never internal moderation reason codes.
- **Public Capability State** — seller capability flag where family-relevant; never raw subscription state, never billing state.
- **Public Verification Indicator** — coarse boolean indicator only when verification chain is canonical (per Identity / Trust Model); never internal verification metadata.
- **Public Commerce Attributes** — listing/auction-relevant public fields (price reference where authoritative, lifecycle, public location reference where family permits).
- **Public Content Attributes** — content-relevant public fields (title, public excerpt, attachment references through their own families).
- **Public Relationship Indicator** — viewer-relative coarse indicators (e.g., "you follow", "blocked you" surfaces only where family explicitly permits and only as boundary-applied transformation).
- **Public Audit Reference** — opaque references suitable for support/audit lookups (e.g., governance_decision_id passthrough where family permits); never raw moderation metadata.

### 4.2 Forbidden field categories

These categories are forbidden in **every** card family without exception. There is no opt-in path.

- **Auth Identity** — email, phone, firebase_uid, OTP material, session tokens.
- **Internal Moderation Metadata** — moderation case IDs, moderator notes, internal reason codes, appeal internals.
- **Financial Authority Fields** — ledger balances, gateway payloads, payout state, seller payable, subscription billing state.
- **Pricing Authority Fields** — raw pricing tokens, raw pricing snapshot internals, fee/discount internals beyond family-defined public commerce attributes.
- **Inventory Internals** — reservation flags (which do not exist by canonical rule), hidden hold counters, internal FCFS sequencing data.
- **Capability Internals** — raw subscription state, raw entitlement bitmaps, internal capability derivation data.
- **Relationship Graph Internals** — raw block/follow/mute set membership beyond viewer-relative indicators the family explicitly permits; never the counterparty's relationship sets.
- **Verification Internals** — uploaded document references, verification provider payloads, raw verifier identity.
- **Realtime Transport Internals** — connection IDs, room IDs internal, frame sequence numbers, replay cursors.

This enumeration is durable. Additions to the allowed list require an ADR; subtractions from the forbidden list require an ADR.

### 4.3 Category-only contract

This contract intentionally does not enumerate runtime field names. Field names are implementation, not architecture. What this contract durably defines is the category-level architecture, which is what governs whether a code change is doctrine-aligned.

---

## 5. Exposure Semantics

The boundary applies exactly five exposure semantics. Each is canonical; ad-hoc semantics are forbidden.

### 5.1 REDACT

Applied when evaluator returns REDACT for an entity-with-permitted-existence-but-restricted-fields case (e.g., suspended seller still surfacing as a counterparty in chat where chat policy requires identity persistence).

The boundary:
- emits the family's allowed field categories with the redacted-shape variant for any category the evaluator marks redacted,
- preserves Public Identity Reference where the evaluator decision permits, replaced with the canonical anonymous-safe fallback (Section 5.5) where it does not.

REDACT is not silent omission. The emitted card carries an explicit redaction marker so the consumer can render the appropriate UI affordance.

### 5.2 TOMBSTONE

Applied when the entity slot must persist but the entity's content must not. Used for moderation removal, deleted-content placeholders where business policy requires slot persistence, and reference integrity in chat / comment threads.

The boundary:
- emits a tombstone shape canonical to the card family,
- preserves only the structural reference (slot existence + opaque reference for audit),
- emits no Public Display Attributes, no Public Commerce Attributes, no Public Content Attributes.

TOMBSTONE is distinct from DENY (no slot, no card) and from DELETED (terminal state with possibly different rendering).

### 5.3 DELETED

Applied when the underlying entity is in a terminal deleted state (per Foundation — Governance: deletion is terminal unless business explicitly defines restoration).

The boundary:
- emits the family's deleted-shape variant if the surface requires reference integrity,
- otherwise emits nothing and the surface omits the slot.

DELETED differs from TOMBSTONE: TOMBSTONE is a moderation-driven content suppression that preserves slot semantics; DELETED is a lifecycle terminal that may or may not preserve slot semantics depending on the surface's reference-integrity needs.

### 5.4 SUSPENDED

Applied when the underlying actor is in a suspended state (account_status = suspended, or seller capability suspended).

The boundary:
- emits the family's suspended-shape variant — typically Public Identity Reference replaced with the canonical anonymous-safe fallback, Public Display Attributes redacted, Public Capability State coarsened to "unavailable",
- preserves reference integrity for audit and support contexts.

SUSPENDED is reversible by definition (per Foundation — Governance). The boundary therefore does not collapse SUSPENDED into DELETED.

### 5.5 ANONYMOUS FALLBACK

Applied whenever the canonical Public Identity Reference is unavailable, redacted, suspended, or deleted but the surface still requires a stable reference.

The boundary emits a deterministic anonymous-safe identifier shape (per Identity / Trust Model — `user_<short_hash>` style). The fallback is:
- deterministic (same input → same output),
- non-leaking (does not embed email, phone, firebase_uid, or any internal identifier),
- stable across renderings within the relevant lifecycle window.

Email fallback is forbidden under all five semantics. There is no exception path.

---

## 6. Hydration Topology

The canonical hydration flow for any public emission is:

```
Repository
  → raw entity + metadata
Evaluator
  → decision (ALLOW / DENY / TOMBSTONE / REDACT) + decision metadata
Card-Builder
  → card family selection + field-category emission + exposure semantics
Outbound Response
  → wire bytes
```

Each arrow is a one-way data flow. Specifically:

- Repository → Evaluator: repository hands raw entity + metadata; evaluator does not call back into repository (per Evaluator Authority Design — caller-hydrates).
- Evaluator → Card-Builder: evaluator hands decision + metadata; card-builder does not call back into evaluator (Section 7.4).
- Card-Builder → Outbound: card-builder produces a complete card; outbound serializer does not add or remove fields.
- Outbound: serializer writes bytes; it does not invent fields, does not omit fields based on its own logic, does not apply ad-hoc privacy rules.

Embedded references (e.g., a ListingCard embedding a SellerCard) flow through this same topology recursively: the embedded entity has its own repository fetch, its own evaluator decision (which may differ — a listing may be ALLOW while its seller is SUSPENDED), and its own card-builder pass.

---

## 7. Forbidden Patterns

Each pattern below is durable and doctrine-aligned forbidden.

### 7.1 Direct entity serialization

Serializing a raw entity (write-model row, projection row, or repository result) to the wire on a public surface, bypassing the card-builder.

Forbidden because: it makes every endpoint its own exposure authority, exposes whatever fields the entity happens to carry (including email if the row carries email), and prevents evaluator decisions from being honored at the byte level.

### 7.2 DTO bypass

Constructing an endpoint-specific DTO that selects fields from a raw entity and emits them, instead of routing through the card family's builder.

Forbidden because: each DTO becomes a hidden exposure authority; field-level redaction, tombstone, and anonymous-fallback semantics drift across endpoints; the boundary becomes operationally unenforceable.

### 7.3 Repository-built cards

A repository that returns "public-shaped" rows (pre-redacted, pre-anonymized, with internal columns omitted) instead of raw entity + metadata.

Forbidden because: it makes the repository a hidden exposure authority; it couples repository changes to exposure semantics changes (so a schema change becomes an exposure-policy change); and it prevents the evaluator from operating on full target context.

### 7.4 Card-builder evaluator re-entry

A card-builder that calls the evaluator (or any visibility predicate) during card construction, instead of consuming the decision the caller already obtained.

Forbidden because: it duplicates evaluator authority, creates the possibility of evaluator divergence within a single response, and breaks the "caller hydrates truth" rule by making the card-builder a hidden caller.

### 7.5 Implicit omitempty exposure

Using serializer-level `omitempty` (or equivalent) as an exposure mechanism — i.e., relying on the absence of a field in serialization output to "redact" it.

Forbidden because: omitempty is a serialization optimization, not an exposure decision. Whether a field is present or absent in the wire output must be determined by the card-builder per the family's field categories and the evaluator's decision, not by whether the underlying value happens to be a zero value.

### 7.6 Email fallback

Using email as a fallback for any Public Identity Reference, Public Display Attribute, or anonymous-fallback shape.

Forbidden because: email is auth / contact identity (Foundation — Identity; ADR-004). Email is never a public field, never a fallback for any public-identity slot.

---

## 8. Card Ownership Rules

### 8.1 Card family ownership

Each card family is owned by the domain that owns the underlying entity:

- UserCard, SellerCard, CommentAuthorCard, NotificationActorCard → Identity domain.
- ListingCard, AuctionCard → Commerce domain.
- ContentCard → Social domain.
- ChatParticipantCard → Chat domain.

Ownership means: the owning domain defines the card family's allowed field categories, exposure semantics application, and tombstone/suspended/deleted shapes — within the constraints of this contract. Owning a card family does not grant the right to violate the forbidden field categories (Section 4.2).

### 8.2 No endpoint-specific card variants

An endpoint **may not** define its own card variant (e.g., "FeedListingCard" vs "SearchListingCard" vs "StorefrontListingCard"). The single ListingCard family applies to all surfaces, with surface-specific variation expressed only through:
- which field categories the family makes available to that surface (declared in the family's owning domain),
- evaluator decision differences driven by surface context (per Evaluator Authority Design — Surface Context).

Surface context is an evaluator input, not a card-family multiplier.

### 8.3 No ad-hoc exposure logic

Once a card family exists, exposure decisions for entities of that family must flow exclusively through the family's builder. Inline `if` checks at handlers, in serializers, in middleware, or in projection prefilters that omit or modify fields are ad-hoc exposure logic and are forbidden.

The only legal place to add a new exposure rule is inside the card-builder of the relevant family, governed by an ADR change to this contract.

---

## 9. ADR References

- **ADR-003 — Governance Evaluator.** This contract is the exposure-side complement to ADR-003. ADR-003 mandates evaluator as final visibility authority and warns that evaluator must not bypass the public-card boundary; this contract defines what that boundary is.
- **ADR-004 — Discovery / Projection Boundary.** ADR-004 establishes the layered topology Write Model → Projection → Evaluator → Public Card Boundary → Ranking. This contract is the materialized Public Card Boundary layer of that topology.

---

## 10. Contract Status

This contract is materialized doctrine, not implementation guidance.

It defines:
- the only legal path from raw entity to public bytes,
- the canonical card families,
- the field categories allowed and forbidden,
- the canonical exposure semantics,
- the durable forbidden patterns,
- card-family ownership rules.

It does not define runtime types, field names, handler signatures, builder implementation, or serializer implementation.

Implementation that cannot cite this contract for affected public surfaces must not proceed.
