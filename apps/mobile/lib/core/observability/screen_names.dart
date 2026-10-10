/// Canonical product screen taxonomy for Labuda mobile.
///
/// A screen name describes the KIND of screen, never a resource identity.
/// Resource identifiers (user / product / auction / order / contract ids) must
/// NEVER appear in a screen name — they are neither meaningful as a product
/// concept nor privacy-safe to ship to a third-party analytics backend.
///
/// This is the ONE authority that converts a GoRouter route identity into a
/// canonical screen name. The router observer consumes it; nothing else
/// derives screen names.
library;

import '../src/router/route_paths.dart';

/// Canonical screen names + route resolution.
abstract final class AnalyticsScreen {
  /// Emitted only when a route cannot be mapped to a known product screen.
  static const String unknown = 'unknown';

  // Onboarding / auth
  static const String splash = 'splash';
  static const String welcome = 'welcome';
  static const String home = 'home';
  static const String signIn = 'sign_in';
  static const String signUp = 'sign_up';
  static const String forgotPassword = 'forgot_password';
  static const String completeProfile = 'complete_profile';
  static const String verifyEmail = 'verify_email';
  static const String accountRestricted = 'account_restricted';

  // Profile
  static const String profile = 'profile';
  static const String userProfile = 'user_profile';
  static const String editProfile = 'edit_profile';
  static const String personalInformation = 'personal_information';
  static const String addresses = 'addresses';
  static const String followList = 'follow_list';

  // Account & safety
  static const String security = 'security';
  static const String loginSessions = 'login_sessions';
  static const String blockedUsers = 'blocked_users';
  static const String myReports = 'my_reports';
  static const String termsOfService = 'terms_of_service';
  static const String privacyPolicy = 'privacy_policy';
  static const String settings = 'settings';
  static const String notificationSettings = 'notification_settings';

  // Help & support
  static const String helpCenter = 'help_center';
  static const String helpCategory = 'help_category';
  static const String helpArticle = 'help_article';
  static const String supportTickets = 'support_tickets';
  static const String supportTicketThread = 'support_ticket_thread';

  // Discovery
  static const String search = 'search';
  static const String searchResults = 'search_results';
  static const String productDetail = 'product_detail';
  static const String auctionDetail = 'auction_detail';
  static const String savedItems = 'saved_items';
  static const String notifications = 'notifications';

  // Content / social
  static const String contentDetail = 'content_detail';
  static const String discussion = 'discussion';
  static const String createContent = 'create_content';
  static const String chat = 'chat';
  static const String newChat = 'new_chat';
  static const String chatConversation = 'chat_conversation';

  // Commerce
  static const String checkout = 'checkout';
  static const String payment = 'payment';
  static const String paymentResult = 'payment_result';
  static const String orderList = 'order_list';
  static const String orderDetail = 'order_detail';
  static const String coins = 'coins';
  static const String coinsHistory = 'coins_history';
  static const String report = 'report';

  // Seller
  static const String sellerDashboard = 'seller_dashboard';
  static const String sellerOrders = 'seller_orders';
  static const String sellerWarnings = 'seller_warnings';
  static const String sellerForSales = 'seller_for_sales';
  static const String sellerAuctions = 'seller_auctions';
  static const String sellerAuctionEdit = 'seller_auction_edit';
  static const String sellerAuctionRelist = 'seller_auction_relist';
  static const String createForSale = 'create_for_sale';
  static const String createAuction = 'create_auction';
  static const String sellerUpgrade = 'seller_upgrade';
  static const String sellerRenewal = 'seller_renewal';
  static const String sellerVerification = 'seller_verification';
  static const String sellerEarnings = 'seller_earnings';
  static const String sellerAnalytics = 'seller_analytics';
  static const String sellerPerformance = 'seller_performance';
  static const String sellerShipping = 'seller_shipping';
  static const String sellerBankAccounts = 'seller_bank_accounts';
  static const String sellerDiscounts = 'seller_discounts';
  static const String sellerDiscountCreate = 'seller_discount_create';
  static const String sellerDiscountEdit = 'seller_discount_edit';
  static const String sellerExternalProducts = 'seller_external_products';
  static const String sellerExternalProductDetail =
      'seller_external_product_detail';
  static const String promotionList = 'promotion_list';
  static const String promotionAnalytics = 'promotion_analytics';

  /// Canonical GoRouter route NAME -> screen name.
  ///
  /// Route names are the preferred identity: they are static and carry no
  /// dynamic resource segment. Every [RouteNames] constant MUST be present;
  /// this is locked by `test/core/observability/screen_name_taxonomy_test.dart`.
  static const Map<String, String> byRouteName = <String, String>{
    RouteNames.splash: splash,
    RouteNames.welcome: welcome,
    RouteNames.home: home,
    RouteNames.signIn: signIn,
    RouteNames.signUp: signUp,
    RouteNames.forgotPassword: forgotPassword,
    RouteNames.completeProfile: completeProfile,
    RouteNames.verifyEmail: verifyEmail,
    RouteNames.accountRestricted: accountRestricted,
    RouteNames.profile: profile,
    RouteNames.userProfile: userProfile,
    RouteNames.editProfile: editProfile,
    RouteNames.personalInformation: personalInformation,
    RouteNames.addresses: addresses,
    RouteNames.followList: followList,
    RouteNames.security: security,
    RouteNames.loginSessions: loginSessions,
    RouteNames.blockedUsers: blockedUsers,
    RouteNames.myReports: myReports,
    RouteNames.termsOfService: termsOfService,
    RouteNames.privacyPolicy: privacyPolicy,
    RouteNames.settings: settings,
    RouteNames.helpCenter: helpCenter,
    RouteNames.helpCategory: helpCategory,
    RouteNames.helpArticle: helpArticle,
    RouteNames.supportTickets: supportTickets,
    RouteNames.supportTicketThread: supportTicketThread,
    RouteNames.forSaleDetail: productDetail,
    RouteNames.createForSale: createForSale,
    RouteNames.auctionDetails: auctionDetail,
    RouteNames.chat: chat,
    RouteNames.newChat: newChat,
    RouteNames.chatConversation: chatConversation,
    RouteNames.notifications: notifications,
    RouteNames.search: search,
    RouteNames.searchResults: searchResults,
    RouteNames.createAuction: createAuction,
    RouteNames.coins: coins,
    RouteNames.coinsHistory: coinsHistory,
    RouteNames.report: report,
    RouteNames.savedItems: savedItems,
    RouteNames.sellerDashboard: sellerDashboard,
    RouteNames.sellerForSales: sellerForSales,
    RouteNames.sellerAuctions: sellerAuctions,
    RouteNames.sellerUpgrade: sellerUpgrade,
    RouteNames.sellerRenewal: sellerRenewal,
    RouteNames.sellerVerification: sellerVerification,
    RouteNames.sellerEarnings: sellerEarnings,
    RouteNames.sellerAnalytics: sellerAnalytics,
    RouteNames.sellerPerformance: sellerPerformance,
    RouteNames.sellerBankAccounts: sellerBankAccounts,
    RouteNames.sellerWarnings: sellerWarnings,
    RouteNames.sellerExternalProducts: sellerExternalProducts,
    RouteNames.sellerDiscounts: sellerDiscounts,
    RouteNames.sellerDiscountCreate: sellerDiscountCreate,
    RouteNames.sellerDiscountEdit: sellerDiscountEdit,
    RouteNames.sellerAuctionEdit: sellerAuctionEdit,
    RouteNames.sellerAuctionRelist: sellerAuctionRelist,
    RouteNames.sellerExternalProductDetail: sellerExternalProductDetail,
    RouteNames.sellerCanonicalPromotionAnalytics: promotionAnalytics,
    RouteNames.sellerCanonicalPromotions: promotionList,
    RouteNames.checkout: checkout,
    RouteNames.paymentResult: paymentResult,
    RouteNames.paymentWebview: payment,
    // Content module registers its create route under a bare literal name.
    'create-content': createContent,
    // Notification settings registers under a bare literal name.
    'notificationSettings': notificationSettings,
  };

  /// Path-template -> screen name, used only as a defensive fallback when a
  /// route arrives without a canonical name. Order matters: more specific
  /// templates must precede less specific ones.
  static const List<(String, String)> _byPath = <(String, String)>[
    (RoutePaths.forSaleDetail, productDetail),
    (RoutePaths.createForSale, createForSale),
    (RoutePaths.createContent, createContent),
    (RoutePaths.auctionDetails, auctionDetail),
    (RoutePaths.createAuction, createAuction),
    (RoutePaths.chatConversation, chatConversation),
    (RoutePaths.newChat, newChat),
    (RoutePaths.chat, chat),
    (RoutePaths.searchResults, searchResults),
    (RoutePaths.search, search),
    (RoutePaths.notifications, notifications),
    (RoutePaths.savedItems, savedItems),
    (RoutePaths.coinsHistory, coinsHistory),
    (RoutePaths.coins, coins),
    (RoutePaths.report, report),
    (RoutePaths.checkout, checkout),
    (RoutePaths.paymentResult, paymentResult),
    (RoutePaths.paymentWebview, payment),
    (RoutePaths.userProfile, userProfile),
    (RoutePaths.followList, followList),
    (RoutePaths.editProfile, editProfile),
    (RoutePaths.personalInformation, personalInformation),
    (RoutePaths.addresses, addresses),
    (RoutePaths.profile, profile),
    (RoutePaths.security, security),
    (RoutePaths.loginSessions, loginSessions),
    (RoutePaths.blockedUsers, blockedUsers),
    (RoutePaths.myReports, myReports),
    (RoutePaths.termsOfService, termsOfService),
    (RoutePaths.privacyPolicy, privacyPolicy),
    (RoutePaths.settings, settings),
    (RoutePaths.helpArticle, helpArticle),
    (RoutePaths.helpCategory, helpCategory),
    (RoutePaths.helpCenter, helpCenter),
    (RoutePaths.supportTicketThread, supportTicketThread),
    (RoutePaths.supportTickets, supportTickets),
    (RoutePaths.sellerAuctionRelist, sellerAuctionRelist),
    (RoutePaths.sellerAuctionEdit, sellerAuctionEdit),
    (RoutePaths.sellerCanonicalPromotionAnalytics, promotionAnalytics),
    (RoutePaths.sellerExternalProductDetail, sellerExternalProductDetail),
    (RoutePaths.sellerExternalProducts, sellerExternalProducts),
    (RoutePaths.sellerCanonicalPromotions, promotionList),
    (RoutePaths.sellerDiscountEdit, sellerDiscountEdit),
    (RoutePaths.sellerDiscountCreate, sellerDiscountCreate),
    (RoutePaths.sellerDiscounts, sellerDiscounts),
    (RoutePaths.sellerShippingSetupCityRules, sellerShipping),
    (RoutePaths.sellerShippingSetup, sellerShipping),
    (RoutePaths.sellerShipping, sellerShipping),
    (RoutePaths.sellerPerformance, sellerPerformance),
    (RoutePaths.sellerAnalytics, sellerAnalytics),
    (RoutePaths.sellerEarnings, sellerEarnings),
    (RoutePaths.sellerBankAccounts, sellerBankAccounts),
    (RoutePaths.sellerVerification, sellerVerification),
    (RoutePaths.sellerRenewal, sellerRenewal),
    (RoutePaths.sellerUpgrade, sellerUpgrade),
    (RoutePaths.sellerAuctions, sellerAuctions),
    (RoutePaths.sellerForSales, sellerForSales),
    (RoutePaths.sellerOrders, sellerOrders),
    (RoutePaths.sellerDashboard, sellerDashboard),
  ];

  /// Resolve a canonical screen name from a route identifier.
  ///
  /// Accepts either a GoRouter route NAME (preferred) or a location path.
  /// Returns [unknown] when no canonical screen can be determined. The result
  /// is always a static product concept — never a resource identifier.
  static String resolve(String? identifier) {
    if (identifier == null || identifier.isEmpty) return unknown;

    final byName = byRouteName[identifier];
    if (byName != null) return byName;

    // Defensive path fallback.
    final uri = Uri.tryParse(identifier);
    final path = (uri != null && uri.hasScheme == false)
        ? uri.path
        : identifier;
    final segments = path
        .split('/')
        .where((segment) => segment.isNotEmpty)
        .toList(growable: false);
    if (segments.isEmpty) return unknown;

    for (final (template, screenName) in _byPath) {
      if (_matchesTemplate(template, segments)) return screenName;
    }
    return unknown;
  }

  static bool _matchesTemplate(String template, List<String> pathSegments) {
    final templateSegments = template
        .split('/')
        .where((segment) => segment.isNotEmpty)
        .toList(growable: false);
    if (templateSegments.length != pathSegments.length) return false;
    for (var i = 0; i < templateSegments.length; i++) {
      final expected = templateSegments[i];
      if (expected.startsWith(':')) continue;
      if (expected != pathSegments[i]) return false;
    }
    return true;
  }
}
