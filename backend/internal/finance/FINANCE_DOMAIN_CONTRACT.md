# FINANCE DOMAIN CONTRACT

---
**DOMAIN CLASSIFICATION: SUPPORT DOMAIN**
**Role:** Accounting mirror, billing ledger, and reporting — NOT financial authority
**Authority:** DERIVED — All money state comes from Wallet Domain
---

## Status: SUPPORT DOMAIN — NOT FINANCIAL AUTHORITY

This document establishes the **Finance Domain** as a **derived accounting system** that mirrors the Wallet Domain for billing, reporting, and payout purposes.

---

## 1. FINANCE DOMAIN RESPONSIBILITIES

| Operation | Finance Domain Role | Source of Truth |
|-----------|-------------------|-----------------|
| Maintain financial_accounts | ✅ OWNERSHIP | Derived from wallets table |
| Billing ledger | ✅ OWNERSHIP | Derived from wallet transactions |
| Payout processing | ✅ OWNERSHIP | Based on financial_accounts |
| Reconciliation | ✅ OWNERSHIP | Verifies finance = wallet |
| Mutate wallet balances | ❌ FORBIDDEN | Wallet Domain ONLY |

---

## 2. ❌ FORBIDDEN OPERATIONS

**This domain MUST NOT:**

- Directly modify `wallets.available_balance`
- Directly modify `wallets.held_balance`
- Create wallet `LedgerEntry` (only finance ledger entries)
- Initiate money transfers independently

**All money state MUST come from:**
```go
// Wallet domain is the source of truth
// Finance domain mirrors wallet state
wallet_total = available_balance + held_balance
finance_total = SUM(financial_accounts WHERE user_id = X)
// These MUST always match
```

---

## 3. ✅ ALLOWED OPERATIONS

**This domain MAY:**

- Create and update `financial_accounts` records
- Create ledger entries (see §3a for write authority)
- Process payouts to external gateways
- Run reconciliation jobs
- Generate financial reports

---

## 3a. LEDGER WRITE AUTHORITY (`ledgerRepo.CreateTransaction`)

`ledgerRepo.CreateTransaction()` is **finance-internal only**. Foreign domains (order, wallet, payment, etc.) MUST NOT call it.

Within the finance domain, **authorized direct writers** are:

| Writer | Justification |
|--------|--------------|
| `FinanceService` | Canonical gateway — all callers should prefer this |
| `WithdrawService.MarkWithdrawalProcessed` | Intra-domain; transactional with withdrawal row update; idempotency key `withdraw_<id>` |
| `finance/worker WebhookHandler.handleFailedCallback` | Gateway failure reversal; idempotency key `withdrawal_gateway_restore_<id>` |
| `finance/worker PayoutWorker.markSubmissionFailed` | Same reversal path as above; shares idempotency key as race guard |

**Adding a new direct writer requires:**
1. Explicit justification for why `FinanceService` cannot be used
2. A unique idempotency key preventing double-write
3. Documentation in this table

---

## 4. DEPENDENCY RULES

### ✅ PERMITTED DIRECTION

```
finance ───→ wallet (for reconciliation queries only)
```

Finance domain may:
- Read from wallets table for reconciliation
- Read ledger entries for billing purposes

### ❌ FORBIDDEN DIRECTION

```
finance ──→ wallet (for mutations)
```

Finance must NOT:
- Write to wallets table
- Create wallet ledger entries

---

## 5. CODE ENFORCEMENT PATTERN

All files in `finance/` MUST include:

```go
// ⚠️ FINANCE DOMAIN RULE:
// This domain is NOT financial authority.
// It is ONLY for billing ledger and reporting.
// All wallet balance mutations are FORBIDDEN.
```

---

## 6. ARCHITECTURAL INVARIANTS

1. **Derived State**: Finance is a mirror of Wallet, never the source
2. **Reconciliation**: Periodic jobs verify finance = wallet
3. **Payout Authority**: Finance can initiate payouts, but only from derived balances
4. **Audit Trail**: All finance operations are traceable to wallet transactions

---

## 7. RECONCILIATION

`ReconciliationWorkerV2` runs every 5 minutes (canonical reconciliation):

1. Double-entry invariant: SUM(debits) == SUM(credits) per transaction
2. Account balance drift: ledger net movement vs financial_accounts stored balance
3. Critical account non-negativity: ESCROW, WITHDRAWAL_PENDING, etc.
4. Withdrawal consistency: SUM(active withdrawals) vs WITHDRAWAL_PENDING balance

Constitutional: verification + escalation only — NO auto-repair (ADR-002 §7.1).
All corrections require attributable operator via FinanceService.

---

## 8. VIOLATION PROTOCOL

If a violation is detected:

1. **Immediate**: Code review rejection
2. **Remediation**: Remove direct wallet mutations
3. **Verification**: Add reconciliation test

---

## Version History

| Version | Date | Change |
|---------|------|--------|
| 1.0 | 2026-04-13 | Initial contract - establish Finance as SUPPORT domain |


