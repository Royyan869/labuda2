-- PASS_18C: track cumulative soft-close (anti-sniping) extension per auction
-- so PlaceBid can enforce the owner-approved 30-minute total extension cap.

ALTER TABLE auctions
	ADD COLUMN anti_snipe_extension_seconds bigint NOT NULL DEFAULT 0;
