package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/bootstrap"
	"github.com/labuda/backend/internal/platform/capability"
	capRepo "github.com/labuda/backend/internal/platform/capability/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/database"
)

func main() {
	var userIDStr string
	var emailStr string
	var dryRun bool
	flag.StringVar(&userIDStr, "user-id", "", "Target user UUID (canonical)")
	flag.StringVar(&emailStr, "email", "", "Target user email (normalized lookup)")
	flag.BoolVar(&dryRun, "dry-run", false, "Validate only, do not mutate")
	flag.Parse()

	if dryRun {
		fmt.Fprintln(os.Stderr, "--dry-run is recognized but this build executes validation via read-only transaction; use without --dry-run to commit")
	}

	hasID := strings.TrimSpace(userIDStr) != ""
	hasEmail := strings.TrimSpace(emailStr) != ""
	if hasID == hasEmail {
		fmt.Fprintln(os.Stderr, "error: exactly one of --user-id or --email is required (mutually exclusive)")
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init failed: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	db, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect failed: %v\n", err)
		os.Exit(1)
	}
	defer database.CloseDB(db, log)

	capRepository := capRepo.NewCapabilityRepository(db.Pgx())
	auditLogger := audit.NewAdminAuditLoggerDB(db.Pgx().Pool())
	svc := bootstrap.NewService(db.Pgx(), capRepository, auditLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var result *bootstrap.Result
	var svcErr error
	if hasID {
		uid, parseErr := uuid.Parse(strings.TrimSpace(userIDStr))
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "invalid --user-id: %v\n", parseErr)
			os.Exit(2)
		}
		if dryRun {
			result, svcErr = svc.DryRunByID(ctx, uid)
		} else {
			result, svcErr = svc.BootstrapByID(ctx, uid)
		}
	} else {
		if dryRun {
			result, svcErr = svc.DryRunByEmail(ctx, emailStr)
		} else {
			result, svcErr = svc.BootstrapByEmail(ctx, emailStr)
		}
	}
	if svcErr != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", svcErr)
		os.Exit(1)
	}
	if result == nil {
		fmt.Fprintln(os.Stderr, "bootstrap failed: nil result without error")
		os.Exit(1)
	}

	// Clear reporting.
	fmt.Println("bootstrap-admin: SUCCESS")
	fmt.Printf("  target_id: %s\n", result.TargetID.String())
	if result.Email != "" {
		fmt.Printf("  email: %s\n", result.Email)
	}
	if result.RoleChanged {
		fmt.Printf("  role: %s -> %s (changed)\n", result.PreviousRole, result.NewRole)
	} else {
		fmt.Printf("  role: %s (already admin, preserved idempotency)\n", result.NewRole)
	}
	fmt.Printf("  capabilities_created: %d\n", result.Created)
	fmt.Printf("  capabilities_skipped: %d\n", result.SkippedExisting)
	if result.Invalid > 0 {
		fmt.Printf("  invalid_capabilities: %d\n", result.Invalid)
	}
	if len(result.Errors) > 0 {
		fmt.Printf("  errors: %v\n", result.Errors)
		os.Exit(1)
	}
	fmt.Printf("  final: role=admin + %d/%d canonical capabilities active -> derived full access\n",
		result.Created+result.SkippedExisting, len(capability.AllCapabilities()))
}
