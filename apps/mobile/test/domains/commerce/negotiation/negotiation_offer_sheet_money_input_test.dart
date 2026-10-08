// End-to-end money-input proof on a live consumer: the negotiation offer
// sheet. Typing digits must display Indonesian thousands grouping, and the
// submitted business value must be the plain integer (no display punctuation,
// no double).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_offer_sheet.dart';

/// Pushes the sheet as a second route so the success path can pop safely.
Future<void> _openSheet(
  WidgetTester tester, {
  required Future<String?> Function(int price) onSubmit,
}) async {
  final key = GlobalKey<NavigatorState>();
  await tester.pumpWidget(
    MaterialApp(
      navigatorKey: key,
      home: const Scaffold(body: SizedBox.shrink()),
    ),
  );
  key.currentState!.push(
    MaterialPageRoute<void>(
      builder: (_) => Scaffold(
        body: NegotiationOfferSheet(
          productTitle: 'Koi Kohaku 50cm',
          onSubmit: onSubmit,
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('typing 1000000 shows 1.000.000 and submits 1000000', (
    tester,
  ) async {
    int? submitted;
    await _openSheet(
      tester,
      onSubmit: (price) async {
        submitted = price;
        return null; // accepted
      },
    );

    await tester.enterText(find.byType(TextFormField), '1000000');
    await tester.pump();

    final field = tester.widget<TextFormField>(find.byType(TextFormField));
    expect(
      field.controller!.text,
      '1.000.000',
      reason: 'the money mask groups while typing',
    );

    await tester.tap(find.text('Kirim Penawaran'));
    await tester.pumpAndSettle();

    expect(
      submitted,
      1000000,
      reason: 'the submitted value is the punctuation-free integer',
    );
  });

  testWidgets('a partial amount keeps its exact numeric value', (
    tester,
  ) async {
    int? submitted;
    await _openSheet(
      tester,
      onSubmit: (price) async {
        submitted = price;
        return 'rejected';
      },
    );

    await tester.enterText(find.byType(TextFormField), '50000');
    await tester.pump();
    await tester.tap(find.text('Kirim Penawaran'));
    await tester.pump();

    expect(submitted, 50000);
    // Failure keeps the sheet open with the typed (grouped) input intact.
    expect(find.text('50.000'), findsOneWidget);
    expect(find.text('rejected'), findsOneWidget);
  });
}
