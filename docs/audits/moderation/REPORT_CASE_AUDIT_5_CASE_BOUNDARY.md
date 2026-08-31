# AUDIT 5 — CASE BOUNDARY AUDIT

- **Tanggal audit:** 2026-08-31
- **Mode:** READ-ONLY PRE-IMPLEMENTATION AUDIT — tidak ada implementasi
- **Satu-satunya artefak baru:** laporan ini
- **Baseline:** current filesystem (bukan git history)
- **Authority desain:** `LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1.md`, `LABUDA — CANONICAL MODERATION DESIGN v1.md`, `LABUDA — CANONICAL MODERATION SPECIFICATION v1.md`
- **Input faktual:** Slice 1 & 2 implementation (migration 000055–000057, entity/report.go, service/report_service.go, repository/report_repository.go)
- **Evidence rule:** setiap klaim disertai `file:line`, nama migration, tabel/index/constraint

---

## 1. Executive Factual Summary

### 1.1 Current State

**FACT:** Slice 2 (Report) telah diimplementasikan dan diverifikasi. Report runtime berjalan di atas `reports` table (migration 000057) dengan immutable trigger dan unique constraint untuk duplicate protection.

**FACT:** Database schema untuk `cases`, `decisions`, `enforcements` sudah dibuat oleh migration 000055. **Tetapi tidak ada Go code yang mengimplementasikan Case, Decision, atau Enforcement sebagai domain entity.**

**FACT:** Legacy `GovernanceCase` entity masih ada dan digunakan oleh Appeal domain (Slice 9 scope). `ModerationRepository.GetByID` runtime-dead (reads dropped table `moderation_cases`).

**FACT:** `DomainAction` entity dan worker PARKED — tidak ada migration, tidak ada application code yang membuat rows.

### 1.2 Gap Summary

| Canonical Entity | DB Schema | Go Entity | Service | Repository | Handler | Status |
|---|---|---|---|---|---|---|
| Report | ✅ | ✅ | ✅ | ✅ | ✅ | **COMPLETE** |
| Case | ✅ | ❌ | ❌ | ❌ | ❌ | **DB ONLY** |
| Decision | ✅ | ❌ | ❌ | ❌ | ❌ | **DB ONLY** |
| Enforcement | ✅ | ❌ | ❌ | ❌ | ❌ | **DB ONLY** |
| Warning | ✅ (altered) | ✅ (legacy) | ✅ (legacy standalone) | ✅ (legacy) | ✅ (legacy) | **LEGACY** |
| Appeal | ✅ (altered) | ✅ (legacy) | ✅ (legacy) | ✅ (legacy) | ✅ (legacy) | **LEGACY** |

---

## 2. Report → Case Map

### 2.1 Factual Relationship

**FACT:** Report entity (`entity/report.go:130-148`) memiliki field `CaseID *uuid.UUID` — nullable FK ke `cases` table.

**FACT:** Migration 000057 (`000057_report_slice_canonical_alignment.up.sql:57`) membuat `reports` dengan `case_id uuid` nullable, FK ke `cases(id) ON DELETE SET NULL`.

**FACT:** Tidak ada Go code yang mengisi `CaseID` saat ini. Field selalu nil karena Slice 2 tidak mengimplementasikan Report → Case correlation.

**FACT:** Report → Case correlation adalah scope Slice 3 (ini audit).

### 2.2 Canonical Cardinality (Design §5)

```text
Report N → 1 Case
Case 1 → N Decision
Decision 1 → N Enforcement
Decision 1 → 0..1 Warning
Decision 1 → 0..N Appeal
```

### 2.3 How Report → Case Should Work

Berdasarkan canonical design:

1. **Multiple Reports → 1 Case:** User A, B, C melaporkan content X → semua masuk Case X
2. **Subject identity:** `subject_type + subject_id` = Case identity
3. **One active Case per subject:** Partial unique index sudah ada di migration 000055
4. **Case correlation timing:** Report dapat dibuat sebelum Case ada (nullable `case_id`), atau Case langsung dibuat saat Report pertama

### 2.4 Ambiguities

**UNKNOWN / BUSINESS DECISION REQUIRED:**

1. **Correlation timing:** Apakah Case dibuat bersamaan Report pertama, atau ada batch/async correlation?
   - Design §7: "Case baru dapat dibuat ketika case sebelumnya sudah terminal"
   - Tidak menentukan kapan tepatnya Case dibuat

2. **Report submission flow:** Apakah user melaporkan → langsung Case dibuat, atau user melaporkan → Report disimpan → admin mengkorelasikan ke Case?
   - Design §32: "Canonical correlation v1: subject_type + subject_id"
   - Tidak menentukan siapa yang melakukan correlation (user flow vs admin flow)

3. **Case creation ownership:** Siapa yang membuat Case — ReportService saat Report pertama dibuat, atau AdminService saat review dimulai?

---

## 3. Case Entity / Schema Map

### 3.1 Database Schema (Migration 000055)

```sql
CREATE TABLE cases (
    id            uuid DEFAULT gen_random_uuid() NOT NULL,
    subject_type  moderation_target_type_enum NOT NULL,
    subject_id    uuid NOT NULL,
    status        case_status_enum DEFAULT 'open' NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    closed_at     timestamptz,
    updated_at    timestamptz NOT NULL DEFAULT now()
);
```

**Constraints:**
- `cases_pkey` — PK on `id`
- `cases_subject_id_not_null` — CHECK `subject_id IS NOT NULL`
- `uniq_active_case_per_subject` — UNIQUE `(subject_type, subject_id) WHERE status = 'open'`

**Indexes:**
- `idx_cases_subject` — `(subject_type, subject_id, created_at DESC)`
- `idx_cases_status` — `(status, created_at) WHERE status = 'open'`

**Status Enum:**
```sql
CREATE TYPE case_status_enum AS ENUM ('open', 'resolved');
```

### 3.2 Schema vs Canonical Design

| Aspect | Schema | Design | Match |
|---|---|---|---|
| Columns | id, subject_type, subject_id, status, created_at, closed_at, updated_at | id, subject_type, subject_id, status, created_at, closed_at | ✅ (updated_at = bonus) |
| Status values | open, resolved | open, resolved | ✅ |
| Active case invariant | Partial unique WHERE status='open' | "satu subject memiliki maksimal satu active Case" | ✅ |
| Case = moderation subject | subject_type + subject_id | "subject_type + subject_id" | ✅ |
| No decision fields | ✓ (no reviewed_by, decision_note, etc.) | "Case tidak menjadi tempat menyimpan seluruh history decision" | ✅ |
| No enforcement fields | ✓ | "Case juga tidak menjadi authority enforcement" | ✅ |

### 3.3 Missing Go Code

**FACT:** Tidak ada file `entity/case.go` atau `entity/canonical_case.go`.

**FACT:** Tidak ada `application/case_service.go`.

**FACT:** Tidak ada `infrastructure/repository/case_repository.go` atau `case_repository_impl.go`.

**FACT:** Tidak ada `delivery/http/case_handler.go`.

**INFERENCE:** Case adalah "DB-only" foundation — schema ready, implementation belum dimulai.

### 3.4 Case Lifecycle Factual Map

**Schema:** `open → resolved` (two values)

**Design §8:** 
```
open = masih membutuhkan governance resolution
resolved = current governance review selesai
```

**Design §7:** Terminal Case tidak pernah dibuka kembali. Report baru setelah terminal → Case baru.

**Ambiguity:** Tidak ada `under_review` atau `pending_assignment` status. Design §8 menyatakan "kita tidak perlu memaksakan terlalu banyak status" dan "Detail status internal boleh disesuaikan saat implementation design selama tidak mencampurkan decision dengan enforcement."

**BUSINESS DECISION REQUIRED:** Apakah `open` cukup untuk v1, atau perlu `under_review` (untuk membedakan Case yang belum dilihat admin vs sedang direview)?

---

## 4. Decision Boundary

### 4.1 Database Schema (Migration 000055)

```sql
CREATE TABLE decisions (
    id            uuid DEFAULT gen_random_uuid() NOT NULL,
    case_id       uuid NOT NULL,
    decided_by    uuid NOT NULL,
    outcome       decision_outcome_enum NOT NULL,
    decision_note text,
    created_at    timestamptz NOT NULL DEFAULT now()
);
```

**Constraints:**
- `decisions_pkey` — PK on `id`
- `decisions_case_id_fkey` — FK `case_id → cases(id) ON DELETE CASCADE`
- `decisions_decided_by_fkey` — FK `decided_by → users(id)`
- `trg_decisions_immutable` — BEFORE UPDATE → RAISE EXCEPTION

**Outcome Enum:**
```sql
CREATE TYPE decision_outcome_enum AS ENUM ('no_violation', 'violation');
```

### 4.2 Schema vs Canonical Design

| Aspect | Schema | Design | Match |
|---|---|---|---|
| Immutability trigger | ✅ `trg_decisions_immutable` | "Decision immutable" (Design §9) | ✅ |
| Case FK | ✅ `case_id → cases(id)` | "Decision → Case" (Design §5) | ✅ |
| Moderator identity | ✅ `decided_by → users(id)` | "moderator's authoritative decision" (Business Truth §2) | ✅ |
| Decision note | ✅ `decision_note text` | "operator note" (Business Truth §22) | ✅ |
| Append-only | ✅ (trigger blocks UPDATE) | "Decision adalah historical, append-only record" (Design §9) | ✅ |
| Multiple decisions per Case | ✅ (no unique constraint on case_id) | "Case 1 → N Decision" (Design §5) | ✅ |
| Outcome vocabulary | `no_violation`, `violation` | Design §5: "no_violation, violation" | ✅ |

### 4.3 Missing Go Code

**FACT:** Tidak ada `entity/decision.go`.

**FACT:** Tidak ada `application/decision_service.go`.

**FACT:** Tidak ada `infrastructure/repository/decision_repository.go`.

**INFERENCE:** Decision adalah "DB-only" foundation — schema ready, implementation belum dimulai.

### 4.4 Decision Boundary vs Legacy GovernanceCase

**FACT:** Legacy `GovernanceCase` (`entity/governance_case.go:30-52`) memiliki status: `pending`, `approved`, `rejected`, `enforced`. Ini mencampur Case lifecycle + Decision outcome dalam satu field.

**FACT:** `GovernanceCase.DecisionNote` dan `GovernanceCase.ReviewedBy` adalah mutable fields pada Case — melanggar invariant "Case ≠ Decision".

**Canonical boundary:**
```text
GovernanceCase.status = 'enforced'
  = Case + Decision + Enforcement dalam satu baris

Canonical:
  Case.status = 'resolved'  (hanya lifecycle)
  Decision.outcome = 'violation'  (hanya decision)
  Enforcement.status = 'succeeded'  (hanya execution)
```

### 4.5 Ambiguities

**BUSINESS DECISION REQUIRED:**

1. **Decision action vocabulary:** Schema saat ini hanya punya `outcome` (`no_violation`, `violation`). Design §10 menyebutkan action yang lebih spesifik: `no_action`, `remove`, `restore`, `suspend`, `warning`. Apakah perlu kolom `action` terpisah, atau `outcome` sudah cukup dan action adalah implikasi dari outcome + target type?

2. **Decision note requirement:** Apakah `decision_note` wajib untuk outcome `violation`? (seperti legacy `ErrEnforceRequiresNote`)

---

## 5. Enforcement Boundary

### 5.1 Database Schema (Migration 000055)

```sql
CREATE TABLE enforcements (
    id              uuid DEFAULT gen_random_uuid() NOT NULL,
    decision_id     uuid NOT NULL,
    target_type     moderation_target_type_enum NOT NULL,
    target_id       uuid NOT NULL,
    status          enforcement_status_enum DEFAULT 'pending' NOT NULL,
    attempt_count   integer DEFAULT 0 NOT NULL,
    requested_at    timestamptz NOT NULL DEFAULT now(),
    started_at      timestamptz,
    finished_at     timestamptz,
    last_error      text,
    next_attempt_at timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
```

**Constraints:**
- `enforcements_pkey` — PK on `id`
- `enforcements_decision_id_fkey` — FK `decision_id → decisions(id) ON DELETE CASCADE`
- `enforcements_attempt_count_nonneg` — CHECK `attempt_count >= 0`
- `enforcements_decision_target_unique` — UNIQUE `(decision_id, target_type, target_id)`

**Status Enum:**
```sql
CREATE TYPE enforcement_status_enum AS ENUM ('pending', 'processing', 'succeeded', 'failed');
```

### 5.2 Schema vs Canonical Design

| Aspect | Schema | Design | Match |
|---|---|---|---|
| Decision FK | ✅ `decision_id → decisions(id)` | "Enforcement berasal dari Decision" (Business Truth §10) | ✅ |
| Status lifecycle | ✅ `pending → processing → succeeded/failed` | "Minimal lifecycle: PENDING → PROCESSING → SUCCEEDED/FAILED" (Business Truth §11) | ✅ |
| Target identity | ✅ `target_type + target_id` | "consequence apa, target apa" (Business Truth §11) | ✅ |
| Attempt tracking | ✅ `attempt_count`, `started_at`, `finished_at`, `last_error` | "berapa attempt, apakah berhasil, jika gagal kenapa" (Business Truth §11) | ✅ |
| Retry support | ✅ `next_attempt_at` | "apakah dapat retry" (Business Truth §11) | ✅ |
| Immutability | ❌ (no trigger) | — | N/A (status mutates) |
| Idempotency | ✅ UNIQUE `(decision_id, target_type, target_id)` | "one Enforcement per (Decision, target, action)" | ✅ |

### 5.3 Critical Enforcement Boundary

**FACT (Business Truth §10):**
```text
Decision = apa yang diputuskan
Enforcement = apakah consequence tersebut berhasil diterapkan

Decision FINAL + Enforcement PENDING = state yang valid
Decision FINAL + Enforcement SUCCEEDED = state yang valid
Decision FINAL + Enforcement FAILED = state yang valid
```

**FACT (Design §13):**
```text
Admin UI harus membaca Enforcement state.
Bukan: Decision action = remove → tampil "Enforced"
```

**FACT (legacy violation):** `GovernanceCase.status = 'enforced'` berarti "decision enforce dibuat + outbox event di-emit" — bukan "target berubah". Ini adalah false-success yang harus dihentikan.

### 5.4 Missing Go Code

**FACT:** Tidak ada `entity/enforcement.go`.

**FACT:** Tidak ada `application/enforcement_service.go`.

**FACT:** Tidak ada `infrastructure/repository/enforcement_repository.go`.

**INFERENCE:** Enforcement adalah "DB-only" foundation — schema ready, implementation belum dimulai.

### 5.5 Enforcement vs Outbox

**FACT (Design §12):** Transaction boundary untuk Decision creation:
```text
BEGIN
  create Decision
  create Enforcement(status=pending)
  create Outbox(event referencing Enforcement)
  create governance audit record
COMMIT
```

**FACT (Business Truth §12):** Outbox bukan domain authority Enforcement. Outbox hanya reliable delivery mechanism.

**FACT:** Current `ModerationEventHandler` (`worker/moderation_event_handler.go`) menerima outbox event dan mengeksekusi target domain mutation, **tetapi tidak menulis hasil kembali ke database**. Tidak ada enforcement write-back.

**INFERENCE:** Enforcement write-back (status update ke `succeeded`/`failed` setelah worker selesai) belum diimplementasikan. Ini adalah P1 yang harus diatasi saat Slice 5 (Enforcement).

---

## 6. Admin Capability Requirements

### 6.1 Per Target Type

Berdasarkan Business Truth §29 dan Design §29-30:

| Target | Admin Can Inspect | Admin Can Decide | Executor |
|---|---|---|---|
| **Content** | subject content, author, visibility, moderation history | remove / restore (jika valid) | Content Domain |
| **Comment** | subject comment, author, parent content context | remove / restore (jika valid) | Comment Domain |
| **For Sale** | listing, seller, commerce state, order status | moderation removal/restore (jika valid) | For Sale Domain |
| **Auction** | auction, bid state, winner/highest bidder | moderation stop/restore (jika valid) | Auction Domain |
| **User/Profile** | user, moderation history | moderation lifecycle consequence | User Domain |

### 6.2 Admin Workspace Requirements

Design §29:
```text
Cases:
  list, inspect, inspect reports, inspect evidence,
  inspect decisions, inspect enforcement, make decision

Enforcement:
  view execution state, view failure, retry retryable execution

Appeals:
  list, inspect, review, produce new Decision

Warnings:
  view (issuance via Decision only)
```

### 6.3 Information Admin Needs Per Case

1. **Subject:** target_type, target_id, current target state (live)
2. **Reports:** all reports for this subject (reporter, reason, evidence_snapshot, created_at)
3. **Case:** status, created_at, closed_at
4. **Decisions:** all decisions (decided_by, outcome, decision_note, created_at) — immutable history
5. **Enforcements:** all enforcements per decision (status, attempt_count, last_error, started_at, finished_at)
6. **Prior Cases:** previous cases for same subject (if any)
7. **Warning history:** warnings issued for this user (if applicable)

### 6.4 Admin Authorization Capabilities

Design §34:
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

### 6.5 Unknowns

**BUSINESS DECISION REQUIRED:**

1. **Case assignment:** Apakah perlu admin assignment ke Case? Design §39: "no investigator assignment" — tetapi tidak menentukan apakah Case otomatis visible ke semua admin atau perlu assignment.

2. **Case priority:** Apakah perlu priority/sla pada Case? Design §39: "no SLA engine".

3. **Case filtering:** Bagaimana admin memfilter Case? By status? By target type? By date range?

---

## 7. Legacy / Residue Map

### 7.1 GovernanceCase

| Aspect | Factual Status | Classification |
|---|---|---|
| `entity/governance_case.go` | Masih ada, masih diimport oleh appeal_service.go, appeal_handler.go | **LEGITIMATE FUTURE DEPENDENCY** (appeal domain, Slice 9) |
| `entity/governance_case.go` status vocabulary | `pending/approved/rejected/enforced` — mencampur Case+Decision+Enforcement | **REJECTED** (harus diganti) |
| `NewGovernanceCase()` | Masih dipanggil oleh appeal_service_test.go | **LEGITIMATE** (test) |
| `CanTransition()` | Masih dipanggil oleh governance_case.go | **LEGITIMATE** (entity logic) |
| `ShouldEmitEnforcementEvents()` | Masih dipanggil oleh worker/outbox patterns | **DEAD/ZOMBIE** (moderation_cases dropped) |

### 7.2 ModerationRepository

| Aspect | Factual Status | Classification |
|---|---|---|
| `repository/moderation_repository.go` | Interface with `GetByID` only | **DEAD/ZOMBIE** (runtime-dead, reads dropped table) |
| `repository/moderation_repository_impl.go` | Queries `moderation_cases` table | **DEAD/ZOMBIE** (table dropped in 000056) |
| Wiring in `dependencies.go:2371` | Created for appeal domain compilation | **LEGITIMATE FUTURE DEPENDENCY** (appeal, Slice 9) |

### 7.3 DomainAction

| Aspect | Factual Status | Classification |
|---|---|---|
| `entity/domain_action.go` | Full entity with idempotency, execution groups, rollback | **PARKED/ZOMBIE** (no migration, no application code) |
| `infrastructure/repository/domain_action_repository.go` | Interface defined | **PARKED/ZOMBIE** |
| `infrastructure/repository/domain_action_repository_impl.go` | Implementation exists | **PARKED/ZOMBIE** |
| `worker/domain_action_worker.go` | Worker exists but never instantiated | **PARKED/ZOMBIE** (outbox_event_registry.go:198) |
| `outbox_event_registry.go` | "DomainActionWorker is PARKED: never instantiated" | **PARKED/ZOMBIE** |

### 7.4 ResourceType Entity

| Aspect | Factual Status | Classification |
|---|---|---|
| `entity/moderation_resource_type.go` | Includes `chat_message` | **LEGACY** (canonical scope excludes chat_message) |
| `ResourceType` enum values | `content, comment, for_sale, auction, user, chat_message` | **LEGACY** (chat_message harus dihapus) |

### 7.5 Appeal Repository

| Aspect | Factual Status | Classification |
|---|---|---|
| `appeal_repository_impl.go` reads `appeals.report_id` | Stores CaseID (legacy naming) | **LEGACY** (canonical: Appeal → Decision) |
| `appeal.CaseID` field | Maps to DB column `report_id` | **LEGACY** (harus diganti ke `decision_id`) |
| `appeals` table has `decision_id` FK | Migration 000055 adds `decision_id` | **ACTIVE** (canonical) |
| `appeals.report_id` column | Masih ada, masih dibaca oleh repository | **LEGACY** (harus di-drop setelah appeal rebuild) |

### 7.6 ModerationEventHandler

| Aspect | Factual Status | Classification |
|---|---|---|
| `worker/moderation_event_handler.go` | Handles content/comment/for_sale/auction/user enforcement | **ACTIVE CONSUMER** (outbox-based enforcement) |
| Reads `moderation_event_handler.go:50-52` `moderationRemovedPayload` | Contains `case_id, resource_type, resource_id` | **ACTIVE** (legacy payload format) |
| Writes enforcement result back to DB | **NO** — does not update enforcement status | **MISSING** (P1 — enforcement write-back) |

### 7.7 Warning Entity

| Aspect | Factual Status | Classification |
|---|---|---|
| `entity/warning.go` | Standalone warning without governance provenance | **LEGACY** (canonical: Decision → Warning) |
| `user_warnings` table has `decision_id` FK | Migration 000055 adds `decision_id NOT NULL` | **ACTIVE** (canonical) |
| Warning standalone creation path | `POST /admin/warnings` (routes_core.go:920-922) | **LEGACY** (harus dihapus) |
| Warning entity has `IssuedBy` but no `DecisionID` | Field not in Go entity | **LEGACY** (harus ditambah) |

---

## 8. Authority Map

### 8.1 Current Authority State

| Concern | Authority | Status |
|---|---|---|
| Report | ReportService + ReportRepository + `reports` table | ✅ **ACTIVE CANONICAL** |
| Case | **TIDAK ADA** (DB schema only) | ❌ **NOT IMPLEMENTED** |
| Decision | **TIDAK ADA** (DB schema only) | ❌ **NOT IMPLEMENTED** |
| Enforcement | **TIDAK ADA** (DB schema only) | ❌ **NOT IMPLEMENTED** |
| Warning | Legacy standalone (entity/warning.go) | ⚠️ **LEGACY** |
| Appeal | Legacy (entity/appeal.go + repository) | ⚠️ **LEGACY** |
| Governance audit | `admin_audit_logs` (best-effort) + `audit_events` (unused by moderation) | ⚠️ **INSUFFICIENT** |
| Target state | Target domain services (content/comment/for_sale/auction/user) | ✅ **ACTIVE** |
| Moderation event delivery | Outbox + ModerationEventHandler | ✅ **ACTIVE** (but no write-back) |

### 8.2 No Duplicate Authority for Report

**FACT:** Report runtime (Slice 2) menggunakan `reports` table. Legacy `moderation_cases` (yang mengandung report fields) telah di-drop (migration 000056). Tidak ada dual authority untuk Report.

### 8.3 No Authority for Case/Decision/Enforcement Yet

**FACT:** `cases`, `decisions`, `enforcements` tables exist (migration 000055) tetapi tidak ada Go code yang mengelola mereka.

**INFERENCE:** Database foundation sudah siap. Implementation Slice 3–5 akan membangun authority di atas foundation ini.

---

## 9. Business Ambiguities

### 9.1 Locked (Owner-Approved)

Berikut sudah locked berdasarkan Business Truth dan Design:

1. **One active Case per subject** — Design §7, Invariant I2
2. **Case lifecycle: open → resolved** — Design §8
3. **Case tidak pernah dibuka kembali** — Design §7: "Terminal Case tidak pernah dibuka kembali"
4. **Decision immutable** — Design §9, Invariant I3
5. **Decision append-only** — Design §9
6. **Decision ≠ Enforcement** — Business Truth §10, Invariant I4
7. **Enforcement persistent** — Business Truth §11, Invariant I5
8. **Case tidak boleh menjadi bukti enforcement success** — Business Truth §8, Invariant I6
9. **Outbox bukan enforcement authority** — Business Truth §12
10. **Target domain tetap menjadi authority** — Business Truth §13

### 9.2 Business Decision Required

1. **Correlation timing:** Kapan Case dibuat — saat Report pertama, atau saat admin review?
   - Rekomendasi: Case dibuat saat Report pertama untuk subject yang belum memiliki active Case
   - Alasan: memungkinkan multiple reports langsung masuk Case; admin hanya perlu resolve Case

2. **Report submission → Case creation ownership:** Siapa yang membuat Case?
   - Opsi A: ReportService membuat Case saat Report pertama
   - Opsi B: Admin membuat Case saat pertama kali review
   - Rekomendasi: Opsi A (otomatis, konsisten dengan "one active Case per subject")

3. **Decision action vocabulary:** Hanya `no_violation`/`violation`, atau perlu action lebih spesifik?
   - Design §10: "no_violation, remove, restore, suspend, warning"
   - Rekomendasi: tambah kolom `action` terpisah dari `outcome`

4. **Case reopening:** Sudah locked — "Terminal Case tidak pernah dibuka kembali"

5. **Report terhadap sold/ended object:** Sudah diizinkan (Audit 4 §41.2), tapi perlu dikonfirmasi

6. **Auction ber-bid moderation:** Sudah diizinkan dengan Auction Domain handling (Business Truth §41.3)

---

## 10. Implementation Risks

### 10.1 Schema Ready, Code Missing

**RISK: MEDIUM** — Database schema sudah siap (migration 000055–000057), tetapi tidak ada Go code. Implementation harus membangun: entity, service, repository, handler untuk Case, Decision, Enforcement.

**MITIGATION:** Schema foundation sudah terbukti oleh migration tests. Implementation hanya perlu membangun Go layer di atas schema yang sudah ada.

### 10.2 Legacy Appeal Domain Dependency

**RISK: HIGH** — Appeal domain (`appeal_service.go`, `appeal_handler.go`, `appeal_repository_impl.go`) masih menggunakan `GovernanceCase` dan `ModerationRepository.GetByID`. Saat Case authority baru diimplementasikan, Appeal domain harus di-rebuild.

**MITIGATION:** Appeal domain adalah Slice 9 scope. Case implementation (Slice 3) tidak boleh mempertahankan kompatibilitas dengan Appeal legacy. Appeal rebuild harus menggunakan Decision sebagai referensi, bukan Case.

### 10.3 Enforcement Write-Back Missing

**RISK: HIGH** — Current `ModerationEventHandler` tidak menulis hasil kembali ke database. Enforcement status tidak pernah di-update ke `succeeded`/`failed`. Admin UI tidak dapat melihat execution state.

**MITIGATION:** Slice 5 (Enforcement) harus mengimplementasikan enforcement write-back. Worker harus update `enforcements.status` setelah target domain mutation selesai.

### 10.4 Outbox Retry Broken

**RISK: HIGH** — Outbox retry runtime-broken (Audit 2 §4.2, Audit 4 §S). `MarkProcessing` hanya menerima `pending`, `FetchPendingBatch` mengambil `pending` + `failed`. Event `failed` di-fetch → `MarkProcessing` gagal → worker skip forever.

**MITIGATION:** Outbox fix harus dilakukan sebelum atau bersamaan dengan Enforcement implementation.

### 10.5 DomainAction Zombie

**RISK: LOW** — `DomainAction` entity dan worker sudah ada tetapi PARKED. Tidak ada migration, tidak ada application code. Tidak mengganggu implementation Case.

**MITIGATION:** DomainAction harus dihapus setelah Enforcement authority terbukti (cleanup slice).

### 10.6 Warning Standalone Path

**RISK: MEDIUM** — Current `POST /admin/warnings` membuat warning tanpa Decision provenance. Migration 000055 menambah `decision_id NOT NULL` pada `user_warnings`, tetapi Go entity dan handler masih menggunakan path standalone.

**MITIGATION:** Slice 8 (Warning) harus menghapus standalone path dan membangun Decision → Warning provenance.

---

## 11. Recommended Implementation Slices

Berdasarkan canonical design §44 (Implementation Order) dan current state:

### Slice 3 — Case Domain (SELANJUTNYA)

**Scope:**
- Case entity (`entity/canonical_case.go`)
- Case service (`application/case_service.go`)
- Case repository (`infrastructure/repository/case_repository.go`)
- Case handler (`delivery/http/case_handler.go`)
- Report → Case correlation logic
- Case lifecycle management (open → resolved)
- Active Case invariant enforcement
- Admin Case list/inspect endpoints

**Acceptance Criteria:**
1. Report pertama untuk subject yang belum memiliki active Case → Case dibuat
2. Report berikutnya untuk subject yang sama → masuk ke Case yang sama
3. Case hanya bisa resolved (tidak reopen)
4. Partial unique index aktif dan terbukti
5. Admin dapat list dan inspect Cases

### Slice 4 — Decision Domain

**Scope:**
- Decision entity (`entity/decision.go`)
- Decision service (`application/decision_service.go`)
- Decision repository
- Decision handler
- Decision creation (admin makes decision on Case)
- Decision immutability (trigger already exists)
- Case status update (open → resolved when Decision made)

**Acceptance Criteria:**
1. Decision dibuat oleh admin terhadap Case
2. Decision immutable (UPDATE ditolak trigger)
3. Multiple decisions per Case didukung
4. Case resolved setelah Decision final

### Slice 5 — Enforcement Domain

**Scope:**
- Enforcement entity (`entity/enforcement.go`)
- Enforcement service
- Enforcement repository
- Enforcement write-back (worker updates status)
- Outbox event referencing Enforcement ID
- Retry logic

**Acceptance Criteria:**
1. Enforcement dibuat saat Decision violation
2. Worker menulis hasil kembali (succeeded/failed)
3. Admin dapat melihat execution state
4. Retry support

### Slice 6–12 (sesuai Design §44)

---

## 12. Final Verdict

### **PASS WITH FINDINGS**

**PASS** karena:
1. Database foundation untuk Case, Decision, Enforcement sudah benar (migration 000055)
2. Schema sesuai dengan canonical design (partial unique, immutability trigger, proper FKs)
3. Tidak ada dual authority — legacy `moderation_cases` sudah di-drop
4. Report runtime (Slice 2) sudah siap sebagai input untuk Case
5. Authority map jelas — tidak ada competing authority

**FINDINGS** (non-blocking):

1. **Case/Decision/Enforcement belum ada Go code** — DB schema ready, implementation dimulai dari nol
2. **Correlation timing ambiguous** — business decision needed: kapan Case dibuat?
3. **Decision action vocabulary unclear** — perlu konfirmasi: hanya `outcome` atau perlu `action` column?
4. **Appeal domain masih legacy** — harus di-rebuild melawan Decision, bukan GovernanceCase
5. **Enforcement write-back missing** — worker tidak menulis hasil ke DB
6. **Outbox retry broken** — harus fix sebelum atau bersamaan Enforcement
7. **Warning standalone path masih ada** — harus dihapus saat Warning slice

**Tidak ada BLOCKER** untuk memulai Slice 3 (Case implementation).

---

*Audit selesai. Tidak ada implementasi yang dilakukan.*
