-- Owner decision (Oct 2026): a SCHEDULED auction whose seller's market
-- authority (subscription) expires before activation does NOT become
-- 'cancelled'. The cancellation vocabulary stays reserved for
-- seller-initiated and moderation/admin outcomes. The auction becomes
-- 'lapsed' instead: it never went live, it is hidden from every viewer
-- surface by the read-side market-authority filters, and it becomes
-- relistable (lapsed -> scheduled) after renewal.
ALTER TYPE auction_status_enum ADD VALUE IF NOT EXISTS 'lapsed';
