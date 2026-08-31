package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/middleware"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/internal/platform/response"
	"github.com/labuda/backend/internal/serverboot"
	"github.com/labuda/backend/internal/worker"
	"github.com/labuda/backend/pkg/database"
	"github.com/labuda/backend/pkg/firebase"
	pkgRedis "github.com/labuda/backend/pkg/redis"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// requireAnyCapability is a middleware that checks if the actor has ANY of the specified capabilities
// STEP 2: Helper for split config capabilities
func requireAnyCapability(caps ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := middleware.GetActorFromContext(c)
		if actor == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		for _, cap := range caps {
			if actor.HasCapability(cap) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "One of the following capabilities required: " + caps[0] + " or " + caps[1],
		})
	}
}

// SetupRoutes configures all application routes for CORE domains only
// CORE domains: finance, outbox, payment, user
func SetupRoutes(
	router *gin.Engine,
	cfg *config.Config,
	deps *serverboot.Dependencies,
	firebaseClient *firebase.Client,
	db *database.DB,
	redisClient *pkgRedis.Client,
	log *logger.Logger,
) {
	// Health check endpoints
	router.GET("/health", healthCheckHandler(cfg, db, redisClient))
	router.GET("/health/ready", readinessHandler(cfg, db, redisClient))
	router.GET("/health/live", livenessHandler())
	router.GET("/health/system", deps.SystemHealthHandler.GetSystemHealth)

	// Prometheus metrics endpoint (no auth - for monitoring)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Public OG metadata endpoints for shared links (unauthenticated).
	router.GET("/og/for-sale/:id", deps.OGHandler.GetForSale)
	router.GET("/og/auction/:id", deps.OGHandler.GetAuction)
	router.GET("/og/profile/:id", deps.OGHandler.GetProfile)
	router.GET("/og/content/:id", deps.OGHandler.GetContent)
	// Public share-path aliases for hosting proxy compatibility.
	// Keep mobile share URLs stable while still serving OG metadata from Go.
	router.GET("/for-sale/:id", deps.OGHandler.GetForSale)
	router.GET("/auction/:id", deps.OGHandler.GetAuction)
	router.GET("/profile/:id", deps.OGHandler.GetProfile)
	router.GET("/content/:id", deps.OGHandler.GetContent)

	// Payment webhook endpoint (no auth - called by Midtrans)
	// This must be mounted before the authenticated routes.
	//
	// webhookDropFilter is a dev-only gateway-loss simulator used by Phase
	// 1B corpus generation. It no-ops unless cfg.Server.Env=development AND
	// WEBHOOK_DROP_ENABLED=true — production traffic always passes through
	// untouched. See webhook_drop_middleware.go.
	webhookDrop := newWebhookDropFilter(cfg, log.Logger)
	webhookGroup := router.Group("/webhooks")
	{
		webhookGroup.POST("/payment/midtrans",
			webhookDrop.Middleware(),
			deps.PaymentWebhookHandler.HandleMidtransWebhook)
		// Payout webhook endpoint (no auth - called by payout gateway)
		// Signature verification is enforced for security
		webhookGroup.POST("/payout", deps.PayoutWebhookHandler.HandlePayoutWebhook)
		webhookGroup.GET("/payout/health", deps.PayoutWebhookHandler.HandleHealthCheck)
	}

	// Dev-only hot-arm endpoint for the webhook drop filter. Mounted ONLY
	// when filter.Armable() is true (i.e. env=development AND
	// WEBHOOK_DROP_ENABLED=true). In production the route does not exist.
	// See webhook_drop_admin.go for the request contract; the Phase 1B
	// scenario-d11 corpus driver calls this between settlement and refund
	// dispatch so the settlement webhook passes through normally and only
	// the later refund callback is suppressed.
	if webhookDrop.Armable() {
		router.POST("/dev/webhook-drop/arm", webhookDropArmHandler(webhookDrop))
		log.Warn("development-only routes mounted — do not use in production")
	}

	if cfg.IsDevelopment() {
		router.POST("/dev/webhooks/payment/midtrans/replay/:payment_id",
			deps.PaymentWebhookHandler.HandleMidtransWebhookDevReplay)
	}

	// ===== PHASE 1: CANONICAL AUTH ENTRY POINT (Public - Firebase token verified internally) =====
	// POST /api/v1/auth/firebase/exchange - canonical Firebase exchange route
	// This endpoint verifies Firebase ID token from request JSON and returns
	// either a restricted completion token or a full session.
	router.POST("/api/v1/auth/firebase/exchange", deps.AuthHandler.FirebaseExchange)
	router.POST("/api/v1/auth/refresh", deps.AuthHandler.RefreshToken)
	router.POST("/api/v1/auth/complete-profile", deps.AuthHandler.CompleteProfile)

	authRoutes := router.Group("/api/v1/auth")
	authRoutes.Use(middleware.AuthMiddleware(firebaseClient))
	authRoutes.Use(middleware.UserLookupMiddleware(middleware.NewDBUserLookupService(db.Pgx())))
	authRoutes.POST("/logout", deps.AuthHandler.Logout)
	authRoutes.POST("/logout-all", deps.AuthHandler.LogoutAll)
	authRoutes.GET("/sessions", deps.AuthHandler.ListSessions)
	authRoutes.DELETE("/sessions/:family_id", deps.AuthHandler.RevokeSession)

	// ===== PUBLIC BROWSE ROUTES (StrictBrowseAuthMiddleware) =====
	// These routes allow unauthenticated (anonymous) GET access for product discovery.
	// Rules:
	//   - No Authorization header → anonymous viewer (uuid.Nil) — allowed
	//   - Authorization header present but malformed/invalid → 401 (client must fix credentials)
	//   - Valid token → authenticated viewer with full context injected
	//
	// DO NOT move POST/PUT/DELETE routes here.
	// DO NOT move /api/v1/feed here (FeedHandler explicitly rejects anonymous).
	v1Browse := router.Group("/api/v1")
	v1Browse.Use(middleware.ErrorHandler(log.Logger))
	v1Browse.Use(middleware.StrictBrowseAuthMiddleware(firebaseClient))
	v1Browse.Use(middleware.UserLookupMiddleware(middleware.NewDBUserLookupService(db.Pgx())))
	v1Browse.Use(middleware.RolesLookupMiddleware(db.Pgx()))
	v1Browse.Use(middleware.ActorContextInject(deps.ActorResolver, middleware.ActorContextInjectOptions{Log: log.Logger}))
	{
		// ForSale browse (public discovery)
		v1Browse.GET("/for-sale", deps.ForSaleHandler.ListForSales)
		v1Browse.GET("/for-sale/:id", deps.ForSaleHandler.GetForSale)
		v1Browse.GET("/search/for-sale", deps.ForSaleHandler.SearchForSales)

		// Auction browse (public discovery)
		v1Browse.GET("/auctions", deps.AuctionHandler.ListAuctions)
		v1Browse.GET("/auctions/:id", deps.AuctionHandler.GetAuction)
		v1Browse.GET("/auctions/:id/bids", deps.AuctionHandler.ListBids)
		v1Browse.GET("/search/auctions", deps.SearchHandler.SearchAuctions)

		// Search browse (public discovery)
		v1Browse.GET("/search/content", deps.SearchHandler.SearchContent)
		v1Browse.GET("/search/users", deps.SearchHandler.SearchUsers)

		// User profile browse (public)
		v1Browse.GET("/users/:id", deps.UserProfileHandler.GetPublicUser)
		v1Browse.GET("/users/:id/contents", deps.ContentHandler.GetUserContent)

		// Content browse (public)
		v1Browse.GET("/contents/:id", deps.ContentHandler.GetContent)

		// Like stats (public, viewer-optional)
		v1Browse.GET("/likes/stats", deps.LikeHandler.GetLikeStats)
	}

	// ===== GLOBAL MIDDLEWARE CHAIN FOR /api/v1 =====
	// All authenticated routes under /api/v1 share the same middleware pipeline:
	// 1. AuthMiddleware - Validates Firebase tokens
	// 2. UserLookupMiddleware - Looks up user in database (errors if not provisioned)
	// 3. RolesLookupMiddleware - Fetches role from PostgreSQL for authorization
	// 4. ActorContextInject - Injects Actor (role + capabilities) into request context
	//
	// DATABASE-BASED AUTHORIZATION: Roles are queried from PostgreSQL on every request.
	// This allows immediate role revocation without requiring Firebase token refresh.
	//
	// USER PROVISIONING REMOVED: Middleware no longer creates users automatically.
	// Users must be created through explicit signup flow (POST /api/v1/auth/firebase/exchange).
	// If user not found in database, middleware returns "USER_NOT_PROVISIONED" error.

	v1 := router.Group("/api/v1")
	v1.Use(middleware.ErrorHandler(log.Logger))
	v1.Use(middleware.AuthMiddleware(firebaseClient))
	v1.Use(middleware.UserLookupMiddleware(middleware.NewDBUserLookupService(db.Pgx())))
	v1.Use(middleware.RolesLookupMiddleware(db.Pgx()))
	// SLICE 2+3: Capability-based auth - inject Actor with capabilities into context
	v1.Use(middleware.ActorContextInject(deps.ActorResolver, middleware.ActorContextInjectOptions{Log: log.Logger}))
	{
		// Ping endpoint for testing
		v1.GET("/ping", func(c *gin.Context) {
			response.Success(c, gin.H{
				"message": "pong from core server",
			})
		})

		// Payment domain routes (CORE)
		paymentRoutes := v1.Group("/payments")
		paymentRoutes.POST("", deps.PaymentHandler.CreatePayment)
		paymentRoutes.POST("/billing", deps.PaymentHandler.CreateBillingPayment)
		paymentRoutes.GET("/methods", deps.PaymentHandler.ListPaymentMethods)
		paymentRoutes.GET("/:id", deps.PaymentHandler.GetPayment)
		// NOTE: Midtrans webhook is at POST /webhooks/payment/midtrans (no auth required)

		// Coin domain routes (CORE - loyalty points)
		// Coins are loyalty points earned through activities, NOT purchased
		// Coins can only be: earned (rewards), spent (discount), expired
		coinRoutes := v1.Group("/coins")
		coinRoutes.GET("/balance", deps.CoinHandler.GetBalance)
		coinRoutes.GET("/transactions", deps.CoinHandler.GetTransactions)

		// User domain routes (CORE)
		// NOTE: GET /:id and GET /:id/contents moved to v1Browse (public browse group)
		userRoutes := v1.Group("/users")
		// Own profile endpoints - get/update authenticated user's profile
		userRoutes.GET("/check-username", deps.UserProfileHandler.CheckUsername)
		userRoutes.GET("/me", deps.UserProfileHandler.GetMyProfile)
		userRoutes.PATCH("/me/profile", deps.UserProfileHandler.UpdateMyProfile)
		userRoutes.POST("/me/verification/refresh", deps.UserProfileHandler.RefreshMyVerification)
		userRoutes.DELETE("/me", deps.UserProfileHandler.DeleteMyAccount)

		// General media upload — presigned S3 PUT URL for non-KYC files.
		// Requires auth; no seller gate. KYC uploads use a separate endpoint
		// in the seller verification group.
		v1.POST("/media/upload-url", deps.MediaUploadHandler.RequestUploadURL)

		// Pricing token routes (CORE)
		pricingRoutes := v1.Group("/pricing")
		pricingRoutes.POST("/preview", deps.PricingTokenHandler.GeneratePreview)
		pricingRoutes.POST("/validate", deps.PricingTokenHandler.ValidateToken)
		pricingRoutes.GET("/tokens/:token", deps.PricingTokenHandler.GetToken)

		// Order domain routes (CORE)
		orderRoutes := v1.Group("/orders")
		orderRoutes.GET("", deps.OrderHandler.ListMyOrders)
		orderRoutes.GET("/:id", deps.OrderHandler.GetOrder)
		orderRoutes.POST("", middleware.RequireActiveAccount(db.Pgx()), deps.OrderHandler.CreateOrder)

		// Order action routes - buyer actions for protection window
		// B4A: "Terima Barang" = /complete. mark-delivered route removed from buyer flow.
		// Auth: active account required — suspended/banned/deleted users must not mutate order state.
		orderRoutes.POST("/:id/extend-confirmation", middleware.RequireActiveAccount(db.Pgx()), deps.OrderHandler.ExtendConfirmation)
		orderRoutes.POST("/:id/complete", middleware.RequireActiveAccount(db.Pgx()), deps.OrderHandler.CompleteOrder)

		// Order action routes - seller actions
		orderRoutes.POST("/:id/ship", middleware.RequireActiveAccount(db.Pgx()), deps.OrderHandler.MarkShipped)

		// Order action routes - buyer protection
		orderRoutes.POST("/:id/cancel", middleware.RequireActiveAccount(db.Pgx()), deps.OrderHandler.CancelOrder)
		orderRoutes.POST("/:id/dispute", middleware.RequireActiveAccount(db.Pgx()), deps.OrderHandler.CreateDispute)
		orderRoutes.POST("/:id/refund", middleware.RequireActiveAccount(db.Pgx()), deps.OrderHandler.CreateRefund)

		// Refund seller decision routes (H2-A)
		// Seller approves or rejects a buyer's refund request.
		// Auth: authenticated user + active account; ownership enforced in service layer.
		refundRoutes := v1.Group("/refunds")
		refundRoutes.POST("/:id/approve", middleware.RequireActiveAccount(db.Pgx()), deps.SellerRefundHandler.ApproveRefund)
		refundRoutes.POST("/:id/reject", middleware.RequireActiveAccount(db.Pgx()), deps.SellerRefundHandler.RejectRefund)
		refundRoutes.POST("/:id/escalate", middleware.RequireActiveAccount(db.Pgx()), deps.BuyerEscalationHandler.EscalateRefund)

		// Auction domain routes (CORE)
		// NOTE: GET "", GET /:id, GET /:id/bids moved to v1Browse (public browse group)
		auctionRoutes := v1.Group("/auctions")

		// Seller-only endpoints
		// RequireActiveAccount runs FIRST (email verification gate, matching for_sale seller routes).
		// RequireSellerMiddleware runs SECOND (4-gate seller authority).
		auctionSellerRoutes := auctionRoutes.Group("")
		auctionSellerRoutes.Use(middleware.RequireActiveAccount(db.Pgx()))
		auctionSellerRoutes.Use(middleware.RequireSellerMiddleware(deps.RoleChecker))
		{
			auctionSellerRoutes.POST("", deps.AuctionHandler.CreateAuction)
			auctionSellerRoutes.PUT("/:id", deps.AuctionHandler.UpdateAuction)
			auctionSellerRoutes.POST("/:id/schedule", deps.AuctionHandler.ScheduleAuction)
			auctionSellerRoutes.POST("/:id/cancel", deps.AuctionHandler.CancelAuction)
		}

		// Authenticated buyer endpoints
		auctionRoutes.POST("/:id/bid", middleware.RequireActiveAccount(db.Pgx()), deps.AuctionHandler.PlaceBid)
		auctionRoutes.POST("/:id/claim-token", middleware.RequireActiveAccount(db.Pgx()), deps.AuctionHandler.GeneratePricingTokenForClaim)
		auctionRoutes.POST("/:id/claim", middleware.RequireActiveAccount(db.Pgx()), deps.AuctionHandler.ClaimAuction)

		// Saved Items endpoints (unified shortlist + auction watch)
		savedItemsRoutes := v1.Group("/saved-items")
		{
			// Get user's saved items (for_sale + auctions)
			savedItemsRoutes.GET("", deps.SavedItemHandler.GetSavedItems)

			// Add item to saved items
			savedItemsRoutes.POST("", deps.SavedItemHandler.AddSavedItem)

			// Remove item from saved items
			savedItemsRoutes.DELETE("/:id", deps.SavedItemHandler.RemoveSavedItem)

			// Clear all saved items (or by type with ?type=for_sale|auction)
			savedItemsRoutes.DELETE("", deps.SavedItemHandler.ClearSavedItems)

			// Get saved items count
			savedItemsRoutes.GET("/count", deps.SavedItemHandler.GetSavedItemsCount)

			// Check if item is saved
			savedItemsRoutes.GET("/check", deps.SavedItemHandler.IsSaved)
		}

		// Bidding endpoints (authenticated users)
		// Returns all auctions where the user has placed bids
		v1.GET("/bidding", deps.BiddingHandler.GetMyBidding)

		// ForSale domain routes (CORE)
		// NOTE: GET "", GET /:id, and GET /search/for-sale moved to v1Browse (public browse group)
		// Seller-only endpoints (for_sale CRUD operations)
		forSaleSellerRoutes := v1.Group("/for-sale")
		{
			// ForSale create gate: verified email + active account + seller authority.
			// Suspended/banned sellers cannot publish new for_sale items.
			forSaleSellerRoutes.POST("",
				middleware.RequireActiveAccount(db.Pgx()),
				middleware.RequireSellerMiddleware(deps.RoleChecker),
				deps.ForSaleHandler.CreateForSale,
			)
			// ForSale owner mutations: verified email + active account + seller authority.
			// Matches the create gate; suspended/banned sellers cannot edit,
			// delete, or reconfigure shipping on their own for_sale items.
			forSaleSellerRoutes.PUT("/:id",
				middleware.RequireActiveAccount(db.Pgx()),
				middleware.RequireSellerMiddleware(deps.RoleChecker),
				deps.ForSaleHandler.UpdateForSale,
			)
			forSaleSellerRoutes.DELETE("/:id",
				middleware.RequireActiveAccount(db.Pgx()),
				middleware.RequireSellerMiddleware(deps.RoleChecker),
				deps.ForSaleHandler.DeleteForSale,
			)
		}

		productSellerRoutes := v1.Group("/products")
		{
			productSellerRoutes.PUT("/:id/shipping",
				middleware.RequireActiveAccount(db.Pgx()),
				middleware.RequireSellerMiddleware(deps.RoleChecker),
				deps.ProductShippingHandler.SetProductShippingOptions,
			)
		}

		// ===== SHIPPING DOMAIN (CORE) =====
		// Shipping delivery options for buyers and sellers
		//
		// DESIGN PRINCIPLES:
		// - Buyer routes: Check availability for for_sale items
		// - Seller routes: Manage shipping options (TODO: not yet implemented)
		//
		// CURRENT IMPLEMENTATION:
		// - GET /api/v1/shipping/options - Check delivery availability (buyer-facing)
		// - PUT /api/v1/products/:id/shipping - Set shipping options for product (seller-facing)

		// Buyer-facing shipping routes (authenticated users)
		shippingRoutes := v1.Group("/shipping")
		{
			// Check delivery availability for a product
			// Query params: product_id (required), province_code (required), city_code (optional)
			// Returns: Available delivery options with rates
			shippingRoutes.GET("/options", deps.ShippingHandler.GetDeliveryOptions)

			// Flutter-compatible endpoint (POST with JSON body)
			// Request body: product_id (required), province_code (required), city_code (optional)
			// Returns: Available delivery options with rates
			shippingRoutes.POST("/check", deps.ShippingHandler.CheckDelivery)
		}

		// ===== FEDERATED SEARCH CONTRACT REALIGN PACK V1 =====
		// Search domain routes (CORE)
		// Federated search across content, users, for_sale items, auctions
		//
		// FEDERATED SEARCH STRATEGY:
		// - Each domain has its own search endpoint
		// - Search domain provides unified handlers that query domain-specific repositories
		// - No unified search backend abstraction - queries are federated
		//
		// SEARCH ENDPOINTS:
		// - GET /api/v1/search/content - Search content/posts
		// - GET /api/v1/search/users - Search users/profiles
		// - GET /api/v1/search/auctions - Search auctions (PHASE 3.5)
		// - GET /api/v1/search/for-sale - Already handled by ForSaleHandler
		// - GET /api/v1/search/history - Get search history
		// - POST /api/v1/search/history - Save search history
		// - DELETE /api/v1/search/history - Clear search history
		// - DELETE /api/v1/search/history/:id - Delete specific history item
		// NOTE: GET /search/content, /search/users, /search/auctions moved to v1Browse (public browse group)
		// Search history management (authenticated — requires user context)
		searchRoutes := v1.Group("/search")
		{
			searchRoutes.GET("/history", deps.SearchHandler.GetSearchHistory)
			searchRoutes.POST("/history", deps.SearchHandler.AddSearchHistory)
			searchRoutes.DELETE("/history", deps.SearchHandler.ClearSearchHistory)      // Clear all
			searchRoutes.DELETE("/history/:id", deps.SearchHandler.DeleteSearchHistory) // Delete specific
		}

		// Chat domain routes (CORE)
		// All authenticated users can access chat
		chatRoutes := v1.Group("/chat")
		{
			// List all rooms for the authenticated user
			chatRoutes.GET("/rooms", deps.ChatHandler.ListRooms)

			// Get or create a direct chat room with another user
			chatRoutes.POST("/direct/:user_id", middleware.RequireActiveAccount(db.Pgx()), deps.ChatHandler.GetOrCreateDirectRoom)

			// Link order to chat (order↔chat commerce continuity)
			// Mutation: requires active account + email verification (PASS_6A / F2).
			chatRoutes.PUT("/rooms/:room_id/link-order", middleware.RequireActiveAccount(db.Pgx()), deps.ChatHandler.LinkOrderToChat)

			// Create order from chat (CHAT-CENTRIC COMMERCE ENTRY POINT)
			// Creates an order from an accepted negotiation in the chat room
			chatRoutes.POST("/rooms/:room_id/order", middleware.RequireActiveAccount(db.Pgx()), deps.ChatHandler.CreateOrderFromChat)

			// Get room by order ID for dispute resolution
			chatRoutes.GET("/rooms/by-order/:order_id", deps.ChatHandler.GetRoomByOrderID)

			// List messages in a room
			chatRoutes.GET("/rooms/:room_id/messages", deps.ChatHandler.ListMessages)

			// Send a message to a room
			chatRoutes.POST("/rooms/:room_id/messages", middleware.RequireActiveAccount(db.Pgx()), deps.ChatHandler.SendMessage)

			// Mark messages as read in a room
			// Mutation: requires active account + email verification (PASS_6A / F2).
			chatRoutes.POST("/rooms/:room_id/read", middleware.RequireActiveAccount(db.Pgx()), deps.ChatHandler.MarkAsRead)

			// Get unread message count for a room
			chatRoutes.GET("/rooms/:room_id/unread", deps.ChatHandler.GetUnreadCount)

			// ===== NEGOTIATION ROUTES (Chat-Owned) =====
			// Price negotiation endpoints scoped to chat rooms.
			// All business logic delegated to NegotiationService; chat handler provides room membership gating.
			chatRoutes.POST("/rooms/:room_id/negotiate", middleware.RequireActiveAccount(db.Pgx()), deps.ChatHandler.StartNegotiation)
			chatRoutes.POST("/rooms/:room_id/counter", middleware.RequireActiveAccount(db.Pgx()), deps.ChatHandler.SendCounterOffer)
			chatRoutes.POST("/rooms/:room_id/respond", middleware.RequireActiveAccount(db.Pgx()), deps.ChatHandler.RespondToNegotiation)
			chatRoutes.GET("/rooms/:room_id/negotiation", deps.ChatHandler.GetNegotiation)

			// ===== SHIPPING QUOTE ROUTES =====
			// Shipping quote endpoints for manual shipping quotes via chat
			// Used as fallback when for_sale lacks shipping coverage
			chatRoutes.POST("/:chat_id/shipping-quote", middleware.RequireActiveAccount(db.Pgx()), deps.ShippingQuoteHandler.CreateShippingQuote)
		}

		// Shipping quote direct routes (by ID)
		v1.GET("/shipping-quote/:quote_id", middleware.RequireActiveAccount(db.Pgx()), deps.ShippingQuoteHandler.GetShippingQuoteByID)

		// Seller domain routes (CORE)
		// Seller onboarding route (authenticated users who want to become sellers).
		// Gate: verified email + active account. Suspended/banned users cannot
		// onboard; unverified users still cannot onboard either.
		v1.POST("/seller/onboarding", middleware.RequireActiveAccount(db.Pgx()), deps.SellerHandler.Onboarding)

		// Subscription payment initiation (pre-seller: user has onboarded but
		// does not yet have seller authority). Gate: active account only.
		v1.POST("/seller/subscription/initiate", middleware.RequireActiveAccount(db.Pgx()), deps.SellerHandler.InitiateSubscriptionPayment)

		// Subscription payment sync — polls Midtrans for the user's own latest
		// pending subscription payment and activates the subscription when the
		// gateway confirms success. Recovers from webhook delivery failures
		// (e.g. Cloudflare tunnel was down). Gate: active account only.
		v1.POST("/seller/subscription/sync", middleware.RequireActiveAccount(db.Pgx()), deps.SellerHandler.SyncSubscriptionPayment)

		// Subscription config disclosure for the seller upgrade/onboarding flow.
		// This is authenticated-account scoped, not seller-authority scoped.
		v1.GET("/seller/subscription/config", middleware.RequireActiveAccount(db.Pgx()), deps.SellerHandler.GetSubscriptionConfig)

		// ===== SELLER MARKET AUTHORITY ROUTES =====
		// Require active subscription (RequireSellerMiddleware).
		// Expired sellers are rejected — these routes create new market obligations.
		//
		// SELLER_WORKSPACE_DOCTRINE: workspace/payout-prep routes are in a
		// separate group below that uses RequireSellerProfileMiddleware instead.
		sellerRoutes := v1.Group("/seller")
		sellerRoutes.Use(middleware.RequireActiveAccount(db.Pgx()))
		sellerRoutes.Use(middleware.RequireSellerMiddleware(deps.RoleChecker))
		{
			// Seller profile and subscription status (own metadata — market-scoped)
			sellerRoutes.GET("/profile", deps.SellerHandler.GetProfile)
			sellerRoutes.GET("/subscription", deps.SellerHandler.GetSubscription)

			// Seller dashboard / analytics / performance (market operation surfaces)
			sellerRoutes.GET("/dashboard", deps.SellerHandler.GetDashboard)
			sellerRoutes.GET("/analytics", deps.SellerHandler.GetAnalytics)
			sellerRoutes.GET("/performance", deps.SellerHandler.GetPerformance)

			// ===== SELLER SHIPPING MANAGEMENT (market configuration) =====
			// Shipping option management for sellers
			//
			// DESIGN PRINCIPLES:
			// - Sellers create and manage their shipping options
			// - Shipping options are linked to products for buyer visibility
			// - Coverage defines which provinces are served and at what rate
			//
			// SHIPPING OPTION CRUD:
			// - POST /api/v1/seller/shipping/options - Create shipping option
			// - GET /api/v1/seller/shipping/options - List shipping options
			// - GET /api/v1/seller/shipping/options/:id - Get shipping option with coverages
			// - PUT /api/v1/seller/shipping/options/:id - Update shipping option
			// - DELETE /api/v1/seller/shipping/options/:id - Delete shipping option
			//
			// COVERAGE CRUD:
			// - POST /api/v1/seller/shipping/options/:id/coverages - Create coverage
			// - GET /api/v1/seller/shipping/options/:id/coverages - List coverages
			// - PUT /api/v1/seller/shipping/coverages/:id - Update coverage
			// - DELETE /api/v1/seller/shipping/coverages/:id - Delete coverage
			sellerRoutes.POST("/shipping/options", deps.SellerShippingHandler.CreateShippingOption)
			sellerRoutes.GET("/shipping/options", deps.SellerShippingHandler.ListShippingOptions)
			sellerRoutes.GET("/shipping/options/:id", deps.SellerShippingHandler.GetShippingOption)
			sellerRoutes.PUT("/shipping/options/:id", deps.SellerShippingHandler.UpdateShippingOption)
			sellerRoutes.DELETE("/shipping/options/:id", deps.SellerShippingHandler.DeleteShippingOption)

			// Coverage management routes
			sellerRoutes.POST("/shipping/options/:id/coverages", deps.SellerShippingHandler.CreateCoverage)
			sellerRoutes.GET("/shipping/options/:id/coverages", deps.SellerShippingHandler.ListCoverages)
			sellerRoutes.PUT("/shipping/coverages/:id", deps.SellerShippingHandler.UpdateCoverage)
			sellerRoutes.DELETE("/shipping/coverages/:id", deps.SellerShippingHandler.DeleteCoverage)

		}

		// ===== SELLER WORKSPACE / PAYOUT-PREP ROUTES =====
		// SELLER_WORKSPACE_DOCTRINE: these routes require seller profile existence only.
		// Expired subscription does NOT close access to earnings visibility, bank
		// account management for payout, or KYC verification status/submission.
		//
		// Gate: RequireSellerProfileMiddleware (Gates 1+2 — account active + profile exists)
		// NOT gated by subscription status. Service-layer payout/trust guards remain.
		sellerWorkspaceRoutes := v1.Group("/seller")
		sellerWorkspaceRoutes.Use(middleware.RequireActiveAccount(db.Pgx()))
		sellerWorkspaceRoutes.Use(middleware.RequireSellerProfileMiddleware(deps.RoleChecker))
		{
			// Earnings / balance visibility — survives subscription expiry.
			// Expired sellers earned balance is theirs; they must be able to see it.
			sellerWorkspaceRoutes.GET("/earnings", deps.SellerHandler.GetEarnings)

			// ===== SELLER VERIFICATION (Phase 2) =====
			// Verification opens payout authority, not selling authority.
			// Expired sellers must be able to check status and resubmit KYC
			// to restore payout authority without renewing subscription.
			//
			// Step 1: Request a presigned S3 PUT URL for a KYC document.
			// Step 2: PUT the file bytes to upload_url with matching Content-Type.
			// Step 3: Submit KYC with the storage_key values returned by Step 1.
			sellerWorkspaceRoutes.POST("/verification/documents/upload-url", deps.VerificationHandler.RequestUploadURL)
			sellerWorkspaceRoutes.POST("/verification/submit", deps.VerificationHandler.SubmitKYC)
			sellerWorkspaceRoutes.GET("/verification/status", deps.VerificationHandler.GetStatus)
		}

		// Bank account domain routes (CORE)
		// PAYOUT-PREP DOCTRINE: bank account management is a payout-prep surface.
		// Expired sellers must be able to manage bank accounts to receive payouts
		// for past sales. Gate: RequireSellerProfileMiddleware (profile only).
		// Service-layer guards handle withdrawal eligibility separately.
		// Actor ID is always sourced from auth context, never from request body.
		bankAccountRoutes := v1.Group("/bank-accounts")
		bankAccountRoutes.Use(middleware.RequireActiveAccount(db.Pgx()))
		bankAccountRoutes.Use(middleware.RequireSellerProfileMiddleware(deps.RoleChecker))
		{
			bankAccountRoutes.POST("", deps.BankAccountHandler.CreateBankAccount)
			bankAccountRoutes.GET("", deps.BankAccountHandler.ListBankAccounts)
			bankAccountRoutes.GET("/:id", deps.BankAccountHandler.GetBankAccount)
			bankAccountRoutes.PATCH("/:id/default", deps.BankAccountHandler.SetDefaultBankAccount)
			bankAccountRoutes.DELETE("/:id", deps.BankAccountHandler.DeleteBankAccount)
		}

		// Address domain routes (CORE)
		// Authenticated user CRUD for shipping/sender addresses.
		// Active account gate applies to the whole address book so suspended/banned
		// users cannot mutate structured addresses.
		addressRoutes := v1.Group("/addresses")
		addressRoutes.Use(middleware.RequireActiveAccount(db.Pgx()))
		{
			addressRoutes.POST("", deps.AddressHandler.CreateAddress)
			addressRoutes.GET("", deps.AddressHandler.ListAddresses)
			addressRoutes.GET("/primary", deps.AddressHandler.GetPrimary)
			addressRoutes.GET("/count", deps.AddressHandler.GetCount)
			addressRoutes.GET("/:id", deps.AddressHandler.GetAddress)
			addressRoutes.PUT("/:id", deps.AddressHandler.UpdateAddress)
			addressRoutes.DELETE("/:id", deps.AddressHandler.DeleteAddress)
			addressRoutes.POST("/:id/primary", deps.AddressHandler.SetPrimary)
		}

		// Withdraw domain routes (CORE)
		// PAYOUT AUTHORITY DOCTRINE: Payout authority is independent of selling
		// authority. RequireSellerMiddleware (subscription gate) is intentionally
		// NOT applied here. A seller with an expired subscription still owns their
		// earned balance and may withdraw it. The service-layer GUARD 0–5 enforce
		// the correct payout gates: account active, verification approved, dispute-
		// aware balance ceiling, no in-flight withdrawal, reviewed bank account.
		// See: CORE_SELLABILITY_AUTHORITY_RUNTIME_AUDIT P1 finding.
		withdrawRoutes := v1.Group("/withdraw")
		{
			// Request a withdrawal — PHASE 2D / TASK 43 canonical path.
			// Authority: finance.SELLER_PAYABLE − Σ(active dispute freeze).
			// Books DR SELLER_PAYABLE / CR WITHDRAWAL_PENDING in the same
			// tx as the wallet.withdrawals row insert.
			withdrawRoutes.POST("",
				middleware.RequireActiveAccount(db.Pgx()),
				deps.WithdrawalHandlerUnified.RequestWithdraw,
			)

			// Withdrawal history — read-only finance visibility.
			// Historical finance records survive subscription expiry;
			// sellerID is sourced from auth context (no cross-seller leak).
			withdrawRoutes.GET("/history",
				middleware.RequireActiveAccount(db.Pgx()),
				deps.WithdrawalHandlerUnified.ListWithdrawals,
			)
		}

		// Shortlist domain routes (CORE)
		// Shortlist is for interest parking (save for later), NOT a shopping cart

		// Discount domain routes (CORE)
		discountRoutes := v1.Group("/discounts")
		{
			// Public endpoints - authenticated users
			discountRoutes.GET("/code/:code", deps.DiscountHandler.GetDiscountByCode)
			discountRoutes.POST("/validate", deps.DiscountHandler.ValidateDiscount)
			discountRoutes.GET("/active", deps.DiscountHandler.ListActiveDiscounts)
			discountRoutes.GET("/seller/:sellerId", deps.DiscountHandler.GetDiscountsBySeller)

			// Seller-only endpoints (create/update own discounts)
			discountSellerRoutes := v1.Group("/discounts")
			discountSellerRoutes.Use(middleware.RequireActiveAccount(db.Pgx()))
			discountSellerRoutes.Use(middleware.RequireSellerMiddleware(deps.RoleChecker))
			discountSellerRoutes.POST("", deps.DiscountHandler.CreateDiscount)
			discountSellerRoutes.PUT("/:id", deps.DiscountHandler.UpdateDiscount)
			discountSellerRoutes.DELETE("/:id", deps.DiscountHandler.DeactivateDiscount)
		}

		// Realtime WebSocket endpoint (CORE)
		// WebSocket endpoint for realtime chat message delivery
		// Uses the same auth middleware as other API routes
		// Client sends: {"action": "subscribe", "room_id": "..."}
		// Server broadcasts: {"event": "chat.message.sent", "room_id": "...", "message_id": "..."}
		v1.GET("/ws", deps.RealtimeHandler.HandleWebSocket)

		// Realtime stats endpoint (for monitoring)
		v1.GET("/ws/stats", deps.RealtimeHandler.GetStats)

		// Admin routes - requires active account + admin role
		adminRoutes := v1.Group("/admin")
		adminRoutes.Use(middleware.RequireActiveAccount(db.Pgx()))
		adminRoutes.Use(middleware.RequireAdminMiddleware(deps.RoleChecker))
		{
			// Set user role (SLICE 5: MIGRATED to capability-based auth)
			// DUAL PROTECTION: RequireAdminMiddleware (existing) + RequireCapability (new)
			adminRoutes.PUT("/users/:id/role",
				middleware.RequireCapability("governance.role.assign"),
				deps.UserHandler.SetRole)

			// Current admin identity (used by admin web on startup)
			adminRoutes.GET("/me", deps.AdminHandler.GetAdminMe)

			// ===== AUCTION ADMIN EMERGENCY CANCEL/OVERRIDE (PASS_5B) =====
			// Governance authority, not seller authority — cancels ANY seller's
			// auction (unreachable/abusive seller, trust-and-safety stop).
			// Never mutates money/escrow/order/refund state; see
			// AuctionService.AdminCancel for the safe/conflict state contract.
			adminRoutes.POST("/auctions/:id/cancel",
				middleware.RequireCapability("governance.auction.cancel"),
				deps.AdminAuctionHandler.CancelAuction)

			// Test endpoint for admin verification
			adminRoutes.GET("/test", func(c *gin.Context) {
				response.Success(c, gin.H{
					"message": "Admin access verified - database-based authorization working!",
				})
			})

			// Dispute resolution routes - admin only
			// RBAC IMPLEMENTATION: Added capability checks for all dispute operations
			adminRoutes.POST("/disputes/:id/approve",
				middleware.RequireCapability("finance.dispute.resolve"),
				deps.DisputeHandler.ResolveDisputeApprove)
			adminRoutes.POST("/disputes/:id/reject",
				middleware.RequireCapability("finance.dispute.resolve"),
				deps.DisputeHandler.ResolveDisputeReject)
			adminRoutes.POST("/disputes/:id/partial-split",
				middleware.RequireCapability("finance.dispute.resolve"),
				deps.DisputeHandler.ResolveDisputePartialSplit)

			// Dispute query routes - admin only (read-only)
			// RBAC IMPLEMENTATION: Using finance.withdraw.read for viewing disputes (financial context)
			adminRoutes.GET("/disputes",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.DisputeHandler.ListDisputes) // List all disputes
			adminRoutes.GET("/disputes/:id",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.DisputeHandler.GetDisputeDetail) // Get dispute details

			// Order management routes - admin only (read-only)
			// RBAC IMPLEMENTATION: Added order.read capability
			adminRoutes.GET("/orders",
				middleware.RequireCapability("order.read"),
				deps.AdminOrderHandler.ListOrders) // List all orders
			adminRoutes.GET("/orders/:id",
				middleware.RequireCapability("order.read"),
				deps.AdminOrderHandler.GetOrderDetail) // Get order details
			adminRoutes.GET("/orders/:id/timeline",
				middleware.RequireCapability("order.read"),
				deps.AdminOrderHandler.GetOrderTimeline) // Get order timeline

			// TASK 34 / Phase 2a: gateway refund trigger (admin-only,
			// feature-flagged via ENABLE_GATEWAY_REFUND_PHASE2). The
			// handler returns 503 FEATURE_DISABLED when the flag is off.
			// Capability gate: finance.refund.gateway.initiate.
			adminRoutes.POST("/refunds/:refund_id/gateway/initiate",
				middleware.RequireCapability("finance.refund.gateway.initiate"),
				deps.AdminRefundHandler.InitiateGatewayRefund)

			// ===== SELLER VERIFICATION REVIEW (Phase 2) =====
			// Review queue + transition handlers. Capability gate:
			// seller.verification.review. Reject and request-resubmission
			// require a reason in the body (mandatory per doctrine).
			adminRoutes.GET("/seller-verifications/pending",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.ListPending)
			adminRoutes.GET("/seller-verifications/:seller_id",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.GetDetail)
			adminRoutes.POST("/seller-verifications/:seller_id/approve",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.Approve)
			adminRoutes.POST("/seller-verifications/:seller_id/reject",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.Reject)
			adminRoutes.POST("/seller-verifications/:seller_id/request-resubmission",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.RequestResubmission)
			adminRoutes.POST("/seller-verifications/:seller_id/suspend",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.Suspend)
			adminRoutes.POST("/seller-verifications/:seller_id/revoke",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.Revoke)
			adminRoutes.POST("/seller-verifications/:seller_id/investigate",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.Investigate)
			adminRoutes.POST("/seller-verifications/:seller_id/restore",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.Restore)
			// Mark a specific bank account as reviewed for payout without full re-KYC.
			// Use when seller adds a post-approval account that admin has manually verified.
			// Idempotent; requires seller to be in approved status.
			adminRoutes.POST("/seller-verifications/:seller_id/bank-accounts/:bank_account_id/mark-reviewed",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.MarkBankAccountReviewed)
			// Short-lived presigned GET URL for a KYC document (5 min TTL).
			// Useful when the URL embedded in GetDetail has expired.
			adminRoutes.GET("/seller-verifications/:seller_id/documents/:document_id/view-url",
				middleware.RequireCapability("seller.verification.review"),
				deps.AdminVerificationHandler.GetDocumentViewURL)

			// ===== MODERATION ADMIN ROUTES — REMOVED (SLICE 2) =====
			// The legacy admin Case review endpoints (ListCases/GetCase/
			// GetCaseEvidence/ApplyAction) were backed by the rejected
			// GovernanceCase runtime reading the dropped moderation_cases table.
			// They are removed with that runtime. The canonical Case/Decision/
			// Enforcement admin workflow is rebuilt in a later slice.

			// ===== PROMOTION EXTERNAL PRODUCT REVIEW =====
			// Admin review queue and moderation actions for external products.
			adminRoutes.GET("/external-products",
				middleware.RequireCapability("promotion.external_product.review"),
				deps.PromotionHandler.ListAdminExternalProducts)
			adminRoutes.GET("/external-products/:id",
				middleware.RequireCapability("promotion.external_product.review"),
				deps.PromotionHandler.GetAdminExternalProduct)
			adminRoutes.GET("/external-products/:id/reviews",
				middleware.RequireCapability("promotion.external_product.review"),
				deps.PromotionHandler.ListAdminExternalProductReviews)
			adminRoutes.POST("/external-products/:id/approve",
				middleware.RequireCapability("promotion.external_product.review"),
				deps.PromotionHandler.ApproveExternalProduct)
			adminRoutes.POST("/external-products/:id/reject",
				middleware.RequireCapability("promotion.external_product.review"),
				deps.PromotionHandler.RejectExternalProduct)
			adminRoutes.POST("/external-products/:id/request-changes",
				middleware.RequireCapability("promotion.external_product.review"),
				deps.PromotionHandler.RequestChangesExternalProduct)
			adminRoutes.POST("/external-products/:id/hide",
				middleware.RequireCapability("promotion.external_product.review"),
				deps.PromotionHandler.HideExternalProduct)

			// ===== PROMOTION PACKAGES ADMIN =====
			// CRUD + enable/disable for promotion packages.
			// Capability: promotion.package.manage
			adminRoutes.GET("/promotions/packages",
				middleware.RequireCapability("promotion.package.manage"),
				deps.PromotionHandler.AdminListPackages)
			adminRoutes.POST("/promotions/packages",
				middleware.RequireCapability("promotion.package.manage"),
				deps.PromotionHandler.AdminCreatePackage)
			adminRoutes.PATCH("/promotions/packages/:id",
				middleware.RequireCapability("promotion.package.manage"),
				deps.PromotionHandler.AdminUpdatePackage)
			adminRoutes.POST("/promotions/packages/:id/enable",
				middleware.RequireCapability("promotion.package.manage"),
				deps.PromotionHandler.AdminEnablePackage)
			adminRoutes.POST("/promotions/packages/:id/disable",
				middleware.RequireCapability("promotion.package.manage"),
				deps.PromotionHandler.AdminDisablePackage)

			// ===== PROMOTION CAMPAIGNS ADMIN =====
			// Campaign visibility and force-stop.
			adminRoutes.GET("/promotions/campaigns",
				middleware.RequireCapability("promotion.campaign.view"),
				deps.PromotionHandler.AdminListCampaigns)
			adminRoutes.GET("/promotions/campaigns/:id/analytics",
				middleware.RequireCapability("promotion.campaign.view"),
				deps.PromotionHandler.AdminGetCampaignAnalytics)
			adminRoutes.POST("/promotions/campaigns/:id/stop",
				middleware.RequireCapability("promotion.campaign.stop"),
				deps.PromotionHandler.AdminForceStopCampaign)

			// ========================================================================
			// ADMIN / OPERABILITY HARDENING PACK V1
			// ========================================================================

			// ===== ADMIN DASHBOARD & USER MANAGEMENT =====
			// AdminHandler routes - complete admin operability
			// RBAC IMPLEMENTATION: Added governance.dashboard.view and governance.user.read
			adminRoutes.GET("/dashboard",
				middleware.RequireCapability("governance.dashboard.view"),
				deps.AdminHandler.GetDashboard) // Platform metrics
			adminRoutes.GET("/users",
				middleware.RequireCapability("governance.user.read"),
				deps.AdminHandler.ListUsers) // User listing with filters
			adminRoutes.GET("/users/:id",
				middleware.RequireCapability("governance.user.read"),
				deps.AdminHandler.GetUserDetails) // User details
			adminRoutes.POST("/users/:id/suspend",
				middleware.RequireCapability("governance.user.suspend"),
				deps.AdminHandler.SuspendUser) // Suspend user
			adminRoutes.POST("/users/:id/activate",
				middleware.RequireCapability("governance.user.activate"),
				deps.AdminHandler.ActivateUser) // Activate suspended user (cannot revive banned)
			adminRoutes.POST("/users/:id/ban",
				middleware.RequireCapability("governance.user.ban"),
				deps.AdminHandler.BanUser) // Ban user
			adminRoutes.POST("/users/:id/unban",
				middleware.RequireCapability("governance.user.unban"),
				deps.AdminHandler.UnbanUser) // Unban user (explicit ban reversal)
			adminRoutes.POST("/users/:id/bnr-strikes/reset",
				middleware.RequireCapability("governance.bnr.reset"),
				deps.AdminHandler.ResetBNRStrikesForUser) // Reset all active BNR strikes for buyer
			adminRoutes.POST("/bnr-strikes/:strike_id/reset",
				middleware.RequireCapability("governance.bnr.reset"),
				deps.AdminHandler.ResetBNRStrike) // Reset single BNR strike
			adminRoutes.GET("/audit-logs",
				middleware.RequireCapability("governance.audit.read"),
				deps.AdminHandler.GetAuditLogs) // Audit logs

			// ===== CAPABILITY MANAGEMENT (Admin) =====
			// CapabilityHandler routes - manage user capabilities
			// RBAC IMPLEMENTATION: All endpoints require governance.capability.assign
			adminRoutes.GET("/capabilities",
				middleware.RequireCapability("governance.capability.assign"),
				deps.CapabilityHandler.ListCapabilities) // List all available capabilities
			adminRoutes.GET("/users/:id/capabilities",
				middleware.RequireCapability("governance.capability.assign"),
				deps.CapabilityHandler.GetUserCapabilities) // Get user's capabilities
			adminRoutes.POST("/users/:id/capabilities",
				middleware.RequireCapability("governance.capability.assign"),
				deps.CapabilityHandler.AssignCapability) // Assign capability to user
			adminRoutes.DELETE("/users/:id/capabilities/:cap",
				middleware.RequireCapability("governance.capability.assign"),
				deps.CapabilityHandler.RevokeCapability) // Revoke capability from user

			adminRoutes.GET("/sla/metrics",
				middleware.RequireCapability("governance.dashboard.view"),
				deps.AdminHandler.GetSLAMetrics) // SLA metrics for support and disputes

			// ===== O4: NOTIFICATION DELIVERY MONITORING =====
			adminRoutes.GET("/notifications/failed-deliveries",
				middleware.RequireCapability("governance.dashboard.view"),
				deps.AdminHandler.GetFailedDeliveries) // Failed notification delivery log

			// ===== APPEALS SYSTEM (Admin) =====
			// AppealHandler admin routes - moderation appeal review
			// PASS_13B: appeal content is escalation/governance content and requires
			// its own trust boundary — moderation.case.read (generic case reading)
			// no longer implies appeal read access. See moderation.appeal.read.
			adminRoutes.GET("/appeals",
				middleware.RequireCapability("moderation.appeal.read"),
				deps.AppealHandler.AdminListAppeals) // List all appeals
			adminRoutes.GET("/appeals/pending",
				middleware.RequireCapability("moderation.appeal.read"),
				deps.AppealHandler.AdminListPendingAppeals) // Pending appeals queue
			adminRoutes.GET("/appeals/:id",
				middleware.RequireCapability("moderation.appeal.read"),
				deps.AppealHandler.AdminGetAppeal) // W1-B2: Get appeal with original case context
			// SLICE 7: MIGRATED to capability-based auth with moderation.appeal.review
			// DUAL PROTECTION: RequireAdminMiddleware (existing) + RequireCapability (new)
			adminRoutes.PUT("/appeals/:id/review",
				middleware.RequireCapability("moderation.appeal.review"),
				deps.AppealHandler.AdminReviewAppeal) // Review/appeal decision

			// ===== WARNINGS SYSTEM (Admin) =====
			// WarningHandler admin routes - list, issue, and revoke warnings
			adminRoutes.GET("/warnings",
				middleware.RequireCapability("moderation.case.read"),
				deps.WarningHandler.AdminListWarnings) // List all warnings
			adminRoutes.POST("/warnings",
				middleware.RequireCapability("moderation.case.resolve"),
				deps.WarningHandler.AdminIssueWarning) // Issue warning to user
			adminRoutes.DELETE("/warnings/:id/revoke",
				middleware.RequireCapability("moderation.case.resolve"),
				deps.WarningHandler.AdminRevokeWarning) // Revoke warning

			// ===== SUPPORT SYSTEM (Admin) =====
			// SupportHandler admin routes - support ticket management
			// RBAC IMPLEMENTATION: Added support.ticket.read for viewing tickets
			adminRoutes.GET("/support/tickets",
				middleware.RequireCapability("support.ticket.read"),
				deps.SupportHandler.ListAllTickets) // List all tickets
			adminRoutes.GET("/support/tickets/:id",
				middleware.RequireCapability("support.ticket.read"),
				deps.SupportHandler.AdminGetTicket) // Get ticket details
			adminRoutes.PUT("/support/tickets/:id/claim",
				middleware.RequireCapability("support.ticket.claim"),
				deps.SupportHandler.ClaimTicket) // Claim ticket
			adminRoutes.PUT("/support/tickets/:id/resolve",
				middleware.RequireCapability("support.ticket.resolve"),
				deps.SupportHandler.ResolveTicket) // Resolve ticket
			adminRoutes.PUT("/support/tickets/:id/close",
				middleware.RequireCapability("support.ticket.resolve"),
				deps.SupportHandler.CloseTicket) // Close ticket
			adminRoutes.PUT("/support/tickets/:id/priority",
				middleware.RequireCapability("support.ticket.resolve"),
				deps.SupportHandler.UpdateTicketPriority) // Update priority
			adminRoutes.PUT("/support/tickets/:id/category",
				middleware.RequireCapability("support.ticket.resolve"),
				deps.SupportHandler.UpdateTicketCategory) // Update category
			adminRoutes.PUT("/support/tickets/:id/waiting",
				middleware.RequireCapability("support.ticket.resolve"),
				deps.SupportHandler.SetWaitingForUser) // Set waiting for user
			adminRoutes.GET("/support/statistics",
				middleware.RequireCapability("support.admin.read"),
				deps.SupportHandler.GetStatistics) // Support statistics
			adminRoutes.GET("/support/admins",
				middleware.RequireCapability("support.admin.read"),
				deps.SupportHandler.ListAdmins) // List support admins
			adminRoutes.GET("/support/admins/available",
				middleware.RequireCapability("support.admin.read"),
				deps.SupportHandler.GetAvailableAdmins) // Available admins
			adminRoutes.POST("/support/tickets/:id/messages",
				middleware.RequireCapability("support.ticket.respond"),
				deps.SupportHandler.SendMessage) // Send message to ticket
			adminRoutes.GET("/support/tickets/:id/messages",
				middleware.RequireCapability("support.ticket.read"),
				deps.SupportHandler.AdminListMessages) // List ticket messages
			adminRoutes.POST("/support/tickets/:id/escalate-to-dispute",
				middleware.RequireCapability("support.ticket.escalate"),
				deps.SupportHandler.EscalateToDispute) // Escalate ticket to dispute

			// ===== ADMIN BLOCK VISIBILITY (Operational) =====
			// Admin visibility into block relationships for abuse investigation
			// RBAC IMPLEMENTATION: Using governance.user.read for viewing user blocks
			adminRoutes.GET("/users/:id/blocks",
				middleware.RequireCapability("governance.user.read"),
				deps.FollowHandler.GetBlockedUsers) // View user's blocked list

			// ===== ADMIN PAYOUT OPERATIONS (Payout Hardening Batch #1) =====
			// Admin endpoints for payout withdrawal management
			// These endpoints provide operational control over the payout lifecycle
			//
			// List withdrawals with filtering
			// GET /api/v1/admin/payouts/withdrawals?page=1&page_size=20&status=REQUESTED
			// RBAC IMPLEMENTATION: Added finance.withdraw.read for viewing withdrawals
			adminRoutes.GET("/payouts/withdrawals",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.AdminPayoutHandler.ListWithdrawals)

			// Get withdrawal details
			// GET /api/v1/admin/payouts/withdrawals/:id
			// RBAC IMPLEMENTATION: Added finance.withdraw.read for viewing withdrawal details
			adminRoutes.GET("/payouts/withdrawals/:id",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.AdminPayoutHandler.GetWithdrawalDetails)

			// Approve withdrawal (REQUESTED -> PROCESSING)
			// POST /api/v1/admin/payouts/withdrawals/:id/approve
			// SLICE 3: MIGRATED to capability-based auth with finance.withdraw.review
			// DUAL PROTECTION: RequireAdminMiddleware (existing) + RequireCapability (new)
			adminRoutes.POST("/payouts/withdrawals/:id/approve",
				middleware.RequireCapability("finance.withdraw.review"),
				deps.AdminPayoutHandler.ApproveWithdrawal)

			// Reject withdrawal (REQUESTED -> FAILED, funds returned to seller)
			// POST /api/v1/admin/payouts/withdrawals/:id/reject
			// SLICE 3: MIGRATED to capability-based auth with finance.withdraw.review
			// DUAL PROTECTION: RequireAdminMiddleware (existing) + RequireCapability (new)
			adminRoutes.POST("/payouts/withdrawals/:id/reject",
				middleware.RequireCapability("finance.withdraw.review"),
				deps.AdminPayoutHandler.RejectWithdrawal)

			// Mark withdrawal as processed (PROCESSING -> SETTLED, manual completion)
			// POST /api/v1/admin/payouts/withdrawals/:id/mark-processed
			// SLICE 3: MIGRATED to capability-based auth with finance.withdraw.review
			// DUAL PROTECTION: RequireAdminMiddleware (existing) + RequireCapability (new)
			// CRITICAL: Cannot be used if withdrawal has external_reference_id set (gateway submitted)
			adminRoutes.POST("/payouts/withdrawals/:id/mark-processed",
				middleware.RequireCapability("finance.withdraw.review"),
				deps.AdminPayoutHandler.MarkWithdrawalProcessed)

			// ===== PAYOUT OPS HARDENING: FINANCE CANONICAL VISIBILITY =====
			// Read-only canonical ledger and verifier endpoints.
			// Protected by RequireAdminMiddleware + finance.withdraw.read capability.
			// NO mutations from any of these endpoints.

			// Canonical ledger export
			// GET /api/v1/admin/finance/ledger?from=&to=&reference_type=&limit=&offset=
			adminRoutes.GET("/finance/ledger",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.AdminFinanceHandler.ListLedger)

			// Invariant verifier (forensic by default, ?mode=strict to escalate all findings)
			// POST /api/v1/admin/finance/verify?mode=forensic|strict
			adminRoutes.POST("/finance/verify",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.AdminFinanceHandler.RunVerifier)

			// ===== PASS_18Z: FINANCE/RECONCILIATION SUMMARY (read-only) =====
			// GET /api/v1/admin/finance/summary
			// Aggregates existing ledger/alert/reconciliation data for
			// owner/admin visibility. Introduces no new accounting model —
			// see AdminFinanceHandler.GetSummary doc comment. Reuses
			// finance.withdraw.read (same capability as the sibling
			// ledger/verifier/reconciliation endpoints above); no more
			// specific "finance.summary.view" capability exists yet.
			adminRoutes.GET("/finance/summary",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.AdminFinanceHandler.GetSummary)

			// Payout pilot whitelist audit history (read-only, append-only log)
			// GET /api/v1/admin/payouts/whitelist/audit?seller_id=<uuid>&limit=50&offset=0
			adminRoutes.GET("/payouts/whitelist/audit",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.AdminPayoutHandler.ListWhitelistAudit)

			// Dev-only: mock webhook test + signature generator for sandbox payout completion proof.
			// POST /api/v1/admin/payouts/webhooks/test  — inject SUCCESS/FAILED callback without signature
			// POST /api/v1/admin/payouts/webhooks/sign  — generate HMAC signature for real webhook testing
			// NOT mounted in production (cfg.IsDevelopment() guard).
			if cfg.IsDevelopment() {
				adminRoutes.POST("/payouts/webhooks/test",
					middleware.RequireCapability("finance.withdraw.review"),
					deps.PayoutWebhookHandler.MockWebhookTestHandler)
				adminRoutes.POST("/payouts/webhooks/sign",
					middleware.RequireCapability("finance.withdraw.review"),
					deps.PayoutWebhookHandler.GenerateTestSignature)

				// Dev-only projection worker control — BATCH F3 smoke test surface.
				// GET  /api/v1/admin/projection/status   — lag + row counts
				// POST /api/v1/admin/projection/rebuild  — full RebuildAll (slow, idempotent)
				// POST /api/v1/admin/projection/process  — one incremental batch
				adminRoutes.GET("/projection/status",
					middleware.RequireCapability("order.read"),
					deps.ProjectionAdminHandler.GetStatus)
				adminRoutes.POST("/projection/rebuild",
					middleware.RequireCapability("order.read"),
					deps.ProjectionAdminHandler.Rebuild)
				adminRoutes.POST("/projection/process",
					middleware.RequireCapability("order.read"),
					deps.ProjectionAdminHandler.Process)
			}

			// ===== MANAGEMENT PRE-FIX M1: PLATFORM CONFIG =====
			// Platform configuration management with capability-based authority
			//
			// List all configs (GET /api/v1/admin/config)
			// MANAGEMENT PRE-FIX M1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			adminRoutes.GET("/config",
				middleware.RequireCapability("config.view"),
				deps.PlatformConfigHandler.GetAllConfigs)

			// Get single config (GET /api/v1/admin/config/:key)
			// MANAGEMENT PRE-FIX M1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			adminRoutes.GET("/config/:key",
				middleware.RequireCapability("config.view"),
				deps.PlatformConfigHandler.GetConfig)

			// Update config (PUT /api/v1/admin/config/:key)
			// MANAGEMENT PRE-FIX M1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			// + Audit logging in handler for all mutations
			// STEP 2: Split capabilities - accepts either config.update.general or config.update.financial
			// Handler enforces specific capability based on config key type
			adminRoutes.PUT("/config/:key",
				requireAnyCapability("config.update.general", "config.update.financial"),
				deps.PlatformConfigHandler.UpdateConfig)

			// ===== SELLER SUBSCRIPTION CONFIG - Admin singleton read/update =====
			// GET  /api/v1/admin/seller-subscription-config  — read active config row
			// PUT  /api/v1/admin/seller-subscription-config  — update active config row (financial capability)
			// DUAL PROTECTION: RequireAdminMiddleware (group) + RequireCapability (per route)
			adminRoutes.GET("/seller-subscription-config",
				middleware.RequireCapability("config.view"),
				deps.AdminSubscriptionConfigHandler.GetConfig)
			adminRoutes.PUT("/seller-subscription-config",
				middleware.RequireCapability("config.update.financial"),
				deps.AdminSubscriptionConfigHandler.UpdateConfig)

			// ===== PASS_18W: PAYMENT METHOD FEE CONFIG - Admin governance =====
			// GET  /api/v1/admin/payment-methods              — list all methods (enabled+disabled)
			// GET  /api/v1/admin/payment-methods/:code         — get one method's full config
			// PUT  /api/v1/admin/payment-methods/:code         — edit fee formula/enabled/channels/etc
			// POST /api/v1/admin/payment-methods/:code/preview — simulate fee for a sample base amount
			// DUAL PROTECTION: RequireAdminMiddleware (group) + RequireCapability (per route)
			// SAFETY: only ever writes payment_methods; never touches existing
			// orders/payments — see AdminPaymentMethodHandler package doc comment.
			adminRoutes.GET("/payment-methods",
				middleware.RequireCapability("finance.payment_method.view"),
				deps.AdminPaymentMethodHandler.ListMethods)
			adminRoutes.GET("/payment-methods/:code",
				middleware.RequireCapability("finance.payment_method.view"),
				deps.AdminPaymentMethodHandler.GetMethod)
			adminRoutes.PUT("/payment-methods/:code",
				middleware.RequireCapability("finance.payment_method.manage"),
				deps.AdminPaymentMethodHandler.UpdateMethod)
			adminRoutes.POST("/payment-methods/:code/preview",
				middleware.RequireCapability("finance.payment_method.view"),
				deps.AdminPaymentMethodHandler.PreviewFee)

			// ===== SELLER SUBSCRIPTION RECOVERY - Manual webhook-miss remediation =====
			// POST /api/v1/admin/seller-subscriptions/recover/:payment_id
			// Recovers a settled subscription payment that has no seller_subscriptions row.
			// DUAL PROTECTION: RequireAdminMiddleware (group) + RequireCapability (per route)
			adminRoutes.POST("/seller-subscriptions/recover/:payment_id",
				middleware.RequireCapability("seller.subscription.recover"),
				deps.AdminSubscriptionRecoveryHandler.Recover)

			// ===== ALERT SYSTEM V1 - Anomaly Detection and Alerting =====
			// Alert system endpoints for managing system alerts
			//
			// List alerts with filtering (GET /api/v1/admin/alerts)
			// SLICE 1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			adminRoutes.GET("/alerts-v1",
				middleware.RequireCapability("governance.alert.read"),
				deps.AdminAlertHandler.ListAlerts) // List all alerts with filters

			// Get alert details (GET /api/v1/admin/alerts/:id)
			// SLICE 1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			adminRoutes.GET("/alerts-v1/:id",
				middleware.RequireCapability("governance.alert.read"),
				deps.AdminAlertHandler.GetAlertDetail) // Get alert details

			// Get alert statistics (GET /api/v1/admin/alerts/stats)
			// SLICE 1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			adminRoutes.GET("/alerts-v1/stats",
				middleware.RequireCapability("governance.alert.read"),
				deps.AdminAlertHandler.GetAlertStats) // Alert statistics summary

			// Acknowledge alert (POST /api/v1/admin/alerts/:id/acknowledge)
			// SLICE 1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			adminRoutes.POST("/alerts-v1/:id/acknowledge",
				middleware.RequireCapability("governance.alert.resolve"),
				deps.AdminAlertHandler.AcknowledgeAlert) // Mark alert as acknowledged

			// Resolve alert (POST /api/v1/admin/alerts/:id/resolve)
			// SLICE 1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			adminRoutes.POST("/alerts-v1/:id/resolve",
				middleware.RequireCapability("governance.alert.resolve"),
				deps.AdminAlertHandler.ResolveAlert) // Mark alert as resolved

			// Mark as false positive (POST /api/v1/admin/alerts/:id/false-positive)
			// SLICE 1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			adminRoutes.POST("/alerts-v1/:id/false-positive",
				middleware.RequireCapability("governance.alert.resolve"),
				deps.AdminAlertHandler.MarkAsFalsePositive) // Mark alert as false positive

			// Cleanup old alerts (POST /api/v1/admin/alerts/cleanup)
			// SLICE 1: DUAL PROTECTION - RequireAdminMiddleware + RequireCapability
			adminRoutes.POST("/alerts-v1/cleanup",
				middleware.RequireCapability("governance.alert.resolve"),
				deps.AdminAlertHandler.CleanupOldAlerts) // Cleanup old resolved alerts

			// ===== RECONCILIATION VISIBILITY - Read-only result history (PASS_9B) =====
			// Reconciliation is verification-only (RUNTIME-INVARIANTS §7.1, ADR-002).
			// These routes are strictly GET — no mutation, no repair. Reuses
			// finance.withdraw.read, the existing capability already gating the
			// sibling read-only ledger/verifier finance-visibility endpoints below.
			adminRoutes.GET("/reconciliation",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.AdminReconciliationHandler.ListReconciliationResults)
			adminRoutes.GET("/reconciliation/latest",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.AdminReconciliationHandler.GetLatestReconciliationResult)
			adminRoutes.GET("/reconciliation/:id",
				middleware.RequireCapability("finance.withdraw.read"),
				deps.AdminReconciliationHandler.GetReconciliationResult)
		}

		// ===== MANAGEMENT PRE-FIX M1: PUBLIC FEATURE FLAG ENDPOINT =====
		// Feature flag check (GET /api/v1/config/feature/:key)
		// Accessible by any authenticated user for client-side feature availability
		v1.GET("/config/feature/:key", deps.PlatformConfigHandler.GetFeatureFlag)

		// Feed domain routes (CORE - Home Feed)
		// Social-first timeline: posts, requests, and reposts only
		// Does NOT include commerce objects (for_sale items, auctions) directly
		feedRoutes := v1.Group("/feed")
		feedRoutes.GET("", deps.FeedHandler.GetFeed)

		// Like domain routes (CORE - Content and Comment Engagement)
		// Like system for: content (post/request), comment
		// NOTE: GET /stats moved to v1Browse (public browse group)
		likeRoutes := v1.Group("/likes")
		{
			// Toggle like/unlike on supported target types
			// Interaction gate: verified email + active account (mirrors
			// CreateContent / RepostContent — social mutations require
			// the same interaction authority, not just active account).
			likeRoutes.POST("/toggle", middleware.RequireActiveAccount(db.Pgx()), deps.LikeHandler.ToggleLike)
		}

		// Content domain routes (CORE - Social Content)
		// Social posts, requests, and reposts
		// NOTE: GET /:id moved to v1Browse (public browse group)
		contentRoutes := v1.Group("/contents")
		contentRoutes.POST("", middleware.RequireActiveAccount(db.Pgx()), deps.ContentHandler.CreateContent)
		contentRoutes.PUT("/:id", middleware.RequireActiveAccount(db.Pgx()), deps.ContentHandler.UpdateContent)
		contentRoutes.DELETE("/:id", middleware.RequireActiveAccount(db.Pgx()), deps.ContentHandler.DeleteContent)

		// Repost routes (SHARE CONTRACT V1)
		// Repost IS content creation — same interaction-authority gate as POST /contents.
		contentRoutes.POST("/:id/repost", middleware.RequireActiveAccount(db.Pgx()), deps.ContentHandler.RepostContent)
		// Comment domain routes (CORE - Content Comments)
		// Comment system for content engagement
		// Interaction gate: verified email + active account.
		contentRoutes.POST("/:id/comments", middleware.RequireActiveAccount(db.Pgx()), deps.CommentHandler.CreateComment)
		contentRoutes.GET("/:id/comments", deps.CommentHandler.ListComments)

		// Comment-specific routes (delete by comment ID)
		// Content-removal doctrine: comment deletion is content removal, not a social-graph
		// reduction (unlike unfollow/unblock). All content operations require active account.
		v1.DELETE("/comments/:id", middleware.RequireActiveAccount(db.Pgx()), deps.CommentHandler.DeleteComment)

		// Commerce reference comment routes (seller responses to requests / auctions)
		contentRoutes.POST("/:id/comments/reference", middleware.RequireActiveAccount(db.Pgx()), deps.CommentHandler.CreateCommerceReferenceComment)

		// Notification domain routes (CORE)
		// Push notification management and delivery
		notificationRoutes := v1.Group("/notifications")
		{
			// List notifications for authenticated user
			notificationRoutes.GET("", deps.NotificationHandler.GetNotifications)

			// Get unread count
			notificationRoutes.GET("/unread-count", deps.NotificationHandler.GetUnreadCount)

			// Mark notification as read
			notificationRoutes.POST("/:id/read", deps.NotificationHandler.MarkNotificationAsRead)

			// Mark notifications as read by entity type and entity ID
			// Used for cross-domain sync (e.g., chat read → chat notifications read)
			notificationRoutes.POST("/read-by-entity", deps.NotificationHandler.MarkAsReadByEntity)

			// Mark all notifications as read
			notificationRoutes.POST("/read-all", deps.NotificationHandler.MarkAllAsRead)

			// Delete a notification
			notificationRoutes.DELETE("/:id", deps.NotificationHandler.DeleteNotification)

			// Register FCM token for push notifications
			notificationRoutes.POST("/fcm-token", deps.FCMTokenHandler.RegisterToken)

			// Unregister FCM token
			notificationRoutes.DELETE("/fcm-token", deps.FCMTokenHandler.UnregisterToken)
		}

		// ===== MODERATION MODULE =====
		// CANONICAL REPORT RUNTIME (SLICE 2)
		// The legacy POST /moderation/cases (CreateCase → GovernanceCase →
		// moderation_cases) intake has been removed. The canonical Report
		// contract is the single Report authority:
		//   POST /reports      — create an immutable Report
		//   GET  /reports/mine — list own Reports
		//   GET  /reports/:id  — get own Report by ID
		reportRoutes := v1.Group("/reports")
		{
			// Report submission is an interaction — require verified email + active account.
			reportRoutes.POST("", middleware.RequireActiveAccount(db.Pgx()), deps.ReportHandler.CreateReport)
			// Get user's own reports
			reportRoutes.GET("/mine", deps.ReportHandler.ListMyReports)
			// Get specific report (user's own reports only)
			reportRoutes.GET("/:id", deps.ReportHandler.GetMyReport)
		}

		// ========================================================================
		// ADMIN / OPERABILITY HARDENING PACK V1 - USER FACING ROUTES
		// ========================================================================

		// ===== APPEALS SYSTEM (User) =====
		// AppealHandler user routes - create and view appeals
		v1.POST("/appeals", deps.AppealHandler.CreateAppeal)    // Create appeal for moderation decision
		v1.GET("/appeals/:id", deps.AppealHandler.GetAppeal)    // Get specific appeal
		v1.GET("/appeals/me", deps.AppealHandler.ListMyAppeals) // List my appeals

		// ===== WARNINGS SYSTEM (User) =====
		// WarningHandler user routes - view own warnings
		v1.GET("/warnings/:id", deps.WarningHandler.GetWarning)                     // Get specific warning
		v1.GET("/warnings", deps.WarningHandler.ListWarnings)                       // List my warnings
		v1.GET("/users/:id/warnings/active", deps.WarningHandler.GetActiveWarnings) // Get active warnings for user
		v1.GET("/users/:id/warnings", deps.WarningHandler.GetUserWarnings)          // Get all warnings for user

		// ===== SUPPORT SYSTEM (User) =====
		// SupportHandler user routes - create and manage support tickets
		v1.POST("/support/tickets", deps.SupportHandler.CreateTicket)           // Create support ticket
		v1.GET("/support/tickets", deps.SupportHandler.ListMyTickets)           // List my tickets
		v1.GET("/support/tickets/my/open", deps.SupportHandler.GetMyOpenTicket) // Get my open ticket
		v1.GET("/support/tickets/:id", deps.SupportHandler.GetTicket)           // Get ticket details
		v1.GET("/support/tickets/:id/events", deps.SupportHandler.ListEvents)   // List ticket events
		v1.PUT("/support/tickets/:id/reopen", deps.SupportHandler.ReopenTicket) // Reopen resolved/closed ticket

		// ===== SOCIAL DOMAIN (CORE) =====
		// Social graph operations: follow, block, mute
		//
		// DESIGN PRINCIPLES:
		// - follow = social graph connection (bidirectional visibility)
		// - block = hard restriction (removes follows, prevents interactions)
		// - mute = content filtering (hides content, doesn't prevent interactions)

		// User-scoped routes (original handler pattern)
		// These routes operate on a target user ID in the path
		userSocialRoutes := v1.Group("/users/:id")
		{
			// Follow operations
			// Follow is an interaction that requires verified email + active account;
			// unfollow is left open so that REDUCING a relationship is never blocked
			// by an interaction gate.
			userSocialRoutes.POST("/follow", middleware.RequireActiveAccount(db.Pgx()), deps.FollowHandler.FollowUser)
			userSocialRoutes.DELETE("/follow", deps.FollowHandler.UnfollowUser)

			// Block operations
			// Adding a block is a write action — requires active account.
			// Removing a block is a reducing action — intentionally open (matches unfollow doctrine).
			userSocialRoutes.POST("/block", middleware.RequireActiveAccount(db.Pgx()), deps.FollowHandler.BlockUser)
			userSocialRoutes.DELETE("/block", deps.FollowHandler.UnblockUser)

			// Mute operations
			// Same doctrine as block: adding mute requires active account; removing mute is open.
			userSocialRoutes.POST("/mute", middleware.RequireActiveAccount(db.Pgx()), deps.FollowHandler.MuteUser)
			userSocialRoutes.DELETE("/mute", deps.FollowHandler.UnmuteUser)
		}

		// Follow list routes (get followers/following for a user)
		v1.GET("/users/:id/followers", deps.FollowHandler.ListFollowers)
		v1.GET("/users/:id/following", deps.FollowHandler.ListFollowing)

		// Follow status check (between current user and target user)
		v1.GET("/follows/status/:userId", deps.FollowHandler.GetFollowStatus)

		// Current user's blocked/muted lists (uses authenticated user from context)
		v1.GET("/blocks", deps.FollowHandler.GetBlockedUsers)
		v1.GET("/mutes", deps.FollowHandler.GetMutedUsers)

		// ===== RATING DOMAIN (CORE) =====
		// Buyer→Seller order ratings (immutable, no edit/delete)
		//
		// BUSINESS TRUTH:
		// - Rating is IMMUTABLE (no update/delete after creation)
		// - Only buyer can rate seller (not seller→buyer)
		// - Order must be completed before rating
		// - One rating per order (enforced by UNIQUE constraint)
		//
		// Canonical RESTful endpoints:
		// - POST /api/v1/orders/{id}/ratings - Create rating for completed order
		// - GET /api/v1/users/{id}/ratings - Get ratings received by seller
		// - GET /api/v1/users/me/ratings/given - Get ratings given by buyer
		orderRoutes.POST("/:id/ratings", middleware.RequireActiveAccount(db.Pgx()), deps.RatingHandler.CreateRating)
		v1.GET("/users/:id/ratings", deps.RatingHandler.ListRatingsReceived)
		v1.GET("/users/:id/ratings/summary", deps.RatingHandler.GetRatingSummary)
		v1.GET("/users/me/ratings/given", deps.RatingHandler.ListRatingsGiven)

		// ===== SOCIAL ROUTES - FLUTTER COMPATIBILITY =====
		// These routes follow the Flutter app's expected API pattern
		// They map to the same handlers but with different path structures
		//
		// NOTE: The Flutter datasource expects these routes:
		// - POST /api/v1/follows (body: { following_id })
		// - DELETE /api/v1/follows/{userId}
		// - GET /api/v1/follows/{userId}/followers
		// - GET /api/v1/follows/{userId}/following
		// - POST /api/v1/blocks (body: { blocked_user_id })
		// - DELETE /api/v1/blocks/{userId}
		// - GET /api/v1/blocks
		// - POST /api/v1/mutes (body: { muted_user_id })
		// - DELETE /api/v1/mutes/{userId}
		// - GET /api/v1/mutes
		//
		// CURRENTLY: The existing handlers expect routes in the pattern:
		// - POST /api/v1/users/{id}/follow
		// - GET /api/v1/users/{id}/followers
		// etc.
		//
		// The Flutter routes above are NOT yet implemented.
		// This is a TODO for Flutter route parity if needed.

		// ===== PROMOTION DISCOVERY ROUTES (Phase 4) =====
		// Public discovery endpoints for promoted items
		// These are used by search, home, and other discovery surfaces
		//
		// GET /api/v1/promotions/discover - Get all promoted items
		// GET /api/v1/promotions/discover/:target_type - Get promoted items by type
		promotionRoutes := v1.Group("/promotions")
		{
			// Public discovery endpoints (used by search, home)
			promotionRoutes.GET("/discover", deps.PromotionHandler.GetPromotedItems)
			promotionRoutes.GET("/discover/:target_type", deps.PromotionHandler.GetPromotedItemsByTarget)

			// Package endpoints (seller-only — purchase is a seller growth action)
			promotionRoutes.GET("/packages", deps.PromotionHandler.ListPackages)
			// Purchase requires active account + active seller subscription.
			promotionRoutes.POST("/packages/purchase",
				middleware.RequireActiveAccount(db.Pgx()),
				middleware.RequireSellerMiddleware(deps.RoleChecker),
				deps.PromotionHandler.PurchasePackage)

			// Ownership endpoints (authenticated users — reads only)
			promotionRoutes.GET("/my/ownerships", deps.PromotionHandler.ListMyOwnerships)
			promotionRoutes.GET("/ownerships/:id", deps.PromotionHandler.GetOwnership)

			// Instance endpoints (authenticated users — reads only)
			promotionRoutes.GET("/my/instances", deps.PromotionHandler.ListMyInstances)
			promotionRoutes.GET("/instances/:id", deps.PromotionHandler.GetInstance)

			// Seller-gated activation/resume/reassign endpoints.
			// Activate, resume, and reassign are seller growth actions: they open or restore
			// promoted visibility. An active seller subscription is required.
			// Deactivate (pause/cancel) is allowed regardless of subscription state.
			promotionSellerRoutes := promotionRoutes.Group("")
			promotionSellerRoutes.Use(
				middleware.RequireActiveAccount(db.Pgx()),
				middleware.RequireSellerMiddleware(deps.RoleChecker),
			)
			{
				promotionSellerRoutes.POST("/activate", deps.PromotionHandler.ActivatePromotion)
				promotionSellerRoutes.POST("/instances/:id/resume", deps.PromotionHandler.ResumePromotion)
				promotionSellerRoutes.POST("/instances/:id/reassign", deps.PromotionHandler.ReassignPromotion)
			}
			// Deactivate does NOT require seller subscription — degraded sellers must still
			// be able to cancel/pause their active promotions.
			promotionRoutes.POST("/instances/:id/deactivate", deps.PromotionHandler.DeactivatePromotion)

			// Analytics: record a viewer interaction (click) with a promoted item.
			// Auth: required (all promotion surfaces are authenticated).
			// Analytics-only — zero finance or lifecycle effect.
			promotionRoutes.POST("/events", deps.PromotionHandler.RecordEvent)

			// External product user APIs (seller-only — external product is a promotion asset)
			externalProductRoutes := promotionRoutes.Group("")
			externalProductRoutes.Use(
				middleware.RequireActiveAccount(db.Pgx()),
				middleware.RequireSellerMiddleware(deps.RoleChecker),
			)
			{
				externalProductRoutes.POST("/external-products", deps.PromotionHandler.CreateExternalProduct)
				externalProductRoutes.PATCH("/external-products/:id", deps.PromotionHandler.UpdateExternalProduct)
				externalProductRoutes.POST("/external-products/:id/submit", deps.PromotionHandler.SubmitExternalProduct)
				externalProductRoutes.POST("/external-products/:id/resubmit", deps.PromotionHandler.ResubmitExternalProduct)
				externalProductRoutes.POST("/external-products/:id/media", deps.PromotionHandler.AttachExternalProductMedia)
				externalProductRoutes.GET("/external-products/:id/media", deps.PromotionHandler.ListExternalProductMedia)
				externalProductRoutes.DELETE("/external-products/:id/media/:media_id", deps.PromotionHandler.DeleteExternalProductMedia)
				externalProductRoutes.GET("/my/external-products", deps.PromotionHandler.ListMyExternalProducts)
				externalProductRoutes.GET("/external-products/:id", deps.PromotionHandler.GetExternalProduct)
			}
		}
	}
}

// healthCheckHandler returns the health status of the service
func healthCheckHandler(cfg *config.Config, db *database.DB, redisClient *pkgRedis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := "healthy"
		httpStatus := http.StatusOK

		// Check database
		if db != nil {
			if err := database.HealthCheck(db); err != nil {
				status = "unhealthy"
				httpStatus = http.StatusServiceUnavailable
			}
		}

		c.JSON(httpStatus, gin.H{
			"status":      status,
			"version":     cfg.App.Version,
			"environment": cfg.Server.Env,
		})
	}
}

// evaluateReadiness computes the overall readiness result from basic infra
// checks (DB/Redis), the runtime activation state of money-safety-critical
// detector workers (PASS_18R), and the payout completion-loop safety state
// (PASS_18S). Extracted as a pure function, independent of gin/HTTP/DB/Redis,
// so it can be unit tested directly without a real server.
//
// Semantics:
//   - dbOK/redisOK false always fails readiness, in every environment.
//   - A critical detector worker fully disabled ("dark"), OR the payout
//     completion loop being unsafe (PayoutWorker enabled with neither a
//     configured webhook nor a functional reconciliation path), degrades
//     readiness. In development this is reported but does not block boot —
//     local/dev must remain usable. In staging, production, or an
//     unknown/nil config (fail-closed — never assume development), either
//     condition fails readiness, so an operator cannot mistake a dark safety
//     net or an unsafe payout loop for a healthy runtime.
func evaluateReadiness(cfg *config.Config, dbOK, redisOK bool, workerStatuses []worker.CriticalWorkerStatus, payoutSafety config.PayoutCompletionSafety) (ready bool, degraded bool) {
	ready = dbOK && redisOK
	degraded = worker.AnyCriticalWorkerDark(workerStatuses) || payoutSafety.Degraded

	isDevelopment := cfg != nil && cfg.IsDevelopment()
	if degraded && !isDevelopment {
		ready = false
	}

	return ready, degraded
}

// readinessHandler checks if the service is ready to accept traffic
func readinessHandler(cfg *config.Config, db *database.DB, redisClient *pkgRedis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		dbOK := true
		if db != nil {
			if err := database.HealthCheck(db); err != nil {
				dbOK = false
			}
		}

		redisOK := true
		if redisClient != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
			defer cancel()
			if err := redisClient.HealthCheck(ctx); err != nil {
				redisOK = false
			}
		}

		workerStatuses := worker.CriticalWorkerStatuses()
		var payoutSafety config.PayoutCompletionSafety
		if cfg != nil {
			payoutSafety = cfg.EvaluatePayoutCompletionSafety()
		}
		ready, degraded := evaluateReadiness(cfg, dbOK, redisOK, workerStatuses, payoutSafety)

		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, gin.H{
			"ready":         ready,
			"degraded":      degraded,
			"worker_safety": workerStatuses,
			"payout_safety": payoutSafety,
		})
	}
}

// livenessHandler checks if the service is alive
func livenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"alive": true,
		})
	}
}
