// Feed API Datasource
// HTTP operations for Feed domain - isolates ApiClient

import 'package:labuda/core/api/api.dart';
import 'package:labuda/features/home/data/dto/feed_dto.dart';

/// Feed API Datasource
///
/// FEED OWNERSHIP LOCK (BATCH C2):
/// This is the CANONICAL source for social timeline feed.
/// Use this for:
/// - Home screen feed
/// - Social timeline (followed users' content)
/// - Follow-aware feed projections
///
/// DO NOT use for:
/// - User profile content listing (use ContentRepository.getContentsByAuthor)
/// - Generic content browsing (use ContentRepository.getContents)
/// - Content search (use ContentRepository.searchContents)
///
/// The Feed domain provides:
/// - Content from followed users only
/// - Excludes blocked users
/// - Cursor-based pagination
/// - Social content only (universal content and reposts)
class FeedApiDatasource {
  final ApiClient _apiClient;

  FeedApiDatasource(this._apiClient);

  /// Get feed from Feed domain (/api/v1/feed)
  ///
  /// CANONICAL SOCIAL TIMELINE: This is the correct endpoint for home feed.
  ///
  /// Fetches personalized feed for the authenticated user.
  /// Uses cursor-based pagination for efficient scrolling.
  ///
  /// Parameters:
  /// - [cursor]: RFC3339 timestamp for pagination (optional)
  /// - [limit]: Number of items to return (default 20, max 50)
  ///
  /// Returns [FeedResponseDto] with feed items and pagination info.
  Future<FeedResponseDto> getFeed({String? cursor, int limit = 20}) async {
    final params = <String, dynamic>{'limit': limit};
    if (cursor != null && cursor.isNotEmpty) {
      params['cursor'] = cursor;
    }

    final response = await _apiClient.get('/feed', queryParameters: params);

    // Backend wraps every response in the platform envelope
    // { success, data: { ... }, timestamp }. The feed payload (data/items,
    // next_cursor, has_more) lives at envelope.data. Unwrap before parsing.
    final body = response.data;
    if (body is! Map<String, dynamic>) {
      throw const FormatException('Feed response body is not a JSON object');
    }
    final payload = body['data'];
    if (payload is! Map<String, dynamic>) {
      throw const FormatException(
        'Feed response envelope missing data payload',
      );
    }
    return FeedResponseDto.fromJson(payload);
  }
}
