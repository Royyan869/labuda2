class RoutePaths {
  static const String splash = '/splash';
  static const String welcome = '/welcome';
  static const String home = '/home';
  static const String signIn = '/auth/sign-in';
  static const String signUp = '/auth/sign-up';
  static const String forgotPassword = '/auth/forgot-password';

  /// Complete Profile Route - Required for new Google users
  static const String completeProfile = '/auth/complete-profile';

  /// Verify Email Route - D2 hard gate: exclusive surface while a Firebase
  /// identity's email is unverified (AuthStatePendingEmailVerification).
  static const String verifyEmail = '/auth/verify-email';

  /// Account Restricted Route - Shown when account is suspended or banned
  static const String accountRestricted = '/account-restricted';

  static const String profile = '/profile';
  static const String addresses = '/profile/addresses';
  static const String editProfile = '/profile/edit';
  static const String personalInformation = '/profile/personal-info';
  static const String userProfile = '/user/:userId';

  /// Follow graph of a user profile. `?type=followers|following` selects the
  /// list; the path itself is the canonical shareable identity.
  static const String followList = '/user/:userId/follows';

  /// Account & safety surfaces (settings subtree).
  static const String security = '/settings/security';
  static const String loginSessions = '/settings/security/sessions';
  static const String blockedUsers = '/settings/blocked-users';
  static const String myReports = '/settings/reports';
  static const String termsOfService = '/settings/terms';
  static const String privacyPolicy = '/settings/privacy';

  /// Help center (self-help entry) and the article reading surface.
  /// `/help` is the shareable entry; the category path carries the canonical
  /// [HelpCategory] name. The article surface has no stable content id yet
  /// (articles are localized strings resolved from AppLocalizations), so it
  /// carries its content as route extra and is deliberately not externally
  /// deep-linkable (see HelpArticleScreen).
  static const String helpCenter = '/help';
  static const String helpCategory = '/help/category/:category';
  static const String helpArticle = '/help/article';

  /// User's support ticket inbox and the ticket thread.
  static const String supportTickets = '/support/tickets';
  static const String supportTicketThread = '/support/tickets/:ticketId';

  // ============================================================================
  // PUBLIC COMMERCE ROUTES - Use these for product browsing
  // ============================================================================
  static const String forSales = '/for-sale';
  static const String forSaleDetail = '/for-sale/:forSaleId';
  static const String createForSale = '/create/for-sale';

  // Content creation routes
  static const String createContent = '/create/content';

  // ============================================================================
  // INTERNAL ONLY - Seller Management Routes (DO NOT USE IN PUBLIC UI)
  // For public product browsing, use `/for-sale`
  // and `/for-sale/:forSaleId` above
  // ============================================================================
  static const String auctionDetails = '/auction/:auctionId';
  static String auctionDetail(String auctionId) => '/auction/$auctionId';
  static const String chat = '/chat';
  static const String newChat = '/chat/new';
  static const String chatConversation = '/chat/:conversationId';
  static const String notifications = '/notifications';
  static const String settings = '/settings';
  static const String search = '/search';
  static const String searchResults = '/search/results';
  // ============================================================================
  // ⚠️ INTERNAL ONLY - DO NOT USE IN PUBLIC UI ⚠️
  // For public product creation, use `/create/for-sale` instead
  // ============================================================================
  static const String createAuction = '/create/auction';

  // Report routes
  static const String report = '/report';

  // Saved Items routes (unified shortlist + auction watch)
  static const String savedItems = '/saved-items';

  // Coins routes (loyalty points - NOT wallet/payment)
  static const String coins = '/coins';
  static const String coinsHistory = '/coins/history';

  // Seller routes
  static const String sellerDashboard = '/seller/dashboard';
  // Canonical seller order list (registered in seller_module). Named constant
  // so UI never hardcodes the path literal.
  static const String sellerOrders = '/seller/orders';
  // PARKED V1: No SellerWarningsScreen exists yet. Path reserved for future
  // warnings inbox. Do not navigate here — the route is not wired in the router.
  static const String sellerWarnings = '/seller/warnings';
  // Seller For Sale management surface (V1)
  static const String sellerForSales = '/seller/for-sale';
  // Seller auction management surface (V1): owner inventory across every
  // status, including the relist entry point for auctions ended with no bid.
  static const String sellerAuctions = '/seller/auctions';
  static const String sellerUpgrade = '/seller/upgrade';
  static const String sellerRenewal = '/seller/renewal';
  static const String sellerVerification = '/verification/seller';
  static const String sellerEarnings = '/seller/earnings';
  static const String sellerAnalytics = '/seller/analytics';
  static const String sellerPerformance = '/seller/performance';
  static const String sellerShipping = '/seller/shipping';
  static const String sellerShippingSetup = '/seller/shipping/setup';
  static const String sellerShippingSetupCityRules =
      '/seller/shipping/setup/city-rules';
  static const String sellerBankAccounts = '/seller/bank-accounts';
  // Seller discount management (legacy discount surface with a live settings
  // entry). Create/edit are separate canonical routes; edit carries the
  // discount entity as route extra because the form is a seller-only
  // management surface, not an externally shareable destination.
  static const String sellerDiscounts = '/seller/discounts';
  static const String sellerDiscountCreate = '/seller/discounts/create';
  static const String sellerDiscountEdit = '/seller/discounts/:discountId/edit';
  // Seller auction management forms (owner-only, not externally shareable).
  static const String sellerAuctionEdit = '/seller/auctions/:auctionId/edit';
  static const String sellerAuctionRelist =
      '/seller/auctions/:auctionId/relist';
  static const String sellerCanonicalPromotionAnalytics =
      '/seller/promotions/:contractId/analytics';
  static String sellerCanonicalPromotionAnalyticsPath(String contractId) =>
      '/seller/promotions/$contractId/analytics';
  // Canonical promotion management list (seller self-service).
  static const String sellerCanonicalPromotions =
      '/seller/canonical-promotions';
  static const String sellerPromotionContractCreate =
      '/seller/canonical-promotions/create';
  // Canonical promotion product-queue (refill) management.
  static const String sellerCanonicalPromotionQueue =
      '/seller/canonical-promotions/:contractId/queue';
  static String sellerCanonicalPromotionQueuePath(String contractId) =>
      '/seller/canonical-promotions/$contractId/queue';
  static const String sellerExternalProducts =
      '/seller/promotions/external-products';
  static const String sellerExternalProductDetail =
      '/seller/promotions/external-products/:productId';

  // Checkout routes
  static const String checkout = '/checkout/:forSaleId';
  static const String paymentResult = '/payment-result/:orderId';
  static const String paymentWebview = '/payment-webview';

  // ==========================================================================
  // CANONICAL LOCATION BUILDERS
  // ==========================================================================
  // One spelling per destination. Callers build the location here instead of
  // concatenating path literals, so a route move is a single edit and every
  // navigation stays observable by the router.

  static String forSaleDetailPath(String forSaleId) => '/for-sale/$forSaleId';

  static String orderDetailPath(String orderId) => '/orders/$orderId';

  static String paymentResultPath(String orderId) => '/payment-result/$orderId';

  static String supportTicketThreadPath(String ticketId) =>
      '/support/tickets/$ticketId';

  static String sellerAuctionEditPath(String auctionId) =>
      '/seller/auctions/$auctionId/edit';

  static String sellerAuctionRelistPath(String auctionId) =>
      '/seller/auctions/$auctionId/relist';

  static String sellerDiscountEditPath(String discountId) =>
      '/seller/discounts/$discountId/edit';

  /// Follow graph of a user profile: `?type=followers|following`.
  static String followListPath(String userId, {bool following = false}) =>
      Uri(
        path: '/user/$userId/follows',
        queryParameters: {'type': following ? 'following' : 'followers'},
      ).toString();

  /// Help center category browsing surface (canonical category name).
  static String helpCategoryPath(String category) => '/help/category/$category';

  /// Canonical report location (`?type=&id=&title=`). The report form is the
  /// ONE report destination; the target travels as stable query parameters.
  static String reportLocation({
    required String targetType,
    required String targetId,
    String? targetTitle,
  }) => Uri(
    path: report,
    queryParameters: <String, String>{
      'type': targetType,
      'id': targetId,
      if (targetTitle != null && targetTitle.isNotEmpty) 'title': targetTitle,
    },
  ).toString();
}

class RouteNames {
  static const String splash = 'splash';
  static const String welcome = 'welcome';
  static const String home = 'home';
  static const String signIn = 'signIn';
  static const String signUp = 'signUp';
  static const String forgotPassword = 'forgotPassword';
  static const String completeProfile = 'completeProfile';
  static const String verifyEmail = 'verifyEmail';
  static const String accountRestricted = 'accountRestricted';
  static const String profile = 'profile';
  static const String addresses = 'addresses';
  static const String editProfile = 'editProfile';
  static const String personalInformation = 'personalInformation';
  static const String userProfile = 'userProfile';
  static const String followList = 'followList';

  // Account & safety surfaces
  static const String security = 'security';
  static const String loginSessions = 'loginSessions';
  static const String blockedUsers = 'blockedUsers';
  static const String myReports = 'myReports';
  static const String termsOfService = 'termsOfService';
  static const String privacyPolicy = 'privacyPolicy';

  // Help center surfaces
  static const String helpCenter = 'helpCenter';
  static const String helpCategory = 'helpCategory';
  static const String helpArticle = 'helpArticle';

  static const String supportTickets = 'supportTickets';
  static const String supportTicketThread = 'supportTicketThread';

  // Public commerce route names
  static const String forSales = 'forSales';
  static const String forSaleDetail = 'forSaleDetail';
  static const String createForSale = 'createForSale';

  // Internal-only seller management route names
  static const String auctionDetails = 'auctionDetails';
  static const String chat = 'chat';
  static const String newChat = 'newChat';
  static const String chatConversation = 'chatConversation';
  static const String notifications = 'notifications';
  static const String settings = 'settings';
  static const String search = 'search';
  static const String searchResults = 'searchResults';
  static const String createAuction = 'createAuction';

  // Coins route names
  static const String coins = 'coins';
  static const String coinsHistory = 'coinsHistory';

  // Report route names
  static const String report = 'report';

  // Saved Items route names
  static const String savedItems = 'savedItems';

  // Seller route names
  static const String sellerDashboard = 'sellerDashboard';
  // PARKED V1: route name reserved, no screen wired (see RoutePaths.sellerWarnings).
  static const String sellerWarnings = 'sellerWarnings';
  // Seller For Sale management surface (V1)
  static const String sellerForSales = 'sellerForSales';
  // Seller auction management surface (V1) — see RoutePaths.sellerAuctions.
  static const String sellerAuctions = 'sellerAuctions';
  static const String sellerUpgrade = 'sellerUpgrade';
  static const String sellerRenewal = 'sellerRenewal';
  static const String sellerVerification = 'sellerVerification';
  static const String sellerEarnings = 'sellerEarnings';
  static const String sellerAnalytics = 'sellerAnalytics';
  static const String sellerPerformance = 'sellerPerformance';
  static const String sellerBankAccounts = 'sellerBankAccounts';
  static const String sellerExternalProducts = 'sellerExternalProducts';
  static const String sellerDiscounts = 'sellerDiscounts';
  static const String sellerDiscountCreate = 'sellerDiscountCreate';
  static const String sellerDiscountEdit = 'sellerDiscountEdit';
  static const String sellerAuctionEdit = 'sellerAuctionEdit';
  static const String sellerAuctionRelist = 'sellerAuctionRelist';
  static const String sellerExternalProductDetail =
      'sellerExternalProductDetail';
  static const String sellerCanonicalPromotionAnalytics =
      'sellerCanonicalPromotionAnalytics';
  static const String sellerCanonicalPromotions = 'sellerCanonicalPromotions';

  // Checkout route names
  static const String checkout = 'checkout';
  static const String paymentResult = 'paymentResult';
  static const String paymentWebview = 'paymentWebview';
}
