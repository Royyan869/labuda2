// ============================================================================
// FOR SALE ROUTER MODULE - fixed-price sale routes
// ============================================================================
//
// This module connects to the backend For Sale API directly
// (/api/v1/for-sale). Auction is a sibling sale channel over the same
// Product with its own separate module (auction_module.dart) — For Sale is
// not a parent of Auction and this is not the only commerce entry point.
//
// Routes:
// - `/for-sale` - For Sale catalog
// - `/for-sale/:forSaleId` - For Sale detail page
// - `/create/for-sale` - Create new For Sale
//
// ## Architecture:
// ```
// ForSaleRemoteDatasource → /api/v1/for-sale (backend)
//           ↓
// ForSaleRepositoryImpl → ForSale entity
//           ↓
// ForSaleController → UI Screens
// ```
//
// ## Flow:
// Explore → CreateForSale → ForSales → ForSaleDetail → Checkout
// ============================================================================

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/for_sale.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'base_module.dart';

/// ForSale Module — fixed-price sale routes.
///
/// A sibling of AuctionModule, not a parent of it.
class ForSaleModule extends BaseModule {
  @override
  String get moduleName => 'ForSaleModule';

  @override
  List<GoRoute> get routes => [
    // ============================================================================
    // FOR SALE CATALOG
    // ============================================================================
    GoRoute(
      path: RoutePaths.forSales,
      name: RouteNames.forSales,
      builder: (context, state) => const ForSaleListScreen(),
    ),

    // ============================================================================
    // FOR SALE DETAIL PAGE
    // ============================================================================
    GoRoute(
      path: RoutePaths.forSaleDetail,
      name: RouteNames.forSaleDetail,
      builder: (context, state) {
        final forSaleId = state.pathParameters['forSaleId']!;
        return ForSaleDetailScreen(forSaleId: forSaleId);
      },
    ),

    // ============================================================================
    // CREATE FOR SALE - Seller creates new For Sale
    // ============================================================================
    GoRoute(
      path: RoutePaths.createForSale,
      name: RouteNames.createForSale,
      pageBuilder: (context, state) =>
          MaterialPage(key: state.pageKey, child: const CreateForSaleScreen()),
    ),

    // ============================================================================
    // EDIT FOR SALE - Seller edits their existing For Sale
    // ============================================================================
    GoRoute(
      path: RoutePaths.editForSale,
      name: RouteNames.editForSale,
      builder: (context, state) {
        final forSaleId = state.pathParameters['forSaleId']!;
        return EditForSaleScreen(forSaleId: forSaleId);
      },
    ),

    // ============================================================================
    // MY FOR SALES - Seller's For Sale management surface (V1)
    // ============================================================================
    GoRoute(
      path: RoutePaths.sellerForSales,
      name: RouteNames.sellerForSales,
      builder: (context, state) => const MyForSalesScreen(),
    ),
  ];

  @override
  Future<void> initialize() async {
    // No special setup needed - dependencies registered via ListingApiDI
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // No cleanup needed
  }
}
