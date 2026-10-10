/// CHAT GENERIC PRODUCT CARD — CONTRACT.
///
/// LOCKED: the generic Chat Product Reference Card is a display/reference
/// layer — product identity, price, lifecycle, informational product
/// attributes — plus a whole-card tap into the canonical Commerce detail. It
/// is NOT a mini Commerce screen.
///
/// 1. No Commerce capability matrix: no "Beli Sekarang", no "Bid", no
///    "Kirim Ongkir", no transactional Nego CTA, no Chat/Kelola badge, no
///    button widget of any kind.
/// 2. `Nego` is rendered ONLY from the canonical PRODUCT-LEVEL attribute
///    `ForSaleLivePayload.negotiationEnabled` and is never actionable.
/// 3. The card still renders the canonical price + lifecycle caption and
///    navigates to the canonical detail on a whole-card tap.
/// 4. Tombstone renders no money and cannot navigate.
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/domains/chat/chat/presentation/widgets/chat_resource_projection_card.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';

Map<String, dynamic> _fpsLiveJson({
  String resourceId = 'fps-cta-1',
  bool negotiationEnabled = false,
  String status = 'active',
}) {
  return {
    'state': 'LIVE',
    'resource_type': 'for_sale',
    'resource_id': resourceId,
    'canonical_url': '/for-sale/$resourceId',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': false,
      'blocked_by_tombstone': false,
    },
    'for_sale': {
      'title': 'Kohaku 45 cm',
      'media': [
        {'url': 'https://cdn.example.test/fps.jpg', 'kind': 'image'},
      ],
      'price': {'amount': 1250000, 'currency': 'IDR'},
      'status': status,
      'quantity_available': 3,
      'negotiation_enabled': negotiationEnabled,
      'seller': {
        'user': {
          'id': 'seller-1',
          'username': 'seller_user',
          'lifecycle': 'active',
        },
        'lifecycle': 'active',
      },
    },
  };
}

Map<String, dynamic> _auctionLiveJson({
  String resourceId = 'auction-cta-1',
  String lifecycle = 'active',
}) {
  return {
    'state': 'LIVE',
    'resource_type': 'auction',
    'resource_id': resourceId,
    'canonical_url': '/auction/$resourceId',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': false,
      'blocked_by_tombstone': false,
    },
    'auction': {
      'title': 'Lelang Jumbo',
      'media': [
        {'url': 'https://cdn.example.test/auction.jpg', 'kind': 'image'},
      ],
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
/// resource, so a whole-card tap can be observed.
Future<void> _pumpCard(
  WidgetTester tester,
  ResourceProjection projection, {
  required String detailDestination,
}) async {
  final router = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => Scaffold(
          body: SingleChildScrollView(
            child: ChatResourceProjectionCard(resourceProjection: projection),
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

Future<void> _tapWholeCard(WidgetTester tester) async {
  await tester.tap(find.byType(CommerceMarketplaceCardShell));
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
}

void main() {
  group('for-sale generic card exposes no Commerce CTA', () {
    testWidgets('renders no transactional or capability CTA/badge', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(negotiationEnabled: true)),
        detailDestination: 'detail destination',
      );

      expect(find.text('Beli Sekarang'), findsNothing);
      expect(find.text('Kirim Ongkir'), findsNothing);
      expect(find.text('Chat'), findsNothing);
      expect(find.text('Kelola'), findsNothing);
      expect(find.byType(FilledButton), findsNothing);
      expect(find.byType(ElevatedButton), findsNothing);
      expect(find.byType(OutlinedButton), findsNothing);
      expect(find.byType(TextButton), findsNothing);
    });

    testWidgets('the card still renders the canonical price and lifecycle', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson()),
        detailDestination: 'detail destination',
      );

      // Same money string as discovery — grouped thousands, envelope-owned.
      expect(find.text('Rp 1.250.000'), findsOneWidget);
      expect(find.text('Tersedia'), findsOneWidget);
      expect(_renderedText(tester), isNot(contains('Rp 1250000')));
    });

    testWidgets('a whole-card tap still navigates to the canonical detail', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson()),
        detailDestination: 'detail destination',
      );

      await _tapWholeCard(tester);

      expect(find.text('detail destination'), findsOneWidget);
    });

    testWidgets('a non-available status keeps its honest label and price', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(status: 'sold')),
        detailDestination: 'detail destination',
      );
      expect(find.text('Terjual'), findsOneWidget);
      expect(find.text('Rp 1.250.000'), findsOneWidget);

      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(status: 'unavailable')),
        detailDestination: 'detail destination',
      );
      expect(find.text('Tidak tersedia'), findsOneWidget);
      expect(find.text('Rp 1.250.000'), findsOneWidget);
    });
  });

  group('negotiation is an informational product attribute', () {
    testWidgets('negotiation_enabled=true renders the Nego badge', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(negotiationEnabled: true)),
        detailDestination: 'detail destination',
      );

      expect(find.text('Nego'), findsOneWidget);
    });

    testWidgets('negotiation_enabled=false renders no Nego badge', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_fpsLiveJson(negotiationEnabled: false)),
        detailDestination: 'detail destination',
      );

      expect(find.text('Nego'), findsNothing);
    });
  });

  group('auction generic card exposes no Commerce CTA', () {
    testWidgets('renders no Bid / Beli and no capability badge', (tester) async {
      await _pumpCard(
        tester,
        _parse(_auctionLiveJson()),
        detailDestination: 'bid destination',
      );

      expect(find.text('Bid'), findsNothing);
      expect(find.text('Beli Sekarang'), findsNothing);
      expect(find.text('Chat'), findsNothing);
      expect(find.text('Kelola'), findsNothing);
      expect(find.text('Nego'), findsNothing);
      expect(find.byType(FilledButton), findsNothing);
    });

    testWidgets('the card still renders the current bid and lifecycle', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_auctionLiveJson()),
        detailDestination: 'bid destination',
      );

      // Current bid wins over buy-now, exactly like discovery.
      expect(find.text('Rp 1.450.000'), findsOneWidget);
      expect(find.text('Berlangsung'), findsOneWidget);
      expect(_renderedText(tester), isNot(contains('Rp 1.750.000')));
    });

    testWidgets('a whole-card tap still navigates to the auction detail', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_auctionLiveJson()),
        detailDestination: 'bid destination',
      );

      await _tapWholeCard(tester);

      expect(find.text('bid destination'), findsOneWidget);
    });

    testWidgets('a non-active lifecycle keeps the amount and the mapped label', (
      tester,
    ) async {
      await _pumpCard(
        tester,
        _parse(_auctionLiveJson(lifecycle: 'ended')),
        detailDestination: 'bid destination',
      );

      expect(find.text('Rp 1.450.000'), findsOneWidget);
      expect(find.text('Berlangsung'), findsNothing);
      // ended + has_winner=false → no-winner outcome.
      expect(find.text('Berakhir tanpa pemenang'), findsOneWidget);
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

      final shell = tester.widget<CommerceMarketplaceCardShell>(
        find.byType(CommerceMarketplaceCardShell),
      );
      expect(shell.onTap, isNull);
    });
  });
}
