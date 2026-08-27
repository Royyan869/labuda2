import 'package:labuda/features/search/search/data/mappers/search_mapper.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/domain/entities/search_history.dart';
import 'package:labuda/features/search/search/domain/repositories/search_history_repository.dart';

/// Search History Repository Implementation using API backend
class SearchHistoryRepositoryImpl implements SearchHistoryRepository {
  final SearchApiService _apiService;

  SearchHistoryRepositoryImpl(this._apiService);

  @override
  Future<ApiResult<void>> saveSearchHistory(SearchHistory history) async {
    try {
      await _apiService.saveSearchHistory(
        query: history.query,
        searchType: history.type?.name,
        resultsCount: history.resultCount,
      );

      return (data: null, error: null);
    } catch (e) {
      return (data: null, error: 'Failed to save history: ${e.toString()}');
    }
  }

  @override
  Future<ApiResult<List<SearchHistory>>> getSearchHistory(
    String userId, {
    int limit = 10,
  }) async {
    try {
      final dtos = await _apiService.getSearchHistory(limit: limit);

      final history = dtos.map((dto) => dto.toDomain(userId)).toList();

      return (data: history, error: null);
    } catch (e) {
      return (data: null, error: 'Failed to fetch history: ${e.toString()}');
    }
  }

  @override
  Future<ApiResult<void>> clearSearchHistory(String userId) async {
    try {
      await _apiService.clearSearchHistory();
      return (data: null, error: null);
    } catch (e) {
      return (data: null, error: 'Failed to clear history: ${e.toString()}');
    }
  }

  @override
  Future<ApiResult<void>> deleteSearchHistoryItem(
    String userId,
    String historyId,
  ) async {
    try {
      await _apiService.deleteSearchHistoryItem(historyId);
      return (data: null, error: null);
    } catch (e) {
      return (data: null, error: 'Failed to delete item: ${e.toString()}');
    }
  }
}
