# PRESENCE-REALTIME-CONTRACT-AUDIT

Read-only audit. Filesystem = factual truth. Git = backup only.
Nol implementasi, nol purge. Output: contract + implementation gap + SATU next slice.

---

## 1. VERDICT

`PRESENCE_REALTIME_CONTRACT_AUDIT_COMPLETE`

Audit berhasil memetakan contract canonical, membuktikan gap implementasi secara faktual,
dan menetapkan satu slice berikutnya. Dua keputusan Owner masih diperlukan (§16).

Tidak memakai `CLOSED GREEN` (ini audit-only).

---

## 2. OWNER DECISIONS ALREADY LOCKED

| # | Keputusan | Konsekuensi audit |
|---|---|---|
| 1 | Presence = **REQUIRED product capability** | Artifact Presence tidak lagi boleh diklasifikasikan OBSOLETE secara global |
| 2 | Presence state authority = **Redis presence lease/state** | `internal/presence/redis_repository.go` = authority state |
| 3 | Realtime transport = **WebSocket** | WAJIB lewat `/api/v1/ws` + `realtime.Hub` |
| 4 | Cross-instance fan-out = **Redis Pub/Sub** | `presence:v1:events` + `PresenceSubscriber` = canonical fan-out |
| 5 | Persisted historical last-seen = **PostgreSQL** | `user_presence.last_seen_at` = durable store |
| 6 | Realtime event = **`presence.changed`** | String event type terkunci |
| 7 | REST `/users/presence` **BUKAN** canonical | Stack REST mobile = OBSOLETE (purge tertunda) |

---

## 3. CANONICAL ARCHITECTURE MAP

Legenda: `✅ ada` · `⚠️ ada tapi tidak terhubung` · `❌ tidak ada`

```
[CLIENT]  Flutter
   │  WS: Authorization: Bearer <labuda access JWT>
   ▼
GET /api/v1/ws                     ✅ routes_core.go:109
   │  LabudaAuthMiddleware         ✅ routes_core.go:107  → c.Set("userID")
   │  UserLookupMiddleware         ✅ routes_core.go:108
   ▼
realtime.Handler.HandleWebSocket   ✅ websocket_handler.go:57
   │  → NewConnection(userID)      ✅ connection.go:62   (ID = uuid, per-session)
   │  → hub.Register(conn)         ✅ hub.go
   │  → go WritePump()             ✅ connection.go (heartbeat 54s, IsAlive eviction)
   │  → ReadPump()                 ✅ connection.go (read deadline 60s, ping/pong, sub/unsub)
   ▼
[X] PRESENCE LEASE ACQUIRE         ❌ TIDAK ADA HOOK    ← GAP #1
   ▼
presence.Service.ResumeLease       ⚠️ ada, 0 production caller
   ▼
presence.RedisRepository (Lua)     ⚠️ atomic, multi-session capable, 0 production caller
   │  presence:v1:leases:{user}  (ZSET member=connectionID)
   │  presence:v1:state:{user}   (HASH online,version)
   │  presence:v1:deadlines      (ZSET)
   ▼
[RENEW]  keepalive 54s             ❌ TIDAK ADA HOOK    ← GAP #2
   ▼
[RELEASE] conn.Close()             ❌ TIDAK ADA HOOK    ← GAP #3
   ▼
[SWEEP] claim+expire               ❌ tidak ada loop    ← GAP #4
   │  Service.ClaimDueUsers/SweepUser ⚠️ ada, 0 caller
   ▼
[TRANSITION → presence.changed]
   │  Service.PublishChanged        ⚠️ ada, 0 production caller
   ▼
Redis PUBLISH presence:v1:events   ⚠️ satu-satunya Pub/Sub di backend
   ▼
realtime.PresenceSubscriber        ⚠️ ada + test cross-instance, TIDAK PERNAH di-Start ← GAP #5
   │  → ResolveVisibleStatesForTarget (block/lifecycle/privacy) ✅
   │  → hub.GetUserConnections(target)                         ✅
   │  → marshalPresenceChanged                                 ⚠️ BUKAN WSEnvelope ← GAP #6
   ▼
WS frame ke watcher
   ▼
[MOBILE] parse + dispatch          ❌ MessageType 'presence' ≠ 'presence.changed'   ← GAP #7

[INITIAL STATE]  "apakah user X online?"
   │  Service.BuildSnapshot        ⚠️ ada, 0 production caller
   │  transport (REST atau WS snapshot)                        ❌ TIDAK ADA        ← GAP #8
   ▼
[LAST-SEEN]
   │  transisi online→offline → PersistLastSeen
   │  DBRepository.UpsertLastSeen  ✅ SQL benar
   │  trigger eksternal                                        ❌ TIDAK ADA        ← GAP #9
```

---

## 4. WS LIFECYCLE AUDIT

Mounting: `cmd/core_server/routes_core.go:105-110`
→ `wsGroup := router.Group("/api/v1")` + `LabudaAuthMiddleware` + `UserLookupMiddleware` + `GET /ws`.

| Pertanyaan | Jawaban faktual | Evidence |
|---|---|---|
| Siapa menerima connection? | `realtime.Handler.HandleWebSocket` | `websocket_handler.go:57` |
| Siapa authenticated user ID? | `LabudaAuthMiddleware` → `c.Get("userID")` (uuid.UUID) | `routes_core.go:107`; `websocket_handler.go:69-86` |
| Multiple WS per user? | **YA** — didukung & memang dipakai | `hub.GetUserConnections` (`hub.go`); `BroadcastToUserFiltered` sengaja kirim ke semua device |
| Connection ID? | `uuid.New().String()` per koneksi | `connection.go:68` |
| Kapan alive? | Read deadline 60s direset tiap read sukses | `connection.go:110,128` |
| Heartbeat? | **2 arah**: server→client `heartbeat` 54s; client→server `ping`→`pong`; mobile ping 30s | `connection.go` WritePump ticker + `marshalWSHeartbeat`; `connection.go` `case "ping"`; `websocket_service.dart` `pingInterval=30s` |
| Siapa deteksi disconnect? | Server: read error/timeout (ReadPump) atau write error (WritePump) → `c.Close()`; Client: `onDone/onError` | `connection.go` `defer c.Close()`; `Close()` → `hub.Unregister` |
| Clean vs abnormal? | **TIDAK dibedakan** — tanpa close-code; dua jalur konvergen ke error I/O. `CloseWithCode` ada tapi unused | `connection.go Close()`; `websocket_handler.go CloseWithCode` |
| Multi-instance limitation? | `Hub` in-process (`map` + `RWMutex`), tidak ada registry lintas instance | `hub.go:38-42` |
| Hub in-process / distributed? | **IN-PROCESS** | `hub.go` |

**Kesimpulan §4:** lifecycle WS **sudah menyediakan hook yang layak** untuk lease Presence:
titik acquire = setelah `hub.Register(conn)`; renew = ticker 54s (grace 90s > 54s → aman);
release = `Connection.Close()` (idempotent via `sync.Once`, dipanggil dari ReadPump **dan** WritePump).
Tidak ada solusi baru yang diperlukan.

---

## 5. REDIS LEASE AUDIT

Key/const: `internal/presence/types.go:13-21`
`presence:v1:leases:{userID}` · `presence:v1:state:{userID}` · `presence:v1:deadlines` ·
`presence:v1:events` · grace 90s · worker interval 5s · claim batch 200.

| Operation | Existing implementation | Production caller | Status |
|---|---|---|---|
| acquire lease | `RedisRepository.ResumeLease` → Lua `leaseMutationLua` (`resume`: `ZADD leases expiryMs connID`) | **0** | INCOMPLETE |
| renew lease | **tidak ada method khusus** — renew = `ResumeLease` ulang dgn `connectionID` sama (`ZADD` reset score) | **0** | INCOMPLETE |
| release lease | `RedisRepository.LeaveLease` (`leave`: `ZREM leases connID`) | **0** | INCOMPLETE |
| bump version (visibility re-eval) | `RedisRepository.BumpVersion` → `bumpVersionLua` | **0** | OBSOLETE-ish |
| expiry | `ZREMRANGEBYSCORE leases '-inf' nowMs` di dalam Lua (setiap mutation + sweep) | 0 | INCOMPLETE |
| sweep | `RedisRepository.ClaimDueUsers` (`claimDueUsersLua`) + `SweepLease` (`sweepUserLua`) + `RequeueDeadline` | **0** | INCOMPLETE (tidak ada worker loop) |
| publish changed | `RedisRepository.Publish` → `PUBLISH presence:v1:events` | `Service.PublishChanged` (**0 caller**) | INCOMPLETE |
| subscribe changed | `RedisRepository.Subscribe` → `SUBSCRIBE presence:v1:events` | `PresenceSubscriber.run` (**tidak pernah Start**) | INCOMPLETE |
| read state (batch) | `RedisRepository.GetStatesBatch` (HGETALL pipeline) | `Service.BuildSnapshot` (**0 caller**) | INCOMPLETE |
| read last-seen (DB) | `DBRepository.GetLastSeen` / `GetLastSeenBatch` | **0** (hanya test) | OBSOLETE-ish |

**Atomicity:** semua mutasi lewat Lua `EVAL` (single round-trip, atomik) — `redis_repository.go:23-190`.
`load_state` memvalidasi malformed state (`PRESENCE_MALFORMED_STATE`), tidak menelan error.

---

## 6. MULTI-SESSION CONTRACT

Skenario:
```
User A ├─ session 1 ├─ session 2 └─ session 3
```

| Pertanyaan | Jawaban faktual | Evidence |
|---|---|---|
| Lease per-user atau per-session? | **per-session**, diagregasi per-user | ZSET `presence:v1:leases:{userID}`, member = `connectionID`; `ZCARD` = `ActiveLeaseCount` |
| Session 1 disconnect, session 2 hidup → tetap online? | **YA** | Lua `leave`: `ZREM` 1 member, lalu `activeCount = ZCARD`; transisi `online=0` hanya bila `activeCount == 0` |
| Key structure mendukung? | **YA** | `leases:{user}` ZSET + `state:{user}` HASH + `deadlines` ZSET |
| Version mencegah stale disconnect mematikan sesi aktif? | **YA (sebagian)** — version naik hanya saat transisi (`transitioned=1`); `PresenceSubscriber.shouldDeliver(userID, version)` men-drop `version <= delivered`. Tapi **bukan** guard per-session: `leave` yang datang terlambat dari koneksi mati hanya `ZREM` member miliknya sendiri (aman secara struktur), bukan mematikan sesi lain | `leaseMutationLua`; `presence_subscriber.go shouldDeliver`; `presence_subscriber_test.go TestPresenceSubscriber_ShouldDeliver_DedupesVersions` |
| Existing test membuktikan multi-session? | **TIDAK** | Test yang ada hanya single-connection (`"presence-conn"`, `"conn-target"`, `"conn-self"`). **MISSING CONTRACT** |

**MISSING CONTRACT:** belum ada proof bahwa `ResumeLease(conn1) + ResumeLease(conn2) + LeaveLease(conn1)`
menghasilkan `IsOnline == true` dan **tidak** ada transisi. Ini wajib dijadikan acceptance gate slice berikutnya.

---

## 7. PRESENCE.CHANGED CONTRACT

Ada **dua bentuk payload** yang berbeda (Redis internal vs WS wire).

### 7a. Redis Pub/Sub payload (internal backend) — `presence.Events`
`internal/presence/types.go:31-34` + `service.go:56-60`:
```json
{ "type": "presence.changed",
  "state": { "user_id": "<uuid>", "is_online": true,
             "last_seen_at": "2026-...Z"|null, "version": 42 } }
```
Hanya dibaca `PresenceSubscriber.handleMessage`. Konsisten. **OK.**

### 7b. WS frame (client-facing) — NON-CANONICAL
`realtime/presence_subscriber.go:215-224` `marshalPresenceChanged` memakai `presencepkg.Event`
→ `{"type":"presence.changed","state":{...}}`.

Canonical WS envelope backend adalah `WSEnvelope` (`envelope.go:22-28`):
```json
{ "id": "...", "type": "...", "timestamp": "...", "from": "server", "data": { ... } }
```

**CONTRACT GAP (blocking):**

| Field | Canonical `WSEnvelope` | `presence.changed` sekarang | Dampak |
|---|---|---|---|
| `id` | wajib | **tidak ada** | mobile `WebSocketMessage.fromJson` cast `json['id'] as String` → **throw** |
| `from` | wajib | **tidak ada** | throw |
| `data` | wajib (map) | **tidak ada** (`state` di top-level) | throw |
| `timestamp` | wajib | **tidak ada** | throw |
| state | di dalam `data` | top-level `state` | field tidak terbaca |

Konsekuensi terverifikasi di mobile (`websocket_service.dart _handleMessage`):
parse `WebSocketMessage.fromJson` gagal → `_messageController.addError(FormatException)`.
Artinya **bahkan jika subscriber diaktifkan sekarang, tidak ada satu pun frame presence yang
bisa diparse mobile.**

Ketidakcocokan tambahan:

| Aspek | Backend | Mobile | Gap |
|---|---|---|---|
| Nama type | `presence.changed` | `MessageType.presence = 'presence'` (`websocket_message.dart:62`) | berbeda |
| Field state | `is_online` (bool JSON) | `data['status'] == 'online'` (`app_presence_service_api.dart:54-58`) | berbeda |
| Identitas | `is_online`, `last_seen_at`, `version` | `user_id`, `status` | berbeda |

**CONTRACT GAP:** payload `data` canonical yang diperlukan (berdasarkan authority & consumer):
`user_id`, `is_online`, `last_seen_at` (nullable), `version`.
Tidak boleh dikarang di luar field yang sudah ada di `presence.State` (`types.go:27-32`).
Visibility **tidak** dikirim ke client (sudah di-enforce server-side di
`visibleStateForViewer`, `service.go:239-272`) — jadi tidak perlu field visibility di wire.

---

## 8. PRIVACY AUTHORITY

Dua kandidat, **keduanya tanpa writer di backend**.

| Kandidat | Writer | Reader | DTO/Model | Settings UI | Persistence | Test |
|---|---|---|---|---|---|---|
| `user_profiles.privacy->>'show_activity_status'` | **TIDAK ADA** (grep `SET privacy`/`privacy =` = 0) | `presence/service.go:176` (COALESCE default `'true'`) | `user_response.go:98` (`ShowActivityStatus`), mobile `user_api_models.dart:245,268,335,682` | — | kolom `user_profiles.privacy` jsonb (`000001_canonical_schema.up.sql:1767`) | `presence/service_snapshot_test.go` (5 test seed privacy manual) |
| `show_online_status` (mobile lokal) | `PresenceManager._loadEnabledSetting/setEnabled` → `localStorage.setBool` (`presence_provider.dart:40,63,86`) | `settings_screen.dart:41-42` (toggle) | — | ✅ "Show Online Status" (`settings_security_privacy_section.dart:52-59`) | **local storage saja** | — |

Fakta penting:
- Backend Presence **sudah membaca** `show_activity_status` dan meng-enforce di
  `visibleStateForViewer` (hide online + last_seen) — `service.go:239-272`.
- Tapi **tidak ada endpoint yang menulis** field itu → selalu default `'true'`
  → toggle user tidak berpengaruh.
- Toggle Settings menulis ke local storage → **tidak** sampai ke backend.
- Jadi saat ini ada **dua authority visibility yang saling lepas** untuk satu konsep.

**`OWNER DECISION REQUIRED`** — lihat §16 #1. Audit **tidak memilih**.

---

## 9. LAST-SEEN AUDIT

Chain: `transisi → last_seen → PostgreSQL`.

### Bukti self-loop (faktual)

```
Service.PersistLastSeen(userID, occurredAt, version)        service.go:75-105
  ├─ (A) tx: DBRepository.UpsertLastSeen → SUKSES → return nil   [tidak ada event]
  └─ (B) GAGAL && outbox != nil →
         OutboxInserter.InsertTx(EventUserPresenceLastSeenRecord, payload, "{userID}.{version}")
                 │
                 ▼  outbox row: presence.last_seen_record
         OutboxWorker dispatcher
                 │
                 ▼
         PresenceLastSeenHandler.Handle                        presence_last_seen_handler.go:28-49
                 │
                 ▼
         Service.PersistLastSeen(...)  ← KEMBALI KE (A)/(B)
```

| Peran | Faktual | Evidence |
|---|---|---|
| Producer event | `Service.PersistLastSeen` cabang gagal | `service.go:85-95` |
| Consumer event | `PresenceLastSeenHandler.Handle` | `presence_last_seen_handler.go:49` |
| Trigger eksternal | **TIDAK ADA** | grep `PersistLastSeen` → hanya handler + test |
| Persistence writer | `DBRepository.UpsertLastSeen` (monotonic CASE, tidak pernah memundurkan waktu) | `db_repository.go:75-100` |
| Retry semantics | idempotencyKey `{userID}.{version}`; handler replay = upsert monotonik | `service.go:87`; `presence_last_seen_handler_test.go TestPresenceLastSeenHandler_ReplaysMonotonicUpsert` |

**Kesimpulan self-loop:** satu-satunya producer `presence.last_seen_record` adalah method yang
dipanggil oleh consumer-nya sendiri. Karena **tidak ada** caller produksi `PersistLastSeen`,
branch (B) tidak pernah tereksekusi → event tidak pernah ada → handler tidak pernah jalan →
`user_presence` **tidak pernah ditulis** di production (hanya oleh test).

Akar masalahnya: transisi yang membawa `LeaseResult.LastSeenAt` (`leaseMutationLua` mengisi
`lastSeenMs` saat `transitioned=1`) **tidak pernah dikonsumsi**. `LeaveLease`/`SweepUser`
= 0 production caller.

**Target canonical (belum diimplementasi):**
```
WS session release/expiry (GracePeriod)
  → LeaseResult{Transitioned=true, LastSeenAt=t}
  → durable event presence.last_seen_record   (outbox)
  → PresenceLastSeenHandler
  → user_presence.last_seen_at (PostgreSQL)
```

---

## 10. CROSS-INSTANCE AUDIT

| Pertanyaan | Jawaban faktual | Evidence |
|---|---|---|
| Redis state shared? | **YA** — satu Redis, key `presence:v1:*` | `redis_repository.go` |
| Pub/Sub shared? | **YA** — channel `presence:v1:events` | `types.go:17`, `redis_repository.go:283,290` |
| WS Hub in-process? | **YA** — `map[string]*Connection` per proses | `hub.go:38-42` |
| Bagaimana `presence.changed` sampai ke client di instance lain? | Instance penerima `Publish` → `PUBLISH` channel → **setiap** instance menjalankan `PresenceSubscriber` → fan-out ke **hub lokal**-nya | `presence_subscriber.go:147-213` |
| Infra realtime existing sudah punya cross-instance fan-out? | **HANYA presence.** Redis Pub/Sub adalah **satu-satunya** pemakai di backend (grep `.Publish(`/`.Subscribe(` = hanya presence). Chat realtime **tidak** punya pub/sub: `realtime.Worker` claim outbox row lalu dispatch ke hub lokal → hanya instance yang meng-claim | `dispatcher.go`; `realtime_worker.go:290-321` |
| `PresenceSubscriber` usable atau dead? | **USABLE + TEST-PROVEN, tapi tidak pernah di-Start.** Test lintas-instance dua hub hijau | `presence_subscriber_test.go:112-205` `TestPresenceSubscriber_DistributesPresenceChangedAcrossInstances` |

**Architecture gap:** mekanisme cross-instance **sudah ada dan terbukti**; yang hilang hanyalah
`Start()` di serverboot + jumlah producer > 0. Tidak ada pekerjaan infrastruktur baru yang diperlukan.
(Catatan out-of-scope: chat realtime tidak punya cross-instance fan-out — dicatat, **tidak** disentuh.)

---

## 11. INITIAL STATE / REALTIME STATE

Dua contract **berbeda** dan wajib dipisahkan.

### A. Initial state — "apakah user ini sedang online?"
| Aspek | Faktual |
|---|---|
| Authority yang tersedia | `presence.Service.BuildSnapshot(ctx, viewerID, targetIDs) ([]State, error)` — `service.go:108-137` |
| Sifat authority | batch, viewer-scoped, sudah menegakkan block + lifecycle + privacy (fail-closed) |
| Transport | **TIDAK ADA** (0 production caller; tidak ada HTTP route; tidak ada WS snapshot frame) |
| REST presence read (`/users/{id}/presence`) | **tidak ada** di backend |
| Mobilitas saat ini | `AppPresenceServiceApi.getUserOnlineStatus/getUserLastSeen` → GET `/users/{id}/presence` → **404** → `Result.success(false)` (fail-open) |

**Kesimpulan:** WebSocket event **tidak cukup** untuk initial state (event hanya terjadi saat ada
transisi; watcher yang baru connect tidak akan tahu state saat ini tanpa read eksplisit).
Authority-nya sudah ada (`BuildSnapshot`); transportnya gap (**GAP #8**).

### B. Realtime update — ONLINE↔OFFLINE
Authority = Redis state + `presence.changed` via Pub/Sub → WS (GAP #5, #6).
Consumer mobile = `userOnlineStatusProvider` / `AppPresenceServiceApi.watchUserPresence`
(sekarang mati — lihat §12).

---

## 12. MOBILE CONTRACT

Provider → datasource → WS → UI (kondisi faktual).

**Stack REST (Owner: BUKAN canonical):**
- `IPresenceService` (`core/src/interfaces/services/i_presence_service.dart`)
- `AppPresenceServiceApi` (`core/src/services/app_presence_service_api.dart`)
  → POST `/users/presence`, `/users/presence/start`, `/users/presence/stop`; GET `/users/{id}/presence`
  → **semua 404** (backend grep = 0 route)
- `presenceServiceProvider` (`core_providers.dart:134`), di-override `main.dart:89-90` dgn `AppPresenceServiceApi` (`main.dart:205`)
- `PresenceManager` / `presenceManagerProvider` (`core/src/providers/presence_provider.dart`)
- `presenceAuthSyncProvider` (`presence_provider.dart:271`) — 0 watcher

**Production callers stack REST (eksak):**

| Caller | Lokasi | Perilaku sekarang |
|---|---|---|
| Login/logout | `auth_controller.dart:1742` (`setUser`), `:1617`, `:1685` (`clearUser`) | memicu `startTracking`/`stopTracking` → 404 |
| App resume/inactive | `PresenceManager.didChangeAppLifecycleState` | timer 60s → `updatePresence` → 404 |
| Settings toggle | `settings_screen.dart:41,86` | `setEnabled` → local storage saja |
| Avatar/header UI | `profile_avatar.dart:40`, `user_header_widget.dart:115`, `hybrid_avatar.dart:80` | baca `userOnlineStatusProvider` → selalu `false` |

**Klasifikasi (§12 mandate):** stack REST = **OBSOLETE** (Owner-declared),
tetapi purge **BLOCKED UNTIL CANONICAL WS CONTRACT EXISTS** — karena
`userOnlineStatusProvider` masih menjadi sumber live untuk 3 consumer UI di atas.
Purge tanpa replacement = UI rusak, bukan convergence.

**Chat-domain stack (design ke-2):**
`chat_state.PresenceState` (`chat_state.dart:174`) · `Presence` notifier (`chat_notifier.dart:537-571`)
· `ManagePresenceUseCase` · `ChatRepository` 5 method (impl `chat_repository_impl.dart:664-703`
→ `Result.error('Presence tracking not available')`) · `UserPresence` entity (`chat_entities.dart:630`)
· `formatChatLastSeen` · `presenceEnabledProvider` (`chat_providers.dart:69`).
`presenceProvider` dibaca `chat_detail_screen.dart:414` → `'Online'` (`:457-463`) yang **tidak pernah** bisa true.

---

## 13. FOUR-WAY CLASSIFICATION

| Artifact | Classification | Evidence | Action |
|---|---|---|---|
| `presence.RedisRepository` (lease/state/deadline/Lua) | **CANONICAL / REQUIRED** | Owner #2; atomik, multi-session capable | KEEP — butuh caller |
| `presence.Service.{ResumeLease,LeaveLease,SweepUser,ClaimDueUsers,PublishChanged,SubscribeEvents}` | **CANONICAL / REQUIRED** | Owner #2,#4,#6; 0 caller | KEEP — wire (§15) |
| `presence.Service.BuildSnapshot` / `ResolveVisibleStatesForTarget` | **CANONICAL / REQUIRED** | initial state authority + visibility enforcement (`service.go:108,139`) | KEEP — butuh transport |
| `presence.DBRepository.UpsertLastSeen` | **CANONICAL / REQUIRED** | Owner #5; monotonic upsert | KEEP |
| `migrations/000025_user_presence_foundation.up.sql` (`user_presence`) | **CANONICAL / REQUIRED** | Owner #5 | KEEP (tambah `.down.sql`) |
| `presence.Event` / `presence.State` (Redis payload) | **CANONICAL / REQUIRED** | `types.go:27-34` | KEEP |
| `realtime.PresenceSubscriber` | **INCOMPLETE** | test lintas-instance hijau, `Start()` tidak pernah dipanggil | WIRE (§15 Slice 2) |
| `marshalPresenceChanged` (WS frame) | **INCOMPLETE** | bukan `WSEnvelope`; mobile pasti gagal parse | FIX → `WSEnvelope` |
| `worker.PresenceLastSeenHandler` + `SetupPresenceLastSeenHandler` | **INCOMPLETE** | terdaftar (`dependencies.go:2766`) tapi input self-loop | KEEP; butuh producer eksternal |
| `events.EventUserPresenceLastSeenRecord` | **CANONICAL / REQUIRED** | Owner #5 | KEEP |
| `presence.DBRepository.GetLastSeen` / `GetLastSeenBatch` | **OBSOLETE** | 0 caller (hanya test) | PURGE (setelah last-seen read path diputuskan) |
| `presence.RedisRepository.RequeueDeadline` | **OBSOLETE** | 0 caller di seluruh repo | PURGE |
| `presence.Service.BumpVisibilityVersion` | **AMBIGUOUS** | 0 caller; belum jelas apakah privacy-change perlu re-eval realtime | OWNER/arsitek — bukan bagian slice 1 |
| `user_profiles.privacy->>'show_activity_status'` | **AMBIGUOUS** | dibaca `service.go:176`; **0 writer** | `OWNER DECISION` (§16 #1) |
| `show_online_status` (mobile local) | **AMBIGUOUS** | UI nyata, hanya local storage | `OWNER DECISION` (§16 #1) |
| `IPresenceService` / `AppPresenceServiceApi` / REST routes | **OBSOLETE** | Owner #7; endpoint 404 | PURGE — **BLOCKED** sampai WS contract hidup |
| `presenceServiceProvider` + `PresenceManager` (REST-driven) | **OBSOLETE** | diturunkan dari stack REST | PURGE — BLOCKED |
| `AppLifecycleObserver` | **OBSOLETE** | 0 referensi | PURGE |
| `presenceAuthSyncProvider` | **OBSOLETE** | 0 watcher | PURGE |
| `OnlineAvatarWidget` / `OnlineAvatarCompact` / `OnlineStatusText` | **OBSOLETE** | 0 pemakaian; hanya barrel `shared/shared.dart:53` | PURGE |
| `WebSocketService.updatePresence` (`websocket_service.dart:470`) | **OBSOLETE** | 0 caller; pakai `MessageType.presence` lama | PURGE |
| `MessageType.presence` (`websocket_message.dart:62`) | **OBSOLETE** | type salah (`'presence'` ≠ `'presence.changed'`) | PURGE / ganti |
| chat `Presence` notifier + `ManagePresenceUseCase` + `presenceEnabledProvider` | **OBSOLETE** | 0 production caller; flag literal `false` | PURGE — BLOCKED sampai consumer chat dipindah |
| `UserPresence` entity | **OBSOLETE** | 0 consumer | PURGE |
| `formatChatLastSeen` | **OBSOLETE** | hanya dikonsumsi test | PURGE |
| `chat_state.PresenceState` | **AMBIGUOUS** | dibaca `chat_detail_screen.dart:414` | OWNER/arsitek: ganti ke canonical state |
| Settings "Show Online Status" | **AMBIGUOUS** | UI nyata, tanpa efek backend | ikut §16 #1 |
| `presence_residue_contract_test.dart` | **CANONICAL (negative proof)** | saat ini **RED** (3 hit) | KEEP sebagai gate |
| `chat_presence_provider_contract_test.dart` | **CANONICAL** | GREEN; `presenceEnabledProvider == false` | UPDATE setelah canonical hidup |
| `chat_detail_presence_behavior_test.dart` | **OBSOLETE (stale)** | 4 FAILING | AUDIT berikutnya |

---

## 14. ANTI-RESURRECTION / PURGE MANIFEST

**Tidak ada purge dijalankan.** Manifest ini hanya untuk slice cleanup setelah contract canonical hidup.

| Artifact | Reason obsolete | Evidence | Dependent callers | Safe purge boundary |
|---|---|---|---|---|
| `apps/mobile/lib/core/src/services/app_presence_service_api.dart` | REST bukan canonical (Owner #7) | endpoint 404; residue test RED | `presenceServiceProvider` ← `main.dart:89` | Setelah `userOnlineStatusProvider` punya sumber canonical |
| `apps/mobile/lib/core/src/interfaces/services/i_presence_service.dart` | Kontrak REST | — | `AppPresenceServiceApi`, barrel `core/core.dart:27` | Bersama di atas |
| `PresenceManager` + `presenceAuthSyncProvider` | REST-driven, local-only | `presence_provider.dart:240,271` | `settings_screen.dart:41,86`, `auth_controller.dart:1617,1685,1742` | Bersama §16 #1 |
| `apps/mobile/lib/core/src/services/app_lifecycle_observer.dart` | 0 referensi | grep = 0 | — | langsung aman |
| `apps/mobile/lib/shared/widgets/online_avatar_widget.dart` | 0 pemakaian | hanya `shared/shared.dart:53` | — | langsung aman |
| `WebSocketService.updatePresence` + `MessageType.presence` | 0 caller; type mismatch | `websocket_service.dart:470` | hanya stack REST | setelah mobile WS contract |
| `manage_presence_usecase.dart` (+ ekspor `chat_usecases.dart:6`) + `ManagePresenceUseCase` provider | 0 production caller | `chat_usecase_providers.dart:42` | chat `Presence` notifier | setelah `chat_detail_screen` dipindah |
| `ChatRepository` 5 presence method + impl stub | `Result.error('Presence tracking not available')` | `chat_repository_impl.dart:664-703` | `ManagePresenceUseCase` | bersama di atas |
| `UserPresence` entity | 0 consumer | `chat_entities.dart:630` | — | langsung aman |
| `chat_last_seen_formatter.dart` | test-only consumer | hanya `chat_detail_presence_behavior_test.dart` | — | setelah test diselaraskan |
| `DBRepository.GetLastSeen`/`GetLastSeenBatch`, `RequeueDeadline` | 0 caller | grep | — | setelah read path diputuskan |
| Komentar bohong "Backend does not have presence endpoints" | menyesatkan future agent | `chat_providers.dart:67-68`, `chat_repository_impl.dart:673,680,687,694,701` | — | bersamaan dgn purge stack REST |
| `realtime/presence_subscriber.go` impl yang tidak di-Start | mislead: terlihat aktif | `dependencies.go` tidak membuatnya | — | **JANGAN purge** — ini canonical (§15 Slice 2) |

**Resurrection risk tertinggi:** dua desain hadir bersamaan + komentar `false` + `MessageType.presence`
yang salah. Sebelum canonical hidup, agent berikutnya sangat mungkin "memperbaiki" REST alih-alih WS.

---

## 15. NEXT IMPLEMENTATION SLICE

### SLICE 1 — "WS session memiliki presence lease" (backend-only)

**Invariant tunggal:** setiap koneksi WS aktif memegang tepat satu presence lease yang
di-renew selama koneksi hidup dan di-release saat koneksi berakhir, sehingga Redis presence
state mencerminkan sesi nyata.

**Mengapa ini slice terkecil yang aman:**
- Menutup GAP #1/#2/#3 — akar yang membuat semua artefak lain (subscriber, sweeper, last-seen)
  tidak punya producer.
- Memakai implementasi yang **sudah ada dan teruji** (`ResumeLease`/`LeaveLease`), bukan menulis authority baru.
- Tidak membuat authority paralel; tidak menyentuh chat realtime; tidak menyentuh mobile.
- Tidak ada consumer yang bergantung padanya (subscriber belum Start, mobile belum wired) →
  tidak ada risiko regresi user-facing.
- Dapat dibuktikan sendiri lewat integration test: connect → online; disconnect → offline;
  2 koneksi + 1 disconnect → tetap online.

**Files (backend only):**
1. `backend/internal/realtime/connection.go`
   - Tambah port kecil (interface) `PresenceLease` + field opsional pada `Connection`.
   - `ResumeLease(ctx, conn.UserID, conn.ID)` setelah `hub.Register` (titik acquire).
   - Di `Close()` (idempotent) → `LeaveLease(ctx, conn.UserID, conn.ID)`.
   - Di ticker WritePump 54s → renew (`ResumeLease` ulang dgn `conn.ID` sama).
   - Nil-safe: jika port nil → perilaku sekarang (tidak berubah).
2. `backend/internal/realtime/websocket_handler.go`
   - `NewHandler(...)` menerima port lease dan meneruskannya ke `NewConnection`.
3. `backend/internal/serverboot/dependencies.go`
   - Teruskan `presenceService` (sudah dibuat di `:938-944`) ke `realtime.NewHandler` (`:1024`).
   - Tidak perlu membuat worker/subscriber baru di slice ini.

**Tests (slice gate):**
- Multi-session: `ResumeLease(u, c1) + ResumeLease(u, c2) + LeaveLease(u, c1)` → `IsOnline == true`, tanpa transisi.
- Single-session release → `IsOnline == false`, `Version` naik, `LastSeenAt != nil`.
- Renew memperpanjang deadline (score ZSET naik) tanpa menaikkan `version`.
- Nil-port: jalur WS lama tetap utuh (tidak ada panic).

**Explicitly NOT in Slice 1:** sweeper loop (GAP #4), `PresenceSubscriber.Start()` + fix frame
`WSEnvelope` (GAP #5/#6), initial-state transport (GAP #8), last-seen trigger (GAP #9),
mobile apa pun, privacy authority (§16 #1).

**Known limitation to declare honestly:** tanpa sweeper, jika proses backend mati paksa
(lease tidak pernah di-release dan tidak ada mutasi lanjutan), `state.online` bisa tertinggal `1`
sampai lease kedaluwarsa **dan** ada mutasi berikutnya. Tidak ada user yang terpengaruh pada
slice ini karena belum ada consumer. Sweeper = Slice 2.

### Slice berikutnya (urutan, jangan digabung)
- **Slice 2:** sweeper worker (`ClaimDueUsers` → `SweepUser` → publish transisi) + `PersenceSubscriber.Start()` di serverboot + `marshalPresenceChanged` → `WSEnvelope` (GAP #4/#5/#6).
- **Slice 3:** last-seen producer (transisi → outbox `presence.last_seen_record` → `user_presence`) (GAP #9).
- **Slice 4:** initial-state transport (rekomendasi: read REST tipis di atas `BuildSnapshot`, dipisah dari realtime WS) (GAP #8).
- **Slice 5:** mobile convergence (WS contract + purge manifest §14).

---

## 16. OWNER DECISIONS REQUIRED

**#1 — Privacy authority (tidak dapat dibuktikan dari codebase).**
Dua kandidat, keduanya tanpa writer:
- `user_profiles.privacy->>'show_activity_status'` — dibaca backend (`service.go:176`), di-enforce
  (`visibleStateForViewer`), punya DTO/`show_activity_status`, **0 writer**.
- `show_online_status` — toggle Settings nyata, **hanya local storage**, tidak pernah sampai backend.

Konflik eksak: toggle user saat ini **tidak dapat** mempengaruhi apapun di server; dan server
sudah punya gate privacy yang tidak dapat diubah user.
**Pilihan yang diminta:** (a) `show_activity_status` canonical → butuh endpoint write + konvergen
DTO/mobile; (b) `show_online_status` canonical → pindahkan authority ke backend; (c) samakan keduanya.

**#2 — Transport initial state.**
Owner sudah mengunci WS untuk *realtime update*. Initial state (§11) belum ditentukan.
Authority sudah ada: `presence.Service.BuildSnapshot` (batch, viewer-scoped, block+privacy aware).
**Pilihan yang diminta:** REST read tipis di atas `BuildSnapshot`, atau frame WS snapshot saat connect.
**Rekomendasi audit:** REST read terpisah (initial state ≠ realtime transport; menghindari race
"connected sebelum snapshot" dan tetap menghormati privacy per-viewer).

Tidak ada keputusan Owner lain yang diperlukan untuk Slice 1.

---

## 17. FILESYSTEM INTEGRITY

```
files changed  = 0
files deleted  = 0
tests modified = 0
migrations modified = 0
```

- Audit memakai operasi read-only: `read_file`, `read_directory`, `glob`, `grep`, `git status --porcelain`.
- Tidak ada `checkout`/`restore`/`reset`/`cherry-pick`/`revert`/`pull` dari Git atau GitHub.
- **External/unrelated changes (bukan milik scope ini):** working tree sudah kotor sejak awal sesi
  (banyak `M`/`D`/`??` pra-eksisting di chat, commerce, seller, search, worker, dll.).
  Dua entri `??` (`chat_resource_projection_content_media_contract_test.dart`,
  `content_shared_reference_media_render_contract_test.dart`) tidak dibuat oleh audit ini —
  kemungkinan sesi paralel di worktree yang sama. Tidak diklaim sebagai hasil scope ini.

---

## VERIFICATION (untuk slice berikutnya)

Gate Slice 1, dijalankan pada implementasi nanti:
1. `go build ./internal/realtime/... ./internal/presence/... ./internal/serverboot/...`
2. `go vet ./internal/realtime/... ./internal/presence/...`
3. `go test ./internal/presence/... ./internal/realtime/...` (butuh Redis + Postgres test DB:
   `pkg/testdb`, Redis DB 14/15 sesuai test yang ada)
4. Bukti multi-session eksplisit (test baru, lihat §15) — ini **MISSING CONTRACT** yang wajib ditutup.
5. Residue search: `presenceProvider`, `userOnlineStatusProvider`, `MessageType.presence`,
   `users/presence` → pastikan tidak ada authority baru yang lahir dari slice ini.
6. `git status --porcelain` → delta hanya pada file yang dideklarasikan.
