import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/social/comment/domain/entities/comment.dart';
import 'package:hishumi/domains/social/comment/presentation/widgets/comment_card.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/domains/social/content/presentation/widgets/content_resource_projection_card.dart';
import 'package:hishumi/shared/attachment/entities/share_reference.dart';
import 'package:hishumi/shared/object/presentation/widgets/object_preview_card.dart';

/// Canonical fixtures — the wire shapes the backend projection authority emits
/// for a commerce-reference comment (see CommentResponse.resource_projection).
/// Product attributes live on the payload; a TOMBSTONE carries none of the
/// live halves (canonical_url, payload) but always keeps the id.
Map<String, dynamic> _liveEnvelope(String resourceType, String id) => {
  'state': 'LIVE',
  'resource_type': resourceType,
  'resource_id': id,
  'canonical_url': '/${resourceType.replaceAll('_', '-')}/$id',
  'viewer_capabilities': {
    'can_view': true,
    'can_interact': true,
    'blocked_by_tombstone': false,
  },
};

Map<String, dynamic> _tombstoneEnvelope(String resourceType, String id) => {
  'state': 'TOMBSTONE',
  'resource_type': resourceType,
  'resource_id': id,
  'viewer_capabilities': {
    'can_view': false,
    'can_interact': false,
    'blocked_by_tombstone': true,
  },
};

ResourceProjection _forSaleProjection({
  String id = 'sale-1',
  String state = 'LIVE',
  bool negotiationEnabled = true,
}) {
  if (state == 'TOMBSTONE') {
    return ResourceProjection.fromJson(_tombstoneEnvelope('for_sale', id));
  }
  return ResourceProjection.fromJson({
    ..._liveEnvelope('for_sale', id),
    'for_sale': {
      'title': 'Kohaku 50cm',
      'media': <Map<String, dynamic>>[],
      'price': {'amount': 500000, 'currency': 'IDR'},
      'status': 'active',
      'quantity_available': 3,
      'negotiation_enabled': negotiationEnabled,
      'seller': {
        'user': {'id': 'seller-1', 'username': 'seller'},
      },
    },
  });
}

ResourceProjection _auctionProjection({String id = 'auction-1'}) {
  return ResourceProjection.fromJson({
    ..._liveEnvelope('auction', id),
    'auction': const {
      'title': 'Lelang Kohaku',
      'media': <Map<String, dynamic>>[],
      'end_at': '2026-10-01T00:00:00Z',
      'lifecycle': 'active',
      'current_bid': 150000,
      'seller': {
        'user': {'id': 'seller-1', 'username': 'seller'},
      },
    },
  });
}

Comment _commerceComment(ResourceProjection projection) {
  return Comment(
    id: 'c1',
    authorId: 'u1',
    contentId: 'content-1',
    authorUsername: 'seller',
    type: 'commerce_reference',
    body: 'cek listing',
    createdAt: DateTime.utc(2026, 1, 1),
    reference: ShareReference.forSale(
      forSaleId: projection.resourceId,
      title: 'snapshot-title',
    ),
    resourceProjection: projection,
  );
}

Widget _wrap(CommentCard card) => ProviderScope(
  child: MaterialApp(
    home: Scaffold(
      // The production surface is a ListView — scrollable, unbounded height.
      body: SingleChildScrollView(child: card),
    ),
  ),
);

void main() {
  group('comment commerce reference identity', () {
    test('backend wire fixed_price_sale maps to canonical object type', () {
      final ref = ShareReference.fromJson(const {
        'targetType': 'for_sale',
        'targetId': 'sale-1',
        'preview': {
          'title': 'Kohaku 50cm',
          'imageUrl': 'https://example.com/sale.jpg',
          'isAvailable': true,
          'isSold': false,
          'isClosed': false,
          'isDeleted': false,
        },
      });

      expect(ref.targetType, ShareTargetType.forSale);
      expect(ref.objectType, 'for_sale');
    });

    test('rejected legacy target type fails closed (zero-legacy)', () {
      expect(
        () => ShareReference.fromJson(const {
          'targetType': 'forSale',
          'targetId': 'sale-1',
          'preview': {'title': 'x'},
        }),
        throwsA(isA<FormatException>()),
      );
    });
  });

  group('comment resource projection contract', () {
    testWidgets(
      'commerce comment renders the canonical projection envelope',
      (tester) async {
        await tester.pumpWidget(
          _wrap(
            CommentCard(
              comment: _commerceComment(_forSaleProjection()),
              userName: '@seller',
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byType(ContentResourceProjectionCard), findsOneWidget);
        expect(find.text('Kohaku 50cm'), findsOneWidget);

        // Negative proof: the comment surface must never fall back to the
        // legacy snapshot/detail-endpoint card.
        expect(find.byType(ObjectPreviewCard), findsNothing);
      },
    );

    testWidgets(
      'for-sale attachment tap dispatches with the canonical resource id',
      (tester) async {
        String? tapped;
        await tester.pumpWidget(
          _wrap(
            CommentCard(
              comment: _commerceComment(_forSaleProjection()),
              userName: '@seller',
              onFixedPriceSaleTap: (id) => tapped = id,
            ),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.byType(ContentResourceProjectionCard));
        await tester.pumpAndSettle();

        expect(tapped, 'sale-1');
      },
    );

    testWidgets(
      'auction attachment tap dispatches onAuctionTap (previously dead CTA)',
      (tester) async {
        String? tapped;
        await tester.pumpWidget(
          _wrap(
            CommentCard(
              comment: _commerceComment(_auctionProjection()),
              userName: '@seller',
              onAuctionTap: (id) => tapped = id,
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Lelang Kohaku'), findsOneWidget);

        await tester.tap(find.byType(ContentResourceProjectionCard));
        await tester.pumpAndSettle();

        expect(tapped, 'auction-1');
      },
    );

    testWidgets(
      'the generic product reference renders the Nego product attribute',
      (tester) async {
        await tester.pumpWidget(
          _wrap(
            CommentCard(
              comment: _commerceComment(
                _forSaleProjection(negotiationEnabled: true),
              ),
              userName: '@seller',
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('Nego'), findsOneWidget);

        await tester.pumpWidget(
          _wrap(
            CommentCard(
              comment: _commerceComment(
                _forSaleProjection(negotiationEnabled: false),
              ),
              userName: '@seller',
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('Nego'), findsNothing);
      },
    );

    testWidgets(
      'TOMBSTONE envelope disables attachment navigation (fail-closed)',
      (tester) async {
        String? tapped;
        await tester.pumpWidget(
          _wrap(
            CommentCard(
              comment: _commerceComment(
                _forSaleProjection(state: 'TOMBSTONE'),
              ),
              userName: '@seller',
              onFixedPriceSaleTap: (id) => tapped = id,
            ),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.byType(ContentResourceProjectionCard));
        await tester.pumpAndSettle();

        expect(tapped, isNull, reason: 'a tombstoned resource is never tappable');
        expect(find.byType(ObjectPreviewCard), findsNothing);
      },
    );
  });

  // The legacy chat path this file used to exercise — a per-row client-side
  // resolver (`objectPreviewProvider` keyed on a client-built `ObjectReference`)
  // — is deleted; see `shared/object/reference_attachment_live_fetch_purge_test.dart`.
}
