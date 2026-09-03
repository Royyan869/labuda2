package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/config"
	financeApp "github.com/labuda/backend/internal/finance/application"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/serverboot"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/firebase"
	"github.com/labuda/backend/pkg/midtrans"
	pkgRedis "github.com/labuda/backend/pkg/redis"
	"go.uber.org/zap"
)

// @title           Labuda Core Backend API
// @version         1.0
// @description     Labuda Financial Core Backend API - Clean Architecture with DDD
// @termsOfService  https://labuda.com/terms

// @contact.name   Labuda Support
// @contact.url    https://labuda.com/support
// @contact.email  support@labuda.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Firebase ID Token (Bearer token). Format: "Bearer <token>". Obtain token from Firebase Auth after user signs in. Token is validated on each request.

// @tag.name health
// @tag.description Health check endpoints

// @tag.name payments
// @tag.description Payment processing with Midtrans

// @tag.name coins
// @tag.description Coin/Loyalty points management

// @tag.name users
// @tag.description User management operations

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// PRODUCTION SAFETY: Validate critical security settings
	// PANICS immediately if unsafe config detected in production
	cfg.ValidateProductionSafety()

	// PAYOUT GATEWAY SAFETY: Validate payout gateway provider
	cfg.ValidatePayoutGatewayProvider()

	// PAYOUT COMPLETION LOOP SAFETY (PASS_18S): refuses to boot in
	// staging/production if PayoutWorker is enabled with no completion path
	// (no webhook secret, no functional reconciliation). No-ops in development.
	if err := cfg.ValidatePayoutCompletionPath(); err != nil {
		fmt.Printf("Payout completion path invalid: %v\n", err)
		os.Exit(1)
	}

	// STAGING ACTIVATION GUARD: Enforces invariants required before staging workers run.
	// No-ops in development; fatal in staging if guards fail.
	if err := cfg.ValidateStagingActivation(); err != nil {
		fmt.Printf("Staging activation blocked: %v\n", err)
		os.Exit(1)
	}

	// MIDTRANS PROVIDER ACTIVATION (STEP B): fail-fast guards.
	// Refuses to boot if Midtrans config is malformed or production-activated
	// before the production-unlock follow-up has landed.
	if err := validateMidtransConfig(cfg); err != nil {
		fmt.Printf("Midtrans config invalid: %v\n", err)
		os.Exit(1)
	}

	// Validate production environment configuration
	if err := validateProductionConfig(cfg); err != nil {
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting Labuda Core Backend",
		zap.String("version", cfg.App.Version),
		zap.String("environment", cfg.Server.Env),
		zap.String("port", cfg.Server.Port),
	)

	// Initialize infrastructure
	db, redisClient, firebaseClient, midtransClient := initInfrastructure(cfg, log)
	defer database.CloseDB(db, log)
	defer redisClient.Close()

	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// FINANCE RESURRECTION PR-C: activate canonical finance runtime.
	// The finance ledger is the canonical accounting authority and silent
	// degradation is not acceptable — missing schema or failed bootstrap
	// must halt boot. This must run AFTER migrations and BEFORE any
	// dependency wiring or worker startup.
	bootstrapFinanceOrFatal(db, log)

	// Check if database schema is ready for workers
	schemaReady := checkDatabaseSchemaReady(db, log)
	if !schemaReady {
		log.Warn("Database schema not ready — skipping worker startup")
	}

	// Initialize all dependencies
	// PHASE 1B SPLIT: InitServices builds the canonical service graph
	// without starting any worker goroutines; StartWorkers then invokes the
	// deferred startup closures recorded during InitServices. Production
	// behavior remains identical to the prior single-step InitDependencies
	// because the two calls are paired here unconditionally.
	deps := serverboot.InitServices(appCtx, db, firebaseClient, midtransClient, redisClient, log, cfg, schemaReady)
	serverboot.StartWorkers(deps)

	// Setup Gin router with rate limiter
	router, rateLimiter := setupRouter(cfg, log)

	// Setup all routes
	SetupRoutes(router, cfg, deps, firebaseClient, db, redisClient, log)

	// Start server
	startServer(appCtx, cfg, router, deps, rateLimiter, log)
}

// validateProductionConfig validates configuration for production environment
func validateProductionConfig(cfg *config.Config) error {
	if cfg.Server.Env != "production" {
		return nil
	}

	fmt.Println("Running production environment validation...")

	// Critical secrets that MUST NOT use default values in production
	insecureDefaults := []struct {
		name         string
		value        string
		defaultValue string
	}{
		{"JWT_SECRET", cfg.JWT.Secret, "change-me-in-production"},
		{"JWT_SECRET", cfg.JWT.Secret, "CHANGE_ME_GENERATE_WITH_OPENSSL"},
		{"DB_PASSWORD", cfg.Database.Password, "labuda123"},
		{"DB_PASSWORD", cfg.Database.Password, "CHANGE_ME_SET_VIA_ENV"},
		{"DB_PASSWORD", cfg.Database.Password, "CHANGE_ME_THIS_IS_NOT_SECURE"},
	}

	hasInsecureDefaults := false
	for _, check := range insecureDefaults {
		if check.value == check.defaultValue {
			fmt.Printf("SECURITY ERROR: %s is using insecure default value in production!\n", check.name)
			fmt.Printf("   Current value: %s\n", check.defaultValue)
			fmt.Printf("   Please set %s environment variable with a secure value\n", check.name)
			hasInsecureDefaults = true
		}
	}

	// Required secrets that MUST be set in production
	requiredSecrets := []struct {
		name  string
		value string
	}{
		{"MIDTRANS_SERVER_KEY", cfg.Midtrans.ServerKey},
		{"FIREBASE_PROJECT_ID", cfg.Firebase.ProjectID},
	}

	for _, check := range requiredSecrets {
		if check.value == "" {
			fmt.Printf("CONFIG ERROR: %s is required in production but not set\n", check.name)
			fmt.Printf("   Please set %s environment variable\n", check.name)
			hasInsecureDefaults = true
		}
	}

	if hasInsecureDefaults {
		fmt.Println("\nProduction deployment blocked due to insecure configuration")
		fmt.Println("   Set proper environment variables before running in production")
		fmt.Println("   For development, set ENV=development to skip this check")
		return fmt.Errorf("production validation failed")
	}

	fmt.Println("Production environment validation passed")
	return nil
}

// bootstrapFinanceOrFatal activates the canonical finance runtime.
//
// Behavior contract (FAIL-FAST — silent degradation forbidden):
//   - finance schema missing  -> log.Fatal (BOOT FAIL)
//   - bootstrap returns error -> log.Fatal (BOOT FAIL)
//   - bootstrap succeeds      -> structured info log with created_count
//
// Idempotent: rerun on every boot is safe. The bootstrap uses a partial-
// unique-index-aware ON CONFLICT and a SELECT-then-INSERT-then-verify
// pattern, so concurrent boots / restarts cannot create duplicate rows.
//
// Scope (PR-C): system accounts only (user_id IS NULL). User-level
// accounts such as SELLER_PAYABLE[seller] remain lazy-created on first
// release via ledgerRepo.GetOrCreateUserAccount, which is race-safe via
// the partial unique index uniq_financial_accounts_user_account_type.
func bootstrapFinanceOrFatal(db *database.DB, log *logger.Logger) {
	ctx := context.Background()

	// Step 1 - assert the finance ledger tables exist. Missing tables mean
	// the DB has not been migrated yet, so fail loudly and point the
	// operator at the explicit migration command.
	requiredTables := []string{
		"financial_accounts",
		"ledger_transactions",
		"ledger_entries",
	}
	for _, table := range requiredTables {
		var exists bool
		err := db.Pool().QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public'
				AND table_name = $1
			)
		`, table).Scan(&exists)

		if err != nil {
			log.Fatal("finance_bootstrap_failed",
				zap.String("phase", "schema_check"),
				zap.String("table", table),
				zap.Error(err),
			)
		}
		if !exists {
			log.Fatal("finance_bootstrap_failed: required finance table missing - run `go run ./cmd/migrate` from backend/ before starting the server",
				zap.String("phase", "schema_check"),
				zap.String("table", table),
			)
		}
	}

	// Step 2 — bootstrap canonical system accounts. Idempotent.
	log.Info("finance_bootstrap_started",
		zap.Strings("required_tables", requiredTables),
	)

	bootstrap := financeApp.NewSystemAccountBootstrap(db)
	createdCount, err := bootstrap.EnsureSystemAccounts(ctx)
	if err != nil {
		log.Fatal("finance_bootstrap_failed",
			zap.String("phase", "ensure_system_accounts"),
			zap.Error(err),
		)
	}

	log.Info("finance_bootstrap_completed",
		zap.Int("created_count", createdCount),
	)
}

// checkDatabaseSchemaReady checks if core database tables exist
// Returns true if schema is ready, false otherwise
func checkDatabaseSchemaReady(db *database.DB, log *logger.Logger) bool {
	ctx := context.Background()

	// List of core tables that must exist for workers to function safely
	coreTables := []string{
		"orders",
		"outbox",
		"payments",
	}

	for _, table := range coreTables {
		var exists bool
		err := db.Pool().QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public'
				AND table_name = $1
			)
		`, table).Scan(&exists)

		if err != nil {
			log.Warn("Failed to check if table exists",
				zap.String("table", table),
				zap.Error(err),
			)
			return false
		}

		if !exists {
			log.Warn("Core database table not found - run `go run ./cmd/migrate` from backend/ before starting the server",
				zap.String("table", table),
			)
			return false
		}
	}

	return true
}

// initInfrastructure initializes all infrastructure components
func initInfrastructure(cfg *config.Config, log *logger.Logger) (
	*database.DB,
	*pkgRedis.Client,
	*firebase.Client,
	*midtrans.Client,
) {
	// Initialize database
	db, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}

	log.Info("Database migrations are not applied automatically; run `go run ./cmd/migrate` from backend/ before starting the server")

	// Initialize Redis
	redis, err := pkgRedis.NewRedisClient(&cfg.Redis, log)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize Firebase - REQUIRED for authentication
	var fb *firebase.Client
	if cfg.Dev.MockFirebaseAuth {
		log.Warn("Firebase MOCK mode enabled - authentication will bypass Firebase")
		fb = firebase.NewMockClient(log)
	} else {
		if cfg.Firebase.ProjectID == "" {
			log.Fatal("FIREBASE_PROJECT_ID is required - set this environment variable or enable DEV_MOCK_FIREBASE_AUTH")
		}
		if cfg.Firebase.ServiceAccountKeyPath == "" {
			log.Fatal("FIREBASE_SERVICE_ACCOUNT_KEY_PATH is required - set this environment variable or enable DEV_MOCK_FIREBASE_AUTH")
		}

		fb, err = firebase.NewFirebaseClient(&cfg.Firebase, log)
		if err != nil {
			log.Fatal("Failed to initialize Firebase Admin SDK", zap.Error(err))
		}
		log.Info("Firebase Admin SDK initialized successfully", zap.String("project_id", cfg.Firebase.ProjectID))
	}

	// Initialize Midtrans client
	mt := midtrans.NewClient(&cfg.Midtrans, log)
	log.Info("Midtrans provider configured",
		zap.String("environment", cfg.Midtrans.Environment),
		zap.Bool("notification_url_configured", cfg.Midtrans.NotificationURL != ""),
		zap.Bool("is_production", mt.IsProduction()),
	)

	return db, redis, fb, mt
}

// validateMidtransConfig enforces the STEP B fail-fast contract for the Midtrans
// provider. The simulator stub has been removed; the server must refuse to boot
// rather than silently fall back to a fake URL.
//
// Rules:
//   - Environment must be "sandbox" or "production".
//   - Production is FORBIDDEN until the production-unlock follow-up lands.
//   - ServerKey must be non-empty.
//   - In sandbox, ServerKey/ClientKey should carry "SB-Mid-server-"/"SB-Mid-client-"
//     prefixes (warn-only — keys printed by Midtrans dashboard always carry them, but
//     test fixtures may differ; we don't want to break local dev that's already wired).
func validateMidtransConfig(cfg *config.Config) error {
	env := cfg.Midtrans.Environment
	switch env {
	case "sandbox":
		// allowed
	case "production":
		return fmt.Errorf("MIDTRANS_ENVIRONMENT=production is forbidden in this build (sandbox-only activation)")
	case "":
		return fmt.Errorf("MIDTRANS_ENVIRONMENT is required (got empty); set to \"sandbox\"")
	default:
		return fmt.Errorf("MIDTRANS_ENVIRONMENT must be \"sandbox\" or \"production\"; got %q", env)
	}

	if cfg.Midtrans.ServerKey == "" {
		return fmt.Errorf("MIDTRANS_SERVER_KEY is required for sandbox payment activation")
	}
	if cfg.Midtrans.ClientKey == "" {
		return fmt.Errorf("MIDTRANS_CLIENT_KEY is required for sandbox payment activation")
	}

	// Soft prefix check — print, don't fail.
	if env == "sandbox" {
		if !strings.HasPrefix(cfg.Midtrans.ServerKey, "SB-Mid-server-") {
			fmt.Println("WARNING: MIDTRANS_SERVER_KEY does not carry the 'SB-Mid-server-' prefix; verify it is a sandbox key.")
		}
		if !strings.HasPrefix(cfg.Midtrans.ClientKey, "SB-Mid-client-") {
			fmt.Println("WARNING: MIDTRANS_CLIENT_KEY does not carry the 'SB-Mid-client-' prefix; verify it is a sandbox key.")
		}
	}

	return nil
}

// setupRouter creates and configures the Gin router
func setupRouter(cfg *config.Config, log *logger.Logger) (*gin.Engine, *middleware.ManagedRateLimiter) {
	gin.SetMode(cfg.Server.GinMode)

	router := gin.New()

	// Add middleware in order:
	// 1. Recovery - must be first
	// 2. Request ID - must be before logger for tracing
	// 3. Logger - uses request_id from context
	// 4. CORS
	// 5. Rate limiting (optional)
	router.Use(gin.Recovery())
	router.Use(middleware.RequestIDMiddleware())
	router.Use(LoggerMiddleware(log))
	router.Use(CORSMiddleware(cfg))

	// Add rate limiting middleware
	var rateLimiter *middleware.ManagedRateLimiter
	if cfg.RateLimit.Enabled {
		log.Info("Rate limiting enabled",
			zap.Float64("requests_per_second", cfg.RateLimit.RequestsPerSecond),
			zap.Int("burst", cfg.RateLimit.Burst),
		)
		rateLimiter = middleware.RateLimitMiddlewareWithConfig(middleware.RateLimitConfig{
			RequestsPerSecond: cfg.RateLimit.RequestsPerSecond,
			Burst:             cfg.RateLimit.Burst,
			Enabled:           cfg.RateLimit.Enabled,
		})
		router.Use(rateLimiter.Middleware())
	} else {
		log.Warn("Rate limiting disabled - not recommended for production")
		rateLimiter = middleware.RateLimitMiddlewareWithConfig(middleware.RateLimitConfig{
			Enabled: false,
		})
	}

	return router, rateLimiter
}

// startServer starts the HTTP server with graceful shutdown
func startServer(appCtx context.Context, cfg *config.Config, router *gin.Engine, deps *serverboot.Dependencies, rateLimiter *middleware.ManagedRateLimiter, log *logger.Logger) {
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Server listening", zap.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server.
	<-appCtx.Done()

	log.Info("Shutting down server...")

	// ===== WORKER SHUTDOWN =====
	// Shutdown all workers in dependency order

	// Payment expiry worker shutdown
	if deps.PaymentExpiryWorker != nil && deps.PaymentExpiryWorker.IsRunning() {
		deps.PaymentExpiryWorker.Stop()
		log.Info("Payment expiry worker stopped")
	}

	// Reconciliation worker shutdown
	if deps.ReconciliationWorker != nil && deps.ReconciliationWorker.IsRunning() {
		deps.ReconciliationWorker.Stop()
		log.Info("Reconciliation worker stopped")
	}

	// Order auto-complete worker shutdown
	if deps.OrderAutoCompleteWorker != nil && deps.OrderAutoCompleteWorker.IsRunning() {
		deps.OrderAutoCompleteWorker.Stop()
		log.Info("Order auto-complete worker stopped")
	}

	// Order overdue reminder worker shutdown
	if deps.OrderOverdueReminderWorker != nil && deps.OrderOverdueReminderWorker.IsRunning() {
		deps.OrderOverdueReminderWorker.Stop()
		log.Info("Order overdue reminder worker stopped")
	}

	// Dispute timeout worker shutdown (DISPUTE HARDENING - DEADLOCK PREVENTION)
	if deps.DisputeTimeoutWorker != nil && deps.DisputeTimeoutWorker.IsRunning() {
		deps.DisputeTimeoutWorker.Stop()
		log.Info("Dispute timeout worker stopped")
	}

	// Outbox worker shutdown
	if deps.OutboxWorker != nil && deps.OutboxWorker.IsRunning() {
		deps.OutboxWorker.Stop()
		log.Info("Outbox worker stopped")
	}

	// Projection worker shutdown
	if deps.ProjectionWorker != nil && deps.ProjectionWorker.IsRunning() {
		deps.ProjectionWorker.Stop()
		log.Info("Projection worker stopped")
	}

	// Auction start worker shutdown
	if deps.AuctionStartWorker != nil && deps.AuctionStartWorker.IsRunning() {
		deps.AuctionStartWorker.Stop()
		log.Info("Auction start worker stopped")
	}

	// Auction end worker shutdown
	if deps.AuctionEndWorker != nil && deps.AuctionEndWorker.IsRunning() {
		deps.AuctionEndWorker.Stop()
		log.Info("Auction end worker stopped")
	}

	// System monitoring worker shutdown
	if deps.SystemMonitoringWorker != nil && deps.SystemMonitoringWorker.IsRunning() {
		deps.SystemMonitoringWorker.Stop()
		log.Info("System monitoring worker stopped")
	}

	// Escrow integrity worker shutdown (ESCROW RECONCILIATION — shadow rollout)
	if deps.EscrowIntegrityWorker != nil && deps.EscrowIntegrityWorker.IsRunning() {
		deps.EscrowIntegrityWorker.Stop()
		log.Info("Escrow integrity worker stopped")
	}

	// Total money invariant worker shutdown (TOTAL MONEY INVARIANT — shadow rollout)
	if deps.TotalMoneyInvariantWorker != nil && deps.TotalMoneyInvariantWorker.IsRunning() {
		deps.TotalMoneyInvariantWorker.Stop()
		log.Info("Total money invariant worker stopped")
	}

	// ===== END WORKER SHUTDOWN =====

	// Stop rate limiter cleanup goroutine
	if rateLimiter != nil {
		rateLimiter.Stop()
		log.Info("Rate limiter stopped")
	}

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}
