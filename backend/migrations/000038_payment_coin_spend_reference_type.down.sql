ALTER TABLE coins_transactions
	DROP CONSTRAINT IF EXISTS coins_transactions_reference_type_check;

ALTER TABLE coins_transactions
	ADD CONSTRAINT coins_transactions_reference_type_check
	CHECK (
		reference_type = ANY (
			ARRAY[
				'order_reward'::text,
				'order_spend'::text,
				'refund_earn'::text,
				'refund_spend'::text
			]
		)
	);
