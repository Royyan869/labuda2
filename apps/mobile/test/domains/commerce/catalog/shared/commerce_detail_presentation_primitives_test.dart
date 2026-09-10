import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';

Widget _wrap(Widget child) {
  return MaterialApp(
    theme: ThemeData.light(),
    home: Scaffold(body: child),
  );
}

void main() {
  test('CommerceViewerCapabilities guest is fully closed and round-trips', () {
    const guest = CommerceViewerCapabilities.guest();

    expect(guest.isGuest, isTrue);
    expect(guest.isOwner, isFalse);
    expect(guest.isBuyer, isFalse);
    expect(guest.canManage, isFalse);
    expect(guest.canEdit, isFalse);
    expect(guest.canPromote, isFalse);
    expect(guest.canChat, isFalse);
    expect(guest.canNegotiate, isFalse);
    expect(guest.canBuy, isFalse);
    expect(guest.canBid, isFalse);
    expect(guest.canBuyNow, isFalse);
    expect(CommerceViewerCapabilities.fromJson(guest.toJson()), guest);
  });

  testWidgets('CommerceDetailSectionCard renders child on card surface', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        const CommerceDetailSectionCard(
          child: Text('section-content'),
        ),
      ),
    );

    expect(find.text('section-content'), findsOneWidget);
    final container = tester.widget<Container>(
      find.ancestor(
        of: find.text('section-content'),
        matching: find.byType(Container),
      ),
    );
    final decoration = container.decoration! as BoxDecoration;
    expect(decoration.borderRadius, const BorderRadius.all(Radius.circular(16)));
    expect(tester.takeException(), isNull);
  });

  testWidgets('CommerceDetailLabelValue honors explicit layout modes', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        const Column(
          children: [
            CommerceDetailLabelValue(
              label: 'Breeder',
              value: 'Showa Farm',
              layout: CommerceDetailValueLayout.vertical,
            ),
            CommerceDetailLabelValue(
              label: 'Harga',
              value: 'Rp 1.500.000',
              layout: CommerceDetailValueLayout.horizontal,
            ),
          ],
        ),
      ),
    );

    expect(find.text('Breeder'), findsOneWidget);
    expect(find.text('Showa Farm'), findsOneWidget);
    expect(find.text('Harga'), findsOneWidget);
    expect(find.text('Rp 1.500.000'), findsOneWidget);

    // vertical mode stacks label above value (Column inside the Padding).
    final verticalPadding = tester.widget<Padding>(
      find
          .ancestor(of: find.text('Breeder'), matching: find.byType(Padding))
          .first,
    );
    expect(verticalPadding.child, isA<Column>());

    // horizontal mode lays label and value on one row.
    final horizontalPadding = tester.widget<Padding>(
      find
          .ancestor(of: find.text('Harga'), matching: find.byType(Padding))
          .first,
    );
    expect(horizontalPadding.child, isA<Row>());
    expect(tester.takeException(), isNull);
  });
}