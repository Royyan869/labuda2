-- ============================================================
-- 000087 WALLET HARD PURGE
--
-- Owner truth (locked):
--   - Ordinary users do NOT have monetary wallets.
--   - Labuda does NOT store user deposits/top-up funds.
--   - Seller monetary entitlement lives in the Finance ledger
--     (SELLER_PAYABLE), not in any wallet table.
--   - The Wallet architecture is rejected forbidden legacy.
--   - Escrow is canonical operational state and must survive
--     decoupled from Wallet.
--
-- This migration is forward-only. Labuda is a zero-to-one
-- application: no production wallets exist, no compatibility
-- shims are preserved.
--
-- Steps:
--   1. Drop escrows wallet-ID indexes.
--   2. Drop escrows wallet-ID FK constraints.
--   3. Drop escrows.buyer_wallet_id / escrows.seller_wallet_id.
--   4. Exhaustively drop every remaining wallets reference
--      (indexes, constraints).
--   5. DROP TABLE wallets.
-- ============================================================

-- ------------------------------------------------------------
-- 1-3. ESCROWS WALLET COUPLING REMOVAL
-- ------------------------------------------------------------

DROP INDEX IF EXISTS idx_escrows_buyer_wallet_id;
DROP INDEX IF EXISTS idx_escrows_seller_wallet_id;

ALTER TABLE escrows DROP CONSTRAINT IF EXISTS escrows_buyer_wallet_id_fkey;
ALTER TABLE escrows DROP CONSTRAINT IF EXISTS escrows_seller_wallet_id_fkey;

ALTER TABLE escrows DROP COLUMN IF EXISTS buyer_wallet_id;
ALTER TABLE escrows DROP COLUMN IF EXISTS seller_wallet_id;

-- ------------------------------------------------------------
-- 4-5. WALLETS TABLE REMOVAL
-- ------------------------------------------------------------

DROP INDEX IF EXISTS idx_wallets_user_id;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallets_user_id_fkey;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallets_user_id_key;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallets_pkey;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallets_available_balance_check;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallets_held_balance_check;

DROP TABLE IF EXISTS wallets;
