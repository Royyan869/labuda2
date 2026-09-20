//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

// TestMigration000102_OrderSourceEnumNarrowed proves that after migration
// 000102 UP the effective order_source_enum contains exactly the three canonical
// values (for_sale, auction, negotiation) and no longer carries the obsolete
// 'seller_quote' residue.
//
// It also locks the canonical OrderSourceType set at the schema layer so a
// future migration cannot silently re-add a value that no production code can
// produce.
func TestMigration000102_OrderSourceEnumNarrowed(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	labels := func() []string {
		rows, err := pool.Query(ctx, `
			SELECT e.enumlabel
			FROM pg_enum e
			JOIN pg_type t ON t.oid = e.enumtypid
			WHERE t.typname = 'order_source_enum'
			ORDER BY e.enumsortorder
		`)
		require.NoError(t, err)
		defer rows.Close()

		var out []string
		for rows.Next() {
			var label string
			require.NoError(t, rows.Scan(&label))
			out = append(out, label)
		}
		require.NoError(t, rows.Err())
		return out
	}

	got := labels()
	require.Equal(t, []string{"for_sale", "auction", "negotiation"}, got,
		"order_source_enum must contain exactly the canonical OrderSourceType values")

	exists := func(q string) bool {
		var ok bool
		require.NoError(t, pool.QueryRow(ctx, q).Scan(&ok))
		return ok
	}

	require.False(t, exists(`SELECT EXISTS(
		SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid = e.enumtypid
		WHERE t.typname = 'order_source_enum' AND e.enumlabel = 'seller_quote')`),
		"order_source_enum must not retain the obsolete 'seller_quote' value")

	// The orders.source_type column remains NOT NULL of the narrowed enum type.
	require.True(t, exists(`SELECT EXISTS(
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'orders'
		  AND column_name = 'source_type' AND udt_name = 'order_source_enum'
		  AND is_nullable = 'NO')`),
		"orders.source_type must remain a NOT NULL order_source_enum column")

	// Negative proof: no live row can hold the purged value.
	require.False(t, exists(`SELECT EXISTS(
		SELECT 1 FROM orders WHERE source_type::text = 'seller_quote')`),
		"no orders row may use the purged 'seller_quote' source type")
}
