import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart'
    show Auction;
import 'package:labuda/domains/commerce/catalog/auction/presentation/create_auction_route_contract.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/seller_auction_edit_screen.dart'
    show SellerAuctionEditScreen;
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/seller_auction_relist_screen.dart'
    show SellerAuctionRelistScreen;
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/seller_auctions_screen.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'base_module.dart';

/// Auction Module - Auction management routes
///
/// Handles all auction-related navigation including:
/// - Auction creation
/// - Auction detail screens
/// - Bidding activity screen
/// - Seller auction management (owner inventory, incl. relist)
class AuctionModule extends BaseModule {
  @override
  String get moduleName => 'AuctionModule';

  @override
  List<GoRoute> get routes => [
    // Create auction route
    GoRoute(
      path: RoutePaths.createAuction,
      name: RouteNames.createAuction,
      pageBuilder: (context, state) => MaterialPage(
        key: state.pageKey,
        name: RouteNames.createAuction,
        child: CreateAuctionScreen(
          routeArgs: state.extra is CreateAuctionRouteArgs
              ? state.extra as CreateAuctionRouteArgs
              : null,
        ),
      ),
    ),

    // Auction detail route
    GoRoute(
      path: RoutePaths.auctionDetails,
      name: RouteNames.auctionDetails,
      builder: (context, state) {
        final auctionId = state.pathParameters['auctionId']!;
        return AuctionDetailScreen(auctionId: auctionId);
      },
    ),

    // Seller auction management surface: the owner's auctions across every
    // status, including draft and ended (relist entry point).
    GoRoute(
      path: RoutePaths.sellerAuctions,
      name: RouteNames.sellerAuctions,
      pageBuilder: (context, state) =>
          MaterialPage(
            key: state.pageKey,
            name: RouteNames.sellerAuctions,
            child: const SellerAuctionsScreen(),
          ),
    ),

    // Owner-only auction forms. The path identifies the auction; the form
    // needs the loaded entity, so it travels as route extra. These are seller
    // management surfaces and are deliberately not externally shareable —
    // without the entity the route lands on the owner's inventory (the
    // canonical parent surface) rather than inventing an error screen.
    GoRoute(
      path: RoutePaths.sellerAuctionEdit,
      name: RouteNames.sellerAuctionEdit,
      builder: (context, state) {
        final auction = state.extra;
        return auction is Auction
            ? SellerAuctionEditScreen(auction: auction)
            : const SellerAuctionsScreen();
      },
    ),
    GoRoute(
      path: RoutePaths.sellerAuctionRelist,
      name: RouteNames.sellerAuctionRelist,
      builder: (context, state) {
        final auction = state.extra;
        return auction is Auction
            ? SellerAuctionRelistScreen(auction: auction)
            : const SellerAuctionsScreen();
      },
    ),
  ];

  @override
  Future<void> initialize() async {
    // Auction module initialized - no special setup needed
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // No cleanup needed for Auction module
  }
}
