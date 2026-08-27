import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/bidding_screen.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'base_module.dart';

/// Auction Module - Auction management routes
///
/// Handles all auction-related navigation including:
/// - Auction creation
/// - Auction detail screens
/// - Bidding activity screen
class AuctionModule extends BaseModule {
  @override
  String get moduleName => 'AuctionModule';

  @override
  List<GoRoute> get routes => [
    // Create auction route
    GoRoute(
      path: RoutePaths.createAuction,
      name: RouteNames.createAuction,
      pageBuilder: (context, state) =>
          MaterialPage(key: state.pageKey, child: const CreateAuctionScreen()),
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

    // Bidding screen route
    GoRoute(
      path: RoutePaths.bidding,
      name: RouteNames.bidding,
      pageBuilder: (context, state) =>
          MaterialPage(key: state.pageKey, child: const BiddingScreen()),
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
