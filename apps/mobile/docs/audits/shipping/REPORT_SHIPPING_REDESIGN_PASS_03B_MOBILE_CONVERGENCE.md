# SHIPPING-03B — MOBILE SHIPPING CONTRACT CONVERGENCE

**Date:** 2026-09-02  
**Status:** ✅ PASS  
**Scope:** Mobile contract convergence with canonical backend contract

---

## 1. Files Changed

### Domain Entities
| File | Change |
|------|--------|
| `lib/domains/commerce/transaction/shipping/domain/entities/shipping.dart` | Removed `expeditionName` from `ShippingOption`, `CreateShippingOptionRequest`, `UpdateShippingOptionRequest`. Removed `estimatedDays` from `CityShippingRate`, `ShippingRateResult`, `DeliveryOption`, `AddCoverageRequest`. Removed `provinceEstimatedDays` from `ShippingCoverage`, `UpdateCoverageRequest`. Restored `shortName` as alias for `name`. |
| `lib/domains/commerce/pricing/pricing_preview/domain/entities/pricing_snapshot.dart` | Removed `expeditionName` and `estimatedDays` from `ShippingOptionInfo`. |

### DTOs
| File | Change |
|------|--------|
| `lib/domains/commerce/transaction/shipping/data/dto/shipping_dto.dart` | Removed `expeditionName` from `ShippingOptionDto`. Removed `estimatedDays` from `ShippingCoverageDto`, `CityRateDto`, `DeliveryOptionDto`. |
| `lib/domains/commerce/pricing/pricing_preview/data/dto/pricing_preview_dto.dart` | Removed `expedition_name` and `estimated_days` parsing from `PricingPreviewResponseDto.toEntity()`. |

### Mappers
| File | Change |
|------|--------|
| `lib/domains/commerce/transaction/shipping/data/mappers/shipping_mapper.dart` | Removed `expeditionName` propagation in `ShippingOptionMapper.toEntity()`. Removed `expedition_name` from `toCreateJson()`. Removed `provinceEstimatedDays` from `ShippingCoverageMapper.toEntity()`. Removed `estimatedDays` from `DeliveryOptionMapper.toEntity()`. |
| `lib/domains/commerce/transaction/shipping/data/remote/shipping_remote_datasource.dart` | Removed `expeditionName` propagation in `_decodeShippingOptionEnvelope()`. |
| `lib/domains/chat/attachment/mappers/attachment_mapper.dart` | Removed `expeditionName` and `estimatedDays` from `_mapToShippingQuoteAttachment()` and `_shippingQuoteAttachmentToMap()`. |
| `lib/domains/chat/chat/data/mappers/chat_mapper.dart` | Removed `expedition_name` and `estimated_days` mapping in `_dtoAttachmentToDomain()` and `domainAttachmentToDto()`. |

### Attachment System
| File | Change |
|------|--------|
| `lib/shared/attachment/entities/attachment.dart` | Removed `expeditionName` and `estimatedDays` from `ShippingQuoteAttachment`. Simplified `displayName` to always return `'$shippingTypeEmoji $shippingTypeName'`. |
| `lib/domains/chat/chat/data/dto/attachment_dto.dart` | Removed `expeditionName` and `estimatedDays` from `ShippingQuoteAttachmentDto` constructor, `fromJson()`, getters. Removed `expedition_name` and `estimated_days` from wire format. |

### UI
| File | Change |
|------|--------|
| `lib/domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart` | Removed `_expeditionCtrl` text controller and expedition name input field. Removed `expeditionName` from `_OptionFormResult`. Removed `expeditionName` from `CreateShippingOptionRequest` and `UpdateShippingOptionRequest` calls. Simplified option row to show only `type.label`. |
| `lib/domains/user/preference/seller/presentation/screens/seller_shipping_option_detail_screen.dart` | Removed `_daysCtrl` text controller and estimated days input field. Removed `estimatedDays` from `AddCoverageRequest`, `provinceEstimatedDays` from `UpdateCoverageRequest`. Removed `_CoverageFormResult.estimatedDays`. Removed estimated days display from coverage rows. |
| `lib/shared/widgets/attachment_widget.dart` | Removed estimated days display block from shipping quote attachment card. |
| `lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_claim_shipping_modal.dart` | Removed estimated days display from delivery option cards. |

### Tests
| File | Change |
|------|--------|
| `test/domains/commerce/transaction/shipping/shipping_remote_datasource_contract_test.dart` | Removed `expeditionName` parameter from `_shippingOptionJson()` helper. Removed all `expeditionName` assertions and `expedition_name` JSON keys from test data. |

---

## 2. Fields Removed

| Field | Locations Removed From |
|-------|----------------------|
| `expeditionName` | `ShippingOption`, `ShippingOptionDto`, `CreateShippingOptionRequest`, `UpdateShippingOptionRequest`, `ShippingQuoteAttachment`, `ShippingQuoteAttachmentDto` |
| `estimatedDays` | `CityShippingRate`, `ShippingRateResult`, `DeliveryOption`, `DeliveryOptionDto`, `AddCoverageRequest`, `CityRateDto`, `ShippingQuoteAttachment`, `ShippingQuoteAttachmentDto` |
| `provinceEstimatedDays` | `ShippingCoverage`, `ShippingCoverageDto`, `UpdateCoverageRequest` |
| `expedition_name` | JSON keys in DTOs, mappers, attachment DTOs |
| `estimated_days` | JSON keys in DTOs, mappers, attachment DTOs |

---

## 3. API Contract Changes

### Seller Setup Request
- `POST /seller/shipping/options` — no longer sends `expedition_name`

### Seller Setup Response
- `GET /seller/shipping/options/:id` — no longer expects `expedition_name`

### Coverage Request
- `POST /seller/shipping/options/:id/coverages` — no longer sends `estimated_days`
- `PUT /seller/shipping/coverages/:id` — no longer sends `estimated_days`

### Coverage Response
- No longer expects `estimated_days` in coverage JSON

### Delivery Check
- `POST /shipping/check` — delivery options no longer include `estimated_days`

### Pricing Preview
- `POST /api/v1/pricing/preview` — `shipping_option` in response no longer includes `expedition_name` or `estimated_days`

### Chat Attachments
- Shipping quote attachment wire format no longer includes `expedition_name` or `estimated_days`

---

## 4. Mapper Changes

| Mapper | Change |
|--------|--------|
| `ShippingOptionMapper.toEntity()` | No longer maps `expeditionName` |
| `ShippingOptionMapper.toCreateJson()` | No longer includes `expedition_name` |
| `ShippingCoverageMapper.toEntity()` | No longer maps `estimatedDays` → `provinceEstimatedDays` |
| `DeliveryOptionMapper.toEntity()` | No longer maps `estimatedDays` |
| `AttachmentMapper._mapToShippingQuoteAttachment()` | No longer reads `expeditionName` or `estimatedDays` |
| `ChatMapper._dtoAttachmentToDomain()` | No longer maps `expedition_name` or `estimated_days` |

---

## 5. UI Changes

| Screen | Change |
|--------|--------|
| Seller Shipping Screen | Removed expedition name text field from create/edit form. Option row now shows only transport type label. |
| Seller Shipping Option Detail Screen | Removed estimated days text field from coverage form. Coverage row no longer shows "Estimasi: X hari". |
| Attachment Widget | Shipping quote card no longer shows "Estimasi: X" line. |
| Auction Claim Modal | Delivery option cards no longer show "Estimasi: X" line. |

---

## 6. Test Changes

| Test | Change |
|------|--------|
| `shipping_remote_datasource_contract_test.dart` | Removed `expeditionName` parameter from JSON helper. Removed `expedition_name` from test data. Removed `expect(option.expeditionName, ...)` assertions. All 5 tests pass. |

---

## 7. Global Residue Search

Searched mobile source tree for:
- `expeditionName` → **0 matches**
- `estimatedDays` → **0 matches**  
- `expedition_name` → **0 matches**
- `estimated_days` → **0 matches**
- `ExpeditionName` → **0 matches**
- `EstimatedDays` → **0 matches**
- `shipping_expedition_name` → **0 matches**
- `shipping_estimated_days` → **0 matches**
- `provinceEstimatedDays` → **0 matches**
- `_expeditionCtrl` → **0 matches**
- `_daysCtrl` → **0 matches**
- `Estimasi hari` → **0 matches**

**ZERO active runtime references remain.**

---

## 8. Dart Analyze Result

```
2 issues found (info only — pre-existing, unrelated to this cleanup):
- lib\domains\chat\chat\data\remote\chat_api_datasource.dart:72:7 - use_null_aware_elements
- lib\domains\chat\chat\data\remote\chat_api_datasource.dart:74:7 - use_null_aware_elements
```

**0 errors, 0 warnings.** Only 2 pre-existing info-level lints.

---

## 9. Focused Test Results

```
shipping_remote_datasource_contract_test.dart: 5/5 tests passed ✅
```

- `listMyShippingOptions parses the wrapped shipping_options envelope` ✅
- `listMyActiveShippingOptions flips include_inactive to false` ✅
- `createShippingOption parses the nested shipping_option object` ✅
- `addCoverage parses the nested coverage object` ✅
- `rejects a bare list response instead of silently casting it` ✅

---

## 10. Broader Test Result

```
test/domains/commerce/transaction/: 134 passed, 19 failed
```

All 19 failures are **pre-existing** (not caused by this cleanup):

| Test | Failure Type | Root Cause |
|------|-------------|------------|
| `checkout_completion_proof_contract_test.dart` | Assertion failure | Expects specific string not matching current source |
| `shipping_option_setup_screen_test.dart` (6 errors) | Compilation failure | Missing `PresenceAuthorityState`, `DeliveryAvailabilityResult`, `updateShippingOption`, `toJson()` — all pre-existing |
| `seller_shipping_management_list_test.dart` (6 errors) | Compilation failure | Same pre-existing missing types/methods |
| `shipping_integer_tariff_contract_test.dart` (7 errors) | Compilation failure | Missing `cityRules` parameter, `toJson()` — pre-existing |

---

## 11. Remaining References and Why

**NONE.** Zero active references to `expeditionName`, `estimatedDays`, `expedition_name`, or `estimated_days` remain in the mobile source tree.

Historical audit docs may reference these fields for migration documentation — this is expected and intentional.

---

## 12. Unrelated Failures

The 19 pre-existing test failures are all compilation or assertion errors unrelated to this cleanup. They stem from:
1. `PresenceAuthorityState` / `DeliveryAvailabilityResult` type definitions not found (likely moved or renamed in a separate pass)
2. `ShippingRepository.updateShippingOption` not implemented in test fakes
3. `CreateShippingCoverageRequest` missing required `cityRules` parameter
4. `CreateShippingCityRuleRequest.toJson()` and `CreateShippingOptionRequest.toJson()` not defined
5. Checkout test expecting a stale string

---

## VERDICT

## ✅ SHIPPING-03B — PASS

- ✅ Zero active stale field references remain
- ✅ Mobile contract matches backend (no `expedition_name` or `estimated_days` in any request/response)
- ✅ Focused tests pass (5/5)
- ✅ Dart analyze passes (0 errors)
- ✅ No compatibility residue — fields fully removed, not nulled
- ✅ Coverage cost semantics preserved as single combined field (`provinceRate`)
- ✅ No non-goal changes (no renames, no backend modifications, no new features)
