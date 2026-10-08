// Command migrate applies the canonical migration chain to the database
// described by the loaded configuration.
//
// It is a thin CLI over pkg/migration — the single migration executor — and
// deliberately contains no SQL splitting, loading, or version bookkeeping of
// its own. Every path that touches the chain (this CLI, cmd/core_server's
// boot check, pkg/testdb) therefore resolves, applies, and reads migrations
// with identical semantics.
//
// Usage (from backend/):
//
//	go run ./cmd/migrate                 # apply pending migrations
//	go run ./cmd/migrate -dir ../migrations
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/geography"
	"github.com/labuda/backend/pkg/migration"
)

func main() {
	dirFlag := flag.String("dir", "", "migration chain directory (default: resolved from the working directory)")
	budget := flag.Duration("timeout", 15*time.Minute, "overall budget for locating, loading, and applying the chain")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load database configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *budget)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Database.GetDSN())
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to reach database: %v", err)
	}
	log.Printf("database: %s:%s/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)

	// Bootstrap the ledger before reading the applied version: CurrentVersion
	// queries public.schema_migrations, which does not exist on a freshly
	// created database. EnsureSchemaMigrationsTable creates it when missing
	// (and fails closed on a legacy ledger) so this CLI can migrate an empty
	// database, not only one that has been migrated before. Run() calls it
	// again internally; the call is idempotent.
	if err := migration.EnsureSchemaMigrationsTable(ctx, pool); err != nil {
		log.Fatalf("failed to prepare the schema_migrations ledger: %v", err)
	}

	dir := *dirFlag
	if dir == "" {
		dir, err = migration.ResolveDir(".")
		if err != nil {
			log.Fatalf("failed to locate the migration chain: %v", err)
		}
	}

	migrations, err := migration.LoadMigrations(dir)
	if err != nil {
		log.Fatalf("failed to load the migration chain from %s: %v", dir, err)
	}
	if len(migrations) == 0 {
		log.Fatalf("migration chain at %s is empty", dir)
	}
	head := migrations[len(migrations)-1].Version

	before, err := migration.CurrentVersion(ctx, pool)
	if err != nil {
		log.Fatalf("failed to read the applied schema version: %v", err)
	}

	pending := 0
	for _, m := range migrations {
		if m.Version > before {
			pending++
		}
	}
	log.Printf("chain %s: head=%d applied=%d pending=%d", dir, head, before, pending)

	if err := migration.Run(ctx, pool, dir); err != nil {
		log.Fatalf("migration run failed: %v", err)
	}

	// Seed the ONE canonical Geography Master. Idempotent: a master that is
	// already populated is left untouched. This is seed input only — the
	// embedded dataset never serves as a runtime geography authority.
	if err := geography.Seed(ctx, pool); err != nil {
		log.Fatalf("geography seed failed: %v", err)
	}

	after, err := migration.CurrentVersion(ctx, pool)
	if err != nil {
		log.Fatalf("failed to read the schema version after the run: %v", err)
	}
	log.Printf("schema version %d -> %d (chain head %d)", before, after, head)

	if after != head {
		log.Fatalf("schema version %d does not match chain head %d after the run", after, head)
	}
}
