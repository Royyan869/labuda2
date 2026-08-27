import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/home/domain/domain.dart';
import 'package:labuda/features/home/data/data.dart';
import 'feed_state.dart';

part 'feed_notifier.g.dart';

/// Provider untuk FeedApiDatasource
/// Uses Feed domain (/api/v1/feed) as canonical source
@riverpod
FeedApiDatasource feedApiDatasource(Ref ref) {
  final apiClient = ref.watch(apiClientProvider);
  return FeedApiDatasource(apiClient);
}

/// Provider untuk HomeRepository
/// Uses Feed domain as canonical source for Home feed
@riverpod
HomeRepository homeRepository(Ref ref) {
  final feedDatasource = ref.watch(feedApiDatasourceProvider);
  final logger = ref.watch(loggerServiceProvider);

  return HomeRepositoryImpl(feedDatasource: feedDatasource, logger: logger);
}

/// Notifier untuk Home Feed management
/// Application layer - menggantikan UseCase classes
@riverpod
class FeedNotifier extends _$FeedNotifier {
  // Concurrency guards to prevent race conditions
  bool _isLoadingFeed = false;
  bool _isLoadingMore = false;
  bool _isRefreshing = false;

  // Defensive guard against infinite-loop pagination. If the backend
  // ever returns the same cursor twice in a row (server bug, broken
  // cursor doctrine, or repeated empty page), we stop advancing.
  String? _lastSeenCursor;

  @override
  FeedState build() {
    // Auto-load feed saat pertama kali dibuat
    Future.microtask(() => loadFeed());

    return const FeedState();
  }

  HomeRepository get _repository => ref.read(homeRepositoryProvider);
  ILoggerService get _logger => ref.read(loggerServiceProvider);

  /// Load feed items (initial page)
  Future<void> loadFeed({int limit = 20}) async {
    // Guard against concurrent calls
    if (_isLoadingFeed) return;

    try {
      _isLoadingFeed = true;
      state = state.copyWith(isLoading: true, errorMessage: null);

      final currentUserId = _getCurrentUserId();

      final page = await _repository.getFeedPage(
        limit: limit,
        currentUserId: currentUserId,
        loadMore: false, // Initial load, not pagination
      );

      _lastSeenCursor = page.nextCursor;

      state = state.copyWith(
        items: page.items,
        isLoading: false,
        // Backend has_more is authoritative — do NOT derive from
        // items.length < limit (premature exhaustion if backend ever
        // returns a short page with more available, e.g. evaluator
        // post-filter).
        hasReachedMax: !page.hasMore,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        errorMessage: 'Coba lagi beberapa saat.',
      );
    } finally {
      _isLoadingFeed = false;
    }
  }

  /// Refresh feed items
  Future<void> refresh() async {
    // Guard against concurrent refresh calls
    if (_isRefreshing) return;

    try {
      _isRefreshing = true;
      await _repository.refreshFeedItems();
      _lastSeenCursor = null;
      // Reset exhaustion + items so a refreshing pass starts clean.
      // loadFeed() will overwrite items on success; the explicit reset
      // here guarantees we don't carry stale hasReachedMax / leftover
      // items into the failure case.
      state = state.copyWith(items: const [], hasReachedMax: false);
      await loadFeed();
    } finally {
      _isRefreshing = false;
    }
  }

  /// Load more items (pagination)
  Future<void> loadMore({int limit = 20}) async {
    // Guard against concurrent pagination
    if (_isLoadingMore) return;
    if (state.isLoading || state.hasReachedMax) return;

    try {
      _isLoadingMore = true;
      // Surface the in-flight pagination to the UI so the scroll trigger
      // and bottom spinner can debounce against it. The private flag
      // above remains the synchronous lock — state.isLoadingMore is the
      // public, rebuild-driven mirror.
      state = state.copyWith(isLoadingMore: true);
      final currentUserId = _getCurrentUserId();

      final page = await _repository.getFeedPage(
        limit: limit,
        currentUserId: currentUserId,
        loadMore: true, // Pagination mode
      );

      // Dedupe by feed item id. Backend cursor today is created_at-only
      // with strict `<`, so a tie at the page boundary can either skip
      // or repeat items. We never want visible duplicates in the list,
      // and we must distinguish "all-dup page with more available" from
      // "genuine end of feed".
      final seenIds = state.items.map((e) => e.id).toSet();
      final uniqueItems = page.items
          .where((item) => !seenIds.contains(item.id))
          .toList();

      // Defensive cursor-stall guard. If backend keeps returning the
      // same cursor with no new unique items, we exhaust to avoid an
      // infinite loadMore loop, regardless of what has_more claims.
      final cursorRepeated =
          page.nextCursor != null &&
          page.nextCursor == _lastSeenCursor &&
          uniqueItems.isEmpty;
      _lastSeenCursor = page.nextCursor;

      // Backend has_more is the authority for exhaustion. Override only
      // when we detect the cursor-stall pathology above.
      final reachedMax = !page.hasMore || cursorRepeated;

      state = state.copyWith(
        items: uniqueItems.isEmpty
            ? state.items
            : [...state.items, ...uniqueItems],
        hasReachedMax: reachedMax,
        isLoadingMore: false,
      );
    } catch (e) {
      // Pagination errors are handled silently to avoid disrupting scroll UX
      // User can still refresh manually if content stops loading
      // Error is logged for monitoring/debugging
      _logger.error('Feed pagination error', extra: {'error': e.toString()});
      state = state.copyWith(isLoadingMore: false);
    } finally {
      _isLoadingMore = false;
    }
  }

  /// Get current user ID from auth state
  String? _getCurrentUserId() {
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated) {
      return authState.user.id;
    }
    return null;
  }
}

// Riverpod generates feedProvider automatically for FeedNotifier class
// For AsyncValue pattern, use: ref.watch(feedProvider)
