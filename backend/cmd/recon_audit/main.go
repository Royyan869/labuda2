// Command recon_audit is the Phase 1B one-shot Gateway Payment
// Reconciliation audit. It is a read-only validation tool — it produces
// findings, NOT mutations.
//
// Constitutional posture (locked by owner, see Phase 1B brief):
//   - READ-ONLY across DB and gateway.
//   - NO scheduler, NO daemon, NO goroutine fan-out.
//   - NO alerts, NO outbox emission, NO replay, NO auto-fix, NO queue.
//   - On error: log and exit non-zero; never retry with side-effects.
//
// Example invocations:
//
//	# Local-only summary over the last 24h, 100-order limit.
//	./recon_audit
//
//	# Detailed audit of a specific order, with gateway call.
//	./recon_audit --mode=detailed --order-id=<uuid>
//
//	# JSON export to stdout for post-processing.
//	./recon_audit --mode=json --limit=500 --since=72h > findings.json
//
//	# Local-only mode (skip Midtrans entirely).
//	./recon_audit --no-gateway
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/integration/payment/application/recon"
	"github.com/labuda/backend/internal/integration/payment/application/recon/audit"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/internal/platform/logger"
)

type flags struct {
	mode               string
	limit              int
	sinceDur           time.Duration
	orderIDsRaw        string
	skipGateway        bool
	gatewayBudget      int
	gatewayTimeout     time.Duration
	outputPath         string
	logLevel           string
	pendingGrace       time.Duration
	orphanGrace        time.Duration
	stuckRefundGrace   time.Duration
	pendingExpiryGrace time.Duration
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "recon_audit failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	f := parseFlags()

	mode, err := audit.ParseOutputMode(f.mode)
	if err != nil {
		return err
	}

	orderIDs, err := parseOrderIDs(f.orderIDsRaw)
	if err != nil {
		return fmt.Errorf("parse --order-id: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logger.New(f.logLevel, "console", "stdout")
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	dbWrap, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		return fmt.Errorf("connect DB: %w", err)
	}
	defer func() { _ = database.CloseDB(dbWrap, log) }()

	var gw audit.GatewayQuery
	if !f.skipGateway {
		gw = audit.NewGatewayClient(
			cfg.Midtrans.ServerKey,
			cfg.Midtrans.Environment == "production",
			f.gatewayTimeout,
		)
	}

	resolver := audit.NewResolver(audit.ResolverConfig{
		Pool:    dbWrap.Pool(),
		Gateway: gw,
		Thresholds: recon.Thresholds{
			PendingPaymentGrace:       f.pendingGrace,
			OrphanRecoveryGrace:       f.orphanGrace,
			StuckRefundGrace:          f.stuckRefundGrace,
			PendingPaymentExpiryGrace: f.pendingExpiryGrace,
		},
		SkipGateway:   f.skipGateway,
		GatewayBudget: f.gatewayBudget,
	})

	orch := audit.NewOrchestrator(resolver, func() time.Time { return time.Now().UTC() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	since := time.Now().UTC().Add(-f.sinceDur)
	report, runErr := orch.Run(ctx, audit.RunSpec{
		OrderIDs: orderIDs,
		Since:    since,
		Limit:    f.limit,
	})

	out := os.Stdout
	if f.outputPath != "" && f.outputPath != "-" {
		file, err := os.Create(f.outputPath)
		if err != nil {
			return fmt.Errorf("open output: %w", err)
		}
		defer file.Close()
		out = file
	}

	if err := audit.Render(out, report, mode); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if runErr != nil {
		// Non-fatal: orchestration may have completed partial work, but the
		// caller wants to know the run was not clean.
		return fmt.Errorf("audit completed with run error: %w", runErr)
	}
	return nil
}

func parseFlags() *flags {
	f := &flags{}
	flag.StringVar(&f.mode, "mode", "summary", "output mode: summary | detailed | json")
	flag.IntVar(&f.limit, "limit", 100, "max candidates to scan when --order-id is not provided")
	flag.DurationVar(&f.sinceDur, "since", 24*time.Hour, "candidate scan window (orders.created_at >= now-since)")
	flag.StringVar(&f.orderIDsRaw, "order-id", "", "comma-separated order UUIDs; bypasses candidate scan")
	flag.BoolVar(&f.skipGateway, "no-gateway", false, "skip Midtrans; classifier sees Gateway.Available=false")
	flag.IntVar(&f.gatewayBudget, "gateway-budget", 100, "max gateway HTTP calls per run (0 = unlimited)")
	flag.DurationVar(&f.gatewayTimeout, "gateway-timeout", 15*time.Second, "per-request HTTP timeout for Midtrans status")
	flag.StringVar(&f.outputPath, "output", "-", "output file path; '-' or empty = stdout")
	flag.StringVar(&f.logLevel, "log-level", "warn", "zap log level: debug | info | warn | error")
	flag.DurationVar(&f.pendingGrace, "pending-grace", 3*time.Minute, "D1 grace window — pending payments younger than this are not flagged")
	flag.DurationVar(&f.orphanGrace, "orphan-grace", 2*time.Minute, "D1/D6 suppression window for orphaned webhook recovery")
	flag.DurationVar(&f.stuckRefundGrace, "stuck-refund-grace", 5*time.Minute, "D11 grace window — pending refunds younger than this are not flagged")
	flag.DurationVar(&f.pendingExpiryGrace, "pending-expiry-grace", 1*time.Minute, "D12 grace window past payment expiry")
	flag.Parse()
	return f
}

func parseOrderIDs(raw string) ([]uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]uuid.UUID, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := uuid.Parse(p)
		if err != nil {
			return nil, fmt.Errorf("invalid uuid %q: %w", p, err)
		}
		out = append(out, id)
	}
	return out, nil
}
