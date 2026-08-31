# LABUDA — CANONICAL MODERATION DESIGN v1

**Status:** OWNER APPROVED DESIGN  
**Scope:** Report / Case / Decision / Enforcement / Warning / Appeal / Governance Audit / Admin / Mobile  
**Implementation:** NOT STARTED  
**Authority:** Owner Business Decisions + ChatGPT Technical Design  
**Factual Inputs:** Audit 1, Audit 2, Audit 3  
**PRD:** Reference only; must not override approved Business Truth

---

# 1. PURPOSE

Labuda moderation harus dibangun ulang dari business truth, bukan dengan memperbaiki `GovernanceCase` secara bertahap.

Current implementation terbukti mencampur:

- Report;
- Case;
- Decision;
- Enforcement intent.

Selain itu terdapat:

- warning tanpa governance provenance;
- appeal yang secara faktual menunjuk Case;
- enforcement yang false-success;
- outbox retry broken;
- audit logging best-effort;
- auction moderation yang dapat meninggalkan state commerce tidak konsisten;
- parked/zombie moderation architecture.

Karena Labuda masih from zero dan belum memiliki production data yang harus dipertahankan, desain yang salah **harus diganti**, bukan dipertahankan melalui compatibility layer.

---

# 2. CANONICAL PRINCIPLE

## 2.1 Governance authority

Moderation memiliki authority atas:

```text
Report
Case
Decision
Enforcement
Warning
Appeal
Governance Audit History
```

Moderation menentukan:

> apa yang diputuskan terhadap suatu subject.

Moderation **tidak memiliki authority atas state internal target domain**.

---

# 3. CANONICAL ARCHITECTURE

```text
                    USER
                     │
                     │ Report
                     ▼
                  REPORT
                     │
                     │ correlated
                     ▼
                    CASE
                     │
                     │ governance decision
                     ▼
                  DECISION
                     │
                     ├──────────────► WARNING
                     │
                     │ consequence
                     ▼
                ENFORCEMENT
                     │
                     │ transactional outbox
                     ▼
                  OUTBOX
                     │
                     │ async delivery
                     ▼
                  WORKER
                     │
                     │ domain command
                     ▼
              TARGET DOMAIN
```

Appeal:

```text
DECISION
   │
   ▼
 APPEAL
   │
   ▼
APPEAL REVIEW
   │
   ▼
NEW DECISION
   │
   ▼
NEW ENFORCEMENT
```

Tidak ada inverse mutation langsung terhadap Decision lama.

---

# 4. CANONICAL ENTITY SET

## Required entities

### 4.1 Report

Represents:

> user-submitted allegation/report terhadap subject.

Minimum conceptual fields:

```text
id
reporter_id
subject_type
subject_id
reason_code
reason_note
evidence_snapshot
created_at
```

Report adalah immutable historical intake record.

Report tidak menyimpan:

- decision;
- enforcement status;
- target lifecycle state.

---

### 4.2 Case

Represents:

> governance investigation/review unit untuk satu subject.

Minimum conceptual fields:

```text
id
subject_type
subject_id
status
created_at
closed_at
```

Case tidak menyimpan:

- reporter sebagai authority;
- decision fields;
- enforcement execution state.

---

### 4.3 Decision

Represents:

> keputusan governance yang dibuat moderator.

Minimum conceptual fields:

```text
id
case_id
decided_by
action
decision_note
created_at
```

Decision immutable.

History tidak boleh di-overwrite.

---

### 4.4 Enforcement

Represents:

> eksekusi consequence yang diminta oleh Decision.

Minimum conceptual fields:

```text
id
decision_id
action
status
attempt_count
requested_at
started_at
completed_at
failure_code/reason
```

Enforcement adalah authority untuk:

> apakah consequence benar-benar berhasil dieksekusi.

`Decision = desired governance outcome`

`Enforcement = actual execution lifecycle`

---

### 4.5 Warning

Warning adalah consequence governance.

Relationship:

```text
Decision
   ↓
Warning
```

Warning tidak boleh berdiri sendiri.

Minimum conceptual provenance:

```text
decision_id
user_id
```

---

### 4.6 Appeal

Appeal menunjuk **Decision**, bukan Report dan bukan Case.

```text
Appeal
  ↓
Decision
```

Appeal adalah request untuk meninjau Decision tertentu.

---

### 4.7 Governance Audit History

Durable governance history harus mampu merekonstruksi:

```text
Report
→ Case
→ Decision
→ Enforcement
→ Warning
→ Appeal
→ new Decision
→ reversal Enforcement
```

Audit history harus durable.

`LogSafe` best-effort tidak boleh menjadi satu-satunya governance history.

Audit 3 sendiri menemukan `audit_events` append-only sudah tersedia tetapi moderation belum memanfaatkannya, sementara LogSafe dapat gagal tanpa rollback.

---

# 5. RELATIONSHIPS

Canonical cardinality:

```text
Report N → 1 Case

Case 1 → N Decision

Decision 1 → N Enforcement

Decision 1 → 0..1 Warning

Decision 1 → 0..N Appeal
```

Subject:

```text
subject_type + subject_id
```

Target types canonical:

```text
content
comment
for_sale
auction
user
```

`profile` secara business identity tetap direpresentasikan sebagai `user`.

`chat_message` **bukan bagian dari canonical moderation scope**.

---

# 6. REPORT → CASE

## Report

Multiple users dapat melaporkan subject yang sama.

Contoh:

```text
User A → Report → Content X
User B → Report → Content X
User C → Report → Content X
```

Semua dapat dikorelasikan ke:

```text
Case X
```

Duplicate constraint:

```text
same reporter + same subject
```

tidak boleh menghasilkan duplicate active report.

Namun:

```text
different reporter + same subject
```

tetap valid.

---

# 7. CASE INVARIANT

Canonical rule:

> **Satu active Case untuk satu moderation subject.**

PostgreSQL partial unique index digunakan untuk menegakkan invariant tersebut.

Terminal Case tidak pernah dibuka kembali.

Jika ada Report baru setelah Case terminal:

```text
Case #1
   ↓
terminal

new Report
   ↓
Case #2
```

Tidak ada:

```text
reopen
resurrect
revive
```

---

# 8. CASE LIFECYCLE

Gunakan state minimum.

Recommended:

```text
open
resolved
```

Interpretation:

```text
open
  = masih membutuhkan governance resolution

resolved
  = current governance review selesai
```

Decision tidak direpresentasikan sebagai Case status.

Enforcement juga tidak direpresentasikan sebagai Case status.

Dengan demikian tidak ada:

```text
Case.status = enforced
```

Case hanya menjawab:

> apakah governance case masih terbuka?

---

# 9. DECISION LIFECYCLE

Decision immutable.

Tidak ada:

```text
Decision.update(...)
Decision.override(...)
Decision.status = reversed
```

Jika review berikutnya mengubah outcome:

```text
Decision #1
   ↓
Appeal
   ↓
Decision #2
```

Decision #1 tetap utuh.

---

# 10. DECISION ACTION

Action harus controlled.

V1 tidak membutuhkan generic policy engine.

Actions dapat berupa target-appropriate governance consequences seperti:

```text
no_action
remove
restore
suspend
warning
```

Exact target/action matrix harus divalidasi oleh implementation audit sebelum coding.

Jangan membuat action matrix generik yang memungkinkan:

```text
moderation → arbitrary target mutation
```

---

# 11. ENFORCEMENT

Enforcement harus persistent.

Minimum lifecycle:

```text
pending
   ↓
processing
   ├── succeeded
   └── failed
```

Untuk retry:

```text
failed
   ↓
pending/retryable
   ↓
processing
```

Permanent failure harus dapat dibedakan dari transient failure.

Tidak perlu membuat distributed workflow engine.

---

# 12. CRITICAL TRANSACTION BOUNDARY

Decision creation harus atomik dengan enforcement intent dan outbox.

```text
BEGIN

create Decision
create Enforcement(status=pending)
create Outbox(event referencing Enforcement)

create governance audit record

COMMIT
```

Jika salah satu gagal:

```text
ROLLBACK ALL
```

Tidak boleh:

```text
Decision committed
↓
worker event missing
```

atau:

```text
Enforcement exists
↓
no Decision
```

---

# 13. ENFORCEMENT EXECUTION

Setelah commit:

```text
Enforcement pending
       ↓
Outbox
       ↓
Worker
       ↓
Target Domain Command
```

Target domain menjalankan mutation.

Kemudian:

```text
success
    ↓
Enforcement = succeeded
```

atau:

```text
failure
    ↓
Enforcement = failed
```

Admin UI harus membaca Enforcement state.

Bukan:

```text
Decision action = remove
→ tampil "Enforced"
```

Decision action sendiri **bukan proof execution**.

---

# 14. OUTBOX AUTHORITY

Outbox bukan Enforcement authority.

Outbox hanya:

> reliable delivery mechanism.

Relationship:

```text
Enforcement 1 → 1 Outbox delivery intent
```

Event minimum harus dapat mengidentifikasi:

```text
event_id
enforcement_id
```

Additional identifiers hanya ditambahkan jika memang diperlukan untuk execution/idempotency.

---

# 15. OUTBOX RETRY

Current retry implementation terbukti broken.

Canonical behavior:

```text
pending
  ↓
processing
  ↓
success
```

Failure:

```text
processing
  ↓
failed
  ↓
retryable
  ↓
processing
```

Worker crash harus dapat direcover.

Duplicate delivery harus aman karena executor wajib idempotent.

Dead-letter/permanent failure hanya digunakan bila failure memang tidak lagi retryable.

Tidak boleh ada status yang dapat di-fetch worker tetapi tidak dapat di-transition kembali.

---

# 16. TARGET DOMAIN AUTHORITY

Moderation:

```text
DECIDES
```

Target domain:

```text
MUTATES ITS OWN STATE
```

Forbidden:

```text
moderation → UPDATE content directly
moderation → UPDATE comment directly
moderation → UPDATE listing directly
moderation → UPDATE auction directly
moderation → UPDATE bids directly
moderation → UPDATE orders directly
moderation → UPDATE payments directly
moderation → UPDATE ledger directly
moderation → UPDATE user lifecycle columns directly
```

Yang benar:

```text
Moderation
   ↓
Enforcement
   ↓
Target Domain Command
   ↓
Target Domain mutation
```

---

# 17. CONTENT

Moderation dapat memutuskan removal/restoration.

Content domain tetap menjadi authority:

```text
contents.visibility
```

Restoration hanya valid jika deletion provenance menunjukkan bahwa removal berasal dari moderation.

Tidak boleh:

```text
Appeal
→ blindly restore
```

karena target mungkin sudah dihapus oleh owner/user melalui jalur non-moderation.

---

# 18. COMMENT

Sama dengan Content.

Moderation dapat memberikan Decision.

Comment domain menjadi authority untuk state comment.

Restore hanya boleh jika state provenance membuktikan bahwa removal sebelumnya berasal dari moderation.

---

# 19. FOR SALE

Moderation dapat meminta removal/withdrawal.

Executor berada di For Sale Domain.

Safe boundary:

```text
listing state
+
shipping quote invalidation
```

Moderation tidak boleh menyentuh:

```text
order
payment
ledger
seller proceeds
settlement
```

Jika listing sudah:

```text
sold
```

report tetap valid.

Tetapi moderation tidak boleh memaksa:

```text
sold → withdrawn
```

atau merusak terminal commerce state.

---

# 20. AUCTION

Auction adalah target paling sensitif.

Report tetap diperbolehkan terhadap auction.

Auction yang sudah memiliki bid juga dapat dikenai moderation decision.

Tetapi:

```text
Moderation
   ↓
Decision
   ↓
Enforcement
   ↓
Auction Domain
```

Auction Domain wajib menjadi authority untuk:

- auction state;
- bid state;
- winner state;
- order consequence;
- payment/settlement consequence bila ada;
- notification.

Moderation tidak boleh mengimplementasikan shortcut cancellation.

Current `CancelForModeration` yang meninggalkan bid/winner state tidak dipertahankan.

Itu **REPLACE**, bukan patch.

Audit 3 secara eksplisit mengidentifikasi auction moderation cancellation sebagai P1 yang harus diganti dengan command yang dimiliki Auction Domain.

---

# 21. USER / PROFILE

Profile report menggunakan:

```text
subject_type = user
```

Moderation dapat memutuskan user consequence.

Namun User Domain tetap menjadi authority lifecycle.

Moderation tidak boleh mengubah:

- seller subscription;
- subscription expiry;
- KYC;
- ledger;
- order;
- payment.

Suspension tidak otomatis membatalkan subscription.

Subscription tetap memiliki lifecycle sendiri.

Capability evaluation kemudian mempertimbangkan user lifecycle sesuai canonical seller capability rules.

---

# 22. WARNING

Canonical:

```text
Decision
   ↓
Warning
```

Warning wajib memiliki provenance.

Tidak ada standalone:

```text
POST /admin/warnings
```

yang membuat warning tanpa Decision.

Warning v1:

```text
active
   ↓
expired

active
   ↓
revoked
```

History tetap ada.

Tidak ada Strike system.

Tidak ada Violation entity.

Tidak ada automatic punishment escalation engine.

---

# 23. APPEAL

Appeal hanya tersedia untuk:

> affected party terhadap Decision yang memberikan consequence kepada dirinya/subject yang menjadi tanggung jawabnya.

Tidak ada appeal terhadap pure rejection/no-action.

Relationship:

```text
Appeal
   ↓
Decision #1
```

Review:

```text
Appeal
   ↓
Review
   ↓
Decision #2
```

Decision #1 tetap immutable.

---

# 24. APPEAL OUTCOME

Appeal dapat menghasilkan governance outcome baru.

Contoh:

```text
Decision #1
action = remove

Appeal
   ↓
Decision #2
action = restore
```

Kemudian:

```text
Decision #2
   ↓
Enforcement #2
   ↓
Target Domain
```

Tidak boleh langsung:

```text
Appeal
→ target.restore()
```

Appeal bukan target-domain executor.

---

# 25. RESTORATION

Restoration selalu merupakan:

```text
new Decision
+
new Enforcement
```

bukan inverse mutation otomatis.

Contoh:

```text
Original Decision
→ remove
→ Enforcement succeeded

Appeal
→ approved

New Decision
→ restore
→ New Enforcement
→ target domain restore command
```

Target domain wajib memvalidasi current state.

### Content/comment

Restore hanya jika provenance membuktikan removal berasal dari moderation.

### For Sale

Restore hanya jika lifecycle masih reversible.

`sold` tidak boleh di-restore.

### Auction

Auction restoration tidak menjadi capability v1 apabila terminal/bid state tidak dapat dipulihkan secara aman.

### User

Suspension dapat direstore bila current state masih compatible.

Ban/terminal lifecycle tidak boleh dibypass oleh appeal.

Audit 3 sudah mengidentifikasi pola ini: restoration harus melalui Decision + Enforcement baru dan tidak boleh menjadi blind restore.

---

# 26. EVIDENCE

Report membuat:

```text
immutable minimal evidence snapshot
```

Tujuan:

> mengetahui apa yang dilaporkan dan konteksnya saat report dibuat.

Snapshot bukan target authority.

Tidak membuat full object archival system.

Tidak membuat event sourcing.

Tidak membuat versioned copy seluruh domain.

Target live tetap authoritative untuk current state.

---

# 27. REASON

Canonical model:

```text
reason_code
reason_note
```

`reason_code`:

- controlled;
- queryable;
- analytics-friendly.

`reason_note`:

- optional;
- contextual;
- human explanation.

Tidak membuat policy engine.

Final taxonomy harus berupa daftar sederhana yang sesuai actual Labuda abuse/moderation cases.

---

# 28. GOVERNANCE AUDIT HISTORY

History minimum harus memungkinkan reconstruction:

```text
who reported
what was reported
when reported
which Case
which Decision
who decided
what consequence
which Enforcement
execution result
warning
appeal
appeal Decision
reversal Enforcement
```

Governance history harus durable.

Mutation dan governance audit record harus memiliki transaction boundary yang benar.

Best-effort logging tidak cukup untuk canonical governance history.

---

# 29. ADMIN AUTHORITY

Admin Console menjadi moderation workspace.

Moderator dapat:

### Cases

```text
list
inspect
inspect reports
inspect evidence
inspect decisions
inspect enforcement
make decision
```

### Enforcement

```text
view execution state
view failure
retry retryable execution
```

### Appeals

```text
list
inspect
review
produce new Decision
```

### Warnings

```text
view
```

Warning issuance dilakukan sebagai consequence dari Decision.

Tidak ada standalone warning mutation.

---

# 30. ADMIN TARGET MATRIX

| Target | Moderator can decide | Executor |
|---|---|---|
| Content | remove / restore bila valid | Content Domain |
| Comment | remove / restore bila valid | Comment Domain |
| For Sale | moderation removal/restore bila valid | For Sale Domain |
| Auction | moderation stop/restore bila valid | Auction Domain |
| User/Profile | moderation lifecycle consequence | User Domain |

Admin tidak diberikan generic:

```text mutate target
```

Admin hanya diberikan:

```text make governance decision
```

---

# 31. ADMIN UX INVARIANT

Admin UI tidak boleh menampilkan:

```text Enforced
```

hanya karena Decision berhasil dibuat.

UI harus membedakan:

```textDecision made
Enforcement pending
Enforcement processing
Enforcement succeeded
Enforcement failed
```

Ini langsung menghilangkan false-success problem yang ditemukan Audit 2/3.

---

# 32. MOBILE AUTHORITY

Mobile hanya membutuhkan:

```text
Create Report
View Own Reports
View relevant outcome
Appeal eligible Decision
View Own Warnings
```

Mobile tidak boleh melihat internal enforcement machinery yang tidak diperlukan.

Tidak boleh menjadi authority untuk:

- report status;
- decision;
- enforcement;
- target moderation state.

---

# 33. API RESOURCE MODEL

Canonical conceptual API:

```text
Reports
Cases
Decisions
Enforcements
Appeals
Warnings
```

User:

```text
POST   /reports
GET    /reports/mine
GET    /cases/:id
GET    /decisions/:id
POST   /appeals
GET    /appeals/mine
GET    /warnings/mine
```

Admin:

```text
GET    /admin/cases
GET    /admin/cases/:id
POST   /admin/cases/:id/decisions
GET    /admin/cases/:id/decisions
GET    /admin/enforcements/:id
POST   /admin/enforcements/:id/retry
GET    /admin/appeals
PUT    /admin/appeals/:id/review
GET    /admin/warnings
```

Exact URL naming tetap merupakan implementation contract dan boleh disesuaikan selama authority dan resource boundary tidak berubah.

---

# 34. AUTHORIZATION

Authorization harus enforced server-side.

Minimal capabilities:

```text
moderation.report.read
moderation.case.read
moderation.case.decide
moderation.enforcement.view
moderation.enforcement.retry
moderation.appeal.read
moderation.appeal.review
moderation.evidence.read
moderation.warning.read
```

Capability naming dapat disederhanakan selama privilege boundary tetap jelas.

---

# 35. DATABASE INVARIANTS

Required:

### Report

```text
PK(report.id)

unique(reporter_id, subject_type, subject_id)
```

sesuai duplicate policy.

### Case

```text
PK(case.id)

partial unique active case:
(subject_type, subject_id)
WHERE case is active
```

### Decision

```text
PK(decision.id)
FK(decision.case_id)
```

### Enforcement

```text
PK(enforcement.id)
FK(enforcement.decision_id)
```

Idempotency identity harus durable.

### Warning

```text
PK(warning.id)
FK(warning.decision_id)
```

dan duplicate protection terhadap Decision/User.

### Appeal

```text
PK(appeal.id)
FK(appeal.decision_id)
```

Concurrency constraint harus mencegah duplicate active appeal bila business rule mengharuskannya.

---

# 36. POLYMORPHIC SUBJECT

PostgreSQL tidak dapat memberikan normal FK terhadap:

```text
subject_type + subject_id
```

ke lima tabel berbeda.

Karena itu:

- domain validation wajib;
- subject existence harus divalidasi;
- action executor harus melakukan current-state validation;
- polymorphic reference tidak boleh dianggap relational integrity penuh.

Jangan membuat generic cross-domain foreign-key abstraction.

---

# 37. IDEMPOTENCY

Enforcement harus idempotent.

Jika worker menjalankan dua kali:

```text
same enforcement
+
same target
```

hasil akhirnya harus tetap konsisten.

Contoh:

```text
remove content
```

tidak boleh:

```text
remove
→ side effect #1
→ duplicate side effect #2
```

Target domain harus menyediakan command yang aman terhadap retry.

---

# 38. CONCURRENCY

Critical races:

### Report duplicate

Concurrent:

```text
User A → Report X
User A → Report X
```

DB constraint harus menjadi final guard.

### Active Case

Concurrent:

```text
Report A → Case
Report B → Case
```

Partial unique index memastikan satu active Case.

### Admin Decision

Case/decision operation harus memiliki concurrency control.

### Enforcement

Worker duplicate delivery harus aman melalui idempotency.

---

# 39. FORBIDDEN ARCHITECTURES

Tidak boleh dibuat atau dipertahankan:

```text
GovernanceCase super-entity
Report+Case+Decision single row
enforced-as-case-status
outbox-as-enforcement
standalone warning
Strike
Violation subsystem
generic policy engine
case assignment engine
escalation engine
SLA workflow engine
case merge/split engine
event sourcing
distributed saga
microservice moderation
DomainAction architecture
AppealReversalService parallel authority
compatibility layer
legacy alias
fallback to old moderation model
```

Business Truth dan Audit 3 secara eksplisit menolak overengineering tersebut.

---

# 40. CURRENT DESIGN TO KILL

Implementation berikut harus dianggap rejected:

```text
GovernanceCase
moderation_cases
enforced case status
moderation event as enforcement authority
standalone warning creation
appeals.report_id
AuctionService.CancelForModeration
worker direct user lifecycle mutation
broken outbox retry
DomainAction
DomainActionWorker
AppealReversalService
chat_message moderation target
fixed_price_sale moderation vocabulary
removed moderation status residue
false-success admin badge
```

Tidak boleh dibuat compatibility wrapper terhadap desain tersebut.

---

# 41. CLEANUP REQUIREMENT

Setelah canonical implementation terbukti:

cleanup harus mencakup seluruh producer/consumer/residue yang relevan:

```text
entity
DTO
mapper
repository
service
handler
route
schema
migration
worker
event
outbox contract
admin API
admin UI
mobile API
mobile repository
mobile UI
tests
fixtures
docs
comments
fallback
alias
feature flag
```

Hanya residue yang terbukti obsolete dalam scope yang dihapus.

"Belum jelas" bukan alasan otomatis menghapus.

Tetapi:

> jika sudah terbukti rejected/obsolete, jangan dipertahankan hanya karena historical compatibility.

---

# 42. FINAL AUTHORITY MATRIX

| Concern | Authority |
|---|---|
| Report | Report domain |
| Case | Case domain |
| Decision | Decision domain |
| Enforcement execution state | Enforcement domain |
| Async delivery | Outbox infrastructure |
| Content visibility | Content domain |
| Comment visibility | Comment domain |
| For Sale lifecycle | For Sale domain |
| Auction lifecycle | Auction domain |
| Bid/winner | Auction domain |
| Order | Order domain |
| Payment | Payment domain |
| Ledger | Finance/Ledger domain |
| User lifecycle | User domain |
| Seller subscription | Seller Subscription domain |
| Warning | Governance/Moderation |
| Appeal | Governance/Moderation |
| Governance audit history | Audit/Governance history |

---

# 43. FINAL LIFECYCLE MODEL

```text
REPORT
  created
    │
    ▼
CASE
  open
    │
    ▼
DECISION
  immutable
    │
    ├─────────────┐
    ▼             ▼
ENFORCEMENT    WARNING
  │
  ├─ pending
  ├─ processing
  ├─ succeeded
  └─ failed
```

Appeal:

```text
DECISION
   │
   ▼
APPEAL
   │
   ▼
NEW DECISION
   │
   ▼
NEW ENFORCEMENT
```

No state is overwritten to pretend historical action never happened.

---

# 44. IMPLEMENTATION ORDER

Implementation harus dilakukan bertahap.

## Phase 1 — Schema/Foundation

- canonical tables;
- relationships;
- constraints;
- enums;
- audit history;
- outbox relationship.

## Phase 2 — Report

- report entity;
- intake;
- duplicate protection;
- evidence snapshot;
- reason taxonomy.

## Phase 3 — Case

- case entity;
- Report → Case correlation;
- active-case invariant;
- lifecycle.

## Phase 4 — Decision

- immutable Decision;
- moderator authorization;
- decision transaction.

## Phase 5 — Enforcement

- enforcement entity;
- execution state;
- idempotency;
- retry.

## Phase 6 — Outbox

- transactional event creation;
- retry repair;
- worker execution;
- enforcement write-back.

## Phase 7 — Target Executors

Bounded one target at a time:

1. content;
2. comment;
3. for_sale;
4. user;
5. auction.

Auction mendapat dedicated implementation scope karena P1.

## Phase 8 — Warning

- Decision provenance;
- issuance;
- revoke/expiry;
- remove standalone path.

## Phase 9 — Appeal

- Decision relationship;
- eligibility;
- review;
- new Decision;
- reversal Enforcement.

## Phase 10 — Admin

- cases;
- evidence;
- decisions;
- enforcement status;
- appeals;
- warnings.

## Phase 11 — Mobile

- reports;
- outcomes;
- appeals;
- warnings.

## Phase 12 — Destructive Cleanup

Hapus rejected architecture end-to-end.

## Phase 13 — Regression Proof

- unit;
- integration;
- DB/migration;
- worker;
- admin;
- mobile;
- concurrency;
- idempotency;
- residue/negative proof.

---

# 45. ACCEPTANCE GATE

Moderation tidak boleh dinyatakan CLOSED sebelum:

### Business

- [ ] seluruh locked business rules terpenuhi;
- [ ] tidak ada unresolved business ambiguity.

### Authority

- [ ] satu authority per concern;
- [ ] moderation tidak menjadi commerce authority;
- [ ] no competing moderation architecture.

### Database

- [ ] Report/Case/Decision/Enforcement terpisah;
- [ ] active Case uniqueness terbukti;
- [ ] provenance FK/constraint sesuai;
- [ ] concurrency tested.

### Enforcement

- [ ] Decision tidak disamakan dengan execution;
- [ ] Enforcement persistent;
- [ ] outbox retry proven;
- [ ] worker write-back proven;
- [ ] duplicate delivery safe.

### Target

- [ ] content safe;
- [ ] comment safe;
- [ ] for_sale safe;
- [ ] auction safe;
- [ ] user safe;
- [ ] commerce boundaries intact.

### Warning

- [ ] no standalone warning;
- [ ] Decision provenance;
- [ ] duplicate protection.

### Appeal

- [ ] affected-party authorization;
- [ ] Decision relationship;
- [ ] immutable original Decision;
- [ ] new Decision for appeal outcome;
- [ ] restoration provenance.

### Admin

- [ ] execution status truthful;
- [ ] failed enforcement visible;
- [ ] target-specific information sufficient;
- [ ] privileged actions authorized.

### Mobile

- [ ] report contract correct;
- [ ] own report access safe;
- [ ] appeal contract correct;
- [ ] warning contract correct.

### Cleanup

- [ ] GovernanceCase removed;
- [ ] old moderation_cases authority removed;
- [ ] standalone warning path removed;
- [ ] old appeal Case relationship removed;
- [ ] DomainAction removed;
- [ ] AppealReversalService removed;
- [ ] fixed_price_sale residue removed;
- [ ] obsolete removed status removed;
- [ ] false-success UI removed;
- [ ] stale tests/docs/comments removed;
- [ ] no alias/fallback/compatibility bridge remains.

### Negative proof

Search must prove rejected designs cannot return.

---

# 46. OPEN ITEMS

Tidak ada lagi business decision besar yang perlu menahan desain dasar ini.

Yang masih harus ditentukan saat implementation audit:

1. final `reason_code` vocabulary;
2. exact evidence snapshot fields;
3. exact Case `open/resolved` semantics;
4. exact action matrix per target;
5. exact enforcement retry policy;
6. exact governance audit schema;
7. exact target-domain command contracts.

Ini adalah **technical implementation details**, bukan izin untuk mengubah canonical architecture.

Jika implementation audit menemukan business ambiguity baru:

STOP → `Fakta → Risiko → Opsi → Rekomendasi → Owner Decision`.

---

# 47. FINAL CANONICAL MODEL

```text
                         ┌──────────────┐
                         │    REPORT    │
                         │              │
                         │ reporter     │
                         │ subject      │
                         │ reason       │
                         │ evidence     │
                         └──────┬───────┘
                                │
                                ▼
                         ┌──────────────┐
                         │     CASE     │
                         │              │
                         │ subject      │
                         │ open/resolved│
                         └──────┬───────┘
                                │
                                ▼
                         ┌──────────────┐
                         │   DECISION   │
                         │              │
                         │ actor        │
                         │ action       │
                         │ note         │
                         └──────┬───────┘
                                │
                    ┌───────────┴───────────┐
                    ▼                       ▼
             ┌──────────────┐       ┌──────────────┐
             │ ENFORCEMENT  │       │   WARNING    │
             │              │       │              │
             │ pending      │       │ decision_id  │
             │ processing   │       │ user_id      │
             │ succeeded    │       └──────────────┘
             │ failed       │
             └──────┬───────┘
                    │
                    ▼
             ┌──────────────┐
             │    OUTBOX    │
             │  delivery    │
             └──────┬───────┘
                    │
                    ▼
             ┌──────────────┐
             │    WORKER    │
             └──────┬───────┘
                    │
                    ▼
        ┌─────────────────────────────┐
        │        TARGET DOMAIN        │
        │                             │
        │ Content / Comment           │
        │ For Sale / Auction / User  │
        └─────────────────────────────┘


Decision ───────► Appeal
                    │
                    ▼
                 Review
                    │
                    ▼
              New Decision
                    │
                    ▼
           New Enforcement
```

## CANONICAL DESIGN STATUS

**APPROVED FOR IMPLEMENTATION PLANNING**

Bukan berarti implementation boleh langsung dikerjakan seluruhnya dalam satu prompt.

Langkah berikutnya harus:

> **Implementation Planning Audit**

Kita pecah menjadi bounded implementation slices dengan acceptance gate masing-masing. Slice pertama sebaiknya **Schema/Foundation**, karena semua entity dan authority berikutnya bergantung kepadanya.

Tidak boleh mulai dari UI, tidak boleh mulai dari admin, dan tidak boleh menambal `GovernanceCase`.

**GovernanceCase → mati.  
Report → Case → Decision → Enforcement → Target Domain → canonical.**