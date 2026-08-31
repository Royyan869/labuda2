//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

// TestMigration000047_SchemaStateProof verifies that after migration 000047 UP:
//   - table `for_sales` exists, `fixed_price_sales` does not
//   - enum `for_sale_status_enum` exists, `fixed_price_sale_status_enum` does not
//   - FK columns are renamed: `for_sale_id`, `for_sale_source_id`
//   - enum values use `for_sale`, not `fixed_price_sale` or `listing`
//   - triggers/indexes/constraints use canonical names
//   - orphaned `listing_*` enums are dropped
func TestMigration000047_SchemaStateProof(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	exists := func(q string) bool {
		var ok bool
		require.NoError(t, pool.QueryRow(ctx, q).Scan(&ok))
		return ok
	}

	// 1. Tables
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='for_sales')`),
		"for_sales must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='fixed_price_sales')`),
		"fixed_price_sales must not exist")

	// 2. Enums
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='for_sale_status_enum')`),
		"for_sale_status_enum must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='fixed_price_sale_status_enum')`),
		"fixed_price_sale_status_enum must not exist")

	// 3. Orphaned listing enums dropped
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='listing_status_enum')`),
		"listing_status_enum must be dropped")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='listing_type_enum')`),
		"listing_type_enum must be dropped")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='listing_visibility_enum')`),
		"listing_visibility_enum must be dropped")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='listing_origin_enum')`),
		"listing_origin_enum must be dropped")

	// 4. FK columns renamed
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='comment_commerce_references' AND column_name='for_sale_id')`),
		"comment_commerce_references.for_sale_id must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='comment_commerce_references' AND column_name='fixed_price_sale_id')`),
		"comment_commerce_references.fixed_price_sale_id must not exist")

	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='negotiation_sessions' AND column_name='for_sale_id')`),
		"negotiation_sessions.for_sale_id must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='negotiation_sessions' AND column_name='fixed_price_sale_id')`),
		"negotiation_sessions.fixed_price_sale_id must not exist")

	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='content_resource_occurrences' AND column_name='for_sale_source_id')`),
		"content_resource_occurrences.for_sale_source_id must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='content_resource_occurrences' AND column_name='fixed_price_sale_source_id')`),
		"content_resource_occurrences.fixed_price_sale_source_id must not exist")

	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='chat_message_resource_occurrences' AND column_name='for_sale_source_id')`),
		"chat_message_resource_occurrences.for_sale_source_id must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='chat_message_resource_occurrences' AND column_name='fixed_price_sale_source_id')`),
		"chat_message_resource_occurrences.fixed_price_sale_source_id must not exist")

	// 5. Enum values converged
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='order_source_enum' AND enumlabel='for_sale')`),
		"order_source_enum must have 'for_sale'")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='order_source_enum' AND enumlabel='fixed_price_sale')`),
		"order_source_enum must not have 'fixed_price_sale'")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='order_source_enum' AND enumlabel='listing')`),
		"order_source_enum must not have 'listing'")

	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='sale_surface_type_enum' AND enumlabel='for_sale')`),
		"sale_surface_type_enum must have 'for_sale'")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='sale_surface_type_enum' AND enumlabel='fixed_price_sale')`),
		"sale_surface_type_enum must not have 'fixed_price_sale'")

	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='negotiation_resource_enum' AND enumlabel='for_sale')`),
		"negotiation_resource_enum must have 'for_sale'")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='negotiation_resource_enum' AND enumlabel='fixed_price_sale')`),
		"negotiation_resource_enum must not have 'fixed_price_sale'")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='negotiation_resource_enum' AND enumlabel='listing')`),
		"negotiation_resource_enum must not have 'listing'")

	// moderation_resource_enum was dropped by migration 000056 (legacy
	// GovernanceCase schema removed). The canonical replacement enum is
	// moderation_target_type_enum (000055).
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type t WHERE t.typname='moderation_resource_enum')`),
		"moderation_resource_enum must not exist after migration 000056 (rejected legacy schema)")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='moderation_target_type_enum' AND enumlabel='for_sale')`),
		"moderation_target_type_enum must have 'for_sale'")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='moderation_target_type_enum' AND enumlabel='fixed_price_sale')`),
		"moderation_target_type_enum must not have 'fixed_price_sale'")

	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='discount_applies_to_enum' AND enumlabel='for_sale')`),
		"discount_applies_to_enum must have 'for_sale'")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='discount_applies_to_enum' AND enumlabel='listing')`),
		"discount_applies_to_enum must not have 'listing'")

	// chat_commerce_reference_target_type_enum dropped by migration 000054
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type t WHERE t.typname='chat_commerce_reference_target_type_enum')`),
		"chat_commerce_reference_target_type_enum must not exist after migration 000054")

	// 6. Triggers renamed
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE NOT tgisinternal AND tgname='trg_for_sales_permanent_exclusivity')`),
		"trg_for_sales_permanent_exclusivity must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE NOT tgisinternal AND tgname='trg_fixed_price_sales_single_active_channel')`),
		"trg_fixed_price_sales_single_active_channel must not exist")

	// 7. Indexes renamed
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='uniq_active_for_sale_per_product')`),
		"uniq_active_for_sale_per_product must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='uniq_active_fixed_price_sale_per_product')`),
		"uniq_active_fixed_price_sale_per_product must not exist")

	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='idx_for_sales_product_id')`),
		"idx_for_sales_product_id must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='idx_fixed_price_sales_product_id')`),
		"idx_fixed_price_sales_product_id must not exist")

	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='idx_comment_commerce_ref_for_sale')`),
		"idx_comment_commerce_ref_for_sale must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname='public' AND indexname='idx_comment_commerce_ref_fps')`),
		"idx_comment_commerce_ref_fps must not exist")

	// 8. Constraints renamed
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='for_sales_product_id_fkey')`),
		"for_sales_product_id_fkey must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='fixed_price_sales_product_id_fkey')`),
		"fixed_price_sales_product_id_fkey must not exist")

	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='comment_commerce_references_for_sale_id_fkey')`),
		"comment_commerce_references_for_sale_id_fkey must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='comment_commerce_references_fixed_price_sale_id_fkey')`),
		"comment_commerce_references_fixed_price_sale_id_fkey must not exist")
}
