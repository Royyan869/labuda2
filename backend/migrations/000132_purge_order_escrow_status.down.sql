ALTER TABLE order_summaries ADD COLUMN IF NOT EXISTS escrow_status text DEFAULT ''::text NOT NULL;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS escrow_status escrow_status_enum DEFAULT 'none'::escrow_status_enum NOT NULL;
