# FOUNDATION PASS 01 — SECURITY BOUNDARY AUDIT

**Date:** 2026-09-01
**Baseline commit:** `d25467c`
**Verdict:** **PASS WITH FINDINGS**

---

## 1. Scope Coverage

All 10 scope areas were traced exhaustively:
1. CORS ✅
2. HTTP Exposure ✅
3. Authentication ✅
4. Authorization ✅
5. Admin ✅
6. Webhooks ✅
7. Upload/Storage ✅
8. Secrets/Config ✅
9. Rate Limiting ✅
10. Sensitive Logging ✅

---

## 2. Findings

### 2.1 DELETED: Dead CORS implementation

**File:** `internal/middleware/cors.go` (DELETED)
- **Evidence:** Zero importers across the entire codebase. The runtime CORS is `cmd/core_server/middleware.go:CORSMiddleware` which uses `cfg.CORS.AllowedOrigins` with production-safe logic.
- **What it was:** A broken CORS middleware that reflected any origin back with credentials (`Access-Control-Allow-Origin: <origin>` + `Access-Control-Allow-Credentials: true`). It was never mounted.
- **Why safe to delete:** `cmd/core_server/main.go:436` uses `CORSMiddleware(cfg)` from `cmd/core_server/middleware.go` — completely independent implementation.

### 2.2 DELETED: Dead `HasRole`/`IsAdmin`/`GetUserRolesFromContext`

**File:** `internal/middleware/role_middleware.go` — removed `HasRole()` and `IsAdmin()` helpers (15 lines)
**File:** `internal/middleware/auth.go` — removed `GetUserRolesFromContext()` (12 lines)
- **Evidence:** Zero callers across the entire codebase (verified via negative search). These checked `UserClaims.Roles` which is explicitly documented as "NON-AUTHORITATIVE" (auth.go:35).
- **Why safe to delete:** The canonical authorization uses `RequireAdminMiddleware` → `RoleChecker.IsAdmin()` which queries PostgreSQL directly. The helpers were dead code.

### 2.3 FIXED: Dev config flags not validated in production

**File:** `internal/config/config.go` — added STEP 3.5 to `ValidateProductionSafety()`
**File:** `internal/config/config_test.go` — added 4 regression tests

**Before:** `Dev.MockFirebaseAuth`, `Dev.AutoApproveVerification`, `Dev.SkipPaymentGateway` could be enabled in production without any guard. If `DEV_MOCK_FIREBASE_AUTH=true` was accidentally set in production, Firebase authentication would be bypassed entirely.

**After:** `ValidateProductionSafety()` now panics if any `Dev.*` flag is `true` when `ENV=production`.

Tests added:
- `TestValidateProductionSafety_MockFirebaseAuth_PanicsInProduction`
- `TestValidateProductionSafety_AutoApproveVerification_PanicsInProduction`
- `TestValidateProductionSafety_SkipPaymentGateway_PanicsInProduction`
- `TestValidateProductionSafety_DevFlagsFalse_DoesNotPanic`

---

## 3. Scope-by-Scope Analysis

### 3.1 CORS

| Aspect | Evidence |
|--------|----------|
| Runtime implementation | `cmd/core_server/middleware.go:CORSMiddleware` — config-aware, production-safe |
| Production behavior | Only exact-match origins from `CORS_ALLOWED_ORIGINS`; unknown origins → 403 |
| Wildcard behavior | Only allowed in non-production; blocked by `ValidateProductionSafety()` in production |
| Dead implementation | `internal/middleware/cors.go` — **DELETED** (zero importers) |
| CORS config defaults | `AllowedOrigins: ["*"]` — but production validation blocks wildcard |

### 3.2 HTTP Exposure

| Endpoint | Protection | Status |
|----------|-----------|--------|
| `/health` | None (intentional — liveness probe) | CORRECT |
| `/health/ready` | None (intentional — readiness probe) | CORRECT |
| `/health/live` | None (intentional — liveness probe) | CORRECT |
| `/health/system` | None — returns system health metrics | ACCEPTABLE (no sensitive data) |
| `/metrics` | None — Prometheus metrics | ACCEPTABLE (intentionally public for monitoring) |
| `/webhooks/payout/health` | None — payout webhook health | CORRECT |
| OG endpoints (`/og/*`) | None — public metadata for share links | CORRECT |

### 3.3 Authentication

| Component | Implementation | Status |
|-----------|---------------|--------|
| Firebase token verification | `AuthMiddleware` → `firebase.VerifyIDToken()` | CANONICAL |
| Optional auth | `OptionalAuthMiddleware` → same verification, non-blocking | CORRECT |
| Browse auth | `StrictBrowseAuthMiddleware` → anonymous allowed, malformed token → 401 | CORRECT |
| Token refresh | `RefreshToken` → JWT validation + rotation + reuse detection | CORRECT |
| Banned user rejection | `AuthMiddleware` checks `claims.Banned` → 403 | CORRECT |
| Email verification | `RequireActiveAccount` checks `EmailVerified` | CORRECT |
| User provisioning | `UserLookupMiddleware` → DB lookup, rejects unprovisioned users | CORRECT |
| Duplicate auth authority | NONE — single Firebase token verification path | CLEAN |

### 3.4 Authorization

| Layer | Implementation | Status |
|-------|---------------|--------|
| Admin RBAC | `RequireAdminMiddleware` → `RoleChecker.IsAdmin()` (DB query) | CANONICAL |
| Seller authority | `RequireSellerMiddleware` → `RoleChecker.HasActiveSellerCapability()` | CANONICAL |
| Capability-based | `RequireCapability` / `RequireAnyCapability` / `RequireAllCapabilities` | CANONICAL |
| Actor context | `ActorContextInject` → resolves Actor with capabilities | CANONICAL |
| Admin group protection | ALL admin routes: `RequireActiveAccount` + `RequireAdminMiddleware` + individual `RequireCapability` | DUAL PROTECTION |
| `UserClaims.Roles` | Documented as "NON-AUTHORITATIVE" (auth.go:35), never used for authorization | CLEAN |
| Dead helpers | `HasRole()`/`IsAdmin()` — **DELETED** | CLEANED |
| Fallback to admin | `RequireCapability` explicitly does NOT fall back to admin role | SECURE |

### 3.5 Admin

| Aspect | Evidence |
|--------|----------|
| Route group protection | `adminRoutes.Use(RequireActiveAccount) + RequireAdminMiddleware` | 
| Sensitive operations | Individual `RequireCapability` on each (dual protection) |
| Audit trail | Governance audit events recorded for Case/Decision/Enforcement mutations |
| `/admin/test` endpoint | Protected by group middleware; returns non-sensitive status |
| Dev-only routes | Guarded by `cfg.IsDevelopment()` — not mounted in production |
| Dev-only webhook tools | Guarded by `cfg.IsDevelopment()` + `RequireCapability` |

### 3.6 Webhooks

| Endpoint | Authentication | Status |
|----------|---------------|--------|
| `/webhooks/payment/midtrans` | `VerifySignature` (HMAC) + `webhookDrop` (dev-only) | SECURE |
| `/webhooks/payout` | `PayoutWebhookVerifier.VerifySignature` — fail-closed | SECURE |
| Payout: no secret key | Rejects ALL webhooks (explicit error) | FAIL-CLOSED |
| Payout: missing signature header | Rejects with 401 | FAIL-CLOSED |
| Payout: bad signature | Rejects | FAIL-CLOSED |
| Payout: tampered payload | Rejects | FAIL-CLOSED |
| Dev replay endpoint | `cfg.IsDevelopment()` guard | SECURE |
| Webhook drop filter | `cfg.IsDevelopment() + WEBHOOK_DROP_ENABLED=true` double guard | SECURE |

### 3.7 Upload/Storage

| Aspect | Evidence |
|--------|----------|
| Media upload | Presigned S3 PUT URL (15 min TTL) via `s3presign` package |
| KYC upload | Presigned S3 PUT URL (15 min TTL) with content-type validation |
| KYC view | Presigned S3 GET URL (5 min TTL) for admin viewing only |
| Storage key ownership | `mediaupload` handler validates owner prefix |
| Arbitrary access | Not possible — presigned URLs are scoped to specific keys + content types |
| KYC namespace isolation | `kyc/` prefix enforced, general upload namespace rejected for KYC |

### 3.8 Secrets/Config

| Secret | Production Validation | Status |
|--------|----------------------|--------|
| JWT_SECRET | Blocks known insecure defaults (`change-me-in-production`) | VALIDATED |
| DB_PASSWORD | Blocks known insecure defaults (`labuda123`) | VALIDATED |
| DB_SSLMODE | Must be `require`/`verify-ca`/`verify-full` in production | VALIDATED |
| MIDTRANS_SERVER_KEY | Required in production | VALIDATED |
| FIREBASE_PROJECT_ID | Required in production | VALIDATED |
| PAYOUT_SECRET_KEY | Required when `PAYOUT_ENVIRONMENT=production` | VALIDATED |
| PAYOUT_ENABLE_PRODUCTION | Required for real payouts | VALIDATED |
| PAYOUT_GATEWAY_PROVIDER | Required in non-development | VALIDATED |
| GIN_MODE | Must be `release` in production | VALIDATED |
| CORS_ALLOWED_ORIGINS | Wildcard (`*`) blocked in production | VALIDATED |
| Dev flags | **NEWLY VALIDATED:** `MockFirebaseAuth`, `AutoApproveVerification`, `SkipPaymentGateway` now panic in production | FIXED |

### 3.9 Rate Limiting

| Aspect | Evidence |
|--------|----------|
| Implementation | In-memory per-IP token bucket (`golang.org/x/time/rate`) |
| Purpose | Local load protection (not security authority) |
| Scope | Applied globally to all routes |
| Config | Default 100 req/s with burst 200 |
| Assessment | Local-only rate limiter is appropriate for a single-instance server. Not a security boundary. Production would need edge/CDN rate limiting for DDoS protection, which is outside this scope. |
| Cleanup needed | None |

### 3.10 Sensitive Logging

| Check | Evidence | Status |
|-------|----------|--------|
| Auth tokens | Never logged (auth errors only log `zap.Error(err)`) | CLEAN |
| FCM tokens | Truncated to first 20 chars + "..." (firebase.go:216,222,229) | SAFE |
| Payment credentials | Never logged (Midtrans keys not in any log statement) | CLEAN |
| Firebase credentials | Service account path logged once at startup (not the key itself) | CLEAN |
| Webhook secrets | Never logged (only "no secret configured" warnings) | CLEAN |
| Pricing tokens | Logged as opaque IDs (not secrets) | CLEAN |
| JWT secret | Never logged | CLEAN |
| DB password | Never logged | CLEAN |
| KYC documents | Never logged (only storage keys) | CLEAN |

---

## 4. Canonical Security Architecture (Post-Cleanup)

```
Request → Recovery → RequestID → Logger → CORS (cmd/core_server/middleware.go)
         → RateLimit → StrictBrowseAuth | Auth → UserLookup → RolesLookup → ActorContextInject
         → [RequireCapability] → Handler
```

**Single CORS authority:** `cmd/core_server/middleware.go:CORSMiddleware`
**Single auth authority:** `internal/middleware/auth.go:AuthMiddleware`
**Single admin authority:** `internal/middleware/role_middleware.go:RequireAdminMiddleware`
**Single capability authority:** `internal/middleware/capability_middleware.go:RequireCapability`
**Single config validation:** `internal/config/config.go:ValidateProductionSafety()`

No duplicate authorities. No fallback grants. No zombie middleware.

---

## 5. Files Changed

| File | Change | Lines |
|------|--------|-------|
| `internal/middleware/cors.go` | DELETED | -32 |
| `internal/middleware/role_middleware.go` | Removed `HasRole`/`IsAdmin` | -15 |
| `internal/middleware/auth.go` | Removed `GetUserRolesFromContext` | -12 |
| `internal/config/config.go` | Added Dev.* production guards | +11 |
| `internal/config/config_test.go` | Added 4 regression tests | +61 |
| **Total** | | +72 -59 |

---

## 6. Files Deleted

| File | Reason |
|------|--------|
| `internal/middleware/cors.go` | Dead code — zero importers, runtime uses `cmd/core_server/middleware.go` |

---

## 7. Test Results

| Suite | Result |
|-------|--------|
| `go build ./...` | ✅ PASS |
| `go test ./internal/middleware/...` | ✅ PASS (0.211s) |
| `go test ./internal/config/...` | ✅ PASS (0.481s, includes 4 new tests) |
| `go test ./internal/config/... -run TestValidateProductionSafety` | ✅ PASS (8/8) |
| `npx vitest run` (admin) | ✅ PASS (96/96) |

---

## 8. Negative Search Evidence

| Search | Results |
|--------|---------|
| `internal/middleware/cors` in *.go | 0 matches |
| `middleware.HasRole(` in *.go | 0 matches |
| `middleware.IsAdmin(` in *.go | 0 matches |
| `GetUserRolesFromContext` in *.go | 0 matches (only definition removed) |

All deleted artifacts have zero remaining references. No compilation breakage. No dead references.

---

## 9. Explicit Security Statements

1. **No obsolete security implementation remains.** The only dead CORS middleware (`internal/middleware/cors.go`) has been deleted. The only dead authorization helpers (`HasRole`/`IsAdmin`/`GetUserRolesFromContext`) have been deleted.

2. **The canonical security design is the ONLY active design.** CORS, Auth, RBAC, and Capability middleware are each implemented exactly once and used exclusively by the runtime.

3. **Production fail-closed is enforced.** `ValidateProductionSafety()` now validates ENV, DB SSL, Gin mode, CORS, payout, and Dev flags — panicking on any unsafe configuration.

4. **No duplicate authority exists.** Each security concern has exactly one implementation: CORS → `cmd/core_server/middleware.go`, Auth → `auth.go`, RBAC → `role_middleware.go`, Capabilities → `capability_middleware.go`.

5. **No zombie code can be resurrected.** Deleted files have zero importers. Removed functions have zero callers. The negative search confirms clean removal.

---

## 10. Remaining Non-Blocking Observations

These are NOT defects — they are acknowledged design decisions:

| Observation | Assessment |
|-------------|-----------|
| Rate limiting is in-memory (local only) | Appropriate for single-instance; DDoS protection belongs at edge/CDN |
| `/health/*` endpoints are unauthenticated | Intentional — liveness/readiness probes must be accessible by orchestrators |
| `/metrics` is unauthenticated | Intentional — Prometheus scraping pattern |
| `InternalAPIKey` in config but unused in routes | Config field exists for future service-to-service auth; not a security risk |
| JWT_SECRET default `"change-me-in-production"` | Has production validation that blocks this default; weak but present |

---

## 11. Scope Status

**SCOPE IS CLOSED.**

All 10 scope areas have been exhaustively traced, the canonical security architecture is verified as the sole active implementation, all dead artifacts have been deleted, and the production safety validation has been hardened with new regression tests.
