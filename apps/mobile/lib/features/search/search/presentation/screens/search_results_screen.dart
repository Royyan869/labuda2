import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/presentation/providers/search_notifier.dart';
import 'package:labuda/features/search/search/presentation/providers/search_state.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/presentation/utils/search_result_type_helper.dart';
import 'package:labuda/features/search/search/presentation/widgets/global_search_bar.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_result_item.dart';
import 'package:labuda/shared/widgets/external_link_interstitial.dart';
import 'package:visibility_detector/visibility_detector.dart';

// Session-level dedupe set for search promotion impressions.
// Module-level Set — persists for the app session, resets on restart.
// Independent from feed dedupe — same instance in both surfaces records both.
final _searchImpressionSeen = <String>{};

// Fire-and-forget impression helper for search promoted results.
// Records at most once per instance per session; errors silently ignored.
void _recordSearchImpression(WidgetRef ref, String instanceId) {
  if (instanceId.isEmpty) return;
  if (_searchImpressionSeen.contains(instanceId)) return;
  _searchImpressionSeen.add(instanceId);
  () async {
    try {
      await ref
          .read(apiClientProvider)
          .post(
            '/promotions/events',
            data: {
              'promotion_instance_id': instanceId,
              'event_type': 'impression',
              'surface': 'search',
            },
          );
    } catch (_) {}
  }();
}

/// Screen displaying search results with tabs for different types
class SearchResultsScreen extends ConsumerStatefulWidget {
  final String query;
  final SearchResultType? initialType;

  const SearchResultsScreen({super.key, required this.query, this.initialType});

  @override
  ConsumerState<SearchResultsScreen> createState() =>
      _SearchResultsScreenState();
}

class _SearchResultsScreenState extends ConsumerState<SearchResultsScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  late String _currentQuery;

  final _tabs = const [
    Tab(text: 'All'),
    Tab(text: 'Listings'),
    Tab(text: 'Auctions'),
    Tab(text: 'User'),
    Tab(text: 'Content'),
  ];

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: _tabs.length, vsync: this);
    _currentQuery = widget.query;

    // Set initial tab based on type
    if (widget.initialType != null) {
      _tabController.index = SearchResultTypeHelper.getTabIndex(
        widget.initialType,
      );
      ref.read(searchProvider.notifier).setSelectedType(widget.initialType);
    }

    _tabController.addListener(_onTabChanged);

    // Execute initial search
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _executeSearch();
    });
  }

  @override
  void dispose() {
    _tabController.removeListener(_onTabChanged);
    _tabController.dispose();
    super.dispose();
  }

  void _onTabChanged() {
    if (!_tabController.indexIsChanging) {
      final type = SearchResultTypeHelper.getTypeFromIndex(
        _tabController.index,
      );
      ref.read(searchProvider.notifier).setSelectedType(type);
    }
  }

  void _executeSearch() {
    ref.read(searchProvider.notifier).searchAll(_currentQuery);
  }

  void _onSearch(String query) {
    setState(() {
      _currentQuery = query;
    });
    _executeSearch();
  }

  void _onResultTap(SearchResult result) {
    handleSearchResultTap(context, ref, result);
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final searchState = ref.watch(searchProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Search Results'),
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        elevation: 0,
        bottom: TabBar(
          controller: _tabController,
          isScrollable: true,
          labelColor: AppColors.primary,
          unselectedLabelColor: isDark
              ? AppColors.neutralGray400
              : AppColors.neutralGray500,
          indicatorColor: AppColors.primary,
          tabs: _tabs,
        ),
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: GlobalSearchBar(
              initialQuery: _currentQuery,
              onSearch: _onSearch,
              showCategoryChips: false,
            ),
          ),
          Expanded(
            child: searchState.isSearching
                ? _buildLoading()
                : _buildResults(searchState),
          ),
        ],
      ),
    );
  }

  Widget _buildLoading() {
    return const Center(child: CircularProgressIndicator());
  }

  Widget _buildResults(SearchState state) {
    if (state.error != null) {
      return _buildError(state.error!);
    }

    final results = state.displayResults;

    if (results.isEmpty) {
      return _buildEmptyState();
    }

    return ListView.separated(
      padding: const EdgeInsets.symmetric(vertical: 8),
      itemCount: results.length,
      separatorBuilder: (_, _) => Divider(
        height: 1,
        color: Theme.of(context).brightness == Brightness.dark
            ? AppColors.darkGray600
            : AppColors.neutralGray200,
      ),
      itemBuilder: (context, index) {
        final result = results[index];
        final item = SearchResultItem(
          result: result,
          onTap: () => _onResultTap(result),
        );
        final instanceId = result.promotionInstanceId;
        if (!result.isPromoted || instanceId == null || instanceId.isEmpty) {
          return item;
        }
        return VisibilityDetector(
          key: Key('search_promo_imp_$instanceId'),
          onVisibilityChanged: (info) {
            if (info.visibleFraction >= 0.5) {
              _recordSearchImpression(ref, instanceId);
            }
          },
          child: item,
        );
      },
    );
  }

  Widget _buildError(String error) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 64, color: AppColors.primaryRed),
            const SizedBox(height: 16),
            Text(
              error,
              style: TextStyle(
                fontSize: 16,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: _executeSearch,
              child: const Text('Try Again'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildEmptyState() {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.search_off,
              size: 64,
              color: isDark
                  ? AppColors.neutralGray600
                  : AppColors.neutralGray300,
            ),
            const SizedBox(height: 16),
            Text(
              'No results found',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w600,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Try different keywords or different filters',
              style: TextStyle(
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray500,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

Future<void> handleSearchResultTap(
  BuildContext context,
  WidgetRef ref,
  SearchResult result,
) async {
  // Fire-and-forget promotion click tracking for promoted search results.
  final instanceId = result.promotionInstanceId;
  if (result.isPromoted && instanceId != null && instanceId.isNotEmpty) {
    () async {
      try {
        await ref
            .read(apiClientProvider)
            .post(
              '/promotions/events',
              data: {
                'promotion_instance_id': instanceId,
                'event_type': 'click',
                'surface': 'search',
              },
            );
      } catch (_) {}
    }();
  }

  final navHandler = ref.read(navigationHandlerProvider);

  switch (result.type) {
    case SearchResultType.user:
      navHandler.navigateToUserProfile(result.id);
      return;
    case SearchResultType.listing:
      navHandler.navigateToForSaleDetail(result.id);
      return;
    case SearchResultType.externalProduct:
      final url = result.metadata['externalUrl'] as String?;
      if (url != null && context.mounted) {
        await showExternalLinkInterstitial(context, url: url);
      }
      return;
    case SearchResultType.auction:
      navHandler.navigateToAuction(result.id);
      return;
    case SearchResultType.content:
      navHandler.navigateToContentDetail(result.id);
      return;
  }
}
