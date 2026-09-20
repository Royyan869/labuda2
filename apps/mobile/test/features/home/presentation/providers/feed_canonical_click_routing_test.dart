// ============================================================================
// CANONICAL PROMOTION CLICK ROUTING — FEED CARD BOUNDARY
//
// Proves that a tap on a promoted feed card routes to the CORRECT click
// authority:
//
//   C16: a canonical card (carries canonical_exposure_id) taps → POST
//        /promotions/clicks echoing the exposure identity.
//   C17: the same canonical tap NEVER posts to the legacy /promotions/events
//        (the legacy endpoint is purged).
//   C18: a card without a canonical exposure identity posts NO click of any
//        kind — the legacy /promotions/events click path is purged.
//   C19: no exposure identity → the client never fabricates a click against
//        the canonical endpoint (no /promotions/clicks call at all).
//
// Tests exercise the ACTUAL production card widgets (PromotedForSaleCard,
// PromotedAuctionCard) and the real _recordPromotionClick routing in
// feed_renderers.dart.
// ============================================================================

import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:visibility_detector/visibility_detector.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/services/logger_service.dart';

// ============================================================================
// Captured request model
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
// Fake Dio HttpClientAdapter — captures every POST body
// ============================================================================

class _CaptureHttpAdapter implements HttpClientAdapter {
  final List<_CapturedPost> capturedPosts = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
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
    }
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
  void close({bool force = false}) {}

  List<_CapturedPost> get canonicalClicks =>
      capturedPosts.where((p) => p.isCanonicalClick).toList();

  List<_CapturedPost> get legacyClicks =>
      capturedPosts.where((p) => p.isLegacyClickEvent).toList();
}

ApiClient _fakeApiClient(_CaptureHttpAdapter adapter) {
  final client = ApiClient(logger: null);
  client.dio.httpClientAdapter = adapter;
  return client;
}

// ============================================================================
// FeedItem builders — the canonical discriminant is canonicalExposureId
// ============================================================================

FeedItem _makeFeedItem({
  required String id,
  required FeedItemType type,
  required String contractId,
  required String title,
  Map<String, dynamic> extra = const {},
}) {
  return FeedItem(
    id: id,
    content: title,
    authorId: 'author-1',
    type: type,
    createdAt: DateTime.utc(2026, 8, 5, 10, 0),
    additionalData: <String, dynamic>{
      'isPromoted': true,
      'contractId': contractId,
      'title': title,
      'imageUrl': 'https://example.com/img.jpg',
      'targetType': 'forSale',
      ...extra,
    },
  );
}

FeedItem _listingItem({
  required String contractId,
  String title = 'Click Test ForSale',
  String forSaleId = 'fps-click-1',
  String? canonicalExposureId,
}) {
  return _makeFeedItem(
    id: contractId,
    type: FeedItemType.promotedForSale,
    contractId: contractId,
    title: title,
    extra: {
      'forSaleId': forSaleId,
      'pricePerUnit': 5000000,
      'canonicalExposureId': ?canonicalExposureId,
    },
  );
}

FeedItem _auctionItem({
  required String contractId,
  String title = 'Click Test Auction',
  String auctionId = 'auc-click-1',
  String? canonicalExposureId,
}) {
  return _makeFeedItem(
    id: contractId,
    type: FeedItemType.promotedAuction,
    contractId: contractId,
    title: title,
    extra: {
      'auctionId': auctionId,
      'startPrice': 1000000,
      'currentBid': null,
      'bidCount': 1,
      'endAt': '2026-08-10T10:00:00Z',
      'canonicalExposureId': ?canonicalExposureId,
    },
  );
}

// ============================================================================
// Harness — real card widget + real router ancestor (tap navigates)
// ============================================================================

void _setViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 2400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

Widget _buildCardHarness(
  _CaptureHttpAdapter adapter,
  Widget card,
) {
  final router = GoRouter(
    initialLocation: '/home',
    routes: [
      GoRoute(
        path: '/home',
        builder: (context, state) => Scaffold(
          body: ListView(children: [card]),
        ),
      ),
      GoRoute(
        path: '/for-sale/:forSaleId',
        builder: (context, state) =>
            const Scaffold(body: Text('for-sale-detail')),
      ),
      GoRoute(
        path: '/auction/:auctionId',
        builder: (context, state) =>
            const Scaffold(body: Text('auction-detail')),
      ),
    ],
  );
  return ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),
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

void main() {
  setUp(() {
    resetCanonicalClickAcks();
    // The production VisibilityDetector polls visibility; zero the interval
    // in tests so no periodic timer stays pending (project convention — see
    // feed_promoted_impression_authority_test.dart).
    VisibilityDetectorController.instance.updateInterval = Duration.zero;
  });

  // ==========================================================================
  // C16 + C17 — canonical card tap → /promotions/clicks (never /events)
  // ==========================================================================
  group('C16/C17: canonical forSale card click', () {
    testWidgets(
        'tap echoes the exposure identity to /promotions/clicks and never '
        'calls /promotions/events', (tester) async {
      _setViewport(tester);
      final adapter = _CaptureHttpAdapter();

      const exposureId = 'exposure-canonical-001';
      await tester.pumpWidget(
        _buildCardHarness(
          adapter,
          PromotedForSaleCard(
            item: _listingItem(
              contractId: 'promo-canonical-001',
              canonicalExposureId: exposureId,
            ),
          ),
        ),
      );
      await _pump(tester);

      await tester.tap(find.byType(PromotedForSaleCard));
      await _pump(tester);

      // C16 — exactly one canonical click acknowledgement echoing the
      // exposure identity (never a naked promotion id).
      expect(adapter.canonicalClicks, hasLength(1));
      final click = adapter.canonicalClicks.first;
      expect(click.exposureId, exposureId);
      expect(click.body.containsKey('promotion_id'), isFalse,
          reason: 'the server derives the promotion from the exposure');
      expect(click.body.containsKey('promotion_instance_id'), isFalse,
          reason: 'canonical clicks never carry a legacy instance id');

      // C17 — no legacy /promotions/events click for this canonical card.
      expect(adapter.legacyClicks, isEmpty,
          reason: 'canonical clicks must never enter the legacy events path');
    });

    testWidgets('canonical auction card tap routes the same way', (
      tester,
    ) async {
      _setViewport(tester);
      final adapter = _CaptureHttpAdapter();

      const exposureId = 'exposure-canonical-002';
      await tester.pumpWidget(
        _buildCardHarness(
          adapter,
          PromotedAuctionCard(
            item: _auctionItem(
              contractId: 'promo-canonical-002',
              canonicalExposureId: exposureId,
            ),
          ),
        ),
      );
      await _pump(tester);

      await tester.tap(find.byType(PromotedAuctionCard));
      await _pump(tester);

      expect(adapter.canonicalClicks, hasLength(1));
      expect(adapter.canonicalClicks.first.exposureId, exposureId);
      expect(adapter.legacyClicks, isEmpty);
    });
  });

  // ==========================================================================
  // C18 — no exposure identity → no click of any kind
  // ==========================================================================
  group('C18: card without exposure identity posts no click', () {
    testWidgets('no exposure identity → no canonical click, no legacy click', (
      tester,
    ) async {
      _setViewport(tester);
      final adapter = _CaptureHttpAdapter();

      await tester.pumpWidget(
        _buildCardHarness(
          adapter,
          PromotedForSaleCard(
            item: _listingItem(contractId: 'instance-no-exposure-001'),
          ),
        ),
      );
      await _pump(tester);

      await tester.tap(find.byType(PromotedForSaleCard));
      await _pump(tester);

      // C18 — the legacy /promotions/events click path is purged: a card
      // without a canonical exposure identity posts NOTHING.
      expect(adapter.legacyClicks, isEmpty,
          reason: 'the legacy /promotions/events endpoint is purged');

      // C19 — no exposure identity ever reaches the canonical click endpoint.
      expect(adapter.canonicalClicks, isEmpty,
          reason: 'the client must not fabricate an exposure-based click');
    });
  });

  // ==========================================================================
  // C19 — no click of any kind without a card identity
  // ==========================================================================
  group('C19: no fabricated click without an identity', () {
    testWidgets('a card with no instance and no exposure posts nothing', (
      tester,
    ) async {
      _setViewport(tester);
      final adapter = _CaptureHttpAdapter();

      // A card that cannot navigate (forSaleId null) and has no instance id.
      await tester.pumpWidget(
        _buildCardHarness(
          adapter,
          PromotedForSaleCard(
            item: _makeFeedItem(
              id: 'no-identity',
              type: FeedItemType.promotedForSale,
              contractId: '',
              title: 'No Identity Card',
            ),
          ),
        ),
      );
      await _pump(tester);

      final cardFinder = find.byType(PromotedForSaleCard);
      if (tester.any(cardFinder)) {
        await tester.tap(cardFinder, warnIfMissed: false);
      }
      await _pump(tester);

      expect(adapter.canonicalClicks, isEmpty);
      expect(adapter.legacyClicks, isEmpty);
    });
  });
}
