import 'package:hishumi/core/api/structured_api_exception.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/features/search/search/data/mappers/search_mapper.dart';
import 'package:hishumi/features/search/search/data/remote/search_api_service.dart';
import 'package:hishumi/features/search/search/domain/entities/search_history.dart';
import 'package:hishumi/features/search/search/domain/repositories/search_history_repository.dart';

/// Search History Repository Implementation using API backend
class SearchHistoryRepositoryImpl implements SearchHistoryRepository {
  final SearchApiService _apiService;

  SearchHistoryRepositoryImpl(this._apiService);

  @override
  Future<Result<void>> saveSearchHistory(SearchHistory history) async {
    try {
      await _apiService.saveSearchHistory(
        query: history.query,
        searchType: history.type?.name,
        resultsCount: history.resultCount,
      );

      return Result.success(null);
    } catch (e) {
      return _failure(e);
    }
  }

  @override
  Future<Result<List<SearchHistory>>> getSearchHistory(
    String userId, {
    int limit = 10,
  }) async {
    try {
      final dtos = await _apiService.getSearchHistory(limit: limit);

      final history = dtos.map((dto) => dto.toDomain(userId)).toList();

      return Result.success(history);
    } catch (e) {
      return _failure(e);
    }
  }

  @override
  Future<Result<void>> clearSearchHistory(String userId) async {
    try {
      await _apiService.clearSearchHistory();
      return Result.success(null);
    } catch (e) {
      return _failure(e);
    }
  }

  @override
  Future<Result<void>> deleteSearchHistoryItem(
    String userId,
    String historyId,
  ) async {
    try {
      await _apiService.deleteSearchHistoryItem(historyId);
      return Result.success(null);
    } catch (e) {
      return _failure(e);
    }
  }

  /// Convert a thrown failure into a `Result` failure, keeping the canonical
  /// API error code when the API layer preserved one.
  Result<T> _failure<T>(Object error) {
    if (error is StructuredApiException) {
      return Result.error(
        error.message,
        code: error.code,
        details: error.details,
      );
    }
    return Result.error(error.toString());
  }
}
