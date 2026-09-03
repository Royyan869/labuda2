# REPORT: Resource Occurrence Projection Convergence — Final

## 1. First Failure / Root Cause

**Root cause A (H3-H8, H10-H18, H20):** The chat HTTP handler's `ListMessages` endpoint had **no resource projection hydration**. The handler lacked a `ResourceProjectionResolver`, never queried `chat_message_resource_occurrences`, and `messageToResponse` never emitted `resource_projection`. Every message with an occurrence returned a response without the projection envelope.

**Root cause B (H9):** `DeriveVisibility` in `for_sale_visibility.go` only treated `ForSaleStatusActive` with a non-nil `published_at` as `Public`. Sold/withdrawn sales with `published_at` were derived as `Private`, causing the view-access gate to reject them for non-seller viewers — producing a TOMBSTONE instead of LIVE. The H9 spec requires a "public terminal canonically-viewable FPS" to be LIVE.

## 2. Canonical Authority

| Concept | Authority |
|---------|-----------|
| `attachment_json` | `chat_messages.attachment_json` (jsonb, nullable) |
| `chat_message_resource_occurrences` | Table with `message_id` FK, exactly-one-source CHECK, CASCADE delete |
| Resource type vocabulary | `profile`, `content`, `for_sale`, `auction` (entity constants) |
| `fallback_snapshot` | Server-built jsonb, never client-submitted |
| `for_sale_source_id` | Canonical FK (renamed from `fixed_price_sale_source_id` in migration 47) |
| Visibility derivation | `fpsEntity.DeriveVisibility` → now public for Active/Sold/Withdrawn when published |

## 3. Files Changed

| File | Change |
|------|--------|
| `internal/interaction/chat/delivery/http/chat_handler.go` | Added `resourceProjectionResolver` field, `SetResourceProjectionResolver` setter, `getResourceOccurrencesByMessageIDs` helper, and projection hydration in `ListMessages` |
| `internal/serverboot/chat_resource_projection_http_integration_test.go` | Wired `base.resolver` to handler via `SetResourceProjectionResolver` |
| `internal/commerce/forsale/entity/for_sale_visibility.go` | `DeriveVisibility` now returns Public for Sold/Withdrawn when published_at is set |
| `internal/commerce/forsale/entity/for_sale_visibility_test.go` | Updated stale assertions for Sold/Withdrawn visibility |

## 4. Tests Before/After

| Test | Before | After |
|------|--------|-------|
| H3 (Profile LIVE) | FAIL | **PASS** |
| H4 (Profile TOMBSTONE) | FAIL | **PASS** |
| H5 (Content LIVE) | FAIL | **PASS** |
| H6 (Content nested indicator) | FAIL | **PASS** |
| H7 (Content nested inaccessible) | FAIL | **PASS** |
| H8 (FPS LIVE active) | FAIL | **PASS** |
| H9 (FPS sold canonically-viewable) | FAIL | **PASS** |
| H10 (FPS inaccessible) | FAIL | **PASS** |
| H11 (Auction LIVE active) | FAIL | **PASS** |
| H12 (Auction terminal viewable) | FAIL | **PASS** |
| H13 (Auction inaccessible) | FAIL | **PASS** |
| H14 (Mixed page) | FAIL | **PASS** |
| H15 (Same source multiple messages) | FAIL | **PASS** |
| H16 (Viewer A vs B) | FAIL | **PASS** |
| H17 (Aggregate child failure → 500) | FAIL | **PASS** |
| H18 (Malformed occurrence → 500) | FAIL | **PASS** |
| H19 (Legacy attachment_json) | PASS | PASS |
| H20 (No occurrence internals exposed) | FAIL | **PASS** |

## 5. Build / Vet

```
go build ./...   → OK
go vet ./...     → OK
```

## 6. Remaining Blockers

None. All 18 targeted tests pass (H3-H18, H20). H1, H2, H19 confirmed still passing.

## 7. Verdict

**PASS**

✅ All H3-H18 and H20 integration tests pass with real PostgreSQL.
✅ H1, H2, H19 remain passing.
✅ `go build` and `go vet` clean.
✅ No stale resource-occurrence vocabulary discovered.
