-- Purge incomplete shipping option drafts that never received a coverage row.
-- This removes orphan option records so the seller shipping authority only
-- contains complete, buyer-visible shipping methods.

DELETE FROM shipping_options so
WHERE NOT EXISTS (
    SELECT 1
    FROM shipping_coverages sc
    WHERE sc.shipping_option_id = so.id
);
