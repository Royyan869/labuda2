/// REFERENCE ATTACHMENT SNAPSHOT SHELL CONTRACT
///
/// Pins the shape of the fallback card a communication row renders when the
/// server has NOT resolved a canonical `resource_projection` for it:
/// - it renders the transport snapshot only (type badge, title, thumbnail);
/// - it renders NO money and NO availability claim — Commerce truth belongs to
///   the projection envelope, never to a cached snapshot;
/// - navigation is identity-only and stays available even when the snapshot's
///   flags say the target is gone (the target surface answers with truth and
///   stays fail-closed on its own).
///
/// Before the purge this card resolved live data per item through
/// `objectPreviewProvider`, i.e. one detail-endpoint call per shared message.
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';

Widget _wrap(ShareReference reference, {VoidCallback? onTap}) {
  return MaterialApp(
    home: Scaffold(
      body: ObjectPreviewCard(reference: reference, onTap: onTap),
    ),
  );
}

/// Every `Text` string currently in the tree.
List<String> _renderedTexts(WidgetTester tester) {
  return tester
      .widgetList<Text>(find.byType(Text))
      .map((text) => text.data ?? '')
      .toList();
}

ShareReference _forSaleReference({
  String title = 'Kohaku 50cm',
  bool isAvailable = true,
  bool isSold = false,
  bool isDeleted = false,
}) {
  return ShareReference.forSale(
    forSaleId: 'sale-1',
    title: title,
    imageUrl: 'https://example.com/sale.jpg',
    isAvailable: isAvailable,
    isSold: isSold,
    isDeleted: isDeleted,
  );
}

void main() {
  testWidgets('renders the transport identity: type badge, title, thumbnail', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(_forSaleReference()));

    expect(find.text('Produk Dijual'), findsOneWidget);
    expect(find.text('Kohaku 50cm'), findsOneWidget);
    expect(find.byType(Image), findsOneWidget);
    expect(find.byIcon(Icons.chevron_right), findsOneWidget);
  });

  testWidgets(
    'a snapshot claiming the target is gone does not block navigation',
    (tester) async {
      var tapped = false;

      await tester.pumpWidget(
        _wrap(
          _forSaleReference(isAvailable: false, isDeleted: true),
          onTap: () => tapped = true,
        ),
      );

      await tester.tap(find.byType(InkWell));
      await tester.pump();

      expect(
        tapped,
        isTrue,
        reason:
            'navigation is identity-only: the target surface decides truth, '
            'so a stale snapshot flag never becomes a client-side verdict',
      );
    },
  );

  testWidgets('renders no money and no availability claim from the snapshot', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        ShareReference.forSale(
          forSaleId: 'sale-2',
          title: 'Sanke 45cm',
          isAvailable: false,
          isSold: true,
        ),
      ),
    );

    final texts = _renderedTexts(tester);
    expect(texts, contains('Sanke 45cm'));
    expect(
      texts.where((text) => text.contains('Rp')),
      isEmpty,
      reason: 'money comes only from the canonical projection envelope',
    );
    for (final claim in const ['Terjual', 'Tersedia', 'Ditarik', 'Dihapus']) {
      expect(
        texts,
        isNot(contains(claim)),
        reason: 'availability claims belong to the projection envelope',
      );
    }
  });
}
