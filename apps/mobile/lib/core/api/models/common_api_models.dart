// Common API Models for Go Backend Integration
//
// These are shared DTOs used across multiple features.
// They match the Go backend DTOs and should be kept in sync.
//
// Backend references:
// - backend/internal/social/ (publiccard.UserCard is the canonical public identity type)

import 'package:equatable/equatable.dart';

/// Brief user information for social contexts
///
/// Used across Follow, Like, Rating, Comment features.
/// Public identity is username-only.
class UserBriefApiResponse extends Equatable {
  final String id;
  final String? username;
  final String? avatar;

  const UserBriefApiResponse({required this.id, this.username, this.avatar});

  factory UserBriefApiResponse.fromJson(Map<String, dynamic> json) {
    return UserBriefApiResponse(
      id: json['id'] as String,
      username: json['username'] as String?,
      avatar: json['avatar'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    if (username != null) 'username': username,
    if (avatar != null) 'avatar': avatar,
  };

  @override
  List<Object?> get props => [id, username, avatar];
}

/// Paginated response wrapper
///
/// Generic pagination wrapper used across all API endpoints.
/// Backend returns paginated data in this format:
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
class PaginatedApiResponse<T> extends Equatable {
  final List<T> data;
  final int page;
  final int pageSize;
  final int totalItems;
  final int totalPages;
  final String? timestamp;

  const PaginatedApiResponse({
    required this.data,
    required this.page,
    required this.pageSize,
    required this.totalItems,
    required this.totalPages,
    this.timestamp,
  });

  factory PaginatedApiResponse.fromJson(
    Map<String, dynamic> json,
    T Function(dynamic) itemParser,
  ) {
    // Read from nested meta object (backend structure)
    final meta = json['meta'] as Map<String, dynamic>? ?? {};

    return PaginatedApiResponse(
      data: (json['data'] as List<dynamic>?)?.map(itemParser).toList() ?? [],
      page: meta['page'] as int? ?? 1,
      pageSize: meta['per_page'] as int? ?? 20,
      totalItems: (meta['total'] as num?)?.toInt() ?? 0,
      totalPages: meta['total_pages'] as int? ?? 0,
      timestamp: json['timestamp'] as String?,
    );
  }

  /// Check if there's a next page
  bool get hasNextPage => page < totalPages;

  /// Check if this is the first page
  bool get isFirstPage => page == 1;

  /// Check if this is the last page
  bool get isLastPage => page >= totalPages;

  /// Check if data is empty
  bool get isEmpty => data.isEmpty;

  @override
  List<Object?> get props => [
    data.length,
    page,
    pageSize,
    totalItems,
    totalPages,
    timestamp,
  ];
}
