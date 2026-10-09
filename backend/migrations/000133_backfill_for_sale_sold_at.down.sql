-- 000133 backfill is a one-way history repair: the pre-backfill NULL state of
-- sold_at on already-sold rows cannot be reconstructed, so there is nothing
-- to revert. Future transitions stamp sold_at via ForSale.ReduceQuantity and
-- are unaffected by this rollback.
SELECT 1;
