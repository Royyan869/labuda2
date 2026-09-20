# Labuda Infrastructure

## Payment callback endpoint ownership (INFRA-1)

The Midtrans payment callback endpoint is **payment infrastructure**, not a UI
callback. Payment completion is:

```
gateway → verified webhook → canonical finalization → Payment/Order/Subscription state
```

If the callback endpoint is unreachable, the gateway cannot confirm payment and
Labuda cannot settle it. Proven in PAY-INFRA-01: the configured callback host
resolved to NXDOMAIN, `payment_webhook_events` was empty, and every subscription
payment expired while still pending.

### The one canonical endpoint contract

```
https://<stable-public-host>/webhooks/payment/midtrans
```

* The **path** is defined exactly once, in code:
  `backend/cmd/core_server/routes_core.go` → `canonicalPaymentWebhookPath`.
  That same constant is used both to mount the route and to validate the
  configured target, so the configured target cannot point at a route that does
  not exist.
* The route is mounted without auth (Midtrans calls it) and is always mounted,
  before the authenticated route groups.

### The one configuration authority

| Item | Value |
|---|---|
| Configuration key | `MIDTRANS_NOTIFICATION_URL` |
| Defined in | `backend/.env` (gitignored). Template: `backend/.env.example` |
| Read by | `internal/config` → `config.Midtrans.NotificationURL` |
| Consumed by | `pkg/midtrans` — sent per transaction as the `Notification-Url` header on `POST /snap/v1/transactions` |
| Validated by | `backend/cmd/core_server/main.go` → `validateMidtransNotificationURL`, called from `validateMidtransConfig` at boot |
| Owner | whoever operates the environment (this repository owns the *contract*, not the DNS) |

**The Midtrans merchant-dashboard notification URL is NOT a fallback authority.**
It used to be: when `MIDTRANS_NOTIFICATION_URL` was empty the header was omitted
and the gateway silently used the dashboard value. An empty value is now a hard
boot failure, so there is exactly one authority and no hidden second one.

### Boot enforcement (configuration validity only)

The server refuses to boot unless the target is:

1. set (non-empty);
2. an absolute `https://` URL;
3. a public DNS name — `localhost`, IP literals and single-label names are rejected;
4. on the canonical path `/webhooks/payment/midtrans`.

**Deliberately NOT enforced here: reachability.** DNS/uptime death is a runtime
condition, and making boot depend on an external host would make legitimate
local development impossible. Runtime callback health is reported by the
canonical readiness endpoint (INFRA-2 / `GET /health/ready`).

### Local public ingress — INCOMPLETE / MISSING IMPLEMENTATION

Canonical payment callback contract exists, but canonical local-development
public ingress for real Midtrans Sandbox has not yet been established.

The repository does not ship a tunnel, a tunnel abstraction, or a fallback
ingress provider. The callback target must be an operator-provisioned, stable,
public HTTPS endpoint; how that endpoint is provisioned is outside this
repository. A hostname that is reissued per process start cannot be a callback
authority, because the gateway would silently lose every notification once it
changes.

This is an honest implementation gap, not a defect in the canonical callback
contract: the route, the configuration authority, the signature verification,
the persistence and the finalization are all in place and tested. Only the
local public ingress needed to *prove* a real Midtrans Sandbox delivery is
missing.

### Explicitly out of scope for INFRA-1

Webhook transaction boundary, event persistence, HTTP response semantics,
signature/amount verification, finalization, payment status transitions,
gateway reconciliation/polling, payment success authority, and the callback
health endpoint (INFRA-2).
