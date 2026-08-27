# DISPUTE DOMAIN CONTRACT

---
**DOMAIN CLASSIFICATION: CONTROL DOMAIN**
**Role:** Governs order state during disputes; delegates money to Wallet
**Authority:** DELEGATED — All money operations MUST go through WalletService
---

## Status: CONTROL DOMAIN — NOT FINANCIAL AUTHORITY

This document establishes the **Dispute Domain** as a **control authority** that manages dispute state while delegating all money operations to the Wallet Domain.

---

## 1. DISPUTE DOMAIN RESPONSIBILITIES

| Operation | Dispute Domain Role | Delegate To |
|-----------|-------------------|-------------|
| Manage dispute lifecycle | ✅ OWNERSHIP | None (entity managed here) |
| Freeze escrow during dispute | ✅ CONTROL | Via OrderService (state change) |
| Admin resolution decisions | ✅ OWNERSHIP | None (decision recorded here) |
| Execute money refund | ❌ FORBIDDEN | **WalletService** |
| Release escrow to seller | ❌ FORBIDDEN | **WalletService** |
| Mutate wallet balances | ❌ FORBIDDEN | **WalletService** |

---

## 2. ❌ FORBIDDEN OPERATIONS

**This domain MUST NOT:**

- Directly modify `wallets.available_balance`
- Directly modify `wallets.held_balance`
- Create `LedgerEntry` directly
- Call escrow release/refund directly

**All money operations MUST go through:**
```go
// Via OrderService (which delegates to WalletService)
orderService.MarkDisputeOpen(ctx, tx, orderID)      // Control state only
orderService.RefundFromDispute(ctx, tx, orderID)   // Money via Wallet
orderService.ReleaseFromDispute(ctx, tx, orderID)  // Money via Wallet
```

---

## 3. ✅ ALLOWED OPERATIONS

**This domain MAY:**

- Create and update `Dispute` entity state
- Create and update `DisputeMedia` records
- Emit outbox events for dispute workflow
- Validate dispute eligibility (business rules)
- Track admin resolution decisions
- Trigger escrow freeze (state control, not money mutation)

---

## 4. DEPENDENCY RULES

### ✅ PERMITTED DIRECTION

```
dispute ───→ order ───→ wallet (CORE)
```

Dispute domain may call:
- `OrderService.MarkDisputeOpen()` — state control
- `OrderService.RefundFromDispute()` — money via Wallet
- `OrderService.ReleaseFromDispute()` — money via Wallet

### ❌ FORBIDDEN DIRECTION

```
wallet ──→ dispute
```

Wallet must NOT depend on Dispute domain.

---

## 5. CODE ENFORCEMENT PATTERN

All files in `dispute/` MUST include:

```go
// ⚠️ FINANCIAL RULE:
// All money operations MUST go through WalletService.
// Direct balance mutation is forbidden.
```

This is already present in `dispute_service.go` lines 1-3.

---

## 6. ARCHITECTURAL INVARIANTS

1. **State Control Only**: Dispute manages order/escrow state, not money
2. **Financial Delegation**: All money operations delegated to Wallet
3. **Audit Trail**: Every dispute resolution is attributable to admin
4. **Deadlock Prevention**: Force-resolve capability for stuck disputes

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
| 1.0 | 2026-04-13 | Initial contract - establish Dispute as CONTROL domain |


