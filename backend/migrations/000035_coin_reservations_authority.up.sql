-- ============================================================================
-- 000035: COIN RESERVATION AUTHORITY
-- ============================================================================
-- Canonical reservation table for explicit coin redemption at payment intent.
--
-- MODEL R: RESERVE → CONSUME / RELEASE
--
-- Lifecycle:
--   reserved  — coins are held for an active payment intent; unavailable for
--               other payments but NOT yet spent. Total balance unchanged.
--   consumed  — payment settled; reservation consumed. Balance deducted and
--               spend transaction created exactly once.
--   released  — payment failed/expired; reservation removed. Total balance
--               unchanged. No earn/refund transaction created.
--
-- INVARIANTS:
--   TotalUnspentCoins  = user_coin_balance.balance
--   ReservedCoins      = SUM(amount) WHERE status = 'reserved'
--   AvailableCoins     = TotalUnspentCoins - ReservedCoins
--
-- Reserve:  does NOT decrement balance, does NOT create spend transaction.
-- Consume:  decrements balance exactly once, creates exactly one spend tx.
-- Release:  does NOT credit balance, no earn/refund tx.
--
-- One payment. One reservation. One immutable K. No double redemption.
-- ============================================================================

-- ============================================================================
-- COIN RESERVATION STATUS ENUM
-- ============================================================================
CREATE TYPE coin_reservation_status_enum AS ENUM (
    'reserved',
    'consumed',
    'released'
);

-- ============================================================================
-- COIN RESERVATIONS TABLE
-- ============================================================================
CREATE TABLE coin_reservations (
    id              uuid DEFAULT gen_random_uuid() NOT NULL,
    payment_id      uuid NOT NULL,
    user_id         uuid NOT NULL,
    amount          bigint NOT NULL,
    status          coin_reservation_status_enum DEFAULT 'reserved' NOT NULL,
    expires_at      timestamp with time zone NOT NULL,
    consumed_at     timestamp with time zone,
    released_at     timestamp with time zone,
    created_at      timestamp with time zone DEFAULT now() NOT NULL,
    updated_at      timestamp with time zone DEFAULT now() NOT NULL
);

-- ============================================================================
-- PRIMARY KEY
-- ============================================================================
ALTER TABLE coin_reservations
    ADD CONSTRAINT coin_reservations_pkey PRIMARY KEY (id);

-- ============================================================================
-- UNIQUE CONSTRAINTS
-- ============================================================================

-- One reservation per payment for its ENTIRE lifetime.
-- Terminal (consumed/released) reservations block a second row.
ALTER TABLE coin_reservations
    ADD CONSTRAINT coin_reservations_payment_id_key UNIQUE (payment_id);

-- ============================================================================
-- FOREIGN KEYS
-- ============================================================================
ALTER TABLE coin_reservations
    ADD CONSTRAINT coin_reservations_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE coin_reservations
    ADD CONSTRAINT coin_reservations_payment_id_fkey
        FOREIGN KEY (payment_id) REFERENCES payments(id) ON DELETE CASCADE;

-- ============================================================================
-- CHECK CONSTRAINTS
-- ============================================================================
ALTER TABLE coin_reservations
    ADD CONSTRAINT chk_coin_reservations_amount_positive CHECK (amount > 0);

-- ============================================================================
-- INDEXES
-- ============================================================================

-- Fast lookup of active reservations per user (for available balance calc).
CREATE INDEX idx_coin_reservations_user_active
    ON coin_reservations (user_id, created_at DESC)
    WHERE (status = 'reserved');

-- Fast lookup by payment (already covered by UNIQUE, but explicit for clarity).
CREATE INDEX idx_coin_reservations_payment_id
    ON coin_reservations (payment_id);

-- Expiry reconciliation: find stale reservations.
CREATE INDEX idx_coin_reservations_expires_at
    ON coin_reservations (expires_at)
    WHERE (status = 'reserved');
