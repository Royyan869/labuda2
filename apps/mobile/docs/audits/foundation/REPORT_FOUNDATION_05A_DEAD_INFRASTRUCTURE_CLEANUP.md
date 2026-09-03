# FOUNDATION-05A — Dead Infrastructure & Configuration Residue Cleanup

**Date:** 2026-09-01
**Verdict:** FOUNDATION-05A — CLOSED
**Scope:** Delete infrastructure proven dead by FOUNDATION-05 audit and locked foundation truths

---

## 1. Executive Summary

All dead infrastructure identified by FOUNDATION-05 has been removed. The repository now contains only infrastructure with a legitimate current purpose.

**Deleted:**
- `infra/firebase/` — dead Firestore rules + Cloud Functions deploy scripts (6 files)
- `render.yaml` — Render staging/demo Blueprint
- `backend/cmd/staging_rollout_ab/` — staging rollout tool (1 file, hardcoded credentials)
- `backend/scripts/` — 32 one-off developer utility scripts with zero active consumers
- `backend/validation/` — 4 validation scripts with zero active consumers

**Cleaned:**
- `backend/docker-entrypoint.sh` — removed Render-specific `derive_db()`/`derive_redis()` fallback functions and Render comments
- `backend/Dockerfile` — removed Render-specific comments
- `infra/README.md` — removed dead Firebase/Docker sections
- `.gitignore` — removed stale `.firebaserc`, `infra/firebase/`, `infra/docker/` entries

**Regression:** All validation passes — Go build, config tests, Flutter pub get, Dart analyze.

---

## 2. Deleted Artifacts

### 2.1 `infra/firebase/` (6 files)

| File | Purpose | Why Dead | Active Consumers |
|---|---|---|---|
| `firebase.json` | Firestore rules + Cloud Functions config | Firestore and Cloud Functions are dead (Truth #9, #13) | ZERO |
| `README.md` | Firebase infrastructure documentation | References deleted Firestore/Functions | ZERO |
| `scripts/deploy-seller-handler.sh` | Deploy Cloud Function | Cloud Functions are dead (Truth #13) | ZERO |
| `scripts/deploy-webhook.sh` | Deploy Cloud Function | Cloud Functions are dead (Truth #13) | ZERO |
| `scripts/DEPLOY_NOW.sh` | Deploy all Cloud Functions | Cloud Functions are dead (Truth #13) | ZERO |
| `scripts/DEPLOY_SELLER_FIX.sh` | Deploy seller fix Cloud Function | Cloud Functions are dead (Truth #13) | ZERO |

**Evidence:** Zero imports, zero callers, zero CI references, zero Makefile targets. The only references to `infra/firebase/` in the entire repository were within `infra/README.md` (now cleaned) and historical audit reports (preserved).

### 2.2 `render.yaml` (1 file)

| File | Purpose | Why Dead | Active Consumers |
|---|---|---|---|
| `render.yaml` | Render Blueprint for staging/demo deployment | Render is not current architecture (Truth #14-16) | ZERO |

**Evidence:** Zero active references from Go code, Makefiles, CI, or scripts. The only references were in `docker-entrypoint.sh` (now cleaned) and historical audit reports (preserved).

### 2.3 `backend/cmd/staging_rollout_ab/` (1 file)

| File | Purpose | Why Dead | Active Consumers |
|---|---|---|---|
| `main.go` | Staging rollout execution tool (Phase A verifier + Phase B reconciliation) | Tied to Render staging rollout mechanism | ZERO |

**Evidence:** Hardcoded DSN `postgres://labuda:labuda123@localhost:5432/labuda`. Zero imports, zero callers, zero CI references. The staging rollout mechanism was part of the Render deployment architecture.

### 2.4 `backend/scripts/` (32 files)

| Category | Files | Purpose |
|---|---|---|
| DB inspection | `db_inspect.go`, `check_constraints.go`, `check_outbox_schema.go`, `check_users_schema.go`, `check_db_columns.go`, `check_subscription.go` | One-off schema inspection |
| DB modification | `reset_db.go`, `create_test_users.go`, `add_role_column.go`, `fix_subscription.go`, `comprehensive_cleanup.go` | One-off data operations |
| Identity | `lock_identity.go`, `lock_identity_safe.go` | Identity lock operations |
| Validation | `api_flow_validation.go`, `negotiation_final_validation.go`, `validate_safe_conversion.go` | One-off validation runs |
| Audit | `media_inventory_audit.go`, `verify_financial_boundary.go`, `verify_json_fix.go`, `verify_wallet_authority.go` | One-off audit scripts |
| Shell | `api_flow_validation.sh`, `quick_start_validation.sh`, `test_notification_flow.sh`, `test_stuck_event_recovery.sh` | Shell script wrappers |
| Documentation | 3 markdown files, go.mod, go.sum | Supporting artifacts |

**Evidence:** The `backend/scripts/` directory has its own `go.mod` (separate Go module `validation`). Zero external references from CI, Makefile, or any active code. These are standalone one-off developer utility scripts with hardcoded `labuda:labuda123@localhost` DSNs.

### 2.5 `backend/validation/` (4 files)

| File | Purpose |
|---|---|
| `query_db.go` | One-off DB query tool |
| `README.md` | Documentation |
| `run_all_flows.ps1` | PowerShell validation runner |
| `run_all_flows.sh` | Shell validation runner |

**Evidence:** Zero external references from CI, Makefile, or any active code.

---

## 3. Configuration Cleanup

### 3.1 `backend/docker-entrypoint.sh`

**Removed:**
- `derive_db()` function (parsed `RENDER_DATABASE_URL` into `DB_*` variables)
- `derive_redis()` function (parsed `RENDER_REDIS_URL` into `REDIS_*` variables)
- Render-specific conditional fallback logic
- All `RENDER_*` variable references
- Render-specific comments

**Preserved:**
- Firebase credential materialization (1 → 3)
- Migration startup (2 → 2)
- HTTP server start (3 → 3)

**Result:** Entrypoint reduced from 4 sections to 3, all Render-specific logic removed.

### 3.2 `backend/Dockerfile`

**Removed:**
- Comment: "Labuda backend runtime image (Render staging/demo)." → "Labuda backend runtime image."
- Comment: "Build context: backend/ (see dockerContext in render.yaml)." → "Build context: backend/."
- Comment: "The application reads PORT (injected by Render)" → "The application reads PORT (injected by the environment)"

**Preserved:** All build logic, multi-stage build, non-root user, CA certificates.

### 3.3 `infra/README.md`

**Removed:** Entire Firebase section (setup, deploy functions, deploy Firestore rules), Docker section (referenced non-existent `infra/docker/`), AWS future section.

**Result:** Reduced to minimal structure showing only `infra/prometheus/`.

### 3.4 `.gitignore`

**Removed:**
- `.firebaserc` — dead Firebase project alias file
- `.firebase/` — dead Firebase cache directory
- `infra/firebase/functions/node_modules/` — dead Firebase Functions dependencies
- `infra/firebase/functions/lib/` — dead Firebase Functions build output
- `infra/docker/data/` — referenced non-existent directory

---

## 4. Negative Search Results

| Pattern | Active Source References | Historical Report References |
|---|---|---|
| `RENDER_DATABASE_URL` | **ZERO** | Allowed (audit reports) |
| `RENDER_REDIS_URL` | **ZERO** | Allowed (audit reports) |
| `render.yaml` | **ZERO** | Allowed (audit reports) |
| `infra/firebase` | **ZERO** | Allowed (audit reports) |
| `firebase deploy` | **ZERO** | Allowed (audit reports) |
| `firebase functions` | **ZERO** | Allowed (audit reports) |
| `firestore.rules` | **ZERO** | Allowed (audit reports) |
| `staging_rollout` | **ZERO** | Allowed (audit reports) |

**All dead references removed from active source code, configuration, and scripts.**

---

## 5. Regression Results

| Check | Result |
|---|---|
| `go build ./cmd/core_server/` | ✅ PASS — clean build |
| `go test ./internal/config/... -count=1` | ✅ PASS — 11/11 tests pass |
| `flutter pub get` | ✅ PASS — dependencies resolved |
| `dart analyze lib/` | ✅ PASS — zero errors, zero warnings, 40 pre-existing info-level style issues |

---

## 6. Remaining Artifacts

| Artifact | Status | Justification |
|---|---|---|
| `firebase_options.dart` `storageBucket` field | PRESENT | Auto-generated by FlutterFire CLI — inert, no code reads it |
| Historical audit reports | PRESERVED | Evidence only (Truth #19) — never modification authority |
| `backend/.gitignore` Firebase entries | PRESENT | Active — protects `firebase-service-account.json` from being committed |
| `docker-compose.yml` (root) | PRESENT | Active — local development Postgres/Redis |
| `ops/prometheus/` | PRESENT | Active — monitoring configuration |

---

## 7. Remaining Risks

**None identified.** All deleted artifacts had zero active consumers. The production safety architecture is unchanged. Firebase Auth, FCM, and Analytics remain canonical.

---

## 8. Working Tree Status

**Deleted:**
- `infra/firebase/` (6 files)
- `render.yaml` (1 file)
- `backend/cmd/staging_rollout_ab/main.go` (1 file)
- `backend/scripts/` (32 files)
- `backend/validation/` (4 files)

**Modified:**
- `backend/docker-entrypoint.sh` — Render-specific logic removed
- `backend/Dockerfile` — Render comments cleaned
- `infra/README.md` — dead Firebase/Docker sections removed
- `.gitignore` — stale entries removed

**Unchanged:**
- All Go source code (zero imports of deleted modules)
- All Flutter/Dart source code
- All active configuration
- All test files
- All historical audit reports

---

## 9. Final Verdict

```
FOUNDATION-05A — CLOSED
```

### Closure Gate Verification

| Gate | Status |
|---|---|
| Dead Firebase infrastructure deleted | ✅ `infra/firebase/` removed |
| Render infrastructure deleted | ✅ `render.yaml` removed |
| Staging rollout tool deleted | ✅ `backend/cmd/staging_rollout_ab/` removed |
| Dead developer scripts deleted | ✅ `backend/scripts/` + `backend/validation/` removed |
| Docker entrypoint Render logic removed | ✅ derive_db/derive_redis removed |
| No active references remain | ✅ Negative search: ZERO dead references |
| Build passes | ✅ `go build` clean |
| Tests pass | ✅ Config tests 11/11 |
| Mobile validation passes | ✅ `flutter pub get` + `dart analyze` clean |
| Historical audit evidence preserved | ✅ All audit reports intact |
| No new infrastructure introduced | ✅ Zero new files |
| No replacement services added | ✅ Zero new dependencies |

---

*Generated by FOUNDATION-05A cleanup. All deletions are evidence-based with zero active consumer loss.*
