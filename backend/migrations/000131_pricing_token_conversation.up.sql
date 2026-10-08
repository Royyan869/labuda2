-- Canonical conversation binding for pricing tokens.
--
-- A manual shipping quote is created ONLY through a Chat and is scoped to that
-- exact conversation. The pricing token is the checkout-time authority, so it
-- must carry the originating conversation id (chat_id) alongside the quote id.
-- Order creation then consumes the quote through the ONE Commerce authority and
-- enforces that the quote is used in the exact conversation that produced it.
--
-- shipping_quote_id already exists on pricing_tokens (FK -> shipping_quotes).
ALTER TABLE pricing_tokens ADD COLUMN IF NOT EXISTS chat_id uuid;
