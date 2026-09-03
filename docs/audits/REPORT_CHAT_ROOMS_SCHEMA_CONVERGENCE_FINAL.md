# VERDICT

**PASS**

## 1. Current Canonical Chat Room Schema

Live schema authority (verified against PostgreSQL `labuda_test` after the full
migration chain 000001→000062):

```
chat_rooms (
  id              uuid PRIMARY KEY,
  room_type       chat_room_type_enum NOT NULL,
  participant_a   uuid NOT NULL,
  participant_b   uuid NOT NULL,
  linked_order_id uuid NULL REFERENCES orders(id) ON DELETE SET NULL,
  created_at      timestamptz NOT NULL,
  updated_at      timestamptz NOT NULL,
  last_message_at timestamptz NOT NULL
)
```

- **`context_json` / `context_set_by` DO NOT EXIST** in the live schema.
- **`chat_commerce_references` table DOES NOT EXIST** (dropped by 000054).
- The current chat commerce/resource reference authority is **message-level**:
  `chat_messages.attachment_json` + `chat_message_resource_occurrences`
  (migration 000034, with 000047 vocabulary convergence `for_sale_source_id`).
- The only room-level commerce continuity signal is `linked_order_id`.

Entity authority: `internal/interaction/chat/entity/chat_room.go` — the `ChatRoom`
entity now carries identity + `linked_order_id` only. No `ContextJSON`,
`ContextSetBy`, `HasContext`, `SetContext`, or `NewChatRoomWithContext`.

## 2. Root Cause

The Go chat layer was left **half-migrated** against the schema:

1. Migration `000030` (`chat_room_context_backfill_and_purge`) backfilled legacy
   room context into `chat_commerce_references`, then **dropped**
   `chat_rooms.context_json` and `chat_rooms.context_set_by`.
2. Migration `000054` then **dropped `chat_commerce_references`** entirely —
   but its evidence comment incorrectly claimed "the chat domain stores commerce
   context in `chat_rooms.context_json`", contradicting 000030.
3. The backend Go chat layer (entity, repository, service, HTTP handler, outbox
   projection, realtime envelope, support adapter) was **never converged** to the
   removed columns. Every `ChatRoom` read/write referenced
   `context_json`/`context_set_by`, so:
   - `ChatRepositoryImpl.CreateRoom` failed at runtime:
     `column "context_json" of relation "chat_rooms" does not exist (SQLSTATE 42703)`.
   - This blocked the shipping-quote race/context-separation test path that seeds
     rooms via the chat repository, exactly as recorded in the Auction Settlement
     Phase 1 reports.

## 3. Removed/Canonical Authority

The old room-level `context_json` mechanism was a transient UI-preview hint on the
room. Its canonical replacement already existed and was **not invented here**:

- Commerce/resource references shared into chat are persisted **per message** in
  `chat_message_resource_occurrences` (typed immutable occurrence + server-built
  `fallback_snapshot`, migration 000034) and carried on the message wire as
  `attachment_json`.
- Order ↔ chat continuity is carried by `chat_rooms.linked_order_id`
  (order linkage — the canonical commerce-continuity signal used by block
  enforcement, `HasOrderContext`, room-by-order lookup).
- Support-ticket linkage is carried by `support_tickets.chat_room_id` and the
  ticket's own `linked_order_id`, not by room JSON.

The room-level context was **removed entirely** (no replacement invented, no
`context_json_v2`, no compatibility aliases, no fallback reads/writes).

## 4. Production Consumers

| File | Change |
|---|---|
| `internal/interaction/chat/entity/chat_room.go` | Removed `ContextJSON`, `ContextSetBy`, `HasContext()`, `SetContext()`, `NewChatRoomWithContext()`; kept `LinkedOrderID`/`HasOrderContext`/`LinkOrder`/`UnlinkOrder`; doc comments updated |
| `internal/interaction/chat/repository/chat_repository.go` | Removed `UpdateRoomContext` from the `Repository` interface |
| `internal/interaction/chat/infrastructure/repository/chat_repository_impl.go` | `CreateRoom` INSERT uses canonical columns only (8 cols); all room SELECTs (`GetRoomByID`, `GetRoomByIDForUpdate`, `GetDirectRoom`, `GetSupportRoom`, `ListRoomsByUser`, `GetRoomByOrderID`) drop `context_json`/`context_set_by`; `UpdateRoomContext` removed |
| `internal/interaction/chat/application/chat_service.go` | `GetOrCreateDirectRoom`/`getOrCreateDirectRoomTx` drop `contextJSON`/`contextSetBy` params and context branches; `GetOrCreateSupportRoomWithContext` removed; `getOrCreateSupportRoomTx` drops the context param/branches; `AutoLinkOrderToDirectRoom` updated; block-enforcement comments corrected |
| `internal/interaction/chat/application/chat_room_event_projection.go` | `buildChatRoomSummaryOutboxPayload` no longer emits `context`/`context_set_by` |
| `internal/interaction/chat/delivery/http/chat_handler.go` | `CreateDirectRoomRequest` removed (dead); `GetOrCreateDirectRoom` no longer accepts/parses a `context` body; `roomToResponse` no longer emits `context`/`context_set_by` |
| `internal/realtime/envelope.go` | `ChatRoomSummaryPayload` drops `Context` and `ContextSetBy` fields and their `toMap()` emission |
| `internal/governance/support/application/support_service.go` | `ChatService.CreateSupportTicketRoom` drops `ticketID`/`contextJSON` params; `CreateTicket` no longer marshals support-ticket room context (ticket carries `chat_room_id` + `linked_order_id` canonically) |
| `internal/serverboot/dependencies.go` | `supportChatServiceAdapter` drops deprecated `GetOrCreateSupportRoomWithContext`; `CreateSupportTicketRoom` now delegates to `GetOrCreateSupportRoom`; removed now-unused `encoding/json` import |

## 5. Test/Fixture Consumers

| File | Change |
|---|---|
| `internal/realtime/envelope_test.go` | `TestMarshalChatRoomCreated_CanonicalEnvelope` drops `Context`/`ContextSetBy` setup and `context_set_by`/`context` assertions |
| `internal/interaction/chat/application/chat_service_room_created_producer_test.go` | `GetOrCreateDirectRoom` calls updated to 2-arg signature |
| `internal/interaction/chat/application/chat_service_room_updated_producer_test.go` | `roomUpdatedMockRepo.UpdateRoomContext` removed; test room no longer sets `ContextJSON`/`ContextSetBy`; `context_set_by` outbox assertion removed; `linked_order_id` assertion retained |
| `internal/interaction/chat/consumer/negotiation_event_handler_chatroom_test.go` | `negotiationFakeChatRepo.UpdateRoomContext` stub removed |
| `internal/interaction/chat/delivery/http/chat_link_order_authorization_test.go` | `linkOrderFakeRepo.UpdateRoomContext` stub removed |
| `internal/governance/support/application/support_service_test.go` | `mockChatService.CreateSupportTicketRoom` signature updated (dropped `ticketID`/`contextJSON`) |
| `internal/governance/support/delivery/http/support_handler_ownership_test.go` | `ownershipMockChatService.CreateSupportTicketRoom` signature updated |
| `internal/governance/support/delivery/http/support_handler_capability_test.go` | `mockChatService.CreateSupportTicketRoom` signature updated |

No chat fixtures/helpers seed the removed columns; the two chat integration
helpers that INSERT into `chat_rooms` already used the canonical column set and
were left unchanged.

## 6. Files Changed

Production:
1. `backend/internal/interaction/chat/entity/chat_room.go`
2. `backend/internal/interaction/chat/repository/chat_repository.go`
3. `backend/internal/interaction/chat/infrastructure/repository/chat_repository_impl.go`
4. `backend/internal/interaction/chat/application/chat_service.go`
5. `backend/internal/interaction/chat/application/chat_room_event_projection.go`
6. `backend/internal/interaction/chat/delivery/http/chat_handler.go`
7. `backend/internal/realtime/envelope.go`
8. `backend/internal/governance/support/application/support_service.go`
9. `backend/internal/serverboot/dependencies.go`

Tests:
10. `backend/internal/realtime/envelope_test.go`
11. `backend/internal/interaction/chat/application/chat_service_room_created_producer_test.go`
12. `backend/internal/interaction/chat/application/chat_service_room_updated_producer_test.go`
13. `backend/internal/interaction/chat/consumer/negotiation_event_handler_chatroom_test.go`
14. `backend/internal/interaction/chat/delivery/http/chat_link_order_authorization_test.go`
15. `backend/internal/governance/support/application/support_service_test.go`
16. `backend/internal/governance/support/delivery/http/support_handler_ownership_test.go`
17. `backend/internal/governance/support/delivery/http/support_handler_capability_test.go`

## 7. Verification

### Normal Go tests (no integration tag)

| Command | Result |
|---|---|
| `go test -count=1 ./internal/interaction/chat/...` | PASS |
| `go test -count=1 ./internal/realtime/...` | PASS |
| `go test -count=1 ./internal/governance/support/...` | PASS |
| `go test -count=1 ./internal/commerce/shipping/quote/...` | PASS |

### Integration compile

| Command | Result |
|---|---|
| `go test -tags integration -count=1 -run '^$' ./internal/interaction/chat/...` | PASS |
| `go test -tags integration -count=1 -run '^$' ./internal/realtime/...` | PASS |
| `go test -tags integration -count=1 -run '^$' ./internal/governance/support/...` | **BLOCKED** — pre-existing `supportApp.NewService` arity drift in `support_handler_capability_test.go:284` (missing `DisputeService`); out of scope (see §8) |
| `go test -tags integration -count=1 -run '^$' ./internal/commerce/shipping/quote/...` | PASS |
| `go test -tags integration -count=1 -run '^$' ./internal/serverboot/...` | PASS |

### Integration runtime (real PostgreSQL)

| Command | Result |
|---|---|
| `go test -tags integration -run '^TestShippingQuote_ContextSeparation_AllowsDistinctCanonicalContexts$' ./internal/commerce/shipping/quote/infrastructure/repository/` | **PASS** (previously BLOCKED by `context_json does not exist`) |
| `go test -tags integration -run '^TestShippingQuote_CreateConcurrentReplacement_SupersedesPriorRevision$' ./internal/commerce/shipping/quote/infrastructure/repository/` | PASS |
| `go test -tags integration -run '^TestShippingQuote_ReactivationVsReplacement_ReactivationFailsClosedAfterReplacement$' ./internal/commerce/shipping/quote/infrastructure/repository/` | PASS |
| `go test -tags integration -run '^TestShippingQuote_DuplicateReactivation_SecondAttemptNoopsWithoutDoubleIncrement$' ./internal/commerce/shipping/quote/infrastructure/repository/` | PASS |
| `go test -tags integration -run '^TestShippingQuote_ReuseCap_RejectsQuoteAtLimit$' ./internal/commerce/shipping/quote/infrastructure/repository/` | PASS |
| Chat DB runtime tests (`chat_unread_runtime_closure_integration_test.go`) | **BLOCKED** — pre-existing `chat_messages.command_fingerprint` NOT NULL drift (migration 000033); unrelated to room context (see §8) |

### Build / vet

| Command | Result |
|---|---|
| `go build ./...` | PASS |
| `go vet ./...` | PASS |

## 8. Remaining Blockers

Only blockers actually demonstrated, with evidence and scope boundary:

1. **`chat_messages.command_fingerprint` NOT NULL drift (pre-existing).**
   Migration 000032/000033 added a mandatory non-empty `command_fingerprint`
   (server-computed SHA-256) to `chat_messages`, but the Go `ChatMessage` entity
   and `ChatRepositoryImpl.CreateMessage` were never updated to compute/write it.
   Runtime: `null value in column "command_fingerprint" of relation
   "chat_messages" violates not-null constraint (23502)`. This blocks the
   chat-unread/moderation Postgres-backed runtime tests. It is a **message-level**
   fingerprint drift — not the room-context drift this task owns — and was present
   before this task (verified: `chat_message.go` untouched by this task; the
   message INSERT was not modified). Scope boundary: chat message idempotency /
   actor-scoping hardening is outside "cleanup chat_rooms schema/consumer drift".
   Evidence: `migrations/000032`, `migrations/000033`, failing trace in
   `chat_unread_runtime_closure_integration_test.go`.

2. **`supportApp.NewService` arity drift (pre-existing, integration-tagged).**
   `internal/governance/support/delivery/http/support_handler_capability_test.go:284`
   calls `NewService` with 6 args; the canonical signature needs 7 (missing
   `DisputeService`). Blocks `go test -tags integration` on that one package.
   Present before this task (this task changed the `CreateSupportTicketRoom`
   signature in that file but not the `NewService` call); named in the scope
   firewall ("support service arity drift"). Evidence: compile trace above.

3. **Shared `labuda_test` DB concurrency race (pre-existing).**
   A full `go test ./...` run races multiple DB-backed suites against one test
   database, producing transient `relation "users" does not exist` /
   `deadlock detected` failures and unrelated nil-pointer panics in unit tests
   that construct services with nil DBs. Known class: "presence DB test harness
   race" (out of scope). Chat/realtime/support unit packages pass in isolation.

4. Mobile app still sends/reads room-level `context`/`contextSetBy`
   (`apps/mobile/lib/domains/chat/...`). Out of scope per the task firewall.
   A separate mobile-convergence task must remove the room-context fields from
   the mobile DTO/entity/mapper/notifier surface, since the backend no longer
   stores or serves them.

## 9. Residue Audit

Global searches over the whole repository after implementation:

| Term | Live Go (backend) | Remaining references |
|---|---|---|
| `ContextJSON` | 0 | — |
| `ContextSetBy` | 0 | — |
| `context_json` | 0 | Migrations 000001/000030 (historical schema + backfill); migration 000054 comment (historical); audit docs |
| `context_set_by` | 0 | Migrations 000001/000030/000054 (historical); mobile tests (out of scope); audit docs |
| `NewChatRoomWithContext` | 0 | — |
| `SetContext` | 0 | — |
| `UpdateRoomContext` | 0 | — |
| `HasContext` | 0 | — |
| `GetOrCreateSupportRoomWithContext` | 0 | — |
| `"context"` / `Context` on `ChatRoomSummaryPayload` | 0 | Stdlib `context.Context` params (false positive); `envelope_test.go` tombstone assertion that removed payloads must NOT carry `context` (canonical minimalism guarantee) |
| `ChatRoomSummaryPayload{...Context` | 0 | — |
| Room `context`/`context_set_by` on REST/outbox/realtime wire | 0 | — |

Classification of remaining references:

- **Historical migration reference**: 000001 (columns created), 000030 (backfill +
  drop), 000054 (drop of `chat_commerce_references`; its "context lives in
  chat_rooms.context_json" comment is factually stale and documents the drift that
  this task resolved). Migration history is intentionally not rewritten.
- **Legitimate canonical**: `context.Context` stdlib usage; the removed-envelope
  tombstone test asserting `context` absence.
- **Out-of-scope client residue**: mobile Dart chat DTO/entity/mapper/notifier
  still carry `contextSetBy`; mobile chat detail/input/card widgets still read
  room `context`. Recorded as blocker §8.4.

## 10. Final Assessment

- **Is `chat_rooms` now single-authority?**
  YES. The entity, repository, service, handler, outbox projection, realtime
  envelope, and support adapter all agree on the canonical room shape:
  identity + room_type + participants + `linked_order_id` + timestamps. Zero Go
  references to the removed columns exist.

- **Are all active consumers aligned?**
  YES. All production chat consumers and all chat/realtime/support unit tests
  compile and pass against the canonical room shape. The only compile gap is the
  out-of-scope `supportApp.NewService` integration-tagged drift (§8.2).

- **Are removed columns referenced anywhere in live code?**
  NO. Zero `context_json`/`context_set_by`/`ContextJSON`/`ContextSetBy` references
  remain in live Go. The chat repository `CreateRoom`/`SELECT`s now target the
  canonical columns, proven at runtime by the shipping-quote integration tests
  that seed rooms through the chat repository.

- **Can the previously blocked shipping-quote/chat verification now run?**
  YES for the shipping-quote path: the race/context-separation suite that was
  blocked by `chat_rooms.context_json does not exist` now passes against real
  PostgreSQL. The broader chat DB runtime suite remains blocked by the separate
  pre-existing `command_fingerprint` drift (§8.1), which is not a room-context
  issue.

- **Did this task avoid resurrecting obsolete architecture?**
  YES. No column was re-added, no migration was written to restore columns, no
  compatibility alias/v2/fallback was introduced, and the canonical message-level
  occurrence authority (000034) was left untouched. The deprecated
  `GetOrCreateSupportRoomWithContext` adapter path was removed rather than kept.

- **Is this blocker actually closed?**
  YES for the `chat_rooms.context_json`/`context_set_by` schema-consumer drift:
  the canonical schema is established, all live Go consumers are aligned, no
  residue remains, and the previously blocked shipping-quote verification path
  executes green. The distinct `chat_messages.command_fingerprint` drift and the
  support `NewService` integration-tagged drift are recorded as remaining
  out-of-scope blockers with evidence.
