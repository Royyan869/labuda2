# Staging Activation Playbook — Phase 1 Controlled Rollout
## Unified Withdrawal + Payout Worker Pilot

**Status:** DRAFT — not yet activated  
**Scope:** staging environment only  
**Owner:** operator with `finance.withdraw.review` + `finance.withdraw.read` capabilities  
**Last updated:** 2026-05-01

---

## A. STAGING ACTIVATION PRECHECKS

All items must be confirmed before any activation phase begins.
Run in order. Stop and investigate any failure before continuing.

### A1. Infrastructure

```bash
# 1. Confirm ENV=staging is set
echo $ENV  # must be "staging"

# 2. Confirm migrations applied — finance tables must exist
psql $DATABASE_URL -c "SELECT table_name FROM information_schema.tables
  WHERE table_schema='public'
  AND table_name IN ('financial_accounts','ledger_transactions','ledger_entries',
                     'withdrawals','dispute_freezes')
  ORDER BY table_name;"
# Expected: 6 rows returned

# 3. Confirm system accounts bootstrapped
psql $DATABASE_URL -c "SELECT account_type, balance FROM financial_accounts
  WHERE user_id IS NULL ORDER BY account_type;"
# Expected: GATEWAY_CLEARING, PLATFORM_REVENUE, WITHDRAWAL_PENDING at minimum
```

### A2. Verifier Clean

```bash
# MUST return HTTP 200 and "passed": true before ANY activation
curl -s -X POST https://<staging-host>/api/v1/admin/finance/verify?mode=forensic \
  -H "Authorization: Bearer <admin-token>" | jq '{passed, error_count, warning_count}'
# Required: passed=true, error_count=0
# Acceptable: warning_count>0 (warnings are informational, not blockers)
# BLOCKER: any error_count>0 or passed=false
```

### A3. No Stuck Payouts

```bash
# No withdrawals stuck in SUBMITTED or SETTLING
curl -s "https://<staging-host>/api/v1/admin/payouts/withdrawals?status=SUBMITTED" \
  -H "Authorization: Bearer <admin-token>" | jq '.total'
# Required: 0

curl -s "https://<staging-host>/api/v1/admin/payouts/withdrawals?status=SETTLING" \
  -H "Authorization: Bearer <admin-token>" | jq '.total'
# Required: 0
```

### A4. Pilot Whitelist Ready

```bash
# At least 1 seller UUID confirmed for pilot testing
echo $PAYOUT_PILOT_WHITELIST
# Must be non-empty: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

### A5. Workers Disabled Initially

```bash
# Both workers must start disabled — verify server logs on startup:
# EXPECTED LOG: "Payout worker DISABLED: set PAYOUT_ENABLE_WORKER=true to activate"
# EXPECTED LOG: "Payout reconciliation worker DISABLED: set PAYOUT_ENABLE_RECONCILIATION=true to activate"
grep "Payout worker DISABLED\|reconciliation worker DISABLED" <server-log>
```

### A6. Admin Endpoints Reachable

```bash
# Ledger endpoint must return 200
curl -s "https://<staging-host>/api/v1/admin/finance/ledger?limit=1" \
  -H "Authorization: Bearer <admin-token>" | jq '.total'
# Must not return 401/403/500
```

### A7. Precheck Gate

| Check | Command | Expected | Status |
|-------|---------|----------|--------|
| Migrations applied | psql SELECT tables | 6 tables exist | ☐ |
| System accounts exist | psql SELECT account_type | ≥3 rows | ☐ |
| Verifier forensic clean | POST /verify?mode=forensic | passed=true | ☐ |
| No stuck SUBMITTED | GET /withdrawals?status=SUBMITTED | total=0 | ☐ |
| No stuck SETTLING | GET /withdrawals?status=SETTLING | total=0 | ☐ |
| Workers disabled in logs | grep DISABLED | both disabled | ☐ |
| Admin ledger reachable | GET /finance/ledger | 200 OK | ☐ |
| Pilot whitelist populated | echo env | non-empty UUID | ☐ |

**GATE: All 8 items must be ✓ before Phase A.**

---

## B. CONFIG MATRIX

### Canonical Staging Config

| Variable | Default | Staging Value | Notes |
|----------|---------|---------------|-------|
| `ENV` | `development` | `staging` | Activates startup guards |
| `USE_UNIFIED_WITHDRAWAL` | `true` | `true` | Must be true; guard fatal if false |
| `PAYOUT_ENABLE_WORKER` | `false` | `false` → `true` (Phase D) | Explicit opt-in only |
| `PAYOUT_ENABLE_RECONCILIATION` | `false` | `false` → `true` (Phase B) | Safe to enable independently |
| `PAYOUT_ENABLE_PILOT_MODE` | `true` | `true` | Must be true in staging |
| `PAYOUT_PILOT_WHITELIST` | `""` | `"<uuid1>[,<uuid2>]"` | Must be non-empty when worker enabled |
| `PAYOUT_GATEWAY_PROVIDER` | `sandbox` | `sandbox` | Never `midtrans_payout` in staging pilot |
| `PAYOUT_ENVIRONMENT` | `sandbox` | `sandbox` | Keep sandbox for pilot |
| `PAYOUT_ENABLE_PRODUCTION` | `false` | `false` | Never true in staging |

### Dangerous Combinations

| Combination | Risk | Fail-fast behavior |
|-------------|------|--------------------|
| `USE_UNIFIED_WITHDRAWAL=false` + `ENV=staging` | Legacy path re-enabled | `ValidateStagingActivation()` → `os.Exit(1)` |
| `PAYOUT_ENABLE_WORKER=true` + empty whitelist + `ENV=staging` | Worker runs, zero sellers served | `ValidateStagingActivation()` → `os.Exit(1)` |
| `PAYOUT_ENABLE_WORKER=true` + `PAYOUT_ENABLE_PILOT_MODE=false` + `ENV=staging` | Unconstrained worker in staging | `ValidateStagingActivation()` → `os.Exit(1)` |
| `PAYOUT_ENABLE_PRODUCTION=true` + `PAYOUT_ENVIRONMENT=production` | Real money movement | `ValidateProductionSafety()` → panic |
| `PAYOUT_GATEWAY_PROVIDER=midtrans_payout` + `PAYOUT_ENVIRONMENT=sandbox` | Mismatch — real gateway, sandbox keys | No explicit guard; operator must check |

---

## C. ACTIVATION SEQUENCE

### Phase A: Verifier Governance

**Goal:** confirm ledger is forensic-clean before any withdrawal traffic.

**Config delta:** none (verifier is always-on)

```bash
# Run forensic verify
curl -X POST "https://<staging>/api/v1/admin/finance/verify?mode=forensic" \
  -H "Authorization: Bearer <admin-token>"
```

**Expected response:**
```json
{ "mode": "forensic", "passed": true, "error_count": 0, "warning_count": 0,
  "sections": [{ "name": "...", "passed": true }] }
```

**Stop conditions:** `passed=false`, `error_count > 0`, HTTP 422 or 500.  
**Evidence to capture:** full JSON response, timestamp, actor ID.

---

### Phase B: Reconciliation Worker (Read-Only)

**Goal:** enable background stuck-payout detection before any real payouts run.

**Config delta:**
```
PAYOUT_ENABLE_RECONCILIATION=true
# (worker, pilot mode, whitelist unchanged)
```

**Expected startup log:**
```
"Payout reconciliation worker STARTED (read-only)"
  stuck_threshold_minutes=30
  reconciliation_interval_minutes=10
```

**Validation:**
```bash
# Wait 11 minutes, then check logs for reconciliation cycle
grep "Starting payout reconciliation check\|Reconciliation check completed" <server-log>
# Expected: periodic cycles, no mutations, stuck_payouts=0
```

**Stop conditions:** any log showing DB write from reconciliation worker, error rate in logs.  
**Rollback:** set `PAYOUT_ENABLE_RECONCILIATION=false`, redeploy. Worker stops on `Stop()` call at shutdown; no state to clean up (read-only).

---

### Phase C: Unified Withdrawal Validation

**Goal:** confirm withdrawal requests flow through canonical path correctly.

**Config delta:** none (`USE_UNIFIED_WITHDRAWAL` defaults to `true`)

**Test withdrawal request (pilot seller):**
```bash
curl -X POST "https://<staging>/api/v1/withdraw" \
  -H "Authorization: Bearer <seller-token>" \
  -H "Content-Type: application/json" \
  -d '{"amount": 100000}'
# Expected: 201 Created
# {"withdrawal_id": "<uuid>", "status": "REQUESTED", "amount": 100000}
```

**Expected ledger (verify via admin):**
```bash
curl "https://<staging>/api/v1/admin/finance/ledger?reference_type=withdrawal_request&limit=5" \
  -H "Authorization: Bearer <admin-token>"
```
```
DR SELLER_PAYABLE[seller_id]   -100000   → balance decreases
CR WITHDRAWAL_PENDING          +100000   → balance increases
```

**Expected withdrawal record:**
```bash
curl "https://<staging>/api/v1/admin/payouts/withdrawals?status=REQUESTED" \
  -H "Authorization: Bearer <admin-token>"
# withdrawal appears with status=REQUESTED
```

**Post-check — verifier must still be clean:**
```bash
curl -X POST "https://<staging>/api/v1/admin/finance/verify?mode=forensic" \
  -H "Authorization: Bearer <admin-token>"
# passed=true, error_count=0
```

**Stop conditions:** verifier finds error after withdrawal, 5xx on withdrawal request, balance inconsistency.

---

### Phase D: Payout Worker Sandbox Pilot (1 Seller)

**Goal:** activate payout worker for 1 whitelisted seller using sandbox gateway.

**Config delta:**
```
PAYOUT_ENABLE_WORKER=true
PAYOUT_ENABLE_PILOT_MODE=true
PAYOUT_PILOT_WHITELIST=<seller-uuid-1>
PAYOUT_GATEWAY_PROVIDER=sandbox
PAYOUT_ENVIRONMENT=sandbox
```

**Expected startup log:**
```
"Payout worker STARTED"
  environment=sandbox
  pilot_mode=true
  pilot_whitelist_count=1
  sandbox_mode=true
```

**Approve withdrawal to trigger worker:**
```bash
curl -X POST "https://<staging>/api/v1/admin/payouts/withdrawals/<withdrawal-id>/approve" \
  -H "Authorization: Bearer <admin-token>"
# Expected: 200 OK, status changes REQUESTED → PROCESSING
```

**Expected ledger on approval:**
```
DR WITHDRAWAL_PENDING   -100000
CR WITHDRAWAL_COMMITTED +100000
```

**Worker picks up PROCESSING withdrawal (within 30s poll interval):**
```
Expected log: "payout_submitted" withdrawal_id=<uuid> gateway_reference_id=<sandbox-ref>
Status: PROCESSING → SUBMITTED
```

**Expected ledger on settlement (webhook or manual mark-processed):**
```bash
curl -X POST "https://<staging>/api/v1/admin/payouts/withdrawals/<id>/mark-processed" \
  -H "Authorization: Bearer <admin-token>"
# Status: SUBMITTED → SETTLED
```

**Post-check:**
```bash
curl -X POST "https://<staging>/api/v1/admin/finance/verify?mode=forensic" \
  -H "Authorization: Bearer <admin-token>"
# passed=true, error_count=0
```

**Stop conditions:** payout worker processes a seller NOT on whitelist, verifier error, any production gateway call detected, ledger inconsistency.

---

### Phase E: Expand Whitelist

**Gate before expansion:**
- Phase D ran cleanly for ≥24 hours
- Zero verifier errors since Phase D activation
- Zero stuck payouts
- Reconciliation worker shows no anomalies

**Config delta:** add more UUIDs to `PAYOUT_PILOT_WHITELIST`
```
PAYOUT_PILOT_WHITELIST=<uuid1>,<uuid2>,<uuid3>
```

**Log after redeploy:**
```
"Pilot mode whitelist loaded"  whitelisted_sellers=3  pilot_mode_enabled=true
```

**Validation:** repeat Phase D validation for each new seller.

---

## D. RUNTIME VALIDATION PROCEDURES

### D1. Withdrawal Request

```bash
POST /api/v1/withdraw {"amount": N}
→ 201 Created
→ DR SELLER_PAYABLE[seller]  -N  (balance_after = old - N)
→ CR WITHDRAWAL_PENDING      +N  (balance_after = old + N)
→ withdrawals row: status=REQUESTED
```

Verify: `GET /admin/finance/ledger?reference_type=withdrawal_request`

### D2. Payout Approval

```bash
POST /admin/payouts/withdrawals/:id/approve
→ 200 OK
→ DR WITHDRAWAL_PENDING    -N  (balance decreases)
→ CR WITHDRAWAL_COMMITTED  +N  (balance increases)
→ withdrawals row: status=PROCESSING
```

Verify: ledger `reference_type=withdrawal_approve`

### D3. Payout Completion (Sandbox)

```bash
# Worker submits → status=SUBMITTED, external_reference_id set
# mark-processed or webhook → status=SETTLED
POST /admin/payouts/withdrawals/:id/mark-processed
→ WITHDRAWAL_COMMITTED drained (balance_after decreases to 0 for this tx)
→ withdrawals row: status=SETTLED
```

### D4. Stuck Payout Visibility

```bash
GET /admin/payouts/withdrawals?status=SUBMITTED
# Any row with updated_at > 30 min ago = stuck
# Cross-check with reconciliation worker log:
grep "stuck_payouts" <server-log>
```

### D5. Verifier Endpoint

```bash
# Forensic (default — informational findings)
POST /admin/finance/verify?mode=forensic → 200 if passed, 422 if not

# Strict (all findings raised to errors)
POST /admin/finance/verify?mode=strict   → 200 if passed, 422 if not, 500 if panic
```

Invariants checked: account balance consistency, ledger double-entry, withdrawal status ↔ ledger alignment, WITHDRAWAL_PENDING balance = Σ(REQUESTED withdrawals), dispute freeze correctness.

### D6. Ledger Export

```bash
# All transactions in last 1 hour
GET /admin/finance/ledger?from=<unix-1h-ago>&limit=200

# Filter by type
GET /admin/finance/ledger?reference_type=withdrawal_request
GET /admin/finance/ledger?reference_type=withdrawal_approve
GET /admin/finance/ledger?reference_type=withdrawal_reject
```

### D7. Dispute Freeze Behavior

When a dispute is opened on a released order:
- `dispute_freezes` row created: `status=active`, `frozen_amount=N`
- Seller's withdrawable = `SELLER_PAYABLE.balance − Σ(active dispute_freeze.frozen_amount)`
- Withdrawal blocked if requested amount > withdrawable
- Verifier checks freeze consistency

### D8. Refund After Payout Reservation

If a withdrawal is in `REQUESTED` state and a dispute/refund is initiated:
- Dispute freeze is applied to `SELLER_PAYABLE`
- If `SELLER_PAYABLE` after freeze < `WITHDRAWAL_PENDING`, verifier will find inconsistency
- Operator must reject the withdrawal first (returns funds to `SELLER_PAYABLE`), then process refund
- Refund flows through gateway semantics (Midtrans refund API), not direct ledger credit

---

## E. ROLLBACK PROCEDURES

### Rollback Decision Tree

```
Is verifier showing errors?
├── YES → STOP all phases. Do NOT rollback yet.
│         Run: GET /admin/finance/ledger to identify the mismatch.
│         Identify which withdrawal caused it.
│         If it's a stuck PROCESSING/SUBMITTED withdrawal:
│           → Mark-processed (if safe) OR reject (if no external_reference_id)
│         After ledger clean: re-run verifier. Then roll back config.
└── NO  → Safe to roll back config flags directly.
```

### Phase B Rollback (Reconciliation Worker)

```bash
# Set env
PAYOUT_ENABLE_RECONCILIATION=false
# Redeploy
# Worker stops cleanly via Stop() on shutdown signal
# No ledger state to verify — worker never mutated
```

### Phase C Rollback (Unified Withdrawal)

Unified withdrawal cannot be "rolled back" to legacy — `WithdrawService.RequestWithdraw` panics (hard block, Phase 3.1). The only rollback is:

```bash
# Stop accepting withdrawals entirely by taking the endpoint offline
# OR: use rate limiting / feature flag at reverse proxy level
# Then verify: no REQUESTED withdrawals left in limbo
GET /admin/payouts/withdrawals?status=REQUESTED
```

If any REQUESTED withdrawals exist from the test: reject them.
```bash
POST /admin/payouts/withdrawals/:id/reject
# Expected ledger: DR WITHDRAWAL_PENDING → CR SELLER_PAYABLE (funds restored)
```

Verify after each rejection: `POST /admin/finance/verify?mode=forensic` must pass.

### Phase D Rollback (Payout Worker)

```bash
# 1. Disable worker
PAYOUT_ENABLE_WORKER=false
# Redeploy → worker.Stop() called, goroutine exits, no new submissions

# 2. Check for in-flight payouts
GET /admin/payouts/withdrawals?status=PROCESSING   # must be 0
GET /admin/payouts/withdrawals?status=SUBMITTED    # note each one

# 3. For each SUBMITTED payout:
#    Option A: wait for sandbox webhook (safe, no action needed)
#    Option B: mark-processed manually
POST /admin/payouts/withdrawals/:id/mark-processed

# 4. Verify ledger clean
POST /admin/finance/verify?mode=forensic  # must pass

# 5. For any PROCESSING withdrawal that was never submitted:
#    Reject it to restore SELLER_PAYABLE
POST /admin/payouts/withdrawals/:id/reject
```

**Rollback invariant check after Phase D rollback:**
- `WITHDRAWAL_PENDING.balance` = 0 (all pending resolved)
- `WITHDRAWAL_COMMITTED.balance` = 0 (all committed resolved)
- Verifier: `passed=true`, `error_count=0`

---

## F. INCIDENT PLAYBOOKS

### F1. Payout Stuck in SUBMITTED

```
Diagnosis (read-only first):
1. GET /admin/payouts/withdrawals?status=SUBMITTED
2. Note: withdrawal_id, external_reference_id, updated_at
3. Check reconciliation log: grep "stuck_payouts" → count and recommendation
4. Elapsed time in SUBMITTED?
   < 30 min → wait_for_callback (normal sandbox delay)
   30–60 min → check sandbox gateway dashboard for external_reference_id
   > 60 min → mark_processed OR query gateway manually

Resolution:
POST /admin/payouts/withdrawals/:id/mark-processed
Post-check: POST /admin/finance/verify?mode=forensic
```

### F2. Payout Stuck in SETTLING

```
Diagnosis:
1. GET /admin/payouts/withdrawals?status=SETTLING
2. Elapsed > 2h → escalate to gateway query

Resolution options:
A) Wait for gateway callback (preferred)
B) mark-processed if gateway confirms completion
C) Reject if gateway confirms failure (restores SELLER_PAYABLE)
   → POST /admin/payouts/withdrawals/:id/reject

Post-check: POST /admin/finance/verify?mode=forensic
```

### F3. Duplicate Payout Callback

```
Diagnosis:
1. Check withdrawals row: status=SETTLED already?
2. If status=SETTLED and second webhook arrives:
   → Webhook handler uses idempotency key (external_reference_id)
   → Second webhook should be a no-op (idempotent)
3. Verify: ledger has only 1 WITHDRAWAL_COMMITTED drain entry for the withdrawal

If duplicate created duplicate ledger entry:
→ This is a CRITICAL verifier finding
→ DO NOT auto-repair
→ Escalate: preserve both rows, identify root cause, manual admin correction
```

### F4. Verifier CRITICAL Finding

```
Severity: BLOCKER — stop all payout operations immediately

Diagnosis:
1. POST /admin/finance/verify?mode=forensic → read full findings JSON
2. Note: section name, code, class, detail
3. GET /admin/finance/ledger → identify transaction sequence

Actions:
- DO NOT approve any new withdrawals
- DO NOT mark-processed any in-flight payouts
- DO NOT run strict mode (may panic, adds noise)
- Capture full ledger export as evidence

Recovery:
- Identify the withdrawal_id or transaction_id in the finding detail
- If WITHDRAWAL_PENDING mismatch: count REQUESTED withdrawals, compare to WITHDRAWAL_PENDING.balance
- If extra REQUESTED withdrawal not in ledger: reject it (restores balance)
- Re-run verifier after each correction
```

### F5. Gateway ↔ Ledger Mismatch

```
Symptom: gateway shows payout settled, but withdrawal still SUBMITTED in DB

Diagnosis:
1. Check gateway dashboard for external_reference_id
2. GET /admin/payouts/withdrawals/:id → check status
3. GET /admin/finance/ledger?reference_type=withdrawal_settled → check if entry exists

Resolution:
If gateway settled but DB is SUBMITTED:
→ POST /admin/payouts/withdrawals/:id/mark-processed
→ This creates WITHDRAWAL_COMMITTED drain entry
→ Post-check: verifier must pass

If DB shows SETTLED but gateway shows failed:
→ CRITICAL — do not auto-repair
→ SELLER_PAYABLE may be under-credited
→ Requires manual admin ledger correction through canonical admin flow only
```

### F6. Reconciliation Mismatch Logged

```
Symptom: reconciliation log shows "stuck_payouts > 0" or "requires_manual_review > N"

This is INFORMATIONAL — reconciliation never mutates. Treat as alert, not incident.

Response:
1. grep "StuckPayoutInfo" in logs → get withdrawal_id, status, duration
2. Follow F1 or F2 playbook depending on status
3. After resolution, next reconciliation cycle should show stuck_payouts=0
```

### F7. Dispute Freeze Inconsistency

```
Symptom: verifier reports dispute_freeze mismatch
         OR: seller can withdraw more than (SELLER_PAYABLE - active_freezes)

Diagnosis:
1. POST /admin/finance/verify?mode=forensic → read dispute freeze section
2. Check dispute_freezes table: SELECT * FROM dispute_freezes WHERE status='active'
3. Verify: sum of frozen_amount ≤ SELLER_PAYABLE.balance for that seller

Resolution:
- If dispute_freeze.status='active' but dispute is resolved: requires dispute resolution flow
- If SELLER_PAYABLE is negative (should be impossible due to CHECK constraint):
  → CRITICAL — stop all operations, escalate
- Do not manually update dispute_freezes — use canonical dispute resolution endpoint
```

---

## G. CLEANUP READINESS GATES

The following objective criteria must ALL be met before any legacy-code cleanup batch is permitted.

| Gate | Measurable Criterion | Measurement Method |
|------|---------------------|-------------------|
| G1. Unified withdrawal stable | `USE_UNIFIED_WITHDRAWAL=true` continuously ≥ 7 days with zero verifier errors | Verifier log audit |
| G2. Zero wallet-finance drift | Verifier `passed=true` for 7 consecutive days | Daily verifier run |
| G3. Payout worker stable | Zero CRITICAL verifier findings since worker activation | Verifier log audit |
| G4. No stuck payouts at threshold | Zero withdrawals stuck > 60 min in SUBMITTED/SETTLING for 7 days | Reconciliation logs |
| G5. Reconciliation clean | `stuck_payouts=0` in all reconciliation cycles for 7 days | Recon worker logs |
| G6. Pilot expansion done | At least 3 whitelisted sellers have completed successful withdrawals | ledger export count |
| G7. No legacy path attempts | Zero panics from `WithdrawService.RequestWithdraw` | Server error logs |
| G8. Dispute freeze stable | Zero dispute freeze inconsistency findings in verifier | Verifier section audit |

**All 8 gates must be ✓ before a cleanup batch PR is opened.**

---

## H. CI / GOVERNANCE RECOMMENDATION

### H1. Verifier CI Mode

```go
// Run in CI as part of integration test suite:
POST /admin/finance/verify?mode=strict
// strict mode raises all findings to errors — any finding = CI failure
// Use forensic for pre-production checks (warnings don't block)
// Use strict in CI to prevent regression merges
```

### H2. Merge Blocking Rules

| Rule | Enforcement |
|------|-------------|
| Finance domain changes (verifier, ledger, accounts) | Must include verifier test (forensic passes) |
| New ledger transaction type | Must add to verifier invariant check |
| New account type | Must add to `account_types.go` AND `account_types_canonical_test.go` |
| Withdrawal flow changes | Must pass `POST /verify?mode=strict` in integration test |
| Config flag changes | Must update staging playbook config matrix |

### H3. Release Gating Rules

| Environment | Verifier Mode | Required Result |
|-------------|--------------|-----------------|
| CI (every PR) | strict | passed=true, error_count=0 |
| Staging deploy | forensic | passed=true, error_count=0 |
| Production deploy | strict | passed=true, error_count=0, warning_count=0 |

### H4. Operational Alert Recommendations

| Alert | Trigger | Severity |
|-------|---------|----------|
| Verifier error | `error_count > 0` on scheduled run | CRITICAL — page oncall |
| Stuck payout | withdrawal in SUBMITTED/SETTLING > 30 min | HIGH |
| Worker not running | `PAYOUT_ENABLE_WORKER=true` but worker log absent | HIGH |
| Reconciliation failure | reconciliation worker errors > 3 consecutive | MEDIUM |
| Pilot whitelist exhausted | all pilot sellers successfully paid, whitelist unchanged for 7d | LOW (expansion signal) |

### H5. Scheduled Verifier Run

```bash
# Recommend: daily cron POST /admin/finance/verify?mode=forensic
# Alert on: passed=false OR error_count > 0
# Log to: structured log with timestamp for audit trail
```

---

## I. REMAINING OPERATIONAL RISKS

| Risk | Probability | Mitigation | Owner |
|------|-------------|-----------|-------|
| Sandbox webhook never arrives | MEDIUM | `mark-processed` fallback exists; reconciliation detects | Operator |
| Whitelist UUID typo | LOW | `ValidatePayoutWorkerConfig` rejects invalid UUIDs at startup | Config review |
| Verifier 30s timeout on large DB | LOW | Verifier has explicit 30s context timeout; 500 returned, not crash | Monitor response time |
| Reconciliation misses stuck payout | LOW | Interval=10min, threshold=30min; worst case 40min detection lag | Acceptable for staging |
| `WITHDRAWAL_COMMITTED` drain missing | LOW | Covered by verifier invariant; would surface as CRITICAL finding | Verifier |
| Dispute opened while withdrawal pending | MEDIUM | Freeze applied to SELLER_PAYABLE; withdrawal must be rejected first | Operator flow F7 |
| Pilot whitelist expansion race | LOW | Worker picks up all PROCESSING; whitelist only affects new submissions | Staged rollout |

---

## J. VERDICT

**STAGING PLAYBOOK: READY**

All activation phases are grounded in running code. Guards, endpoints, and ledger invariants are verified in Task 51. The playbook is:
- Rollback-first: every phase has explicit rollback steps
- Evidence-driven: uses actual endpoint URLs, account types, and log strings from code
- Non-destructive: read-only diagnosis precedes every mutation
- Sequenced: each phase has a binary stop condition

**Next operator action:** complete Precheck Gate (Section A7), then execute Phase A verifier check.
