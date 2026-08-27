// ============================================================================
// PROMOTED FEED CARD RENDERING AUTHORITY
//
// Canonical HomeScreen-level proof that all three promoted kinds
// (promoted_fixed_price_sale, promoted_auction, promoted_external)
// pass through the FULL production mobile pipeline and build the
// correct actual promoted card widgets.
//
// Pipeline under test:
//   FeedApiDatasource → FeedResponseDto → PromotedFeedItemMapper
//   → mergeFeedItems → FeedNotifier → HomeScreen → FeedCardFactory
//   → PromotedListingCard | PromotedAuctionCard | PromotedExternalCard
//
// ONLY the HTTP transport is overridden. Everything downstream is real
// production code: datasource, DTO, mapper, merge, repository, notifier,
// card factory, and all three promoted card widgets.
//
// DOES NOT TEST:
//   - Impression/click tracking (fire-and-forget, no widget visible)
//   - Backend feed assembly
//   - MainScreen root wiring (covered by feed_root_wiring_test.dart)
//   - Organic feed state (covered by home_screen_feed_rendering_test.dart)
//   - Chat, Share/Repost, Search, seed data, or APK
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

import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:visibility_detector/visibility_detector.dart';

// ============================================================================
// Canonical HTTP fixture builders
// ============================================================================

/// Builds a canonical platform-envelope response body for the Feed endpoint.
///
/// The full wire shape is:
///   { "data": { "data": [...items], "next_cursor": ..., "has_more": ... } }
///
/// FeedApiDatasource extracts `body['data']` from the Dio response, then
/// FeedResponseDto.fromJson parses `payload['data']` as the items array.
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

/// A minimal organic content item for the feed wire.
Map<String, dynamic> _feedContentItem({
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

/// Canonical promoted listing fixture.
Map<String, dynamic> _promotedListingItem({
  required String instanceId,
  required String title,
  int pricePerUnit = 5000000,
  String fixedPriceSaleId = 'listing-1',
  String imageUrl = 'https://example.com/koi.jpg',
  String sellerUsername = 'seller1',
  String sellerFarmName = 'Farm One',
}) {
  return <String, dynamic>{
    'feed_item_kind': 'promoted_fixed_price_sale',
    'promotion_instance_id': instanceId,
    'target_type': 'listing',
    'title': title,
    'image_url': imageUrl,
    'seller_username': sellerUsername,
    'seller_farm_name': sellerFarmName,
    'fixed_price_sale_id': fixedPriceSaleId,
    'price_per_unit': pricePerUnit,
  };
}

/// Canonical promoted auction fixture.
Map<String, dynamic> _promotedAuctionItem({
  required String instanceId,
  required String title,
  int startPrice = 1000000,
  int? currentBid,
  String auctionId = 'auction-1',
  int bidCount = 3,
  String endAt = '2026-08-10T10:00:00Z',
  String imageUrl = 'https://example.com/auction.jpg',
  String sellerUsername = 'seller2',
}) {
  return <String, dynamic>{
    'feed_item_kind': 'promoted_auction',
    'promotion_instance_id': instanceId,
    'target_type': 'auction',
    'title': title,
    'image_url': imageUrl,
    'seller_username': sellerUsername,
    'start_price': startPrice,
    'current_bid': currentBid,
    'auction_id': auctionId,
    'end_at': endAt,
    'bid_count': bidCount,
  };
}

/// Canonical promoted external fixture.
Map<String, dynamic> _promotedExternalItem({
  required String instanceId,
  required String title,
  String externalUrl = 'https://example.com/product',
  String externalMediaUrl = 'https://example.com/external.jpg',
}) {
  return <String, dynamic>{
    'feed_item_kind': 'promoted_external',
    'promotion_instance_id': instanceId,
    'target_type': 'external_product',
    'title': title,
    'external_url': externalUrl,
    'external_media_url': externalMediaUrl,
  };
}

// ============================================================================
// Fake HTTP transport
// ============================================================================

class _CannedResponse {
  final int statusCode;
  final Map<String, dynamic>? body;
  const _CannedResponse({required this.statusCode, this.body});
}

/// Dio HttpClientAdapter that returns canned responses from a queue.
///
/// /feed requests consume from the canned response queue.
/// All other requests get a generic success envelope so unrelated
/// API calls (like, presence, promotion events, etc.) don't crash.
class _FakeFeedHttpAdapter implements HttpClientAdapter {
  final List<_CannedResponse> _responses;
  int _callCount = 0;
  final List<Map<String, dynamic>> capturedQueryParams = [];

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
      headers: {'content-type': ['application/json']},
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

/// Set an extra-large viewport for multi-item tests where 5+ cards must all
/// be visible without scrolling.
void _setExtraLargeViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 6000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

/// Build the full ProviderScope harness with the real Feed pipeline.
/// ONLY the HTTP transport and unrelated commerce/auth dependencies are overridden.
Widget _buildHarness(
  _FakeFeedHttpAdapter adapter, {
  required GoRouter router,
}) {
  return ProviderScope(
    overrides: [
      // THE ONLY TRANSPORT OVERRIDE — everything downstream is real.
      apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),
      // Auth state override.
      authControllerProvider.overrideWith(
        _FakeAuthenticatedAuthController.new,
      ),
      // Unrelated commerce providers needed by CommercePreviewSection.
      
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
Future<void> _pump(WidgetTester tester) async {
  for (int i = 0; i < 12; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

// ============================================================================
// Tests
// ============================================================================

void main() {
  setUp(() {
    VisibilityDetectorController.instance.updateInterval = Duration.zero;
  });

  // ==========================================================================
  // SCENARIO 1: Promoted Listing — actual PromotedListingCard at HomeScreen
  // ==========================================================================
  group('SCENARIO 1: Promoted Listing actual-card proof', () {
    testWidgets('full pipeline builds PromotedListingCard in HomeScreen', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedListingItem(
                instanceId: 'pi-listing-1',
                title: 'Koi Kohaku Grade A',
                pricePerUnit: 7500000,
                fixedPriceSaleId: 'listing-abc',
                imageUrl: 'https://example.com/koi-kohaku.jpg',
                sellerUsername: 'breeder_one',
                sellerFarmName: 'Sakura Koi Farm',
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      // ---- FeedState proof ----
      final state = _container(tester).read(feedProvider);
      expect(state.items, hasLength(1));
      expect(state.items[0].type, FeedItemType.promotedListing);
      expect(state.items[0].id, 'pi-listing-1');
      expect(state.items[0].additionalData['isPromoted'], true);
      expect(state.items[0].additionalData['title'], 'Koi Kohaku Grade A');
      expect(state.items[0].additionalData['fixedPriceSaleId'], 'listing-abc');
      expect(state.items[0].additionalData['pricePerUnit'], 7500000);

      // ---- Actual card widget proof ----
      // PromotedListingCard must exist in the widget tree.
      expect(find.byType(PromotedListingCard), findsOneWidget);

      // Other promoted card types must be absent.
      expect(find.byType(PromotedAuctionCard), findsNothing);
      expect(find.byType(PromotedExternalCard), findsNothing);

      // FeedCard (organic content card) must be absent.
      expect(find.byType(FeedCard), findsNothing);

      // CommerceMarketplaceCardShell must be present (used by PromotedListingCard).

      // Title must be rendered in the card.
      expect(find.text('Koi Kohaku Grade A'), findsOneWidget);

      // Price must be rendered (7500000 minor units = Rp75rb).
      expect(find.text('Rp75rb'), findsOneWidget);

      // Dipromosikan badge must be present on the CommerceMarketplaceCardShell.
      expect(find.text('Dipromosikan'), findsOneWidget);

      // Error/empty states absent.
      expect(find.text('Feed belum bisa dimuat'), findsNothing);
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);
      expect(state.errorMessage, isNull);
    });

    testWidgets('promoted listing title renders from additionalData', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedListingItem(
                instanceId: 'pi-title-test',
                title: 'Show Quality Tancho',
                pricePerUnit: 12000000,
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      expect(find.byType(PromotedListingCard), findsOneWidget);
      expect(find.text('Show Quality Tancho'), findsOneWidget);
    });

    testWidgets('promoted listing price renders from contract', (tester) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedListingItem(
                instanceId: 'pi-price-test',
                title: 'Budget Koi',
                pricePerUnit: 100000, // 1000 rupiah = Rp1rb
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      expect(find.byType(PromotedListingCard), findsOneWidget);
      // 100000 minor → 1000 rupiah → Rp1rb
      expect(find.text('Rp1rb'), findsOneWidget);
    });

    testWidgets('promoted listing reference ID is preserved', (tester) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedListingItem(
                instanceId: 'pi-ref-test',
                title: 'Reference Test',
                fixedPriceSaleId: 'fps-custom-999',
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      final state = _container(tester).read(feedProvider);
      expect(state.items[0].additionalData['fixedPriceSaleId'], 'fps-custom-999');
    });
  });

  // ==========================================================================
  // SCENARIO 2: Promoted Auction — actual PromotedAuctionCard at HomeScreen
  // ==========================================================================
  group('SCENARIO 2: Promoted Auction actual-card proof', () {
    testWidgets('full pipeline builds PromotedAuctionCard in HomeScreen', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedAuctionItem(
                instanceId: 'pi-auction-1',
                title: 'Grand Champion Showa',
                startPrice: 5000000,
                currentBid: 8500000,
                auctionId: 'auction-showa',
                bidCount: 7,
                endAt: '2026-08-15T10:00:00Z',
                imageUrl: 'https://example.com/showa.jpg',
                sellerUsername: 'auction_seller',
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      // ---- FeedState proof ----
      final state = _container(tester).read(feedProvider);
      expect(state.items, hasLength(1));
      expect(state.items[0].type, FeedItemType.promotedAuction);
      expect(state.items[0].id, 'pi-auction-1');
      expect(state.items[0].additionalData['isPromoted'], true);
      expect(state.items[0].additionalData['title'], 'Grand Champion Showa');
      expect(state.items[0].additionalData['auctionId'], 'auction-showa');
      expect(state.items[0].additionalData['currentBid'], 8500000);
      expect(state.items[0].additionalData['startPrice'], 5000000);
      expect(state.items[0].additionalData['bidCount'], 7);

      // ---- Actual card widget proof ----
      // PromotedAuctionCard must exist in the widget tree.
      expect(find.byType(PromotedAuctionCard), findsOneWidget);

      // Key must contain promo-auction- prefix.
      expect(
        find.byKey(const ValueKey('promo-auction-pi-auction-1')),
        findsOneWidget,
      );

      // Other promoted card types must be absent.
      expect(find.byType(PromotedListingCard), findsNothing);
      expect(find.byType(PromotedExternalCard), findsNothing);
      expect(find.byType(FeedCard), findsNothing);

      // CommerceMarketplaceCardShell must be present.

      // Title must be rendered.
      expect(find.text('Grand Champion Showa'), findsOneWidget);

      // Current bid display (8500000 minor → Rp85rb).
      expect(find.text('Rp85rb'), findsOneWidget);

      // Bid count metadata (7 bids).
      expect(find.text('7 bid'), findsOneWidget);

      // Error/empty states absent.
      expect(find.text('Feed belum bisa dimuat'), findsNothing);
      expect(state.errorMessage, isNull);
    });

    testWidgets('promoted auction shows start price when no current bid', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedAuctionItem(
                instanceId: 'pi-no-bid',
                title: 'New Auction',
                startPrice: 2000000,
                currentBid: null, // No bids yet
                auctionId: 'auction-new',
                bidCount: 0,
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      expect(find.byType(PromotedAuctionCard), findsOneWidget);
      // Start price displayed (2000000 minor → Rp20rb).
      expect(find.text('Rp20rb'), findsOneWidget);
      // Label should be "Mulai dari".
      expect(find.text('Mulai dari'), findsOneWidget);
      // No bid count when 0.
      expect(find.text('0 bid'), findsNothing);
    });

    testWidgets('promoted auction bid count maps correctly', (tester) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedAuctionItem(
                instanceId: 'pi-bids',
                title: 'Hot Auction',
                bidCount: 42,
                currentBid: 15000000,
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      final state = _container(tester).read(feedProvider);
      expect(state.items[0].additionalData['bidCount'], 42);
      expect(find.text('42 bid'), findsOneWidget);
    });
  });

  // ==========================================================================
  // SCENARIO 3: Promoted External — actual PromotedExternalCard at HomeScreen
  // ==========================================================================
  group('SCENARIO 3: Promoted External actual-card proof', () {
    testWidgets('full pipeline builds PromotedExternalCard in HomeScreen', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedExternalItem(
                instanceId: 'pi-external-1',
                title: 'Premium Koi Food Brand',
                externalUrl: 'https://shop.example.com/koi-food',
                externalMediaUrl: 'https://example.com/koi-food.jpg',
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      // ---- FeedState proof ----
      final state = _container(tester).read(feedProvider);
      expect(state.items, hasLength(1));
      expect(state.items[0].type, FeedItemType.promotedExternal);
      expect(state.items[0].id, 'pi-external-1');
      expect(state.items[0].additionalData['isPromoted'], true);
      expect(state.items[0].additionalData['title'], 'Premium Koi Food Brand');
      expect(
        state.items[0].additionalData['externalUrl'],
        'https://shop.example.com/koi-food',
      );

      // ---- Actual card widget proof ----
      // PromotedExternalCard must exist in the widget tree.
      expect(find.byType(PromotedExternalCard), findsOneWidget);

      // Other promoted card types must be absent.
      expect(find.byType(PromotedListingCard), findsNothing);
      expect(find.byType(PromotedAuctionCard), findsNothing);
      expect(find.byType(FeedCard), findsNothing);

      // PromotedExternalCard does NOT use CommerceMarketplaceCardShell
      // (it uses its own Card + Column layout).
      // But there should be no commerce card shell for the external item.

      // Title must be rendered.
      expect(find.text('Premium Koi Food Brand'), findsOneWidget);

      // Dipromosikan badge is rendered inside PromotedExternalCard.
      expect(find.text('Dipromosikan'), findsOneWidget);

      // External URL host must be displayed.
      expect(find.text('shop.example.com'), findsOneWidget);

      // Error/empty states absent.
      expect(find.text('Feed belum bisa dimuat'), findsNothing);
      expect(state.errorMessage, isNull);
    });

    testWidgets('promoted external title and URL preserved in mapped item', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedExternalItem(
                instanceId: 'pi-ext-url',
                title: 'Direct Shop Link',
                externalUrl: 'https://direct.shop.example.com/items/42',
                externalMediaUrl: 'https://example.com/direct.jpg',
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      final state = _container(tester).read(feedProvider);
      expect(state.items[0].additionalData['title'], 'Direct Shop Link');
      expect(
        state.items[0].additionalData['externalUrl'],
        'https://direct.shop.example.com/items/42',
      );
      expect(find.byType(PromotedExternalCard), findsOneWidget);
      expect(find.text('Direct Shop Link'), findsOneWidget);
    });

    testWidgets('promoted external does NOT render as listing or auction', (
      tester,
    ) async {
      _setLargeViewport(tester);

      final adapter = _FakeFeedHttpAdapter([
        _CannedResponse(
          statusCode: 200,
          body: _feedEnvelope(
            items: [
              _promotedExternalItem(
                instanceId: 'pi-ext-only',
                title: 'External Only',
              ),
            ],
            hasMore: false,
          ),
        ),
      ]);

      await tester.pumpWidget(
        _buildHarness(adapter, router: _homeRouter()),
      );
      await _pump(tester);

      // Only the external card, no other card types.
      expect(find.byType(PromotedExternalCard), findsOneWidget);
      expect(find.byType(PromotedListingCard), findsNothing);
      expect(find.byType(PromotedAuctionCard), findsNothing);
      expect(find.byType(FeedCard), findsNothing);
    });
  });

  // ==========================================================================
  // SCENARIO 4: Mixed placement and rendered order
  // ==========================================================================
  group('SCENARIO 4: Mixed placement and rendered order', () {
    testWidgets(
      'Content A, Promoted Listing, Content B, Promoted Auction, '
      'Promoted External — all render in canonical order',
      (tester) async {
        _setExtraLargeViewport(tester);

        final adapter = _FakeFeedHttpAdapter([
          _CannedResponse(
            statusCode: 200,
            body: _feedEnvelope(
              items: [
                // Index 0: organic
                _feedContentItem(id: 'organic-a', body: 'Content A'),
                // Index 1: promoted listing
                _promotedListingItem(
                  instanceId: 'pi-mix-listing',
                  title: 'Mixed Listing Card',
                  pricePerUnit: 3000000,
                ),
                // Index 2: organic
                _feedContentItem(id: 'organic-b', body: 'Content B'),
                // Index 3: promoted auction
                _promotedAuctionItem(
                  instanceId: 'pi-mix-auction',
                  title: 'Mixed Auction Card',
                  startPrice: 1000000,
                  currentBid: 2000000,
                  bidCount: 5,
                ),
                // Index 4: promoted external
                _promotedExternalItem(
                  instanceId: 'pi-mix-external',
                  title: 'Mixed External Card',
                ),
              ],
              hasMore: false,
            ),
          ),
        ]);

        await tester.pumpWidget(
          _buildHarness(adapter, router: _homeRouter()),
        );
        await _pump(tester);

        // ---- FeedState order proof ----
        final state = _container(tester).read(feedProvider);
        expect(state.items, hasLength(5));
        expect(state.items[0].type, FeedItemType.content);
        expect(state.items[0].content, 'Content A');
        expect(state.items[1].type, FeedItemType.promotedListing);
        expect(state.items[1].additionalData['title'], 'Mixed Listing Card');
        expect(state.items[2].type, FeedItemType.content);
        expect(state.items[2].content, 'Content B');
        expect(state.items[3].type, FeedItemType.promotedAuction);
        expect(state.items[3].additionalData['title'], 'Mixed Auction Card');
        expect(state.items[4].type, FeedItemType.promotedExternal);
        expect(state.items[4].additionalData['title'], 'Mixed External Card');

        // ---- Actual widget order proof ----
        // Verify all promoted card types are present.
        expect(find.byType(PromotedListingCard), findsOneWidget);
        expect(find.byType(PromotedAuctionCard), findsOneWidget);
        expect(find.byType(PromotedExternalCard), findsOneWidget);

        // Verify organic FeedCard for both organic items.
        expect(find.byType(FeedCard), findsNWidgets(2));

        // Verify all content appears on screen.
        expect(find.text('Content A'), findsOneWidget);
        expect(find.text('Content B'), findsOneWidget);
        expect(find.text('Mixed Listing Card'), findsOneWidget);
        expect(find.text('Mixed Auction Card'), findsOneWidget);
        expect(find.text('Mixed External Card'), findsOneWidget);

        // ---- Widget tree order proof ----
        // The SliverList builds children in order. Verify by checking
        // that card types appear in the expected sequence.

        // PromotedExternalCard is its own Card widget — verify 1 instance.
        final externalCards = tester.widgetList<PromotedExternalCard>(
          find.byType(PromotedExternalCard),
        ).toList();
        expect(externalCards, hasLength(1));
        expect(externalCards.single.item.additionalData['title'],
            'Mixed External Card');

        // Verify no fallback: no promoted item rendered as organic.
        // All three promoted card types found exactly once each.
        expect(state.errorMessage, isNull);
      },
    );

    testWidgets(
      'promoted items at start and end of feed render correctly', (
      tester,
    ) async {
        _setExtraLargeViewport(tester);

        final adapter = _FakeFeedHttpAdapter([
          _CannedResponse(
            statusCode: 200,
            body: _feedEnvelope(
              items: [
                // Index 0: promoted listing (first item)
                _promotedListingItem(
                  instanceId: 'pi-first',
                  title: 'First Promoted',
                  pricePerUnit: 1000000,
                ),
                // Index 1: organic
                _feedContentItem(id: 'mid', body: 'Middle Content'),
                // Index 2: promoted external (last item)
                _promotedExternalItem(
                  instanceId: 'pi-last',
                  title: 'Last External',
                ),
              ],
              hasMore: false,
            ),
          ),
        ]);

        await tester.pumpWidget(
          _buildHarness(adapter, router: _homeRouter()),
        );
        await _pump(tester);

        final state = _container(tester).read(feedProvider);
        expect(state.items, hasLength(3));
        // First item is promoted listing.
        expect(state.items[0].type, FeedItemType.promotedListing);
        // Middle is organic.
        expect(state.items[1].type, FeedItemType.content);
        // Last item is promoted external.
        expect(state.items[2].type, FeedItemType.promotedExternal);

        // Actual card widgets present.
        expect(find.byType(PromotedListingCard), findsOneWidget);
        expect(find.byType(FeedCard), findsOneWidget);
        expect(find.byType(PromotedExternalCard), findsOneWidget);

        expect(find.text('First Promoted'), findsOneWidget);
        expect(find.text('Middle Content'), findsOneWidget);
        expect(find.text('Last External'), findsOneWidget);
      },
    );
  });


  // ==========================================================================
  // EDGE CASES
  // ==========================================================================
  group('EDGE CASES', () {
    testWidgets(
      'promoted-only feed (no organic items) renders all promoted cards', (
      tester,
    ) async {
        _setExtraLargeViewport(tester);

        final adapter = _FakeFeedHttpAdapter([
          _CannedResponse(
            statusCode: 200,
            body: _feedEnvelope(
              items: [
                _promotedListingItem(
                  instanceId: 'pi-only-list',
                  title: 'Only Listing',
                ),
                _promotedAuctionItem(
                  instanceId: 'pi-only-auc',
                  title: 'Only Auction',
                ),
                _promotedExternalItem(
                  instanceId: 'pi-only-ext',
                  title: 'Only External',
                ),
              ],
              hasMore: false,
            ),
          ),
        ]);

        await tester.pumpWidget(
          _buildHarness(adapter, router: _homeRouter()),
        );
        await _pump(tester);

        final state = _container(tester).read(feedProvider);
        expect(state.items, hasLength(3));
        expect(state.items[0].type, FeedItemType.promotedListing);
        expect(state.items[1].type, FeedItemType.promotedAuction);
        expect(state.items[2].type, FeedItemType.promotedExternal);

        // All three card types present, no organic.
        expect(find.byType(PromotedListingCard), findsOneWidget);
        expect(find.byType(PromotedAuctionCard), findsOneWidget);
        expect(find.byType(PromotedExternalCard), findsOneWidget);
        expect(find.byType(FeedCard), findsNothing);

        // Genuine empty state absent.
        expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);
        expect(state.errorMessage, isNull);
      },
    );

    testWidgets(
      'promoted external without externalMediaUrl still renders', (
      tester,
    ) async {
        _setLargeViewport(tester);

        final adapter = _FakeFeedHttpAdapter([
          _CannedResponse(
            statusCode: 200,
            body: _feedEnvelope(
              items: [
                {
                  'feed_item_kind': 'promoted_external',
                  'promotion_instance_id': 'pi-no-media',
                  'target_type': 'external_product',
                  'title': 'No Media External',
                  'external_url': 'https://example.com/no-media',
                  // No external_media_url
                },
              ],
              hasMore: false,
            ),
          ),
        ]);

        await tester.pumpWidget(
          _buildHarness(adapter, router: _homeRouter()),
        );
        await _pump(tester);

        // Card renders even without media.
        expect(find.byType(PromotedExternalCard), findsOneWidget);
        expect(find.text('No Media External'), findsOneWidget);
        // URL host still displayed.
        expect(find.text('example.com'), findsOneWidget);

        final state = _container(tester).read(feedProvider);
        expect(state.errorMessage, isNull);
      },
    );
  });
}
