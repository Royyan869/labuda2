# Content Detail Visibility Doctrine

Related Documents:
- docs/contracts/public-card-boundary.md (Public Card Boundary Contract — exposure semantics, ContentCard family)
- docs/contracts/viewer-context.md (ViewerContext Contract — overlay topology, hydration discipline)
- docs/adr/003-governance-evaluator.md (Governance Evaluator — visibility authority)
- docs/adr/010-content-card-family.md (ContentCard family ownership)

---

## 1. Purpose and Scope

This doctrine governs the public content-detail read endpoint:

```
GET /api/v1/contents/:id
```

It defines who may receive a 200 response for a given content row, who must receive 404, and which authority owns each decision. It does not govern create / update / delete on the same path; those are owner-management semantics with their own authorization rules.

### 1.1 Endpoint posture

`GET /api/v1/contents/:id` is a **public viewer-governed read endpoint**. It is not an owner-management endpoint, not a moderation tool surface, and not a recovery path. Its only legitimate role is "render the canonical public view of a content row to a caller who is allowed to see it."

Owner recovery, edit-history, and self-introspection of moderated/deleted content are out of scope. They MUST be served by a separate owner-management endpoint (future doctrine — Section 6).

### 1.2 Authority topology

Visibility on this endpoint follows the canonical Evaluator → Public Card Boundary topology (public-card-boundary.md §1.2, §6). The evaluator decides whether the row may be exposed at all; the boundary decides what bytes are emitted given that decision. This doctrine fixes the **decision-to-HTTP-status mapping** that is specific to this endpoint.

---

## 2. Visibility Doctrine

### 2.1 Active, non-hidden content

ALLOW for any authenticated viewer where viewer lifecycle is active, viewer↔author relationship is unblocked, and author lifecycle is active.

Wire: `200 OK` + canonical ContentCard.

### 2.2 Deleted content (`status = deleted`)

| Caller | Outcome |
|---|---|
| Normal viewer | **404** |
| Owner / self | **404** (no self-override on this endpoint) |
| Admin / moderator | **200** for review and investigation |

Rationale: deleted content is terminal lifecycle (foundation.md — Governance: deletion is terminal). Public surfaces emit nothing; admin review preserves audit access.

### 2.3 Hidden / moderated content (`is_hidden = true`)

| Caller | Outcome |
|---|---|
| Normal viewer | **404** |
| Owner / self | **404** (no self-override on this endpoint) |
| Admin / moderator | **200** for review and investigation |

Rationale: hidden is a moderation suppression flag, not a soft-lifecycle. Treat as removed for every non-moderator caller including the author.

### 2.4 Blocked viewer↔author relation

| Caller | Outcome |
|---|---|
| Normal viewer (block exists in either direction) | **404** |
| Admin / moderator | **200** only if the moderator's capability set explicitly authorizes block-override review; otherwise **404** |

No public tombstone. The caller does not learn whether the block, the deletion, or a never-existed-content is the cause — all three collapse to 404.

### 2.5 Suspended / banned / deleted author

| Caller | Outcome |
|---|---|
| Normal viewer | **404** |
| Admin / moderator | **200** for review and investigation |

No public tombstone on this endpoint. Reversible author suspension does not differ from terminal author deletion at the wire — both collapse to 404 for non-moderators. Surfaces that require slot-persistence semantics (chat, comment threads) use their own card families and apply public-card-boundary.md §5.2 / §5.4 there; **this endpoint does not.**

### 2.6 Anonymous viewer

Out of scope. The endpoint requires authentication (route mounted under the authenticated `/api/v1` group). An unauthenticated request is rejected with `401` by middleware before any visibility evaluation runs.

---

## 3. Response Contract

### 3.1 DENY → 404

Every non-ALLOW evaluator outcome (DENY / TOMBSTONE / REDACT) maps to HTTP `404 Not Found` on this endpoint.

The body shape on 404 follows the platform's standard not-found envelope. The caller MUST NOT be able to distinguish, from the response, between:
- content never existed,
- content was deleted,
- content was hidden by moderation,
- content's author is suspended / banned / deleted,
- viewer is blocked by the author,
- author is blocked by the viewer.

This is a deliberate information-disclosure constraint. Information about why the content is unavailable is available only through admin / moderation tooling.

### 3.2 No tombstone rendering yet

This doctrine does **not** adopt the "200 + tombstone-shaped ContentCard with `lifecycle = removed`" pattern that public-card-boundary.md §5.2 / §5.3 makes mechanically available. Reasons:

- Mobile and any other clients today interpret 404 on this endpoint as "content not found." Switching to 200 + degraded card is a wire-contract change that requires coordinated client work.
- The doctrine in §2.2–§2.5 deliberately collapses multiple distinct causes into a single response code; emitting a tombstone card on some causes but not others would re-leak the distinction.
- Slot persistence (the canonical justification for TOMBSTONE rendering) does not apply to a single-content fetch — there is no surrounding sequence whose integrity requires the slot to remain visible.

Future adoption of 200 + tombstone on this endpoint requires both (a) explicit doctrine amendment here and (b) a coordinated client rollout. Until both land, evaluator DENY / TOMBSTONE / REDACT all map to 404.

### 3.3 Lifecycle field on ALLOW

When the evaluator returns ALLOW, the response body emits a ContentCard whose `lifecycle` follows public-card-boundary.md §5: the coarsened `{active, unavailable, removed}` vocabulary derived from `entity.Status.PublicLifecycle()`.

`fulfilled` content (request socially-closed by its creator) coarsens to `lifecycle = "unavailable"` and **remains visible on this endpoint** — it is ALLOW with coarsened lifecycle, not DENY. This preserves the social-history use case (a fulfilled request remains readable on its detail page).

---

## 4. Self-Author Override Policy

There is no self-author override on this endpoint.

A caller whose `userID` equals `content.author_id` is treated exactly as a normal viewer for visibility purposes. The author cannot, through this endpoint:
- read their own deleted content,
- read their own hidden / moderated content,
- bypass blocked-relation visibility against another author's content.

This is doctrine, not a current-implementation accident. The justification:

- This endpoint is a viewer-governed public read. Owner-introspection is a separate concern with separate authorization (e.g., audit-trail review, content recovery workflows, edit history).
- A self-override here would create a hidden second authority for visibility — every consumer of the response would need to know "this 200 might be only-visible-to-the-author" and re-apply the rule. This is exactly the doctrine violation public-card-boundary.md §1.4 / §8.3 forbids.
- The Update path (`PUT /api/v1/contents/:id`) already special-cases self: that is an owner-management operation and its own authorization is correct. Visibility on the Read path is not symmetric with mutation authority on the Update path.

Future owner-recovery surfaces (e.g., `GET /api/v1/contents/me/:id` or `/api/v1/me/contents/:id`) MAY relax this rule in their own doctrine, scoped to that endpoint, but MUST NOT reintroduce self-override on this public read.

---

## 5. Admin / Moderator Override Policy

### 5.1 Admin / moderator ALLOW

Callers whose role resolves to `admin` or `moderator` receive 200 for §2.2 (deleted), §2.3 (hidden), and §2.5 (suspended/banned/deleted author). This is review and investigation access — not public surfacing.

### 5.2 Block-override is capability-gated

Admin / moderator override of §2.4 (blocked relation) is **not** automatic. It requires an explicit moderation capability authorizing block-override review (capability name and grant policy to be defined when the shadow seam lands in Batch 3Q).

A moderator without that explicit capability receives 404 for blocked-relation content, identical to a normal viewer. This preserves the principle that block relationships are user-controlled privacy primitives and moderator override of them is an explicitly-granted authority, not a side effect of the moderator role.

### 5.3 Admin response shape

Admin 200 responses MUST still flow through the canonical ContentCard builder (public-card-boundary.md §2.3, §8.3). The response body shape is identical to a normal-viewer ALLOW response. Admin tooling that needs additional internal moderation metadata (case IDs, decision history, moderator notes — all forbidden field categories per public-card-boundary.md §4.2) MUST consume that metadata from dedicated admin / moderation endpoints, not from a side-channel on this public endpoint.

---

## 6. Future Owner / Admin Tooling

This doctrine deliberately punts owner-management semantics to future endpoints. Two future surfaces are anticipated:

- **Owner recovery / edit-history** — a future authenticated endpoint scoped to the caller's own contents, allowing the author to enumerate their own deleted and hidden content for recovery or audit. Path and doctrine to be specified when the feature lands.
- **Admin moderation review** — a future admin-only endpoint with full internal-metadata access (moderation case IDs, decision history, appeal state, etc.), satisfying the constraints in §5.3.

Until those surfaces land, owner introspection and admin moderation review beyond what §5.1 permits are out of scope. Backfilling them by widening this endpoint is doctrine-violating.

---

## 7. Authority Source Map

Each decision input on this endpoint is owned by exactly one authority. The handler hydrates these inputs and passes them to the evaluator; the evaluator returns the decision.

| Decision input | Authority source |
|---|---|
| Content `status` (active / fulfilled / deleted) | `contents.status` column (write model) |
| Content `is_hidden` | `contents.is_hidden` column (moderation write authority) |
| Author lifecycle (active / suspended / banned / deleted) | `users.account_status` + `users.deleted_at` |
| Viewer lifecycle | `users.account_status` + `users.deleted_at` for caller |
| Viewer↔author block relation | `user_blocks` table, bidirectional resolution |
| Caller role (admin / moderator) | Roles lookup via RoleChecker (Actor context) |
| Caller block-override capability | Capability set on the injected Actor |

The evaluator never reaches into these sources itself (Evaluator Authority Design — caller-hydrates). The handler is responsible for hydrating all of them into the ViewerContext + TargetContext shapes before invoking the evaluator.

---

## 8. Shadow Seam Specification (preparation for Batch 3Q)

This section specifies the shape of the next batch's implementation. It does not authorize implementation; that authority is the Batch 3Q prompt.

### 8.1 Pure decision function

```
EvaluateContentDetail(vc *ViewerContextContentDetail, tc *TargetContextContentDetail)
    → (decision ShadowDecision, reason UnknownReason)
```

Pure: no IO, no DB reads, no logging. All inputs are caller-hydrated.

### 8.2 ViewerContext fields needed

| Field | Source | Purpose |
|---|---|---|
| `ViewerID uuid.UUID` | Auth middleware | Caller identity |
| `IsAdmin bool` | RoleChecker.IsAdmin | Admin/moderator override gate (§5.1) |
| `IsModerator bool` | RoleChecker.IsModerator | Moderator override gate (§5.1) |
| `HasBlockOverrideCapability bool` | Capability lookup | Block-override gate (§5.2) |
| `HasLifecycle bool` + `AccountStatus string` + `Deleted bool` | `users.account_status` / `deleted_at` for ViewerID | Viewer lifecycle precedence |
| `HasRelationship bool` + `BlockedSet map[uuid.UUID]struct{}` | `user_blocks` scoped to (viewer, author) | Block precedence |

The shape may reuse the existing `evaluator.ViewerContextShadow` if extended with the admin / moderator / capability fields; or it may be a fresh `ViewerContextContentDetail` type. Choice is implementation-detail for Batch 3Q.

### 8.3 TargetContext fields needed

| Field | Source | Purpose |
|---|---|---|
| `ContentID uuid.UUID` | Row from `contents` | Identity |
| `AuthorID uuid.UUID` | Row from `contents` | Owner reference |
| `Status string` | `contents.status` | Lifecycle gate (active / fulfilled / deleted) |
| `IsHidden bool` | `contents.is_hidden` | Moderation gate |
| `HasOwnerLifecycle bool` + `OwnerAccountStatus string` + `OwnerDeleted bool` | `users.account_status` / `deleted_at` for AuthorID | Author lifecycle precedence |

Reusing `evaluator.TargetContextShadow` is acceptable — its fields already cover this set.

### 8.4 Precedence order

The decision function evaluates in this order. The first matching step returns; later steps are not consulted.

1. **Input validity.** If `vc == nil` or `tc == nil`, return `UNKNOWN / input_invalid`.
2. **Admin / moderator bypass for non-block cases.** If `IsAdmin || IsModerator`, ALLOW for §2.2 (deleted), §2.3 (hidden), §2.5 (suspended/banned/deleted author). Block-override (§2.4) is NOT covered here — see step 6.
3. **Viewer lifecycle.** If `!HasLifecycle` → UNKNOWN / viewer_overlay_missing. If viewer `Deleted` or `AccountStatus IN (deleted, banned, suspended)` → DENY.
4. **Target lifecycle.** If `tc.Status != "active" && tc.Status != "fulfilled"` (i.e. `deleted`) → DENY. (Admin bypass already handled in step 2.)
5. **Target moderation.** If `tc.IsHidden` → DENY. (Admin bypass already handled in step 2.)
6. **Author lifecycle.** If `!HasOwnerLifecycle` → UNKNOWN / target_overlay_missing. If owner `Deleted` or `OwnerAccountStatus IN (deleted, banned, suspended)` → DENY. (Admin bypass already handled in step 2.)
7. **Relationship.** If `!HasRelationship` → UNKNOWN / viewer_overlay_missing. If `tc.AuthorID ∈ BlockedSet` and **not** `HasBlockOverrideCapability` → DENY. (Block-override is capability-gated per §5.2 — admin role alone does not bypass.)
8. **Allow.** Reached only when none of the above matched. ALLOW with lifecycle coarsened per §3.3.

### 8.5 Decision-to-HTTP mapping

| Evaluator decision | Shadow seam emits | Future enforce action |
|---|---|---|
| ALLOW | `would_200` | 200 + ContentCard |
| DENY | `would_404` | 404 |
| TOMBSTONE | `would_404` | 404 (per §3.2) |
| REDACT | `would_404` | 404 (per §3.2) |
| UNKNOWN | `shadow_unknown` | **fail-CLOSED → 404** (recommended) |

UNKNOWN fail-CLOSED on this endpoint is the recommended posture for the eventual enforce flip, distinct from the feed-evaluator fail-OPEN policy (feed is high-traffic and a hydration outage that blanks Home is worse than briefly over-allowing). Content detail is single-row, low-traffic, and a hydration outage that 404s a handful of requests is preferable to leaking moderated content. This recommendation is doctrine here; the enforce-mode pilot in a later batch will validate it operationally before committing.

### 8.6 ContentDetailShadowRunner

Fire-and-forget runner constructed at boot, dispatched by the handler **after** the response is written. Strict shadow rules (mirroring `FeedShadowRunner` at `backend/internal/governance/evaluator/feed_shadow.go:79–146`):

- Never mutates the response.
- Never returns a decision to the caller.
- Performs only the documented hydration SELECTs (users × 2, user_blocks × 1, roles already in Actor context).
- On hydration failure, emits UNKNOWN with a classified reason; never synthesizes fallback truth.
- Emits telemetry under `surface="content_detail"` with the existing bounded label set (decision, divergence cell, overlay status, hydration error source).

### 8.7 Telemetry surfaces

Emit under the existing shadow-telemetry namespace, with surface label `content_detail`. Required counters:

- `shadow_request_total{surface="content_detail"}` — denominator.
- `shadow_decision_total{surface="content_detail",decision=…}` — per-request decision.
- `shadow_divergence_total{surface="content_detail",category=…}` — legacy-vs-shadow cell.
- `shadow_unknown_total{surface="content_detail",reason=…}` — UNKNOWN classification.
- `shadow_hydration_error_total{surface="content_detail",source=…}` — hydration failure source.

Divergence categories on this surface follow the standard taxonomy. Because the legacy handler returns 200 for deleted/hidden when the caller is admin AND returns 200 for blocked / suspended-author cases for every caller, the dominant expected divergence cell is `legacy_allow_shadow_deny` driven by author-lifecycle and block precedence steps that the legacy path does not enforce. This is the same magnitude-of-broadening signal the search-shadow seam observed (search-shadow-seam-landing-task-design §284).

### 8.8 What Batch 3Q delivers

- `EvaluateContentDetail` pure function under `backend/internal/governance/evaluator/`.
- `ContentDetailShadowRunner` + the three hydration helpers.
- Boot wiring under `serverboot/dependencies.go` (env-gated, same pattern as `EVALUATOR_SHADOW_FEED_ENABLED`).
- Handler dispatch from `GetContent` (post-response, fire-and-forget).
- Telemetry registration.
- Tests for the pure function (precedence table coverage).
- No enforce mode, no handler authority change, no wire-contract change.

Enforce activation is explicitly **not in scope** for Batch 3Q. It requires its own pilot batch after staging observation, mirroring the search/feed enforce sequencing.

---

## 9. Doctrine Status

This doctrine is materialized policy, not implementation guidance.

It defines:
- the wire-level visibility contract for `GET /api/v1/contents/:id`,
- the per-decision-input authority map,
- the canonical decision-to-HTTP-status mapping,
- the self-author and admin-override policy,
- the shape of the next batch's shadow seam.

It does not define handler implementation, evaluator type names, or telemetry metric implementation. Those are bound at implementation time in the relevant Batch 3Q artefact.

Implementation that cannot cite this doctrine for affected endpoint behavior must not proceed.
