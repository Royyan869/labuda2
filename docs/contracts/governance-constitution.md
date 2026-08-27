# Governance Constitution

**Status:** CANONICAL
**Scope:** governance, evaluator, ViewerContext, visibility, exposure, realtime, notification delivery
**Authority:** This constitution is authoritative. When runtime disagrees with this constitution, runtime is TRANSITIONAL DEBT or FORBIDDEN — never doctrine.

Related Documents:
- docs/contracts/viewer-context.md (ViewerContext Contract — input-side companion)
- docs/contracts/content-detail-visibility-doctrine.md
- docs/contracts/public-card-boundary.md
- docs/foundation.md
- docs/architecture.md
- docs/adr/003-governance-evaluator.md
- docs/adr/005-realtime-signal-not-authority.md

---

## 1. Executive Verdict

The following are locked. Implementation MUST NOT reopen them.

1. **Pure evaluator is canonical.** The evaluator decides; it does not fetch.
2. **One ViewerContext is canonical.** The single canonical type is `viewercontext.ViewerContext` — immutable, constructor-only, no exported mutable fields.
3. **Caller hydrates truth.** Every overlay is an input. The caller assembles overlays at the construction boundary.
4. **Evaluator evaluates truth.** The evaluator reads inputs and emits decisions. It performs no IO.
5. **No evaluator IO.** The evaluator package MUST NOT hold a DB pool. MUST NOT contain SQL. MUST NOT launch IO-performing goroutines.
6. **`/search/content` is the canonical pattern.** Handler-package construction boundary; caller-batched overlays; pure evaluator.
7. **`/feed` and `/contents/:id` are TRANSITIONAL DEBT.** Their current evaluator-package hydration violates this constitution and MUST be rebuilt on the canonical pattern before further authority promotion.
8. **Current WebSocket topology is FORBIDDEN long-term.** Subscribe without governance, frozen identity, and broadcast without per-subscriber filter are durable violations.
9. **Push delivery asymmetry is FORBIDDEN.** Push MUST consume the same governance gate as in-app delivery.
10. **`Actor` is middleware-layer input only.** Actor MUST NOT be the visibility authority for cross-actor decisions; it feeds ViewerContext construction.
11. **Raw `gin` userID is not visibility authority.** On visibility surfaces, the handler MUST construct ViewerContext (or reject 401); `c.Get("userID")` MUST NOT be the visibility authority.

---

## 2. Final Target Topology

### 2.1 HTTP — visibility / exposure surfaces

```
HTTP request
  -> middleware chain: error handler -> auth -> user lookup -> roles -> ActorContextInject
  -> per-route hard gates: RequireCapability / RequireSeller / RequireTransactionAuthority / ...
  -> HANDLER BOUNDARY (single construction site per request)
       -> ViewerContext construction
            viewercontext.NewAnonymous(...)   for anonymous surfaces
            viewercontext.NewAuthenticated(...) otherwise
       -> Caller-batched overlay hydration via the request tx
            viewer lifecycle
            viewer x target relationship (block, mute)
            target lifecycle
            target moderation
            capability (from Actor)
       -> Pure evaluator call
            evaluator.Evaluate*(viewerContext, targetContext, candidates) -> decisions
       -> Decision application
            DROP   on DENY
            coarsen TOMBSTONE -> lifecycle = "removed"
            coarsen REDACT    -> lifecycle = "unavailable"
            UNKNOWN per surface class policy (see §5)
       -> publiccard serialization
            Lifecycle enum populated on the wire
  -> response
```

### 2.2 WebSocket — realtime governance

```
WS upgrade
  -> session ViewerContext constructed once (identity + capability + lifecycle)
SUBSCRIBE message
  -> MANDATORY subscribe authorizer call
       evaluator.EvaluateSubscribe(session vc, room target) -> ALLOW / DENY
BROADCAST fanout
  -> MANDATORY per-subscriber filter
       for each subscriber:
         evaluator.EvaluateBroadcast(vc, event target) -> ALLOW / DROP / COARSEN
SESSION REFRESH / EVICTION
  -> token TTL boundary forces re-upgrade
  -> governance event (user banned / deleted / block changed / mute changed) -> evict affected sessions
  -> periodic time-bound refresh for long-lived sessions
```

Interim relaxation: minimal-envelope fanout (`room_id` + `event_id` only) with REST re-fetch as the gate is TRANSITIONAL ACCEPTED while the WebSocket batch is pending. Envelope expansion immediately withdraws this relaxation.

### 2.3 Worker / notification / replay

```
outbox event consumed
  -> per-recipient ViewerContext constructed from CURRENT authority state
       outbox payload supplies identity references only
       outbox payload MUST NOT be trusted as current governance truth
  -> caller-batched overlay hydration
  -> pure evaluator call
       evaluator.EvaluateNotificationDelivery(vc, target) -> DELIVER / DROP / COARSEN
  -> delivery action
       in-app insert AND/OR push send
       both gated by the same decision
```

Read-only verification workers (reconciliation, integrity verifier) that publish nothing user-visible do not require ViewerContext.

---

## 3. Forbidden Pattern Registry

Each item is a durable doctrine rule. Status applies to all future implementation; existing violations are bounded by §4.

| ID | Pattern | Status |
|----|---------|--------|
| F1  | Evaluator package owns `*pgxpool.Pool` (or any DB pool) field | FORBIDDEN |
| F2  | Evaluator package contains SQL strings or executes `Query` / `QueryRow` | FORBIDDEN |
| F3  | Evaluator package hydrates overlays internally | FORBIDDEN |
| F4  | ViewerContext type has exported mutable fields or exported shared collections | FORBIDDEN |
| F5  | `ViewerContextShadow` (or any shadow-only type) used on a synchronous authority / enforce path | FORBIDDEN |
| F6  | Nil userID treated as anonymous without explicit `NewAnonymous(...)` construction | FORBIDDEN |
| F7  | Two ViewerContext construction boundaries within a single request | FORBIDDEN |
| F8  | Raw `gin.Context` userID consumed as visibility authority on visibility surfaces | FORBIDDEN |
| F9  | `*Actor` consumed as visibility authority for cross-actor decisions | FORBIDDEN |
| F10 | Ad-hoc ViewerContext construction via struct literal outside its package | FORBIDDEN |
| F11 | WebSocket `Subscribe` writing room membership without a governance authorizer call | FORBIDDEN |
| F12 | WebSocket session retaining frozen `UserID` for lifetime, with no governance refresh or eviction | FORBIDDEN |
| F13 | WebSocket broadcast carrying expanded payload (beyond minimal envelope) without per-subscriber filter | FORBIDDEN |
| F14 | Push delivery skipping the governance gate that in-app delivery applies | FORBIDDEN |
| F15 | Outbox payload state trusted as current governance truth | FORBIDDEN |
| F16 | `publiccard.*Card.Lifecycle` unable to render coarsened lifecycle on the wire | TRANSITIONAL DEBT |
| F17 | Mute stored via API and never enforced on any read surface | TRANSITIONAL DEBT |
| F18 | A type doctrinally marked "transitional" or "shadow-only" promoted to runtime authority | FORBIDDEN |

---

## 4. Transitional Debt Registry

The violations below exist today. Each is bounded by the implementation sequence in §9.

| ID | Debt | Resolution batch |
|----|------|------------------|
| D1  | `/feed` evaluator hydration inside the evaluator package | C1 |
| D2  | `/feed` enforce path consuming `ViewerContextShadow` | C1 |
| D3  | `/contents/:id` shadow runner hydration inside the evaluator package | D1 |
| D4  | `evaluator.ViewerContextShadow` exported mutable fields | C1 + D1 |
| D5  | `publiccard.UserCard.Lifecycle` (and sibling cards) reserved-but-nil on the wire | B5 |
| D6  | WebSocket `RoomAuthorizer` defined but unwired (dead doctrine) | WS-batch |
| D7  | WebSocket frozen `UserID` for connection lifetime | WS-batch |
| D8  | Notification push delivery skips block + lifecycle gate | Worker-batch |
| D9  | `user_mutes` CRUD without read-side enforcement on canonical consumers | Per-surface shadow rollout (§6) |
| D10 | SQL-only visibility on profile, listing detail, auction detail, auction bids, comments list, notifications list, chat messages, saved-items, likes, follow lists | Surface-by-surface batches |
| D11 | Handler-owned ad-hoc visibility predicates | Surface-by-surface batches |
| D12 | Ad-hoc DTOs on profile, follower / following / blocks / mutes lists, auction bids, saved-items | Surface-by-surface batches |
| D13 | Raw UUID-array relationship list responses | Surface-by-surface batches |
| D14 | Auction detail / bids leak (no visibility gate; bidder PII enumerable by auction ID) | Surface batch |
| D15 | Listing detail seller lifecycle gap (`GetByID` lacks the filter that `GetPublic` applies) | Surface batch |
| D16 | Comments list missing `user_blocks` JOIN and author lifecycle JOIN | Surface batch |
| D17 | Saved-items LEFT JOIN tombstone-as-NULL semantics; unpaginated | After B5 |
| D18 | Stale doctrine citations in Go docstrings (`docs/03-architecture/...`, `docs/05-rollout/...`, uppercase `FOUNDATION.md` / `DOCTRINE.md` / `ADR.md`) | B4.1 |

---

## 5. Surface Classification + UNKNOWN Policy

Every surface MUST declare its class. The class fixes the UNKNOWN policy.

| Surface class | UNKNOWN policy | Evaluator long-term | publiccard long-term | ViewerContext long-term |
|---|---|---|---|---|
| Discovery (feed, search content, search auctions, search listings, listing discovery, auction discovery, promotion discovery) | fail-OPEN on overlay-missing; fail-CLOSED on input-invalid | YES | YES | YES |
| Detail (content detail, listing detail, auction detail, profile detail) | **fail-CLOSED** on overlay-missing; **fail-CLOSED** on input-invalid | YES | YES | YES |
| Comments list | fail-OPEN on overlay-missing; fail-CLOSED on input-invalid | YES | YES | YES |
| Notifications list (in-app) | **fail-CLOSED** | YES | YES | YES |
| Notification push delivery | **fail-CLOSED** | YES | N/A (no card on wire) | YES |
| WebSocket subscribe | **fail-CLOSED (DENY)** | YES | N/A | YES |
| WebSocket broadcast | **fail-CLOSED (DROP per subscriber)** | YES | varies by event | YES |
| Chat messages list | fail-CLOSED on relationship overlay; fail-OPEN on participant overlay | YES | YES | YES |
| Chat send (mutation) | fail-CLOSED on relationship overlay | YES | N/A | YES |
| Saved items list | fail-CLOSED | YES | YES (after B5) | YES |
| Profile detail | fail-OPEN on target overlay; fail-CLOSED on viewer x target relationship overlay | YES | YES | YES |
| Profile follower / following / blocks / mutes lists | fail-CLOSED | YES | YES (after D12 resolved) | YES |
| Ratings on user | fail-OPEN on seller lifecycle; fail-CLOSED on rating-validity | YES | YES | YES |
| Admin surfaces (`/api/v1/admin/*`) | N/A — evaluator not required by default | NO | NO | NO |
| Self-only surfaces (`/users/me`, `/saved-items/check`, `/bidding`) | N/A — self-scope is the authority | NO | N/A | NO (transitional accepted: implicit identity sufficient) |
| Mutation-only surfaces (POST / PATCH / DELETE on self data) | N/A — capability gate is the authority | NO directly | N/A | NO (transitional accepted) |
| Webhook / backend-to-backend | N/A — out of governance | NO | NO | NO |

**Owner decision (locked):** content-detail UNKNOWN policy = **fail-CLOSED**.

---

## 6. Mute Doctrine

**Status:** Mute is a CANONICAL overlay.

**Current state:** TRANSITIONAL DEBT. `user_mutes` is written via API. No read surface enforces it.

**Canonical consumers (locked):**

- feed
- comments list
- notifications list (in-app)
- chat messages

**Rollout doctrine:**

- Mute consumption MUST be added per-surface, **shadow-first**.
- Each surface MUST emit shadow telemetry for mute-driven divergence before promoting to enforce.
- Global mute enforcement (flipping all four consumers in a single step) is FORBIDDEN.
- Each surface requires a surface-specific audit before flipping to enforce.

**Hydration:** Mute is a relationship overlay attached at the same construction boundary as block. The evaluator reads it as input; it MUST NOT re-query `user_mutes`.

---

## 7. Canonical Governance Checklist

Any future surface adopting the evaluator MUST answer each item with PROVEN, PARTIALLY PROVEN, or N/A. ASSUMED is not acceptable.

1. Exactly one construction boundary per request.
2. Canonical `viewercontext.ViewerContext` used (NOT `evaluator.ViewerContextShadow`).
3. AnonymousViewer policy explicitly declared (allowed / forbidden-with-401 / N/A).
4. All overlays caller-hydrated at the handler boundary via the request `tx`.
5. Evaluator performs zero IO and launches zero IO goroutines.
6. Evaluator package holds zero pool fields and contains zero SQL.
7. UNKNOWN policy for the surface class declared per §5.
8. publiccard can render ALLOW / DENY / TOMBSTONE / REDACT on the wire (or surface is DROP-only and that is explicit).
9. Lifecycle coarsening uses `viewercontext.CoarsenLifecycle` only.
10. Admin / self override explicit; any required capability registered.
11. Overlay ownership explicit per overlay (block, mute, follow, lifecycle, moderation, capability).
12. Pagination authority explicit; preserved across evaluator (further-restrict only).
13. Shadow / enforce boundary explicit (env var, mode constant, safe-default normalizer).
14. Rollback path explicit (single-knob flip back to shadow).
15. Divergence telemetry observable; bounded label cardinality (no user-ID, no decision-string labels).
16. Mobile wire compatibility confirmed (existing fields preserved; new fields additive).
17. WebSocket parity declared (required for this surface OR explicitly out of scope).
18. Worker / replay parity declared (required for this surface OR explicitly out of scope).
19. All §3 forbidden patterns absent (or inherited debt cited with batch slot).
20. All §4 transitional debts inherited by this surface declared with batch slot.

---

## 8. Anti-Drift Guardrail Candidates

This constitution does not introduce guardrails. The candidates below are documented for future phasing.

### 8.1 Safe to introduce immediately (no current violations)

- **G2** — Evaluator package MUST NOT import `database/sql`. Detection: import scan. Phase: CI lint.
- **G5** — `evaluator.ViewerContextShadow` and `evaluator.TargetContextShadow` MUST NOT be imported outside `backend/internal/governance/evaluator/*`. Detection: cross-package import scan. Phase: CI lint.
- **G15** — `*Actor` fields MUST NOT be assigned outside `backend/internal/middleware/actor_context.go` and Actor constructors. Detection: static analysis. Phase: CI lint.

### 8.2 Deferred until rebuild

- Evaluator package MUST NOT import `pgxpool` — deferred until C1 + D1 eliminate F1 violations.
- Evaluator package MUST NOT contain SQL keyword string literals — deferred until C1 + D1.
- `Enforce*` functions in the evaluator package MUST NOT accept `*ViewerContextShadow` — deferred until C1.
- WebSocket `Hub.Subscribe` MUST be preceded by an authorizer call — deferred until WS-batch.
- Push delivery MUST consume the canonical evaluator gate — deferred until Worker-batch.
- `publiccard.*Card.Lifecycle` MUST have a canonical setter / constructor path — deferred until B5.
- `docs/03-architecture/...` MUST NOT appear in Go docstrings — deferred until B4.1.
- `docs/05-rollout/...` Go references MUST use the `docs/archive/05-rollout/` prefix — deferred until B4.1.

---

## 9. Implementation Sequence

The ordering below is doctrine. Skipping a step or running steps in parallel violates the lock. Modification requires a future constitution amendment batch.

1. **B4** — Doc lock (this batch).
2. **B4.1** — Citation cleanup. Retarget stale doctrine citations in Go docstrings to canonical paths.
3. **B4.2** — Safe guardrails (G2, G5, G15) introduced.
4. **B5** — publiccard lifecycle audit. Resolve F16 / D5; define the canonical lifecycle enum wire shape and the construction path.
5. **B6** — `/search/content` enforce promotion. Single env-var flip on the canonical-pattern surface.
6. **C1** — `/feed` canonical rebuild. Extract hydration from the evaluator package; retire `ViewerContextShadow` use on `/feed`.
7. **C2** — `/feed` enforce promotion on the rebuilt canonical pattern.
8. **D1** — `/contents/:id` canonical rebuild.
9. **D2** — `/contents/:id` enforce promotion.
10. **WS governance batch** — subscribe authorizer wired; per-subscriber broadcast filter; session refresh / eviction; minimal-envelope relaxation withdrawn.
11. **Worker / notification batch** — worker constructs per-recipient ViewerContext; push delivery consumes the canonical gate; in-app / push asymmetry resolved.
12. **Surface-by-surface batches** — remaining transitional debts (D10–D17) drained in priority order.

---

## 10. Contract Status

This constitution is CANONICAL.

It defines:

- the only legal governance topology going forward,
- the only legal evaluator authority shape,
- the only legal ViewerContext construction pattern,
- the bounded list of acceptable transitional debts,
- the surface classification and UNKNOWN policy for every visibility surface,
- the mute overlay activation doctrine,
- the implementation sequence.

When runtime disagrees with this constitution, runtime is TRANSITIONAL DEBT or FORBIDDEN — not doctrine.

Implementation that cannot cite this constitution for affected surfaces MUST NOT proceed.
