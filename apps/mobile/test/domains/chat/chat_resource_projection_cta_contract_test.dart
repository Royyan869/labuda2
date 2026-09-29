/// CHAT RESOURCE PROJECTION CTA CONTRACT (owner decision: CTA = navigation
/// shortcut, card carries no price)
///
/// 1. for-sale + can_buy  → "Beli Sekarang" button → delegates to the owning
///    screen (checkout navigation stays in Commerce: product id, fresh preview,
///    seller trust gate). The card itself never resolves a transaction.
/// 2. auction + can_bid   → "Bid" button → the canonical auction detail, which
///    is the bidding surface.
/// 3. PRICE IS RENDERED ON EVERY SURFACE (owner decision, 2026-09-27): the chat
///    card shows the very same canonical money string as discovery, with the
///    availability/lifecycle label kept as the caption.
/// 4. Tombstone / missing capability → no CTA at all, no money rendered, and
///    the identity is still carried for dedup/audit.
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_resource_projection_card.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';

Map<String, dynamic> _fpsLiveJson({
  String resourceId = 'fps-cta-1',
  required bool canBuy,
  bool canBid = false,
  bool canChat = true,
  bool canNegotiate = false,
  bool canManage = false,
  String status = 'available',
}) {
  return {
    'state': 'LIVE',
    'resource_type': 'for_sale',
    'resource_id': resourceId,
    'canonical_url': '/for-sale/$resourceId',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': canBuy || canNegotiate,
      'blocked_by_tombstone': false,
    },
    'commerce_actions': {
      'role': 'buyer',
      'can_chat': canChat,
      'can_negotiate': canNegotiate,
      'can_buy': canBuy,
      'can_bid': canBid,
      'can_manage': canManage,
    },
    'for_sale': {
      'title': 'Kohaku 45 cm',
      'media': [
        {'url': 'https://cdn.example.test/fps.jpg', 'kind': 'image'},
      ],
      // The price exists on the wire and MUST NOT reach the card.
      'price': {'amount': 1250000, 'currency': 'IDR'},
      'status': status,
      'seller': {
        'user': {
          'id': 'seller-1',
          'username': 'seller_user',
          'lifecycle': 'active',
        },
        'lifecycle': 'active',
      },
      'quantity_available': 3,
    },
  };
}

Map<String, dynamic> _auctionLiveJson({
  String resourceId = 'auction-cta-1',
  required bool canBid,
  bool canBuy = false,
  bool canChat = true,
  bool canManage = false,
  String lifecycle = 'active',
}) {
  return {
    'state': 'LIVE',
    'resource_type': 'auction',
    'resource_id': resourceId,
    'canonical_url': '/auction/$resourceId',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': canBid || canBuy,
      'blocked_by_tombstone': false,
    },
    'commerce_actions': {
      'role': 'buyer',
      'can_chat': canChat,
      'can_negotiate': false,
      'can_buy': canBuy,
      'can_bid': canBid,
      'can_manage': canManage,
    },
    'auction': {
      'title': 'Lelang Jumbo',
      'media': [
        {'url': 'https://cdn.example.test/auction.jpg', 'kind': 'image'},
      ],
      // Bid/buy-now amounts exist on the wire and MUST NOT reach the card.
      'current_bid': 1450000,
      'buy_now_price': 1750000,
      'end_at': '2026-12-10T12:34:56Z',
      'lifecycle': lifecycle,
      'seller': {
        'user': {
          'id': 'seller-2',
          'username': 'auction_seller',
          'lifecycle': 'active',
        },
        'lifecycle': 'active',
      },
    },
  };
}

Map<String, dynamic> _fpsTombstoneJson() {
  return {
    'state': 'TOMBSTONE',
    'resource_type': 'for_sale',
    // Canonical contract: identity survives death.
    'resource_id': 'fps-cta-1',
    'viewer_capabilities': {
      'can_view': false,
      'can_interact': false,
      'blocked_by_tombstone': true,
    },
  };
}

ResourceProjection _parse(Map<String, dynamic> json) =>
    ResourceProjection.fromJson(json);

/// Every text actually rendered in the tree, joined for negative assertions.
String _renderedText(WidgetTester tester) => tester
    .widgetList<Text>(find.byType(Text))
    .map((text) => text.data ?? '')
    .join(' | ');

/// Pumps the card under a router with a canonical detail destination for the
/// resource, so a CTA tap can be told apart from a card-body tap.
Future<void> _pumpCard(
  WidgetTester tester,
  ResourceProjection projection, {
  VoidCallback? onBuy,
  required String detailDestination,
}) async {
  final router = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => Scaffold(
          body: SingleChildScrollView(
            child: ChatResourceProjectionCard(
              resourceProjection: projection,
              onBuy: onBuy,
            ),
          ),
        ),
      ),
      GoRoute(
        path: '/for-sale/fps-cta-1',
        builder: (context, state) =>
            Scaffold(body: Text(detailDestination)),
      ),
      GoRoute(
        path: '/auction/auction-cta-1',
        builder: (context, state) =>
            Scaffold(body: Text(detailDestination)),
      ),
    ],
  );

  await tester.pumpWidget(MaterialApp.router(routerConfig: router));
  // Never pumpAndSettle here: the card's AppImage shimmer skeletons animate
  // forever BY DESIGN — bounded pump instead (the test follows the codebase,
  // not the other way round).
  await tester.pump(const Duration(milliseconds: 400));
}

void main() {
  group('for-sale CTA (Beli Sekarang → checkout, delegated)', () {
    testWidgets('can_buy renders the CTA and delegates the buy intent', (
      tester,
    ) async {
      var delegated = 0;
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(canBuy: true)),
        detailDestination: 'detail destination',
        onBuy: () => delegated++,
      );

      expect(find.text('Beli Sekarang'), findsOneWidget);

      await tester.ensureVisible(find.text('Beli Sekarang'));
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Beli Sekarang'));
      await tester.pump(const Duration(milliseconds: 400));

      expect(delegated, 1);
      // The CTA must not silently degrade into a plain card-body navigation.
      expect(find.text('detail destination'), findsNothing);
    });

    testWidgets('without an owner the CTA falls back to the canonical detail', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(canBuy: true)),
        detailDestination: 'detail destination',
      );

      await tester.ensureVisible(find.text('Beli Sekarang'));
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Beli Sekarang'));
      // Route materialisation needs two frames: one to process the router
      // notification, one to finish the transition. Never pumpAndSettle —
      // the card's shimmer skeletons animate forever by design.
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(find.text('detail destination'), findsOneWidget);
    });

    testWidgets('missing can_buy renders no buy CTA', (tester) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(canBuy: false)),
        detailDestination: 'detail destination',
      );

      expect(find.text('Beli Sekarang'), findsNothing);
    });

    testWidgets('the card renders the canonical price plus the status caption', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(canBuy: true)),
        detailDestination: 'detail destination',
      );

      // Same money string as discovery — grouped thousands, envelope-owned.
      expect(find.text('Rp 1.250.000'), findsOneWidget);
      expect(find.text('Tersedia'), findsOneWidget);
      // The raw/ungrouped form is not a display string.
      expect(_renderedText(tester), isNot(contains('Rp 1250000')));
    });

    testWidgets('a non-available status keeps its honest label beside the price', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(canBuy: false, status: 'sold')),
        detailDestination: 'detail destination',
      );
      expect(find.text('Terjual'), findsOneWidget);
      expect(find.text('Rp 1.250.000'), findsOneWidget);

      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(canBuy: false, status: 'inactive')),
        detailDestination: 'detail destination',
      );
      expect(find.text('Tidak tersedia'), findsOneWidget);
      expect(find.text('Rp 1.250.000'), findsOneWidget);
    });
  });

  group('auction CTA (Bid → auction detail, the bidding surface)', () {
    testWidgets('can_bid renders the CTA and navigates to the auction', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_auctionLiveJson(canBid: true)),
        detailDestination: 'bid destination',
      );

      expect(find.text('Bid'), findsOneWidget);

      await tester.ensureVisible(find.text('Bid'));
      await tester.pump(const Duration(milliseconds: 400));
      await tester.tap(find.text('Bid'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(find.text('bid destination'), findsOneWidget);
    });

    testWidgets('missing can_bid renders no bid CTA', (tester) async {
      await _pumpCard(
        tester,
        _parse(_auctionLiveJson(canBid: false, canBuy: false)),
        detailDestination: 'bid destination',
      );

      expect(find.text('Bid'), findsNothing);
    });

    testWidgets('the card renders the current bid plus the lifecycle caption', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_auctionLiveJson(canBid: true)),
        detailDestination: 'bid destination',
      );

      // Current bid wins over buy-now, exactly like discovery.
      expect(find.text('Rp 1.450.000'), findsOneWidget);
      expect(find.text('Berlangsung'), findsOneWidget);
      expect(_renderedText(tester), isNot(contains('Rp 1.750.000')));
    });

    testWidgets('a non-active lifecycle keeps the amount, drops the label', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_auctionLiveJson(canBid: false, lifecycle: 'ended')),
        detailDestination: 'bid destination',
      );

      expect(find.text('Rp 1.450.000'), findsOneWidget);
      expect(find.text('Berlangsung'), findsNothing);
      expect(find.text('ended'), findsOneWidget);
    });
  });

  group('tombstone', () {
    testWidgets('renders no CTA and cannot navigate', (tester) async {
      final projection = _parse(_fpsTombstoneJson());
      // Identity survives death: the envelope still identifies the resource.
      expect(projection.resourceId, 'fps-cta-1');
      expect(projection.isLive, isFalse);

      await _pumpCard(
        tester,
        projection,
        detailDestination: 'detail destination',
      );

      expect(find.byType(FilledButton), findsNothing);
      expect(find.text('Beli Sekarang'), findsNothing);
      // A dead resource never renders money, only the honest block state.
      expect(_renderedText(tester), isNot(contains('Rp')));
      expect(find.text('Tidak dapat ditampilkan'), findsOneWidget);

      final card = tester.widget<ChatResourceProjectionCard>(
        find.byType(ChatResourceProjectionCard),
      );
      expect(card.onBuy, isNull);
    });
  });
}
