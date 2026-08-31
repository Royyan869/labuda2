# LABUDA — AUDIT 2: MODERATION DECISION / ENFORCEMENT / OUTBOX / COMMERCE BOUNDARY

- **Tanggal audit:** 2026-08-30
- **Mode:** AUDIT ONLY — tidak ada implementasi, perubahan schema, migration, test, admin, mobile, atau commit
- **Satu-satunya artefak baru:** laporan ini
- **Input:** temuan Audit 1 (`REPORT_CASE_AUDIT_1_FACTUAL.md`) — diperlakukan sebagai **klaim yang harus diverifikasi ulang**, bukan authority final

---

## 1. Executive Summary

Audit 2 membuktikan boundary Decision → Enforcement → Outbox → Worker → Target Domain secara end-to-end. Temuan utama:

### 1.1 Outbox retry runtime-broken — P1 (dikonfirmasi, dengan jalur presisi baru)

Kombinasi tiga metode repository membuktikan **event outbox berstatus `failed` tidak akan pernah diproses ulang**:

1. `FetchPendingBatch` mengambil `status IN ('pending','failed')` — `outbox_repository.go:231,238`
2. `MarkProcessing` hanya meng-update `WHERE status = 'pending'` — `outbox_repository.go:271-277`
3. Saat event `failed` di-fetch, `MarkProcessing` mengembalikan `ErrInvalidStatusTransition` → worker menganggap "race" → **skip** — `outbox_worker.go:446-449`

Konsekuensi: `MarkFailedWithRetry` (yang menulis status `failed` + retry_count + next_attempt_at, `outbox_repository.go:321-344`) menciptakan status yang **tidak dapat di-retry**. `ResetStuckEvents` hanya menyentuh status `processing` (`outbox_repository.go:391-418`), bukan `failed`. Tidak ada jalur lain yang mengembalikan `failed` → `pending`. **Temuan Audit 1 terkonfirmasi dengan jalur code exact.**

### 1.2 Header "STANDBY" outbox_worker.go adalah residue kontradiktif — worker AKTIF

`outbox_worker.go:1-6` menyatakan worker "intentionally disabled pending business validation", tapi runtime membuktikan worker di-start:
- `dependencies.go:1250-1253` mendaftarkan `outboxWorker.Start()` ke `workerStartups`
- `StartWorkers` dipanggil di `main.go:143` dan menjalankan semua startup
- `main.go:520-521` memanggil `OutboxWorker.Stop()` saat shutdown

Header STANDBY adalah komentar usang yang kontradiktif dengan runtime. **P2 observability/documentation.**

### 1.3 False-success enforcement — sistem berhenti di level B/C, bukan D/E

Current system menganggap enforcement "berhasil" pada saat **outbox intent di-persist dalam transaksi yang sama dengan decision** (level B), bukan setelah target mutation berhasil (level D) atau terverifikasi (level E). Case `enforced` di-set dalam tx yang sama dengan event insert — tidak ada penulisan enforcement result kembali. **Dikonfirmasi.**

### 1.4 Commerce boundary — For Sale aman, Auction berisiko

- **For Sale `Withdraw`**: aman untuk commerce — hanya mengubah `status`/`visibility` for_sale + invalidasi shipping quotes + emit `for_sale.withdrawn`. Tidak menyentuh order/payment/ledger/escrow. PROVEN.
- **Auction `CancelForModeration`**: membatalkan auction **dengan bid aktif** tanpa guard bid/winner. `CanCancel()` (guard bid) hanya dipakai jalur seller `Cancel`, **bukan** `CancelForModeration`. Tidak ada OrderID guard, tidak ada refund/void path, tidak ada notifikasi ke bidder. Status `cancelled` dengan `auction_bids`/`current_bid`/`current_winner_id` tersisa. **P1 commerce boundary.**

### 1.5 Auction PARKED conflict — komentar obsolete, runtime AKTIF

Komentar `dependencies.go:2320-2322` ("PARKED no-op") obsolete. Runtime: `SetupModerationHandlers` men-register `moderation.auction.removed` → `ModerationEventHandler.handleAuctionRemoved` → `CancelForModeration` → `Auction.Cancel()`. Test `TestModerationHandler_AuctionRemoved_Success` membuktikan. **PROVEN reachable.**

### 1.6 Dead consumers untuk restored events

`moderation.for_sale.restored`, `moderation.auction.restored`, `moderation.user.restored` ter-register sebagai handler (enforcement + notification + promotion) tapi **tidak ada producer** — `AppealService` hanya emit content/comment restore (`isAutoRestorableType`). Handler adalah dead consumer; docs & wiring menciptakan ilusi restoration support yang tidak ada.

**AUDIT STATUS: PROVEN**

---

## 2. Decision Persistence

### 2.1 Decision current representation

Decision tidak memiliki entity/table sendiri. Representasi = field pada `moderation_cases`:

| Aspek | Implementasi | Evidence |
|---|---|---|
| Actor | `reviewed_by` (admin UUID) | `governance_case.go:187` |
| Case | `id` (case row) | `moderation_cases` |
| Action | `status` transition (`approved`/`rejected`/`enforced`) | `governance_case.go:46-60` |
| Reason/note | `decision_note` | `governance_case.go:188` |
| Timestamp | `reviewed_at` | `governance_case.go:189` |
| Status transition | `pending → {approved,rejected,enforced}` via `transitionTo` | `governance_case.go:166-192` |
| Transaction | `WithTx` — satu tx: lock → transition → (event) → update | `moderation_service.go:173-220` |
| Row locking | `GetForUpdate` (FOR UPDATE) | `moderation_repository_impl.go:105-130` |
| Persistence | `repo.Update` (overwrite fields) | `moderation_repository_impl.go:143-156` |
| Audit trail | `LogSafe("moderation_action_applied", ...)` | `moderation_handler.go:698-708` |
| Event creation | enforce → `InsertEvent` outbox (tx sama) | `moderation_service.go:199-211` |

### 2.2 Jawaban pertanyaan 3.1–3.5

**3.1 — Satu decision dapat direpresentasikan immutable/historical?**
**TIDAK.** Decision adalah mutasi in-place pada case row. Tidak ada tabel decision, tidak ada append, tidak ada versioning. Satu case hanya menyimpan decision terakhir (dan hanya ada satu decision karena terminal state). **PROVEN.**

**3.2 — Jika case sudah diputuskan, apakah keputusan dapat direvisi?**
**TIDAK melalui API.** `transitionTo` menolak non-pending (`ErrAlreadyReviewed`, `governance_case.go:168-173`). Tidak ada endpoint re-review. **PROVEN.**
(Reversal satu-satunya jalur adalah Appeal → restoration event — lihat §11.)

**3.3 — Jika dapat direvisi, apakah decision lama tetap dapat diketahui?**
Tidak relevan — tidak dapat direvisi. Namun jika dilihat dari audit `admin_audit_logs`: `LogSafe` mencatat `previous_status`/`new_status` — **best-effort**, bisa hilang (lihat §13). **PROVEN.**

**3.4 — Jika tidak dapat direvisi, apakah model tetap membutuhkan Decision entity/history?**
Faktual current: **tidak ada** decision entity. Namun canonical spec §5 mewajibkan decision historical; tanpa history, riwayat keputusan hanya bisa direkonstruksi parsial dari `admin_audit_logs` (best-effort) — ini **design implication**, bukan keputusan audit.

**3.5 — Apakah approved/rejected/enforced merupakan decision types atau execution states?**

Analisis semantik:
- `approved` — **decision type murni** (konten dianggap patuh; tidak ada execution). PROVEN: `Approve` tidak emit event.
- `rejected` — **decision type murni** (false positive; tidak ada execution). PROVEN: `Reject` tidak emit event.
- `enforced` — **campuran decision + enforcement trigger**: status berarti "decision enforce dibuat DAN event enforcement di-emit". Bukan bukti execution sukses. **PROVEN** — `ShouldEmitEnforcementEvents` = status `enforced`; tidak ada tracking execution.

Kesimpulan: `enforced` adalah **decision type yang melekatkan asumsi enforcement trigger**, dan tidak membedakan enforcement outcome. Ini adalah **design implication** untuk canonical (Decision terpisah dari Enforcement lifecycle).

---

## 3. Enforcement Execution (per target)

### 3.1 Content

```text
decision enforce → moderation_service.ReviewCase → InsertEvent("moderation.content.removed", ResourceID, payload{case_id,resource_type,resource_id,decision_note})
→ outbox row (idempotency_key="moderation.content.removed.<contentID>")
→ ModerationEventHandler.handleContentRemoved
→ ContentService.SoftDeleteForModeration (content_service.go:729-758)
→ content.Delete() → status=deleted, deleted_at=now (content.go:76-95)
→ contentRepo.Update
```

- Payload: `{case_id, resource_type, resource_id, decision_note}` (`moderation_service.go:351-366`)
- Event identifier: outbox `id` (random UUID) + `idempotency_key` (`eventType.resourceID`)

### 3.2 Comment

```text
→ InsertEvent("moderation.comment.removed", commentID, ...)
→ handleCommentRemoved → CommentService.SoftDeleteForModeration (comment_service.go:627-652)
→ SoftDelete: deleted_at=now (bukan status)
```

### 3.3 For Sale

```text
→ InsertEvent("moderation.for_sale.removed", forSaleID, ...)
→ handleForSaleRemoved → ForSaleService.Withdraw (for_sale_service.go:358-396)
→ MarkWithdrawn() → status=withdrawn, withdrawn_at=now (for_sale.go:161-173)
→ UpdateStatus + InvalidateQuotesByProduct (shipping) + InsertEvent("for_sale.withdrawn")
```

### 3.4 Auction

```text
→ InsertEvent("moderation.auction.removed", auctionID, ...)
→ handleAuctionRemoved → AuctionService.CancelForModeration (auction_service.go:1289-1320)
→ GetForUpdate → auction.Cancel() (auction.go:476-483)
→ UpdateTx + InsertEvent("auction.cancelled")
```

### 3.5 User

```text
→ InsertEvent("moderation.user.suspended", userID, ...)
→ handleUserAction → userRepo.GetByIDForUpdate → set AccountStatus="suspended" → userRepo.Update (moderation_event_handler.go:603-666)
```

- **Catatan:** user enforcement memakai repository langsung, bukan service method (user domain tidak punya `Suspend()` method; `User` entity hanya field string). Masih dalam batas user domain authority (repo), tapi melewati service layer. **P2/P3 design note.**

### 3.6 Chat message

```text
→ InsertEvent("moderation.chat_message.hidden", messageID, ...)
→ handleChatMessageHidden → chatService.SoftHideForModeration (chat_service.go:1174)
→ deleted_at + deletion_reason + moderation_key + room.updated projection
```

---

## 4. Outbox Lifecycle

### 4.1 Status machine aktual

```text
pending ──MarkProcessing──> processing ──MarkSucceeded──> succeeded
   │                             │
   │ (worker fetch)              └──MarkFailedWithRetry──> failed
   │                                                       │
   │                                                       └──MoveToDeadLetter──> dead_letter
```

- `FetchPendingBatch`: `status IN ('pending','failed') AND next_attempt_at <= NOW()`, `FOR UPDATE SKIP LOCKED` (`outbox_repository.go:221-258`)
- `MarkProcessing`: `WHERE status='pending'` only (`outbox_repository.go:271-277`)
- `MarkFailedWithRetry`: set status failed + retry_count + next_attempt_at (`outbox_repository.go:321-344`)
- `ResetStuckEvents`: reset `processing` → `pending` if stale (`outbox_repository.go:391-418`)
- `MoveToDeadLetter`: `failed` (or any) → `dead_letter` (`outbox_repository.go:350-371`)

### 4.2 P1 — Dead retry path (konfirmasi Audit 1 dengan jalur presisi)

1. Worker fetch event `failed` (karena FetchPendingBatch menyertakannya)
2. `processEventInTx` → `MarkProcessing` → SQL `WHERE id=$3 AND status='pending'` → **0 rows** → `ErrInvalidStatusTransition`
3. Worker: `errors.Is(err, ErrInvalidStatusTransition)` → return `("", 0, nil)` = **skipped** (`outbox_worker.go:447-449`)
4. Disposition kosong → bukan succeeded/failed/dead_letter → counter 0 (`outbox_worker.go:410-420`)
5. Row tetap `failed` → di-fetch lagi tiap poll → skip lagi → **tidak pernah diproses ulang**

`ResetStuckEvents` hanya reset `processing`, tidak pernah `failed`. Tidak ada mekanisme lain yang menormalkan `failed` → `pending`.

**Konklusi:** setiap enforcement event yang gagal sekali (`handler error` → `MarkFailedWithRetry`) **tidak akan pernah di-retry**. Ini adalah **P1** — core moderation enforcement reliability rusak, dan enforcement false-success diperparah (case `enforced` tetap, event `failed` selamanya).

### 4.3 Outbox schema

```sql
CREATE TABLE outbox (
    id uuid PK, aggregate_type text, aggregate_id uuid, event_type text,
    payload jsonb, status outbox_status_enum DEFAULT 'pending',
    retry_count integer DEFAULT 0, next_attempt_at timestamptz,
    idempotency_key text NOT NULL UNIQUE, created_at, updated_at
)
```
(`migrations/000001:1157-1169`, unique `outbox_idempotency_key_key` :1979)

**Tidak ada kolom `last_error`** — `handleFailureInTx` mencatat error hanya di log (`outbox_worker.go:501-509`). Error message hilang setelah retry. **P2 observability.**

### 4.4 Outbox worker registration

- Di-start: `dependencies.go:1250-1253` via `workerStartups`; `StartWorkers` di `main.go:143`.
- Handler moderation: `SetupModerationHandlers` (`outbox_worker.go:881-925`) — enforcement + notification fanout untuk semua moderation events.
- Env gate: `MODERATION_EVENT_HANDLER` default **ON** (`dependencies.go:2326`, test `moderation_event_handler_gate_test.go`).

### 4.5 Duplicate outbox repository (competing interface)

`internal/platform/outbox/repository/outbox_repository.go` = interface-only (147 baris, `OutboxRepository interface`), tidak di-import oleh file non-test manapun. Implementasi aktif di `internal/platform/outbox/infrastructure/repository`. Interface lama adalah **dead residue**. **P3.**

---

## 5. Failure / Retry

### 5.1 Perilaku failure handler moderation

| Failure | Perilaku handler | Retry? | Evidence |
|---|---|---|---|
| nil service (content/comment/forSale/user/auction) | log warn, return nil | Tidak | `moderation_event_handler.go:315-318, 358-361, 496-500, 557-562, 615-620` |
| malformed payload | log, return nil | Tidak | `moderation_event_handler.go:181-193` |
| invalid resource_id | log, return nil | Tidak | `moderation_event_handler.go:187-194` |
| target not found (content/comment) | return nil (idempotent) | Tidak | `content_service.go:736-740` |
| target terminal state (for_sale/auction) | `InvalidTransitionError` → treated success | Tidak | `moderation_event_handler.go:508-520, 569-579` |
| target DB error (transient) | return error | **YA (harusnya)** | `moderation_event_handler.go:331-333` |
| restore sold for_sale | `isNonRetryableRestoreError` → return nil | Tidak | `moderation_event_handler.go:704-711` |

### 5.2 Retry infrastructure vs reality

- **Intent:** worker punya exponential backoff (`calculateBackoff`), max 20 attempts, dead letter. **Design solid.**
- **Reality:** jalur retry broken karena `MarkProcessing` tidak menerima `failed` (§4.2). Event yang seharusnya retry akan **skip forever**.
- `dead_letter` tercapai hanya jika... `failed` → worker skip sebelum `handleFailureInTx` dipanggil lagi. `newRetryCount >= maxAttempts` **tidak akan pernah dievaluasi ulang** karena event `failed` tidak pernah sampai `handleFailureInTx`. **Dead letter unreachable untuk moderation events setelah gagal pertama.** Ini berarti `MaxOutboxAttempts`/`MoveToDeadLetter` adalah **dead code path** untuk events yang sudah pernah gagal.

### 5.3 P1 tambahan — enforcement failure tidak terlihat

- Tidak ada field enforcement status/result di `moderation_cases`.
- Worker tidak menulis ke `admin_audit_logs` maupun `audit_events` (tidak ada import audit di `moderation_event_handler.go`).
- Satu-satunya visibility: log `h.log.Error` + outbox row `failed` (yang tidak akan pernah move on).

---

## 6. Idempotency

### 6.1 Per-target repeated-event safety

| Target | Idempotency basis | Bukti | Status |
|---|---|---|---|
| Content | status guard (`StatusDeleted` → nil) + `GetForUpdate` | `content_service.go:735-750` | PROVEN |
| Comment | `deleted_at != nil` guard | `comment_service.go:633-648` | PROVEN |
| For Sale | `InvalidTransitionError` → treated success (withdrawn/sold) | `moderation_event_handler.go:508-520` | PROVEN |
| Auction | `InvalidTransitionError` → treated success (terminal) | `moderation_event_handler.go:569-579` | PROVEN |
| User | `AccountStatus=="suspended"` guard + `GetByIDForUpdate` | `moderation_event_handler.go:624-647` | PROVEN |
| Chat msg | `deleted_at IS NULL` guard | repo `SoftHideForModeration` | PROVEN |

**Semua enforcement target idempotent terhadap repeated event.** PROVEN.

### 6.2 Namun — idempotency menyembunyikan masalah

Karena repeated event dianggap sukses tanpa verifikasi state, idempotency **memperkuat false-success**: jika target tidak pernah benar-benar berubah (mis. event pertama malformed, target tetap), event retry yang sama akan tetap "sukses" lewat guard, bukan memperbaiki. Ini design implication, bukan bug tambahan.

---

## 7. Target Domain Authority

| Target | Authority state | Moderation path | Direct DB? | Domain service? | Verdict |
|---|---|---|---|---|---|
| Content | `ContentService` (soft-delete/restore) | via outbox → service method | Tidak | **YA** | Moderation decide → Content domain executes. **PROPER** |
| Comment | `CommentService` (soft-delete/restore) | via outbox → service method | Tidak | **YA** | **PROPER** |
| For Sale | `ForSaleService.Withdraw/RestoreFromModeration` | via outbox → service method | Tidak | **YA** | **PROPER** (namun lihat §8 side effect) |
| Auction | `AuctionService.CancelForModeration` | via outbox → service method | Tidak | **YA** (gov bypass) | **PROPER secara arsitektur**, tapi guard commerce tidak memadai (§9) |
| User | `UserRepository` (field `account_status`) | via outbox → **repository langsung** | Tidak (via repo) | **TIDAK** (tanpa service method) | **BOUNDARY WEAK** — tidak lewat service lifecycle; tidak ada validasi status transition di entity |
| Chat msg | `ChatService.SoftHideForModeration` | via outbox → service method | Tidak | **YA** | **PROPER** |

**Kesimpulan:** moderation memutuskan, target domain mengeksekusi — kecuali User yang memakai repository langsung tanpa service/entity validation. Prinsip canonical "Moderation decides → Target domain executes" **terpenuhi secara arsitektur** untuk content/comment/for_sale/auction/chat_message, **lemah** untuk user.

---

## 8. For Sale Commerce Boundary

### 8.1 Trace `ForSaleService.Withdraw` (dipanggil moderation)

`for_sale_service.go:358-396`:
1. `GetForUpdate` for_sale
2. `MarkWithdrawn()` — status only (`for_sale.go:161-173`)
3. `UpdateStatus`
4. **`InvalidateQuotesByProduct`** — invalidasi shipping quotes untuk product
5. `InsertEvent("for_sale.withdrawn")`

### 8.2 Side-effect audit

| Pertanyaan | Jawaban | Evidence |
|---|---|---|
| Kapan moderation memanggil? | `handleForSaleRemoved` via outbox | `moderation_event_handler.go:484-538` |
| Hanya visibility/lifecycle? | YA — status → withdrawn | `for_sale.go:161-173` |
| Shipping quote berubah? | **YA** — `InvalidateQuotesByProduct` | `for_sale_service.go:377-381` |
| Order berubah? | **TIDAK** | tidak ada order mutation |
| Checkout/payment berubah? | **TIDAK** | tidak ada |
| Seller capability berubah? | **TIDAK** | tidak ada |
| Buyer/order state berubah? | **TIDAK** | tidak ada |
| Ledger/financial berubah? | **TIDAK** | tidak ada |
| Existing purchase/order tetap valid? | **YA** — sold → withdrawn invalid; order yang ada tidak disentuh | — |

**Verdict: PROVEN AMAN.** Moderation terhadap For Sale tidak menjadi commerce settlement authority. Satu-satunya side-effect non-listing adalah invalidasi shipping quotes (desain wajar — listing withdrawn → quote usang). `for_sale.withdrawn` dikonsumsi promotion handler (pause promosi) — bukan commerce mutation.

---

## 9. Auction Commerce Boundary

### 9.1 Trace `CancelForModeration` (`auction_service.go:1289-1320`)

1. `GetForUpdate` auction
2. `auction.Cancel()` — `canTransition(status → cancelled)` only (`auction.go:476-483`)
3. `UpdateTx`
4. `InsertEvent("auction.cancelled")`

### 9.2 Guard perbandingan jalur cancel

| Jalur | Bid guard | OrderID guard | Ownership |
|---|---|---|---|
| Seller `Cancel` (`auction_service.go:628-670`) | **YA** — `CanCancel()`: active + bid → reject | — | IsSeller |
| `AdminCancel` (`auction_service.go:1418-1454`) | **TIDAK** (bypass, didokumentasi) | **YA** — `applyAdminCancel` OrderID guard (`:1359-1365`) | — |
| `CancelForModeration` (`:1289-1320`) | **TIDAK** | **TIDAK** | — |

`CanCancel()` (`auction.go:615-624`): active → hanya jika `CurrentBid == nil`.
**`CancelForModeration` TIDAK memanggil `CanCancel()`** — auction ACTIVE dengan bid bisa di-cancel.

### 9.3 State yang tersisa setelah cancel

`Cancel()` hanya mengubah `status → cancelled`. Yang TIDAK disentuh:
- `auction_bids` rows — tetap tersimpan
- `current_bid` — tetap
- `current_winner_id` — tetap (jika ada)
- Order — tidak ada (OrderID hanya set saat transisi ke `ended`; untuk status yang bisa di-cancel, OrderID nil)

### 9.4 Apakah ada refund/void/notification untuk bidder?

- **Refund/void: TIDAK ADA.** Tidak ada kode refund/void bid di `CancelForModeration` maupun consumer `auction.cancelled`.
- **Notification bidder: TIDAK ADA.** `auction.cancelled` hanya dikonsumsi promotion handler (`outbox_worker.go:959`). Tidak ada notification handler untuk bidder.
- **Bidder experience:** bid dipersist, auction jadi `cancelled` (public lifecycle "unavailable"), tidak ada info ke bidder. Jika bidder adalah `current_winner_id`, state "menang" palsu tersisa di row.

### 9.5 Jawaban: apakah moderation cancel dapat meninggalkan state inconsistent?

**YA — potensial, exact path:**

```text
Auction active + bids exist
→ admin enforce (decision)
→ moderation.auction.removed
→ CancelForModeration (tanpa CanCancel, tanpa OrderID guard)
→ status=cancelled
→ auction_bids + current_bid + current_winner_id tetap tersimpan
→ tidak ada refund, tidak ada void, tidak ada notifikasi bidder
→ outbox "auction.cancelled" hanya untuk promotion
```

Auction dengan **active bid + current_winner** (waiting_settlement) yang di-cancel meninggalkan **winner-determined state tanpa settlement path**. Tidak ada order, tapi bidder "menang" di data. **P1 commerce boundary.** (Catatan: `AdminCancel` didokumentasi dengan konfirmasi bidder safety rationale — `:1397-1401` — tapi `CancelForModeration` tidak mewarisi guard tersebut.)

---

## 10. Auction PARKED Conflict

| Pertanyaan | Jawaban | Evidence |
|---|---|---|
| 1. Handler reachable? | **YA** | `handleAuctionRemoved` memanggil `CancelForModeration` (`moderation_event_handler.go:564-566`) |
| 2. Siapa producer event? | `ModerationService.ReviewCase` — `moderation.auction.removed` | `moderation_service.go:199-211, 379-388` |
| 3. Siapa proses event? | `ModerationEventHandler` (via outbox worker) | `moderation_event_handler.go:547-595` |
| 4. Production daftarkan handler? | **YA** — `SetupModerationHandlers` register fanout `moderation.auction.removed` (`outbox_worker.go:908-921`), dipanggil di `dependencies.go:2327` |
| 5. Test membuktikan runtime path? | **YA** — `TestModerationHandler_AuctionRemoved_Success` (`moderation_event_handler_test.go:233-247`) memanggil `CancelForModeration` |
| 6. Komentar serverboot obsolete? | **YA** — `dependencies.go:2320-2322` klaim "PARKED no-op" bertentangan dengan kode handler & test | CONFLICT resolved |

**Verdict: komentar serverboot adalah OBSOLETE residue; runtime auction enforcement AKTIF dan memanggil `CancelForModeration` tanpa guard bid/OrderID.**

---

## 11. Appeal / Reversal

### 11.1 Flow appeal approval

```text
Appeal pending → AdminReviewAppeal → AppealService.ReviewAppeal (appeal_service.go:368-447)
→ jika approved:
   → kase.Status == enforced && isAutoRestorableType(kase.ResourceType)
        (content/comment ONLY)
   → InsertEvent("moderation.{type}.restored", ...) SEBELUM appeal status update
→ appeal.Approve → Update
```

### 11.2 Per-target reversal

| Target | Auto-restore? | Event | Executor | Target-state guard |
|---|---|---|---|---|
| Content | **YA** | `moderation.content.restored` | `ContentService.RestoreFromModeration` | deleted_at/status guard; **TIDAK membedakan origin delete** |
| Comment | **YA** | `moderation.comment.restored` | `CommentService.RestoreFromModeration` | deleted_at guard |
| For Sale | **TIDAK** (record-only) | — (tidak pernah di-emit) | — | — |
| Auction | **TIDAK** (intentionally unsupported) | `handleAuctionRestored` = no-op | — | — |
| User | **TIDAK** (record-only) | — (tidak pernah di-emit) | — | banned guard hanya di handler yang tidak pernah dipanggil |

### 11.3 Temuan kritis — dead consumers

`moderation.for_sale.restored`, `moderation.auction.restored`, `moderation.user.restored` **ter-register** sebagai handler (enforcement + notification fanout, `outbox_worker.go:916-918`) dan punya notification handler (`notification_worker_moderation.go:218-259, 287-311`), TAPI `AppealService` **tidak pernah meng-emit** event tersebut (`isAutoRestorableType` = content/comment only, `appeal_service.go:266-268`).

**Dead consumer** — wiring dan UI (admin notification, promotion resume) mengasumsikan restoration yang tidak pernah terjadi. **P1/P2 design mismatch.**

### 11.4 Apakah restoration inverse enforcement yang aman?

**Content — TIDAK aman secara provenance:** `RestoreFromModeration` set `status=active` + `deleted_at=nil` tanpa membedakan apakah content di-delete oleh moderation atau oleh jalur lain (entity tidak punya `deleted_by`/`deletion_reason`; `Delete()` tidak mencatat actor). Jika author self-delete content, lalu appeal lama/keliru di-approve → content **dihidupkan kembali** padahal bukan deletion moderation. **P1 reversal correctness.**

**Comment — sama:** restore berdasarkan `deleted_at != nil`; tidak ada provenance.

**For Sale — aman:** tidak ada auto-restore.

**Auction — aman (no-op).**

**User — handler `handleUserRestored` punya banned guard (`moderation_event_handler.go:808-818`)** — TAPI event tidak pernah di-produce → guard tidak pernah dijalankan.

### 11.5 Jika target state berubah setelah enforcement?

- Content dihapus user setelah enforce: restore tetap berhasil (menghidupkan kembali content yang sengaja dihapus owner).
- For Sale sold setelah enforce: restore ditolak (`MarkActiveFromModeration` sold → error; `isNonRetryableRestoreError` → no-op). **PROVEN guard aman** (`repost_governance_test.go:75-83`).
- Auction ended setelah enforce: restore no-op. Aman.
- Banned user: restore ditolak (handler guard) — tapi unreachable.

---

## 12. Warning

| Aspek | Fakta | Evidence |
|---|---|---|
| Siapa issue | Admin (handler `AdminIssueWarning`) | `warning_handler.go:382-454` |
| Endpoint | `POST /api/v1/admin/warnings` | `routes_core.go:920-922` |
| Permission | `moderation.case.resolve` | `routes_core.go:921` |
| Service | `WarningService.IssueWarning` | `warning_service.go:59-111` |
| Persistence | `user_warnings` insert | `warning_repository_impl.go:39` |
| Expiry | `expires_at` optional; status computed on-read (`GetStatus()`) | `warning.go:122-133` |
| Revoke | `DELETE /api/v1/admin/warnings/:id/revoke` → `RevokeWarning` + `GetForUpdate` | `warning_handler.go:459-518`, `warning_service.go:158-183` |
| Provenance | **TIDAK ADA** relasi ke case/decision — warning berdiri sendiri | schema `user_warnings` tidak punya FK ke moderation |
| Dibuat tanpa case/decision? | **YA** — request hanya `{user_id, level, reason, expires_at}` | `warning_handler.go:52-57` |
| Dua kali untuk incident sama? | **YA** — tidak ada unique constraint / dedup | `warning_repository_impl.go:39` (insert polos), tidak ada DB unique |
| Immutable history | Sebagian — row append-only; revoke in-place (`is_active=false`) | `warning.go:109-120` |

**Verdict:** warning boundary TIDAK terikat governance context. Melanggar canonical spec §9 (Decision → Warning provenance). **P1 canonical compliance / P2 operasional.**

---

## 13. Audit Trail

### 13.1 Coverage matrix

| Mutation | Mekanisme | Atomic? | Failure behavior |
|---|---|---|---|
| Case action (approve/reject/enforce) | `LogSafe("moderation_action_applied")` | **TIDAK** — best-effort | gagal → mutation tetap sukses |
| Appeal review | `LogSafe("appeal_reviewed")` | **TIDAK** | gagal → review tetap sukses |
| Warning issue | `LogSafe("admin_warning_issued")` | **TIDAK** | gagal → warning tetap dibuat |
| Warning revoke | `LogSafe("admin_warning_revoked")` | **TIDAK** | gagal → revoke tetap sukses |
| Evidence read | `LogTx("moderation.evidence.read")` | **YA** — rollback jika gagal | gagal → read error |
| Enforcement (worker) | **TIDAK ADA** — tidak menulis audit sama sekali | — | — |
| Appeal restoration | **TIDAK ADA** | — | — |

Evidence:
- `moderation_handler.go:698` — LogSafe
- `appeal_handler.go:544` — LogSafe
- `warning_handler.go:445,510` — LogSafe
- `moderation_handler.go:569` — LogTx (evidence)
- `admin_audit_logger.go:88-90` — LogSafe = `_ = l.Log(...)` ignore error

### 13.2 Jawaban pertanyaan kunci

> Apakah moderation mutation dapat berhasil walaupun audit record tidak berhasil dibuat?

**YA, PROVEN.** Semua mutation moderation (case action, appeal review, warning issue/revoke) memakai `LogSafe` yang secara eksplisit meng-ignore error. Transaction boundary: mutation (tx) selesai commit, audit ditulis **di luar** transaksi via pool (`admin_audit_logger.go:77`). Tidak ada rollback. `admin_audit_logs` juga memfilter `SystemCallerID` — worker enforcement tidak akan pernah tercatat.

### 13.3 `audit_events` — infra terpisah yang tidak dipakai moderation

`audit_events` (tabel) + `AuditService.Emit` tersedia (`governance/audit/...`), dipakai order service, **TIDAK dipakai moderation sama sekali**. Moderation hanya pakai `admin_audit_logs` (best-effort).

---

## 14. Zombie Architecture

| Komponen | Status | Producer | Consumer | Runtime registration | DB support | Tests | References |
|---|---|---|---|---|---|---|---|
| `DomainAction` entity | **PARKED/DEAD** | tidak ada | `DomainActionWorker` (PARKED) | tidak ada | tidak ada migration `domain_actions` | entity test (`domain_action_test.go`) | `outbox_event_registry.go:198-203` |
| `DomainActionWorker` | **PARKED/DEAD** | — | — | tidak di-instantiate | — | — | `domain_action_worker.go` |
| `DomainActionRepository` | **PARKED/DEAD** | — | — | — | SQL INSERT `domain_actions` (tabel tidak ada) | — | `domain_action_repository_impl.go:66` |
| `AppealReversalService` | **PARKED/DEAD** | — | — | tidak di-instantiate | — | — | `appeal_reversal_service.go:1` |
| Event stubs (`for_sale.visibility.apply`, `auction.pause.apply`, `domain_action.executed`, `appeal.reversed`) | **PARKED** | — | — | `NoHandlerAuditOnly` | — | registry test | `outbox_event_registry.go:190-236` |
| `platform/outbox/repository` interface-only | **DEAD** | — | — | tidak di-import non-test | — | — | grep kosong |

**Klasifikasi:**
- `DomainAction`/`DomainActionWorker`/`DomainActionRepository`/`AppealReversalService`: **PARKED** (dengan seluruh kode lengkap; berisiko resurrection sebagai competing authority).
- Interface `platform/outbox/repository`: **DEAD** (tidak ada consumer non-test).

---

## 15. Current Authority Matrix

| Concern | Current authority | Current problem | Evidence | Design implication |
|---|---|---|---|---|
| Report | `moderation_cases.reported_by` | Report = case; tidak ada entity report; tidak ada grouping | `governance_case.go:30-41` | Canonical perlu Report entity terpisah dari Case |
| Case | `moderation_cases` | 1 report = 1 case; tidak ada agregasi multi-report | unique index `(reported_by, resource_type, resource_id)` | Perlu relasi Case → banyak Report |
| Decision | `moderation_cases.status + decision_note` | Tidak historical; overwrite; tidak ada decision entity | `governance_case.go:43-60,166-192` | Perlu Decision entity append-only |
| Enforcement | outbox event (tidak ada status enforcement) | Tidak ada execution state/result; false-success | §4 | Perlu Enforcement lifecycle (pending/succeeded/failed) |
| Outbox | `outbox` table + `OutboxWorker` | **Retry broken** (`failed` tak bisa di-mark processing); `dead_letter` unreachable | §4.2 | Perlu fix `MarkProcessing` menerima `failed` (atau jalur retry terpisah) |
| Warning | `user_warnings` | Tidak ada provenance ke decision/case; bisa tanpa governance context; duplikat bebas | §12 | Perlu relasi Warning → Decision |
| Appeal | `appeals` (refer ke Case via kolom `report_id`) | Naming mismatch `report_id` = CaseID; tidak ada FK; menunjuk Case bukan Decision | `appeal_repository_impl.go` | Perlu Appeal → Decision + FK; rename kolom |
| Content enforcement | `ContentService` | Restore tanpa provenance delete-origin | §11.4 | Perlu provenance deletion actor |
| Comment enforcement | `CommentService` | Restore tanpa provenance | §11.4 | Sama |
| For Sale enforcement | `ForSaleService.Withdraw` | Aman untuk commerce; restore guard sold baik | §8 | Pertahankan |
| Auction enforcement | `AuctionService.CancelForModeration` | Cancel auction ber-bid tanpa guard/refund/notifikasi; komentar PARKED obsolete | §9-10 | Perlu keputusan bisnis: apakah moderasi boleh cancel bid-auction; jika ya perlu boundary bidder notification |
| User enforcement | `UserRepository` langsung | Tidak lewat service method; tanpa validasi entity | §3.5, §7 | Perlu user domain service method untuk lifecycle |
| Audit trail | `admin_audit_logs` (best-effort) + `audit_events` (tidak dipakai moderation) | Mutation sukses tanpa audit; enforcement tanpa audit | §13 | Perlu atomic audit untuk moderation decision |

---

## 16. P0 / P1 / P2 / P3

### P0
Tidak ditemukan P0 (tidak ada bukti kehilangan/korupsi uang langsung pada audit ini).

### P1
1. **Outbox retry broken** — event `failed` tidak pernah diproses ulang; enforcement failure permanen tidak terlihat. (`outbox_repository.go:231,277` + `outbox_worker.go:447-449`)
2. **Dead letter unreachable** untuk event yang pernah gagal — `MoveToDeadLetter` tidak akan pernah dieksekusi untuk `failed` events.
3. **Auction cancel ber-bid tanpa guard** — `CancelForModeration` membatalkan auction aktif ber-bid tanpa refund/void/notifikasi bidder. (`auction_service.go:1289-1320` vs `CanCancel()` di jalur seller)
4. **Restoration tanpa provenance** — content/comment yang di-delete bukan oleh moderation dapat di-restore via appeal. (`content_service.go:769-799`; entity tanpa `deleted_by`)
5. **Appeal record-only tidak pernah restore** — `for_sale/auction/user.restored` ter-register tapi tidak pernah di-emit (dead consumers); appeal approval untuk target tersebut tidak menghasilkan apa pun selain status appeal. (`appeal_service.go:266-268,409`)
6. **Enforcement tanpa audit trail** — worker enforcement tidak menulis `admin_audit_logs`/`audit_events`.

### P2
1. Header `STANDBY` outbox_worker.go kontradiktif dengan runtime (worker aktif).
2. `admin_audit_logs` best-effort untuk semua mutation moderation (LogSafe).
3. `outbox.last_error` tidak ada — error message hilang.
4. User enforcement tanpa service method.
5. Duplicate outbox interface (`platform/outbox/repository`).
6. Komentar serverboot auction PARKED obsolete (membingungkan operator).

### P3
1. Vocabulary residue (dari Audit 1): `fixed_price_sale` di admin/mobile, `appeals.report_id`, `ReportStatus.underReview`.
2. `moderation_status_enum.removed` ghost.

---

## 17. Design Implications

Berikut konsekuensi teknis dari fakta (bukan implementasi):

1. **Enforcement perlu lifecycle terpisah dari Decision** — current tidak punya status execution; `status='enforced'` bukan bukti target berubah.
2. **Retry path perlu jalur dari `failed` → processing** — current `MarkProcessing` hanya menerima `pending`, membuat seluruh retry infra tidak berfungsi.
3. **Enforcement result perlu dicatat kembali ke persistence** — tidak ada cara membedakan "decision + intent" vs "target berubah".
4. **Decision perlu append-only** — history tidak dapat direkonstruksi dari case row.
5. **Appeal perlu menunjuk Decision, bukan Case** — saat ini menunjuk case (kolom salah nama) dan tidak dapat menunjuk keputusan spesifik.
6. **Warning perlu provenance** — tanpa relasi ke decision, warning tidak dapat diaudit konteksnya.
7. **Auction moderation cancel perlu batas commerce** — apakah boleh membatalkan bid-auction adalah keputusan bisnis; jika ya, boundary bidder settlement harus eksplisit.
8. **Restoration perlu membedakan asal deletion** — tanpa provenance, restore dapat menghidupkan kembali konten yang seharusnya mati.
9. **Zombie DomainAction/AppealReversalService harus di-resolve** (dihapus atau di-resurrect dengan desain jelas) — saat ini competing authority potensial.

---

## 18. Remaining Business Decisions

1. Apakah moderasi boleh membatalkan auction yang memiliki bid aktif? (saat ini: YA secara runtime — tanpa guard)
2. Apakah perlu refund/void bid + notifikasi bidder ketika moderation cancel auction?
3. Apakah appeal approval untuk for_sale/auction/user HARUS menghasilkan restoration, atau record-only memang keputusan produk?
4. Apakah `chat_message` tetap menjadi target moderation (canonical spec tidak memasukkan)?
5. Apakah warning wajib terikat decision (canonical §9) atau independen diperbolehkan?
6. Bagaimana penanganan content yang di-delete user lalu di-restore appeal — perlukah provenance delete?

---

## 19. Recommended Design Audit / Next Scope

1. **Design Decision/Enforcement/Outbox retry** — fix `MarkProcessing` acceptance (failed → processing), enforcement status lifecycle, result write-back.
2. **Commerce boundary auction** — keputusan bisnis + design integration (refund/void/notifikasi bidder atau larangan cancel ber-bid).
3. **Restoration provenance** — desain deletion actor tracking.
4. **Appeal → Decision re-point** + schema rename (`appeals.report_id`).
5. **Warning provenance** — relasi decision.
6. **Resolve zombie** — DomainAction/AppealReversalService + duplicate outbox interface.

---

```text
AUDIT STATUS: PROVEN

DECISION AUTHORITY:
- Tidak ada entity Decision. Decision = field status/decision_note/reviewed_by pada moderation_cases
  (overwrite, non-historical, tidak dapat direvisi). Tidak dapat membedakan "decision" dari "execution trigger".

ENFORCEMENT AUTHORITY:
- Enforcement = outbox event + ModerationEventHandler. Tidak ada persisted execution state/result.
- Outbox retry runtime-broken: event berstatus failed tidak pernah diproses ulang (MarkProcessing hanya
  menerima pending; FetchPendingBatch mengambil failed; ResetStuckEvents hanya reset processing).
- Dead letter unreachable untuk events yang pernah gagal.
- Worker enforcement tidak menulis audit trail.

CRITICAL P1:
1. Outbox retry broken (failed events skip forever) — enforcement reliability rusak.
2. False-success: status='enforced' berarti "event di-emit", bukan "target berubah".
3. Auction CancelForModeration membatalkan auction ber-bid tanpa guard/refund/notifikasi bidder.
4. Restoration content/comment tanpa provenance delete-origin — dapat menghidupkan konten salah.
5. for_sale/auction/user restored handlers ter-register tanpa producer (dead consumers).
6. Enforcement tanpa audit trail.

FUNDAMENTAL DESIGN FINDINGS:
- Decision dan Enforcement harus menjadi lifecycle terpisah (current: satu status 'enforced' melekatkan keduanya).
- Outbox retry machine tidak pernah berfungsi untuk failed events — seluruh jalur retry/dead-letter adalah
  dead code path untuk moderation enforcement.
- Auction moderation cancel adalah jalur commerce-berisiko yang tidak didokumentasi di serverboot
  (komentar PARKED obsolete).
- Restoration (appeal) membutuhkan provenance deletion untuk menjadi inverse enforcement yang aman.
- Warning dan Appeal tidak memiliki relasi governance yang benar (warning tanpa decision, appeal menunjuk case).

BUSINESS DECISIONS REQUIRED:
- Auction ber-bid: boleh di-cancel moderation? Jika ya, boundary bidder (refund/void/notifikasi) apa?
- Record-only appeal (for_sale/auction/user): konfirmasi keputusan produk atau perlu restoration nyata?
- chat_message: pertahankan sebagai target atau matikan?
- Warning: wajib terikat decision?
- Content restore: perlu provenance delete actor?

RECOMMENDED NEXT SCOPE:
- Design audit: Decision vs Enforcement lifecycle + outbox retry fix.
- Design audit: auction commerce boundary (bidder state).
- Design audit: restoration provenance.
- Keputusan bisnis: auction ber-bid, record-only appeal, chat_message, warning provenance.
```
