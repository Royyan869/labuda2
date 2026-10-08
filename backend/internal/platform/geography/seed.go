package geography

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedGz is the complete Indonesian Province -> Regency -> District -> Village
// tree (BPS codes, dots removed) with village postal codes. It is seed input
// for the canonical master only — it is never a runtime authority.
//
//go:embed data/geography.tsv.gz
var seedGz []byte

// Seed populates canonical_geographies from the embedded dataset.
//
// It is idempotent: when the master already holds rows it is a no-op, so it is
// safe to call from cmd/migrate and from the test bootstrap on every run. Rows
// are copied level by level (province, then regency, then district, then
// village) so the self-referencing parent FK is always satisfied.
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	var existing int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM canonical_geographies`).Scan(&existing); err != nil {
		return fmt.Errorf("geography seed: count master: %w", err)
	}
	if existing > 0 {
		return nil
	}

	gz, err := gzip.NewReader(bytes.NewReader(seedGz))
	if err != nil {
		return fmt.Errorf("geography seed: open embedded dataset: %w", err)
	}
	defer gz.Close()

	byLevel := map[Level][][]any{
		LevelProvince: {},
		LevelRegency:  {},
		LevelDistrict: {},
		LevelVillage:  {},
	}

	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 5 {
			return fmt.Errorf("geography seed: malformed row %q", line)
		}
		level := Level(fields[1])
		if _, ok := byLevel[level]; !ok {
			return fmt.Errorf("geography seed: unknown level %q", fields[1])
		}
		var parent, postal *string
		if fields[3] != "" {
			p := fields[3]
			parent = &p
		}
		if fields[4] != "" {
			p := fields[4]
			postal = &p
		}
		byLevel[level] = append(byLevel[level], []any{fields[0], fields[1], fields[2], parent, postal})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("geography seed: read dataset: %w", err)
	}

	cols := []string{"code", "level", "name", "parent_code", "postal_code"}
	for _, level := range []Level{LevelProvince, LevelRegency, LevelDistrict, LevelVillage} {
		rows := byLevel[level]
		if len(rows) == 0 {
			continue
		}
		if _, err := pool.CopyFrom(ctx, pgx.Identifier{"canonical_geographies"}, cols, pgx.CopyFromRows(rows)); err != nil {
			return fmt.Errorf("geography seed: copy %s: %w", level, err)
		}
	}
	return nil
}

// Count returns the number of rows currently in the master (diagnostics/tests).
func Count(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var n int64
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM canonical_geographies`).Scan(&n)
	return n, err
}
