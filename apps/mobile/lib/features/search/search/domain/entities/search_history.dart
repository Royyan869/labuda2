import 'package:equatable/equatable.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';

/// Search history entry
class SearchHistory extends Equatable {
  final String id;
  final String userId;
  final String query;
  final SearchResultType? type;
  final DateTime searchedAt;
  final int resultCount;

  const SearchHistory({
    required this.id,
    required this.userId,
    required this.query,
    this.type,
    required this.searchedAt,
    this.resultCount = 0,
  });

  @override
  List<Object?> get props => [id, userId, query, type, searchedAt, resultCount];

  SearchHistory copyWith({
    String? id,
    String? userId,
    String? query,
    SearchResultType? type,
    DateTime? searchedAt,
    int? resultCount,
  }) {
    return SearchHistory(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      query: query ?? this.query,
      type: type ?? this.type,
      searchedAt: searchedAt ?? this.searchedAt,
      resultCount: resultCount ?? this.resultCount,
    );
  }
}
