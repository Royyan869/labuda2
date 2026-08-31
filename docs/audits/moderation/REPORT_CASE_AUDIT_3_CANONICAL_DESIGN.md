# LABUDA — AUDIT 3: CANONICAL MODERATION DESIGN AUDIT

- **Tanggal audit:** 2026-08-30
- **Mode:** READ-ONLY DESIGN AUDIT — tidak ada implementasi, schema, migration, test, admin, mobile, atau commit
- **Satu-satunya artefak baru:** laporan ini
- **Authority desain:** `LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1.md` (root, draft ready for owner review)
- **Input faktual:** `docs/audits/moderation/REPORT_CASE_AUDIT_1_FACTUAL.md`, `REPORT_CASE_AUDIT_2_ENFORCEMENT_BOUNDARY.md`

---

## 1. Executive Summary

Business Truth v1 menetapkan model lima entitas governance terpisah:

```text
Report → Case → Decision → Enforcement → Target Domain
Warning (provenance Decision)
Appeal (menunjuk Decision)
```

Audit 1 & 2 membuktikan current implementation memakai satu super-entity `GovernanceCase` (`moderation_cases`) yang mencampur Report + Case + Decision, enforcement tanpa persisted execution state, outbox retry yang runtime-broken, warning tanpa provenance, appeal menunjuk Case (kolom `report_id`), dan zombie `DomainAction`/`AppealReversalService`.

Audit 3 menurunkan Business Truth menjadi spesifikasi teknis minimum: entity boundary, cardinality, lifecycle, authority, transaction boundary, API surface, target-domain command boundary, DB constraints, dan cleanup map.

**Temuan kunci:**

1. **Business Truth dapat diterjemahkan ke arsitektur teknis yang bersih** — tidak membutuhkan perubahan arsitektur fundamental di luar governance module. Target domain (content/comment/for_sale/auction/user) sudah memiliki authority service yang dapat menjadi executor. PROVEN.
2. **`GovernanceCase` harus di-REPLACE, bukan di-MODIFY.** Menambahkan kolom ke `moderation_cases` akan memperkuat super-entity — persis yang Business Truth larang (§37, §43).
3. **Auction `CancelForModeration` (P1 Audit 2) harus dibongkar** dan diganti dengan command yang di-own Auction Domain, bukan moderation. Business Truth §17 mengunci prinsip ini.
4. **Satu active Case per subject** feasible via partial unique index — pattern sudah dipakai di codebase (`uniq_active_auction_per_product`, `uniq_active_fixed_price_sale_per_product`).
5. **Outbox retry harus diperbaiki** (P1 Audit 2), dan Enforcement record menjadi authority execution state — outbox hanya delivery mechanism.
6. **Tidak diperlukan komponen enterprise** — Business Truth §39 eksplisit menolak Strike, Violation subsystem, policy engine, escalation, SLA, microservice, event-sourcing, saga. Design ini tetap simple.

**AUDIT STATUS: PROVEN** (semua klaim design didukung Business Truth + evidence code).

---

## 2. Current Implementation Inventory (relevant-only)

### Governance module (`backend/internal/governance/moderation/`)

| Lapisan | Artefak | Evidence |
|---|---|---|
| Entity | `GovernanceCase` (Report+Case+Decision fields) | `entity/governance_case.go:30-41` |
| Entity | `UserWarning` (standalone) | `entity/warning.go` |
| Entity | `Appeal` (CaseID field) | `entity/appeal.go:16` |
| Entity | `DomainAction` + `ExecutionGroup` — **PARKED** | `entity/domain_action.go`; `worker/outbox_event_registry.go:198-203` |
| Service | `ModerationService.CreateCase/ReviewCase` | `application/moderation_service.go` |
| Service | `WarningService` | `application/warning_service.go` |
| Service | `AppealService` | `application/appeal_service.go` |
| Service | `AppealReversalService` — **PARKED** | `application/appeal_reversal_service.go:1` |
| Repository | `moderation_repository_impl.go` (SQL ke `moderation_cases`) | `infrastructure/repository/` |
| Handler | `ModerationHandler`, `AppealHandler`, `WarningHandler` | `delivery/http/` |
| Routes | `/api/v1/moderation/*`, `/api/v1/admin/moderation/*`, `/appeals`, `/warnings` | `cmd/core_server/routes_core.go:762-775, 896-925, 1290-1316` |

### Worker & outbox

| Artefak | Status | Evidence |
|---|---|---|
| `OutboxWorker` | AKTIF (header STANDBY usang) | `dependencies.go:1250-1253`; `main.go:143` |
| `ModerationEventHandler` | AKTIF — enforcement per target | `worker/moderation_event_handler.go` |
| `DomainActionWorker` | **PARKED** | `worker/domain_action_worker.go` |
| `OutboxArchivalWorker` | AKTIF — archive succeeded | `dependencies.go:1673-1682` |
| Outbox retry `failed` | **P1 BROKEN** | `outbox_repository.go:231,277` vs `outbox_worker.go:447-449` |

### DB schema (`migrations/000001`)

| Tabel | Kolom relevan | Evidence |
|---|---|---|
| `moderation_cases` | `resource_type, resource_id, status, reported_by, reviewed_by, reason, decision_note, reviewed_at, deleted_at` | :963-976 |
| `appeals` | `report_id` (menyimpan CaseID), `appealed_by, status, message, admin_response, reviewed_by` | :454-466 |
| `user_warnings` | `user_id, issued_by, level, reason, is_active, revoked_at, revoked_by, expires_at` — **tanpa FK governance** | :1772-1783 |
| `outbox` | `status, retry_count, next_attempt_at, idempotency_key` | :1157-1169 |
| `outbox_archive` | copy outbox | :1171-1184 |
| `admin_audit_logs` | `actor_id, action_type, target_type, target_id, metadata` | :444-452 |
| `audit_events` | `event_type, entity_type, entity_id, actor_type, actor_id, payload_json` | :500-509 |

### Enums moderation

```sql
moderation_resource_enum: content, comment, for_sale, auction, user, chat_message  (000047 converged)
moderation_status_enum:   pending, approved, rejected, removed, enforced           (removed = ghost, :184-190)
```

### Target domain surface

| Target | Table | Status/lifecycle fields | Enforcement method (current) |
|---|---|---|---|
| content | `contents` | `status`(active/deleted), `is_hidden`, `visibility`(content_visibility_enum), `deleted_at` | `ContentService.SoftDeleteForModeration` / `RestoreFromModeration` (`content_service.go:729,769`) |
| comment | `comments` | `deleted_at` | `CommentService.SoftDeleteForModeration` / `RestoreFromModeration` (`comment_service.go:627,663`) |
| for_sale | `for_sales` (dulu `fixed_price_sales`) | `status`(draft/active/sold/withdrawn), `visibility`(public/private) | `ForSaleService.Withdraw` / `RestoreFromModeration` (`for_sale_service.go:358,407`) |
| auction | `auctions` | `status`(draft/scheduled/active/waiting_settlement/expired_bnr/ended/cancelled), `current_bid`, `current_winner_id`, `order_id` | `AuctionService.CancelForModeration` (`auction_service.go:1289`) — **P1** |
| user | `users` | `account_status`(active/suspended/banned), `deleted_at` | worker `handleUserAction` via `userRepo` (`moderation_event_handler.go:603-666`) |

### Capability cluster moderation

`moderation.case.read`, `moderation.case.resolve`, `moderation.evidence.read`, `moderation.appeal.read`, `moderation.appeal.review` — `capability.go:125-148`. (`moderation.content.view`, `moderation.content.remove` terdefinisi tapi tidak dipakai route — residue.)

---

## 3. Report Design

### 3.1 Identity

Report identity = **`report_id` (UUID)**. Satu baris = satu report. **PROVEN by Business Truth §3.1** ("setiap laporan user adalah record tersendiri").

### 3.2 Ownership

Report lifecycle dimiliki **Report domain/service** (dalam governance module). User hanya membuat; sistem menyimpan. Tidak ada mutation setelah create kecuali status internal (active/superseded). **PROVEN by BT §3.1, §13.**

### 3.3 Subject reference

Polymorphic reference:

```text
subject_type (content|comment|for_sale|auction|user)
subject_id  (UUID)
```

**Constraint penting (Business Truth §22, §37):** PostgreSQL **tidak bisa enforce FK terhadap polymorphic target**. FK `subject_id → contents.id` tidak mungkin untuk semua tipe sekaligus. Maka: **application-level validation** wajib (seperti current `ResourceExists` di `moderation_repository_impl.go:313-364`), dan **snapshot metadata** (title/author preview) disarankan agar governance history tidak bergantung pada live object (BT §23, §26). **UNPROVEN** di current (tidak ada snapshot).

### 3.4 Reporter

`reporter_id uuid` — dengan FK ke `users(id)` (authority identity). Current `moderation_cases.reported_by` tidak punya FK (`migrations/000001`). **Design: tambahkan FK pada entity Report.**

### 3.5 Reason

Pisahkan:

```text
reason_code  (controlled enum/code, owned backend — BT §22)
note         (free-text operator/user note, optional)
```

Current menggabung jadi satu `reason text` free-form (`moderation_service.go:87` — `binding:"required,min=1,max=500"`). **REPLACE.**

### 3.6 Duplicate constraint

```text
UNIQUE (reporter_id, subject_type, subject_id) — untuk active report
```

Bukan untuk semua report selamanya (supaya report baru setelah report lama terminal tetap bisa dibuat). Memungkinkan:

- reporter sama + subject sama → **satu** active report;
- reporter beda + subject sama → **banyak** report (masing-masing unik karena reporter beda).

**PROVEN by BT §5.**

### 3.7 Case correlation

Report → Case via `case_id FK` pada Report (nullable saat belum terkorelasi), atau relasi join `report_cases`. Correlation rule v1 (BT §4, §32): **`subject_type + subject_id` → satu active Case**. Saat report dibuat, cari active Case untuk subject; jika ada, attach; jika tidak, buat Case baru. **Bukan grouping engine.**

### 3.8 Terminal target

Jika target hidden/sold/ended/deleted saat report dibuat atau setelahnya:

- Report tetap sah sebagai governance history (BT §26: "Report yang sudah ada tetap menjadi governance history").
- Report terhadap sold/ended object: **tetap diizinkan** — BT §41.2 cenderung mempertahankan (report bisa datang setelah state berubah). **BUSINESS DECISION REQUIRED** untuk lock final.
- Enforcement terhadap deleted target: terminal — **tidak boleh restore** (BT §14, §26, §36).

---

## 4. Case Design

### 4.1 Cardinality

```text
Report → Case : many-to-one (banyak report → satu case)
Case → Report : one-to-many
```

**PROVEN by BT §4.** Report menyimpan `case_id` (nullable hingga terkorelasi) atau join table. **Design: Report.case_id FK.**

### 4.2 Active Case invariant

> Satu subject → maksimal satu active Case.

DB constraint yang dibutuhkan — **partial unique index** (pattern sudah dipakai):

```sql
CREATE UNIQUE INDEX uniq_active_case_per_subject
  ON cases (subject_type, subject_id)
  WHERE status IN ('open','under_review');  -- active states only
```

Pattern precedent: `uniq_active_auction_per_product` (`migrations/000001:2015`), `uniq_active_fixed_price_sale_per_product` (:2092), `idx_promotion_instances_active_target_unique` (:2187). **PROVEN feasible.**

- **Concurrency:** DB adalah enforcement terakhir (BT §33). Application check tidak cukup.
- **Soft deletion:** tidak dibutuhkan untuk Case — case terminal (`decided/closed`) tetap row (history), bukan di-delete. Partial unique mengecualikan terminal → case baru bisa dibuat. **Tanpa soft-delete lebih sederhana.**
- **Reopened Case:** BT §41.1 menandai sebagai business decision. Default rekomendasi: **case terminal = case baru berikutnya** (tidak reopen). Jika reopen diizinkan, partial unique perlu state transition kembali ke `under_review` — kompleksitas ekstra tanpa kebutuhan v1.

### 4.3 Case lifecycle (minimum)

```text
open → under_review → decided → closed
```

Atau lebih sederhana:

```text
open → decided (closed)
```

Current `pending → approved/rejected/enforced` menggabungkan workflow + decision + enforcement dalam satu status — **dilarang** (BT §8: "Tidak boleh menggunakan enforced sebagai Case state"). Case workflow state harus **terpisah** dari Decision state dan Enforcement state.

**Minimum state machine (design):**

| Konsep | States | Notes |
|---|---|---|
| Case workflow | `open` → `decided` (→ `closed` jika perlu) | Case selesai = decision final; enforcement failure tidak mengubah case (BT §10) |
| Decision | append-only rows | outcome: `no_violation` / `violation_confirmed` / `reversed` (contoh BT §9) |
| Enforcement | `pending → processing → succeeded/failed` | lifecycle sendiri (BT §11) |

---

## 5. Decision Design

| Aspek | Persyaratan | Evidence |
|---|---|---|
| 6.1 Identity | `decision_id` (UUID) | BT §9 |
| 6.2 Case | `case_id FK` → cases | BT §9 |
| 6.3 Maker | `decided_by` (admin UUID, FK users) | BT §9 "Moderator A" |
| 6.4 Outcome/action | `outcome` (controlled enum: no_violation/violation_confirmed/reversed) + `action` (consequence type, e.g. remove_content, suspend_account, warn) | BT §9, §20 |
| 6.5 Note | `note` free-text + optional `policy_code` | BT §22 |
| 6.6 Multiple per Case | **YA** — append-only; appeal menghasilkan Decision #2 untuk Case yang sama | BT §9 |
| 6.7 Immutable | Append-only: insert-only, no update/delete; appeal TIDAK mengubah Decision #1, menambah Decision #2 | BT §9 |
| 6.8 Current/latest | Determined by `created_at` DESC (atau `decision_sequence` int per case) | BT §9 |
| 6.9 Appeal | Appeal menunjuk Decision tertentu; appeal review menghasilkan Decision baru (`reversed`/`upheld`) | BT §24-25 |

**Design implication:** Decision membutuhkan table sendiri + index `(case_id, created_at DESC)`. Ini menggantikan `moderation_cases.status + decision_note + reviewed_by`.

---

## 6. Enforcement Design

Enforcement harus menjawab: **"Apakah consequence benar-benar dieksekusi?"** (BT §10-11).

| Aspek | Persyaratan minimum |
|---|---|
| Decision relationship | `decision_id FK` → decisions |
| Target | `subject_type + subject_id` (copy dari decision/subject) |
| Action | `action_type` (controlled: remove_content, remove_comment, withdraw_for_sale, cancel_auction, suspend_account, warn) |
| Lifecycle | `pending → processing → succeeded` / `pending → processing → failed → retry → ... → succeeded | permanent_failure` |
| Attempts | `attempt_count` int |
| Timestamps | `created_at`, `started_at`, `finished_at` |
| Failure reason | `last_error text` (nullable) |
| Retry state | `next_attempt_at`, `max_attempts` (bisa default constant) |
| Terminal failure | `permanent_failure` — case TIDAK kembali pending (BT §10: `Decision FINAL, Enforcement FAILED` valid) |
| Success | `succeeded` hanya setelah **target domain mutation berhasil** (BT §10-11, §30) |
| Idempotency identity | `enforcement_id` di payload event + `idempotency_key` (see §9) |

**Design implication:** Enforcement adalah entity + table sendiri. Ini menggantikan `DomainAction` (PARKED) dan outbox-event-as-enforcement.

---

## 7. Decision/Enforcement Transaction Boundary

**Persyaratan:** Decision + Enforcement-intent + Outbox harus **atomik** dalam satu transaksi. (BT §10: `Decision FINAL, Enforcement PENDING` valid — artinya enforcement record dibuat bersama decision, bukan nanti.)

Alasan:

- Mencegah "Decision dengan tidak ada Enforcement intent" (false no-consequence);
- Mencegah "Enforcement tanpa Decision" (orphan execution);
- Mencegah orphaned outbox work (event tanpa enforcement record);
- Mencegah false "enforced" state (case tidak menyimpan enforcement outcome).

```text
TX A (satu transaksi):
  INSERT decision (outcome=violation_confirmed, action=remove_content)
  INSERT enforcement (status=pending, decision_id, target, action)
  INSERT outbox (event=moderation.enforcement.requested, payload{enforcement_id, decision_id, target, action})
  → commit
```

Kemudian:

```text
Worker:
  fetch outbox event
  dispatch ke executor (target domain service)
  executor success → UPDATE enforcement SET status=succeeded, finished_at
  executor failure → UPDATE enforcement SET status=failed, attempt++, next_attempt_at
  retry → enforcement tetap di case yang sama, tidak membuat decision baru (BT §10)
```

**Write-back enforcement result adalah write terpisah dari worker tx** (eventual consistency). Enforcement record adalah source of truth execution state; outbox hanya delivery.

---

## 8. Outbox Design

### 8.1 Enforcement ↔ Outbox

Outbox adalah **mekanisme delivery**, bukan enforcement record (BT §12). Relationship: `Enforcement 1→1 Outbox event` (satu enforcement request = satu event; retry = event yang sama diproses ulang, bukan event baru).

### 8.2 Event identity (minimum)

Payload harus membawa:

```text
enforcement_id  — idempotency anchor (unik per enforcement)
decision_id     — traceability governance
case_id         — traceability (opsional, bisa via decision)
subject_type + subject_id — target
action_type     — consequence
```

Current payload: `{case_id, resource_type, resource_id, decision_note}` — **tidak punya enforcement_id/decision_id** (`moderation_service.go:351-366`). **REPLACE.**

Idempotency key outbox: `enforcement.<enforcement_id>` — unik per enforcement. **Penting:** current idempotency key `eventType.resourceID` (`outbox_repository.go:100`) collision-prone — dua enforcement berbeda untuk target sama (content dihapus 2x dari 2 case berbeda) akan collide → event kedua di-DO NOTHING (`ON CONFLICT idempotency_key`). **P1 design fix.**

### 8.3 Lifecycle

```text
pending → processing → succeeded
                   → failed → (retry) → pending (next_attempt_at) → ... → dead_letter
```

**Current lifecycle sudah punya 5 status** (`outbox_repository.go:28-39`) — cukup. **TAPI retry broken** (`MarkProcessing` hanya menerima `pending`, `FetchPendingBatch` mengambil `failed` — `outbox_repository.go:231,277` + `outbox_worker.go:447-449`). **REPLACE diperlukan:** `MarkProcessing` harus menerima `failed` (atau fetch hanya `pending` + jalur requeue terpisah). Ini P1 blocker untuk canonical enforcement reliability.

### 8.4 Retry semantics

| Skenario | Required behavior |
|---|---|
| Transient failure | `failed` + `retry_count++` + `next_attempt_at` (backoff) → requeue |
| Permanent failure | `dead_letter` setelah max attempts; enforcement = `permanent_failure`; case tetap decided |
| Worker crash | `processing` stale → reset ke `pending` (current `ResetStuckEvents` — perlu dipastikan bekerja; `outbox_repository.go:391-418`) |
| Duplicate delivery | worker idempotent via `enforcement_id` + target-domain idempotency |

### 8.5 Write-back

Worker result → `UPDATE enforcement SET status/attempt/last_error/finished_at`. **Design implication:** worker harus punya akses tulis ke enforcement table (via governance repository method), bukan hanya outbox.

---

## 9. Idempotency Design

Per-target identity untuk repeated delivery aman (BT §34):

| Target | Idempotency basis (design) | Current fit |
|---|---|---|
| content | `enforcement_id` + `contents.status=deleted` guard | **FIT** — `SoftDeleteForModeration` idempotent (`content_service.go:742-745`) |
| comment | `enforcement_id` + `deleted_at IS NULL` guard | **FIT** — `comment_service.go:640-643` |
| for_sale | `enforcement_id` + terminal state (withdrawn/sold) treated success | **FIT** — `moderation_event_handler.go:508-520` |
| auction | `enforcement_id` + terminal state treated success | **FIT secara idempotensi, TAPI command boundary salah** (P1 — §12) |
| user | `enforcement_id` + `account_status=='suspended'` guard | **FIT** — `moderation_event_handler.go:631-636` |

**Kesimpulan:** target-domain methods saat ini sudah idempotent dan **layak dipertahankan sebagai executor** (kecuali auction command boundary). Tidak perlu redesign idempotency; hanya perlu enforcement_id sebagai identity yang konsisten di seluruh retry.

---

## 10. Target-Specific Executor Matrix

| Target | Moderation decision | Executor | Target authority | Current fit |
|---|---|---|---|---|
| content | remove/hide content | `ContentService.SoftDeleteForModeration` | Content Domain (`contents` state) | **FIT** — moderation decides, content domain executes |
| comment | remove comment | `CommentService.SoftDeleteForModeration` | Comment Domain (`comments.deleted_at`) | **FIT** |
| for_sale | withdraw/hide listing | `ForSaleService.Withdraw` | For Sale Domain (`for_sales.status/visibility`) | **FIT** — side effect shipping quote invalidation dapat diterima (BT §16 mempertahankan prinsip ini) |
| auction | cancel/stop auction | **HARUS ganti** — `CancelForModeration` meninggalkan bid state | Auction Domain (bid/winner/settlement) | **VIOLATION** — moderation membatalkan auction ber-bid tanpa bidder consequence (Audit 2 P1) |
| user/profile | suspend account | **HARUS lewat user domain service** — current langsung `userRepo` (tanpa service method) | User Domain (`users.account_status`) | **PARTIAL** — mutation lewat repo (dalam domain), tapi tanpa service lifecycle validation |

**Rule yang dilanggar saat ini:** auction (moderation mengubah auction lifecycle tanpa auction-domain consequence untuk bidder). **Wajib diperbaiki dalam design.**

---

## 11. For Sale Design Feasibility

**VERDICT: FEASIBLE — boundary aman.**

`ForSaleService.Withdraw` (`for_sale_service.go:358-396`) hanya:

1. `MarkWithdrawn()` → `status=withdrawn` (`for_sale.go:161-173`)
2. `UpdateStatus`
3. `InvalidateQuotesByProduct` (shipping quotes — consequence logis dari listing non-aktif)
4. `InsertEvent("for_sale.withdrawn")` (promotion pause)

Tidak menyentuh: order, payment, ledger, seller proceeds, settlement (Audit 2 §8.2 — PROVEN).

**Batasan aman moderation:** moderation hanya boleh memanggil command listing-lifecycle (`withdraw` / restore-if-withdrawn). Jika for_sale sudah `sold` (stock claimed, `for_sale.go:199-201`) — **tidak boleh di-restore** (guard sudah ada). Jika listing punya order aktif — moderation tidak boleh menyentuh commerce state (BT §16: "moderation tidak boleh secara otomatis mengubah order/payment/ledger"). **Policy target-domain untuk listing terminal commerce state** (BT §16 penutup).

**Design requirement:** command moderation ke For Sale Domain = `WithdrawForModeration(forSaleID, reason)` — mempertahankan boundary saat ini, tambah provenance (reason/enforcement_id).

---

## 12. Auction Design Feasibility

**VERDICT: FEASIBLE hanya jika command boundary diubah — P1.**

Current `CancelForModeration` (`auction_service.go:1289-1320`):

- Bypass `CanCancel()` (bid guard) — `auction.go:615-624` hanya dipakai seller path (`auction_service.go:645-647`)
- Tidak ada OrderID guard (tidak seperti `AdminCancel` yang punya `applyAdminCancel` guard — `auction_service.go:1352-1380`)
- Tidak refund/void bid, tidak notifikasi bidder (`auction.cancelled` hanya konsumsi promotion — `outbox_worker.go:959`)
- Bid/winner state tersisa

**Design requirement (BT §17):**

```text
Moderation Decision
   ↓
Auction enforcement command (new, owned by Auction Domain)
   ↓
Auction Domain menangani konsekuensi bidder:
   - cancel auction
   - resolusi bid state (void/refund policy)
   - notifikasi bidder
   - tidak ada settlement/refund langsung oleh moderation
```

**Batas canonical:** moderation mengirim command "stop auction ini" — Auction Domain menentukan lifecycle consequence dan bidder consequence. Moderation **tidak boleh** melakukan `auction.Cancel()` langsung. Domain mana yang own refund/ledger: **Auction/Commerce Domain** (bukan moderation).

**Business decision still required (BT §41.3):** moderation boleh menghentikan auction ber-bid — direkomendasikan YES, dengan Auction Domain menangani bidder consequence.

---

## 13. User/Profile Design Feasibility

**VERDICT: FEASIBLE dengan perbaikan boundary.**

- Profile report = User target (BT §18) — tidak perlu entity Profile terpisah.
- Moderation memutuskan account consequence (suspend).
- **User Domain harus own mutation** — perlu service method `UserService.SuspendForModeration(userID, reason)` yang memvalidasi lifecycle, bukan worker langsung set `AccountStatus` (`moderation_event_handler.go:638-645`).
- **Batas ketat:** moderation tidak boleh menjadi seller-subscription authority (BT §13). Suspension tidak boleh otomatis: cancel subscription, refund, mengubah KYC, membatalkan order, atau mengubah seller tier kecuali User/Commerce domain memutuskan sendiri.
- **BUSINESS DECISION REQUIRED:** apakah suspension harus berinteraksi dengan listing/auction aktif seller (pause? cancel?) — domain user/auction policy, bukan moderation improvisasi.

---

## 14. Warning Design

| Aspek | Design |
|---|---|
| Relationship | `Decision → Warning` (BT §19) |
| Provenance field | `decision_id FK` — WAJIB non-null |
| Satu Decision → berapa Warning | **Satu** warning per decision (per user). Jika decision menargetkan banyak user (jarang v1), bisa multi — design default: 1 warning per (decision, user) |
| Duplicate | Unique `(decision_id, user_id)` — mencegah duplikat dari retry/decision yang sama (BT §34: retry → duplicate warning dilarang) |
| Lifecycle | `active → revoked` (admin) / `active → expired` (waktu) — BT §19 |
| History | Warning row append-only; revoke/expiry via field update (`is_active=false`, `revoked_at`) — mempertahankan history |
| Standalone | **DILARANG** — warning hanya dari Decision (BT §19: "Tidak ada standalone warning dari admin") |

Current `user_warnings` tidak punya FK governance + bisa dibuat standalone (`POST /admin/warnings` tanpa case reference — Audit 1 §10). **REPLACE.**

---

## 15. Appeal Design

| Aspek | Design |
|---|---|
| Relationship | `Appeal → Decision` (BT §24) — bukan Case, bukan Report |
| Eligibility | Appeal menunjuk Decision tertentu (biasanya enforcement decision — BT §41.7 business decision) |
| Current Case state | Case harus `decided` (decision final) sebelum appeal |
| Appeal state | `pending → approved/rejected` (atau `upheld/overturned` semantics) |
| Reviewer | Independen dari decision maker bila memungkinkan (BT §24) |
| Outcome | Appeal review menghasilkan **Decision baru** (append-only) — `reversed`/`upheld` (BT §9, §25) |
| Reversal | Hanya jika target state masih reversible (BT §25, §36) |
| Enforcement interaction | Reversal decision → reversal enforcement (baru) → target domain restore |

**Bagaimana appeal menghindari masalah:**

| Masalah | Mekanisme design |
|---|---|
| Overwrite original Decision | Appeal TIDAK update decision lama — append decision baru (BT §9) |
| Blind restoration | Appeal hanya menghasilkan consequence valid terhadap current state (BT §25) |
| Restore non-moderation deletion | Perlu provenance deletion (siapa menghapus) — content/comment saat ini tidak punya (`content.go:76-95` — `Delete()` tanpa actor). **Design requirement: `deleted_by`/`deletion_reason` provenance** (BT §15) |
| Restore terminal commerce state | For Sale sold → tidak bisa restore (guard ada); auction ended → tidak bisa (no-op) — **dipertahankan** |

---

## 16. Restoration Design Feasibility

Untuk setiap target, restoration = **new decision + new enforcement** (bukan inverse otomatis dari enforcement lama):

```text
Appeal
  ↓
Appeal review → Decision #2 (reversed/upheld)
  ↓
Reversal Enforcement (baru, decision_id = Decision #2)
  ↓
Target Domain restore command (validasi current state)
```

| Target | Reversal feasible? | Kondisi | Evidence |
|---|---|---|---|
| content | **YA (dengan provenance fix)** | hanya jika deletion dari moderation; bukan user-delete | `content_service.go:769-799` (restore tanpa provenance — **harus ditambah**) |
| comment | **YA (dengan provenance fix)** | sama | `comment_service.go:663-679` |
| for_sale | **YA** | hanya jika `withdrawn` (bukan sold) | `for_sale.go:186-206` guard sold sudah ada |
| auction | **TIDAK untuk v1** | bids/timing unrecoverable; no-op sudah didokumentasi | `moderation_event_handler.go:754-765` |
| user | **YA (conditional)** | banned tidak bisa di-restore via appeal (guard di handler); suspend → active | `moderation_event_handler.go:808-818` |

**Design implication:** restoration bukan "blind restore"; harus lewat decision baru + validasi current domain state. **Ini bukan inverse otomatis** — Business Truth §25, §36.

---

## 17. Audit History Design

**Persyaratan rekonstruksi (BT §28):** Report created, Case correlated, Decision made, Enforcement requested/attempted/succeeded/failed, Warning issued, Appeal submitted/decided, Reversal executed.

**Assessment current:**

| Sumber | Coverage | Reliability |
|---|---|---|
| `admin_audit_logs` | admin actions (LogSafe) | **TIDAK reliable** — best-effort, bisa gagal tanpa rollback (Audit 2 §13) |
| `audit_events` | order/umum (AuditService.Emit) | reliable append-only, **tapi moderation tidak memakainya** |
| Domain histories | tidak ada untuk governance | — |
| Event/outbox records | enforcement trigger hanya | outbox `succeeded` di-archive (OutboxArchivalWorker) — bukan history lengkap |

**Verdict:** **Diperlukan dedicated governance history** — atau memakai `audit_events` yang sudah ada (append-only, ActorType user/admin/system/worker — `audit_event.go:31-40`). Moderation mutations harus menulis ke mekanisme audit **dalam transaksi yang sama** dengan mutation (bukan LogSafe). **P1: LogSafe tidak cukup sebagai satu-satunya governance history (BT §28: "LogSafe best-effort tidak cukup").**

---

## 18. API Boundary

### User/Mobile

| Resource | Actions |
|---|---|
| Reports | `POST` create, `GET` own reports |
| Cases | `GET` own report's case (status visibility limited) |
| Decisions | `GET` decision on own case (outcome visible) |
| Appeals | `POST` create (terhadap Decision), `GET` own appeals |
| Warnings | `GET` own warnings |

**Tidak boleh:** user melihat enforcement internals, case queue admin, evidence penuh.

### Admin

| Resource | Actions |
|---|---|
| Cases | list, filter, detail (dengan evidence per target) |
| Decisions | create (decide), view history |
| Enforcements | inspect status (pending/succeeded/failed), retry (via governance) |
| Appeals | list, inspect, review |
| Warnings | issue (hanya dari Decision), revoke |

**Tidak ada endpoint lama yang dipertahankan untuk backward compatibility (BT §37).**

---

## 19. Admin Design

Satu moderation workspace (BT §29), dengan panel per target:

| Target | Evidence admin perlu | Valid actions | Forbidden direct mutation |
|---|---|---|---|
| content | preview caption, author, deletion state, visibility | dismiss, remove, hide, warning | `contents` langsung |
| comment | comment context, parent content, author | dismiss, remove, warning | `comments` langsung |
| for_sale | listing title, seller, status, shipping-relevant | dismiss, withdraw-for-moderation, warning | order/payment/ledger/seller proceeds |
| auction | auction state, bid state, winner/highest bidder | dismiss, stop-auction (via auction domain), warning | bid/settlement/refund langsung |
| profile | user status, moderation history | dismiss, suspend (via user domain), warning | `users` langsung, subscription, KYC, order |

**Kritikal (BT §30):** UI admin TIDAK boleh menampilkan "Enforced" ketika baru decision + outbox. UI harus membedakan:

```text
Decision finalized + Enforcement pending
Enforcement succeeded
Enforcement failed
```

Current UI menampilkan status case `enforced` sebagai badge final (`CaseDetailModal.tsx` — `moderationCaseStatusLabels` = enforced: 'Enforced'). **REPLACE.**

---

## 20. Mobile Design

Minimum user contract:

- report (subject type + id + reason_code + optional note);
- lihat own reports + status ringkas;
- lihat decision outcome pada own case;
- appeal decision yang eligible.

**Konflik terminologi current:**

| Current mobile | Canonical | Note |
|---|---|---|
| `ReportTargetType` + `backendValue` (`chat_message` supported) | content/comment/for_sale/auction/user only | `chat_message` harus dihapus (BT §31) |
| `ReportStatus {pending, underReview, approved, rejected, resolved}` | mapping ke case/decision state baru | `underReview` tidak ada di backend; `approved/rejected` decision semantics |
| `ReportAction {none, warning, contentRemoved, userSuspended, userBanned, dismissed}` | decision outcome/action code | legacy vocabulary |
| `ReviewReportRequest`/`ReviewReportRequestDto` (admin actions di mobile) | hapus — admin-only | dead code |

**Design:** mobile memakai API baru yang hanya expose user-facing resources. Tidak mempertahankan DTO lama (BT §37).

---

## 21. Database Constraint Audit

| Entity | PK | FK | Unique | Notes |
|---|---|---|---|---|
| reports | `id` | `reporter_id → users`, `case_id → cases` (nullable) | partial unique `(reporter_id, subject_type, subject_id)` WHERE active | polymorphic subject: **tidak bisa FK** — aplikasi validasi wajib |
| cases | `id` | — | **partial unique active case per subject** `(subject_type, subject_id) WHERE status IN (open, under_review)` | polymorphic subject |
| decisions | `id` | `case_id → cases`, `decided_by → users` | — | append-only |
| enforcements | `id` | `decision_id → decisions` | unique `(decision_id, action_type)` jika 1:1 | write-back dari worker |
| warnings | `id` | `decision_id → decisions`, `user_id → users`, `issued_by → users` | **unique `(decision_id, user_id)`** | BT §34 anti-duplicate |
| appeals | `id` | `decision_id → decisions`, `appealed_by → users`, `reviewed_by → users` | — | `report_id` naming harus hilang |
| outbox | `id` | — | `idempotency_key` UNIQUE | — |

**Penting:** polymorphic references (`subject_type+subject_id`) tidak dapat FK — validasi di application layer wajib (seperti `ResourceExists` current). **UNPROVEN** di current untuk integrity; design harus eksplisit.

---

## 22. Lifecycle Matrix

| Entity | States | Terminal states | Who changes it |
|---|---|---|---|
| Report | `active` → `superseded` (opsional) | `superseded` (atau tetap `active` seumur) | Report service (create); auto pada case baru (opsional) |
| Case | `open` → `under_review` → `decided` | `decided` (→ `closed` opsional) | Moderator (via Decision); admin workflow |
| Decision | append-only (no state) | — (immutable) | Moderator create |
| Enforcement | `pending → processing → succeeded / failed → retry → permanent_failure` | `succeeded`, `permanent_failure` | Worker (write-back); governance retry |
| Warning | `active → revoked / expired` | `revoked`, `expired` | Admin (via Decision) issue; admin revoke; waktu expire |
| Appeal | `pending → approved / rejected` | `approved`, `rejected` | User create; admin review |

**Tidak ada status `enforced` pada Case.** Tidak ada state invention.

---

## 23. Authority Matrix

| State | Canonical authority | Forbidden writers |
|---|---|---|
| Report lifecycle | Report service (governance module) | admin moderation, target domains |
| Case lifecycle | Case service (governance module) | target domains, mobile users (selain create report) |
| Decision | Decision service (governance module) | worker, target domains, appeal (kecuali create new decision via appeal review) |
| Enforcement | Enforcement service + worker write-back | admin UI (hanya read), target domains |
| Content visibility | Content Domain | moderation repository, admin direct DB |
| Comment visibility | Comment Domain | moderation repository |
| For Sale lifecycle | For Sale Domain | moderation (kecuali command withdraw), admin direct DB |
| Auction lifecycle | Auction Domain | moderation (hanya command stop via domain), worker langsung |
| User lifecycle | User Domain | moderation worker langsung (harus lewat user service) |
| Warning | Warning service (governance) | admin standalone (tanpa decision) |
| Appeal | Appeal service (governance) | target domains |

**Ini menjadi acceptance criteria cleanup** (BT §40 I9-I10).

---

## 24. Current → Canonical Gap Map

| Current implementation | Canonical requirement | Classification |
|---|---|---|
| `GovernanceCase` (`moderation_cases` super-entity) | Report/Case/Decision separation | **REPLACE** |
| Case status `approved/rejected/enforced` | Case workflow state terpisah + Decision rows + Enforcement lifecycle | **REPLACE** |
| `enforced` = decision+event proxy | Enforcement record dengan execution state | **REPLACE** |
| Outbox event = enforcement representation | Outbox = delivery; Enforcement = authority | **REPLACE** |
| Outbox retry broken (`MarkProcessing` only pending) | Reliable retry (failed → processing) | **REPLACE** |
| `reported_by/reason` di `moderation_cases` | `reports` entity | **REPLACE** |
| `decision_note/reviewed_by/reviewed_at` di case | `decisions` entity | **REPLACE** |
| Standalone `user_warnings` (tanpa decision FK) | Warning dengan `decision_id` provenance | **REPLACE** |
| `appeals.report_id` (menyimpan CaseID) | `appeals.decision_id` | **REPLACE** |
| `AuctionService.CancelForModeration` (bypass bid) | Auction-domain-owned stop command | **REPLACE** |
| Worker langsung set `users.account_status` | User domain service method | **REPLACE** |
| `DomainAction`/`DomainActionWorker`/`AppealReversalService` (PARKED) | Canonical executor/enforcement model | **DELETE** |
| `moderation.chat_message.*` target | out of v1 scope | **DELETE** |
| `fixed_price_sale` residue (admin/mobile/mapper) | `for_sale` | **DELETE** |
| `moderation_status_enum.removed` ghost | hapus dari enum baru | **DELETE** |
| `ContentService.SoftDeleteForModeration` (tanpa provenance) | + deletion provenance (`deleted_by`/reason) | **MODIFY** |
| `CommentService.SoftDeleteForModeration` | + provenance | **MODIFY** |
| `ForSaleService.Withdraw/RestoreFromModeration` | Pertahankan boundary (command moderation) | **KEEP** (boundary) |
| Target domain idempotency (content/comment/for_sale/user) | Pertahankan sebagai executor | **KEEP** |
| `admin_audit_logs` + `audit_events` infra | Governance mutations tulis ke audit reliable (dalam tx) | **MODIFY** (pakai) |
| `outbox` table + worker + archival | Pertahankan sebagai delivery infra | **KEEP** (fix retry) |
| Capability cluster `moderation.*` | Perluas (case.read, case.decide, enforcement.view, appeal.review, warning) | **MODIFY** |

---

## 25. Cleanup Impact (inventory only)

| Surface | Terdampak |
|---|---|
| Backend entity | `GovernanceCase`, `UserWarning`, `Appeal`, `DomainAction`, `DomainActionExecutionGroup` |
| Backend service | `ModerationService`, `WarningService`, `AppealService`, `AppealReversalService` |
| Backend repository | `moderation_repository_impl.go`, `appeal_repository_impl.go`, `warning_repository_impl.go`, `domain_action_repository_impl.go` |
| SQL | `moderation_cases`, `appeals`, `user_warnings` (+ `domain_actions` jika pernah ada) |
| Migration | `000001` (tabel/enum), `000047` (enum for_sale) — schema baru akan add/replace |
| API | `/moderation/cases`, `/moderation/my-cases`, `/admin/moderation/*`, `/appeals`, `/warnings` |
| Routes | `routes_core.go:762-775, 896-925, 1290-1316` |
| Worker | `ModerationEventHandler`, `DomainActionWorker`, `ModerationWSEvictionHandler` (user.suspended) |
| Events | `moderation.*.removed/suspended/hidden/restored`, `domain_action.*`, `appeal.reversed` |
| Outbox | retry fix + event payload baru |
| Admin API | moderation.ts, appeals, warnings hooks |
| Admin UI | `ModerationCasesPage`, `CaseDetailModal`, `AppealsPage`, `WarningsPage`, `IssueWarningModal`, `AppealDetailModal` |
| Mobile API | `report_api_datasource.dart` |
| Mobile repository | `report_repository_impl.dart`, appeal/warning repos |
| Mobile UI | `ReportSubmissionDialog`, `MyReportsScreen`, `ReportScreen`, `report_mapper.dart` |
| Mobile DTO | `report_dto.dart`, `report.dart` entity |
| Tests | moderation service/handler/repo tests, worker tests, mobile contract/widget tests, admin tests |
| Fixtures | test fixtures memakai `moderation_cases` |
| Docs/comments | komentar "000207", "STANDBY", "PARKED", `fixed_price_sale` |

---

## 26. Complexity Check

| Komponen enterprise | Required? | Alasan |
|---|---|---|
| Strike system | **NOT REQUIRED** | BT §21 LOCKED |
| Violation subsystem | **NOT REQUIRED** | BT §20 — outcome field pada Decision cukup |
| Policy engine | **NOT REQUIRED** | BT §22 — enum code backend cukup |
| Case assignment | **NOT REQUIRED** | BT §39 |
| Escalation engine | **NOT REQUIRED** | BT §39 |
| SLA engine | **NOT REQUIRED** | BT §39 |
| Case merge/split | **NOT REQUIRED** | BT §4/§32 — correlation sederhana |
| Generic workflow engine | **NOT REQUIRED** | state machine sederhana |
| Microservice | **NOT REQUIRED** | BT §39 |
| Event-sourcing | **NOT REQUIRED** | append-only decision cukup, bukan event-sourcing penuh |
| Distributed saga | **NOT REQUIRED** | outbox + worker cukup |
| Complex evidence snapshot system | **NOT REQUIRED** | BT §23 — hybrid reference sederhana |

---

## 27. Design Risks

| Risiko | Severity | Catatan |
|---|---|---|
| Auction bid state inconsistency jika command boundary tidak diubah | **P1** | Audit 2 bukti; harus Auction-Domain-owned |
| Outbox retry tetap broken → enforcement false-success persist | **P1** | Harus fix `MarkProcessing`/retry path |
| Restoration tanpa provenance → restore non-moderation deletion | **P1** | Content/comment perlu `deleted_by` provenance |
| Audit best-effort → governance history hilang | **P1** | BT §28; mutation + audit harus satu tx |
| Case reopening ambiguous | **BUSINESS DECISION REQUIRED** | BT §41.1 |
| Report sold/ended object | **BUSINESS DECISION REQUIRED** | BT §41.2 (cenderung izinkan) |
| Auction ber-bid boleh di-stop moderation | **BUSINESS DECISION REQUIRED** | BT §41.3 (rekomendasi YA) |
| Warning repeat policy | **BUSINESS DECISION REQUIRED** | BT §41.4 (rekomendasi no cap v1) |
| Reason taxonomy final | **BUSINESS DECISION REQUIRED** | BT §41.5 |
| Evidence retention | **BUSINESS DECISION REQUIRED** | BT §41.6 |
| Appeal eligibility scope | **BUSINESS DECISION REQUIRED** | BT §41.7 |
| User suspension interaction dengan seller subscription/listing | **BUSINESS DECISION REQUIRED** | §13 |
| Migration complexity (replace moderation tables + enum) | P2 | Labuda from zero — aman |
| Coordinator failure antara enforcement write-back & outbox | P2 | eventual consistency diterima |
| `chat_message` removal impact | P2 | mobile chat report entry harus dihapus |

---

## 28. Final Design Input

### A. Canonical Entity Set

```text
reports
cases
decisions
enforcements
warnings
appeals
governance_audit_history (atau reuse audit_events)
+ outbox (infrastructure, existing)
```

### B. Canonical Relationships

```text
Report      N → 1 Case        (report.case_id nullable FK)
Case        1 → N Decision    (decision.case_id FK)
Decision    1 → N Enforcement (enforcement.decision_id FK)
Decision    1 → 0..1 Warning  (warning.decision_id FK UNIQUE per decision)
Decision    1 → 0..N Appeal   (appeal.decision_id FK)
Enforcement 1 → 1 Outbox event (enforcement_id dalam payload)
Case        1 subject (polymorphic subject_type + subject_id)
```

### C. Canonical Lifecycle

```text
Case:        open → under_review → decided
Decision:    append-only (immutable rows)
Enforcement: pending → processing → succeeded | failed → retry → ... | permanent_failure
Warning:     active → revoked | expired
Appeal:      pending → approved | rejected
Report:      active (lifecycle minimal)
```

### D. Canonical Authority

```text
Report lifecycle   → Report service
Case lifecycle     → Case service
Decision           → Decision service (moderator create)
Enforcement        → Enforcement service + worker write-back
Content visibility → Content Domain
Comment visibility → Comment Domain
For Sale lifecycle → For Sale Domain
Auction lifecycle  → Auction Domain
User lifecycle     → User Domain
Warning            → Warning service (provenance decision)
Appeal             → Appeal service
```

### E. Canonical Transaction Boundaries

```text
TX1 (report create): insert report + attach/find active case      — satu tx
TX2 (decision+enforcement+outbox): insert decision + enforcement  — satu tx
     (pending) + outbox event
TX3 (worker execute): target domain mutation                       — target domain tx
TX4 (worker write-back): update enforcement (succeeded/failed)    — governance tx (terpisah)
TX5 (audit): mutation + governance audit insert                   — SAMA TX dengan mutation
```

### F. Canonical Execution Boundary

```text
Decision (violation_confirmed + action)
  → Enforcement (pending) dibuat atomik
  → Outbox event (enforcement_id, decision_id, target, action)
  → Worker (ModerationEventHandler-style, per-target executor)
  → Target Domain command (ContentService / CommentService / ForSaleService / AuctionDomain command / UserService)
  → Worker write-back: Enforcement succeeded/failed
  → UI admin menampilkan enforcement state sebenarnya (bukan "enforced")
```

### G. Canonical API Resources

```text
User:   POST /reports | GET /reports/mine | GET /cases/:id (own) | GET /decisions/:id (own) | POST /appeals | GET /appeals/mine | GET /warnings/mine
Admin:  GET /admin/cases | GET /admin/cases/:id | POST /admin/cases/:id/decisions | GET /admin/cases/:id/decisions | GET /admin/enforcements/:id | POST /admin/enforcements/:id/retry | GET /admin/appeals | PUT /admin/appeals/:id/review | GET /admin/warnings
```

### H. Canonical Admin Capabilities

```text
moderation.case.read
moderation.case.decide
moderation.enforcement.view
moderation.enforcement.retry (opsional)
moderation.appeal.read
moderation.appeal.review
moderation.warning.manage (via decision)
moderation.evidence.read
```

### I. Canonical Mobile Capabilities

```text
report (create)
lihat own reports
lihat decision outcome
appeal eligible decision
lihat own warnings
```

### J. Cleanup Scope

```text
moderation_cases (table) + GovernanceCase (entity/service/repo/handler/routes)
enforced status semantics
appeals.report_id
user_warnings standalone + POST /admin/warnings tanpa decision
moderation.chat_message.*
DomainAction + DomainActionWorker + AppealReversalService (parked)
fixed_price_sale residue (admin/mobile/mapper)
moderation_status_enum.removed
Mobile legacy ReportStatus/ReportAction vocabulary
Admin UI "Enforced" badge
Komentar STANDBY/PARKED/000207/000100
```

### K. Open Business Decisions

```text
1. Case reopening policy (default: new case after terminal)
2. Report sold/ended object (default: allow)
3. Auction ber-bid moderation stop (default: YES, Auction Domain handles bidder consequence)
4. Warning repeat/cap policy (default: no cap v1)
5. Reason taxonomy final list
6. Evidence retention
7. Appeal eligibility (semua decision atau hanya enforcement decision)
8. User suspension × seller subscription/listing/order interaction
```

---

```text
AUDIT STATUS: PROVEN

CANONICAL ENTITY SET:
reports, cases, decisions, enforcements, warnings, appeals, governance audit history (+ outbox infra)

CANONICAL RELATIONSHIPS:
Report N→1 Case; Case 1→N Decision; Decision 1→N Enforcement; Decision 1→0..1 Warning;
Decision 1→0..N Appeal; Enforcement 1→1 outbox event; polymorphic subject (type+id)

CANONICAL AUTHORITY:
Governance = Report/Case/Decision/Enforcement/Warning/Appeal.
Target Domain = content/comment/for_sale/auction/user state (mutasi via domain command).
Commerce = order/payment/ledger (moderation tidak pernah menyentuh).

CANONICAL TRANSACTION BOUNDARIES:
TX report-create; TX decision+enforcement+outbox (atomik); TX target mutation (target domain);
TX enforcement write-back (terpisah); audit inline dengan mutation.

CANONICAL EXECUTION BOUNDARY:
Decision → Enforcement(pending) → outbox event(enforcement_id) → worker → target domain command
→ write-back succeeded/failed → admin UI menampilkan execution state nyata.

CURRENT DESIGN THAT MUST BE REPLACED:
- GovernanceCase super-entity (moderation_cases)
- enforced-as-case-status + outbox-event-as-enforcement
- Standalone warning + appeals.report_id (CaseID in disguise)
- AuctionService.CancelForModeration (bid-state leak, P1)
- Worker direct users.account_status mutation
- Outbox retry broken (P1)
- DomainAction/AppealReversalService zombie
- chat_message target + fixed_price_sale residue

CURRENT DESIGN THAT CAN BE RETAINED:
- Target domain idempotent enforcement methods (content/comment/for_sale/user) sebagai executor
- For Sale Withdraw boundary (listing + shipping quotes only; no commerce mutation)
- Outbox table + worker + archival sebagai delivery infrastructure (setelah retry fix)
- Capability RBAC pattern (moderation.*)
- audit_events append-only infra (governance harus memakainya)
- Partial unique index pattern (untuk active case per subject)

CRITICAL P0/P1:
- P1: Auction moderation cancel meninggalkan bid/winner state inconsistent
- P1: Outbox retry broken — enforcement failure permanen tidak terlihat
- P1: Restoration tanpa provenance dapat menghidupkan kembali non-moderation deletion
- P1: Governance audit best-effort (LogSafe) — history bisa hilang
- P1: False-success enforcement state di admin UI

BUSINESS DECISIONS STILL REQUIRED:
- Case reopening
- Report sold/ended object
- Auction ber-bid moderation stop (rekomendasi YA)
- Warning repeat/cap policy
- Reason taxonomy final
- Evidence retention
- Appeal eligibility scope
- User suspension × subscription/listing interaction

OVERENGINEERING REJECTED:
Strike, Violation subsystem, policy engine, case assignment, escalation, SLA, merge/split,
generic workflow engine, microservice, event-sourcing, distributed saga, complex evidence snapshot
— semua NOT REQUIRED per Business Truth §39.

RECOMMENDED NEXT STEP:
Owner + ChatGPT approval gate atas Business Truth v1 dan design input di atas, diikuti
Implementation Plan bertahap (schema foundation → report → case → decision → enforcement →
warning → appeal → admin → mobile → target integration → cleanup → regression) sesuai BT §44.
Belum ada implementasi pada audit ini.
```
