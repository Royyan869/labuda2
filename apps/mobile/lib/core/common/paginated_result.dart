/// Paginated result wrapper for list queries
///
/// Generic wrapper for paginated API responses. Used across all features
/// that need pagination support.
///
/// Example usage:
/// ```dart
/// final result = await repository.listItems(page: 1, limit: 20);
/// if (result.isSuccess) {
///   final paginated = result.data!;
///   print('Got ${paginated.items.length} of ${paginated.total} items');
///   if (paginated.hasNextPage) {
///     // Load more...
///   }
/// }
/// ```
class PaginatedResult<T> {
  /// List of items for current page
  final List<T> items;

  /// Total count of all items (across all pages)
  final int total;

  /// Current page number (1-indexed)
  final int page;

  /// Number of items per page
  final int pageSize;

  /// Total number of pages
  final int totalPages;

  const PaginatedResult({
    required this.items,
    required this.total,
    required this.page,
    required this.pageSize,
    required this.totalPages,
  });

  /// Create from API response data
  factory PaginatedResult.fromResponse({
    required List<T> items,
    required int total,
    required int page,
    required int pageSize,
  }) {
    final totalPages = (total / pageSize).ceil();
    return PaginatedResult(
      items: items,
      total: total,
      page: page,
      pageSize: pageSize,
      totalPages: totalPages == 0 ? 1 : totalPages,
    );
  }

  /// Create empty result
  factory PaginatedResult.empty({int pageSize = 20}) {
    return PaginatedResult(
      items: [],
      total: 0,
      page: 1,
      pageSize: pageSize,
      totalPages: 1,
    );
  }

  /// Whether there's a next page available
  bool get hasNextPage => page < totalPages;

  /// Whether there's a previous page available
  bool get hasPreviousPage => page > 1;

  /// Whether the result is empty
  bool get isEmpty => items.isEmpty;

  /// Whether the result has items
  bool get isNotEmpty => items.isNotEmpty;

  /// Number of items in current page
  int get count => items.length;

  /// Transform items to different type
  PaginatedResult<R> map<R>(R Function(T item) transform) {
    return PaginatedResult<R>(
      items: items.map(transform).toList(),
      total: total,
      page: page,
      pageSize: pageSize,
      totalPages: totalPages,
    );
  }

  @override
  String toString() {
    return 'PaginatedResult(page: $page/$totalPages, items: ${items.length}, total: $total)';
  }
}
