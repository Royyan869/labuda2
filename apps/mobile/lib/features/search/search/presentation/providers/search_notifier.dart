import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'search_state.dart';
import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/domain/usecases/search_usecase_providers.dart';

part 'search_notifier.g.dart';

/// Search Notifier - handles search state management using UseCase
@riverpod
class SearchNotifier extends _$SearchNotifier {
  // Concurrency guards to prevent race conditions
  String? _currentSearchId;
  String? _currentTypeSearchId;

  @override
  SearchState build() {
    return const SearchState();
  }

  /// Perform unified search across all types
  Future<void> searchAll(String query, {SearchFilters? filters}) async {
    // Generate unique search ID to ignore outdated responses
    final searchId = '${query}_${DateTime.now().millisecondsSinceEpoch}';
    _currentSearchId = searchId;

    state = state.copyWith(
      isSearching: true,
      query: query,
      filters: filters ?? state.filters,
      error: null,
    );

    // Use usecase for search business logic
    final searchUseCase = ref.read(searchUseCaseProvider);
    final result = await searchUseCase.searchAll(
      query,
      filters: filters ?? state.filters,
    );

    // Only update state if this is still the current search
    if (_currentSearchId != searchId) return;

    if (result.isError) {
      state = state.copyWith(isSearching: false, error: result.error);
    } else {
      state = state.copyWith(
        results: result.data,
        isSearching: false,
        error: null,
      );
    }
  }

  /// Search by specific type
  Future<void> searchByType(
    String query,
    SearchResultType type, {
    SearchFilters? filters,
  }) async {
    // Generate unique search ID to ignore outdated responses
    final searchId =
        '${query}_${type.name}_${DateTime.now().millisecondsSinceEpoch}';
    _currentTypeSearchId = searchId;

    state = state.copyWith(
      isSearching: true,
      query: query,
      selectedType: type,
      filters: filters ?? state.filters,
      error: null,
    );

    // Use usecase for search business logic
    final searchUseCase = ref.read(searchUseCaseProvider);
    final result = await searchUseCase.searchByType(
      query,
      type,
      filters: filters ?? state.filters,
      existingResults: state.results,
    );

    // Only update state if this is still the current search
    if (_currentTypeSearchId != searchId) return;

    if (result.isError) {
      state = state.copyWith(isSearching: false, error: result.error);
    } else {
      state = state.copyWith(
        results: result.data,
        isSearching: false,
        error: null,
      );
    }
  }

  /// SEARCH SURFACE PURGE V1: getSuggestions() and getTrendingSearches() removed
  /// These were calling removed repository methods that returned fake/empty data

  /// Set selected type filter
  void setSelectedType(SearchResultType? type) {
    state = state.copyWith(selectedType: type);
  }

  /// Set filters
  void setFilters(SearchFilters filters) {
    state = state.copyWith(filters: filters);
  }

  /// Set sort order
  void setSortBy(SearchSortBy sortBy) {
    state = state.copyWith(sortBy: sortBy);
    // Re-search with new sort order
    if (state.query.isNotEmpty) {
      if (state.selectedType != null) {
        searchByType(state.query, state.selectedType!);
      } else {
        searchAll(state.query);
      }
    }
  }

  /// Clear search
  void clearSearch() {
    state = const SearchState();
  }

  /// Clear error
  void clearError() {
    state = state.copyWith(error: null);
  }
}
