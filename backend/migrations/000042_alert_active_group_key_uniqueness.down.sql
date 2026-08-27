-- Migration 000042 down: drop the active-group-key uniqueness index.

DROP INDEX IF EXISTS idx_system_alerts_active_group_key;
