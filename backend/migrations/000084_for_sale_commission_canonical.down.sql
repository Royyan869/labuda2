-- 000084_for_sale_commission_canonical (down)
-- Rollback to the pre-convergence state: restore the obsolete key row and
-- remove the canonical key. Only used for explicit migration rollback.

DELETE FROM platform_configs WHERE key = 'for_sale_commission_percent';

INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
VALUES ('listing_commission_percent', 4, NULL, NULL, (EXTRACT(epoch FROM now()))::bigint)
ON CONFLICT (key) DO NOTHING;