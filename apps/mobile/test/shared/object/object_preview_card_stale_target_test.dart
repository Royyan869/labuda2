import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';

void main() {
  testWidgets(
    'withdrawn live preview disables CTA and shows unavailable state',
    (tester) async {
      var tapped = false;

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: ObjectPreviewCard(
              reference: ShareReference.fixedPriceSale(
                fixedPriceSaleId: 'sale-1',
                title: 'Kohaku 50cm',
                imageUrl: 'https://example.com/sale.jpg',
                isAvailable: true,
                isSold: false,
                isClosed: false,
                isDeleted: false,
              ),
              preResolved: const ObjectPreview(
                id: 'sale-1',
                type: 'fixed_price_sale',
                title: 'Kohaku 50cm',
                imageUrl: 'https://example.com/sale.jpg',
                price: 500000,
                status: 'withdrawn',
                isAvailable: false,
                isSold: false,
                isClosed: false,
                isDeleted: false,
              ),
              onTap: () {
                tapped = true;
              },
            ),
          ),
        ),
      );

      expect(find.text('Ditarik'), findsOneWidget);
      expect(find.text('Kohaku 50cm'), findsOneWidget);

      await tester.tap(find.byType(InkWell));
      await tester.pump();

      expect(tapped, isFalse);
    },
  );
}
