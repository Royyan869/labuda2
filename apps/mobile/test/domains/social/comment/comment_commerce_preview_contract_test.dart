import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/object/object_reference.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';

void main() {
  group('comment commerce preview contract', () {
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
          'targetType': 'listing',
          'targetId': 'sale-1',
          'preview': {'title': 'x'},
        }),
        throwsA(isA<FormatException>()),
      );
    });

    testWidgets('ObjectPreviewCard dispatches the resolver with canonical '
        'fixed_price_sale type and renders live preview', (tester) async {
      ObjectReference? seen;

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            objectPreviewProvider.overrideWith((ref, reference) async {
              seen = reference;
              return ObjectPreview(
                id: 'sale-1',
                type: reference.type,
                title: 'Kohaku 50cm',
                price: 500000,
                status: 'active',
              );
            }),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: ObjectPreviewCard(
                reference: ShareReference.forSale(
                  forSaleId: 'sale-1',
                  title: 'snapshot-title',
                ),
              ),
            ),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(seen, isNotNull);
      expect(seen!.type, 'for_sale');
      expect(seen!.id, 'sale-1');
      expect(find.text('Kohaku 50cm'), findsOneWidget);
      expect(find.textContaining('500'), findsOneWidget);
    });

    test(
      'objectPreviewProvider fails closed for an unsupported type',
      () async {
        final container = ProviderContainer();
        addTearDown(container.dispose);

        final preview = await container.read(
          objectPreviewProvider(
            const ObjectReference(type: 'bogus', id: 'x'),
          ).future,
        );

        expect(preview, isNull);
      },
    );
  });
}
