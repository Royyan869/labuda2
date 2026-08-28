# Local Dev Quickstart

**Status: STABLE**
Last reviewed: 2026-06-22

One-page guide for starting the full Labuda stack locally for development or owner testing. This document is the canonical startup reference — other docs link here.

For full seed data setup (buyer/seller/orders), see [`dev-seed-guide.md`](./dev-seed-guide.md).
For admin user bootstrapping, see [`admin-bootstrap.md`](./admin-bootstrap.md).
For owner test flows, see [`owner-test-guide.md`](./owner-test-guide.md).

---

## Prerequisites

| Tool | Min version | Notes |
|---|---|---|
| Go | 1.22+ | `go version` |
| Docker Desktop | any | For local Postgres + Redis via `docker-compose` |
| Flutter | 3.41+ | `flutter --version` |
| Node.js | 20+ | `node --version` |
| npm | 10+ | bundled with Node |
| Android Studio / SDK | any | Required for emulator or device builds |
| Android USB debugging | — | Required for real-device testing only |
| Firebase project | — | Real credentials for auth; set `DEV_MOCK_FIREBASE_AUTH=true` to bypass locally |

**Windows note:** `make` is not part of standard Windows. All Windows commands use `go run` and `npm` directly. `make` targets are Linux/Mac convenience wrappers only — see per-platform sections below.

---

## Golden Order (start in this sequence every time)

```
1. Start Postgres + Redis (docker-compose)
2. Run migrations  (go run ./cmd/migrate)
3. Run seeder      (go run ./cmd/seed)   ← optional, skip if already seeded
4. Start backend   (go run ./cmd/core_server)
5. Verify health from laptop browser
6. Verify health from phone browser     ← real device only
7. Start mobile    (flutter run -d <device-id>)
8. Start admin     (npm run dev)         ← in apps/admin/
```

Do not start `core_server` before migrations are applied. The server does not auto-run migrations.

---

## Step 1 — Start Postgres and Redis

From the **repo root**:

```bash
docker-compose up -d
```

Starts:
- PostgreSQL on `:5432` — user `labuda`, password `labuda123`, database `labuda`
- Redis on `:6379`

Verify:

```bash
docker ps
# Should show labuda-postgres and labuda-redis containers as Up
```

---

## Step 2 — Configure Backend Environment

```bash
cd backend
cp .env.example .env
# .env has correct defaults for docker-compose DB credentials
# Edit only if you use a custom DB setup or need Firebase/Midtrans keys
```

Key variables and their defaults (see [`.env.example`](../../backend/.env.example) for full list):

| Variable | Default | Notes |
|---|---|---|
| `DB_HOST` | `localhost` | — |
| `DB_USER` | `labuda` | Matches docker-compose |
| `DB_PASSWORD` | `labuda123` | Matches docker-compose |
| `DB_NAME` | `labuda` | Matches docker-compose |
| `REDIS_HOST` | `localhost` | — |
| `PORT` | `8080` | Server listens here |
| `DEV_MOCK_FIREBASE_AUTH` | `false` | Set `true` for local dev without real Firebase |
| `FIREBASE_PROJECT_ID` | `labuda-79de2` | Required if `DEV_MOCK_FIREBASE_AUTH=false` |
| `MIDTRANS_SERVER_KEY` | _(empty)_ | Backend only. Required for server-side payment and refund verification. Never copy into mobile env files. |
| `MIDTRANS_CLIENT_KEY` | _(empty)_ | Client-safe. Use in mobile docs/env if the app needs it. |

---

## Step 3 — Run Migrations

**This step is required before starting the server.**

```bash
# Windows, Linux, Mac — all the same
cd backend
go run ./cmd/migrate
```

Expected last line: `MIGRATION COMPLETE`.
Current canonical version: **212**.
Latest migration file: `backend/migrations/000212_add_admin_audit_logs.up.sql`.

> **Do not use `make migrate-up`.** The Makefile uses the `migrate` CLI tool, which has a different `schema_migrations` schema from `go run ./cmd/migrate`. Mixing them on the same DB corrupts the migration state. Always use `go run ./cmd/migrate`.

---

## Step 4 — Run Reference Data Seeder (optional)

Skip this step if the DB is already seeded from a previous run.

```bash
cd backend
go run ./cmd/seed
```

Seeds: platform configs, seller subscription configs, 3 fixed test users.
The seeder is idempotent for users (upsert). Content/comments are appended on each run.

---

## Step 5 — Start Backend

### Windows (and universal)

```bash
cd backend
go run ./cmd/core_server
```

### Linux / Mac

```bash
# Direct (same as above — canonical fallback)
cd backend && go run ./cmd/core_server

# Or using make (Linux/Mac only — requires `make` installed)
cd backend && make run
```

The server starts on port `8080`.

---

## Step 6 — Verify Backend Health

### From the laptop (all platforms)

Open a browser or run:

```
http://localhost:8080/health
```

Expected response: `{"status":"healthy",...}`

Additional endpoints:
- `GET /health/ready` — readiness probe (DB + Redis required)
- `GET /health/live` — liveness probe (always 200 if process is up)

### From the phone (real Android device only)

Find your laptop's LAN IP:

```bash
# Windows
ipconfig
# Look for IPv4 Address under your Wi-Fi adapter, e.g. 192.168.1.42

# Linux/Mac
ip addr show
# or: ifconfig | grep "inet " | grep -v 127.0.0.1
```

Open the phone's browser:

```
http://<YOUR-LAN-IP>:8080/health
```

If the phone cannot reach the backend, see **Troubleshooting** → "Phone cannot open /health".

---

## Step 7 — Windows Firewall (real Android device only)

If the phone cannot reach port 8080 from the same Wi-Fi network, add a firewall rule. Run in an **Administrator** PowerShell:

```powershell
netsh advfirewall firewall add rule `
  name="Labuda Backend Port 8080" `
  dir=in action=allow protocol=TCP localport=8080 profile=any
```

Verify: retry `http://<LAN-IP>:8080/health` from the phone browser.

To remove the rule later:

```powershell
netsh advfirewall firewall delete rule name="Labuda Backend Port 8080"
```

---

## Step 8 — Mobile App

### Find your device

```bash
flutter devices
# Shows connected emulators and physical devices with their <device-id>
```

### Android Emulator

Emulator uses the platform-aware dev default `10.0.2.2` (no flag needed). The same default comes from [api_config.dart](../../apps/mobile/lib/core/api/config/api_config.dart) `ApiConfig.baseUrl`/`wsUrl` when no `--dart-define` is provided.

Run:

```bash
cd apps/mobile
flutter run -d <emulator-id>
# e.g. flutter run -d emulator-5554
# Flutter selects the emulator automatically if only one device is connected
```

### Real Android Device

Physical devices **must** use explicit `--dart-define` overrides — do not edit source. `<LAN-IP>` is your laptop's LAN IP from Step 6 (same Wi-Fi as the phone).

Canonical command:

```bash
cd apps/mobile
flutter run -d <device-id> \
  --dart-define=API_BASE_URL=http://<LAN-IP>:8080/api/v1 \
  --dart-define=API_WS_URL=ws://<LAN-IP>:8080/api/v1/ws
# Example: --dart-define=API_BASE_URL=http://192.168.1.42:8080/api/v1
```

Prerequisites (same as Steps 6-7):

* Backend must be listening on `0.0.0.0:8080` (`go run ./cmd/core_server` does this by default — `:8080`).
* Windows firewall must allow inbound TCP 8080 (Step 7).
* Phone browser must open `http://<LAN-IP>:8080/health` **before** running Flutter (proves LAN path).

> Do not use `localhost` or `10.0.2.2` for a real device. Those only work on emulators/simulators. Do not hardcode a LAN IP in `api_config.dart`; the explicit `--dart-define` is the runtime authority.

### Google Maps / Places Client Config

The canonical mobile Maps/Places config is tracked in
[`apps/mobile/lib/core/src/config/google_config.dart`](../../apps/mobile/lib/core/src/config/google_config.dart).
It contains restricted client-side keys for the app, not backend secrets.
Manage package, bundle, and API restrictions in Google Cloud Console.
Do not copy an example file to create this source.

---

## Step 9 — Admin Panel

```bash
cd apps/admin
cp .env.local.example .env.local
# Edit .env.local with Firebase project values
# VITE_API_BASE_URL defaults to http://localhost:8080

npm install
npm run dev
# Opens on http://localhost:5173
```

Required `.env.local` variables:

| Variable | Where to find |
|---|---|
| `VITE_FIREBASE_API_KEY` | Firebase Console → Project Settings → Web app |
| `VITE_FIREBASE_AUTH_DOMAIN` | Firebase Console |
| `VITE_FIREBASE_PROJECT_ID` | Firebase Console |
| `VITE_FIREBASE_STORAGE_BUCKET` | Firebase Console |
| `VITE_FIREBASE_MESSAGING_SENDER_ID` | Firebase Console |
| `VITE_FIREBASE_APP_ID` | Firebase Console |
| `VITE_API_BASE_URL` | Set to `http://localhost:8080` (default if omitted) |

The first time you run the admin panel, you must bootstrap an admin user via SQL. See [`admin-bootstrap.md`](./admin-bootstrap.md).

---

## Owner Startup Checklist

Run through this list before starting an owner test session:

- [ ] Docker running — `docker ps` shows `labuda-postgres` and `labuda-redis` as Up
- [ ] Migrations applied — `go run ./cmd/migrate` completed with `MIGRATION COMPLETE`
- [ ] Seeder run — `go run ./cmd/seed` completed (skip if already seeded)
- [ ] Backend started — `go run ./cmd/core_server` running
- [ ] Laptop health check — `http://localhost:8080/health` returns `{"status":"healthy",...}`
- [ ] Phone health check — `http://<LAN-IP>:8080/health` returns healthy (real device only)
- [ ] Mobile `ApiConfig` — emulator: default `10.0.2.2` (no flag); real device: explicit `--dart-define=API_BASE_URL=http://<LAN-IP>:8080/api/v1 --dart-define=API_WS_URL=ws://<LAN-IP>:8080/api/v1/ws`
- [ ] Windows firewall rule added for port 8080 (real device only, if on Public network)
- [ ] Mobile app cold-started after backend restart — stale connections cleared
- [ ] Admin panel running — `http://localhost:5173` loads login page
- [ ] Admin user bootstrapped — admin can log in and see full sidebar

> If login hangs: check backend logs for `/auth/firebase` or `/users/sync` errors.

---

## Troubleshooting

| Problem | Fix |
|---|---|
| `ping failed: dial tcp: connection refused` | Start Postgres: `docker-compose up -d`. Wait 10s for health checks to pass. |
| `Failed to read migrations directory` | Run `go run ./cmd/migrate` from the `backend/` directory, not from repo root. |
| `pq: relation "schema_migrations" does not exist` | Run migrations before starting the server: `cd backend && go run ./cmd/migrate`. |
| Migration version stays at 0 or wrong number | Do not use `make migrate-up` — it uses a different toolchain that corrupts the migration state. Drop the `schema_migrations` table and rerun `go run ./cmd/migrate` on a clean DB. |
| `Firebase: could not fetch token` | Check `FIREBASE_SERVICE_ACCOUNT_KEY_PATH` in `.env` and that the JSON file exists. Or set `DEV_MOCK_FIREBASE_AUTH=true` to bypass Firebase for local dev. |
| Port 8080 in use | Change `PORT=` in `backend/.env` and update `api_config.dart` to match. |
| Phone cannot open `/health` | (1) Check both laptop and phone are on the same Wi-Fi network. (2) Add Windows firewall rule (Step 7). (3) Confirm laptop LAN IP is correct (`ipconfig`). |
| Login spins forever on mobile | Backend is unreachable. Verify phone can open `/health` in browser. Check `--dart-define=API_BASE_URL` value. |
| Feed returns 500 | Migrations not fully applied. Run `go run ./cmd/migrate` and restart backend. |
| WebSocket fails / notifications not received | WS URL must use `ws://` (not `wss://`) for local dev and must match the same IP as `API_BASE_URL`'s host (`--dart-define=API_WS_URL=ws://<LAN-IP>:8080/api/v1/ws`). |
| `npm run dev` fails (admin) | Run `npm install` first. Check Node.js ≥ 20: `node --version`. |
| Firebase warning noise in Flutter logs | `[firebase_core]` or `[FirebaseCore]` warnings about `GoogleService-Info.plist` or `google-services.json` are expected in debug mode. They are not errors unless login itself fails. |
| Seed duplicates content | Expected — content/comments use random IDs on each seed run. Users are upserted (no duplicates). Running seed twice gives 50 content items, which is fine for testing. |
| Midtrans webhook not delivered | Use a public URL for webhook delivery, or use the dev hot-arm endpoint (requires `DEV_SKIP_PAYMENT_GATEWAY=false` and `DEV_MODE=true`): `POST http://localhost:8080/dev/webhooks/midtrans/arm` with `{"external_id": "<id>"}`. |

---

## Dev-Only Bypass Flags

These are safe for local dev but must never be set in staging or production:

| Flag | Effect |
|---|---|
| `DEV_MOCK_FIREBASE_AUTH=true` | Bypasses Firebase token validation. All requests are accepted as authenticated. |
| `DEV_AUTO_APPROVE_VERIFICATION=true` | Auto-approves seller KTP verification submissions. |
| `DEV_SKIP_PAYMENT_GATEWAY=false` | Keep `false` to use Midtrans sandbox. Set `true` only to skip payment entirely (not realistic). |
