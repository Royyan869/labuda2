# Auction Bid-Win → Shared Checkout → Canonical Order Machine — RECONSTRUCTION PLAN

## VERDICT (from completed deep audit — read-only phase done)

Current `POST /auctions/:id/claim` MIXES three domain responsibilities in ONE transaction
(`backend/internal/commerce/auction/delivery/http/auction_handler.go:899-1063`):
1. **Auction settlement** — eligibility lock (winner/deadline/unclaimed), `auction.ResolveShipping(now)`, `auction.OrderID` binding.
2. **Checkout** — buyer address + shipping option/quote + discount + coins (collected by mobile `AuctionClaimShippingModal`, a full-screen Checkout-lite).
3. **Order creation** — server-internal pricing-token generation (`GenerateForAuction`), snapshot (`buildClaimPricingSnapshot`, hardcodes `PaymentMethod:"default"`), `CreateOrderFromAuction`.

Meanwhile the canonical path already exists and works for For Sale + Auction Buy-Now:
`POST /pricing/preview` (incl. auction **bid-win branch — already implemented** in
`backend/internal/pricing/token/application/pricing_token_service.go:1248-1262`) →
`POST /orders` → `OrderCreationService` → `finalizeOrderCreationTx` → Order machine.

Mobile bid-win bypasses `CheckoutScreen` entirely (`auction_detail_screen.dart:653-712`,
`chat_detail_screen.dart:1040-1086`) and lands on Payment Result instead of Order Detail.

**RECONSTRUCT.** Claim endpoint is OBSOLETE after reconstruction → PURGE TOTAL.

## OWNER BUSINESS TRUTH (authority)

- All purchases: `X → Checkout → Order`. Bid-win included. ONE Checkout, ONE Order machine.
- Checkout owns: address, shipping, pricing preview, readiness, Order creation.
- Payment method for bid-win is chosen at Order Detail AFTER Order creation
  (`payment_method_code = nil` at creation; first `POST /payments` binds — backend
  migration `000127` already encodes this invariant).
- 24h window: seller setup + buyer response — an **Auction settlement concern**.
  After Order creation, lifecycle belongs to the canonical Order machine.
- No Auction-specific checkout, no second pricing engine, no claim-specific order engine,
  no default/fallback payment method.

## CANONICAL MODEL (decided)

### Two consecutive windows, one authority each
- **Window 1 — Auction settlement / buyer-response**: `end_at + 24h` (derived, `Auction.SettlementDeadline()`), starts at auction end, ends when the buyer **completes Checkout → Order created** (inside the order tx: `ResolveShipping(now)` + `OrderID` bound). Enforced by `AuctionSettlementWorker` (only selects `shipping_resolved_at IS NULL`); expiry → violation + auto-reschedule; **no order exists in this window**. Unchanged worker.
- **Window 2 — Order payment**: starts at Order creation. `orders.payment_expires_at = creation + 24h` for bid-win orders (**unbound-method order policy**, preserves current buyer-facing behavior; value decided by Order domain, NOT inherited from auction). Enforced ONLY by `PaymentExpiryWorker` + `OrderPaymentTimeoutWorker` → `OrderService.Expire` → `buyer_bnr` + auction auto-reschedule via order-completion service. Auction worker has zero authority post-order (proven: worker query requires `shipping_resolved_at IS NULL`).
- Payment success settles auction `waiting_settlement → ended` via `settleAuctionOnPaymentSuccess` (unchanged).

### Payment method
- Bid-win: `payment_method_code` absent at `POST /orders`; bound at first `POST /payments` (existing code path in `serverboot/dependencies.go:3737-3755` + `order_repository.go` COALESCE). No pre-order method loading for bid-win (fee unknown until method chosen — do not fake final total; show escrow base PD+S + honest copy).
- For Sale / Buy-Now: method still required at creation (unchanged).

### Claim endpoint
OBSOLETE. Responsibilities redistribute:
- Eligibility (winner/deadline/unclaimed/winner-price-match) → preview (early honest failure) + **authoritative re-check under `FOR UPDATE` in `POST /orders` auction bid-win branch**.
- `ResolveShipping` + `OrderID` binding → same `POST /orders` tx (mirrors buy-now's bind+`End()` at `order_handler.go:1203-1215`).
- Token/snapshot/order creation → existing POST /orders machinery + `OrderCreationService.CreateFromAuction` (kept as the legitimate pre-Order auction **context adapter** used by BOTH buy-now and bid-win; it already funnels into shared `finalizeOrderCreationTx`).

## IMPLEMENTATION STEPS

### PHASE A — Backend: make POST /orders the sole bid-win order-creation authority

A1. `backend/internal/commerce/order/delivery/http/order_handler.go`
- `CreateOrderRequest.PaymentMethodCode`: drop `binding:"required"` (line 918); enforce in handler:
  - `source_type=for_sale` OR auction **buy-now** → required (400 if empty; existing
    `applySelectedPaymentMethod` already errors on empty — keep explicit early check);
  - auction **bid-win** → must be ABSENT (400 `PAYMENT_METHOD_NOT_ALLOWED` if provided).
- Auction branch (lines 1167-1215): after `GetForUpdate(sourceID)`:
  - existing buy-now predicate (`StatusActive && BuyNowPrice != nil`) stays;
  - NEW bid-win predicate: `(StatusEnded || StatusWaitingSettlement) && HasWinner() && OrderID == nil`:
    - `*auction.WinnerID() == userID` else 403 `ErrNotWinner`;
    - `time.Now().After(auction.SettlementDeadline())` else 410 `ErrSettlementDeadlinePassed`;
    - `validatedToken.UnitPrice.Int64() == *auction.WinningBid()` else price-changed error;
    - `auction.ResolveShipping(time.Now())` (first-resolution-wins);
    - `CreateFromAuction(... AuctionSettlementType: AuctionSettlementBidWin,
      PaymentMethodCode: nil, ShippingResolvedAt: *auction.ShippingResolvedAt,
      WinningBid: validatedToken.UnitPrice.Int64(), IdempotencyKey: &idempotencyKey)`;
    - `auction.OrderID = &order.ID`; **do NOT `End()`** (stays `waiting_settlement` until payment success);
      `auctionRepo.UpdateTx`;
  - else → 409 "auction is not available for checkout".
- Error mapping after tx: `errors.Is` for `auctionEntity.ErrNotWinner`→403,
  `ErrSettlementDeadlinePassed`→410 (`AUCTION_SETTLEMENT_DEADLINE_PASSED`),
  `ErrAlreadySettled`→409 (`AUCTION_ALREADY_SETTLED`); update buy-now comment at
  line 1207 ("mirrors claim handler" → obsolete wording).
- `resolveOrderCoins` is reused as-is (kill duplicate `resolveClaimCoins` in purge phase).

A2. `backend/internal/pricing/token/application/pricing_token_service.go` — `GenerateForAuction`
- Bid-win branch (1248-1262): add settlement-deadline check
  (`now.After(auction.SettlementDeadline())` → error) so Checkout preview fails early
  and honestly. (Order creation re-checks under lock — preview is advisory only.)

A3. `backend/internal/commerce/order/application/order_creation_service.go`
- Reframe comments only: `calculateAuctionPaymentExpiry`/`auctionOrderPaymentExpiry`
  (lines 530-554) = Order-owned payment-window policy for unbound-method bid-win orders
  (creation anchor + 24h), not an auction settlement deadline. Behavior unchanged.
- Comment at 524-527, 832: "auction-claim path" → "bid-win checkout path (method chosen
  at Order Detail)".

A4. Update comments referencing claim as the bid-win path:
- `routes_core.go` (none besides route itself), `auction_settlement_worker.go:37`,
  `internal/interaction/notification/policy/category.go:203-206` ("24h claim window" →
  "24h settlement/checkout window" copy), `chat_order_route_absent_contract_test.go`
  (canonical path doc: mention bid-win via POST /orders),
  `backend/tests/shipping_quote_auction_integration_test.go:262` ("Mirror the /claim
  handler" → "Mirror the POST /orders bid-win path").

### PHASE B — Backend purge (claim design death)

B1. Delete route `POST /auctions/:id/claim` — `routes_core.go:316`.
B2. Delete from `auction_handler.go`: `ClaimAuction`, `ClaimAuctionRequest`,
  `buildClaimPricingSnapshot`, `resolveClaimCoins`, orphan comment line 868; prune now-unused imports.
B3. Delete from `auction_service.go`: `CreateOrderFromAuction` + `CreateOrderFromAuctionInput`
  (1018-1101), `auctionShippingResolvedAt` (1103-1113), `GeneratePricingTokenForAuctionClaim`
  + `GeneratePricingTokenForAuctionInput` (1435+), `PersistAuctionUpdate` (1309+)
  — verified sole caller was the claim handler.
B4. Delete tests: `auction/delivery/http/build_claim_pricing_snapshot_test.go`,
  `auction_claim_error_test.go`, `middleware/auction_claim_gate_test.go`.
B5. Rewrite `auction/application/auction_settlement_deadline_test.go`: deadline boundary
  now enforced in `GenerateForAuction` (pricing token service) and the order-handler
  bid-win branch — retarget the test to `PricingTokenService.GenerateForAuction`
  (keep same fixtures if the harness allows; otherwise construct the pricing-token
  service harness mirroring existing pricing token tests).
B6. `backend/tests/shipping_quote_auction_integration_test.go` + `order/tests/auction_settlement_test.go`
  + `tests/order_item_product_identity_convergence_integration_test.go` use service-level
  `CreateFromAuction` — KEEP (that adapter survives); only comment/copy updates.
B7. Grep-driven residue sweep in backend for `claim` (auction-scoped), `ClaimAuction`,
  `buildClaimPricingSnapshot`, `GeneratePricingTokenForAuctionClaim`, `PersistAuctionUpdate`.

### PHASE C — Mobile: bid-win enters the SAME CheckoutScreen

C1. `auction/presentation/checkout_intent.dart`
- `AuctionCheckoutIntent` gains `bidWin` (default false), optional `shippingQuoteId`,
  `chatId`; `openAuctionCheckout` appends `bid_win=1` (+ quote params) to the query;
  rewrite stale header comment (claim flow no longer exists).
C2. `checkout_router_module.dart`: parse `bid_win` query → `CheckoutScreen.bidWin`.
C3. `checkout_screen_impl.dart` + widgets:
- New flag `bool get _isBidWin => widget.bidWin`; rename semantics:
  `_isAuctionWinner` → `_isBidWin` (winner banner, CTA label "Amankan Kemenangan",
  discount contextType stays 'auction' via `_isAuctionContext => auctionId != null`).
- Payment-method UI (`PaymentMethodTrigger` block, lines 614-626) hidden when `_isBidWin`;
  coin toggle no longer triggers `_loadPreOrderPaymentMethods` for bid-win.
- `_readiness`: when `_isBidWin`, skip payment-method inputs entirely (pass
  hasSelectedPaymentMethod: true / skip loading flags) — readiness = product+address+
  shipping+current preview+token.
- Honest total for bid-win: summary shows escrow base (preview `totalPayableAmount`)
  + copy that service fee follows the method chosen at payment ("Biaya layanan
  dihitung saat memilih metode pembayaran").
C4. `checkout_screen_logic.dart`:
- `_checkoutLoadPreOrderPaymentMethods`: early-return no-op when `_isBidWin`.
- `_checkoutHandleCreateOrder`: readiness switch handles bid-win (no
  missingPaymentMethod case reachable); `submittedPaymentMethodCode` nullable —
  omit from request when bid-win.
C5. `checkout_request.dart` + `checkout_repository_impl.dart`:
- `paymentMethodCode` → `final String?`; wire key `payment_method_code` sent only when
  non-null.
C6. `checkout_order_summary_section.dart` — fix latent auction defect: for auction
  context do NOT `ref.watch(forSaleDetailProvider(forSaleId))` (currently collapses the
  whole summary incl. readiness indicator for auction checkout). Render indicator +
  content from preview; forSale-only extras (koi details) only in for-sale mode.
C7. `auction_detail_screen.dart`: `_handleWinnerCheckout` → seller-trust/auth/winner/
  productId guards stay, then `openAuctionCheckout(context, ref,
  AuctionCheckoutIntent(auctionId: auction.id, bidWin: true))`. Delete `_showClaimDialog`.
C8. `chat_detail_screen.dart` `_handleShippingQuotePurchase` auction branch →
  `openAuctionCheckout(context, ref, AuctionCheckoutIntent(auctionId: target.auctionId!,
  bidWin: true, shippingQuoteId: shippingQuote.offerId, chatId: widget.chatId))`.
  Chat still resolves no route/product id (intent does everything). Update header docs
  (lines 52-57).
C9. Mobile purge:
- Delete `auction_claim_shipping_modal.dart` (+ exports/references).
- Delete `claimAuction` from `auction_repository.dart`, `auction_repository_impl.dart`,
  `auction_remote_datasource.dart`, `auction_notifier.dart` (incl. `_isClaiming`).
- Sweep `lib/` for `claimAuction`, `AuctionClaimShippingModal`, `'/claim'`.

C10. Mobile tests:
- DELETE: `auction_claim_shipping_modal_inset_authority_test.dart`,
  `auction_claim_address_convergence_test.dart`.
- UPDATE `auction_contract_p1_test.dart`: remove claim request/response contract section.
- INVERT `chat_checkout_authority_contract_test.dart`: auction winner path MUST be
  `openAuctionCheckout` + `AuctionCheckoutIntent(... bidWin: true ...)` carrying
  shippingQuoteId+chatId; MUST NOT contain `AuctionClaimShippingModal`/`claimAuction`;
  keep "chat builds zero checkout routes" + "chat resolves no product id".
- UPDATE `checkout_single_funnel_contract_test.dart`: auction detail winner CTA routes
  via `openAuctionCheckout` with `bidWin: true`; auction detail contains no claim flow.
- UPDATE `checkout_readiness_contract_test.dart`: bid-win ready WITHOUT payment method;
  non-bid-win still requires one.
- UPDATE `auction_notifier_authority_test.dart`, `auction_detail_restriction_dispatch_test.dart`,
  `seller_auctions_screen_test.dart`: drop `claimAuction` fakes/mocks.
- ADD: `checkout_request` test proof — bid-win request omits `payment_method_code` on the wire.
- KEEP `checkout_order_creation_handoff_contract_test.dart` (Order Detail handoff — now
  also applies to bid-win).
- Sweep all mobile tests for `claimAuction|AuctionClaimShippingModal|/claim`.

### PHASE D — Proof

D1. Backend: `go build ./...`; `go test` for
`internal/commerce/order/...`, `internal/commerce/auction/...`, `internal/pricing/token/...`,
`internal/middleware/...`, `internal/worker/...`; E2E `backend/tests` (Postgres-dependent
tests run only if DB available — note in report otherwise).
D2. Mobile: `dart analyze` / `flutter analyze` on apps/mobile; `flutter test` for
checkout, auction, chat contract tests.
D3. POSITIVE PROOF (test-enforced): For Sale checkout unchanged; Auction Buy-Now checkout
unchanged; Bid-Win: auction detail/chat → same CheckoutScreen (bid_win) → preview →
POST /orders (no payment method) → Order Detail → PaymentMethodPicker (existing unbound
order path `order_detail_handlers.dart:273-298`) → POST /payments (existing first-bind).
D4. NEGATIVE PROOF (grep + contract tests): zero `claimAuction`/`AuctionClaimShippingModal`/
`POST /auctions/:id/claim`/`ClaimAuction`/`buildClaimPricingSnapshot`/`GeneratePricingTokenForAuctionClaim`
in lib+backend; no Auction-specific checkout screen; no payment picker before order for
bid-win; no second pricing authority.
D5. Runtime proof: only if a runnable stack is available; otherwise declare as known
limitation (compile+test proof only).

## CLASSIFICATION SUMMARY

| Item | Class |
|---|---|
| POST /auctions/:id/claim + handler/service helpers | OBSOLETE → PURGE |
| AuctionClaimShippingModal + mobile claim repo/datasource/notifier | OBSOLETE → PURGE |
| Claim tests (backend + mobile), chat claim contract | OBSOLETE → PURGE/INVERT |
| OrderCreationService.CreateFromAuction + finalizeOrderCreationTx | CANONICAL (context adapter, shared engine) |
| Pricing preview auction bid-win branch | CANONICAL/REQUIRED (add deadline check) |
| AuctionSettlementWorker (window 1) | CANONICAL/REQUIRED (unchanged) |
| PaymentExpiryWorker + OrderPaymentTimeoutWorker (window 2) | CANONICAL/REQUIRED (unchanged) |
| payment_method_code nil + first-payment binding (000127) | CANONICAL/REQUIRED |
| shipping_resolved_at + OrderID set at order creation | CANONICAL (moves into POST /orders tx) |
| Bid-win payment window = creation+24h (unbound-method policy) | CANONICAL (preserved value; Order-owned authority; re-anchored on creation) |
| CheckoutScreen with bid_win context param | CANONICAL (one Checkout concept) |
| my_bids `'waiting_claim'` status string | CANONICAL (bid read-model vocabulary, not the claim API) |

## FORBIDDEN (will not be done)

No AuctionCheckoutScreen, no claim compatibility bridge, no dual flow, no default payment
method, no git rollback, no feature flags, no payment-method picker in checkout for
bid-win, no changes to escrow/payment/fulfillment machinery.
