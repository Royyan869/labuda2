# LABUDA — AUDIT 1: MODERATION / REPORT / CASE — FACTUAL CURRENT-STATE AUDIT

- **Tanggal audit:** 2026-08-30
- **Mode:** AUDIT ONLY — tidak ada implementasi, tidak ada perubahan kode/schema/test/admin/mobile
- **Baseline:** current filesystem (bukan git history)
- **Satu-satunya artefak baru:** laporan ini

---

## 1. Executive Summary

Implementasi moderation saat ini berpusat pada satu entitas: **`GovernanceCase`** (tabel `moderation_cases`), yang dalam satu baris menyimpan:

- data **Report** (`reported_by`, `reason`, `created_at`);
- data **Case** (`resource_type`, `resource_id`, `status`);
- data **Decision** (`reviewed_by`, `decision_note`, `reviewed_at`).

Tidak ada entitas `Report` terpisah di backend. Satu report = satu case (baris `moderation_cases`), dibatasi oleh unique index `idx_moderation_cases_one_report_per_user (reported_by, resource_type, resource_id)`. Canonical spec (v1) mensyaratkan **Report ≠ Case** dan **satu Case menampung banyak Report** — implementasi saat ini **tidak memenuhi** kedua invariant ini (I1, I2).

Keputusan (Decision) tidak disimpan historical; decision di-overwrite pada status case (`pending → approved/rejected/enforced`). Enforcement tidak memiliki status/entity sendiri; enforcement direpresentasikan sebagai **outbox event** (`moderation.{type}.removed` / `.suspended` / `.hidden`) yang diproses `ModerationEventHandler`. Tidak ada tracking enforcement status (retry/failure/target-ack) pada runtime — **Decision vs Enforcement tidak dapat dibedakan setelah event di-emit** (I4, I5, I6 tidak terpenuhi secara struktural; hanya idempotensi handler yang mencegah duplikasi).

Warning berdiri sendiri (`user_warnings`), **tidak terhubung** ke case/decision (tidak ada FK ke `moderation_cases`), dan **dapat dibuat tanpa konteks governance** melalui `POST /api/v1/admin/warnings` — melanggar canonical "Decision → Warning" provenance (canonical §9).

Appeal menunjuk **CaseID** (entity) namun disimpan di kolom **`appeals.report_id`** (DB) — mapping SQL terbukti menulis `appeal.CaseID` ke `report_id`. Ini DB/code naming mismatch, bukan sekadar istilah. Appeal approval melakukan auto-restore hanya untuk content/comment via `moderation.{type}.restored`; for_sale/auction/user record-only.

Admin (React) memiliki queue case, detail modal, appeal queue, warning issue/revoke. **Namun UI masih memakai vocabulary `fixed_price_sale`** yang sudah tidak valid di backend (`for_sale`) — filter dan label resource type akan gagal/menampilkan undefined untuk for_sale. Admin `Appeal` type memakai `report_id` (backend mengirim `case_id`).

Terdapat **dua komponen enforcement tumpang tindih**: jalur aktif `ModerationEventHandler` (outbox events) dan jalur **PARKED/dead** `DomainActionWorker` + `DomainAction` + `AppealReversalService` (tidak di-instantiate di serverboot, tidak ada migration `domain_actions`). Kode PARKED masih ada dan dapat dihidupkan kembali (risiko resurrection).

**AUDIT STATUS: PROVEN** (semua klaim utama di bawah didukung evidence langsung).

---

## 2. Current Entity Map

| Konsep canonical | Implementasi saat ini | Lokasi |
|---|---|---|
| Report | **TIDAK ADA entity terpisah.** Report = baris `moderation_cases` (satu report = satu case) | `backend/internal/governance/moderation/entity/governance_case.go` |
| Case | `GovernanceCase` (`moderation_cases`) — menyimpan report fields + status + decision fields | `governance_case.go:30-41` |
| Decision | **TIDAK ADA entity Decision.** Decision = field `status` + `decision_note` + `reviewed_by` pada `GovernanceCase` | `governance_case.go:43-60` |
| Enforcement | **TIDAK ADA entity Enforcement aktif.** Outbox event `moderation.{type}.removed`/`.suspended`/`.hidden` | `moderation_service.go:199-211`, `getRemovedEventType():379-388` |
| DomainAction (enforcement unit) | **PARKED** — entity ada, repository ada, worker ada, tapi tidak di-instantiate; tidak ada migration tabel | `worker/outbox_event_registry.go:198-203`; `worker/domain_action_worker.go` |
| Warning | `UserWarning` (`user_warnings`) — berdiri sendiri | `entity/warning.go` |
| Appeal | `Appeal` (`appeals`) — menunjuk CaseID (kolom `report_id`) | `entity/appeal.go:16` |
| Audit history | `admin_audit_logs` (hanya admin actions), `audit_events` (umum) | `migrations/000001:444-452, 500-509` |

### Resource types yang didukung (enum `moderation_resource_enum`)

`content`, `comment`, `for_sale`, `auction`, `user`, `chat_message` — `entity/moderation_resource_type.go:10-38`; schema `migrations/000001:175-182` (sebelum migrasi 000047 berisi `fixed_price_sale`).

- `chat_message` adalah **target bisnis di luar canonical v1** (canonical §12: `chat_message` tidak menjadi target baru hanya karena implementation lama pernah mendukungnya). Status: **potentially valid but undecided** — perlu keputusan Owner.
- `auction` enforcement AKTIF (lihat §13), bertentangan dengan komentar serverboot yang menyatakan PARKED.

---

## 3. Current API/Route Map

Semua route terdaftar di `backend/cmd/core_server/routes_core.go`.

### User routes (auth: `RequireAuth`; create-case juga `RequireActiveAccount`)

| Method & Path | Handler | Permission |
|---|---|---|
| `POST /api/v1/moderation/cases` | `ModerationHandler.CreateCase` | `RequireActiveAccount` (routes_core.go:1294) |
| `GET /api/v1/moderation/my-cases` | `ModerationHandler.GetMyCases` | auth (1296) |
| `GET /api/v1/moderation/cases/:id` | `ModerationHandler.GetMyCase` | auth + ownership reporter (1298) |
| `POST /api/v1/appeals` | `AppealHandler.CreateAppeal` | `RequireAuth` only (1307) — suspensi boleh appeal |
| `GET /api/v1/appeals/:id` | `AppealHandler.GetAppeal` | auth + ownership (1308) |
| `GET /api/v1/appeals/me` | `AppealHandler.ListMyAppeals` | auth (1309) |
| `GET /api/v1/warnings/:id` | `WarningHandler.GetWarning` | auth + ownership (1313) |
| `GET /api/v1/warnings` | `WarningHandler.ListWarnings` | auth (1314) |
| `GET /api/v1/users/:id/warnings/active` | `WarningHandler.GetActiveWarnings` | auth + self (1315) |
| `GET /api/v1/users/:id/warnings` | `WarningHandler.GetUserWarnings` | auth + self (1316) |

### Admin routes (`/api/v1/admin`)

| Method & Path | Handler | Capability (route) | Capability (handler, defense-in-depth) |
|---|---|---|---|
| `GET /admin/moderation/cases` | `ListCases` | `moderation.case.read` | — |
| `GET /admin/moderation/cases/:id` | `GetCase` | `moderation.case.read` | — |
| `GET /admin/moderation/cases/:id/evidence` | `GetCaseEvidence` | `moderation.evidence.read` | `capability.HasCapability` (moderation_handler.go:504) |
| `POST /admin/moderation/cases/:id/action` | `ApplyAction` | `moderation.case.resolve` | `capability.HasCapability` (612) |
| `GET /admin/appeals` | `AdminListAppeals` | `moderation.appeal.read` | — |
| `GET /admin/appeals/pending` | `AdminListPendingAppeals` | `moderation.appeal.read` | — |
| `GET /admin/appeals/:id` | `AdminGetAppeal` | `moderation.appeal.read` | — |
| `PUT /admin/appeals/:id/review` | `AdminReviewAppeal` | `moderation.appeal.review` | `capability.HasCapability` (474) |
| `GET /admin/warnings` | `AdminListWarnings` | `moderation.case.read` | — |
| `POST /admin/warnings` | `AdminIssueWarning` | `moderation.case.resolve` | — |
| `DELETE /admin/warnings/:id/revoke` | `AdminRevokeWarning` | `moderation.case.resolve` | — |

Catatan: **tidak ada capability khusus `moderation.warning.issue`** — issue/revoke warning memakai `moderation.case.resolve` yang sama dengan resolve case. Capability `moderation.content.view` dan `moderation.content.remove` didefinisikan (`capability.go:129,132`) tapi **tidak dipakai** pada route mana pun yang ditemukan (candidate authority mati/terdaftar tapi tidak aktif).

---

## 4. Current Backend Flow

### Report intake (CreateCase)

```text
POST /moderation/cases
→ RequireActiveAccount
→ ModerationHandler.CreateCase
→ ModerationService.CreateCase (satu transaksi)
   ├─ validasi resourceType (6 tipe)
   ├─ ResourceExists (per-tipe tabel; content/comment/user/chat_message dengan deleted_at IS NULL;
   │    for_sale/auction tanpa guard deleted_at — reporting withdrawn/sold/terminal sengaja diizinkan)
   ├─ chat_message: ValidateChatMessageReporter (harus participant, room bukan support)
   ├─ HasUserReportedResource (duplicate check per user per resource)
   └─ NewGovernanceCase(...) → repo.Create (INSERT moderation_cases)
```

- Self-report (user melaporkan objek sendiri): **TIDAK dicegah** di service/handler. Tidak ada pengecekan `resource.author_id != reporterID`. `BUSINESS DECISION REQUIRED`.
- Reason: free-text, controlled di UI mobile (enum diserialisasi `"spam: description"`), tapi backend menerima arbitrary string 1-500 (`binding:"required,min=1,max=500"`).
- Unique constraint: `idx_moderation_cases_one_report_per_user (reported_by, resource_type, resource_id)` — `migrations/000001:2113`. **Komentar service menyebut "migration 000207" yang TIDAK ADA** (migration tertinggi = 000054; index ada di 000001) — `moderation_service.go:71`.

### Admin decision (ApplyAction / ReviewCase)

```text
POST /admin/moderation/cases/:id/action
→ RequireCapability(moderation.case.resolve) (route) + capability check (handler)
→ ModerationService.ReviewCase (satu transaksi)
   ├─ GetForUpdate (FOR UPDATE lock)
   ├─ entity transition: approve/reject/enforce (hanya pending → terminal)
   ├─ enforce: jika case.status = enforced → InsertEvent outbox
   │    moderation.{type}.removed | moderation.user.suspended | moderation.chat_message.hidden
   └─ repo.Update
```

- Terminal states: `approved`, `rejected`, `enforced`. Tidak ada re-review.
- `enforce` WAJIB `notes` non-empty (`ErrEnforceRequiresNote`, governance_case.go:87-91,153-157).
- Audit: `adminAuditLogger.LogSafe("moderation_action_applied", ...)`.

### Enforcement (outbox worker)

```text
outbox row (moderation.{type}.removed/suspended/hidden/restored)
→ ModerationEventHandler.Handle
   ├─ content → ContentService.SoftDeleteForModeration (idempotent)
   ├─ comment → CommentService.SoftDeleteForModeration (idempotent)
   ├─ for_sale → ForSaleService.Withdraw (idempotent pada terminal state)
   ├─ auction → AuctionService.CancelForModeration (idempotent pada terminal state) ← AKTIF
   ├─ user → userRepo suspend account_status='suspended' (idempotent)
   └─ chat_message → chatService.SoftHideForModeration
notification fanout: notifEventHandler (notification_worker.go:687-688)
WS eviction: moderation_ws_eviction_handler.go (moderation.user.suspended)
```

---

## 5. Current Mobile Flow

Domain: `apps/mobile/lib/domains/system/report/`.

### Trigger points (report UI)

| Target | File | Entry |
|---|---|---|
| content | `content_detail_screen.dart:146` | `ReportSubmissionDialog.show` |
| comment | `comment_card.dart:367` | `ReportSubmissionDialog.show` |
| for_sale | `for_sale_detail_screen.dart:122` | `ReportSubmissionDialog.show` |
| auction | `auction_detail_screen.dart:413` | `ReportSubmissionDialog.show` |
| user (profile) | `profile_screen.dart:1340` | `ReportScreen` (push) |
| chat message | `chat_detail_screen.dart:1443` | `ReportScreen` (push) |
| user (chat) | `chat_detail_screen.dart:1539` | `ReportScreen` (push) |

### Flow

```text
ReportSubmissionDialog
→ CreateReportRequest (targetType, targetId, reason, description)
→ ReportActionsNotifier.submitReport
→ ReportRepositoryImpl.createReport
→ ReportMapper.toCreateRequestDto
   → CreateCaseRequestDto {entity_type, entity_id, reason}
→ ReportApiDatasourceImpl.createCase → POST /moderation/cases
→ ModerationCaseDto.fromCreateJson (case_id key) → Report entity
```

- My Reports: `MyReportsScreen` → `ReportListNotifier.loadReports` → `GET /moderation/my-cases`.
- Pre-flight gate: email harus verified (`report_submission_dialog.dart:276-283`).
- Evidence upload: `ReportRepositoryImpl.uploadEvidence` → `_S3ImageUploader` (`data/providers.dart:97`) — masih ada implementasi via S3Service, **tapi upload evidence tidak dipakai di submit flow** (CreateCaseRequestDto hanya membawa text reason). `evidenceUrls` selalu `[]` di mapper.
- Status mapping: backend `enforced` → `ReportStatus.resolved` (mapper `_mapStatusString`); `approved`/`rejected` langsung. **Entity mobile masih punya status `underReview` yang tidak pernah dikirim backend.**

### DTO/entity divergence (mobile)

- Entity `Report` (domain) memiliki field `reporterName`, `targetTitle`, `evidenceUrls`, `action`, `moderatorId`, `resolvedAt` — **sebagian besar tidak di-populate** dari DTO (hanya diisi `''`/`const []`/`null`).
- `ReportStatus.underReview` dan `ReportAction` enum tidak memiliki padanan di backend.
- Mapper masih punya case `'fixed_price_sale'` → `ReportTargetType.forSale` (`report_mapper.dart:85-87`) — string yang sudah tidak valid di backend enum.
- `hasUserReported` mobile melakukan fetch `getMyCases()` lalu `any(resourceId == targetId)` — **tidak membedakan target type**, dan abaikan `userId` param (backend pakai token).
- `getReportsByUser` memakai `getMyCases(page: (limit/20).ceil())` — parameter `userId` diabaikan.

---

## 6. Current Admin Flow

`apps/admin/src/` — React + Vite + Tailwind.

| Halaman | File | Capability gate |
|---|---|---|
| Moderation Cases | `pages/ModerationCasesPage.tsx` | `moderation.case.read` |
| Case Detail Modal | `components/moderation/CaseDetailModal.tsx` | — (di dalam halaman) |
| Appeals | `pages/AppealsPage.tsx` | `moderation.appeal.read` |
| Appeal Detail Modal | `components/moderation/AppealDetailModal.tsx` | — |
| Warnings | `pages/WarningsPage.tsx` | `moderation.case.read` (routes_core App.tsx:102) |
| Issue Warning Modal | `components/moderation/IssueWarningModal.tsx` | — |

### Capability yang dapat dilakukan admin

- **Case queue**: list + filter status + filter resource_type + detail + resource_preview + approve/reject/enforce + notes. CaseDetailModal melakukan data-freshness check (refetch sebelum action).
- **Evidence**: hanya untuk chat_message; button "Lihat bukti asli" muncul jika `evidence_available` dan admin punya `moderation.evidence.read`.
- **Appeal**: queue + detail + original_case context + review approve/reject.
- **Warning**: list, issue, revoke.
- **Reported By**: hanya UUID ditampilkan (tanpa username).
- **Tidak ada** kemampuan melihat report history (multiple reports per subject) — tidak ada UI grouping; setiap case adalah satu report.
- **Tidak ada** enforcement result view (status enforcement tidak ada di backend).

### Admin vocabulary mismatch (CONFLICT)

- `types/moderation.ts:11` — `ResourceType = 'content' | 'comment' | 'user' | 'chat_message' | 'fixed_price_sale' | 'auction'`. Backend enum (setelah 000047) = `for_sale`. Filter `resource_type=fixed_price_sale` akan ditolak backend (`IsValid()` false → 400).
- `CaseDetailModal.tsx:175` — `isMarketplaceResource = resource_type === 'fixed_price_sale' || 'auction'` → tidak akan pernah true untuk `for_sale`.
- `Appeal` type admin memakai `report_id: string` (`types/moderation.ts:125`), backend mengirim `case_id` (`appeal_handler.go:141`). **Kolom "Report ID" di tabel appeal akan menampilkan `undefined`** untuk data nyata.
- `ResourceType` admin response untuk appeal table: `appeal.report_id.slice(0,8)` akan crash/undefined bila field tidak ada.

---

## 7. Current Database/Schema Map

### Tabel `moderation_cases` (`migrations/000001:963-976`)

```sql
id uuid PK
resource_type moderation_resource_enum NOT NULL
resource_id uuid NOT NULL
status moderation_status_enum NOT NULL DEFAULT 'pending'
reported_by uuid NOT NULL            -- TIDAK ada FK ke users
reviewed_by uuid                     -- TIDAK ada FK ke users
reason text NOT NULL
decision_note text
created_at, reviewed_at, updated_at
deleted_at timestamptz               -- TIDAK pernah dibaca/ditulis repository
```

Index:
- `idx_moderation_cases_one_report_per_user` UNIQUE `(reported_by, resource_type, resource_id)` — :2113
- `idx_moderation_cases_reported_by`, `idx_moderation_cases_resource` — :2114-2115
- `idx_moderation_pending` partial `(status, created_at) WHERE status='pending'` — :2116
- `idx_moderation_reporter`, `idx_moderation_resource`, `idx_moderation_reviewer` — :2117-2119

### Enum `moderation_status_enum` (`migrations/000001:184-190`)

```sql
'pending', 'approved', 'rejected', 'removed', 'enforced'
```

**`removed` ada di DB enum tapi TIDAK ADA di code entity** (`GovernanceCaseStatus` hanya pending/approved/rejected/enforced). `removed` adalah semantic ghost — dapat di-insert via SQL tapi tidak pernah ditulis code. **Semantic mismatch DB vs code: CONFLICT.**

### Enum `moderation_resource_enum`

- 000001: `content, comment, fixed_price_sale, auction, user, chat_message`
- 000047: converged → `content, comment, for_sale, auction, user, chat_message`

### Tabel `appeals` (`migrations/000001:454-466`)

```sql
id uuid PK
report_id uuid NOT NULL            -- MENYIMPAN CaseID (bukti di appeal_repository_impl)
appealed_by uuid NOT NULL
message text NOT NULL
status text NOT NULL DEFAULT 'pending'   -- text, bukan enum
reviewed_by uuid, admin_response text, reviewed_at, created_at, updated_at
deleted_at timestamptz            -- soft-delete column TIDAK digunakan repository
```

- Index: hanya `idx_appeals_report_id` (:2006). **Tidak ada unique constraint** untuk satu pending appeal per case → duplicate guard hanya lewat CTE repository (`CreateWithPendingCheck`), bukan DB constraint.
- **TIDAK ada FK** dari `appeals.report_id` ke `moderation_cases.id` — referensial integrity tidak dijaga DB.

### Tabel `user_warnings` (`migrations/000001:1772-1783`)

```sql
id uuid PK
user_id uuid NOT NULL  -- FK → users
issued_by uuid NOT NULL -- FK → users
level text NOT NULL (info|warning|severe — CHECK :2533)
reason text NOT NULL
is_active bool DEFAULT true
revoked_at, revoked_by (FK), created_at, expires_at
```

- Index: `idx_user_warnings_user_id_active` partial `WHERE is_active=true`, `idx_user_warnings_user_id_created`.
- **TIDAK ada FK ke moderation_cases/decision** — warning tidak terhubung governance.

### Tabel `admin_audit_logs` (`migrations/000001:444-452`)

`actor_id, action_type, target_type, target_id, metadata jsonb, created_at`. Tidak ada FK. Ditulis oleh `AdminAuditLoggerDB.LogSafe/LogTx` (`audit/admin_audit_logger.go`).

### Migration vocabulary check

- `000047` converged `fixed_price_sale` → `for_sale` termasuk pada `moderation_cases.resource_type` (:96-103).
- `000054` menghapus `chat_commerce_references` (bukan moderation).
- **Komentar `moderation_service.go:71` merujuk "migration 000207" yang tidak ada** — referensi migration palsu.
- `migration_000047_schema_state_proof_test.go:98-99` membuktikan `fixed_price_sale` tidak lagi ada di enum.
- Tidak ada migration khusus moderation lain; seluruh schema moderation lahir di `000001`.

---

## 8. Report vs Case Analysis

| Pertanyaan | Jawaban faktual | Status |
|---|---|---|
| Apakah Report entity terpisah? | **Tidak.** Tidak ada tabel `reports`; tidak ada struct `Report` moderation (hanya `Report` di finance/verifier yang tak terkait). Report = baris `moderation_cases` | PROVEN |
| Apakah report langsung menjadi case? | **Ya.** `CreateCase` menamai input "REPORT" dan output "CASE" (satu fungsi) | PROVEN |
| Satu report = satu case? | **Ya.** 1 report → 1 row `moderation_cases` | PROVEN |
| Multiple report pada subject sama bisa masuk case sama? | **Tidak.** Setiap report baru = case baru. Tidak ada grouping | PROVEN |
| Unique constraint? | `UNIQUE (reported_by, resource_type, resource_id)` | PROVEN |
| Duplicate rule? | Satu user tidak bisa report resource yang sama dua kali (app + DB unique). **User lain BISA report resource sama → case terpisah** | PROVEN |
| Concurrent duplicate submission? | Application check + DB unique index; handler menangkap 23505 → 409 | PROVEN |
| Grouping oleh database/service/tidak ada? | **Tidak ada grouping** | PROVEN |
| Report history dipertahankan? | Ya, sebagai row case. Tapi **tidak ada agregasi per subject** | PROVEN |
| Reporter identity disimpan? | `reported_by` (UUID), tanpa FK | PROVEN |
| Reason controlled atau arbitrary? | Backend: arbitrary text 1-500. Mobile: enum → teks. **Tidak ada controlled vocabulary di backend** | PROVEN |
| Self-report dicegah? | **Tidak.** Tidak ada pengecekan kepemilikan | PROVEN → **BUSINESS DECISION REQUIRED** |
| Report subject tidak tersedia? | `ResourceExists`: content/comment/user/chat_message memakai `deleted_at IS NULL` → deleted tak bisa direport. for_sale/auction: **tanpa guard** — withdrawn/sold/cancelled tetap bisa direport (sengaja, komentar :345-346) | PROVEN |
| Subject berubah state setelah report? | Case tidak menyimpan snapshot; enforcement handler idempotent pada terminal state. `approved` case tidak menyimpan state subject | PROVEN |

**Kesimpulan §8:** implementasi saat ini secara fundamental mencampur Report + Case + Decision dalam satu baris. Ini melanggar invariant canonical I1 (Report ≠ Case) dan I2 (multiple reports → satu case). Canonical spec §17 secara eksplisit menyebut `GovernanceCase` yang mencampur Report/Case/Decision/Enforcement "boleh dan seharusnya dibongkar total".

---

## 9. Decision vs Enforcement Analysis

### Decision

- Decision tidak punya entity/table. Decision = transisi status case `pending → approved/rejected/enforced` + `decision_note` + `reviewed_by` + `reviewed_at` (overwrite, **bukan append**).
- **Decision history TIDAK ada.** Re-review dilarang (`ErrAlreadyReviewed`); satu case hanya punya satu keputusan final. Tidak ada riwayat keputusan yang bisa direkonstruksi (canonical I3, I8 dilanggar).

### Enforcement

- Enforcement tidak punya entity/table runtime. Enforcement = **outbox event** yang di-insert dalam transaksi yang sama dengan update case.
- Event: `moderation.content.removed`, `moderation.comment.removed`, `moderation.for_sale.removed`, `moderation.auction.removed`, `moderation.user.suspended`, `moderation.chat_message.hidden`.
- Tidak ada field enforcement status pada case (`enforced` = "event sudah di-emit", bukan "target sudah berubah").
- Tidak ada retry tracking, tidak ada target ack, tidak ada enforcement result yang tersimpan.

### Jawaban kritikal

> Apakah current system dapat membedakan "admin memutuskan violation" dari "consequence berhasil diterapkan"?

**TIDAK.** `status='enforced'` berarti "decision enforce dibuat dan event di-emit dalam satu transaksi". Tidak ada mekanisme yang mencatat apakah worker berhasil melakukan mutation. Jika enforcement gagal (worker retry habis), case tetap `enforced` tanpa tanda failure. **I4, I5, I6 tidak terpenuhi secara struktural.**

Mitigasi parsial (bukan bukti invariant):
- Worker idempotent: content/comment/user/for_sale/auction semuanya treat terminal state sebagai sukses.
- `err` dari worker memicu retry outbox (`moderation_event_handler.go` mengembalikan error untuk retry).

Status: **UNPROVEN** untuk "enforcement berhasil"; **PROVEN** untuk "tidak ada pembedaan decision vs enforcement di persistence".

### DomainAction (candidate enforcement authority)

- `DomainAction` entity (`entity/domain_action.go`), `DomainActionRepository` (`infrastructure/repository/domain_action_repository_impl.go` — INSERT ke `domain_actions`), `DomainActionWorker` (`worker/domain_action_worker.go`), `AppealReversalService` (`application/appeal_reversal_service.go`).
- Semua **PARKED**: `outbox_event_registry.go:198-203` menyatakan worker tidak pernah di-instantiate, tidak ada migration `domain_actions`. `appeal_reversal_service.go:1` menyatakan PARKED.
- Event stubs `for_sale.visibility.apply`, `auction.pause.apply`, `domain_action.executed`, `appeal.reversed` terdaftar sebagai `NoHandlerAuditOnly`.
- Klasifikasi: **dead/zombie code (residue)** — dapat dihidupkan kembali kapan saja (risiko competing authority). **OBSOLETE** hanya jika Owner memutuskan; audit ini tidak menghapus apa pun.

---

## 10. Warning Analysis

| Aspek | Fakta | Status |
|---|---|---|
| Entity | `UserWarning` (`entity/warning.go`) | PROVEN |
| Table | `user_warnings` (migration 000001:1772-1783) | PROVEN |
| Writer | `WarningService.IssueWarning` — hanya via `POST /api/v1/admin/warnings` | PROVEN |
| Reader | `WarningHandler` user + admin list; mobile `getWarnings/getActiveWarnings` | PROVEN |
| Lifecycle | `active → revoked/expired`; `GetStatus()` menghitung expired dari `expires_at` | PROVEN |
| Expiry | `expires_at` optional; status dihitung on-read (`warning.go:122-133`) | PROVEN |
| Revoke | `RevokeWarning` dengan `GetForUpdate` lock + `Revoke()`; hanya active → revoked | PROVEN |
| Provenance | **TIDAK ADA** relasi ke case/decision. Field `issued_by` hanya admin. Warning tidak membawa `case_id` | PROVEN |
| Dapat dibuat tanpa governance context? | **YA** — `POST /admin/warnings` menerima `user_id, level, reason, expires_at` tanpa case reference | PROVEN → **melanggar canonical §9 "Decision → Warning"** |
| Duplicate warning behavior | **Tidak ada guard** — warning ganda untuk user yang sama diperbolehkan; tidak ada cap/limit | PROVEN (service tidak punya unique constraint; `warning_repository_impl.go:39` INSERT polos) |
| Admin permission | `POST/revoke` → `moderation.case.resolve`; list → `moderation.case.read` | PROVEN |
| Mobile visibility | `GET /warnings`, `GET /users/:id/warnings` (self-only) | PROVEN |
| Violation/Strike | **Tidak ada** entity `Violation`/`Strike` di codebase | PROVEN (tidak ditemukan) |

Catatan: `WarningService.IssueWarning` punya komentar "OWNER DECISION REQUIRED: Warning cap/frequency policy is intentionally not enforced" (`warning_service.go:85-87`).

---

## 11. Appeal Analysis

| Aspek | Fakta | Status |
|---|---|---|
| Entity | `Appeal` (`entity/appeal.go`) — field `CaseID` | PROVEN |
| Table | `appeals` — kolom `report_id` | PROVEN |
| FK/reference | **`appeals.report_id` menyimpan CaseID.** Bukti: `appeal_repository_impl.go:38-55` INSERT `appeal.CaseID` ke kolom `report_id`; `CreateWithPendingCheck` WHERE `report_id = appeal.CaseID`; `scanRow` membaca `report_id` ke `CaseID` | PROVEN |
| Menunjuk Report/Case/Decision? | Menunjuk **Case** (`moderation_cases`), walau kolom bernama `report_id`. Bukan Decision (Decision tidak ada sebagai entity) | PROVEN |
| Create authorization | `POST /appeals` = RequireAuth; service memverifikasi **resource owner** (`getResourceOwner`) dan case terminal (`enforced`/`rejected`) | PROVEN |
| Review authorization | `PUT /admin/appeals/:id/review` = `moderation.appeal.review` + handler check | PROVEN |
| Lifecycle | `pending → approved/rejected`; hanya pending bisa di-review | PROVEN |
| State transitions | `CanAppealTransition` (appeal.go:161-167) | PROVEN |
| Response | `admin_response` + `reviewed_by` + `reviewed_at` | PROVEN |
| Reversal/restoration | Approve content/comment → outbox `moderation.{type}.restored` → `RestoreFromModeration`. ForSale/auction/user → **record-only** (tidak ada auto-restore; `isAutoRestorableType` hanya content/comment) | PROVEN |
| Duplicate appeal | `CreateWithPendingCheck` CTE + `FOR UPDATE` (repository) → `ErrDuplicatePendingAppeal`. **TAPI tidak ada DB unique constraint** — guard hanya di aplikasi | PROVEN |
| Soft deletion | Kolom `deleted_at` ada di tabel, **tidak dipakai** code | PROVEN |
| DB/code naming mismatch | **YA — `report_id` vs `CaseID`** | PROVEN → CONFLICT |

Catatan menarik: canonical spec §10 menginginkan `Appeal → Decision`. Implementasi menunjuk Case (karena Decision tidak ada). Status **CONFLICT dengan canonical**, tapi secara fungsional appeal menunjuk keputusan terakhir case (status terminal).

---

## 12. Target-by-Target Authorization

Pola umum: `client → route auth → service → repo.ResourceExists → insert`. **Tidak ada target-specific authorization selain existence check** untuk content/comment/for_sale/auction/user. Chat_message punya guard khusus participant + non-support.

| Target | Siapa boleh report | Owner boleh report sendiri? | Existence verify | Deleted dapat direport? | Hidden dapat direport? | Selesai/terjual dapat direport? |
|---|---|---|---|---|---|---|
| content | semua auth user | **YA (tidak dicegah)** — BUSINESS DECISION REQUIRED | `contents WHERE deleted_at IS NULL` | Tidak | Tidak (hidden = deleted_at set) | n/a |
| comment | semua auth user | **YA (tidak dicegah)** — BUSINESS DECISION REQUIRED | `comments WHERE deleted_at IS NULL` | Tidak | Tidak | n/a |
| for_sale | semua auth user | **YA (tidak dicegah)** — BUSINESS DECISION REQUIRED | `for_sales` (tanpa deleted guard) | n/a | **Ya — hidden/withdrawn dapat direport** (sengaja, komentar repo:345-346) | **Ya — sold dapat direport** |
| auction | semua auth user | **YA (tidak dicegah)** — BUSINESS DECISION REQUIRED | `auctions` (tanpa guard) | n/a | Ya (draft/withdrawn) | **Ya — ended/cancelled dapat direport** |
| user/profile | semua auth user | user melaporkan dirinya sendiri? — **tidak dicegah** — BUSINESS DECISION REQUIRED | `users WHERE deleted_at IS NULL` | Tidak | n/a | suspended tetap bisa direport (deleted_at null) |
| chat_message | participant ruang, non-support | **YA (tidak dicegah)** | `chat_messages WHERE deleted_at IS NULL` | Tidak | Tidak | n/a |

- Report terhadap user/profile == report object biasa (`resource_type='user'`), tidak ada bedanya dengan report objek.
- Authorization **hanya generic** (`RequireActiveAccount` + existence), tidak target-specific.

---

## 13. Target-by-Target Enforcement

### Content
- Authority state: `ContentService` (`SoftDeleteForModeration` / `RestoreFromModeration` — `content_service.go:729,769`). Moderation via outbox event, **bukan direct DB mutation**.
- Idempotent: already-deleted → nil; restore keyed on `deleted_at` marker.
- Duplicate enforcement: worker retry-safe (event handler mengembalikan error utk retry; method idempotent).
- Restoration: appeal approve → `moderation.content.restored`.

### Comment
- Authority: `CommentService.SoftDeleteForModeration` / `RestoreFromModeration` (handler `handleCommentRemoved/Restored`).
- Idempotent, restore via appeal.

### For Sale
- Authority: `ForSaleService.Withdraw` (`for_sale_service.go:358`) dan `RestoreFromModeration` (`:407`).
- **Side effect:** `Withdraw` meng-invalidasi shipping quotes (`InvalidateQuotesByProduct`) dan emit `for_sale.withdrawn`. Ini adalah interaksi commerce — tapi bukan order/payment/ledger mutation.
- Sold inventory: restore ditolak (`MarkActiveFromModeration` → error "status is sold"); handler treat sebagai non-retryable (`isNonRetryableRestoreError`).
- **Moderation TIDAK menyentuh order/payment/ledger/settlement** untuk for_sale — PROVEN (trace `Withdraw` tidak ada mutation finance).

### Auction
- Authority: `AuctionService.CancelForModeration` (`auction_service.go:1289`) — memanggil `auction.Cancel()` (entity) → `status=cancelled`, emit `auction.cancelled`.
- **KONFLIK:** serverboot `dependencies.go:2320-2322` mengklaim `auction.removed` "PARKED (no-op)" karena `IsSeller rejects uuid.Nil; CanCancel rejects active-with-bids`. **Tapi** `ModerationEventHandler.handleAuctionRemoved` (`moderation_event_handler.go:547-595`) benar-benar memanggil `CancelForModeration`, dan test `TestModerationHandler_AuctionRemoved_Success` (`moderation_event_handler_test.go:233-247`) membuktikan pemanggilan. `Auction.Cancel()` hanya memerlukan `canTransition(status, cancelled)` — **tidak ada pengecekan seller/bids**.
  - Klaim "PARKED" di komentar wiring **OBSOLETE/keliru**; runtime actual = auction di-cancel oleh moderation.
  - Konsekuensi: auction dengan bids aktif dapat di-cancel oleh moderation (status → cancelled) tanpa mekanisme refund/payment reversal otomatis. `CancelForModeration` tidak menyentuh bids/payment/settlement, tapi mengubah auction ke cancelled yang mempengaruhi lifecycle commerce.
- Restore: **intentionally unsupported** (`handleAuctionRestored` no-op, `moderation_event_handler.go:754-765`).
- `BUSINESS DECISION REQUIRED`: apakah moderasi boleh cancel auction aktif-dengan-bid? Canonical §8 melarang moderation mengambil alih commerce settlement, tapi cancel auction bukan settlement — perlu keputusan Owner.

### User/Profile
- Authority: `userRepo.GetByIDForUpdate` + set `account_status='suspended'` (handler `handleUserAction`, `moderation_event_handler.go:603-666`). **Direct user table mutation oleh moderation worker** (bukan lewat UserService method) — tapi tetap via repo user.
- Restore: appeal approve → `handleUserRestored` set `active`, dengan guard: **banned tidak bisa di-restore** (perlu admin unban).
- Idempotent.

### Enforcement failure visibility
- Tidak ada persistence enforcement result. Failure hanya terlihat via log (`h.log.Error`) dan retry outbox.
- `moderation_cases` tidak menyimpan enforcement attempt/success/failure.
- **PROVEN: enforcement failure TIDAK terlihat** di data layer.

---

## 14. Concurrency / Idempotency Proof

### Report creation
- Duplicate sequential: app check `HasUserReportedResource` → error → handler 409. **PROVEN** (`moderation_service.go:123-130`).
- Concurrent duplicate: DB unique `idx_moderation_cases_one_report_per_user` + handler maps pg 23505 → 409. **PROVEN** (`moderation_handler.go:204-210`).
- Transaction: `CreateCase` dalam `WithTx`. **PROVEN**.

### Admin decision
- Concurrent admin action: `GetForUpdate` (FOR UPDATE) + entity guard `ErrAlreadyReviewed`. **PROVEN** (`moderation_repository_impl.go:105-130`, `governance_case.go:166-192`).
- Terminal behavior: pending → terminal; terminal cannot change. **PROVEN**.
- Repeated action: kedua request serialize; kedua gagal `ErrAlreadyReviewed`. **PROVEN**.

### Enforcement
- Duplicate event: outbox worker — `InsertEvent` dalam tx yang sama dgn case update; worker retry pada error. Idempotency handler per-target **PROVEN** (content/comment/user/for_sale/auction/chat_message semuanya treat terminal sebagai sukses).
- Target mutation idempotency: **PROVEN** per method (dibuktikan di §13).
- **Enforcement success persistence: UNPROVEN / TIDAK ADA** (tidak ada status enforcement).

### Warning
- Repeated issue: **TIDAK ada guard** — warning ganda diperbolehkan. **PROVEN** (tidak ada unique/limit). Status: kebijakan cap "OWNER DECISION REQUIRED" (`warning_service.go:85-87`).
- Repeated revoke: `GetForUpdate` + `ErrWarningAlreadyRevoked`. **PROVEN**.

### Appeal
- Duplicate appeal (sequential): `CreateWithPendingCheck`. **PROVEN**.
- Duplicate appeal (concurrent): CTE `FOR UPDATE` + `WHERE NOT EXISTS`. **PROVEN** (repository `CreateWithPendingCheck`).
- Concurrent review: `GetForUpdate` + `ErrAppealAlreadyReviewed`. **PROVEN**.
- **Catatan:** tidak ada DB unique constraint untuk pending appeal — jika repository guard di-skip (mis. bug), DB tidak menolak. **UNPROVEN secara DB-level.**

---

## 15. Test / Runtime Proof Inventory

### Backend tests (moderation)
- `moderation_service_test.go` — service behavior (unit/mock).
- `moderation_service_intake_test.go`, `chat_message_intake_test.go` — intake validation.
- `moderation_handler_test.go`, `moderation_handler_intake_test.go`, `moderation_handler_enforce_notes_test.go`, `moderation_handler_list_cases_test.go`, `moderation_handler_my_cases_test.go` — handler HTTP.
- `moderation_evidence_test.go` (+integration) — evidence endpoint.
- `appeal_service_test.go`, `appeal_handler_test.go`, `appeal_capability_guard_test.go`.
- `warning_service_test.go`, `warning_handler_test.go`.
- `moderation_repository_*_test.go` — SQL parsing via mock tx (bukan DB real).
- `worker/moderation_event_handler_test.go` — enforcement routing (mock target services).
- `worker/outbox_event_registry_test.go` — event registry.
- `serverboot/moderation_event_handler_gate_test.go` — default-ON gate.
- `tests/migration_000047_schema_state_proof_test.go` — schema vocabulary proof.

### Mobile tests
- `test/domains/system/report/`: `report_dto_contract_test.dart`, `report_mapper_contract_test.dart`, `my_reports_screen_widget_test.dart`, `report_target_type_contract_test.dart`, `report_repository_impl_contract_test.dart`, `report_provider_graph_test.dart`, `report_notifier_contract_test.dart`, `report_submission_dialog_test.dart`.

### Admin tests
- `useModeration.test.ts`, `useModeration.test.tsx`, `CaseDetailModal.test.tsx`, `AppealsPage.test.tsx`, `useAppeals.test.ts`, `useWarnings.test.ts`.

### Runtime/divergence notes
- **Sebagian besar backend moderation tests memakai mock tx / fixture, bukan DB real** — membuktikan SQL string & logic, bukan persistence runtime. Integration proofs terbatas pada evidence endpoint dan schema migration.
- `moderation_handler.go` komentar menyebut schema `000100_initial_schema` dan kolom `body` (fetchContentPreview:773-780) — **referensi migration yang tidak ada** (migration tertinggi 000054, tabel contents pakai `caption`).
- Worker gate test membuktikan handler default-ON (`serverboot/moderation_event_handler_gate_test.go`).
- Mobile `ReportStatus.underReview` dan `ReportAction` tidak punya padanan backend — mapper test menguji contract mobile-only, bukan wire real.

---

## 16. Competing Authority

| Candidate | Path | Role | Producer | Consumer | Active? | Bisa jadi authority? | Bisa resurrect? |
|---|---|---|---|---|---|---|---|
| `GovernanceCase`/`moderation_cases` | entity+repo+service+handler | Report+Case+Decision | `ModerationService.CreateCase` | admin queue, mobile my-cases | **YA** | Ya (satu-satunya saat ini) | — |
| Outbox `moderation.*.removed/suspended/hidden` | `moderation_service.go:199` | Enforcement trigger | `ReviewCase` | `ModerationEventHandler` | **YA** | Ya | — |
| `DomainAction` + `domain_actions` | entity+repo | Enforcement unit (alternatif) | **tidak ada** | `DomainActionWorker` (PARKED) | **TIDAK** | Ya | **YA — kode lengkap, tinggal wiring** |
| `AppealReversalService` | application | Reversal alternatif | tidak ada | worker PARKED | **TIDAK** | Ya | **YA** |
| `CapModerationContentView`/`ContentRemove` | capability.go:129,132 | Permission | — | — | **TIDAK dipakai route mana pun** | — | Ya |
| `removed` status enum | migration 000001 | Status ghost | tidak ada | tidak ada | **TIDAK** | Tidak (bukan authority) | Hanya via SQL |
| `admin_audit_logs` | audit | Audit trail | handler moderation | admin UI | **YA** | Audit saja | — |

**Kesimpulan:** authority aktif saat ini adalah `moderation_cases` (case+decision) dan outbox events (enforcement). `DomainAction`/`DomainActionWorker`/`AppealReversalService` adalah **competing authority potensial yang hidup sebagai zombie** — meski tidak aktif, seluruh kode (termasuk worker, repository, event registry stubs) masih ada dan dapat di-resurrect tanpa perubahan arsitektur.

---

## 17. Legacy / Residue

| Item | Status | Evidence |
|---|---|---|
| `fixed_price_sale` di admin types + UI filter + CaseDetailModal | **OBSOLETE** — backend sudah `for_sale`; UI akan broken untuk for_sale | `types/moderation.ts:11,262-269`; `ModerationCasesPage.tsx:35`; `CaseDetailModal.tsx:175` |
| `fixed_price_sale` case di mobile mapper | **OBSOLETE** — tidak akan pernah diterima backend | `report_mapper.dart:85-87` |
| `appeals.report_id` naming | **CONFLICT** — kolom menyimpan CaseID | `appeal_repository_impl.go` |
| `moderation_status_enum 'removed'` | **Ghost** — di DB enum, tidak di code | `migrations/000001:184-190` vs `governance_case.go:46-51` |
| `deleted_at` di `moderation_cases` & `appeals` | **Residue** — tidak dibaca/ditulis | schema vs repository |
| `DomainAction` + worker + repo | **Zombie/PARKED** | `outbox_event_registry.go:198-203` |
| `AppealReversalService` | **PARKED** | `appeal_reversal_service.go:1` |
| `ImageUploader` mobile (upload evidence) | **Dead path** — upload tidak pernah dipanggil dalam submit flow; implementasi S3 masih ada | `report_repository_impl.dart:94-110`; provider `_S3ImageUploader` |
| `ReportStatistics` + `ReportStatisticsMapper` mobile | **Dead code** — tidak ada backend endpoint statistics | `report_mapper.dart:251-304`; repository REMOVED comments |
| `ReviewReportRequest`/`ReviewReportRequestDto` mobile | **Dead code** — admin-only, repo method dihapus | `report.dart:434-478`; `report_dto.dart:98-109` |
| Komentar "migration 000207" dan "schema 000100" | **False reference** | `moderation_service.go:71`; `moderation_handler.go:773-780` |
| `underReview` mobile status & `ReportAction` | **Legacy vocabulary** — tidak ada di backend | `report.dart:46-56` |
| Komentar serverboot "auction PARKED no-op" | **OBSOLETE/keliru** — handler aktif memanggil CancelForModeration | `dependencies.go:2320-2322` vs `moderation_event_handler.go:547` |

---

## 18. Industry/Architecture Assessment

### A. Proper (layak dipertahankan)

1. **Outbox pattern untuk enforcement** — decision → outbox event → worker → domain service. Sudah sesuai canonical "Moderation → Enforcement → Target Domain Authority" (§7). Tidak ada direct DB mutation cross-domain (kecuali user suspend via userRepo yang masih dalam batas user domain).
2. **Idempotency enforcement handler** — semua target treat terminal state sebagai sukses; retry aman.
3. **Forbidden self-mutation** — moderation tidak menyentuh order/payment/ledger/settlement untuk for_sale/auction (dibuktikan §13).
4. **Row locking** — `GetForUpdate` pada case review, appeal review, warning revoke.
5. **Enforce requires note** — auditability keputusan enforcement.
6. **Appeal ownership & terminal-state gating** — appeal hanya oleh resource owner, case harus terminal.
7. **Restore guard** — sold for_sale tidak bisa di-restore; banned user tidak bisa di-restore via appeal.
8. **Capability-based admin RBAC** — route + handler double-check untuk resolve/evidence/appeal review.
9. **Evidence endpoint** — chat evidence hanya via dedicated audited endpoint (bukan preview biasa).
10. **DB unique untuk satu-report-per-user** — mencegah duplikat.

### B. Needs correction (benar secara konsep, implementasi salah/incomplete)

1. **Decision tidak historical** — decision di-overwrite pada case; perlu entity Decision dengan history (I3, I8).
2. **Enforcement tidak memiliki status** — perlu tracking enforcement (pending/succeeded/failed) + retry + failure visibility (I4-I6).
3. **Appeal menunjuk case, bukan decision** — setelah Decision jadi entity, appeal harus menunjuk Decision (canonical §10).
4. **Warning tanpa provenance** — perlu relasi ke Decision; warning tidak boleh muncul tanpa konteks governance (canonical §9).
5. **Admin vocabulary `fixed_price_sale`** — harus diselaraskan ke `for_sale` (breaking UI saat ini).
6. **Admin `Appeal.report_id`** — harus `case_id`.
7. **Duplicate pending appeal tidak ada DB constraint** — guard hanya di repository CTE.
8. **Moderation status enum `removed` ghost** — selaraskan DB enum dengan code (atau sebaliknya).
9. **Case `deleted_at`/appeal `deleted_at` tak terpakai** — putuskan apakah soft-delete diperlukan.

### C. Fundamentally wrong (boundary/authority/lifecycle salah — lebih baik dibongkar daripada ditambal)

1. **`GovernanceCase` mencampur Report + Case + Decision** — ini inti masalah. Satu baris = satu report = satu case = satu decision. Tidak ada cara menambah report kedua ke case yang sama tanpa merombak schema. Tambal-sulam (mis. menambah tabel report terpisah tanpa memutus `moderation_cases.reported_by/reason`) akan menghasilkan **dua authority report** (kolom lama vs tabel baru) — persis anti-pattern yang dilarang `cara-kerja` §2.1.
2. **Model "satu case per report"** — melanggar invariant bisnis canonical (multiple reports → satu case). Menggabungkan case saat runtime (tanpa schema) akan memunculkan duplikasi dan semantik ambigu (case mana yang decision-nya berlaku?).
3. **Tidak ada pemisahan Decision vs Enforcement di persistence** — membuat sistem tidak mampu membedakan "diputuskan" vs "tereksekusi", yang merupakan invariant paling kritikal canonical. Menambal dengan kolom `enforcement_status` di `moderation_cases` akan kembali mencampur tanggung jawab (case menyimpan enforcement), bukan memisah.

**Mengapa tambal-sulam berisiko untuk kategori C:** setiap kolom/relasi yang ditambahkan ke `moderation_cases` akan memperkuat otoritasnya sebagai super-entity, sehingga pembongkaran berikutnya semakin mahal dan semakin banyak consumer yang harus diputus (admin, mobile, worker, test). Prinsip `cara-kerja` §16.6 dan canonical §17 mendukung pembongkaran total dibanding compatibility.

### D. Business decision required

1. Apakah user boleh melaporkan objek/content miliknya sendiri? (saat ini: boleh)
2. Apakah for_sale/auction yang sudah sold/ended/cancelled boleh tetap direport? (saat ini: ya, sengaja)
3. Apakah moderasi boleh cancel auction aktif-dengan-bid? (saat ini: ya — runtime AKTIF walau komentar bilang PARKED)
4. `chat_message` — pertahankan sebagai target moderation atau matikan? (canonical §12 tidak memasukkan chat_message; implementasi penuh: report + enforcement + evidence + appeal-excluded)
5. Warning cap/frequency policy (komentar service sudah menandai "OWNER DECISION REQUIRED").
6. Apakah perlu Strike system? (canonical §9: tidak; implementasi: tidak ada — jangan tambah tanpa kebutuhan nyata)

### E. Out of scope

- `evaluator` package (`search_*`, `feed_*`) memakai string `"removed"` untuk lifecycle coarsening — **terkait tapi bukan moderation authority**; ada kemungkinan kolusi istilah dengan status moderation lama. Dicatat saja.
- `staging_rollout_ab/main.go` "warnings" — tidak terkait moderation warnings.

---

## 19. Business Decisions Required

1. **Model report**: pisahkan Report entity dari Case? (canonical: ya)
2. **Grouping**: satu subject maksimal satu active case dengan banyak report? (canonical: ya)
3. **Decision history**: simpan setiap keputusan sebagai entity append-only? (canonical: ya)
4. **Enforcement tracking**: buat entity Enforcement dengan status & retry? (canonical: ya)
5. **Self-report**: dicegah atau dibiarkan?
6. **Terminal-state targets**: boleh direport atau tidak (for_sale sold, auction ended/cancelled)?
7. **Auction cancel oleh moderasi**: dipertahankan (aktif) atau benar-benar di-park?
8. **chat_message**: canonical atau legacy untuk dibunuh?
9. **Warning provenance**: wajib terikat decision, atau tetap independen?
10. **`fixed_price_sale` residue di admin/mobile**: konfirmasi pembongkaran.
11. **DomainAction/AppealReversalService zombie**: hapus total atau resureksi?

---

## 20. Recommended Next Audit Scope

Audit berikut (jika disetujui) sebaiknya:

1. **Decision vs Enforcement design** — detail transisi canonical dan mapping dari current.
2. **Schema redesign** untuk Report/Case/Decision/Enforcement/Warning/Appeal (belum implementasi — hanya desain).
3. **Commerce boundary** — audit penuh `ForSaleService.Withdraw` side effects (shipping quote invalidation) dan `CancelForModeration` terhadap order/bid state.
4. **Outbox reliability** — retry policy, dead-letter, idempotency key untuk enforcement events (saat ini tidak ada idempotency key eksplisit di payload).
5. **Test stratifikasi** — mana test yang membuktikan runtime (DB real) vs contract-only, dan mana yang perlu dimigrasi ke proof canonical setelah desain baru.

---

## Lampiran — Dokumen yang dibaca

| Dokumen | Ditemukan | Status |
|---|---|---|
| `cara-kerja-updated.md` | Root repository — **YA, dibaca penuh (1086 baris)** | Canonical working rules |
| `PRD.md` | **TIDAK ADA di filesystem** — terhapus (`git status`: `D PRD.md`) | Tidak dapat dibaca; bukan authority final |
| `LABUDA — CANONICAL MODERATION SPECIFICATION v1.md` | Root repository (nama: `LABUDA — CANONICAL MODERATION SPECIFICATION v1.md`) — **YA, dibaca penuh (637 baris)** | **Canonical moderation direction** |
| Laporan audit sesi sebelumnya | Tidak ditemukan di `docs/` (kosong) | Tidak dianggap authority |

---

```text
AUDIT STATUS: PROVEN

CANONICAL AUTHORITY FINDING:
- Satu-satunya authority aktif: moderation_cases (GovernanceCase) untuk report+case+decision,
  dan outbox events moderation.*.removed/suspended/hidden untuk enforcement trigger.
- GovernanceCase mencampur Report + Case + Decision dalam satu baris (melanggar I1/I2/I3/I8).
- Decision vs Enforcement TIDAK dapat dibedakan di persistence (melanggar I4/I5/I6).
- Warning TIDAK memiliki provenance ke Decision (melanggar canonical §9).
- Appeal menunjuk Case (kolom report_id menyimpan CaseID) — naming mismatch terbukti.
- DomainAction/DomainActionWorker/AppealReversalService adalah zombie PARKED (residue competing authority).
- Auction enforcement AKTIF di runtime walau komentar wiring menyatakan PARKED (CONFLICT).

FUNDAMENTAL DESIGN PROBLEMS:
1. GovernanceCase = super-entity Report+Case+Decision (satu baris, tanpa grouping, tanpa decision history).
2. Tidak ada pemisahan Decision (persisted, historical) vs Enforcement (execution status/retry/failure).
3. Tidak ada entity Report; multiple reports per subject tidak mungkin.
4. Warning independen tanpa provenance governance.
5. Appeal mereferensikan Case dan kolom DB bernama report_id (mismatch).
6. Vocabulary residue (fixed_price_sale) aktif di admin/mobile dan akan merusak UI for_sale.
7. Dua jalur enforcement (aktif outbox + zombie DomainAction) berisiko competing authority.

BUSINESS DECISIONS REQUIRED:
- Self-report: izinkan atau larang?
- Report terminal-state targets (sold/ended/cancelled): izinkan atau larang?
- Auction cancel oleh moderasi: pertahankan (runtime AKTIF) atau benar-benar parkir?
- chat_message: canonical atau legacy?
- Warning: wajib terikat decision atau tetap independen?
- Warning cap/frequency policy.
- Nasib DomainAction/AppealReversalService zombie: hapus atau resureksi?

NEXT AUDIT:
- Design decision vs enforcement (persistence, retry, idempotency key, failure visibility).
- Schema redesign Report/Case/Decision/Enforcement (+Warning provenance, Appeal→Decision).
- Commerce boundary untuk Withdraw (shipping quote) dan CancelForModeration (bid/order state).
```
