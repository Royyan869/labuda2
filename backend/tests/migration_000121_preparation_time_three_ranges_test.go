//go:build integration

package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hishumi/backend/pkg/testdb"
)

// TestMigration000121_PreparationTimeThreeRanges proves that after migration
// 000121 UP the effective preparation_time_enum contains exactly the three
// canonical range values (1_3_days, 4_7_days, 8_15_days) and no longer carries
// the pre-000121 four-value vocabulary (immediate, short, medium, long) or the
// six legacy residues purged by 000103.
//
// It also locks the canonical consumer set (products.preparation_time and
// orders.preparation_time_snapshot) and proves the DB layer accepts every
// canonical producer value while rejecting every purged value.
func TestMigration000121_PreparationTimeThreeRanges(t *testing.T) {
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

	require.Equal(t, []string{"1_3_days", "4_7_days", "8_15_days"}, labels,
		"preparation_time_enum must contain exactly the three canonical range values")

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

	// 3. Zero rows in any consumer column hold a value outside the three ranges
	// (covers both the six 000103 legacy residues and the four pre-000121
	// canonical values — 000121's data mapping converted every one of them).
	var outside int64
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM products
			 WHERE preparation_time::text NOT IN ('1_3_days','4_7_days','8_15_days')) +
			(SELECT COUNT(*) FROM orders
			 WHERE preparation_time_snapshot IS NOT NULL
			   AND preparation_time_snapshot::text NOT IN ('1_3_days','4_7_days','8_15_days'))
	`).Scan(&outside))
	require.Zero(t, outside, "no live row may use a value outside the three canonical ranges")

	// 4. The DB layer accepts every canonical producer value...
	for _, v := range []string{"1_3_days", "4_7_days", "8_15_days"} {
		var out string
		require.NoError(t, pool.QueryRow(ctx, `SELECT $1::preparation_time_enum::text`, v).Scan(&out))
		require.Equal(t, v, out)
	}

	// ...and rejects every purged value (pre-000121 vocabulary + legacy residues).
	purged := []string{
		"immediate", "short", "medium", "long",
		"same_day", "1_2_days", "3_5_days", "6_10_days", "11_15_days", "more_than_15_days",
	}
	for _, legacy := range purged {
		var out string
		err := pool.QueryRow(ctx, `SELECT $1::preparation_time_enum::text`, legacy).Scan(&out)
		require.Error(t, err, "purged value %q must be rejected by the narrowed enum", legacy)
	}
}
