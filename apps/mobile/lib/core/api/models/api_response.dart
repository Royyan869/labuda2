import 'meta_response.dart';

// API Response models for parsing Go backend responses

/// Standard API response wrapper
///
/// Go backend returns responses in this format:
/// ```json
/// {
///   "success": true,
///   "data": { ... },
///   "message": "Optional message",
///   "timestamp": "2026-01-01T00:00:00Z"
/// }
/// ```
///
/// Or for errors:
/// ```json
/// {
///   "success": false,
///   "error": {
///     "code": "ERROR_CODE",
///     "message": "Error description",
///     "details": { ... }
///   },
///   "timestamp": "2026-01-01T00:00:00Z"
/// }
/// ```
///
/// Or for paginated responses:
/// ```json
/// {
///   "success": true,
///   "data": [...],
///   "meta": {
///     "page": 1,
///     "per_page": 20,
///     "total": 100,
///     "total_pages": 5
///   },
///   "timestamp": "2026-01-01T00:00:00Z"
/// }
/// ```
class ApiResponse<T> {
  final bool success;
  final T? data;
  final String? message;
  final MetaResponse? meta;
  final ApiError? error;
  final String? timestamp;

  const ApiResponse({
    required this.success,
    this.data,
    this.message,
    this.meta,
    this.error,
    this.timestamp,
  });

  factory ApiResponse.fromJson(
    Map<String, dynamic> json,
    T Function(dynamic json)? fromJsonT,
  ) {
    return ApiResponse(
      success: json['success'] as bool? ?? false,
      data: json['data'] != null && fromJsonT != null
          ? fromJsonT(json['data'])
          : json['data'] as T?,
      message: json['message'] as String?,
      meta: json['meta'] != null
          ? MetaResponse.fromJson(json['meta'] as Map<String, dynamic>)
          : null,
      error: json['error'] != null ? ApiError.fromJson(json['error']) : null,
      timestamp: json['timestamp'] as String?,
    );
  }

  /// Check if response has data
  bool get hasData => data != null;

  /// Check if response has error
  bool get hasError => error != null || !success;

  /// Check if response has pagination metadata
  bool get hasMeta => meta != null;
}

/// Paginated response wrapper
///
/// @deprecated Use `ApiResponse<List<T>>` with `meta` field instead.
/// Backend returns paginated data in this format:
/// ```json
/// {
///   "success": true,
///   "data": [ ... ],
///   "meta": {
///     "page": 1,
///     "per_page": 20,
///     "total": 100,
///     "total_pages": 5
///   },
///   "timestamp": "2026-01-01T00:00:00Z"
/// }
/// ```
@Deprecated('Use ApiResponse<List<T>> with meta field instead')
class PaginatedResponse<T> {
  final bool success;
  final List<T> data;
  final Pagination pagination;
  final String? message;

  const PaginatedResponse({
    required this.success,
    required this.data,
    required this.pagination,
    this.message,
  });

  factory PaginatedResponse.fromJson(
    Map<String, dynamic> json,
    T Function(Map<String, dynamic> json) fromJsonT,
  ) {
    final dataList = json['data'] as List? ?? [];

    return PaginatedResponse(
      success: json['success'] as bool? ?? false,
      data: dataList.map((e) => fromJsonT(e as Map<String, dynamic>)).toList(),
      pagination: Pagination.fromJson(
        json['meta'] as Map<String, dynamic>? ?? {},
      ),
      message: json['message'] as String?,
    );
  }

  /// Check if there are more pages
  bool get hasMore => pagination.page < pagination.totalPages;

  /// Check if this is the first page
  bool get isFirstPage => pagination.page == 1;

  /// Check if this is the last page
  bool get isLastPage => pagination.page >= pagination.totalPages;

  /// Check if data is empty
  bool get isEmpty => data.isEmpty;
}

/// Pagination metadata
///
/// @deprecated Use `MetaResponse` instead.
class Pagination {
  final int page;
  final int limit;
  final int total;
  final int totalPages;

  const Pagination({
    required this.page,
    required this.limit,
    required this.total,
    required this.totalPages,
  });

  factory Pagination.fromJson(Map<String, dynamic> json) {
    return Pagination(
      page: json['page'] as int? ?? 1,
      limit: json['per_page'] as int? ?? 20, // Backend uses per_page
      total: json['total'] as int? ?? 0,
      totalPages: json['total_pages'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
    'page': page,
    'limit': limit,
    'total': total,
    'total_pages': totalPages,
  };
}

/// API Error details
class ApiError {
  final String code;
  final String message;
  final Map<String, dynamic>? details;
  final Map<String, List<String>>? fieldErrors;

  const ApiError({
    required this.code,
    required this.message,
    this.details,
    this.fieldErrors,
  });

  factory ApiError.fromJson(dynamic json) {
    if (json is! Map<String, dynamic>) {
      return ApiError(
        code: 'UNKNOWN',
        message: json?.toString() ?? 'Unknown error',
      );
    }

    Map<String, List<String>>? fieldErrors;
    if (json['field_errors'] is Map) {
      fieldErrors = {};
      (json['field_errors'] as Map).forEach((key, value) {
        if (key is String) {
          if (value is List) {
            fieldErrors![key] = value.map((e) => e.toString()).toList();
          } else if (value is String) {
            fieldErrors![key] = [value];
          }
        }
      });
    }

    return ApiError(
      code: json['code'] as String? ?? 'UNKNOWN',
      message: json['message'] as String? ?? 'Unknown error',
      details: json['details'] as Map<String, dynamic>?,
      fieldErrors: fieldErrors,
    );
  }

  @override
  String toString() => 'ApiError(code: $code, message: $message)';
}
