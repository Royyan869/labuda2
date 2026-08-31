# REPORT: GOVERNANCE FINAL CLEANUP AUDIT — SLICE 9

## 1. Canonical Authority Map

| Domain | Canonical Table | Canonical Entity | Canonical Service | Canonical UI |
|---|---|---|---|---|
| Report | `reports` | `entity.Report` | `ReportService` | `ReportHandler` |
| Case | `cases` | `entity.CanonicalCase` | `CaseService` | `GovernanceCasesPage` |
| Decision | `decisions` | `entity.Decision` | `DecisionService` | `GovernanceCaseDetailPage` |
| Enforcement | `enforcements` | `entity.Enforcement` | `ModerationEventHandler` | (embedded in Decision) |
| Audit | `audit_events` | `auditentity.AuditEvent` | `AuditService` | `AuditTimeline` |

No competing authorities detected for any of these.

---

## 2. Legacy Implementation Inventory

### Backend Go Code

| Artifact | Location | Status |
|---|---|---|
| GovernanceCase entity | `entity/governance_case.go` | FUTURE DEPENDENCY (Appeal) |
| GovernanceCaseDecision type | `entity/governance_case.go` | FUTURE DEPENDENCY (Appeal) |
| ModerationRepository interface | `repository/moderation_repository.go` | FUTURE DEPENDENCY (Appeal) |
| ModerationRepositoryImpl | `repository/moderation_repository_impl.go` | DEAD/ZOMBIE (reads dropped table) |
| DomainAction entity | `entity/domain_action.go` | DEAD/ZOMBIE (no migration, parked) |
| DomainActionRepository | `repository/domain_action_repository.go` | DEAD/ZOMBIE (parked) |
| DomainActionRepositoryImpl | `repository/domain_action_repository_impl.go` | DEAD/ZOMBIE (parked) |
| DomainActionWorker | `worker/domain_action_worker.go` | DEAD/ZOMBIE (never instantiated) |
| domain_action_test.go | `entity/domain_action_test.go` | DEAD/ZOMBIE (tests parked code) |
| moderation_action.go | `entity/moderation_action.go` | DEAD/ZOMBIE (ActionType enum for DomainAction) |
| AppealReversalService | `application/appeal_reversal_service.go` | DEAD/ZOMBIE (parked) |
| ResourceType enum | `entity/moderation_resource_type.go` | MIXED (chat_message=FUTURE, rest=DEAD) |
| ShouldEmitEnforcementEvents | `entity/governance_case.go` | DEAD/ZOMBIE |

### Frontend Code

| Artifact | Location | Status |
|---|---|---|
| ModerationCasesPage | `pages/ModerationCasesPage.tsx` | DEAD/ZOMBIE (not routed) |
| CaseDetailModal | `components/moderation/CaseDetailModal.tsx` | DEAD/ZOMBIE (only used by dead page) |
| CaseDetailModal.test.tsx | `components/moderation/CaseDetailModal.test.tsx` | DEAD/ZOMBIE (tests dead component) |
| useModeration | `hooks/useModeration.ts` | DEAD/ZOMBIE (only used by dead page) |
| useModeration.test.tsx | `hooks/useModeration.test.tsx` | DEAD/ZOMBIE (tests dead hook) |
| moderation.ts API | `lib/api/moderation.ts` | DEAD/ZOMBIE (only used by dead hook) |
| moderation.test.ts | `lib/api/moderation.test.ts` | DEAD/ZOMBIE (tests dead API) |
| types/moderation.ts | `types/moderation.ts` | DEAD/ZOMBIE (only used by dead hook) |

### Routes

| Route | Backend Status | Frontend Status |
|---|---|---|
| `GET /admin/moderation/cases` | REMOVED (Slice 2) | DEAD (ModerationCasesPage not routed) |
| `GET /admin/moderation/cases/:id` | REMOVED (Slice 2) | DEAD |
| `GET /admin/moderation/cases/:id/evidence` | REMOVED (Slice 2) | DEAD |
| `POST /admin/moderation/cases/:id/action` | REMOVED (Slice 2) | DEAD |
| `GET /admin/governance/cases` | ACTIVE | ACTIVE (GovernanceCasesPage) |
| `GET /admin/governance/cases/:id` | ACTIVE | ACTIVE (GovernanceCaseDetailPage) |
| `GET /admin/governance/cases/:id/audit` | ACTIVE | ACTIVE (AuditTimeline) |
| `POST /admin/governance/cases/:id/decisions` | ACTIVE | ACTIVE (Decision form) |

---

## 3. Dependency Graph

### GovernanceCase entity

```
GovernanceCase (entity/governance_case.go)
  ↓ imported by
  appeal_service.go (FUTURE DEPENDENCY — Appeal Slice 10)
  appeal_handler.go (FUTURE DEPENDENCY)
  appeal_service_test.go (FUTURE DEPENDENCY)
  appeal_handler_test.go (FUTURE DEPENDENCY)
  moderation_resource_type.go (contains ResourceType enum)
  ↓ NOT imported by
  decision_service.go (canonical — uses CanonicalCase)
  case_service.go (canonical — uses CanonicalCase)
  report_service.go (canonical — uses Report)
  governance_admin_handler.go (canonical)
```

**Classification:** FUTURE DEPENDENCY — Appeal Slice 10 requires GovernanceCase entity.

### ModerationRepository

```
ModerationRepository (interface)
  ↓ implemented by
  ModerationRepositoryImpl (reads dropped moderation_cases table)
  ↓ used by
  appeal_service.go (FUTURE DEPENDENCY)
  appeal_reversal_service.go (DEAD/ZOMBIE)
  dependencies.go (wired for compilation only)
  ↓ NOT used by
  canonical governance chain (DecisionService, CaseService, ReportService)
```

**Classification:** FUTURE DEPENDENCY — Appeal Slice 10 requires ModerationRepository interface. ModerationRepositoryImpl is DEAD/ZOMBIE (reads dropped table).

### DomainAction

```
DomainAction (entity/domain_action.go)
  ↓ used by
  DomainActionRepository (interface + impl) — PARKED
  DomainActionWorker — PARKED (never instantiated)
  domain_action_test.go — PARKED
  moderation_action.go (ActionType enum) — PARKED
  ↓ NOT used by
  any active code path
  any route
  any worker instantiation
  any migration
```

**Classification:** DEAD/ZOMBIE — complete parallel enforcement mechanism that was never activated. No migration exists for `domain_actions` table. Worker is explicitly documented as "PARKED: never instantiated".

### chat_message

```
ResourceTypeChatMessage (moderation_resource_type.go)
  ↓ used by
  GovernanceCase entity (FUTURE DEPENDENCY)
  ↓ handler exists
  ModerationEventHandler.handleChatMessageHidden() — WIRED but no producer
  ModerationEventHandler.handleChatMessageRestored() — WIRED but no producer
  ↓ NOT produced by
  any active code path (no code creates moderation.chat_message.hidden events)
```

**Classification:** FUTURE DEPENDENCY — chat moderation handler is wired but has no active producer. Could be re-enabled if chat moderation is decided as a future feature. The handler itself is canonical infrastructure.

### fixed_price_sale

```
"fixed_price_sale" in governance code
  ↓ found in
  report.go:24 (comment: "NOT canonical targets")
  report_service.go:55 (comment: "chat_message and fixed_price_sale are rejected")
  ↓ NOT found in
  any active code path
  any entity field value
  any migration enum
```

**Classification:** DEAD/ZOMBIE — only appears in comments documenting what is NOT canonical. No code symbol uses this value.

---

## 4. GovernanceCase Analysis

**Current state:** GovernanceCase entity exists in `entity/governance_case.go` with:
- `GovernanceCaseStatus` (pending/approved/rejected/enforced) — LEGACY vocabulary
- `GovernanceCaseDecision` (approve/reject/enforce) — LEGACY vocabulary
- `Enforce()` method — DEAD/ZOMBIE (reads dropped table)
- `ShouldEmitEnforcementEvents()` — DEAD/ZOMBIE
- Various status check methods — used by Appeal domain

**Active consumers:** Only Appeal domain files:
- `appeal_service.go` — checks `GovernanceCase.Status`
- `appeal_handler.go` — returns `GovernanceCase` to admin
- `appeal_service_test.go` — creates mock `GovernanceCase`
- `appeal_handler_test.go` — creates mock `GovernanceCase`

**Future consumers:** Appeal Slice 10 will need to be rebuilt against canonical `cases` table.

**Classification:** FUTURE DEPENDENCY — Appeal Slice 10. The entity will be REPLACED (not kept) when Appeal is rebuilt against canonical cases.

---

## 5. ModerationRepository Analysis

**Current state:** Interface in `repository/moderation_repository.go`, implementation in `repository/moderation_repository_impl.go`.

**ModerationRepositoryImpl.GetByID** reads from `moderation_cases` table which was DROPPED in migration 000056. This method ALWAYS FAILS at runtime.

**Active consumers:** Only Appeal domain:
- `appeal_service.go` — calls `moderationRepo.GetByID()`
- `appeal_reversal_service.go` — calls `moderationRepo.GetByGovernanceCaseID()`
- `dependencies.go` — wires `NewModerationRepository()` for compilation

**Classification:** FUTURE DEPENDENCY — Appeal Slice 10 needs the interface (will be rewritten to read from `cases`). ModerationRepositoryImpl is DEAD/ZOMBIE.

---

## 6. DomainAction Analysis

**Current state:**
- Entity: `entity/domain_action.go` — full ActionType enum, metadata, status fields
- Repository: `domain_action_repository.go` + `domain_action_repository_impl.go` — full CRUD
- Worker: `worker/domain_action_worker.go` — full poll/process/retry loop
- Tests: `entity/domain_action_test.go` — 8 tests
- wiring: `outbox_event_registry.go:198-203` — documented as "PARKED: never instantiated"

**Migration:** NONE — no `domain_actions` table was ever created.

**Active consumers:** ZERO — no code instantiates DomainActionWorker, no code creates DomainAction rows, no route exposes DomainAction.

**Classification:** DEAD/ZOMBIE — complete parallel enforcement mechanism that was abandoned before activation. Should be removed entirely.

---

## 7. chat_message Analysis

**Current state:**
- `ResourceTypeChatMessage` in `entity/moderation_resource_type.go`
- `ModerationEventHandler.handleChatMessageHidden()` — WIRED, processes events
- `ModerationEventHandler.handleChatMessageRestored()` — WIRED, processes events
- `ChatMessageModerationService` interface in `moderation_event_handler.go`

**Producer:** NONE — no code creates `moderation.chat_message.hidden` or `moderation.chat_message.restored` outbox events. The events would need to be produced by a report/decision flow targeting chat_message, but the canonical Report target vocabulary EXCLUDES chat_message.

**Classification:** FUTURE DEPENDENCY — the handler infrastructure is wired and functional, but has no active producer. Could be re-enabled if chat moderation is decided as a future feature.

---

## 8. fixed_price_sale Analysis

**All occurrences in governance code are COMMENTS only:**
- `report.go:24` — "chat_message and fixed_price_sale are NOT canonical targets"
- `report_service.go:55` — "chat_message and fixed_price_sale are rejected"

**In commerce code:** `fixed_price_sale` is the legitimate commerce entity name (not governance vocabulary).

**Classification:** DEAD/ZOMBIE in governance context. Comments should be updated to reference `for_sale` (the canonical term). Commerce usage is KEEP.

---

## 9. Legacy Admin UI Analysis

**ModerationCasesPage** (`pages/ModerationCasesPage.tsx`):
- NOT imported in `App.tsx` — zero routes
- Calls removed backend endpoints (`GET /admin/moderation/cases`)
- Uses legacy types from `types/moderation.ts`
- Classification: DEAD/ZOMBIE

**CaseDetailModal** (`components/moderation/CaseDetailModal.tsx`):
- Only imported by `ModerationCasesPage.tsx`
- Uses `useModeration` hook
- Classification: DEAD/ZOMBIE

**useModeration** (`hooks/useModeration.ts`):
- Only imported by `ModerationCasesPage.tsx` and `CaseDetailModal.tsx`
- Calls removed backend endpoints
- Classification: DEAD/ZOMBIE

**lib/api/moderation.ts**:
- Only imported by `useModeration.ts`
- Calls removed backend endpoints
- Classification: DEAD/ZOMBIE

**types/moderation.ts**:
- Only imported by `useModeration.ts` and `moderation.ts`
- Contains legacy types (ModerationCase, ModerationCaseDetail, etc.)
- Classification: DEAD/ZOMBIE

**Tests:**
- `CaseDetailModal.test.tsx` — tests dead component
- `useModeration.test.tsx` — tests dead hook
- `moderation.test.ts` — tests dead API
- Classification: DELETE WITH IMPLEMENTATION

---

## 10. Legacy Routes Analysis

**Backend:** All `/admin/moderation/cases/*` routes were REMOVED in Slice 2. No backend code serves these endpoints.

**Frontend:** `ModerationCasesPage` is NOT routed in `App.tsx`. The `/moderation/cases` route renders `GovernanceCasesPage` (canonical).

**Stale references:** `lib/api/moderation.ts` still references `/api/v1/admin/moderation/cases` URLs — but this file is DEAD (not imported by any active code).

**Classification:** DEAD/ZOMBIE — backend routes removed, frontend not routed.

---

## 11. admin_audit_logs Analysis

**Active consumers (NON-governance):**
- `audit/admin_audit_logger.go` — shared infrastructure
- `finance/delivery/http/admin_payout_handler.go` — LogSafe
- `finance/delivery/http/admin_finance_handler.go` — LogSafe
- `finance/refund/delivery/http/admin_refund_handler.go` — LogSafe
- `platform/admin/delivery/http/admin_handler.go` — LogSafe
- `platform/admin/infrastructure/repository/admin_repository_impl.go` — reads for dashboard
- `platform/capability/application/capability_service.go` — LogSafe
- `platform/config/delivery/http/platform_config_handler.go` — LogSafe
- `platform/alert/delivery/http/admin_alert_handler.go` — LogSafe
- `pricing/promotion/delivery/http/promotion_handler.go` — LogSafe
- `pricing/promotion/delivery/http/external_product_handler.go` — LogSafe
- `commerce/auction/delivery/http/admin_auction_handler.go` — LogSafe
- `commerce/subscription/delivery/http/` — LogSafe
- `commerce/paymentmethod/delivery/http/` — LogSafe
- `governance/verification/application/verification_service.go` — LogSafe
- `governance/support/delivery/http/support_handler.go` — LogSafe
- `governance/dispute/delivery/http/dispute_handler.go` — LogSafe

**Governance chain:** NOT used by canonical governance (Decision uses audit_events).

**Classification:** KEEP — shared infrastructure for finance/platform/commerce/support/verification domains. Not governance residue.

---

## 12. LogSafe Analysis

**Used by:** Same domains as admin_audit_logs (see §11).

**Governance chain:** NOT used by canonical governance Decision creation. `GovernanceDecisionCreated` uses `audit_events` (transactional, error-returning).

**Classification:** KEEP — best-effort logging is appropriate for non-governance admin actions (finance withdrawals, dispute resolution, capability grants, etc.). Not governance residue.

---

## 13. Migration Analysis

| Migration | Purpose | Status |
|---|---|---|
| 000001 | Canonical schema (includes moderation_cases, admin_audit_logs, audit_events) | CANONICAL (moderation_cases dropped by 000056) |
| 000055 | Canonical moderation foundation (cases, decisions, enforcements) | CANONICAL |
| 000056 | Drop legacy moderation_cases + enums | CANONICAL |
| 000057 | Canonical Report alignment | CANONICAL |
| 000058 | Audit events immutability trigger | CANONICAL |

**No migration exists solely for dead legacy structures.** DomainAction never had a migration (no `domain_actions` table).

**Historical migrations (000001-000054):** Contain `moderation_cases` table creation. These are historical records — DO NOT rewrite per forward-only doctrine.

**Classification:** KEEP ALL — no cleanup needed for migrations.

---

## 14. Test/Fixture Analysis

### Dead tests (DELETE WITH IMPLEMENTATION)

| Test File | Purpose | Status |
|---|---|---|
| `entity/domain_action_test.go` | Tests parked DomainAction entity | DEAD/ZOMBIE |
| `entity/enforce_notes_test.go` | Tests DomainAction enforcement notes | DEAD/ZOMBIE |
| `entity/moderation_action.go` | ActionType enum for DomainAction | DEAD/ZOMBIE |
| `application/appeal_service_test.go` | Tests legacy Appeal using GovernanceCase | FUTURE (keep until Appeal rebuild) |
| `delivery/http/appeal_handler_test.go` | Tests legacy Appeal handler | FUTURE (keep until Appeal rebuild) |
| `delivery/http/appeal_capability_guard_test.go` | Tests Appeal capability guard | FUTURE (keep until Appeal rebuild) |
| `apps/admin/src/components/moderation/CaseDetailModal.test.tsx` | Tests dead modal | DEAD/ZOMBIE |
| `apps/admin/src/hooks/useModeration.test.tsx` | Tests dead hook | DEAD/ZOMBIE |
| `apps/admin/src/lib/api/moderation.test.ts` | Tests dead API | DEAD/ZOMBIE |

### Test residue in dev-reset-data

| File | Reference | Status |
|---|---|---|
| `cmd/dev-reset-data/main.go:82` | `"moderation_cases"` in table list | DEAD (harmless — TRUNCATE on non-existent table is no-op) |
| `cmd/dev-reset-data/main.go:115` | `"moderation_cases"` in cleanup list | DEAD (harmless) |

---

## 15. Documentation Residue

**Historical audit reports** (`docs/audits/moderation/`):
- Multiple reports reference `GovernanceCase`, `moderation_cases`, `DomainAction`, `approve/reject` vocabulary
- These are HISTORICAL RECORDS of what was found and decided
- Classification: KEEP — do not rewrite history

**Active implementation docs:**
- `LABUDA — CANONICAL MODERATION DESIGN v1.md` — references `GovernanceCase` as rejected design
- `LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1.md` — references `GovernanceCase` as rejected
- `LABUDA — CANONICAL MODERATION SPECIFICATION v1.md` — references `GovernanceCase`
- These document the REJECTION of GovernanceCase — accurate and useful
- Classification: KEEP

**Scripts:**
- `scripts/DOMAIN_NAMING_CONSISTENCY_SUMMARY.md` — references legacy naming
- Classification: DOCUMENTATION RESIDUE — could be updated but not urgent

---

## 16. Duplicate Authority Analysis

| Domain | Authority | Competing? | Status |
|---|---|---|---|
| Report | `reports` table | NO | CLEAN |
| Case | `cases` table | NO (moderation_cases dropped) | CLEAN |
| Decision | `decisions` table | NO (GovernanceCase.status is legacy) | CLEAN |
| Enforcement | `enforcements` table | NO (DomainAction is parked) | CLEAN |
| Audit | `audit_events` table | NO (admin_audit_logs is for other domains) | CLEAN |

**No duplicate authorities detected.** The canonical chain is the sole authority for all governance operations.

---

## 17. Cleanup Candidates

### HIGH PRIORITY — Dead Code (safe to delete)

| Artifact | Why Dead | Active Consumers | Future Consumers | Safe to Delete? |
|---|---|---|---|---|
| `entity/domain_action.go` | No migration, no instantiation, PARKED | ZERO | NONE | YES |
| `entity/domain_action_test.go` | Tests parked code | ZERO | NONE | YES |
| `entity/moderation_action.go` | ActionType enum for DomainAction | ZERO | NONE | YES |
| `entity/enforce_notes_test.go` | Tests DomainAction notes | ZERO | NONE | YES |
| `repository/domain_action_repository.go` | Interface for parked entity | ZERO | NONE | YES |
| `repository/domain_action_repository_impl.go` | Impl for parked entity | ZERO | NONE | YES |
| `worker/domain_action_worker.go` | Worker for parked entity | ZERO | NONE | YES |
| `application/appeal_reversal_service.go` | Uses parked DomainAction | ZERO | NONE | YES |
| `worker/outbox_event_registry.go:198-203` | DomainActionWorker PARKED comment | ZERO | NONE | YES (comment only) |
| `pages/ModerationCasesPage.tsx` | Not routed, calls removed endpoints | ZERO | NONE | YES |
| `components/moderation/CaseDetailModal.tsx` | Only used by dead page | ZERO | NONE | YES |
| `components/moderation/CaseDetailModal.test.tsx` | Tests dead component | ZERO | NONE | YES |
| `hooks/useModeration.ts` | Only used by dead page/modal | ZERO | NONE | YES |
| `hooks/useModeration.test.tsx` | Tests dead hook | ZERO | NONE | YES |
| `lib/api/moderation.ts` | Only used by dead hook | ZERO | NONE | YES |
| `lib/api/moderation.test.ts` | Tests dead API | ZERO | NONE | YES |
| `types/moderation.ts` | Only used by dead hook/API | ZERO | NONE | YES |

### MEDIUM PRIORITY — Legacy Appeal (keep until rebuild)

| Artifact | Why Legacy | Active Consumers | Future Consumers | Safe to Delete? |
|---|---|---|---|---|
| `entity/governance_case.go` | Legacy GovernanceCase super-entity | Appeal domain | Appeal Slice 10 (will REPLACE) | NO — keep until Appeal rebuild |
| `repository/moderation_repository.go` | Legacy interface | Appeal domain | Appeal Slice 10 (will REWRITE) | NO — keep until Appeal rebuild |
| `repository/moderation_repository_impl.go` | Reads dropped table | Compilation only | NONE (will be replaced) | NO — keep until Appeal rebuild |
| `application/appeal_service.go` | Uses GovernanceCase | Appeal routes | Appeal Slice 10 (will REBUILD) | NO — keep until Appeal rebuild |
| `delivery/http/appeal_handler.go` | Uses GovernanceCase | Appeal routes | Appeal Slice 10 (will REBUILD) | NO — keep until Appeal rebuild |
| `application/appeal_service_test.go` | Tests legacy Appeal | Compilation | Appeal Slice 10 | NO — keep until Appeal rebuild |
| `delivery/http/appeal_handler_test.go` | Tests legacy Appeal handler | Compilation | Appeal Slice 10 | NO — keep until Appeal rebuild |
| `delivery/http/appeal_capability_guard_test.go` | Tests Appeal guard | Compilation | Appeal Slice 10 | NO — keep until Appeal rebuild |

### KEEP — Shared Infrastructure

| Artifact | Why Keep | Consumers |
|---|---|---|
| `audit/admin_audit_logger.go` | Shared admin audit (LogSafe) | finance, platform, commerce, support, verification |
| `admin_audit_logs` table | Shared admin audit storage | Same as above |
| `LogSafe` pattern | Best-effort logging for non-governance domains | Same as above |
| `ResourceTypeChatMessage` | Future chat moderation | Handler wired, no producer |

### KEEP — Documentation

| Artifact | Why Keep |
|---|---|
| Historical audit reports | Record of what was found and decided |
| Canonical design docs | Reference for rejected designs |
| `DOMAIN_NAMING_CONSISTENCY_SUMMARY.md` | Could be updated but not urgent |

---

## 18. KEEP / FUTURE / REMOVE Classification Summary

### KEEP (CANONICAL)
- `reports`, `cases`, `decisions`, `enforcements`, `audit_events` tables
- All canonical entities, services, handlers, routes, UI
- `admin_audit_logs` + `AdminAuditLogger` + `LogSafe` (shared infrastructure)
- All migrations (historical records)
- All historical audit reports

### FUTURE DEPENDENCY (KEEP UNTIL REBUILD)
- GovernanceCase entity → Appeal Slice 10
- ModerationRepository interface → Appeal Slice 10
- appeal_service.go → Appeal Slice 10
- appeal_handler.go → Appeal Slice 10
- appeal_service_test.go → Appeal Slice 10
- appeal_handler_test.go → Appeal Slice 10
- appeal_capability_guard_test.go → Appeal Slice 10
- ResourceTypeChatMessage → future chat moderation (if decided)
- ModerationEventHandler chat_message handlers → future chat moderation

### REMOVE (DEAD/ZOMBIE)
- DomainAction entity + repository + worker + tests
- moderation_action.go (ActionType enum)
- enforce_notes_test.go
- appeal_reversal_service.go
- ModerationCasesPage (frontend)
- CaseDetailModal + test (frontend)
- useModeration + test (frontend)
- moderation.ts API + test (frontend)
- types/moderation.ts (frontend)
- dev-reset-data `moderation_cases` reference (harmless dead ref)

---

## 19. Recommended Destructive Cleanup Sequence

### Phase A: Dead Frontend (lowest risk, highest visibility)
```
DELETE: pages/ModerationCasesPage.tsx
DELETE: components/moderation/CaseDetailModal.tsx
DELETE: components/moderation/CaseDetailModal.test.tsx
DELETE: hooks/useModeration.ts
DELETE: hooks/useModeration.test.tsx
DELETE: lib/api/moderation.ts
DELETE: lib/api/moderation.test.ts
DELETE: types/moderation.ts
VERIFY: admin build passes
VERIFY: admin tests pass
```

### Phase B: Dead DomainAction (medium risk, complete removal)
```
DELETE: entity/domain_action.go
DELETE: entity/domain_action_test.go
DELETE: entity/moderation_action.go
DELETE: entity/enforce_notes_test.go
DELETE: repository/domain_action_repository.go
DELETE: repository/domain_action_repository_impl.go
DELETE: worker/domain_action_worker.go
DELETE: application/appeal_reversal_service.go
REMOVE: outbox_event_registry.go:198-203 DomainActionWorker comment
VERIFY: go build ./...
VERIFY: go test ./internal/governance/moderation/...
VERIFY: go test ./internal/worker/...
```

### Phase C: Dead ModerationRepositoryImpl (medium risk)
```
DELETE: repository/moderation_repository_impl.go
UPDATE: dependencies.go — remove moderationRepository wiring
UPDATE: dependencies.go — remove _ = caseService
VERIFY: go build ./...
VERIFY: go test ./internal/governance/moderation/...
```

### Phase D: Legacy Appeal entity cleanup (high risk — do after Appeal rebuild)
```
PENDING: Appeal Slice 10 rebuild
AFTER REBUILD:
  DELETE: entity/governance_case.go (replaced by canonical case)
  DELETE: repository/moderation_repository.go (replaced by canonical repo)
  DELETE: all appeal_service.go GovernanceCase references
  DELETE: all appeal_handler.go GovernanceCase references
  VERIFY: full governance test suite
```

### Phase E: Documentation + fixtures (low risk)
```
UPDATE: dev-reset-data main.go — remove moderation_cases reference
UPDATE: DOMAIN_NAMING_CONSISTENCY_SUMMARY.md — mark legacy sections
VERIFY: dev-reset-data compiles
```

### Final Regression
```
go build ./...
go test ./internal/governance/...
go test ./internal/worker/...
admin build
admin tests
governance E2E integration tests
```

---

## 20. Risks

1. **Appeal domain depends on legacy GovernanceCase** — removing GovernanceCase before Appeal rebuild WILL break compilation. Phase D MUST wait for Appeal Slice 10.

2. **DomainAction removal is safe** — no active consumer, no migration, no route. But verify worker tests still pass.

3. **Frontend removal is safe** — no active routes, no imports in App.tsx. But verify admin build.

4. **dev-reset-data `moderation_cases`** — harmless (TRUNCATE on non-existent table). Low priority.

5. **chat_message handler** — wired but has no producer. Keeping it is low cost and enables future chat moderation.

---

## PROOF

```
go build ./...                                          — PASS
go test ./internal/governance/moderation/...             — PASS
go test ./internal/governance/audit/...                  — PASS
go test ./internal/worker/...                            — PASS (91s)
admin TypeScript build (npx tsc --noEmit)                — PASS
```

All existing canonical governance tests pass. No regression.

---

## FILES

```
docs/audits/moderation/REPORT_GOVERNANCE_FINAL_CLEANUP_AUDIT.md — NEW
```

---

## COMMIT

```
docs(audit): add governance final cleanup audit report (Slice 9)
```

---

## PUSH

Pending user approval.

---

## WORKING TREE

Clean (only .commandcode/taste/* unstaged — not touched).

---

## FINAL VERDICT

**PASS**

The cleanup map is complete. Every candidate has a proven dependency classification:
- 17 artifacts classified as DEAD/ZOMBIE (safe to delete)
- 8 artifacts classified as FUTURE DEPENDENCY (Appeal Slice 10)
- Shared infrastructure (admin_audit_logs, LogSafe) correctly identified as KEEP
- No duplicate authorities detected
- Recommended 5-phase cleanup sequence derived from evidence
