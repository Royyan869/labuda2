# Migrations

The server does not auto-run migrations. Run explicitly:

```bash
cd backend
go run ./cmd/migrate
```

## Current state

`backend/migrations/` contains the canonical baseline plus additive hardening migrations:

```
000001_canonical_schema.up.sql                                    — full schema from zero (106 tables, 52 enum types)
000002_negotiation_schema_alignment.up.sql                        — negotiation_sessions columns; adds 'fixed_price_sale' to negotiation_resource_enum
000003_identity_email_uniqueness.up.sql                           — hardens canonical email identity invariants
000004_auction_anti_sniping.up.sql                                — adds auctions.anti_snipe_extension_seconds
000005_payment_webhook_captured_after_expiry.up.sql                — adds payment_webhook_status_enum value 'captured_after_expiry'
000006_payment_method_fee_model.up.sql                             — creates payment_methods table
000007_payment_method_rate_source_baseline.up.sql                  — Midtrans fee baseline/rate-source metadata
000008_payment_webhook_captured_after_expiry_index.up.sql          — index split out of 000005 (new enum value can't be used in the same tx that added it)
000009_fixed_price_sale_quantity_persistence.up.sql                — adds fixed_price_sales.quantity_available (was previously faked from status alone)
000010_product_sale_channel_canonicalization.up.sql                — PASS_21C: drops the legacy listings table, auctions.listing_id, and all other dead
                                                                       listing_id columns (pricing_tokens, order_items, shipping_quotes — the last of
                                                                       these was a live NOT NULL-with-no-writer bug); adds a DB trigger enforcing one
                                                                       active selling channel (Listing or Auction) per product
000011_prune_orphan_tables.up.sql                                  — PHASE 1 CLEANUP: drops 6 tables with zero application code references
                                                                       (actors, bnr_classifications, financial_reconciliations, search_results,
                                                                       ticket_escalations, user_online_status)
000044_product_lifecycle_removal.up.sql                           — Model B (Stage 3): drops products.status, products.sold_at, idx_products_status,
                                                                     product_status_enum (Product carries no selling lifecycle; availability is
                                                                     derived from the active selling surface)
000045_order_item_product_identity_convergence.up.sql             — Stage 5: order_items.product_id is always products.id (FK enforced);
                                                                     converges historical FPS/negotiation rows onto products.id
000046_product_content_authority_convergence.up.sql               — Stage 8: Product is sole canonical authority for title/description/media/
                                                                     koi attributes/preparation. Drops auctions.title, auctions.description,
                                                                     auctions.preparation_time, auctions.preparation_note. Drops dead media
                                                                     tables fixed_price_sale_media and auction_media (canonical media is
                                                                     products.media_urls).
```

This baseline was generated from the live DB state (v100–v229) on 2026-07-03 and represents
the authoritative schema for a clean Labuda installation.

## Adding new migrations

New migrations start at `000012`. Use `NNNNNN_description.{up,down}.sql` naming.
Both `.up.sql` and `.down.sql` files are required.

## Legacy history

The old incremental chain (v100–v229, 115 pairs) has been squashed into `000001`.
Those files are no longer in the repo. Git history (`763bff97` and prior) preserves
the full chain for forensic lookup.

Do not restore old migration files under `backend/migrations/`. The canonical baseline
is the single source of truth for schema bootstrapping on a fresh database.
