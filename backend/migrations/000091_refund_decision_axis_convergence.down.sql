-- ============================================================
-- 000091 DOWN — restore the previous refund_status_enum value name.
-- ============================================================

ALTER TYPE refund_status_enum RENAME VALUE 'system_refunded' TO 'refunded';
