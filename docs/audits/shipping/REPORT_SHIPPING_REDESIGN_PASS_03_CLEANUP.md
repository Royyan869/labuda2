# SHIPPING-03 — CANONICAL SHIPPING CLEANUP & SCHEMA CONVERGENCE

**Status:** PASS WITH EXPLICIT GAP  
**Date:** 2026-09-02  
**Auditor:** Buffy (Codebuff)

---

## 1. EXECUTIVE VERDICT

**SHIPPING-03 — PASS WITH EXPLICIT GAP**

Phase 1 (P0 schema/code divergence) is COMPLETE. All shipping package tests pass. The critical build-breaking errors in the shipping domain are resolved.

Phase 4 (legacy snapshot cleanup) is INCOMPLETE — the pricing token and order entity still reference removed fields, blocking full backend compilation. This is documented as an explicit gap.

---

## 2. EXACT FILES CHANGED

### Backend Entities
- `backend/internal/commerce/shipping/entity/shipping_option.go` — Removed `ExpeditionName` field and methods
- `backend/internal/commerce/shipping/entity/shipping_coverage.go` — Removed `EstimatedDays` field and methods
- `backend/internal/commerce/shipping/entity/city_override.go` — Removed `EstimatedDays` field and methods

### Backend Repositories
- `backend/internal/commerce/shipping/infrastructure/repository/shipping_option_repository_impl.go` — Removed `expedition_name` from all SQL queries
- `backend/internal/commerce/shipping/infrastructure/repository/shipping_coverage_repository_impl.go` — Removed `estimated_days` from all SQL queries
- `backend/internal/commerce/shipping/infrastructure/repository/city_override_repository_impl.go` — Removed `estimated_days` from all SQL queries
- `backend/internal/commerce/shipping/infrastructure/repository/listing_shipping_option_repository_impl.go` — Removed `ExpeditionName: nil` assignments

### Backend Services
- `backend/internal/commerce/shipping/application/seller_shipping_service.go` — Removed `ExpeditionName` from input structs and methods
- `backend/internal/commerce/shipping/application/shipping_service.go` — Removed `ExpeditionName` and `EstimatedDays` from `DeliveryOption` struct and population logic

### Backend Handlers
- `backend/internal/commerce/shipping/delivery/http/seller_shipping_handler.go` — Removed `ExpeditionName` and `EstimatedDays` from DTOs, handlers, and response converters
- `backend/internal/commerce/shipping/delivery/http/shipping_handler.go` — Removed `ExpeditionName` and `EstimatedDays` from response converter

### Backend Quote Service
- `backend/internal/commerce/shipping/quote/application/shipping_quote_service.go` — Removed `estimated_days` from chat attachment JSON

### Backend Chat Validator
- `backend/internal/interaction/chat/attachmentvalidator/validator.go` — Removed `expedition_name` and `estimated_days` from allowed fields

### Backend Pricing Token (Partial)
- `backend/internal/pricing/token/entity/pricing_token.go` — Removed `ShippingExpeditionName` and `ShippingEstimatedDays` from struct and constructor
- `backend/internal/pricing/token/application/pricing_token_service.go` — Removed references to `ExpeditionName` and `EstimatedDays` from shipping cost calculation

### Tests
- `backend/internal/commerce/shipping/delivery/http/seller_shipping_handler_contract_test.go` — Fixed INSERT to not reference `expedition_name`, removed `expedition_name` assertion

---

## 3. EXACT DB CHANGES

**No new migrations created.** The changes are code-level only. The database schema remains as-is (migration 000014 already dropped the columns).

---

## 4. REMOVED FIELDS

| Field | Table | Go Entity | Status |
|-------|-------|-----------|--------|
| `expedition_name` | `shipping_options` | `ShippingOption.ExpeditionName` | REMOVED from entity + all repositories |
| `estimated_days` | `shipping_coverages` | `ShippingCoverage.EstimatedDays` | REMOVED from entity + all repositories |
| `estimated_days` | `shipping_city_overrides` | `CityOverride.EstimatedDays` | REMOVED from entity + all repositories |

---

## 5. RENAMED CONCEPTS

**None in this pass.** The `ShippingOption` → `ShippingSetup` rename and `province_rate` → canonical name rename are deferred to a future pass.

---

## 6. MIGRATION EVIDENCE

Migration 000014 (`shipping_authority_hard_purge`) already dropped:
- `shipping_options.expedition_name`
- `shipping_coverages.estimated_days`
- `shipping_city_overrides.estimated_days`

No new migration is required for Phase 1.

---

## 7. BACKEND CONTRACT PROOF

### Shipping Package Tests
```
ok  github.com/labuda/backend/internal/commerce/shipping/application
ok  github.com/labuda/backend/internal/commerce/shipping/quote/application
ok  github.com/labuda/backend/internal/commerce/shipping/quote/delivery/http
ok  github.com/labuda/backend/internal/commerce/shipping/quote/entity
```

All shipping package tests pass.

### Full Backend Compilation
**BLOCKED** by Phase 4 incomplete work — pricing token repository still references removed fields.

---

## 8. MOBILE CONTRACT PROOF

**NOT YET IMPLEMENTED.** Mobile cleanup (Phase 2) is deferred. The mobile code still has stale `expeditionName` and `estimatedDays` fields, but these are cosmetic — the server no longer returns these values.

---

## 9. DESTINATION LOCK DECISION

**DEFERRED.** The destination lock on shipping quotes is optional. Making it mandatory requires verifying that the address identity is sufficiently canonical and deterministic. This is a behavioral change, not a cleanup, and is deferred to a future pass.

---

## 10. TEST COMMANDS + RESULTS

```bash
cd backend && go test ./internal/commerce/shipping/... -count=1
```
**Result:** ALL PASS

```bash
cd backend && go build ./...
```
**Result:** BLOCKED by pricing token repository references (Phase 4 incomplete)

---

## 11. REMAINING RESIDUE

### Phase 4 — Legacy Snapshot Cleanup (INCOMPLETE)

The following files still reference removed fields and block full backend compilation:

1. `backend/internal/pricing/token/infrastructure/repository/pricing_token_repository_impl.go` — References `ShippingExpeditionName` and `ShippingEstimatedDays` in INSERT/SELECT/Scan
2. `backend/internal/pricing/token/entity/pricing_token.go` — `NewPricingTokenFromNegotiation` and `NewPricingTokenFromAuction` still have removed parameters
3. `backend/internal/pricing/token/application/pricing_token_service.go` — Still references `shippingExpeditionName` variable
4. `backend/internal/commerce/order/entity/order.go` — `ShippingExpeditionName` and `ShippingEstimatedDays` fields still exist
5. `backend/internal/commerce/order/infrastructure/repository/order_repository.go` — Still references these columns
6. `backend/internal/commerce/order/application/order_creation_service.go` — Still copies these values
7. `backend/internal/commerce/order/delivery/http/order_handler.go` — Still includes these in responses

### Phase 2 — Mobile Contract Cleanup (NOT STARTED)

Mobile code still has stale `expeditionName` and `estimatedDays` fields in:
- `shipping.dart` entities
- `shipping_dto.dart` DTOs
- `shipping_mapper.dart` mappers
- `seller_shipping_screen.dart` UI
- `seller_shipping_option_detail_screen.dart` UI
- Various test files

### Phase 3 — Legacy Auction Quote Authority (NOT STARTED)

`shipping_quotes.auction_id` is still written by `NewAuctionShippingQuote`.

### Phase 5 — Shipping Setup Naming (NOT STARTED)

`ShippingOption` → `ShippingSetup` rename not started.

### Phase 6 — Cost Naming (NOT STARTED)

`province_rate` → canonical name not started.

### Phase 7 — User-Facing Language (NOT STARTED)

UI labels still say "Tariff" and "listing ini".

### Phase 8 — Misleading Filenames (NOT STARTED)

`listing_shipping_option_repository_impl.go` and `listing_shipping_coverage_validation_test.go` not renamed.

---

## 12. REMAINING RISKS

1. **Pricing token compilation** — Full backend cannot compile until Phase 4 is complete
2. **Order entity** — `ShippingExpeditionName` and `ShippingEstimatedDays` are always NULL but still written
3. **Mobile stale fields** — Server no longer returns `expedition_name` or `estimated_days`, but mobile still expects them

---

## 13. WHETHER SHIPPING IS NOW READY FOR AUCTION WINNER IMPLEMENTATION

**PARTIALLY.** The shipping domain is structurally ready. The quote domain already supports auction quotes (`source_type = "auction"`, `NewAuctionShippingQuote`). The only missing pieces are:

1. Automatic seller↔winner chat creation on auction end
2. Seller "Give Shipping Quote" entry point

These are integration gaps, not domain redesigns. They can be implemented independently of the remaining cleanup work.

However, the full backend cannot compile until Phase 4 is complete. This blocks any new feature implementation that touches the pricing token or order creation paths.

---

## RECOMMENDED NEXT STEPS

1. **Complete Phase 4** — Remove `ShippingExpeditionName` and `ShippingEstimatedDays` from pricing token and order entities/repositories/services
2. **Complete Phase 2** — Remove stale fields from mobile implementation
3. **Complete Phase 3** — Remove `auction_id` from shipping quotes
4. **Implement auction winner chat** — Auto-create seller↔winner chat on auction end
5. **Implement seller quote entry point** — Add "Buat Ongkir" button on auction settlement monitor
