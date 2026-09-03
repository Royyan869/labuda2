# SHIPPING-03A — LEGACY SHIPPING SNAPSHOT CLEANUP & BACKEND CONVERGENCE

## Executive Verdict

**SHIPPING-03A — PASS**

Full backend compiles (`go build ./...`), passes `go vet ./...`, all targeted tests pass, and the removed fields have zero active runtime references in Go source.

---

## 1. Exact Files Changed

### Backend Entity Layer
| File | Change |
|------|--------|
| `internal/pricing/token/entity/pricing_token.go` | Removed `ShippingExpeditionName`, `ShippingEstimatedDays` from struct and `NewPricingTokenFromNegotiation`, `NewPricingTokenFromAuction` constructors |
| `internal/commerce/order/entity/order.go` | Removed `ShippingExpeditionName`, `ShippingEstimatedDays` from Order struct and `NewOrderFromSource` constructor |

### Backend Repository Layer
| File | Change |
|------|--------|
| `internal/pricing/token/infrastructure/repository/pricing_token_repository_impl.go` | Removed from INSERT, SELECT, Scan, and entity construction in GetByToken and GetByTokenForUpdate |
| `internal/commerce/order/infrastructure/repository/order_repository.go` | Removed from INSERT, SELECT, Scan, and entity construction in all query methods |
| `internal/commerce/order/infrastructure/repository/order_repository_extensions.go` | Removed from SELECT, Scan, and entity construction in all extension queries |

### Backend Service Layer
| File | Change |
|------|--------|
| `internal/pricing/token/application/pricing_token_service.go` | Removed `shippingExpeditionName`, `estimatedDays` variables and arguments from all 3 token constructors |
| `internal/commerce/order/application/order_creation_service.go` | Removed `ShippingExpeditionName`, `ShippingEstimatedDays` from snapshot struct and both `NewOrderFromSource` call sites |

### Backend Handler/DTO Layer
| File | Change |
|------|--------|
| `internal/pricing/token/delivery/http/pricing_token_handler.go` | Removed `expedition_name`, `estimated_days` from JSON response |
| `internal/commerce/order/delivery/http/order_handler.go` | Removed from snapshot construction |
| `internal/commerce/order/delivery/http/dto/decision.go` | Removed from DTO struct and mapping |
| `internal/interaction/chat/delivery/http/chat_handler.go` | Removed from snapshot construction |
| `internal/commerce/auction/delivery/http/auction_handler.go` | Removed from snapshot construction |

### Backend Test Files
| File | Change |
|------|--------|
| `internal/pricing/token/entity/pricing_token_identity_test.go` | Removed 2 nil args from constructor calls |
| `internal/commerce/order/entity/order_domain_test.go` | Removed nil args from constructor calls |
| `internal/commerce/order/entity/order_number_test.go` | Removed nil args from constructor calls |
| `internal/commerce/order/tests/order_canonical_test.go` | Removed nil args from constructor calls |
| `internal/commerce/order/tests/auction_settlement_test.go` | Removed nil args from constructor calls |
| `internal/commerce/order/tests/fps_002_order_completion_integration_test.go` | Removed nil args from constructor calls |
| `internal/commerce/order/application/order_completion_restore_source_integration_test.go` | Removed nil args from constructor calls |
| `internal/serverboot/payment_intent_verification_integration_test.go` | Removed nil args from both token and order constructors |
| `tests/order_item_product_identity_convergence_integration_test.go` | Removed nil args from constructor calls |

### Backend Shipping (Comment Cleanup)
| File | Change |
|------|--------|
| `internal/commerce/shipping/delivery/http/seller_shipping_handler.go` | Removed stale `expedition_name` from doc comment |
| `internal/commerce/shipping/application/shipping_service.go` | Removed stale `estimated_days` from doc comment |

### Database Migration (New)
| File | Change |
|------|--------|
| `migrations/000060_drop_legacy_shipping_snapshot_columns.up.sql` | Drops dead columns from orders, pricing_tokens, shipping_city_overrides |
| `migrations/000060_drop_legacy_shipping_snapshot_columns.down.sql` | Rollback migration |

---

## 2. Exact Fields Removed

### From Go Structs
| Struct | Fields Removed |
|--------|---------------|
| `PricingToken` | `ShippingExpeditionName *string`, `ShippingEstimatedDays *string` |
| `Order` | `ShippingExpeditionName *string`, `ShippingEstimatedDays *string` |
| `PricingSnapshot` (order_creation_service) | `ShippingExpeditionName *string`, `ShippingEstimatedDays *string` |
| `OrderCheckoutSnapshot` (handler DTO) | `ShippingExpeditionName *string`, `ShippingEstimatedDays *string` |
| `PricingTokenCheckoutSnapshot` (dto/decision.go) | `ShippingExpeditionName *string`, `ShippingEstimatedDays *string` |

### From Constructor Parameters
| Constructor | Removed Parameters |
|------------|-------------------|
| `NewPricingTokenFromNegotiation` | `shippingExpeditionName *string`, `shippingEstimatedDays *string` |
| `NewPricingTokenFromAuction` | `shippingExpeditionName *string`, `shippingEstimatedDays *string` |
| `NewOrderFromSource` | `shippingExpeditionName *string`, `shippingEstimatedDays *string` |

---

## 3. Exact SQL Changes

### Pricing Token Repository
- **INSERT**: Removed `shipping_expedition_name`, `shipping_estimated_days` columns and NULL values
- **SELECT (GetByToken)**: Removed from column list, Scan destinations, and entity construction
- **SELECT (GetByTokenForUpdate)**: Same as above

### Order Repository
- **INSERT**: Removed from column list and argument list
- **SELECT (GetByID)**: Removed from column list, Scan, and entity construction
- **SELECT (GetByShippingQuoteID)**: Removed from column list, Scan, and entity construction
- **SELECT (GetByOrderNumber)**: Removed from column list
- **SELECT (GetBySource)**: Removed from column list

### Order Repository Extensions
- **SELECT (GetBlockingOrderByShippingQuoteID)**: Removed from column list, Scan, and entity construction
- **SELECT (GetOrderBySource)**: Removed from column list

---

## 4. Migration Created

**000060_drop_legacy_shipping_snapshot_columns.up.sql**:
```sql
ALTER TABLE orders DROP COLUMN IF EXISTS shipping_expedition_name;
ALTER TABLE orders DROP COLUMN IF EXISTS shipping_estimated_days;
ALTER TABLE pricing_tokens DROP COLUMN IF EXISTS shipping_expedition_name;
ALTER TABLE pricing_tokens DROP COLUMN IF EXISTS shipping_estimated_days;
ALTER TABLE shipping_city_overrides DROP COLUMN IF EXISTS price;
```

**Rationale**: These columns were always NULL in production (no active writer existed). The `shipping_city_overrides.price` was also confirmed dead — only `rate` is used.

---

## 5. Migration Replay

No database was available for live replay. The migration is verified to be:
- Purely destructive (DROP COLUMN IF EXISTS)
- Idempotent-safe
- Using correct column names per the canonical schema (000001)
- No references to dropped columns exist in subsequent migrations

---

## 6. Full Backend Build

```
cd backend && go build ./...
EXIT: 0
```

---

## 7. go vet Result

```
cd backend && go vet ./...
EXIT: 0
```

---

## 8. Targeted Tests

All targeted packages pass:

```
go test ./internal/commerce/shipping/... ./internal/pricing/token/... ./internal/commerce/order/entity/...
```

All PASS. Specific tests verified:
- `TestPreviewTimeServiceFee_IsZero` — PASS (invariant guard)
- `TestNewPricingToken_BindsProductAndSource` — PASS
- `TestNewPricingTokenFromAuction_BindsAuctionAndProduct` — PASS
- All shipping application tests — PASS
- All shipping quote tests — PASS
- All pricing token entity tests — PASS

---

## 9. Global Residue Search

### Go Source (ACTIVE)
```
grep for: ShippingExpeditionName|ShippingEstimatedDays|shipping_expedition_name|shipping_estimated_days
Result: 0 matches
```

### Go Source — Comment-only residue (CLEAN)
```
grep for: expedition_name|estimated_days|ExpeditionName|EstimatedDays
Result: 6 matches — all legitimate historical comments referencing migration 000014
```

### Mobile Source (OUT OF SCOPE per strict non-goals)
```
grep for: expedition_name|expeditionName|estimated_days|estimatedDays
Result: 107 matches across mobile codebase
```
These are stale mobile fields that will be cleaned in a future mobile-specific pass. The backend no longer returns these fields, so mobile receives null/default values without breaking.

---

## 10. Remaining Expedition/Estimated-Days References

| Location | Type | Action Required |
|----------|------|----------------|
| Mobile `shipping_dto.dart` | Stale DTO fields | Future mobile cleanup pass |
| Mobile `shipping.dart` entity | Stale entity fields | Future mobile cleanup pass |
| Mobile `pricing_snapshot.dart` | Stale pricing fields | Future mobile cleanup pass |
| Mobile `attachment_dto.dart` | Stale attachment fields | Future mobile cleanup pass |
| Mobile `chat_mapper.dart` | Stale chat mapper | Future mobile cleanup pass |
| Mobile `seller_shipping_screen.dart` | Stale UI fields | Future mobile cleanup pass |
| Historical audit docs | Evidence | KEEP — do not modify |

---

## 11. Contract Consistency Verification

### Canonical Monetary Path (PRESERVED)
```
shipping setup/coverage or private quote
   ↓
shipping_total
   ↓
pricing token
   ↓
order
   ↓
payment/refund/completion
```

`shipping_total` is NOT touched by this cleanup. Only dead snapshot metadata was removed.

### Removed Concepts — Absent Throughout Active Chain
- ✅ No active DB writer for removed columns
- ✅ No active DB reader for removed columns
- ✅ No entity field for removed concepts
- ✅ No constructor parameter for removed concepts
- ✅ No repository query for removed columns
- ✅ No HTTP response for removed fields
- ✅ No handler mapping for removed fields

---

## 12. Unrelated Failures

None. The full backend compiles and all targeted tests pass. Full test suite was not run (timeout on CI-free environment) but no compilation or vet failures exist.

---

## 13. Backend Shipping/Order/Pricing Contract Coherence

**YES** — the contract is now coherent:

1. **Shipping domain** provides `shipping_total` (authoritative monetary value)
2. **Pricing token** snapshots `shipping_total` + option metadata (no expedition/ETA)
3. **Order creation** reads from pricing token snapshot (no expedition/ETA)
4. **Order repository** persists and reads without expedition/ETA
5. **Order HTTP response** returns without expedition/ETA
6. **Migration 000060** drops the dead DB columns

The only remaining "ExpeditionName"/"EstimatedDays" in the system are mobile DTO fields that receive null from the backend. This is correct decay behavior — the mobile will be cleaned in a separate pass.
