import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';

/// Search State
class SearchState {
  final UnifiedSearchResults? results;
  final String query;
  final SearchResultType? selectedType;
  final SearchFilters filters;
  final SearchSortBy sortBy;
  final bool isSearching;
  final String? error;

  const SearchState({
    this.results,
    this.query = '',
    this.selectedType,
    this.filters = const SearchFilters(),
    this.sortBy = SearchSortBy.relevance,
    this.isSearching = false,
    this.error,
  });

  SearchState copyWith({
    UnifiedSearchResults? results,
    String? query,
    SearchResultType? selectedType,
    SearchFilters? filters,
    SearchSortBy? sortBy,
    bool? isSearching,
    String? error,
  }) {
    return SearchState(
      results: results ?? this.results,
      query: query ?? this.query,
      selectedType: selectedType ?? this.selectedType,
      filters: filters ?? this.filters,
      sortBy: sortBy ?? this.sortBy,
      isSearching: isSearching ?? this.isSearching,
      error: error ?? this.error,
    );
  }

  /// Clear type filter
  SearchState clearTypeFilter() {
    return SearchState(
      results: results,
      query: query,
      selectedType: null,
      filters: filters,
      sortBy: sortBy,
      isSearching: isSearching,
      error: error,
    );
  }

  /// Check if there are results
  bool get hasResults => results != null && results!.isNotEmpty;

  /// Get results for selected type or all
  List<SearchResult> get displayResults {
    if (results == null) return [];
    if (selectedType == null) return results!.allResults;
    return results!.getByType(selectedType!);
  }
}
