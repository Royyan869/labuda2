package main

// ⚠️ THIS SCRIPT BYPASSES BUSINESS LOGIC AND EVENT SYSTEM
// ⚠️ IT DOES NOT TRIGGER OUTBOX, EVENTS, OR DOMAIN RULES
// ⚠️ DO NOT USE FOR SYSTEM VALIDATION

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/audit"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/internal/config"
	financeRepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
	ledgerepo "github.com/labuda/backend/internal/finance/repository"
	"github.com/labuda/backend/internal/identity/auth"
	coinsApp "github.com/labuda/backend/internal/incentive/coins/application"
	coinsRepo2 "github.com/labuda/backend/internal/incentive/coins/infrastructure/repository"
	paymentSettlementRepo "github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	platformconfigRepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/logger"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	socialApp "github.com/labuda/backend/internal/social/graph/application"
	likeApp "github.com/labuda/backend/internal/social/like/application"
	likerepo "github.com/labuda/backend/internal/social/like/infrastructure/repository"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// Seeder holds all dependencies for seeding test data
type Seeder struct {
	db                       *database.DB
	log                      *zap.Logger
	cfg                      *config.Config
	contentService           *contentApp.ContentService
	commentService           *contentApp.CommentService
	likeService              *likeApp.Service
	socialService            *socialApp.SocialService
	orderService             *orderApp.OrderService
	paymentSettlementService *paymentSettlementRepo.PaymentSettlementService
	ledgerRepo               ledgerepo.LedgerRepository
	roleChecker              auth.RoleChecker
	accountStatusChecker     auth.AccountStatusChecker
}

// main entry point
func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Validate config has database settings
	if cfg.Database.Host == "" {
		fmt.Printf("ERROR: Database configuration is missing. Please set DATABASE_* environment variables.\n")
		os.Exit(1)
	}

	// Print masked DSN before connecting (for diagnostics)
	maskedDSN := maskDSNPassword(cfg.Database.GetDSN())
	fmt.Printf("Connecting to PostgreSQL: %s\n", maskedDSN)

	// Initialize logger using the same mechanism as core_server
	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Parse command line flags
	cleanMode := len(os.Args) > 1 && os.Args[1] == "--clean"

	if cleanMode {
		log.Info("Running in CLEAN mode - tables will be truncated")
	}

	// Initialize database (passing logger instead of nil)
	db, err := database.NewPostgresDB(&cfg.Database, log)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer database.CloseDB(db, log)

	// Initialize seeder with dependencies
	// Note: initSeeder uses zap.Logger directly, so we unwrap the embedded logger
	seed, err := initSeeder(db, cfg, log.Logger)
	if err != nil {
		log.Fatal("Failed to initialize seeder", zap.Error(err))
	}

	// Run seeding
	if err := seed.Run(cleanMode); err != nil {
		log.Fatal("Seeding failed", zap.Error(err))
	}

	log.Info("Seeding completed successfully")
	printSummary()
}

// initSeeder initializes the seeder with all required dependencies
func initSeeder(db *database.DB, cfg *config.Config, log *zap.Logger) (*Seeder, error) {
	ctx := context.Background()

	// Ensure role column exists in users table
	if err := ensureRoleColumn(ctx, db, log); err != nil {
		log.Warn("Could not verify role column", zap.Error(err))
	}

	// Initialize auth components
	adminAuditLogger := audit.NewAdminAuditLoggerDB(db.Pgx().Pool())
	roleChecker := auth.NewRoleCheckerDB(db.Pgx(), adminAuditLogger)
	accountStatusChecker := auth.NewAccountStatusCheckerDB(db.Pgx())

	// Initialize outbox repository
	outboxRepository := outboxRepo.NewOutboxRepository(db.Pgx())

	// Initialize platform config service
	platformConfigRepo := platformconfigRepo.NewPlatformConfigRepository()
	configService := platformconfigApp.NewConfigService(platformConfigRepo)

	// Initialize stub shipping service
	stubShippingRepo := &stubShippingSetupRepository{}
	stubCoverageRepo := &stubShippingCoverageRepository{}
	stubCityOverrideRepo := &stubCityOverrideRepository{}
	stubProductShippingSetupRepo := &stubProductShippingSetupRepository{}
	shippingService := shippingApp.NewShippingService(
		stubShippingRepo,
		stubCoverageRepo,
		stubCityOverrideRepo,
		stubProductShippingSetupRepo,
	)

	// Initialize repos + like service (constructed ahead of content service
	// so ContentService can receive its dependencies by injection)
	contentRepo := contentrepo.NewContentRepository()
	likeRepo := likerepo.NewLikeRepository()
	likeService := likeApp.NewService(db, contentRepo, likeRepo, outboxRepository, nil, nil, nil) // blockChecker/invariantLogger/scrubber not needed for seed

	// Initialize content service
	contentService := contentApp.NewContentService(
		contentRepo,
		likeRepo,
		roleChecker,
		accountStatusChecker,
		nil, // InvariantLogger - not needed for seed
	)

	// Initialize comment service (repositories are internal to the service)
	commentService := contentApp.NewCommentService(
		nil, // contentRepo - internal to service
		nil, // commentRepo - internal to service
		nil, // listingService - not needed for seed
		outboxRepository,
		nil, // blockChecker - not needed for seed
		nil, // InvariantLogger - not needed for seed
	)

	// Initialize social service
	socialService := socialApp.NewSocialServiceWithDefaults(db, outboxRepository)

	// Initialize order service
	coinsRepo := coinsRepo2.NewCoinsRepository()
	coinsService := coinsApp.NewCoinsService(coinsRepo, db.Pgx())
	orderService := orderApp.NewOrderService(
		accountStatusChecker,
		shippingService,
		outboxRepository,
		configService,
		coinsService,
		roleChecker,
		nil, // actorResolver not needed for seed
		nil, // auditService not needed for seed
		stubProductShippingSetupRepo,
		nil, // walletService not needed for seed
		nil, // shippingQuoteService not needed for seed
	)

	// Initialize payment settlement service
	paymentSettlementService := paymentSettlementRepo.NewPaymentSettlementService()

	// Initialize ledger repository
	ledgerRepo := financeRepo.NewLedgerRepository()

	return &Seeder{
		db:                       db,
		log:                      log,
		cfg:                      cfg,
		contentService:           contentService,
		commentService:           commentService,
		likeService:              likeService,
		socialService:            socialService,
		orderService:             orderService,
		paymentSettlementService: paymentSettlementService,
		ledgerRepo:               ledgerRepo,
		roleChecker:              roleChecker,
		accountStatusChecker:     accountStatusChecker,
	}, nil
}

// ensureRoleColumn ensures the role column exists in the users table
func ensureRoleColumn(ctx context.Context, db *database.DB, log *zap.Logger) error {
	// Check if role column exists
	var columnName string
	err := db.Pgx().Pool().QueryRow(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_name = 'users' AND column_name = 'role'
	`).Scan(&columnName)

	if err == nil {
		return nil // Column exists
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to check for role column: %w", err)
	}

	// Column doesn't exist, add it
	log.Info("Adding role column to users table")
	_, err = db.Pgx().Pool().Exec(ctx, `
		ALTER TABLE users
		ADD COLUMN role TEXT NOT NULL DEFAULT 'user',
		ADD CONSTRAINT users_role_check
		CHECK (role IN ('user', 'admin'))
	`)
	if err != nil {
		return fmt.Errorf("failed to add role column: %w", err)
	}

	log.Info("Role column added successfully")
	return nil
}

// Run executes the seeding process
func (s *Seeder) Run(cleanMode bool) error {
	ctx := context.Background()

	// STEP 0: Clean mode - truncate tables
	if cleanMode {
		if err := s.cleanTables(ctx); err != nil {
			return fmt.Errorf("clean tables failed: %w", err)
		}
	}

	// STEP 1: Create users
	buyerID, sellerID, adminID, err := s.seedUsers(ctx)
	if err != nil {
		return fmt.Errorf("seed users failed: %w", err)
	}

	// STEP 2: Create content (25 items)
	normalContentIDs, _, _, err := s.seedContent(ctx, sellerID)
	if err != nil {
		return fmt.Errorf("seed content failed: %w", err)
	}

	// STEP 3: Create comments (15 comments on first normal content)
	if len(normalContentIDs) > 0 {
		if err := s.seedComments(ctx, normalContentIDs[0], buyerID, sellerID); err != nil {
			return fmt.Errorf("seed comments failed: %w", err)
		}
	}

	// STEP 4: Create likes (buyer and seller like first content)
	if len(normalContentIDs) > 0 {
		if err := s.seedLikes(ctx, normalContentIDs[0], buyerID, sellerID); err != nil {
			return fmt.Errorf("seed likes failed: %w", err)
		}
	}

	// STEP 5: Create follows (buyer follow seller, seller follow buyer)
	if err := s.seedFollows(ctx, buyerID, sellerID); err != nil {
		return fmt.Errorf("seed follows failed: %w", err)
	}

	// STEP 6: Create 1 completed trade
	// DISABLED: Collection purchase flow removed - see seedTrade function below
	// TODO: Re-implement using listing-based trade creation
	// if err := s.seedTrade(ctx, buyerID, sellerID); err != nil {
	// 	return fmt.Errorf("seed trade failed: %w", err)
	// }

	// Store IDs for potential future use
	s.log.Info("Seeded users",
		zap.String("buyer_id", buyerID.String()),
		zap.String("seller_id", sellerID.String()),
		zap.String("admin_id", adminID.String()),
	)

	return nil
}

// cleanTables truncates relevant tables in clean mode
func (s *Seeder) cleanTables(ctx context.Context) error {
	s.log.Info("Cleaning tables before seeding")

	tables := []string{
		"comments",
		"content_likes",
		"user_follows",
		"user_blocks",
		"ledger_transactions",
		"ratings",
		"contents",
	}

	for _, table := range tables {
		_, err := s.db.Pgx().Pool().Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			s.log.Warn("Failed to truncate table", zap.String("table", table), zap.Error(err))
		} else {
			s.log.Debug("Truncated table", zap.String("table", table))
		}
	}

	return nil
}

// seedUsers creates 3 test users with different roles
// IMPORTANT: Uses fixed UUIDs to match middleware local auth bypass
// These UUIDs MUST match the constants in internal/middleware/auth.go
func (s *Seeder) seedUsers(ctx context.Context) (buyerID, sellerID, adminID uuid.UUID, err error) {
	s.log.Info("Seeding users with FIXED UUIDs for local auth bypass")

	// FIXED UUIDs - MUST match middleware MockBuyerUID, MockSellerUID, MockAdminUID
	// See: internal/middleware/auth.go
	buyerID, _ = uuid.Parse("00000000-0000-0000-0000-000000000001")
	sellerID, _ = uuid.Parse("00000000-0000-0000-0000-000000000002")
	adminID, _ = uuid.Parse("00000000-0000-0000-0000-000000000003")

	users := []struct {
		id    uuid.UUID
		email string
		role  string
	}{
		{buyerID, "buyer@test.local", "user"},
		{sellerID, "seller@test.local", "seller"},
		{adminID, "admin@test.local", "admin"},
	}

	for _, u := range users {
		// Insert user directly into database
		// Using raw SQL because there's no UserService for creating users
		query := `
			INSERT INTO users (id, firebase_uid, email, email_verified_at, account_status, role, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, role = EXCLUDED.role
		`
		_, err := s.db.Pgx().Pool().Exec(ctx, query,
			u.id,
			u.id.String(), // Use UUID as firebase_uid for testing
			u.email,
			time.Now(),
			"active",
			u.role,
		)
		if err != nil {
			return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("failed to insert user %s: %w", u.email, err)
		}
		s.log.Debug("Created user", zap.String("email", u.email), zap.String("role", u.role))
	}

	// Seed admin capabilities: minimum set for admin UI access.
	// governance.capability.assign: lets admin grant further caps via admin panel.
	// governance.dashboard.view: lets admin reach the admin dashboard route.
	adminCaps := []string{
		"governance.capability.assign",
		"governance.dashboard.view",
	}
	for _, cap := range adminCaps {
		_, err := s.db.Pgx().Pool().Exec(ctx, `
			INSERT INTO user_capabilities (id, user_id, capability, granted_by, granted_at)
			VALUES ($1, $2, $3, NULL, NOW())
			ON CONFLICT DO NOTHING
		`, uuid.New(), adminID, cap)
		if err != nil {
			return uuid.Nil, uuid.Nil, uuid.Nil, fmt.Errorf("failed to seed capability %s for admin: %w", cap, err)
		}
		s.log.Debug("Seeded admin capability", zap.String("capability", cap))
	}

	return buyerID, sellerID, adminID, nil
}

// seedContent creates 25 content items: 20 normal, 3 hidden, 2 deleted
func (s *Seeder) seedContent(ctx context.Context, sellerID uuid.UUID) (normal, hidden, deleted []uuid.UUID, err error) {
	s.log.Info("Seeding content (25 items)")

	// Use system caller for bypassing auth in seeder context
	systemCaller := auth.SystemCallerID

	baseTime := time.Now().Add(-25 * time.Minute) // Start 25 minutes ago

	var normalIDs, hiddenIDs, deletedIDs []uuid.UUID

	// Create 20 normal contents
	for i := 0; i < 20; i++ {
		contentTime := baseTime.Add(time.Duration(i) * time.Minute)
		contentID, err := s.createContentWithTimestamp(ctx, sellerID, contentTime, "post", false)
		if err != nil {
			return nil, nil, nil, err
		}
		normalIDs = append(normalIDs, contentID)
	}

	// Create 3 hidden contents
	for i := 0; i < 3; i++ {
		contentTime := baseTime.Add(time.Duration(20+i) * time.Minute)
		contentID, err := s.createContentWithTimestamp(ctx, sellerID, contentTime, "post", true)
		if err != nil {
			return nil, nil, nil, err
		}
		hiddenIDs = append(hiddenIDs, contentID)
	}

	// Create 2 deleted contents (create then delete)
	for i := 0; i < 2; i++ {
		contentTime := baseTime.Add(time.Duration(23+i) * time.Minute)
		contentID, err := s.createContentWithTimestamp(ctx, sellerID, contentTime, "post", false)
		if err != nil {
			return nil, nil, nil, err
		}

		// Delete the content using ContentService
		if err := s.db.WithTx(ctx, func(tx db.Tx) error {
			return s.contentService.DeleteContent(ctx, tx, systemCaller, contentID)
		}); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to delete content %s: %w", contentID, err)
		}

		deletedIDs = append(deletedIDs, contentID)
	}

	s.log.Info("Content seeded",
		zap.Int("normal", len(normalIDs)),
		zap.Int("hidden", len(hiddenIDs)),
		zap.Int("deleted", len(deletedIDs)),
	)

	return normalIDs, hiddenIDs, deletedIDs, nil
}

// createContentWithTimestamp creates content with a specific created_at timestamp
func (s *Seeder) createContentWithTimestamp(ctx context.Context, authorID uuid.UUID, createdAt time.Time, contentType string, isHidden bool) (uuid.UUID, error) {
	// Generate content ID first
	contentID := uuid.New()
	_ = contentType

	// Insert content directly to control created_at
	status := "active"
	caption := fmt.Sprintf("Test content %s", contentID.String()[:8])

	query := `
		INSERT INTO contents (id, author_id, status, caption, is_hidden, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.Pgx().Pool().Exec(ctx, query,
		contentID,
		authorID,
		status,
		caption,
		isHidden,
		createdAt,
		createdAt,
	)

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create content: %w", err)
	}

	return contentID, nil
}

// seedComments creates 15 comments on a content
func (s *Seeder) seedComments(ctx context.Context, contentID uuid.UUID, buyerID, sellerID uuid.UUID) error {
	s.log.Info("Seeding comments (15 items)")

	baseTime := time.Now().Add(-15 * time.Minute)

	for i := 0; i < 15; i++ {
		// Alternate between buyer and seller
		authorID := buyerID
		if i%2 == 1 {
			authorID = sellerID
		}

		commentTime := baseTime.Add(time.Duration(i) * time.Minute)
		body := fmt.Sprintf("Comment %d: This is a test comment with incremental timestamp", i+1)

		// Insert comment directly to control created_at.
		// comments table uses target_id/target_type (not content_id) since schema convergence.
		// target_type has only one valid value: 'content'.
		// Canonical columns: legacy `type` / `share_reference` were dropped by migration 000031.
		commentID := uuid.New()
		query := `
			INSERT INTO comments (id, target_id, target_type, author_id, body, created_at, updated_at)
			VALUES ($1, $2, 'content', $3, $4, $5, $5)
		`
		_, err := s.db.Pgx().Pool().Exec(ctx, query,
			commentID,
			contentID,
			authorID,
			body,
			commentTime,
		)
		if err != nil {
			return fmt.Errorf("failed to create comment %d: %w", i, err)
		}
	}

	s.log.Info("Comments seeded", zap.Int("count", 15))
	return nil
}

// seedLikes creates likes from buyer and seller on a content
func (s *Seeder) seedLikes(ctx context.Context, contentID, buyerID, sellerID uuid.UUID) error {
	s.log.Info("Seeding likes")

	// Buyer likes content
	if err := s.likeService.Like(ctx, contentID, buyerID); err != nil {
		// Log but don't fail - might be duplicate
		s.log.Debug("Buyer like result", zap.Error(err))
	}

	// Seller likes content
	if err := s.likeService.Like(ctx, contentID, sellerID); err != nil {
		// Log but don't fail - might be duplicate
		s.log.Debug("Seller like result", zap.Error(err))
	}

	s.log.Info("Likes seeded", zap.Int("count", 2))
	return nil
}

// seedFollows creates follow relationships
func (s *Seeder) seedFollows(ctx context.Context, buyerID, sellerID uuid.UUID) error {
	s.log.Info("Seeding follows")

	// Buyer follows seller
	if err := s.socialService.Follow(ctx, buyerID, sellerID); err != nil {
		return fmt.Errorf("buyer follow seller failed: %w", err)
	}

	// Seller follows buyer
	if err := s.socialService.Follow(ctx, sellerID, buyerID); err != nil {
		return fmt.Errorf("seller follow buyer failed: %w", err)
	}

	s.log.Info("Follows seeded", zap.Int("count", 2))
	return nil
}

// printSummary prints the seeding summary
func printSummary() {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Seeding complete:")
	fmt.Println("  Users: 3")
	fmt.Println("    - buyer@test.local (user)")
	fmt.Println("    - seller@test.local (seller)")
	fmt.Println("    - admin@test.local (admin)")
	fmt.Println("  Contents: 25")
	fmt.Println("    - 20 normal (active, visible)")
	fmt.Println("    - 3 hidden (is_hidden = true)")
	fmt.Println("    - 2 deleted (deleted_at != null)")
	fmt.Println("  Comments: 15")
	fmt.Println("    - On first normal content")
	fmt.Println("    - Alternating buyer/seller authors")
	fmt.Println("  Likes: 2")
	fmt.Println("    - buyer likes content A")
	fmt.Println("    - seller likes content A")
	fmt.Println("  Follows: 2")
	fmt.Println("    - buyer follows seller")
	fmt.Println("    - seller follows buyer")
	fmt.Println("  Trades: 1")
	fmt.Println("    - Status: completed")
	fmt.Println("    - Escrow: released")
	fmt.Println(strings.Repeat("=", 50))
}

// =============================================================================
// STUB SHIPPING REPOSITORIES
// =============================================================================

type stubShippingSetupRepository struct{}

func (r *stubShippingSetupRepository) Create(ctx context.Context, tx db.Tx, option *shippingEntity.ShippingSetup) error {
	return nil
}
func (r *stubShippingSetupRepository) Update(ctx context.Context, tx db.Tx, option *shippingEntity.ShippingSetup) error {
	return nil
}
func (r *stubShippingSetupRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingEntity.ShippingSetup, error) {
	return nil, nil
}
func (r *stubShippingSetupRepository) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingEntity.ShippingSetup, error) {
	return nil, nil
}
func (r *stubShippingSetupRepository) GetBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, onlyActive bool) ([]*shippingEntity.ShippingSetup, error) {
	return []*shippingEntity.ShippingSetup{
		{ID: uuid.New(), Name: "Test Shipping", IsActive: true},
	}, nil
}
func (r *stubShippingSetupRepository) GetByName(ctx context.Context, tx db.Tx, sellerID uuid.UUID, name string) (*shippingEntity.ShippingSetup, error) {
	return nil, nil
}
func (r *stubShippingSetupRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}

type stubShippingCoverageRepository struct{}

func (r *stubShippingCoverageRepository) Create(ctx context.Context, tx db.Tx, coverage *shippingEntity.ShippingCoverage) error {
	return nil
}
func (r *stubShippingCoverageRepository) Update(ctx context.Context, tx db.Tx, coverage *shippingEntity.ShippingCoverage) error {
	return nil
}
func (r *stubShippingCoverageRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingEntity.ShippingCoverage, error) {
	return nil, nil
}
func (r *stubShippingCoverageRepository) GetByShippingSetup(ctx context.Context, tx db.Tx, shippingSetupID uuid.UUID) ([]*shippingEntity.ShippingCoverage, error) {
	return nil, nil
}
func (r *stubShippingCoverageRepository) GetByOptionAndProvince(ctx context.Context, tx db.Tx, shippingSetupID uuid.UUID, provinceCode string) (*shippingEntity.ShippingCoverage, error) {
	return &shippingEntity.ShippingCoverage{ID: uuid.New()}, nil
}
func (r *stubShippingCoverageRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}
func (r *stubShippingCoverageRepository) DeleteByShippingSetup(ctx context.Context, tx db.Tx, shippingSetupID uuid.UUID) error {
	return nil
}

type stubCityOverrideRepository struct{}

func (r *stubCityOverrideRepository) Create(ctx context.Context, tx db.Tx, override *shippingEntity.CityOverride) error {
	return nil
}
func (r *stubCityOverrideRepository) Update(ctx context.Context, tx db.Tx, override *shippingEntity.CityOverride) error {
	return nil
}
func (r *stubCityOverrideRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingEntity.CityOverride, error) {
	return nil, nil
}
func (r *stubCityOverrideRepository) GetByCoverage(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID) ([]*shippingEntity.CityOverride, error) {
	return nil, nil
}
func (r *stubCityOverrideRepository) GetByCoverageAndCity(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID, cityCode string) (*shippingEntity.CityOverride, error) {
	return nil, nil
}
func (r *stubCityOverrideRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}
func (r *stubCityOverrideRepository) DeleteByCoverage(ctx context.Context, tx db.Tx, shippingCoverageID uuid.UUID) error {
	return nil
}

type stubProductShippingSetupRepository struct{}

func (r *stubProductShippingSetupRepository) Create(ctx context.Context, tx db.Tx, listingID uuid.UUID, shippingSetupID uuid.UUID, sortOrder int) error {
	return nil
}
func (r *stubProductShippingSetupRepository) Delete(ctx context.Context, tx db.Tx, listingID uuid.UUID, shippingSetupID uuid.UUID) error {
	return nil
}
func (r *stubProductShippingSetupRepository) GetByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*shippingEntity.ShippingSetup, error) {
	return []*shippingEntity.ShippingSetup{
		{ID: uuid.New(), Name: "Test Shipping", IsActive: true},
	}, nil
}
func (r *stubProductShippingSetupRepository) GetAvailableByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*shippingEntity.ShippingSetup, error) {
	return []*shippingEntity.ShippingSetup{
		{ID: uuid.New(), Name: "Test Shipping", IsActive: true},
	}, nil
}
func (r *stubProductShippingSetupRepository) DeleteByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) error {
	return nil
}
func (r *stubProductShippingSetupRepository) DeleteByShippingSetup(ctx context.Context, tx db.Tx, shippingSetupID uuid.UUID) error {
	return nil
}
func (r *stubProductShippingSetupRepository) CreateBulk(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingSetupIDs []uuid.UUID) error {
	return nil
}
func (r *stubProductShippingSetupRepository) CountByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) (int64, error) {
	return 1, nil
}

// maskDSNPassword masks the password in a DSN string for safe logging
// Example: "host=localhost port=5432 user=labuda password=secret dbname=labuda sslmode=disable"
//
//	-> "host=localhost port=5432 user=labuda password=*** dbname=labuda sslmode=disable"
func maskDSNPassword(dsn string) string {
	// Find password= and replace everything until the next space or end of string
	idx := strings.Index(dsn, "password=")
	if idx == -1 {
		return dsn
	}
	// Find the end of the password value (next space or end of string)
	start := idx + 9 // len("password=")
	end := strings.Index(dsn[start:], " ")
	if end == -1 {
		// Password is at the end of string
		return dsn[:start] + "***"
	}
	return dsn[:start] + "***" + dsn[start+end:]
}
