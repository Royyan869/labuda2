# COMMISSION VERIFIER CONVERGENCE — IMPLEMENTATION REPORT

Read the audits first: `COMMISSION_IDENTITY_AUDIT.md`, `COMMISSION_AUTHORITY_CONVERGENCE_AUDIT.md`.
This report documents the executor pass that converged the verifier onto the canonical Commission Identity.

---

## VERDICT

**COMMISSION_AUTHORITY_CONVERGED** (for the verifier commission logic).

The single conflicting commission formula in the verifier (`verifierProportionalCommission` with `orderGross := EscrowAmount` denominator) has been **removed and replaced** with the canonical PD-based allocation. The verifier now loads `discount_amount` from the `orders` table and computes the commission expectation using `PD = Subtotal − Discount`, matching `refund_math.go` / `refund_gateway.go` exactly (floor division, product-only denominator).

There is now **exactly ONE commission identity authority** (`order.CommissionAmount`) and **no surviving verifier formula that treats `EscrowAmount`/`orderGross` as the commission denominator**.

---

## CANONICAL COMMISSION IDENTITY

- `order.CommissionAmount` — immutable order snapshot, product-only.
- `CommissionAmount = floor(PD × rate / 100)` where `PD = Subtotal − Discount`.
- Shipping has NO commission.
- `CommissionDelta` is a derived per-refund-event allocation, not a second identity.
- Canonical refund allocation: `refund_math.go` (`proportionalFloor`, denominator PD).

---

## EXACT CONTRADICTORY SYMBOLS REMOVED/CONVERGED

| Symbol | Action |
|---|---|
| `verifierProportionalCommission(amount, orderCommission, orderGross)` | **REMOVED** (deleted; no compatibility alias created) |
| `expectedCommissionBefore := verifierProportionalCommission(previouslyRefunded, order.CommissionAmount, orderGross)` | **CONVERGED** → `verifierProportionalCommissionPD(previouslyRefunded, order.CommissionAmount, pd)` |
| `expectedCommissionAfter := verifierProportionalCommission(previouslyRefunded+*r.FinalRefundAmount, order.CommissionAmount, orderGross)` | **CONVERGED** → PD-based |
| `orderGross := order.EscrowAmount` as commission denominator | **REMOVED from commission path** (the variable remains only for the legitimate escrow-cap checks `refund_requested_exceeds_order` / `refund_final_exceeds_order` / `cumulative_refund_exceeds_order`, which are escrow semantics, NOT commission) |

## ADDED

| Symbol | Purpose |
|---|---|
| `Order.DiscountAmount int64` (verifier snapshot struct) | Canonical PD input |
| `verifierProportionalCommissionPD(amount, orderCommission, pd)` | Canonical floor-division allocation mirroring `refund_math.proportionalFloor` |
| `order_invalid_discount` finding | Guard: discount out of `[0, Subtotal]` |
| `order_invalid_pd` finding | Guard: `PD <= 0` |

---

## EXACT FILE PATHS

| File | Change |
|---|---|
| `backend/internal/finance/verifier/verifier.go` | Only file changed |

---

## EXACT TEST COMMANDS + EXIT RESULTS

| Command | Result |
|---|---|
| `go test ./internal/finance/verifier/... -count=1 -timeout 120s` | **PASS** (ok) |
| `go vet ./internal/finance/verifier/...` | **PASS** (no output) |
| `go test ./internal/finance/verifier/... ./internal/finance/delivery/... ./internal/finance/worker/... ./internal/finance/infrastructure/... ./internal/finance/ -count=1 -timeout 240s` | **PASS** (all ok) |
| `go build ./cmd/staging_rollout_ab/... ./internal/finance/delivery/...` | **PASS** (verifier consumers compile) |
| `go test ./internal/finance/refund/...` | **FAIL** — pre-existing, unrelated (see REGRESSION_RESULTS) |
| `go test ./internal/finance/application/...` | **FAIL** — pre-existing, unrelated (see REGRESSION_RESULTS) |
| `go test ./internal/commerce/order/...` | **FAIL** — pre-existing, unrelated (see REGRESSION_RESULTS) |

---

## FILES CHANGED

- `backend/internal/finance/verifier/verifier.go` (1 file, +28/−9)

## PRODUCTION_FILES_CHANGED

- `backend/internal/finance/verifier/verifier.go` — production logic (verifier), converged to canonical commission formula.

## TEST_FILES_CHANGED

- **NONE.** No test files were modified. (The verifier's in-file fixtures `fixtureMissingRelease` / `fixtureDuplicateRefundReversal` in `verifier.go` received explicit `DiscountAmount: 0` to stay honest with the new field — this is production fixture code within the verifier package, not a `_test.go` file.)

## DATABASE_CHANGED

- **NONE.** The `orders.discount_amount` column already exists (canonical schema `000001_canonical_schema.up.sql:1104`, `DEFAULT 0 NOT NULL`). No migration, no schema change, no DB write.

---

## REGRESSION_RESULTS

All packages that were green before this change remain green. The only failures observed are **pre-existing and unrelated** to the verifier convergence:

1. **`internal/finance/application`** — build failure in `withdraw_request_idempotency_test.go` (untracked WIP file referencing `IdempotencyKey` field / `ErrWithdrawalIdempotencyConflict` that do not exist in the current production types). Production code builds clean (`go build ./internal/finance/...` passes).
2. **`internal/finance/refund/entity`** — `refund_policy_test.go` references `OrderSnapshot.Gross` / `RefundPolicyResult.Amount` which no longer exist in `refund_policy.go` (fields were renamed to `ProductGross`/`CashRefund` in the S2C2 rebase; the tracked test file is stale).
3. **`internal/finance/refund/application`** — `refund_seller_approval_dispatch_test.go` (untracked, parallel-work test) panics in `InitiateGatewayRefund` (`refund_gateway.go:214`) with a nil deref — a test-fixture wiring issue in new parallel work, not related to commission.
4. **`internal/finance/refund/infrastructure/repository`** — `refund_history_contract_test.go` expects `created_at < $2` in `refund_repository_impl.go` which the current HEAD does not contain (contract not yet merged).
5. **`internal/commerce/order/...`** — `TestOrderRefundHistoryHttpContract` (missing `ListRefundHistory`) and `rating/delivery/http` build (undefined `toRatingResponse` etc.) — pre-existing contract/build gaps.

None of these touch commission identity, the verifier, `refund_math.go`, the ledger, or escrow. All were failing identically before this change (verified: only `verifier.go` differs from HEAD; the failing files are unmodified tracked files or untracked parallel-work files).

---

## CLEANUP STATUS

| Item | Status |
|---|---|
| Conflicting `verifierProportionalCommission` | **DELETED** |
| Verifier commission expectation | **CONVERGED** to PD denominator |
| `orderGross := EscrowAmount` commission binding | **REMOVED** from commission path (retained only for escrow-cap checks) |
| Verifier `Order` snapshot | **Now loads `discount_amount`** (canonical PD input) |
| Verifier fixtures | Updated with explicit `DiscountAmount: 0` |
| Compatibility aliases/shims | **NONE created** (deleted, not deprecated) |
| `LegacyGross()` | **NOT touched** — audit found it test-only, but removing it is outside this task's verifier scope and it is not a verifier formula; left in place per "do not touch unrelated accounting" |
| `refund_math.go` | **UNCHANGED** (canonical formula preserved) |
| `order.CommissionAmount` | **UNCHANGED** |
| `RecordPartialRefundRelease` / ledger / escrow / settlement | **UNCHANGED** |

---

## REMAINING BLOCKERS

1. **Pre-existing unrelated failures** (listed above) are owned by the parallel Order/Payment/Coins/Discount work and by stale tracked tests. They are NOT blockers for this convergence and NOT caused by it.
2. **`LegacyGross()` in `refund_policy.go:36`** remains a duplicate derived helper (test-only). The authority audit classifies it LEGACY/ZOMBIE; removing it is a separate, low-risk cleanup for a future pass (it is not a verifier formula and not a commission identity).
3. **Verifier `orderGross` for dispute-freeze checks** (`verifier.go:739`, `Subtotal + ShippingTotal`) is a *different* denominator used for freeze-vs-gross validation — it is not a commission formula and was intentionally left untouched (escrow/dispute semantics, out of commission scope).

---

## STOP RULE CHECK

No new business/accounting contradiction was revealed during implementation. The only discovered divergence (verifier denominator) was the one already identified and authorized for convergence. No opportunistic changes were made. No ledger/escrow/settlement/refund-policy behavior was modified.

---

*This pass converges the verifier onto the canonical Commission Identity. The rejected verifier commission model is deleted, not deprecated. Future agents cannot accidentally resurrect `verifierProportionalCommission` / the `EscrowAmount`-denominator commission check.*
