import 'package:labuda/features/search/search/domain/entities/search_history.dart';

/// Search History State
class SearchHistoryState {
  final List<SearchHistory> history;
  final bool isLoading;
  final String? error;

  const SearchHistoryState({
    this.history = const [],
    this.isLoading = false,
    this.error,
  });

  SearchHistoryState copyWith({
    List<SearchHistory>? history,
    bool? isLoading,
    String? error,
  }) {
    return SearchHistoryState(
      history: history ?? this.history,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
    );
  }
}
