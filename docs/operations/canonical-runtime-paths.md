# Canonical Runtime Paths

This page is the quick index for "where should I patch runtime behavior?" in Labuda.
The goal is to reduce duplicate-authority mistakes and point future edits at the
actual runtime entrypoints first.

## Mobile App

| Concept | Canonical path | Notes |
|---|---|---|
| App bootstrap | `apps/mobile/lib/main.dart` | Entry from Flutter tooling. It should stay tiny. |
| App shell | `apps/mobile/lib/app.dart` | Hosts `LabudaApp`, theme, router wiring, and top-level banners. |
| Deprecated app shell | ~~`apps/mobile/lib/core/app/app.dart`~~ | **DELETED.** Was a legacy duplicate `LabudaApp` implementation. Confirmed removed. |
| Router authority | `apps/mobile/lib/core/src/router/app_router.dart` | Owns GoRouter config and redirect rules. |
| API config | `apps/mobile/lib/core/api/config/api_config.dart` | Canonical API base/config path. |
| WebSocket service | `apps/mobile/lib/core/websocket/websocket_service.dart` | Canonical real-time socket client. |
| Email verification banner | `apps/mobile/lib/domains/user/identity/authentication/presentation/widgets/email_verification_banner.dart` | Banner is injected by the root app shell. |
| Notification bootstrap | `apps/mobile/lib/domains/system/notification/presentation/widgets/notification_initializer.dart` | Canonical notification setup wrapper. |

## Admin App

| Concept | Canonical path | Notes |
|---|---|---|
| Admin bootstrap | `apps/admin/src/main.tsx` | Vite/React entrypoint. |
| Admin shell | `apps/admin/src/App.tsx` | Root React application component. |
| Admin API config | `apps/admin/src/lib/api/client.ts` | Canonical admin API base URL and fetch wrapper. |

## Backend

| Concept | Canonical path | Notes |
|---|---|---|
| Production server | `backend/cmd/core_server/main.go` | Main HTTP server entrypoint. Does not auto-run migrations. |
| Route registration | `backend/cmd/core_server/routes_core.go` | Single route tree owner. |
| Manual migration command | `backend/cmd/migrate/main.go` | Explicit migration apply command. Run this before starting the server. |
| Migration helper library | `backend/pkg/database/migrate.go` | Helper for tooling only. Not wired into `core_server` in the current runtime. |
| Seed command | `backend/cmd/seed/main.go` | Canonical seed entrypoint. |
| Staging rollout driver | `backend/cmd/staging_rollout_ab/main.go` | Canonical staging rollout tool. |
| Scenario / corpus driver | `backend/cmd/corpus_driver/main.go` | Canonical CI scenario runner. |
| Reconciliation (finance) | `backend/internal/worker/reconciliation_worker_v2.go` | The `_v2` suffix is the current name — no v1 exists. Canonical reconciliation worker. |
| Notification worker (core) | `backend/internal/worker/notification_worker.go` | Core struct, dispatcher `Handle()`, and DI helpers only. Domain handlers are in shards (see below). |
| Seller verification (mobile) | `apps/mobile/lib/domains/user/identity/verification/data/repositories/seller_verification_repository_v2.dart` | `_v2` is the current naming — no v1 exists. Canonical repository. |

## Backend `cmd/` Tier Classification

See `backend/cmd/README.md` for the full classification table.

| Tier | Directories | Notes |
|---|---|---|
| Tier 1 — Production/CI | `core_server`, `migrate`, `seed`, `corpus_driver` | Run in production and CI. |
| Tier 2 — Dev/Staging | `staging_rollout_ab`, `verify_partial_refund_semantics`, `midtrans_sandbox_validation`, `recon_audit` | Maintained dev/staging tools. |
| Tier 3 — DELETED | All historical proof fixtures (b44_, b59_, query_*, check_*, trace, validate, etc.) | Removed in DUPLICATE_AUTHORITY_FULL_CLOSURE_V1. |
| Tier 4 — DELETED | `verify_seller_debt` | Removed with `seller_debt` entity (migration 000192). |

## Backend `validation/` Classification

`backend/validation/` contains **dev-only** SQL flow scripts. Not runtime source. See `backend/validation/README.md`.

## Implementation Shards

Some public runtime files are thin wrappers that export or `part` into companion
implementation shards. Patch the implementation shard when changing behavior, but
keep the public wrapper as the stable import path.

| Public path | Implementation shard |
|---|---|
| `apps/mobile/lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen.dart` | `checkout_screen_impl.dart`, `checkout_screen_logic.dart`, and the nearby `part` widgets |
| `apps/mobile/lib/domains/commerce/transaction/checkout/presentation/screens/payment_result_screen.dart` | `payment_result_screen_impl.dart` and `payment_result_screen_sections.dart` |
| `apps/mobile/lib/domains/commerce/transaction/order/presentation/widgets/order_widgets.dart` | `order_widgets_impl.dart` and sibling `part` widgets |
| `backend/internal/worker/notification_worker.go` | Domain shards: `notification_worker_order.go`, `_commerce.go`, `_finance.go`, `_governance.go`, `_moderation.go`, `_social.go`, `_system.go`, `_shared.go` |

## Noncanonical Paths

| Path | Why noncanonical |
|---|---|
| `backend/cmd/migrate/main.go` | Current explicit migration apply command. Use this before `core_server`. |
| `backend/migrations/legacy_do_not_run/000_init/` | Legacy split-init schema quarantined away from the active chain. |
| `backend/migrations/legacy_do_not_run/archive/` | Historical evidence only; not runnable. |
| `backend/migrations/legacy_do_not_run/snapshots/` | Snapshot material only; not runnable. |
| ~~`apps/mobile/lib/core/app/app.dart`~~ | **DELETED.** Legacy duplicate `LabudaApp` shell. Runtime uses `apps/mobile/lib/app.dart`. |
| `backend/migrations/legacy_do_not_run/000_init/` | Legacy split-init schema quarantined. Active chain is `backend/migrations/000100+`. |
| `backend/migrations/legacy_do_not_run/archive/` | Historical evidence only; not runnable. |
| `backend/migrations/legacy_do_not_run/snapshots/` | Snapshot material only; not runnable. |
| Historical proof / cleanup scripts | One-off tools have no ongoing role. Deleted or archived in docs. |
| `backend/cmd/` Tier 3+4 dirs | All historical proof cmd dirs are DELETED (see Tier table above). |

## Editing Rule

Before touching runtime behavior, open the canonical path first. If that file
delegates to a shard or wrapper, follow the chain until you reach the actual
implementation that owns the behavior.
