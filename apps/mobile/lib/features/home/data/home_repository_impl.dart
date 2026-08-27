import 'package:labuda/core/core.dart';
import 'package:labuda/features/home/domain/domain.dart';
import 'package:labuda/features/home/data/data.dart';

/// Repository implementation untuk Home
/// Data layer - uses Feed domain (/api/v1/feed) as canonical source
///
/// FEED OWNERSHIP LOCK (BATCH C2):
/// This is the CANONICAL implementation for social timeline feed.
/// DO NOT use for user profile content - use ContentRepository.getContentsByAuthor.
class HomeRepositoryImpl implements HomeRepository {
  final FeedApiDatasource _feedDatasource;
  final ILoggerService _logger;

  // Store cursor for pagination
  String? _nextCursor;

  HomeRepositoryImpl({
    required FeedApiDatasource feedDatasource,
    required ILoggerService logger,
  }) : _feedDatasource = feedDatasource,
       _logger = logger;

  @override
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  }) async {
    try {
      _logger.info(
        'Fetching feed items from Feed domain: '
        'limit=$limit, cursor=${loadMore ? _nextCursor : "null"}',
      );

      final response = await _feedDatasource.getFeed(
        cursor: loadMore ? _nextCursor : null,
        limit: limit,
      );

      // Store cursor for next pagination
      _nextCursor = response.nextCursor;

      _logger.info(
        'Fetched ${response.data.length} feed items, '
        'hasMore=${response.hasMore}, nextCursor=${response.nextCursor}',
      );

      // P3A — Merge organic items with promoted items at their original
      // interleaved positions from the wire data array.
      final organicItems = response.data.toFeedItems();
      final items = mergeFeedItems(
        organicItems,
        response.promotedItems,
        response.promotedSlotIndices,
      );

      return FeedPage(
        items: items,
        hasMore: response.hasMore,
        nextCursor: response.nextCursor,
      );
    } catch (e) {
      _logger.error('Failed to fetch feed items: $e');
      rethrow;
    }
  }

  @override
  Stream<List<FeedItem>> watchFeedItems({
    int limit = 20,
    String? currentUserId,
  }) async* {
    // Feed domain doesn't support streaming yet
    // Yield initial fetch
    final page = await getFeedPage(limit: limit, currentUserId: currentUserId);
    yield page.items;
  }

  @override
  Future<void> refreshFeedItems() async {
    // Reset cursor to fetch from beginning
    _nextCursor = null;
    _logger.info('Refresh feed items requested - cursor reset');
  }
}
