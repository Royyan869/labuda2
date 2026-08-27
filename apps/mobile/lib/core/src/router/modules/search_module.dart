import 'package:go_router/go_router.dart';

import '../route_paths.dart';
import 'base_module.dart';

// Import Search screens from search_refactor module
import 'package:labuda/features/search/search/search.dart';

/// Search module routing implementation
///
/// Handles routing for:
/// - Main search screen with suggestions and history
/// - Search results screen with type filtering
class SearchModule extends BaseModule {
  @override
  String get moduleName => 'SearchModule';

  @override
  List<GoRoute> get routes => [
    // Main Search Screen
    GoRoute(
      path: RoutePaths.search,
      name: RouteNames.search,
      builder: (context, state) {
        return const SearchScreen();
      },
    ),

    // Search Results Screen
    GoRoute(
      path: RoutePaths.searchResults,
      name: RouteNames.searchResults,
      builder: (context, state) {
        final query = state.uri.queryParameters['q'] ?? '';
        final type = state.uri.queryParameters['type'];
        return SearchResultsScreen(
          query: query,
          initialType: type != null ? _parseSearchType(type) : null,
        );
      },
    ),
  ];

  @override
  Future<void> initialize() async {
    // search_refactor uses Riverpod providers - no manual DI needed
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // Riverpod handles cleanup automatically
  }

  SearchResultType? _parseSearchType(String type) {
    switch (type.toLowerCase()) {
      case 'user':
        return SearchResultType.user;
      case 'listing':
        return SearchResultType.listing;
      case 'auction':
        return SearchResultType.auction;
      case 'content':
        return SearchResultType.content;
      default:
        return null;
    }
  }
}
