# LABUDA — MODERATION SLICE 2: CANONICAL REPORT DOMAIN + REPORT INTAKE

- **Tanggal:** 2026-08-31
- **Mode:** IMPLEMENTATION SLICE 2 — REPORT DOMAIN + REPORT INTAKE
- **Authority:** `LABUDA — CANONICAL MODERATION SPECIFICATION v1.md`, `LABUDA — CANONICAL MODERATION DESIGN v1.md`, `LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1.md`, `cara-kerja-updated.md`, `docs/audits/moderation/REPORT_CASE_IMPLEMENTATION_SLICE_1.md`
- **Baseline:** current filesystem (post-Slice 1)

---

## STATUS: PASS

Slice 2 acceptance gate (see §17) is fully met. Every mandatory item is proven with
concrete evidence against real PostgreSQL and real HTTP request paths.

---

## 1. Scope

Canonical Report domain + Report intake:

- Report entity/model (`reason_code` taxonomy, canonical target types, evidence snapshot)
- Report DTO/request/response
- Report repository (`reports` table, target existence, duplicate-safe insert)
- Report service/use-case
- Report HTTP handler + routes
- Report validation (target + reason + reporter + self-report)
- Report target existence checks (all 5 canonical target domains)
- Report duplicate protection (race-safe, DB-level)
- Report evidence snapshot (minimal immutable)
- Legacy Report-intake code removal (`CreateCase → GovernanceCase → moderation_cases`)
- Mobile Report consumer migration to canonical contract
- Relevant tests/fixtures migration or removal

Out of scope (NOT touched): Case correlation, Case review, Decision, Enforcement,
Warning, Appeal, Admin moderation workflow, Outbox retry, target executors.

---

## 2. Canonical Authority

- **Report** → `ReportService` + `ReportRepository` (governance/moderation package),
  writing exclusively to the canonical `reports` table.
- **Target state** → target domain (contents/comments/for_sales/auctions/users).
- **Case/Decision/Enforcement** → not implemented in this slice (later slices).
- **Appeal/Warning** → out of scope; their legacy runtime is runtime-dead and kept
  compile-only for a later slice.

Report is an immutable historical intake record: `reporter_id`, `subject_type`,
`subject_id`, `reason_code`, `reason_note`, `evidence_snapshot`, `created_at` cannot
be mutated after creation (DB trigger `trg_reports_immutable`).

---

## 3. Business Rules Implemented

| Rule | Implementation |
|---|---|
| Report = immutable intake record | `reports` table + `trg_reports_immutable` (UPDATE rejected) |
| Canonical targets: content, comment, for_sale, auction, user | `moderation_target_type_enum` (from 000055) + service validation |
| `chat_message` / `fixed_price_sale` rejected | Not in canonical enum; service `ReportTargetType.IsValid()` rejects |
| `reason_code` locked taxonomy (7 codes) | `report_reason_code_enum` (000057) + `ReportReasonCode.IsValid()` |
| `reason_note` optional free text | nullable `reason_note` column; not a reason_code replacement |
| Backend is reason/target validation authority | service-side validation before any DB write |
| Target existence validated per canonical domain | `ValidateTarget` queries contents/comments/for_sales/auctions/users |
| No target lifecycle changes | Report only reads target existence; never mutates target |
| Same reporter + same subject → duplicate rejected | `uniq_reports_one_per_reporter_subject` unique index (race-safe) |
| Different reporters + same subject → valid | unique is on `(reporter_id, subject_type, subject_id)` |
| Self-report DENIED | Owner decision (Business Truth §6); service checks snapshot author vs reporter |
| No Case auto-creation on report | `case_id` stays NULL; correlation is a later slice |
| No Decision/Enforcement from Report | no such code path exists |

---

## 4. Files Changed

### New files

| File | Purpose |
|---|---|
| `backend/migrations/000057_report_slice_canonical_alignment.up.sql` | Align `reports` to canonical shape + `report_reason_code_enum` + immutability trigger + duplicate unique index |
| `backend/migrations/000057_report_slice_canonical_alignment.down.sql` | Reverse of 000057 |
| `backend/internal/governance/moderation/entity/report.go` | Canonical Report entity, `ReportTargetType`, `ReportReasonCode`, `EvidenceSnapshot` |
| `backend/internal/governance/moderation/application/report_service.go` | Canonical Report intake service |
| `backend/internal/governance/moderation/application/report_service_test.go` | Report service unit tests (7) |
| `backend/internal/governance/moderation/infrastructure/repository/report_repository.go` | Report repository interface + domain errors |
| `backend/internal/governance/moderation/infrastructure/repository/report_repository_impl.go` | Report persistence (reports table, target validation + snapshot) |
| `backend/internal/governance/moderation/delivery/http/report_handler.go` | Report HTTP handler (create/list/get own) |
| `backend/tests/report_runtime_integration_test.go` | Full-path integration proof (HTTP → service → repo → PostgreSQL) |

### Modified files

| File | Change |
|---|---|
| `backend/cmd/core_server/routes_core.go` | Replace `/moderation/cases` with `/reports`; remove dead admin moderation routes |
| `backend/internal/serdeboot/dependencies.go` | Wire canonical Report service/handler; keep compile-only legacy repo for Appeal |
| `backend/internal/governance/moderation/infrastructure/repository/moderation_repository.go` | Trim legacy interface to `GetByID` only (Appeal compile-only) |
| `backend/internal/governance/moderation/infrastructure/repository/moderation_repository_impl.go` | Trim to `GetByID` only (runtime-dead, documented) |
| `backend/tests/migration_000055_canonical_moderation_foundation_test.go` | Update to canonical `subject_type`/`subject_id`/`reason_code` columns + new index names |
| `apps/mobile/lib/domains/system/report/domain/entities/report.dart` | Canonical target types + reason taxonomy + snake_case backend values |
| `apps/mobile/lib/domains/system/report/data/dto/report_dto.dart` | `ReportDto`/`CreateReportRequestDto` canonical contract |
| `apps/mobile/lib/domains/system/report/data/mappers/report_mapper.dart` | Canonical mapping |
| `apps/mobile/lib/domains/system/report/data/remote/report_api_datasource.dart` | `/reports` endpoints |
| `apps/mobile/lib/domains/system/report/data/repositories/report_repository_impl.dart` | Canonical repository impl |
| `apps/mobile/lib/domains/system/report/presentation/**` | Canonical enums, reason selector, submission dialog, my-reports screen |
| `apps/mobile/lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart` | Chat message report → report the sender **user** (canonical target) |
| `apps/mobile/test/domains/system/report/*` | Migrate contract tests to canonical shape |

### Deleted files (obsolete legacy Report intake tests/runtime)

- `application/moderation_service.go` (CreateCase/ReviewCase intake)
- `delivery/http/moderation_handler.go` (legacy user + admin intake)
- `application/moderation_service_test.go`, `moderation_service_intake_test.go`, `chat_message_intake_test.go`
- `delivery/http/moderation_handler_*_test.go`, `moderation_evidence*_test.go`, `chat_message_preview_test.go`
- `infrastructure/repository/moderation_repository_*_test.go` (legacy tests)
- `apps/mobile/test/.../report_notifier_contract_test.dart`, `report_submission_dialog_test.dart`, `my_reports_screen_widget_test.dart`, `report_provider_graph_test.dart` (pre-existing broken tests for non-existent APIs)

---

## 5. Legacy Paths Removed

| Legacy path | Status |
|---|---|
| `POST /api/v1/moderation/cases` (CreateCase) | Route removed; replaced by `POST /api/v1/reports` |
| `GET /api/v1/moderation/my-cases` | Removed; replaced by `GET /api/v1/reports/mine` |
| `GET /api/v1/moderation/cases/:id` | Removed; replaced by `GET /api/v1/reports/:id` |
| `ModerationService.CreateCase/ReviewCase` | File deleted |
| `ModerationRepository.Create/ResourceExists/HasUserReportedResource/ValidateChatMessageReporter` | Interface + impl trimmed |
| `admin /moderation/cases*` (ListCases/GetCase/GetCaseEvidence/ApplyAction) | Routes removed with the dead legacy runtime |
| `moderation_cases` write path | No producer remains (only a documented compile-only `GetByID` for out-of-scope Appeal) |

No compatibility bridge, no alias, no fallback, no dual-write exists.

---

## 6. Target Validation Matrix

| Target | Table | Guard | Evidence |
|---|---|---|---|
| content | `contents` | `deleted_at IS NULL` | integration test `valid_reports_all_canonical_targets` |
| comment | `comments` | `deleted_at IS NULL` | integration test |
| for_sale | `for_sales` (join `products`) | status-based (no deleted_at) | integration test |
| auction | `auctions` (join `products`) | status-based (no deleted_at) | integration test |
| user | `users` | `deleted_at IS NULL` | integration test |
| chat_message | — | rejected (not canonical) | integration test `invalid_targets_rejected` |
| fixed_price_sale | — | rejected (not canonical) | integration test `invalid_targets_rejected` |
| profile/listing | — | rejected (not canonical) | integration test `invalid_targets_rejected` |
| non-existent UUID | — | `ErrReportTargetNotFound` → 404 | integration test `non_existent_target_rejected` |

Lifecycle: for_sale/auction status-based existence (withdrawn/sold/ended remain
reportable as governance history per Business Truth §26); content/comment/user
guard soft-delete.

---

## 7. Reason Taxonomy Proof

`reason_code` accepts exactly:

```
scam_or_fraud, prohibited_content, harassment_or_abuse, impersonation,
misleading_information, commerce_violation, other
```

- DB enum `report_reason_code_enum` (000057) rejects anything else (integration proof).
- Service `ReportReasonCode.IsValid()` rejects non-taxonomy codes before DB write
  (unit test `TestReportService_CreateReport_RejectsInvalidReason`).
- Integration test `invalid_reason_rejected` proves `spam`, `fake_product`,
  `copyright`, `harassment`, `anything_else` → 400.
- No dynamic reason config, no policy engine, no DB-configurable taxonomy.

---

## 8. Duplicate / Race Proof

Business rule: same reporter + same subject → rejected; different reporter + same
subject → valid.

- **DB constraint:** `uniq_reports_one_per_reporter_subject (reporter_id, subject_type, subject_id)` (000057).
- **Integration test `duplicate_same_reporter_rejected`:** second report for same
  reporter+subject → 409; exactly 1 row remains.
- **Integration test `duplicate_race_safe`:** 8 concurrent identical HTTP requests
  (same reporter+subject) → exactly 1 `201`, 7 `409`; exactly 1 DB row. This proves
  the unique index is the final race-safe guard (SELECT-then-INSERT alone would
  allow duplicates).
- **Integration test `different_reporters_same_subject_valid`:** reporter A + reporter B
  both report the same subject → both `201`; 2 rows.
- **Unit test `TestReportService_CreateReport_ConcurrentDuplicateFromDB`:** service
  propagates the unique-index duplicate error when the pre-check passed but the
  insert raced.

---

## 9. Immutability Proof

- DB trigger `trg_reports_immutable` (`prevent_reports_update()`) rejects any `UPDATE`
  on `reports`.
- **Integration test `reports_immutable`:** `UPDATE ... SET reason_code` → error
  contains `immutable`; `UPDATE ... SET reporter_id` → error; row unchanged.
- No generic update endpoint exists: `POST /reports` only inserts; `GET /reports/:id`
  only reads with ownership check; `GET /reports/mine` only lists own reports.

---

## 10. PostgreSQL Proof (real PostgreSQL 16)

| Proof | Command | Result |
|---|---|---|
| Full-path integration test | `go test -tags integration -run TestCanonicalReportRuntime -v ./tests/ -count=1 -timeout 300s` | **PASS** (11 subtests) |
| Migration replay from zero (000001–000057) | `go run ./cmd/migrate` on fresh `labuda_slice2_replay` DB | **PASS** — `reports` table canonical, `moderation_cases` absent |
| 000057 down migration | `psql -f 000057...down.sql` on replay DB | **PASS** — preliminary 000055 shape restored |
| Foundation test (updated) | `go test -tags integration -run TestCanonicalModerationFoundation ./tests/` | **PASS** |
| Schema state proof | `go test -tags integration -run TestMigration000047_SchemaStateProof ./tests/` | **PASS** |
| Chat commerce references drop | `go test -tags integration -run TestMigration000054 ./tests/` | **PASS** |

`reports` table (verified via `\d reports`): `id, reporter_id, subject_type,
subject_id, reason_code, reason_note, evidence_snapshot, case_id, created_at`,
with `uniq_reports_one_per_reporter_subject`, `idx_reports_reporter`,
`idx_reports_subject`, `idx_reports_case_id`, and `trg_reports_immutable`.

---

## 11. API Contract Proof

The integration test exercises the real path:

```
POST /api/v1/reports  → ReportHandler.CreateReport
                       → ReportService.CreateReport
                       → ReportRepository (ValidateTarget + Create)
                       → reports table (PostgreSQL)
```

Verified wire shape (201):

```json
{
  "id": "...", "reporter_id": "...", "subject_type": "content",
  "subject_id": "...", "reason_code": "scam_or_fraud",
  "evidence_snapshot": { "author_id": "...", "author_username": "...", ... },
  "created_at": "..."
}
```

`GET /api/v1/reports/mine` returns `{ "reports": [...], "page", "limit", "count" }`
with only the caller's reports. `GET /api/v1/reports/:id` returns a report only if
the caller is the reporter (404 otherwise).

Mobile consumer migrated to the canonical contract (all 25 mobile report tests pass,
production analyzer clean for the report domain).

---

## 12. Negative / Residue Proof

Searched: `GovernanceCase`, `moderation_cases`, `CreateCase`, `POST /moderation/cases`,
`moderation/my-cases`, `entity_type`, `entity_id`, `chat_message`, `fixed_price_sale`.

| Residue | Classification | Status |
|---|---|---|
| `moderation_repository.go`/`_impl.go` (`GetByID` only) | Legacy read path, **runtime-dead** (reads dropped `moderation_cases`) | Kept compile-only for out-of-scope Appeal (Slice 9); documented in code |
| `entity/governance_case.go`, `moderation_resource_type.go` | Legacy entity types | Kept compile-only for out-of-scope Appeal; documented |
| `worker/moderation_event_handler.go`, `outbox_worker.go` | Enforcement/outbox consumer (Enforcement domain, Slice 5-6) | Out of scope; untouched |
| `domain_action_repository*` | DomainAction (PARKED zombie) | Out of scope; untouched |
| `cmd/dev-reset-data/main.go` (`moderation_cases`) | Tool; tolerates missing tables | Harmless; out of scope |
| Admin `types/moderation.ts`, `ModerationCasesPage`, `useModeration` | Admin Case review UI (Slice 3+) — **backend path removed** | Out of scope; reported (see §15) |
| Mobile commerce/notification `fixed_price_sale` strings | Commerce/notification domains (order source, notification type) | Legitimate commerce vocabulary; out of scope |
| Report domain mobile code | Clean — no legacy strings | Verified |

**Proven:** no active producer/consumer can revive the legacy Report intake.
`CreateCase` route is gone; `ModerationService` file is gone; the only remaining
`moderation_cases` reference is a read that fails at runtime and cannot write.

---

## 13. Tests / Build / Integration Commands

| Command | Result |
|---|---|
| `go build ./...` (backend) | **PASS** |
| `go test ./internal/governance/moderation/... -count=1` | **PASS** (all 4 packages) |
| `go test -tags integration -run TestCanonicalReportRuntime -v ./tests/` | **PASS** (11 subtests) |
| `go test -tags integration -run TestCanonicalModerationFoundation ./tests/` | **PASS** |
| `go test -tags integration -run TestMigration000047_SchemaStateProof ./tests/` | **PASS** |
| `go test -tags integration -run TestMigration000054 ./tests/` | **PASS** |
| `go test ./internal/serverboot/ -run TestModerationEventHandler -count=1` | **PASS** |
| `go run ./cmd/migrate` on fresh replay DB | **PASS** (000001–000057 clean) |
| `go test ./cmd/... ./internal/worker/...` | **PASS** |
| `flutter analyze lib/domains/system/report` | **No issues** |
| `flutter test test/domains/system/report/` | **PASS** (25 tests) |

### Pre-existing failures (NOT caused by Slice 2, counter-evidence provided)

| Failure | Evidence it is pre-existing |
|---|---|
| `serverboot` full-suite `duplicate metrics collector registration` panic | Occurs only when multiple `InitServices` tests run in one process (Prometheus global registry); passes in isolation; unrelated to moderation wiring |
| `governance/dispute` `TestResolveDispute_BlocksCompletedReleasedForSystemCaller` nil pointer | Dispute/commerce domain, untouched by Slice 2 |
| `governance/evaluator` build failure (`contententity.StatusFulfilled`) | Pre-existing test referencing a removed content status; unrelated to moderation |
| `migration_000047_for_sale_vocabulary_replay_test` down-replay `moderation_cases` | Down-chain replay incompatible with 000056's drop (Slice 1 consequence); test file untouched by Slice 2 |
| `migration_000047_rule9_invariant_test` trigger expectation | 000049-era trigger semantics; test file untouched by Slice 2 |

### Server runtime boot

`go run ./cmd/core_server` fails to boot with `Midtrans config invalid:
MIDTRANS_SERVER_KEY is required for sandbox payment activation` — a pre-existing
environment/config issue unrelated to Slice 2 (the Report API path is proven by the
integration test, which runs the real handler over real HTTP against real PostgreSQL).

---

## 14. Known Limitations

- **`reports.case_id`** stays NULL: Report → Case correlation is a later slice.
  The column and FK exist (000055) and are untouched.
- **Report has no status/lifecycle**: the Slice 2 model treats every report as
  permanent; the duplicate constraint is a hard unique on
  `(reporter_id, subject_type, subject_id)`. If a future slice introduces report
  terminal states, the constraint may need to become partial — a documented
  schema-evolution decision, not a business guess.
- **Appeal/Warning runtime remains dead**: it depends on the dropped
  `moderation_cases` read path. It is kept compile-only and must be rebuilt in the
  Appeal/Warning slices against the canonical Decision schema.

---

## 15. Out-of-Scope Findings

| Finding | Classification | Recommendation |
|---|---|---|
| Admin moderation UI (`ModerationCasesPage`, `useModeration`, `types/moderation.ts`) still calls `GET/POST /admin/moderation/cases*` | Its backend path was removed with the dead legacy runtime | Rebuild in the Case/Decision/Enforcement slice (Slice 3+) against the canonical API |
| `moderation_repository.GetByID` + `GovernanceCase` entity (compile-only) | Zombie for out-of-scope Appeal | Rebuild Appeal in its slice; then delete |
| `DomainAction` + `DomainActionWorker` | PARKED zombie (no table) | Delete in a dedicated cleanup slice |
| `worker/moderation_event_handler.go` + outbox enforcement events | Enforcement domain (legacy outbox-as-enforcement) | Replace in Enforcement/Outbox slices |
| `chat_message` moderation vocabulary in `moderation_resource_type.go` | Out-of-scope legacy entity for Appeal | Deleted when Appeal is rebuilt |
| `fixed_price_sale` strings in admin orders/promotion types and mobile commerce code | Commerce vocabulary, not moderation | Out of scope for moderation slices |
| Server boot Midtrans config error | Environment/config | Not moderation; owner/dev to fix env |

---

## 16. Unresolved Owner Decisions

None for this slice. The locked inputs (reason taxonomy, self-report DENY, duplicate
rule, evidence snapshot as minimal immutable metadata) were all determinable from the
canonical specification, design, business truth, and prior audits. No business
ambiguity was silently guessed.

---

## 17. Exact Final Scope Status

### Acceptance gate

- [x] Canonical Report authority exists
- [x] Report intake no longer depends on GovernanceCase
- [x] Canonical targets only: content, comment, for_sale, auction, user
- [x] chat_message rejected
- [x] fixed_price_sale rejected
- [x] reason_code taxonomy enforced
- [x] reason_note optional
- [x] target existence validated
- [x] duplicate protection race-safe
- [x] different reporters may report same subject
- [x] Report historical fields cannot be mutated
- [x] persistence proven against real PostgreSQL
- [x] API → service → repository → DB path proven
- [x] legacy Report producer no longer active
- [x] no compatibility bridge
- [x] no dual-write
- [x] obsolete Report tests/fixtures migrated or removed
- [x] scoped residue search completed
- [x] no unresolved business ambiguity silently guessed
- [x] changes remain bounded to Slice 2
- [x] final report contains concrete evidence

### Handoff to next slice

**STOP.** Do not implement Case/Decision/Enforcement/Warning/Appeal/Admin/Mobile in
this session. The next domain (Case correlation + Case runtime) is opened only after
this report is verified.

**Next-slice prerequisites (documented, not implemented):**

1. **Case slice** must build the `cases` repository/service and Report → Case
   correlation (`reports.case_id` is ready).
2. **Appeal/Warning** must be rebuilt against canonical `decisions`; then delete
   `moderation_repository.go`/`_impl.go` `GetByID`, `governance_case.go`, and
   `moderation_resource_type.go`.
3. **Admin moderation UI** must be rebuilt against the canonical Case/Decision API.
4. **Enforcement/Outbox** slices replace `moderation_event_handler.go` and the
   outbox-as-enforcement pattern.
