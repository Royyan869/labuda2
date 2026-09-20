import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart'
    show OrderListScreen;
import 'package:labuda/domains/user/profile/presentation/screens/bank_account_screen.dart'
    show BankAccountScreen;
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart'
    show SellerDashboardScreen;
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_earnings_screen.dart'
    show SellerEarningsScreen;
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_verification_screen.dart'
    show SellerVerificationScreen;
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart'
    show SellerShippingScreen;
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_shipping_option_detail_screen.dart'
    show SellerShippingSetupDetailScreen;
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_analytics_screen.dart'
    show CanonicalPromotionAnalyticsScreen;
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_create_screen.dart'
    show CanonicalPromotionCreateScreen;
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_list_screen.dart'
    show CanonicalPromotionListScreen;
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/external_product_management_screen.dart'
    show ExternalProductManagementScreen;
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/external_product_detail_screen.dart'
    show ExternalProductDetailScreen;
// STUBBED: seller_stubs.dart imports removed - stub screens disabled in routes
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart'
    show SellerUpgradeWizardScreen;
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_renewal_screen.dart'
    show SellerRenewalScreen;

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

    // Phase 1: Seller global shipping setup (option list + create/edit/delete/toggle)
    GoRoute(
      path: RoutePaths.sellerShipping,
      name: 'sellerShipping',
      builder: (context, state) => const SellerShippingScreen(),
    ),

    // Phase 1: Per-option coverage (province-level rates) management
    GoRoute(
      path: RoutePaths.sellerShippingSetupDetail,
      name: 'sellerShippingSetupDetail',
      builder: (context, state) {
        final optionId = state.pathParameters['optionId']!;
        return SellerShippingSetupDetailScreen(optionId: optionId);
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
