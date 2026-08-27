import 'package:labuda/core/src/router/modules/base_module.dart';
import 'package:labuda/core/src/router/modules/onboarding_module.dart';
import 'package:labuda/core/src/router/modules/auth_module.dart';
import 'package:labuda/core/src/router/modules/home_module.dart';
import 'package:labuda/core/src/router/modules/profile_module.dart';
import 'package:labuda/core/src/router/modules/chat_module.dart';
import 'package:labuda/core/src/router/modules/content_module.dart';
import 'package:labuda/core/src/router/modules/auction_module.dart';
import 'package:labuda/core/src/router/modules/order_module.dart';
import 'package:labuda/core/src/router/modules/coins_module.dart';
import 'package:labuda/core/src/router/modules/verification_module.dart';
import 'package:labuda/core/src/router/modules/search_module.dart';
import 'package:labuda/core/src/router/modules/report_module.dart';
import 'package:labuda/core/src/router/modules/support_module.dart';
import 'package:labuda/core/src/router/modules/saved_item_module.dart';
import 'package:labuda/core/src/router/modules/seller_module.dart';
import 'package:labuda/core/src/router/modules/for_sale_module.dart';
import 'package:labuda/domains/commerce/transaction/checkout/presentation/checkout_router_module.dart';
import 'package:go_router/go_router.dart';

class RouterModulesManager {
  /// List module yang digunakan aplikasi
  ///
  /// Setiap module bertanggung jawab atas routes dan dependencies mereka sendiri.
  /// Untuk menambah module baru, tambahkan instance ke list ini.
  final List<BaseModule> _modules = [
    OnboardingModule(), // Splash & Welcome screens
    AuthModule(), // Authentication related routes
    HomeModule(), // Home/Dashboard routes
    ProfileModule(), // Profile related routes (includes seller dashboard)
    ContentModule(), // Social Content (universal composer and detail)
    ChatModule(), // Chat & messaging functionality
    AuctionModule(), // Auction management
    OrderModule(), // Order management (buyer & seller)
    CoinsModule(), // Labuda Coins (loyalty points system)
    VerificationModule(), // KYC, Business, and Seller verification
    SearchModule(), // Global & hybrid search functionality
    ReportModule(), // User reports & appeals
    SupportModule(), // Customer support & help
    SavedItemModule(), // Unified saved items (saved listings + watched auctions)
    SellerModule(), // Seller dashboard & management
    ForSaleModule(), // For Sale fixed-price commerce
    CheckoutModule(), // Direct buy checkout flow
  ];

  /// Initialize all modules
  Future<void> initializeModules() async {
    // Initialize all modules and register their dependencies
    for (final module in _modules) {
      await module.initialize();
    }
  }

  /// Build consolidated routes from all modules
  List<GoRoute> buildRoutes() {
    final allRoutes = <GoRoute>[];

    // Collect routes from all modules
    for (final module in _modules) {
      final moduleRoutes = module.routes;
      allRoutes.addAll(moduleRoutes);
    }

    return allRoutes;
  }

  /// Dispose all modules
  void dispose() {
    for (final module in _modules) {
      module.dispose();
    }
  }
}
