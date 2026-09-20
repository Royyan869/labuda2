-- Down migration: restore account_type_enum for chain integrity only.
-- WARNING: This type is OBSOLETE. Canonical authority is
-- financial_accounts.account_type VARCHAR + Go constants + AccountClassOf().
-- Restored only so `migrate down` reproduces the historical 000001 snapshot.

CREATE TYPE account_type_enum AS ENUM (
    'ESCROW',
    'SELLER_PAYABLE',
    'BUYER_REFUNDABLE',
    'PLATFORM_REVENUE',
    'WITHDRAWAL_PENDING',
    'PLATFORM_BANK',
    'VAT_LIABILITY',
    'GATEWAY_CLEARING',
    'BANK_SETTLEMENT'
);
