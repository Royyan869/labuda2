import 'package:labuda/features/search/search/domain/entities/search_history.dart';

/// Result type for operations that can fail
typedef ApiResult<T> = ({T? data, String? error});

/// Repository interface for search history operations
abstract interface class SearchHistoryRepository {
  /// Save a search to history
  Future<ApiResult<void>> saveSearchHistory(SearchHistory history);

  /// Get user's search history
  Future<ApiResult<List<SearchHistory>>> getSearchHistory(
    String userId, {
    int limit = 10,
  });

  /// Clear all search history for a user
  Future<ApiResult<void>> clearSearchHistory(String userId);

  /// Delete a specific search history item
  Future<ApiResult<void>> deleteSearchHistoryItem(
    String userId,
    String historyId,
  );
}
