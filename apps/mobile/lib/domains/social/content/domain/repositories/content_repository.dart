// Content Repository Interface
// Domain layer - pure Dart, bebas dari implementation details

import 'package:labuda/domains/social/content/domain/entities/content.dart';

/// Interface repository untuk content management.
///
/// Mengikuti Clean Architecture - interface di domain,
/// implementation di data layer.
abstract class ContentRepository {
  // ==========================================================================
  // CRUD Operations
  // ==========================================================================

  /// Create new content
  ///
  /// Returns [Content] jika berhasil, atau error message jika gagal
  /// CLEANUP V1: Removed shippingCity/shippingProvince - use location.city/province instead
  Future<ContentRepositoryResult<Content>> createContent({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    List<MediaEntity> media = const [],
    List<String> tags = const [],
    List<String> mentionedUserIds = const [],
    ContentSettings settings = const ContentSettings(),
    ContentLocation? location,
  });

  /// Get content by ID
  Future<ContentRepositoryResult<Content>> getContentById(String contentId);

  /// Get contents by author with pagination
  Future<ContentRepositoryResult<List<Content>>> getContentsByAuthor(
    String authorId, {
    int? limit,
    int? offset,
  });

  /// Get contents by author with cursor pagination (C3B — profile feed tab).
  ///
  /// Returns a [ContentAuthorPage] envelope carrying the item list,
  /// an opaque [ContentAuthorPage.nextCursor] for the next page, and
  /// [ContentAuthorPage.hasMore]. Pass [nextCursor] verbatim as [cursor]
  /// on subsequent calls; omit [cursor] on the initial fetch.
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  });

  /// Get all published contents with filters
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
    String? location,
    ContentStatus? status,
  });

  /// Update content
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  );

  /// Delete content
  Future<ContentRepositoryResult<void>> deleteContent(String contentId);

  // ==========================================================================
  // Discovery & Search
  // ==========================================================================

  // FEED OWNERSHIP LOCK (BATCH C2):
  // ContentRepository does NOT provide a "feed" method for social timeline.
  // For social timeline (followed users), use Home/Feed domain instead.
  // This repository handles content-centric operations only.

  /// Search contents by text query
  Future<ContentRepositoryResult<ContentSearchResult>> searchContents({
    required String query,
    int? limit,
    int? offset,
    String? location,
  });

  /// Get trending contents (most engagement)
  Future<ContentRepositoryResult<List<Content>>> getTrendingContents({
    int? limit,
  });

  /// Get contents by location
  Future<ContentRepositoryResult<List<Content>>> getContentsByLocation({
    required String location,
    int? limit,
  });

  // ==========================================================================
  // Engagement
  // ==========================================================================

  // Note: View tracking is handled by backend automatically via GET /content/contents/:id
  // No explicit incrementViewCount needed
}

// ============================================================================
// Result Types
// ============================================================================

/// Result type untuk repository operations
/// Menggunakan pattern Either (dartz) atau custom Result
class ContentRepositoryResult<T> {
  final T? data;
  final String? error;

  const ContentRepositoryResult._({this.data, this.error});

  /// Create success result
  factory ContentRepositoryResult.success(T data) {
    return ContentRepositoryResult._(data: data);
  }

  /// Create error result
  factory ContentRepositoryResult.error(String error) {
    return ContentRepositoryResult._(error: error);
  }

  /// Check if result is success
  bool get isSuccess => error == null;

  /// Check if result is error
  bool get isError => error != null;

  /// Fold pattern - transform result based on success/error
  R fold<R>(R Function(String error) onError, R Function(T data) onSuccess) {
    if (isError) {
      return onError(error!);
    }
    return onSuccess(data as T);
  }

  /// Get data or throw if error
  T get dataOrThrow {
    if (isError) {
      throw Exception(error);
    }
    return data as T;
  }
}

// ============================================================================
// DTOs untuk request/response kompleks
// ============================================================================

/// Search result dengan metadata
class ContentSearchResult {
  final List<Content> contents;
  final int total;
  final int limit;
  final int offset;
  final String query;

  const ContentSearchResult({
    required this.contents,
    required this.total,
    required this.limit,
    required this.offset,
    required this.query,
  });
}

/// Cursor-paginated result for profile content listing.
///
/// Returned by [ContentRepository.getContentsByAuthorPaged].
/// [nextCursor] is opaque; pass verbatim as [cursor] on the next call.
/// [nextCursor] is null on the terminal page.
class ContentAuthorPage {
  final List<Content> items;
  final String? nextCursor;
  final bool hasMore;

  const ContentAuthorPage({
    required this.items,
    required this.nextCursor,
    required this.hasMore,
  });
}

// FeedResult removed (BATCH C2):
// Content domain does not provide "feed" functionality.
// For social timeline, use Home/Feed domain (/api/v1/feed).
// For content listing, use specific methods: getContents(), getContentsByAuthor(), etc.
