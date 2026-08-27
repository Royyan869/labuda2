import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/finance/wallet/coins/coins.dart';
import 'base_module.dart';

/// Coins module routing implementation
///
/// IMPORTANT: Coins are LOYALTY POINTS, NOT money.
///
/// Handles routing for:
/// - Coin balance screen
/// - Transaction history
class CoinsModule extends BaseModule {
  @override
  String get moduleName => 'CoinsModule';

  @override
  List<GoRoute> get routes => [
    // Coin Balance - Main coins screen
    GoRoute(
      path: RoutePaths.coins,
      name: RouteNames.coins,
      builder: (context, state) => const CoinBalanceScreen(),
    ),

    // Transaction History
    GoRoute(
      path: RoutePaths.coinsHistory,
      name: RouteNames.coinsHistory,
      builder: (context, state) {
        final userId = state.uri.queryParameters['userId'] ?? '';
        return CoinHistoryScreen(userId: userId);
      },
    ),
  ];

  @override
  Future<void> initialize() async {
    // Coins module initialized - routes registered
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // No cleanup needed for coins routes
  }
}
