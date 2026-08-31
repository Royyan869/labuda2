# GOVERNANCE ADMIN UI IMPLEMENTATION — SLICE 7

**Date:** 2026-09-01
**Baseline:** 58642a8 (Slice 6 admin governance backend)
**Status:** PASS

---

## 1. UI Architecture

```
Canonical Admin Governance UI
├── types/governance.ts          — canonical types (Case, Decision, Enforcement)
├── lib/api/governance.ts        — API client for /admin/governance/* endpoints
├── hooks/useGovernance.ts       — React hooks (case list, case detail, create decision)
├── pages/GovernanceCasesPage.tsx — case list with status filter + pagination
├── pages/GovernanceCaseDetailPage.tsx — case detail + reports + decisions + enforcement + decision form
├── App.tsx                      — routing (governance pages replace legacy)
└── types/index.ts               — barrel export
```

**No legacy moderation APIs called.** All API calls go to `/admin/governance/*`.

---

## 2. Case List

**Page:** `GovernanceCasesPage` at route `/moderation/cases`

| Feature | Status |
|---------|--------|
| Displays Case ID, Subject Type, Subject ID, Status, Created, Updated | ✅ |
| Status filter: All / Open / Resolved | ✅ |
| Pagination with real API counts | ✅ |
| Loading state | ✅ |
| Error state (truthful — shows API error message, not fake "no cases") | ✅ |
| Empty state (differentiates zero results from error) | ✅ |
| View button → navigates to Case Detail | ✅ |

**Filters use canonical vocabulary:** `open`, `resolved` (NOT `pending`, `approved`, `rejected`, `enforced`).

---

## 3. Case Detail

**Page:** `GovernanceCaseDetailPage` at route `/moderation/cases/:id`

Displays:
- Case ID, Subject Type, Subject ID, Status, Created, Updated, Closed
- Reports associated with the Case
- Decisions with Enforcement status
- Decision creation form (when Case is open)

**Uses canonical API:** `GET /admin/governance/cases/:id`

---

## 4. Reports

Reports are displayed within Case Detail:

| Field | Displayed |
|-------|-----------|
| Report ID | ✅ |
| Reporter ID | ✅ |
| Reason Code | ✅ |
| Reason Note | ✅ |
| Evidence Snapshot (author, title, status) | ✅ |
| Created At | ✅ |

No fake fields. Only fields actually returned by the backend.

---

## 5. Decision History

Every Decision associated with the Case is displayed:

| Field | Displayed |
|-------|-----------|
| Decision ID | ✅ |
| Decided By | ✅ |
| Outcome | ✅ (`no_violation` / `violation`) |
| Decision Note | ✅ |
| Created At | ✅ |
| Enforcement (for violation decisions) | ✅ |

**Uses canonical vocabulary:** `no_violation`, `violation` (NOT `approved`, `rejected`, `enforced`).

**Decision is visually immutable:** No edit/delete controls. Decision card is read-only.

---

## 6. Decision Creation

**Form:** Inline card within Case Detail, shown only when Case is `open`.

| Feature | Status |
|---------|--------|
| Outcome selector: No Violation / Violation | ✅ |
| Target Type (violation only): content, comment, for_sale, auction, user | ✅ |
| Target ID (violation only): UUID input | ✅ |
| Decision Note: optional textarea (max 2000) | ✅ |
| Validation: target_type + target_id required for violation | ✅ |
| Submit calls `POST /admin/governance/cases/:id/decisions` | ✅ |
| Loading state during submission | ✅ |
| Error state (truthful — shows API error message) | ✅ |
| On success: refreshes Case, shows new Decision + Enforcement | ✅ |
| No optimistic fake Decision appended | ✅ |

**Uses canonical vocabulary:** `no_violation`, `violation` (NOT `approve`, `reject`, `enforce`).

---

## 7. Enforcement Display

For violation Decisions, Enforcement status is shown inline:

| Field | Displayed |
|-------|-----------|
| Status | ✅ (`pending`, `processing`, `succeeded`, `failed`) |
| Target Type | ✅ |
| Target ID | ✅ |
| Attempt Count | ✅ |
| Last Error | ✅ (truncated to 50 chars with tooltip) |

**Uses canonical vocabulary:** `pending`, `processing`, `succeeded`, `failed` (NOT `enforced`).

**No retry button** — correct, backend does not expose manual retry.

---

## 8. Sidebar / Navigation

| Entry | Path | Capability | Status |
|-------|------|-----------|--------|
| Moderation | `/moderation/cases` | `moderation.case.read` | ✅ Now renders canonical `GovernanceCasesPage` |
| Appeals | `/moderation/appeals` | `moderation.appeal.read` | ✅ Unchanged, working |
| Warnings | `/moderation/warnings` | `moderation.case.read` | ✅ Unchanged, working |

No sidebar changes required — existing paths and capabilities are correct.

---

## 9. Authorization

| Route | Capability | Status |
|-------|-----------|--------|
| `/moderation/cases` | `moderation.case.read` | ✅ Via RequireCapability wrapper |
| `/moderation/cases/:id` | `moderation.case.read` | ✅ Via RequireCapability wrapper |
| Create Decision | `moderation.case.resolve` | ⚠️ Backend enforces; UI does not gate button (relies on backend 403) |

**Note:** Create Decision button is visible when Case is open. Backend enforces `moderation.case.resolve` capability. If unauthorized, backend returns 403 and UI shows error.

---

## 10. Error / Loading / Empty States

| State | Behavior |
|-------|----------|
| Loading | Spinner + "Loading..." message |
| API error | Error message + Retry button (truthful) |
| Empty (zero cases) | "No Cases Found" with filter-appropriate message |
| Case not found | "Case not found" message |
| Decision creation error | Red banner with API error message |

**No state is faked.** Loading ≠ Empty ≠ Error.

---

## 11. Legacy UI Replacement

| Legacy File | Status |
|-------------|--------|
| `ModerationCasesPage.tsx` | ⚠️ DEAD CODE — no longer imported by App.tsx |
| `CaseDetailModal.tsx` | ⚠️ DEAD CODE — only imported by ModerationCasesPage |
| `useModeration.ts` | ⚠️ DEAD CODE — only imported by ModerationCasesPage + CaseDetailModal |
| `lib/api/moderation.ts` | ⚠️ STILL ACTIVE — Appeals/Warnings use `getAppeals`, `getWarnings`, etc. |
| `types/moderation.ts` | ⚠️ STILL ACTIVE — Appeals/Warnings types used |

**Decision:** Legacy Case-related files are dead code but not deleted in this slice to avoid breaking Appeals/Warnings which share the same `moderation.ts` API module and `moderation.ts` types. Cleanup is deferred to dedicated cleanup slice.

**Canonical active UI has ZERO legacy vocabulary** for Case/Decision/Enforcement.

---

## 12. Real API Verification

The canonical UI is built against the verified Slice 6 backend endpoints:

| Endpoint | Backend | UI Consumer |
|----------|---------|-------------|
| `GET /admin/governance/cases` | ✅ Slice 6 | ✅ `GovernanceCasesPage` |
| `GET /admin/governance/cases/:id` | ✅ Slice 6 | ✅ `GovernanceCaseDetailPage` |
| `POST /admin/governance/cases/:id/decisions` | ✅ Slice 6 | ✅ `useCreateDecision` hook |
| `GET /admin/governance/decisions/:id` | ✅ Slice 6 | Available (not yet wired to separate page) |
| `GET /admin/governance/decisions/:id/enforcement` | ✅ Slice 6 | Available (enforcement shown inline in Case Detail) |

**Backend endpoints proven with 12/12 integration tests against real PostgreSQL (Slice 6).**

---

## 13. End-to-End Verification

The governance workflow is now:

```
Mobile User submits Report → POST /reports → Report persisted
    ↓
Case correlation → cases table
    ↓
Admin opens /moderation/cases → GovernanceCasesPage
    ↓
Admin clicks "View" → GovernanceCaseDetailPage
    ↓
Admin sees Reports → real data from GET /admin/governance/cases/:id
    ↓
Admin clicks "Create Decision" → fills form → POST /admin/governance/cases/:id/decisions
    ↓
DecisionService creates Decision + Enforcement + Outbox atomically
    ↓
Page refreshes → shows new Decision + Enforcement status
    ↓
Worker executes outbox event → target mutation → Enforcement updated
```

**All steps use real backend endpoints and real PostgreSQL data.**

---

## 14. Tests

| Suite | Count | Status |
|-------|-------|--------|
| Admin UI unit tests (vitest) | 99/99 | PASS |
| Backend integration tests (Slice 6) | 12/12 | PASS |
| Backend build | ✅ | PASS |
| Admin build (tsc + vite) | ✅ | PASS |

---

## 15. Build / Typecheck / Lint

| Check | Status |
|-------|--------|
| `tsc -b` (TypeScript) | ✅ PASS |
| `vite build` | ✅ PASS |
| `vitest run` | ✅ 99/99 PASS |
| `go build ./...` (backend) | ✅ PASS |

---

## 16. Remaining Findings

| Finding | Severity | Notes |
|---------|----------|-------|
| Legacy ModerationCasesPage.tsx is dead code | P3 | Cleanup deferred to dedicated cleanup slice |
| Legacy CaseDetailModal.tsx is dead code | P3 | Cleanup deferred |
| Legacy useModeration.ts is dead code | P3 | Cleanup deferred |
| No Decision detail page (standalone) | P3 | Decisions shown inline in Case Detail; standalone page optional |
| Create Decision button not capability-gated in UI | P2 | Backend enforces `moderation.case.resolve`; UI shows button for all case.read users |

---

## 17. Cleanup Candidates

| File | Action | Prerequisite |
|------|--------|--------------|
| `ModerationCasesPage.tsx` | Delete | Confirm no other imports |
| `CaseDetailModal.tsx` | Delete | Confirm no other imports |
| `CaseDetailModal.test.tsx` | Delete | Depends on CaseDetailModal |
| `useModeration.ts` | Delete | Confirm no other imports |
| `useModeration.test.tsx` | Delete | Depends on useModeration |
| `lib/api/moderation.ts` | Trim Case-related functions | Appeals/Warnings functions must remain |
| `lib/api/moderation.test.ts` | Update to remove Case-related tests | Keep Appeals/Warnings tests |
| `types/moderation.ts` | Trim Case-related types | Appeals/Warnings types must remain |

**Note:** `lib/api/moderation.ts` and `types/moderation.ts` CANNOT be deleted because Appeals and Warnings still use them. Only Case-related exports should be removed in cleanup.

---

## 18. Files Changed

| File | Change |
|------|--------|
| `types/governance.ts` | **NEW** — canonical Case/Decision/Enforcement types |
| `lib/api/governance.ts` | **NEW** — governance API client |
| `hooks/useGovernance.ts` | **NEW** — governance React hooks |
| `pages/GovernanceCasesPage.tsx` | **NEW** — canonical case list page |
| `pages/GovernanceCaseDetailPage.tsx` | **NEW** — canonical case detail + decision form |
| `App.tsx` | **MOD** — routes to new canonical pages |
| `types/index.ts` | **MOD** — exports governance types |
| `lib/api/index.ts` | **MOD** — exports governance API |

---

## 19. Verdict

**PASS**

- Canonical UI built against verified Slice 6 backend ✅
- Zero legacy vocabulary in active canonical UI ✅
- Real API contract (no mocked endpoints) ✅
- Truthful states (loading/empty/error) ✅
- No fake capabilities (no retry button, no edit/delete Decision) ✅
- All 99 admin tests pass ✅
- Backend builds clean ✅
