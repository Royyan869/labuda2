# LABUDA — GLOBAL DOMAIN / SURFACE AUDIT

READ-ONLY AUDIT — ZERO IMPLEMENTATION.
Filesystem + migrations = truth. GitHub = backup only.
Order/Payment/Coins/Discount = PROTECTED PARALLEL BOUNDARY (not deep-audited).

---

# 1. SYSTEM ARCHITECTURE MAP

## Stack
- **Mobile:** Flutter (Dart) + Riverpod 3 + GoRouter, single Dio `ApiClient` (`core/api/api_client.dart`), one WebSocket client (`core/websocket/websocket_service.dart`), Firebase Auth + FCM.
- **Backend:** Go + Gin, PostgreSQL (migration chain `000001`–`000045`, active), Redis (presence), Firebase Admin, Midtrans (Snap + Iris payouts), Prometheus + health endpoints.
- **Admin:** separate Vite/React app (`apps/admin`) — not mapped in this pass beyond canonical entrypoints.
- **Runtime:** single production binary `cmd/core_server`, single DI graph `internal/serverboot/dependencies.go` (`InitServices` → `StartWorkers`), single route tree `cmd/core_server/routes_core.go`.

## Bootstrap chain (production)
```
cmd/core_server/main.go
  → pkg/database (Postgres) + pkg/redis + pkg/firebase + pkg/midtrans
  → serverboot.InitServices(...)   // full graph; workers deferred as closures
  → serverboot.StartWorkers(deps)  // ordered goroutine starts
  → routes_core.go                 // HTTP tree (health, public browse, authed v1, admin)
```

## Core architecture facts
- **One canonical DI graph.** No alternate production wiring; `corpus_driver` reuses the same graph with workers dormant.
- **Worker activation is env-gated and deliberately conservative:** ~30 workers exist; each is behind `DISABLE_<NAME>` (+ default-on/off), and two ("dangerous dormant") additionally require `ACK_DANGEROUS_<NAME>=true`. Two money-integrity workers default to **shadow mode**.
- **Event plumbing is outbox-only:** services write events in the same DB tx (outbox), `OutboxWorker` dispatches to registered handlers; `RealtimeWorker` consumes `chat.message.sent` for WS fanout. No Kafka/NATS.
- **Capability-based auth:** roles + capabilities stored in DB, resolved per-request (`ActorContextInject`), admin routes double-gated (`RequireAdminMiddleware` + `RequireCapability`).

---

# 2. DOMAIN INVENTORY

Discovered domains (backend package → mobile counterpart), grouped:

| # | Domain | Backend | Mobile |
|---|---|---|---|
| 1 | Identity / Auth | `internal/identity/auth` | `domains/user/identity/authentication` |
| 2 | User Profile | `internal/identity/user` | `domains/user/profile` |
| 3 | Seller | `internal/commerce/seller` | `domains/user/preference/seller` |
| 4 | Seller Subscription | `internal/commerce/subscription` | (seller upgrade flow) |
| 5 | KYC / Seller Verification | `internal/governance/verification` | `domains/user/identity/verification` |
| 6 | Social Content | `internal/social/content` | `domains/social/content` |
| 7 | Feed | `internal/social/feed` | `features/home` |
| 8 | Comments | `internal/social/content` (comment) | `domains/social/comment` |
| 9 | Mentions | `internal/social/content` (mention outbox) | (in content) |
| 10 | Social Graph (follow/block/mute) | `internal/social/graph` | `domains/social/follow` |
| 11 | Likes | `internal/social/like` | `domains/social/like` |
| 12 | Ratings | `internal/commerce/order/rating` | `domains/social/rating` |
| 13 | Promotion | `internal/pricing/promotion` | `domains/commerce/pricing/promotion` |
| 14 | Listing (fixed-price sale) | `internal/commerce/fixedprice` | `domains/commerce/catalog/listing` |
| 15 | Product (shared catalog) | `internal/commerce/product` | `domains/commerce/catalog/models` |
| 16 | Auction | `internal/commerce/auction` | `domains/commerce/catalog/auction` |
| 17 | Negotiation | `internal/commerce/negotiation` | `domains/commerce/negotiation` |
| 18 | Order | `internal/commerce/order` | `domains/commerce/transaction/order` |
| 19 | Payment | `internal/integration/payment` + `serverboot.CorePaymentHandler` | `domains/finance/transaction/payment` |
| 20 | Coins (loyalty) | `internal/incentive/coins` | `domains/finance/wallet/coins` |
| 21 | Discount | `internal/pricing/discount` | `domains/commerce/pricing/discount` |
| 22 | Pricing Token | `internal/pricing/token` | `domains/commerce/pricing/pricing_preview` |
| 23 | Chat | `internal/interaction/chat` | `domains/chat` |
| 24 | Notification | `internal/interaction/notification` | `domains/system/notification` |
| 25 | Push (FCM) | `internal/interaction/notification/service` | `core/messaging` |
| 26 | Realtime / WebSocket | `internal/realtime` + `internal/presence` | `core/websocket` |
| 27 | Media / Upload | `internal/platform/mediaupload`, `s3presign`, `commerce/media` | `shared/widgets` + upload services |
| 28 | Search | `internal/discovery/search` | `features/search` |
| 29 | Moderation | `internal/governance/moderation` | `domains/system/report` |
| 30 | Appeals | `internal/governance/moderation` (appeal) | (report screens) |
| 31 | Warnings | `internal/governance/moderation` (warning) | — |
| 32 | Admin | `internal/platform/admin` | separate `apps/admin` |
| 33 | Workers / Background | `internal/worker` | — |
| 34 | Outbox / Events | `internal/platform/outbox`, `internal/platform/events` | `core/src/events` (client bus) |
| 35 | Ledger / Finance | `internal/finance` + `internal/core/wallet` | `domains/user/preference/seller` (earnings) |
| 36 | Refund | `internal/finance/refund` | (order detail) |
| 37 | Billing (promo packages) | `internal/finance/billing` | (promotion screens) |
| 38 | Bank Account | `internal/finance/bankaccount` | (profile) |
| 39 | Withdrawal / Payout | `internal/finance/worker` + `wallet` | (seller) |
| 40 | Shipping | `internal/commerce/shipping` + `shipping/quote` | `domains/commerce/transaction/shipping` |
| 41 | Address | `internal/identity/address` | (profile/checkout) |
| 42 | Saved Items | `internal/interaction/saved_item` | `domains/user/preference/saved_item` |
| 43 | Disputes | `internal/governance/dispute` | (order detail) |
| 44 | Support Tickets | `internal/governance/support` | `domains/system/support` |
| 45 | Alerts / Monitoring | `internal/platform/alert`, `internal/monitoring` | `domains/system/analytics` |
| 46 | Audit | `internal/governance/audit` | — |
| 47 | Capability / Role | `internal/platform/capability` | — |
| 48 | Platform Config | `internal/platform/config` | `core/config` |
| 49 | Projection (read model) | `internal/projection` + `worker` | — |
| 50 | Evaluator (shadow) | `internal/governance/evaluator` | — |
| 51 | OG share metadata | `internal/platform/og` | (share) |
| 52 | Saved / Shortlist | `internal/interaction/saved_item` | `domains/user/preference/saved_item` |

---

# 3. DOMAIN RESPONSIBILITY MAP

Specific ownership per domain (from code + schema evidence):

- **Identity/Auth:** Firebase token exchange, JWT issue/refresh/session family, account status (active/suspended/banned/deleted), role + capability resolution. `users` row is identity root.
- **User Profile:** `user_profiles` composition across user/seller/subscription; public card; username uniqueness; self-delete (event `user.deleted`).
- **Seller:** `seller_profiles` (store name/image/cover), seller authority (4-gate middleware), analytics/dashboard (read-only), monthly metrics snapshot worker (analytics-only, explicitly NOT tier authority).
- **Subscription:** `seller_subscriptions` lifecycle (active→expired sweep worker), `seller_subscription_configs` (singleton admin row), onboarding validation service (SINGLE SOURCE OF TRUTH), payment recovery worker + admin manual recover, reconciliation worker.
- **KYC/Verification:** `seller_verifications` (8-state lifecycle), `verification_documents`, reviewed-bank-account snapshot (GUARD 5), admin review queue.
- **Social Content:** `contents` (post/request/repost), `content_media`, hashtags, mentions (outbox + `content_mentioned_users`), content visibility (moderation soft-delete), commerce references on comments.
- **Feed:** read-time feed composition; promotion injector; evaluator shadow runner (default off). Explicitly excludes commerce objects.
- **Comments:** `comments` (+ likes, commerce reference comments). Comment deletion = content removal doctrine.
- **Social Graph:** `user_follows`, `user_blocks`, `user_mutes`; block/mute checkers used cross-domain (comments, notifications, negotiation, chat).
- **Likes:** `content_likes`, `comment_likes`; toggle authority.
- **Rating:** `order_ratings` — immutable, one per order, buyer→seller only; invalidation worker on refunds.
- **Promotion:** `promotion_packages/ownerships/instances/events`, `external_products`; activation/resume/reassign; auto-stop on sale/auction end (event-driven + safety worker + expiry worker); billing integration for package purchase.
- **Listing:** `fixed_price_sales` (quantity persistence, sale channels), shipping coverage, publish/withdraw lifecycle; `products` + `product_shipping_options` shared catalog.
- **Auction:** `auctions`, `auction_bids`, anti-sniping extension, scheduled start/end/settlement workers, BNR strike + decay, claim/pricing-token flow.
- **Negotiation:** `negotiation_sessions` + `negotiation_messages` — time-bound, no stock reservation; expire worker; chat room auto-creation via outbox.
- **Order:** `orders` + `order_items` — status machine (pending_payment → paid/shipped/delivered/completed + cancelled/expired/refunded), escrow lifecycle, auto-complete + overdue-cancel + payment-timeout workers, order↔chat link via outbox.
- **Payment:** `payments` + `payment_attempts` + `payment_webhook_events` — Midtrans Snap creation, webhook finalization (CanonicalFinalizationService), expiry worker (dangerous-dormant), orphan recovery worker (env-gated), one-successful-payment-per-order constraint (migration 000044).
- **Coins:** `user_coin_balance`, `coins_transactions`, `coin_reservations` (reserved/consumed/released) — earn/spend/expire; refund via `coins.refund_required` handler.
- **Discount:** `discounts`, `discount_targets`, `discount_usages` — seller-created, validate/apply at order time.
- **Pricing Token:** `pricing_tokens` — server-issued snapshot for checkout; amount authority derived from order + token (client never submits fees).
- **Chat:** `chat_rooms`, `chat_messages`, `chat_read_states`, media attachments, commerce references, idempotency (fingerprint + actor scoping), resource projections (auction/content/listing/profile resolvers).
- **Notification:** `notifications` (in-app row), `notification_delivery_log` (delivery audit), FCM tokens, push retry queue, mute/block policies, per-domain handler shards.
- **Realtime:** WS hub + subscribe gate (room authorizer + account status), outbox→WS dispatcher, presence (`user_presence`, last_seen), ban/suspension WS eviction.
- **Finance/Ledger:** `ledger_entries`, `ledger_transactions`, `financial_accounts`, `account_balances`, `wallets`, `escrows` — double-entry, dispute-freeze authority, escrow integrity + total-money-invariant shadow workers, reconciliation V2 (verification-only, no auto-repair).
- **Refund:** `refunds` + `refund_evidence` — gateway refund orchestration, seller approve/reject, buyer escalate, webhook ack + ledger reversal, refund↔escrow safety check on auto-complete.
- **Shipping:** `shipping_options`, `shipping_coverages`, `shipping_city_overrides`, `product_shipping_options`, `shipping_quotes` (stateless, chat-based fallback).
- **Address:** `addresses` with primary-address invariant hardening.
- **Saved Items:** `saved_items` (unified shortlist + auction watch).
- **Dispute:** `disputes`, `dispute_freezes`, `dispute_media` — freeze authority, timeout/escalation worker (escalate-only, no auto-resolve).
- **Support:** `support_tickets` + `support_ticket_events`, `support_admins`, SLA metrics (worker default OFF), escalate-to-dispute.
- **Moderation:** `moderation_cases` → soft-delete/restore content/comment/listing, suspend/restore user, hide/restore chat messages; appeals + warnings + bans.
- **Alert:** `system_alerts`, detection rules (payment_failure_spike, payment_stuck, dispute_spike, seller_risk, coins_anomaly, withdrawal_anomaly, outbox DLQ/stuck, seller non-shipment, subscription, stale dispute freeze).
- **Audit:** `audit_events`, `admin_audit_logs`, `payout_whitelist_audit_logs`.
- **Projection:** `order_summaries` read model (worker default OFF; query service falls back to live `orders` when disabled).

---

# 4. CANONICAL AUTHORITY MAP

| Domain | Concern | Candidate Authority | Readers | Writers | Status | Confidence |
|---|---|---|---|---|---|---|
| Identity | user state | `users` (+ account_status) | auth, middleware, capability, all services | auth handler, admin | CANONICAL | HIGH |
| Identity | role/capability | `user_capabilities` + capability checker | ActorContextInject, admin | admin/capability | CANONICAL | HIGH |
| Profile | profile truth | `user_profiles` | profile service, userdisplay, publiccard | UserProfileService | CANONICAL | HIGH |
| Seller | store identity | `seller_profiles` | seller service, sellerdisplay, order | SellerService, onboarding | CANONICAL | HIGH |
| Seller | tier/reputation | `seller_reputation_state` (recompute worker = canonical authority) | seller dashboards, admin | recompute worker | CANONICAL | HIGH |
| Seller | analytics | `seller_monthly_metrics` | dashboard/analytics | metrics worker | SPLIT (analytics-only, authority explicitly separated) | HIGH |
| Subscription | subscription state | `seller_subscriptions` | seller handler, expiry worker, subscription services | subscription payment service, expiry worker, recovery | CANONICAL | HIGH |
| Subscription | config | `seller_subscription_configs` (singleton) | onboarding, admin | admin config handler | CANONICAL | HIGH |
| KYC | verification state | `seller_verifications` (8 statuses) | verification service, withdraw guard | document service, admin | CANONICAL | HIGH |
| KYC | documents | `verification_documents` | admin, verification service | document service | CANONICAL | HIGH |
| Content | content truth | `contents` (+ visibility) | content service, feed, comment, moderation | content service, moderation handler | CANONICAL | HIGH |
| Content | media | `content_media` | content/feed | media service | CANONICAL | HIGH |
| Feed | feed rows | `contents` (read-time) | feed service | content service | CANONICAL | HIGH |
| Comment | comment truth | `comments` | comment service, moderation | comment service, moderation | CANONICAL | HIGH |
| Social graph | follow/block/mute | `user_follows`/`user_blocks`/`user_mutes` | blockcheck, notification policy, negotiation, chat | social graph service | CANONICAL | HIGH |
| Like | likes | `content_likes`/`comment_likes` | like service, feed | like service | CANONICAL | HIGH |
| Rating | ratings | `order_ratings` | rating service, reputation worker | rating service, invalidation worker | CANONICAL | HIGH |
| Listing | listing truth | `fixed_price_sales` | listing service, order, chat projection, search | listing service, moderation | CANONICAL | HIGH |
| Product | catalog truth | `products` + `product_shipping_options` | auction (inline create), shipping, order | product repo, auction | SPLIT (product created via auction service inline; no standalone product HTTP) | MEDIUM |
| Auction | auction truth | `auctions` (+ bids) | auction service, workers, chat projection | auction service, workers, admin cancel | CANONICAL | HIGH |
| Negotiation | negotiation state | `negotiation_sessions`/`negotiation_messages` | negotiation service, chat | negotiation service, expire worker | CANONICAL | HIGH |
| Order | order truth | `orders` | order service, payment boundary, chat, admin | order service, workers, webhook | CANONICAL | HIGH |
| Order | order read model | `order_summaries` | OrderQueryService (when projection enabled) | projection worker (default OFF) | SPLIT — derived read model; fallback to `orders` live | HIGH |
| Payment | payment truth | `payments` | payment handler, webhook, expiry worker | CorePaymentHandler, webhook, workers | CANONICAL | HIGH |
| Payment | method/fee | `payment_methods` | CorePaymentHandler, admin | admin payment method handler | CANONICAL | HIGH |
| Payment | amount | `orders` + `pricing_tokens` (derived; client never submits fee) | payment handler | order service | CANONICAL | HIGH |
| Coins | balance | `user_coin_balance` | coins service, checkout | coins service, refund handler | CANONICAL | HIGH |
| Coins | reservation | `coin_reservations` | payment handler, coins service | payment handler, refund/expiry | CANONICAL | HIGH |
| Coins | ledger | `coins_transactions` (unique reference index) | coins service | coins service | CANONICAL | HIGH |
| Discount | discount truth | `discounts` + `discount_targets`/`discount_usages` | discount handler, checkout | discount service | CANONICAL | HIGH |
| Pricing | snapshot | `pricing_tokens` | order/checkout, payment | pricing token service | CANONICAL | HIGH |
| Chat | chat truth | `chat_rooms`/`chat_messages` | chat service, realtime, support adapter, projections | chat service, negotiation handlers, moderation | CANONICAL | HIGH |
| Notification | notification truth | `notifications` | notification handler, admin delivery monitor | notification worker | CANONICAL | HIGH |
| Notification | delivery log | `notification_delivery_log` | admin O4, cleanup worker | DeliveryLogger, push retry | SPLIT (log ≠ business authority) | HIGH |
| Push | token registry | `fcm_tokens` | push service | FCMTokenHandler | CANONICAL | HIGH |
| Presence | online/last_seen | `user_presence` (+ Redis) | presence service, realtime | presence service | CANONICAL | HIGH |
| Finance | ledger | `ledger_entries` | finance service, admin export, verifier | finance service, settlement, refund reversal | CANONICAL | HIGH |
| Finance | balances | `account_balances`/`financial_accounts` | finance service | finance service | SPLIT (two balance tables) | MEDIUM |
| Wallet | escrow | `escrows` + `wallets` | wallet service, integrity worker | order payment service, webhook settlement | CANONICAL | HIGH |
| Refund | refund truth | `refunds` | refund service, admin, seller/buyer handlers | refund service, webhook ack | CANONICAL | HIGH |
| Billing | billing truth | `billing_transactions` | billing service, payment handler | billing service | CANONICAL | HIGH |
| Withdrawal | withdrawal truth | `wallet.withdrawals` (finance shape, status REQUESTED...) | withdraw service, admin payout, payout worker/webhook | withdraw service, admin, payout worker | CANONICAL | HIGH |
| Shipping | options/coverage | `shipping_options`/`shipping_coverages`/`city_overrides` | shipping service | seller shipping service | CANONICAL | HIGH |
| Shipping | quote | `shipping_quotes` (stateless) | shipping quote service, chat | seller via chat | CANONICAL | HIGH |
| Address | address truth | `addresses` (primary invariant) | address service, order | address service | CANONICAL | HIGH |
| Saved items | saved truth | `saved_items` | saved item service, auction watch | saved item service | CANONICAL | HIGH |
| Dispute | dispute truth | `disputes` + `dispute_freezes` | dispute service, wallet freeze, admin | dispute service, timeout worker | CANONICAL | HIGH |
| Support | ticket truth | `support_tickets` + `support_ticket_events` | support service, SLA metrics | support service, user-reply handler | CANONICAL | HIGH |
| Moderation | case truth | `moderation_cases` | moderation service, appeal | moderation service | CANONICAL | HIGH |
| Alert | alert truth | `system_alerts` | alert service, admin | alert detection worker, refund-failed handler | CANONICAL | HIGH |
| Audit | audit trail | `audit_events`/`admin_audit_logs` | admin, verifier | audit service, admin actions | CANONICAL (trail, not business authority) | HIGH |
| Config | platform config | `platform_configs` | config service, feature flags | platform config handler | CANONICAL | HIGH |
| Capability | capability catalog | `user_capabilities` | capability service | capability service | CANONICAL | HIGH |
| Search | search history | `search_history` | search handler | search handler | CANONICAL | HIGH |
| Projection | order read model | `order_summaries` (projection_tracker) | OrderQueryService (fast path) | projection worker (default OFF) | SPLIT — derived; live fallback exists | HIGH |

---

# 5. PRODUCTION RUNTIME SURFACE

Per domain classification of path liveness:

| Domain | Path | Classification |
|---|---|---|
| Identity/Auth | `routes_core` auth group → AuthHandler → firebase + DB | REGISTERED + RUNTIME-PROVEN (auth is exercised by every request) |
| User Profile | `/users/me` etc → UserProfileHandler → UserProfileService | REGISTERED |
| Seller | `/seller/*` → SellerHandler → SubscriptionService/Onboarding | REGISTERED |
| Subscription | routes + expiry/recovery/reconciliation workers | REGISTERED + workers default ON |
| KYC | seller workspace routes + admin queue | REGISTERED |
| Content/Feed/Comments/Likes | browse + authed routes | REGISTERED |
| Mentions | content mention outbox → notification handler | REGISTERED (handler wired, idempotent) |
| Social graph | `/users/:id/follow|block|mute` + `/follows` parity TODO | REGISTERED (note: Flutter-parity routes documented as NOT implemented) |
| Listing/Auction | browse + seller routes + workers | REGISTERED + auction workers default ON |
| Negotiation | chat-owned routes + expire worker | REGISTERED (routes live under `/chat`); mobile NOT PROVEN reachable |
| Order | order routes + workers (auto-complete, overdue-cancel, payment-timeout, reminder) | REGISTERED + workers default ON |
| Payment | `/payments` + webhook + finalization | REGISTERED; **PaymentExpiryWorker default OFF (dangerous-dormant)**; orphan recovery env-gated OFF |
| Coins | `/coins` + refund handler | REGISTERED |
| Discount | `/discounts` | REGISTERED |
| Promotion | `/promotions` + admin external products + safety/expiry workers | REGISTERED + workers default ON |
| Chat | `/chat` + WS + realtime worker | REGISTERED + RealtimeWorker default ON |
| Notification/Push | `/notifications` + outbox handlers + push retry + cleanup | REGISTERED + handlers active; push retry/cleanup default ON |
| Finance/Ledger | admin finance + reconcile worker + integrity shadow workers | REGISTERED; reconciliation default ON; escrow/total-money default ON in SHADOW mode |
| Refund | `/refunds` + admin gateway initiate (feature flag OFF) + webhook ack | REGISTERED; **gateway refund phase2 flag default false** |
| Withdrawal/Payout | `/withdraw` + admin payouts + payout worker | REGISTERED; **payout worker only when PAYOUT_ENABLE_WORKER=true**; gateway sandbox or Iris |
| Shipping | `/shipping` + seller shipping + quotes | REGISTERED |
| Moderation | user + admin routes + outbox enforcement handlers | REGISTERED; **UserBanEventHandler default OFF (dangerous-dormant)**; moderation event handlers default ON |
| Support | user + admin routes + user-reply handler | REGISTERED; **SLA worker default OFF** (events unregistered) |
| Dispute | user routes + admin + timeout worker | REGISTERED + worker default ON |
| Alert | admin alerts + detection worker | REGISTERED + worker default ON |
| Search | browse + history routes | REGISTERED |
| Projection | **worker default OFF**; admin dev-only control; query fallback live | REGISTERED-but-inactive (by design, fallback path live) |
| Evaluator | shadow runners default OFF (feed/content-detail/search content) | STATIC unless env-enabled |

**Workers enabled by default (production-reachable):** Outbox, Realtime, NegotiationExpire, OrderPaymentTimeout, OrderAutoComplete, AuctionStart/End/Settlement, BNRDecay, SellerSubscriptionExpiry, OutboxArchival, OrderOverdueCancel, SubscriptionReconciliation, WithdrawalMonitoring, PushRetry, NotificationCleanup, IdempotencyCleanup, PromotionSafety, PromotionExpiration, OrderOverdueReminder, DisputeTimeout, AlertDetection, RatingInvalidation, SellerMetrics, SellerReputationRecompute, Reconciliation, EscrowIntegrity (shadow), TotalMoneyInvariant (shadow), ModerationEventHandler.

**Workers default OFF / dormant:** PaymentExpiryWorker (dangerous), UserBanEventHandler (dangerous), SLAEscalationWorker, ProjectionWorker, SystemMonitoringWorker (commented out), OrphanWebhookRecoveryWorker, PayoutWorker (PAYOUT_ENABLE_WORKER), PayoutReconciliationWorker (env).

---

# 6. STATE AUTHORITY MAP

| Domain | Business State | Durable Authority | Derived State | Cache | Audit/Log | Conflict |
|---|---|---|---|---|---|---|
| Identity | account status | `users.account_status` | Firebase claims | none | admin_audit_logs | none detected |
| Seller | selling authority | `seller_subscriptions.status` + `seller_profiles` | RoleChecker/RequireSeller | none | admin_audit_logs | none detected |
| Seller | tier | `seller_reputation_state` | recompute worker | none | audit_events | analytics (`seller_monthly_metrics`) explicitly non-authority |
| KYC | verification | `seller_verifications.status` | reviewed bank snapshot | none | admin_audit_logs | none detected |
| Order | lifecycle | `orders.status` (+ escrow_status) | `order_summaries` (disabled) | none | audit_events, order timeline | order_summaries derived; fallback live |
| Payment | lifecycle | `payments.status` | — | none | payment_webhook_events, audit | expiry vs webhook race mitigated by constraints + alert (PASS_18T) |
| Coins | balance | `user_coin_balance` | reservation state | none | coins_transactions | none detected |
| Finance | money truth | `ledger_entries` | account_balances / financial_accounts | none | audit_events | two balance tables (ledger is canonical) |
| Wallet | escrow | `escrows` | wallet balance | none | escrow integrity checks | shadow-detected drift possible (no auto-repair by design) |
| Refund | refund lifecycle | `refunds.status` + gateway_status | — | none | audit, webhook log | none detected |
| Withdrawal | payout lifecycle | `wallet.withdrawals` status | ledger WITHDRAWAL_PENDING | none | payout_whitelist_audit_logs | none detected |
| Chat | rooms/messages | `chat_rooms`/`chat_messages` | WS in-memory hub | client-side caches | none | chat cache vs REST = mobile-side merge (documented) |
| Notification | notification state | `notifications` (is_read) | unread count | none | notification_delivery_log (NOT authority) | none detected |
| Presence | online | Redis + `user_presence.last_seen_at` | WS hub state | Redis | none | Redis vs DB dual-write (DB is durable) |
| Content | visibility | `contents.visibility`/status + moderation | feed read-time filter | none | moderation_cases | moderation vs author delete both mutate visibility |
| Subscription config | pricing | `seller_subscription_configs` | — | none | admin_audit_logs | none detected |
| Platform config | feature flags | `platform_configs` | cfg | none | admin_audit_logs | none detected |

---

# 7. LIFECYCLE RISK MAP

| Domain | States | Owner | Terminal | Transitions via | Workers | Risk |
|---|---|---|---|---|---|---|
| Order | pending_payment, paid, shipped, delivered, completed, cancelled, expired, refunded | OrderService | completed/cancelled/expired/refunded | handler + workers | auto-complete, overdue-cancel, payment-timeout, expiry(dormant) | HIGH complexity; escrow + refunds interlock; payment-timeout + expiry split coverage (by design, complementary) |
| Payment | pending, settled, failed, deny, expired | PaymentService/Webhook | settled/failed/expired | webhook + expiry worker(dormant) + payment-timeout | orphan recovery(dormant) | MEDIUM-HIGH; expiry worker dormant → relies on order timeout + webhook; alert on late-success |
| Coin reservation | reserved, consumed, released | CoinsService | consumed/released | payment handler, refund, expiry | coins.refund_required handler | MEDIUM; unique index protects idempotency |
| Auction | draft, scheduled, active, ended, waiting_settlement, expired_bnr, cancelled | AuctionService | ended/cancelled/expired_bnr | workers | start/end/settlement | MEDIUM; anti-sniping extension; BNR strike + decay |
| Negotiation | active, accepted, expired, cancelled | NegotiationService | accepted/expired/cancelled | handler + expire worker | expire worker (default ON) | LOW-MEDIUM; time-bound, no stock reservation |
| Seller subscription | active, expired (+ payment pending) | SubscriptionService | expired | expiry worker + payment webhook + sync + recovery | expiry, reconciliation | MEDIUM; orphan-payment recovery exists |
| KYC verification | 8 statuses (pending_review, approved, rejected, needs_resubmission, suspended, revoked, under_investigation, restored) | VerificationService | (approved/rejected/revoked are stable-ish, restore exists) | admin + document submit | — | MEDIUM; GUARD 5 reviewed-bank snapshot staleness (events = future hooks) |
| Dispute | open, resolved, rejected, partial_split, overdue, timeout_escalation | DisputeService | resolved/rejected | handler + timeout worker | timeout worker (escalate-only) | MEDIUM; no auto-resolve; freeze authority |
| Refund | opened, approved, rejected, escalated, gateway dispatched, succeeded/failed | RefundService | succeeded/failed (gateway) | seller/buyer/admin handlers + webhook ack | refund_failed alert handler | HIGH; gateway + ledger reversal + escrow safety; flag-gated admin initiate |
| Moderation case | open, actioned, appealed, resolved | ModerationService | resolved | admin action + appeal review | moderation event handlers | MEDIUM; enforcement → notification fanout ordering |
| Withdrawal | REQUESTED, PROCESSING, SETTLED, FAILED | WithdrawService/Admin/PayoutWorker | SETTLED/FAILED | admin + payout worker + webhook | payout worker (env), monitoring (read-only) | MEDIUM; stuck monitoring; completion-path safety check |
| Support ticket | open, in_progress, waiting_user, resolved, closed (+ escalation) | SupportService | resolved/closed | handlers + user-reply + SLA(dormant) | SLA worker (default OFF) | MEDIUM; SLA pipeline not built (events unregistered) |
| Notification | row + delivery log + retry queue | NotificationWorker | (deleted by user/cleanup) | outbox handlers | push retry, cleanup | LOW; retry 10x/24h; cleanup 30d |

---

# 8. CROSS-DOMAIN DEPENDENCY GRAPH

Verified edges (evidence: `serverboot/dependencies.go` wiring + outbox registrations):

```
Identity/auth ──► EVERYTHING (account status checker, role/capability)
     │
     ├─► Social (content/feed/like/comment) ──► Outbox ──► NotificationWorker ──► FCM push
     ├─► Social graph (follow/block/mute) ──► block/mute checkers ──► comment, notification policy, negotiation, chat
     ├─► Chat ──► Outbox (chat.message.sent) ──► RealtimeWorker ──► WS hub
     │        └─► Order (link-order, order chat) ──► order.chat_link_requested consumer
     │        └─► Negotiation (chat-owned) ──► chat room auto-create via outbox
     │        └─► ShippingQuote (chat-based) ──► quote reactivation validates order
     ├─► Commerce:
     │     Seller ──► Listing/Auction (authority gate)
     │     Listing/Auction ──► Product (inline) ──► Shipping (coverage/options)
     │     Listing/Auction ──► Promotion (auto-stop on sold/ended/cancelled)
     │     Listing ──► Order ──► Payment ──► Midtrans webhook
     │     Order ──► Escrow/Wallet ──► Ledger/Finance
     │     Order ──► Refund ──► Gateway refund + Ledger reversal
     │     Order ──► Rating (completed orders) ──► Seller reputation recompute
     │     Order ──► Dispute ──► Freeze (wallet) + timeout worker
     │     Coins: Order/Refund ──► coins.refund_required ──► CoinsService
     │     Discount/Pricing token: checkout ──► order snapshot ──► payment amount authority
     ├─► Governance:
     │     Moderation ──► content/comment/listing/chat soft-delete + user suspend + WS eviction
     │     Appeal ──► moderation cases (+ record-only listing/auction)
     │     Support ──► Chat (adapter) + Dispute (adapter) + escalation
     │     Warning ──► users
     ├─► Admin:
     │     AdminHandler ──► delivery logger (O4), BNR reset, SLA metrics
     │     AdminVerification ──► bank accounts (mark-reviewed), documents
     │     AdminPayout ──► whitelist audit, withdrawal lifecycle
     ├─► Platform:
     │     Outbox ──► every worker; ProjectionWorker (dormant) ──► order_summaries
     │     Alert detection ──► orders/payments/outbox/disputes/coins/withdrawals (read-only)
     │     Capability ──► admin + actor resolution
     │     Config ──► pricing token, feature flags
     └─► Realtime/Presence ──► Redis + DB dual-write, presence.last_seen outbox handler
```

Cross-domain writers to watch (multiple domains write one table):
- `notifications` ← notification worker (outbox) only — clean.
- `chat_rooms` ← chat service, negotiation handler, order chat-link consumer, support adapter — all via outbox (single dispatcher). Clean by design.
- `escrows`/`ledger_entries` ← order payment service, webhook settlement, refund reversal, withdraw — all funneled through FinanceService/OrderService. Clean.
- `order_summaries` ← projection worker only (dormant). Clean.
- `contents.visibility` ← content service + moderation handler — both funnel through ContentService. Clean.
- `fcm_tokens` ← FCMTokenHandler only. Clean.

---

# 9. AUTHORITY CONFLICT INVENTORY

Known/observed conflicts and their resolution status:

| Conflict | Status | Evidence |
|---|---|---|
| user role vs seller profile | RESOLVED — role authority removed (migration 000012 `remove_seller_role_authority`); seller authority = subscription + profile | migration + `RequireSellerMiddleware` |
| profile identity vs Firebase identity | RESOLVED — DB `users` is canonical; Firebase token only authenticates | auth handler exchange flow |
| content visibility vs moderation state | ACTIVE but funneled — author delete + moderation soft-delete both mutate `contents.visibility` through ContentService; no separate conflicting state | content service + moderation handler |
| payment amount vs order amount | RESOLVED — order + pricing token are sole amount authority; client never submits fee (PASS_18V) | CorePaymentHandler |
| notification state vs delivery log | RESOLVED — `notifications` is business truth; `notification_delivery_log` is delivery audit (O4 monitor reads it) | delivery_logger |
| chat cache vs REST | PARTIAL — mobile client merges WS signals + REST; documented merge/replace semantics; repo disposal on logout unproven | mobile chat repo |
| subscription config vs seller capability | RESOLVED — configs table + capability checker; onboarding service is single source of truth | serverboot |
| pricing config vs order snapshot | RESOLVED — pricing_tokens snapshot at order time; payment validates token vs order base | CorePaymentHandler.loadOrderPricingTokenSnapshot |
| ledger vs account_balances vs financial_accounts | SPLIT — ledger_entries canonical; two balance tables exist; integrity worker verifies | finance service + workers |
| order status vs escrow status | PARTIAL — two columns on `orders` (status + escrow_status) with interlock checks; integrity shadow worker detects drift | order entity + escrow integrity worker |
| seller tier vs monthly metrics | RESOLVED — metrics explicitly analytics-only; reputation state is authority | worker docs |
| withdrawal state vs ledger | RESOLVED — single finance-shape lifecycle (REQUESTED...) consumed end-to-end | withdraw service |

---

# 10. LEGACY / ZOMBIE / DUPLICATE SURFACE

## Backend
- **PROVEN LEGACY (migrated away):** `listings` table + `listing_shipping_options` + `listing_views` dropped (000010, 000016); orphan tables dropped (000011: `actors`, `bnr_classifications`, `financial_reconciliations`, `search_results`, `ticket_escalations`, `user_online_status`); seller role authority dropped (000012); `payment.net_amount` dropped (000037); legacy `content_share_reference` dropped (000041). Historical `cmd/` proof tools deleted (Tier 3/4).
- **UNKNOWN / RESIDUE:** `ticket_escalation.go` entity struct left after table drop (documented in 000011 comment — PROVEN orphan struct, left in place).
- **ZOMBIE (gated off by design, not dead code):** `domain_action_worker.go` (PARKED — no instantiation, events reserved in registry); SLA worker (OFF, events unregistered, noted idempotency-key mismatch in comments); `sla_escalation_panictest` (test-only).
- **DUPLICATE-ish adapters (compat seams):** `*Adapter` types in serverboot (`socialBlockCheckerAdapter`, `notificationBlockCheckerAdapter`, `notificationMuteCheckerAdapter`, `supportChatServiceAdapter`, `disputeServiceAdapter`, `chatOrderOwnershipAdapter`, `shippingQuoteRoomGetterAdapter`, `s3PresignerAdapter`, `realtimeOutboxRepositoryAdapter`) — runtime-required interface adapters, ACTIVE not residue.
- **DUPLICATE gateway surface:** payout gateway has `sandbox` + `midtrans_payout` (Iris) — ACTIVE split by config, not duplicate.
- **UNKNOWN:** `CoreUserHandler` (legacy stub "will be replaced") vs `UserProfileHandler` — both wired; `SetRole` admin route uses `UserHandler.SetRole`. Active duplicate-ish: legacy stub still reachable.
- **DUPLICATE notification handlers:** `money.refund_failed` has both alert handler and notification fanout (intentional, composed fanout).

## Mobile
- **PROVEN ZOMBIE/DEAD:** `negotiation_di.dart`, `payment_di.dart`, `coins_di.dart`, `report_di.dart` overrides() helpers never invoked; `notification/di/notification_di.dart` (dead GetIt stack, would construct ApiClient-less datasource); `finance_gateway.dart` (contract-only, no impl); `core/api/di/api_di.dart` (init never called); `OnboardingNotifier.checkAuthState` hardcoded stub.
- **PROVEN LEGACY:** `core/src/services/service_locator.dart` (declared bootstrap-only, no production callers); `AppRouter` legacy wrapper; deleted `core/app/app.dart`.
- **ACTIVE DUPLICATE:** `SavedItemRepository` constructed 3× (screen, badge, auction-watch) vs single provider — with a second `ApiClient()` bypassing bootstrap; `PromotionController` builds second `PromotionRepositoryImpl`; two EventBuses (core + chat).
- **PARTIAL:** `RoutePaths.bidding` registered but no UI call sites; negotiation module has no route/consumers; ticket-list screen has no route; report domain's S3/userId providers never overridden.

---

# 11. EVENT / OUTBOX / WORKER MAP

## Pipeline
```
Service tx ──► INSERT outbox (same tx) ──► OutboxWorker poll ──► Dispatcher ──► handler(s)
                                            │                      └─► retry up to 20 (OUTBOX_MAX_ATTEMPTS)
                                            ├─► RealtimeWorker (chat.message.sent) ──► WS hub
                                            └─► OutboxArchivalWorker (30d) + Dead-letter via retry exhaustion
```

## Registered handler groups (evidence: `outbox_worker.go` + `notification_worker.go` + serverboot)
- **Default handlers:** `payment.completed/expired/failed` (log), `user.created/role_changed` (log).
- **Notification (active, full set):** user.followed, content.liked, content.mentioned, comment.created, comment.reply, seller.response, chat.message.sent, order.created/paid/shipped/completed/cancelled/cancelled_timeout/expired/refunded/partially_refunded/dispute_open/confirmation_extended, refund.opened/approved/rejected/escalated, dispute.opened/resolved/overdue/timeout_escalation, money.refund_failed, order.overdue_reminder.{seller,buyer}, moderation.warning.issued, support.ticket.*, negotiation.accepted/expired/cancelled, withdrawal.*, verification.*, seller.verification.*, auction.bid.placed/waiting_settlement/ended, seller.tier.upgraded/downgraded, seller.subscription.expiring/expired, external_product.review.*, user.blocked/unfollowed cleanup.
- **Moderation enforcement (default ON):** moderation.content/comment/fixed_price_sale/auction/chat_message/user.{removed,restored,suspended} + WS eviction.
- **Promotion (default ON):** fixed_price_sale.sold/withdrawn/updated, auction.ended/cancelled, seller.subscription.activated/expired, seller.verification.restored/suspended/revoked, moderation.fixed_price_sale.restored.
- **Negotiation → chat:** negotiation.started, negotiation.message_sent (fanout).
- **Order → chat link:** order.chat_link_requested.
- **Coins:** coins.refund_required (idempotent via unique index).
- **Presence:** presence.last_seen_record.
- **BNR:** auction_bnr_detected (strike + notification fanout).
- **User ban (dormant, dangerous):** user.banned (mass refund/dispute).
- **Alerts:** money.refund_failed → system_alert.

## Acknowledged no-handler events (audit-only / future hooks)
`money.released`, `money.refund_pending`, `money.refund_succeeded`, `money.refunded`, `money.partial_refund`, `money.partial_release`, `fixed_price_sale.created/published`, `auction.created/scheduled/activated/claimed/bid.updated/order.created`, `auction.extended` (future hook), `refund.admin_refunded/released`, `order.dispute_refund_initiated/partial`, `appeal.reversed`, domain-action stubs, `chat.room.created/updated` (future hook — realtime producer wired first), `user.unblocked`, `user.deleted` (future hook), `bank_account.*_after_verification` (future hooks).

## Vulnerabilities (factual, not resolved)
- **SLA worker OFF** → support/dispute SLA events never emitted; ticket SLA metrics surface is fed by queries, not events.
- **chat.room.created/updated** consumed by realtime producer only; no room-list consumer yet (mobile polls REST).
- **PaymentExpiryWorker dormant** → expired-payment handling depends on OrderPaymentTimeoutWorker (orders without payment rows) + late-success webhook alert. Coverage is complementary but payment-row-only expiry is unproven at runtime.
- **ProjectionWorker dormant** → order_summaries empty; OrderQueryService live fallback is the actual runtime path (tested).
- **user.deleted / user.banned** future hooks → account deletion/ban mass-effects unbuilt (ban handler dormant).
- **Outbox retry cap 20** + DLQ + stuck-rule alert = bounded, observable.

---

# 12. REALTIME SURFACE

- **Canonical transport:** WebSocket at `/api/v1/ws`, auth via Authorization header, hub + subscribe gate.
- **Producer:** services write `chat.message.sent` (+ `chat.message.hidden/restored`, `chat.room.updated` envelopes) to outbox in the same tx.
- **Consumer:** `RealtimeWorker` polls outbox → `Dispatcher` → hub → per-room broadcast with per-subscriber account-status re-check (CHAT-3).
- **Governance:** `SubscribeGate` = room authorizer (DB membership) + account status checker; rate limiter.
- **Moderation/ban eviction:** WS eviction handlers on `user.banned` / moderation `user.suspended` / chat-message hidden-restored.
- **Presence:** Redis + DB dual-write; `presence.last_seen_record` outbox handler for durable retry; `user_presence` table (migration 000025).
- **Mobile reconnect:** max 5 attempts / 5s delay; resubscribe before `connected` broadcast; ack-based subscribe; missed-event recovery = resubscribe + REST refresh (chat), polling (auction/order/coins), none (feed).
- **Duplicate handling:** WS is fanout-only (no client state authority); outbox idempotency + unique indexes protect the event side.
- **NOT PROVEN:** WS presence across reconnect storms; room-list realtime (producer-only); feed realtime (none).

---

# 13. DATABASE / SCHEMA SURFACE

- **~110 tables** in canonical schema (000001) minus 11 pruned (000011 dropped 6; 000010 dropped `listings`, `listing_views`, `listing_shipping_options`; 000016 dropped legacy `listing_shipping_options`; 000023 dropped legacy media tables; 000027 dropped legacy chat media tables; 000002 dropped `negotiation_price_history`).
- **Overlapping tables:** `ledger_entries` vs `ledger_transactions` vs `financial_accounts`/`account_balances` — ledger_entries canonical, others support balances; `wallets` vs `account_balances` (escrow/wallet vs seller payable); `order_summaries` (derived) vs `orders` (canonical); `billing_transactions` vs `payments` (billing references payments via reference_type).
- **Migration evidence of convergence:** 000010 product/sale-channel canonicalization (drop listings → fixed_price_sales + products), 000012 remove seller role authority, 000013/14/15 shipping quote supersession + authority hard purge, 000016 purge legacy shipping options, 000017 primary address invariant, 000022/23/24 typed commerce media authority, 000026 FCM device identity hardening, 000027 chat media reply authority, 000029/30/31 chat/comment commerce reference canonicalization, 000032/33 chat idempotency actor scoping + fingerprint, 000034/39 resource occurrences, 000035 coin reservations, 000036/37/38 payment coins rename/net drop/ref type, 000040 refund product-shipping split, 000041 drop legacy content share reference, 000042 alert active group key uniqueness, 000043 content_mentioned_users, 000044 one-successful-payment-per-order, 000045 order coin refund authority.
- **Status fields:** orders.status + escrow_status; payments.status; coins_transactions.status; refunds.status + gateway_status; seller_subscriptions.status; seller_verifications.status; moderation_cases.status; disputes.status + is_overdue; withdrawals.status; support_tickets.status; alerts.status. No orphan status fields found in this pass (readers exist per domain).
- **Config tables:** `configs` AND `platform_configs` both exist — `configs` appears to be the legacy one; platform_configs is wired to the PlatformConfigService. **DUPLICATE-CONFIG RISK: UNKNOWN — needs verification of which table runtime reads.**
- **Legacy enum residue:** schema may retain dead enum types (`listing_status_enum`, `listing_origin_enum`, `comment_type_enum`) after table drops — **UNKNOWN, needs verification** (agent was cut off before confirming).

---

# 14. MOBILE SURFACE

Full screen→notifier→provider→repo→datasource chains are complete for: auth, profile, content, comment, follow, like, rating, share, home, explore, search, chat, listing, auction, checkout, order, payment, coins, notification, seller, verification, promotion, discount, shipping, support.

**Findings (highest-signal):**
1. **Chat** is the most complete realtime domain (WS + REST fallback, resubscribe, honest errors) — BUT `ChatRepositoryImpl.dispose()` is never called (no `ref.onDispose`); logout leaves stale WS subscriptions. Account-isolation risk: MEDIUM, not fully mitigated.
2. **Saved-item** — route's screen bypasses Riverpod (`SavedItemRepository()` direct), errors swallowed in empty catch; 3 construction sites incl. one that builds a second `ApiClient()`.
3. **Report** — `reportCurrentUserIdProvider` defaults to null (never overridden); S3/userName providers throw `UnimplementedError` unless overridden; `ReportDI.overrides()` never called in main.dart → **report submission NOT PROVEN functional**.
4. **Negotiation** — complete DI chain, zero UI consumers, zero routes → NOT PROVEN reachable.
5. **Onboarding notifier** — hardcoded stub; real work done by router redirect.
6. **FinanceGateway** — contract file, no implementation; commerce calls finance providers directly.
7. **Dead GetIt scaffolding** — service_locator, api_di, notification_di (broken ApiClient-less datasource), payment/coins/negotiation/report di overrides.
8. **KeepAlive caches** — `fixedPriceSaleDetailProvider`, `auctionDetailProvider` keyed by ID, low cross-account risk (public data).
9. **Router** — centralized redirect logic (pure testable function); several nominal destinations collapse into `/settings`; `bidding` route registered but unreachable from UI.

---

# 15. BACKEND SURFACE

- **Single HTTP tree** (`routes_core.go`): health/metrics, public browse (`StrictBrowseAuthMiddleware` — anonymous allowed), authenticated `/api/v1` (full middleware stack), admin group (dual-gated).
- **Public browse is deliberate:** listings, auctions, search, public user/content read, like stats.
- **Admin surface is large and capability-matrixed** (~80 admin routes).
- **Multiple paths performing the same business action:** refund path can be initiated by seller approve/reject, buyer escalate, admin gateway initiate, dispute resolution, order cancel-overdue/expire — all funnel through RefundService/OrderService (single authority). Moderation enforcement via outbox handler + admin direct action + appeal review — funneled through ModerationService/ContentService.
- **Two payment-related workers split coverage** (PaymentExpiryWorker dormant vs OrderPaymentTimeoutWorker active) — complementary by design but worth confirming ownership boundaries under parallel work.
- **Compatibility adapters** in serverboot are the main "multiple path" seams (interface adaptation, not business logic duplication).

---

# 16. ADMIN SURFACE

- **Separate authority from backend:** admin routes are capability-gated (`governance.*`, `finance.*`, `moderation.*`, `config.*`, `support.*`, `promotion.*`); RequireAdminMiddleware + RequireCapability dual protection.
- **Direct DB mutation:** admin handlers mostly go through the same services (AdminService, VerificationService, ModerationService, PayoutService) — plus `adminAuditLogger` on every mutation. No raw SQL mutation handlers found in this pass.
- **Status mutation paths:** user suspend/activate/ban/unban, BNR reset, dispute approve/reject/partial-split, withdrawal approve/reject/mark-processed, verification approve/reject/suspend/revoke/restore/investigate, moderation case action, appeal review, warning issue/revoke, subscription recovery, platform config update, payment method config update, alert ack/resolve.
- **Read-only visibility:** audit logs, ledger export, finance verifier, reconciliation results, failed notification deliveries, whitelist audit, campaign analytics.
- **Dev-only admin routes** (cfg.IsDevelopment): payout webhook test/sign, projection status/rebuild/process, webhook-drop arm.
- **No evidence** of an admin web that mutates the DB directly outside the backend API in this pass (admin web is a separate app not deep-audited).

---

# 17. RUNTIME PROOF MATRIX

| Domain | Static Proof | Registration Proof | Runtime Proof | State Proof | Overall |
|---|---|---|---|---|---|
| Identity/Auth | PROVEN | PROVEN | PROVEN (every request) | PROVEN | PROVEN |
| User Profile | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Seller | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Subscription | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| KYC/Verification | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Content/Feed/Comments/Likes | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Mentions | PROVEN | PROVEN | PARTIALLY PROVEN (idempotency proven by tests) | PROVEN | PARTIALLY PROVEN |
| Social Graph | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Listing | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Auction | PROVEN | PROVEN | PARTIALLY PROVEN (workers default ON) | PROVEN | PARTIALLY PROVEN |
| Negotiation | PROVEN | PROVEN (routes) | NOT PROVEN (mobile unreachable) | PARTIALLY PROVEN | NOT PROVEN |
| Order | PROVEN | PROVEN | PARTIALLY PROVEN (workers ON) | PROVEN | PARTIALLY PROVEN |
| Payment | PROVEN | PROVEN | PARTIALLY PROVEN (expiry worker dormant) | PROVEN | PARTIALLY PROVEN |
| Coins | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Discount | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Pricing Token | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Chat | PROVEN | PROVEN | PROVEN (WS + REST + tests) | PARTIALLY PROVEN (logout disposal) | PARTIALLY PROVEN |
| Notification/Push | PROVEN | PROVEN | PARTIALLY PROVEN (retry/cleanup ON) | PROVEN | PARTIALLY PROVEN |
| Realtime | PROVEN | PROVEN | PARTIALLY PROVEN | PARTIALLY PROVEN | PARTIALLY PROVEN |
| Media/Upload | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Search | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Moderation | PROVEN | PROVEN | PARTIALLY PROVEN (ban handler dormant) | PROVEN | PARTIALLY PROVEN |
| Appeals/Warnings | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Support | PROVEN | PROVEN | PARTIALLY PROVEN (SLA off) | PROVEN | PARTIALLY PROVEN |
| Dispute | PROVEN | PROVEN | PARTIALLY PROVEN (timeout ON) | PROVEN | PARTIALLY PROVEN |
| Admin | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Finance/Ledger | PROVEN | PROVEN | PARTIALLY PROVEN (shadow workers) | PROVEN | PARTIALLY PROVEN |
| Refund | PROVEN | PROVEN | PARTIALLY PROVEN (flag-gated) | PROVEN | PARTIALLY PROVEN |
| Withdrawal/Payout | PROVEN | PROVEN | PARTIALLY PROVEN (env-gated worker) | PROVEN | PARTIALLY PROVEN |
| Shipping | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Projection | PROVEN | PROVEN | NOT PROVEN (worker OFF; fallback live) | NOT PROVEN (empty table) | NOT PROVEN (by design) |
| Outbox/Workers | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Evaluator | PROVEN | NOT PROVEN (default off) | NOT PROVEN | NOT PROVEN | NOT PROVEN (by design) |
| Alerts | PROVEN | PROVEN | PARTIALLY PROVEN | PROVEN | PARTIALLY PROVEN |
| Mobile Report | PROVEN | NOT PROVEN (unoverridden providers) | NOT PROVEN | NOT PROVEN | NOT PROVEN |
| Mobile Saved-item | PROVEN | PARTIALLY PROVEN | NOT PROVEN (swallowed errors) | PARTIALLY PROVEN | NOT PROVEN |

---

# 18. DOMAIN RISK SCORE

Scoring: A=Authority 0-5, B=Runtime 0-5, C=Lifecycle 0-5, D=Residue 0-5, E=Cross-Domain 0-5, F=False-Closure 0-5. Max 30.

| Domain | A | B | C | D | E | F | Total | Rationale |
|---|---|---|---|---|---|---|---|---|
| Order | 1 | 2 | 5 | 1 | 5 | 4 | 18 | Highest lifecycle complexity; many workers; escrow+refund interlock; under parallel work (protected) |
| Payment | 1 | 3 | 5 | 1 | 5 | 4 | 19 | Expiry worker dormant, orphan recovery dormant, webhook race alerts; amount authority clean |
| Refund | 2 | 3 | 5 | 1 | 4 | 4 | 19 | Gateway + ledger + escrow safety; flag-gated; many entry paths (all funneled) |
| Coins | 1 | 2 | 3 | 1 | 3 | 3 | 13 | Reservation lifecycle + refund handler; unique index idempotency |
| Discount | 1 | 2 | 2 | 1 | 3 | 2 | 11 | Simple lifecycle; parallel protected |
| Chat | 1 | 2 | 3 | 2 | 5 | 3 | 16 | Realtime + projections + moderation + support adapters; mobile logout disposal gap |
| Notification/Push | 1 | 2 | 3 | 2 | 5 | 3 | 16 | Delivery log + retry queue + per-domain shards; push suppression notes |
| Realtime/WS | 1 | 3 | 2 | 1 | 4 | 3 | 14 | Room-list producer-only; presence dual-write; eviction paths |
| Seller/Subscription | 1 | 2 | 3 | 1 | 4 | 3 | 14 | Expiry/recovery/reconciliation workers; onboarding SSOT |
| KYC/Verification | 1 | 2 | 3 | 1 | 3 | 2 | 12 | 8-state lifecycle + reviewed-bank snapshot future hooks |
| Moderation | 1 | 2 | 3 | 1 | 4 | 3 | 14 | Enforcement fanout + dormant ban handler + appeals |
| Social (content/feed/comment/like/graph) | 1 | 1 | 2 | 1 | 3 | 2 | 10 | Clean authority; evaluator shadow dormant |
| Auction | 1 | 2 | 3 | 1 | 3 | 3 | 13 | Start/end/settlement + BNR + anti-sniping |
| Listing/Product | 2 | 2 | 2 | 1 | 4 | 2 | 13 | Product created inline by auction (split authority, MEDIUM) |
| Finance/Ledger/Wallet | 3 | 2 | 3 | 2 | 5 | 3 | 18 | Two balance tables; shadow integrity workers; escrow drift detection |
| Withdrawal/Payout | 2 | 3 | 3 | 1 | 3 | 3 | 15 | Env-gated worker; sandbox/Iris split; completion-safety check |
| Support | 1 | 3 | 3 | 1 | 2 | 3 | 13 | SLA worker OFF; escalate-to-dispute; user-reply handler |
| Dispute | 1 | 2 | 3 | 1 | 4 | 2 | 13 | Freeze authority + timeout escalate-only |
| Shipping | 1 | 2 | 2 | 1 | 4 | 2 | 12 | Quotes stateless; coverage authority; product shipping |
| Address/Saved | 1 | 2 | 1 | 2 | 2 | 2 | 10 | Mobile saved-item residue |
| Platform (outbox/config/capability/alert/audit) | 2 | 2 | 2 | 2 | 5 | 3 | 16 | configs vs platform_configs duplicate risk; outbox hub of everything |
| Projection | 2 | 3 | 1 | 1 | 3 | 2 | 12 | Dormant by design; fallback live |
| Mobile Report | 1 | 4 | 1 | 3 | 1 | 4 | 14 | Unoverridden providers → submission not proven |
| Mobile Negotiation | 1 | 4 | 1 | 3 | 1 | 3 | 13 | No route/consumers; dead DI scaffolding |
| Mobile Saved-item | 1 | 3 | 1 | 3 | 1 | 3 | 12 | 3 construction sites; swallowed errors |

---

# 19. PRIORITIZED DEEP-AUDIT CANDIDATES

## P0 — Refund lifecycle (finance.refund + order escrow interlock)
- **Why:** Highest money-safety surface; gateway orchestration + ledger reversal + escrow release interlock + seller/buyer/admin entry points + webhook ack; feature-flagged admin initiate; many regression/proof tests already exist (good baseline to converge on).
- **Systemic question:** "When money moves, is there exactly one authority, and does every entry point funnel through it?" — the highest-value question in the whole system.
- **Cross-domain leverage:** order, payment, escrow, ledger, dispute freeze, coins refund, notification.
- **Known dependencies:** order/payment/coins/discount = PARALLEL PROTECTED. Refund is adjacent — audit must be read-only and NOT touch those domains' mutable internals.
- **Protected boundary:** order, payment, coins, discount.

## P0 — Finance/Ledger authority (ledger_entries vs balances; escrow vs wallet)
- **Why:** Two balance representations (`account_balances`/`financial_accounts`) + escrow/wallet dual state; integrity + total-money workers exist in shadow mode — a deep audit can define canonical balance authority and reconcile the shadow detectors.
- **Systemic question:** "What is the single source of money truth, and which tables are derived?"
- **Leverage:** every money domain (order/payment/refund/withdrawal/coins) reads finance state.

## P1 — Chat realtime + logout lifecycle
- **Why:** Most-connected domain; WS + REST + projections + moderation + support adapters; mobile repo disposal gap on logout (account-isolation risk); room-list realtime producer-only.
- **Systemic question:** "What happens to client state and subscriptions on account change?" — reusable across every mobile domain.
- **Leverage:** notification, realtime, moderation, support, presence.

## P1 — Notification/Push delivery authority
- **Why:** Large handler shard surface; delivery log vs notifications split; push suppression notes; block/mute policy adapters; cleanup/retry workers.
- **Systemic question:** "Is there one notification contract, or do handler shards diverge?"
- **Leverage:** every domain that emits events.

## P2 — Platform/outbox/config
- **Why:** Outbox is the event hub; `configs` vs `platform_configs` duplicate-config risk (UNKNOWN); capability matrix is the auth spine.
- **Systemic question:** "Which config table does runtime actually read, and are event producers/consumers fully registered?"

## P2 — Social moderation enforcement
- **Why:** Content/comment/listing/chat/user enforcement + appeals + warnings + dormant ban handler; cross-domain visibility mutation.
- **Systemic question:** "Does moderation mutate each domain through the domain's own authority, or does it own visibility directly?"

## P3 — Mobile fragmentary domains (Report, Negotiation, Saved-item, Onboarding scaffolding)
- **Why:** Dead scaffolding + unproven paths; low cross-domain impact but cheap to converge.

## P3 — Projection read model
- **Why:** Dormant by design; fallback live; low urgency.

---

# 20. RECOMMENDED NEXT DOMAIN

## Refund lifecycle (finance.refund) — as a READ-ONLY deep audit

**Why it is the highest-leverage SAFE target while Order/Payment/Coins/Discount are under parallel work:**

1. **High cross-domain impact without touching protected internals:** Refund sits at the intersection of order (escrow release), payment (gateway), finance (ledger reversal), dispute (freeze), and coins (refund). A refund deep-audit can map all these edges **read-only** — reading the interfaces/adapters rather than mutating the protected domains' implementations.
2. **Highest lifecycle risk already surfaced:** gateway_status + refund.status + escrow_status + ledger reversal + coins refund + notification fanout is the densest state machine outside the protected boundary. The parallel work is *changing* order/payment/coins — a refund audit produces a **dependency contract** those changes must respect, rather than racing them.
3. **Systemic pattern reveal:** refund is the canonical "money moves backwards" flow. Auditing it answers the highest-value systemic question (single money authority + entry-point funneling) that the protected domains will also need.
4. **Reusable cleanup map:** the refund surface already has strong test evidence (real-DB proofs, gateway webhook spy, refund_math) — a convergence audit can produce an authority + lifecycle + event map the whole finance domain can reuse.
5. **Not under parallel modification** (refund is outside the protected four), so the audit will not collide.

Alternative (if refund feels too close to the protected boundary): **Finance/Ledger authority** is the second-best safe target — it is upstream of the protected domains and read-only.

---

# 21. NOT-PROVEN QUESTIONS

Questions that materially affect prioritization (unresolved in this pass):

1. **Does runtime read `configs` or `platform_configs`?** (duplicate config tables — affects platform/config authority score).
2. **Are `listing_status_enum`, `listing_origin_enum`, `comment_type_enum` dead schema residue?** (affects schema-surface residue score; agent truncated before confirming).
3. **Is `CoreUserHandler` (legacy stub) still reachable via any route that matters?** (affects duplicate-authority score for user domain).
4. **Mobile report submission end-to-end** — is any production code path actually overriding `reportCurrentUserIdProvider` / S3 / userName providers? (mobile exploration found none in main.dart; if truly unoverridden, the report feature is dead in production).
5. **Chat repository disposal on logout** — is there any `ref.onDispose`/reset path outside the providers we found? (account-isolation risk).
6. **PaymentExpiryWorker dormant vs OrderPaymentTimeoutWorker active** — what is the *intended* runtime split under the parallel work, and is any payment-row-only expiry reachable today?
7. **Payout worker + Iris gateway activation state** in the deployment env (PAYOUT_ENABLE_WORKER / provider) — config-level, not code-level.
8. **Redis vs DB presence** — is presence authoritative in Redis with DB as durable fallback, or is there drift risk on reconnect storms?
9. **Projection worker** — is it intended to be enabled at all, or permanently superseded by the live fallback?

---

# 22. FINAL VERDICT

## PARTIALLY PROVEN

The system is **architecturally converged** — single DI graph, single route tree, single outbox event bus, canonical authorities per domain with funneled writers, aggressive migration-level cleanup of legacy tables/roles/columns, and deliberate conservative worker gating (dangerous-dormant, shadow-mode integrity checks, env-gated activation). That is a genuinely strong authority posture.

However, **runtime proof is partial by design and by evidence**:
- Several production-reachable surfaces are deliberately dormant (PaymentExpiryWorker, UserBanEventHandler, SLA worker, ProjectionWorker, payout worker, gateway-refund flag, evaluator shadow seams) — their absence is *documented* but unproven at runtime.
- The two highest-risk money domains (Order/Payment/Coins/Discount) are under active parallel work — their final runtime posture is NOT PROVEN and intentionally not audited here.
- Mobile has proven dead scaffolding (report/negotiation/saved-item/onboarding/GetIt) and at least one account-isolation gap (chat repo disposal on logout).
- Two concrete UNKNOWNs remain (configs vs platform_configs; legacy enum residue in schema).

No CONTRADICTED findings surfaced in this pass: no domain was found where two live writers fight over the same business state outside a single funneled authority, and the known historical conflicts (seller-role authority, payment amount authority, comment/chat commerce references, share references) are documented as resolved by migration or code.

**Verdict: PARTIALLY PROVEN — with Order/Payment/Coins/Discount explicitly PARALLEL / PROVENANCE UNKNOWN, and Refund lifecycle recommended as the next read-only deep-audit target.**

---

*Audit scope: read-only. No source, schema, migration, configuration, or documentation was modified. Order/Payment/Coins/Discount were intentionally not deep-audited. Evidence: backend/internal (serverboot, worker, realtime, migrations), cmd/core_server (routes), apps/mobile/lib, docs/operations/canonical-runtime-paths.md.*
