import 'package:labuda/core/common/result.dart';
import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/domain/repositories/search_repository.dart';

/// Search Use Case
///
/// **DOMAIN:** Search domain
/// **RESPONSIBILITY:** Handle unified search operations across all content types
/// **BOUNDARY:** Encapsulates search business rules and validation
class SearchUseCase {
  final SearchRepository _repository;

  SearchUseCase(this._repository);

  /// Perform unified search across all types
  ///
  /// Business Rules:
  /// - Query must be at least 2 characters
  /// - Returns unified results from all searchable content types
  /// - Supports optional filters for narrowing results
  Future<Result<UnifiedSearchResults>> searchAll(
    String query, {
    SearchFilters? filters,
  }) async {
    // Validate query
    final validationError = _validateQuery(query);
    if (validationError != null) {
      return Result.error(validationError);
    }

    try {
      final result = await _repository.searchAll(
        query: query,
        filters: filters,
      );

      if (result.data != null) {
        return Result.success(result.data!);
      } else {
        return Result.error(result.error ?? 'Search failed');
      }
    } catch (e) {
      return Result.error('Search failed: $e');
    }
  }

  /// Perform search by specific type
  ///
  /// Business Rules:
  /// - Query must be at least 2 characters
  /// - Returns results only from specified content type
  /// - Merges with existing results if provided
  Future<Result<UnifiedSearchResults>> searchByType(
    String query,
    SearchResultType searchType, {
    SearchFilters? filters,
    UnifiedSearchResults? existingResults,
  }) async {
    // Validate query
    final validationError = _validateQuery(query);
    if (validationError != null) {
      return Result.error(validationError);
    }

    try {
      final result = await _repository.searchByType(
        query: query,
        type: searchType,
        filters: filters,
      );

      if (result.data != null) {
        // Business Rule: Merge type-specific results with existing results
        final mergedResults = _mergeTypeResults(
          searchType,
          result.data!,
          existingResults,
          query,
        );
        return Result.success(mergedResults);
      } else {
        return Result.error(result.error ?? 'Search failed');
      }
    } catch (e) {
      return Result.error('Search failed: $e');
    }
  }

  /// Merge type-specific results with existing unified results
  ///
  /// Business Rule: When searching by type, preserve results from other
  /// types while updating the searched type with new results
  UnifiedSearchResults _mergeTypeResults(
    SearchResultType type,
    List<SearchResult> newResults,
    UnifiedSearchResults? existingResults,
    String query,
  ) {
    final isListingSurface =
        type == SearchResultType.listing ||
        type == SearchResultType.externalProduct;

    // If no existing results, create new structure
    if (existingResults == null) {
      return UnifiedSearchResults(
        allResults: newResults,
        users: type == SearchResultType.user ? newResults : [],
        listings: isListingSurface ? newResults : [],
        auctions: type == SearchResultType.auction ? newResults : [],
        contents: type == SearchResultType.content ? newResults : [],
        totalCount: newResults.length,
        query: query,
        searchDuration: Duration.zero,
      );
    }

    // Merge with existing results
    return UnifiedSearchResults(
      allResults: newResults,
      users: type == SearchResultType.user ? newResults : existingResults.users,
      listings: isListingSurface ? newResults : existingResults.listings,
      auctions: type == SearchResultType.auction
          ? newResults
          : existingResults.auctions,
      contents: type == SearchResultType.content
          ? newResults
          : existingResults.contents,
      totalCount: newResults.length,
      query: query,
      searchDuration: Duration.zero,
    );
  }

  /// Search contents
  ///
  /// Business Rules:
  /// - Query must be at least 2 characters
  Future<Result<List<ContentSearchResult>>> searchContents(
    String query,
  ) async {
    // Validate query
    final validationError = _validateQuery(query);
    if (validationError != null) {
      return Result.error(validationError);
    }

    try {
      final result = await _repository.searchContents(query: query);

      if (result.data != null) {
        return Result.success(result.data!);
      } else {
        return Result.error(result.error ?? 'Search failed');
      }
    } catch (e) {
      return Result.error('Search failed: $e');
    }
  }

  /// Search auctions
  ///
  /// Business Rules:
  /// - Query must be at least 2 characters
  Future<Result<List<AuctionSearchResult>>> searchAuctions(
    String query, {
    int page = 1,
    int pageSize = 20,
  }) async {
    // Validate query
    final validationError = _validateQuery(query);
    if (validationError != null) {
      return Result.error(validationError);
    }

    try {
      final result = await _repository.searchAuctions(
        query: query,
        page: page,
        pageSize: pageSize,
      );

      if (result.data != null) {
        return Result.success(result.data!);
      } else {
        return Result.error(result.error ?? 'Search failed');
      }
    } catch (e) {
      return Result.error('Search failed: $e');
    }
  }

  /// Search users
  ///
  /// Business Rules:
  /// - Query must be at least 2 characters
  Future<Result<List<UserSearchResult>>> searchUsers(
    String query, {
    int page = 1,
    int pageSize = 20,
  }) async {
    // Validate query
    final validationError = _validateQuery(query);
    if (validationError != null) {
      return Result.error(validationError);
    }

    try {
      final result = await _repository.searchUsers(
        query: query,
        page: page,
        pageSize: pageSize,
      );

      if (result.data != null) {
        return Result.success(result.data!);
      } else {
        return Result.error(result.error ?? 'Search failed');
      }
    } catch (e) {
      return Result.error('Search failed: $e');
    }
  }

  /// Validate search query
  ///
  /// Business Rule: Query must be at least 2 characters
  /// Returns error message if validation fails, null if valid
  String? _validateQuery(String query) {
    if (query.trim().length < 2) {
      return 'Query minimal 2 karakter';
    }
    return null;
  }
}
