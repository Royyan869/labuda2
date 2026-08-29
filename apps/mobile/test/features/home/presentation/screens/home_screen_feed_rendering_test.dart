import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:visibility_detector/visibility_detector.dart';

// ============================================================================
// Canonical Home/Feed pipeline
//
//   _FakeFeedHttpAdapter → ApiClient → apiClientProvider
//   → FeedApiDatasource → FeedNotifier → HomeScreen → FeedCardFactory
//
// ONLY the HTTP transport is overridden. Downstream is production code.
// ============================================================================

Map<String, dynamic> _feedEnvelope({
  required List<Map<String, dynamic>> items,
  String? nextCursor,
  bool hasMore = false,
}) {
  return <String, dynamic>{
    'data': <String, dynamic>{
      'data': items,
      'next_cursor': nextCursor,
      'has_more': hasMore,
    },
  };
}

Map<String, dynamic> _feedContentItem({
  required String id,
  required String body,
  String authorId = 'author-1',
  String authorUsername = 'alice',
  String createdAt = '2026-08-05T10:00:00Z',
}) {
  return <String, dynamic>{
    'type': 'post',
    'id': id,
    'status': 'active',
    'body': body,
    'created_at': createdAt,
    'updated_at': createdAt,
    'author': <String, dynamic>{
      'id': authorId,
      'username': authorUsername,
      'avatar_url': 'https://example.com/avatar.jpg',
      'lifecycle': 'active',
    },
    'media': <Map<String, dynamic>>[],
  };
}

class _CannedResponse {
  final int statusCode;
  final Map<String, dynamic>? body;
  const _CannedResponse({required this.statusCode, this.body});
}

/// Dio HttpClientAdapter that returns canned /feed responses from a queue.
class _FakeFeedHttpAdapter implements HttpClientAdapter {
  final List<_CannedResponse> _responses;
  int _callCount = 0;
  Completer<void>? blockUntil;

  _FakeFeedHttpAdapter(List<_CannedResponse> responses)
    : _responses = responses;

  ResponseBody _genericSuccess() {
    return ResponseBody.fromString(
      jsonEncode(<String, dynamic>{
        'success': true,
        'data': <String, dynamic>{},
        'timestamp': '2026-08-05T00:00:00Z',
      }),
      200,
      headers: {
        'content-type': ['application/json'],
      },
    );
  }

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (!options.path.contains('feed') && !options.path.contains('/feed')) {
      return _genericSuccess();
    }

    final blocker = blockUntil;
    if (blocker != null && !blocker.isCompleted) {
      await blocker.future;
    }

    if (_callCount >= _responses.length) {
      final body = _feedEnvelope(
        items: <Map<String, dynamic>>[],
        hasMore: false,
      );
      return ResponseBody.fromString(jsonEncode(body), 200, headers: {
        'content-type': ['application/json'],
      });
    }

    final canned = _responses[_callCount];
    _callCount++;

    if (canned.body == null) {
      throw DioException(
        requestOptions: options,
        type: DioExceptionType.connectionError,
        message: 'Simulated network error',
      );
    }

    return ResponseBody.fromString(
      jsonEncode(canned.body),
      canned.statusCode,
      headers: {
        'content-type': ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}

  int get requestCount => _callCount;
}

ApiClient _fakeApiClient(_FakeFeedHttpAdapter adapter) {
  final client = ApiClient(logger: null);
  client.dio.httpClientAdapter = adapter;
  return client;
}

class _FakeAuthenticatedAuthController extends AuthController {
  @override
  AuthState build() => AuthState.authenticated(
    AuthUser(
      id: 'user-1',
      email: 'test@example.com',
      username: 'testuser',
      isEmailVerified: true,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      roles: const [UserRole.user],
      provider: ShonaAuthProvider.email,
    ),
    emailVerified: true,
  );
}

class _FakeAuctionRepository implements AuctionRepository {
  @override
  Stream<List<Auction>> watchActiveAuctions({int limit = 50}) {
    return Stream.value(const <Auction>[]);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeLikeRepository implements LikeRepository {
  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    return Result.success(false);
  }

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    return Result.success(
      LikeStats(
        targetId: targetId,
        targetType: targetType,
        totalLikes: 0,
        isLikedByCurrentUser: false,
      ),
    );
  }

  @override
  Future<Result<bool>> hasUserLiked({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    return Result.success(false);
  }

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    return Stream.value(
      LikeStats(
        targetId: targetId,
        targetType: targetType,
        totalLikes: 0,
        isLikedByCurrentUser: false,
      ),
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void _setLargeViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 2400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

void _setOverflowViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 800);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

Widget _buildHarness(
  _FakeFeedHttpAdapter adapter, {
  required GoRouter router,
}) {
  return ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),
      authControllerProvider.overrideWith(
        _FakeAuthenticatedAuthController.new,
      ),
      auctionRepositoryProvider.overrideWithValue(_FakeAuctionRepository()),
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository()),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

GoRouter _homeRouter() {
  return GoRouter(
    initialLocation: '/home',
    routes: [
      GoRoute(
        path: '/home',
        builder: (context, state) => const Scaffold(body: HomeScreen()),
      ),
      GoRoute(
        path: '/other',
        builder: (context, state) => const Scaffold(body: SizedBox.shrink()),
      ),
    ],
  );
}

ProviderContainer _container(WidgetTester tester) {
  return ProviderScope.containerOf(tester.element(find.byType(HomeScreen)));
}

Future<void> _pump(WidgetTester tester) async {
  for (int i = 0; i < 12; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

List<Map<String, dynamic>> _manyContentItems() => List.generate(
  20,
  (i) => _feedContentItem(id: 'feed-$i', body: 'Content $i'),
);

void main() {
  setUp(() {
    VisibilityDetectorController.instance.updateInterval = Duration.zero;
  });

  // ==========================================================================
  // DATA: items tersedia
  // ==========================================================================
  group('DATA: items tersedia', () {
    testWidgets('cards/list tampil saat items ada', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _feedContentItem(id: 'feed-1', body: 'hello'),
              _feedContentItem(id: 'feed-2', body: 'world'),
            ],
            hasMore: true,
            nextCursor: 'cursor-1',
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      expect(find.text('hello'), findsOneWidget);
      expect(find.text('world'), findsOneWidget);
      expect(find.byType(FeedCard), findsNWidgets(2));
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);
      expect(find.text('Feed belum bisa dimuat'), findsNothing);
    });

    testWidgets('empty state tidak tampil saat items ada', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'hello')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      expect(find.text('hello'), findsOneWidget);
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);
    });

    testWidgets('full error tidak tampil saat items ada', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'hello')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      expect(find.text('hello'), findsOneWidget);
      expect(find.text('Feed belum bisa dimuat'), findsNothing);
    });
  });

  // ==========================================================================
  // GENUINE EMPTY
  // ==========================================================================
  group('GENUINE EMPTY', () {
    testWidgets(
      'items=[] loading=false errorMessage=null → empty state tampil',
      (tester) async {
        _setLargeViewport(tester);
        final adapter = _FakeFeedHttpAdapter([
          _CannedResponse(
            statusCode: 200,
            body: _feedEnvelope(items: <Map<String, dynamic>>[], hasMore: false),
          ),
        ]);

        await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
        await _pump(tester);

        expect(find.text('🎯 Kamu ingin apa hari ini?'), findsOneWidget);
        expect(find.byType(CircularProgressIndicator), findsNothing);
        expect(find.text('Feed belum bisa dimuat'), findsNothing);
      },
    );
  });

  // ==========================================================================
  // INITIAL LOADING
  // ==========================================================================
  group('INITIAL LOADING', () {
    testWidgets('items=[] loading=true → loading tampil', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter(const []);
      adapter.blockUntil = Completer<void>();

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await tester.pump();

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(find.text('Feed belum bisa dimuat'), findsNothing);
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);

      adapter.blockUntil!.complete();
      await _pump(tester);
    });

    testWidgets('loading tidak tampil jika items sudah ada', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'existing')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      expect(find.text('existing'), findsOneWidget);
    });
  });

  // ==========================================================================
  // INITIAL ERROR
  // ==========================================================================
  group('INITIAL ERROR', () {
    testWidgets(
      'items=[] errorMessage set → full error tampil + retry',
      (tester) async {
        _setLargeViewport(tester);
        final adapter = _FakeFeedHttpAdapter([
          const _CannedResponse(statusCode: 200, body: null),
        ]);

        await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
        await _pump(tester);

        expect(find.text('Feed belum bisa dimuat'), findsOneWidget);
        expect(find.text('Coba Lagi'), findsOneWidget);
        expect(find.text('Coba lagi beberapa saat.'), findsOneWidget);
        expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);
      },
    );

    testWidgets('empty tidak tampil saat initial error', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        const _CannedResponse(statusCode: 200, body: null),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);
      expect(find.text('Feed belum bisa dimuat'), findsOneWidget);
    });

    testWidgets('initial error retry → loadFeed again via feedProvider refresh',
        (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        const _CannedResponse(statusCode: 200, body: null),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'hello')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      expect(adapter.requestCount, 1);
      expect(find.text('Feed belum bisa dimuat'), findsOneWidget);

      await tester.tap(find.text('Coba Lagi'));
      await _pump(tester);

      expect(adapter.requestCount, 2);
      expect(find.text('Feed belum bisa dimuat'), findsNothing);
      expect(find.text('hello'), findsOneWidget);
    });
  });

  // ==========================================================================
  // REFRESH FAILURE
  // HomeScreen shows the full error whenever FeedState.errorMessage is set.
  // FeedNotifier.refresh() clears items then loadFeed(); a failed refresh
  // therefore surfaces the same full-error UI as an initial load failure.
  // ==========================================================================
  group('REFRESH FAILURE', () {
    testWidgets('refresh gagal menampilkan full error', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'hello')],
            hasMore: true,
            nextCursor: 'cursor-1',
          ),
        ),
        const _CannedResponse(statusCode: 200, body: null),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);
      expect(find.text('hello'), findsOneWidget);

      await _container(tester).read(feedProvider.notifier).refresh();
      await _pump(tester);

      expect(find.text('Feed belum bisa dimuat'), findsOneWidget);
      expect(find.text('Coba lagi beberapa saat.'), findsOneWidget);
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);
    });

    testWidgets('refresh error retry memuat ulang feed', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'hello')],
            hasMore: true,
            nextCursor: 'cursor-1',
          ),
        ),
        const _CannedResponse(statusCode: 200, body: null),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-2', body: 'fresh')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      await _container(tester).read(feedProvider.notifier).refresh();
      await _pump(tester);
      expect(find.text('Coba lagi beberapa saat.'), findsOneWidget);

      await tester.tap(find.text('Coba Lagi'));
      await _pump(tester);

      expect(find.text('Feed belum bisa dimuat'), findsNothing);
      expect(find.text('fresh'), findsOneWidget);
    });
  });

  // ==========================================================================
  // PULL-TO-REFRESH
  // HomeScreen RefreshIndicator invalidates feedProvider (rebuild + loadFeed).
  // ==========================================================================
  group('PULL-TO-REFRESH', () {
    testWidgets('pull-to-refresh memanggil loadFeed ulang dan items tidak hilang',
        (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'hello')],
            hasMore: true,
            nextCursor: 'cursor-1',
          ),
        ),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-2', body: 'refreshed')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);
      expect(find.text('hello'), findsOneWidget);
      final requestsBefore = adapter.requestCount;

      await tester.drag(find.byType(HomeScreen), const Offset(0, 300));
      await _pump(tester);

      expect(adapter.requestCount, greaterThan(requestsBefore));
      expect(find.text('refreshed'), findsOneWidget);
    });
  });

  // ==========================================================================
  // PAGINATION FAILURE
  // FeedNotifier.loadMore() logs pagination errors and does not set
  // errorMessage, so HomeScreen keeps existing items and has no pagination
  // error row.
  // ==========================================================================
  group('PAGINATION FAILURE', () {
    testWidgets('items tetap tampil saat pagination gagal', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'hello')],
            hasMore: true,
            nextCursor: 'cursor-1',
          ),
        ),
        const _CannedResponse(statusCode: 200, body: null),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);
      expect(find.text('hello'), findsOneWidget);

      await _container(tester).read(feedProvider.notifier).loadMore();
      await _pump(tester);

      expect(find.text('hello'), findsOneWidget);
      expect(find.text('Feed belum bisa dimuat'), findsNothing);
    });

    testWidgets('pagination error tidak menampilkan full error', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'hello')],
            hasMore: true,
            nextCursor: 'cursor-1',
          ),
        ),
        const _CannedResponse(statusCode: 200, body: null),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      await _container(tester).read(feedProvider.notifier).loadMore();
      await _pump(tester);

      expect(find.text('Feed belum bisa dimuat'), findsNothing);
      expect(find.text('hello'), findsOneWidget);
    });
  });

  // ==========================================================================
  // ITEMS SURVIVE DURING STATES
  // ==========================================================================
  group('ITEMS SURVIVE', () {
    testWidgets('items tetap render saat isLoadingMore = true', (tester) async {
      _setLargeViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-1', body: 'hello')],
            hasMore: true,
            nextCursor: 'cursor-1',
          ),
        ),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'feed-2', body: 'more')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      adapter.blockUntil = Completer<void>();
      unawaited(_container(tester).read(feedProvider.notifier).loadMore());
      await tester.pump();

      expect(find.text('hello'), findsOneWidget);

      adapter.blockUntil!.complete();
      await _pump(tester);
    });
  });

  // ==========================================================================
  // AUTO-PAGINATION
  // HomeScreen._maybeLoadMore skips when errorMessage != null, isLoading,
  // isLoadingMore, or hasReachedMax. Pagination errors do not set
  // errorMessage, so they do not gate auto-pagination.
  // ==========================================================================
  group('AUTO-PAGINATION', () {
    testWidgets('scroll near bottom triggers loadMore HTTP request', (
      tester,
    ) async {
      _setOverflowViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: _manyContentItems(),
            hasMore: true,
            nextCursor: 'cursor-1',
          ),
        ),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'new', body: 'more-content')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);

      final requestsBefore = adapter.requestCount;
      final scrollableFinder = find.byType(Scrollable);
      expect(scrollableFinder, findsWidgets);
      final scrollable = tester.widget<Scrollable>(scrollableFinder.first);
      final position = scrollable.controller!.position;
      if (position.maxScrollExtent > _loadMoreThreshold) {
        scrollable.controller!.jumpTo(position.maxScrollExtent - 100);
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 50));
      }

      expect(adapter.requestCount, greaterThan(requestsBefore));
    });

    testWidgets('pagination resumes after initial-error recovery', (
      tester,
    ) async {
      _setOverflowViewport(tester);
      final adapter = _FakeFeedHttpAdapter([
        const _CannedResponse(statusCode: 200, body: null),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: _manyContentItems(),
            hasMore: true,
            nextCursor: 'cursor-2',
          ),
        ),
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [_feedContentItem(id: 'new', body: 'more-content')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(_buildHarness(adapter, router: _homeRouter()));
      await _pump(tester);
      expect(find.text('Feed belum bisa dimuat'), findsOneWidget);

      await tester.tap(find.text('Coba Lagi'));
      await _pump(tester);
      expect(find.text('Feed belum bisa dimuat'), findsNothing);

      final requestsBefore = adapter.requestCount;
      final scrollableFinder = find.byType(Scrollable);
      final scrollable = tester.widget<Scrollable>(scrollableFinder.first);
      final position = scrollable.controller!.position;
      if (position.maxScrollExtent > _loadMoreThreshold) {
        scrollable.controller!.jumpTo(position.maxScrollExtent - 100);
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 50));
        expect(adapter.requestCount, greaterThan(requestsBefore));
      }
    });
  });
}

const _loadMoreThreshold = 400.0;
