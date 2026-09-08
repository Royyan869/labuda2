-- 000065_promotion_contract_canonical (down)
DROP INDEX IF EXISTS idx_promotion_contracts_status;
DROP INDEX IF EXISTS idx_promotion_contracts_seller_id;
DROP INDEX IF EXISTS ux_promotion_contracts_one_nonfinalized_internal_per_seller;
DROP INDEX IF EXISTS ux_promotion_contracts_one_nonfinalized_external_per_seller;

DROP TABLE IF EXISTS promotion_contracts;

DROP TYPE IF EXISTS promotion_contract_status_enum;
DROP TYPE IF EXISTS promotion_contract_kind_enum;

-- PostgreSQL cannot remove an enum value. 'promote_balance_top_up' remains a
-- member of billing_type_enum after this down migration; it is unused once
-- the promote top-up billing path is no longer in place.
-- ALTER TYPE billing_type_enum DROP VALUE ... (unsupported)
