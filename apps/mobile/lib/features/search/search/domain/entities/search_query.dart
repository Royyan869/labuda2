import 'package:equatable/equatable.dart';
import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';

/// Search query parameters
class SearchQuery extends Equatable {
  final String query;
  final SearchResultType? filterByType;
  final SearchFilters filters;
  final SearchSortBy sortBy;
  final int limit;
  final int offset;
  final String? cursor;

  const SearchQuery({
    required this.query,
    this.filterByType,
    this.filters = const SearchFilters(),
    this.sortBy = SearchSortBy.relevance,
    this.limit = 20,
    this.offset = 0,
    this.cursor,
  });

  @override
  List<Object?> get props => [
    query,
    filterByType,
    filters,
    sortBy,
    limit,
    offset,
    cursor,
  ];

  SearchQuery copyWith({
    String? query,
    SearchResultType? filterByType,
    SearchFilters? filters,
    SearchSortBy? sortBy,
    int? limit,
    int? offset,
    String? cursor,
  }) {
    return SearchQuery(
      query: query ?? this.query,
      filterByType: filterByType ?? this.filterByType,
      filters: filters ?? this.filters,
      sortBy: sortBy ?? this.sortBy,
      limit: limit ?? this.limit,
      offset: offset ?? this.offset,
      cursor: cursor ?? this.cursor,
    );
  }

  /// Clear the type filter
  SearchQuery clearTypeFilter() {
    return SearchQuery(
      query: query,
      filterByType: null,
      filters: filters,
      sortBy: sortBy,
      limit: limit,
      offset: offset,
      cursor: cursor,
    );
  }

  /// Check if query is valid
  bool get isValid => query.trim().length >= 2;

  /// Get trimmed query
  String get trimmedQuery => query.trim().toLowerCase();
}
