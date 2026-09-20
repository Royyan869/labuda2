-- Down for 000089 — restore previous sort_order values
-- before: bank_transfer 10, qris 20, credit_card 30, dana 25, gopay 10, ovo 20, shopeepay 30, convenience_store 40

UPDATE payment_methods SET sort_order = 10 WHERE method_code = 'bank_transfer';
UPDATE payment_methods SET sort_order = 20 WHERE method_code = 'qris';
UPDATE payment_methods SET sort_order = 30 WHERE method_code = 'credit_card';
UPDATE payment_methods SET sort_order = 25 WHERE method_code = 'dana';
UPDATE payment_methods SET sort_order = 10 WHERE method_code = 'gopay';
UPDATE payment_methods SET sort_order = 20 WHERE method_code = 'ovo';
UPDATE payment_methods SET sort_order = 30 WHERE method_code = 'shopeepay';
UPDATE payment_methods SET sort_order = 40 WHERE method_code = 'convenience_store';
