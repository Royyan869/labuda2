//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/testdb"
)

// TestMigration000103_PreparationTimeEnumNarrowed proves that after migration
// 000103 UP the effective preparation_time_enum contains exactly the four
// canonical values (immediate, short, medium, long) and no longer carries the
// six legacy residues.
//
// It also locks the canonical consumer set (products.preparation_time and
// orders.preparation_time_snapshot) and proves the DB layer accepts every
// canonical producer value while rejecting the purged legacy values.
func TestMigration000103_PreparationTimeEnumNarrowed(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	// 1. Enum labels are exactly the canonical set.
	var labels []string
	rows, err := pool.Query(ctx, `
		SELECT e.enumlabel
		FROM pg_enum e
		JOIN pg_type t ON t.oid = e.enumtypid
		WHERE t.typname = 'preparation_time_enum'
		ORDER BY e.enumsortorder
	`)
	require.NoError(t, err)
	for rows.Next() {
		var label string
		require.NoError(t, rows.Scan(&label))
		labels = append(labels, label)
	}
	require.NoError(t, rows.Err())
	rows.Close()

	require.Equal(t, []string{"immediate", "short", "medium", "long"}, labels,
		"preparation_time_enum must contain exactly the canonical values")

	// 2. The consumer set is exactly the two live columns.
	var consumers []string
	crows, err := pool.Query(ctx, `
		SELECT table_name || '.' || column_name
		FROM information_schema.columns
		WHERE udt_name = 'preparation_time_enum'
		ORDER BY table_name, column_name
	`)
	require.NoError(t, err)
	for crows.Next() {
		var c string
		require.NoError(t, crows.Scan(&c))
		consumers = append(consumers, c)
	}
	require.NoError(t, crows.Err())
	crows.Close()

	require.Equal(t, []string{"orders.preparation_time_snapshot", "products.preparation_time"}, consumers,
		"preparation_time_enum consumers must be exactly the canonical two columns")

	// products.preparation_time stays NOT NULL of the narrowed enum type.
	var nullable string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_name = 'products' AND column_name = 'preparation_time'
	`).Scan(&nullable))
	require.Equal(t, "NO", nullable, "products.preparation_time must remain NOT NULL")

	// 3. Zero legacy rows in every consumer column.
	var legacyProducts, legacyOrders int64
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM products
		WHERE preparation_time::text IN
			('same_day','1_2_days','3_5_days','6_10_days','11_15_days','more_than_15_days')
	`).Scan(&legacyProducts))
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM orders
		WHERE preparation_time_snapshot::text IN
			('same_day','1_2_days','3_5_days','6_10_days','11_15_days','more_than_15_days')
	`).Scan(&legacyOrders))
	require.Zero(t, legacyProducts+legacyOrders, "no live row may use a purged legacy preparation time")

	// 4. The DB layer accepts every canonical producer value...
	for _, v := range []string{"immediate", "short", "medium", "long"} {
		var out string
		require.NoError(t, pool.QueryRow(ctx, `SELECT $1::preparation_time_enum::text`, v).Scan(&out))
		require.Equal(t, v, out)
	}

	// ...and rejects the purged legacy values.
	for _, legacy := range []string{"same_day", "1_2_days", "3_5_days", "6_10_days", "11_15_days", "more_than_15_days"} {
		var out string
		err := pool.QueryRow(ctx, `SELECT $1::preparation_time_enum::text`, legacy).Scan(&out)
		require.Error(t, err, "legacy value %q must be rejected by the narrowed enum", legacy)
	}
}
