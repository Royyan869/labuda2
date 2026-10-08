-- Down migration: remove the order payment-method binding column.
ALTER TABLE orders DROP COLUMN IF EXISTS payment_method_code;
