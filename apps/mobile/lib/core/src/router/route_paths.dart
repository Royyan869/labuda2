class RoutePaths {
  static const String splash = '/splash';
  static const String welcome = '/welcome';
  static const String home = '/home';
  static const String signIn = '/auth/sign-in';
  static const String signUp = '/auth/sign-up';
  static const String forgotPassword = '/auth/forgot-password';

  /// Complete Profile Route - Required for new Google users
  static const String completeProfile = '/auth/complete-profile';

  /// Account Restricted Route - Shown when account is suspended or banned
  static const String accountRestricted = '/account-restricted';

  static const String profile = '/profile';
  static const String addresses = '/profile/addresses';
  static const String editProfile = '/profile/edit';
  static const String personalInformation = '/profile/personal-info';
  static const String userProfile = '/user/:userId';
  static const String supportTicketThread = '/support/tickets/:ticketId';

  // ============================================================================
  // PUBLIC COMMERCE ROUTES - Use these for product browsing
  // ============================================================================
  static const String forSales = '/for-sale';
  static const String forSaleDetail = '/for-sale/:fixedPriceSaleId';
  static const String createForSale = '/create/for-sale';
  static const String editForSale = '/for-sale/:fixedPriceSaleId/edit';

  // Content creation routes
  static const String createContent = '/create/content';

  // ============================================================================
  // INTERNAL ONLY - Seller Management Routes (DO NOT USE IN PUBLIC UI)
  // For public product browsing, use `/for-sale`
  // and `/for-sale/:fixedPriceSaleId` above
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
  static const String bidding = '/bidding';

  // Organizer Team routes
  static const String organizerTeamInvitations = '/organizer-team/invitations';
  static const String organizerTeamInvitationDetail =
      '/organizer-team/invitations/:invitationId';

  // Verification routes
  static const String verification = '/verification';

  // Report routes
  static const String report = '/report';

  // Saved Items routes (unified shortlist + auction watch)
  static const String savedItems = '/saved-items';

  // Coins routes (loyalty points - NOT wallet/payment)
  static const String coins = '/coins';
  static const String coinsHistory = '/coins/history';

  // Seller routes
  static const String sellerDashboard = '/seller/dashboard';
  static const String sellerSettings = '/seller/settings';
  static const String sellerAnalytics = '/seller/analytics';
  // PARKED V1: No SellerWarningsScreen exists yet. Path reserved for future
  // warnings inbox. Do not navigate here — the route is not wired in the router.
  static const String sellerWarnings = '/seller/warnings';
  static const String sellerReviews = '/seller/reviews';
  // Seller For Sale management surface (V1)
  static const String sellerForSales = '/seller/for-sale';
  static const String sellerUpgrade = '/seller/upgrade';
  static const String sellerVerification = '/verification/seller';
  static const String sellerEarnings = '/seller/earnings';
  static const String sellerShipping = '/seller/shipping';
  static const String sellerShippingOptionDetail = '/seller/shipping/:optionId';
  static const String sellerShippingSetup = '/seller/shipping/setup';
  static const String sellerShippingSetupCityRules =
      '/seller/shipping/setup/city-rules';
  static const String sellerBankAccounts = '/seller/bank-accounts';
  static const String sellerPromotions = '/seller/promotions';
  static const String sellerPromotionDetail = '/seller/promotions/:instanceId';
  static const String sellerPromotionActivate = '/seller/promotions/activate';
  static const String sellerExternalProducts =
      '/seller/promotions/external-products';
  static const String sellerExternalProductDetail =
      '/seller/promotions/external-products/:productId';

  // Checkout routes
  static const String checkout = '/checkout/:fixedPriceSaleId';
  static const String paymentResult = '/payment-result/:orderId';
}

class RouteNames {
  static const String splash = 'splash';
  static const String welcome = 'welcome';
  static const String home = 'home';
  static const String signIn = 'signIn';
  static const String signUp = 'signUp';
  static const String forgotPassword = 'forgotPassword';
  static const String completeProfile = 'completeProfile';
  static const String accountRestricted = 'accountRestricted';
  static const String profile = 'profile';
  static const String addresses = 'addresses';
  static const String editProfile = 'editProfile';
  static const String personalInformation = 'personalInformation';
  static const String userProfile = 'userProfile';
  static const String supportTicketThread = 'supportTicketThread';

  // Public commerce route names
  static const String forSales = 'forSales';
  static const String forSaleDetail = 'forSaleDetail';
  static const String createForSale = 'createForSale';
  static const String editForSale = 'editForSale';
  static const String createContent = 'createContent';

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
  static const String bidding = 'bidding';

  // Coins route names
  static const String coins = 'coins';
  static const String coinsHistory = 'coinsHistory';
  static const String coinsTopup = 'coinsTopup';

  // Organizer Team route names (preferred)
  static const String organizerTeamInvitations = 'organizerTeamInvitations';
  static const String organizerTeamInvitationDetail =
      'organizerTeamInvitationDetail';

  // Verification route names
  static const String verification = 'verification';

  // Report route names
  static const String report = 'report';

  // Saved Items route names
  static const String savedItems = 'savedItems';

  // Seller route names
  static const String sellerDashboard = 'sellerDashboard';
  static const String sellerSettings = 'sellerSettings';
  static const String sellerAnalytics = 'sellerAnalytics';
  // PARKED V1: route name reserved, no screen wired (see RoutePaths.sellerWarnings).
  static const String sellerWarnings = 'sellerWarnings';
  static const String sellerReviews = 'sellerReviews';
  // Seller For Sale management surface (V1)
  static const String sellerForSales = 'sellerForSales';
  static const String sellerUpgrade = 'sellerUpgrade';
  static const String sellerVerification = 'sellerVerification';
  static const String sellerEarnings = 'sellerEarnings';
  static const String sellerBankAccounts = 'sellerBankAccounts';
  static const String sellerPromotions = 'sellerPromotions';
  static const String sellerPromotionDetail = 'sellerPromotionDetail';
  static const String sellerPromotionActivate = 'sellerPromotionActivate';
  static const String sellerExternalProducts = 'sellerExternalProducts';
  static const String sellerExternalProductDetail =
      'sellerExternalProductDetail';

  // Checkout route names
  static const String checkout = 'checkout';
  static const String paymentResult = 'paymentResult';
}
