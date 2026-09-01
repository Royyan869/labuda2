# APPEAL FORENSIC AUDIT 1 — READ-ONLY REPORT

- **Date:** 2026-09-01
- **Mode:** READ-ONLY FORENSIC AUDIT — no code changes, no deletions, no migrations
- **Scope:** Appeal domain, GovernanceCase dependency, ModerationRepository dependency, reversal behavior, Admin API/UI, tests, authorization, audit trail, money/commerce safety
- **Authority:** `LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1.md`, `LABUDA — CANONICAL MODERATION DESIGN v1.md`, `LABUDA — CANONICAL MODERATION SPECIFICATION v1.md`
- **Evidence rule:** Every claim includes `file:line` evidence

---

## CRITICAL FINDING: SCHEMA–CODE MISMATCH

**Migration 000055 already changed the `appeals` table schema to the canonical model, but ALL Go code still implements the legacy model.**

| Layer | Current DB schema (migration 000055) | Current Go code | Status |
|---|---|---|---|
| Column | `decision_id uuid NOT NULL` (FK → decisions) | `Appeal.CaseID` mapped to `report_id` | **BROKEN** |
| FK | `appeals_decision_id_fkey → decisions(id)` | `ModerationRepository.GetByID → moderation_cases` | **BROKEN** |
| Constraint | `CHECK (decision_id IS NOT NULL)` | Service checks `GovernanceCase.Status` | **BROKEN** |
| Index | `idx_appeals_decision_id` | SQL queries reference `report_id` | **BROKEN** |

**The current Go Appeal code is RUNTIME-DEAD against the current database schema.** Every appeal operation (Create, Get, List, Review) would fail with a column-not-found or table-not-found error at runtime.

---

## 1. CURRENT APPEAL RUNTIME FLOW

### 1.1 Endpoints (active, registered in routes)

**User routes** (`backend/cmd/core_server/routes_core.go:1330-1333`):
```
POST /api/v1/appeals          → AppealHandler.CreateAppeal
GET  /api/v1/appeals/:id      → AppealHandler.GetAppeal
GET  /api/v1/appeals/me       → AppealHandler.ListMyAppeals
```

**Admin routes** (`backend/cmd/core_server/routes_core.go:919-932`):
```
GET  /admin/appeals           → AppealHandler.AdminListAppeals      (moderation.appeal.read)
GET  /admin/appeals/pending   → AppealHandler.AdminListPendingAppeals (moderation.appeal.read)
GET  /admin/appeals/:id       → AppealHandler.AdminGetAppeal        (moderation.appeal.read)
PUT  /admin/appeals/:id/review → AppealHandler.AdminReviewAppeal    (moderation.appeal.review)
```

### 1.2 Create Appeal Flow (would fail at runtime)

```
HTTP POST /api/v1/appeals
  → AppealHandler.CreateAppeal              (delivery/http/appeal_handler.go)
  → AppealService.CreateAppeal              (application/appeal_service.go)
    → ModerationRepository.GetByID(caseID)  → SQL: SELECT ... FROM moderation_cases WHERE id=$1
      ❌ RUNTIME ERROR: relation "moderation_cases" does not exist (dropped by migration 000056)
    → AppealRepository.CreateWithPendingCheck(appeal)
      ❌ RUNTIME ERROR: column "report_id" does not exist (dropped by migration 000055)
```

### 1.3 Review Appeal Flow (would fail at runtime)

```
HTTP PUT /admin/appeals/:id/review
  → AppealHandler.AdminReviewAppeal        (delivery/http/appeal_handler.go)
  → AppealService.ReviewAppeal             (application/appeal_service.go)
    → AppealRepository.GetForUpdate(appealID)
      ❌ RUNTIME ERROR: column "report_id" does not exist
    → ModerationRepository.GetByID(caseID) → moderation_cases
      ❌ RUNTIME ERROR: relation "moderation_cases" does not exist
    → OutboxRepository.InsertEvent("moderation.<type>.restored", ...)
    → Appeal.Approve() / Appeal.Reject()
    → AppealRepository.Update(appeal)
      ❌ RUNTIME ERROR: column "report_id" does not exist
```

### 1.4 Restoration Flow (would fail at runtime if reached)

```
ReviewAppeal (approved, content/comment type)
  → OutboxRepository.InsertEvent("moderation.content.restored", resourceID, payload)
  → ModerationEventHandler.Handle()        (worker/moderation_event_handler.go)
    → handleRestoration()
      → ContentService.RestoreFromModeration() or CommentService.RestoreFromModeration()
  → NotificationWorker: "moderation.content.restored" notification to author
```

This flow is correctly designed but cannot be reached because the upstream appeal operations fail.

---

## 2. APPEAL SCHEMA

### 2.1 Original Schema (migration 000001)

```sql
CREATE TABLE appeals (
    id            uuid DEFAULT gen_random_uuid() NOT NULL,
    report_id     uuid NOT NULL,          -- misleadingly named; stored CaseID
    appealed_by   uuid NOT NULL,
    message       text NOT NULL,
    status        text DEFAULT 'pending' NOT NULL,
    reviewed_by   uuid,
    admin_response text,
    reviewed_at   timestamptz,
    created_at    timestamptz DEFAULT now() NOT NULL,
    updated_at    timestamptz DEFAULT now() NOT NULL,
    deleted_at    timestamptz
);
```

### 2.2 Current Schema (after migration 000055)

```sql
-- Changes applied by 000055_canonical_moderation_foundation.up.sql:185-196:
ALTER TABLE appeals DROP COLUMN IF EXISTS report_id;       -- legacy column removed
ALTER TABLE appeals ADD COLUMN decision_id uuid;           -- canonical FK added
ALTER TABLE appeals ADD CONSTRAINT appeals_decision_id_fkey FOREIGN KEY (decision_id) REFERENCES decisions(id);
ALTER TABLE appeals ADD CONSTRAINT appeals_decision_id_required CHECK (decision_id IS NOT NULL);
ALTER TABLE appeals ADD CONSTRAINT appeals_status_check CHECK (status IN ('pending', 'approved', 'rejected'));
CREATE INDEX idx_appeals_decision_id ON appeals USING btree (decision_id);
```

**Result:**
| Column | Type | Constraints |
|---|---|---|
| `id` | uuid | PK, gen_random_uuid() |
| `appealed_by` | uuid | NOT NULL |
| `message` | text | NOT NULL |
| `status` | text | NOT NULL, CHECK IN ('pending','approved','rejected') |
| `reviewed_by` | uuid | nullable |
| `admin_response` | text | nullable |
| `reviewed_at` | timestamptz | nullable |
| `created_at` | timestamptz | NOT NULL, DEFAULT now() |
| `updated_at` | timestamptz | NOT NULL, DEFAULT now() |
| `deleted_at` | timestamptz | nullable |
| `decision_id` | uuid | NOT NULL, FK → decisions(id) |

**Critical observations:**
- `report_id` column **DOES NOT EXIST** anymore — dropped by migration 000055
- `decision_id` column is the **ONLY** reference column — NOT NULL, FK to `decisions(id)`
- No FK to `moderation_cases` (which was itself dropped by migration 000056)
- The schema is ALREADY at the canonical target state

### 2.3 Go Entity vs DB Schema

**Go entity** (`entity/appeal.go:19`):
```go
type Appeal struct {
    ID         uuid.UUID
    CaseID     uuid.UUID   // maps to: ??? (report_id dropped, decision_id exists)
    AppealedBy uuid.UUID
    Status     AppealStatus
    Message    string
    AdminResponse *string
    ReviewedBy    *uuid.UUID
    CreatedAt     time.Time
    ReviewedAt    *time.Time
}
```

**Repository SQL** (`infrastructure/repository/appeal_repository_impl.go:61`):
```sql
INSERT INTO appeals (id, report_id, appealed_by, status, ...)
```

**MISMATCH:** `report_id` column was dropped by migration 000055. The SQL would fail.

---

## 3. APPEAL → GOVERNANCE RELATIONSHIP

### 3.1 Current DB: Appeal → Decision (ALREADY MIGRATED)

Migration 000055 established:
```text
appeals.decision_id  →  decisions(id)
```

This is the canonical relationship per Business Truth §24:
> "Appeal adalah challenge terhadap Decision."

### 3.2 Current Go Code: Appeal → GovernanceCase (LEGACY, BROKEN)

The Go code still models:
```text
Appeal.CaseID  →  GovernanceCase (via ModerationRepository.GetByID)
GovernanceCase  →  moderation_cases (DROPPED TABLE)
```

### 3.3 Canonical Design (Business Truth §24)

```text
Case
  ↓
Decision
  ↓
  Appeal
  ↓
Decision #2 (new Decision via appeal review)
```

The canonical model is:
- Appeal points to a specific Decision (NOT Case, NOT Report)
- Appeal review produces a NEW Decision record (append-only, immutable)
- Appeal does NOT mutate the original Decision
- Multiple Appeals can target the same Decision

---

## 4. GOVERNANCECASE DEPENDENCY MAP

### 4.1 Where GovernanceCase is Used in Appeal

| File | Usage | What it reads |
|---|---|---|
| `entity/appeal.go:111` | `ErrCaseNotAppealable` uses `GovernanceCaseStatus` | Type reference only |
| `application/appeal_service.go:27` | `moderationRepo ModerationRepository` field | Dependency injection |
| `application/appeal_service.go:100` | `moderationRepo.GetByID(ctx, tx, caseID)` in `CreateAppeal` | Reads GovernanceCase to check status + resource type |
| `application/appeal_service.go:107` | `kase.Status != GovernanceCaseStatusEnforced && kase.Status != GovernanceCaseStatusRejected` | Decision outcome (encoded in legacy case status) |
| `application/appeal_service.go:111` | `kase.ResourceType`, `kase.ResourceID` | Subject type + ID |
| `application/appeal_service.go:117` | `getResourceOwner(ctx, tx, kase.ResourceType, kase.ResourceID)` | Owner lookup |
| `application/appeal_service.go:173-201` | `getResourceOwner` switch on `ResourceType` | Content/Comment/ForSale/Auction/User owner |
| `application/appeal_service.go:274` | `GetAppealWithCase` calls `moderationRepo.GetByID(appeal.CaseID)` | Returns case context to admin UI |
| `application/appeal_service.go:369` | `ReviewAppeal` calls `moderationRepo.GetByID(appeal.CaseID)` | Checks if enforced + auto-restorable type |
| `delivery/http/appeal_handler.go:268` | `governanceCaseToContext(originalCase)` | Maps case to admin response |
| `delivery/http/appeal_handler.go:308` | `mapStatusToDecision(kase.Status)` | Maps legacy status to display |

### 4.2 What Appeal Actually Needs from GovernanceCase

| Appeal need | GovernanceCase field used | Canonical replacement |
|---|---|---|
| Is this appealable? | `GovernanceCase.Status` (must be enforced or rejected) | `decisions.outcome` for latest decision OR `cases.status = 'resolved'` |
| What was the decision? | `GovernanceCase.Status` (= enforced means violation) | `decisions.outcome` |
| What resource type? | `GovernanceCase.ResourceType` | `cases.subject_type` |
| What resource ID? | `GovernanceCase.ResourceID` | `cases.subject_id` |
| Who is the owner? | Derived from ResourceType + ResourceID | Same (via content/comment/etc repos) |
| Should auto-restore? | `kase.Status == GovernanceCaseStatusEnforced` | Decision outcome = violation_confirmed + enforcement succeeded |
| Original case context for admin | `GovernanceCase` fields | Join: `decisions → cases` + `decisions → enforcements` |

**Key insight:** Appeal does NOT actually need "Case state" — it needs:
1. **Decision outcome** (was it a violation? which decision specifically?)
2. **Subject type + ID** (which resource?)
3. **Enforcement state** (was it enforced? for auto-restore eligibility)

### 4.3 Classification

| Dependency | Classification |
|---|---|
| `GovernanceCase.Status` check | **REBUILD** — must become `decisions.outcome` + enforcement state |
| `GovernanceCase.ResourceType/ID` | **REBUILD** — must come from `cases.subject_type/subject_id` via decision→case join |
| `GovernanceCase` entity type | **REMOVE** after rebuild |
| `GovernanceCase` in error types | **REMOVE** after rebuild |

---

## 5. MODERATIONREPOSITORY DEPENDENCY MAP

### 5.1 Interface

```go
// infrastructure/repository/moderation_repository.go
type ModerationRepository interface {
    GetByID(ctx context.Context, tx interface{}, caseID uuid.UUID) (*entity.GovernanceCase, error)
}
```

### 5.2 Implementation

```go
// infrastructure/repository/moderation_repository_impl.go
func (r *ModerationRepositoryImpl) GetByID(...) (*entity.GovernanceCase, error) {
    query := `SELECT id, resource_type, resource_id, status,
                     reported_by, reviewed_by, reason, decision_note,
                     created_at, reviewed_at
              FROM moderation_cases    // ← DROPPED TABLE
              WHERE id = $1`
    // ...
}
```

### 5.3 Callers in Appeal

| Caller | Purpose |
|---|---|
| `AppealService.CreateAppeal` | Verify case exists + check appealable status |
| `AppealService.GetAppealWithCase` | Return case context for admin detail view |
| `AppealService.ReviewAppeal` | Check if restoration needed (enforced + auto-restorable type) |

### 5.4 Canonical Replacement

| Current method | Canonical replacement |
|---|---|
| `ModerationRepository.GetByID(caseID)` → returns GovernanceCase | `DecisionRepository.GetByID(decisionID)` + `CaseRepository.GetByID(caseID)` — OR — a dedicated `AppealContextRepository.GetDecisionWithContext(decisionID)` that JOINs decisions + cases + enforcements |

**The replacement must provide:**
1. Decision outcome + ID
2. Case subject_type + subject_id
3. Enforcement status (for auto-restore eligibility)
4. Decision decided_by (for reviewer independence check)

---

## 6. REVERSAL ANALYSIS

### 6.1 AppealReversalService

**STATUS: ALREADY REMOVED.** No file `appeal_reversal_service.go` exists. No reference to `AppealReversal` in Go code. No reference to `DomainAction` in governance module.

The previous cleanup (before this audit) already deleted the parked `AppealReversalService` and `DomainAction` entities.

### 6.2 Current Reversal Behavior

The current Appeal `ReviewAppeal` flow performs:

```
If approved + enforced + auto-restorable type (content/comment):
  → Insert outbox event: "moderation.<type>.restored"
  → ModerationEventHandler processes restoration:
      → ContentService.RestoreFromModeration() or
      → CommentService.RestoreFromModeration()
  → Notification: "moderation.<type>.restored"

If approved + for_sale/auction/user:
  → Record-only (no auto-restoration)
  → Admin must manually restore/reinstate

If rejected:
  → Record-only (original decision stands)
```

### 6.3 Classification

| Behavior | Classification |
|---|---|
| AppealReversalService | **REMOVE** (already removed) |
| Current restoration via outbox events | **KEEP TEMPORARILY** — this pattern is valid but must be rebuilt against Decision→Enforcement model |
| Restoration event payload | **REBUILD** — must include `decision_id`, `enforcement_id` for traceability |

---

## 7. APPEAL BUSINESS SEMANTICS

### 7.1 What the Current Code Establishes

| Question | Current Implementation Evidence |
|---|---|
| Who can create an Appeal? | Resource owner (content author, comment author, for_sale seller, auction seller, or suspended user themselves). **Evidence:** `appeal_service.go:117-201` `getResourceOwner()` |
| What can be appealed? | Cases with status `enforced` or `rejected`. Only resource types: content, comment, for_sale, auction, user. **Evidence:** `appeal_service.go:107-109, 173-201` |
| Can one Decision have multiple Appeals? | Currently: YES (checked by pending-only constraint, not total constraint). **Evidence:** `CreateWithPendingCheck` checks `WHERE report_id=$1 AND status='pending'` — after review, new appeal allowed |
| Can Appeal be created only after enforcement? | YES — `kase.Status != GovernanceCaseStatusEnforced && kase.Status != GovernanceCaseStatusRejected` → error. **Evidence:** `appeal_service.go:107-109` |
| Can Appeal be created for no_violation? | Currently NO (only enforced/rejected cases). This maps to: appeal only after violation_confirmed or dismissal |
| Appeal states | `pending → approved/rejected` (terminal). **Evidence:** `entity/appeal.go:55-62` |
| Who decides an Appeal? | Admin with `moderation.appeal.review` capability. **Evidence:** `appeal_handler.go:262` |
| Does Appeal reverse a Decision? | CURRENTLY NO — Appeal approval is a side-effect (outbox restoration event). No new Decision record is created. **Evidence:** `ReviewAppeal` in `appeal_service.go:321-416` |
| Does Appeal create a new Decision? | CURRENTLY NO. **Evidence:** No `DecisionRepository` in AppealService |
| Does Appeal mutate the original Decision? | NO — original decision remains. Appeal creates outbox event. **Evidence:** `ReviewAppeal` only calls `outboxRepo.InsertEvent` + `appealRepo.Update` |
| What happens to the target asset? | Content/comment: restored via outbox event. For_sale/auction/user: no auto-restoration. **Evidence:** `isAutoRestorableType()` in `appeal_service.go:219-222` |
| Money mutation? | NONE — ForSale appeals are record-only; content/comment restoration does not touch commerce state. **Evidence:** No finance/payment/escrow references in appeal path |

### 7.2 BUSINESS DECISIONS REQUIRED

Per Business Truth §41.7 and Design §24-25:

| Question | Status |
|---|---|
| Should Appeal review produce a new Decision record (canonical)? | **BUSINESS DECISION REQUIRED** — BT §24-25 says YES (Decision #2 with outcome=reversed/upheld), current code says NO |
| Should appeal eligibility be all decisions or only enforcement decisions? | **BUSINESS DECISION REQUIRED** — current code: only enforced+rejected |
| Can one Decision have unlimited appeals or a cap? | **NOT ESTABLISHED** — current code: unlimited (only pending-duplicate blocked) |
| Should reviewer be independent from original decision maker? | **NOT ESTABLISHED** — BT §24 says "if possible in simple model" |

---

## 8. APPEAL STATE MACHINE

### 8.1 Current Implementation

```
                ┌───────────────────────────┐
                │                           │
                ▼                           │
           ┌─────────┐                      │
           │ PENDING │                      │
           └────┬────┘                      │
                │                           │
        ┌───────┴───────┐                   │
        ▼               ▼                   │
  ┌──────────┐   ┌──────────┐              │
  │ APPROVED │   │ REJECTED │              │
  └──────────┘   └──────────┘              │
                                           │
  (New appeal allowed after resolution) ───┘
```

**State transitions:**
- `pending → approved` (admin approves)
- `pending → rejected` (admin rejects)
- New appeal for same case allowed after previous is resolved

### 8.2 Canonical Design (Business Truth §22, §24-25)

```
Appeal
  ↓
Appeal review → Decision #2 (reversed/upheld)  ← NEW DECISION RECORD
  ↓
If reversed → Reversal Enforcement (new enforcement) → Target restore
```

The canonical state machine is more complex — Appeal review produces a Decision, which then produces an Enforcement. The Appeal status is coupled to the Decision outcome.

### 8.3 Gap

**Current Appeal is self-contained (status only).** Canonical Appeal must produce an immutable Decision record and, if reversed, trigger a reversal Enforcement.

---

## 9. AUTHORIZATION

### 9.1 Capabilities

| Capability | Purpose | Source |
|---|---|---|
| `moderation.appeal.read` | View appeal list/detail (admin) | `capability.go:144` |
| `moderation.appeal.review` | Review/decide appeals (admin) | `capability.go:147` |
| (none) | Create appeal (any authenticated user) | `routes_core.go:1331` — no capability check |

### 9.2 Authorization Flow

**User create appeal:** `RequireAuth` only (no `RequireActiveAccount` — suspended users must be able to appeal). **Evidence:** `appeal_handler.go:130-133` comment explicitly explains this design choice.

**User view own appeal:** `RequireAuth` + ownership check (returns 404 not 403 for IDOR prevention). **Evidence:** `appeal_handler.go:153-168`

**Admin list/view:** `RequireCapability("moderation.appeal.read")`. **Evidence:** `routes_core.go:920,923,926`

**Admin review:** `RequireCapability("moderation.appeal.review")` + handler-level defense-in-depth. **Evidence:** `routes_core.go:931`, `appeal_handler.go:262-265`

### 9.3 Classification

| Component | Classification |
|---|---|
| Capability definitions | **KEEP** — `moderation.appeal.read`, `moderation.appeal.review` |
| Route middleware | **KEEP** — pattern is correct |
| Handler defense-in-depth | **KEEP** — good practice |
| Ownership check (IDOR prevention) | **KEEP** — pattern correct |
| Suspend-user appeal route | **KEEP** — correct design choice |

---

## 10. ADMIN API

### 10.1 Endpoint Inventory

| Method | Path | Handler | Capability | Auth |
|---|---|---|---|---|
| POST | `/api/v1/appeals` | `CreateAppeal` | none | RequireAuth |
| GET | `/api/v1/appeals/:id` | `GetAppeal` | none | RequireAuth + ownership |
| GET | `/api/v1/appeals/me` | `ListMyAppeals` | none | RequireAuth |
| GET | `/admin/appeals` | `AdminListAppeals` | `moderation.appeal.read` | Admin |
| GET | `/admin/appeals/pending` | `AdminListPendingAppeals` | `moderation.appeal.read` | Admin |
| GET | `/admin/appeals/:id` | `AdminGetAppeal` | `moderation.appeal.read` | Admin |
| PUT | `/admin/appeals/:id/review` | `AdminReviewAppeal` | `moderation.appeal.review` | Admin |

### 10.2 Response Format

**Create response:** `{id, case_id, status, message, created_at}` — uses `case_id` (legacy naming, should be `decision_id`)

**List response:** `{appeals: [...], page, limit, count}`

**Detail response:** `{appeal: {id, case_id, status, message, admin_response, reviewed_by, reviewed_at, created_at}}`

**Admin detail (with context):** `{appeal: {..., original_case: {id, resource_type, resource_id, status, reason, decision_status, created_at}}}`

**Review response:** `{id, status, reviewed_at}`

### 10.3 Classification

| Component | Classification |
|---|---|
| Endpoint structure | **CANONICALIZE** — add `decision_id` to responses, remove `case_id` naming |
| Admin detail `original_case` context | **REBUILD** — must join canonical `cases` + `decisions` + `enforcements` |
| `mapStatusToDecision` helper | **REMOVE** — legacy status mapping |
| Response `case_id` field naming | **RENAME** to `decision_id` |

---

## 11. ADMIN UI

### 11.1 Files

| File | Purpose | Classification |
|---|---|---|
| `apps/admin/src/pages/AppealsPage.tsx` | Appeals list page | **KEEP TEMPORARILY** — needs field rename (`report_id` → `decision_id`) |
| `apps/admin/src/components/moderation/AppealDetailModal.tsx` | Appeal detail + review modal | **KEEP TEMPORARILY** — needs `original_case` context rebuild |
| `apps/admin/src/hooks/useAppeals.ts` | API client hooks | **KEEP** — hooks are clean |
| `apps/admin/src/types/moderation.ts` | TypeScript types | **REBUILD** — `Appeal.report_id` → `decision_id`, `OriginalCaseContext` needs canonical fields |
| `apps/admin/src/pages/AppealsPage.test.tsx` | Tests | **KEEP** — needs update after backend changes |
| `apps/admin/src/components/moderation/AppealDetailModal.test.tsx` | Tests | **KEEP** — needs update after backend changes |
| `apps/admin/src/hooks/useAppeals.test.ts` | Tests | **KEEP** — needs update after backend changes |

### 11.2 Current UI Behavior

**AppealsPage:** Shows list of appeals with status filter. Columns: Appeal ID, Report ID (legacy naming), Message, Status, Submitted Date, Reviewed By. Each row has "Review" button.

**AppealDetailModal:** Shows appeal info + original case context card (case ID, resource type, status, reason, decision status). Admin can type response and approve/reject with confirmation dialog. Includes stale-status detection (re-fetches before action).

### 11.3 Frontend Type Issues

```typescript
// apps/admin/src/types/moderation.ts:21
export interface Appeal {
  id: string
  report_id: string   // ← BROKEN: this column no longer exists
  // ...
}
```

**Classification:**
| Component | Classification |
|---|---|
| `Appeal.report_id` | **RENAME** to `decision_id` |
| `OriginalCaseContext` | **REBUILD** — must show canonical case + decision + enforcement state |
| `decision_status` values (`'approved'|'dismissed'|'enforced'`) | **REPLACE** — must use canonical vocabulary |
| Appeal detail display | **REBUILD** — show Decision context instead of legacy GovernanceCase context |
| Stale-status detection | **KEEP** — good pattern |
| Confirmation dialog | **KEEP** — good UX |

---

## 12. AUDIT TRAIL

### 12.1 Current Audit Behavior

**Admin audit log** (`admin_audit_logs`): Appeal review is logged via `AdminAuditLogger.LogSafe`. **Evidence:** `appeal_handler.go:321-331`

```go
h.adminAuditLogger.LogSafe(ctx, adminID,
    "appeal_reviewed", "appeal", appealID,
    map[string]interface{}{
        "decision":        req.Decision,
        "approved":        approved,
        "previous_status": string(previousStatus),
        "new_status":      string(appeal.Status),
        "case_id":         appeal.CaseID.String(),
        "admin_response":  req.AdminResponse,
    },
)
```

**Audit events** (`audit_events`): Appeal does NOT use `AuditService.Emit`. Only `LogSafe` (best-effort, no transaction guarantee).

### 12.2 Classification

| Component | Classification |
|---|---|
| `admin_audit_logs` LogSafe for appeal review | **KEEP TEMPORARILY** — should be upgraded to in-tx audit (per canonical design) |
| No `audit_events` usage | **GAP** — canonical design requires reliable in-tx audit for governance mutations |
| Payload uses `case_id` naming | **RENAME** to `decision_id` |

---

## 13. MONEY/COMMERCE IMPACT

### 13.1 Appeal → Commerce Touch Points

| Target type | Appeal approval effect on commerce | Evidence |
|---|---|---|
| Content | None — restore content visibility only | `appeal_service.go:219-222` `isAutoRestorableType` |
| Comment | None — restore comment visibility only | Same |
| For Sale | Record-only — no auto-restore, no commerce mutation | `appeal_service.go:41-42` comment |
| Auction | Record-only — no auto-restore, no bid/settlement mutation | `appeal_service.go:42-43` comment |
| User | Record-only — no auto-reinstate, no subscription/order mutation | `appeal_service.go:44-45` comment |

### 13.2 Proof: No Money Mutation in Appeal Path

- `AppealService` dependencies: `appealRepo`, `moderationRepo`, `contentRepo`, `commentRepo`, `outboxRepo`, `forSaleRepo`, `auctionRepo`
- No finance, payment, escrow, ledger, coin, order, or settlement imports
- `ReviewAppeal` only touches: `appealRepo.Update` + `outboxRepo.InsertEvent`
- Restoration event handlers (`ModerationEventHandler`) call domain-specific restore methods, NOT commerce mutation

### 13.3 Classification

| Risk | Status |
|---|---|
| Appeal → order mutation | **SAFE** — no path |
| Appeal → payment mutation | **SAFE** — no path |
| Appeal → escrow mutation | **SAFE** — no path |
| Appeal → ledger mutation | **SAFE** — no path |
| Appeal → coin mutation | **SAFE** — no path |
| Appeal → seller proceeds | **SAFE** — no path |
| Appeal → listing state | **SAFE** — ForSale restoration goes through `ForSaleService.RestoreFromModeration` (boundary method) |
| Appeal → auction state | **SAFE** — record-only, no auto-restore |

**VERDICT: Appeal is COMMERCE-SAFE. No money mutations. No order/payment/ledger/escrow/coin touches.**

---

## 14. DUPLICATE AUTHORITY ANALYSIS

### 14.1 Current Duplicate Authorities

| Canonical Authority | Legacy Duplicate | Where |
|---|---|---|
| `decisions.outcome` | `GovernanceCase.Status` (`enforced` = violation) | `AppealService` reads `GovernanceCase.Status` to determine decision outcome |
| `cases.status` | `GovernanceCase.Status` (`pending`/`approved`/`rejected`/`enforced`) | Same field conflates Case lifecycle + Decision outcome |
| Enforcement state | `GovernanceCase.Status = 'enforced'` | `enforced` status conflates decision + enforcement success |
| Appeal state | `GovernanceCase.Status` for appealability check | `kase.Status != GovernanceCaseStatusEnforced` |

### 14.2 Analysis

The current Appeal code reads `GovernanceCase.Status` as a proxy for:
1. **Decision outcome** (`enforced` → violation confirmed)
2. **Enforcement state** (`enforced` → enforcement succeeded)
3. **Case lifecycle** (`pending`/`approved`/`rejected`/`enforced`)

These are THREE distinct governance concerns collapsed into ONE field. The canonical model separates them:
- `cases.status` → `open`/`resolved`
- `decisions.outcome` → `no_violation`/`violation`
- `enforcements.status` → `pending`/`processing`/`succeeded`/`failed`

### 14.3 Classification

| Duplicate | Classification |
|---|---|
| `GovernanceCase.Status` as decision proxy | **REMOVE** — replace with `decisions.outcome` |
| `GovernanceCase.Status` as enforcement proxy | **REMOVE** — replace with `enforcements.status` |
| `GovernanceCase.Status` as case lifecycle | **REMOVE** — replace with `cases.status` |
| GovernanceCase entity | **REMOVE** after rebuild |

---

## 15. TEST INVENTORY

### 15.1 Go Tests

| File | Tests | Classification |
|---|---|---|
| `application/appeal_service_test.go` | 14 tests (integration build tag) | **BUSINESS CONTRACT** — tests are correct business logic against mock repos |
| `delivery/http/appeal_handler_test.go` | 4+ tests (integration build tag) | **LEGACY IMPLEMENTATION** — tests nil-service handler, verify HTTP status codes |
| `delivery/http/appeal_capability_guard_test.go` | 4+ tests (integration build tag) | **CANONICAL BEHAVIOR** — tests capability-based auth, route-level proof |

### 15.2 TypeScript Tests

| File | Tests | Classification |
|---|---|---|
| `apps/admin/src/pages/AppealsPage.test.tsx` | Page rendering tests | **KEEP** — needs update after API field changes |
| `apps/admin/src/components/moderation/AppealDetailModal.test.tsx` | Modal interaction tests | **KEEP** — needs update after field changes |
| `apps/admin/src/hooks/useAppeals.test.ts` | Hook tests | **KEEP** — minimal changes needed |

### 15.3 Mobile Tests

| File | Tests | Classification |
|---|---|---|
| `apps/mobile/lib/domains/system/report/` tests | Appeal repository/notifier tests | **LEGACY** — mobile uses different model (AppealType, sourceId) not matching backend |

### 15.4 Test Gap

No test currently exercises the runtime path against the actual database. All Go tests use `//go:build integration` and mock repositories. With the schema–code mismatch, the SQL in `AppealRepositoryImpl` would fail immediately on any real DB.

---

## 16. LEGACY RESIDUE

### 16.1 Go Code Legacy

| Artifact | Location | Classification |
|---|---|---|
| `entity/appeal.go` `CaseID` field | `entity/appeal.go:19` | **REBUILD** → `DecisionID` |
| `entity/appeal.go` `ErrCaseNotFound` | `entity/appeal.go:91-97` | **RENAME** → `ErrDecisionNotFound` |
| `entity/appeal.go` `ErrNotResourceOwner` `CaseID` field | `entity/appeal.go:100-111` | **RENAME** → `DecisionID` |
| `entity/appeal.go` `ErrDuplicatePendingAppeal` `CaseID` field | `entity/appeal.go:114-121` | **RENAME** → `DecisionID` |
| `entity/appeal.go` `ErrCaseNotAppealable` | `entity/appeal.go:124-132` | **REBUILD** → `ErrDecisionNotAppealable` |
| `entity/governance_case.go` (entire file) | `entity/governance_case.go` | **REMOVE** after rebuild |
| `infrastructure/repository/moderation_repository.go` (entire file) | `infrastructure/repository/moderation_repository.go` | **REMOVE** after rebuild |
| `infrastructure/repository/moderation_repository_impl.go` (entire file) | `infrastructure/repository/moderation_repository_impl.go` | **REMOVE** after rebuild |
| `application/appeal_service.go` `ModerationRepository` dependency | `application/appeal_service.go:27` | **REMOVE** — replace with canonical Decision/Case repos |
| `application/appeal_service.go` `getResourceOwner()` | `application/appeal_service.go:117-201` | **KEEP** — valid ownership logic (may need ResourceType source update) |
| `application/appeal_service.go` `isAutoRestorableType()` | `application/appeal_service.go:219-222` | **KEEP TEMPORARILY** — valid categorization |
| `application/appeal_service.go` `buildRestoredPayload()` | `application/appeal_service.go:462-483` | **REBUILD** — must include `decision_id`, `enforcement_id` |
| `delivery/http/appeal_handler.go` `governanceCaseToContext()` | `delivery/http/appeal_handler.go:302-313` | **REBUILD** — must use canonical case/decision/enforcement context |
| `delivery/http/appeal_handler.go` `mapStatusToDecision()` | `delivery/http/appeal_handler.go:317-327` | **REMOVE** — legacy status mapping |

### 16.2 TypeScript/Admin Legacy

| Artifact | Location | Classification |
|---|---|---|
| `types/moderation.ts` `Appeal.report_id` | `types/moderation.ts:22` | **RENAME** → `decision_id` |
| `types/moderation.ts` `OriginalCaseContext.decision_status` | `types/moderation.ts:40` | **REBUILD** — use canonical vocabulary |
| `AppealsPage.tsx` `Report ID` column header | `AppealsPage.tsx:106` | **RENAME** → `Decision ID` |
| `AppealDetailModal.tsx` `appeal.report_id` display | `AppealDetailModal.tsx:134` | **RENAME** → `decision_id` |
| `AppealDetailModal.tsx` `original_case` display | `AppealDetailModal.tsx:168-195` | **REBUILD** — show canonical context |

### 16.3 Mobile Legacy

| Artifact | Location | Classification |
|---|---|---|
| `appeal.dart` `AppealType` enum (warning/suspension/ban/contentRemoval/penalty) | `entity/appeal.dart:13-17` | **REPLACE** — not used by backend; backend infers from subject_type |
| `appeal.dart` `AppealStatus.underReview`, `.cancelled` | `entity/appeal.dart:22` | **REPLACE** — backend has only pending/approved/rejected |
| `appeal.dart` `Appeal.sourceId` | `entity/appeal.dart:36` | **REMOVE** — backend uses `decision_id` |
| `appeal_dto.dart` `caseId` field | `dto/appeal_dto.dart:15` | **RENAME** → `decisionId` |
| `appeal_dto.dart` `CreateAppealRequestDto.caseId` | `dto/appeal_dto.dart:53` | **RENAME** → `decisionId` |

---

## 17. REBUILD REQUIREMENTS

### 17.1 Entity Layer

1. Rename `Appeal.CaseID` → `Appeal.DecisionID`
2. Update `NewAppeal()` to accept `decisionID` instead of `caseID`
3. Replace all `ErrCase*` types with `ErrDecision*` equivalents
4. Add `DecisionID` to `ErrNotResourceOwner`, `ErrDuplicatePendingAppeal`
5. Remove `entity/governance_case.go` after all callers migrated

### 17.2 Repository Layer

1. Replace all SQL `report_id` references with `decision_id`
2. Remove `ModerationRepository` interface and `ModerationRepositoryImpl`
3. Add `DecisionRepository` (or join-based) dependency to `AppealService`
4. Create appeal context query that JOINs `decisions` + `cases` + `enforcements`

### 17.3 Service Layer

1. Replace `ModerationRepository.GetByID(caseID)` with canonical Decision+Case query
2. Change appealability check from `GovernanceCase.Status` to `decisions.outcome` + enforcement state
3. Change `getResourceOwner()` call to use `cases.subject_type` + `cases.subject_id` (via decision→case join)
4. Change auto-restore check from `GovernanceCase.Status == enforced` to enforcement status check
5. Update restoration payload to include `decision_id` + `enforcement_id`
6. **BUSINESS DECISION:** Whether ReviewAppeal should create a new Decision record (canonical) or remain side-effect-based (current)

### 17.4 Handler Layer

1. Replace `governanceCaseToContext()` with canonical case/decision context builder
2. Remove `mapStatusToDecision()` helper
3. Update response DTOs to use `decision_id` instead of `case_id`

### 17.5 Admin UI

1. Update TypeScript types: `Appeal.report_id` → `decision_id`
2. Update `OriginalCaseContext` to show canonical fields
3. Update AppealsPage column header
4. Update AppealDetailModal display

### 17.6 Mobile

1. Update `AppealDto.caseId` → `decisionId`
2. Update `CreateAppealRequestDto.caseId` → `decisionId`
3. Update `report_api_datasource.dart` appeal endpoints
4. Simplify entity model to match backend (remove unused AppealType, sourceId)

### 17.7 Worker

1. Update `moderationRestoredPayload` to include `decision_id` + `enforcement_id`
2. Verify restoration event handlers work with canonical payload

---

## 18. BUSINESS DECISIONS REQUIRED

| # | Question | Current Behavior | Canonical Requirement | Recommendation |
|---|---|---|---|---|
| BD-1 | Should Appeal review produce a new Decision record? | No — appeal is standalone with status only | Yes — BT §24-25: "Decision #2 (reversed/upheld)" | **YES** — align with canonical |
| BD-2 | Appeal eligibility scope | Only enforced/rejected cases | BT §41.7: "semua decision atau hanya enforcement decision?" | Default: enforced + rejected (current is fine) |
| BD-3 | Multiple appeals per Decision | Unlimited (only pending-duplicate blocked) | Not established | Default: unlimited (current is fine) |
| BD-4 | Reviewer independence from decision maker | Not enforced | BT §24: "independen jika memungkinkan" | Default: not enforced in v1 (current is fine) |
| BD-5 | Appeal status vocabulary | pending/approved/rejected | Canonical: appeal state should reflect Decision #2 outcome | **BUSINESS DECISION** — approved → creates reversed Decision; rejected → creates upheld Decision |

---

## 19. IMPLEMENTATION RISKS

| Risk | Severity | Mitigation |
|---|---|---|
| Current code RUNTIME-DEAD against current schema | **P0** | Must rebuild before any appeal operations work |
| Appeal create/read/review all fail at runtime | **P0** | Complete rewrite of repository + service layer |
| Schema already migrated but Go code not updated | **P0** | Go code change is all that's needed (no new migration) |
| BD-1 (Decision record) affects Appeal core design | **P1** | Must be decided before implementation |
| Restoration event handler may need payload format change | **P2** | Worker already handles `moderation.*.restored` events — update payload |
| Mobile appeal model is completely different from backend | **P2** | Mobile needs significant entity update |
| No integration test against real DB | **P2** | All tests are mocked — add integration test in rebuild |

---

## 20. RECOMMENDED BOUNDED IMPLEMENTATION SLICES

### Slice A: Schema–Code Alignment (P0 — immediate)
**Goal:** Make appeal operations work against current DB schema.

**Scope:**
- Rename `Appeal.CaseID` → `Appeal.DecisionID` in entity
- Update `AppealRepositoryImpl` SQL to use `decision_id` instead of `report_id`
- Replace `ModerationRepository.GetByID` with `DecisionRepository` + `CaseRepository` calls
- Update `AppealService.CreateAppeal` to check appealability via canonical Decision state
- Update `AppealService.GetAppealWithCase` to query canonical case context
- Update `AppealService.ReviewAppeal` to check canonical enforcement state
- Update restoration payload to include `decision_id`

**Verification:** `go build ./...` + appeal integration test against test DB

### Slice B: Appeal Review → Decision Record (P0 — after Slice A)
**Goal:** Appeal review produces an immutable Decision record (canonical behavior).

**Scope:**
- When admin reviews appeal with approve/reject:
  - If approved: INSERT Decision (outcome=violation_confirmed/reversed) + INSERT Enforcement (if reversal needed)
  - If rejected: INSERT Decision (outcome=no_violation/upheld)
- Use same Decision+Enforcement+Outbox atomic transaction pattern (Design §7)
- This is the canonical approach: Appeal doesn't mutate original Decision; it creates a new one

**Verification:** Appeal create → review → verify Decision record exists + correct outcome

### Slice C: Admin API Cleanup (P1 — after Slice B)
**Goal:** Clean up API responses and field naming.

**Scope:**
- Rename response `case_id` → `decision_id`
- Rebuild `original_case` context in admin detail response (JOIN canonical tables)
- Remove `mapStatusToDecision()` helper
- Add `decision_outcome`, `enforcement_status` to admin detail response

**Verification:** Admin API returns truthful, canonical data

### Slice D: Admin UI Update (P1 — after Slice C)
**Goal:** Frontend reflects canonical model.

**Scope:**
- Update TypeScript types (`Appeal.report_id` → `decision_id`)
- Update AppealsPage column header
- Update AppealDetailModal to show canonical context
- Update `OriginalCaseContext` interface and display

**Verification:** Admin UI builds + renders correctly

### Slice E: Mobile Update (P2 — after Slice D)
**Goal:** Mobile appeal model aligns with backend.

**Scope:**
- Update `AppealDto.caseId` → `decisionId`
- Update `CreateAppealRequestDto.caseId` → `decisionId`
- Update API datasource calls
- Simplify entity model

**Verification:** Mobile builds + appeal flow works

### Slice F: Audit Trail Upgrade (P2 — after Slice B)
**Goal:** Appeal governance mutations use reliable in-transaction audit.

**Scope:**
- Appeal review → Decision creation must write to `audit_events` (not just `LogSafe`)
- Follow same pattern as canonical Decision audit

**Verification:** Appeal review is auditable via `audit_events`

### Slice G: Legacy Cleanup (P3 — after all above)
**Goal:** Remove all legacy artifacts.

**Scope:**
- Remove `entity/governance_case.go`
- Remove `infrastructure/repository/moderation_repository.go`
- Remove `infrastructure/repository/moderation_repository_impl.go`
- Remove `ErrCaseNotFound`, `ErrCaseNotAppealable` (replaced by ErrDecision* equivalents)
- Remove `delivery/http/appeal_handler.go:governanceCaseToContext`, `mapStatusToDecision`
- Verify no references remain

**Verification:** `go build ./...` + all tests pass

---

## 21. PROOF

| Check | Result |
|---|---|
| `go build ./...` (backend) | ✅ PASS (0 errors) |
| `go vet ./internal/governance/moderation/...` | ✅ PASS (0 errors) |
| `npx tsc --noEmit` (admin) | ✅ PASS (0 errors) |
| No code changes made | ✅ CONFIRMED |
| Only report file created | ✅ CONFIRMED |

---

## 22. FILES

### Go Files (Appeal Domain)

| File | Classification |
|---|---|
| `backend/internal/governance/moderation/entity/appeal.go` | **REBUILD** — rename CaseID → DecisionID, update error types |
| `backend/internal/governance/moderation/entity/governance_case.go` | **REMOVE** — legacy super-entity |
| `backend/internal/governance/moderation/application/appeal_service.go` | **REBUILD** — replace ModerationRepository with canonical deps |
| `backend/internal/governance/moderation/application/appeal_service_test.go` | **REBUILD** — update mocks and assertions |
| `backend/internal/governance/moderation/delivery/http/appeal_handler.go` | **REBUILD** — update context builders, remove legacy mapping |
| `backend/internal/governance/moderation/delivery/http/appeal_handler_test.go` | **KEEP** — update assertions after handler changes |
| `backend/internal/governance/moderation/delivery/http/appeal_capability_guard_test.go` | **KEEP** — capability tests are canonical |
| `backend/internal/governance/moderation/infrastructure/repository/appeal_repository.go` | **REBUILD** — interface changes for decision_id |
| `backend/internal/governance/moderation/infrastructure/repository/appeal_repository_impl.go` | **REBUILD** — SQL changes for decision_id |
| `backend/internal/governance/moderation/infrastructure/repository/moderation_repository.go` | **REMOVE** — legacy interface |
| `backend/internal/governance/moderation/infrastructure/repository/moderation_repository_impl.go` | **REMOVE** — reads dropped table |
| `backend/internal/worker/moderation_event_handler.go` | **UPDATE** — restoration payload format |
| `backend/cmd/core_server/routes_core.go` | **NO CHANGE** — routes are correct |
| `backend/internal/platform/capability/capability.go` | **NO CHANGE** — capabilities are correct |

### TypeScript Files (Admin)

| File | Classification |
|---|---|
| `apps/admin/src/types/moderation.ts` | **REBUILD** — field rename |
| `apps/admin/src/pages/AppealsPage.tsx` | **UPDATE** — column header rename |
| `apps/admin/src/components/moderation/AppealDetailModal.tsx` | **UPDATE** — field reference rename, context rebuild |
| `apps/admin/src/hooks/useAppeals.ts` | **NO CHANGE** — hooks are clean |
| `apps/admin/src/pages/AppealsPage.test.tsx` | **UPDATE** — after backend changes |
| `apps/admin/src/components/moderation/AppealDetailModal.test.tsx` | **UPDATE** — after backend changes |
| `apps/admin/src/hooks/useAppeals.test.ts` | **UPDATE** — after backend changes |

### Mobile Files

| File | Classification |
|---|---|
| `apps/mobile/lib/domains/system/report/domain/entities/appeal.dart` | **REBUILD** — align with backend model |
| `apps/mobile/lib/domains/system/report/data/dto/appeal_dto.dart` | **REBUILD** — field rename |
| `apps/mobile/lib/domains/system/report/data/repositories/appeal_repository_impl.dart` | **UPDATE** — after DTO changes |
| `apps/mobile/lib/domains/system/report/data/remote/report_api_datasource.dart` | **UPDATE** — after DTO changes |
| `apps/mobile/lib/domains/system/report/presentation/providers/appeal/appeal_notifier.dart` | **UPDATE** — after entity changes |
| `apps/mobile/lib/domains/system/report/presentation/providers/appeal/appeal_state.dart` | **UPDATE** — after entity changes |

### Migrations

| File | Classification |
|---|---|
| `backend/migrations/000055_canonical_moderation_foundation.up.sql` | **NO CHANGE** — schema already canonical |
| No new migration needed | Appeal schema is already at target state |

---

## 23. COMMIT

**No code commit.** Only the audit report file is created.

```
docs/audits/moderation/REPORT_APPEAL_AUDIT_1_FORENSIC.md (NEW)
```

---

## 24. PUSH

**No push.** This is an audit-only checkpoint.

---

## 25. WORKING TREE

| Change | File |
|---|---|
| NEW | `docs/audits/moderation/REPORT_APPEAL_AUDIT_1_FORENSIC.md` |

No other files modified.

---

## 26. FINAL VERDICT

### CURRENT STATUS:
**BLOCKED** — The Appeal domain is RUNTIME-DEAD against the current database schema. Migration 000055 already migrated `appeals` to the canonical `decision_id` model, but all Go code still references the dropped `report_id` column and the dropped `moderation_cases` table. Every appeal operation (Create, Get, List, Review) fails at runtime.

### CANONICAL RELATIONSHIP:
**Appeal → Decision** (already established by migration 000055)

```text
Appeal.decision_id  →  decisions(id)    -- FK exists, NOT NULL
```

### SCHEMA:
**ALREADY CANONICAL** — Migration 000055 transformed `appeals.report_id` → `appeals.decision_id` with FK to `decisions(id)`, CHECK constraint, and index. No further migration needed.

### GOVERNANCECASE:
**REMOVE CANDIDATE** — `GovernanceCase` entity and `ModerationRepository` are the ONLY things keeping Appeal tied to the legacy model. After Appeal is rebuilt against canonical Decision/Case, these can be deleted. DO NOT DELETE YET (build will break).

### MODERATIONREPOSITORY:
**REMOVE CANDIDATE** — `GetByID` reads `moderation_cases` (dropped table). All callers are in Appeal domain. After rebuild, interface + implementation deleted.

### REVERSAL:
**ALREADY REMOVED** — `AppealReversalService` and `DomainAction` no longer exist in the codebase. Restoration is currently handled via outbox events + `ModerationEventHandler.handleRestoration()`.

### BUSINESS SEMANTICS:
**ESTABLISHED** — Current Appeal semantics are well-documented in code and tests. Key gap: Appeal review does NOT produce a new Decision record (Business Truth §24-25 requires it).

### STATE MACHINE:
**SIMPLIFIED** — Current: `pending → approved/rejected`. Canonical (BT §22, §24-25): `pending → review → Decision #2 (reversed/upheld)`. Business decision needed on exact state vocabulary.

### AUTHORIZATION:
**CORRECT** — `moderation.appeal.read` + `moderation.appeal.review` capabilities are well-implemented with defense-in-depth. Suspend-user appeal route correctly uses RequireAuth without RequireActiveAccount.

### ADMIN API:
**LEGACY NAMING** — Endpoints are structurally correct. Response uses `case_id` (should be `decision_id`). `original_case` context shows legacy GovernanceCase fields.

### ADMIN UI:
**LEGACY FIELD REFERENCES** — `Appeal.report_id` (should be `decision_id`). `OriginalCaseContext` shows legacy vocabulary (`decision_status: 'approved'|'dismissed'|'enforced'`).

### AUDIT:
**BEST-EFFORT** — Uses `AdminAuditLogger.LogSafe` (no transaction guarantee). Should upgrade to in-tx `AuditService.Emit` during rebuild.

### MONEY/COMMERCE:
**SAFE** — Appeal has zero commerce touch points. No orders, payments, escrow, ledger, coins, seller proceeds, or settlement mutation.

### DUPLICATE AUTHORITY:
**PRESENT** — `GovernanceCase.Status` is used as proxy for Decision outcome, Enforcement state, and Case lifecycle. All three must be separated in canonical rebuild.

### LEGACY RESIDUE:
**EXTENSIVE** — 13 Go files, 4 TS files, 4 mobile files contain legacy references. Core issue: `CaseID`/`report_id` naming + `GovernanceCase`/`ModerationRepository` dependencies.

### BUSINESS DECISIONS REQUIRED:
1. **BD-1:** Should Appeal review produce a new Decision record? (Recommendation: YES — align with BT §24-25)
2. **BD-2:** Appeal eligibility scope (current: enforced+rejected is reasonable)
3. **BD-3:** Multiple appeals per Decision limit (current: unlimited is reasonable)
4. **BD-4:** Reviewer independence (current: not enforced in v1 is reasonable)

### REBUILD REQUIREMENTS:
1. Entity: `Appeal.CaseID` → `Appeal.DecisionID`
2. Repository: Remove `ModerationRepository`, use canonical Decision+Case repos
3. Service: Replace legacy case checks with canonical decision+enforcement checks
4. Handler: Rebuild context display, remove legacy mapping
5. Admin UI: Update TypeScript types and display
6. Mobile: Update DTOs and entity model

### PROPOSED IMPLEMENTATION SLICES:
```
Slice A: Schema–Code Alignment (P0) — make appeal compile + work against current DB
Slice B: Appeal Review → Decision Record (P0) — canonical behavior
Slice C: Admin API Cleanup (P1) — field naming + context
Slice D: Admin UI Update (P1) — frontend alignment
Slice E: Mobile Update (P2) — mobile alignment
Slice F: Audit Trail Upgrade (P2) — in-tx audit
Slice G: Legacy Cleanup (P3) — delete GovernanceCase, ModerationRepository
```

### PROOF:
```
✅ go build ./... — PASS
✅ go vet ./internal/governance/moderation/... — PASS
✅ npx tsc --noEmit — PASS
✅ No code changes made
✅ Only report file created
```

### FINAL VERDICT:
**BLOCKED** — Appeal domain is runtime-dead. Schema is already canonical. Go code must be rebuilt. Business decision BD-1 (Decision record creation) should be resolved before Slice B implementation. Slice A (schema–code alignment) is unblocked and can proceed immediately.
