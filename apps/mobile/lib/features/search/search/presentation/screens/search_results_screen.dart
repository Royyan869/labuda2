import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/features/search/search/presentation/providers/search_notifier.dart';
import 'package:hishumi/features/search/search/presentation/providers/search_state.dart';
import 'package:hishumi/features/search/search/domain/entities/search_result.dart';
import 'package:hishumi/features/search/search/presentation/utils/search_result_type_helper.dart';
import 'package:hishumi/features/search/search/presentation/widgets/all_tab_results_view.dart';
import 'package:hishumi/features/search/search/presentation/widgets/global_search_bar.dart';
import 'package:hishumi/features/search/search/presentation/widgets/search_result_item.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';
import 'package:hishumi/shared/widgets/external_link_interstitial.dart';
import 'package:hishumi/shared/widgets/loading_indicator.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';

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
    Tab(text: 'For Sale'),
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

    // Execute initial search
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _executeSearch();
    });
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  /// Explicit user-tab selection. Tab presentation only: it switches which
  /// projection of the canonical result state is shown, never a new search.
  void _onTabSelected(int index) {
    final type = SearchResultTypeHelper.getTypeFromIndex(index);
    ref.read(searchProvider.notifier).setSelectedType(type);
  }

  /// "Lihat Semua" — switches to the domain tab. Same query, same canonical
  /// result state; no new search is fired (the tab shows the full canonical
  /// domain collection already held in state).
  void _onSeeAll(SearchResultType type) {
    _onTabSelected(SearchResultTypeHelper.getTabIndex(type));
    _tabController.animateTo(SearchResultTypeHelper.getTabIndex(type));
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
    final searchState = ref.watch(searchProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Search Results'),
        bottom: TabBar(
          controller: _tabController,
          onTap: _onTabSelected,
          isScrollable: true,
          tabs: _tabs,
        ),
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(AppMetrics.p16),
            child: GlobalSearchBar(
              initialQuery: _currentQuery,
              onSearch: _onSearch,
              showCategoryChips: false,
            ),
          ),
          Expanded(child: _buildBody(searchState)),
        ],
      ),
    );
  }

  /// LOADING FOUNDATION (owner-locked):
  /// - No results yet (first request pending/in-flight) → [LoadingIndicator].
  ///   A null result set with no error is "not yet loaded", NEVER EmptyState
  ///   (no empty flash before the first request settles).
  /// - First-load failure (no results) → [PageErrorState].
  /// - Successful zero-result → [EmptyState].
  /// - Re-search with cached results → cached results stay visible with an
  ///   update indicator; failure renders an inline banner, never a full-page
  ///   loading/error swap.
  Widget _buildBody(SearchState state) {
    final results = state.results;
    final hasResults = results != null && results.isNotEmpty;

    if (!hasResults) {
      if (state.isSearching) {
        return const Center(child: LoadingIndicator());
      }
      if (state.error != null) {
        return _buildError();
      }
      // No error and no longer searching, yet no result set: the request
      // has not settled (e.g. first frame before the post-frame search
      // dispatch). Still "not yet loaded" — loading, never EmptyState.
      if (results == null) {
        return const Center(child: LoadingIndicator());
      }
      return _buildEmptyState();
    }

    return Column(
      children: [
        if (state.isSearching) const LinearProgressIndicator(minHeight: 2),
        if (state.error != null) _buildRefreshErrorBanner(),
        Expanded(child: _buildResults(state)),
      ],
    );
  }

  Widget _buildResults(SearchState state) {
    final results = state.results;
    if (results == null || results.isEmpty) {
      return _buildEmptyState();
    }

    // All tab: section-based multi-domain overview over the canonical
    // domain collections (empty domains are omitted). Preview caps live in
    // AllTabPreviewLimits and never truncate the per-type tab collections.
    if (state.selectedType == null) {
      return AllTabResultsView(
        results: results,
        onSeeAll: _onSeeAll,
        onItemTap: _onResultTap,
      );
    }

    // Per-type tab: canonical domain collection, no All preview truncation.
    return _buildTypeResults(state);
  }

  /// Minimum bounded refresh-failure indication: persistent inline banner
  /// with safe localized copy and a retry action. Not a new foundation.
  Widget _buildRefreshErrorBanner() {
    final scheme = Theme.of(context).colorScheme;
    final l10n = context.l10n;
    return Semantics(
      container: true,
      liveRegion: true,
      child: Container(
        width: double.infinity,
        margin: const EdgeInsets.fromLTRB(
          AppMetrics.p16,
          AppMetrics.p12,
          AppMetrics.p16,
          AppMetrics.p4,
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: AppMetrics.p12,
          vertical: AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: scheme.errorContainer,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(color: scheme.error),
        ),
        child: Row(
          children: [
            Icon(
              Icons.refresh_outlined,
              size: AppIconSize.action,
              color: scheme.onErrorContainer,
            ),
            const SizedBox(width: AppMetrics.p8),
            Expanded(
              child: Text(
                l10n.pageErrorMessage,
                style: context.typeRoles.bodyDense.copyWith(
                  color: scheme.onErrorContainer,
                ),
              ),
            ),
            TextButton(
              onPressed: () => _executeSearch(),
              child: Text(l10n.retryAction),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildTypeResults(SearchState state) {
    final results = state.selectedDomainResults;
    final scheme = Theme.of(context).colorScheme;

    return ListView.separated(
      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p8),
      itemCount: results.length,
      separatorBuilder: (_, _) =>
          Divider(height: 1, color: scheme.outlineVariant),
      itemBuilder: (context, index) {
        final result = results[index];
        return SearchResultItem(
          result: result,
          onTap: () => _onResultTap(result),
        );
      },
    );
  }

  /// CANONICAL page-level load error (PageErrorState). The raw search
  /// state.error never reaches the screen — safe localized copy only.
  Widget _buildError() {
    return PageErrorState(
      onRetry: () async {
        _executeSearch();
      },
    );
  }

  /// Search/filter empty: a query was executed and nothing matched. The
  /// persistent search bar above is the affordance that edits or clears the
  /// query, so no separate reset action is offered here.
  Widget _buildEmptyState() {
    return EmptyState(
      icon: Icons.search_off,
      title: context.l10n.emptySearchTitle,
      subtitle: context.l10n.emptySearchMessage,
    );
  }
}

Future<void> handleSearchResultTap(
  BuildContext context,
  WidgetRef ref,
  SearchResult result,
) async {
  // Promotion click tracking removed: the legacy /promotions/events endpoint
  // is purged and search sidecar cards carry no canonical exposure identity,
  // so there is no legitimate measurement to acknowledge. Navigation only.
  final navHandler = ref.read(navigationHandlerProvider);

  switch (result.type) {
    case SearchResultType.user:
      navHandler.navigateToUserProfile(result.id);
      return;
    case SearchResultType.forSale:
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
