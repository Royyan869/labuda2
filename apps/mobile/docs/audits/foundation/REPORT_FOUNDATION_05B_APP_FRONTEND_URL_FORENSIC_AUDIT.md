# FOUNDATION-05B — APP_URL / FRONTEND_URL Forensic Audit

**Date:** 2026-09-01
**Scope:** Trace every runtime consumer of `APP_URL` and `FRONTEND_URL`
**Verdict:** FOUNDATION-05B — PASS

---

## 1. Executive Verdict

| Config | Default | Consumers | Active Runtime Impact | Security Risk | Verdict |
|---|---|---|---|---|---|
| `APP_URL` | `http://localhost:8080` | **ZERO** | NONE | NONE | DEAD CONFIG — cleanup candidate |
| `FRONTEND_URL` | `http://localhost:3000` | **3** (all payment) | Midtrans Snap finish redirect | NONE | OPERATIONAL — P2 at worst if misconfigured |

**Neither URL participates in authentication, CORS, email, OAuth, account recovery, webhook validation, or any security boundary.** `APP_URL` is loaded but never read. `FRONTEND_URL` is a payment UX convenience — if misconfigured, users see a broken page after payment, but no funds or credentials are affected.

---

## 2. APP_URL — Dead Configuration

### 2.1 Authority Chain

```
environment (APP_URL)
    ↓
config.Load() — config.go:374
    ↓
Config.App.URL = "http://localhost:8080" (default)
    ↓
??? — ZERO consumers
```

### 2.2 Evidence

| Surface | Result |
|---|---|
| `cfg.App.URL` in Go source | **ZERO references** after config loading |
| `AppURL`, `app_url`, `appUrl` | **ZERO references** in entire codebase |
| Mobile `ApiConfig` | Independent `--dart-define` mechanism — does NOT read `APP_URL` |
| Admin `VITE_API_BASE_URL` | Independent build-time variable — does NOT read `APP_URL` |
| Email generation | **No email sending code exists** in entire backend |
| OAuth/redirect | **No OAuth flow exists** — Firebase handles auth client-side |
| Share/OG metadata | Uses `http.Request` context, not `APP_URL` config |

### 2.3 Classification

```
APP_URL = PROVEN DEAD
```

**APP_URL is loaded into config but never consumed by any runtime code.** It is a vestigial configuration field from an earlier architecture phase. The `validateProductionConfig()` function does not validate it. Production would silently accept the localhost default with zero consequence.

---

## 3. FRONTEND_URL — Payment Finish Redirect

### 3.1 Authority Chain

```
environment (FRONTEND_URL)
    ↓
config.Load() — config.go:375
    ↓
Config.App.FrontendURL = "http://localhost:3000" (default)
    ↓
3 active consumers (all payment)
```

### 3.2 Consumer Inventory

| # | Consumer | File:Line | Purpose | Semantic |
|---|---|---|---|---|
| 1 | `SellerHandler` (subscription retry) | `seller_handler.go:516-518` | Snap `Callbacks.Finish` for subscription payment retry | PAYMENT_RETURN_URL |
| 2 | `SellerHandler` (new subscription) | `seller_handler.go:584-586` | Snap `Callbacks.Finish` for new subscription payment | PAYMENT_RETURN_URL |
| 3 | `CorePaymentHandler.createPayment` | `dependencies.go:3890` → `midtrans_snap_builder.go:129-131` | Snap `Callbacks.Finish` for order payments | PAYMENT_RETURN_URL |

### 3.3 Exact Runtime Flow

```
Backend sends to Midtrans API:
  SnapRequest.Callbacks.Finish = FRONTEND_URL + "/payment/finish?order_id=<orderID>"
      OR
  SnapRequest.Callbacks.Finish = FRONTEND_URL + "/payment/finish"
```

**What Midtrans does with it:** After payment completion, Midtrans shows a "Finish" button in the Snap payment page. Clicking it redirects the user's browser to `Callbacks.Finish`. This is a **client-side browser redirect**, not a server-side callback.

**What happens at that URL:** The user lands on a web page at `FRONTEND_URL/payment/finish`. This is a user-facing status page — NOT a server webhook. The actual payment confirmation comes via Midtrans → backend webhook (`POST /webhooks/payment/midtrans`), which is independent of `FRONTEND_URL`.

### 3.4 Consumer Detail: SellerHandler

**File:** `backend/internal/commerce/seller/delivery/http/seller_handler.go`

```go
// Line 516-518 (subscription payment retry)
if h.frontendURL != "" {
    snapReq.Callbacks = &midtrans.Callbacks{
        Finish: h.frontendURL + "/payment/finish",
    }
}

// Line 584-586 (new subscription payment — identical pattern)
if h.frontendURL != "" {
    snapReq.Callbacks = &midtrans.Callbacks{
        Finish: h.frontendURL + "/payment/finish",
    }
}
```

**Caller:** `InitiateSubscriptionPayment` and payment retry paths.
**Sent to:** Midtrans Snap API as JSON `callbacks.finish` field.
**Guard:** `if h.frontendURL != ""` — empty value safely omits callback.

### 3.5 Consumer Detail: CorePaymentHandler

**File:** `backend/internal/serverboot/dependencies.go` + `midtrans_snap_builder.go`

```go
// dependencies.go:3890
FrontendURL: h.frontendURL,

// midtrans_snap_builder.go:129-131
if in.FrontendURL != "" {
    callbacks = &midtrans.Callbacks{
        Finish: in.FrontendURL + "/payment/finish?order_id=" + in.MidtransOrderID,
    }
}
```

**Caller:** `CorePaymentHandler.createPayment` → builds Snap request for order payments.
**Sent to:** Midtrans Snap API as JSON `callbacks.finish` field.
**Guard:** `if in.FrontendURL != ""` — empty value safely omits callback.

### 3.6 Key Observation

All 3 consumers produce the same pattern:
```
FrontendURL + "/payment/finish" [+ "?order_id=" + id]
```

The URL is **sent to Midtrans** as a browser redirect target. It is **NOT stored in the database**, **NOT used for server-to-server callbacks**, and **NOT part of webhook verification**.

---

## 4. URL Semantics Matrix

| URL | Semantic | Auth Role | Security Role | Payment Role |
|---|---|---|---|---|
| `APP_URL` | None (dead) | NONE | NONE | NONE |
| `FRONTEND_URL` | PAYMENT_RETURN_URL | NONE | NONE | Post-payment UX redirect |

---

## 5. Security Boundary Analysis

| Security Concern | APP_URL | FRONTEND_URL |
|---|---|---|
| Authentication | NOT USED | NOT USED |
| Authorization | NOT USED | NOT USED |
| OAuth | NOT USED | NOT USED |
| CSRF | NOT USED | NOT USED |
| CORS | NOT USED — `CORS_ALLOWED_ORIGINS` is independent | NOT USED — CORS has its own authority |
| Redirect validation | NOT USED | NOT USED |
| Password reset | NOT USED — no email system exists | NOT USED |
| Email verification | NOT USED — no email system exists | NOT USED |
| Account recovery | NOT USED | NOT USED |
| Webhook validation | NOT USED | NOT USED |
| Session handling | NOT USED | NOT USED |
| Cookie domain | NOT USED | NOT USED |
| Origin validation | NOT USED | NOT USED |
| Signed URLs | NOT USED | NOT USED |

**Both URLs have zero security boundary role.** The backend authentication flow is: Firebase ID token → Go backend verification → JWT. No redirect URLs are involved.

---

## 6. Default Value Analysis

| Variable | Default | Empty Behavior | Production Validation |
|---|---|---|---|
| `APP_URL` | `http://localhost:8080` | Would default to localhost | NONE — `validateProductionConfig()` does not check it |
| `FRONTEND_URL` | `http://localhost:3000` | Empty → callbacks omitted (safe) | NONE — no guard exists |

### What happens if production uses localhost defaults?

- **APP_URL:** Nothing — zero consumers, zero runtime impact
- **FRONTEND_URL:** User clicks "Finish" after payment → browser navigates to `http://localhost:3000/payment/finish` → page fails to load (localhost is the backend server, not the frontend) → **UX broken, no financial or security impact**

### Fail-closed behavior

- `FRONTEND_URL` with empty string: callbacks are nil → Midtrans uses its own default behavior (no "Finish" button shown) → **safe fallback**
- `FRONTEND_URL` with wrong URL: user lands on broken page → **UX failure only**

---

## 7. Environment Cross-Contamination

| Scenario | Possible? | Evidence |
|---|---|---|
| Development APP_URL → Production | NO IMPACT | Zero consumers |
| Development FRONTEND_URL → Production | MINOR RISK | Would cause broken payment finish redirect |
| Production validation exists | NO | Neither variable is validated at boot |

**Assessment:** If someone deploys to production without setting `FRONTEND_URL`, the localhost default causes a broken payment finish redirect. The user sees an error page after payment, but the payment itself succeeds (confirmed by webhook). This is a **P2 operational issue**, not a security or data integrity issue.

---

## 8. Duplicate URL Authority Analysis

| Authority | Value | Source | Purpose |
|---|---|---|---|
| Backend `FRONTEND_URL` | env var | `config.Load()` | Midtrans Snap finish redirect |
| Mobile `ApiConfig.baseUrl` | `--dart-define` or build mode | `api_config.dart` | REST API calls to backend |
| Admin `VITE_API_BASE_URL` | `import.meta.env` | `client.ts` | API calls to backend |

**Assessment:** These are NOT duplicates — they serve completely different purposes:
- `FRONTEND_URL` = where Midtrans redirects the browser after payment
- Mobile `ApiConfig` = where mobile app sends API requests
- Admin `VITE_API_BASE_URL` = where admin web sends API requests

There is no conflict or precedence issue.

---

## 9. Backend URL Generation

| Pattern | Result |
|---|---|
| `fmt.Sprintf("http://...")` hardcoded URLs | Found only in OG metadata rendering (uses `http.Request`, not config) |
| `url.Parse` / `url.URL` | No production URL construction from APP_URL or FRONTEND_URL |
| Redirect/Location headers | Not set by either variable |
| Share links | Mobile generates share URLs independently |
| Callback URLs | Only `FRONTEND_URL` → Midtrans Snap callbacks (detailed above) |

---

## 10. Email Usage

**No email sending code exists in the entire backend.** No SMTP, SendGrid, SES, or email template code was found. Firebase handles email verification client-side (if enabled). All user notifications go through FCM push notifications.

```
APP_URL → no email dependency
FRONTEND_URL → no email dependency
```

---

## 11. Auth / Identity Flows

```
APP_URL → no auth dependency
FRONTEND_URL → no auth dependency
```

Authentication flow: Firebase Auth (mobile) → Firebase ID token → Go backend `AuthMiddleware` → JWT. No redirect URLs, no OAuth, no email links involved.

---

## 12. Payment / Finance Flows

**FRONTEND_URL is the only URL involved in payment:**

| Flow | URL Usage | Impact if Wrong |
|---|---|---|
| Order payment (CorePaymentHandler) | Snap `Callbacks.Finish` = `FRONTEND_URL/payment/finish?order_id=...` | User sees broken page after payment |
| Subscription payment (SellerHandler) | Snap `Callbacks.Finish` = `FRONTEND_URL/payment/finish` | User sees broken page after subscription payment |

**Critical distinction:** The `Callbacks.Finish` is a **browser redirect target**, NOT a server webhook. Payment confirmation flows via Midtrans → `POST /webhooks/payment/midtrans` (independent of `FRONTEND_URL`). Wrong `FRONTEND_URL` does NOT affect:
- Payment settlement
- Order status
- Webhook delivery
- Financial accounting

---

## 13. CORS Relationship

```
APP_URL → not used by CORS
FRONTEND_URL → not used by CORS
```

CORS authority: `CORS_ALLOWED_ORIGINS` env var → `config.CORS.AllowedOrigins` → `CORSMiddleware`. Completely independent.

---

## 14. Mobile Relationship

Mobile defines its own API URLs via `--dart-define=API_BASE_URL` and `--dart-define=API_WS_URL`. The mobile app does NOT read `APP_URL` or `FRONTEND_URL` from the backend config.

---

## 15. Admin Relationship

Admin defines its own API URL via `VITE_API_BASE_URL`. The admin web does NOT read `APP_URL` or `FRONTEND_URL` from the backend config.

---

## 16. Validation Analysis

| Variable | Scheme Check | Host Check | Production Check |
|---|---|---|---|
| `APP_URL` | NONE | NONE | NONE |
| `FRONTEND_URL` | NONE | NONE | NONE |

Neither variable is validated at boot. No scheme, host, or environment-specific guard exists.

---

## 17. Concrete Production Failure Scenarios

### Scenario 1: APP_URL = localhost in production

```
APP_URL = "http://localhost:8080" in production
    ↓
Zero consumers
    ↓
NO MATERIAL IMPACT
```

### Scenario 2: FRONTEND_URL = localhost in production

```
FRONTEND_URL = "http://localhost:3000" in production
    ↓
User completes payment on Midtrans Snap page
    ↓
Clicks "Finish" button
    ↓
Browser navigates to http://localhost:3000/payment/finish
    ↓
Page fails to load (wrong host)
    ↓
Payment already confirmed via webhook — no financial impact
    ↓
User confused but transaction complete
    ↓
P2 — UX issue only
```

### Scenario 3: FRONTEND_URL = empty in production

```
FRONTEND_URL = "" (unset)
    ↓
All 3 consumers: if h.frontendURL != "" { ... }
    ↓
Callbacks = nil
    ↓
Midtrans Snap shows no "Finish" button
    ↓
User returns to Midtrans payment result page (default behavior)
    ↓
P2 — minor UX difference, no financial impact
```

### Scenario 4: FRONTEND_URL = wrong domain

```
FRONTEND_URL = "https://wrong-domain.com"
    ↓
User clicks "Finish" after payment
    ↓
Browser navigates to wrong-domain.com/payment/finish
    ↓
Page may 404 or show irrelevant content
    ↓
Payment confirmed via webhook — no financial impact
    ↓
P2 — UX issue only
```

---

## 18. P0/P1/P2 Findings

### P0: NONE
### P1: NONE

### P2: ONE

| Finding | Component | Impact | Classification |
|---|---|---|---|
| `FRONTEND_URL` has no production validation | Backend config | If unset/misconfigured, payment finish redirect is broken (UX only, no financial impact) | P2 — operational |

### Cleanup Candidate

| Item | Classification | Rationale |
|---|---|---|
| `APP_URL` field in `AppConfig` struct | PROVEN DEAD | Zero consumers — loaded but never read |

---

## 19. Owner Decisions Required

**None.** Both findings are engineering decisions:
- `APP_URL` dead code → cleanup candidate (engineering decision to delete)
- `FRONTEND_URL` missing guard → optional P2 hardening (engineering decision to add validation)

The owner does not need to make business decisions about these values.

---

## 20. Cleanup Candidates

| # | Item | Phase | Priority |
|---|---|---|---|
| 1 | Remove `URL` field from `AppConfig` struct + `APP_URL` from `config.Load()` + `backend/.env.example` + `.env.example` | Dead code removal | LOW |
| 2 | Add optional `FRONTEND_URL` production validation (scheme=https, non-empty) | P2 hardening | LOW |

---

## 21. Verification Gaps

| Gap | Impact |
|---|---|
| Cannot verify Midtrans Snap `Callbacks.Finish` behavior without live Midtrans sandbox test | Low — documentation confirms it's a browser redirect |
| Cannot verify mobile payment flow end-to-end without device | Low — mobile doesn't use `FRONTEND_URL` |

---

## 22. Final Answer

> **If Labuda went to production using the current configuration architecture, could an incorrect `APP_URL` or `FRONTEND_URL` cause a major post-production architectural/code change, or is it merely an operational configuration value?**

**It is merely an operational configuration value.**

- `APP_URL` has **zero consumers**. It cannot cause any production impact whatsoever. It is dead config.
- `FRONTEND_URL` controls only the Midtrans Snap payment finish redirect — a browser-side UX convenience. If misconfigured, the user sees a broken page after payment, but the payment is confirmed via server-side webhook independent of this URL. No financial, security, or architectural consequence.

Neither URL requires a structural code change for production. `FRONTEND_URL` needs an operational value (the real production frontend domain) set as an environment variable, which is standard deployment configuration.

---

*Generated by FOUNDATION-05B forensic audit. No code was modified.*
