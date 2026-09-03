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
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/my_promotions_screen.dart'
    show MyPromotionsScreen;
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/promotion_detail_screen.dart'
    show PromotionDetailScreen;
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/promotion_activation_screen.dart'
    show PromotionActivationScreen;
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/external_product_management_screen.dart'
    show ExternalProductManagementScreen;
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/external_product_detail_screen.dart'
    show ExternalProductDetailScreen;
// STUBBED: seller_stubs.dart imports removed - stub screens disabled in routes
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart'
    show SellerUpgradeWizardScreen;

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
/// /seller/upgrade is always accessible (onboarding entry point).
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

    // Seller Upgrade Route (functional)
    GoRoute(
      path: RoutePaths.sellerUpgrade,
      name: RouteNames.sellerUpgrade,
      builder: (context, state) => const SellerUpgradeWizardScreen(),
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

    GoRoute(
      path: RoutePaths.sellerPromotions,
      name: RouteNames.sellerPromotions,
      builder: (context, state) => const MyPromotionsScreen(),
    ),
    GoRoute(
      path: RoutePaths.sellerPromotionActivate,
      name: RouteNames.sellerPromotionActivate,
      builder: (context, state) {
        final extra = state.extra as Map<String, dynamic>?;
        return PromotionActivationScreen(
          preselectedTargetType: extra?['preselectedTargetType'],
          preselectedTargetId: extra?['preselectedTargetId'],
          preselectedTargetTitle: extra?['preselectedTargetTitle'],
          preselectedOwnershipId: extra?['preselectedOwnershipId'],
          reassignInstanceId: extra?['reassignInstanceId'],
        );
      },
    ),
    GoRoute(
      path: RoutePaths.sellerPromotionDetail,
      name: RouteNames.sellerPromotionDetail,
      builder: (context, state) {
        final instanceId = state.pathParameters['instanceId']!;
        return PromotionDetailScreen(instanceId: instanceId);
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
