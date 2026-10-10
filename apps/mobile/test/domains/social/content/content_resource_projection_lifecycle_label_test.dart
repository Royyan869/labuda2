// COMMENT product reference card uses the SHARED lifecycle mapping.
//
// Proves the Comment card (ContentResourceProjectionCard) renders
// `commerceLifecycleLabel` (the same mapping the Chat card uses) for the
// canonical For Sale + Auction lifecycle projections.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/social/content/presentation/widgets/content_resource_projection_card.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';

ResourceProjection _forSale(String status) => ResourceProjection.fromJson({
  'state': 'LIVE',
  'resource_type': 'for_sale',
  'resource_id': 'fps-1',
  'canonical_url': '/for-sale/fps-1',
  'viewer_capabilities': {
    'can_view': true,
    'can_interact': true,
    'blocked_by_tombstone': false,
  },
  'for_sale': {
    'title': 'Koi',
    'media': const [],
    'price': {'amount': 1000, 'currency': 'IDR'},
    'status': status,
    'quantity_available': 1,
    'seller': {
      'user': {'id': 's1', 'username': 'seller', 'lifecycle': 'active'},
      'lifecycle': 'active',
    },
  },
});

ResourceProjection _auction(String lifecycle, {bool hasWinner = false}) =>
    ResourceProjection.fromJson({
      'state': 'LIVE',
      'resource_type': 'auction',
      'resource_id': 'auc-1',
      'canonical_url': '/auction/auc-1',
      'viewer_capabilities': {
        'can_view': true,
        'can_interact': true,
        'blocked_by_tombstone': false,
      },
      'auction': {
        'title': 'Lelang Koi',
        'media': const [],
        'current_bid': 1000,
        'end_at': '2026-12-10T12:34:56Z',
        'lifecycle': lifecycle,
        'has_winner': hasWinner,
        'seller': {
          'user': {'id': 's1', 'username': 'seller', 'lifecycle': 'active'},
          'lifecycle': 'active',
        },
      },
    });

Future<void> _pump(WidgetTester tester, ResourceProjection projection) async {
  await tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(
          child: ContentResourceProjectionCard(
            resourceProjection: projection,
            onTap: () {},
          ),
        ),
      ),
    ),
  );
  // Bounded pump: the card's media shimmer animates forever by design.
  await tester.pump(const Duration(milliseconds: 400));
}

void main() {
  final cases = <ResourceProjection, String>{
    _forSale('active'): 'Tersedia',
    _forSale('sold'): 'Terjual',
    _forSale('unavailable'): 'Tidak tersedia',
    _auction('scheduled'): 'Terjadwal',
    _auction('active'): 'Berlangsung',
    _auction('waiting_settlement'): 'Menunggu Penyelesaian',
    _auction('ended', hasWinner: true): 'Terjual',
    _auction('ended'): 'Berakhir tanpa pemenang',
    _auction('cancelled'): 'Dibatalkan',
  };

  testWidgets('Comment card renders the shared lifecycle labels', (
    tester,
  ) async {
    for (final entry in cases.entries) {
      await _pump(tester, entry.key);
      expect(
        find.text(entry.value),
        findsOneWidget,
        reason: 'expected label "${entry.value}" for the projected lifecycle',
      );
      expect(tester.takeException(), isNull);
    }
  });
}
