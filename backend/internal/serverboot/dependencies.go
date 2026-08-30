// Package serverboot owns the canonical core_server dependency graph.
//
// PHASE 1B SPLIT (2026-05-12): formerly co-located in cmd/core_server/main
// package, this file was relocated verbatim — same imports, same symbols,
// same comments — so both cmd/core_server and cmd/corpus_driver can import
// the same InitServices/StartWorkers pair. The relocation is purely a
// package-boundary move; no business logic, no signatures, no defaults
// changed.
package serverboot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/audit"
	auctionApp "github.com/labuda/backend/internal/commerce/auction/application"
	auctionHTTP "github.com/labuda/backend/internal/commerce/auction/delivery/http"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	commerceResponse "github.com/labuda/backend/internal/commerce/response"
	"github.com/labuda/backend/internal/config"
	bankaccountApp "github.com/labuda/backend/internal/finance/bankaccount/application"
	bankaccountHTTP "github.com/labuda/backend/internal/finance/bankaccount/delivery/http"
	bankaccountrepo "github.com/labuda/backend/internal/finance/bankaccount/infrastructure/repository"
	auditApp "github.com/labuda/backend/internal/governance/audit/application"
	auditRepo "github.com/labuda/backend/internal/governance/audit/repository"
	"github.com/labuda/backend/internal/identity/auth"
	authHTTP "github.com/labuda/backend/internal/identity/auth/delivery/http"
	coinsApp "github.com/labuda/backend/internal/incentive/coins/application"
	coinsEntity "github.com/labuda/backend/internal/incentive/coins/entity"
	coinsRepo "github.com/labuda/backend/internal/incentive/coins/infrastructure/repository"
	biddingApp "github.com/labuda/backend/internal/interaction/bidding/application"
	biddingHTTP "github.com/labuda/backend/internal/interaction/bidding/delivery/http"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatHTTP "github.com/labuda/backend/internal/interaction/chat/delivery/http"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contentHTTP "github.com/labuda/backend/internal/social/content/delivery/http"
	likerepo "github.com/labuda/backend/internal/social/like/infrastructure/repository"

	forSaleApp "github.com/labuda/backend/internal/commerce/forsale/application"
	forSaleHTTP "github.com/labuda/backend/internal/commerce/forsale/delivery/http"
	forSaleRepo "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	negotiationApp "github.com/labuda/backend/internal/commerce/negotiation/application"
	negotiationWorker "github.com/labuda/backend/internal/commerce/negotiation/worker"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderHTTP "github.com/labuda/backend/internal/commerce/order/delivery/http"
	orderEntity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	ratingApp "github.com/labuda/backend/internal/commerce/order/rating/application"
	ratingHTTP "github.com/labuda/backend/internal/commerce/order/rating/delivery/http"
	paymentmethodHTTP "github.com/labuda/backend/internal/commerce/paymentmethod/delivery/http"
	paymentmethodentity "github.com/labuda/backend/internal/commerce/paymentmethod/entity"
	paymentmethodrepo "github.com/labuda/backend/internal/commerce/paymentmethod/infrastructure/repository"
	sellerHTTP "github.com/labuda/backend/internal/commerce/seller/delivery/http"
	sellerRepoImpl "github.com/labuda/backend/internal/commerce/seller/infrastructure/repository"
	shippingApp "github.com/labuda/backend/internal/commerce/shipping/application"
	shippingHTTP "github.com/labuda/backend/internal/commerce/shipping/delivery/http"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	shippingRepo "github.com/labuda/backend/internal/commerce/shipping/infrastructure/repository"
	shippingQuoteApp "github.com/labuda/backend/internal/commerce/shipping/quote/application"
	shippingQuoteHTTP "github.com/labuda/backend/internal/commerce/shipping/quote/delivery/http"
	shippingQuoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/infrastructure/repository"
	subscriptionApp "github.com/labuda/backend/internal/commerce/subscription/application"
	subscriptionHTTP "github.com/labuda/backend/internal/commerce/subscription/delivery/http"
	subscriptionRepoImpl "github.com/labuda/backend/internal/commerce/subscription/infrastructure/repository"
	financeApp "github.com/labuda/backend/internal/finance/application"
	billingentity "github.com/labuda/backend/internal/finance/billing/entity"
	billingrepo "github.com/labuda/backend/internal/finance/billing/infrastructure/repository"
	financePayoutHTTP "github.com/labuda/backend/internal/finance/delivery/http"
	financeRepo "github.com/labuda/backend/internal/finance/infrastructure/repository"
	financeWorker "github.com/labuda/backend/internal/finance/worker"
	disputeApp "github.com/labuda/backend/internal/governance/dispute/application"
	disputeHTTP "github.com/labuda/backend/internal/governance/dispute/delivery/http"
	disputeRepo "github.com/labuda/backend/internal/governance/dispute/infrastructure/repository"
	"github.com/labuda/backend/internal/governance/evaluator"
	moderationApp "github.com/labuda/backend/internal/governance/moderation/application"
	moderationHTTP "github.com/labuda/backend/internal/governance/moderation/delivery/http"
	moderationRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	verificationApp "github.com/labuda/backend/internal/governance/verification/application"
	verificationHTTP "github.com/labuda/backend/internal/governance/verification/delivery/http"
	verificationRepo "github.com/labuda/backend/internal/governance/verification/infrastructure/repository"
	addressApp "github.com/labuda/backend/internal/identity/address/application"
	addressHTTP "github.com/labuda/backend/internal/identity/address/delivery/http"
	addressRepoImpl "github.com/labuda/backend/internal/identity/address/infrastructure/repository"
	userApp "github.com/labuda/backend/internal/identity/user/application"
	userHTTP "github.com/labuda/backend/internal/identity/user/delivery/http"
	userRepoImpl "github.com/labuda/backend/internal/identity/user/infrastructure/repository"
	paymentApp "github.com/labuda/backend/internal/integration/payment/application"
	paymentHTTP "github.com/labuda/backend/internal/integration/payment/delivery/http"
	"github.com/labuda/backend/internal/integration/payment/infrastructure/repository"
	notificationHTTP "github.com/labuda/backend/internal/interaction/notification/delivery/http"
	notificationRepoImpl "github.com/labuda/backend/internal/interaction/notification/infrastructure/repository"
	notificationPolicy "github.com/labuda/backend/internal/interaction/notification/policy"
	notificationService "github.com/labuda/backend/internal/interaction/notification/service"
	savedItemApp "github.com/labuda/backend/internal/interaction/saved_item/application"
	savedItemHTTP "github.com/labuda/backend/internal/interaction/saved_item/delivery/http"
	savedItemRepoImpl "github.com/labuda/backend/internal/interaction/saved_item/infrastructure/repository"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/monitoring"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	platformconfigHTTP "github.com/labuda/backend/internal/platform/config/delivery/http"
	platformconfigRepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	idempotencyRepoPkg "github.com/labuda/backend/internal/platform/idempotency/repository"
	"github.com/labuda/backend/internal/platform/logger"
	mediauploadHTTP "github.com/labuda/backend/internal/platform/mediaupload"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	ogHTTP "github.com/labuda/backend/internal/platform/og/delivery/http"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/internal/platform/s3presign"
	"github.com/labuda/backend/internal/presence"
	discountHTTP "github.com/labuda/backend/internal/pricing/discount/delivery/http"
	pricingtokenapp "github.com/labuda/backend/internal/pricing/token/application"
	pricingtokenHTTP "github.com/labuda/backend/internal/pricing/token/delivery/http"
	pricingtokenentity "github.com/labuda/backend/internal/pricing/token/entity"
	"github.com/labuda/backend/internal/projection"
	"github.com/labuda/backend/internal/realtime"
	contentrepo "github.com/labuda/backend/internal/social/content/infrastructure/repository"
	feedApp "github.com/labuda/backend/internal/social/feed/application"
	feedHTTP "github.com/labuda/backend/internal/social/feed/delivery/http"
	feedrepo "github.com/labuda/backend/internal/social/feed/infrastructure/repository"
	socialApp "github.com/labuda/backend/internal/social/graph/application"
	socialhttp "github.com/labuda/backend/internal/social/graph/delivery/http"
	socialRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	likeApp "github.com/labuda/backend/internal/social/like/application"
	likeHTTP "github.com/labuda/backend/internal/social/like/delivery/http"
	"github.com/labuda/backend/internal/worker"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/firebase"
	"github.com/labuda/backend/pkg/midtrans"
	"github.com/labuda/backend/pkg/money"
	pkgRate "github.com/labuda/backend/pkg/rate"
	pkgRedis "github.com/labuda/backend/pkg/redis"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	// Admin / Operability handlers - ADMIN HARDENING PACK V1
	appealApp "github.com/labuda/backend/internal/governance/moderation/application"
	warningApp "github.com/labuda/backend/internal/governance/moderation/application"
	appealHTTP "github.com/labuda/backend/internal/governance/moderation/delivery/http"
	appealInfraRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	warningInfraRepo "github.com/labuda/backend/internal/governance/moderation/infrastructure/repository"
	supportApp "github.com/labuda/backend/internal/governance/support/application"
	supportHTTP "github.com/labuda/backend/internal/governance/support/delivery/http"
	supportInfraRepo "github.com/labuda/backend/internal/governance/support/infrastructure/repository"
	adminApp "github.com/labuda/backend/internal/platform/admin/application"
	adminHTTP "github.com/labuda/backend/internal/platform/admin/delivery/http"
	adminInfraRepo "github.com/labuda/backend/internal/platform/admin/infrastructure/repository"

	// Promotion domain - RUNTIME HARDENING PACK V1
	promotionApp "github.com/labuda/backend/internal/pricing/promotion/application"
	promotionHTTP "github.com/labuda/backend/internal/pricing/promotion/delivery/http"
	promotionInfraRepo "github.com/labuda/backend/internal/pricing/promotion/infrastructure/repository"

	// Billing domain - for promotion package purchases
	billingApp "github.com/labuda/backend/internal/finance/billing/application"

	// Refund domain - TASK 34 / Phase 2a: gateway-aware refund foundation
	refundApp "github.com/labuda/backend/internal/finance/refund/application"
	refundHTTP "github.com/labuda/backend/internal/finance/refund/delivery/http"
	refundRepoImpl "github.com/labuda/backend/internal/finance/refund/infrastructure/repository"

	// Search domain - FEDERATED SEARCH CONTRACT REALIGN PACK V1
	searchApp "github.com/labuda/backend/internal/discovery/search/application"
	searchHTTP "github.com/labuda/backend/internal/discovery/search/delivery/http"
	searchRepo "github.com/labuda/backend/internal/discovery/search/infrastructure/repository"

	// Capability domain - SLICE 1: Capability Storage Foundation
	"github.com/labuda/backend/internal/platform/capability"
	capabilityApp "github.com/labuda/backend/internal/platform/capability/application"
	capabilityHTTP "github.com/labuda/backend/internal/platform/capability/delivery/http"
	capabilityEntity "github.com/labuda/backend/internal/platform/capability/entity"
	capabilityInfra "github.com/labuda/backend/internal/platform/capability/infrastructure"
	capabilityRepo "github.com/labuda/backend/internal/platform/capability/infrastructure/repository"

	// Alert domain - ALERT SYSTEM V1
	alertApp "github.com/labuda/backend/internal/platform/alert/application"
	alertHTTP "github.com/labuda/backend/internal/platform/alert/delivery/http"
	alertRepo "github.com/labuda/backend/internal/platform/alert/infrastructure/repository"

	// Fraud domain - ANTI-FRAUD FOUNDATION V1

	// Wallet domain - WALLET PHASE 1 (FOUNDATION)
	walletApp "github.com/labuda/backend/internal/core/wallet/application"
	walletHTTP "github.com/labuda/backend/internal/core/wallet/delivery/http"

	// Product domain - wired into AuctionService for inline product creation
	productRepoImpl "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
)

// Dependencies holds all application dependencies
type Dependencies struct {
	// Handlers
	AuthHandler            *authHTTP.AuthHandler // Phase 1: Canonical auth entry point
	ProductShippingHandler *shippingHTTP.ProductShippingHandler
	ShippingHandler        *shippingHTTP.ShippingHandler       // Buyer-facing: check delivery availability
	SellerShippingHandler  *shippingHTTP.SellerShippingHandler // Seller-facing: shipping option management
	PaymentHandler         *CorePaymentHandler
	CoinHandler            *CoreCoinHandler
	UserHandler            *CoreUserHandler      // Legacy stub - will be replaced
	UserProfileHandler     *userHTTP.UserHandler // New profile handler
	PaymentWebhookHandler  *paymentHTTP.PaymentWebhookHandler
	PayoutWebhookHandler   *financePayoutHTTP.PayoutWebhookHandler
	// PHASE 2D / TASK 43: canonical seller withdrawal request endpoint.
	WithdrawalHandlerUnified *walletHTTP.WithdrawalHandlerUnified
	// C6.1: seller self-service bank account management.
	BankAccountHandler *bankaccountHTTP.BankAccountHandler
	// Address CRUD endpoints (buyer shipping + seller sender)
	AddressHandler         *addressHTTP.AddressHandler
	OrderHandler           *orderHTTP.OrderHandler
	AuctionHandler         *auctionHTTP.AuctionHandler
	AdminAuctionHandler    *auctionHTTP.AdminAuctionHandler // PASS_5B: admin emergency auction cancel/override
	SavedItemHandler       *savedItemHTTP.SavedItemHandler
	BiddingHandler         *biddingHTTP.BiddingHandler
	ForSaleHandler  *forSaleHTTP.ForSaleHandler
	PricingTokenHandler    *pricingtokenHTTP.PricingTokenHandler
	ChatHandler            *chatHTTP.Handler
	DiscountHandler        *discountHTTP.DiscountHandler
	DisputeHandler         *disputeHTTP.DisputeHandler
	SellerHandler          *sellerHTTP.SellerHandler
	SystemHealthHandler    *monitoring.SystemHealthHandler
	RealtimeHandler        *realtime.Handler
	FeedHandler            *feedHTTP.FeedHandler
	ContentHandler         *contentHTTP.ContentHandler
	OGHandler              *ogHTTP.Handler
	CommentHandler         *contentHTTP.CommentHandler
	LikeHandler            *likeHTTP.LikeHandler
	NotificationHandler    *notificationHTTP.NotificationHandler
	FCMTokenHandler        *notificationHTTP.FCMTokenHandler
	ModerationHandler      *moderationHTTP.ModerationHandler  // Trust MVP: moderation endpoints
	FollowHandler          *socialhttp.FollowHandler          // SOCIAL domain: follow/block/mute
	ShippingQuoteHandler   *shippingQuoteHTTP.Handler         // Shipping quote feature (chat-based manual quotes)
	RatingHandler          *ratingHTTP.RatingHandler          // RATING DOMAIN: buyer→seller order ratings
	PromotionHandler       *promotionHTTP.PromotionHandler    // PROMOTION PHASE 4: Discovery endpoints
	SearchHandler          *searchHTTP.SearchHandler          // FEDERATED SEARCH: content, users, forSales
	AdminOrderHandler      *orderHTTP.AdminOrderHandler       // ADMIN ORDER: read-only order management
	AdminRefundHandler     *refundHTTP.AdminRefundHandler     // TASK 34 / Phase 2a: admin-only gateway refund trigger (feature-flagged)
	SellerRefundHandler    *refundHTTP.SellerRefundHandler    // H2-A: seller approve/reject refund endpoints
	BuyerEscalationHandler *refundHTTP.BuyerEscalationHandler // H2-B: buyer escalate rejected refund

	// VERIFICATION (Phase 2 operationalization)
	VerificationHandler      *verificationHTTP.VerificationHandler      // Seller-facing: submit identity/business, status
	AdminVerificationHandler *verificationHTTP.AdminVerificationHandler // Admin-facing: pending queue, approve/reject/request_resubmission

	// MEDIA UPLOAD — general presigned S3 PUT URL (non-KYC)
	MediaUploadHandler *mediauploadHTTP.Handler

	// Admin / Operability handlers - ADMIN HARDENING PACK V1
	AdminHandler                     *adminHTTP.AdminHandler                            // Admin dashboard, user management, metrics
	CapabilityHandler                *capabilityHTTP.CapabilityHandler                  // Capability management endpoints
	AppealHandler                    *appealHTTP.AppealHandler                          // Appeals system for moderation
	WarningHandler                   *appealHTTP.WarningHandler                         // Warnings system for policy violations
	SupportHandler                   *supportHTTP.Handler                               // Support ticket system
	AdminPayoutHandler               *financePayoutHTTP.AdminPayoutHandler              // Admin payout operations
	AdminFinanceHandler              *financePayoutHTTP.AdminFinanceHandler             // Finance ledger export + verifier
	PlatformConfigHandler            *platformconfigHTTP.PlatformConfigHandler          // Platform configuration management
	AdminSubscriptionConfigHandler   *subscriptionHTTP.AdminSubscriptionConfigHandler   // Admin: seller subscription config CRUD
	AdminSubscriptionRecoveryHandler *subscriptionHTTP.AdminSubscriptionRecoveryHandler // Admin: manual subscription payment recovery (webhook miss)
	AdminAlertHandler                *alertHTTP.AdminAlertHandler                       // Admin alert operations
	AdminReconciliationHandler       *financePayoutHTTP.AdminReconciliationHandler      // Admin reconciliation result visibility (read-only)
	AdminPaymentMethodHandler        *paymentmethodHTTP.AdminPaymentMethodHandler       // PASS_18W: Admin payment method fee config

	// Services
	RoleChecker auth.RoleChecker

	// SLICE 1: Capability Storage Foundation
	CapabilityChecker *capability.Checker            // Capability checking service
	ActorResolver     capabilityEntity.ActorResolver // Actor resolution (role + capabilities)

	// OrderService is exposed for sibling binaries (e.g. corpus_driver
	// scenario modes) that need to invoke canonical order flows —
	// specifically the gateway refund initiator reachable via
	// OrderService.PaymentService().InitiateGatewayRefundForOrder.
	// Production HTTP handlers do NOT use this field; they hold their own
	// references constructed during InitServices.
	OrderService *orderApp.OrderService

	// ProjectionWorkerFull is the concrete projection worker, exposed for dev
	// tooling (e.g., corpus_driver scenario-projection) that needs RebuildAll /
	// GetProjectionStatus. HTTP routes must use ProjectionAdminHandler instead.
	ProjectionWorkerFull *worker.ProjectionWorker

	// ProjectionAdminHandler provides dev-only HTTP endpoints (status/rebuild/process).
	// Only registered in routes_core.go when cfg.IsDevelopment() is true.
	ProjectionAdminHandler *worker.ProjectionAdminHandler

	// Workers - All registered workers
	PaymentExpiryWorker              Worker
	ReconciliationWorker             Worker
	OrderAutoCompleteWorker          Worker
	OutboxWorker                     Worker
	ProjectionWorker                 Worker
	AuctionStartWorker               Worker
	AuctionEndWorker                 Worker
	SystemMonitoringWorker           Worker
	RealtimeWorker                   Worker
	PayoutWorker                     Worker
	PayoutReconciliationWorker       Worker
	NegotiationExpireWorker          Worker
	PromotionSafetyWorker            Worker // RUNTIME HARDENING PACK V1
	OrderOverdueReminderWorker       Worker // OVERDUE ENFORCEMENT CLOSURE
	DisputeTimeoutWorker             Worker // DISPUTE HARDENING - DEADLOCK PREVENTION
	AlertDetectionWorker             Worker // ALERT SYSTEM V1
	SellerSubscriptionExpiryWorker   Worker // SUBSCRIPTION LIFECYCLE - hourly active→expired sweep
	OutboxArchivalWorker             Worker // OUTBOX RETENTION - archives succeeded events older than RetentionDays
	OrderOverdueCancelWorker         Worker // ORDER FULFILLMENT - auto-cancels paid orders past shipment deadline
	SubscriptionReconciliationWorker Worker // SUBSCRIPTION HARDENING - recovers orphaned subscription payments
	WithdrawalMonitoringWorker       Worker // PAYOUT MONITORING - read-only alert on stuck withdrawals
	BNRDecayWorker                   Worker // BNR FORGIVENESS - daily decay of oldest strike per buyer after 180d inactivity
	PushRetryWorker                  Worker // Z6: PUSH RELIABILITY - retries failed FCM pushes with exponential backoff
	NotificationCleanupWorker        Worker // Z6: PUSH HYGIENE - deletes old delivery logs + expired retry entries
	EscrowIntegrityWorker            Worker // ESCROW RECONCILIATION - shadow-rollout periodic escrow vs wallet check
	TotalMoneyInvariantWorker        Worker // TOTAL MONEY INVARIANT - shadow-rollout periodic ledger sum check
	SellerMetricsWorker              Worker // SELLER MEASUREMENT - daily seller_monthly_metrics snapshot (measurement only)
	SellerReputationRecomputeWorker  Worker // REPUTATION AUTHORITY - nightly rolling 90-day recompute of seller tier + reputation state

	// workerStartups is the deferred list of worker .Start() actions
	// recorded during InitServices. StartWorkers invokes each closure in
	// the order it was appended, which equals the original order of
	// `worker.Start()` calls in InitServices. Unexported because no
	// consumer outside this package should be invoking workers directly —
	// always go through StartWorkers.
	workerStartups []func()
}

// Worker interface for background workers
type Worker interface {
	Start()
	Stop()
	IsRunning() bool
}

// workerEnabled gates background-worker startup on the DISABLE_<NAME> env
// var so corpus-generation runs can pin a deterministic failure mode (e.g.
// halt the auto-complete worker while exercising a refund path).
//
//   - env unset                  -> defaultOn governs
//   - DISABLE_<NAME>=true|1      -> worker disabled
//   - DISABLE_<NAME>=false|0     -> worker enabled
//   - any other value            -> defaultOn governs, treated as fallback
//
// When the result is "disabled" the helper emits a structured
// `worker_disabled` log with worker name, env key, and reason source so the
// runtime posture is greppable at startup.
func workerEnabled(name string, defaultOn bool, log *zap.Logger) bool {
	envKey := "DISABLE_" + strings.ToUpper(name)
	raw := strings.TrimSpace(os.Getenv(envKey))
	disabled := !defaultOn
	source := "default"
	switch strings.ToLower(raw) {
	case "":
		// keep default
	case "1", "true", "yes", "on":
		disabled = true
		source = "env"
	case "0", "false", "no", "off":
		disabled = false
		source = "env"
	default:
		source = "default_fallback_unrecognized_value"
	}
	if disabled {
		log.Info("worker_disabled",
			zap.String("worker", name),
			zap.String("env_key", envKey),
			zap.String("source", source),
		)
		return false
	}
	return true
}

// orphanWebhookRecoveryEnabled is the explicit env gate for the orphan webhook
// recovery worker. It remains unused by startup wiring until the activation
// phase, but the gate exists now so the runtime policy is explicit.
func orphanWebhookRecoveryEnabled(log *zap.Logger) bool {
	return workerEnabled("ORPHAN_WEBHOOK_RECOVERY_WORKER", false, log)
}

// ─────────────────────────────────────────────────────────────────────────────
// DANGEROUS DORMANT WORKER GUARD
// ─────────────────────────────────────────────────────────────────────────────
//
// Some workers are structurally disabled because their current implementation
// uses incorrect primitives (e.g., buyer-hold escrow instead of gateway-funded)
// or depends on services that are nil-wired in the dependency graph. Enabling
// them without prerequisite fixes would cause panics, data corruption, or
// incorrect financial mutations.
//
// The guard requires BOTH:
//   1. DISABLE_<NAME>=false  (standard workerEnabled gate)
//   2. ACK_DANGEROUS_<NAME>=true  (explicit operator acknowledgment)
//
// If (1) passes but (2) is absent, the process log.Fatals at startup to
// prevent accidental enablement via a single env var flip.
// ─────────────────────────────────────────────────────────────────────────────

// dangerousDormantWorkers enumerates every worker that MUST NOT be enabled
// without prerequisite engineering work.  The map value is a human-readable
// prerequisite description emitted in the fatal log line.
var dangerousDormantWorkers = map[string]string{
	"PAYMENT_EXPIRY_WORKER":  "gateway-funded refund path — activate with DISABLE_PAYMENT_EXPIRY_WORKER=false",
	"USER_BAN_EVENT_HANDLER": "mass-refund/dispute triggers on all active orders of banned user — operator must explicitly acknowledge scope",
}

// CheckDangerousDormantGuard returns a non-nil error if the named worker is
// in the dangerous-dormant registry AND the operator has not set
// ACK_DANGEROUS_<NAME>=true.  Exported for unit testing; production callers
// should use dangerousDormantGuard which log.Fatals on failure.
func CheckDangerousDormantGuard(name string) (prerequisite string, err error) {
	prereq, ok := dangerousDormantWorkers[strings.ToUpper(name)]
	if !ok {
		return "", nil // not in the dangerous list — no guard needed
	}
	ackKey := "ACK_DANGEROUS_" + strings.ToUpper(name)
	if strings.TrimSpace(os.Getenv(ackKey)) == "true" {
		return prereq, nil // operator explicitly acknowledged the risk
	}
	return prereq, fmt.Errorf(
		"worker %q is in the dangerous-dormant registry and cannot be enabled "+
			"without %s=true — prerequisite: %s",
		name, ackKey, prereq,
	)
}

// dangerousDormantGuard calls CheckDangerousDormantGuard and log.Fatals on
// failure, preventing the process from starting with a misconfigured env.
func dangerousDormantGuard(name string, log *zap.Logger) {
	prereq, err := CheckDangerousDormantGuard(name)
	if err != nil {
		log.Fatal("DANGEROUS_DORMANT_WORKER_GUARD: refusing to start",
			zap.String("worker", name),
			zap.String("required_env", "ACK_DANGEROUS_"+strings.ToUpper(name)+"=true"),
			zap.String("prerequisite", prereq),
			zap.Error(err),
		)
	}
	if prereq != "" {
		log.Warn("DANGEROUS_DORMANT_WORKER_GUARD: acknowledged — proceeding with caution",
			zap.String("worker", name),
			zap.String("prerequisite", prereq),
		)
	}
}

// InitServices constructs the full service / repository / handler /
// worker-instance graph that core_server runs. No background workers are
// started here — every `worker.Start()` call site is recorded as a closure
// in deps.workerStartups and invoked later by StartWorkers.
//
// PHASE 1B SPLIT (2026-05-12, mechanical refactor): formerly named
// InitDependencies; the rename + StartWorkers extraction lets corpus_driver
// reuse the canonical service graph without spinning up the worker
// goroutines that would interfere with deterministic corpus generation.
// Production startup posture is preserved because main.go now calls
// StartWorkers(deps) immediately after InitServices(...) returns.
func InitServices(
	appCtx context.Context,
	db *database.DB,
	firebaseClient *firebase.Client,
	midtransClient *midtrans.Client,
	redisClient *pkgRedis.Client,
	log *logger.Logger,
	cfg *config.Config,
	schemaReady bool,
) *Dependencies {
	isProduction := cfg.IsProduction()

	// workerStartups accumulates deferred worker .Start() closures so the
	// returned *Dependencies can be re-driven through StartWorkers either
	// by core_server (canonical path) or left dormant for corpus_driver.
	var workerStartups []func()

	// ===== AUTH MODULE =====
	adminAuditLogger := audit.NewAdminAuditLoggerDB(db.Pgx().Pool())
	roleChecker := auth.NewRoleCheckerDB(db.Pgx(), adminAuditLogger)
	accountStatusChecker := auth.NewAccountStatusCheckerDB(db.Pgx())

	// ===== SLICE 1: CAPABILITY STORAGE FOUNDATION =====
	// Initialize capability repository and checker
	// This provides the foundation for fine-grained authorization
	capabilityRepository := capabilityRepo.NewCapabilityRepository(db.Pgx())
	capabilityChecker := capability.NewChecker(capabilityRepository)

	// Initialize actor resolver (combines role + capabilities)
	// Uses UserStateQuerierAdapter to load all user state atomically from database
	userStateQuerier := capability.NewUserStateQuerierAdapter(db.Pgx())
	actorResolver := capabilityInfra.NewActorResolver(capabilityRepository, userStateQuerier)

	// Phase 1: Initialize AuthHandler - Canonical auth entry point
	// POST /api/v1/auth/firebase/exchange - verifies Firebase token, creates/links user, generates JWT
	authHandler := authHTTP.NewAuthHandler(db.Pgx().Pool(), firebaseClient, &cfg.JWT, log.Logger)

	// ===== PLATFORM CONFIG MODULE =====
	platformConfigRepo := platformconfigRepo.NewPlatformConfigRepository()
	configService := platformconfigApp.NewConfigService(platformConfigRepo)

	// MANAGEMENT PRE-FIX M1: Platform Config handler with audit logging
	platformConfigHandler := platformconfigHTTP.NewPlatformConfigHandler(
		configService,
		db.Pgx(),
		log.Logger,
	)

	// ===== USER MODULE =====
	// User repository for profile service
	userRepository := userRepoImpl.NewUserRepository(db.Pgx())
	// Note: sellerRepository and sellerSubscriptionRepo are created below in SELLER MODULE
	// We need them for UserProfileService, so we'll initialize the user handler after the seller module

	// ===== PAYMENT MODULE (REPOSITORIES ONLY) =====
	// PAYMENT BOUNDARY HARDENING V1: Order repository for amount validation
	orderRepository := orderRepo.NewOrderRepository()
	paymentRepo := repository.NewPaymentRepository()
	// PASS_18V/18W: canonical payment method fee table — shared by buyer
	// checkout (CorePaymentHandler) and admin config (AdminPaymentMethodHandler).
	paymentMethodRepository := paymentmethodrepo.NewPaymentMethodRepository()
	// NOTE: PaymentWebhookService is constructed AFTER orderService and
	// walletService (below) so it can receive the canonical instances.
	// Constructing it here with nil deps is what caused the original
	// MarkPaid nil-deref crash.

	// ===== SHIPPING MODULE =====
	// Wire real shipping repositories
	shippingOptionRepo := shippingRepo.NewShippingOptionRepository()
	coverageRepo := shippingRepo.NewShippingCoverageRepository()
	cityOverrideRepo := shippingRepo.NewCityOverrideRepository()
	productShippingRepo := shippingRepo.NewProductShippingOptionRepository(shippingOptionRepo)

	shippingService := shippingApp.NewShippingService(
		shippingOptionRepo,
		coverageRepo,
		cityOverrideRepo,
		productShippingRepo,
	)

	// ===== PRODUCT SHIPPING MODULE =====
	// Initialize product shipping service and handler for managing shipping options on products
	forSaleRepository := forSaleRepo.NewForSaleRepository()
	productShippingService := shippingApp.NewProductShippingService(
		forSaleRepository,
		shippingOptionRepo,
		productShippingRepo,
		orderRepository,
	)
	productShippingHandler := shippingHTTP.NewProductShippingHandler(
		productShippingService,
		db.Pgx(),
		log.Logger,
	)

	// ===== SHIPPING MODULE (Buyer-facing) =====
	// Initialize shipping handler for checking delivery availability
	shippingHandler := shippingHTTP.NewShippingHandler(
		shippingService,
		db.Pgx(),
		log.Logger,
	)

	// ===== SHIPPING MODULE (Seller-facing) =====
	// Initialize seller shipping handler for managing shipping options
	sellerShippingService := shippingApp.NewSellerShippingService(
		shippingOptionRepo,
		coverageRepo,
		cityOverrideRepo,
		productShippingRepo,
	)
	sellerShippingHandler := shippingHTTP.NewSellerShippingHandler(
		sellerShippingService,
		db.Pgx(),
		log.Logger,
	)

	// ===== OUTBOX MODULE =====
	outboxRepository := outboxRepo.NewOutboxRepository(db.Pgx())
	var presenceService *presence.Service

	// ===== OBSERVABILITY: AUDIT EVENT SYSTEM =====
	// Initialize audit event repository and service
	// This provides comprehensive logging of all critical business events
	auditEventRepository := auditRepo.NewAuditEventRepository()
	auditService := auditApp.NewAuditService(auditEventRepository, db.Pgx(), log.Logger)

	// Wire up audit service for payment repository
	paymentRepo.SetAuditService(auditService)

	// PaymentWebhookService audit wiring is performed after construction below.

	// ===== CONTENT REPOSITORIES =====
	// E3.3 — Content repository is constructed eagerly here because
	// CommentService.AddComment depends on it via contentRepo.GetByID to
	// validate the target content (status check + author lookup for
	// notifications). The prior stale TODO claimed an interface mismatch,
	// but ContentRepository.GetByID has long returned (*entity.Content,
	// error) — exactly what AddComment expects — and the same constructor
	// is consumed by content_service.go and like_service.go
	// without issue. The previous wiring passed
	// nil into NewCommentService for this slot, causing every POST
	// /api/v1/contents/:id/comments to panic with a nil-pointer
	// dereference at AddComment:100 (s.contentRepo.GetByID).
	contentRepo := contentrepo.NewContentRepository()

	// ===== ORDER MODULE =====
	// WALLET PHASE 1: Initialize wallet service for escrow hold on order creation
	walletService := walletApp.NewWalletService(db.Pgx(), log.Logger)

	orderService := orderApp.NewOrderService(accountStatusChecker, shippingService, outboxRepository, configService, nil, roleChecker, actorResolver, auditService, productShippingRepo, walletService, nil) // shippingQuoteService - will be set later
	// Gateway-aware release: wire finance ledger recorder into the order
	// payment service so OrderCompletionService.Complete can book
	// SELLER_PAYABLE / PLATFORM_REVENUE / GATEWAY_CLEARING ledger entries.
	orderService.PaymentService().SetFinanceReleaseRecorder(financeApp.NewFinanceService())
	projectionRepo := projection.NewRepository(db.Pgx())
	projectionEnabled := workerEnabled("PROJECTION_WORKER", false, log.Logger)
	orderQueryService := orderApp.NewOrderQueryService(projectionRepo, projectionEnabled)

	// ===== PAYMENT WEBHOOK SERVICE =====
	// Constructed AFTER orderService and walletService so it receives the
	// canonical instances. Constructing earlier with nil deps was the root
	// cause of the original MarkPaid nil-deref crash.
	paymentWebhookService := paymentApp.NewPaymentWebhookService(
		db.Pgx(),
		midtransClient,
		orderService,
		walletService,
		log.Logger,
	)
	paymentWebhookHandler := paymentHTTP.NewPaymentWebhookHandler(
		paymentWebhookService,
		log.Logger,
	)
	// Wire up audit service for payment webhook service (includes settlement service)
	paymentWebhookService.SetAuditService(auditService)

	// Wire FinanceService for gateway-funded settlement booking (TASK 39E).
	// Records DR GATEWAY_CLEARING / CR BANK_SETTLEMENT inside the same
	// webhook tx as MarkAsSettlement and CreateEscrowFromGatewaySettlement.
	settlementFinanceService := financeApp.NewFinanceService()
	settlementFinanceService.SetLogger(log.Logger)

	canonicalFinalizationService := paymentApp.NewCanonicalFinalizationService(
		settlementFinanceService,
		orderService,
		walletService,
		log.Logger,
	)
	canonicalFinalizationService.SetAuditService(auditService)
	paymentWebhookService.SetCanonicalFinalizationService(canonicalFinalizationService)
	paymentWebhookService.SetFinanceService(settlementFinanceService)

	// ===== REFUND MODULE (TASK 34 / Phase 2a) =====
	//
	// Construct the canonical RefundService singleton AFTER orderService,
	// walletService, and outboxRepository — those are its required deps —
	// and AFTER paymentWebhookService so we can hand the refund service to
	// the webhook dispatcher in the same place.
	//
	// SetGatewayClient wires the refund-orchestration surface
	// (RefundService.InitiateGatewayRefund). All escrows are gateway-funded
	// under the canonical model; the legacy non-gateway escrow kill-switch has
	// been demolished.
	//
	// SAFETY: SetRefundService on the webhook dispatcher means refund
	// webhook acks now reach RefundService.HandleGatewayRefundAck. That
	// method updates the refund row's gateway_status / gateway_refund_id
	// and emits a money.refund_succeeded outbox event — it performs NO
	// financial mutation in this phase.
	refundService := refundApp.NewRefundService(
		orderService,
		walletService,
		outboxRepository,
	)
	refundService.SetOrderRefundStatusSyncer(orderService)
	refundService.SetGatewayClient(midtransClient, log.Logger)
	// PHASE 2B (TASK 41): wire FinanceService so refund.success webhooks
	// book the canonical reversal ledger transaction in the same tx as the
	// gateway-status flip. Reuse a fresh FinanceService instance with the
	// shared logger so observability events surface in the standard pipeline.
	refundReversalFinance := financeApp.NewFinanceService()
	refundReversalFinance.SetLogger(log.Logger)
	refundReversalFinance.SetDisputeFreezeRepo(financeRepo.NewDisputeFreezeRepository())
	refundService.SetFinanceReverser(refundReversalFinance)
	// Wire freeze releaser so the gateway ack handler has the compatibility
	// hook available when policy requires it.
	refundService.SetDisputeFreezeReleaser(refundReversalFinance)
	// CANONICAL COIN K READER: K (coins redeemed for an order) is resolved
	// from the coins domain (coins_transactions via FindSpendByReference),
	// NOT from orders.coins_used which is dead/never persisted. Wired into
	// both the refund ack pipeline and the order-side refund dispatch so the
	// CoinDelta / cashRefund computation uses the canonical coins authority.
	// The repository is constructed here (once) and reused at the coins
	// service wiring site below.
	coinsRepository := coinsRepo.NewCoinsRepository()
	refundService.SetCoinsSpendReader(coinsRepository)
	orderService.PaymentService().SetCoinsSpendReader(coinsRepository)
	paymentWebhookService.SetRefundService(refundService)
	// CANONICAL REFUND CONVERGENCE: wire RefundService back into the order
	// payment service so every PAID refund path (dispute / timeout / manual
	// / expire-with-escrow) dispatches the gateway refund + creates the
	// canonical Refund row in the same tx as the local state move. Without
	// this wire, InitiateGatewayRefundForOrder fails closed and the order
	// flow refuses to advance — preferring an audible failure over a silent
	// escrow-flip-only "refund".
	orderService.PaymentService().SetGatewayRefundInitiator(refundService)
	// H2-F2a MONEY-SAFETY: Wire refund repo as ActiveRefundChecker so
	// OrderCompletionService.Complete blocks auto-release when a refund is
	// being negotiated or awaiting gateway settlement. Without this, the
	// auto-complete worker can release escrow to seller while a refund is
	// in flight, creating a post-release money gap.
	orderService.SetActiveRefundChecker(refundRepoImpl.NewRefundRepository())

	// Build the admin-only gateway refund handler. Capturing flagEnabled
	// by value here means flipping the env var requires a process restart —
	// intentional for Phase 2a (every activation should be deliberate).
	gatewayRefundFlag := cfg.FeatureFlags.GatewayRefundInitiateEnabled
	adminRefundHandler := refundHTTP.NewAdminRefundHandler(
		refundService,
		db.Pgx(),
		gatewayRefundFlag,
		log.Logger,
		adminAuditLogger,
	)
	if gatewayRefundFlag {
		log.Logger.Warn("gateway_refund_phase2_enabled",
			zap.String("env_var", "ENABLE_GATEWAY_REFUND_PHASE2"),
			zap.String("endpoint", "POST /api/v1/admin/refunds/:refund_id/gateway/initiate"),
			zap.String("capability", "finance.refund.gateway.initiate"),
			zap.String("scope", "orchestration_and_webhook_only"),
			zap.String("financial_mutation", "none"),
			zap.String("kill_switch", "still_active_for_legacy_paths"),
		)
	} else {
		log.Logger.Info("gateway_refund_phase2_disabled",
			zap.String("env_var", "ENABLE_GATEWAY_REFUND_PHASE2"),
			zap.String("default", "false"),
		)
	}

	// H2-A: Seller refund decision handler. Uses the same refundService
	// singleton so gateway dispatch, ledger reversal, and webhook ack all
	// share the same wiring.
	sellerRefundHandler := refundHTTP.NewSellerRefundHandler(
		refundService,
		db.Pgx(),
		log.Logger,
	)

	// ===== DISPUTE MODULE =====
	// NOTE: buyerEscalationHandler is constructed AFTER disputeService below
	// because it needs both refundService and disputeService.
	// Initialize dispute repository (shared between dispute service and timeout worker)
	disputeRepository := disputeRepo.NewDisputeRepository()
	// Initialize support repository (used by SLA escalation worker)
	supportRepo := supportInfraRepo.NewSupportRepository()

	// DISPUTE ↔ WALLET INTEGRATION: Wire dispute repository to wallet service
	// This enables wallet service to check for active disputes before escrow release/refund
	walletService.SetDisputeRepository(disputeRepository)
	// STRICT MODE: Wire dispute repository to order service for entry point guards
	orderService.SetDisputeRepository(disputeRepository)
	// Initialize dispute service (requires order repository, order service, wallet service, and outbox)
	disputeService := disputeApp.NewDisputeService(
		orderRepository,
		orderService,
		walletService,
		outboxRepository,
	)
	disputeService.SetLogger(log.Logger)
	// TASK 48: wire dispute freeze authority so active disputes can freeze
	// seller withdrawable balance via the finance layer.
	disputeFreezeFinance := financeApp.NewFinanceService()
	disputeFreezeFinance.SetLogger(log.Logger)
	disputeFreezeFinance.SetDisputeFreezeRepo(financeRepo.NewDisputeFreezeRepository())
	disputeService.SetFreezeAuthority(disputeFreezeFinance)
	// Initialize dispute handler
	disputeHandler := disputeHTTP.NewDisputeHandler(
		disputeService,
		db.Pgx(),
		log.Logger,
		adminAuditLogger,
	)

	// H2-B: Buyer escalation handler. Needs both refundService (for state
	// transition) and disputeService (for dispute creation) — constructed
	// here after both are wired.
	buyerEscalationHandler := refundHTTP.NewBuyerEscalationHandler(
		refundService,
		disputeService,
		db.Pgx(),
		log.Logger,
	)

	// ===== PRICING TOKEN MODULE =====
	// Initialize pricing token service (needed by order handler)
	pricingTokenService := pricingtokenapp.NewPricingTokenService(configService)
	pricingTokenHandler := pricingtokenHTTP.NewPricingTokenHandler(
		configService,
		db.Pgx(),
		log.Logger,
	)

	// Create order handler - includes dispute + refund services for buyer protection paths
	orderHandler := orderHTTP.NewOrderHandler(
		orderQueryService,
		orderService,
		disputeService,
		refundService,
		pricingTokenService,
		roleChecker,
		db.Pgx(),
		log.Logger,
	)

	// Create admin order handler - read-only admin order management
	adminOrderHandler := orderHTTP.NewAdminOrderHandler(
		orderQueryService,
		db.Pgx(),
		log.Logger,
	)

	// ===== AUCTION MODULE =====
	auctionRepository := auctionRepo.NewAuctionRepository()
	auctionService := auctionApp.NewAuctionService(
		accountStatusChecker,
		shippingService,
		shippingOptionRepo,
		coverageRepo,
		productShippingRepo,
		outboxRepository,
		configService,
		orderService,
		roleChecker,
		log.Logger,
	)
	auctionService.SetBNRStrikeChecker(auctionApp.NewBNRStrikeChecker())
	auctionService.SetProductRepo(productRepoImpl.NewProductRepository())
	auctionHandler := auctionHTTP.NewAuctionHandler(auctionService, productRepoImpl.NewProductRepository(), pricingTokenService, db.Pgx(), log.Logger)
	// PASS_5B: admin emergency auction cancel/override (governance authority,
	// not seller authority). Reuses the same auctionService and adminAuditLogger
	// singleton as every other admin money/trust-adjacent handler.
	adminAuctionHandler := auctionHTTP.NewAdminAuctionHandler(auctionService, db.Pgx(), adminAuditLogger, log.Logger)

	// ===== SAVED ITEM MODULE (Unified Shortlist + Auction Watch) =====
	savedItemService := savedItemApp.NewSavedItemService(accountStatusChecker)
	savedItemRepo := savedItemRepoImpl.NewSavedItemRepository(db.Pgx())
	savedItemService.SetSavedItemRepository(savedItemRepo)
	savedItemService.SetAuctionRepository(*auctionRepository)
	savedItemService.SetDB(db.Pgx())
	savedItemHandler := savedItemHTTP.NewSavedItemHandler(
		savedItemService,
		db.Pgx(),
		log.Logger,
	)

	// ===== BIDDING MODULE =====
	// Initialize bidding service - read-only aggregation of user's auction bids
	biddingService := biddingApp.NewBiddingService()
	biddingHandler := biddingHTTP.NewBiddingHandler(biddingService, db.Pgx(), log.Logger)

	// ===== SHIPPING QUOTE REPOSITORY =====
	// Initialize shipping quote repository early for forSale service dependency
	// The service will be initialized later after chat service is available
	shippingQuoteRepository := shippingQuoteRepo.NewShippingQuoteRepository()

	// ===== LISTING MODULE =====
	addressRepository := addressRepoImpl.NewAddressRepository()
	forSaleService := forSaleApp.NewForSaleService(
		outboxRepository,
		roleChecker,
		actorResolver,
		productShippingRepo,
		coverageRepo,
		shippingQuoteRepository,
		addressRepository,
	)
	forSaleHandler := forSaleHTTP.NewForSaleHandler(
		forSaleService,
		db.Pgx(),
		log.Logger,
		orderRepository,
	)

	// ===== SOCIAL REPOSITORY =====
	// Create social repository for block filtering in comments and notifications
	// This must be created early as it's used by multiple services
	blockChecker := socialRepo.NewSocialRepository()

	// ===== CHAT MODULE =====
	// Create shared rate limiter for chat and realtime
	chatRateLimiter := pkgRate.NewRateLimiter()

	// Create realtime metrics (shared between hub and chat service)
	realtimeMetrics := monitoring.NewRealtimeMetrics()
	prometheus.MustRegister(realtimeMetrics)
	if redisClient != nil {
		presenceService = presence.NewService(
			db.Pgx(),
			presence.NewRedisRepository(redisClient, log.Logger),
			presence.NewDBRepository(db.Pgx()),
			log.Logger,
			outboxRepository,
		)
	}

	// accountStatusChecker (already declared above) is passed to chat and negotiation services.

	// ===== NEGOTIATION MODULE =====
	// Initialize negotiation repository
	negotiationForSaleRepo := forSaleRepo.NewForSaleRepository()

	// Initialize negotiation service
	// NEGOTIATION LIFECYCLE HARDENING: Negotiations are time-bound agreements that do NOT reserve stock
	negotiationBlockChecker := &socialBlockCheckerAdapter{db: db.Pgx(), repo: blockChecker}
	negotiationService := negotiationApp.NewNegotiationService(
		db.Pgx(),                // Use pkg/db.DB type
		negotiationForSaleRepo,  // For validation
		outboxRepository,        // For event emission
		accountStatusChecker,    // Account status enforcement
		negotiationBlockChecker, // Block enforcement: denies new negotiation when block exists
		log.Logger,              // For logging
	)

	// Initialize negotiation expire worker
	// Automatically expires negotiations after their expires_at time
	negotiationExpireWorker := negotiationWorker.NewNegotiationExpireWorker(
		negotiationService,
		log.Logger,
		negotiationWorker.DefaultNegotiationExpireConfig(),
	)
	// Default ON: write-time expiry prevents stale accepted negotiations from being checked out.
	// Disable: DISABLE_NEGOTIATION_EXPIRE_WORKER=true
	if workerEnabled("NEGOTIATION_EXPIRE_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			negotiationExpireWorker.Start()
			log.Info("NegotiationExpireWorker ENABLED",
				zap.Duration("poll_interval", negotiationWorker.DefaultNegotiationExpirePollInterval),
			)
		})
	}

	chatOrderOwnershipReader := &chatOrderOwnershipAdapter{orderRepo: orderRepository}
	chatService := chatApp.NewServiceWithDefaults(db.Pgx(), outboxRepository, chatRateLimiter, realtimeMetrics, accountStatusChecker, chatOrderOwnershipReader, log.Logger)
	chatHandler := chatHTTP.NewHandler(
		chatService,
		orderService,
		negotiationService,
		pricingTokenService,
		accountStatusChecker,
		db.Pgx(),
		log.Logger,
	)

	// Public OG share metadata endpoints (unauthenticated).
	ogHandler := ogHTTP.NewHandler(db.Pgx())

	// ===== DISCOUNT MODULE =====
	discountHandler := discountHTTP.NewDiscountHandler(
		db.Pgx(),
		roleChecker,
		log.Logger,
	)

	// ===== REALTIME MODULE =====
	// Initialize the realtime hub with metrics (singleton for connection management)
	realtimeHub := realtime.NewHubWithMetrics(log.Logger, realtimeMetrics)

	// CHAT-3: DatabaseRoomAuthorizer — membership check for subscribe governance gate.
	// Uses dedicated repo instances (reads only; no transaction shared with request path).
	realtimeChatRepo := chatRepo.NewChatRepository()
	realtimeOrderRepo := orderRepo.NewOrderRepository()
	realtimeRoomAuthorizer := realtime.NewDatabaseRoomAuthorizer(db.Pgx(), realtimeChatRepo, realtimeOrderRepo, log.Logger)

	// CHAT-3: SubscribeGate — wraps roomAuthorizer + accountStatusChecker + pure evaluator.
	// Mandatory governance gate; called in ReadPump before every hub.Subscribe().
	realtimeSubscribeGate := realtime.NewSubscribeGate(realtimeRoomAuthorizer, accountStatusChecker, log.Logger)

	// Initialize the realtime WebSocket handler with the subscribe governance gate.
	realtimeHandler := realtime.NewHandler(realtimeHub, realtimeSubscribeGate, chatRateLimiter, log.Logger)

	// Create an adapter for the outbox repository to match realtime.OutboxRepository interface
	// The outbox repository returns repository.Event, we need to convert to realtime.Event
	outboxRepositoryAdapter := &realtimeOutboxRepositoryAdapter{repo: outboxRepository}

	// Initialize the realtime worker (consumes chat.message.sent events).
	// CHAT-3: accountStatusChecker is threaded through to the Dispatcher for per-subscriber
	// broadcast governance (fresh lifecycle check per delivery, not subscribe-time state).
	realtimeWorker := realtime.NewWorker(
		db.Pgx(),
		outboxRepositoryAdapter,
		realtimeHub,
		accountStatusChecker,
		log.Logger,
		realtime.DefaultWorkerConfig(),
	)
	if workerEnabled("REALTIME_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			realtimeWorker.Start()
			log.Info("RealtimeWorker ENABLED - CHAT-3 WS governance active",
				zap.Duration("poll_interval", realtime.DefaultPollInterval),
				zap.Int("batch_size", realtime.DefaultBatchSize),
			)
		})
	}

	// ===== SELLER MODULE =====
	// Initialize seller and subscription repositories
	sellerRepository := sellerRepoImpl.NewSellerRepository()
	sellerSubscriptionRepo := subscriptionRepoImpl.NewSellerSubscriptionRepository()

	// Initialize subscription service
	subscriptionService := subscriptionApp.NewSubscriptionService(
		db,
		sellerSubscriptionRepo,
		sellerRepository,
	)

	// Initialize onboarding validation service (SINGLE SOURCE OF TRUTH)
	// Used by both payment flow and onboarding endpoint to ensure consistent validation
	onboardingService := subscriptionApp.NewSellerOnboardingService(
		userRepoImpl.NewUserRepository(db.Pgx()),
		sellerRepository,
		addressRepository,
	)

	// Initialize subscription payment service with STRICT MODE onboarding validation
	subscriptionPaymentService := subscriptionApp.NewSellerSubscriptionPaymentService(
		db,
		paymentRepo,
		sellerSubscriptionRepo,
		sellerRepository,
		userRepoImpl.NewUserRepository(db.Pgx()),
		onboardingService, // ← SINGLE SOURCE OF TRUTH for validation
		financeApp.NewFinanceService(),
		outboxRepo.NewOutboxRepository(db.Pgx()),
		sellerSubscriptionRepo,
	)

	// Inject subscription payment service into payment webhook service
	// This enables STRICT MODE: no subscription payment without complete onboarding
	paymentWebhookService.SetSubscriptionPaymentService(subscriptionPaymentService)

	// Initialize seller handler
	sellerHandler := sellerHTTP.NewSellerHandler(
		subscriptionService,
		db.Pgx(),
		log.Logger,
		userRepoImpl.NewUserRepository(db.Pgx()),
		sellerRepoImpl.NewSellerRepository(),
		onboardingService, // ← SINGLE SOURCE OF TRUTH for validation
		paymentRepo,
		midtransClient,
		sellerSubscriptionRepo,
		cfg.App.FrontendURL,
		subscriptionPaymentService,
	)

	// Admin handler for seller subscription config (singleton row read/update)
	adminSubscriptionConfigHandler := subscriptionHTTP.NewAdminSubscriptionConfigHandler(
		sellerSubscriptionRepo,
		db.Pgx(),
		log.Logger,
	)

	// PASS_18W: Admin handler for payment method fee config (list/get/update/preview)
	adminPaymentMethodHandler := paymentmethodHTTP.NewAdminPaymentMethodHandler(
		paymentMethodRepository,
		db.Pgx(),
		log.Logger,
	)

	// Admin handler for manual subscription payment recovery (webhook miss scenario)
	adminSubscriptionRecoveryHandler := subscriptionHTTP.NewAdminSubscriptionRecoveryHandler(
		subscriptionPaymentService,
		db.Pgx(),
		log.Logger,
	)

	// ===== USER PROFILE SERVICE (moved here after seller repositories are available) =====
	// UserProfileService handles cross-domain composition for user profiles
	// It orchestrates between user, seller, and subscription domains
	userProfileService := userApp.NewUserProfileService(
		userRepository,
		sellerRepository,
		sellerSubscriptionRepo,
		outboxRepository,
		firebaseClient,
		db.Pgx(),
	)

	// User handler for profile endpoints - now uses service layer
	userHandler := userHTTP.NewUserHandler(userProfileService, db.Pgx(), log.Logger)

	// ===== REPOSITORIES =====
	// tradeRepository := tradeRepoImpl.NewTradeRepository()
	// ratingRepository := ratingRepoImpl.NewTradeRatingRepository()

	// =====================================================================
	// WORKERS - Initialize ALL workers
	// =====================================================================
	//
	// Worker enablement is gated by `DISABLE_<NAME>` env vars (see
	// workerEnabled below). The gate exists so corpus-generation runs can
	// deterministically pin a failure mode (e.g. halt the auto-complete
	// worker while exercising a refund path). Defaults reflect current
	// production posture; flipping the env var inverts the default.

	// 1. Payment Expiry Worker
	//
	// Detects expired pending payments and expires associated orders.
	// Uses the fully-wired orderService for gateway-funded refund on Expire().
	//
	// DANGEROUS DORMANT: operator must ACK to enable — ensures awareness
	// that expiry triggers gateway refunds for PAID orders with escrow.
	// GUARD: requires ACK_DANGEROUS_PAYMENT_EXPIRY_WORKER=true in addition
	// to DISABLE_PAYMENT_EXPIRY_WORKER=false.
	paymentExpiryWorker := worker.NewPaymentExpiryWorker(
		db,
		orderService,
		log.Logger,
		worker.Config{
			PollInterval: 1 * time.Minute,
			BatchSize:    100,
		},
	)
	if workerEnabled("PAYMENT_EXPIRY_WORKER", false, log.Logger) {
		dangerousDormantGuard("PAYMENT_EXPIRY_WORKER", log.Logger)
		workerStartups = append(workerStartups, func() {
			paymentExpiryWorker.Start()
			log.Info("PaymentExpiryWorker started",
				zap.Duration("poll_interval", 1*time.Minute),
				zap.Int("batch_size", 100),
			)
		})
	} else {
		log.Warn("PaymentExpiryWorker disabled — expired pending payments will not be auto-expired; gateway refunds for PAID orders with escrow will not be triggered",
			zap.String("enable", "DISABLE_PAYMENT_EXPIRY_WORKER=false + ACK_DANGEROUS_PAYMENT_EXPIRY_WORKER=true"),
		)
		_ = paymentExpiryWorker
	}

	// 1b. Order Payment Timeout Worker - Expires orphan pending_payment orders
	//
	// COMPLEMENTARY to PaymentExpiryWorker:
	// - PaymentExpiryWorker scans payments table → handles orders WITH payment rows.
	// - This worker scans orders table → handles orders WITHOUT payment rows
	//   (buyer created order but never called /checkout/payment).
	//
	// Delegates to OrderService.Expire() for all business logic (stock restore,
	// coins refund, outbox events). No gateway call expected because no payment
	// was ever made.
	orderPaymentTimeoutWorker := worker.NewOrderPaymentTimeoutWorker(
		db,
		orderService,
		log.Logger,
		worker.DefaultOrderPaymentTimeoutConfig(),
	)
	if workerEnabled("ORDER_PAYMENT_TIMEOUT_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			orderPaymentTimeoutWorker.Start()
			log.Info("OrderPaymentTimeoutWorker started",
				zap.Duration("poll_interval", worker.DefaultPaymentTimeoutPollInterval),
				zap.Int("batch_size", worker.DefaultPaymentTimeoutBatchSize),
			)
		})
	}

	// 2. Reconciliation Worker - MOVED below after all dependencies are initialized

	// 3. Order Auto-Complete Worker - Completes orders after auto_release_at deadline
	//
	// BUSINESS RULE: Auto-complete timer starts when seller marks order as shipped.
	// Buyer has 5 days to confirm or dispute.
	// Buyer may extend once (+3 days) near expiry.
	//
	// SAFETY GUARDS (enforced in OrderService.Complete):
	// - status IN ('shipped', 'delivered') (timer starts at shipped)
	// - Only processes orders with has_dispute = false
	// - Only processes orders with escrow_status = "holding"
	orderAutoCompleteWorker := worker.NewOrderAutoCompleteWorker(
		db,
		orderService,
		log.Logger,
		worker.DefaultOrderAutoCompleteConfig(),
	)
	if workerEnabled("ORDER_AUTO_COMPLETE_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			orderAutoCompleteWorker.Start()
			log.Info("OrderAutoCompleteWorker started",
				zap.Duration("poll_interval", worker.DefaultAutoCompletePollInterval),
				zap.Int("batch_size", worker.DefaultAutoCompleteBatchSize),
			)
		})
	}

	// =============================================================================
	// MONITORING & METRICS — constructed BEFORE workers so SetMetricsRecorder
	// happens-before the worker's run() goroutine reads w.metrics. This avoids a
	// data race on the metrics pointer and ensures the very first batch is
	// already attributable.
	// =============================================================================
	monitoringService := monitoring.NewMonitoringService(db.Pgx().Pool(), log.Logger)
	systemHealthHandler := monitoring.NewSystemHealthHandler(monitoringService, realtimeHub)

	// Read-only metrics collector that exposes system health as Prometheus
	// metrics AND accepts sink-only hooks from outbox/projection workers.
	metricsCollector := monitoring.NewMetricsCollector(monitoringService)
	prometheus.MustRegister(metricsCollector)

	// 4. Outbox Worker
	//
	// OUTBOX_MAX_ATTEMPTS env override lowers the canonical retry cap (default
	// 20) so corpus-generation runs can reach dead_letter through the natural
	// retry-exhaustion path in handleFailureInTx within a bounded time
	// window. Production posture is preserved when the env is unset.
	outboxCfg := worker.DefaultOutboxWorkerConfig()
	if raw := strings.TrimSpace(os.Getenv("OUTBOX_MAX_ATTEMPTS")); raw != "" {
		if v, parseErr := strconv.Atoi(raw); parseErr == nil && v > 0 {
			outboxCfg.MaxAttempts = v
			log.Info("outbox_max_attempts_override",
				zap.Int("max_attempts", v),
				zap.String("source", "env"),
			)
		} else {
			log.Warn("outbox_max_attempts_invalid_ignored",
				zap.String("value", raw),
			)
		}
	}
	outboxWorker := worker.NewOutboxWorker(
		db.Pgx(),
		log.Logger,
		outboxCfg,
	)
	// Attach metrics sink BEFORE Start so the worker's polling goroutine
	// observes a non-nil recorder from its first poll onward.
	outboxWorker.SetMetricsRecorder(metricsCollector)

	// O2B-P1: OrderAutoCompleteWorker liveness — constructed above metricsCollector
	// so wired here after the collector is ready.
	orderAutoCompleteWorker.SetMetricsRecorder(metricsCollector)
	workerStartups = append(workerStartups, func() {
		outboxWorker.Start()
		log.Info("OutboxWorker ENABLED - Safe activation mode")
	})

	// =============================================================================
	// OUTBOX WORKER — SAFE ACTIVATION PLAN
	// =============================================================================
	//
	// STEP 1: Enable worker with LIMITED handlers (safe activation mode)
	// - Worker processes events from outbox table
	// - Only enabled handlers will execute
	// - Comment/uncomment handlers below to control activation
	//
	// ACTIVATION PLAN:
	// - STEP 1: Worker enabled, ALL handlers DISABLED (test infrastructure)  [DONE]
	// - STEP 2: Enable negotiation handlers (chat integration)                [DONE]
	// - STEP 3: Enable notification handlers (push/in-app)                   [DONE — NOTIFICATION-ACTIVATION-1]
	// - STEP 4: Enable moderation handlers (content soft-delete)
	// - STEP 5: Enable promotion handlers (auto-stop)
	// - STEP 6: Enable risk handlers (scoring)
	// - STEP 7: Enable user ban handler (safe refunds)
	//
	// MONITORING:
	// - Check logs for "Event processed successfully"
	// - Monitor outbox table for stuck events
	// - Verify no duplicate effects
	// (the corresponding `OutboxWorker ENABLED` log moved into the deferred
	//  startup closure above so it fires at actual start time)

	// 4.1. Setup notification handlers with push service.
	// Full handler set active (NOTIFICATION-ACTIVATION-1). Social types (chat_message, comment,
	// comment.reply, seller.response) receive push via FCM. CommerceCritical push remains
	// suppressed (allowPush=false in direct-insert handlers) — worker-batch notification suppression.
	//
	// Z6 PUSH RELIABILITY: DeliveryLogger + DBPushRetryQueue now wired.
	// Failed pushes are enqueued for retry (max 10 attempts, 24h window).
	deliveryLogger := notificationService.NewDeliveryLogger(db.Pgx(), log.Logger)
	pushRetryQueue := worker.NewDBPushRetryQueue(db.Pgx())

	var pushSender worker.PushSender
	if firebaseClient != nil {
		pushSender = notificationService.NewPushService(firebaseClient, db.Pgx().Pool(), pushRetryQueue, log.Logger)
		log.Info("PushSender initialized with retry queue (Z6 reliability active)")
	} else {
		log.Warn("Firebase client not available - notifications will be created but push will be skipped")
	}

	// N1: notificationBlockCheckerAdapter — wraps social repo with a self-managed tx so
	// policy.BlockPolicy never passes nil as the transaction. Fixes the nil-tx bug where
	// SocialRepositoryImpl.ExistsBlock would type-assert nil.(db.Tx) and return
	// "invalid transaction type", causing Social to fail-closed and Commerce to anonymize
	// without a real block relation.
	notifBlockChecker := &notificationBlockCheckerAdapter{db: db.Pgx(), repo: blockChecker}

	// N2: notificationMuteCheckerAdapter — wraps social repo with a self-managed tx so
	// policy.MutePolicy never passes nil as the transaction. Mirrors the N1 block checker
	// adapter pattern: policy interface is tx-free; adapter owns db.WithTx boundary.
	notifMuteChecker := &notificationMuteCheckerAdapter{db: db.Pgx(), repo: blockChecker}

	// CHAT-5 + C6C: Mute governance for chat notification delivery.
	// Default: enforce mode — suppress muted chat notifications (C6C promotion).
	// Rollback: set MUTE_CHAT_NOTIFICATION_ENFORCE=false to revert to shadow mode.
	chatMuteMode := notificationPolicy.MuteEnforce
	if os.Getenv("MUTE_CHAT_NOTIFICATION_ENFORCE") == "false" {
		chatMuteMode = notificationPolicy.MuteShadow
	}
	chatMutePolicy := notificationPolicy.NewMutePolicy(notifMuteChecker, chatMuteMode)
	log.Info("Chat mute policy initialized", zap.String("mode", string(chatMuteMode)))

	// NOTIFICATION-ACTIVATION-1: Full handler set. All audited groups active.
	// Dormant by design: SLA handlers (unregistered), order.dispute_open (unregistered),
	// order.confirmation_extended (unregistered). Commerce push suppressed (worker-batch notification suppression).
	// Z6: DeliveryLogger now wired (interface-mismatch debt resolved).
	_, notifEventHandler := outboxWorker.SetupNotificationHandlers(db.Pgx(), notifBlockChecker, pushSender, accountStatusChecker, chatMutePolicy)
	notifEventHandler.SetDeliveryLogger(deliveryLogger)
	log.Info("FULL notification event handlers registered (NOTIFICATION-ACTIVATION-1, Z6 delivery logger active)")

	// ─────────────────────────────────────────────────────────────────────────
	// Z6 PUSH RELIABILITY WORKERS
	// ─────────────────────────────────────────────────────────────────────────

	// Z6-1. PushRetryWorker — retries failed FCM pushes with exponential backoff.
	// Default ON: without this, any FCM failure is a permanent miss for the user.
	// Backoff: 1m → 5m → 15m → 1h → 4h → 8h. Max 10 attempts / 24h window.
	// Disable: DISABLE_PUSH_RETRY_WORKER=true
	pushRetryWorker := worker.NewPushRetryWorker(db.Pgx(), pushSender, log.Logger)
	pushRetryWorker.SetDeliveryLogger(deliveryLogger)
	if workerEnabled("PUSH_RETRY_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			pushRetryWorker.Start()
			log.Info("PushRetryWorker started (Z6 reliability layer)",
				zap.Duration("poll_interval", worker.DefaultPushRetryPollInterval),
				zap.Int("max_attempts", 10),
				zap.Duration("retry_window", 24*time.Hour),
			)
		})
	} else {
		_ = pushRetryWorker
	}

	// Z6-2. CleanupWorker — deletes old delivery logs + expired retry queue entries.
	// Default ON: delivery log table grows unbounded; orphaned retry rows accumulate.
	// Safe: only deletes rows older than 30 days (logs) or past expires_at (retries).
	// Disable: DISABLE_NOTIFICATION_CLEANUP_WORKER=true
	notificationCleanupWorker := worker.NewCleanupWorker(db.Pgx(), log.Logger)
	if workerEnabled("NOTIFICATION_CLEANUP_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			notificationCleanupWorker.Start()
			log.Info("NotificationCleanupWorker started (Z6 hygiene layer)",
				zap.Duration("retention", worker.DefaultCleanupRetention),
				zap.Duration("interval", worker.DefaultCleanupInterval),
			)
		})
	} else {
		_ = notificationCleanupWorker
	}

	// Z6-3. IdempotencyCleanupWorker — deletes old idempotency_records older than 30 days.
	// Default ON: idempotency_records table grows unbounded without cleanup.
	// Safe: only deletes response-cache rows (NOT ledger/outbox/refund/withdrawal idempotency keys).
	// Retention: 30 days default, 7-day minimum enforced by constructor.
	// Disable: DISABLE_IDEMPOTENCY_CLEANUP_WORKER=true
	idempotencyCleanupWorker := worker.NewIdempotencyCleanupWorker(
		db.Pgx(),
		log.Logger,
		worker.DefaultIdempotencyCleanupConfig(),
	)
	if workerEnabled("IDEMPOTENCY_CLEANUP_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			idempotencyCleanupWorker.Start()
			log.Info("IdempotencyCleanupWorker started (Z6 hygiene layer)",
				zap.Int("retention_days", worker.DefaultIdempotencyRetentionDays),
				zap.Duration("interval", worker.DefaultIdempotencyCleanupPollInterval),
			)
		})
	} else {
		_ = idempotencyCleanupWorker
	}

	// 4.2. Create notification HTTP handlers
	notificationHandler := notificationHTTP.NewNotificationHandlerWithDefaults(db.Pgx(), log.Logger)
	fcmTokenHandler := notificationHTTP.NewFCMTokenHandler(db.Pgx(), log.Logger)

	// =============================================================================
	// =============================================================================
	// STEP 2: NEGOTIATION EVENT HANDLERS - CHAT INTEGRATION
	// =============================================================================
	//
	// Enables chat room auto-creation for negotiations:
	// - negotiation.started → creates negotiation chat room
	// - negotiation.message_sent → creates proposal messages
	//
	// IDEMPOTENCY: Event ID used as idempotency key
	// NO CROSS-DOMAIN WRITE: Chat handler does not write to negotiation_sessions
	outboxWorker.SetupNegotiationHandlers(
		db.Pgx(),
		chatService,
		notifEventHandler,
	)
	log.Info("Negotiation event handlers registered (chat + notification fanout) - STEP 2")

	// =============================================================================
	// STEP 2.1: ORDER → CHAT LINK CONSUMER (CROSS-DOMAIN TX CLEANUP)
	// =============================================================================
	//
	// Consumes order.chat_link_requested events emitted by the order tx and
	// performs the canonical buyer↔seller direct-room linkage in the chat
	// domain. Replaces the previous inline cross-domain mutation in
	// OrderCreationService (chat_rooms was being written from the order tx).
	//
	// FAILURE SEMANTICS: eventual consistency. Order is canonical; chat
	// linkage retries via outbox. See RUNTIME-INVARIANTS §1.2, §6.4.
	outboxWorker.SetupOrderChatLinkHandler(chatService)
	log.Info("Order chat-link consumer registered (decoupled from order tx)")

	// PROMOTION DOMAIN - RUNTIME HARDENING PACK V1
	// =============================================================================
	//
	// The Promotion domain manages paid promotion of forSales, auctions, and external products.
	//
	// RUNTIME COMPONENTS:
	// 1. Event-driven auto-stop: for_sale.sold, for_sale.withdrawn, auction.ended, auction.cancelled
	// 2. Safety worker: periodic sweep for missed events
	// 3. Operability checker: validates target state before activation
	//
	// IMPORTANT: These components MUST be wired at boot for promotion to be runtime-truthful.

	// Create promotion repository
	promotionRepository := promotionInfraRepo.NewPromotionRepository()

	// Create operability checker (validates forSale/auction state)
	// This connects Promotion to ForSale/Auction domains for real-time operability
	operabilityChecker := promotionApp.NewOperabilityCheckerImpl(db.Pgx(), promotionRepository)

	// Create promotion service (core business logic)
	promotionService := promotionApp.NewPromotionService(operabilityChecker)
	// Wire outbox emitter so external_product.review.* events are emitted
	// atomically with admin review state transitions.
	promotionService.SetOutboxEmitter(outboxRepository)

	// P3A — Promotion discovery service for feed injection.
	// Provides read-time operability-filtered active promotions.
	promotionDiscoveryService := promotionApp.NewDiscoveryService(db.Pgx(), operabilityChecker)

	// Wire canonical promotion service into payment webhook so that
	// promotion-package billing uses the same governance authority as
	// the normal runtime (OperabilityCheckerImpl, not the V1 placeholder).
	paymentWebhookService.SetPromotionService(promotionService)

	// 4.3. Setup promotion event handlers — moved to after SetupSellerSubscriptionExpiredHandler
	// (P5B-C ordering constraint: seller governance events require fanout composition
	// with notification + auction-cancellation handlers registered earlier).

	// 4.4. Promotion safety worker (mechanism B — periodic sweep)
	// Catches missed events and data inconsistencies.
	// P4C3: ENABLED — canonical entity-based finalization verified in P4B.
	// OperabilityCheckerImpl read-time gate remains as defense in depth.
	promotionSafetyWorker := worker.NewPromotionSafetyWorker(
		operabilityChecker,
		promotionService,
		db,
		log.Logger,
		worker.DefaultPromotionSafetyWorkerConfig(),
	)
	if workerEnabled("PROMOTION_SAFETY_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			promotionSafetyWorker.Start()
			log.Info("PromotionSafetyWorker started (every 5m, batch 100, safety net for missed events)")
		})
	}

	// 4.5. Promotion expiration worker (mechanism C — ownership validity window)
	// Canonical time-based worker: marks expired ownerships + stops all active instances.
	// This is the only mechanism that MUST be a background worker (time-based expiry
	// cannot be handled at read time alone).
	// P4C2: ENABLED default-ON — canonical finalization verified in P4B.
	promotionExpirationWorker := worker.NewPromotionExpirationWorker(
		promotionService,
		db.Pgx(),
		log.Logger,
		worker.DefaultPromotionExpirationWorkerConfig(),
	)
	if workerEnabled("PROMOTION_EXPIRATION_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			promotionExpirationWorker.Start()
			log.Info("PromotionExpirationWorker started (hourly, Phase 1: expired ownerships, Phase 2: duration exhaustion)")
		})
	}

	// =============================================================================
	// ORDER OVERDUE REMINDER WORKER - OVERDUE ENFORCEMENT CLOSURE
	// =============================================================================
	// Sends tiered notifications to sellers and buyers for overdue orders.
	// - Tier 1 (Day 0 overdue): Notify seller only
	// - Tier 2 (Day 3 overdue): Notify seller + buyer
	// - Tier 3 (Day 7 overdue): Strong notification to buyer + seller
	//
	// Uses order_overdue_reminders table for deduplication - each order+tier
	// combination is tracked to prevent spam.
	overdueReminderRepo := worker.NewOrderOverdueReminderRepository()
	orderOverdueReminderWorker := worker.NewOrderOverdueReminderWorker(
		db,
		overdueReminderRepo,
		outboxRepository,
		log.Logger,
	)
	// Default ON: overdue reminders are the only seller accountability mechanism.
	// Requires migration 000168 (order_overdue_reminders table) for deduplication.
	// Disable: DISABLE_ORDER_OVERDUE_REMINDER_WORKER=true
	if workerEnabled("ORDER_OVERDUE_REMINDER_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			orderOverdueReminderWorker.Start()
			log.Info("OrderOverdueReminderWorker started",
				zap.Duration("poll_interval", worker.DefaultOverdueReminderPollInterval),
				zap.Int("batch_size", worker.DefaultOverdueReminderBatchSize),
			)
		})
	}

	// =============================================================================
	// SLA ESCALATION WORKER - Timely response notifications
	// =============================================================================
	// Enforces SLA policies for support tickets and disputes:
	// - First Response: Warning at 45min, Breach at 60min (tickets), 90/120min (disputes)
	// - Resolution: Warning at 75% SLA, Breach at 100% SLA
	//
	// Emits events: sla.warning.first_response, sla.breach.first_response,
	//            sla.warning.resolution, sla.breach.resolution
	//
	// IDEMPOTENCY: Only 1 warning/breach per ticket/dispute per stage
	//
	// Default OFF: emitted SLA events are not registered in the outbox event
	// registry and have no consumer handlers (sla_notification_handlers.go was
	// deleted). The worker also has an idempotency-key format mismatch between
	// eventExists and emitEvent, and ticket checks incorrectly emit dispute.*
	// event types. Gate OFF until the SLA notification pipeline is built.
	// Enable: DISABLE_SLA_ESCALATION_WORKER=false
	slaEscalationWorker := worker.NewSLAEscalationWorker(
		db,
		supportRepo, // SLA repository implementation
		outboxRepository,
		log.Logger,
	)
	if workerEnabled("SLA_ESCALATION_WORKER", false, log.Logger) {
		workerStartups = append(workerStartups, func() {
			slaEscalationWorker.Start()
			log.Info("SLAEscalationWorker ENABLED",
				zap.Duration("poll_interval", worker.DefaultSLAEscalationPollInterval),
				zap.Int("batch_size", worker.DefaultSLAEscalationBatchSize),
			)
		})
	}

	// 5. Projection Worker - Maintains read models by consuming outbox events.
	// Default OFF: schema aligned as of migration 000142 (BATCH F2).
	// Enable once dev/staging smoke test (BATCH F3) passes.
	// Set DISABLE_PROJECTION_WORKER=false to enable.
	projectionWorker := worker.NewProjectionWorker(
		db.Pgx(),
		log.Logger,
		worker.DefaultProjectionWorkerConfig(),
	)
	// Attach metrics sink up-front. The recorder is sink-only so attaching it
	// while the worker is disabled is a no-op until projectionWorker.Start() runs.
	projectionWorker.SetMetricsRecorder(metricsCollector)
	if workerEnabled("PROJECTION_WORKER", false, log.Logger) {
		workerStartups = append(workerStartups, func() {
			projectionWorker.Start()
			log.Info("ProjectionWorker started")
		})
	}
	projectionAdminHandler := worker.NewProjectionAdminHandler(projectionWorker, log.Logger)

	// 6. Auction Start Worker - Activates scheduled auctions
	auctionStartWorker := worker.NewAuctionStartWorker(
		db,
		auctionService,
		log.Logger,
		worker.DefaultAuctionStartWorkerConfig(),
	)
	if workerEnabled("AUCTION_START_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			auctionStartWorker.Start()
			log.Info("AuctionStartWorker started")
		})
	}

	// 7. Auction End Worker - Ends expired auctions
	// Transitions active auctions past their end_time to ended status.
	// No overlap with AuctionSettlementWorker (different lifecycle phase).
	auctionEndWorker := worker.NewAuctionEndWorker(
		db,
		auctionService,
		log.Logger,
		worker.DefaultAuctionEndWorkerConfig(),
	)
	// O2B-P1: AuctionEndWorker liveness metrics.
	auctionEndWorker.SetMetricsRecorder(metricsCollector)
	if workerEnabled("AUCTION_END_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			auctionEndWorker.Start()
			log.Info("AuctionEndWorker started")
		})
	}

	// 7b. Auction Settlement Worker - BNR timeout enforcement
	// Detects waiting_settlement auctions past settlement_deadline, transitions
	// to expired_bnr, emits auction_bnr_detected outbox event. No money, no
	// order, no inventory — pure status flip + trust event.
	// RUNTIME_PROVEN B81 2026-05-26. Default ON.
	auctionSettlementWorker := worker.NewAuctionSettlementWorker(
		db.Pgx(),
		auctionService,
		outboxRepository,
		log.Logger,
		worker.DefaultAuctionSettlementWorkerConfig(),
	)
	if workerEnabled("AUCTION_SETTLEMENT_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			auctionSettlementWorker.Start()
			log.Info("AuctionSettlementWorker started")
		})
	}

	// 7c. BNR Decay Worker - daily forgiveness layer for buyer BNR strikes.
	// If a buyer's most-recent active strike is older than 180 days, decays
	// the oldest active strike (sets decayed_at). One decay per buyer per run.
	// No money mutation, no order changes — pure trust-signal housekeeping.
	// Disable: DISABLE_BNR_DECAY_WORKER=true
	bnrDecayWorker := worker.NewBNRDecayWorker(db.Pgx(), log.Logger)
	if workerEnabled("BNR_DECAY_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			bnrDecayWorker.Start()
		})
	}

	// 8. Seller Subscription Expiry Worker - hourly active→expired
	// transition. Per-row FOR UPDATE lock, deterministic outbox idempotency
	// keys, 1-entity-1-transaction; blast radius is seller_subscriptions table
	// plus outbox rows.
	sellerSubscriptionExpiryWorker := worker.NewSellerSubscriptionExpiryWorker(
		db,
		sellerSubscriptionRepo,
		sellerRepository,
		outboxRepository,
		log.Logger,
	)
	if workerEnabled("SELLER_SUBSCRIPTION_EXPIRY_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			sellerSubscriptionExpiryWorker.Start()
		})
	} else {
		_ = sellerSubscriptionExpiryWorker
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Z4 WORKER COMPLETION — P1 operational workers wired to root
	// ─────────────────────────────────────────────────────────────────────────

	// Z4-1. Outbox Archival Worker — archives succeeded events older than RetentionDays.
	// Default ON: outbox table grows unbounded without archival (no other deletion mechanism).
	// Disable: DISABLE_OUTBOX_ARCHIVAL_WORKER=true
	outboxArchivalWorker := worker.NewOutboxArchivalWorkerFromConfig(
		db,
		db.Pgx(),
		log.Logger,
		cfg,
	)
	if workerEnabled("OUTBOX_ARCHIVAL_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			outboxArchivalWorker.Start()
			log.Info("OutboxArchivalWorker started",
				zap.Int("retention_days", cfg.Outbox.RetentionDays),
				zap.Int("batch_size", cfg.Outbox.ArchiveBatchSize),
			)
		})
	} else {
		_ = outboxArchivalWorker
	}

	// Z4-2. Order Overdue Cancel Worker — auto-cancels paid orders past shipment deadline.
	// Default ON: without this, overdue orders permanently freeze buyer escrow.
	// Uses canonical OrderService.CancelOverdue → InitiateGatewayRefundForOrder path.
	// Disable: DISABLE_ORDER_OVERDUE_CANCEL_WORKER=true
	orderOverdueCancelWorker := worker.NewOrderOverdueCancelWorker(
		db,
		orderService,
		log.Logger,
		worker.DefaultOrderOverdueCancelConfig(),
	)
	if workerEnabled("ORDER_OVERDUE_CANCEL_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			orderOverdueCancelWorker.Start()
			log.Info("OrderOverdueCancelWorker started",
				zap.Duration("poll_interval", worker.DefaultOverdueCancelPollInterval),
				zap.Int("batch_size", worker.DefaultOverdueCancelBatchSize),
			)
		})
	} else {
		_ = orderOverdueCancelWorker
	}

	// Z4-3. Subscription Reconciliation Worker — recovers orphaned subscription payments.
	// Default ON: sellers who paid but whose subscription record was not created
	// (due to webhook failure) remain expired indefinitely without this worker.
	// Disable: DISABLE_SUBSCRIPTION_RECONCILIATION_WORKER=true
	subscriptionReconciliationWorker := worker.NewSubscriptionReconciliationWorker(
		db.Pgx().Pool(),
		log.Logger,
		worker.DefaultSubscriptionReconciliationConfig(),
		subscriptionPaymentService,
	)
	if workerEnabled("SUBSCRIPTION_RECONCILIATION_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			subscriptionReconciliationWorker.Start()
			log.Info("SubscriptionReconciliationWorker started",
				zap.Duration("interval", worker.DefaultSubscriptionReconciliationInterval),
			)
		})
	} else {
		_ = subscriptionReconciliationWorker
	}

	// Z4-4. Withdrawal Monitoring Worker — read-only alert on stuck withdrawals.
	// Default ON: stuck payouts are invisible to ops without this monitor.
	// READ-ONLY: does NOT mutate ledger or withdrawal state.
	// Disable: DISABLE_WITHDRAWAL_MONITORING_WORKER=true
	withdrawalMonitoringWorker := worker.NewWithdrawalMonitoringWorker(
		db,
		log.Logger,
		worker.DefaultWithdrawalMonitoringConfig(),
	)
	if workerEnabled("WITHDRAWAL_MONITORING_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			withdrawalMonitoringWorker.Start()
			log.Info("WithdrawalMonitoringWorker started",
				zap.Duration("interval", worker.DefaultWithdrawalMonitoringInterval),
				zap.Duration("stuck_threshold", worker.DefaultWithdrawalStuckThreshold),
			)
		})
	} else {
		_ = withdrawalMonitoringWorker
	}

	// 9. SellerMonthlyMetricsWorker — generates monthly seller performance snapshots.
	// Default ON: writes only seller_monthly_metrics table (measurement only).
	// Safe: idempotent per (seller_id, year, month) UNIQUE constraint + existence check.
	// No tier/profile/subscription/ledger mutation. 1 seller = 1 transaction.
	// Disable: DISABLE_SELLER_METRICS_WORKER=true
	sellerMetricsWorker := worker.NewSellerMonthlyMetricsWorker(
		db,
		sellerRepository,
		orderRepository,
		log.Logger,
	)
	if workerEnabled("SELLER_METRICS_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			sellerMetricsWorker.Start()
			log.Info("SellerMonthlyMetricsWorker started (fulfillment measurement)",
				zap.Duration("interval", worker.DefaultSellerMetricsInterval),
			)
		})
	} else {
		_ = sellerMetricsWorker
	}

	// 10. SellerReputationRecomputeWorker — CANONICAL REPUTATION AUTHORITY.
	// Queries rolling 90-day window from base tables; writes seller_reputation_state.
	// Late refunds/rating invalidations/dispute resolutions self-correct on next cycle.
	// Never reads seller_monthly_metrics (analytics-only; separation enforced).
	// Default ON. Disable: DISABLE_SELLER_REPUTATION_RECOMPUTE_WORKER=true
	sellerReputationRecomputeWorker := worker.NewSellerReputationRecomputeWorker(
		db,
		sellerRepository,
		outboxRepository,
		log.Logger,
	)
	if workerEnabled("SELLER_REPUTATION_RECOMPUTE_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			sellerReputationRecomputeWorker.Start()
			log.Info("SellerReputationRecomputeWorker started (reputation authority)",
				zap.Duration("interval", worker.DefaultReputationRecomputeInterval),
				zap.Int("window_days", worker.DefaultReputationWindowDays),
			)
		})
	} else {
		_ = sellerReputationRecomputeWorker
	}

	// 11. System Monitoring Worker - READ-ONLY production monitoring checks
	// Runs every 5 minutes, does NOT mutate any data
	systemMonitoringWorker := worker.NewSystemMonitoringWorker(
		db.Pgx().Pool(),
		log.Logger,
		worker.DefaultSystemMonitoringConfig(),
	)
	// TEMPORARILY DISABLED: systemMonitoringWorker.Start()

	// 15-16. MonitoringService + MetricsCollector are constructed earlier (right
	// before outboxWorker) so SetMetricsRecorder can wire into workers before
	// their goroutines start. See the "MONITORING & METRICS" block above.
	_ = monitoringService
	_ = metricsCollector

	// =====================================================================
	// PAYOUT MODULE - Sandbox/Internal-Safe Rail Only
	// =====================================================================
	//
	// PAYMENT PROVIDER ARCHITECTURE:
	// ┌─────────────────────────────────────────────────────────────────────────────┐
	// │ INCOMING PAYMENTS (buyer → platform): Midtrans (canonical provider)         │
	// │ PAYOUTS (platform → seller): Sandbox/internal-safe rail only                │
	// │ REAL PAYOUT PROVIDER: TBD                                                  │
	// └─────────────────────────────────────────────────────────────────────────────┘
	//
	// Initialize payout repositories
	withdrawRepo := financeRepo.NewWithdrawRepository()

	// Create gateway based on provider configuration
	// NOTE: Provider is already validated by cfg.ValidatePayoutGatewayProvider()
	// so we don't need to handle unknown providers here
	var payoutGateway financeWorker.PayoutGateway

	switch strings.ToLower(cfg.Payout.GatewayProvider) {
	case "sandbox":
		// Sandbox gateway for testing (in-memory simulation)
		payoutGateway = financeWorker.NewSandboxPayoutGateway(
			financeWorker.SandboxGatewayConfig{
				SecretKey:           cfg.Payout.SecretKey,
				BaseURL:             cfg.Payout.WebhookURL,
				SimulateRealGateway: false,
				SimulateLatency:     100 * time.Millisecond,
				FailureRate:         0.0,
				AlwaysSucceed:       true,
				Log:                 log.Logger,
			},
		)

		log.Info("Sandbox gateway initialized (in-memory simulation)",
			zap.String("provider", cfg.Payout.GatewayProvider),
			zap.Bool("production_mode", cfg.IsPayoutProduction()),
		)

	case "midtrans_payout":
		// Midtrans Iris disbursement gateway.
		// Requires MIDTRANS_IRIS_OPERATOR_KEY — NOT the same as MIDTRANS_SERVER_KEY.
		irisGateway, irisErr := financeWorker.NewMidtransPayoutGateway(
			financeWorker.MidtransPayoutGatewayConfig{
				IrisOperatorKey: cfg.Payout.IrisOperatorKey,
				IrisApproverKey: cfg.Payout.IrisApproverKey,
				IsProduction:    cfg.IsPayoutProduction(),
				SimulateMode:    false, // SimulateMode=false: use real Iris sandbox
			},
			log.Logger,
		)
		if irisErr != nil {
			panic(fmt.Sprintf("Midtrans Iris gateway init failed: %v — set MIDTRANS_IRIS_OPERATOR_KEY", irisErr))
		}
		payoutGateway = irisGateway

		log.Info("Midtrans Iris payout gateway initialized",
			zap.String("provider", cfg.Payout.GatewayProvider),
			zap.Bool("production_mode", cfg.IsPayoutProduction()),
			zap.Bool("iris_operator_key_set", cfg.Payout.IrisOperatorKey != ""),
			zap.Bool("iris_approver_key_set", cfg.Payout.IrisApproverKey != ""),
		)

	default:
		// This should NEVER happen because config validation fails fast on unknown provider
		// If we reach here, it means config validation was not called or bypassed
		panic(fmt.Sprintf("CODE ERROR: Unknown gateway provider '%s' reached dependencies init. "+
			"Config validation should have caught this.", cfg.Payout.GatewayProvider))
	}

	// Create payout webhook handler with signature verification and gateway config for observability
	payoutWebhookHandler := financePayoutHTTP.NewPayoutWebhookHandlerWithConfig(
		withdrawRepo,
		db,
		cfg.Payout.SecretKey,
		log.Logger,
		cfg.Payout.GatewayProvider,
		cfg.IsPayoutProduction(),
		cfg.IsPayoutSandbox(),
		outboxRepository,
	)

	// Initialize payout worker
	// Parse pilot whitelist from config (comma-separated UUIDs)
	var pilotWhitelist []uuid.UUID
	if cfg.Payout.PilotWhitelist != "" {
		whitelistStrs := strings.Split(cfg.Payout.PilotWhitelist, ",")
		for _, ws := range whitelistStrs {
			ws = strings.TrimSpace(ws)
			if ws != "" {
				if sellerID, err := uuid.Parse(ws); err == nil {
					pilotWhitelist = append(pilotWhitelist, sellerID)
				} else {
					log.Warn("Invalid seller ID in pilot whitelist", zap.String("id", ws))
				}
			}
		}
		log.Info("Pilot mode whitelist loaded",
			zap.Int("whitelisted_sellers", len(pilotWhitelist)),
			zap.Bool("pilot_mode_enabled", cfg.Payout.EnablePilotMode),
		)
	}

	payoutWorkerCfg := financeWorker.PayoutWorkerConfig{
		PollInterval:    financeWorker.DefaultPayoutPollInterval,
		BatchSize:       financeWorker.DefaultPayoutBatchSize,
		RetryBatchSize:  financeWorker.DefaultRetryBatchSize,
		EnablePilotMode: cfg.Payout.EnablePilotMode,
		PilotWhitelist:  pilotWhitelist,
	}

	// Whitelist audit repo: persists every whitelist mutation to DB.
	// Wired always so that staging/production get fail-closed audit on startup.
	whitelistAuditRepo := financeRepo.NewWhitelistAuditRepository(db.Pgx())

	// Initialize ledger repository (required for seller payable balance).
	// Created here so both PayoutWorker and WithdrawService can share it.
	ledgerRepo := financeRepo.NewLedgerRepository()

	payoutWorker, err := financeWorker.NewPayoutWorker(
		db,
		withdrawRepo,
		ledgerRepo,
		payoutGateway,
		log.Logger,
		payoutWorkerCfg,
		whitelistAuditRepo,
	)
	if err != nil {
		log.Fatal("Payout worker initialization failed (whitelist audit)", zap.Error(err))
	}

	// Payout worker starts only when PAYOUT_ENABLE_WORKER=true (never silently).
	// Config validation runs here so it only fires when the worker is actually being activated.
	//
	// PASS_18S: cfg.ValidatePayoutCompletionPath() in main() already fails
	// startup in staging/production if the completion loop is unsafe, so
	// reaching this branch with an unsafe loop only happens in development —
	// there we still log LOUDLY (not just Warn) rather than starting silently,
	// and the same signal is surfaced continuously via GET /health/ready
	// ("payout_safety").
	if cfg.Payout.EnableWorker {
		if err := financeWorker.ValidatePayoutWorkerConfig(payoutWorkerCfg); err != nil {
			log.Fatal("Payout worker startup blocked by config validation", zap.Error(err))
		}
		payoutSafety := cfg.EvaluatePayoutCompletionSafety()
		if payoutSafety.Degraded {
			log.Error("PAYOUT COMPLETION LOOP UNSAFE — starting anyway (development only): "+payoutSafety.Reason,
				zap.String("environment", cfg.Payout.Environment),
				zap.Bool("webhook_configured", payoutSafety.PayoutWebhookConfigured),
				zap.Bool("reconciliation_enabled", payoutSafety.PayoutReconciliationEnabled),
				zap.String("fix", "set PAYOUT_SECRET_KEY or disable PAYOUT_ENABLE_WORKER"),
			)
		}
		workerStartups = append(workerStartups, func() {
			payoutWorker.Start()
			log.Info("Payout worker STARTED",
				zap.String("environment", cfg.Payout.Environment),
				zap.Bool("pilot_mode", cfg.Payout.EnablePilotMode),
				zap.Int("pilot_whitelist_count", len(pilotWhitelist)),
				zap.Bool("sandbox_mode", cfg.IsPayoutSandbox()),
				zap.Bool("completion_path_available", payoutSafety.CompletionPathAvailable),
			)
		})
	} else {
		log.Warn("payout worker not started — withdrawals will not be processed automatically",
			zap.String("enable", "PAYOUT_ENABLE_WORKER=true"),
			zap.String("environment", cfg.Payout.Environment),
		)
	}

	// Initialize payout reconciliation service
	payoutReconciliationService := financeWorker.NewPayoutReconciliationService(
		withdrawRepo,
		db,
		log.Logger,
		financeWorker.PayoutReconciliationConfig{
			StuckThresholdMinutes:         cfg.Payout.StuckThresholdMinutes,
			ReconciliationIntervalMinutes: cfg.Payout.ReconciliationIntervalMinutes,
		},
	)

	// Reconciliation worker: read-only — queries stuck payouts and logs, no mutations.
	// Controlled by PAYOUT_ENABLE_RECONCILIATION (independent of payout worker).
	var payoutReconciliationWorker *financeWorker.PayoutReconciliationWorker
	if cfg.Payout.EnableReconciliation {
		payoutReconciliationWorker = financeWorker.NewPayoutReconciliationWorker(
			payoutReconciliationService,
			log.Logger,
		)
		workerStartups = append(workerStartups, func() {
			payoutReconciliationWorker.Start()
			log.Info("Payout reconciliation worker STARTED (read-only)",
				zap.Int("stuck_threshold_minutes", cfg.Payout.StuckThresholdMinutes),
				zap.Int("reconciliation_interval_minutes", cfg.Payout.ReconciliationIntervalMinutes),
			)
		})
	} else {
		log.Info("Payout reconciliation worker DISABLED: set PAYOUT_ENABLE_RECONCILIATION=true to activate")
	}

	// Wrap nil payout workers in stub to avoid nil pointer issues
	payoutWorkerWrapper := wrapWorkerOrNil(payoutWorker)
	payoutReconWorkerWrapper := wrapWorkerOrNil(payoutReconciliationWorker)

	// =====================================================================
	// WITHDRAW MODULE - Seller Withdrawal API
	// =====================================================================
	//
	// Seller withdrawal endpoints:
	// - POST /api/v1/withdraw - Request withdrawal
	// - GET /api/v1/withdraw/history - Get withdrawal history
	//
	// Initialize verification service (required for seller verification check
	// in withdraw gate, and the seller-facing + admin verification HTTP).
	//
	// PHASE 2: pass adminAuditLogger and outboxRepository so every admin
	// transition (approve / reject / request_resubmission) writes an audit
	// row and emits a seller.verification.* outbox event atomically.
	sellerVerificationRepo := verificationRepo.NewSellerVerificationRepository()
	verificationService := verificationApp.NewVerificationService(
		db.Pgx(),
		sellerVerificationRepo,
		adminAuditLogger,
		outboxRepository,
		capabilityChecker,
	)

	// Initialize bank account repository (required for default bank account check)
	bankAccountRepo := bankaccountrepo.NewBankAccountRepository()

	// C6.1: bank account service + handler — seller self-service CRUD.
	bankAccountService := bankaccountApp.NewBankAccountService()
	bankAccountService.SetLogger(log.Logger)
	// Wire verification checker + outbox so post-approval mutations emit
	// bank_account.*_after_verification events (Patch E).
	bankAccountService.SetVerificationOutbox(verificationService, outboxRepository)
	bankAccountHandler := bankaccountHTTP.NewBankAccountHandler(bankAccountService, db.Pgx(), log.Logger)

	// Address handler — buyer shipping + seller sender CRUD.
	addressService := addressApp.NewAddressService()
	addressService.SetLogger(log.Logger)
	addressHandler := addressHTTP.NewAddressHandler(addressService, roleChecker, db.Pgx(), log.Logger)

	// Initialize withdraw service
	withdrawService := financeApp.NewWithdrawService(
		db.Pgx(),
		ledgerRepo,
		withdrawRepo,
		bankAccountRepo,
		roleChecker,
		accountStatusChecker,
		adminAuditLogger,
		verificationService,
		outboxRepository,
	)

	// Wire the canonical freeze-aware withdrawal authority into WithdrawService.
	withdrawAuthFinance := financeApp.NewFinanceService()
	withdrawAuthFinance.SetLogger(log.Logger)
	withdrawAuthFinance.SetDisputeFreezeRepo(financeRepo.NewDisputeFreezeRepository())
	withdrawService.SetCanonicalAuthority(withdrawAuthFinance)

	// Wire the unified withdrawal HTTP handler. POST /api/v1/withdraw now
	// drives the canonical finance-shape lifecycle (status=REQUESTED), which
	// is consumed end-to-end by AdminPayoutHandler, PayoutWorker, and
	// PayoutWebhookHandler.
	withdrawalHandlerUnified := walletHTTP.NewWithdrawalHandlerUnified(
		withdrawService,
		db.Pgx(),
		log.Logger,
	)

	// Initialize admin payout handler for admin operations
	adminPayoutHandler := financePayoutHTTP.NewAdminPayoutHandler(
		withdrawService,
		db.Pgx(),
		adminAuditLogger,
		log.Logger,
		whitelistAuditRepo,
	)

	// Initialize admin finance handler: canonical ledger export + invariant verifier endpoint
	adminFinanceHandler := financePayoutHTTP.NewAdminFinanceHandler(
		db.Pgx().Pool(),
		adminAuditLogger,
		log.Logger,
	)

	// ===== VERIFICATION MODULE (Phase 2 — operationalized) =====
	// Canonical seller verification surface.
	//
	// SELLER-FACING endpoints (gated by RequireSellerMiddleware):
	//   POST /api/v1/seller/verification/identity
	//   POST /api/v1/seller/verification/business
	//   GET  /api/v1/seller/verification/status
	//
	// ADMIN endpoints (gated by RequireCapability("seller.verification.review")):
	//   GET  /api/v1/admin/seller-verifications/pending
	//   POST /api/v1/admin/seller-verifications/:seller_id/approve
	//   POST /api/v1/admin/seller-verifications/:seller_id/reject
	//   POST /api/v1/admin/seller-verifications/:seller_id/request-resubmission
	//
	// Lifecycle truth lives on seller_verifications (one row per seller, 8
	// canonical statuses); verification_documents holds uploaded evidence
	// (KTP, NPWP, etc.). The document service drives the seller-level
	// lifecycle into pending_review on every submit, atomically.
	verificationDocumentRepo := verificationRepo.NewVerificationDocumentRepository()
	verificationDocumentService := verificationApp.NewVerificationDocumentService(
		db.Pgx(),
		verificationDocumentRepo,
		sellerVerificationRepo,
		outboxRepository,
	)
	verificationHandler := verificationHTTP.NewVerificationHandler(
		verificationDocumentService,
		verificationService,
		log.Logger,
	)
	adminVerificationHandler := verificationHTTP.NewAdminVerificationHandler(
		verificationService,
		verificationDocumentService,
		log.Logger,
	)
	adminVerificationHandler.SetBankAccountReader(db.Pgx(), bankAccountRepo)
	// Wire bank account reader into VerificationService so ApproveVerification
	// can snapshot reviewed_bank_account_ids at KYC approval time (GUARD 5 seed).
	verificationService.SetBankAccountReader(db.Pgx(), bankAccountRepo)

	// Wire S3 presigner for KYC document upload/view URL generation.
	// When AWS credentials are present, both handlers can generate short-lived
	// presigned PUT (upload) and GET (view) URLs without exposing credentials
	// to the mobile client. When credentials are absent (CI/test env), the
	// presigner is nil and the handlers return 503 on the presign-only routes.
	awsPresignCfg := s3presign.Config{
		Region:    cfg.AWS.S3BucketRegion,
		AccessKey: cfg.AWS.AccessKeyID,
		SecretKey: cfg.AWS.SecretAccessKey,
		Bucket:    cfg.AWS.S3BucketName,
	}
	if cfg.AWS.AccessKeyID != "" && cfg.AWS.SecretAccessKey != "" {
		kycPresigner := &s3PresignerAdapter{cfg: awsPresignCfg}
		verificationHandler.SetPresigner(kycPresigner)
		adminVerificationHandler.SetPresigner(kycPresigner)
	}
	mediaUploadHandler := mediauploadHTTP.NewHandler(awsPresignCfg, cfg.AWS.CDNBaseURL, log.Logger)

	// Configure the shared media read-resolution authority (mediaresolve) once
	// at bootstrap so persisted storage keys resolve to CDN (or presigned GET)
	// read URLs on every read surface — user profile cover photo included
	// (STAGE 4F-1). When AWS is unconfigured the resolver falls back to
	// pass-through for absolute URLs, matching the pre-existing behavior of
	// the content/for-sale/auction projections.
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: awsPresignCfg,
		CDNBaseURL: cfg.AWS.CDNBaseURL,
		ReadTTL:    5 * time.Minute,
	})

	// ===== FEED MODULE =====
	// Initialize feed service
	feedService := feedApp.NewFeedService(feedrepo.NewFeedRepository())

	// PHASE C — Feed evaluator shadow observability (BLOCKER-002, BLOCKER-004
	// observability only). Disabled by default; enable via
	// EVALUATOR_SHADOW_FEED_ENABLED=true. Per docs/05-rollout/
	// convergence-sequencing-addendum-viewercontext-evaluator.md (§3.1, §5),
	// feed is the canonical first SHADOW surface but never the first
	// AUTHORITY surface. The runner is fire-and-forget and never affects
	// the runtime feed response, pagination, or legacy authority.
	// F1-W3A — NewFeedShadowRunner no longer accepts a *pgxpool.Pool.
	// Overlay hydration now happens at the handler boundary
	// (feed_viewercontext.go); the runner is a pure observer.
	var feedShadowRunner *evaluator.FeedShadowRunner
	if strings.EqualFold(os.Getenv("EVALUATOR_SHADOW_FEED_ENABLED"), "true") {
		feedShadowRunner = evaluator.NewFeedShadowRunner(log.Logger)
		// BATCH 3M: stamp the configured /feed evaluator operating mode.
		// FEED_EVALUATOR_MODE is parsed in config.go and normalized
		// here. Default + invalid input → shadow (safe default; no
		// runtime visibility change). Enforce flips the handler to the
		// synchronous further-restrict path (evaluator/feed_enforce.go).
		feedEvaluatorMode := evaluator.NormalizeFeedEvaluatorMode(cfg.FeatureFlags.FeedEvaluatorMode)
		feedShadowRunner = feedShadowRunner.WithMode(feedEvaluatorMode)
	}
	// P3A — Promotion feed injector. Interleaves active promoted items into
	// the organic feed. Nil-safe: a nil injector disables injection entirely.
	feedPromotionInjector := feedHTTP.NewFeedPromotionInjector(promotionDiscoveryService, db.Pgx().Pool(), log.Logger)
	feedHandler := feedHTTP.NewFeedHandler(feedService, db.Pgx(), log.Logger, feedShadowRunner, feedPromotionInjector)

	// ===== LIKE MODULE (constructed here so ContentService can receive it
	// as an injected dependency instead of building its own copy) =====
	// LikeService provides governance for content likes: block check,
	// deleted-content guard, outbox event emission, invariant logging.
	likeRepo := likerepo.NewLikeRepository()
	likeService := likeApp.NewService(
		db.Pgx(),
		contentRepo,
		likeRepo,
		outboxRepository,
		blockChecker,
		likeApp.NewZapInvariantLogger(log.Logger),
		// Scrub content.liked notifications on UNLIKE so a LIKE after an
		// UNLIKE emits a fresh notification (occurrence-scoped lifecycle).
		&notificationRepoImpl.NotificationRepository{},
	)

	// ===== CONTENT MODULE =====
	// Initialize content service with account status checker
	// contentRepo/likeRepo are injected (shared instances) rather
	// than self-constructed, so ContentService no longer hides its own DB
	// wiring behind NewContentService. likeService stays in the like
	// module; its content write path routes through the like handler.
	contentService := contentApp.NewContentService(
		contentRepo,
		likeRepo,
		roleChecker,
		accountStatusChecker,
		nil, // InvariantLogger - optional
	)
	// BATCH 3Q — /contents/:id evaluator shadow seam. Gated on env var
	// EVALUATOR_SHADOW_CONTENT_DETAIL_ENABLED=true. Default off so a
	// disabled runner returns a nil pointer and the handler short-
	// circuits dispatch. Mirrors the EVALUATOR_SHADOW_FEED_ENABLED
	// gating pattern used immediately above.
	//
	// D1 — additionally stamp the configured operating mode.
	// CONTENT_DETAIL_EVALUATOR_MODE is parsed in config.go and normalized
	// here. Default + invalid input → shadow (safe default; no runtime
	// visibility change). Enforce flips the handler to the synchronous
	// fail-CLOSED path (evaluator/content_detail_enforce.go).
	// F1-W3B — NewContentDetailShadowRunner no longer accepts a
	// *pgxpool.Pool. Overlay hydration now happens at the handler
	// boundary (content_viewercontext.go); the runner is a pure observer.
	var contentDetailShadowRunner *evaluator.ContentDetailShadowRunner
	if strings.EqualFold(os.Getenv("EVALUATOR_SHADOW_CONTENT_DETAIL_ENABLED"), "true") {
		contentDetailShadowRunner = evaluator.NewContentDetailShadowRunner(log.Logger)
		contentDetailEvaluatorMode := evaluator.NormalizeContentDetailEvaluatorMode(cfg.FeatureFlags.ContentDetailEvaluatorMode)
		contentDetailShadowRunner = contentDetailShadowRunner.WithMode(contentDetailEvaluatorMode)
	}
	contentHandler := contentHTTP.NewContentHandler(
		contentService,
		roleChecker,
		db.Pgx(),
		log.Logger,
		contentDetailShadowRunner,
	)

	// ===== COMMENT MODULE =====
	// Initialize comment repository and service.
	// Constructed AFTER the CONTENT MODULE so the single canonical instance
	// of ContentService (built above) can be injected into the comment
	// handler. GET /contents/:id/comments (ListComments) dereferences
	// CommentHandler.contentService for its parent-content visibility gate;
	// wiring nil earlier made the production list endpoint panic at runtime.
	commentRepo := contentrepo.NewCommentRepository()
	// C-IPC — idempotency_records authority shared with commerce-reference
	// comment creates. Wired so normal comment POST enforces the mandatory
	// Idempotency-Key header (replay vs conflict) inside AddComment.
	idempotencyCommentRepo := idempotencyRepoPkg.NewRepository()
	var commentService *contentApp.CommentService
	commentService = contentApp.NewCommentService(
		contentRepo, // E3.3 — wired (previously nil, causing AddComment panic)
		commentRepo,
		forSaleService,
		nil,              // auctionValidator — not wired on this path
		nil,              // visibilityChecker — falls back to contentRepo in AddCommerceReferenceComment
		outboxRepository,
		idempotencyCommentRepo,
		blockChecker,
		nil, // invariantLogger - optional
	)

	// ===== COMMERCE RESPONSE AUTHORIZATION =====
	// Canonical Commerce Response resource reference validator: single validation point
	// for ForSale/Auction existence + state validation. Displayability only.
	// Consumed by Create Content, Comment, and Chat.
	commerceRefValidator := commerceResponse.NewValidator(
		forSaleRepository,   // ForSaleGetter — already constructed in LISTING MODULE
		auctionRepository,   // AuctionGetter — already constructed in AUCTION MODULE
	)
	contentService.SetCommerceReferenceValidator(commerceRefValidator)
	commentService.SetCommerceReferenceValidator(commerceRefValidator)
	chatService.SetCommerceReferenceValidator(commerceRefValidator)

	commentHandler := contentHTTP.NewCommentHandler(
		commentService,
		contentService, // canonical ContentService instance from CONTENT MODULE above
		forSaleService,
		roleChecker,
		db.Pgx(),
		log.Logger,
	)

	// ─── DO NOT ENABLE ─────────────────────────────────────────────────
	// MODERATION MODULE (Trust MVP) - Event Handlers
	// ───────────────────────────────────────────────────────────────────
	// MODERATION ENFORCEMENT (Batch 71 — safe enablement)
	//
	// Processes moderation.*.removed / .restored events to soft-delete
	// content, comments, forSales, and suspend/restore users.
	//
	// SAFE handlers (zero money risk, zero buyer-hold risk):
	//   content.removed/restored, comment.removed/restored,
	//   forSale.removed/restored, user.suspended/restored
	//
	// PARKED handler (consumed but no-op):
	//   auction.removed — requires moderation-specific cancel authority
	//   (IsSeller rejects uuid.Nil; CanCancel rejects active-with-bids)
	//
	// Default ON. Disable via DISABLE_MODERATION_EVENT_HANDLER=true.
	// ───────────────────────────────────────────────────────────────────
	if workerEnabled("MODERATION_EVENT_HANDLER", true, log.Logger) {
		outboxWorker.SetupModerationHandlers(
			db.Pgx(),
			contentService,    // SoftDeleteForModeration / RestoreFromModeration
			commentService,    // SoftDeleteForModeration / RestoreFromModeration
			forSaleService,    // Withdraw (idempotent on terminal state)
			auctionService,    // Cancel PARKED — handler consumes event as no-op
			userRepository,    // Update account_status (suspended / active)
			chatService,       // Chat moderation service: hide/restore + room.updated projection
			notifEventHandler, // fanout: notification fires AFTER enforcement
		)
		log.Info("Moderation event handlers registered with outbox worker (enforcement + notification fanout)")

		// Moderation suspension → WS eviction (mirrors CHAT-3 ban eviction).
		// Must be called AFTER SetupModerationHandlers so fanout picks up the
		// existing handler that sets account_status='suspended'.
		outboxWorker.SetupModerationWSEvictionHandler(realtimeHub)
	} else {
		log.Warn("moderation enforcement disabled — moderation outbox events will no-op")
	}

	// ===== LIKE MODULE =====
	// likeService/likeRepo were constructed earlier (ahead of the CONTENT
	// MODULE section) so ContentService could receive the same shared
	// instances via injection; reused here for likeHandler.
	commentLikeService := likeApp.NewCommentLikeService(
		db.Pgx(),
		contentRepo,
		commentRepo,
		likerepo.NewTargetLikeRepository(),
		blockChecker,
	)
	likeHandler := likeHTTP.NewLikeHandler(db.Pgx(), log.Logger, likeService, commentLikeService)

	// ===== MODERATION MODULE (Trust MVP) =====
	// Initialize moderation repository
	moderationRepository := moderationRepo.NewModerationRepository()
	// Initialize moderation service with outbox for event emission
	moderationService := moderationApp.NewModerationService(
		db.Pgx(),
		moderationRepository,
		outboxRepository,
	)
	// Initialize moderation handler
	moderationHandler := moderationHTTP.NewModerationHandler(
		moderationService,
		db.Pgx(),
		log.Logger,
		adminAuditLogger,
	)

	// ===== SOCIAL MODULE =====
	// Initialize social service for follow/block/mute operations
	socialService := socialApp.NewSocialServiceWithDefaults(db, outboxRepository)
	// Initialize social handler
	followHandler := socialhttp.NewFollowHandler(
		socialService,
		db.Pgx(),
		log.Logger,
	)

	// ===== SHIPPING QUOTE MODULE =====
	// Initialize shipping quote service and handler for manual shipping quotes via chat
	//
	// BUSINESS TRUTH:
	// - ShippingQuote is STATELESS (no status, no expiration)
	// - Used as fallback when forSale lacks shipping coverage
	// - Seller creates quote → chat message sent → buyer can checkout
	// - API: POST /api/v1/chat/:chat_id/shipping-quote
	// Note: shippingQuoteRepository already initialized above for forSale service
	shippingQuoteRepository = shippingQuoteRepo.NewShippingQuoteRepository()
	chatRepositoryForQuote := chatRepo.NewChatRepository()
	shippingQuoteRoomGetter := &shippingQuoteRoomGetterAdapter{repo: chatRepositoryForQuote}
	forSaleRepositoryForQuote := forSaleRepo.NewForSaleRepository()
	shippingQuoteService := shippingQuoteApp.NewService(
		db.Pgx(),
		shippingQuoteRepository,
		shippingQuoteRoomGetter,
		forSaleRepositoryForQuote,
		auctionRepository,
		chatService,
		orderRepository, // HARD FIX: Order repository for quote reactivation validation
		log.Logger,
	)
	shippingQuoteHandler := shippingQuoteHTTP.NewHandler(
		shippingQuoteService,
		shippingQuoteRoomGetter,
		db.Pgx(),
		log.Logger,
	)

	// HARD FIX: Wire up shipping quote service to order service for quote reactivation
	// This must be done after both services are created to avoid circular dependency
	orderService.SetShippingQuoteService(shippingQuoteService)

	// ===== RATING MODULE =====
	// Initialize rating service and handlers for buyer→seller order ratings
	//
	// BUSINESS TRUTH:
	// - Rating is IMMUTABLE (no update/delete after creation)
	// - Only buyer can rate seller (not seller→buyer)
	// - One rating per order (enforced by UNIQUE constraint)
	// - Order must be completed before rating
	//
	// API Endpoints:
	// - POST /api/v1/orders/{id}/ratings - Create rating for completed order
	// - GET /api/v1/users/{id}/ratings - Get ratings received by seller
	// - GET /api/v1/users/me/ratings/given - Get ratings given by buyer
	ratingService := ratingApp.NewRatingService()
	ratingHandler := ratingHTTP.NewRatingHandler(
		ratingService,
		db.Pgx(),
		log.Logger,
	)

	// Z5. Rating Invalidation Worker — eventual-consistency safety net.
	//
	// PRIMARY PATH: OrderCompletionService.{RefundOrder,PartialRefund,RefundFromDispute}
	// already call ratingMutator.InvalidateForOrder at refund time (best-effort; refund
	// never blocks on rating failure). This worker catches any orders where that call failed.
	//
	// MUTATIONS: Sets invalidated_at on order_ratings only. No ledger, no order state.
	// Default ON: without this, a primary-path failure leaves a refunded order contributing
	// to seller reputation indefinitely.
	// Disable: DISABLE_RATING_INVALIDATION_WORKER=true
	ratingInvalidationWorker := worker.NewRatingInvalidationWorker(
		db,
		log.Logger,
		ratingService,
	)
	if workerEnabled("RATING_INVALIDATION_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			ratingInvalidationWorker.Start()
			log.Info("RatingInvalidationWorker started",
				zap.Duration("check_interval", worker.DefaultRatingInvalidationInterval),
				zap.Int("batch_size", worker.DefaultRatingInvalidationBatchSize),
			)
		})
	} else {
		_ = ratingInvalidationWorker
	}

	// ===== PROMOTION PHASE 4: DISCOVERY ENDPOINTS =====
	// Initialize promotion handler for discovery surfaces (search, home)
	// PROMOTION PHASE 5: Add billing service for package purchases
	billingService := billingApp.NewBillingService(roleChecker, accountStatusChecker)
	promotionHandler := promotionHTTP.NewPromotionHandler(
		promotionService,
		billingService,
		roleChecker,
		db.Pgx(),
		log.Logger,
		adminAuditLogger,
	)
	// Wire analytics event repository for click tracking.
	promotionEventRepo := promotionInfraRepo.NewPromotionEventRepository()
	promotionHandler.SetEventRepo(promotionEventRepo)

	// ===== FEDERATED SEARCH CONTRACT REALIGN PACK V1 =====
	// Initialize search domain for federated search across content, users, forSales
	//
	// FEDERATED SEARCH STRATEGY:
	// - Each domain has its own search endpoint
	// - Search domain provides unified handlers that query domain-specific repositories
	// - No unified search backend abstraction - queries are federated
	//
	// SEARCH ENDPOINTS:
	// - GET /api/v1/search/forSales - Search forSales (already exists via ForSaleHandler)
	// - GET /api/v1/search/content - Search content/posts
	// - GET /api/v1/search/users - Search users/profiles
	// - GET /api/v1/search/history - Get search history
	// - POST /api/v1/search/history - Save search history
	// - DELETE /api/v1/search/history - Clear search history
	// - DELETE /api/v1/search/history/:id - Delete specific history item

	// Initialize search repository and service
	searchRepository := searchRepo.NewSearchRepository()
	searchService := searchApp.NewSearchService(searchRepository)

	// PHASE C — search/content shadow seam Stage 1 (telemetry only) per
	// docs/05-rollout/search-shadow-seam-landing-task-design.md §3.1 /
	// §4.2. The shadow runner is unconditionally constructed and
	// dispatched fire-and-forget.
	//
	// BATCH 3B — Adapter mode plumbed in from cfg.FeatureFlags. In shadow
	// mode (default) the runner is observation-only and the /search/
	// content handler emits its legacy response shape unchanged. In
	// enforce mode the runner additionally labels its telemetry with
	// mode="enforce" AND the handler synchronously calls
	// evaluator.EnforceSearchContent to filter / coarsen rows BEFORE
	// response serialization. Invalid env values fall safely to shadow
	// via NormalizeSearchContentAdapterMode — enforce mode is opt-in
	// only.
	searchContentEvaluatorMode := evaluator.NormalizeSearchContentAdapterMode(cfg.FeatureFlags.SearchContentEvaluatorMode)
	searchContentShadowRunner := evaluator.NewSearchContentShadowRunner(log.Logger).
		WithMode(searchContentEvaluatorMode)

	// P3B — Search promotion injector. Appends promoted sidecar to
	// forSale and auction search responses. Nil-safe.
	searchPromotionInjector := searchHTTP.NewSearchPromotionInjector(
		promotionDiscoveryService, db.Pgx().Pool(), log.Logger,
	)

	// Initialize search handler.
	searchHandler := searchHTTP.NewSearchHandler(
		searchService,
		db.Pgx(),
		log.Logger,
		searchContentShadowRunner,
		searchPromotionInjector,
	)

	// ===== COINS DOMAIN (LOYALTY POINTS) =====
	// Initialize coins service for loyalty points management
	// Coins are earned through activities and spent on order discounts
	// coinsRepository was constructed earlier (refund/order K-reader wiring);
	// reuse the same canonical instance for the coins service.
	coinsService := coinsApp.NewCoinsService(coinsRepository, db.Pgx())

	// Wire up audit service for coins service
	coinsService.SetAuditService(auditService)

	// Post-construction wiring: orderService was built earlier at this point
	// (before coinsService existed), so it received nil for coinsService. Inject
	// the real instance now so OrderCompletionService.Complete can grant loyalty
	// points on order completion. The completion service has a defensive nil
	// guard that skips the loyalty reward if this call is ever missed, so this
	// wiring is a soft-required best-effort hookup, not a financial dependency.
	orderService.SetCoinsService(coinsService)

	// CANONICAL COIN CONSUME+SPEND WIRING: the finalization service was built
	// before coinsService existed; inject the consume+spend surface now so a
	// settling payment with K>0 completes RESERVE → CONSUME atomically with
	// settlement (reservation consume + order_spend + balance deduct).
	canonicalFinalizationService.SetCoinSpendConsumer(coinsService)

	// CANONICAL COINS REFUND HANDLER REGISTRATION.
	// Registers CoinsRefundRequiredHandler against `coins.refund_required` so
	// the producer paths (HandleGatewayRefundAck full-refund ack-time emission,
	// OrderCompletionService.CancelOverdue, OrderCompletionService.Expire) are
	// runtime-reachable. Without this, the dispatcher would hit DispatchResultNoHandler
	// and silently mark the event succeeded, leaving buyer coins un-refunded.
	// Idempotency is guaranteed by the unique index
	// idx_coins_transactions_unique_reference(user_id, reference_type, reference_id).
	// Registered AFTER coinsService is constructed; outbox worker .Start() is
	// deferred via workerStartups, so registration is in place before polling begins.
	outboxWorker.SetupCoinsRefundRequiredHandler(db.Pgx(), coinsService)

	// ===== ADMIN / OPERABILITY HARDENING PACK V1 =====
	// Initialize admin, appeals, warnings, and support handlers

	// 1. Admin Handler - Dashboard, user management, metrics, audit logs
	adminRepository := adminInfraRepo.NewAdminRepository()
	// MODERATION DOMAIN HARD LOCK: Pass outbox repository for ban event emission
	adminSvc := adminApp.NewAdminService(db.Pgx(), adminRepository, adminAuditLogger, outboxRepository)

	// 1.5 Capability Service & Handler - Capability management system
	capabilitySvc := capabilityApp.NewCapabilityService(capabilityRepository, adminAuditLogger)
	capabilityHandler := capabilityHTTP.NewCapabilityHandler(capabilitySvc)

	// Wire capability lister for support ticket admin fanout notifications
	notifEventHandler.SetCapabilityLister(capabilitySvc)

	// Initialize admin handler with capability service for user details
	// Create SLA service for metrics aggregation
	slaSvc := adminApp.NewSLAService(db.Pgx(), disputeRepository, supportInfraRepo.NewSupportRepository())
	adminHandler := adminHTTP.NewAdminHandler(adminSvc, adminAuditLogger, capabilitySvc, slaSvc)
	adminHandler.SetDeliveryQuerier(deliveryLogger)                               // O4: notification delivery failure monitoring
	adminHandler.SetBNRResetter(worker.NewBNRAdminResetter(db.Pgx(), log.Logger)) // BNR admin strike reset

	// 2. Appeal Handler - Appeals system for moderation decisions
	appealRepository := appealInfraRepo.NewAppealRepository()
	appealSvc := appealApp.NewAppealService(appealRepository, moderationRepository, contentRepo, commentRepo, outboxRepository)
	// Wire forSale/auction repos for expanded appeal eligibility (forSale/auction/user).
	// These are record-only appeals: no auto-restoration; approval is administrative.
	appealSvc.SetForSaleRepo(forSaleRepo.NewForSaleRepository())
	appealSvc.SetAuctionRepo(auctionRepo.NewAuctionRepository())
	appealHandler := appealHTTP.NewAppealHandler(appealSvc, db.Pgx(), log.Logger, adminAuditLogger)

	// 3. Warning Handler - Warnings system for policy violations
	warningRepository := warningInfraRepo.NewWarningRepository()
	warningSvc := warningApp.NewWarningService(warningRepository, userRepository, outboxRepository)
	warningHandler := appealHTTP.NewWarningHandler(warningSvc, db.Pgx(), log.Logger, adminAuditLogger)

	// 4. Support Handler - Support ticket system
	// Create an adapter for the chat service to match support handler's interface
	supportChatAdapter := &supportChatServiceAdapter{chatService: chatService}
	// Create an adapter for the dispute service to match support service's interface
	disputeSvcAdapter := &disputeServiceAdapter{disputeService: disputeService}
	supportSvc := supportApp.NewServiceWithDefaults(db.Pgx(), supportChatAdapter, outboxRepository, orderService, disputeSvcAdapter, log.Logger)
	supportHandler := supportHTTP.NewHandler(supportSvc, supportChatAdapter, chatService, db.Pgx(), log.Logger, adminAuditLogger)

	// SUPPORT USER REPLY HANDLER: When a user sends a chat message in a support
	// room, transitions the linked ticket from waiting_user → in_progress.
	outboxWorker.SetupSupportUserReplyHandler(supportSvc)
	log.Info("Support user reply handler registered (ticket status transition)")

	// =============================================================================
	// ALERT SYSTEM V1 - Anomaly Detection and Alerting
	// =============================================================================
	// Detects system anomalies and creates actionable alerts for admin review.
	//
	// DETECTION RULES:
	// - payment_failure_spike: Sudden increase in payment failures
	// - payment_stuck: Payments stuck in pending state
	// - dispute_spike: Sudden increase in disputes
	// - seller_risk: Sellers with high dispute counts
	// - coins_anomaly: Unusual coin activity patterns
	// - withdrawal_anomaly: Suspicious withdrawal patterns
	//
	// ENDPOINTS:
	// - GET /api/v1/admin/alerts - List alerts with filtering
	// - GET /api/v1/admin/alerts/:id - Get alert details
	// - GET /api/v1/admin/alerts/stats - Alert statistics
	// - POST /api/v1/admin/alerts/:id/acknowledge - Acknowledge alert
	// - POST /api/v1/admin/alerts/:id/resolve - Resolve alert
	// - POST /api/v1/admin/alerts/:id/false-positive - Mark as false positive
	// - POST /api/v1/admin/alerts/cleanup - Cleanup old resolved alerts

	// Initialize alert repository and service
	alertRepository := alertRepo.NewAlertRepository()
	alertService := alertApp.NewAlertService(db.Pgx(), alertRepository, log.Logger)

	// PASS_18T: wire the alert sink into the payment webhook service so a
	// gateway success notification arriving after PaymentExpiryWorker has
	// already expired the payment/order raises a visible operator alert
	// instead of only being logged.
	paymentWebhookService.SetAlertService(alertService)

	if orphanWebhookRecoveryEnabled(log.Logger) {
		// Orphan webhook recovery worker â€” dormant by default, activated only via
		// explicit env gate. The worker reuses the canonical finalization service
		// and is started through a cancelable application context.
		orphanWebhookRecoveryCfg := paymentApp.LoadOrphanWebhookRecoveryConfigFromEnv()
		orphanWebhookRecoveryWorker := paymentApp.NewOrphanWebhookRecoveryWorker(
			db.Pgx(),
			midtransClient,
			paymentWebhookService,
			canonicalFinalizationService,
			log.Logger,
			orphanWebhookRecoveryCfg,
		)
		orphanWebhookRecoveryWorker.SetMetricsRecorder(metricsCollector)
		orphanWebhookRecoveryWorker.SetAlertService(alertService)
		registerOrphanWebhookRecoveryWorkerStartup(
			appCtx,
			&workerStartups,
			true,
			orphanWebhookRecoveryWorker.Start,
			log.Logger,
		)
	}

	// O1A: Wire money.refund_failed → operator alert (CRITICAL severity).
	// Previously audit-only; now creates a row in system_alerts visible to admin.
	// Safe: outboxWorker.Start() is deferred via workerStartups, so this
	// registration completes before the worker begins polling.
	outboxWorker.SetupRefundFailedAlertHandler(alertService)
	if presenceService != nil {
		outboxWorker.SetupPresenceLastSeenHandler(presenceService)
	}

	// BNR HANDLERS: auction_bnr_detected → strike recording + notifications.
	// Fanout: BNRStrikeHandler (idempotent INSERT) runs first, then
	// NotificationEventHandler sends seller + winner notifications.
	outboxWorker.SetupBNRHandlers(db.Pgx(), notifEventHandler)

	// Initialize admin alert handler
	adminAlertHandler := alertHTTP.NewAdminAlertHandler(alertService, db.Pgx(), log.Logger, adminAuditLogger)

	// O2C: AlertDetectionWorker — polls detection rules every 5 minutes, creates
	// operator-visible alerts in system_alerts. All 10 rules are pure reads with
	// no domain mutations. Dedup via AlertService prevents alert spam.
	// Default ON: observational-only, no risk. Disable: DISABLE_ALERT_DETECTION_WORKER=true
	alertDetectionCfg := worker.DefaultAlertDetectionConfig()
	alertDetectionWorker := worker.NewAlertDetectionWorker(
		db,
		alertService,
		log.Logger,
		alertDetectionCfg,
	)
	// O2B-P1: AlertDetectionWorker liveness metrics.
	alertDetectionWorker.SetMetricsRecorder(metricsCollector)
	if workerEnabled("ALERT_DETECTION_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			alertDetectionWorker.Start()
			log.Info("AlertDetectionWorker started (O2C: operator visibility)",
				zap.Duration("poll_interval", alertDetectionCfg.PollInterval),
				zap.Duration("cleanup_interval", alertDetectionCfg.CleanupInterval),
				zap.Int("retention_days", alertDetectionCfg.RetentionDays),
				zap.Int("rule_count", 10),
			)
		})
	} else {
		log.Warn("alert detection worker disabled — system_alerts will not be generated",
			zap.String("enable", "DISABLE_ALERT_DETECTION_WORKER=false"),
		)
		_ = alertDetectionWorker
	}

	// ESCROW INTEGRITY WORKER — shadow-rollout periodic gateway-funded escrow reconciliation.
	//
	// PASS_18R: this is a money/escrow drift DETECTOR and must not be silently
	// dormant. Default ENABLED + SHADOW:
	//   DISABLE_ESCROW_INTEGRITY_WORKER=false   (default — started)
	//   ESCROW_INTEGRITY_SHADOW_MODE=true        (default — checker logs only, no alerts)
	//   ESCROW_INTEGRITY_INTERVAL_MINUTES=15     (default)
	//
	// Deactivation path: set DISABLE_ESCROW_INTEGRITY_WORKER=true in env.
	// To promote from shadow to live alerts: set ESCROW_INTEGRITY_SHADOW_MODE=false.
	// Current activation state is also surfaced at runtime via
	// worker.CriticalWorkerStatuses(), consumed by /health/ready.
	escrowIntegrityCfg := worker.ParseEscrowIntegrityConfig()
	escrowIntegrityChecker := walletApp.NewEscrowIntegrityChecker(
		walletService,
		alertService,
		db.Pgx(),
		log.Logger,
		escrowIntegrityCfg.ShadowMode,
	)
	escrowIntegrityWorker := worker.NewEscrowIntegrityWorker(
		escrowIntegrityChecker,
		log.Logger,
		escrowIntegrityCfg.Interval,
		escrowIntegrityCfg.ShadowMode,
	)
	if workerEnabled("ESCROW_INTEGRITY_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			escrowIntegrityWorker.Start()
			if escrowIntegrityCfg.ShadowMode {
				log.Warn("EscrowIntegrityWorker started in shadow mode — gateway-funded escrow discrepancies logged only, no alerts will be raised",
					zap.Duration("interval", escrowIntegrityCfg.Interval),
					zap.String("promote", "ESCROW_INTEGRITY_SHADOW_MODE=false"),
				)
			} else {
				log.Info("EscrowIntegrityWorker started",
					zap.Bool("shadow_mode", false),
					zap.Duration("interval", escrowIntegrityCfg.Interval),
				)
			}
		})
	} else {
		log.Error("EscrowIntegrityWorker disabled — gateway-funded escrow discrepancies will NOT be detected",
			zap.String("enable", "DISABLE_ESCROW_INTEGRITY_WORKER=false"),
		)
		_ = escrowIntegrityWorker
	}

	// TOTAL MONEY INVARIANT WORKER — shadow-rollout periodic ledger sum check.
	//
	// PASS_18R: this is a money/ledger drift DETECTOR and must not be silently
	// dormant. Default ENABLED + SHADOW:
	//   DISABLE_TOTAL_MONEY_INVARIANT_WORKER=false   (default — started)
	//   TOTAL_MONEY_INVARIANT_SHADOW_MODE=true        (default — checker logs only, no alerts)
	//   TOTAL_MONEY_INVARIANT_INTERVAL_MINUTES=15     (default)
	//
	// Deactivation path: set DISABLE_TOTAL_MONEY_INVARIANT_WORKER=true in env.
	// To promote from shadow to live alerts: set TOTAL_MONEY_INVARIANT_SHADOW_MODE=false.
	// Current activation state is also surfaced at runtime via
	// worker.CriticalWorkerStatuses(), consumed by /health/ready.
	totalMoneyInvariantCfg := worker.ParseTotalMoneyInvariantConfig()
	totalMoneyInvariantChecker := walletApp.NewTotalMoneyInvariantChecker(
		alertService,
		db.Pgx(),
		log.Logger,
		totalMoneyInvariantCfg.ShadowMode,
	)
	totalMoneyInvariantWorker := worker.NewTotalMoneyInvariantWorker(
		totalMoneyInvariantChecker,
		log.Logger,
		totalMoneyInvariantCfg.Interval,
		totalMoneyInvariantCfg.ShadowMode,
	)
	if workerEnabled("TOTAL_MONEY_INVARIANT_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			totalMoneyInvariantWorker.Start()
			if totalMoneyInvariantCfg.ShadowMode {
				log.Warn("TotalMoneyInvariantWorker started in shadow mode — ledger violations logged only, no alerts will be raised",
					zap.Duration("interval", totalMoneyInvariantCfg.Interval),
					zap.String("promote", "TOTAL_MONEY_INVARIANT_SHADOW_MODE=false"),
				)
			} else {
				log.Info("TotalMoneyInvariantWorker started",
					zap.Bool("shadow_mode", false),
					zap.Duration("interval", totalMoneyInvariantCfg.Interval),
				)
			}
		})
	} else {
		log.Error("TotalMoneyInvariantWorker disabled — ledger sum invariants will NOT be checked",
			zap.String("enable", "DISABLE_TOTAL_MONEY_INVARIANT_WORKER=false"),
		)
		_ = totalMoneyInvariantWorker
	}

	// =============================================================================
	// RECONCILIATION SYSTEM V2 — VERIFICATION + ESCALATION ONLY
	// =============================================================================
	// Constitutional role (RUNTIME-INVARIANTS §7.1, ADR-002):
	// - Detects ledger inconsistencies (double-entry violations, balance drift)
	// - Persistent audit trail in reconciliation_results
	// - Alert integration for on-call response
	// - Corrective journal entries are NOT this worker's job. Any correction
	//   is the responsibility of an attributable operator invoking canonical
	//   FinanceService methods (e.g. RecordRefundReversal). There is no
	//   "auto-repair" surface anywhere in the codebase.

	// Initialize reconciliation repository for persistent audit trail
	reconcileRepo := financeRepo.NewReconciliationRepository(*db.Pgx())

	// Admin read-only visibility into persisted reconciliation results (list,
	// detail, latest). Strictly GET — no mutation, no repair, no financial
	// authority; see AdminReconciliationHandler doc comment.
	adminReconciliationHandler := financePayoutHTTP.NewAdminReconciliationHandler(
		db,
		reconcileRepo,
		log.Logger,
	)

	reconciliationWorker := worker.NewReconciliationWorkerV2(
		db,
		log.Logger,
		reconcileRepo,
		alertService,
		worker.ReconciliationConfigV2{
			Interval:            5 * time.Minute,
			Strict:              false,
			EnableAlerting:      true,
			EscalationThreshold: 100, // drift > 1 currency unit escalates non-critical mismatches to HIGH
		},
	)
	workerStartups = append(workerStartups, func() {
		reconciliationWorker.Start() // ENABLED: Financial integrity monitoring (read-only)
	})

	// MODERATION DOMAIN — USER BAN EVENT HANDLER
	//
	// Processes user.banned events and triggers mass refund/dispute for all
	// active orders of the banned user.  Decision tree per order:
	//   buyer banned + shipped/delivered + no dispute → auto-complete (seller gets paid)
	//   seller banned + shipped/delivered             → force dispute (admin reviews)
	//   holding + no shipment evidence                → refund buyer
	//   holding + shipment evidence, other statuses   → force dispute
	//   escrow not holding, or already terminal       → no-op
	//
	// RefundOrder uses InitiateGatewayRefundForOrder (gateway-funded, canonical).
	// GUARD: requires ACK_DANGEROUS_USER_BAN_EVENT_HANDLER=true in
	// addition to DISABLE_USER_BAN_EVENT_HANDLER=false.
	// ───────────────────────────────────────────────────────────────────
	if workerEnabled("USER_BAN_EVENT_HANDLER", false, log.Logger) {
		dangerousDormantGuard("USER_BAN_EVENT_HANDLER", log.Logger)
		outboxWorker.SetupUserBanHandler(
			db.Pgx(),
			orderService,
			disputeService,
		)
		log.Info("UserBanEventHandler registered with outbox worker")
	} else {
		log.Warn("UserBanEventHandler disabled — user.banned events will not trigger mass refund/dispute resolution for active orders of banned users",
			zap.String("enable", "DISABLE_USER_BAN_EVENT_HANDLER=false + ACK_DANGEROUS_USER_BAN_EVENT_HANDLER=true"),
		)
	}

	// CHAT-3: WS eviction on ban — event-driven, not polling-based (ADR-005).
	// When SetupUserBanHandler is re-enabled, combine into a composite handler
	// to avoid overwriting (both register on "user.banned").
	outboxWorker.SetupWSEvictionHandler(realtimeHub)

	// Expired-seller visibility — cancel scheduled auctions when seller
	// subscription expires. Active auctions are NOT auto-cancelled in this
	// iteration (PlaceBid + Guard 6 already block new bids and order
	// settlement, so live auctions of expired sellers are functionally inert).
	outboxWorker.SetupSellerSubscriptionExpiredHandler(db.Pgx())

	// P5B-C: Setup promotion event handlers (mechanism A — responsive to target + seller events)
	// Registers handlers for for_sale.sold/withdrawn/updated, auction.ended/cancelled,
	// seller.subscription.activated/expired, seller.verification.restored/suspended/revoked,
	// moderation.for_sale.restored.
	// ORDERING: Must be called AFTER SetupNotificationHandlers, SetupModerationHandlers,
	// and SetupSellerSubscriptionExpiredHandler for fanout composition.
	outboxWorker.SetupPromotionHandlers(db.Pgx(), promotionService)

	// =============================================================================
	// DISPUTE HARDENING — DEADLOCK PREVENTION
	// =============================================================================
	// Dispute timeout worker enforces the dispute timeout policy:
	// - After 3 days: Mark dispute as overdue (for escalation)
	// - After timeout_days (default 14): Escalate for admin review (no auto-resolve)
	//
	// HARD RULE: NO dispute can block escrow/money forever.
	// SAFETY: Worker only writes is_overdue flag + outbox event. Zero money mutation.
	disputeTimeoutWorker := worker.NewDisputeTimeoutWorker(
		db,
		disputeRepository,
		disputeService,
		outboxRepository,
		log.Logger,
	)
	if workerEnabled("DISPUTE_TIMEOUT_WORKER", true, log.Logger) {
		workerStartups = append(workerStartups, func() {
			disputeTimeoutWorker.Start()
			log.Info("DisputeTimeoutWorker ENABLED — escalate-only policy (no auto-resolve)",
				zap.Duration("poll_interval", worker.DefaultDisputeTimeoutPollInterval),
				zap.Int("batch_size", worker.DefaultDisputeTimeoutBatchSize),
			)
		})
	}

	// =============================================================================
	// WALLET-FINANCE RECONCILIATION — FINAL SAFETY LAYER
	// =============================================================================
	return &Dependencies{
		// Handlers
		AuthHandler:            authHandler, // Phase 1: Canonical auth entry point
		ProductShippingHandler: productShippingHandler,
		ShippingHandler:        shippingHandler,       // Buyer-facing: check delivery availability
		SellerShippingHandler:  sellerShippingHandler, // Seller-facing: shipping option management
		PaymentHandler: &CorePaymentHandler{
			db:                  db,
			paymentRepo:         paymentRepo,
			paymentAttemptRepo:  repository.NewPaymentAttemptRepository(log.Logger),
			billingRepo:         billingrepo.NewBillingRepository(),
			orderRepo:           orderRepository, // PAYMENT BOUNDARY HARDENING: Order as source of truth
			paymentMethodRepo:   paymentMethodRepository,
			pricingTokenService: pricingTokenService,
			midtransClient:      midtransClient,
			log:                 log,
			isProduction:        isProduction,
			frontendURL:         cfg.App.FrontendURL,
		},
		CoinHandler:              &CoreCoinHandler{coinsService: coinsService},
		UserHandler:              &CoreUserHandler{db: db, roleChecker: roleChecker, log: log},
		UserProfileHandler:       userHandler,
		PaymentWebhookHandler:    paymentWebhookHandler,
		PayoutWebhookHandler:     payoutWebhookHandler,
		WithdrawalHandlerUnified: withdrawalHandlerUnified,
		BankAccountHandler:       bankAccountHandler,
		AddressHandler:           addressHandler,
		OrderHandler:             orderHandler,
		AuctionHandler:           auctionHandler,
		AdminAuctionHandler:      adminAuctionHandler,
		SavedItemHandler:         savedItemHandler,
		BiddingHandler:           biddingHandler,
		// CollectionHandler:     collectionHandler, // DISABLED: Collection domain being isolated for removal
		// OfferHandler:          offerHandler,      // DISABLED: Offer domain being isolated for removal
		ForSaleHandler:  forSaleHandler,
		PricingTokenHandler:    pricingTokenHandler,
		ChatHandler:            chatHandler,
		DiscountHandler:        discountHandler,
		DisputeHandler:         disputeHandler,
		SellerHandler:          sellerHandler,
		SystemHealthHandler:    systemHealthHandler,
		RealtimeHandler:        realtimeHandler,
		FeedHandler:            feedHandler,
		ContentHandler:         contentHandler,
		OGHandler:              ogHandler,
		CommentHandler:         commentHandler,
		LikeHandler:            likeHandler,
		NotificationHandler:    notificationHandler,
		FCMTokenHandler:        fcmTokenHandler,
		ModerationHandler:      moderationHandler,      // Trust MVP: moderation endpoints
		FollowHandler:          followHandler,          // SOCIAL domain: follow/block/mute
		ShippingQuoteHandler:   shippingQuoteHandler,   // Shipping quote feature (chat-based manual quotes)
		RatingHandler:          ratingHandler,          // RATING DOMAIN: buyer→seller order ratings
		PromotionHandler:       promotionHandler,       // PROMOTION PHASE 4: Discovery endpoints
		SearchHandler:          searchHandler,          // FEDERATED SEARCH: content, users, forSales
		AdminOrderHandler:      adminOrderHandler,      // ADMIN ORDER: read-only order management
		AdminRefundHandler:     adminRefundHandler,     // TASK 34 / Phase 2a: admin-only gateway refund trigger
		SellerRefundHandler:    sellerRefundHandler,    // H2-A: seller approve/reject refund
		BuyerEscalationHandler: buyerEscalationHandler, // H2-B: buyer escalate rejected refund

		// VERIFICATION (Phase 2 operationalization)
		VerificationHandler:      verificationHandler,
		AdminVerificationHandler: adminVerificationHandler,

		// MEDIA UPLOAD — general presigned S3 PUT URL
		MediaUploadHandler: mediaUploadHandler,

		// Admin / Operability handlers - ADMIN HARDENING PACK V1
		AdminHandler:                     adminHandler,                     // Admin dashboard, user management, metrics
		CapabilityHandler:                capabilityHandler,                // Capability management endpoints
		AppealHandler:                    appealHandler,                    // Appeals system for moderation
		WarningHandler:                   warningHandler,                   // Warnings system for policy violations
		SupportHandler:                   supportHandler,                   // Support ticket system
		AdminPayoutHandler:               adminPayoutHandler,               // Admin payout operations (approve, reject, list)
		AdminFinanceHandler:              adminFinanceHandler,              // Finance ledger export + verifier endpoint
		PlatformConfigHandler:            platformConfigHandler,            // MANAGEMENT PRE-FIX M1: Platform config with capability enforcement
		AdminSubscriptionConfigHandler:   adminSubscriptionConfigHandler,   // Admin seller subscription config CRUD
		AdminSubscriptionRecoveryHandler: adminSubscriptionRecoveryHandler, // Admin subscription payment recovery
		AdminAlertHandler:                adminAlertHandler,                // ALERT SYSTEM V1: Admin alert operations
		AdminReconciliationHandler:       adminReconciliationHandler,       // Admin reconciliation result visibility (read-only)
		AdminPaymentMethodHandler:        adminPaymentMethodHandler,        // PASS_18W: Admin payment method fee config

		// Services
		RoleChecker: roleChecker,

		// SLICE 1: Capability Storage Foundation
		CapabilityChecker: capabilityChecker,
		ActorResolver:     actorResolver,

		// Canonical order service exposed for corpus_driver scenarios.
		OrderService: orderService,

		// Workers
		PaymentExpiryWorker:              paymentExpiryWorker,
		ReconciliationWorker:             reconciliationWorker,
		OrderAutoCompleteWorker:          orderAutoCompleteWorker,
		OutboxWorker:                     outboxWorker,
		ProjectionWorkerFull:             projectionWorker,
		ProjectionAdminHandler:           projectionAdminHandler,
		ProjectionWorker:                 projectionWorker,
		AuctionStartWorker:               auctionStartWorker,
		AuctionEndWorker:                 auctionEndWorker,
		SystemMonitoringWorker:           systemMonitoringWorker,
		RealtimeWorker:                   realtimeWorker,
		PayoutWorker:                     payoutWorkerWrapper,
		PayoutReconciliationWorker:       payoutReconWorkerWrapper,
		NegotiationExpireWorker:          negotiationExpireWorker,
		PromotionSafetyWorker:            promotionSafetyWorker, // RUNTIME HARDENING PACK V1
		OrderOverdueReminderWorker:       orderOverdueReminderWorker,
		DisputeTimeoutWorker:             disputeTimeoutWorker,             // DISPUTE HARDENING - DEADLOCK PREVENTION
		AlertDetectionWorker:             alertDetectionWorker,             // ALERT SYSTEM V1
		SellerSubscriptionExpiryWorker:   sellerSubscriptionExpiryWorker,   // SUBSCRIPTION LIFECYCLE - hourly active→expired sweep
		OutboxArchivalWorker:             outboxArchivalWorker,             // Z4-1: OUTBOX RETENTION
		OrderOverdueCancelWorker:         orderOverdueCancelWorker,         // Z4-2: ORDER FULFILLMENT
		SubscriptionReconciliationWorker: subscriptionReconciliationWorker, // Z4-3: SUBSCRIPTION HARDENING
		WithdrawalMonitoringWorker:       withdrawalMonitoringWorker,       // Z4-4: PAYOUT MONITORING
		PushRetryWorker:                  pushRetryWorker,                  // Z6-1: PUSH RELIABILITY
		NotificationCleanupWorker:        notificationCleanupWorker,        // Z6-2: PUSH HYGIENE
		EscrowIntegrityWorker:            escrowIntegrityWorker,            // ESCROW RECONCILIATION (shadow default)
		TotalMoneyInvariantWorker:        totalMoneyInvariantWorker,        // TOTAL MONEY INVARIANT (shadow default)
		BNRDecayWorker:                   bnrDecayWorker,                   // BNR FORGIVENESS - daily 180d decay
		SellerMetricsWorker:              sellerMetricsWorker,              // SELLER MEASUREMENT - daily fulfillment snapshot
		SellerReputationRecomputeWorker:  sellerReputationRecomputeWorker,  // REPUTATION AUTHORITY - nightly 90-day rolling recompute

		workerStartups: workerStartups,
	}
}

// StartWorkers invokes each deferred worker-startup closure that
// InitServices recorded, in the order recorded. Each closure is one
// original `worker.Start()` call site plus its adjacent startup logging.
// Production startup is `deps := InitServices(...); StartWorkers(deps)` —
// this is the canonical pairing. corpus_driver intentionally skips this
// call so it can drive scenarios against the service graph with the worker
// goroutines dormant.
//
// Safe to call multiple times: workerStartups is consumed (set to nil)
// after the first invocation to make double-start a no-op rather than a
// goroutine leak.
func StartWorkers(deps *Dependencies) {
	if deps == nil {
		return
	}
	startups := deps.workerStartups
	deps.workerStartups = nil
	for _, start := range startups {
		start()
	}
}

func registerOrphanWebhookRecoveryWorkerStartup(
	appCtx context.Context,
	workerStartups *[]func(),
	enabled bool,
	start func(context.Context),
	log *zap.Logger,
) {
	if !enabled {
		return
	}
	if appCtx == nil {
		appCtx = context.Background()
	}
	*workerStartups = append(*workerStartups, func() {
		go start(appCtx)
		log.Info("OrphanWebhookRecoveryWorker ENABLED - orphan recovery adapter active")
	})
}

// wrapWorkerOrNil wraps a worker that might be nil in a no-op stub.
func wrapWorkerOrNil(w interface{}) Worker {
	if w == nil {
		return &noopWorker{}
	}
	return w.(Worker)
}

// noopWorker is a stub worker that does nothing.
type noopWorker struct{}

func (n *noopWorker) Start()          {}
func (n *noopWorker) Stop()           {}
func (n *noopWorker) IsRunning() bool { return false }

// =============================================================================
// STUB IMPLEMENTATIONS
// =============================================================================

// CorePaymentHandler handles payment requests
type MidtransGateway interface {
	CreateSnapTransaction(req *midtrans.SnapRequest) (*midtrans.SnapResponse, error)
	IsProduction() bool
}

type CorePaymentHandler struct {
	db                  *database.DB
	paymentRepo         *repository.PaymentRepository
	paymentAttemptRepo  *repository.PaymentAttemptRepository
	billingRepo         *billingrepo.BillingRepository
	orderRepo           *orderRepo.OrderRepository // PAYMENT BOUNDARY HARDENING: Order as source of truth
	paymentMethodRepo   *paymentmethodrepo.PaymentMethodRepository
	pricingTokenService interface {
		GetSnapshot(ctx context.Context, tx db.Tx, token uuid.UUID) (*pricingtokenentity.PricingToken, error)
	}
	coinsRepo interface {
		CreateReservation(ctx context.Context, tx db.Tx, reservation *coinsEntity.CoinReservation) error
		GetReservationByPaymentID(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*coinsEntity.CoinReservation, error)
		ReleaseReservation(ctx context.Context, tx db.Tx, paymentID uuid.UUID) (*coinsEntity.CoinReservation, error)
	}
	midtransClient MidtransGateway
	log            *logger.Logger
	isProduction   bool
	frontendURL    string // for Snap finish callback (cfg.App.FrontendURL)
}

// CreatePaymentRequest holds the request payload for creating a payment
type CreatePaymentRequest struct {
	OrderID uuid.UUID `json:"order_id" binding:"required"`
	// PaymentMethodCode is the canonical method the buyer selected (see
	// payment_methods table / GET /payments/methods). The backend looks up
	// the method's fee formula and computes gross_amount itself — the
	// client never submits a fee or gross amount (PASS_18V).
	PaymentMethodCode string     `json:"payment_method_code" binding:"required"`
	CoinsToUse        int        `json:"coins_to_use"`
	PriceSnapshotID   *uuid.UUID `json:"price_snapshot_id"`
}

// CreateBillingPaymentRequest holds the request payload for initiating a billing payment.
type CreateBillingPaymentRequest struct {
	BillingID uuid.UUID `json:"billing_id" binding:"required"`
}

func (h *CorePaymentHandler) loadOrderPricingTokenSnapshot(
	ctx context.Context,
	tx db.Tx,
	order *orderEntity.Order,
) (*pricingtokenentity.PricingToken, error) {
	if order.PricingTokenID == nil || *order.PricingTokenID == uuid.Nil {
		return nil, fmt.Errorf("order is missing pricing token snapshot")
	}
	if h.pricingTokenService == nil {
		return nil, fmt.Errorf("pricing token service not configured")
	}

	pricingToken, err := h.pricingTokenService.GetSnapshot(ctx, tx, *order.PricingTokenID)
	if err != nil {
		return nil, err
	}
	if pricingToken == nil {
		return nil, fmt.Errorf("pricing token snapshot not found")
	}

	expectedBase := pricingToken.OrderValueForCoins + pricingToken.ShippingTotal.Int64()
	if order.TotalBeforeCoinsAmount.Int64() != expectedBase {
		return nil, fmt.Errorf("order pricing snapshot mismatch")
	}

	return pricingToken, nil
}

// CreatePayment creates a new payment for an order
// PAYMENT BOUNDARY HARDENING V1: Payment amount is DERIVED from Order, not received from client
func (h *CorePaymentHandler) CreatePayment(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// =============================================================================
	// PAYMENT BOUNDARY HARDENING V1: PHASE 1 - HARD VALIDATION
	// =============================================================================
	// RULE 1: CLIENT IS NOT SOURCE OF TRUTH - Fetch order to get EscrowAmount
	// RULE 2: ORDER = SINGLE SOURCE OF PAYMENT AMOUNT
	// RULE 3: NO SILENT CORRECTION - Reject if mismatch
	// =============================================================================

	var order *orderEntity.Order
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		order, err = h.orderRepo.GetByID(ctx, tx, req.OrderID)
		return err
	})
	if err != nil {
		h.log.Error("Order not found for payment validation",
			zap.String("order_id", req.OrderID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Order not found")
		return
	}

	// Validate that the order belongs to the authenticated user (buyer check)
	if order.BuyerID != userID {
		h.log.Warn("Payment creation attempted by non-buyer",
			zap.String("order_id", req.OrderID.String()),
			zap.String("user_id", userID.String()),
			zap.String("buyer_id", order.BuyerID.String()),
		)
		response.Forbidden(c, "You can only create payment for your own orders")
		return
	}

	// =============================================================================
	// PAYMENT BOUNDARY HARDENING V2: ORDER PAYABILITY GUARD
	// =============================================================================
	// RULE: Payment row INSERT and Midtrans Snap call are ONLY allowed when the
	// order is still payable. An order is payable when:
	//   1. Status == pending_payment  (the only status that accepts a payment)
	//   2. PaymentExpiresAt > now     (the payment window has not closed)
	//
	// This guard runs AFTER ownership check and BEFORE any payment-side write,
	// so a cancelled/expired/paid/completed/refunded order can never reach
	// Midtrans or produce a dangling payment row.
	// =============================================================================
	if order.Status != orderEntity.StatusPending {
		h.log.Warn("Payment creation rejected: order not in pending_payment status",
			zap.String("order_id", req.OrderID.String()),
			zap.String("order_status", string(order.Status)),
			zap.String("user_id", userID.String()),
		)
		response.Conflict(c, fmt.Sprintf(
			"Order is not payable. Current status: %s", order.Status,
		))
		return
	}
	if time.Now().After(order.PaymentExpiresAt) {
		h.log.Warn("Payment creation rejected: payment window expired",
			zap.String("order_id", req.OrderID.String()),
			zap.Time("payment_expires_at", order.PaymentExpiresAt),
			zap.String("user_id", userID.String()),
		)
		response.Gone(c, "Payment window has expired for this order")
		return
	}

	// =============================================================================
	// PAYMENT REUSE GUARD: Return existing active pending payment (idempotency)
	// =============================================================================
	// If a non-expired pending payment with a Snap URL already exists for this
	// order, return it immediately. This prevents duplicate Midtrans calls when
	// the buyer retries the pay-now CTA (e.g. after closing the Snap sheet).
	// =============================================================================
	coinsToUse := req.CoinsToUse
	if coinsToUse < 0 {
		response.BadRequest(c, "coins_to_use must be non-negative")
		return
	}

	// =============================================================================
	// PASS_18V: PAYMENT METHOD RESOLUTION + BUYER FEE CALCULATION
	// =============================================================================
	// The buyer selects a payment method code (see GET /payments/methods);
	// the backend is the sole authority for the fee. The client never
	// submits a fee or gross amount. Buyer coins reduce the cash base first,
	// then the method fee is calculated on the reduced cash amount.
	// =============================================================================
	var method *paymentmethodentity.Method
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		method, err = h.paymentMethodRepo.GetByCode(ctx, tx, req.PaymentMethodCode)
		return err
	})
	if err != nil {
		if errors.Is(err, paymentmethodrepo.ErrMethodNotFound) {
			response.BadRequest(c, fmt.Sprintf("Unknown payment method: %s", req.PaymentMethodCode))
			return
		}
		h.log.Error("Failed to load payment method",
			zap.String("method_code", req.PaymentMethodCode),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to load payment method")
		return
	}
	if !method.Enabled {
		response.BadRequest(c, fmt.Sprintf("Payment method is disabled: %s", method.Code))
		return
	}

	baseAmount := order.TotalBeforeCoinsAmount
	var pricingToken *pricingtokenentity.PricingToken
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		pricingToken, err = h.loadOrderPricingTokenSnapshot(ctx, tx, order)
		return err
	})
	if err != nil {
		h.log.Error("Failed to load order pricing token snapshot",
			zap.String("order_id", req.OrderID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to load pricing token snapshot")
		return
	}

	if coinsToUse > int(baseAmount.Int64()) {
		response.BadRequest(c, "coins_to_use cannot exceed the order amount")
		return
	}
	maxCoins := pricingToken.MaxCoinsAllowed
	if int64(coinsToUse) > maxCoins {
		response.BadRequest(c, fmt.Sprintf("coins_to_use exceeds max allowed (%d)", maxCoins))
		return
	}

	cashAmount := baseAmount.Sub(money.New(int64(coinsToUse)))
	buyerPaymentFee, err := paymentmethodentity.CalculateFee(cashAmount, *method)
	if err != nil {
		h.log.Error("Failed to calculate buyer payment fee",
			zap.String("order_id", req.OrderID.String()),
			zap.String("method_code", method.Code),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to calculate payment fee")
		return
	}
	grossMoney := cashAmount.Add(buyerPaymentFee)
	if grossMoney.Int64() <= 0 {
		response.BadRequest(c, "Gross amount must be positive")
		return
	}

	if baseAmount.LessThan(order.CommissionAmount) {
		h.log.Error("PAYMENT BOUNDARY VIOLATION: Order base amount < CommissionAmount",
			zap.String("order_id", req.OrderID.String()),
			zap.Int64("base_amount", baseAmount.Int64()),
			zap.Int64("commission_amount", order.CommissionAmount.Int64()),
		)
		response.InternalServerError(c, "Invalid order configuration: base amount below commission")
		return
	}

	paymentNumber := fmt.Sprintf("PAY-%d", time.Now().UnixNano())
	midtransOrderID := fmt.Sprintf("LAB-%s", uuid.New().String())
	expiredAt := order.PaymentExpiresAt

	var payment *repository.Payment
	var paymentAttempt *repository.PaymentAttempt
	var reusedActivePayment bool
	methodCode := method.Code
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		existingPayment, err := h.paymentRepo.FindExistingPaymentForOrder(ctx, tx, req.OrderID)
		if err != nil && err.Error() != "no rows in result set" {
			return err
		}
		if existingPayment != nil && existingPayment.ExpiredAt.After(time.Now()) {
			sameCoins := existingPayment.CoinsToUse == coinsToUse
			sameMethod := existingPayment.PaymentMethodCode != nil && *existingPayment.PaymentMethodCode == methodCode
			if !sameCoins || !sameMethod {
				return fmt.Errorf("active payment already exists with different intent")
			}
			payment = existingPayment
			reusedActivePayment = true
			return nil
		}

		input := repository.CreatePaymentInput{
			UserID:            userID,
			PaymentNumber:     paymentNumber,
			MidtransOrderID:   midtransOrderID,
			GrossAmount:       grossMoney,
			ServiceFeeAmount:  buyerPaymentFee,
			CoinsToUse:        coinsToUse,
			ReferenceType:     repository.ReferenceTypeOrder,
			ReferenceID:       &req.OrderID,
			PriceSnapshotID:   req.PriceSnapshotID,
			ExpiredAt:         expiredAt,
			PaymentMethodCode: &methodCode,
		}

		p, err := h.paymentRepo.CreatePayment(ctx, tx, input)
		if err != nil {
			if errors.Is(err, repository.ErrReferenceIDRequired) {
				return fmt.Errorf("payment reference_id is required")
			}
			if db.IsUniqueViolation(err) || strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
				existingPayment, lookupErr := h.paymentRepo.FindExistingPaymentForOrder(ctx, tx, req.OrderID)
				if lookupErr != nil {
					return lookupErr
				}
				if existingPayment != nil {
					sameCoins := existingPayment.CoinsToUse == coinsToUse
					sameMethod := existingPayment.PaymentMethodCode != nil && *existingPayment.PaymentMethodCode == methodCode
					if !sameCoins || !sameMethod {
						return fmt.Errorf("active payment already exists with different intent")
					}
					payment = existingPayment
					reusedActivePayment = true
					return nil
				}
			}
			return err
		}

		payment = p

		if coinsToUse > 0 {
			if h.coinsRepo == nil {
				return fmt.Errorf("coins repository not configured")
			}
			reservation, err := coinsEntity.NewCoinReservation(payment.ID, userID, int64(coinsToUse), expiredAt)
			if err != nil {
				return fmt.Errorf("create coin reservation: %w", err)
			}
			if err := h.coinsRepo.CreateReservation(ctx, tx, reservation); err != nil {
				return err
			}
		}

		if err := h.orderRepo.UpdatePaymentSelectionTx(ctx, tx, req.OrderID, buyerPaymentFee, grossMoney); err != nil {
			return fmt.Errorf("update order payment selection: %w", err)
		}

		if h.paymentAttemptRepo != nil {
			userAgent := c.GetHeader("User-Agent")
			ipAddress := c.ClientIP()
			attempt, err := h.paymentAttemptRepo.Create(ctx, tx, repository.CreatePaymentAttemptInput{
				OrderID:               req.OrderID,
				UserID:                userID,
				UserAgent:             &userAgent,
				IPAddress:             &ipAddress,
				PaymentMethodSelected: &methodCode,
			})
			if err != nil {
				h.log.Warn("Failed to create payment attempt (non-critical)",
					zap.String("order_id", req.OrderID.String()),
					zap.Error(err),
				)
			} else {
				paymentAttempt = attempt
			}
		}

		return nil
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "active payment already exists with different intent") {
			response.Conflict(c, "Active payment already exists with different intent")
			return
		}
		var reservationConflict *coinsEntity.ErrReservationConflict
		if errors.As(err, &reservationConflict) {
			response.Conflict(c, reservationConflict.Error())
			return
		}
		var reservationInsufficient *coinsEntity.ErrReservationInsufficientBalance
		if errors.As(err, &reservationInsufficient) {
			response.Conflict(c, reservationInsufficient.Error())
			return
		}
		h.log.Error("Failed to create payment inside transaction",
			zap.String("order_id", req.OrderID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to create payment")
		return
	}
	if payment == nil {
		response.InternalServerError(c, "Failed to create payment")
		return
	}
	if reusedActivePayment {
		paymentURL := ""
		if payment.PaymentURL != nil {
			paymentURL = *payment.PaymentURL
		}
		response.Success(c, gin.H{
			"payment_id":               payment.ID,
			"status":                   payment.Status,
			"payment_number":           payment.PaymentNumber,
			"payment_url":              paymentURL,
			"payment_method_code":      methodCode,
			"buyer_payment_fee_amount": payment.ServiceFeeAmount.Int64(),
			"gross_amount":             payment.GrossAmount.Int64(),
			"coins_to_use":             payment.CoinsToUse,
			"coin_discount_amount":     payment.CoinDiscountAmount.Int64(),
			"reference_type":           payment.ReferenceType,
			"reference_id":             payment.ReferenceID,
			"expired_at":               payment.ExpiredAt,
		})
		return
	}
	if payment.PaymentURL != nil && *payment.PaymentURL != "" {
		response.Success(c, gin.H{
			"payment_id":               payment.ID,
			"status":                   payment.Status,
			"payment_number":           payment.PaymentNumber,
			"payment_url":              *payment.PaymentURL,
			"payment_method_code":      methodCode,
			"buyer_payment_fee_amount": payment.ServiceFeeAmount.Int64(),
			"gross_amount":             payment.GrossAmount.Int64(),
			"coins_to_use":             payment.CoinsToUse,
			"coin_discount_amount":     payment.CoinDiscountAmount.Int64(),
			"reference_type":           payment.ReferenceType,
			"reference_id":             payment.ReferenceID,
			"expired_at":               payment.ExpiredAt,
		})
		return
	}
	paymentURL, err := h.createMidtransTransaction(ctx, payment, order, paymentAttempt, method.MidtransChannels)
	if err != nil {
		if h.isDefinitiveMidtransSnapRefusal(err) {
			if compErr := h.compensateDefinitiveMidtransRefusal(ctx, payment.ID, repository.PaymentStatusDeny); compErr != nil {
				h.log.Error("Failed to compensate definitive Midtrans refusal",
					zap.String("payment_id", payment.ID.String()),
					zap.Error(compErr),
				)
				response.InternalServerError(c, "Failed to initialize payment gateway")
				return
			}
			response.InternalServerError(c, "Failed to initialize payment gateway")
			return
		}
		h.log.Error("Failed to create Midtrans transaction",
			zap.String("payment_id", payment.ID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to initialize payment gateway")
		return
	}

	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		return h.updatePaymentURL(ctx, tx, payment.ID, paymentURL)
	})
	if err != nil {
		h.log.Error("Failed to update payment URL",
			zap.String("payment_id", payment.ID.String()),
			zap.Error(err),
		)
	}

	response.Success(c, gin.H{
		"payment_id":               payment.ID,
		"status":                   payment.Status,
		"payment_number":           payment.PaymentNumber,
		"payment_url":              paymentURL,
		"payment_method_code":      methodCode,
		"buyer_payment_fee_amount": payment.ServiceFeeAmount.Int64(),
		"gross_amount":             payment.GrossAmount.Int64(),
		"coins_to_use":             payment.CoinsToUse,
		"coin_discount_amount":     payment.CoinDiscountAmount.Int64(),
		"reference_type":           payment.ReferenceType,
		"reference_id":             payment.ReferenceID,
		"expired_at":               payment.ExpiredAt,
	})
}

// ListPaymentMethods returns the enabled canonical payment methods, each with
// the buyer payment fee and resulting total calculated for the given order's
// canonical buyer base (total_before_coins_amount = (P−D)+S; the payment fee
// is computed on the cash portion after coin deduction). Mobile/admin
// use this to render a method picker BEFORE calling CreatePayment — they
// never calculate the fee themselves (PASS_18V).
func (h *CorePaymentHandler) ListPaymentMethods(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	orderIDParam := c.Query("order_id")
	orderID, err := uuid.Parse(orderIDParam)
	if err != nil {
		response.BadRequest(c, "order_id query parameter is required and must be a valid UUID")
		return
	}

	var order *orderEntity.Order
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		order, err = h.orderRepo.GetByID(ctx, tx, orderID)
		return err
	})
	if err != nil {
		response.NotFound(c, "Order not found")
		return
	}
	if order.BuyerID != userID {
		response.Forbidden(c, "You can only view payment methods for your own orders")
		return
	}

	var methods []paymentmethodentity.Method
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		methods, err = h.paymentMethodRepo.ListEnabled(ctx, tx)
		return err
	})
	if err != nil {
		h.log.Error("Failed to list payment methods", zap.Error(err))
		response.InternalServerError(c, "Failed to load payment methods")
		return
	}

	baseAmount := order.TotalBeforeCoinsAmount
	var pricingToken *pricingtokenentity.PricingToken
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		pricingToken, err = h.loadOrderPricingTokenSnapshot(ctx, tx, order)
		return err
	})
	if err != nil {
		h.log.Error("Failed to load order pricing token snapshot",
			zap.String("order_id", orderID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to load pricing token snapshot")
		return
	}

	coinsToUse := 0
	if coinsToUseParam := c.Query("coins_to_use"); coinsToUseParam != "" {
		parsedCoins, parseErr := strconv.Atoi(coinsToUseParam)
		if parseErr != nil || parsedCoins < 0 {
			response.BadRequest(c, "coins_to_use query parameter must be a non-negative integer")
			return
		}
		coinsToUse = parsedCoins
	}
	maxCoins := pricingToken.MaxCoinsAllowed
	if int64(coinsToUse) > maxCoins {
		response.BadRequest(c, fmt.Sprintf("coins_to_use exceeds max allowed (%d)", maxCoins))
		return
	}
	cashAmount := baseAmount.Sub(money.New(int64(coinsToUse)))

	out := make([]gin.H, 0, len(methods))
	for _, m := range methods {
		fee, err := paymentmethodentity.CalculateFee(cashAmount, m)
		if err != nil {
			h.log.Warn("Skipping payment method with invalid fee formula",
				zap.String("method_code", m.Code), zap.Error(err))
			continue
		}
		out = append(out, gin.H{
			"method_code":              m.Code,
			"display_name":             m.DisplayName,
			"coins_to_use":             coinsToUse,
			"cash_amount":              cashAmount.Int64(),
			"buyer_payment_fee_amount": fee.Int64(),
			"total_payable_amount":     cashAmount.Add(fee).Int64(),
		})
	}

	response.Success(c, gin.H{
		"order_id":      order.ID,
		"base_amount":   baseAmount.Int64(),
		"escrow_amount": baseAmount.Int64(),
		"coins_to_use":  coinsToUse,
		"methods":       out,
	})
}

// CreateBillingPayment creates a payment for a billing transaction.
//
// Canonical use-case: promotion package purchase.
// Source of truth for amount is billing.gross_amount (server-derived).
func (h *CorePaymentHandler) CreateBillingPayment(c *gin.Context) {
	ctx := c.Request.Context()

	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	var req CreateBillingPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	var (
		billing *billingentity.BillingTransaction
		payment *repository.Payment
	)
	err := h.db.WithTx(ctx, func(tx db.Tx) error {
		var fetchErr error
		billing, fetchErr = h.billingRepo.GetForUpdate(ctx, tx, req.BillingID)
		if fetchErr != nil {
			return fetchErr
		}

		if billing.PayerID != userID {
			return auth.ErrOwnerRequired
		}
		if billing.Status != billingentity.StatusPending {
			return fmt.Errorf("billing is not pending")
		}

		existing, existingErr := h.paymentRepo.GetPaymentByReference(
			ctx, tx, repository.ReferenceTypeBilling, req.BillingID,
		)
		if existingErr == nil && existing != nil &&
			existing.Status == repository.PaymentStatusPending &&
			existing.ExpiredAt.After(time.Now()) {
			payment = existing
			return nil
		}

		paymentNumber := fmt.Sprintf("PAY-BILL-%d", time.Now().UnixNano())
		midtransOrderID := fmt.Sprintf("LAB-BILL-%s", uuid.New().String())
		expiredAt := time.Now().Add(24 * time.Hour)
		referenceID := req.BillingID

		created, createErr := h.paymentRepo.CreatePayment(ctx, tx, repository.CreatePaymentInput{
			UserID:           userID,
			PaymentNumber:    paymentNumber,
			MidtransOrderID:  midtransOrderID,
			GrossAmount:      billing.GrossAmount,
			ServiceFeeAmount: money.Zero(),
			CoinsToUse:       0,
			ReferenceType:    repository.ReferenceTypeBilling,
			ReferenceID:      &referenceID,
			PriceSnapshotID:  nil,
			ExpiredAt:        expiredAt,
		})
		if createErr != nil {
			return createErr
		}
		payment = created
		return nil
	})

	if err != nil {
		if errors.Is(err, auth.ErrOwnerRequired) {
			response.Forbidden(c, "You can only create payment for your own billing")
			return
		}
		if strings.Contains(err.Error(), "not pending") {
			response.Conflict(c, "Billing is not payable")
			return
		}
		h.log.Error("Failed to create billing payment",
			zap.String("user_id", userID.String()),
			zap.String("billing_id", req.BillingID.String()),
			zap.Error(err),
		)
		response.InternalServerError(c, "Failed to create billing payment")
		return
	}

	if payment == nil {
		response.InternalServerError(c, "Failed to create billing payment")
		return
	}

	var paymentURL string
	if payment.PaymentURL != nil && *payment.PaymentURL != "" &&
		payment.Status == repository.PaymentStatusPending &&
		payment.ExpiredAt.After(time.Now()) {
		paymentURL = *payment.PaymentURL
	} else {
		paymentURL, err = h.createMidtransTransaction(ctx, payment, nil, nil, nil)
		if err != nil {
			h.log.Error("Failed to create Midtrans transaction for billing payment",
				zap.String("payment_id", payment.ID.String()),
				zap.String("billing_id", req.BillingID.String()),
				zap.Error(err),
			)
			response.InternalServerError(c, "Failed to initialize payment gateway")
			return
		}

		err = h.db.WithTx(ctx, func(tx db.Tx) error {
			return h.updatePaymentURL(ctx, tx, payment.ID, paymentURL)
		})
		if err != nil {
			h.log.Error("Failed to update billing payment URL",
				zap.String("payment_id", payment.ID.String()),
				zap.Error(err),
			)
		}
	}

	response.Success(c, gin.H{
		"payment_id":     payment.ID,
		"payment_url":    paymentURL,
		"gross_amount":   payment.GrossAmount.Int64(),
		"expired_at":     payment.ExpiredAt,
		"reference_type": payment.ReferenceType,
		"reference_id":   payment.ReferenceID,
	})
}

// createMidtransTransaction calls the real Midtrans Snap API (sandbox-only in this build)
// and returns the redirect URL. Replaces the prior simulator stub.
//
// Safety contract:
//   - Refuses to run if the Midtrans client is nil.
//   - Refuses to run if the client is configured for production.
//   - Refuses an empty RedirectURL response from the provider.
//   - Marks payment_attempt.gateway_reached only AFTER a successful Snap response,
//     so the metric reflects actual provider reach.
func (h *CorePaymentHandler) createMidtransTransaction(
	ctx context.Context,
	payment *repository.Payment,
	order *orderEntity.Order,
	paymentAttempt *repository.PaymentAttempt,
	enabledPayments []string,
) (string, error) {
	if h.midtransClient == nil {
		return "", fmt.Errorf("midtrans client not configured")
	}
	if h.midtransClient.IsProduction() {
		return "", fmt.Errorf("midtrans production mode is forbidden in this build")
	}

	orderNumber := ""
	if order != nil && order.OrderNumber != nil {
		orderNumber = *order.OrderNumber
	}

	snapReq, err := buildSnapRequest(SnapBuilderInput{
		MidtransOrderID: payment.MidtransOrderID,
		GrossAmount:     payment.GrossAmount.Int64(),
		ExpiredAt:       payment.ExpiredAt,
		OrderNumber:     orderNumber,
		// Buyer left empty intentionally for STEP B — buyer enrichment is a separate
		// follow-up that requires injecting a user repository into the handler.
		Buyer:           SnapBuyerInfo{},
		FrontendURL:     h.frontendURL,
		Now:             time.Now(),
		EnabledPayments: enabledPayments,
	})
	if err != nil {
		return "", fmt.Errorf("build snap request: %w", err)
	}

	resp, err := h.midtransClient.CreateSnapTransaction(snapReq)
	if err != nil {
		return "", fmt.Errorf("midtrans snap call: %w", err)
	}
	if resp == nil || resp.RedirectURL == "" {
		return "", fmt.Errorf("midtrans snap returned empty redirect_url")
	}

	// Mark gateway_reached only after a confirmed real provider response.
	if paymentAttempt != nil && h.paymentAttemptRepo != nil {
		mErr := h.db.WithTx(ctx, func(tx db.Tx) error {
			return h.paymentAttemptRepo.MarkGatewayReached(ctx, tx, paymentAttempt.ID, payment.MidtransOrderID)
		})
		if mErr != nil {
			h.log.Warn("Failed to mark gateway reached (non-critical)",
				zap.String("payment_attempt_id", paymentAttempt.ID.String()),
				zap.Error(mErr),
			)
		}
	}

	h.log.Info("Midtrans Snap transaction created",
		zap.String("payment_id", payment.ID.String()),
		zap.String("midtrans_order_id", payment.MidtransOrderID),
		zap.String("snap_token", resp.Token),
	)

	return resp.RedirectURL, nil
}

func (h *CorePaymentHandler) updatePaymentURL(ctx context.Context, tx db.Tx, paymentID uuid.UUID, paymentURL string) error {
	query := `UPDATE payments SET payment_url = $1, updated_at = NOW() WHERE id = $2`
	_, err := tx.Exec(ctx, query, paymentURL, paymentID)
	return err
}

func (h *CorePaymentHandler) isDefinitiveMidtransSnapRefusal(err error) bool {
	var apiErr *midtrans.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.DefinitiveRefusal()
}

// compensateDefinitiveMidtransRefusal atomically terminalizes the payment and
// releases any reserved coins after a provider-side Snap refusal that proves
// the transaction was not created.
func (h *CorePaymentHandler) compensateDefinitiveMidtransRefusal(
	ctx context.Context,
	paymentID uuid.UUID,
	failedStatus string,
) error {
	return h.db.WithTx(ctx, func(tx db.Tx) error {
		payment, err := h.paymentRepo.GetByIDForUpdate(ctx, tx, paymentID)
		if err != nil {
			return fmt.Errorf("lock payment for definitive refusal compensation: %w", err)
		}

		switch {
		case payment.IsSettled():
			return nil
		case payment.IsPending():
			if err := h.paymentRepo.MarkAsFailed(ctx, tx, payment.ID, failedStatus); err != nil {
				return fmt.Errorf("terminalize payment after definitive refusal: %w", err)
			}
		case payment.IsFailed():
			// Already terminalized by a prior compensation attempt or another
			// canonical failure path. Keep going so we can converge reservation
			// state without overwriting a later success state.
		default:
			return fmt.Errorf("payment not eligible for definitive refusal compensation: current status=%s", payment.Status)
		}

		if payment.CoinsToUse <= 0 {
			return nil
		}

		reservation, err := h.coinsRepo.GetReservationByPaymentID(ctx, tx, payment.ID)
		if err != nil {
			return fmt.Errorf("load coin reservation for definitive refusal compensation: %w", err)
		}
		if reservation == nil {
			return fmt.Errorf("missing coin reservation for payment %s", payment.ID)
		}

		switch reservation.Status {
		case coinsEntity.CoinReservationStatusConsumed:
			return fmt.Errorf("reservation already consumed for payment %s", payment.ID)
		case coinsEntity.CoinReservationStatusReleased:
			return nil
		case coinsEntity.CoinReservationStatusReserved:
			if _, err := h.coinsRepo.ReleaseReservation(ctx, tx, payment.ID); err != nil {
				return fmt.Errorf("release coin reservation after definitive refusal: %w", err)
			}
			return nil
		default:
			return fmt.Errorf("reservation in unexpected status for payment %s: %s", payment.ID, reservation.Status)
		}
	})
}

func (h *CorePaymentHandler) GetPayment(c *gin.Context) {
	ctx := context.Background()

	// Get authenticated user ID from context
	userIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	// Parse payment ID from URL parameter
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid payment ID")
		return
	}

	// Fetch payment from database
	var payment *repository.Payment
	err = h.db.WithTx(ctx, func(tx db.Tx) error {
		var fetchErr error
		payment, fetchErr = h.paymentRepo.GetByID(ctx, tx, paymentID)
		return fetchErr
	})

	if err != nil {
		h.log.Error("Failed to fetch payment",
			zap.String("payment_id", paymentID.String()),
			zap.String("user_id", userID.String()),
			zap.Error(err),
		)
		response.NotFound(c, "Payment not found")
		return
	}

	// Authorization check: ensure user owns the payment
	if payment.UserID != userID {
		h.log.Warn("Unauthorized payment access attempt",
			zap.String("payment_id", paymentID.String()),
			zap.String("user_id", userID.String()),
			zap.String("payment_owner_id", payment.UserID.String()),
		)
		response.Forbidden(c, "You can only view your own payments")
		return
	}

	// Convert to response format (snake_case to match Flutter PaymentDto expectations)
	paymentData := gin.H{
		"id":                   payment.ID.String(),
		"payment_number":       payment.PaymentNumber,
		"user_id":              payment.UserID.String(),
		"gross_amount":         payment.GrossAmount.Int64(),
		"coins_to_use":         payment.CoinsToUse,
		"coin_discount_amount": payment.CoinDiscountAmount.Int64(),
		"status":               payment.Status,
		"midtrans_order_id":    payment.MidtransOrderID,
		"midtrans_transaction_id": func() *string {
			if payment.TransactionID != nil && *payment.TransactionID != "" {
				return payment.TransactionID
			}
			return nil
		}(),
		"midtrans_payment_type": func() *string {
			if payment.PaymentType != nil && *payment.PaymentType != "" {
				return payment.PaymentType
			}
			return nil
		}(),
		"reference_type": payment.ReferenceType,
		"reference_id": func() *string {
			if payment.ReferenceID != nil {
				refID := payment.ReferenceID.String()
				return &refID
			}
			return nil
		}(),
		"created_at":  payment.CreatedAt,
		"paid_at":     payment.PaidAt,
		"expired_at":  payment.ExpiredAt,
		"payment_url": payment.PaymentURL,
		"price_snapshot_id": func() *string {
			if payment.PriceSnapshotID != nil {
				psID := payment.PriceSnapshotID.String()
				return &psID
			}
			return nil
		}(),
		"updated_at": payment.UpdatedAt,
	}

	response.Success(c, paymentData)
}

// CoreCoinHandler handles coin requests
type CoreCoinHandler struct {
	coinsService *coinsApp.CoinsService
}

// GetBalance returns the user's current coin balance with lifetime statistics
// GET /api/v1/coins/balance
//
// TRUTH ALIGNMENT: All fields are real data or null. No fake values.
// - lifetime_earned/spent: Computed from actual transactions
// - created_at: First transaction timestamp, or null if no transactions
// - last_transaction_at: Latest transaction timestamp, or null if no transactions
func (h *CoreCoinHandler) GetBalance(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	userID := actor.ID

	// Get balance with lifetime stats from service (TRUTH-aligned)
	balance, err := h.coinsService.GetBalanceWithLifetime(ctx, userID)
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve coin balance")
		return
	}

	// Build response with real data (no fake values)
	// created_at: First transaction timestamp (HONEST - null if no transactions)
	// last_transaction_at: Latest transaction timestamp (HONEST - null if no transactions)
	response.Success(c, gin.H{
		"user_id":             userID.String(),
		"userId":              userID.String(), // Backward compatibility
		"balance":             balance.Balance,
		"updated_at":          balance.UpdatedAt,
		"updatedAt":           balance.UpdatedAt,          // Backward compatibility
		"lifetime_earned":     balance.LifetimeEarned,     // REAL: computed from transactions
		"lifetimeEarned":      balance.LifetimeEarned,     // Backward compatibility
		"lifetime_spent":      balance.LifetimeSpent,      // REAL: computed from transactions
		"lifetimeSpent":       balance.LifetimeSpent,      // Backward compatibility
		"created_at":          balance.FirstTransactionAt, // REAL: first transaction, null if none
		"createdAt":           balance.FirstTransactionAt, // Backward compatibility
		"last_transaction_at": balance.LastTransactionAt,  // REAL: latest transaction, null if none
		"lastTransactionAt":   balance.LastTransactionAt,  // Backward compatibility
	})
}

// GetTransactions returns the user's coin transaction history
// GET /api/v1/coins/transactions
// Query params: page (default 1), page_size (default 20)
func (h *CoreCoinHandler) GetTransactions(c *gin.Context) {
	ctx := c.Request.Context()

	// Get authenticated user ID from context
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	userID := actor.ID

	// Parse pagination parameters
	page := 1
	pageSize := 20
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	} else if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			pageSize = l
		}
	}

	// Get transactions from service
	txPage, err := h.coinsService.GetTransactions(ctx, userID, page, pageSize)
	if err != nil {
		response.InternalServerError(c, "Failed to retrieve transactions")
		return
	}

	// Build running balance and map transactions to response format
	runningBalance := int64(0)
	transactions := make([]gin.H, 0, len(txPage.Transactions))

	// Process transactions in reverse (oldest first) to calculate balance_after
	// But return them in latest-first order as expected by the API
	for i := len(txPage.Transactions) - 1; i >= 0; i-- {
		tx := txPage.Transactions[i]
		if tx.Type == coinsEntity.CoinTransactionTypeEarn {
			runningBalance += tx.Amount
		} else if tx.Type == coinsEntity.CoinTransactionTypeSpend {
			runningBalance -= tx.Amount
		}
	}

	for _, tx := range txPage.Transactions {
		// Update running balance for this transaction
		if tx.Type == coinsEntity.CoinTransactionTypeEarn {
			runningBalance -= tx.Amount
		} else if tx.Type == coinsEntity.CoinTransactionTypeSpend {
			runningBalance += tx.Amount
		}

		// balanceAfter is the balance BEFORE this transaction (for display)
		balanceAfter := runningBalance
		if tx.Type == coinsEntity.CoinTransactionTypeEarn {
			balanceAfter = runningBalance + tx.Amount
		}

		// Map reference_type to source_type for Flutter compatibility
		sourceType := mapReferenceTypeToSourceType(tx.ReferenceType)
		description := buildTransactionDescription(tx)

		var relatedID, referenceID *string
		if tx.ReferenceID != nil {
			s := tx.ReferenceID.String()
			relatedID = &s
			referenceID = &s
		}

		transactions = append(transactions, gin.H{
			"id":               tx.ID.String(),
			"user_id":          tx.UserID.String(),
			"userId":           tx.UserID.String(), // Backward compatibility
			"type":             string(tx.Type),
			"transaction_type": string(tx.Type), // Backward compatibility
			"source_type":      sourceType,
			"sourceType":       sourceType, // Backward compatibility
			"coin_type":        "regular",
			"coinType":         "regular", // Backward compatibility
			"amount":           tx.Amount,
			"balance_after":    balanceAfter,
			"balanceAfter":     balanceAfter, // Backward compatibility
			"related_id":       relatedID,
			"relatedId":        relatedID, // Backward compatibility
			"reference_id":     referenceID,
			"referenceId":      referenceID, // Backward compatibility
			"related_type":     string(tx.ReferenceType),
			"relatedType":      string(tx.ReferenceType), // Backward compatibility
			"description":      description,
			"metadata":         nil,
			"created_at":       tx.CreatedAt,
			"createdAt":        tx.CreatedAt, // Backward compatibility
			// expires_at removed - CoinsTransaction no longer has ExpiresAt field
			"expires_at": nil,
			"expiresAt":  nil, // Backward compatibility
		})
	}

	response.Success(c, gin.H{
		"transactions": transactions,
		"total_count":  txPage.TotalCount,
		"totalCount":   txPage.TotalCount, // Backward compatibility
		"page":         txPage.Page,
		"page_size":    txPage.PageSize,
		"pageSize":     txPage.PageSize, // Backward compatibility
		"has_more":     txPage.HasMore,
		"hasMore":      txPage.HasMore, // Backward compatibility
	})
}

// mapReferenceTypeToSourceType maps the internal reference_type to the expected source_type
// for Flutter compatibility
func mapReferenceTypeToSourceType(refType coinsEntity.CoinReferenceType) string {
	switch refType {
	case coinsEntity.CoinReferenceOrderReward:
		return "orderPayment"
	case coinsEntity.CoinReferenceOrderSpend:
		return "orderPayment"
	case coinsEntity.CoinReferenceRefundEarn:
		return "refundOrder"
	case coinsEntity.CoinReferenceRefundSpend:
		return "refundOrder"
	// CoinReferenceExpiry removed - no longer exists in coins entity
	default:
		return "unknown"
	}
}

// buildTransactionDescription creates a human-readable description for a transaction
func buildTransactionDescription(tx *coinsEntity.CoinsTransaction) string {
	switch tx.ReferenceType {
	case coinsEntity.CoinReferenceOrderReward:
		return "Earned from completed order"
	case coinsEntity.CoinReferenceOrderSpend:
		return "Spent on order"
	case coinsEntity.CoinReferenceRefundEarn:
		return "Refunded from cancelled order"
	case coinsEntity.CoinReferenceRefundSpend:
		return "Refunded to cancelled order"
	// CoinReferenceExpiry removed - no longer exists in coins entity
	default:
		return "Coin transaction"
	}
}

// CoreUserHandler handles user requests
// Note: GetMe and UpdateProfile removed - duplicated by UserProfileHandler
type CoreUserHandler struct {
	db          *database.DB
	roleChecker auth.RoleChecker
	log         *logger.Logger
}

type SetRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// SetRole handles PUT /api/v1/admin/users/:id/role
//
// SLICE 5: MIGRATED to capability-based auth with governance.role.assign
// DUAL PROTECTION: RequireAdminMiddleware (existing) + RequireCapability (new)
//
// This handler implements:
// - Secondary capability check (defense-in-depth)
// - No self-escalation guard
// - Invalid role guard
func (h *CoreUserHandler) SetRole(c *gin.Context) {
	ctx := c.Request.Context()

	// SLICE 5: Handler-level defense - check capability explicitly
	// This provides defense-in-depth even if middleware is bypassed
	actor := middleware.GetActorFromContext(c)
	if actor == nil {
		response.Unauthorized(c, "Authentication required")
		return
	}
	if !actor.HasCapability("governance.role.assign") {
		response.Forbidden(c, "Insufficient permissions: governance.role.assign required")
		return
	}

	callerIDVal, exists := c.Get("userID")
	if !exists {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	callerID, ok := callerIDVal.(uuid.UUID)
	if !ok {
		response.InternalServerError(c, "Invalid user ID in context")
		return
	}

	targetUserID, err := middleware.GetUUIDParam(c, "id")
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}

	var req SetRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// SLICE 5: No self-escalation guard
	// Users cannot modify their own role to admin
	if callerID == targetUserID {
		if req.Role == "admin" {
			h.logError(c, "Self-escalation attempt blocked", nil)
			response.Forbidden(c, "Cannot assign elevated role to yourself")
			return
		}
	}

	// SLICE 5: Invalid role guard (defense-in-depth)
	// Service layer also validates, but we check here too for early rejection
	validRoles := map[string]bool{
		"user":  true,
		"admin": true,
	}
	if !validRoles[req.Role] {
		response.BadRequest(c, "Invalid role: "+req.Role)
		return
	}

	roleCheckerDB, ok := h.roleChecker.(*auth.RoleCheckerDB)
	if !ok {
		response.InternalServerError(c, "Role checker not available")
		return
	}

	if err := roleCheckerDB.SetRole(ctx, callerID, targetUserID, req.Role); err != nil {
		h.logError(c, "Failed to set user role", err)
		response.InternalServerError(c, "Failed to update user role")
		return
	}

	response.Success(c, gin.H{
		"user_id": targetUserID,
		"role":    req.Role,
		"message": fmt.Sprintf("User role updated to %s", req.Role),
	})
}

func (h *CoreUserHandler) logError(c *gin.Context, msg string, err error) {
	if h.log != nil {
		h.log.Error(msg, zap.Error(err))
	} else {
		fmt.Printf("[ERROR] %s: %v\n", msg, err)
	}
}

// =============================================================================
// STUB SERVICES
// =============================================================================

type stubShippingService struct{}

func newStubShippingServiceWithRepos() *shippingApp.ShippingService {
	shippingOptionRepo := &stubShippingOptionRepository{}
	coverageRepo := &stubShippingCoverageRepository{}
	cityOverrideRepo := &stubCityOverrideRepository{}
	productShippingRepo := &stubProductShippingOptionRepository{}

	return shippingApp.NewShippingService(
		shippingOptionRepo,
		coverageRepo,
		cityOverrideRepo,
		productShippingRepo,
	)
}

func (s *stubShippingService) CheckDeliveryAvailability(
	ctx context.Context,
	tx db.Tx,
	input shippingApp.CheckDeliveryAvailabilityInput,
) ([]shippingApp.DeliveryOption, error) {
	return []shippingApp.DeliveryOption{}, nil
}

type stubShippingOptionRepository struct{}

func (r *stubShippingOptionRepository) Create(ctx context.Context, tx db.Tx, option *shippingEntity.ShippingOption) error {
	return nil
}
func (r *stubShippingOptionRepository) Update(ctx context.Context, tx db.Tx, option *shippingEntity.ShippingOption) error {
	return nil
}
func (r *stubShippingOptionRepository) GetByID(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingEntity.ShippingOption, error) {
	return nil, nil
}
func (r *stubShippingOptionRepository) GetForUpdate(ctx context.Context, tx db.Tx, id uuid.UUID) (*shippingEntity.ShippingOption, error) {
	return nil, nil
}
func (r *stubShippingOptionRepository) GetBySeller(ctx context.Context, tx db.Tx, sellerID uuid.UUID, onlyActive bool) ([]*shippingEntity.ShippingOption, error) {
	return nil, nil
}
func (r *stubShippingOptionRepository) GetByName(ctx context.Context, tx db.Tx, sellerID uuid.UUID, name string) (*shippingEntity.ShippingOption, error) {
	return nil, nil
}
func (r *stubShippingOptionRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
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
func (r *stubShippingCoverageRepository) GetByShippingOption(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID) ([]*shippingEntity.ShippingCoverage, error) {
	return nil, nil
}
func (r *stubShippingCoverageRepository) GetByOptionAndProvince(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID, provinceCode string) (*shippingEntity.ShippingCoverage, error) {
	return nil, nil
}
func (r *stubShippingCoverageRepository) Delete(ctx context.Context, tx db.Tx, id uuid.UUID) error {
	return nil
}
func (r *stubShippingCoverageRepository) DeleteByShippingOption(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID) error {
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

type stubProductShippingOptionRepository struct{}

func (r *stubProductShippingOptionRepository) Create(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingOptionID uuid.UUID, sortOrder int) error {
	return nil
}
func (r *stubProductShippingOptionRepository) Delete(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingOptionID uuid.UUID) error {
	return nil
}
func (r *stubProductShippingOptionRepository) GetByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*shippingEntity.ShippingOption, error) {
	return nil, nil
}
func (r *stubProductShippingOptionRepository) GetAvailableByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) ([]*shippingEntity.ShippingOption, error) {
	return nil, nil
}
func (r *stubProductShippingOptionRepository) DeleteByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) error {
	return nil
}
func (r *stubProductShippingOptionRepository) DeleteByShippingOption(ctx context.Context, tx db.Tx, shippingOptionID uuid.UUID) error {
	return nil
}
func (r *stubProductShippingOptionRepository) CreateBulk(ctx context.Context, tx db.Tx, productID uuid.UUID, shippingOptionIDs []uuid.UUID) error {
	return nil
}
func (r *stubProductShippingOptionRepository) CountByProduct(ctx context.Context, tx db.Tx, productID uuid.UUID) (int64, error) {
	return 0, nil
}

// =============================================================================
// REALTIME OUTBOX REPOSITORY ADAPTER
// =============================================================================

// realtimeOutboxRepositoryAdapter adapts the outbox repository to the realtime.OutboxRepository interface.
// The outbox repository returns repository.Event, which needs to be converted to realtime.Event.
type realtimeOutboxRepositoryAdapter struct {
	repo *outboxRepo.OutboxRepository
}

// FetchPendingBatch fetches pending events and converts them to realtime.Event format.
func (a *realtimeOutboxRepositoryAdapter) FetchPendingBatch(ctx context.Context, tx db.Tx, limit int) ([]realtime.Event, error) {
	events, err := a.repo.FetchPendingBatch(ctx, tx, limit)
	if err != nil {
		return nil, err
	}

	// Convert repository.Event to realtime.Event
	result := make([]realtime.Event, len(events))
	for i, e := range events {
		result[i] = realtime.Event{
			ID:            e.ID,
			AggregateType: e.AggregateType,
			AggregateID:   e.AggregateID,
			EventType:     e.EventType,
			Payload:       e.Payload,
			Status:        string(e.Status),
			RetryCount:    e.RetryCount,
			NextAttemptAt: e.NextAttemptAt,
		}
	}
	return result, nil
}

// MarkProcessing marks an event as being processed.
func (a *realtimeOutboxRepositoryAdapter) MarkProcessing(ctx context.Context, tx db.Tx, eventID uuid.UUID) error {
	return a.repo.MarkProcessing(ctx, tx, eventID)
}

// MarkSucceeded marks an event as successfully processed.
func (a *realtimeOutboxRepositoryAdapter) MarkSucceeded(ctx context.Context, tx db.Tx, eventID uuid.UUID) error {
	return a.repo.MarkSucceeded(ctx, tx, eventID)
}

// MarkFailedWithRetry marks an event as failed with retry info.
func (a *realtimeOutboxRepositoryAdapter) MarkFailedWithRetry(ctx context.Context, tx db.Tx, eventID uuid.UUID, retryCount int, nextAttemptAt time.Time) error {
	return a.repo.MarkFailedWithRetry(ctx, tx, eventID, retryCount, nextAttemptAt)
}

// =============================================================================
// SUPPORT CHAT SERVICE ADAPTER
// =============================================================================
// supportChatServiceAdapter adapts the chat service to the support application's ChatService interface.
// This is a minimal bridge to enable the support system to work with the existing chat infrastructure.
type supportChatServiceAdapter struct {
	chatService *chatApp.Service
}

// GetOrCreateSupportRoom creates or retrieves a support chat room for a user.
// Creates RoomTypeSupport with participant_b = uuid.Nil (system placeholder).
// Block-exempt: support rooms are never subject to user-block enforcement.
func (a *supportChatServiceAdapter) GetOrCreateSupportRoom(ctx context.Context, userID uuid.UUID) (*chatEntity.ChatRoom, error) {
	return a.chatService.GetOrCreateSupportRoom(ctx, userID)
}

// GetOrCreateSupportRoomWithContext creates or retrieves a support chat room with context.
// ORDER ↔ CHAT CONTINUITY: Stores linked_order_id in chat room context for mobile retrieval.
// DEPRECATED: Use CreateSupportTicketRoom for new tickets.
func (a *supportChatServiceAdapter) GetOrCreateSupportRoomWithContext(ctx context.Context, userID uuid.UUID, contextJSON json.RawMessage) (*chatEntity.ChatRoom, error) {
	return a.chatService.GetOrCreateSupportRoomWithContext(ctx, userID, contextJSON)
}

// CreateSupportTicketRoom creates or retrieves a support chat room for a ticket.
// Per UNIQUE(participant_a, participant_b, room_type), one support room per user.
// Context semantics: new room gets context; existing room keeps its context.
// Room type is RoomTypeSupport — enables block exemption and support.user_replied emission.
func (a *supportChatServiceAdapter) CreateSupportTicketRoom(ctx context.Context, userID uuid.UUID, ticketID uuid.UUID, contextJSON json.RawMessage) (*chatEntity.ChatRoom, error) {
	ticketContext := map[string]interface{}{
		"ticket_id": ticketID.String(),
		"type":      "support_ticket",
	}

	// Merge with provided context (e.g., linked_order_id)
	if len(contextJSON) > 0 {
		var providedContext map[string]interface{}
		if err := json.Unmarshal(contextJSON, &providedContext); err == nil {
			for k, v := range providedContext {
				ticketContext[k] = v
			}
		}
	}

	mergedContext, _ := json.Marshal(ticketContext)

	return a.chatService.GetOrCreateSupportRoomWithContext(ctx, userID, mergedContext)
}

// SendSystemMessage sends a system message to a support chat room.
// Uses ChatService.SendSystemMessage which bypasses participant checks, rate limits,
// and block enforcement. No outbox event emitted (system messages are internal).
func (a *supportChatServiceAdapter) SendSystemMessage(ctx context.Context, roomID uuid.UUID, body string) error {
	return a.chatService.SendSystemMessage(ctx, roomID, body)
}

// =============================================================================
// CHAT ORDER OWNERSHIP ADAPTER (PASS_6A / F1)
// =============================================================================
// chatOrderOwnershipAdapter adapts the order repository to chat's narrow
// OrderOwnershipReader contract (buyer/seller IDs only), so LinkOrderToChat
// can validate that a room's participants match the order it links to
// without the chat domain depending on the full order entity/repository.
type chatOrderOwnershipAdapter struct {
	orderRepo *orderRepo.OrderRepository
}

func (a *chatOrderOwnershipAdapter) GetOrderParticipants(ctx context.Context, tx db.Tx, orderID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	order, err := a.orderRepo.GetByID(ctx, tx, orderID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return order.BuyerID, order.SellerID, nil
}

// disputeServiceAdapter adapts the dispute service to match support service's interface
type disputeServiceAdapter struct {
	disputeService *disputeApp.DisputeService
}

func (a *disputeServiceAdapter) OpenDispute(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
	callerID uuid.UUID,
	input supportApp.OpenDisputeInput,
) (*interface{}, error) {
	// Convert support input to dispute input
	disputeInput := disputeApp.OpenDisputeInput{
		Reason:      input.Reason,
		Description: input.Description,
	}

	dispute, err := a.disputeService.OpenDispute(ctx, tx, orderID, callerID, disputeInput)
	if err != nil {
		return nil, err
	}

	// Return dispute as interface{}
	result := interface{}(dispute)
	return &result, nil
}

func (a *disputeServiceAdapter) GetDisputeByOrderID(
	ctx context.Context,
	tx db.Tx,
	orderID uuid.UUID,
) (*interface{}, error) {
	dispute, err := a.disputeService.GetDisputeByOrderID(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}

	// Return dispute as interface{}
	result := interface{}(dispute)
	return &result, nil
}

// =============================================================================
// PAYOUT PILOT MODE HELPER
// =============================================================================
// parsePilotWhitelist can be implemented in the future if pilot mode is
// needed for a new payout provider. It should be implemented as part of
// that provider's adapter, not as a standalone helper function.
// ============================================================================

// =============================================================================
// SOCIAL BLOCK CHECKER ADAPTER
// =============================================================================

// socialBlockCheckerAdapter satisfies negotiationApp.BlockChecker by wrapping the
// social repository's bidirectional ExistsBlock check behind a self-managed transaction.
// The negotiation service defines a tx-free interface; this adapter handles the tx detail.
type socialBlockCheckerAdapter struct {
	db   *db.DB
	repo interface {
		ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
	}
}

func (a *socialBlockCheckerAdapter) IsBlockedInEitherDirection(ctx context.Context, userA, userB uuid.UUID) (bool, error) {
	var blocked bool
	err := a.db.WithTx(ctx, func(tx db.Tx) error {
		var e error
		blocked, e = a.repo.ExistsBlock(ctx, tx, userA, userB)
		return e
	})
	return blocked, err
}

// =============================================================================
// NOTIFICATION BLOCK CHECKER ADAPTER
// =============================================================================

// notificationBlockCheckerAdapter satisfies worker.BlockChecker (and policy.BlockChecker)
// by wrapping the social repository's bidirectional ExistsBlock behind a self-managed
// transaction. The notification policy layer has no tx context; this adapter opens its own.
type notificationBlockCheckerAdapter struct {
	db   *db.DB
	repo interface {
		ExistsBlock(ctx context.Context, tx interface{}, userA, userB uuid.UUID) (bool, error)
	}
}

func (a *notificationBlockCheckerAdapter) ExistsBlock(ctx context.Context, userA, userB uuid.UUID) (bool, error) {
	var blocked bool
	err := a.db.WithTx(ctx, func(tx db.Tx) error {
		var e error
		blocked, e = a.repo.ExistsBlock(ctx, tx, userA, userB)
		return e
	})
	return blocked, err
}

// =============================================================================
// NOTIFICATION MUTE CHECKER ADAPTER
// =============================================================================

// notificationMuteCheckerAdapter satisfies policy.MuteChecker by wrapping the
// social repository's ExistsMute behind a self-managed transaction.
// The notification policy layer has no tx context; this adapter opens its own.
// Mirrors the N1 notificationBlockCheckerAdapter pattern exactly.
type notificationMuteCheckerAdapter struct {
	db   *db.DB
	repo interface {
		ExistsMute(ctx context.Context, tx interface{}, muterID, mutedID uuid.UUID) (bool, error)
	}
}

func (a *notificationMuteCheckerAdapter) ExistsMute(ctx context.Context, muterID, mutedID uuid.UUID) (bool, error) {
	var muted bool
	err := a.db.WithTx(ctx, func(tx db.Tx) error {
		var e error
		muted, e = a.repo.ExistsMute(ctx, tx, muterID, mutedID)
		return e
	})
	return muted, err
}

// s3PresignerAdapter wraps the s3presign package functions behind the
// verificationHTTP.S3Presigner interface so both verification handlers can
// generate short-lived presigned URLs without holding AWS credentials.
type s3PresignerAdapter struct {
	cfg s3presign.Config
}

func (a *s3PresignerAdapter) PresignPUT(key, contentType string, ttl time.Duration) (string, error) {
	return s3presign.PresignPUT(a.cfg, key, contentType, ttl)
}

func (a *s3PresignerAdapter) PresignGET(key string, ttl time.Duration) (string, error) {
	return s3presign.PresignGET(a.cfg, key, ttl)
}
