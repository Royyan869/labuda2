import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart'
    show OrderListScreen;
import 'package:hishumi/domains/user/profile/presentation/screens/bank_account_screen.dart'
    show BankAccountScreen;
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart'
    show SellerDashboardScreen;
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_earnings_screen.dart'
    show SellerEarningsScreen;
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_analytics_screen.dart'
    show SellerAnalyticsScreen;
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_performance_screen.dart'
    show SellerPerformanceScreen;
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_verification_screen.dart'
    show SellerVerificationScreen;
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart'
    show SellerShippingScreen;
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/widgets/shipping_option_setup_screen.dart'
    show ShippingSetupScreen, ShippingCityRulesScreen, ShippingCityRulesRouteArgs;
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_analytics_screen.dart'
    show CanonicalPromotionAnalyticsScreen;
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_create_screen.dart'
    show CanonicalPromotionCreateScreen;
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_list_screen.dart'
    show CanonicalPromotionListScreen;
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_queue_screen.dart'
    show CanonicalPromotionQueueScreen;
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/screens/external_product_management_screen.dart'
    show ExternalProductManagementScreen;
import 'package:hishumi/domains/commerce/pricing/promotion/presentation/screens/external_product_detail_screen.dart'
    show ExternalProductDetailScreen;
// STUBBED: seller_stubs.dart imports removed - stub screens disabled in routes
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart'
    show SellerUpgradeWizardScreen;
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_renewal_screen.dart'
    show SellerRenewalScreen;
import 'package:hishumi/domains/commerce/pricing/discount/domain/entities/discount_entity.dart'
    show Discount;
import 'package:hishumi/domains/commerce/pricing/discount/presentation/screens/create_discount_screen.dart'
    show CreateDiscountScreen;
import 'package:hishumi/domains/commerce/pricing/discount/presentation/screens/edit_discount_screen.dart'
    show EditDiscountScreen;
import 'package:hishumi/domains/commerce/pricing/discount/presentation/screens/seller_discount_list_screen.dart'
    show SellerDiscountListScreen;

import 'base_module.dart';

/// Seller Module
///
/// Mengelola semua routes yang berkaitan dengan seller dashboard:
/// - Seller Dashboard
/// - Seller Orders
/// - Seller Earnings
/// - Seller Verification (required for withdrawal)
/// - Seller Upgrade
/// - Seller Shipping
/// - Seller Bank Accounts
/// - Seller Promotions & External Products
///
/// Module ini accessible oleh users dengan seller profile (hasCreatedSellerProfile).
/// /seller/upgrade is always accessible (REGISTRATION entry point for
/// non-sellers; existing sellers are gated inside the wizard itself).
/// /seller/renewal is the payment-only RENEWAL lifecycle for existing sellers.
/// Route guards: router-level (app_router.dart) + screen-level (auth check in build method).
class SellerModule extends BaseModule {
  @override
  String get moduleName => 'SellerModule';

  @override
  List<GoRoute> get routes => [
    // Seller Dashboard Route
    GoRoute(
      path: RoutePaths.sellerDashboard,
      name: RouteNames.sellerDashboard,
      builder: (context, state) => const SellerDashboardScreen(),
    ),

    // Seller Orders Route (uses OrderListScreen with isSeller=true)
    GoRoute(
      path: '/seller/orders',
      name: 'sellerOrders',
      builder: (context, state) => const OrderListScreen(isSeller: true),
    ),

    // Seller Earnings Route (REAL implementation - uses backend API)
    GoRoute(
      path: '/seller/earnings',
      name: 'sellerEarnings',
      builder: (context, state) => const SellerEarningsScreen(),
    ),

    // Seller Analytics Route (30-day read projection over Product View + sales)
    GoRoute(
      path: RoutePaths.sellerAnalytics,
      name: RouteNames.sellerAnalytics,
      builder: (context, state) => const SellerAnalyticsScreen(),
    ),

    // Seller Performance Route (canonical Reputation + Rating projection)
    GoRoute(
      path: RoutePaths.sellerPerformance,
      name: RouteNames.sellerPerformance,
      builder: (context, state) => const SellerPerformanceScreen(),
    ),

    // Seller Upgrade Route (REGISTRATION lifecycle only — never renewal)
    GoRoute(
      path: RoutePaths.sellerUpgrade,
      name: RouteNames.sellerUpgrade,
      builder: (context, state) => const SellerUpgradeWizardScreen(),
    ),

    // Seller Renewal Route (canonical payment-only RENEWAL lifecycle)
    GoRoute(
      path: RoutePaths.sellerRenewal,
      name: RouteNames.sellerRenewal,
      builder: (context, state) => const SellerRenewalScreen(),
    ),

    // Seller Verification Route (REAL implementation - required for withdrawal)
    GoRoute(
      path: RoutePaths.sellerVerification,
      name: 'sellerVerification',
      builder: (context, state) => const SellerVerificationScreen(),
    ),

    // Seller global shipping option list (canonical one-package catalog).
    GoRoute(
      path: RoutePaths.sellerShipping,
      name: 'sellerShipping',
      builder: (context, state) => const SellerShippingScreen(),
    ),

    // ONE-PACKAGE shipping setup: identity (type, name, seller-private note)
    // + destinations (provinces with all-in shipping+packing rates and city
    // qualifications) are authored and saved as ONE unit. Bare options
    // without destinations can never be persisted.
    GoRoute(
      path: RoutePaths.sellerShippingSetup,
      name: 'sellerShippingSetup',
      builder: (context, state) {
        final extra = state.extra;
        // Canonical edit path: extra is the option ID; the screen fetches
        // the full package from the backend detail endpoint.
        return ShippingSetupScreen(
          editOptionId: extra is String ? extra : null,
        );
      },
    ),

    // City-level qualification editor for one province inside the setup flow.
    GoRoute(
      path: RoutePaths.sellerShippingSetupCityRules,
      name: 'sellerShippingSetupCityRules',
      builder: (context, state) {
        final args = state.extra! as ShippingCityRulesRouteArgs;
        return ShippingCityRulesScreen(args: args);
      },
    ),

    // Bank Account Management Route (C6.2: seller self-service)
    // Protected by /seller prefix → router-level seller guard in app_router.dart
    GoRoute(
      path: RoutePaths.sellerBankAccounts,
      name: RouteNames.sellerBankAccounts,
      builder: (context, state) => const BankAccountScreen(),
    ),

    // Canonical Promotion Management List Route
    GoRoute(
      path: RoutePaths.sellerCanonicalPromotions,
      name: RouteNames.sellerCanonicalPromotions,
      builder: (context, state) => const CanonicalPromotionListScreen(),
    ),
    GoRoute(
      path: RoutePaths.sellerPromotionContractCreate,
      name: 'sellerPromotionContractCreate',
      builder: (context, state) => const CanonicalPromotionCreateScreen(),
    ),
    // Canonical promotion product-queue (refill) management.
    GoRoute(
      path: RoutePaths.sellerCanonicalPromotionQueue,
      name: 'sellerCanonicalPromotionQueue',
      builder: (context, state) {
        final contractId = state.pathParameters['contractId']!;
        final kind = state.uri.queryParameters['kind'] ?? 'internal';
        return CanonicalPromotionQueueScreen(
          contractId: contractId,
          kind: kind,
        );
      },
    ),
    // Canonical Promotion Analytics Route
    GoRoute(
      path: RoutePaths.sellerCanonicalPromotionAnalytics,
      name: RouteNames.sellerCanonicalPromotionAnalytics,
      builder: (context, state) {
        final contractId = state.pathParameters['contractId']!;
        return CanonicalPromotionAnalyticsScreen(contractId: contractId);
      },
    ),
    GoRoute(
      path: RoutePaths.sellerExternalProducts,
      name: RouteNames.sellerExternalProducts,
      builder: (context, state) => const ExternalProductManagementScreen(),
    ),
    GoRoute(
      path: RoutePaths.sellerExternalProductDetail,
      name: RouteNames.sellerExternalProductDetail,
      builder: (context, state) {
        final productId = state.pathParameters['productId']!;
        return ExternalProductDetailScreen(productId: productId);
      },
    ),

    // ── Seller discount management ────────────────────────────────────────
    // Owner-only management surfaces (create/edit). The edit route carries
    // the discount entity as route extra: the form is not an externally
    // shareable destination, so a missing entity lands on the canonical
    // discount list instead of inventing an error screen.
    GoRoute(
      path: RoutePaths.sellerDiscounts,
      name: RouteNames.sellerDiscounts,
      builder: (context, state) => const SellerDiscountListScreen(),
    ),
    GoRoute(
      path: RoutePaths.sellerDiscountCreate,
      name: RouteNames.sellerDiscountCreate,
      builder: (context, state) => const CreateDiscountScreen(),
    ),
    GoRoute(
      path: RoutePaths.sellerDiscountEdit,
      name: RouteNames.sellerDiscountEdit,
      builder: (context, state) {
        final discount = state.extra;
        return discount is Discount
            ? EditDiscountScreen(discount: discount)
            : const SellerDiscountListScreen();
      },
    ),
  ];

  @override
  Future<void> initialize() async {
    // Register seller-related services if needed
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // Cleanup seller module resources
  }
}
