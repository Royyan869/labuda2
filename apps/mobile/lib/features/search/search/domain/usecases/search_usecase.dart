import 'package:labuda/core/common/result.dart';
import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/domain/repositories/search_repository.dart';

/// Search Use Case
///
/// **DOMAIN:** Search domain
/// **RESPONSIBILITY:** Execute the canonical unified search for a query.
///
/// SECTION-BASED ALL (canonical): the only unified-search command is
/// [searchAll]. It returns per-domain collections (users, forSales,
/// auctions, contents) that keep their canonical backend ordering; the
/// All tab renders them as independent sections. There is no per-type
/// re-search that merges back into unified state and no cross-domain
/// relevance sort.
class SearchUseCase {
  final SearchRepository _repository;

  SearchUseCase(this._repository);

  /// Perform the canonical search across all content types.
  ///
  /// Business Rules:
  /// - Query must be at least 2 characters
  /// - Each domain result keeps its own canonical ordering
  /// - No unified cross-domain ranking is produced
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
