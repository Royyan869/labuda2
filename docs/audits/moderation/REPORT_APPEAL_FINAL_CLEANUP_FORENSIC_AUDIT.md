# APPEAL FINAL CLEANUP FORENSIC AUDIT

**Date:** 2026-09-01
**Baseline commit:** `d25467c`
**Working tree:** 7 modified frontend files (from consumer verification), 2 new test/report files, 0 backend Go changes
**Methodology:** Exhaustive filesystem search, zero code modification, zero deletion

---

## 1. Baseline

- Commit: `d25467c`
- Working tree contains frontend consumer fixes (decision_id alignment) from previous verification
- New test: `backend/tests/appeal_review_e2e_integration_test.go` (6 real PostgreSQL E2E tests)
- New report: `docs/audits/moderation/REPORT_APPEAL_FINAL_RUNTIME_VERIFICATION.md`

---

## 2. Authority Map

```
Report → Case → Decision #1 → Enforcement #1
  ↓
Appeal → Decision (via decision_id FK)
  ↓ (ReviewAppeal)
Decision #2 (same Case) → Enforcement #2 (reversal) → Outbox → Worker → Restoration
```

**Canonical runtime authority:**
- `AppealService` uses: `AppealRepository`, `DecisionRepository`, `CaseRepository`, `DecisionService`
- `GovernanceAdminHandler` uses: `CaseRepository`, `ReportRepository`, `DecisionService`, `EnforcementRepository`
- **NOT used by Appeal runtime:** `ModerationRepository`, `GovernanceCase`

---

## 3. Candidate-by-Candidate Evidence

### 3A. `GetAppealWithCase`

| Aspect | Evidence |
|--------|----------|
| Definition | `appeal_service.go:389` — method on `*AppealService` |
| Body | `return nil, nil, fmt.Errorf("GetAppealWithCase is deprecated: use GetAppealWithContext")` |
| Active callers | **0** (zero across all .go files) |
| Interface declaration | **NOT** in any interface |
| Tests | **0** (no test calls this method) |
| Handler usage | Handler at `appeal_handler.go:386` uses `GetAppealWithContext` instead |
| Return type | `(*entity.Appeal, *entity.GovernanceCase, error)` — references GovernanceCase |
| Comment says | "kept for backward compatibility during Slice A" / "compilation continuity only" |
| Doc references | 16 mentions across 8 audit reports (all historical documentation) |

**Conclusion:** Dead stub. Zero callers. Exists solely as compilation bridge to GovernanceCase type. Safe to delete.

### 3B. `ErrRestorationEventFailed`

| Aspect | Evidence |
|--------|----------|
| Definition | `entity/appeal.go:158` — struct with `AppealID` and `Err` fields |
| Active Go consumers | **0** (zero references in any .go file besides definition) |
| Error matching / `errors.Is` | **0** |
| HTTP mapping | **0** |
| Tests | **0** (referenced only in historical audit docs) |
| Comment says | "DEFERRED TO SLICE B: This error will be replaced by Decision #2 creation" |
| Slice B delivered | Yes — `ReviewAppeal` creates Decision #2 within single TX |

**Conclusion:** Dead error type. Slice B completed its replacement. Safe to delete.

### 3C. `ModerationRepository` (interface)

| Aspect | Evidence |
|--------|----------|
| Definition | `moderation_repository.go:22` |
| Methods | `GetByID(ctx, tx, caseID) → *GovernanceCase` |
| Comment says | "LEGACY GovernanceCase read path", "RUNTIME-DEAD: moderation_cases dropped in 000056" |
| Table queried | `moderation_cases` — **DROPPED** in migration 000056 |
| Active callers | **0** |
| Blank identifier assignment | `dependencies.go:2398`: `_ = moderationRepo.NewModerationRepository()` |
| Mock in tests | `appeal_service_test.go:222`: `mockModerationRepository` — defined but **NEVER INSTANTIATED** (0 matches for `mockModerationRepository{`) |
| `AppealService` struct dependency | **NO** — struct has `appealRepo`, `decisionRepo`, `caseRepo`, NOT `moderationRepo` |
| `AppealService` constructor params | **NO** — params are `(appealRepo, decisionRepo, caseRepo, decisionService, contentRepo, commentRepo)` |

**Conclusion:** Dead interface. All method calls fail at runtime (table dropped). Zero active consumers. Safe to delete.

### 3D. `ModerationRepositoryImpl`

| Aspect | Evidence |
|--------|----------|
| Definition | `moderation_repository_impl.go:22` |
| Constructor | `NewModerationRepository()` at line 25 |
| Table | `moderation_cases` — **DROPPED** in migration 000056 |
| Runtime behavior | Always fails with "relation not found" |
| Instantiation | Only in `dependencies.go:2398` via `_ = ...` (blank identifier) |
| Active use of result | **0** |

**Conclusion:** Dead implementation. Safe to delete.

### 3E. `GovernanceCase` (entity)

**Full dependency graph — `GovernanceCase → ?`:**

| Symbol | Type | External callers |
|--------|------|-----------------|
| `GovernanceCase` struct | entity | Return type of dead `GetAppealWithCase`, return type of dead `ModerationRepository.GetByID`, field of dead `mockModerationRepository` |
| `GovernanceCaseStatus` type | entity | Only used in `governance_case.go` (internal) |
| `GovernanceCaseDecision` type | entity | Only used in `governance_case.go:60-62` (constants) |
| `GovernanceCaseStatusPending/Approved/Rejected/Enforced` | const | Only used in `governance_case.go` (internal) |
| `GovernanceCaseDecisionApprove/Reject/Enforce` | const | Only used in `governance_case.go:60-62` (internal) |
| `ErrAlreadyReviewed` | error | Only used in `governance_case.go:172` (internal) |
| `ErrInvalidTransition` (governance_case) | error | Only used in `governance_case.go:180` (internal) |
| `ErrEnforceRequiresNote` | error | Only used in `governance_case.go:158` (internal) |
| `NewGovernanceCase()` | func | **0** external callers |
| `ShouldEmitEnforcementEvents()` | method | **0** external callers |
| `Approve/Reject/Enforce()` | methods | **0** external callers |
| `IsPending/IsReviewed/IsTerminal/CanReview/HasEnforcementActions()` | methods | **0** external callers |
| `CanTransition(from, to)` | func | Only used in `governance_case.go:179` (internal) |

**Reverse graph — `? → GovernanceCase`:**

| Consumer | Nature | Impact of deletion |
|----------|--------|-------------------|
| `GetAppealWithCase` (dead method) | Dead return type | Delete together |
| `ModerationRepository.GetByID` (dead interface) | Dead return type | Delete together |
| `moderation_repository_impl.go` (dead impl) | Dead return literal | Delete together |
| `mockModerationRepository` (dead test mock) | Dead mock field | Delete together |
| Migration test assertions | Asserts `moderation_cases` table is gone | **KEEP** — canonical regression proof |
| Historical audit reports (8 files) | Documentation only | **KEEP** — audit evidence |
| `case_repository.go` comment | "no bridge to GovernanceCase" | Stale comment — fix |
| `report_handler.go` comment | "no GovernanceCase, no moderation_cases" | Historical context — can keep |
| `report.go` comment | "replaces rejected GovernanceCase Report intake" | Historical context — can keep |

**Frontend `GovernanceCase` TypeScript type:**
- `apps/admin/src/types/governance.ts:34`: `export interface GovernanceCase`
- Fields: `id, subject_type, subject_id, status: 'open' | 'resolved', created_at, updated_at, closed_at`
- This maps to **canonical `CanonicalCase`** (not the Go `entity.GovernanceCase`)
- Status is `'open' | 'resolved'` — matches `CanonicalCase`, NOT legacy `pending|approved|rejected|enforced`
- **NOT a cleanup target** — it's a naming collision in the TypeScript layer, consuming the canonical Case API

**Conclusion:** `GovernanceCase` entity is entirely dead. All exported symbols have zero external callers. The file is self-contained dead code. Safe to delete.

### 3F. `ErrRestorationEventFailed`

Already covered in 3B. Additional note:
- Defined in `appeal.go:158-164`
- Part of the `entity` package
- Does NOT affect deletion of `governance_case.go` (different file)
- Can be removed from `appeal.go` independently

### 3G. Test/Doc Residue

| Item | File | Classification | Action |
|------|------|---------------|--------|
| `mockModerationRepository` | `appeal_service_test.go:222-231` | Dead fixture (defined, never instantiated) | Delete |
| Comment "ModerationRepository, ContentRepository, CommentRepository" | `appeal_handler_test.go:671` | Stale comment | Fix |
| Comment "ErrCaseNotFound is defined in appeal.go" | `canonical_case.go:21` | Stale comment (ErrCaseNotFound was renamed to ErrDecisionNotFound) | Fix |
| Migration test `moderation_cases must be gone` | `migration_000055_canonical_moderation_foundation_test.go` | **Required canonical regression** | KEEP |
| Migration test `GovernanceCase schema removed` | `migration_000047_schema_state_proof_test.go` | **Required canonical regression** | KEEP |
| Historical audit reports (8 files) | `docs/audits/moderation/` | **Historical audit evidence** | KEEP |
| Report comment `report.go:4` "replaces rejected GovernanceCase" | `entity/report.go` | Historical context | KEEP |
| Report comment `report_service.go:23` "CreateCase → GovernanceCase" | `application/report_service.go` | Historical context | KEEP |
| Routes comment `routes_core.go:1309` | `cmd/core_server/routes_core.go` | Historical context | KEEP |

---

## 4. Deletion Safety Verdict

| Candidate | Verdict | Reason |
|-----------|---------|--------|
| `GetAppealWithCase` method | **SAFE TO DELETE** | Zero callers, returns dead `GovernanceCase`, handler uses `GetAppealWithContext` |
| `ErrRestorationEventFailed` type | **SAFE TO DELETE** | Zero consumers in Go code, Slice B replaced its purpose |
| `ModerationRepository` interface | **SAFE TO DELETE** | Zero active consumers, table dropped in migration 000056 |
| `ModerationRepositoryImpl` struct + `NewModerationRepository()` | **SAFE TO DELETE** | Zero active consumers, always fails at runtime |
| `governance_case.go` (entire file) | **SAFE TO DELETE** | All 15+ exported symbols have zero external callers |
| `mockModerationRepository` in test | **SAFE TO DELETE** | Defined but never instantiated (0 matches for `mockModerationRepository{`) |
| `_ = moderationRepo.NewModerationRepository()` in `dependencies.go` | **SAFE TO DELETE** | Blank identifier assignment, result never used |
| Stale comment `canonical_case.go:21` | **SAFE TO FIX** | References non-existent `ErrCaseNotFound` |
| Stale comment `appeal_handler_test.go:671` | **SAFE TO FIX** | References `ModerationRepository` in description |
| Migration tests | **KEEP** | Canonical regression proof (assert table/schema absence) |
| Historical audit reports | **KEEP** | Audit evidence trail |

---

## 5. Exact Cleanup Sequence

The sequence below avoids any intermediate broken compilation state:

### Phase 1: Remove dead Appeal method
- `appeal_service.go`: Delete `GetAppealWithCase` method (lines 386-397)
- Import `fmt` remains needed for other uses

### Phase 2: Remove dead test mock
- `appeal_service_test.go`: Delete `mockModerationRepository` struct and its `GetByID` method (lines 222-231)

### Phase 3: Remove dead runtime instantiation
- `dependencies.go:2398`: Delete `_ = moderationRepo.NewModerationRepository()` line and its 3 comment lines above it

### Phase 4: Delete dead repository files
- Delete `infrastructure/repository/moderation_repository.go` (interface)
- Delete `infrastructure/repository/moderation_repository_impl.go` (implementation)

### Phase 5: Delete dead entity file
- Delete `entity/governance_case.go` (239 lines of dead code)

### Phase 6: Remove dead error type
- `entity/appeal.go`: Delete `ErrRestorationEventFailed` struct and `Error()` method (lines 156-164)

### Phase 7: Fix stale comments
- `entity/canonical_case.go:21`: Remove stale `ErrCaseNotFound` comment
- `delivery/http/appeal_handler_test.go:671`: Update stale `ModerationRepository` reference

### Phase 8: Verify
- `go build ./...` — must compile
- `go test ./internal/governance/moderation/...` — must pass
- `npx vitest run` — must pass
- `dart analyze lib/domains/system/report/` — must pass

**Note:** After Phase 4, the `moderationRepo` import alias in `dependencies.go` will still be needed (it's used for `NewEnforcementRepository`, `NewReportRepository`, `NewCaseRepository`, `NewDecisionRepository`). Only the `NewModerationRepository()` call is removed.

---

## 6. Negative Search Matrix

| Symbol | Active refs (runtime) | Compile refs (non-dead) | Historical/doc refs | Verdict |
|--------|----------------------:|------------------------:|--------------------:|---------|
| `GetAppealWithCase` | 0 | 0 | 16 (docs) | SAFE TO DELETE |
| `ErrRestorationEventFailed` | 0 | 0 | 5 (docs) | SAFE TO DELETE |
| `ModerationRepository` | 0 | 0 (blank id) | 14 (code+docs) | SAFE TO DELETE |
| `ModerationRepositoryImpl` | 0 | 0 | 4 (docs) | SAFE TO DELETE |
| `NewModerationRepository()` | 0 (blank id) | 1 (blank id) | 3 (docs) | SAFE TO DELETE |
| `GovernanceCase` struct | 0 | 0 (dead paths only) | 42 (code+docs) | SAFE TO DELETE |
| `GovernanceCaseStatus` | 0 | 0 | 19 (internal+docs) | SAFE TO DELETE |
| `GovernanceCaseDecision` | 0 | 0 | 5 (internal+docs) | SAFE TO DELETE |
| `ErrAlreadyReviewed` (governance) | 0 | 0 | 4 (internal+docs) | SAFE TO DELETE |
| `ErrInvalidTransition` (governance) | 0 | 0 | 4 (internal+docs) | SAFE TO DELETE |
| `ErrEnforceRequiresNote` | 0 | 0 | 5 (internal+docs) | SAFE TO DELETE |
| `NewGovernanceCase()` | 0 | 0 | 2 (internal) | SAFE TO DELETE |
| `CanTransition(GovernanceCaseStatus)` | 0 | 0 | 73 (many other domains) | SAFE TO DELETE* |
| `mockModerationRepository` | 0 (never instantiated) | 1 (defined) | 3 (doc comments) | SAFE TO DELETE |
| `moderation_cases` (table) | 0 (dropped in 000056) | 0 | 20 (migrations+docs) | KEEP (already dropped) |

*Note: `CanTransition` is a common function name — the `GovernanceCase`-specific one at `governance_case.go:198` has a different signature from other domains' versions. Deleting `governance_case.go` removes only the GovernanceCase-specific overload.

---

## 7. Regression Baseline

| Test suite | Count | Result | Evidence |
|------------|------:|--------|----------|
| `go build ./...` | — | ✅ PASS | Clean build, exit code 0 |
| `go test ./internal/governance/moderation/application` | 24 | ✅ PASS | 0.758s |
| `go test ./internal/governance/moderation/delivery/http` | 7 | ✅ PASS | 0.315s |
| `go test ./internal/governance/moderation/entity` | 8 | ✅ PASS | 0.661s |
| `go test ./internal/governance/moderation/infrastructure/repository` | 1 | ✅ PASS | 0.640s |
| **Total Go unit tests** | **31** (all PASS) | ✅ PASS | |
| `npx vitest run` (admin) | 96 | ✅ PASS | 31 test files, 27.07s |
| `npx tsc --noEmit` (admin) | — | ✅ PASS | Clean typecheck |
| `dart analyze lib/domains/system/report/` | — | ✅ PASS | No issues found |

### Real PostgreSQL E2E (from prior verification, no code changes since):
| Test | Time | Result |
|------|------|--------|
| `TestAppealE2E_Reversal` | 115.6s | ✅ PASS |
| `TestAppealE2E_Upheld` | 115.7s | ✅ PASS |
| `TestAppealE2E_LateFailureAtomicity` | 115.8s | ✅ PASS |
| `TestAppealE2E_Concurrency` | 116.5s | ✅ PASS |
| `TestAppealE2E_CreateAppealOwnership` | 117.2s | ✅ PASS |
| `TestAppealE2E_WorkerRestorationPath` | 119.8s | ✅ PASS |

---

## 8. Final Verdict

### **PASS — ALL CANDIDATES ARE SAFE TO DELETE**

Every cleanup candidate has been proven through exhaustive dependency graph analysis:

- **`GetAppealWithCase`**: Zero callers. Dead compilation stub.
- **`ErrRestorationEventFailed`**: Zero consumers. Slice B completed its replacement.
- **`ModerationRepository` + `ModerationRepositoryImpl`**: Zero active consumers. Backing table dropped in migration 000056. Only referenced via blank identifier `_ = ...`.
- **`GovernanceCase` entity** (entire file, 239 lines): All 15+ exported symbols have zero external callers. Self-contained dead code.
- **`mockModerationRepository`**: Defined but never instantiated (zero `mockModerationRepository{}` in codebase).

**No active code path** depends on any of these candidates. Deletion will not break:
- Go compilation
- Appeal runtime
- Admin UI
- Mobile consumer
- Integration tests
- Migration tests

**Preserved artifacts:**
- Migration tests asserting table/schema absence (canonical regression)
- Historical audit reports (evidence trail)
- `moderationRepo` package import in `dependencies.go` (still needed for canonical repos)
- TypeScript `GovernanceCase` in admin frontend (consumes canonical `CanonicalCase`, different entity)

**Stale comments to fix (non-critical):**
- `canonical_case.go:21`: References removed `ErrCaseNotFound`
- `appeal_handler_test.go:671`: References `ModerationRepository` in description
