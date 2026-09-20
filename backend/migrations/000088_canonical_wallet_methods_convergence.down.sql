-- Down for 000088 ADDITIVE wallets: remove only wallets added by this migration.
-- Existing methods (bank_transfer, qris, credit_card, dana, convenience_store)
-- are NOT touched — this down only reverses the ADD.
-- No payment rows are modified.
DELETE FROM payment_methods WHERE method_code IN ('gopay', 'ovo', 'shopeepay');
