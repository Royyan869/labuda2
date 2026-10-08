import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/shared.dart';

Widget _host(Widget child) => MaterialApp(home: Scaffold(body: child));

void main() {
  group('PaymentMethodTrigger', () {
    testWidgets('renders the canonical visual and backend display name', (
      tester,
    ) async {
      await tester.pumpWidget(
        _host(
          const PaymentMethodTrigger(
            selectedMethodCode: 'qris',
            selectedMethodDisplayName: 'QRIS',
            isLoading: false,
            hasMethods: true,
            onTap: _noop,
          ),
        ),
      );

      expect(find.text('QRIS'), findsOneWidget);
      expect(find.byType(PaymentMethodLogo), findsOneWidget);
      // qris has a canonical primary asset -> a local image is rendered.
      expect(find.byType(Image), findsOneWidget);
    });

    testWidgets('falls back to the generic visual for an unknown code', (
      tester,
    ) async {
      await tester.pumpWidget(
        _host(
          const PaymentMethodTrigger(
            selectedMethodCode: 'made_up',
            selectedMethodDisplayName: 'Made Up',
            isLoading: false,
            hasMethods: true,
            onTap: _noop,
          ),
        ),
      );

      expect(find.byIcon(Icons.payments_outlined), findsOneWidget);
      expect(find.byType(Image), findsNothing);
    });

    testWidgets('shows the choose-method call to action when none is selected', (
      tester,
    ) async {
      await tester.pumpWidget(
        _host(
          const PaymentMethodTrigger(
            selectedMethodCode: null,
            selectedMethodDisplayName: null,
            isLoading: false,
            hasMethods: true,
            onTap: _noop,
          ),
        ),
      );

      expect(find.text('Pilih metode pembayaran'), findsOneWidget);
      expect(find.byIcon(Icons.chevron_right), findsOneWidget);
    });

    testWidgets('loading state shows progress and never fires onTap', (
      tester,
    ) async {
      var tapped = false;
      await tester.pumpWidget(
        _host(
          PaymentMethodTrigger(
            selectedMethodCode: null,
            selectedMethodDisplayName: null,
            isLoading: true,
            hasMethods: false,
            onTap: () => tapped = true,
          ),
        ),
      );

      expect(find.text('Memuat metode pembayaran...'), findsOneWidget);
      expect(find.byIcon(Icons.chevron_right), findsNothing);
      await tester.tap(find.byType(PaymentMethodTrigger), warnIfMissed: false);
      expect(tapped, isFalse);
    });

    testWidgets('error state shows the message and fires onRetry', (
      tester,
    ) async {
      var retried = false;
      await tester.pumpWidget(
        _host(
          PaymentMethodTrigger(
            selectedMethodCode: null,
            selectedMethodDisplayName: null,
            isLoading: false,
            hasMethods: false,
            errorMessage: 'Gagal memuat metode pembayaran.',
            onRetry: () => retried = true,
          ),
        ),
      );

      expect(find.text('Gagal memuat metode pembayaran.'), findsOneWidget);
      expect(find.text('Coba lagi'), findsOneWidget);
      await tester.tap(find.text('Coba lagi'));
      expect(retried, isTrue);
    });

    testWidgets('empty state fires onRetry when the field is tapped', (
      tester,
    ) async {
      var retried = false;
      await tester.pumpWidget(
        _host(
          PaymentMethodTrigger(
            selectedMethodCode: null,
            selectedMethodDisplayName: null,
            isLoading: false,
            hasMethods: false,
            onRetry: () => retried = true,
          ),
        ),
      );

      expect(find.text('Tidak ada metode pembayaran tersedia'), findsOneWidget);
      await tester.tap(find.text('Tidak ada metode pembayaran tersedia'));
      expect(retried, isTrue);
    });

    testWidgets('tapping the field fires onTap', (tester) async {
      var tapped = false;
      await tester.pumpWidget(
        _host(
          PaymentMethodTrigger(
            selectedMethodCode: 'qris',
            selectedMethodDisplayName: 'QRIS',
            isLoading: false,
            hasMethods: true,
            onTap: () => tapped = true,
          ),
        ),
      );

      await tester.tap(find.text('QRIS'));
      expect(tapped, isTrue);
    });
  });

  group('PaymentMethodTrigger — presentation authority', () {
    test('consumes PaymentMethodVisuals and owns no local method mapping', () {
      final src = File(
        'lib/shared/payment/payment_method_trigger.dart',
      ).readAsStringSync();
      expect(
        src.contains('PaymentMethodVisuals.visual'),
        isTrue,
        reason: 'the trigger must resolve visuals through the one authority',
      );
      expect(
        src.contains('assets/icons/payment/'),
        isFalse,
        reason: 'only PaymentMethodVisuals may map method_code -> asset',
      );
    });
  });
}

void _noop() {}
