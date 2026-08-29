//go:build integration

package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

// TestMigration000047_ForSaleVocabulary_UpDownReplay verifies the Stage C
// vocabulary convergence migration:
//
//	UP:   fixed_price_sales -> for_sales, enum values -> for_sale, columns/FK
//	      columns/indexes/triggers renamed, orphaned listing_* enums dropped.
//	DOWN: restores fixed_price_sales, old enum values, old constraint names.
//	UP:   replay converges again.
//
// It follows the same execSQLFile pattern as the 000044 / 000045 replay
// tests (down/up applied inside one transaction per file).
func TestMigration000047_ForSaleVocabulary_UpDownReplay(t *testing.T) {
	tdb, cleanup := testdb.SetupDB(t) // applies all up migrations incl. 000047
	defer cleanup()
	ctx := context.Background()
	pool := tdb.Pool()

	execSQLFile := func(name string) {
		t.Helper()
		path := filepath.Join("..", "migrations", name)
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "read %s", path)

		src := string(raw)
		var statements []string
		var buf strings.Builder
		inDollar := false
		dollarTag := ""
		runes := []rune(src)
		for i := 0; i < len(runes); i++ {
			c := runes[i]

			if c == '$' {
				j := i + 1
				for j < len(runes) && runes[j] != '$' {
					j++
				}
				if j < len(runes) {
					tag := string(runes[i : j+1])
					if !inDollar {
						inDollar = true
						dollarTag = tag
						buf.WriteString(tag)
						i = j
						continue
					} else if tag == dollarTag {
						inDollar = false
						buf.WriteString(tag)
						i = j
						continue
					}
				}
			}

			if inDollar {
				buf.WriteRune(c)
				continue
			}

			if c == '-' && i+1 < len(runes) && runes[i+1] == '-' {
				for i < len(runes) && runes[i] != '\n' {
					i++
				}
				continue
			}

			if c == ';' {
				stmt := strings.TrimSpace(buf.String())
				if stmt != "" {
					statements = append(statements, stmt)
				}
				buf.Reset()
				continue
			}

			buf.WriteRune(c)
		}
		if s := strings.TrimSpace(buf.String()); s != "" {
			statements = append(statements, s)
		}

		require.NotEmpty(t, statements)
		require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
			for _, stmt := range statements {
				if _, err := tx.Exec(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		}), "exec %s", name)
	}

	exists := func(q string) bool {
		var ok bool
		require.NoError(t, pool.QueryRow(ctx, q).Scan(&ok))
		return ok
	}

	// Post-up state (SetupDB ran 000047).
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE tablename='for_sales')`), "for_sales must exist after up")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE tablename='fixed_price_sales')`), "fixed_price_sales must be gone after up")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='fixed_price_sale_status_enum')`), "old status enum must be gone")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='for_sale_status_enum')`), "for_sale_status_enum must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='listing_status_enum')`), "orphaned listing_status_enum must be dropped")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='comment_commerce_references' AND column_name='for_sale_id')`), "comment_commerce_references.for_sale_id must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='comment_commerce_references' AND column_name='fixed_price_sale_id')`), "old column must be gone")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname='uniq_active_for_sale_per_product')`), "canonical partial index must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='order_source_enum' AND enumlabel='listing')`), "'listing' must be gone from order_source_enum")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='order_source_enum' AND enumlabel='for_sale')`), "'for_sale' must be in order_source_enum")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='trg_for_sales_permanent_exclusivity')`), "canonical trigger must exist")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='trg_fixed_price_sales_single_active_channel')`), "old trigger must be gone")

	// Down: old vocabulary restored.
	execSQLFile("000047_for_sale_vocabulary_convergence.down.sql")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE tablename='fixed_price_sales')`), "fixed_price_sales must be restored by down")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE tablename='for_sales')`), "for_sales must be gone after down")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname='fixed_price_sale_status_enum')`), "old status enum must be restored")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid WHERE t.typname='order_source_enum' AND enumlabel='listing')`), "'listing' must be restored in order_source_enum")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='trg_fixed_price_sales_single_active_channel')`), "old trigger must be restored")

	// Replay up: converges again.
	execSQLFile("000047_for_sale_vocabulary_convergence.up.sql")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE tablename='for_sales')`), "for_sales must exist after replay up")
	require.False(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_tables WHERE tablename='fixed_price_sales')`), "fixed_price_sales must be gone after replay up")
	require.True(t, exists(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname='uniq_active_for_sale_per_product')`), "canonical index must exist after replay up")
}
