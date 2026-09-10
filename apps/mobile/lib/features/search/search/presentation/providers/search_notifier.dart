import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'search_state.dart';
import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/domain/usecases/search_usecase_providers.dart';

part 'search_notifier.g.dart';

/// Search Notifier - handles search state management using UseCase
///
/// SECTION-BASED ALL (canonical): a search query has a single execution
/// authority ([searchAll]). Switching between All and per-type tabs only
/// changes which projection of the canonical result state is displayed —
/// no additional search is fired by tab navigation.
@riverpod
class SearchNotifier extends _$SearchNotifier {
  // Concurrency guard to prevent race conditions
  String? _currentSearchId;

  @override
  SearchState build() {
    return const SearchState();
  }

  /// Perform the canonical search across all domains
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

  /// Set selected type filter (tab presentation only — never re-fetches)
  void setSelectedType(SearchResultType? type) {
    state = state.copyWith(selectedType: type);
  }

  /// Set filters
  void setFilters(SearchFilters filters) {
    state = state.copyWith(filters: filters);
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
