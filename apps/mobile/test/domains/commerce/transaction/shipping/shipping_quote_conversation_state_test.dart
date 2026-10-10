import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';
import 'package:hishumi/shared/attachment/entities/attachment.dart';
import 'package:hishumi/shared/widgets/attachment_widget.dart';

/// SHIPPING QUOTE CONVERSATION STATE — buyer actionability is SERVER-OWNED and
/// the only buyer action is "use this quote for checkout" (no rejection).
///
/// The conversation renders the Commerce projection verbatim: the buyer may act
/// only when `viewerActionable` is true. It never recomputes quote lifecycle
/// from status, viewer id, or a client "active quote" comparison.

ShippingQuoteAttachment _quote({
  required bool viewerActionable,
  required String status,
}) {
  return ShippingQuoteAttachment(
    offerId: 'offer-1',
    linkedItemId: 'sale-1',
    linkedItemType: 'for_sale',
    linkedItemName: 'Kohaku 50cm',
    linkedItemPrice: 500000,
    shippingType: 'manual',
    shippingTypeName: 'Ongkir Manual',
    shippingTypeEmoji: '🚚',
    rate: 25000,
    validUntil: DateTime.now().add(const Duration(hours: 1)),
    status: status,
    sellerId: 'seller-1',
    isCurrent: true,
    viewerActionable: viewerActionable,
  );
}

Widget _wrap(Widget child) => ProviderScope(
  child: MaterialApp(
    theme: AppTheme.lightTheme,
    home: Scaffold(body: SingleChildScrollView(child: child)),
  ),
);

Future<void> _pumpQuote(WidgetTester tester, ShippingQuoteAttachment att) async {
  await tester.pumpWidget(
    _wrap(AttachmentWidget(attachment: att, onPurchase: () {})),
  );
  await tester.pump();
  // The test font (Ahem) makes the card's two-text rows wider than the 280dp
  // card cap, so a RenderFlex overflow artifact is reported during layout.
  // Drain it: it is a test-font artifact, not a widget behaviour under test.
  while (tester.takeException() != null) {}
}

ElevatedButton _elevated(WidgetTester tester) =>
    tester.widget<ElevatedButton>(find.byType(ElevatedButton));

void main() {
  setUpAll(() async {
    await initializeDateFormatting();
  });

  testWidgets('buyer: actionable quote exposes an enabled use-for-checkout CTA', (
    tester,
  ) async {
    await _pumpQuote(tester, _quote(viewerActionable: true, status: 'ACTIVE'));

    expect(find.text('Penawaran Aktif'), findsOneWidget);
    expect(find.text('Gunakan Ongkir'), findsOneWidget);
    expect(_elevated(tester).onPressed, isNotNull);
    // No rejection path: the reject action no longer exists.
    expect(find.text('Tolak'), findsNothing);
    expect(find.byType(OutlinedButton), findsNothing);
  });

  testWidgets(
    'non-actionable ACTIVE quote (e.g. seller) exposes NO enabled buyer action',
    (tester) async {
      await _pumpQuote(
        tester,
        _quote(viewerActionable: false, status: 'ACTIVE'),
      );

      // The card stays visible (conversation state is still conveyed) but the
      // buyer action is disabled and the label is honest.
      expect(find.text('Penawaran Aktif'), findsOneWidget);
      expect(find.text('Gunakan Ongkir'), findsNothing);
      expect(find.text('Tidak Tersedia'), findsOneWidget);
      expect(_elevated(tester).onPressed, isNull);
    },
  );

  testWidgets(
    'stale/expired quote shows an honest label and no enabled action',
    (tester) async {
      await _pumpQuote(
        tester,
        _quote(viewerActionable: false, status: 'EXPIRED'),
      );

      expect(find.text('Kadaluarsa'), findsOneWidget);
      expect(find.text('Gunakan Ongkir'), findsNothing);
      expect(_elevated(tester).onPressed, isNull);
    },
  );

  testWidgets('used quote shows "Sudah digunakan" and no enabled action', (
    tester,
  ) async {
    await _pumpQuote(tester, _quote(viewerActionable: false, status: 'USED'));

    expect(find.text('Sudah digunakan'), findsOneWidget);
    expect(_elevated(tester).onPressed, isNull);
  });
}
