-- PHASE 1 CLEANUP: drop tables with zero application code references.
--
-- Verified via repo-wide grep (backend/) before this migration was written:
-- each of these six tables is referenced ONLY by the truncation list in
-- cmd/dev-reset-data/main.go (a dev reset tool, not application logic).
-- No repository, handler, worker, or raw-SQL query in backend/internal
-- touches them. No other table has a foreign key pointing at any of them
-- (verified: zero "REFERENCES <table>(" hits across backend/migrations).
--
-- ticket_escalations also had an unused Go entity struct
-- (internal/governance/support/entity/ticket_escalation.go) with zero
-- external references; the struct is left in place by this migration
-- since Go source pruning is out of scope for a SQL migration — track
-- separately if the struct should also be removed.
DROP TABLE IF EXISTS actors;
DROP TABLE IF EXISTS bnr_classifications;
DROP TABLE IF EXISTS financial_reconciliations;
DROP TABLE IF EXISTS search_results;
DROP TABLE IF EXISTS ticket_escalations;
DROP TABLE IF EXISTS user_online_status;
