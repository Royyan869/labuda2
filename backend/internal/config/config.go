package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Firebase  FirebaseConfig
	JWT       JWTConfig
	AWS       AWSConfig
	Logging   LoggingConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
	App       AppConfig
	Midtrans  MidtransConfig
	Dev       DevConfig
	Pricing   PricingConfig // P8: Price Authority configuration
	Outbox    OutboxConfig  // Outbox archival configuration
	// CommissionPercent removed - now managed by PlatformConfigService (see platformconfig domain)
	InternalAPIKey string             // Internal API key for service-to-service communication
	Payout         PayoutConfig       // Payout gateway configuration for sandbox integration
	FeatureFlags   FeatureFlagsConfig // Feature flags for controlled rollout/disable
}

type ServerConfig struct {
	Port         string
	GinMode      string
	Env          string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxConnections  int
	MaxIdle         int
	ConnMaxLifetime time.Duration
	AutoMigrate     bool
	// Test database configuration (used when TEST_MODE=true)
	TestName     string
	TestHost     string
	TestPort     string
	TestUser     string
	TestPassword string
	TestSSLMode  string
}

type RedisConfig struct {
	Host       string
	Port       string
	Password   string
	DB         int
	MaxRetries int
	PoolSize   int
}

type FirebaseConfig struct {
	ProjectID             string
	ServiceAccountKeyPath string
}

type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	S3BucketName    string
	S3BucketRegion  string
	// CDNBaseURL is the optional CloudFront / CDN prefix returned as public_url
	// in media upload responses. When empty, raw S3 HTTPS URLs are used.
	// Example: "https://d358tu61i1wrtt.cloudfront.net"
	CDNBaseURL string
}

type LoggingConfig struct {
	Level  string
	Format string
	Output string
}

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int
}

type AppConfig struct {
	Name        string
	Version     string
	FrontendURL string
}

// DevConfig holds development-mode settings for testing
type DevConfig struct {
	AutoApproveVerification bool // Auto-approve KYC/phone verification in dev mode
	SkipPaymentGateway      bool // Skip actual payment gateway calls in dev mode
	MockFirebaseAuth        bool // Use mock Firebase auth for testing
}

type RateLimitConfig struct {
	Enabled           bool
	RequestsPerSecond float64
	Burst             int
}

type MidtransConfig struct {
	ServerKey       string
	ClientKey       string
	Environment     string // "sandbox" or "production"
	NotificationURL string
}

// PricingConfig holds P8 Price Authority settings
// P8 Phase 3: Legacy mode flags removed - strict mode is always ON
type PricingConfig struct {
	// P8 Phase 3: Strict price authority is always enabled
	// - All payments MUST have price_snapshot_id
	// - Client-provided amounts are rejected with 400
	// - No legacy mode support
}

// OutboxConfig holds outbox archival configuration
type OutboxConfig struct {
	// RetentionDays is how many days to keep succeeded events before archiving
	RetentionDays int
	// ArchiveBatchSize is the max number of events to archive per batch
	ArchiveBatchSize int
}

// PayoutConfig holds payout gateway configuration.
//
// PAYMENT PROVIDER ARCHITECTURE:
// ┌─────────────────────────────────────────────────────────────────────────────┐
// │ INCOMING PAYMENTS (buyer → platform): Midtrans Core/Snap                    │
// │ PAYOUTS (platform → seller): Midtrans Iris (separate product + credentials) │
// └─────────────────────────────────────────────────────────────────────────────┘
//
// Midtrans Iris credentials are SEPARATE from Core API / Snap keys.
// Mid-server-* keys are rejected by Iris with HTTP 401 (proven TASK 58).
// Iris keys are obtained from the Iris dashboard independently.
//
// Production payout is DISABLED by default. Requires PAYOUT_ENABLE_PRODUCTION=true.
type PayoutConfig struct {
	// Environment is "sandbox" or "production"
	Environment string

	// EnableProduction enables real production payouts. Default: FALSE.
	EnableProduction bool

	// GatewayProvider selects the payout backend.
	// VALID VALUES: "sandbox" (internal mock), "midtrans_payout" (Midtrans Iris)
	GatewayProvider string

	// SecretKey is used for webhook signature verification
	SecretKey string

	// IrisOperatorKey is the Midtrans Iris operator key for creating payouts.
	// Obtained from the Iris dashboard. NOT the same as MIDTRANS_SERVER_KEY.
	// Required when GatewayProvider=midtrans_payout.
	// If missing, payout gateway initialization fails with an explicit error.
	IrisOperatorKey string

	// IrisApproverKey is the Midtrans Iris approver key for approving queued payouts.
	// Required if Iris account does not have auto-approve enabled.
	// Optional for sandbox testing with auto-approve.
	IrisApproverKey string

	// WebhookURL is the endpoint URL for Iris disbursement callbacks
	WebhookURL string

	// StuckThresholdMinutes is how long a payout can be in SUBMITTED/SETTLING before flagged as stuck
	StuckThresholdMinutes int

	// ReconciliationIntervalMinutes is how often to run reconciliation checks
	ReconciliationIntervalMinutes int

	// EnableWorker controls whether the payout worker goroutine is started. Default FALSE.
	EnableWorker bool

	// EnableReconciliation controls whether the read-only reconciliation worker starts.
	EnableReconciliation bool

	// EnablePilotMode: when true, only whitelisted sellers can process payouts
	EnablePilotMode bool

	// PilotWhitelist is a comma-separated list of seller IDs allowed in pilot mode
	PilotWhitelist string
}

// FeatureFlagsConfig holds feature flags for controlled rollout/disable
type FeatureFlagsConfig struct {
	// UseUnifiedWithdrawal asserts that the canonical unified withdrawal path is active.
	// Default TRUE. Setting to false is forbidden in staging/production — startup guard enforces this.
	// The route already unconditionally calls RequestWithdrawalUnifiedTx; this flag provides an
	// explicit config-level signal and a staging fail-fast if someone tries to disable it.
	UseUnifiedWithdrawal bool

	// GatewayRefundInitiateEnabled controls the admin-only gateway refund
	// trigger endpoint added in TASK 34 / Phase 2a.
	//
	// When false (default), the endpoint returns FEATURE_DISABLED. When true,
	// admins with the 'finance.refund.gateway.initiate' capability may
	// dispatch a Midtrans refund for an existing refund row via
	// POST /admin/refunds/:refund_id/gateway/initiate.
	//
	// NOTE: This flag only gates the admin-manual trigger endpoint.
	// The full gateway refund pipeline — webhook ack, canonical ledger
	// reversal (Phase 2B), escrow flip, order status sync, and
	// coins.refund_required emission — runs unconditionally for all
	// system-initiated refund paths (dispute resolution, timeout, expiry).
	// Phase 2B is fully wired; no separate flag gates the financial mutation.
	//
	// Safe to enable in staging/dev. Production: keep false until admin
	// refund UX is validated in staging.
	GatewayRefundInitiateEnabled bool

	// SearchContentEvaluatorMode controls the /search/content evaluator
	// integration's operating mode (Batch 3A prerequisite for Batch 3B
	// authority promotion). Valid values: "shadow" (default) or "enforce".
	//
	// Env var: SEARCH_CONTENT_EVALUATOR_MODE. Any unset / empty / invalid
	// value is normalized to "shadow" at load time — enforce mode is
	// opt-in only, never reachable by misconfiguration.
	//
	// In this batch the value drives only the enforce_mode_total telemetry
	// label and the shadow runner's WithMode label. The response handler
	// does NOT yet consume this mode; Batch 3B wires the synchronous
	// enforcement path on /search/content alone.
	SearchContentEvaluatorMode string

	// FeedEvaluatorMode controls the /feed evaluator integration's
	// operating mode. Valid values: "enforce" (default) or "shadow".
	//
	// Env var: FEED_EVALUATOR_MODE. Rollback: set to "shadow" to
	// disable enforcement and revert to shadow-only observability.
	// Any unrecognized value is normalized to "shadow" at load time
	// via evaluator.NormalizeFeedEvaluatorMode.
	//
	// In enforce mode the handler runs a synchronous further-restrict
	// pass over the legacy SQL result BEFORE serialization. The shadow
	// runner continues to fire fire-and-forget AFTER the response is
	// written, with the ORIGINAL (pre-filter) item set, so existing
	// shadow telemetry remains comparable across the flip. UNKNOWN
	// items fail OPEN (kept). See evaluator/feed_enforce.go for the
	// per-decision semantics.
	FeedEvaluatorMode string

	// ContentDetailEvaluatorMode controls the /contents/:id evaluator
	// integration's operating mode (D1 convergence). Valid values:
	// "shadow" (default) or "enforce".
	//
	// Env var: CONTENT_DETAIL_EVALUATOR_MODE. Any unset / empty / invalid
	// value is normalized to "shadow" at load time via
	// evaluator.NormalizeContentDetailEvaluatorMode — enforce mode is
	// opt-in only.
	//
	// In enforce mode the handler runs a synchronous fail-CLOSED pass
	// AFTER the legacy gate. Any non-ALLOW evaluator decision (DENY /
	// TOMBSTONE / REDACT / UNKNOWN) converts the response to HTTP 404.
	// UNKNOWN fails CLOSED — doctrine §8.5. See
	// evaluator/content_detail_enforce.go for the per-decision semantics.
	ContentDetailEvaluatorMode string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if exists (ignore error in production)
	_ = godotenv.Load()

	config := &Config{
		Server: ServerConfig{
			Port:    getEnv("PORT", "8080"),
			GinMode: getEnv("GIN_MODE", "debug"),
			// FAIL-CLOSED: unset ENV must never silently behave as
			// development (which mounts dev-only admin/debug routes).
			// ENV must be explicit; an unset environment defaults to
			// "production" so it is caught by ValidateProductionSafety
			// instead of quietly running with development conveniences.
			Env:          getEnv("ENV", "production"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 30) * time.Second,
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 30) * time.Second,
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "labuda"),
			Password:        getEnv("DB_PASSWORD", "labuda123"),
			Name:            getEnv("DB_NAME", ""), // No default - MUST be set
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxConnections:  getIntEnv("DB_MAX_CONNECTIONS", 200),
			MaxIdle:         getIntEnv("DB_MAX_IDLE_CONNECTIONS", 40),
			ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 1800) * time.Second,
			AutoMigrate:     getBoolEnv("AUTO_MIGRATE", false), // Deprecated compatibility flag; ignored by runtime
			// Test database defaults to same host with different database name
			TestName:     getEnv("DB_TEST_NAME", "labuda_test"),
			TestHost:     getEnv("DB_TEST_HOST", ""),
			TestPort:     getEnv("DB_TEST_PORT", ""),
			TestUser:     getEnv("DB_TEST_USER", ""),
			TestPassword: getEnv("DB_TEST_PASSWORD", ""),
			TestSSLMode:  getEnv("DB_TEST_SSLMODE", ""),
		},
		Redis: RedisConfig{
			Host:       getEnv("REDIS_HOST", "localhost"),
			Port:       getEnv("REDIS_PORT", "6379"),
			Password:   getEnv("REDIS_PASSWORD", ""),
			DB:         getIntEnv("REDIS_DB", 0),
			MaxRetries: getIntEnv("REDIS_MAX_RETRIES", 3),
			PoolSize:   getIntEnv("REDIS_POOL_SIZE", 10),
		},
		Firebase: FirebaseConfig{
			ProjectID:             getEnv("FIREBASE_PROJECT_ID", ""),
			ServiceAccountKeyPath: getEnv("FIREBASE_SERVICE_ACCOUNT_KEY_PATH", "./configs/firebase-service-account.json"),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "change-me-in-production"),
			Expiration: getDurationEnv("JWT_EXPIRATION", 86400) * time.Second,
		},
		AWS: AWSConfig{
			Region:          getEnv("AWS_REGION", "ap-southeast-1"),
			AccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
			S3BucketName:    getEnv("S3_BUCKET_NAME", ""),
			S3BucketRegion:  getEnv("S3_BUCKET_REGION", "ap-southeast-1"),
			CDNBaseURL:      getEnv("CDN_BASE_URL", ""),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "debug"),
			Format: getEnv("LOG_FORMAT", "json"),
			Output: getEnv("LOG_OUTPUT", "stdout"),
		},
		CORS: CORSConfig{
			AllowedOrigins: getSliceEnv("CORS_ALLOWED_ORIGINS", []string{"*"}),
			AllowedMethods: getSliceEnv("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
			AllowedHeaders: getSliceEnv("CORS_ALLOWED_HEADERS", []string{"Origin", "Content-Type", "Accept", "Authorization"}),
			MaxAge:         getIntEnv("CORS_MAX_AGE", 86400),
		},
		RateLimit: RateLimitConfig{
			Enabled:           getBoolEnv("RATE_LIMIT_ENABLED", true),
			RequestsPerSecond: getFloatEnv("RATE_LIMIT_REQUESTS_PER_SECOND", 100),
			Burst:             getIntEnv("RATE_LIMIT_BURST", 200),
		},
		App: AppConfig{
			Name:        getEnv("APP_NAME", "Labuda Backend"),
			Version:     getEnv("APP_VERSION", "0.1.0"),
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		},
		Midtrans: MidtransConfig{
			ServerKey:       getEnv("MIDTRANS_SERVER_KEY", ""),
			ClientKey:       getEnv("MIDTRANS_CLIENT_KEY", ""),
			Environment:     getEnv("MIDTRANS_ENVIRONMENT", "sandbox"),
			NotificationURL: getEnv("MIDTRANS_NOTIFICATION_URL", ""),
		},
		Dev: DevConfig{
			AutoApproveVerification: getBoolEnv("DEV_AUTO_APPROVE_VERIFICATION", false),
			SkipPaymentGateway:      getBoolEnv("DEV_SKIP_PAYMENT_GATEWAY", false),
			MockFirebaseAuth:        getBoolEnv("DEV_MOCK_FIREBASE_AUTH", false),
		},
		Pricing: PricingConfig{
			// P8 Phase 3: Strict price authority is always enabled
			// Environment variables PRICING_STRICT_MODE and PRICING_LEGACY_MODE_ALLOWED are ignored
		},
		Outbox: OutboxConfig{
			RetentionDays:    getIntEnv("OUTBOX_RETENTION_DAYS", 30),      // Archive events older than 30 days
			ArchiveBatchSize: getIntEnv("OUTBOX_ARCHIVE_BATCH_SIZE", 500), // Max 500 events per batch
		},
		// CommissionPercent removed - now managed by PlatformConfigService (see platformconfig domain)
		InternalAPIKey: getEnv("INTERNAL_API_KEY", ""),
		Payout: PayoutConfig{
			Environment:      getEnv("PAYOUT_ENVIRONMENT", "sandbox"),
			EnableProduction: getBoolEnv("PAYOUT_ENABLE_PRODUCTION", false),
			GatewayProvider:  getEnv("PAYOUT_GATEWAY_PROVIDER", "sandbox"),
			SecretKey:        getEnv("PAYOUT_SECRET_KEY", ""),
			// Midtrans Iris credentials — separate from Core API / Snap keys (TASK 59)
			IrisOperatorKey:               getEnv("MIDTRANS_IRIS_OPERATOR_KEY", ""),
			IrisApproverKey:               getEnv("MIDTRANS_IRIS_APPROVER_KEY", ""),
			WebhookURL:                    getEnv("PAYOUT_WEBHOOK_URL", ""),
			StuckThresholdMinutes:         getIntEnv("PAYOUT_STUCK_THRESHOLD_MINUTES", 30),
			ReconciliationIntervalMinutes: getIntEnv("PAYOUT_RECONCILIATION_INTERVAL_MINUTES", 10),
			EnableWorker:                  getBoolEnv("PAYOUT_ENABLE_WORKER", false),
			EnableReconciliation:          getBoolEnv("PAYOUT_ENABLE_RECONCILIATION", false),
			EnablePilotMode:               getBoolEnv("PAYOUT_ENABLE_PILOT_MODE", true),
			PilotWhitelist:                getEnv("PAYOUT_PILOT_WHITELIST", ""),
		},
		FeatureFlags: FeatureFlagsConfig{
			// TASK 34 / Phase 2a: gateway refund initiate endpoint.
			// Default FALSE — must be explicitly enabled per environment.
			GatewayRefundInitiateEnabled: getBoolEnv("ENABLE_GATEWAY_REFUND_PHASE2", false),
			// Canonical unified withdrawal path assertion. Default TRUE.
			// Must remain true in staging/production — enforced by ValidateStagingActivation.
			UseUnifiedWithdrawal: getBoolEnv("USE_UNIFIED_WITHDRAWAL", true),
			// BATCH 3A: /search/content evaluator integration mode. Parsed
			// raw here; normalization to the canonical {"shadow","enforce"}
			// set happens in the consumer via
			// evaluator.NormalizeSearchContentAdapterMode so unset / empty
			// / invalid values fall safe to "shadow".
			SearchContentEvaluatorMode: getEnv("SEARCH_CONTENT_EVALUATOR_MODE", "enforce"),
			// /feed evaluator integration mode. Default "enforce" — all
			// three evaluator surfaces now enforce. Rollback:
			// FEED_EVALUATOR_MODE=shadow. Normalization via
			// evaluator.NormalizeFeedEvaluatorMode; unrecognized values
			// fall safe to "shadow".
			FeedEvaluatorMode: getEnv("FEED_EVALUATOR_MODE", "enforce"),
			// D1: /contents/:id evaluator integration mode. Parsed raw
			// here; normalization to the canonical {"shadow","enforce"}
			// set happens in the consumer via
			// evaluator.NormalizeContentDetailEvaluatorMode so unset /
			// empty / invalid values fall safe to "shadow".
			ContentDetailEvaluatorMode: getEnv("CONTENT_DETAIL_EVALUATOR_MODE", "enforce"),
		},
	}

	// FAIL-FAST VALIDATION: Database name is required
	if config.Database.Name == "" {
		return nil, fmt.Errorf("DB_NAME is required but not set. Please set DB_NAME environment variable")
	}

	return config, nil
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "development"
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}

// IsStaging returns true if running in staging environment
func (c *Config) IsStaging() bool {
	return c.Server.Env == "staging"
}

// ValidateStagingActivation enforces staging-safe invariants.
// Must be called after Load() and ValidateProductionSafety().
// Only runs when ENV=staging; no-ops in development and production.
//
// Guards:
//   - Unified withdrawal must be enabled (USE_UNIFIED_WITHDRAWAL=true)
//   - Payout worker must NOT start with pilot mode on but an empty whitelist
//   - Payout worker must NOT start without pilot mode in staging
func (c *Config) ValidateStagingActivation() error {
	if !c.IsStaging() {
		return nil
	}

	// Guard 1: canonical withdrawal path must be active in staging
	if !c.FeatureFlags.UseUnifiedWithdrawal {
		return fmt.Errorf(
			"STAGING ACTIVATION BLOCKED: USE_UNIFIED_WITHDRAWAL=false. " +
				"The legacy withdrawal path is not permitted in staging. " +
				"Set USE_UNIFIED_WITHDRAWAL=true or leave unset (defaults to true)",
		)
	}

	// Guard 2: payout worker must have pilot mode enabled in staging
	if c.Payout.EnableWorker && !c.Payout.EnablePilotMode {
		return fmt.Errorf(
			"STAGING ACTIVATION BLOCKED: PAYOUT_ENABLE_WORKER=true but PAYOUT_ENABLE_PILOT_MODE=false. " +
				"Payout worker must run in pilot mode in staging to limit blast radius",
		)
	}

	// Guard 3: pilot mode must have a non-empty whitelist
	if c.Payout.EnableWorker && c.Payout.EnablePilotMode && strings.TrimSpace(c.Payout.PilotWhitelist) == "" {
		return fmt.Errorf(
			"STAGING ACTIVATION BLOCKED: PAYOUT_ENABLE_WORKER=true and PAYOUT_ENABLE_PILOT_MODE=true " +
				"but PAYOUT_PILOT_WHITELIST is empty. Populate PAYOUT_PILOT_WHITELIST with comma-separated seller UUIDs",
		)
	}

	return nil
}

// GetDSN returns the PostgreSQL connection string (key=value format for GORM/sqlx)
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// GetDatabaseURL returns the PostgreSQL connection URL (URL format for golang-migrate)
func (c *DatabaseConfig) GetDatabaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

// GetTestDSN returns the PostgreSQL connection string for TEST database
// Falls back to main DB config if test-specific values are not set
func (c *DatabaseConfig) GetTestDSN() string {
	host := c.TestHost
	if host == "" {
		host = c.Host
	}
	port := c.TestPort
	if port == "" {
		port = c.Port
	}
	user := c.TestUser
	if user == "" {
		user = c.User
	}
	password := c.TestPassword
	if password == "" {
		password = c.Password
	}
	sslmode := c.TestSSLMode
	if sslmode == "" {
		sslmode = c.SSLMode
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, c.TestName, sslmode,
	)
}

// GetTestDatabaseURL returns the PostgreSQL connection URL for TEST database (for golang-migrate)
// Falls back to main DB config if test-specific values are not set
func (c *DatabaseConfig) GetTestDatabaseURL() string {
	host := c.TestHost
	if host == "" {
		host = c.Host
	}
	port := c.TestPort
	if port == "" {
		port = c.Port
	}
	user := c.TestUser
	if user == "" {
		user = c.User
	}
	password := c.TestPassword
	if password == "" {
		password = c.Password
	}
	sslmode := c.TestSSLMode
	if sslmode == "" {
		sslmode = c.SSLMode
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, c.TestName, sslmode,
	)
}

// GetRedisAddr returns the Redis address
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// ValidateProductionSafety validates critical security settings for production.
// PANICS if unsafe configuration is detected in production mode.
// This must be called immediately after config.Load() in main().
//
// FAIL-CLOSED: this also validates ENV itself, unconditionally (not just
// when Env=="production"). An invalid or empty environment string is never
// safe to boot with — it must not silently fall back to development or
// production behavior. Load() defaults an unset ENV to "production", so
// this only fires for a genuinely garbled ENV value (e.g. a typo).
func (c *Config) ValidateProductionSafety() {
	switch c.Server.Env {
	case "development", "staging", "production":
		// valid — continue.
	default:
		panic("CONFIG ERROR: ENV must be 'development', 'staging', or 'production'. Got: '" + c.Server.Env + "'. Set ENV explicitly.")
	}

	if c.Server.Env != "production" {
		return
	}

	// STEP 1: Validate DB SSL Mode
	if c.Database.SSLMode != "require" && c.Database.SSLMode != "verify-full" && c.Database.SSLMode != "verify-ca" {
		panic("CONFIG ERROR: DB_SSLMODE must be 'require', 'verify-ca', or 'verify-full' in production. Current: " + c.Database.SSLMode)
	}

	// STEP 2: Validate Gin Mode (must be release, not debug or test)
	if c.Server.GinMode != "release" {
		panic("CONFIG ERROR: GIN_MODE must be 'release' in production. Current: " + c.Server.GinMode)
	}

	// STEP 3: Validate CORS - no wildcard allowed in production
	for _, origin := range c.CORS.AllowedOrigins {
		if origin == "*" {
			panic("CONFIG ERROR: CORS wildcard (*) not allowed in production. Set specific origins via CORS_ALLOWED_ORIGINS")
		}
	}

	// STEP 3.5: Validate Dev flags — must never be active in production
	if c.Dev.MockFirebaseAuth {
		panic("CONFIG ERROR: DEV_MOCK_FIREBASE_AUTH must not be enabled in production. This bypasses all Firebase authentication.")
	}
	if c.Dev.AutoApproveVerification {
		panic("CONFIG ERROR: DEV_AUTO_APPROVE_VERIFICATION must not be enabled in production. This bypasses KYC review.")
	}
	if c.Dev.SkipPaymentGateway {
		panic("CONFIG ERROR: DEV_SKIP_PAYMENT_GATEWAY must not be enabled in production. This skips real payment processing.")
	}

	// STEP 4: Validate Payout Configuration
	// Production payouts require explicit enable flag AND webhook secret
	if c.Payout.Environment == "production" && !c.Payout.EnableProduction {
		panic("CONFIG ERROR: PAYOUT_ENVIRONMENT=production requires PAYOUT_ENABLE_PRODUCTION=true for safety. Production payouts are disabled by default.")
	}
	if c.Payout.Environment == "production" && c.Payout.SecretKey == "" {
		panic("CONFIG ERROR: PAYOUT_SECRET_KEY must be set in production for webhook signature verification")
	}
}

// ValidatePayoutGatewayProvider validates the payout gateway provider configuration.
// This must be called immediately after config.Load() in main().
//
// CRITICAL: This FAILS FAST on unknown provider names to prevent silent
// fallback to sandbox mode which could lead to production payouts running
// in sandbox mode unintentionally.
//
// PASS_18S: an EMPTY provider is only allowed to silently default to
// "sandbox" in development. In staging/production, an unset
// PAYOUT_GATEWAY_PROVIDER now fails fast — a production-like deployment
// must explicitly choose its payout gateway, never fall into the fake
// AlwaysSucceed sandbox gateway by omission.
func (c *Config) ValidatePayoutGatewayProvider() {
	provider := strings.ToLower(strings.TrimSpace(c.Payout.GatewayProvider))

	// Valid providers
	// "sandbox" - mock/simulation gateway for testing
	// "midtrans_payout" - real Midtrans disbursement API
	validProviders := map[string]bool{
		"sandbox":         true,
		"midtrans_payout": true,
		// Future: "xendit": true,
		// Future: "doku": true,
	}

	if provider == "" {
		if c.Server.Env != "development" {
			// FAIL FAST - a production-like environment must never silently
			// fall back to the fake sandbox gateway just because the
			// provider was left unset.
			panic("CONFIG ERROR: PAYOUT_GATEWAY_PROVIDER is unset in a non-development environment (ENV=" +
				c.Server.Env + "). An explicit provider (sandbox or midtrans_payout) is required outside " +
				"development to prevent payout extraction silently running against the fake sandbox gateway.")
		}
		// Empty provider defaults to sandbox in development only.
		c.Payout.GatewayProvider = "sandbox"
		return
	}

	if !validProviders[provider] {
		// FAIL FAST - do not silently default to sandbox
		// This prevents production from accidentally running in sandbox mode
		panic(fmt.Sprintf("CONFIG ERROR: Unknown PAYOUT_GATEWAY_PROVIDER '%s'. Valid values: sandbox, midtrans_payout. "+
			"Unknown provider configuration will not be silently defaulted to prevent production accidents.",
			c.Payout.GatewayProvider))
	}

	// Provider is valid
	c.Payout.GatewayProvider = provider
}

// IsPayoutEnabled returns true if payout processing is enabled.
// Production requires both Environment=production AND EnableProduction=true.
func (c *Config) IsPayoutEnabled() bool {
	if c.Payout.Environment == "production" {
		return c.Payout.EnableProduction
	}
	return c.Payout.Environment == "sandbox"
}

// IsPayoutProduction returns true if running in production payout mode.
func (c *Config) IsPayoutProduction() bool {
	return c.Payout.Environment == "production" && c.Payout.EnableProduction
}

// IsPayoutSandbox returns true if running in sandbox payout mode.
func (c *Config) IsPayoutSandbox() bool {
	return c.Payout.Environment == "sandbox" || c.Payout.GatewayProvider == "sandbox"
}

// reconciliationProvidesCompletion is FALSE and must stay false until
// PayoutReconciliationService genuinely queries the gateway and can move a
// payout to a terminal state.
//
// PASS_18S evidence: PayoutReconciliationService.QueryGatewayStatus
// (internal/finance/worker/payout_reconciliation.go) is a stub — it returns
// a hardcoded "gateway_status": "UNKNOWN" / "mode": "sandbox_query" without
// calling any real gateway API, and MarkPayoutStuck explicitly does not
// transition status ("we don't have a dedicated 'stuck' status... leave it
// as-is for manual intervention"). Enabling PAYOUT_ENABLE_RECONCILIATION
// today only produces a periodic log report — it cannot resolve a stuck
// payout. Counting it as a completion path here would fake safety that does
// not exist. Flip this to true only once that worker is rewritten to
// genuinely poll the gateway and finalize state.
const reconciliationProvidesCompletion = false

// PayoutCompletionSafety describes whether a genuine completion path exists
// for the currently configured payout worker (PASS_18S). A "completion
// path" is a mechanism by which a submitted payout can reach a confirmed
// terminal state (settled or failed) without manual DB intervention: either
// a verifiable signed webhook, or a reconciliation worker that actually
// polls the gateway (see reconciliationProvidesCompletion above).
type PayoutCompletionSafety struct {
	PayoutWorkerEnabled         bool   `json:"payout_worker_enabled"`
	PayoutReconciliationEnabled bool   `json:"payout_reconciliation_enabled"`
	PayoutWebhookConfigured     bool   `json:"payout_webhook_configured"`
	PayoutGatewayProvider       string `json:"payout_gateway_provider"`
	PayoutEnvironment           string `json:"payout_environment"`
	CompletionPathAvailable     bool   `json:"completion_path_available"`
	SafeForRealMoney            bool   `json:"safe_for_real_money"`
	Degraded                    bool   `json:"degraded"`
	Reason                      string `json:"reason"`
}

// EvaluatePayoutCompletionSafety computes the current payout completion-loop
// safety state. It is derived entirely from Config fields already loaded
// from the environment, so it can never drift from what dependencies.go
// actually wires up, and can be unit tested without touching the gateway or
// database.
func (c *Config) EvaluatePayoutCompletionSafety() PayoutCompletionSafety {
	webhookConfigured := strings.TrimSpace(c.Payout.SecretKey) != ""
	reconciliationEnabled := c.Payout.EnableReconciliation
	completionPathAvailable := webhookConfigured || (reconciliationEnabled && reconciliationProvidesCompletion)

	safety := PayoutCompletionSafety{
		PayoutWorkerEnabled:         c.Payout.EnableWorker,
		PayoutReconciliationEnabled: reconciliationEnabled,
		PayoutWebhookConfigured:     webhookConfigured,
		PayoutGatewayProvider:       c.Payout.GatewayProvider,
		PayoutEnvironment:           c.Payout.Environment,
		CompletionPathAvailable:     completionPathAvailable,
	}

	if !safety.PayoutWorkerEnabled {
		safety.Reason = "payout worker is disabled — no payout submission loop is running"
		return safety
	}

	if completionPathAvailable {
		safety.SafeForRealMoney = webhookConfigured && c.Payout.GatewayProvider != "sandbox"
		safety.Reason = "payout worker enabled with a configured completion path"
		return safety
	}

	safety.Degraded = true
	safety.Reason = "payout worker is enabled but no completion path is configured " +
		"(PAYOUT_SECRET_KEY is unset and reconciliation is not a functional completion path) " +
		"— payouts can be submitted but may never reach a confirmed terminal state"
	return safety
}

// ValidatePayoutCompletionPath enforces PASS_18S payout completion-loop
// safety in staging/production. No-ops in development (an unsafe payout
// loop there must only degrade readiness, not block boot — see
// EvaluatePayoutCompletionSafety and /health/ready).
//
// Must be called immediately after config.Load() (and after
// ValidatePayoutGatewayProvider, which normalizes GatewayProvider) in
// main().
func (c *Config) ValidatePayoutCompletionPath() error {
	if c.Server.Env == "development" {
		return nil
	}

	safety := c.EvaluatePayoutCompletionSafety()
	if safety.Degraded {
		return fmt.Errorf(
			"PAYOUT COMPLETION LOOP UNSAFE: PAYOUT_ENABLE_WORKER=true in ENV=%s but no completion path is "+
				"configured (PAYOUT_SECRET_KEY unset and reconciliation is not a functional completion path). "+
				"Payouts would be submitted with no way to reach a confirmed terminal state. "+
				"Set PAYOUT_SECRET_KEY (webhook signature verification) or disable PAYOUT_ENABLE_WORKER",
			c.Server.Env,
		)
	}
	return nil
}

// Helper functions

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getIntEnv(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getInt64Env(key string, defaultValue int64) int64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func getDurationEnv(key string, defaultValue int) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return time.Duration(defaultValue)
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return time.Duration(defaultValue)
	}
	return time.Duration(value)
}

func getBoolEnv(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getFloatEnv(key string, defaultValue float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func getSliceEnv(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	// Simple split by comma (can be enhanced with proper parsing)
	var result []string
	for i := 0; i < len(valueStr); {
		j := i
		for j < len(valueStr) && valueStr[j] != ',' {
			j++
		}
		result = append(result, valueStr[i:j])
		i = j + 1
	}
	return result
}


