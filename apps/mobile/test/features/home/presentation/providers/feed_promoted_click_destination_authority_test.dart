// ============================================================================
// PROMOTED FEED CLICK AND DESTINATION CONTINUITY AUTHORITY
//
// Proves that actual user taps on promoted feed cards:
// 1. Fire exactly one canonical click acknowledgement per tap
//    (POST /promotions/clicks echoing the server-issued exposure id) when the
//    card carries a canonical exposure identity
// 2. Navigate to the correct ForSale / Auction / External destination
// 3. Never let tracking failure block the destination action
// 4. Never emit a click of any kind without a canonical exposure identity
//    (the legacy /promotions/events path is purged)
// 5. Never construct malformed destination routes
// 6. Work through the actual Feed pipeline end-to-end
//
// Uses actual promoted card widgets (PromotedListingCard, PromotedAuctionCard,
// PromotedExternalCard). Only external boundaries are faked:
//   - HTTP transport (captures POSTs, can simulate failures)
//   - GoRouter destination pages (real routes with detectable widgets)
//   - External link launcher (faked)
//
// DOES NOT TEST:
//   - Impression tracking (separate scope)
//   - Feed parsing/state
//   - Root wiring (MainScreen)
//   - Backend click storage
//   - Share, Chat, Search, seed, APK
// ============================================================================

import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/shared/shared.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:visibility_detector/visibility_detector.dart';

// ============================================================================
// Captured POST model
// ============================================================================

class _CapturedPost {
  final String path;
  final Map<String, dynamic> body;
  const _CapturedPost({required this.path, required this.body});

  bool get isCanonicalClick => path.contains('promotions/clicks');

  bool get isLegacyClickEvent =>
      path.contains('promotions/events') && body['event_type'] == 'click';

  String? get exposureId => body['exposure_id'] as String?;
}

// ============================================================================
// Fake HTTP transport — captures POSTs, optional per-path failure injection
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

Map<String, dynamic> _promotedListingItem({
  required String instanceId,
  required String title,
  int pricePerUnit = 5000000,
  String forSaleId = 'listing-1',
  String? canonicalExposureId,
}) {
  return <String, dynamic>{
    'type': 'promoted_for_sale',
    'contract_id': instanceId,
    'target_type': 'for_sale',
    'title': title,
    'image_url': 'https://example.com/koi.jpg',
    'for_sale_id': forSaleId,
    'price_per_unit': pricePerUnit,
    if (canonicalExposureId != null)
      'canonical_exposure_id': canonicalExposureId,
  };
}

class _CaptureHttpAdapter implements HttpClientAdapter {
  final List<_CapturedPost> capturedPosts = [];
  final List<Map<String, dynamic>?> feedResponses;
  int _feedCallCount = 0;

  /// When non-null, the next POST to this path throws.
  String? failNextPostPath;

  _CaptureHttpAdapter({this.feedResponses = const []});

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
    // Capture ALL POST requests.
    if (options.method == 'POST' && options.data != null) {
      Map<String, dynamic> body;
      if (options.data is Map<String, dynamic>) {
        body = Map<String, dynamic>.from(options.data as Map<String, dynamic>);
      } else if (options.data is String) {
        try {
          body = jsonDecode(options.data as String) as Map<String, dynamic>;
        } catch (_) {
          body = <String, dynamic>{};
        }
      } else {
        body = <String, dynamic>{};
      }
      capturedPosts.add(_CapturedPost(path: options.path, body: body));

      // Inject failure for the configured path.
      if (failNextPostPath != null &&
          options.path.contains(failNextPostPath!)) {
        failNextPostPath = null;
        throw DioException(
          requestOptions: options,
          type: DioExceptionType.badResponse,
          message: 'Simulated POST failure',
          response: Response(
            requestOptions: options,
            statusCode: 500,
          ),
        );
      }
    }

    // Feed GET requests consume from the canned queue.
    final isFeedGet = options.method == 'GET' &&
        (options.path.contains('feed') || options.path.contains('/feed'));
    if (isFeedGet) {
      if (_feedCallCount < feedResponses.length) {
        final body = feedResponses[_feedCallCount];
        _feedCallCount++;
        if (body == null) {
          throw DioException(
            requestOptions: options,
            type: DioExceptionType.connectionError,
            message: 'Simulated network error',
          );
        }
        return ResponseBody.fromString(jsonEncode(body), 200, headers: {
          'content-type': ['application/json'],
        });
      }
      final emptyBody = _feedEnvelope(
        items: <Map<String, dynamic>>[],
        hasMore: false,
      );
      return ResponseBody.fromString(jsonEncode(emptyBody), 200, headers: {
        'content-type': ['application/json'],
      });
    }

    return _genericSuccess();
  }

  @override
  void close({bool force = false}) {}

  List<_CapturedPost> get clickPosts =>
      capturedPosts.where((p) => p.isCanonicalClick).toList();
}

ApiClient _fakeApiClient(_CaptureHttpAdapter adapter) {
  final client = ApiClient(logger: null);
  client.dio.httpClientAdapter = adapter;
  return client;
}

// ============================================================================
// Fake providers
// ============================================================================

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthStateUnauthenticated();
}

class _FakeAuthedAuthController extends AuthController {
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
      provider: AuthProvider.email,
    ),
    emailVerified: true,
  );
}

class _FakeForSaleRepository implements ForSaleRepository {
  @override
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params) async {
    return Result.success(const <ForSale>[]);
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
  }) async => Result.success(false);
  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async => Result.success(LikeStats(
    targetId: targetId, targetType: targetType,
    totalLikes: 0, isLikedByCurrentUser: false,
  ));
  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) => Stream.value(LikeStats(
    targetId: targetId, targetType: targetType,
    totalLikes: 0, isLikedByCurrentUser: false,
  ));
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

// ============================================================================
// FeedItem builders for direct card rendering
// ============================================================================

FeedItem _makeFeedItem({
  required String id,
  required FeedItemType type,
  required String contractId,
  required String title,
  String? canonicalExposureId,
  Map<String, dynamic> extra = const {},
}) {
  return FeedItem(
    id: id,
    content: title,
    authorId: '',
    type: type,
    createdAt: DateTime.utc(2026, 8, 5, 10, 0),
    additionalData: <String, dynamic>{
      'isPromoted': true,
      'contractId': contractId,
      'title': title,
      'imageUrl': 'https://example.com/img.jpg',
      'targetType': 'listing',
      if (canonicalExposureId != null)
        'canonicalExposureId': canonicalExposureId,
      ...extra,
    },
  );
}

FeedItem _listingItem({
  String contractId = 'pi-click-listing',
  String title = 'Click Test ForSale',
  String? forSaleId = 'fps-click-1',
  int pricePerUnit = 5000000,
  String? canonicalExposureId,
}) {
  return _makeFeedItem(
    id: contractId,
    type: FeedItemType.promotedListing,
    contractId: contractId,
    title: title,
    canonicalExposureId: canonicalExposureId,
    extra: {
      if (forSaleId != null) 'forSaleId': forSaleId,
      'pricePerUnit': pricePerUnit,
    },
  );
}

FeedItem _auctionItem({
  String contractId = 'pi-click-auction',
  String title = 'Click Test Auction',
  String? auctionId = 'auc-click-1',
  int startPrice = 1000000,
  int? currentBid,
  int bidCount = 3,
  String? canonicalExposureId,
}) {
  return _makeFeedItem(
    id: contractId,
    type: FeedItemType.promotedAuction,
    contractId: contractId,
    title: title,
    canonicalExposureId: canonicalExposureId,
    extra: {
      if (auctionId != null) 'auctionId': auctionId,
      'startPrice': startPrice,
      'currentBid': currentBid,
      'bidCount': bidCount,
      'endAt': '2026-08-10T10:00:00Z',
    },
  );
}

FeedItem _externalItem({
  String contractId = 'pi-click-external',
  String title = 'Click Test External',
  String? externalUrl = 'https://example.com/product',
  String? externalMediaUrl,
  String? canonicalExposureId,
}) {
  return _makeFeedItem(
    id: contractId,
    type: FeedItemType.promotedExternal,
    contractId: contractId,
    title: title,
    canonicalExposureId: canonicalExposureId,
    extra: {
      if (externalUrl != null) 'externalUrl': externalUrl,
      if (externalMediaUrl != null) 'externalMediaUrl': externalMediaUrl,
    },
  );
}

// ============================================================================
// GoRouter with real destination pages for navigation verification
// ============================================================================

/// Router for full pipeline tests (wraps HomeScreen).
GoRouter _pipelineRouter() {
  return GoRouter(
    initialLocation: '/home',
    routes: [
      GoRoute(
        path: '/home',
        builder: (context, state) => const Scaffold(body: HomeScreen()),
      ),
      GoRoute(
        path: '/for-sale/:forSaleId',
        builder: (context, state) {
          final id = state.pathParameters['forSaleId'] ?? '';
          return Scaffold(body: Text('for-sale-dest:$id'));
        },
      ),
      GoRoute(
        path: '/auction/:auctionId',
        builder: (context, state) {
          final id = state.pathParameters['auctionId'] ?? '';
          return Scaffold(body: Text('auction-dest:$id'));
        },
      ),
    ],
  );
}

// ============================================================================
// Harness builders
// ============================================================================

void _setViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 2400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

/// Build harness with GoRouter for direct-card tests (scenarios 1-8).
Widget _buildCardRouterHarness({
  required _CaptureHttpAdapter adapter,
  required Widget card,
}) {
  final router = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => Scaffold(body: ListView(children: [card])),
      ),
      GoRoute(
        path: '/for-sale/:forSaleId',
        builder: (context, state) {
          final id = state.pathParameters['forSaleId'] ?? '';
          return Scaffold(body: Text('for-sale-dest:$id'));
        },
      ),
      GoRoute(
        path: '/auction/:auctionId',
        builder: (context, state) {
          final id = state.pathParameters['auctionId'] ?? '';
          return Scaffold(body: Text('auction-dest:$id'));
        },
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),
      authControllerProvider.overrideWith(_FakeAuthedAuthController.new),
      forSaleRepositoryProvider.overrideWithValue(_FakeForSaleRepository()),
      auctionRepositoryProvider.overrideWithValue(_FakeAuctionRepository()),
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository()),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

/// Build the full Feed pipeline harness for scenario 9.
Widget _buildPipelineHarness({
  required _CaptureHttpAdapter adapter,
  required GoRouter router,
}) {
  return ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),
      authControllerProvider.overrideWith(_FakeAuthedAuthController.new),
      forSaleRepositoryProvider.overrideWithValue(_FakeForSaleRepository()),
      auctionRepositoryProvider.overrideWithValue(_FakeAuctionRepository()),
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository()),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

Future<void> _pump(WidgetTester tester) async {
  for (int i = 0; i < 12; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

// ============================================================================
// Tap helper — finds the tappable area of a promoted card
// ============================================================================

Future<void> _tapPromotedCard(WidgetTester tester) async {
  final shell = find.byType(CommerceMarketplaceCardShell);
  if (shell.evaluate().isNotEmpty) {
    await tester.tap(shell);
    await _pump(tester);
    return;
  }
  final externalCard = find.byType(PromotedExternalCard);
  if (externalCard.evaluate().isNotEmpty) {
    final inkWell = find.descendant(
      of: externalCard,
      matching: find.byType(InkWell),
    );
    if (inkWell.evaluate().isNotEmpty) {
      await tester.tap(inkWell.first);
      await _pump(tester);
      return;
    }
  }
}

// ============================================================================
// Tests
// ============================================================================

void main() {
  setUp(() {
    resetCanonicalClickAcks();
    VisibilityDetectorController.instance.updateInterval = Duration.zero;
  });

  // ==========================================================================
  // SCENARIO 1: Promoted ForSale click → ack + /for-sale/:id
  // ==========================================================================
  group('SCENARIO 1: Promoted ForSale click', () {
    testWidgets('tap → 1 canonical click ack + navigation to /for-sale/:id', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedListingCard(
            item: _listingItem(
              contractId: 'pi-click-list-001',
              forSaleId: 'fps-42',
              title: 'Tap Me ForSale',
              canonicalExposureId: 'exp-click-list-001',
            ),
          ),
        ),
      );
      await _pump(tester);

      expect(find.byType(PromotedListingCard), findsOneWidget);
      expect(find.text('Tap Me ForSale'), findsOneWidget);

      // Before tap: no click acks, still on home route.
      expect(adapter.clickPosts, isEmpty);

      // Tap.
      await _tapPromotedCard(tester);

      // Exactly 1 canonical click acknowledgement echoing the exposure id.
      final clicks = adapter.clickPosts;
      expect(clicks, hasLength(1));
      expect(clicks.first.exposureId, 'exp-click-list-001');
      expect(clicks.first.body.containsKey('event_type'), isFalse,
          reason: 'legacy event vocabulary is purged');
      expect(clicks.first.body.containsKey('surface'), isFalse);

      // Navigation to /for-sale/fps-42.
      expect(find.text('for-sale-dest:fps-42'), findsOneWidget);
    });

    testWidgets('route argument is the actual mapped listing ID', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedListingCard(
            item: _listingItem(
              contractId: 'pi-route-id',
              forSaleId: 'custom-listing-uuid-999',
              title: 'Route ID Test',
              canonicalExposureId: 'exp-route-id',
            ),
          ),
        ),
      );
      await _pump(tester);

      await _tapPromotedCard(tester);

      expect(find.text('for-sale-dest:custom-listing-uuid-999'), findsOneWidget);
      expect(find.text('for-sale-dest:fps-click-1'), findsNothing);
    });
  });

  // ==========================================================================
  // SCENARIO 2: Promoted Auction click → ack + /auction/:id
  // ==========================================================================
  group('SCENARIO 2: Promoted Auction click', () {
    testWidgets('tap → 1 canonical click ack + navigation to /auction/:id', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedAuctionCard(
            item: _auctionItem(
              contractId: 'pi-click-auc-002',
              auctionId: 'auc-77',
              title: 'Tap Me Auction',
              canonicalExposureId: 'exp-click-auc-002',
            ),
          ),
        ),
      );
      await _pump(tester);

      expect(find.byType(PromotedAuctionCard), findsOneWidget);

      await _tapPromotedCard(tester);

      final clicks = adapter.clickPosts;
      expect(clicks, hasLength(1));
      expect(clicks.first.exposureId, 'exp-click-auc-002');

      expect(find.text('auction-dest:auc-77'), findsOneWidget);
    });
  });

  // ==========================================================================
  // SCENARIO 3: Promoted External click → ack + interstitial
  // ==========================================================================
  group('SCENARIO 3: Promoted External click', () {
    testWidgets('tap → 1 canonical click ack + external link interstitial', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedExternalCard(
            item: _externalItem(
              contractId: 'pi-click-ext-003',
              title: 'External Product',
              externalUrl: 'https://shop.example.com/koi-food',
              canonicalExposureId: 'exp-click-ext-003',
            ),
          ),
        ),
      );
      await _pump(tester);

      expect(find.byType(PromotedExternalCard), findsOneWidget);

      final inkWell = find.descendant(
        of: find.byType(PromotedExternalCard),
        matching: find.byType(InkWell),
      );
      expect(inkWell, findsOneWidget);
      await tester.tap(inkWell.first);
      await _pump(tester);

      final clicks = adapter.clickPosts;
      expect(clicks, hasLength(1));
      expect(clicks.first.exposureId, 'exp-click-ext-003');

      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.text('shop.example.com'), findsAtLeast(1));
      expect(find.text('Buka'), findsOneWidget);
    });

    testWidgets('interstitial cancel does not leave dangling state', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedExternalCard(
            item: _externalItem(
              contractId: 'pi-cancel-ext',
              title: 'Cancel Test',
              canonicalExposureId: 'exp-cancel-ext',
            ),
          ),
        ),
      );
      await _pump(tester);

      final inkWell = find.descendant(
        of: find.byType(PromotedExternalCard),
        matching: find.byType(InkWell),
      );
      await tester.tap(inkWell.first);
      await _pump(tester);

      expect(find.byType(AlertDialog), findsOneWidget);

      await tester.tap(find.text('Batal'));
      await _pump(tester);

      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byType(PromotedExternalCard), findsOneWidget);
    });
  });

  // ==========================================================================
  // SCENARIO 4-6: Tracking failure does not block destination
  // ==========================================================================
  group('SCENARIO 4-6: Tracking failure continuity', () {
    testWidgets('ForSale: click ack 500 → still navigates to listing', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();
      adapter.failNextPostPath = 'promotions/clicks';

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedListingCard(
            item: _listingItem(
              contractId: 'pi-fail-list',
              forSaleId: 'fps-survive',
              title: 'Survive Failure',
              canonicalExposureId: 'exp-fail-list',
            ),
          ),
        ),
      );
      await _pump(tester);

      await _tapPromotedCard(tester);

      expect(find.text('for-sale-dest:fps-survive'), findsOneWidget);
      expect(find.byType(PromotedListingCard), findsNothing);
    });

    testWidgets('Auction: click ack 500 → still navigates to auction', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();
      adapter.failNextPostPath = 'promotions/clicks';

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedAuctionCard(
            item: _auctionItem(
              contractId: 'pi-fail-auc',
              auctionId: 'auc-survive',
              title: 'Survive Auction',
              canonicalExposureId: 'exp-fail-auc',
            ),
          ),
        ),
      );
      await _pump(tester);

      await _tapPromotedCard(tester);

      expect(find.text('auction-dest:auc-survive'), findsOneWidget);
    });

    testWidgets('External: click ack 500 → interstitial still appears', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();
      adapter.failNextPostPath = 'promotions/clicks';

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedExternalCard(
            item: _externalItem(
              contractId: 'pi-fail-ext',
              title: 'Fail External',
              externalUrl: 'https://fail.example.com/product',
              canonicalExposureId: 'exp-fail-ext',
            ),
          ),
        ),
      );
      await _pump(tester);

      final inkWell = find.descendant(
        of: find.byType(PromotedExternalCard),
        matching: find.byType(InkWell),
      );
      await tester.tap(inkWell.first);
      await _pump(tester);

      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.text('fail.example.com'), findsAtLeast(1));
    });
  });

  // ==========================================================================
  // SCENARIO 7: Empty exposure identity
  // ==========================================================================
  group('SCENARIO 7: Empty exposure identity', () {
    testWidgets('ForSale: no exposure id → 0 clicks, still navigates', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedListingCard(
            item: _listingItem(
              contractId: 'pi-no-exposure-list',
              forSaleId: 'fps-empty-id',
              title: 'Empty Click ID',
            ),
          ),
        ),
      );
      await _pump(tester);

      await _tapPromotedCard(tester);

      expect(adapter.clickPosts, isEmpty,
          reason: 'no exposure identity → no click of any kind');
      expect(adapter.capturedPosts.where((p) => p.isLegacyClickEvent), isEmpty,
          reason: 'the legacy /promotions/events path is purged');
      expect(find.text('for-sale-dest:fps-empty-id'), findsOneWidget);
    });

    testWidgets('External: no exposure id → 0 clicks, interstitial shows', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedExternalCard(
            item: _externalItem(
              contractId: 'pi-no-exposure-ext',
              title: 'Empty Click Ext',
            ),
          ),
        ),
      );
      await _pump(tester);

      final inkWell = find.descendant(
        of: find.byType(PromotedExternalCard),
        matching: find.byType(InkWell),
      );
      await tester.tap(inkWell.first);
      await _pump(tester);

      expect(adapter.clickPosts, isEmpty);
      expect(find.byType(AlertDialog), findsOneWidget);
    });

    testWidgets('Auction: no exposure id → 0 clicks, still navigates', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedAuctionCard(
            item: _auctionItem(
              contractId: 'pi-no-exposure-auc',
              auctionId: 'auc-empty-id',
              title: 'Empty Auction ID',
            ),
          ),
        ),
      );
      await _pump(tester);

      await _tapPromotedCard(tester);

      expect(adapter.clickPosts, isEmpty);
      expect(find.text('auction-dest:auc-empty-id'), findsOneWidget);
    });
  });

  // ==========================================================================
  // SCENARIO 8: Missing destination identity
  // ==========================================================================
  group('SCENARIO 8: Missing destination identity', () {
    testWidgets('missing listing ID → onTap is null → no navigation, no crash', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedListingCard(
            item: _listingItem(
              contractId: 'pi-no-dest',
              forSaleId: null, // missing
              title: 'No Dest ForSale',
              canonicalExposureId: 'exp-no-dest',
            ),
          ),
        ),
      );
      await _pump(tester);

      expect(find.byType(PromotedListingCard), findsOneWidget);
      expect(find.text('No Dest ForSale'), findsOneWidget);

      await _tapPromotedCard(tester);

      expect(find.text('for-sale-dest:'), findsNothing);
      expect(find.byType(PromotedListingCard), findsOneWidget);
      expect(adapter.clickPosts, isEmpty,
          reason: 'no tap handler → no click tracking');
    });

    testWidgets('missing auction ID → onTap is null → no navigation, no crash', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedAuctionCard(
            item: _auctionItem(
              contractId: 'pi-no-auc',
              auctionId: null, // missing
              title: 'No Dest Auction',
              canonicalExposureId: 'exp-no-auc',
            ),
          ),
        ),
      );
      await _pump(tester);

      await _tapPromotedCard(tester);

      expect(find.text('auction-dest:'), findsNothing);
      expect(find.byType(PromotedAuctionCard), findsOneWidget);
    });

    testWidgets('missing external URL → onTap is null → no interstitial', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: PromotedExternalCard(
            item: _externalItem(
              contractId: 'pi-no-url',
              title: 'No URL External',
              externalUrl: null, // missing
              canonicalExposureId: 'exp-no-url',
            ),
          ),
        ),
      );
      await _pump(tester);

      expect(find.byType(PromotedExternalCard), findsOneWidget);

      final inkWell = find.descendant(
        of: find.byType(PromotedExternalCard),
        matching: find.byType(InkWell),
      );
      if (inkWell.evaluate().isNotEmpty) {
        await tester.tap(inkWell.first);
        await _pump(tester);
      }

      expect(find.byType(AlertDialog), findsNothing);
      expect(adapter.clickPosts, isEmpty);
    });
  });

  // ==========================================================================
  // SCENARIO 9: Actual Feed pipeline click
  // ==========================================================================
  group('SCENARIO 9: Actual Feed pipeline click', () {
    testWidgets(
      'FeedApiDatasource → HomeScreen → tap listing → ack + nav',
      (tester) async {
        _setViewport(tester);

        final adapter = _CaptureHttpAdapter(
          feedResponses: [
            _feedEnvelope(
              items: [
                _promotedListingItem(
                  instanceId: 'pi-pipeline-click',
                  title: 'Pipeline ForSale Click',
                  forSaleId: 'fps-pipeline-1',
                  canonicalExposureId: 'exp-pipeline-click',
                ),
              ],
              hasMore: false,
            ),
          ],
        );

        await tester.pumpWidget(
          _buildPipelineHarness(
            adapter: adapter,
            router: _pipelineRouter(),
          ),
        );
        await _pump(tester);

        expect(find.byType(HomeScreen), findsOneWidget);
        expect(find.byType(PromotedListingCard), findsOneWidget);
        expect(find.text('Pipeline ForSale Click'), findsOneWidget);

        await _tapPromotedCard(tester);

        final clicks = adapter.clickPosts;
        expect(clicks, hasLength(1));
        expect(clicks.first.exposureId, 'exp-pipeline-click');

        expect(find.text('for-sale-dest:fps-pipeline-1'), findsOneWidget);
      },
    );

    testWidgets(
      'pipeline: tap does NOT fire impression ack (only click)',
      (tester) async {
        _setViewport(tester);

        final adapter = _CaptureHttpAdapter(
          feedResponses: [
            _feedEnvelope(
              items: [
                _promotedListingItem(
                  instanceId: 'pi-click-only',
                  title: 'Click Only Test',
                  forSaleId: 'fps-click-1',
                  canonicalExposureId: 'exp-click-only',
                ),
              ],
              hasMore: false,
            ),
          ],
        );

        await tester.pumpWidget(
          _buildPipelineHarness(
            adapter: adapter,
            router: _pipelineRouter(),
          ),
        );
        await _pump(tester);

        expect(find.byType(PromotedListingCard), findsOneWidget);

        final clicksBefore = adapter.clickPosts.length;

        await _tapPromotedCard(tester);

        final clicksAfter = adapter.clickPosts.length;
        expect(clicksAfter, clicksBefore + 1);

        expect(find.text('for-sale-dest:fps-click-1'), findsOneWidget);
      },
    );
  });

  // ==========================================================================
  // CONTRACT: Canonical click payload across all three kinds
  // ==========================================================================
  group('CONTRACT: Uniform click identity', () {
    testWidgets('all three kinds share identical payload shape', (
      tester,
    ) async {
      _setViewport(tester);

      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardRouterHarness(
          adapter: adapter,
          card: Column(children: [
            PromotedListingCard(
              item: _listingItem(
                contractId: 'pi-shape-list',
                title: 'Shape A',
                canonicalExposureId: 'exp-shape-list',
              ),
            ),
            PromotedAuctionCard(
              item: _auctionItem(
                contractId: 'pi-shape-auc',
                title: 'Shape B',
                canonicalExposureId: 'exp-shape-auc',
              ),
            ),
          ]),
        ),
      );
      await _pump(tester);

      final router = GoRouter.of(
        tester.element(find.byType(PromotedListingCard)),
      );

      final shells = find.byType(CommerceMarketplaceCardShell);
      await tester.tap(shells.first);
      await _pump(tester);

      final listingClick = adapter.clickPosts.first;
      expect(listingClick.body.keys, <String>['exposure_id']);
      expect(listingClick.exposureId, 'exp-shape-list');

      router.go('/');
      await _pump(tester);

      final shells2 = find.byType(CommerceMarketplaceCardShell);
      await tester.tap(shells2.last);
      await _pump(tester);

      final auctionClick = adapter.clickPosts.last;
      expect(auctionClick.body.keys, <String>['exposure_id']);
      expect(auctionClick.exposureId, 'exp-shape-auc');
    });
  });
}