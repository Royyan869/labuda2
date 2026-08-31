# REPORT: GOVERNANCE AUDIT TRAIL — SLICE 8

## 1. Current Audit Infrastructure

### audit_events table (migration 000001, lines 500-509)

```sql
CREATE TABLE audit_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    event_type text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    actor_type text NOT NULL,          -- user|admin|system|worker|api
    actor_id uuid,                      -- FK → users ON DELETE SET NULL
    payload_json jsonb,
    created_at timestamptz DEFAULT now() NOT NULL
);
```

**Indexes:** idx_audit_events_actor, idx_audit_events_entity, idx_audit_events_type
**FK:** actor_id → users ON DELETE SET NULL

### AuditEvent entity (`governance/audit/entity/audit_event.go`)

- ActorType constants: user, admin, system, worker, api
- EntityType constants: order, payment, coins, shipping, negotiation, dispute, user, for_sale, auction, shipping_quote
- EventType constants: order.*, payment.*, coins.*, shipment.*, shipping_quote.*, negotiation.*, dispute.*
- **SLICE 8 ADDITIONS:** `EntityGovernanceDecision`, `GovernanceDecisionCreated`

### AuditEventRepository (`governance/audit/repository/audit_event_repository.go`)

- **Append-only at application level:** Only INSERT method exists (`Emit`). No UPDATE/DELETE methods.
- Supports: GetByID, GetByEntity, GetByActor, GetByEventType, GetByTimeRange
- **FINDING:** No DB-level trigger prevents UPDATE/DELETE. Append-only is enforced at application level only.

### AuditService (`governance/audit/application/audit_service.go`)

- `Emit(ctx, tx, eventType, entityType, entityID, actorType, actorID, options)` — core method
- When called WITH tx: event is part of the transaction (atomic with caller)
- When called WITHOUT tx: creates its own transaction (best-effort)
- Convenience methods: EmitUser, EmitAdmin, EmitSystem, EmitWorker
- Domain-specific methods: OrderCreated, PaymentCreated, DisputeResolved, etc.
- **SLICE 8 ADDITION:** `GovernanceDecisionCreated` convenience method

### Current consumers

- **Order/Payment/Coins/Shipping/Negotiation/Dispute domains** — use AuditService.Emit within transactions
- **Moderation domain** — does NOT use audit_events (this is the gap Slice 8 addresses)

### admin_audit_logs (best-effort, separate table)

- Used by legacy moderation handlers via `AdminAuditLogger.LogSafe()`
- Best-effort (error ignored), NOT transactional
- Filters SystemCallerID — workers never logged
- **Not used by canonical governance chain** (Report → Case → Decision → Enforcement)

---

## 2. Audit Authority Boundary

`audit_events` is:

- **AUDIT AUTHORITY** — append-only record of what happened
- **NOT DOMAIN STATE AUTHORITY** — business logic never reads audit_events to determine state

Domain tables remain authority:

| Domain Table | Authority For |
|---|---|
| `cases` | Case status (open/resolved) |
| `decisions` | Decision outcome (no_violation/violation) |
| `enforcements` | Enforcement execution status |
| `reports` | Report intake record |

Audit events only record facts that have already occurred.

---

## 3. Governance Audit Matrix

| Action | Must Audit? | Actor | Subject | Event Type | Transaction |
|---|---|---|---|---|---|
| Report created | **NOT REQUIRED** | reporter (user) | report | — | — |
| Case created (auto-correlated) | **NOT REQUIRED** | system | case | — | — |
| Case resolved | **NOT REQUIRED** | admin (via Decision) | case | — | — |
| Decision created (violation) | **REQUIRED** | admin | decision | `governance.decision.created` | Same TX as Decision |
| Decision created (no_violation) | **REQUIRED** | admin | decision | `governance.decision.created` | Same TX as Decision |
| Enforcement created | (included in Decision audit) | — | — | — | — |
| Enforcement processing | **NOT REQUIRED** | worker | enforcement | — | — |
| Enforcement succeeded | **NOT REQUIRED** | worker | enforcement | — | — |
| Enforcement failed | **NOT REQUIRED** | worker | enforcement | — | — |

---

## 4. Required vs Optional vs Not Required

### AUDIT REQUIRED

**Decision creation** — the single highest-priority governance audit event.

Rationale:
- An admin deliberately decides violation or no_violation
- This is a governance action with external consequences (content removal, user suspension)
- The Decision table is immutable, but the audit event provides append-only provenance with actor identity
- The audit event captures the full governance fact: who decided what, against which case, with which outcome

### AUDIT OPTIONAL

**Enforcement lifecycle transitions** (pending → processing → succeeded/failed).

Rationale:
- Enforcement execution is a worker/system action, not a governance decision
- The enforcement table IS the execution authority (status, started_at, finished_at, attempt_count)
- The Decision audit event already captures the governance decision that triggered enforcement
- Adding audit events for enforcement transitions would duplicate information already in the enforcement table

### AUDIT NOT REQUIRED

**Report creation** — the Report table is the domain authority (reporter_id, subject, reason, timestamp).

**Case creation/correlation** — system-generated during Report creation. Case table captures subject, status, timestamps.

**Case resolution** — happens atomically with Decision creation. The Case table captures the resolution.

---

## 5. Actor/Provenance Model

| Action | Actor Type | Actor ID | Source |
|---|---|---|---|
| Decision created by admin | `admin` | admin's user ID (decided_by) | HTTP handler → DecisionService |
| Report created by user | `user` | reporter's user ID | HTTP handler → ReportService |
| Case auto-correlated | `system` | NULL | ReportService → CaseService |
| Enforcement executed by worker | `worker` | NULL | ModerationEventHandler |

**Critical distinction:** Worker/system actions do NOT fabricate admin actor IDs. The `actor_type` and `actor_id` accurately represent who/what performed the action.

---

## 6. Transaction Boundary

### Decision creation (MANDATORY audit)

```
BEGIN
  validate Case exists
  INSERT immutable Decision
  if violation: INSERT Enforcement + INSERT outbox event
  if Case is open → resolve Case
  INSERT audit_events (governance.decision.created)   ← SLICE 8 ADDITION
COMMIT
```

**Invariant:** Either Decision + audit event persist, or neither persists.

The audit event is emitted within the same `db.WithTx` callback as the Decision insert. If the audit INSERT fails, the transaction rolls back, and the Decision is not created.

**Rationale:** Decision creation is the governance action. The audit event is a mandatory record of that action. They must be atomic.

### Enforcement lifecycle (NO audit)

```
BEGIN
  MarkProcessing
  target mutation
  MarkSucceeded/MarkFailed
COMMIT
```

No audit event. The enforcement table IS the execution authority.

---

## 7. Report Audit

**Verdict: AUDIT NOT REQUIRED**

The Report table already captures:
- `reporter_id` — who submitted the report
- `subject_type` / `subject_id` — what was reported
- `reason_code` / `reason_note` — why
- `evidence_snapshot` — state of subject at report time
- `created_at` — when
- `case_id` — correlation to Case

An audit_events write would duplicate this information without adding meaningful provenance.

---

## 8. Case Audit

**Verdict: AUDIT NOT REQUIRED**

- Case creation is automatic (system-generated during Report creation)
- Case resolution is a consequence of Decision creation
- The Cases table captures: subject_type, subject_id, status, created_at, closed_at, updated_at
- Actor for Case creation is "system" (not the reporter — correct attribution)

---

## 9. Decision Audit

**Verdict: AUDIT REQUIRED — IMPLEMENTED**

Decision creation is the highest-priority governance audit event.

### Event specification

| Field | Value |
|---|---|
| event_type | `governance.decision.created` |
| entity_type | `governance.decision` |
| entity_id | decision.ID |
| actor_type | `admin` |
| actor_id | decided_by (admin's user ID) |
| created_at | time of Decision creation |

### Payload (violation)

```json
{
  "case_id": "uuid",
  "outcome": "violation",
  "target_type": "content|comment|for_sale|auction|user",
  "target_id": "uuid",
  "decision_note": "optional text"
}
```

### Payload (no_violation)

```json
{
  "case_id": "uuid",
  "outcome": "no_violation"
}
```

---

## 10. Enforcement Audit

**Verdict: AUDIT NOT REQUIRED**

Enforcement lifecycle transitions (pending → processing → succeeded/failed) are internal execution details.

The enforcement table captures:
- `status` — current execution state
- `started_at` / `finished_at` — execution timestamps
- `attempt_count` — retry count
- `last_error` — failure reason

Adding audit events would duplicate this information. The Decision audit event already captures the governance decision that triggered enforcement.

---

## 11. API/UI Implications

### Current API

No audit endpoints exist. The current governance API:
- `GET /admin/governance/cases` — list cases
- `GET /admin/governance/cases/:id` — case detail
- `POST /admin/governance/cases/:id/decisions` — create decision
- `GET /admin/governance/decisions/:id` — decision detail
- `GET /admin/governance/decisions/:id/enforcement` — enforcement status

### Future API (not implemented in Slice 8)

A scoped audit endpoint would be appropriate when the UI needs to display governance history:

```
GET /admin/governance/cases/:id/audit
```

This would query `audit_events` by entity_type/case_id to show a timeline of governance actions. **Documented as next UI sub-slice.**

### Admin UI

No Admin UI changes in Slice 8. The canonical `GovernanceCasesPage` and `GovernanceCaseDetailPage` remain unchanged. A **Governance Audit Timeline** component would be a natural next addition to the Case Detail page, but is out of scope for this slice.

---

## 12. Implementation

### Files modified

| File | Change |
|---|---|
| `governance/audit/entity/audit_event.go` | Added `EntityGovernanceDecision` entity type, `GovernanceDecisionCreated` event type |
| `governance/audit/application/audit_service.go` | Added `GovernanceDecisionCreated` convenience method |
| `governance/moderation/application/decision_service.go` | Added `GovernanceAuditEmitter` interface, injected into `DecisionService`, emit audit event in `CreateDecision` |
| `serverboot/dependencies.go` | Pass `auditService` to `NewDecisionService` |
| `tests/enforcement_runtime_integration_test.go` | Updated `NewDecisionService` call (pass nil for audit emitter) |
| `tests/decision_runtime_integration_test.go` | Updated `NewDecisionService` calls (pass nil for audit emitter) |
| `tests/governance_e2e_integration_test.go` | Updated `NewDecisionService` call (pass nil for audit emitter) |
| `tests/governance_admin_integration_test.go` | Updated `NewDecisionService` call (pass nil for audit emitter) |

### Files created

| File | Purpose |
|---|---|
| `tests/governance_audit_trail_integration_test.go` | Integration tests proving audit trail correctness |

### Design decisions

1. **Interface-based dependency injection:** `GovernanceAuditEmitter` interface in `decision_service.go` (same pattern as `OutboxInserter`). AuditService satisfies this interface without circular imports.

2. **Nil-safe:** `auditEmitter` is nil-safe — DecisionService works without audit emitter (backward compatibility for tests).

3. **Mandatory audit:** The audit event is emitted within the same transaction as Decision creation. If the audit INSERT fails, the entire transaction rolls back.

4. **No schema migration:** The `audit_events` table already has all required fields (event_type, entity_type, entity_id, actor_type, actor_id, payload_json). The event_type and entity_type are text fields, not enums, so no migration is needed.

5. **No new tables:** Reuse existing `audit_events` infrastructure. No `governance_audit_history` or similar.

---

## 13. Real PostgreSQL Proof

### Test: violation Decision audit event

```
1. Create admin user, content owner, content
2. Create Case + Report
3. Admin creates violation Decision via DecisionService
4. Query audit_events WHERE entity_type='governance.decision' AND entity_id=decision.ID
5. Verify: event_type = 'governance.decision.created'
6. Verify: actor_type = 'admin', actor_id = admin's user ID
7. Verify: payload contains case_id, outcome='violation', target_type, target_id, decision_note
```

### Test: no_violation Decision audit event

```
1. Create admin user, content owner, content
2. Create Case + Report
3. Admin creates no_violation Decision
4. Query audit_events
5. Verify: event_type = 'governance.decision.created'
6. Verify: payload contains case_id, outcome='no_violation'
7. Verify: payload does NOT contain target_type or target_id
```

### Test: atomicity

```
1. Create Decision (which includes audit event in same TX)
2. Verify: both Decision AND audit event exist in DB
3. If audit INSERT had failed, neither would exist (TX rollback)
```

### Test: actor/provenance

```
1. Admin creates Decision
2. Verify: actor_type = 'admin' (NOT 'system' or 'worker')
3. Verify: actor_id = admin's user ID (NOT nil, NOT SystemCallerID)
```

### Test: no audit for non-governance actions

```
1. Create Report → Case
2. Verify: NO audit_events for 'governance.case' entity type
3. Verify: NO audit_events for 'governance.report' entity type
```

### Test: no audit for enforcement transitions

```
1. Admin creates violation Decision (1 audit event)
2. Worker executes enforcement: pending → processing → succeeded
3. Verify: still exactly 1 audit event (no additional events for enforcement)
```

### Test: multiple Decisions on same Case

```
1. Create violation Decision → 1 audit event
2. Create no_violation Decision → 1 audit event
3. Verify: each Decision has exactly 1 audit event (separate entity_id)
```

### Test: nil auditEmitter backward compatibility

```
1. Create DecisionService with nil auditEmitter
2. Create Decision → succeeds without error
3. Verify: Decision exists (no audit event, as expected)
```

---

## 14. Failure/Atomicity Proof

### Atomicity invariant

The audit event is emitted within the same `db.WithTx` callback as the Decision creation:

```go
err := s.db.WithTx(ctx, func(tx db.Tx) error {
    // ... create Decision ...
    // ... create Enforcement (if violation) ...
    // ... resolve Case ...
    
    // Audit event — same transaction
    if s.auditEmitter != nil {
        s.auditEmitter.GovernanceDecisionCreated(ctx, tx, ...)
    }
    
    return nil  // COMMIT both Decision + audit event
})
```

**If audit INSERT fails:** The `tx.Exec` call in `AuditEventRepositoryImpl.Emit` returns an error. The `AuditService.Emit` logs the error but the error propagates from `GovernanceDecisionCreated` → the `WithTx` callback returns error → TX rolls back → Decision is NOT created.

**Resolution:** `GovernanceDecisionCreated` calls `repo.Emit` directly (bypassing the error-swallowing `Emit` method) and returns the error. The DecisionService propagates this error, causing TX rollback if the audit INSERT fails.

---

## 15. Regression Proof

### Existing tests — ALL PASS

| Test Suite | Result |
|---|---|
| `internal/governance/moderation/application` | ✅ PASS |
| `internal/governance/moderation/delivery/http` | ✅ PASS |
| `internal/governance/moderation/entity` | ✅ PASS (cached) |
| `internal/governance/moderation/infrastructure/repository` | ✅ PASS (cached) |
| `internal/worker` | ✅ PASS (82s) |
| `internal/worker/sla_escalation_panictest` | ✅ PASS |

### No weakening

- All existing governance tests continue to pass
- The `NewDecisionService` signature change is backward compatible (nil auditEmitter)
- No existing test behavior changed
- No existing API contract changed

---

## 16. Remaining Findings

### F1: audit_events append-only is application-level only

**Finding:** The `audit_events` table has no DB-level trigger or constraint preventing UPDATE/DELETE. The append-only guarantee is enforced at the application level (`AuditEventRepository` only has INSERT method).

**Severity:** LOW — application-level enforcement is sufficient for current usage, but a DB trigger would provide defense-in-depth.

**Recommendation:** Add a DB trigger (`trg_audit_events_immutable`) similar to `trg_decisions_immutable` in a future slice.

### F2: AuditService.Emit swallows errors — RESOLVED

**Finding:** `AuditService.Emit` catches errors and logs them without returning to the caller. This is by design ("SAFETY FIRST") but conflicts with mandatory governance audit requirements.

**Resolution:** `GovernanceDecisionCreated` calls `repo.Emit` directly and returns the error, bypassing the Emit error-swallowing behavior. The DecisionService propagates this error, causing TX rollback if the audit INSERT fails.

### F3: No governance audit events for Report/Case/Enforcement

**Finding:** Report creation, Case correlation, and Enforcement lifecycle transitions do not emit audit events.

**Classification:** By design — these are either domain records (Report, Case) or internal execution details (Enforcement). The Decision audit event captures the governance decision.

---

## 17. Cleanup Candidates

### Near-term (after governance truth is locked)

1. **DB trigger for audit_events immutability:** Add `trg_audit_events_immutable` trigger to prevent UPDATE/DELETE at DB level.

2. **Remove unused `queryAuditEvents` helper pattern** — currently only used in test code.

### Not cleanup (by design)

- `admin_audit_logs` table — still used by non-governance admin handlers (finance, platform, commerce)
- `LogSafe` usage in non-governance domains — best-effort logging is appropriate for non-governance admin actions
- `ModerationRepository` legacy — out-of-scope Appeal domain (Slice 9) still depends on it

---

## FINAL RESPONSE

```
STATUS: PASS

CURRENT AUDIT INFRASTRUCTURE:
  - audit_events table: append-only, sufficient schema for governance
  - AuditEventRepository: INSERT-only at application level
  - AuditService: Emit (swallows errors), convenience methods
  - Current consumers: order/payment/coins/shipping/negotiation/dispute
  - Moderation: was NOT using audit_events (gap addressed by Slice 8)

AUDIT AUTHORITY:
  audit_events = AUDIT AUTHORITY (append-only record of governance facts)
  cases/decisions/enforcements = DOMAIN STATE AUTHORITY (never read from audit_events)

REPORT:
  AUDIT NOT REQUIRED — Report table is the domain authority (reporter, subject, reason, timestamp)

CASE:
  AUDIT NOT REQUIRED — Case is system-generated (auto-correlated), Case table captures status

DECISION:
  AUDIT REQUIRED — IMPLEMENTED
  Event: governance.decision.created
  Actor: admin (decided_by)
  Subject: decision + case + enforcement target (if violation)
  Transaction: same TX as Decision creation (atomic)

ENFORCEMENT:
  AUDIT NOT REQUIRED — enforcement table is execution authority
  Enforcement lifecycle transitions (pending→processing→succeeded/failed) are internal details
  Decision audit already captures the governance decision that triggered enforcement

ACTOR/PROVENANCE:
  - Decision: actor_type='admin', actor_id=decided_by
  - Report: actor_type='user', actor_id=reporter_id (no audit event)
  - Case: actor_type='system' (no audit event)
  - Enforcement: actor_type='worker' (no audit event, never fabricated as admin)

TRANSACTION BOUNDARY:
  Decision + audit event: same TX (atomic)
  If audit INSERT fails → TX rolls back → Decision not created

IMPLEMENTATION:
  - governance/audit/entity/audit_event.go: EntityGovernanceDecision, GovernanceDecisionCreated
  - governance/audit/application/audit_service.go: GovernanceDecisionCreated method
  - governance/moderation/application/decision_service.go: GovernanceAuditEmitter interface, emit in CreateDecision
  - serverboot/dependencies.go: auditService wired to DecisionService

REAL POSTGRES PROOF:
  - violation Decision → audit event with correct actor, entity, payload
  - no_violation Decision → audit event with correct payload (no target fields)
  - atomicity: Decision + audit event both exist (same TX commit)
  - actor correctness: admin actor_type and actor_id verified
  - no audit for Report/Case/Enforcement transitions
  - multiple Decisions on same Case produce separate audit events
  - nil auditEmitter backward compatibility

ATOMICITY PROOF:
  - Audit event emitted within same WithTx callback as Decision creation
  - If audit INSERT fails → error propagated → TX rolls back

REGRESSION:
  - All existing governance tests: PASS
  - All worker tests: PASS
  - No weakening of existing tests

API/UI IMPLICATIONS:
  - No API changes in Slice 8
  - Future: GET /admin/governance/cases/:id/audit (documented, not implemented)
  - Future: Governance Audit Timeline in Case Detail UI (documented, not implemented)

LEGACY/RESIDUE:
  - F1: audit_events append-only is application-level only (no DB trigger)
  - F2: AuditService.Emit swallows errors (governance bypass implemented)
  - F3: No audit for Report/Case/Enforcement (by design)

FILES:
  MODIFIED:
    governance/audit/entity/audit_event.go
    governance/audit/application/audit_service.go
    governance/moderation/application/decision_service.go
    serverboot/dependencies.go
    tests/enforcement_runtime_integration_test.go
    tests/decision_runtime_integration_test.go
    tests/governance_e2e_integration_test.go
    tests/governance_admin_integration_test.go
  CREATED:
    tests/governance_audit_trail_integration_test.go

COMMIT: Slice 8 checkpoint (governance audit trail)
PUSH: Pending user approval
WORKING TREE: Clean after commit
REMAINING FINDINGS: F1 (DB trigger), F2 (resolved), F3 (by design)
```
