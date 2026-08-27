// ignore_for_file: unused_local_variable

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/features/home/data/dto/feed_dto.dart';
import 'package:labuda/features/home/data/mappers/feed_mapper.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/domain/entities/feed_page.dart';
import 'package:labuda/features/home/domain/repositories/home_repository.dart';
import 'package:labuda/features/home/presentation/providers/feed/feed_notifier.dart';
import 'package:labuda/features/home/presentation/providers/feed/feed_state.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/services/logger_service.dart';

// ============================================================================
// Fakes
// ============================================================================

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthStateUnauthenticated();
}

class _FakeHomeRepository implements HomeRepository {
  _FakeHomeRepository({this.initialPages = const [], this.loadMorePages = const []});

  final List<FeedPage> initialPages;
  final List<FeedPage> loadMorePages;

  int initialCalls = 0;
  int loadMoreCalls = 0;
  int refreshCalls = 0;

  @override
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  }) async {
    if (loadMore) {
      final i = loadMoreCalls++;
      final pages = loadMorePages;
      return pages[i < pages.length ? i : pages.length - 1];
    }
    final i = initialCalls++;
    final pages = initialPages;
    return pages[i < pages.length ? i : pages.length - 1];
  }

  @override
  Stream<List<FeedItem>> watchFeedItems({
    int limit = 20,
    String? currentUserId,
  }) {
    return const Stream<List<FeedItem>>.empty();
  }

  @override
  Future<void> refreshFeedItems() async {
    refreshCalls += 1;
  }
}

class _ThrowingHomeRepository implements HomeRepository {
  final Object exception;
  int callCount = 0;
  _ThrowingHomeRepository(this.exception);

  @override
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  }) async {
    callCount++;
    throw exception;
  }

  @override
  Stream<List<FeedItem>> watchFeedItems({
    int limit = 20,
    String? currentUserId,
  }) async* {
    throw exception;
  }

  @override
  Future<void> refreshFeedItems() async {
    throw exception;
  }
}

/// Fake that may throw based on a toggleable flag. Used to simulate
/// retry flows without swapping repositories (which Riverpod forbids).
class _ToggleHomeRepository implements HomeRepository {
  _ToggleHomeRepository({
    this.initialPages = const [],
    this.loadMorePages = const [],
  });

  final List<FeedPage> initialPages;
  final List<FeedPage> loadMorePages;

  int initialCalls = 0;
  int loadMoreCalls = 0;
  int refreshCalls = 0;

  /// When non-null, the next getFeedPage call throws this.
  /// Reset to null after throwing so subsequent calls succeed.
  Object? nextError;

  /// When true, refreshFeedItems throws.
  bool refreshThrows = false;

  @override
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  }) async {
    final error = nextError;
    nextError = null;
    if (error != null) throw error;

    if (loadMore) {
      final i = loadMoreCalls++;
      final pages = loadMorePages;
      return pages[i < pages.length ? i : pages.length - 1];
    }
    final i = initialCalls++;
    final pages = initialPages;
    return pages[i < pages.length ? i : pages.length - 1];
  }

  @override
  Stream<List<FeedItem>> watchFeedItems({
    int limit = 20,
    String? currentUserId,
  }) {
    return const Stream<List<FeedItem>>.empty();
  }

  @override
  Future<void> refreshFeedItems() async {
    refreshCalls += 1;
    if (refreshThrows) throw Exception('refresh failed');
  }
}

// ============================================================================
// Helpers
// ============================================================================

const _authorId = '00000000-0000-0000-0000-000000000123';

FeedItem _feedItem({
  required String id,
  required String content,
  ContentAuthor? author,
}) {
  return FeedItem(
    id: id,
    content: content,
    author: author ?? _activeAuthor(),
    type: FeedItemType.content,
    createdAt: DateTime.utc(2026, 7, 23, 10, 0),
    additionalData: const {'status': 'active'},
  );
}

ContentAuthor _activeAuthor() {
  return const ContentAuthor(
    id: _authorId,
    username: 'alice',
    avatarUrl: 'https://example.com/avatar.jpg',
    lifecycle: ContentLifecycle.active,
  );
}

Future<void> _settle() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
}

void main() {
  // ==========================================================================
  // INITIAL LOAD
  // ==========================================================================
  group('initial load', () {
    test('success with items', () async {
      final repo = _FakeHomeRepository(initialPages: [
        FeedPage(
          items: [_feedItem(id: 'feed-1', content: 'hello'), _feedItem(id: 'feed-2', content: 'world')],
          hasMore: true,
          nextCursor: 'cursor-1',
        ),
      ]);
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      final state = container.read(feedProvider);
      expect(state.items, hasLength(2));
      expect(state.items[0].id, 'feed-1');
      expect(state.items[1].id, 'feed-2');
      expect(state.isLoading, isFalse);
      expect(state.errorMessage, isNull);
      expect(state.errorKind, isNull);
      expect(state.hasReachedMax, isFalse);
      expect(repo.initialCalls, 1);
    });

    test('success with empty list — genuine empty', () async {
      final repo = _FakeHomeRepository(initialPages: [
        const FeedPage(items: [], hasMore: false),
      ]);
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      final state = container.read(feedProvider);
      expect(state.items, isEmpty);
      expect(state.isLoading, isFalse);
      expect(state.errorMessage, isNull);
      expect(state.errorKind, isNull);
      expect(state.hasReachedMax, isTrue);
    });

    test('FormatException from repository produces initial error, not genuine empty', () async {
      // Simulates the exact data:null contract path:
      // FeedApiDatasource → FeedResponseDto.fromJson(data:null) → FormatException
      // → HomeRepositoryImpl (rethrows) → FeedNotifier.loadFeed (catches)
      final repo = _ThrowingHomeRepository(
        const FormatException('Feed response data must be a JSON array'),
      );
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      final state = container.read(feedProvider);
      expect(state.items, isEmpty);
      expect(state.isLoading, isFalse);
      expect(state.errorMessage, isNotNull);
      expect(state.errorKind, FeedErrorKind.initial);
    });

    test('repository failure is not genuine empty', () async {
      final repo = _ThrowingHomeRepository(Exception('network error'));
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      final state = container.read(feedProvider);
      expect(state.items, isEmpty);
      expect(state.isLoading, isFalse);
      expect(state.errorMessage, isNotNull);
      expect(state.errorKind, FeedErrorKind.initial);
    });

    test('retry after initial failure recovers to data', () async {
      final repo = _ToggleHomeRepository(
        initialPages: [
          FeedPage(items: [_feedItem(id: 'feed-1', content: 'recovered')], hasMore: false),
        ],
      );
      // First call (initial load in build) throws
      repo.nextError = Exception('network error');

      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      // Verify initial failure
      expect(container.read(feedProvider).errorKind, FeedErrorKind.initial);
      expect(container.read(feedProvider).items, isEmpty);

      // Retry via refresh — nextError was consumed, so this call succeeds
      final notifier = container.read(feedProvider.notifier);
      await notifier.refresh();

      final state = container.read(feedProvider);
      expect(state.items, hasLength(1));
      expect(state.items[0].id, 'feed-1');
      expect(state.errorMessage, isNull);
      expect(state.errorKind, isNull);
    });
  });

  // ==========================================================================
  // REFRESH
  // ==========================================================================
  group('refresh', () {
    test('refresh success updates state', () async {
      final repo = _FakeHomeRepository(initialPages: [
        FeedPage(items: [_feedItem(id: 'feed-1', content: 'old')], hasMore: true, nextCursor: 'cursor-1'),
        FeedPage(items: [_feedItem(id: 'feed-2', content: 'new')], hasMore: false),
      ]);
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      expect(container.read(feedProvider).items[0].id, 'feed-1');

      final notifier = container.read(feedProvider.notifier);
      await notifier.refresh();

      final state = container.read(feedProvider);
      expect(state.items, hasLength(1));
      expect(state.items[0].id, 'feed-2');
      expect(state.isLoading, isFalse);
      expect(state.errorMessage, isNull);
      expect(state.errorKind, isNull);
      expect(state.hasReachedMax, isTrue);
      expect(repo.refreshCalls, 1);
      expect(repo.initialCalls, 2);
    });

    test('refresh failure preserves last-good items', () async {
      final repo = _ToggleHomeRepository(
        initialPages: [
          FeedPage(items: [_feedItem(id: 'feed-1', content: 'hello')], hasMore: true, nextCursor: 'cursor-1'),
        ],
      );
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      // Initial load succeeded
      expect(container.read(feedProvider).items, hasLength(1));

      // Next call (refresh via loadFeed) throws
      repo.nextError = Exception('refresh failed');
      final notifier = container.read(feedProvider.notifier);
      await notifier.refresh();

      final state = container.read(feedProvider);
      // Items preserved from last-good fetch
      expect(state.items, hasLength(1));
      expect(state.items[0].content, 'hello');
      expect(state.errorMessage, isNotNull);
      expect(state.errorKind, FeedErrorKind.refresh);
      expect(state.isLoading, isFalse);
    });

    test('refresh failure does not produce genuine empty', () async {
      final repo = _ToggleHomeRepository(
        initialPages: [
          FeedPage(items: [_feedItem(id: 'feed-1', content: 'hello')], hasMore: true),
        ],
      );
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      // Refresh fails
      repo.nextError = Exception('error');
      final notifier = container.read(feedProvider.notifier);
      await notifier.refresh();

      final state = container.read(feedProvider);
      expect(state.items, isNotEmpty);
      expect(state.errorMessage, isNotNull);
      expect(state.errorKind, FeedErrorKind.refresh);
    });

    test('retry refresh clears error', () async {
      final repo = _ToggleHomeRepository(
        initialPages: [
          FeedPage(items: [_feedItem(id: 'feed-1', content: 'hello')], hasMore: true),
          FeedPage(items: [_feedItem(id: 'feed-2', content: 'fresh')], hasMore: false),
        ],
      );
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      // Refresh fails
      repo.nextError = Exception('fail');
      await container.read(feedProvider.notifier).refresh();
      expect(container.read(feedProvider).errorKind, FeedErrorKind.refresh);

      // Retry: nextError was consumed, this call succeeds
      await container.read(feedProvider.notifier).refresh();

      final state = container.read(feedProvider);
      expect(state.items, isNotEmpty);
      expect(state.errorMessage, isNull);
      expect(state.errorKind, isNull);
    });
  });

  // ==========================================================================
  // PAGINATION
  // ==========================================================================
  group('pagination', () {
    test('pagination success adds page', () async {
      final repo = _FakeHomeRepository(
        initialPages: [
          FeedPage(items: [_feedItem(id: 'feed-1', content: 'page-1')], hasMore: true, nextCursor: 'cursor-1'),
        ],
        loadMorePages: [
          FeedPage(items: [_feedItem(id: 'feed-2', content: 'page-2')], hasMore: false),
        ],
      );
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      final notifier = container.read(feedProvider.notifier);
      await notifier.loadMore();

      final state = container.read(feedProvider);
      expect(state.items, hasLength(2));
      expect(state.items[0].id, 'feed-1');
      expect(state.items[1].id, 'feed-2');
      expect(state.hasReachedMax, isTrue);
      expect(state.isLoadingMore, isFalse);
      expect(repo.loadMoreCalls, 1);
    });

    test('pagination failure preserves old pages', () async {
      final repo = _ToggleHomeRepository(
        initialPages: [
          FeedPage(items: [_feedItem(id: 'feed-1', content: 'page-1')], hasMore: true, nextCursor: 'cursor-1'),
        ],
        loadMorePages: [
          FeedPage(items: [_feedItem(id: 'feed-2', content: 'page-2')], hasMore: false),
        ],
      );
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      // loadMore throws
      repo.nextError = Exception('pagination error');
      final notifier = container.read(feedProvider.notifier);
      await notifier.loadMore();

      final state = container.read(feedProvider);
      expect(state.items, hasLength(1));
      expect(state.items[0].id, 'feed-1');
      expect(state.isLoadingMore, isFalse);
      expect(state.errorKind, FeedErrorKind.pagination);
    });

    test('pagination failure preserves cursor authority', () async {
      final repo = _ToggleHomeRepository(
        initialPages: [
          FeedPage(items: [_feedItem(id: 'feed-1', content: 'page-1')], hasMore: true, nextCursor: 'cursor-1'),
        ],
        loadMorePages: [
          FeedPage(items: [_feedItem(id: 'feed-2', content: 'page-2')], hasMore: true, nextCursor: 'cursor-2'),
        ],
      );
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      // Pagination fails — first loadMore throws
      repo.nextError = Exception('error');
      await container.read(feedProvider.notifier).loadMore();

      // hasReachedMax should remain false — we didn't exhaust the feed, we just failed
      expect(container.read(feedProvider).hasReachedMax, isFalse);
      expect(container.read(feedProvider).errorKind, FeedErrorKind.pagination);
    });

    test('retry pagination succeeds', () async {
      final repo = _ToggleHomeRepository(
        initialPages: [
          FeedPage(items: [_feedItem(id: 'feed-1', content: 'page-1')], hasMore: true, nextCursor: 'cursor-1'),
        ],
        loadMorePages: [
          FeedPage(items: [_feedItem(id: 'feed-2', content: 'page-2')], hasMore: false),
        ],
      );
      final container = ProviderContainer(overrides: [
        homeRepositoryProvider.overrideWithValue(repo),
        loggerServiceProvider.overrideWithValue(LoggerService.instance),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ]);
      addTearDown(container.dispose);
      container.listen(feedProvider, (_, _) {});
      await _settle();

      final notifier = container.read(feedProvider.notifier);

      // First loadMore fails
      repo.nextError = Exception('fail');
      await notifier.loadMore();
      expect(container.read(feedProvider).errorKind, FeedErrorKind.pagination);
      expect(container.read(feedProvider).items, hasLength(1));

      // Second loadMore succeeds (nextError was consumed)
      await notifier.loadMore();

      final state = container.read(feedProvider);
      expect(state.items, hasLength(2));
      expect(state.items[1].id, 'feed-2');
      expect(state.errorKind, isNull);
      expect(state.hasReachedMax, isTrue);
    });
  });

  // ==========================================================================
  // PARSING CONTRACT (DTO boundary)
  // ==========================================================================
  group('parsing contract', () {
    test('missing feed_item_kind throws FormatException', () {
      expect(
        () => FeedResponseDto.fromJson({
          'data': [
            {'id': 'some-id', 'body': 'content'}, // no feed_item_kind
          ],
          'next_cursor': null,
          'has_more': false,
        }),
        throwsA(isA<FormatException>().having(
          (e) => e.message,
          'message',
          contains('missing feed_item_kind'),
        )),
      );
    });

    test('unknown feed_item_kind throws FormatException', () {
      expect(
        () => FeedResponseDto.fromJson({
          'data': [
            {'feed_item_kind': 'unknown_type', 'id': 'some-id', 'body': 'content'},
          ],
          'next_cursor': null,
          'has_more': false,
        }),
        throwsA(isA<FormatException>().having(
          (e) => e.message,
          'message',
          contains('Unknown feed_item_kind'),
        )),
      );
    });

    test('non-Map item in data array throws FormatException', () {
      expect(
        () => FeedResponseDto.fromJson({
          'data': ['not_a_map'],
          'next_cursor': null,
          'has_more': false,
        }),
        throwsA(isA<FormatException>().having(
          (e) => e.message,
          'message',
          contains('not a JSON object'),
        )),
      );
    });

    test('malformed organic item (missing required fields) throws', () {
      expect(
        () => FeedResponseDto.fromJson({
          'data': [
            // feed_item_kind present but missing id and other required fields
            // json_serializable will fail on the generated parser
            {'feed_item_kind': 'content'},
          ],
          'next_cursor': null,
          'has_more': false,
        }),
        throwsA(isA<TypeError>()),
      );
    });

    test('empty feed_item_kind throws FormatException', () {
      expect(
        () => FeedResponseDto.fromJson({
          'data': [
            {'feed_item_kind': '', 'id': 'x', 'body': 'y'},
          ],
          'next_cursor': null,
          'has_more': false,
        }),
        throwsA(isA<FormatException>().having(
          (e) => e.message,
          'message',
          contains('missing feed_item_kind'),
        )),
      );
    });

    test('data: null throws FormatException', () {
      expect(
        () => FeedResponseDto.fromJson({
          'data': null,
          'next_cursor': null,
          'has_more': false,
        }),
        throwsA(isA<FormatException>().having(
          (e) => e.message,
          'message',
          contains('must be a JSON array'),
        )),
      );
    });

    test('data key absent throws FormatException', () {
      expect(
        () => FeedResponseDto.fromJson({
          'next_cursor': null,
          'has_more': false,
        }),
        throwsA(isA<FormatException>().having(
          (e) => e.message,
          'message',
          contains('must be a JSON array'),
        )),
      );
    });

    test('valid organic content items parse successfully', () {
      final response = FeedResponseDto.fromJson({
        'data': [
          {
            'feed_item_kind': 'content',
            'id': 'feed-1',
            'status': 'active',
            'body': 'hello world',
            'created_at': '2026-07-23T10:00:00Z',
            'updated_at': '2026-07-23T10:00:00Z',
            'author': {
              'id': 'author-1',
              'username': 'alice',
              'avatar_url': 'https://example.com/avatar.jpg',
            },
            'media': [],
          },
        ],
        'next_cursor': 'cursor-1',
        'has_more': true,
      });
      expect(response.data, hasLength(1));
      expect(response.data[0].id, 'feed-1');
      expect(response.data[0].feedItemKind, 'content');
      expect(response.nextCursor, 'cursor-1');
      expect(response.hasMore, isTrue);
    });

    test('valid promoted items are extracted into promotedItems list', () {
      final response = FeedResponseDto.fromJson({
        'data': [
          {
            'feed_item_kind': 'promoted_fixed_price_sale',
            'promotion_instance_id': 'pi-1',
            'target_type': 'listing',
            'title': 'Nice Koi',
          },
          {
            'feed_item_kind': 'content',
            'id': 'feed-1',
            'status': 'active',
            'body': 'hello',
            'created_at': '2026-07-23T10:00:00Z',
            'updated_at': '2026-07-23T10:00:00Z',
            'author': {'id': 'author-1'},
            'media': [],
          },
          {
            'feed_item_kind': 'promoted_auction',
            'promotion_instance_id': 'pi-2',
            'target_type': 'auction',
            'title': 'Auction Item',
          },
        ],
        'next_cursor': null,
        'has_more': false,
      });
      expect(response.data, hasLength(1)); // only organic
      expect(response.data[0].feedItemKind, 'content');
      expect(response.promotedItems, hasLength(2));
      expect(response.promotedItems[0].feedItemKind, 'promoted_fixed_price_sale');
      expect(response.promotedItems[1].feedItemKind, 'promoted_auction');
      expect(response.promotedSlotIndices, [0, 2]);
    });
  });

  // ==========================================================================
  // MAPPER CONTRACT
  // ==========================================================================
  group('mapper contract', () {
    test('organic FeedItemDto with content kind maps successfully', () {
      final dto = FeedItemDto(
        id: 'feed-1',
        feedItemKind: 'content',
        status: 'active',
        body: 'hello',
        createdAt: DateTime.utc(2026, 7, 23),
        updatedAt: DateTime.utc(2026, 7, 23),
        author: const FeedAuthorDto(id: 'author-1', username: 'alice'),
      );
      final item = dto.toFeedItem();
      expect(item.id, 'feed-1');
      expect(item.type, FeedItemType.content);
      expect(item.content, 'hello');
    });

    test('organic FeedItemDto with non-content kind throws FormatException', () {
      final dto = FeedItemDto(
        id: 'feed-1',
        feedItemKind: 'something_else',
        status: 'active',
        body: 'hello',
        createdAt: DateTime.utc(2026, 7, 23),
        updatedAt: DateTime.utc(2026, 7, 23),
      );
      expect(
        () => dto.toFeedItem(),
        throwsA(isA<FormatException>().having(
          (e) => e.message,
          'message',
          contains('non-content feed_item_kind'),
        )),
      );
    });

    test('promoted type with unknown kind throws FormatException', () {
      final dto = PromotedFeedItemDto(
        feedItemKind: 'promoted_unknown',
        promotionInstanceId: 'pi-1',
        targetType: 'listing',
      );
      expect(
        () => dto.toFeedItem(),
        throwsA(isA<FormatException>().having(
          (e) => e.message,
          'message',
          contains('Unknown promoted feed_item_kind'),
        )),
      );
    });
  });
}
