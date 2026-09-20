import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';

/// Search State
///
/// SECTION-BASED ALL (canonical): [results] holds separate canonical domain
/// collections. When [selectedType] is null the UI renders the All overview
/// (independent sections, each capped at its own preview limit); otherwise
/// the selected domain collection is shown as-is — never truncated by the
/// All preview limits.
class SearchState {
  final UnifiedSearchResults? results;
  final String query;
  final SearchResultType? selectedType;
  final SearchFilters filters;
  final bool isSearching;
  final String? error;

  const SearchState({
    this.results,
    this.query = '',
    this.selectedType,
    this.filters = const SearchFilters(),
    this.isSearching = false,
    this.error,
  });

  SearchState copyWith({
    UnifiedSearchResults? results,
    String? query,
    SearchResultType? selectedType,
    SearchFilters? filters,
    bool? isSearching,
    String? error,
  }) {
    return SearchState(
      results: results ?? this.results,
      query: query ?? this.query,
      // selectedType and error are nullable on purpose: passing null CLEARS
      // them (selectedType null = All tab; error null = no error). The
      // `?? this.x` pattern would silently keep the previous value and make
      // "back to All" impossible.
      selectedType: selectedType,
      filters: filters ?? this.filters,
      isSearching: isSearching ?? this.isSearching,
      error: error,
    );
  }

  /// Domain collection backing the currently selected per-type tab.
  ///
  /// All is not a domain collection — when [selectedType] is null (All tab)
  /// this returns an empty list and the UI renders the section overview
  /// from [results] instead. `externalProduct` rows live on the For Sale
  /// (forSale) surface and share its collection.
  List<SearchResult> get selectedDomainResults {
    if (results == null || selectedType == null) return const [];
    switch (selectedType!) {
      case SearchResultType.user:
        return results!.users;
      case SearchResultType.forSale:
      case SearchResultType.externalProduct:
        return results!.forSales;
      case SearchResultType.auction:
        return results!.auctions;
      case SearchResultType.content:
        return results!.contents;
    }
  }
}
