import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/user/preference/saved_item/saved_item.dart';
import 'package:labuda/core/src/router/modules/base_module.dart';

/// Saved Item module routing implementation
///
/// SAVED ITEM DOMAIN:
///
/// This module handles routing for the unified Saved Items (shortlist + auction watches).
///
/// Purpose: Unified stream of saved items (listings + auctions)
/// Screen: SavedItemScreen
/// Icon: Bookmark
/// Tooltip: "Disimpan" (Saved)
///
/// REPLACES: ShortlistModule + WatchlistModule (NO DUAL SYSTEM)
class SavedItemModule extends BaseModule {
  @override
  String get moduleName => 'SavedItemModule';

  @override
  List<GoRoute> get routes => [
    GoRoute(
      path: RoutePaths.savedItems,
      name: RouteNames.savedItems,
      builder: (context, state) => const SavedItemScreen(),
    ),
  ];

  @override
  Future<void> initialize() async {
    // Saved item module initialized
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {}
}
