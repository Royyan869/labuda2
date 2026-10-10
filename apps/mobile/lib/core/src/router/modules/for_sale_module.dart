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
// - `/for-sale/:forSaleId` - For Sale detail page
// - `/create/for-sale` - Create new For Sale
// - `/seller/for-sale` - Seller For Sale management
//
// PUBLIC BROWSE: there is NO standalone `/for-sale` list route. The canonical
// public For Sale browse surface is MarketplaceForSaleTab inside the
// Marketplace screen (bottom-nav tab) — Owner decision: Marketplace is the
// sole public browse destination for For Sale and Auction.
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
// Marketplace (For Sale tab) → ForSaleDetail → Checkout
// ============================================================================

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/for_sale.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/create_for_sale_route_contract.dart';
import 'package:hishumi/core/src/router/route_paths.dart';
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
      pageBuilder: (context, state) => MaterialPage(
        key: state.pageKey,
        name: RouteNames.createForSale,
        child: CreateForSaleScreen(
          // Canonical return-mode contract: chat's direct-commerce attach
          // pushes CreateForSaleRouteArgs.chatDirectCommerce() and consumes
          // a CreatedForSaleResult; other callers get the raw ForSale.
          routeArgs: state.extra is CreateForSaleRouteArgs
              ? state.extra as CreateForSaleRouteArgs
              : null,
        ),
      ),
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
    // No special setup needed - dependencies registered via ForSaleApiDI
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
