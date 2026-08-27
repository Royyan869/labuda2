/// Meta response for pagination
///
/// Backend returns pagination metadata in this format:
/// ```json
/// {
///   "meta": {
///     "page": 1,
///     "per_page": 20,
///     "total": 100,
///     "total_pages": 5
///   }
/// }
/// ```
class MetaResponse {
  final int page;
  final int perPage;
  final int total;
  final int totalPages;

  const MetaResponse({
    required this.page,
    required this.perPage,
    required this.total,
    required this.totalPages,
  });

  factory MetaResponse.fromJson(Map<String, dynamic> json) {
    return MetaResponse(
      page: json['page'] as int? ?? 1,
      perPage: json['per_page'] as int? ?? 20,
      total: json['total'] as int? ?? 0,
      totalPages: json['total_pages'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
    'page': page,
    'per_page': perPage,
    'total': total,
    'total_pages': totalPages,
  };

  /// Check if there's a next page
  bool get hasNextPage => page < totalPages;

  /// Check if this is the first page
  bool get isFirstPage => page == 1;

  /// Check if this is the last page
  bool get isLastPage => page >= totalPages;

  @override
  String toString() =>
      'MetaResponse(page: $page, perPage: $perPage, total: $total, totalPages: $totalPages)';
}
