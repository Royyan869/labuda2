// ============================================================================
// FEED MOBILE CROSS-BOUNDARY PIPELINE PROOF
//
// Integration-style tests that exercise the real production pipeline:
//   FeedApiDatasource → FeedResponseDto → HomeRepositoryImpl → FeedNotifier → HomeScreen
//
// ONLY the transport layer (ApiClient/Dio HTTP adapter) is overridden.
// Datasource, repository, notifier, and HomeScreen are all the real production code.
// ============================================================================

import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/commerce/catalog/listing/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/listing/presentation/providers/listing_providers.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/shared/services/logger_service.dart';

// ============================================================================
// Canonical HTTP fixture builder
// ============================================================================

/// Builds a canonical platform-envelope response body for the Feed endpoint.
///
/// The full wire shape is:
///   { "data": { "data": [...items], "next_cursor": ..., "has_more": ... } }
///
/// FeedApiDatasource extracts `body['data']` from the Dio response, then
/// FeedResponseDto.fromJson parses `payload['data']` as the items array.
Map<String, dynamic> feedEnvelope({
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

/// A minimal organic content item for the feed wire.
Map<String, dynamic> feedContentItem({
  required String id,
  required String body,
  String authorId = 'author-1',
  String authorUsername = 'alice',
  String createdAt = '2026-08-05T10:00:00Z',
}) {
  return <String, dynamic>{
    'feed_item_kind': 'content',
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

/// A promoted listing item for the feed wire.
Map<String, dynamic> feedPromotedListingItem({
  required String instanceId,
  required String title,
  int pricePerUnit = 5000000,
  String fixedPriceSaleId = 'listing-1',
}) {
  return <String, dynamic>{
    'feed_item_kind': 'promoted_fixed_price_sale',
    'promotion_instance_id': instanceId,
    'target_type': 'listing',
    'title': title,
    'image_url': 'https://example.com/koi.jpg',
    'seller_username': 'seller1',
    'seller_farm_name': 'Farm One',
    'fixed_price_sale_id': fixedPriceSaleId,
    'price_per_unit': pricePerUnit,
  };
}

/// A promoted auction item for the feed wire.
Map<String, dynamic> feedPromotedAuctionItem({
  required String instanceId,
  required String title,
  int startPrice = 1000000,
  int? currentBid,
  String auctionId = 'auction-1',
}) {
  return <String, dynamic>{
    'feed_item_kind': 'promoted_auction',
    'promotion_instance_id': instanceId,
    'target_type': 'auction',
    'title': title,
    'image_url': 'https://example.com/auction.jpg',
    'seller_username': 'seller2',
    'start_price': startPrice,
    'current_bid': currentBid,
    'auction_id': auctionId,
    'end_at': '2026-08-10T10:00:00Z',
    'bid_count': 3,
  };
}

/// A promoted external item for the feed wire.
Map<String, dynamic> feedPromotedExternalItem({
  required String instanceId,
  required String title,
  String externalUrl = 'https://example.com/product',
}) {
  return <String, dynamic>{
    'feed_item_kind': 'promoted_external',
    'promotion_instance_id': instanceId,
    'target_type': 'external_product',
    'title': title,
    'external_url': externalUrl,
    'external_media_url': 'https://example.com/external.jpg',
  };
}

// ============================================================================
// Fake HTTP transport
// ============================================================================

/// A canned response for the fake HTTP adapter.
class _CannedResponse {
  final int statusCode;
  final Map<String, dynamic>? body;
  const _CannedResponse({required this.statusCode, this.body});
}

/// Dio HttpClientAdapter that returns canned responses from a queue.
///
/// Each call to [fetch] for /feed requests consumes the next [_CannedResponse].
/// All other requests get a generic success envelope so unrelated API calls
/// (like, presence, etc.) don't crash their parsers.
class FakeFeedHttpAdapter implements HttpClientAdapter {
  final List<_CannedResponse> _responses;
  int _callCount = 0;

  /// Captured query parameters for /feed requests, in call order.
  final List<Map<String, dynamic>> capturedQueryParams = [];

  FakeFeedHttpAdapter(List<_CannedResponse> responses)
    : _responses = responses;

  ResponseBody _genericSuccess() {
    return ResponseBody.fromString(
      jsonEncode(<String, dynamic>{
        'success': true,
        'data': <String, dynamic>{},
        'timestamp': '2026-08-05T00:00:00Z',
      }),
      200,
      headers: {'content-type': ['application/json']},
    );
  }

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    // Only /feed requests consume from the canned response queue.
    if (!options.path.contains('feed') && !options.path.contains('/feed')) {
      return _genericSuccess();
    }

    capturedQueryParams.add(Map<String, dynamic>.from(
      options.queryParameters,
    ));

    if (_callCount >= _responses.length) {
      final body = feedEnvelope(items: <Map<String, dynamic>>[], hasMore: false);
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
      headers: {'content-type': ['application/json']},
    );
  }

  @override
  void close({bool force = false}) {}

  int get requestCount => _callCount;
}

/// Create an ApiClient that routes all requests through [adapter].
ApiClient _fakeApiClient(FakeFeedHttpAdapter adapter) {
  final client = ApiClient.testing();
  client.dio.httpClientAdapter = adapter;
  return client;
}

// ============================================================================
// Fake providers for unrelated dependencies
// ============================================================================

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

class _FakeUnauthenticatedAuthController extends AuthController {
  @override
  AuthState build() => const AuthStateUnauthenticated();
}

class _FakeListingRepository implements ListingRepository {
  @override
  Future<Result<List<Listing>>> getListings(GetListingsParams params) async {
    return Result.success(const <Listing>[]);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
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

// ============================================================================
// Test harness
// ============================================================================

/// Set a large viewport so multiple feed cards can render without scrolling.
void _setLargeViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 2400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

/// Build the full ProviderScope harness with real Feed pipeline.
Widget _buildHarness(
  FakeFeedHttpAdapter adapter, {
  required bool authenticated,
  required GoRouter router,
}) {
  return ProviderScope(
    overrides: [
      // THE ONLY TRANSPORT OVERRIDE — everything downstream is real.
      apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),
      // Auth state override.
      authControllerProvider.overrideWith(
        authenticated
            ? _FakeAuthenticatedAuthController.new
            : _FakeUnauthenticatedAuthController.new,
      ),
      // Unrelated commerce providers needed by CommercePreviewSection.
      listingRepositoryProvider.overrideWithValue(_FakeListingRepository()),
      auctionRepositoryProvider.overrideWithValue(_FakeAuctionRepository()),
      // Like repository — prevents ContentLikeAction from making real API calls.
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository()),
      // Logger service override.
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

/// Simple GoRouter wiring HomeScreen at /home.
GoRouter _homeRouter() {
  return GoRouter(
    initialLocation: '/home',
    routes: [
      GoRoute(
        path: '/home',
        builder: (context, state) => const Scaffold(body: HomeScreen()),
      ),
    ],
  );
}

/// Helper to access the nearest ProviderContainer from the widget tree.
ProviderContainer _container(WidgetTester tester) {
  return ProviderScope.containerOf(tester.element(find.byType(HomeScreen)));
}

/// Pump the widget through enough microtask/frame cycles for the Dio+async
/// chain (FeedNotifier.build → loadFeed → HomeRepositoryImpl.getFeedPage →
/// FeedApiDatasource.getFeed → ApiClient.get → fake adapter) to resolve.
///
/// Uses small incremental pumps to avoid FakeAsync timer cascades from
/// VisibilityDetector/UserAvatar.
Future<void> _pump(WidgetTester tester) async {
  for (int i = 0; i < 12; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

// ============================================================================
// Tests
// ============================================================================

void main() {
  // ==========================================================================
  // SCENARIO 1: Canonical first page with organic Content
  // ==========================================================================
  group('SCENARIO 1: canonical first page with organic Content', () {
    testWidgets('real pipeline renders Content card from wire response', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: feedEnvelope(
            items: [
              feedContentItem(id: 'feed-1', body: 'Hello from the feed!'),
              feedContentItem(id: 'feed-2', body: 'Second post here'),
            ],
            nextCursor: 'cursor-page-1',
            hasMore: true,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, authenticated: true, router: _homeRouter()),
      );
      await _pump(tester);

      // Proof: HomeScreen renders Content cards from the real pipeline.
      expect(find.text('Hello from the feed!'), findsOneWidget);
      expect(find.text('Second post here'), findsOneWidget);

      // Proof: FeedState reflects the parsed and mapped items.
      final state = _container(tester).read(feedProvider);
      expect(state.items, hasLength(2));
      expect(state.items[0].type, FeedItemType.content);
      expect(state.items[1].type, FeedItemType.content);
      expect(state.isLoading, isFalse);
      expect(state.errorKind, isNull);
      expect(state.hasReachedMax, isFalse);

      // Empty/error states absent.
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);
      expect(find.text('Feed belum bisa dimuat'), findsNothing);

      // Verify request went to correct endpoint with correct params.
      expect(adapter.capturedQueryParams, isNotEmpty);
      expect(adapter.capturedQueryParams.first['limit'], 20);
      expect(adapter.capturedQueryParams.first['cursor'], isNull);
    });
  });

  // ==========================================================================
  // SCENARIO 2: Mixed organic and promoted response
  // ==========================================================================
  group('SCENARIO 2: mixed organic and promoted response', () {
    testWidgets(
      'all four kinds survive the real pipeline with correct types',
      (tester) async {
        // Use default viewport (not large) to minimize VisibilityDetector
        // surface area. The pipeline proof is via FeedState, not widget
        // rendering for every item.

        final adapter = FakeFeedHttpAdapter([
          _CannedResponse(
            statusCode: 200,
            body: feedEnvelope(
              items: [
                feedContentItem(id: 'organic-1', body: 'An organic post'),
                feedPromotedListingItem(
                  instanceId: 'pi-listing',
                  title: 'Promoted Listing Koi',
                ),
                feedPromotedAuctionItem(
                  instanceId: 'pi-auction',
                  title: 'Promoted Auction Koi',
                ),
                feedContentItem(id: 'organic-2', body: 'Another post'),
                feedPromotedExternalItem(
                  instanceId: 'pi-external',
                  title: 'External Shop Link',
                ),
              ],
              hasMore: false,
            ),
          ),
        ]);

        await tester.pumpWidget(
          _buildHarness(adapter, authenticated: true, router: _homeRouter()),
        );
        // Use _pump to process the async chain.
        for (int i = 0; i < 12; i++) {
          await tester.pump(const Duration(milliseconds: 50));
        }

        // Proof: FeedState has all 5 items with correct types.
        // This proves the ENTIRE pipeline: HTTP response → DTO parse →
        // FeedResponseDto → HomeRepositoryImpl → mergeFeedItems → FeedNotifier.
        final state = _container(tester).read(feedProvider);
        expect(state.items, hasLength(5));
        expect(state.items[0].type, FeedItemType.content);
        expect(state.items[1].type, FeedItemType.promotedListing);
        expect(state.items[2].type, FeedItemType.promotedAuction);
        expect(state.items[3].type, FeedItemType.content);
        expect(state.items[4].type, FeedItemType.promotedExternal);

        // Proof: promoted items carry correct additionalData from DTO mapping.
        expect(state.items[1].additionalData['isPromoted'], true);
        expect(state.items[1].additionalData['title'], 'Promoted Listing Koi');
        expect(state.items[2].additionalData['auctionId'], 'auction-1');
        expect(state.items[2].additionalData['bidCount'], 3);
        expect(state.items[4].additionalData['externalUrl'],
            'https://example.com/product');

        // Proof: no item fell back to another kind.
        // All items are present with their canonical FeedItemType.
        expect(state.isLoading, isFalse);
        expect(state.errorKind, isNull);

        // Proof: at least the organic items render on screen.
        expect(find.text('An organic post'), findsOneWidget);

        // Verify endpoint hit exactly once.
        expect(adapter.requestCount, 1);

        // Flush VisibilityDetector timers so the framework invariant passes.
        // The double-shrink pattern breaks the cascade: each pumpWidget
        // disposes old render objects, then pump consumes leftover timers.
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pump(const Duration(seconds: 3));
        await tester.pumpWidget(const SizedBox.shrink());
        await tester.pump(const Duration(milliseconds: 600));
      },
    );
  });

  // ==========================================================================
  // SCENARIO 3: Genuine empty
  // ==========================================================================
  group('SCENARIO 3: genuine empty', () {
    testWidgets('success with empty data[] renders genuine empty state', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: feedEnvelope(items: <Map<String, dynamic>>[], hasMore: false),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, authenticated: true, router: _homeRouter()),
      );
      await _pump(tester);

      // Proof: genuine empty state displayed.
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsOneWidget);

      // Proof: FeedState reflects genuine empty (not error).
      final state = _container(tester).read(feedProvider);
      expect(state.items, isEmpty);
      expect(state.isLoading, isFalse);
      expect(state.errorKind, isNull);

      // Proof: no error, no loading.
      expect(find.text('Feed belum bisa dimuat'), findsNothing);
      expect(find.byType(CircularProgressIndicator), findsNothing);
      expect(adapter.requestCount, 1);
    });
  });

  // ==========================================================================
  // SCENARIO 4: Initial server failure then retry recovery
  // ==========================================================================
  group('SCENARIO 4: initial failure then retry recovery', () {
    testWidgets('500 → retry → 200 with Content', (tester) async {
      _setLargeViewport(tester);

      final adapter = FakeFeedHttpAdapter([
        _CannedResponse(statusCode: 500, body: <String, dynamic>{}),
        _CannedResponse(
          statusCode: 200,
          body: feedEnvelope(
            items: [feedContentItem(id: 'recovered-1', body: 'Recovered!')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, authenticated: true, router: _homeRouter()),
      );
      await _pump(tester);

      // After first 500: initial full error appears.
      expect(find.text('Feed belum bisa dimuat'), findsOneWidget);
      // FeedState: error, no items.
      final errorState = _container(tester).read(feedProvider);
      expect(errorState.items, isEmpty);
      expect(errorState.errorKind, FeedErrorKind.initial);

      // Empty state absent.
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);

      // Tap "Coba Lagi" to retry.
      await tester.tap(find.text('Coba Lagi'));
      await _pump(tester);

      // Second request occurred and content renders.
      expect(find.text('Recovered!'), findsOneWidget);
      expect(find.text('Feed belum bisa dimuat'), findsNothing);

      // FeedState: success, error cleared.
      final successState = _container(tester).read(feedProvider);
      expect(successState.items, hasLength(1));
      expect(successState.errorKind, isNull);

      expect(adapter.requestCount, 2);
    });
  });

  // ==========================================================================
  // SCENARIO 5: Malformed payload is not empty
  // ==========================================================================
  group('SCENARIO 5: malformed payload', () {
    testWidgets('data:null produces initial error, not genuine empty', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: <String, dynamic>{
            'data': <String, dynamic>{
              'data': null,
              'next_cursor': null,
              'has_more': false,
            },
          },
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, authenticated: true, router: _homeRouter()),
      );
      await _pump(tester);

      // Proof: initial error appears, not empty.
      expect(find.text('Feed belum bisa dimuat'), findsOneWidget);
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);

      // Proof: FeedState is error, not success with 0 items.
      final state = _container(tester).read(feedProvider);
      expect(state.errorKind, FeedErrorKind.initial);
      expect(state.items, isEmpty);
      expect(adapter.requestCount, 1);
    });

    testWidgets('missing feed_item_kind produces initial error', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: feedEnvelope(
            items: [
              <String, dynamic>{
                'id': 'bad-item',
                'body': 'no kind field',
                'status': 'active',
                'created_at': '2026-08-05T10:00:00Z',
                'updated_at': '2026-08-05T10:00:00Z',
                'media': <Map<String, dynamic>>[],
              },
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, authenticated: true, router: _homeRouter()),
      );
      await _pump(tester);

      expect(find.text('Feed belum bisa dimuat'), findsOneWidget);
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);
      expect(adapter.requestCount, 1);
    });
  });

  // ==========================================================================
  // SCENARIO 6: Refresh failure preserves rendered items
  // ==========================================================================
  group('SCENARIO 6: refresh failure preserves items', () {
    testWidgets(
      'Content A visible after refresh 500, retry shows Content B',
      (tester) async {
        _setLargeViewport(tester);

        final adapter = FakeFeedHttpAdapter([
          // Initial load: Content A.
          _CannedResponse(
            statusCode: 200,
            body: feedEnvelope(
              items: [feedContentItem(id: 'a-1', body: 'Content A')],
              nextCursor: 'cursor-a',
              hasMore: true,
            ),
          ),
          // Refresh: HTTP 500.
          _CannedResponse(statusCode: 500, body: <String, dynamic>{}),
          // Refresh retry: Content B.
          _CannedResponse(
            statusCode: 200,
            body: feedEnvelope(
              items: [feedContentItem(id: 'b-1', body: 'Content B')],
              hasMore: false,
            ),
          ),
        ]);

        await tester.pumpWidget(
          _buildHarness(adapter, authenticated: true, router: _homeRouter()),
        );
        await _pump(tester);

        // Initial load succeeded.
        expect(find.text('Content A'), findsOneWidget);

        // Trigger refresh programmatically.
        final container = _container(tester);
        unawaited(container.read(feedProvider.notifier).refresh());
        await _pump(tester);

        // Proof: Content A remains visible after refresh failure.
        expect(find.text('Content A'), findsOneWidget);

        // Proof: refresh error banner appears.
        expect(find.text('Coba lagi beberapa saat.'), findsOneWidget);

        // FeedState: items preserved, errorKind = refresh.
        final refreshErrorState = container.read(feedProvider);
        expect(refreshErrorState.items, hasLength(1));
        expect(refreshErrorState.errorKind, FeedErrorKind.refresh);

        // Full error and empty are absent.
        expect(find.text('Feed belum bisa dimuat'), findsNothing);
        expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);

        // Tap retry in the refresh banner.
        final retryButtons = find.text('Coba Lagi');
        await tester.tap(retryButtons.last);
        await _pump(tester);

        // Proof: Content B replaces Content A (refresh contract).
        expect(find.text('Content B'), findsOneWidget);
        expect(find.text('Coba lagi beberapa saat.'), findsNothing);

        // FeedState: new items, error cleared.
        final recoveredState = container.read(feedProvider);
        expect(recoveredState.items[0].content, 'Content B');
        expect(recoveredState.errorKind, isNull);

        expect(adapter.requestCount, 3);
      },
    );
  });

  // ==========================================================================
  // SCENARIO 7: Pagination failure and explicit retry
  // ==========================================================================
  group('SCENARIO 7: pagination failure and explicit retry', () {
    testWidgets('loadMore 500 preserves page, retry appends Content B', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = FakeFeedHttpAdapter([
        // Page 1: Content A, has_more=true, next_cursor=X.
        _CannedResponse(
          statusCode: 200,
          body: feedEnvelope(
            items: [feedContentItem(id: 'a-1', body: 'Content A')],
            nextCursor: 'cursor-X',
            hasMore: true,
          ),
        ),
        // loadMore: HTTP 500.
        _CannedResponse(statusCode: 500, body: <String, dynamic>{}),
        // Explicit pagination retry: page 2 with Content B.
        _CannedResponse(
          statusCode: 200,
          body: feedEnvelope(
            items: [feedContentItem(id: 'b-1', body: 'Content B')],
            nextCursor: 'cursor-Y',
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, authenticated: true, router: _homeRouter()),
      );

      final container = _container(tester);

      await _pump(tester);

      // Page 1: Content A visible.
      expect(find.text('Content A'), findsOneWidget);

      // Trigger loadMore.
      unawaited(container.read(feedProvider.notifier).loadMore());
      await _pump(tester);

      // Proof: Content A remains visible after pagination failure.
      expect(find.text('Content A'), findsOneWidget);
      // Proof: no duplicate A.
      expect(find.text('Content A'), findsOneWidget);

      // Proof: pagination retry row appears.
      expect(find.text('Gagal memuat halaman berikutnya'), findsOneWidget);

      // FeedState: items preserved, errorKind = pagination.
      final errorState = container.read(feedProvider);
      expect(errorState.items, hasLength(1));
      expect(errorState.errorKind, FeedErrorKind.pagination);

      // Record cursor: page 1 (no cursor), loadMore sent cursor-X.
      expect(adapter.capturedQueryParams.length, 2);
      expect(adapter.capturedQueryParams[0]['cursor'], isNull);
      // The loadMore request sends the cursor from _nextCursor.
      expect(adapter.capturedQueryParams[1]['cursor'], isNotNull);
      expect(adapter.capturedQueryParams[1]['cursor'], 'cursor-X');

      // Tap pagination retry.
      await tester.tap(find.text('Coba Lagi'));
      await _pump(tester);

      // Proof: Content B is appended.
      expect(find.text('Content B'), findsOneWidget);
      expect(find.text('Content A'), findsOneWidget);
      // Pagination error clears.
      expect(find.text('Gagal memuat halaman berikutnya'), findsNothing);

      // FeedState: 2 items, error cleared.
      final successState = container.read(feedProvider);
      expect(successState.items, hasLength(2));
      expect(successState.errorKind, isNull);

      expect(adapter.requestCount, 3);
    });

    testWidgets('pagination error blocks repeat auto-requests', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: feedEnvelope(
            items: [feedContentItem(id: 'a-1', body: 'Content A')],
            nextCursor: 'cursor-X',
            hasMore: true,
          ),
        ),
        _CannedResponse(statusCode: 500, body: <String, dynamic>{}),
        // Third response: should NOT be consumed while error is active.
        _CannedResponse(
          statusCode: 200,
          body: feedEnvelope(
            items: [feedContentItem(id: 'c-1', body: 'SHOULD NOT APPEAR')],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, authenticated: true, router: _homeRouter()),
      );

      await _pump(tester);

      // Trigger loadMore failure.
      final container3 = _container(tester);
      unawaited(container3.read(feedProvider.notifier).loadMore());
      await _pump(tester);

      // Pagination error active.
      expect(find.text('Gagal memuat halaman berikutnya'), findsOneWidget);

      // Only 2 requests consumed: initial + failed loadMore.
      // The third response should NOT be consumed (no auto-retry while error).
      expect(adapter.requestCount, 2);
    });
  });

  // ==========================================================================
  // CROSS-BOUNDARY PIPELINE PROOF
  //
  // NOTE: These tests use a simplified GoRouter that embeds HomeScreen directly
  // in a Scaffold. This is a valid cross-boundary pipeline test (it exercises the
  // real FeedApiDatasource → HomeRepositoryImpl → FeedNotifier → HomeScreen chain)
  // but it does NOT test production root wiring (MainScreen → IndexedStack).
  // For actual production root-wiring proof, see:
  //   test/features/home/presentation/root_wiring/feed_root_wiring_test.dart
  // ==========================================================================
  group('CROSS-BOUNDARY PIPELINE PROOF', () {
    testWidgets(
      'actual HomeScreen with real providers reached via GoRouter /home',
      (tester) async {
        _setLargeViewport(tester);

        final adapter = FakeFeedHttpAdapter([
          _CannedResponse(
            statusCode: 200,
            body: feedEnvelope(
              items: [
                feedContentItem(id: 'root-1', body: 'Root-wired feed content'),
              ],
              hasMore: false,
            ),
          ),
        ]);

        await tester.pumpWidget(
          _buildHarness(adapter, authenticated: true, router: _homeRouter()),
        );
        await _pump(tester);

        // Proof: actual route reaches HomeScreen.
        expect(find.byType(HomeScreen), findsOneWidget);

        // Proof: feedProvider initialized and loaded real pipeline.
        final container = _container(tester);
        final state = container.read(feedProvider);
        expect(state.items, hasLength(1));
        expect(state.items[0].content, 'Root-wired feed content');
        expect(state.items[0].type, FeedItemType.content);

        // Proof: real datasource/repository/notifier pipeline used.
        expect(state.isLoading, isFalse);
        expect(state.errorKind, isNull);
        expect(state.errorMessage, isNull);

        // Content renders on screen.
        expect(find.text('Root-wired feed content'), findsOneWidget);
      },
    );

    testWidgets('FeedNotifier reads authenticated user ID from Auth provider', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: feedEnvelope(
            items: [
              feedContentItem(id: 'auth-1', body: 'Authenticated content'),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, authenticated: true, router: _homeRouter()),
      );
      await _pump(tester);

      final container = _container(tester);
      final authState = container.read(authControllerProvider);
      // Must be authenticated state — FeedNotifier._getCurrentUserId() reads it.
      expect(authState, isA<AuthStateAuthenticated>());
      final user = (authState as AuthStateAuthenticated).user;
      expect(user.id, 'user-1');

      // Feed pipeline works with authenticated user.
      final state = container.read(feedProvider);
      expect(state.items, hasLength(1));
      expect(state.errorKind, isNull);
    });
  });
}
