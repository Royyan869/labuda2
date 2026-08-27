import 'package:labuda/features/home/domain/domain.dart'; // R3.1: Import FeedItem from home domain

/// Repository interface untuk Feed aggregation
/// Domain layer - bebas dari implementation details
///
/// FEED OWNERSHIP LOCK (BATCH C2):
/// This is the CANONICAL repository for social timeline feed.
/// Use for: Home screen, social timeline, follow-aware content.
/// For user profile content: Use ContentRepository.getContentsByAuthor instead.
abstract class HomeRepository {
  /// Get a single page of feed items from the Feed domain.
  ///
  /// Returns a [FeedPage] carrying the backend-authoritative `hasMore`
  /// signal. The cursor is repository-owned and remains opaque to the
  /// notifier — pass `loadMore: true` to advance, `loadMore: false` to
  /// fetch the first page.
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  });

  /// Watch feed items as stream
  Stream<List<FeedItem>> watchFeedItems({
    int limit = 20,
    String? currentUserId,
  });

  /// Refresh feed items (resets pagination cursor)
  Future<void> refreshFeedItems();
}
