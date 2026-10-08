-- 000129_purge_legacy_promotion_funding
--
-- Purge the dead promotion_funding table created by 000071. It references the
-- purged `promotions` aggregate, has ZERO application code references, and was
-- never superseded in the migration chain. The canonical promotion funding
-- authority is promotion_funding_intents + billing_transactions + payments.

DROP TABLE IF EXISTS promotion_funding CASCADE;
