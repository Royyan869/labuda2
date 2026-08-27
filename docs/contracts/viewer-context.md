# ViewerContext Contract

Related Documents:
- docs/contracts/governance-constitution.md (Governance Constitution — canonical lock for evaluator, surface classification, UNKNOWN policy, mute doctrine, implementation sequence)
- docs/foundation.md (Canonical Authorities — Identity, Visibility)
- docs/architecture.md (Evaluator Authority Design, Discovery / Projection Design)
- docs/adr/ (ADR-003 Governance Evaluator, ADR-005 Realtime Signal Not Authority)

---

## 1. Purpose

ViewerContext exists to make viewer truth a **first-class, explicit input** to every governance, exposure, and delivery decision in the platform.

### 1.1 Why ViewerContext exists

Visibility, block semantics, followers-only semantics, moderation overlays, and lifecycle propagation all depend on *who is asking*. Without an explicit viewer carrier, visibility predicates degenerate into one of three doctrine violations:

- assuming an anonymous viewer (leaking governance state),
- skipping relationship checks,
- performing ad-hoc viewer lookups inside repositories or evaluator-adjacent code.

ViewerContext is the canonical replacement: a structured, explicit, immutable carrier for viewer truth that travels with every request.

### 1.2 Evaluator dependency

The Canonical Visibility / Governance Evaluator (ADR-003) is contractually bound by the rule:

> Caller hydrates truth. Evaluator evaluates truth.

Without ViewerContext as an input, the evaluator either becomes a hidden DB authority (forbidden) or operates on partial truth (drift). ViewerContext is therefore a **prerequisite** for evaluator rollout, not an optimization.

### 1.3 Governance convergence dependency

Governance convergence (cross-surface allow/deny/tombstone/redact consistency) requires every surface to feed the same evaluator the same shape of viewer input. ViewerContext is that shape.

---

## 2. Core Rules

These rules are non-negotiable.

### 2.1 Viewer never nil

Every governance/exposure decision call site must receive a non-nil ViewerContext. A missing viewer is not represented by `nil`, by an empty string, or by an implicit "anonymous" default — it is represented by an **explicit** AnonymousViewer instance.

### 2.2 AnonymousViewer explicit

AnonymousViewer is a real, named state in the topology, not an absence. It carries explicit metadata (e.g., surface, request origin classification, anonymity reason) and exists so that public surfaces can still receive a structured viewer input without manufacturing one ad hoc.

### 2.3 Caller hydrates truth

The boundary that has access to authoritative identity, account state, and relationship truth is responsible for constructing the ViewerContext. Downstream layers (evaluator, public-card boundary, realtime broadcaster) must not "complete" or "fix up" a partially built ViewerContext.

### 2.4 Evaluator never fetches internally

The evaluator must not perform DB reads to acquire viewer truth, target truth, or relationship truth. If the input is incomplete, the evaluator returns UNKNOWN (in shadow) or DENY (in production) — it does not silently hydrate. This rule prevents the evaluator from becoming a hidden authority.

**Operational lock.** The evaluator package MUST NOT hold a DB pool field, MUST NOT contain SQL strings or `Query` / `QueryRow` calls, and MUST NOT launch IO-performing goroutines. Hydration helpers belong to the caller's package, never the evaluator's. See docs/contracts/governance-constitution.md §3 (forbidden patterns F1–F3).

### 2.5 Overlays are inputs not authority lookups

Identity overlay, lifecycle overlay, capability overlay, relationship overlay, and moderation overlay are all **inputs** carried inside ViewerContext. They are not lookups the evaluator performs against tables. The caller assembles overlays at the system boundary; the evaluator reads them.

---

## 3. ViewerContext Topology

ViewerContext has exactly two top-level shapes. There is no third "system" or "service" shape (system actors are modelled separately and never traverse evaluator boundaries as a ViewerContext).

### 3.1 AnonymousViewer

Represents a viewer with no authenticated identity.

Carries:
- explicit anonymity flag (true)
- surface classification (public discovery, public detail, public listing, etc.)
- request origin classification (public web, public mobile, replay, system-initiated public reconciliation)
- no identity overlay
- no relationship overlay
- empty capability overlay (no capabilities granted by anonymity)
- empty moderation overlay (anonymous viewer cannot have moderation state against it)

Permitted on:
- public discovery surfaces
- public detail surfaces
- public card hydration paths

Forbidden on:
- personal surfaces (saved items, profile of self, notification inbox)
- websocket subscribe to non-public rooms
- websocket broadcast scoping where viewer-relative semantics apply
- notification delivery (notifications always have an addressee)

### 3.2 AuthenticatedViewer

Represents a viewer with a resolved canonical identity.

Carries:
- explicit anonymity flag (false)
- canonical identity overlay (firebase_uid binding, account row reference)
- account_status (active, suspended, banned, deleted)
- public handle reference (username/profile binding for self-identification)
- capability overlay (seller capability, admin capability, system-overlay capability where applicable)
- relationship overlay (follow/block/mute as inputs, not authority queries)
- moderation overlay (warnings, restrictions, appeals state relevant to the viewer themselves)
- surface classification (same enumeration as Anonymous)
- request origin classification (REST, WS subscribe, WS broadcast scoping, notification delivery, replay)

AuthenticatedViewer never contains:
- gateway/payment authority
- ledger balances
- pricing tokens
- inventory state
- write-model entity rows

Those are target/context truths, not viewer truths.

---

## 4. Overlay Model

Overlays are the structured inputs that make ViewerContext sufficient for evaluator decisions. Each overlay is hydrated at the caller boundary and is **read-only** downstream.

### 4.1 Identity overlay

Carries the canonical viewer identity binding:
- firebase_uid binding (auth identity)
- canonical user/account reference (actor identity)
- public handle reference (username/profile, never email)

Identity overlay is **never** the place to expose email, phone, raw verification metadata, or auth secrets. Email is auth/contact identity only (Foundation — Identity); it is not part of ViewerContext exposure semantics.

### 4.2 Lifecycle overlay

Carries the viewer's own lifecycle state:
- account_status (active / suspended / banned / deleted)
- soft-deletion / restoration markers if applicable
- terminal state markers

Lifecycle overlay is consumed by the evaluator to fail closed on viewer-side ban/suspension precedence (Evaluator Authority Design — Precedence Model).

### 4.3 Capability overlay

Carries declared capabilities of the viewer:
- seller capability (derived canonically from seller profile + subscription + account_status)
- admin capability (auditable; admin override path)
- system-initiated context capability (e.g., reconciliation, where authorized)

Capability overlay is **declared** by the caller, not inferred mid-evaluation. Misuse of capability overlay (e.g., synthesizing seller capability where none exists) is a doctrine violation.

### 4.4 Relationship overlay

Carries viewer-to-target relationship inputs:
- follow set or follow membership probe result
- block set or block membership probe result (bidirectional semantics resolved by caller)
- mute set or mute membership probe result

Relationship overlay is **inputs**. The evaluator never re-queries the relationship graph; if the overlay does not contain the required relationship, the evaluator returns DENY in production or UNKNOWN in shadow.

**Mute activation status.** Mute is a CANONICAL overlay. Canonical consumers are feed, comments list, notifications list (in-app), and chat messages. Current "stored but never enforced" state is TRANSITIONAL DEBT. Rollout MUST be per-surface, shadow-first; global activation across consumers in a single step is FORBIDDEN. See docs/contracts/governance-constitution.md §6.

### 4.5 Moderation overlay

Carries moderation-relevant state pertaining to the viewer:
- active warnings affecting visibility
- restriction states relevant to the surface
- appeal-in-progress markers if surface-relevant

Moderation overlay is not the same as **target** moderation state. Target moderation state is a Target Context input (per Evaluator Authority Design), not a Viewer Context input.

---

## 5. Lifecycle

ViewerContext has a strict lifecycle. Violations of the lifecycle are forbidden patterns (Section 8).

### 5.1 Where created

ViewerContext is created at exactly one boundary per request:
- **HTTP boundary**: the request authentication middleware constructs ViewerContext from the verified auth token (or AnonymousViewer if absent/anonymous).
- **WebSocket boundary**: the connection upgrade / subscribe handshake constructs ViewerContext.
- **Notification delivery boundary**: the delivery worker constructs ViewerContext for the addressee at delivery time (not at enqueue time — see Section 6 Pattern E).
- **Replay boundary**: the replay/reconciliation runner constructs ViewerContext as part of replay setup; replay never reuses a stale enqueue-time ViewerContext.

### 5.2 Where enriched

ViewerContext is enriched only at the same boundary that created it, and only before being passed to downstream layers. Enrichment includes:
- attaching account_status,
- attaching capability overlay,
- attaching relationship overlay (or relationship probes the caller resolves before evaluator entry),
- attaching moderation overlay.

Once the ViewerContext leaves the construction boundary, it is **immutable**.

### 5.3 Where required

ViewerContext is required at:
- every evaluator entry point,
- every public-card boundary entry point that performs viewer-relative redaction,
- every notification delivery and retry path,
- every websocket subscribe authorization point,
- every websocket broadcast scoping point,
- every saved-item / personal-surface hydration path,
- every profile rendering path.

### 5.4 Where optional

ViewerContext is optional only on surfaces that are intrinsically subject-only and viewer-invariant (Section 7.3). Even on those surfaces, when ViewerContext is present, it must be honored — it is never silently discarded.

### 5.5 Where forbidden

ViewerContext is forbidden as input to:
- pricing token construction (pricing authority must not be viewer-relative beyond the explicit buyer binding already encoded in the token itself),
- ledger mutation paths (ledger authority is not viewer-relative),
- gateway/payment authority paths,
- inventory mutation paths (FCFS is viewer-agnostic at the authority layer),
- ranking pure-ordering logic (ranking does not gate visibility; evaluator does).

---

## 6. Propagation Patterns

Five canonical propagation patterns. All surfaces must map to exactly one. Inventing a new pattern outside this enumeration is a Freeze Protocol violation.

### Pattern A — Public Discovery

Applies to: public feed, public search, public storefront, public listing/auction listings, public content listings.

Construction: HTTP boundary builds either AuthenticatedViewer (if request is authenticated) or AnonymousViewer (otherwise).

Propagation: the same ViewerContext travels through:
- discovery layer (read optimization),
- evaluator (visibility decision),
- public-card boundary (exposure decision),
- ranking (ordering only, visibility-agnostic).

Constraint: discovery layer may use ViewerContext for projection prefilter only as a **subset** of evaluator deny (per Discovery / Projection Design); evaluator remains final authority.

### Pattern B — Personal Surfaces

Applies to: saved items, profile (self), notification inbox, personal storefront management, personal commerce dashboards, personal moderation/appeals views.

Construction: HTTP boundary builds AuthenticatedViewer. AnonymousViewer is **forbidden** on Pattern B; the surface must reject the request at the boundary.

Propagation: ViewerContext travels through hydration, evaluator (for any embedded references — e.g., a saved listing's seller card), and public-card boundary (for any embedded public-card emissions).

Constraint: viewer identity in Pattern B is also the subject identity for self-rendering paths; the evaluator must distinguish self-view from other-view to avoid unnecessary redaction of self.

### Pattern C — WebSocket Subscribe

Applies to: every WS subscribe handshake (chat rooms, auction rooms, notification streams, presence streams, realtime listing/order streams).

Construction: WS boundary constructs AuthenticatedViewer (or AnonymousViewer for explicitly public streams) at handshake time.

Propagation: the subscribe-time ViewerContext authorizes the **subscription** itself but is never trusted as the broadcast-time governance state (per Pattern D and ADR-005).

Constraint: subscribe authorization must call the evaluator with the constructed ViewerContext. A subscribe accepted on stale or absent ViewerContext is governance-blind subscribe — forbidden.

### Pattern D — WebSocket Broadcast

Applies to: every per-recipient broadcast scoping decision (chat fanout, auction state push, moderation push, notification push, presence delivery).

Construction: the broadcast worker constructs a **fresh** per-recipient ViewerContext at broadcast time. It does not reuse the subscribe-time ViewerContext, and it does not reuse any enqueue-time snapshot.

Propagation: per-recipient ViewerContext flows into the evaluator at delivery time; if the evaluator returns DENY/TOMBSTONE, the frame is suppressed/redacted before the wire.

Constraint: replay-driven broadcast must follow the same rule. Replay ViewerContext is constructed fresh from current authority state, never resurrected from the original enqueue-time snapshot. Trust of an enqueue-time snapshot for broadcast is forbidden.

### Pattern E — Notification Delivery / Retry

Applies to: notification dispatch, retry, push, email, in-app delivery.

Construction: the delivery worker constructs the recipient ViewerContext at **delivery time** (and again at each retry), from current authority state.

Propagation: ViewerContext flows into the evaluator before send; evaluator returns ALLOW/DENY/TOMBSTONE/REDACT; deny suppresses send and emits an audited blocked-delivery event; redact rewrites the payload through the public-card boundary.

Constraint: enqueue-time ViewerContext or enqueue-time evaluator decision is observability / correlation only — never authority for delivery (Foundation — Notification Authority; Realtime Authority). Retry trusting a stale ViewerContext is forbidden.

---

## 7. Surface Classification

Every surface in the system falls into exactly one classification.

### 7.1 Full ViewerContext surfaces

Require complete, non-nil ViewerContext (Authenticated or Anonymous) with all overlays hydrated where applicable.

Members:
- public feed
- public search
- public storefront / discovery
- public listing detail
- public auction detail
- public content detail
- profile rendering (self and other)
- saved items / shortlist hydration
- notification delivery (Pattern E)
- websocket subscribe (Pattern C)
- websocket broadcast scoping (Pattern D)
- chat room rendering and message visibility checks

### 7.2 Partial ViewerContext surfaces

Surfaces that may legitimately operate with a reduced overlay set during shadow-stage rollout, when the full overlay is not yet operationally available. Partial classification is transitional only — never steady state, never an architectural endpoint. A partial surface MUST measure overlay-completeness divergence and MUST NOT be promoted to authority while partial.

### 7.3 Subject-only surfaces

Surfaces whose output is **invariant** with respect to viewer identity (the same bytes are returned to every authorized caller).

Members:
- write-model administrative reconciliation reads (system-context only),
- raw event ingestion observability,
- internal projection rebuild diagnostics.

Subject-only surfaces are not user-facing. ViewerContext, when present, must still be carried for audit; it is never used to gate the response.

### 7.4 Forbidden surfaces

Surfaces where ViewerContext **must not be introduced** because doing so would create a hidden viewer-relative authority where none belongs:

- pricing token construction,
- ledger mutation,
- gateway/payment authority writes,
- inventory mutation,
- ranking pure-ordering computation.

Introducing ViewerContext as an authority input on these surfaces is forbidden under the Convergence Constitution.

---

## 8. Forbidden Patterns

Each forbidden pattern below is durable. Violations must be logged and reverted; they are not "tech debt" — they are doctrine violations.

### 8.1 Nil viewer fallback

Forbidden:
- defaulting a missing viewer to `nil`,
- defaulting a missing viewer to a synthesized "system" or "service" actor,
- defaulting a missing viewer to "anonymous" implicitly (i.e., without constructing an explicit AnonymousViewer),
- treating a missing viewer as ALLOW.

Required: explicit AnonymousViewer or DENY at the boundary.

### 8.2 Internal evaluator DB reads

Forbidden:
- evaluator querying users, account_status, relationships, or moderation tables internally,
- evaluator hydrating overlays from inside the decision path,
- evaluator "patching" a partial ViewerContext.

Required: all overlays hydrated by the caller before evaluator entry; UNKNOWN (shadow) or DENY (production) on missing input.

### 8.3 ViewerContext mutation

Forbidden:
- downstream layers writing into ViewerContext fields,
- evaluator-emitted decisions reflected back into ViewerContext,
- post-hoc overlay attachment after evaluator entry.

Required: ViewerContext is immutable after construction-boundary handoff.

### 8.4 Outbox snapshot trust

Forbidden:
- broadcasting using a ViewerContext reconstructed from outbox event payload,
- delivering notifications using enqueue-time ViewerContext or enqueue-time evaluator decision as authority,
- replaying historical events using historical ViewerContext as governance authority.

Required: per-recipient, delivery-time ViewerContext built from current authority state (Patterns D and E).

### 8.5 Ad-hoc ViewerContext creation

Forbidden:
- repositories constructing ViewerContext mid-query,
- evaluator-adjacent helpers synthesizing ViewerContext from partial data,
- card builders or response serializers constructing ViewerContext to satisfy a missing input.

Required: construction at the canonical boundaries enumerated in Section 5.1, and only there.

---

## 9. ADR References

- **ADR-003 — Governance Evaluator.** This contract is the input-side complement to ADR-003. ADR-003 mandates evaluator as final visibility authority; this contract mandates the only legal shape of viewer input the evaluator may receive.
- **ADR-005 — Realtime Signal Not Authority.** Patterns C, D, and E operationalize ADR-005's delivery-time-governance principle. ADR-005 forbids realtime authority; this contract forbids stale-snapshot ViewerContext, which is the mechanism by which realtime would otherwise become a hidden authority.

---

## 10. Contract Status

This contract is materialized doctrine, not implementation guidance.

It defines:
- the only legal shape of viewer input to evaluator and exposure surfaces,
- the only legal lifecycle of that input,
- the only legal propagation patterns,
- the only legal classification of surfaces against that input,
- the durable forbidden patterns that surround it.

It does not define runtime types, DTO field names, handler signatures, evaluator implementation, card-builder implementation, or websocket transport implementation.

Implementation that cannot cite this contract for affected surfaces must not proceed.

---

## 11. Lock Status and Operator Decisions

This contract is companion to `docs/contracts/governance-constitution.md`. The constitution carries the canonical lock for surface classification, UNKNOWN policy per surface class, the forbidden-pattern registry, the transitional-debt registry, the canonical governance checklist, the mute activation doctrine, and the implementation sequence. When this contract and the constitution overlap, the constitution is authoritative.

### 11.1 Transitional types

The transitional `evaluator.ViewerContextShadow` type is **TRANSITIONAL DEBT**. It exists today on `/feed` and `/contents/:id` shadow paths and on the `/feed` enforce code path. Its presence on the `/feed` enforce path violates §8.2 (internal evaluator DB reads) and §8.3 (mutation), and is FORBIDDEN under the constitution; the enforce flip is deferred until `/feed` is rebuilt on the canonical pattern. The type MUST NOT be extended to new surfaces. Retirement is scheduled by constitution §9 (implementation sequence) batches C1 and D1.

### 11.2 UNKNOWN policy reference

Per-surface-class UNKNOWN policy is locked in constitution §5. Content-detail surfaces (and all detail surfaces) are **fail-CLOSED**. Discovery surfaces are fail-OPEN on overlay-missing, fail-CLOSED on input-invalid. WebSocket subscribe and broadcast are fail-CLOSED. Push delivery is fail-CLOSED. Mutations and self-only surfaces are N/A.

### 11.3 Mute decision reference

Mute is canonical (this contract §4.4). Canonical consumers and rollout doctrine are locked in constitution §6.

### 11.4 Forbidden runtime patterns

The eighteen forbidden patterns (F1–F18) enumerated in constitution §3 supersede any informal allowance elsewhere in the codebase. Existing violations are bounded by the transitional-debt registry (constitution §4); no new violations are permitted.
