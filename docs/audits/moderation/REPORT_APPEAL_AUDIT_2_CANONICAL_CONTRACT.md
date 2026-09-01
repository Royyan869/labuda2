# APPEAL CANONICAL CONTRACT AUDIT 2 — READ-ONLY VERIFICATION

- **Date:** 2026-09-01
- **Mode:** READ-ONLY — no code changes, no migrations, no deletions, no renames
- **Authority order:** Business Truth > Design > Specification > Filesystem > Previous audits
- **Evidence rule:** Every claim cites exact document, section, and line numbers
- **Input:** Forensic Audit 1 (`REPORT_APPEAL_AUDIT_1_FORENSIC.md`), canonical Business Truth, Design, Specification

---

## CANONICAL TARGET

**Appeal → Decision.** This is locked and immutable across all three canonical documents.

| Source | Section | Evidence |
|---|---|---|
| Business Truth | §24 | "Appeal adalah challenge terhadap **Decision**." Diagram: `Case → Decision ↑ Appeal` |
| Business Truth | §I8 | "Appeal menunjuk Decision." |
| Business Truth | §42 | "Appeal → Decision \| YES" |
| Design | §4.6 | "Appeal menunjuk **Decision**, bukan Report dan bukan Case." |
| Design | §5 | "Decision 1 → 0..N Appeal" |
| Design | §35 | `FK(appeal.decision_id)` |
| Specification | §10 | "Canonical relationship: Decision → Appeal. Bukan: Report → Appeal." |
| Specification | §15 | "Decision ◄───── Appeal" |

**NOT VALID targets:**
- `Appeal → Report` — explicitly rejected (BT §24: "Bukan: Appeal → report_id"; Spec §10: "Karena Report hanyalah allegation")
- `Appeal → Case` — implicitly rejected (Appeal targets the Decision, not the Case container)
- `Appeal → Enforcement` — not mentioned as a target

---

## PURPOSE

**Appeal challenges the outcome of a Decision that produced consequences against the appellant.**

| Source | Section | Evidence |
|---|---|---|
| Design | §23 | "Appeal hanya tersedia untuk: **affected party terhadap Decision yang memberikan consequence kepada dirinya/subject yang menjadi tanggung jawabnya.**" |
| Design | §23 | "Tidak ada appeal terhadap pure rejection/no-action." |
| Specification | §10 | "User mengajukan appeal terhadap hasil moderation yang benar-benar diputuskan." |
| Business Truth | §1 | Goal 7: "menyediakan appeal" (of moderation decisions with consequences) |

**What is NOT appealable:**
- `no_violation` Decisions (pure rejection / no-action) — Design §23 explicitly excludes these
- Warning-only Decisions (passive record, admin can revoke directly) — Audit 3 §15

---

## ELIGIBILITY

**BUSINESS DECISION REQUIRED.**

| Source | Section | Evidence |
|---|---|---|
| Business Truth | §41.7 | "apakah setiap Decision tertentu dapat di-appeal? atau hanya enforcement decision?" — listed as **unresolved** |

**Established rules:**
1. Appeal requires a Decision with consequences (Design §23)
2. Pure rejection/no-action is NOT appealable (Design §23)
3. The appellant must be the affected party (Design §23)

**NOT established:**
- Whether ALL Decisions with consequences are appealable, or only enforcement Decisions
- The current implementation (enforced + rejected only) is one possible answer but is NOT canonically locked

**Classification:** This is genuinely absent from canonical documents. It is NOT merely a gap between current code and canonical design — the canonical documents themselves leave it open.

---

## MULTIPLE APPEALS

**Multiple Appeals per Decision are CANONICAL.**

| Source | Section | Evidence |
|---|---|---|
| Design | §5 | "Decision 1 → 0..N Appeal" — explicit cardinality allows zero or many |
| Design | §35 | "Concurrency constraint harus mencegah duplicate active appeal **bila business rule mengharuskannya.**" — conditional, not mandatory |

**What this means:**
- One Decision can have 0, 1, or N Appeals
- Duplicate active (pending) appeal prevention is a conditional business rule, not a hard invariant
- The canonical documents do NOT establish whether duplicate active appeals should be prevented

**Current implementation:** Prevents duplicate pending appeals (atomic CTE check in `CreateWithPendingCheck`). This is reasonable but is NOT canonically locked.

**Classification:** CANONICAL — multiple appeals per Decision are allowed. Duplicate active prevention is implementation choice.

---

## APPEAL STATE MACHINE

**The canonical documents do NOT explicitly enumerate Appeal states.**

What IS established:

| Source | Section | Evidence |
|---|---|---|
| Business Truth | §28 | Audit events: "Appeal submitted" and "Appeal decided" — two observable events |
| Design | §23 | Flow: "Appeal → Review → Decision #2" — three stages |
| Design | §43 | Lifecycle: "DECISION → APPEAL → NEW DECISION" — appeal is an intermediate stage |
| Business Truth | §25 | "Appeal bukan otomatis berarti: approved → blindly restore" — "approved" appears as a concept |
| Business Truth | §36 | "Tidak boleh: Appeal approved → blindly restore" — "approved" is used as appeal state |

**Vocabulary observed in canonical documents:**
- "Appeal submitted" — audit event (BT §776)
- "Appeal decided" — audit event (BT §777)
- "approved" — referenced as appeal outcome in BT §25, §36, §1012

**NOT established:**
- The exact enum values for appeal status
- Whether "pending", "approved", "rejected" is the canonical vocabulary
- Whether appeal states are independent lifecycle or derived from Decision #2

**Classification:** The state vocabulary is NOT canonically locked. The current `pending → approved/rejected` is a reasonable implementation choice. The critical invariant is: **appeal review produces Decision #2**, which is the governance authority — the appeal status is ancillary metadata.

---

## REVIEW — DOES APPEAL REVIEW CREATE DECISION #2?

**YES. This is CANONICAL and LOCKED.**

This is the single most important canonical rule for Appeal. It is stated in multiple places across all three documents.

| Source | Section | Evidence |
|---|---|---|
| Business Truth | §9 | Example: "Appeal → Decision #2. Outcome: reversed" |
| Business Truth | §9 | "Decision #1 tidak diubah menjadi Decision #2." |
| Business Truth | §25 | Canonical flow: "Appeal → Appeal Decision → Reversal / new governance decision → Enforcement" |
| Design | §23 | "Review: Appeal → Review → Decision #2" |
| Design | §23 | "Decision #1 tetap immutable." |
| Design | §24 | "Appeal dapat menghasilkan governance outcome baru." Example: "Decision #2 action = restore" |
| Design | §24 | "Decision #2 → Enforcement #2 → Target Domain" |
| Design | §25 | "Restoration selalu merupakan: new Decision + new Enforcement" |
| Design | §43 | Lifecycle diagram: "APPEAL → NEW DECISION → NEW ENFORCEMENT" |
| Design | §45 | Acceptance gate: "new Decision for appeal outcome" |
| Specification | §10 | "Appeal review menghasilkan governance outcome yang historical." |

**Decision #2 relationship to original Decision:**
- Same Case (Decision #2 belongs to the same Case as Decision #1)
- Append-only (Decision #1 is immutable, Decision #2 is new row)
- Different moderator (BT §24: "Appeal reviewer harus independen dari original decision maker")

**Decision #2 outcome vocabulary:**
- BT §9 example: "Outcome: reversed"
- Design §24 example: "action = restore"
- The canonical vocabulary for Decision #2 outcomes is NOT explicitly defined beyond the "reversed" example

**Decision #2 actor:**
- The appeal reviewer (admin/moderator)
- Design §29: "Appeals: list, inspect, review, **produce new Decision**"

**Decision #2 target:**
- Same Case as Decision #1
- Same subject (via Case → subject)

**Decision #2 must be created atomically with Enforcement #2:**
- Design §7: "TX2 (decision+enforcement+outbox): insert decision + enforcement (pending) + outbox event — satu tx"
- Design §24: "Decision #2 → Enforcement #2 → Target Domain" — the enforcement is the execution path

---

## ORIGINAL DECISION MUTATION

**Original Decision MUST remain immutable. This is an invariant.**

| Source | Section | Evidence |
|---|---|---|
| Business Truth | §9 | "Decision #1 tidak diubah menjadi Decision #2." |
| Design | §9 | "Tidak ada: Decision.update(...), Decision.override(...), Decision.status = reversed" |
| Design | §23 | "Decision #1 tetap immutable." |
| Design | §45 | Acceptance gate: "immutable original Decision" |
| Migration 000055 | decisions table | `trg_decisions_immutable` trigger: BEFORE UPDATE → RAISE EXCEPTION |

**This means:**
- Appeal review NEVER writes to the original Decision row
- A new Decision row is always created (Decision #2)
- The original Decision remains as historical truth
- "reversed" is a state of Decision #2, not a mutation of Decision #1

---

## REVERSAL SEMANTICS

**Reversal goes through Decision #2 + Enforcement #2 → Worker → Target Domain.**

Canonical flow (BT §25, Design §24-25):

```
Decision #1 (violation_confirmed, action=remove_content)
   ↓
Enforcement #1 (succeeded — content was removed)
   ↓
Appeal submitted
   ↓
Appeal review → Decision #2 (outcome: reversed/violation_overturned, action: restore_content)
   ↓
Enforcement #2 (pending → processing → succeeded)
   ↓
Worker → Target Domain restore command (e.g., ContentService.RestoreFromModeration)
   ↓
Target domain validates current state before executing
```

**Restoration rules (Design §25):**
- Content/comment: restore only if provenance shows moderation caused deletion
- For_sale: restore only if lifecycle still reversible (sold → no restore)
- Auction: NOT restorable in v1 if terminal/bid state
- User: suspension can be restored if current state compatible; ban cannot be bypassed

**What reversal does NOT do:**
- Does NOT mutate original Decision (invariant)
- Does NOT bypass target domain authority (BT §I9)
- Does NOT touch commerce directly (BT §I10)
- Does NOT blindly restore (BT §36, Design §25)

---

## ENFORCEMENT RELATIONSHIP

**Decision #2 produces Enforcement #2 through the same atomic transaction pattern as Decision #1.**

| Source | Section | Evidence |
|---|---|---|
| Design | §24 | "Decision #2 → Enforcement #2 → Target Domain" |
| Design | §25 | "new Decision + new Enforcement" |
| Design | §7 | TX boundary: "Decision + Enforcement(pending) + Outbox — satu tx" |

**Enforcement #2 lifecycle:**
- `pending → processing → succeeded/failed → retry`
- Same lifecycle as Enforcement #1
- Target domain executor same pattern (content/comment/for_sale/auction/user)

---

## ACTOR / REVIEWER

**Admin/moderator reviews appeal. Independence is guidance, not invariant.**

| Source | Section | Evidence |
|---|---|---|
| Business Truth | §24 | "Appeal reviewer harus independen dari original decision maker **jika memungkinkan** dalam role/permission model yang sederhana." |
| Design | §29 | Admin authority: "Appeals: list, inspect, review, produce new Decision" |
| Design | §33 | Admin API: `PUT /admin/appeals/:id/review` |
| Specification | §13 | "Appeal \| Appeal governance" — appeal is governance/moderation authority |

**Classification:** "jika memungkinkan" = guidance (best effort), not hard invariant. The reviewer MUST be an authorized admin (capability-gated), but independence from original decision maker is recommended, not required.

---

## AUDIT REQUIREMENTS

**Appeal lifecycle events must be in durable audit history.**

| Source | Section | Evidence |
|---|---|---|
| Business Truth | §28 | Audit must trace: "Appeal submitted, Appeal decided, Reversal executed" |
| Business Truth | §28 | "Audit harus reliable. LogSafe best-effort tidak cukup sebagai satu-satunya governance history." |
| Specification | §11 | "Appeal created/reviewed" must be traceable |
| Design | §4.7 | "Audit history harus durable. Mutation dan governance audit record harus memiliki transaction boundary yang benar." |

**What must be auditable:**
1. Appeal created/submitted — who, when, against which Decision
2. Appeal reviewed/decided — who, when, outcome
3. Decision #2 created — who, when, outcome, relationship to Decision #1
4. Enforcement #2 created — who, when, target
5. Reversal executed — target domain restoration result

**How:** Design §7 specifies audit should be in the SAME transaction as the mutation (not LogSafe). The existing `audit_events` table (append-only) is the canonical mechanism.

---

## COMMERCE IMPLICATIONS

**Appeal does NOT touch commerce directly. Restoration goes through target domain via Enforcement.**

| Source | Section | Evidence |
|---|---|---|
| Business Truth | §10, §I10 | "Moderation tidak mengambil alih order/payment/ledger." |
| Business Truth | §I9 | "Target domain tetap menjadi authority target state." |
| Design | §24 | "Tidak boleh langsung: Appeal → target.restore()" |
| Design | §24 | "Appeal bukan target-domain executor." |
| Design | §25 | "Target domain wajib memvalidasi current state." |

**What this means:**
- Appeal review produces Decision #2 + Enforcement #2
- Enforcement #2 is processed by Worker → Target Domain service
- Target Domain service decides whether restoration is valid against current state
- No commerce table is mutated by appeal code directly
- For_sale restoration goes through `ForSaleService.RestoreFromModeration` (which has sold-guard)
- Auction restoration is NOT available in v1 for terminal state

---

## AUTHORITY SEPARATION

**Each entity has distinct authority. Appeal does NOT become authority for Decision/Enforcement/Case.**

| Source | Section | Evidence |
|---|---|---|
| Specification | §13 | Authority table: "Appeal \| Appeal governance" |
| Business Truth | §I15 | "Tidak ada competing moderation authority." |
| Design | §41 | "AppealReversalService parallel authority" is in the KILL list |

**Separation:**

| Entity | Authority |
|---|---|
| `cases.status` | Case lifecycle (open → resolved) |
| `decisions.outcome` | Governance decision (no_violation / violation / reversed) |
| `enforcements.status` | Execution lifecycle (pending → processing → succeeded/failed) |
| `appeals.*` | Appeal request lifecycle (submitted → decided → Decision #2 created) |

**Appeal does NOT become authority for:**
- Decision outcome (Decision #2 is the authority)
- Enforcement status (Enforcement #2 is the authority)
- Case lifecycle (Case is the authority)

---

## CURRENT IMPLEMENTATION COMPARISON

| Canonical Rule | Current Implementation | Match | Gap |
|---|---|---|---|
| Appeal → Decision | Appeal → GovernanceCase (via `CaseID` field + `ModerationRepository`) | **NO** | Must point to Decision via `decision_id` FK |
| Appeal targets Decision with consequences | Appeal targets enforced/rejected GovernanceCase | **PARTIAL** | Uses legacy `GovernanceCase.Status` as proxy for Decision outcome |
| Decision #2 created on review | NOT created — appeal is standalone with status only | **NO** | ReviewAppeal must INSERT Decision + Enforcement atomically |
| Original Decision immutable | No Decision exists yet in Go code | **N/A** | Invariant will be enforced when Decision entity is used |
| Appeal reviewer independence | Not enforced | **OK** | Guidance, not invariant |
| Multiple appeals per Decision | Allowed (pending-duplicate check only) | **OK** | Matches canonical "0..N" cardinality |
| Restoration via Enforcement | Restoration via outbox event + Worker | **PARTIAL** | Missing Decision #2 + Enforcement #2 — current path emits outbox directly from AppealService |
| Appeal eligibility | enforced + rejected cases | **PARTIAL** | BD-1: eligibility scope not canonically locked |
| Appeal states | pending / approved / rejected | **OK** | State vocabulary not canonically locked |
| Audit trail | `admin_audit_logs` via `LogSafe` | **NO** | Must use in-tx `audit_events` (BT §28, Design §4.7) |
| Authorization | `moderation.appeal.read` / `moderation.appeal.review` | **OK** | Capabilities are correct |
| Commerce safety | No commerce touch points | **OK** | Matches canonical boundary |
| Authority separation | GovernanceCase conflates Decision + Enforcement + Case | **NO** | Must separate into canonical entities |
| Schema relationship | `decision_id` FK exists (migration 000055) | **OK** | DB is already canonical |
| Go entity | `CaseID` (maps to dropped `report_id`) | **NO** | Runtime-dead against current schema |
| Repository SQL | References `report_id` (dropped) | **NO** | Runtime-dead — all queries fail |
| Restoration event payload | `case_id, appeal_id, resource_type, resource_id` | **PARTIAL** | Must include `decision_id`, `enforcement_id` |

---

## GENUINE BUSINESS DECISIONS REQUIRED

### BD-1: Appeal Eligibility Scope

**Status: GENUINELY ABSENT from canonical documents.**

BT §41.7 explicitly states:
> "apakah setiap Decision tertentu dapat di-appeal? atau hanya enforcement decision?"

This is listed as an **unresolved business decision**. The canonical documents do not resolve it.

**Current implementation:** Appeals allowed for enforced + rejected cases only.

**Recommendation:** Accept current behavior (enforced + rejected) as default. This aligns with Design §23 ("Tidak ada appeal terhadap pure rejection/no-action") and covers the two most common scenarios. Refinement possible later without breaking changes.

### BD-2: Appeal Review Rejection Path

**Status: NOT EXPLICITLY ESTABLISHED in canonical documents.**

The canonical documents describe the reversal path (appeal approved → Decision #2 reversed → Enforcement #2 → restoration) in detail. But they do NOT explicitly describe what happens when an appeal is rejected.

Two interpretations:
1. **No Decision #2 created** — original Decision stands, appeal status changes to "rejected"
2. **Decision #2 created with "upheld" outcome** — creates explicit governance record that the original Decision was reviewed and upheld

**Design §43** shows the canonical lifecycle as:
```
DECISION → APPEAL → NEW DECISION
```
This implies a Decision is ALWAYS created on appeal review (both approve and reject paths).

**However**, BT §25 only shows the reversal flow:
```
Appeal → Appeal Decision → Reversal / new governance decision → Enforcement
```
This could be read as only applying to approved appeals.

**Recommendation:** Create Decision #2 for BOTH paths (reversed and upheld). This is more auditable and matches the Design §43 lifecycle. Classification: this is an interpretation gap, not a genuine business ambiguity.

### BD-3: Duplicate Active Appeal Prevention

**Status: NOT LOCKED.**

Design §35: "Concurrency constraint harus mencegah duplicate active appeal **bila business rule mengharuskannya.**"

The "bila" (if) makes this conditional. The canonical documents do not establish whether duplicate active appeals should be prevented.

**Current implementation:** Prevents duplicate pending appeals.

**Recommendation:** Keep current behavior. It is a reasonable default and the current DB constraint supports it.

---

## SLICE A IMPLEMENTATION GATE

**UNBLOCKED.**

The canonical relationship and implementation contract are sufficiently clear:

1. `Appeal → Decision` is canonically locked (all three documents agree)
2. `decision_id` FK exists in the DB (migration 000055)
3. Appeal review MUST create Decision #2 (canonical invariant)
4. Original Decision MUST remain immutable (canonical invariant)
5. Restoration goes through Enforcement #2 → Worker → Target Domain (canonical pattern)
6. Appeal eligibility scope (BD-1) can default to current behavior
7. Rejection path (BD-2) can follow Design §43 lifecycle (create Decision #2)
8. State vocabulary can remain `pending → approved/rejected`

**Slice A can proceed without any further business decisions.**

The canonical contract is:
- `Appeal.decision_id → decisions(id)` — already in DB
- `AppealService` must use canonical Decision/Case repos instead of `ModerationRepository`
- `ReviewAppeal` must INSERT Decision + Enforcement atomically
- Appeal entity must use `DecisionID` instead of `CaseID`
- Repository SQL must use `decision_id` instead of `report_id`

---

## EVIDENCE / SOURCE REFERENCES

| Document | Section | Lines | Topic |
|---|---|---|---|
| Business Truth v1 | §1 | 32 | Goal 7: provide appeal |
| Business Truth v1 | §9 | 252-264 | Decision #1 immutability, Decision #2 example |
| Business Truth v1 | §10 | 270-290 | Decision ≠ Enforcement |
| Business Truth v1 | §24 | 668-700 | Appeal definition, target, required fields |
| Business Truth v1 | §25 | 703-735 | Appeal outcome, canonical flow, restoration rules |
| Business Truth v1 | §28 | 776-790 | Audit: "Appeal submitted, Appeal decided, Reversal executed" |
| Business Truth v1 | §36 | 1000-1030 | Restoration principle |
| Business Truth v1 | §41.7 | 1200-1201 | Appeal eligibility (unresolved) |
| Business Truth v1 | §42 | 1223 | "Appeal → Decision \| YES" |
| Business Truth v1 | §I8 | 1145 | "Appeal menunjuk Decision" |
| Design v1 | §2.1 | 50 | Governance authority includes Appeal |
| Design v1 | §4.6 | 253-263 | "Appeal menunjuk Decision, bukan Report dan bukan Case" |
| Design v1 | §4.7 | 265-280 | Governance audit history |
| Design v1 | §5 | 293-303 | "Decision 1 → 0..N Appeal" |
| Design v1 | §9 | 435-455 | Decision immutability, no update/override |
| Design v1 | §23 | 892-920 | APPEAL: eligibility, relationship, review → Decision #2 |
| Design v1 | §24 | 922-960 | APPEAL OUTCOME: Decision #2 + Enforcement #2 |
| Design v1 | §25 | 962-1010 | RESTORATION: new Decision + new Enforcement |
| Design v1 | §29 | 1079-1085 | Admin: "Appeals: list, inspect, review, produce new Decision" |
| Design v1 | §33 | 1226-1241 | API: `POST /appeals`, `PUT /admin/appeals/:id/review` |
| Design v1 | §35 | 1320-1330 | Appeal schema: `PK(id), FK(decision_id)` |
| Design v1 | §40 | 1441-1470 | KILL list: "AppealReversalService parallel authority" |
| Design v1 | §43 | 1530-1580 | Final lifecycle: "APPEAL → NEW DECISION → NEW ENFORCEMENT" |
| Design v1 | §44 | 1656-1662 | Phase 9 — Appeal: "Decision relationship, eligibility, review, new Decision, reversal Enforcement" |
| Design v1 | §45 | 1743-1760 | Acceptance gate: "immutable original Decision, new Decision for appeal outcome" |
| Specification v1 | §10 | 292-320 | Appeal = keberatan terhadap moderation outcome |
| Specification v1 | §11 | 330-340 | Audit: "Appeal created/reviewed" |
| Specification v1 | §13 | 370-390 | "Appeal \| Appeal governance" authority |
| Specification v1 | §15 | 470-480 | Conceptual model: "Decision ◄───── Appeal" |
| Specification v1 | §19 | 605-620 | DoD: "Appeal menunjuk governance outcome yang benar" |

---

## PROOF

| Check | Result |
|---|---|
| `go build ./...` (backend) | ✅ PASS (0 errors) |
| `go vet ./internal/governance/moderation/...` | ✅ PASS (0 errors) |
| `npx tsc --noEmit` (admin) | ✅ PASS (0 errors) |
| No code changes made | ✅ CONFIRMED |
| Only report file created | ✅ CONFIRMED |

---

## COMMIT

**No code commit.** Only the audit report file is created.

```
docs/audits/moderation/REPORT_APPEAL_AUDIT_2_CANONICAL_CONTRACT.md (NEW)
```

---

## PUSH

**No push.** This is an audit-only checkpoint.

---

## WORKING TREE

| Change | File |
|---|---|
| NEW | `docs/audits/moderation/REPORT_APPEAL_AUDIT_2_CANONICAL_CONTRACT.md` |

No other files modified.

---

## FINAL VERDICT

**PASS**

The canonical Appeal contract is sufficiently clear for implementation:

1. **Appeal → Decision** is canonically locked across all three documents
2. **Decision #2 creation on review** is canonical and stated explicitly in 10+ locations
3. **Original Decision immutability** is a canonical invariant
4. **Reversal flow** (Decision #2 + Enforcement #2 + Worker) is fully specified
5. **Schema is already canonical** (migration 000055: `decision_id` FK)
6. **Two business decisions remain** (BD-1 eligibility, BD-2 rejection path) — both can be resolved with reasonable defaults without blocking implementation
7. **Slice A is UNBLOCKED**

The next implementation agent has enough evidence to rebuild Appeal without guessing or ambiguity on the core contract. The only decisions it needs to make are:
- Default eligibility scope (enforced + rejected — reasonable)
- Rejection path behavior (create Decision #2 "upheld" — more auditable)
