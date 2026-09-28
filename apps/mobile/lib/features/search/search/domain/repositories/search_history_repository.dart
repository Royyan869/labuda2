import 'package:labuda/core/common/result.dart';
import 'package:labuda/features/search/search/domain/entities/search_history.dart';

/// Repository interface for search history operations
abstract interface class SearchHistoryRepository {
  /// Save a search to history
  Future<Result<void>> saveSearchHistory(SearchHistory history);

  /// Get user's search history
  Future<Result<List<SearchHistory>>> getSearchHistory(
    String userId, {
    int limit = 10,
  });

  /// Clear all search history for a user
  Future<Result<void>> clearSearchHistory(String userId);

  /// Delete a specific search history item
  Future<Result<void>> deleteSearchHistoryItem(String userId, String historyId);
}
