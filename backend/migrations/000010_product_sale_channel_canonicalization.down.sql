-- Rollback for 000010: recreate the dropped legacy schema exactly as it
-- existed in 000001, and remove the cross-table exclusivity trigger.
-- All dropped data was empty (0 rows) at migration time, so this rollback
-- restores structure only, not data.

DROP TRIGGER IF EXISTS trg_auctions_single_active_channel ON auctions;
DROP TRIGGER IF EXISTS trg_fixed_price_sales_single_active_channel ON fixed_price_sales;
DROP FUNCTION IF EXISTS enforce_single_active_sale_channel_per_product();

CREATE TABLE IF NOT EXISTS listings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    seller_id uuid NOT NULL,
    title text NOT NULL,
    description text NOT NULL,
    media_urls jsonb DEFAULT '[]'::jsonb NOT NULL,
    variety text NOT NULL,
    size_cm integer,
    age_months integer,
    gender text,
    breeder text,
    bloodline text,
    certificates text[] DEFAULT ARRAY[]::text[] NOT NULL,
    listing_type listing_type_enum NOT NULL,
    price_per_unit bigint NOT NULL,
    quantity_available integer NOT NULL,
    negotiation_enabled boolean DEFAULT false NOT NULL,
    visibility listing_visibility_enum DEFAULT 'private'::listing_visibility_enum NOT NULL,
    status listing_status_enum DEFAULT 'draft'::listing_status_enum NOT NULL,
    origin listing_origin_enum DEFAULT 'manual'::listing_origin_enum NOT NULL,
    farm_address_id uuid,
    preparation_time preparation_time_enum NOT NULL,
    preparation_note text,
    view_count bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    search_vector tsvector GENERATED ALWAYS AS ((((setweight(to_tsvector('simple'::regconfig, COALESCE(title, ''::text)), 'A'::"char") || setweight(to_tsvector('simple'::regconfig, COALESCE(description, ''::text)), 'B'::"char")) || setweight(to_tsvector('simple'::regconfig, COALESCE(variety, ''::text)), 'A'::"char")) || setweight(to_tsvector('simple'::regconfig, COALESCE(breeder, ''::text)), 'C'::"char"))) STORED
);
ALTER TABLE listings ADD CONSTRAINT listings_pkey PRIMARY KEY (id);
ALTER TABLE listings ADD CONSTRAINT listings_farm_address_id_fkey FOREIGN KEY (farm_address_id) REFERENCES addresses(id) ON DELETE SET NULL;
ALTER TABLE listings ADD CONSTRAINT listings_seller_id_fkey FOREIGN KEY (seller_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE listings ADD CONSTRAINT listings_media_urls_check CHECK ((jsonb_typeof(media_urls) = 'array'::text));
ALTER TABLE listings ADD CONSTRAINT listings_price_per_unit_check CHECK ((price_per_unit >= 0));
ALTER TABLE listings ADD CONSTRAINT listings_quantity_available_check CHECK ((quantity_available >= 0));

CREATE TABLE IF NOT EXISTS listing_shipping_options (
    listing_id uuid NOT NULL,
    shipping_option_id uuid NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);
ALTER TABLE listing_shipping_options ADD CONSTRAINT listing_shipping_options_pkey PRIMARY KEY (listing_id, shipping_option_id);
ALTER TABLE listing_shipping_options ADD CONSTRAINT listing_shipping_options_listing_id_fkey FOREIGN KEY (listing_id) REFERENCES listings(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS listing_views (
    listing_id uuid NOT NULL,
    viewer_id uuid NOT NULL,
    viewed_at timestamp with time zone DEFAULT now() NOT NULL,
    view_date date DEFAULT CURRENT_DATE NOT NULL
);
ALTER TABLE listing_views ADD CONSTRAINT listing_views_pkey PRIMARY KEY (listing_id, viewer_id, view_date);
ALTER TABLE listing_views ADD CONSTRAINT listing_views_listing_id_fkey FOREIGN KEY (listing_id) REFERENCES listings(id) ON DELETE CASCADE;
ALTER TABLE listing_views ADD CONSTRAINT listing_views_viewer_id_fkey FOREIGN KEY (viewer_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE shipping_quotes ADD COLUMN IF NOT EXISTS listing_id uuid;
CREATE INDEX IF NOT EXISTS idx_shipping_quotes_listing_id ON shipping_quotes USING btree (listing_id) WHERE (listing_id IS NOT NULL);

ALTER TABLE order_items ADD COLUMN IF NOT EXISTS listing_id uuid;
ALTER TABLE order_items ADD CONSTRAINT order_items_listing_id_fkey FOREIGN KEY (listing_id) REFERENCES listings(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_order_items_listing_id ON order_items USING btree (listing_id);

ALTER TABLE pricing_tokens ADD COLUMN IF NOT EXISTS listing_id uuid;
CREATE INDEX IF NOT EXISTS idx_pricing_tokens_listing_id ON pricing_tokens USING btree (listing_id);

ALTER TABLE auctions ADD COLUMN IF NOT EXISTS listing_id uuid;
ALTER TABLE auctions ADD CONSTRAINT auctions_listing_id_fkey FOREIGN KEY (listing_id) REFERENCES listings(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_auctions_listing_id ON auctions USING btree (listing_id);
