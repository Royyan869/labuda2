# REFUND DOMAIN CONTRACT

---
**DOMAIN CLASSIFICATION: WORKFLOW DOMAIN**
**Role:** Manages buyer-seller refund negotiation workflow
**Authority:** DELEGATED — All money operations MUST go through WalletService
---

## Status: WORKFLOW DOMAIN — NOT FINANCIAL AUTHORITY

This document establishes the **Refund Domain** as a **workflow orchestrator** that delegates all money operations to the Wallet Domain.

---

## 1. REFUND DOMAIN RESPONSIBILITIES

| Operation | Refund Domain Role | Delegate To |
|-----------|-------------------|-------------|
| Manage refund request state | ✅ OWNERSHIP | None (entity managed here) |
| Handle seller approval/rejection | ✅ OWNERSHIP | None (workflow managed here) |
| Track refund evidence | ✅ OWNERSHIP | None (metadata managed here) |
| Execute money refund | ❌ FORBIDDEN | **WalletService** |
| Mutate wallet balances | ❌ FORBIDDEN | **WalletService** |

---

## 2. ❌ FORBIDDEN OPERATIONS

**This domain MUST NOT:**

- Directly modify `wallets.available_balance`
- Directly modify `wallets.held_balance`
- Create `LedgerEntry` directly
- Call escrow operations directly

**All money operations MUST go through:**
```go
// Via OrderService (which delegates to WalletService)
orderService.PartialRefund(ctx, tx, orderID, amount)
orderService.RefundFromDispute(ctx, tx, orderID)
orderService.ReleaseFromDispute(ctx, tx, orderID)
```

---

## 3. ✅ ALLOWED OPERATIONS

**This domain MAY:**

- Create and update `Refund` entity state
- Create and update `RefundEvidence` records
- Emit outbox events for refund workflow
- Validate refund eligibility (business rules)
- Track seller approval/rejection decisions

---

## 4. DEPENDENCY RULES

### ✅ PERMITTED DIRECTION

```
refund ───→ order ───→ wallet (CORE)
```

Refund domain may call:
- `OrderService.PartialRefund()`
- `OrderService.RefundFromDispute()`
- `OrderService.ReleaseFromDispute()`

### ❌ FORBIDDEN DIRECTION

```
wallet ──→ refund
```

Wallet must NOT depend on Refund domain.

---

## 5. CODE ENFORCEMENT PATTERN

All files in `refund/` MUST include:

```go
// ⚠️ FINANCIAL RULE:
// This domain MUST NOT mutate money directly.
// All refund operations MUST go through OrderService/WalletService.
// Direct balance mutation is forbidden.
```

---

## 6. ARCHITECTURAL INVARIANTS

1. **Single Responsibility**: Refund domain ONLY manages workflow state
2. **Financial Delegation**: All money operations delegated to Wallet
3. **Audit Trail**: Every refund decision creates immutable records
4. **Idempotency**: Refund operations are safe to retry

---

## 7. VIOLATION PROTOCOL

If a violation is detected:

1. **Immediate**: Code review rejection
2. **Remediation**: Route money operations through WalletService
3. **Verification**: Add test proving proper delegation

---

## Version History

| Version | Date | Change |
|---------|------|--------|
| 1.0 | 2026-04-13 | Initial contract - establish Refund as WORKFLOW domain |


