// Command corpus_driver is the Phase 1B controlled corpus generator.
//
// PURPOSE (current scaffold step, 2026-05-12):
//
//	Prove that the canonical service graph produced by serverboot.InitServices
//	can be re-driven by a sibling binary WITHOUT starting background workers.
//	No business actions are taken yet — scenarios for D1/D3/D4/D8/D9/D11/D12
//	will land on top of this scaffold in a follow-up.
//
// CONSTITUTIONAL POSTURE:
//   - corpus_driver MUST go through canonical services. It MUST NOT
//     fabricate business rows, bypass outbox emission, write to the ledger,
//     or flip escrow state by direct SQL.
//   - corpus_driver does NOT call serverboot.StartWorkers. Workers
//     observed by recon_audit during corpus runs are the SAME worker
//     goroutines run by a separately-launched core_server process.
//   - The DB connection here SHARES the DB with the running core_server;
//     the two processes coordinate via canonical rows (orders / payments /
//     escrows / outbox).
//
// USAGE:
//
//	./corpus_driver --mode=list-services
//	  # Prints a summary of which canonical handlers and workers were
//	  # constructed by InitServices. Used to validate that the service
//	  # graph reuse path works end-to-end. Workers stay DORMANT.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/serverboot"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/firebase"
	"github.com/labuda/backend/pkg/midtrans"
	pkgRedis "github.com/labuda/backend/pkg/redis"
	"go.uber.org/zap"
)

func main() {
	mode := flag.String("mode", "list-services",
		"corpus_driver mode: list-services | scenario-d11 | scenario-projection | scenario-governance-content | scenario-governance-feed | scenario-governance-detail")

	// Flags below are consumed only by scenario-governance-content. Other
	// scenarios ignore them — declaring them globally keeps `flag.Parse`
	// from rejecting them when present.
	baseURL := flag.String("base-url", "http://localhost:8080",
		"scenario-governance-content: base URL of the running core_server")
	outputDir := flag.String("output-dir", "",
		"scenario-governance-content: artifact dir (default scenario_logs/governance-content-<runID>)")
	keyword := flag.String("keyword", "",
		"scenario-governance-content: unique search keyword embedded in created content (default auto-generated)")
	timeout := flag.Duration("timeout", 15*time.Second,
		"scenario-governance-content: per-request HTTP timeout")
	verbose := flag.Bool("verbose", false,
		"scenario-governance-content: also print the run summary to stdout")
	flag.Parse()

	if err := run(*mode, governanceContentConfig{
		BaseURL:   *baseURL,
		OutputDir: *outputDir,
		Keyword:   *keyword,
		Timeout:   *timeout,
		Verbose:   *verbose,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "corpus_driver failed: %v\n", err)
		os.Exit(1)
	}
}

func run(mode string, govCfg governanceContentConfig) error {
	env := os.Getenv("ENV")
	printDisclosure(mode, env)

	if err := checkEnvironmentGate(mode, env); err != nil {
		return err
	}

	// HTTP-only scenarios dispatch BEFORE the heavy service-graph init.
	// scenario-governance-content speaks only to the running core_server
	// over HTTP, so it has zero need for a DB / Redis / Firebase /
	// Midtrans client in this process. Skipping initInfra also lets the
	// scenario run when this dev box has no local Postgres but the
	// operator points base-url at a remote core_server.
	if mode == "scenario-governance-content" {
		err := runScenarioGovernanceContent(govCfg)
		printPostExecutionResidueDisclosure(mode, err)
		return err
	}
	if mode == "scenario-governance-feed" {
		err := runScenarioGovernanceFeed(governanceFeedConfig{
			BaseURL:   govCfg.BaseURL,
			OutputDir: govCfg.OutputDir,
			Keyword:   govCfg.Keyword,
			Timeout:   govCfg.Timeout,
			Verbose:   govCfg.Verbose,
		})
		printPostExecutionResidueDisclosure(mode, err)
		return err
	}
	if mode == "scenario-governance-detail" {
		err := runScenarioGovernanceDetail(governanceDetailConfig{
			BaseURL:   govCfg.BaseURL,
			OutputDir: govCfg.OutputDir,
			Keyword:   govCfg.Keyword,
			Timeout:   govCfg.Timeout,
			Verbose:   govCfg.Verbose,
		})
		printPostExecutionResidueDisclosure(mode, err)
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("corpus_driver_starting",
		zap.String("mode", mode),
		zap.String("env", cfg.Server.Env),
	)

	db, redisClient, firebaseClient, midtransClient, err := initInfra(cfg, log)
	if err != nil {
		return fmt.Errorf("init infra: %w", err)
	}
	defer database.CloseDB(db, log)
	defer redisClient.Close()

	// Re-use the canonical service graph. schemaReady=true: corpus_driver
	// runs against a DB that core_server has already migrated. We do NOT
	// call serverboot.StartWorkers — the whole point of corpus_driver is
	// to leave worker goroutines dormant so scenario timing is deterministic.
	deps := serverboot.InitServices(context.Background(), db, firebaseClient, midtransClient, redisClient, log, cfg, true)

	var scenarioErr error
	switch mode {
	case "list-services":
		printServiceGraphSummary(deps)
	case "scenario-d11":
		scenarioErr = runScenarioD11(deps, db, cfg, log)
	case "scenario-projection":
		scenarioErr = runScenarioProjection(deps, db, log.Logger)
	default:
		return fmt.Errorf("unknown mode %q (supported: list-services, scenario-d11, scenario-projection, scenario-governance-content, scenario-governance-feed, scenario-governance-detail)", mode)
	}
	printPostExecutionResidueDisclosure(mode, scenarioErr)
	return scenarioErr
}

// initInfra connects to the same primitives core_server uses. It does NOT
// apply migrations or seed data — corpus_driver expects the DB to already
// be in the state core_server left it in.
func initInfra(cfg *config.Config, log *logger.Logger) (
	*database.DB, *pkgRedis.Client, *firebase.Client, *midtrans.Client, error,
) {
	db, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("connect DB: %w", err)
	}

	redis, err := pkgRedis.NewRedisClient(&cfg.Redis, log)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("connect Redis: %w", err)
	}

	var fb *firebase.Client
	if cfg.Dev.MockFirebaseAuth {
		log.Warn("firebase_mock_enabled")
		fb = firebase.NewMockClient(log)
	} else if cfg.Firebase.ProjectID != "" && cfg.Firebase.ServiceAccountKeyPath != "" {
		fb, err = firebase.NewFirebaseClient(&cfg.Firebase, log)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("firebase init: %w", err)
		}
	} else {
		// corpus_driver does not exercise auth flows, but InitServices
		// expects a non-nil firebase client to wire AuthHandler. Fall back
		// to the mock client so the scaffold can boot without a Firebase
		// service-account file present. This does not relax production
		// auth: only corpus_driver uses this path.
		log.Warn("firebase_fallback_mock_for_corpus_driver",
			zap.String("reason", "no project_id or service_account_key_path configured"),
		)
		fb = firebase.NewMockClient(log)
	}

	mt := midtrans.NewClient(&cfg.Midtrans, log)

	return db, redis, fb, mt, nil
}

// checkEnvironmentGate returns a non-nil error if the scenario carries
// RUNTIME_MUTATION capability and the current environment is not development.
// Safe: ENV="" (unset — config defaults to "development") or ENV="development".
// Unsafe: ENV="production" or ENV="staging".
// READ_ONLY, CONTROLLED_WRITE, and OBSERVATION scenarios are not gated.
func checkEnvironmentGate(mode, env string) error {
	runtimeMutationScenarios := map[string]bool{
		"scenario-d11": true,
	}
	if !runtimeMutationScenarios[mode] {
		return nil
	}
	if env == "" || env == "development" {
		return nil
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "corpus_driver -- environment safety gate BLOCKED")
	fmt.Fprintln(os.Stderr, "------------------------------------------------")
	fmt.Fprintf(os.Stderr, "  Scenario        : %s\n", mode)
	fmt.Fprintf(os.Stderr, "  Capability Class: RUNTIME_MUTATION\n")
	fmt.Fprintf(os.Stderr, "  Environment     : %s\n", env)
	fmt.Fprintf(os.Stderr, "  Reason          : RUNTIME_MUTATION scenarios may not execute outside development\n")
	fmt.Fprintln(os.Stderr, "------------------------------------------------")
	fmt.Fprintln(os.Stderr, "")
	return fmt.Errorf("environment safety gate: scenario %q (RUNTIME_MUTATION) blocked in %q environment; only development is permitted", mode, env)
}

// printDisclosure writes a pre-execution governance disclosure to stderr.
// Visibility-only: no enforcement, no blocking, no ACK gate.
// Called before every scenario regardless of mode.
func printDisclosure(mode, env string) {
	type disclosureMeta struct {
		capabilityClass string
		mutationScope   string
		residuePosture  string
	}

	table := map[string]disclosureMeta{
		"list-services": {
			capabilityClass: "READ_ONLY",
			mutationScope:   "none — reads service graph only, no DB writes",
			residuePosture:  "NO_RESIDUE",
		},
		"scenario-d11": {
			capabilityClass: "RUNTIME_MUTATION",
			mutationScope:   "refunds (INSERT + UPDATE), outbox (INSERT), Midtrans gateway (HTTP POST), core_server webhook-drop arm (HTTP POST)",
			residuePosture:  "PERSISTENT_RESIDUE — refund rows and gateway records are not cleaned up automatically",
		},
		"scenario-projection": {
			capabilityClass: "CONTROLLED_WRITE",
			mutationScope:   "order_summaries (TRUNCATE + rebuild from orders) — business state unchanged",
			residuePosture:  "SELF_CLEANING — identical result on every run; no unrecoverable state",
		},
		"scenario-governance-content": {
			capabilityClass: "OBSERVATION",
			mutationScope:   "users, contents (created via HTTP to core_server) — test data persists in DB",
			residuePosture:  "PERSISTENT_RESIDUE — created rows remain; no automated cleanup",
		},
		"scenario-governance-feed": {
			capabilityClass: "OBSERVATION",
			mutationScope:   "users, contents, follow_relations (created via HTTP to core_server) — test data persists in DB",
			residuePosture:  "PERSISTENT_RESIDUE — created rows remain; no automated cleanup",
		},
		"scenario-governance-detail": {
			capabilityClass: "OBSERVATION",
			mutationScope:   "users, contents (created via HTTP to core_server) — test data persists in DB",
			residuePosture:  "PERSISTENT_RESIDUE — created rows remain; no automated cleanup",
		},
	}

	m, known := table[mode]
	if !known {
		m = disclosureMeta{
			capabilityClass: "UNKNOWN",
			mutationScope:   "unknown — unrecognised mode",
			residuePosture:  "UNKNOWN",
		}
	}

	envDisplay := env
	if envDisplay == "" {
		envDisplay = "(not set)"
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "corpus_driver -- execution disclosure")
	fmt.Fprintln(os.Stderr, "--------------------------------------")
	fmt.Fprintf(os.Stderr, "  Scenario        : %s\n", mode)
	fmt.Fprintf(os.Stderr, "  Capability Class: %s\n", m.capabilityClass)
	fmt.Fprintf(os.Stderr, "  Environment     : %s\n", envDisplay)
	fmt.Fprintf(os.Stderr, "  Mutation Scope  : %s\n", m.mutationScope)
	fmt.Fprintf(os.Stderr, "  Residue Posture : %s\n", m.residuePosture)
	fmt.Fprintln(os.Stderr, "--------------------------------------")
	fmt.Fprintln(os.Stderr, "")
}

// printPostExecutionResidueDisclosure writes a post-execution governance
// residue summary to stderr. Static disclosure only — no DB inspection.
// Called after every scenario result is known, before process exit.
func printPostExecutionResidueDisclosure(mode string, scenarioErr error) {
	type residueMeta struct {
		posture               string
		likelyRemainingState  string
		cleanupResponsibility string
	}

	table := map[string]residueMeta{
		"list-services": {
			posture:               "NO_RESIDUE",
			likelyRemainingState:  "none — read-only service graph inspection; no writes performed",
			cleanupResponsibility: "none required",
		},
		"scenario-d11": {
			posture:               "PERSISTENT_RESIDUE",
			likelyRemainingState:  "refund rows (refunds table), outbox events (outbox table), Midtrans gateway refund records; rows are NOT rolled back on scenario exit",
			cleanupResponsibility: "operator — manual DB cleanup or dedicated teardown script required before repeating; gateway records persist on Midtrans side",
		},
		"scenario-projection": {
			posture:               "SELF_CLEANING",
			likelyRemainingState:  "order_summaries table rebuilt to match current orders; idempotent — rerunning produces identical state",
			cleanupResponsibility: "none required — every run leaves the projection in a valid, deterministic state",
		},
		"scenario-governance-content": {
			posture:               "PERSISTENT_RESIDUE",
			likelyRemainingState:  "user accounts, content rows, follow_relations created via HTTP to core_server remain in DB; outbox events may have been consumed by the running core_server's outbox worker",
			cleanupResponsibility: "operator — rows are permanent test fixtures; delete via admin API or direct SQL before rerunning to avoid duplicate keyword collisions",
		},
		"scenario-governance-feed": {
			posture:               "PERSISTENT_RESIDUE",
			likelyRemainingState:  "user accounts, content rows, follow_relations created via HTTP to core_server remain in DB; outbox events may have been consumed by the running core_server's outbox worker",
			cleanupResponsibility: "operator — rows are permanent test fixtures; delete via admin API or direct SQL before rerunning to avoid duplicate keyword collisions",
		},
		"scenario-governance-detail": {
			posture:               "PERSISTENT_RESIDUE",
			likelyRemainingState:  "user accounts, content rows created via HTTP to core_server remain in DB; outbox events may have been consumed by the running core_server's outbox worker",
			cleanupResponsibility: "operator — rows are permanent test fixtures; delete via admin API or direct SQL before rerunning",
		},
	}

	m, known := table[mode]
	if !known {
		m = residueMeta{
			posture:               "UNKNOWN",
			likelyRemainingState:  "unrecognised mode — residue state cannot be determined statically",
			cleanupResponsibility: "operator — inspect DB and outbox manually",
		}
	}

	resultLine := "SUCCESS"
	if scenarioErr != nil {
		resultLine = fmt.Sprintf("FAILED (%v)", scenarioErr)
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "corpus_driver -- post-execution residue disclosure")
	fmt.Fprintln(os.Stderr, "--------------------------------------------------")
	fmt.Fprintf(os.Stderr, "  Scenario              : %s\n", mode)
	fmt.Fprintf(os.Stderr, "  Execution Result      : %s\n", resultLine)
	fmt.Fprintf(os.Stderr, "  Residue Posture       : %s\n", m.posture)
	fmt.Fprintf(os.Stderr, "  Likely Remaining State: %s\n", m.likelyRemainingState)
	fmt.Fprintf(os.Stderr, "  Cleanup Responsibility: %s\n", m.cleanupResponsibility)
	fmt.Fprintln(os.Stderr, "--------------------------------------------------")
	fmt.Fprintln(os.Stderr, "")
}

// printServiceGraphSummary writes a human-readable inventory of which
// canonical handlers + workers were materialised by InitServices. Used as
// the proof artifact that corpus_driver can re-use the production service
// graph.
func printServiceGraphSummary(d *serverboot.Dependencies) {
	if d == nil {
		fmt.Println("service graph: <nil>")
		return
	}
	report := []struct {
		name    string
		present bool
	}{
		{"AuthHandler", d.AuthHandler != nil},
		{"PaymentHandler", d.PaymentHandler != nil},
		{"PaymentWebhookHandler", d.PaymentWebhookHandler != nil},
		{"PayoutWebhookHandler", d.PayoutWebhookHandler != nil},
		{"OrderHandler", d.OrderHandler != nil},
		{"DiscountHandler", d.DiscountHandler != nil},
		{"DisputeHandler", d.DisputeHandler != nil},
		{"AdminRefundHandler", d.AdminRefundHandler != nil},
		{"ForSaleHandler", d.ForSaleHandler != nil},
		{"PricingTokenHandler", d.PricingTokenHandler != nil},
		{"AuctionHandler", d.AuctionHandler != nil},
		{"SellerHandler", d.SellerHandler != nil},
		{"CoinHandler", d.CoinHandler != nil},

		{"Worker:PaymentExpiry", d.PaymentExpiryWorker != nil},
		{"Worker:OrderAutoComplete", d.OrderAutoCompleteWorker != nil},
		{"Worker:Outbox", d.OutboxWorker != nil},
		{"Worker:Projection", d.ProjectionWorker != nil},
		{"Worker:Reconciliation", d.ReconciliationWorker != nil},
		{"Worker:Payout", d.PayoutWorker != nil},
		{"Worker:PayoutReconciliation", d.PayoutReconciliationWorker != nil},
		{"Worker:NegotiationExpire", d.NegotiationExpireWorker != nil},
		{"Worker:PromotionSafety", d.PromotionSafetyWorker != nil},
		{"Worker:OrderOverdueReminder", d.OrderOverdueReminderWorker != nil},
		{"Worker:DisputeTimeout", d.DisputeTimeoutWorker != nil},
		{"Worker:AlertDetection", d.AlertDetectionWorker != nil},
		{"Worker:SystemMonitoring", d.SystemMonitoringWorker != nil},
		{"Worker:Realtime", d.RealtimeWorker != nil},
		{"Worker:AuctionStart", d.AuctionStartWorker != nil},
		{"Worker:AuctionEnd", d.AuctionEndWorker != nil},
	}

	fmt.Println("corpus_driver service graph inventory")
	fmt.Println("=====================================")
	present := 0
	for _, r := range report {
		status := "MISSING"
		if r.present {
			status = "ok"
			present++
		}
		fmt.Printf("  %-44s %s\n", r.name, status)
	}
	fmt.Printf("\nPresent: %d/%d\n", present, len(report))
	fmt.Println("\nWorker goroutines: NOT STARTED (corpus_driver intentionally skips StartWorkers).")
}
