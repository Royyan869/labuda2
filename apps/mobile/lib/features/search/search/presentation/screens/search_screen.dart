import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';
import 'package:labuda/features/search/search/presentation/providers/search_history_notifier.dart';
import 'package:labuda/features/search/search/presentation/providers/search_history_state.dart';
import 'package:labuda/features/search/search/presentation/widgets/global_search_bar.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_history_list.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_suggestions_list.dart';

/// Main search screen with search bar, history, and suggestions
///
/// SEARCH SURFACE PURGE V1:
/// - "Trending Searches" renamed to "Popular Koi Varieties" (honest label)
/// - Removed fake trending data from repository - now using static curated list
class SearchScreen extends ConsumerStatefulWidget {
  const SearchScreen({super.key});

  @override
  ConsumerState<SearchScreen> createState() => _SearchScreenState();
}

class _SearchScreenState extends ConsumerState<SearchScreen> {
  final _searchController = TextEditingController();
  String _query = '';

  @override
  void initState() {
    super.initState();
    // Load history after first frame when user ID is available
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadHistoryIfNeeded();
    });
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _loadHistoryIfNeeded() {
    final userId = ref.read(currentUserIdProvider);
    if (userId.isNotEmpty) {
      ref.read(searchHistoryProvider.notifier).loadHistory(userId);
    }
  }

  void _onQueryChanged(String query) {
    setState(() {
      _query = query;
    });
  }

  void _onSearch(String query) {
    if (query.trim().length < 2) return;

    final userId = ref.read(currentUserIdProvider);

    // Save to history
    if (userId.isNotEmpty) {
      ref
          .read(searchHistoryProvider.notifier)
          .saveSearch(userId, query.trim(), 0);
    }

    // Navigate to search results
    final navHandler = ref.read(navigationHandlerProvider);
    navHandler.navigateToSearchResults(query.trim());
  }

  void _onSuggestionTap(String suggestion) {
    final cleanQuery = suggestion.startsWith('@')
        ? suggestion.substring(1)
        : suggestion;
    _searchController.text = cleanQuery;
    _onSearch(cleanQuery);
  }

  void _onHistoryTap(String query) {
    _searchController.text = query;
    _onSearch(query);
  }

  void _onClearAllHistory() {
    final userId = ref.read(currentUserIdProvider);
    if (userId.isNotEmpty) {
      ref.read(searchHistoryProvider.notifier).clearHistory(userId);
    }
  }

  void _onDeleteHistory(String historyId) {
    final userId = ref.read(currentUserIdProvider);
    if (userId.isNotEmpty) {
      ref
          .read(searchHistoryProvider.notifier)
          .deleteItem(userId, historyId);
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final historyState = ref.watch(searchHistoryProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Search'),
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        elevation: 0,
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: GlobalSearchBar(
              initialQuery: _query,
              onSearch: _onSearch,
              onQueryChanged: _onQueryChanged,
              autofocus: true,
              showCategoryChips: false,
            ),
          ),
          Expanded(
            child: _query.isEmpty
                ? _buildInitialContent(historyState)
                : _buildSuggestions(),
          ),
        ],
      ),
    );
  }

  Widget _buildInitialContent(SearchHistoryState historyState) {
    return SingleChildScrollView(
      child: Column(
        children: [
          SearchHistoryList(
            history: historyState.history,
            onHistoryTap: _onHistoryTap,
            onDeleteTap: _onDeleteHistory,
            onClearAll: _onClearAllHistory,
          ),
          SearchSuggestionsList(
            suggestions: const [],
            // HONEST LABEL: This is curated content, not real trending data
            trendingSearches: _getPopularKoiVarieties(),
            popularItemsTitle: 'Popular Koi Varieties',
            onSuggestionTap: _onSuggestionTap,
          ),
        ],
      ),
    );
  }

  Widget _buildSuggestions() {
    return SearchSuggestionsList(
      suggestions: const [],
      trendingSearches: _getPopularKoiVarieties(),
      popularItemsTitle: 'Popular Koi Varieties',
      onSuggestionTap: _onSuggestionTap,
    );
  }

  /// Popular koi varieties for quick search
  /// SEARCH SURFACE PURGE V1: Renamed from _getTrendingSearches() for honesty
  List<String> _getPopularKoiVarieties() {
    return ['Kohaku', 'Sanke', 'Showa', 'Tancho', 'Shiro Utsuri'];
  }
}
