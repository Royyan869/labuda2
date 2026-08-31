# GOVERNANCE CONSUMER / UI AUDIT 1

**Date:** 2026-09-01
**Baseline:** 0be0305 (Slice 5 correctness fix)
**Scope:** Report → Case → Decision → Enforcement — consumer/UI truth audit
**Author:** Adversarial Audit Agent

---

## 1. EXECUTIVE VERDICT

**Verdict: BLOCKED — Backend is canonical, Admin UI is entirely legacy and disconnected.**

The canonical backend engine (Report, Case, Decision, Enforcement, Outbox) is production-safe and fully proven with 53/53 integration tests passing. However, the Admin UI is **entirely legacy** — it still references removed backend endpoints (`/api/v1/admin/moderation/cases`) and uses a non-canonical vocabulary (`moderation_cases`, `fixed_price_sale`, `chat_message`, `enforced` status). The admin cannot currently perform any canonical governance workflow through the UI.

**Mobile is CORRECT** — it uses the canonical Report contract (`POST /reports`) with the correct 5-target vocabulary.

---

## 2. BACKEND CONSUMER MAP

### 2a. Active Moderation Endpoints

| Endpoint | Method | Purpose | Status |
|----------|--------|---------|--------|
| `POST /reports` | POST | Create Report (canonical intake) | ✅ ACTIVE |
| `GET /reports/mine` | GET | List own Reports | ✅ ACTIVE |
| `GET /reports/:id` | GET | Get own Report | ✅ ACTIVE |
| `POST /appeals` | POST | Create Appeal | ✅ ACTIVE |
| `GET /appeals/me` | GET | List own Appeals | ✅ ACTIVE |
| `GET /appeals/:id` | GET | Get own Appeal | ✅ ACTIVE |
| `GET /warnings` | GET | List own Warnings | ✅ ACTIVE |
| `GET /warnings/:id` | GET | Get own Warning | ✅ ACTIVE |
| `GET /users/:id/warnings/active` | GET | Active warnings for user | ✅ ACTIVE |

### 2b. Active Admin Endpoints

| Endpoint | Method | Purpose | Status |
|----------|--------|---------|--------|
| `GET /admin/appeals` | GET | List all appeals | ✅ ACTIVE |
| `GET /admin/appeals/pending` | GET | Pending appeals queue | ✅ ACTIVE |
| `GET /admin/appeals/:id` | GET | Appeal detail | ✅ ACTIVE |
| `PUT /admin/appeals/:id/review` | PUT | Review appeal decision | ✅ ACTIVE |
| `GET /admin/warnings` | GET | List all warnings | ✅ ACTIVE |
| `POST /admin/warnings` | POST | Issue warning | ✅ ACTIVE |
| `DELETE /admin/warnings/:id/revoke` | DELETE | Revoke warning | ✅ ACTIVE |

### 2c. REMOVED (Legacy) Endpoints

| Endpoint | Method | Purpose | Status |
|----------|--------|---------|--------|
| `GET /admin/moderation/cases` | GET | List moderation cases | ❌ REMOVED |
| `GET /admin/moderation/cases/:id` | GET | Case detail | ❌ REMOVED |
| `GET /admin/moderation/cases/:id/evidence` | GET | Case evidence | ❌ REMOVED |
| `POST /admin/moderation/cases/:id/action` | POST | Case action | ❌ REMOVED |

**Evidence:** `backend/cmd/core_server/routes_core.go` lines 677-681:
```go
// ===== MODERATION ADMIN ROUTES — REMOVED (SLICE 2) =====
// The legacy admin Case review endpoints (ListCases/GetCase/
// GetCaseEvidence/ApplyAction) were backed by the rejected
// GovernanceCase runtime reading the dropped moderation_cases table.
// They are removed with that runtime. The canonical Case/Decision/
// Enforcement admin workflow is rebuilt in a later slice.
```

### 2d. Missing Canonical Admin Endpoints

The following endpoints do NOT exist but are required for the canonical governance workflow:

| Needed Endpoint | Purpose | Status |
|----------------|---------|--------|
| `GET /admin/cases` | List canonical Cases (from `cases` table) | ❌ MISSING |
| `GET /admin/cases/:id` | Case detail (canonical) | ❌ MISSING |
| `POST /admin/cases/:id/decision` | Create Decision for Case | ❌ MISSING |
| `GET /admin/cases/:id/decisions` | List Decisions for Case | ❌ MISSING |
| `GET /admin/decisions/:id` | Decision detail | ❌ MISSING |
| `GET /admin/decisions/:id/enforcement` | Enforcement status | ❌ MISSING |
| `POST /admin/enforcements/:id/retry` | Retry failed enforcement (if business rule allows) | ❌ MISSING |

---

## 3. ADMIN UI MAP

### 3a. Pages and Routes

| Page | Route | Status |
|------|-------|--------|
| ModerationCasesPage | `/moderation/cases` | 🔴 LEGACY — calls removed endpoints |
| AppealsPage | `/moderation/appeals` | ✅ CANONICAL — calls active endpoints |
| WarningsPage | `/moderation/warnings` | ✅ CANONICAL — calls active endpoints |
| CaseDetailModal | (modal) | 🔴 LEGACY — calls removed endpoints |
| AppealDetailModal | (modal) | ✅ CANONICAL |
| IssueWarningModal | (modal) | ✅ CANONICAL |

### 3b. ModerationCasesPage — Detailed Audit

**File:** `apps/admin/src/pages/ModerationCasesPage.tsx`

**CRITICAL:** This page is 100% legacy and disconnected from the canonical backend.

**Findings:**

| Finding | Evidence | Severity |
|---------|----------|----------|
| Calls removed endpoints | `getModerationCases()` → `GET /api/v1/admin/moderation/cases` (removed in Slice 2) | P0 |
| Uses `fixed_price_sale` resource type | `RESOURCE_TYPES` array includes `fixed_price_sale` (not canonical) | P1 |
| Uses `chat_message` resource type | `RESOURCE_TYPES` array includes `chat_message` (not canonical) | P1 |
| Uses legacy status vocabulary | Statuses: `pending`, `approved`, `rejected`, `enforced` | P1 |
| Uses legacy action vocabulary | Actions: `approve`, `reject`, `enforce` | P1 |
| Missing canonical statuses | No `open`, `resolved` case statuses | P1 |
| Missing enforcement display | No enforcement status/timeline | P2 |
| Missing Decision display | No Decision model in UI | P2 |

**Impact:** Admin clicking "Moderation" in the sidebar gets a page that makes API calls to endpoints that no longer exist. The page will display an error or empty state.

### 3c. CaseDetailModal — Detailed Audit

**File:** `apps/admin/src/components/moderation/CaseDetailModal.tsx`

| Finding | Evidence | Severity |
|---------|----------|----------|
| Calls removed detail endpoint | `useModerationCase()` → `GET /api/v1/admin/moderation/cases/:id` (removed) | P0 |
| Calls removed evidence endpoint | `getModerationCaseEvidence()` → `GET /api/v1/admin/moderation/cases/:id/evidence` (removed) | P0 |
| Calls removed action endpoint | `useCaseAction()` → `POST /api/v1/admin/moderation/cases/:id/action` (removed) | P0 |
| Uses `fixed_price_sale` vocabulary | `isMarketplaceResource` check | P1 |
| Uses `chat_message` vocabulary | `isChatMessage` checks, evidence panel | P1 |
| Uses `enforced` status | Status badge uses legacy vocabulary | P1 |
| Missing Decision model | No Decision display or creation | P2 |
| Missing Enforcement display | No enforcement status/timeline | P2 |

### 3d. AppealsPage and AppealDetailModal — Audit

**File:** `apps/admin/src/pages/AppealsPage.tsx`
**File:** `apps/admin/src/components/moderation/AppealDetailModal.tsx`

**Status: ✅ CANONICAL**

Both pages call active backend endpoints:
- `GET /admin/appeals` ✅
- `GET /admin/appeals/:id` ✅
- `PUT /admin/appeals/:id/review` ✅

Capability gates are correct:
- View: `moderation.appeal.read` ✅
- Review: `moderation.appeal.review` ✅

Data freshness checks (stale status detection) are implemented. ✅

**One minor finding:** The appeal detail shows `original_case.status` using legacy vocabulary (`moderationCaseStatusLabels`). Since the canonical `cases` table uses `open`/`resolved`, this label mapping is incorrect if the appeal references a canonical case.

### 3e. WarningsPage — Audit

**File:** `apps/admin/src/pages/WarningsPage.tsx`

**Status: ✅ CANONICAL**

Calls active endpoints:
- `GET /admin/warnings` ✅
- `POST /admin/warnings` ✅
- `DELETE /admin/warnings/:id/revoke` ✅

### 3f. Sidebar Navigation

**File:** `apps/admin/src/components/layout/Sidebar.tsx`

```
{ name: 'Moderation', path: '/moderation/cases', requiredCapability: 'moderation.case.read' }
{ name: 'Appeals', path: '/moderation/appeals', requiredCapability: 'moderation.appeal.read' }
{ name: 'Warnings', path: '/moderation/warnings', requiredCapability: 'moderation.case.read' }
```

| Entry | Target | Status |
|-------|--------|--------|
| Moderation | `/moderation/cases` | 🔴 Dead — page calls removed endpoints |
| Appeals | `/moderation/appeals` | ✅ Working |
| Warnings | `/moderation/warnings` | ✅ Working |

---

## 4. MOBILE UI MAP

### 4a. Report Flow

| Step | Status |
|------|--------|
| Report target type selection | ✅ Correct 5-target vocabulary (content, comment, for_sale, auction, user) |
| `chat_message` NOT in target enum | ✅ Correct — explicitly excluded |
| `fixed_price_sale` NOT in target enum | ✅ Correct — uses `for_sale` |
| Reason selection | ✅ Correct 7-reason locked taxonomy |
| Report submission | ✅ `POST /reports` (canonical endpoint) |
| My Reports list | ✅ `GET /reports/mine` |
| Appeal creation | ✅ `POST /appeals` |
| My Appeals list | ✅ `GET /appeals/me` |
| Warning viewing | ✅ `GET /warnings`, `GET /users/:id/warnings` |

### 4b. Report Target Matrix

| Target | Backend | Mobile UI | Admin Visibility | Status |
|--------|---------|-----------|------------------|--------|
| content | ✅ Accepted | ✅ Selectable | 🔴 No canonical admin view | P1 |
| comment | ✅ Accepted | ✅ Selectable | 🔴 No canonical admin view | P1 |
| for_sale | ✅ Accepted | ✅ Selectable | 🔴 No canonical admin view | P1 |
| auction | ✅ Accepted | ✅ Selectable | 🔴 No canonical admin view | P1 |
| user | ✅ Accepted | ✅ Selectable | 🔴 No canonical admin view | P1 |
| chat_message | ❌ Rejected | ❌ Not in enum | N/A | ✅ Correct |
| fixed_price_sale | ❌ Rejected | ❌ Not in enum | 🔴 Still in admin filter | P1 |

### 4c. Mobile Notification Types

**File:** `apps/mobile/lib/core/interfaces/i_notification_trigger.dart`

Uses correct canonical event types:
- `moderation.report.created` ✅
- `moderation.case.opened` ✅
- `moderation.decision.violation` ✅
- `moderation.enforcement.succeeded` ✅
- `moderation.enforcement.failed` ✅
- `moderation.appeal.submitted` ✅
- `moderation.appeal.decision` ✅

---

## 5. END-TO-END FLOW TRACE

### 5a. Complete Governance Flow

```
User submits Report (Mobile)
  → POST /reports → ReportHandler.CreateReport → reports table ✅
  → Case correlation → cases table ✅
  → Admin sees Case → ❌ BLOCKED (admin UI calls removed endpoint)
  → Admin creates Decision → ❌ BLOCKED (no admin endpoint)
  → Decision persisted → decisions table ✅
  → Enforcement created → enforcements table ✅
  → Outbox emitted → outbox_events table ✅
  → Worker executes → target mutation ✅
  → Enforcement updated → enforcements table ✅
  → Admin observes result → ❌ BLOCKED (no admin endpoint)
```

**Broken chain:** Steps 3, 4, and 10 are blocked because the admin backend endpoints were removed in Slice 2 and the canonical admin workflow was never rebuilt.

### 5b. Working Flows

| Flow | Status |
|------|--------|
| User submits Report | ✅ |
| User views own Reports | ✅ |
| User creates Appeal | ✅ |
| User views own Appeals | ✅ |
| User views own Warnings | ✅ |
| Admin reviews Appeals | ✅ |
| Admin issues/revokes Warnings | ✅ |
| Backend Decision→Enforcement pipeline | ✅ (but no admin UI) |
| Backend Outbox delivery | ✅ |
| Backend Worker execution | ✅ |

---

## 6. UI TRUTH FINDINGS

### P0 — Blocking

| ID | Finding | Evidence |
|----|---------|----------|
| F1 | ModerationCasesPage calls removed backend endpoints | `getModerationCases()` → `GET /api/v1/admin/moderation/cases` which does not exist |
| F2 | CaseDetailModal calls removed backend endpoints | `useModerationCase()` → `GET /api/v1/admin/moderation/cases/:id` (removed) |
| F3 | `executeCaseAction()` calls removed endpoint | `POST /api/v1/admin/moderation/cases/:id/action` (removed) |
| F4 | `getModerationCaseEvidence()` calls removed endpoint | `GET /api/v1/admin/moderation/cases/:id/evidence` (removed) |

**Impact:** The "Moderation" link in the admin sidebar leads to a page that cannot function. Every action on that page will fail with API errors. This is a zombie UI — it appears to work (renders, shows loading, shows empty/error state) but cannot perform any governance action.

### P1 — Incorrect Vocabulary

| ID | Finding | Evidence |
|----|---------|----------|
| F5 | `fixed_price_sale` in admin resource type filter | `RESOURCE_TYPES` array in ModerationCasesPage |
| F6 | `chat_message` in admin resource type filter | `RESOURCE_TYPES` array in ModerationCasesPage |
| F7 | `enforced` in status vocabulary | `CASE_STATUSES` array: `pending`, `approved`, `rejected`, `enforced` |
| F8 | `approve`/`reject`/`enforce` action vocabulary | `CaseAction` type, `handleAction` |
| F9 | `fixed_price_sale` in admin moderation types | `ResourceType` in `apps/admin/src/types/moderation.ts` |
| F10 | `chat_message` in admin moderation types | `ResourceType` in `apps/admin/src/types/moderation.ts` |
| F11 | `enforced` in `ModerationCaseStatus` type | Type definition in `apps/admin/src/types/moderation.ts` |

### P2 — Missing Capability

| ID | Finding | Evidence |
|----|---------|----------|
| F12 | No admin UI to list canonical Cases | No page calls `GET /admin/cases` (endpoint doesn't exist) |
| F13 | No admin UI to create Decisions | No page calls `POST /admin/cases/:id/decision` (endpoint doesn't exist) |
| F14 | No admin UI to view Enforcement status | No enforcement status display anywhere in admin |
| F15 | No admin UI to view enforcement retry/error state | No retry/error display |
| F16 | Appeal detail shows legacy case status labels | `original_case.status` rendered with `moderationCaseStatusLabels` |

---

## 7. AUDIT TRAIL FINDINGS

| Area | Status | Evidence |
|------|--------|----------|
| Report creation audit | ⚠️ No `audit_events` write | Report handler persists to `reports` table only |
| Case creation audit | ⚠️ No `audit_events` write | Case correlation in DecisionService writes to `cases` table |
| Decision creation audit | ⚠️ No `audit_events` write | DecisionService writes to `decisions` table |
| Enforcement lifecycle audit | ⚠️ No `audit_events` write | Enforcement transitions persisted to `enforcements` table |
| Admin governance action audit | ⚠️ No `audit_events` write | `governance_admin_actions` table used (separate from `audit_events`) |

**Assessment:** The enforcement lifecycle states ARE persisted and queryable from the database, which provides a factual audit trail. However, there is no dedicated append-only governance audit event stream. The `audit_events` table exists in the schema but moderation lifecycle transitions do not write to it.

**Severity:** P2 for correctness (states are persisted), P1 for governance compliance if regulatory audit trail is required.

---

## 8. LEGACY / RESIDUE CLASSIFICATION

| Artifact | Classification | Evidence |
|----------|---------------|----------|
| `GovernanceCase` entity | FUTURE DEPENDENCY | Used by Appeal system (`original_case` context) |
| `DomainAction` type | DEAD/ZOMBIE | Worker parked, never instantiated by any active path |
| `moderation_cases` table | DEAD/ZOMBIE | Referenced by removed legacy endpoints, not by canonical runtime |
| `chat_message` handler | LEGACY RESIDUE | Handler exists in worker, but no producer can invoke it |
| `chat_message` in admin types | LEGACY RESIDUE | `ResourceType` includes it; DB enum test asserts it must NOT be in enum |
| `fixed_price_sale` in admin types | LEGACY RESIDUE | `ResourceType` includes it; canonical backend uses `for_sale` |
| `enforced` status in admin UI | LEGACY RESIDUE | Not a canonical case status; canonical is `open`/`resolved` |
| `ModerationCasesPage` | LEGACY RESIDUE | Entirely calls removed endpoints |
| `CaseDetailModal` | LEGACY RESIDUE | Entirely calls removed endpoints |
| `useModeration` hooks | LEGACY RESIDUE | Call removed endpoints |
| `getModerationCases` API client | LEGACY RESIDUE | Calls removed endpoint |
| Admin types (`moderation.ts`) | LEGACY RESIDUE | Mixed canonical + legacy vocabulary |
| `ModerationRepository` | FUTURE DEPENDENCY | Used by Appeal system |

---

## 9. MISSING CAPABILITIES

### Admin Governance Workflow (P0)

To close the governance loop, the admin needs:

1. **Case List Page** — List canonical cases from `cases` table (not `moderation_cases`)
2. **Case Detail Page** — View case with related Reports, existing Decisions, Enforcement status
3. **Decision Creation** — Select canonical outcome (`violation`, `no_violation`, `dismissed`), provide reason, target type + target ID
4. **Enforcement Display** — Show enforcement lifecycle (pending → processing → succeeded/failed), attempt count, errors, retry status
5. **Decision Immutability** — Once created, Decision cannot be edited or deleted (already enforced backend-side)

### Admin Backend Endpoints Needed (P0)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /admin/governance/cases` | GET | List Cases (open/resolved) |
| `GET /admin/governance/cases/:id` | GET | Case detail with Reports + Decisions |
| `POST /admin/governance/cases/:id/decisions` | POST | Create Decision |
| `GET /admin/governance/decisions/:id` | GET | Decision detail |
| `GET /admin/governance/decisions/:id/enforcement` | GET | Enforcement status |

### Admin Enforcement Visibility (P1)

| Capability | Description |
|-----------|-------------|
| Enforcement status | See pending/processing/succeeded/failed per Decision |
| Enforcement attempts | See attempt_count, last_error, next_attempt_at |
| Retry visibility | See when next retry will occur |

### Audit Trail (P2)

| Capability | Description |
|-----------|-------------|
| Governance action log | Append-only record of admin decisions + enforcement results |
| `audit_events` integration | Decision creation, Enforcement transitions |

---

## 10. RECOMMENDED IMPLEMENTATION SLICES

### Slice 6 — Admin Governance Backend (P0)

Build canonical admin endpoints:
- `GET /admin/governance/cases` — list Cases
- `GET /admin/governance/cases/:id` — Case detail
- `POST /admin/governance/cases/:id/decisions` — create Decision
- `GET /admin/governance/decisions/:id` — Decision detail
- `GET /admin/governance/decisions/:id/enforcement` — Enforcement status

**Dependencies:** Canonical Case, Decision, Enforcement runtime (all complete in Slice 5).

### Slice 7 — Admin Governance UI Rebuild (P0)

Rebuild admin moderation pages:
- Replace `ModerationCasesPage` with canonical Case list page
- Replace `CaseDetailModal` with canonical Case detail + Decision creation
- Add Enforcement status display
- Remove legacy vocabulary (`fixed_price_sale`, `chat_message`, `enforced`)
- Update types to canonical vocabulary

**Dependencies:** Slice 6 (backend endpoints).

### Slice 8 — Admin Governance Audit Trail (P1)

- Wire Decision creation to `audit_events`
- Wire Enforcement lifecycle transitions to `audit_events`
- Add governance action log display in admin UI

### Slice 9 — Legacy Cleanup (P2)

After Slices 6-7 are verified:
- Remove `ModerationCasesPage.tsx`
- Remove `CaseDetailModal.tsx`
- Remove `useModeration.ts` hooks
- Remove `getModerationCases` API client
- Remove `chat_message` handler from worker (if confirmed dead)
- Clean `ResourceType` to canonical 5-target vocabulary
- Remove `enforced` from `ModerationCaseStatus`

---

## 11. CLEANUP CANDIDATES

| File | Action | Prerequisite |
|------|--------|--------------|
| `ModerationCasesPage.tsx` | Delete after Slice 7 | Slice 7 complete |
| `CaseDetailModal.tsx` | Delete after Slice 7 | Slice 7 complete |
| `useModeration.ts` | Delete after Slice 7 | Slice 7 complete |
| `lib/api/moderation.ts` | Rewrite after Slice 6-7 | Slice 6-7 complete |
| `types/moderation.ts` | Rewrite after Slice 7 | Slice 7 complete |
| `handleChatMessageHidden` in worker | Delete after confirmation | Confirm no producer |

---

## 12. BLOCKERS

| Blocker | Impact | Resolution |
|---------|--------|------------|
| No admin backend endpoints for canonical Case/Decision/Enforcement | Admin cannot perform governance workflow | Slice 6 |
| Admin UI calls removed endpoints | "Moderation" page is non-functional | Slice 7 |
| Legacy vocabulary in admin types | Future admin UI will need type rewrite | Slice 7 |

---

## 13. TEST EVIDENCE

### Backend (Real PostgreSQL)

| Test Suite | Count | Status |
|-----------|-------|--------|
| TestCanonicalReportRuntime | 11/11 | PASS |
| TestCanonicalCaseRuntime | 8/8 | PASS |
| TestCanonicalDecisionRuntime | 9/9 | PASS |
| TestEnforcementRuntime | 16/16 | PASS |
| TestOutboxRetryLifecycle | 7/7 | PASS |
| TestOutboxConcurrentClaimRaceSafety | 2/2 | PASS |
| **Total** | **53/53** | **PASS** |

### Admin UI

| Test Suite | Status |
|-----------|--------|
| CaseDetailModal.test.tsx | PASS (but tests mock removed endpoints) |
| AppealDetailModal.test.tsx | PASS |
| AppealsPage.test.tsx | PASS |
| useModeration.test.tsx | PASS (but tests mock removed endpoints) |
| useAppeals.test.tsx | PASS |
| App.test.tsx | PASS (route guards) |

**Note:** Admin UI tests pass because they mock the API layer. The mocks mask the fact that the real endpoints no longer exist.

---

## 14. GIT STATUS

Clean — only pre-existing `.commandcode/taste/` changes. No code modifications in this audit.
