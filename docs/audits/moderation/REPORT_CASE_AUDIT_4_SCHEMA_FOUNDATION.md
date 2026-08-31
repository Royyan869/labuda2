# AUDIT 4 — MODERATION SCHEMA / FOUNDATION

- **Tanggal audit:** 2026-08-30
- **Mode:** READ-ONLY PRE-IMPLEMENTATION AUDIT — tidak ada implementasi, schema, migration, test, admin, mobile, worker, atau commit
- **Satu-satunya artefak baru:** laporan ini
- **Baseline:** current filesystem (bukan git history)
- **Authority desain:** `LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1.md`, `LABUDA — CANONICAL MODERATION DESIGN v1.md`, `LABUDA — CANONICAL MODERATION SPECIFICATION v1.md`, `cara-kerja-updated.md`
- **Input faktual:** `docs/audits/moderation/REPORT_CASE_AUDIT_1_FACTUAL.md`, `REPORT_CASE_AUDIT_2_ENFORCEMENT_BOUNDARY.md`, `REPORT_CASE_AUDIT_3_CANONICAL_DESIGN.md`
- **Evidence rule:** setiap klaim disertai `file:line`, nama migration, tabel/index/constraint.

---

## A. Executive Factual Summary

### A.1 Canonical direction yang dipakai (owner-approved)

```text
Report → Case → Decision → Enforcement → Outbox delivery → Target Domain
```

Target canonical: `content`, `comment`, `for_sale`, `auction`, `user`. `profile` = `user`. `chat_message` = **out of scope** (tidak dipertahankan).

Entity canonical: Report, Case, Decision, Enforcement, Warning, Appeal, Governance Audit History. Outbox = delivery infrastructure, **bukan** enforcement authority.

### A.2 State faktual schema moderation saat ini

**FACT:** Seluruh schema moderation lahir di **satu migration baseline `000001_canonical_schema.up.sql`** (106 tables, 52 enum types). Tabel moderation terkait:

| Table | Lokasi | Fungsi aktual |
|---|---|---|
| `moderation_cases` | `000001:963-976` | **Super-entity GovernanceCase**: Report + Case + Decision dalam satu baris |
| `appeals` | `000001:454-466` | Appeal; kolom `report_id` menyimpan **CaseID** (bukan ReportID, bukan DecisionID) |
| `user_warnings` | `000001:1772-1783` | Warning standalone tanpa provenance governance |
| `audit_events` | `000001:500-509` | Audit append-only (umum); **tidak dipakai moderation** |
| `admin_audit_logs` | `000001:444-452` | Audit admin best-effort (LogSafe); dipakai moderation mutation |
| `outbox` | `000001:1157-1169` | Delivery infra; retry **runtime-broken** (Audit 2) |
| `outbox_archive` | `000001:1171-1184` | Arsip outbox |

**TIDAK ADA** tabel: `reports`, `cases`, `decisions`, `enforcements`, `warnings` (canonical), `governance_audit_history`. **TIDAK ADA** tabel `domain_actions` (DomainAction = zombie PARKED, tidak ada migration).

**FACT:** Tidak ada entity Report terpisah, tidak ada entity Decision, tidak ada entity Enforcement di schema maupun Go code.

### A.3 GovernanceCase adalah satu-satunya authority aktif

**FACT:** `moderation_cases` mencampur:
- **Report**: `reported_by`, `reason`, `created_at`
- **Case**: `resource_type`, `resource_id`, `status`
- **Decision**: `reviewed_by`, `decision_note`, `reviewed_at`

**FACT:** unique index `idx_moderation_cases_one_report_per_user (reported_by, resource_type, resource_id)` (`000001:2113`) membuat **satu report = satu case**. Multiple reports per subject **tidak mungkin** (I1, I2 canonical dilanggar secara struktural).

### A.4 Enforcement = outbox event, bukan persisted execution

**FACT:** `status='enforced'` pada `moderation_cases` berarti "decision enforce dibuat + outbox event di-emit dalam satu transaksi" (`moderation_service.go:198-211`). Tidak ada enforcement status/result di persistence. Worker `ModerationEventHandler` (`worker/moderation_event_handler.go`) mengeksekusi per-target tapi **tidak menulis hasil kembali** ke database.

### A.5 Warning tanpa provenance, Appeal menunjuk Case, Audit best-effort

- **FACT:** `user_warnings` tidak punya FK ke governance mana pun; hanya FK ke `users` (`000001:2424-2426`). Dapat dibuat standalone via `POST /admin/warnings` (`routes_core.go:920-922`).
- **FACT:** `appeals.report_id` menyimpan CaseID (`appeal_repository_impl.go:38-55` INSERT `appeal.CaseID` → `report_id`). Tidak ada FK (`appeals` tidak muncul di daftar FK 000001).
- **FACT:** moderation mutations memakai `adminAuditLogger.LogSafe(...)` (best-effort, ignore error). `audit_events` (reliable, append-only) **tidak dipakai moderation** (`AuditService` hanya dipakai order/payment/coin/dll).

### A.6 Migration chain dan authority

**FACT:** 54 migration pairs di `backend/migrations/` (000001–000054), dijalankan oleh `cmd/migrate` / `pkg/migration/executor.go` (pgx, `schema_migrations` table). `pkg/database/migrate.go` (golang-migrate) adalah **jalur legacy/tooling-only** yang tidak dipakai runtime (guard test `migration_authority_guard_test.go:29-39` menegaskan).

### A.7 Verdict awal

**FACT:** schema foundation saat ini **tidak dapat mendukung** canonical Report/Case/Decision/Enforcement separation. `GovernanceCase` adalah super-entity yang harus **dibongkar**, bukan diperpanjang. Migration reset/rewrite **diperlukan**.

---

## B. Current Migration Chain

### B.1 File yang benar-benar ada (inventory)

**FACT:** `backend/migrations/` berisi 54 numbered pairs (up+down) + `README.md`:

```
000001_canonical_schema
000002_negotiation_schema_alignment
000003_identity_email_uniqueness
000004_auction_anti_sniping
000005_payment_webhook_captured_after_expiry
000006_payment_method_fee_model
000007_payment_method_rate_source_baseline
000008_payment_webhook_captured_after_expiry_index
000009_fixed_price_sale_quantity_persistence
000010_product_sale_channel_canonicalization
000011_prune_orphan_tables
000012_remove_seller_role_authority
000013_shipping_quote_supersession_hardening
000014_shipping_authority_hard_purge
000015_shipping_authority_purge_orphan_drafts
000016_purge_legacy_listing_shipping_options
000017_primary_address_invariant_hardening
000018_add_seller_profile_store_image
000019_media_reference_repair
000020_add_profile_cover
000021_seller_profile_store_name_non_empty_check
000022_asset_specific_media_authority
000023_typed_commerce_media_authority
000024_typed_commerce_media_metadata
000025_user_presence_foundation
000026_fcm_token_device_identity_hardening
000027_chat_media_reply_authority
000028_bank_account_primary_invariant_hardening
000029_chat_commerce_references
000030_chat_room_context_backfill_and_purge
000031_comment_commerce_reference_canonical
000032_chat_message_idempotency_actor_scoping
000033_chat_message_fingerprint_hardening
000034_chat_message_resource_occurrences
000035_coin_reservations_authority
000036_payment_coins_to_use_rename
000037_payment_net_amount_drop
000038_payment_coin_spend_reference_type
000039_content_resource_occurrences
000040_refund_product_shipping_split
000041_drop_legacy_content_share_reference
000042_alert_active_group_key_uniqueness
000043_content_mentioned_users
000044_product_lifecycle_removal
000045_order_item_product_identity_convergence
000046_product_content_authority_convergence
000047_for_sale_vocabulary_convergence
000048_product_selling_surface_exclusivity
000049_product_selling_surface_permanent_exclusivity
000050_for_sale_single_row_per_product
000051_discount_targeting_convergence
000052_drop_dead_content_mentioned_users
000053_restore_content_mentioned_users
000054_drop_dead_chat_commerce_references
```

**FACT:** `000025_user_presence_foundation` hanya memiliki `.up.sql` (tidak ada `.down.sql`) — lihat `backend/migrations/` listing. Ini satu-satunya pair yang tidak lengkap.

**FACT:** `000001` header (`000001_canonical_schema.up.sql:1-11`) menyatakan baseline di-generate dari live DB (v100–v229), menggantikan seluruh migration lama yang sudah di-squash.

### B.2 Runner dan version tracking

**FACT:** `pkg/migration/executor.go:15-23` — tabel `public.schema_migrations (version INTEGER PK, name TEXT, applied_at TIMESTAMPTZ)`. Runner memfilter statement `BEGIN;/COMMIT;` (`executor.go:256-263`), dan mendeteksi `ADD VALUE` untuk menjalankan non-transaksional (`executor.go:371-387`).

**FACT:** `cmd/migrate/main.go` adalah CLI production canonical. `pkg/database/migrate.go` (golang-migrate) **tidak dipanggil** oleh siapa pun (`grep database.RunMigrations` → 0 hits non-definition). Guard test `migration_authority_guard_test.go:29-39` memastikan `pkg/database/migrate.go` tetap "core_server does not [auto-run]".

### B.3 Migration ordering (moderation-relevant)

| Version | Perubahan moderasi-relevan | Evidence |
|---|---|---|
| 000001 | `moderation_cases`, `appeals`, `user_warnings`, `audit_events`, `admin_audit_logs`, `outbox`, `outbox_archive`, enums `moderation_resource_enum` (berisi `fixed_price_sale`) + `moderation_status_enum` | `000001:175-190, 444-466, 500-509, 963-976, 1157-1184, 1772-1783` |
| 000010 | Drops `listings` table + trigger single-active-channel (rule 9) | `000010_product_sale_channel_canonicalization.up.sql:26-111` |
| 000011 | Drops 6 orphan tables (actors, bnr_classifications, financial_reconciliations, search_results, ticket_escalations, user_online_status) | `000011_prune_orphan_tables.up.sql:15-20` |
| 000047 | **Renames `fixed_price_sales` → `for_sales`**; enum `fixed_price_sale_status_enum` → `for_sale_status_enum`; `moderation_resource_enum` value `'fixed_price_sale'` → `'for_sale'`; drops orphan `listing_*` enums | `000047_for_sale_vocabulary_convergence.up.sql:94-103, 165-193, 336-339` |
| 000048/000049 | Selling-surface exclusivity trigger (permanent) menggantikan active-only trigger | `000049:24-83` |
| 000050 | Single for_sale row per product trigger + advisory lock | `000050_for_sale_single_row_per_product.up.sql:1-36` |
| 000054 | Drops dead `chat_commerce_references` table + enum | `000054_drop_dead_chat_commerce_references.up.sql:12-13` |

**Tidak ada migration moderation-specific lain.** Semua tabel moderation lahir di 000001; 000047 hanya mengganti vocabulary enum.

---

## C. Current Moderation Tables

### C.1 `moderation_cases` (tabel GovernanceCase)

**FACT:** `000001_canonical_schema.up.sql:963-976`:

```sql
CREATE TABLE moderation_cases (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    resource_type moderation_resource_enum NOT NULL,
    resource_id uuid NOT NULL,
    status moderation_status_enum DEFAULT 'pending'::moderation_status_enum NOT NULL,
    reported_by uuid NOT NULL,            -- tanpa FK ke users
    reviewed_by uuid,                     -- tanpa FK ke users
    reason text NOT NULL,
    decision_note text,
    created_at timestamptz NOT NULL DEFAULT now(),
    reviewed_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz                -- TIDAK dibaca/ditulis repository
);
```

**FACT:** `deleted_at` tidak pernah di-read/write oleh repository (`moderation_repository_impl.go` SELECT/INSERT/UPDATE tidak menyentuh `deleted_at`; `selectColumns` di :72-74 tidak memuatnya).

### C.2 `appeals`

**FACT:** `000001:454-466`:

```sql
CREATE TABLE appeals (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    report_id uuid NOT NULL,              -- MENYIMPAN CaseID (bukti repository)
    appealed_by uuid NOT NULL,
    message text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,  -- text, bukan enum
    reviewed_by uuid,
    admin_response text,
    reviewed_at timestamptz,
    created_at timestamptz DEFAULT now() NOT NULL,
    updated_at timestamptz DEFAULT now() NOT NULL,
    deleted_at timestamptz                -- TIDAK dipakai
);
```

### C.3 `user_warnings`

**FACT:** `000001:1772-1783`:

```sql
CREATE TABLE user_warnings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,                -- FK → users (:2426)
    issued_by uuid NOT NULL,              -- FK → users (:2424)
    level text NOT NULL,                  -- CHECK info|warning|severe (:2533)
    reason text NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    revoked_at timestamptz,
    revoked_by uuid,                      -- FK → users (:2425)
    created_at timestamptz DEFAULT now() NOT NULL,
    expires_at timestamptz
);
```

### C.4 `audit_events` (umum, tidak dipakai moderation)

**FACT:** `000001:500-509`:

```sql
CREATE TABLE audit_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    event_type text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    actor_type text NOT NULL,             -- user|admin|system|worker|api (entity/audit_event.go:30-41)
    actor_id uuid,                        -- FK → users ON DELETE SET NULL (:2287)
    payload_json jsonb,
    created_at timestamptz DEFAULT now() NOT NULL
);
```

**FACT:** `AuditService.Emit` (`internal/governance/audit/application/audit_service.go:77-127`) bersifat **non-blocking** (error di-log, tidak di-return) bila dipanggil tanpa tx; tapi bila dipanggil dengan `tx` dalam transaksi mutation, event ikut commit/rollback bersama mutation. Moderation **tidak memanggilnya sama sekali** (grep moderation → audit: hanya test; handler memakai `adminAuditLogger`).

### C.5 `admin_audit_logs` (best-effort admin audit)

**FACT:** `000001:444-452`:

```sql
CREATE TABLE admin_audit_logs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    actor_id uuid NOT NULL,
    action_type text NOT NULL,
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    metadata jsonb,
    created_at timestamptz DEFAULT now() NOT NULL
);
```

**FACT:** Tidak ada FK. `LogSafe` meng-ignore error (`internal/audit/admin_audit_logger.go:88-90`). `LogTx` dapat dipakai atomic (misal evidence read), tapi moderation mutation memakai `LogSafe`.

### C.6 `outbox` / `outbox_archive`

**FACT:** `000001:1157-1169` (outbox), `:1171-1184` (outbox_archive):

```sql
CREATE TABLE outbox (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status outbox_status_enum DEFAULT 'pending' NOT NULL,
    retry_count integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamptz,
    idempotency_key text NOT NULL,        -- UNIQUE (:1979)
    created_at, updated_at
);
```

**FACT:** `idx_outbox_status` partial `WHERE status IN ('pending','processing')` (`000001:2156`).

---

## D. Current Enums

### D.1 `moderation_resource_enum`

- **FACT (000001 baseline):** `('content','comment','fixed_price_sale','auction','user','chat_message')` — `000001:175-182`.
- **FACT (setelah 000047):** `('content','comment','for_sale','auction','user','chat_message')` — `000047_for_sale_vocabulary_convergence.up.sql:96`.

**Canonical gap:** `chat_message` masih ada di enum (harus dihapus — out of v1 scope). Nilai lain sesuai canonical target (content/comment/for_sale/auction/user).

### D.2 `moderation_status_enum`

**FACT:** `('pending','approved','rejected','removed','enforced')` — `000001:184-190`.

**FACT:** `removed` adalah **semantic ghost**: tidak ada di Go entity `GovernanceCaseStatus` (`governance_case.go:46-51` hanya pending/approved/rejected/enforced). `removed` hanya bisa di-insert via raw SQL. **Canonical menolak `removed` dan `enforced` sebagai case status.**

### D.3 `outbox_status_enum`

**FACT:** `('pending','processing','succeeded','failed','dead_letter')` — `000001:226-232`. Sesuai kebutuhan canonical delivery (5 status cukup).

### D.4 Enum lain yang relevan

| Enum | Nilai | Keterangan |
|---|---|---|
| `for_sale_status_enum` | `draft, active, sold, withdrawn` | Rename dari `fixed_price_sale_status_enum` (000047:43) |
| `user_account_status_enum` | `active, suspended, banned` | `000001:379-383`; enforcement worker menulis `suspended` langsung |
| `content_visibility_enum` | `public, followers_only, private` | `000001:89-93`; moderation **tidak boleh** menulis visibility |
| `sale_surface_type_enum` | `for_sale, auction, negotiation` | `000047:70` |

---

## E. Current Constraints and Indexes

### E.1 PK

**FACT:** `moderation_cases_pkey`, `appeals_pkey`, `user_warnings_pkey`, `audit_events_pkey`, `admin_audit_logs_pkey`, `outbox_pkey`, `outbox_archive_pkey` — `000001:1856, 1859, 1896, 1907-1908, 1953, 1855`.

### E.2 Unique

| Constraint | Table | Kolom | Lokasi | Makna |
|---|---|---|---|---|
| `idx_moderation_cases_one_report_per_user` | moderation_cases | `(reported_by, resource_type, resource_id)` | `000001:2113` | 1 report per user per subject; **membuat 1 report = 1 case** |
| `outbox_idempotency_key_key` | outbox | `(idempotency_key)` | `000001:1979` | Idempotent delivery |
| `outbox_archive_idempotency_key_key` | outbox_archive | `(idempotency_key)` | `000001:1980` | — |
| `uniq_active_for_sale_per_product` | for_sales | `(product_id) WHERE status IN (draft,active)` | `000047:191-193` | Pattern partial unique |
| `uniq_active_auction_per_product` | auctions | `(product_id) WHERE status IN (...)` | `000001:2015` | Pattern partial unique |

**FACT:** Tidak ada unique constraint untuk pending appeal (guard hanya di repository CTE `appeal_repository_impl.go:86-99`). Tidak ada unique untuk warning.

### E.3 Indexes (non-constraint)

| Index | Table | Kolom | Lokasi |
|---|---|---|---|
| `idx_appeals_report_id` | appeals | `(report_id)` | `000001:2006` |
| `idx_moderation_cases_reported_by` | moderation_cases | `(reported_by)` | `000001:2114` |
| `idx_moderation_cases_resource` | moderation_cases | `(resource_type, resource_id)` | `000001:2115` |
| `idx_moderation_pending` | moderation_cases | `(status, created_at) WHERE status='pending'` | `000001:2116` |
| `idx_moderation_reporter` | moderation_cases | `(reported_by, created_at DESC)` | `000001:2117` |
| `idx_moderation_resource` | moderation_cases | `(resource_type, resource_id, created_at DESC)` | `000001:2118` |
| `idx_moderation_reviewer` | moderation_cases | `(reviewed_by) WHERE reviewed_by IS NOT NULL` | `000001:2119` |
| `idx_user_warnings_user_id_active` | user_warnings | `(user_id) WHERE is_active=true` | `000001:2265` |
| `idx_user_warnings_user_id_created` | user_warnings | `(user_id, created_at DESC)` | `000001:2266` |
| `idx_outbox_status` | outbox | `(status, next_attempt_at) WHERE status IN (pending,processing)` | `000001:2156` |
| `idx_audit_events_actor` | audit_events | `(actor_type, actor_id, created_at DESC)` | `000001:2016` |
| `idx_audit_events_entity` | audit_events | `(entity_type, entity_id, created_at DESC)` | `000001:2017` |
| `idx_audit_events_type` | audit_events | `(event_type, created_at DESC)` | `000001:2018` |
| `idx_admin_audit_logs_*` | admin_audit_logs | (4 index) | `000001:2002-2005` |

### E.4 FK

**FACT:** moderation tables:
- `user_warnings` FK: `issued_by → users`, `revoked_by → users`, `user_id → users` (`000001:2424-2426`).
- `audit_events` FK: `actor_id → users ON DELETE SET NULL` (`000001:2287`).
- `moderation_cases`: **TIDAK ADA FK** (reported_by, reviewed_by).
- `appeals`: **TIDAK ADA FK** (report_id, appealed_by, reviewed_by).

**INFERENCE:** tidak ada referential integrity untuk reported_by/reviewed_by/appealed_by/reviewed_by pada governance tables.

### E.5 Check

- `user_warnings_level_check` `level IN (info,warning,severe)` — `000001:2533`.
- `outbox_retry_count_check` `retry_count >= 0` — `000001:2494`.
- Tidak ada check pada `moderation_cases` / `appeals` (status appeals bebas text).

### E.6 Triggers / Functions

**FACT:** seluruh trigger/functions non-moderation:
- `enforce_permanent_selling_surface_exclusivity()` + `trg_for_sales_permanent_exclusivity`, `trg_auctions_permanent_exclusivity` (`000049:24-63`)
- `enforce_selling_surface_immutability()` + `trg_products_selling_surface_immutability` (`000049:67-83`)
- `enforce_single_for_sale_row_per_product()` + `trg_for_sales_single_row_per_product` (`000050:1-36`)
- `prevent_content_resource_occurrences_update()` + `trg_content_resource_occurrences_immutable` (`000039:68-77`)

**FACT:** `enforce_single_active_sale_channel_per_product()` (000010/000047) telah **diganti** oleh permanent exclusivity (000049) — tidak lagi ada di schema final. **Tidak ada trigger moderation-specific** (tidak ada trigger pada moderation_cases/appeals/user_warnings).

---

## F. GovernanceCase Forensic Map

### F.1 Apakah `moderation_cases` masih authority?

**FACT:** **YA — satu-satunya authority report+case+decision.** Producer: `ModerationService.CreateCase`/`ReviewCase` (`moderation_service.go:78-227`). Consumer: admin queue, mobile my-cases, appeal ownership check, worker enforcement.

### F.2 Seluruh kolom

**FACT:** `id, resource_type, resource_id, status, reported_by, reviewed_by, reason, decision_note, created_at, reviewed_at, updated_at, deleted_at` — `000001:963-976`.

### F.3 Seluruh FK

**FACT:** Tidak ada FK sama sekali pada `moderation_cases`.

### F.4 Seluruh indexes

**FACT:** `000001:2113-2119` (7 index, lihat E.3).

### F.5 Consumers / writers (visible dari schema + code)

| Peran | Artefak | Evidence |
|---|---|---|
| Writer (create) | `ModerationRepositoryImpl.Create` INSERT | `moderation_repository_impl.go:43-61` |
| Writer (decision) | `ModerationRepositoryImpl.Update` (overwrite status/reviewed_by/decision_note/reviewed_at) | `moderation_repository_impl.go:143-156` |
| Reader (admin) | `ListPending`, `ListWithStatus`, `ListByResource` | `moderation_repository_impl.go:167-308` |
| Reader (user) | `ListByReporter` | `moderation_repository_impl.go:222-246` |
| Enforcement consumer | `ModerationEventHandler.Handle` (payload case_id/resource_id) | `worker/moderation_event_handler.go:127-133, 153-224` |
| Appeal check | `appeal_repository_impl.go` (report_id = CaseID) | `appeal_repository_impl.go:38-55` |

### F.6 Status enum yang dipakai

**FACT:** `pending → approved/rejected/enforced` (Go entity `governance_case.go:46-51`). `removed` ghost di DB enum. `enforced` = decision+event proxy (bukan execution proof).

### F.7 Apakah `resource_type/resource_id` masih polymorphic authority?

**FACT:** **YA.** `moderation_cases.resource_type` (`moderation_resource_enum`) + `resource_id` (uuid) adalah referensi polimorfik tanpa FK ke tabel target mana pun. Validasi existence dilakukan aplikasi (`ResourceExists`, `moderation_repository_impl.go:313-364`).

### F.8 Apakah schema sudah memiliki bentuk baru sebagian?

**FACT:** **TIDAK.** Tidak ada tabel `reports`, `decisions`, `enforcements`. Tidak ada kolom case_id di manapun. Tidak ada partial unique active-case-per-subject. `000047` hanya mengubah vocabulary, bukan bentuk.

### F.9 Kesimpulan forensic

**INFERENCE:** `moderation_cases` adalah satu-satunya authority Report+Case+Decision yang **harus dibongkar total**. Tidak ada jalur untuk "menambal" tanpa menciptakan dua authority (kolom lama report vs tabel reports baru; status case vs decision rows).

---

## G. Report Persistence Map

### G.1 Ada persistence entity khusus Report?

**FACT:** **TIDAK ADA.** Tidak ada tabel `reports`; tidak ada struct moderation `Report` (`grep "reports"` di `backend/internal/governance/moderation` → hanya referensi komentar/DTO). Report = baris `moderation_cases`.

### G.2 Kandidat report tables

**FACT:** Satu-satunya kandidat adalah `moderation_cases` (mengandung `reported_by`, `reason`, `created_at` — field report). Klasifikasi:

- **canonical candidate:** TIDAK ADA.
- **duplicate candidate:** TIDAK ADA (tidak ada tabel lain dengan semantics report).
- **legacy/residue candidate:** `moderation_cases` (super-entity; report field bercampur case+decision).
- **unknown:** TIDAK ADA.

### G.3 Field report yang ada di `moderation_cases`

**FACT:** `reported_by` (tanpa FK), `reason` (free-text, `binding:"required,min=1,max=500"` di `moderation_service.go`), `created_at`. Tidak ada `reason_code`, tidak ada `evidence_snapshot`, tidak ada `reporter_id` bernama kanonik.

### G.4 Duplicate report rule saat ini

**FACT:** `idx_moderation_cases_one_report_per_user (reported_by, resource_type, resource_id)` (`000001:2113`) — reporter sama + subject sama dilarang selamanya (tidak dibatasi active). Canonical v1: larang duplicate **active** report per reporter+subject, izinkan reporter beda → report berbeda masuk case sama. **Skema saat ini tidak mendukung hal ini** (report = case, tanpa konsep active/terminal report).

### G.5 Self-report

**FACT:** Tidak dicegah di service/handler (Audit 1 §8). **Tidak ada kolom/constraint untuk mencegah.** (Keputusan bisnis: canonical merekomendasikan DENY — lihat Business Truth §6.)

---

## H. Case Persistence Map

### H.1 Apakah schema dapat mendukung Case dengan invariant "one active Case per subject"?

**FACT:** Tidak ada tabel `cases`. `moderation_cases` berfungsi sebagai case, tetapi:
- satu case = satu report (unique `(reported_by, resource_type, resource_id)`);
- status case mencampur workflow + decision + enforcement;
- tidak ada partial unique index active-case-per-subject.

### H.2 Apakah PostgreSQL partial unique index pattern dapat diterapkan pada current migration architecture?

**FACT:** **YA — pattern sudah terbukti dipakai** di codebase:
- `uniq_active_auction_per_product` (`000001:2015`)
- `uniq_active_for_sale_per_product` (`000047:191-193`)
- `idx_promotion_instances_active_target_unique` (`000001:2187`)

**INFERENCE:** partial unique index `(subject_type, subject_id) WHERE status IN (active states)` untuk tabel `cases` baru adalah **feasible** pada migration architecture saat ini (pgx executor mendukung multi-statement dalam satu tx, termasuk CREATE UNIQUE INDEX).

### H.3 Recommended columns (RECOMMENDATION — bukan FACT, tidak diimplementasikan)

```sql
cases (
  id            uuid PK,
  subject_type  moderation_target_enum NOT NULL,  -- content|comment|for_sale|auction|user (tanpa chat_message)
  subject_id    uuid NOT NULL,
  status        case_status_enum NOT NULL DEFAULT 'open',  -- open|resolved (design v1)
  created_at    timestamptz NOT NULL DEFAULT now(),
  closed_at     timestamptz,
  updated_at    timestamptz NOT NULL DEFAULT now()
)
```

### H.4 Recommended status representation (RECOMMENDATION)

**FACT (canonical design §8):** `open` / `resolved`. **TIDAK** ada `enforced`/`approved`/`rejected` sebagai case status. Decision dan Enforcement adalah entity terpisah.

### H.5 Required constraints (RECOMMENDATION)

- PK `cases(id)`
- Partial unique active case per subject:
  ```sql
  CREATE UNIQUE INDEX uniq_active_case_per_subject
    ON cases (subject_type, subject_id)
    WHERE status = 'open';
  ```
- FK `cases.subject_type` tidak dapat dibuat (polymorphic) — aplikasi validasi wajib (pattern `ResourceExists`).

### H.6 Required indexes (RECOMMENDATION)

- `(subject_type, subject_id, created_at DESC)` untuk history subject.
- `(status, created_at)` untuk queue admin.
- `(created_at DESC)` untuk list.

### H.7 Separasi FACT / RECOMMENDATION / UNKNOWN

- **FACT:** pattern partial unique feasible; tidak ada tabel cases; invariant tidak terpenuhi saat ini.
- **RECOMMENDATION:** kolom/status/constraint/index di atas (dari canonical design).
- **UNKNOWN:** apakah `open` cukup atau perlu `under_review` (design v1 menyebut `open|resolved`; detail status internal boleh disesuaikan implementation — Spec §4). Case reopening policy (Business Truth §41.1) — keputusan Owner.

---

## I. Decision Persistence Map

### I.1 Apakah ada persistence untuk Decision?

**FACT:** **TIDAK ADA** tabel `decisions`. Decision = field `status` (terminal) + `decision_note` + `reviewed_by` + `reviewed_at` pada `moderation_cases`, di-overwrite in-place (`moderation_repository_impl.go:143-156`). **Tidak append-only, tidak immutable.**

### I.2 Adakah table lain yang diam-diam menjalankan fungsi Decision?

**FACT:** `admin_audit_logs` menyimpan `action_type='moderation_action_applied'` dengan `metadata` (previous_status/new_status) — **best-effort, non-atomic** (`LogSafe`, `admin_audit_logger.go:88-90`). Ini bukan decision record; hanya log. **TIDAK ADA** table lain.

### I.3 Minimal relational requirements (RECOMMENDATION — canonical)

```text
decision_id    uuid PK
case_id        uuid NOT NULL FK → cases
decided_by     uuid NOT NULL FK → users (moderator/actor)
action         text/enum NOT NULL (controlled: no_action|remove|restore|suspend|warning)
outcome        text/enum (no_violation|violation_confirmed|reversed)  -- Spec §5
decision_note  text (free-form operator note)
created_at     timestamptz NOT NULL
```

**FACT (canonical):** Decision immutable, append-only; appeal menghasilkan Decision baru, bukan update Decision lama.

### I.4 Index yang dibutuhkan (RECOMMENDATION)

- `(case_id, created_at DESC)` untuk history per case.

---

## J. Enforcement Persistence Map

### J.1 Apakah ada persistence entity untuk Enforcement?

**FACT:** **TIDAK ADA** tabel `enforcements`. Enforcement saat ini = **outbox event** (`moderation.{type}.removed/suspended/hidden`) yang di-insert dalam tx yang sama dengan update case (`moderation_service.go:198-211`). `status='enforced'` adalah proxy "event sudah di-emit", bukan "target berubah".

### J.2 Outbox event ≠ enforcement record (verifikasi)

**FACT:** outbox event berisi payload `{case_id, resource_type, resource_id, decision_note}` (`moderation_service.go:351-366`) — **tidak ada** `enforcement_id`, `decision_id`, `status`, `attempt_count`, `started_at`, `finished_at`, `failure_reason`. Tidak ada write-back execution result. `MarkSucceeded` hanya menandai delivery outbox, bukan execution.

**INFERENCE:** outbox-as-enforcement = false-success (Audit 2 §1.3). Enforcement harus entity + table sendiri.

### J.3 Case status ≠ enforcement status (verifikasi)

**FACT:** `moderation_cases.status='enforced'` berarti decision+event, tidak membedakan `pending/processing/succeeded/failed`. Kegagalan worker (retry habis) tidak tercatat di mana pun (`outbox_worker.go` handleFailureInTx hanya log + status outbox failed, yang tidak pernah diproses ulang — Audit 2 §4.2).

### J.4 Schema yang diperlukan untuk membedakan pending/processing/succeeded/failed (RECOMMENDATION)

```sql
enforcements (
  id               uuid PK,
  decision_id      uuid NOT NULL FK → decisions,
  action           text/enum NOT NULL,     -- controlled consequence
  subject_type     moderation_target_enum NOT NULL,
  subject_id       uuid NOT NULL,
  status           enforcement_status_enum NOT NULL DEFAULT 'pending',
                   -- pending|processing|succeeded|failed|permanent_failure
  attempt_count    integer NOT NULL DEFAULT 0,
  requested_at     timestamptz NOT NULL DEFAULT now(),
  started_at       timestamptz,
  finished_at      timestamptz,
  last_error       text,
  next_attempt_at  timestamptz
)
```

### J.5 Retry / idempotency identity (RECOMMENDATION)

- **FACT (canonical §8.2):** idempotency anchor = `enforcement_id`; outbox event payload membawa `enforcement_id` + `decision_id` + subject + action.
- **FACT:** current outbox idempotency key `"<eventType>.<entityID>"` (`outbox_repository.go:99-100`) **collision-prone** untuk dua enforcement berbeda pada target sama (Audit 3 §8.2).
- **RECOMMENDATION:** outbox idempotency key `enforcement.<enforcement_id>`; retry path `failed → pending` harus diperbaiki (P1 Audit 2).

---

## K. Warning Persistence Map

### K.1 Apakah warning punya Decision provenance?

**FACT:** **TIDAK.** `user_warnings` tidak punya kolom `decision_id`, tidak ada FK ke governance mana pun. Field provenance hanya `issued_by` (admin UUID, FK users).

### K.2 Apakah warning dapat dibuat standalone?

**FACT:** **YA.** `POST /api/v1/admin/warnings` menerima `{user_id, level, reason, expires_at}` tanpa case/decision reference (`routes_core.go:920-922`; Audit 1 §10). **Melanggar canonical "Decision → Warning".**

### K.3 Apakah schema memungkinkan duplicate warning?

**FACT:** **YA.** Tidak ada unique constraint `(decision_id, user_id)` ataupun `(user_id, ...)`; `warning_repository_impl.go:38-63` INSERT polos tanpa dedup. Audit 1 §10: warning ganda untuk user yang sama diperbolehkan.

### K.4 Apakah revoke/expiry history tersedia?

**FACT:** Sebagian. `revoked_at`, `revoked_by`, `is_active`, `expires_at` ada (`000001:1772-1783`). Status computed on-read (`warning.go:122-133`). Expiry **tidak dipersist** (dihitung dari `expires_at` vs now). Revoke in-place (`is_active=false`). History row append-only untuk creation; revoke/expiry bukan append — hanya flag update.

### K.5 Warning = consequence record atau user flag?

**FACT:** **User flag + consequence record tanpa provenance.** Bisa dianggap consequence (level, reason, expiry), tetapi **tidak terhubung ke governance decision** — secara canonical tidak memenuhi syarat sebagai governance consequence.

### K.6 Canonical requirement

**FACT (canonical):** `Decision → Warning`; `warning.decision_id FK NOT NULL`; unique `(decision_id, user_id)` untuk anti-duplicate; lifecycle `active → revoked/expired`. Tidak ada standalone warning.

---

## L. Appeal Persistence Map

### L.1 Trace `appeals.report_id`

**FACT:** Kolom DB `appeals.report_id` menyimpan **CaseID**:
- INSERT: `appeal.CaseID` → kolom `report_id` (`appeal_repository_impl.go:38-55`)
- CTE duplicate check: `WHERE report_id = $1` dengan `appeal.CaseID` (`appeal_repository_impl.go:86-111`)
- SELECT/scan: `report_id` dibaca ke `CaseID` (`appeal_repository_impl.go:146-176, 198-229, 458-487`)

### L.2 Apakah ada FK?

**FACT:** **TIDAK ADA FK** dari `appeals.report_id` (atau kolom lain) ke tabel mana pun. `appeals` tidak muncul di daftar FK `000001:2278-2429`.

### L.3 Repository/entity naming mismatch?

**FACT:** **YA — terbukti.** Entity Go `Appeal.CaseID` (`entity/appeal.go:16`); DB kolom `report_id`; API request/response memakai `case_id` (`appeal_handler.go:55,141,551,601`). Tiga lapisan tiga nama: entity `CaseID`, DB `report_id`, wire `case_id`.

### L.4 Apakah schema dapat diubah menjadi `decision_id`?

**FACT (feasibility):** Kolom `appeals.report_id` adalah plain `uuid NOT NULL` tanpa FK. **Infrastructure-wise** dapat di-rename/di-drop dan diganti `decision_id uuid NOT NULL FK → decisions` pada migration baru. Tidak ada consumer DB lain yang mereferensikan `appeals.report_id` (tidak ada FK inbound). **Canonical requirement: Appeal → Decision**, bukan Report, bukan Case.

### L.5 Kondisi tambahan

**FACT:** Tidak ada unique constraint untuk pending appeal (guard hanya CTE). `deleted_at` tidak dipakai. `status` adalah `text` bebas (bukan enum).

---

## M. Governance Audit History Map

### M.1 Existing authority

**FACT:** Dua mekanisme:
1. `admin_audit_logs` — best-effort (`LogSafe`), dipakai moderation actions (`moderation_handler.go:698`, `appeal_handler.go:544`, `warning_handler.go:445,510`). **TIDAK reliable** (error di-ignore; ditulis di luar tx mutation via pool — `admin_audit_logger.go:77`).
2. `audit_events` — append-only, reliable bila dipanggil dalam tx (`AuditService.Emit`), **TIDAK dipakai moderation**.

### M.2 Append-only guarantee

**FACT:** `audit_events` append-only (repo hanya INSERT; tidak ada UPDATE/DELETE path — `audit_event_repository.go:51-82`). `admin_audit_logs` juga insert-only secara praktik (tidak ada update/delete path).

### M.3 FK availability

**FACT:** `audit_events.actor_id → users ON DELETE SET NULL` (`000001:2287`). `admin_audit_logs` tanpa FK.

### M.4 Apakah moderation mutations dapat direconstruct?

**FACT:** **TIDAK secara reliable.** `LogSafe` dapat gagal tanpa rollback (Audit 2 §13.2). Worker enforcement **tidak menulis audit sama sekali** (moderation_event_handler.go tidak import audit). Restoration (appeal approved) juga tanpa audit record. `admin_audit_logs` memfilter `SystemCallerID` (`admin_audit_logger.go:56-58`) sehingga worker tidak akan pernah tercatat di sana.

### M.5 Apakah current schema cukup?

**FACT:** **TIDAK cukup untuk canonical** (BT §28, Design §28): governance mutation harus tulis audit **dalam transaksi yang sama** dengan mutation. `audit_events` sudah menyediakan infra yang diperlukan (actor types termasuk `worker`), tapi moderation belum memakainya.

### M.6 Apakah diperlukan perubahan?

**INFERENCE:** Perlu (a) moderation menulis ke `audit_events` (atau governance history table baru) dalam tx yang sama dengan mutation; (b) enforcement write-back di-audit; (c) jangan membuat event-sourcing baru — append-only audit cukup.

---

## N. Outbox Boundary Map

### N.1 Current outbox table

**FACT:** `outbox` (`000001:1157-1169`) + `outbox_archive` (`000001:1171-1184`). Status enum 5 nilai (`000001:226-232`).

### N.2 Event identity

**FACT:** `id` (uuid), `event_type`, `aggregate_type`, `aggregate_id`, `payload jsonb`, `idempotency_key` (UNIQUE). Payload moderation saat ini: `{case_id, resource_type, resource_id, decision_note}` (`moderation_service.go:351-366`) — **tanpa enforcement_id/decision_id**.

### N.3 Status

**FACT:** `pending|processing|succeeded|failed|dead_letter`.

### N.4 Retry fields

**FACT:** `retry_count`, `next_attempt_at`. **Retry runtime-broken** (Audit 2 §4.2): `MarkProcessing` hanya menerima `pending` (`outbox_repository.go:265-287`), `FetchPendingBatch` mengambil `pending` + `failed` (`outbox_repository.go:221-258`), `ResetStuckEvents` hanya reset `processing` (`outbox_repository.go:391-418`). Event `failed` di-fetch → `MarkProcessing` gagal → worker skip forever.

### N.5 Dead-letter fields

**FACT:** `status='dead_letter'` (tidak ada kolom DLQ terpisah; `dead_letter` adalah status). `MoveToDeadLetter` (`outbox_repository.go:350-371`) **unreachable** untuk event yang sudah pernah gagal (karena retry path broken).

### N.6 Indexes

**FACT:** `outbox_idempotency_key_key` UNIQUE (`000001:1979`); `idx_outbox_status` partial `(status, next_attempt_at) WHERE status IN (pending,processing)` (`000001:2156`).

### N.7 Apakah dapat mereferensikan Enforcement?

**FACT:** Secara schema, `outbox.aggregate_id` + `payload jsonb` dapat membawa `enforcement_id`. **Tidak ada FK** dari outbox ke entity lain (by design — outbox tidak boleh memiliki FK yang mengunci lifecycle). **Infrastructure-wise feasible** untuk payload membawa `enforcement_id`.

### N.8 Apakah schema memungkinkan idempotent delivery?

**FACT:** **YA** — `idempotency_key UNIQUE` + `ON CONFLICT (idempotency_key) DO NOTHING` (`outbox_repository.go:111-117`). Namun key saat ini `eventType.entityID` rawan collision antar-enforcement berbeda (Audit 3 §8.2). **RECOMMENDATION:** `enforcement.<enforcement_id>`.

### N.9 Boundary verdict

**FACT:** Outbox adalah delivery infrastructure yang layak dipertahankan (dengan fix retry + key pattern baru). **Outbox bukan authority enforcement** — enforcement harus entity sendiri dengan write-back.

---

## O. Vocabulary Conflicts

### O.1 `fixed_price_sale`

- **FACT:** Bukan lagi active schema authority: tabel `fixed_price_sales` → `for_sales` (000047:165), enum `fixed_price_sale_status_enum` → `for_sale_status_enum` (000047:43-51), enum values `'fixed_price_sale'` → `'for_sale'` (000047:58-125). Test `migration_000047_schema_state_proof_test.go:34-43` membuktikan.
- **FACT:** **Application-only residue tetap ada:**
  - Admin: `apps/admin/src/types/moderation.ts:11,267` — `ResourceType` masih berisi `'fixed_price_sale'`; label `fixed_price_sale: 'Fixed-Price Sale'`. Juga di `CaseDetailModal.tsx`, `ModerationCasesPage.tsx`, hooks/tests (11 files, 18 occurrences).
  - Mobile: `apps/mobile/lib/domains/system/report/data/mappers/report_mapper.dart:85-87` — case `'fixed_price_sale'` → `forSale`. Juga 18 files, 52 occurrences di mobile (search/object/preview/order/promotion — mayoritas commerce, bukan moderation).
- **INFERENCE:** `fixed_price_sale` adalah **residue aplikasi** (admin moderation types + mobile report mapper) yang harus dihapus di slice cleanup. **Bukan** active schema authority.

### O.2 `for_sale`

- **FACT:** **Active schema authority** (tabel `for_sales`, enum `for_sale_status_enum`, `moderation_resource_enum='for_sale'`, `order_source_enum='for_sale'`). Canonical target name.

### O.3 `removed`

- **FACT:** Hanya ada sebagai **enum value ghost** di `moderation_status_enum` (`000001:189`). Tidak di code entity. **Bukan active schema authority** (tidak ditulis). **Bukan** moderation enforcement event type — event types `moderation.{type}.removed` (`moderation_service.go:379-388`) adalah string event, bukan enum value.
- **INFERENCE:** `removed` = historical migration residue (ghost enum) yang harus dihapus saat enum dibangun ulang.

### O.4 `enforced`

- **FACT:** **Active schema authority** (enum value `moderation_status_enum='enforced'` `000001:190`, ditulis `governance_case.go:50`). **Rejected semantics** — canonical melarang `enforced` sebagai case status. Harus dibongkar bersama `moderation_cases`.

### O.5 `report`

- **FACT:** **Bukan table** (tidak ada `reports`). Hanya konsep di code (komentar/entity name `GovernanceCase` menyebut REPORT; endpoint `/moderation/cases`). Wire `POST /moderation/cases` adalah intake report yang menamai output case (Audit 1 §8).
- **INFERENCE:** vocabulary `report` saat ini = application-only label pada flow case. Canonical memerlukan entity Report nyata.

### O.6 `case`

- **FACT:** **Active schema authority** (`moderation_cases`), tapi super-entity rejected. Canonical Case baru = tabel `cases` terpisah.

### O.7 Ringkasan klasifikasi

| String | Active schema authority? | Historical migration residue? | Application-only residue? |
|---|---|---|---|
| `fixed_price_sale` | Tidak (000047 menghapus) | Ya (000047 up/down menyimpan vocabulary lama) | **Ya (admin types, mobile mapper)** |
| `for_sale` | **Ya** | Tidak | Tidak |
| `removed` | Tidak (ghost enum value) | **Ya** (000001:189) | Tidak (kecuali string event `moderation.*.removed`) |
| `enforced` | **Ya (rejected)** | Ya (000001:190) | Ya (admin UI badge, mobile mapper) |
| `report` | Tidak | Tidak | Ya (label pada flow case) |
| `case` | Ya (super-entity rejected) | Ya | Ya |

---

## P. Migration Reset Assessment

### P.1 FACT — apa yang dihasilkan jika seluruh chain dijalankan dari zero

**FACT:** `cmd/migrate` (pgx, `pkg/migration/executor.go`) menerapkan 000001–000054 secara ascending. Hasil akhir:
- `for_sales` (bukan `fixed_price_sales`), `moderation_resource_enum` dengan `'for_sale'`, `moderation_cases` dengan resource_type for_sale;
- `listings` TIDAK ada (di-drop 000010);
- trigger permanent exclusivity (000049) + single row (000050);
- `chat_commerce_references` TIDAK ada (000054);
- seluruh moderation tables masih super-entity.

**FACT:** `000025` tidak punya down.sql — down-chain tidak lengkap untuk full rollback.

### P.2 CONFLICT — migration yang masih mengandung rejected design

**FACT:**
- `000001_canonical_schema.up.sql` — memuat `moderation_cases` super-entity (:963-976), `moderation_status_enum` dengan `removed`+`enforced` (:184-190), `moderation_resource_enum` dengan `chat_message` (:175-182), `appeals.report_id` (:456), `user_warnings` standalone (:1772-1783).
- `000047` up/down — mempertahankan vocabulary `fixed_price_sale` di enum `moderation_resource_enum` (down) dan menghapusnya (up); ini adalah **historical residue** yang akan obsolete setelah canonical reset.
- `000049/000050` — bukan moderation conflict (commerce rule 9), tidak perlu disentuh.

### P.3 RESIDUE — migration yang hanya menyimpan historical vocabulary/architecture

**FACT:** `000001` (bagian moderation) dan `000047` (bagian moderation_resource_enum + for_sale vocabulary transition) adalah satu-satunya tempat schema moderation hidup. Setelah canonical reset, keduanya menjadi historical residue untuk moderation (namun 000001 tetap menjadi baseline untuk seluruh domain lain — **tidak boleh dihapus**).

### P.4 REQUIRED RESET — apakah clean migration rewrite/reset diperlukan?

**FACT:** Karena Labuda from zero (tidak ada production data) dan canonical memerlukan schema yang sama sekali berbeda (reports/cases/decisions/enforcements baru; moderation_cases/appeals.report_id/user_warnings standalone dibongkar), **clean migration rewrite/reset diperlukan**.

**Dua opsi (RECOMMENDATION, keputusan ChatGPT/Owner):**
1. **Opsi A — new numbered migrations (preferred):** Biarkan 000001–000054 utuh sebagai history, tambahkan migration baru (mis. 000055+) yang:
   - membuat tabel canonical baru (reports, cases, decisions, enforcements, + alter user_warnings/appeals),
   - **belum** drop tabel lama (drop di migration cleanup terakhir setelah implementation proof, per design §44 Phase 12).
   Ini menjaga `migration_authority_guard` dan replay idempotency tetap valid.
2. **Opsi B — baseline rewrite:** Squash ulang (seperti yang dilakukan 000001 dari v100–v229) menjadi baseline baru tanpa moderation lama. Lebih agresif; menyentuh seluruh chain; **tidak disarankan** karena 000001 juga menjadi baseline non-moderation yang sudah stabil.

**INFERENCE:** Opsi A lebih aman dan sesuai doctrine (bounded, evidence-based, cleanup terpisah). Jangan membuat compatibility migration untuk mempertahankan `moderation_cases`.

---

## Q. Canonical Schema Gap

| Canonical entity | Current schema | Gap | Classification |
|---|---|---|---|
| Report | Tidak ada; field di `moderation_cases` | Perlu tabel `reports` baru | **CREATE** |
| Case | `moderation_cases` (super-entity) | Perlu tabel `cases` baru; `moderation_cases` dibongkar | **CREATE + REPLACE** |
| Decision | Field status/decision_note/reviewed_by di case (overwrite) | Perlu tabel `decisions` append-only | **CREATE** |
| Enforcement | Outbox event (tanpa status) | Perlu tabel `enforcements` + write-back | **CREATE** |
| Warning | `user_warnings` standalone | Tambah `decision_id FK`; hapus standalone path | **MODIFY + provenance** |
| Appeal | `appeals.report_id` = CaseID | Rename → `decision_id FK` | **MODIFY** |
| Governance audit | `admin_audit_logs` best-effort; `audit_events` tidak dipakai | Moderation menulis `audit_events` (atau governance history) dalam tx | **MODIFY** |
| Outbox | `outbox` table | Fix retry; payload enforcement_id; idempotency key baru | **MODIFY (infra)** |
| `moderation_resource_enum` | termasuk `chat_message` | Hapus `chat_message` (out of scope) | **MODIFY** |
| `moderation_status_enum` | `pending/approved/rejected/removed/enforced` | Dihapus bersama `moderation_cases` (case status baru: open/resolved) | **DROP + CREATE** |
| Target domain | contents/comments/for_sales/auctions/users | Enforcement via domain command; user perlu service method | **MODIFY (executor boundary)** |

**FACT:** Tidak ada gap yang menghalangi design: seluruh entity target sudah ada dengan authority service (content/comment/for_sale/auction), kecuali user enforcement yang masih lewat repository langsung.

---

## R. Required Destructive Changes

> Tidak diimplementasikan pada audit ini. Deletion dilakukan setelah canonical schema foundation disetujui (per instruction §20).

**FACT (rencana destruktif, dari design §40):**

1. **DROP/replace `moderation_cases`** (super-entity) + `idx_moderation_cases_*` indexes + `moderation_status_enum` (termasuk `removed`, `enforced`).
2. **DROP `appeals.report_id`** → ganti `decision_id` (rename atau drop/add kolom).
3. **MODIFY `user_warnings`** → tambah `decision_id` NOT NULL FK; hapus kemampuan standalone.
4. **DROP `chat_message` dari `moderation_resource_enum`** (recreate enum tanpa chat_message, atau gunakan enum baru `moderation_target_enum`).
5. **DELETE zombie:** `DomainAction` + `DomainActionWorker` + `AppealReversalService` (kode PARKED; tidak ada tabel `domain_actions` yang perlu di-drop).
6. **MODIFY `outbox`:** fix `MarkProcessing` retry path; idempotency key pattern `enforcement.<id>`.
7. **DELETE residue vocabulary:** `fixed_price_sale` di admin types (`moderation.ts:11,267`), mobile report mapper (`report_mapper.dart:85-87`).
8. **MODIFY executor boundary:** user enforcement lewat `UserService` method (bukan repo langsung); auction moderation lewat auction-domain-owned command (menggantikan `CancelForModeration`).
9. **MODIFY audit:** moderation mutation menulis audit dalam tx yang sama (pakai `audit_events` atau governance history).

---

## S. Risks

| Risiko | Severity | Catatan | Evidence |
|---|---|---|---|
| Outbox retry broken (`failed` tidak pernah retry; dead_letter unreachable) | **P1** | Blocker enforcement reliability; harus fix di slice foundation/outbox | `outbox_repository.go:231,265-287` vs `outbox_worker.go:447-449` |
| False-success enforcement (`enforced` = event emitted) | **P1** | Enforcement entity + write-back wajib | `moderation_service.go:198-211` |
| Auction `CancelForModeration` cancel auction ber-bid tanpa guard | **P1** | Commerce boundary; harus diganti auction-domain command | `auction_service.go:1289-1320` (Audit 2 §9) |
| Restoration tanpa provenance (content/comment) | **P1** | Appeal dapat restore non-moderation deletion | `content_service.go:769-799` (Audit 2 §11.4) |
| Governance audit best-effort (LogSafe) | **P1** | History bisa hilang; canonical memerlukan audit dalam tx | `admin_audit_logger.go:88-90` |
| Dead consumers restored events (for_sale/auction/user) | **P2** | Wiring ilusi restoration | Audit 2 §11.3 |
| `chat_message` target masih ada di enum + mobile UI | **P2** | Harus dihapus dari canonical scope | `000001:181`; `moderation_resource_type.go:37` |
| `moderation_cases` FK-less (reported_by/reviewed_by) | **P2** | Referential integrity lemah; canonical Report/Case harus FK | `000001:963-976` |
| Migration reset risk (000001 adalah baseline non-moderation) | **P2** | Jangan rewrite 000001; tambah migration baru | migrations/README.md:43-58 |
| Warning duplicate / appeal duplicate tanpa DB constraint | **P2** | Canonical butuh unique constraints | `warning_repository_impl.go:38-63`; `appeal_repository_impl.go:86-111` |
| `000025` tanpa down.sql | **P3** | Down-chain tidak lengkap | `backend/migrations/` listing |

---

## T. Unknowns

1. **Case reopening policy** (Business Truth §41.1): apakah case terminal boleh dibuka kembali? Rekomendasi canonical: tidak; case baru setelah terminal. **Keputusan Owner.**
2. **Report terhadap sold/ended object** (BT §41.2): canonical cenderung izinkan. **Keputusan Owner** — memengaruhi apakah `ResourceExists` perlu guard terminal state.
3. **Auction ber-bid moderation stop** (BT §41.3): canonical rekomendasi YA dengan auction-domain handling bidder consequence. **Keputusan Owner.**
4. **Warning repeat/cap policy** (BT §41.4): rekomendasi no cap v1. **Keputusan Owner.**
5. **Reason taxonomy final** (BT §41.5): daftar `reason_code` belum final — memengaruhi kolom `reports.reason_code` enum/check.
6. **Evidence retention & snapshot fields** (BT §41.6): field `evidence_snapshot` exact belum ditentukan.
7. **Appeal eligibility scope** (BT §41.7): semua decision atau hanya enforcement decision.
8. **User suspension × subscription/listing/order interaction** (design §13): boundary user domain.
9. **Status internal Case** (Spec §4): apakah `open`/`resolved` cukup atau perlu `under_review` — technical detail, boleh disesuaikan saat implementation design.
10. **Governance audit storage**: pakai `audit_events` existing atau buat `governance_audit_history` terpisah — technical decision (design §28 menyebut keduanya).

---

## U. Recommended Implementation Slice 1

> Ini rekomendasi — **tidak diimplementasikan pada audit ini**.

### U.1 Scope

**Schema/Foundation** (Design §44 Phase 1). Bounded: hanya migration baru + schema canonical + proof migration replay. **Tidak** menyentuh service/handler/worker/admin/mobile pada slice ini.

### U.2 Deliverables

1. **Migration baru `000055_*` (dan seterusnya, berurutan):**
   - `CREATE TABLE reports` (id, reporter_id FK users, subject_type, subject_id, reason_code, reason_note, evidence_snapshot jsonb, case_id FK nullable, created_at; partial unique `(reporter_id, subject_type, subject_id) WHERE case_id IS NOT NULL` atau active-report flag).
   - `CREATE TABLE cases` (id, subject_type, subject_id, status open|resolved, created_at, closed_at; **partial unique active case per subject**).
   - `CREATE TABLE decisions` (id, case_id FK, decided_by FK users, outcome, action, decision_note, created_at; index `(case_id, created_at DESC)`; append-only — no UPDATE path di repository).
   - `CREATE TABLE enforcements` (id, decision_id FK, action, subject_type, subject_id, status pending|processing|succeeded|failed|permanent_failure, attempt_count, timestamps, last_error, next_attempt_at; index `(status, next_attempt_at)`).
   - `ALTER TABLE user_warnings ADD COLUMN decision_id uuid` (+ FK, + unique `(decision_id, user_id)`), nullable di migration ini, diisi oleh slice Warning berikutnya; atau drop/recreate di slice Warning.
   - `ALTER TABLE appeals` rename/replace `report_id` → `decision_id` FK.
   - Enum baru: `moderation_target_enum` (content|comment|for_sale|auction|user) — **tanpa** chat_message; `case_status_enum` (open|resolved); `enforcement_status_enum`.
   - **TIDAK drop** `moderation_cases` pada slice ini (drop di Phase 12 destructive cleanup, setelah replacement proof).
2. **Outbox retry fix** (bisa slice tersendiri atau termasuk foundation karena enforcement write-back bergantung padanya): `MarkProcessing` menerima `failed` (atau jalur requeue), sehingga retry + dead_letter benar-benar berjalan.
3. **Outbox idempotency key pattern** untuk enforcement: `enforcement.<enforcement_id>`.
4. **Migration replay proof** + negative proof (moderation_cases lama tetap ada sampai cleanup; vocabulary `for_sale` tidak regresi).

### U.3 Tidak termasuk (slice berikutnya)

Report → Case correlation (Phase 2-3), Decision transaction (Phase 4), Enforcement worker write-back (Phase 5-6), target executors (Phase 7), Warning (Phase 8), Appeal (Phase 9), Admin (Phase 10), Mobile (Phase 11), destructive cleanup (Phase 12).

---

## V. Acceptance Criteria

Schema foundation Slice 1 dianggap selesai bila:

1. **FACT:** Tabel `reports`, `cases`, `decisions`, `enforcements` ada setelah migration replay dari zero.
2. **FACT:** Partial unique index active-case-per-subject terbukti (insert case kedua untuk subject sama dengan status open → gagal 23505).
3. **FACT:** `decisions` append-only (tidak ada UPDATE path; history terlihat via `(case_id, created_at)`).
4. **FACT:** `enforcements` memiliki status lifecycle + attempt_count + idempotency anchor.
5. **FACT:** `user_warnings` memiliki `decision_id` provenance (nullable sementara atau wajib di slice Warning).
6. **FACT:** `appeals` menunjuk `decision_id`, bukan `report_id`/CaseID.
7. **FACT:** Enum canonical: `moderation_target_enum` tanpa `chat_message`; tidak ada `removed`/`enforced` pada case status.
8. **FACT:** Migration replay dari zero sukses (`go run ./cmd/migrate` pada fresh DB), seluruh 000001–000054 + migration baru berjalan.
9. **FACT:** `moderation_cases` lama **belum di-drop** (cleanup menunggu replacement proof) — tidak ada data loss.
10. **FACT:** Outbox retry path (`failed → processing`) terbukti (unit/integration proof).
11. **FACT:** Tidak ada `fixed_price_sale` baru di schema; `for_sale` tetap canonical.
12. **FACT:** Tidak ada perubahan pada service/handler/worker/admin/mobile/test di luar migration schema (kecuali proof yang menyertainya).

---

## FINAL RECOMMENDATION

```text
AUDIT STATUS: PASS (schema foundation audit selesai; implikasi bersifat design-fakta, bukan blocker audit)
```

### Jawaban atas pertanyaan final

1. **Apakah schema foundation siap langsung diimplementasikan?**
   **YA, secara teknis.** Seluruh gap terpetakan (Q), pattern partial unique terbukti feasible (H.2), target domain sudah punya authority service. **Syarat:** keputusan Owner pada business ambiguity T.1–T.8 (terutama case reopening, report sold/ended, auction ber-bid, warning policy) tidak memblokir schema — namun beberapa memengaruhi nilai enum/constraint (reason_code, evidence_snapshot). Keputusan minimal yang dibutuhkan sebelum Slice 1: **Case status set** (open/resolved vs +under_review) dan **reason taxonomy draft**.

2. **Apakah migration reset/rewrite diperlukan?**
   **YA — clean new migrations diperlukan** (Opsi A: migration baru berurutan; `moderation_cases` di-drop di Phase 12). **TIDAK** diperlukan rewrite 000001 (baseline non-moderation stabil). Compatibility migration **tidak** diusulkan.

3. **Entity apa saja yang harus dibuat?**
   `reports`, `cases`, `decisions`, `enforcements` (+ enum baru `moderation_target_enum`, `case_status_enum`, `enforcement_status_enum`). Warning/Appeal dimodifikasi, bukan dibuat baru (kecuali keputusan drop-recreate).

4. **Entity apa saja yang harus dihapus?**
   `moderation_cases` (super-entity) + `moderation_status_enum` (termasuk `removed`, `enforced`) — di Phase 12. `chat_message` dari moderation target enum. Zombie code `DomainAction`/`DomainActionWorker`/`AppealReversalService` (tidak ada tabel). `fixed_price_sale` residue di admin/mobile types.

5. **Constraint apa yang wajib ada?**
   - Partial unique active case per subject: `UNIQUE (subject_type, subject_id) WHERE status = 'open'`.
   - Report duplicate: `UNIQUE (reporter_id, subject_type, subject_id)` untuk active report.
   - FK: `reports.reporter_id → users`, `cases` subject (app-level, polymorphic), `decisions.case_id → cases`, `decisions.decided_by → users`, `enforcements.decision_id → decisions`, `warnings.decision_id → decisions` + `UNIQUE (decision_id, user_id)`, `appeals.decision_id → decisions`.
   - Outbox `idempotency_key UNIQUE` (existing) dengan key pattern `enforcement.<enforcement_id>`.

6. **Apakah ada business ambiguity yang benar-benar menghalangi schema?**
   **TIDAK menghalangi schema inti.** Entity boundary Report/Case/Decision/Enforcement, cardinality, dan constraint utama sudah locked. Ambiguity yang tersisa (T.1–T.8) memengaruhi **detail field/enum** (reason_code, evidence_snapshot, case status internal) dan **executor policy** (auction/user boundary) — bukan struktur. Namun **wajib** diselesaikan sebelum slice yang menyentuh entity tersebut (Reason taxonomy sebelum Report; Case status sebelum Case; Auction/User boundary sebelum Phase 7).

7. **Apa implementation slice berikutnya?**
   **Slice 1 = Schema/Foundation** (U.2): migration baru canonical tables + enums + constraints + partial unique; outbox retry fix; idempotency key pattern; migration replay + negative proof. Setelah itu: Report intake (Phase 2) → Case correlation (Phase 3) → Decision (Phase 4) → Enforcement + write-back (Phase 5-6) → target executors (Phase 7) → Warning (Phase 8) → Appeal (Phase 9) → Admin (Phase 10) → Mobile (Phase 11) → Destructive cleanup (Phase 12) → Regression (Phase 13).

---

## Lampiran — Dokumen yang dibaca

| Dokumen | Path | Status |
|---|---|---|
| Aturan kerja | `/cara-kerja-updated.md` | Dibaca penuh |
| PRD | `/PRD.md` | Dibaca (referensi; bukan authority final) |
| Canonical Business Truth | `/LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1.md` | Dibaca penuh |
| Canonical Design | `/LABUDA — CANONICAL MODERATION DESIGN v1.md` | Dibaca penuh |
| Canonical Specification | `/LABUDA — CANONICAL MODERATION SPECIFICATION v1.md` | Dibaca penuh |
| Audit 1 | `docs/audits/moderation/REPORT_CASE_AUDIT_1_FACTUAL.md` | Dibaca (diverifikasi ulang) |
| Audit 2 | `docs/audits/moderation/REPORT_CASE_AUDIT_2_ENFORCEMENT_BOUNDARY.md` | Dibaca (diverifikasi ulang) |
| Audit 3 | `docs/audits/moderation/REPORT_CASE_AUDIT_3_CANONICAL_DESIGN.md` | Dibaca (diverifikasi ulang) |
| Migrations | `backend/migrations/000001–000054` + `README.md` | Dibaca langsung (line-by-line untuk moderation) |
| Migration runner | `backend/pkg/migration/executor.go`, `backend/cmd/migrate/main.go` | Dibaca |
| Governance moderation code | `backend/internal/governance/moderation/**` (entity/repo/service/handler) | Dibaca |
| Worker | `backend/internal/worker/moderation_event_handler.go`, `outbox_worker.go` | Dibaca |
| Audit infra | `backend/internal/audit/admin_audit_logger.go`, `backend/internal/governance/audit/**` | Dibaca |
| Outbox repo | `backend/internal/platform/outbox/infrastructure/repository/outbox_repository.go` | Dibaca |
| Admin/mobile residue | `apps/admin/src/types/moderation.ts`, `apps/mobile/lib/domains/system/report/data/mappers/report_mapper.dart` | Dibaca (spot-check) |
