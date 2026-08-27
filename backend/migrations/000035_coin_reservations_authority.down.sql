-- ============================================================================
-- 000035: COIN RESERVATION AUTHORITY — ROLLBACK
-- ============================================================================
DROP TABLE IF EXISTS coin_reservations CASCADE;
DROP TYPE IF EXISTS coin_reservation_status_enum CASCADE;
