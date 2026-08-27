import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart'
    as order_domain;
import 'package:labuda/domains/commerce/transaction/order/presentation/widgets/dynamic_action_buttons.dart';

/// Locks the pending-buyer "pay" CTA label rendering for all four payment
/// states the backend can emit (Phase 2B-1: action.pay_now /
/// action.payment_continue / action.payment_check_status / action.pay_again),
/// and confirms the action still routes through the generic action callback
/// regardless of which label is shown - the label is presentation-only, the
/// routing key is always the action `type` ("pay"), never the `label_key`.
order_domain.Action _payAction(String labelKey) {
  return order_domain.Action(
    type: 'pay',
    labelKey: labelKey,
    enabled: true,
    endpoint: '/api/v1/payments',
    method: 'POST',
    requiresIdempotency: true,
    financial: false,
  );
}

void main() {
  group('DynamicActionButtons - pay CTA label rendering (Phase 2B-1)', () {
    testWidgets('action.pay_now renders "Bayar Sekarang"', (tester) async {
      final decision = order_domain.DecisionContract(
        state: 'pending',
        primaryAction: _payAction('action.pay_now'),
      );

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: DynamicActionButtons(
              decision: decision,
              callbacks: ActionCallbacks(onAction: (_) {}),
            ),
          ),
        ),
      );

      expect(find.text('Bayar Sekarang'), findsOneWidget);
    });

    testWidgets('action.payment_continue renders "Lanjutkan Pembayaran"', (
      tester,
    ) async {
      final decision = order_domain.DecisionContract(
        state: 'pending',
        primaryAction: _payAction('action.payment_continue'),
      );

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: DynamicActionButtons(
              decision: decision,
              callbacks: ActionCallbacks(onAction: (_) {}),
            ),
          ),
        ),
      );

      expect(find.text('Lanjutkan Pembayaran'), findsOneWidget);
    });

    testWidgets('action.payment_check_status renders "Cek Status Pembayaran"', (
      tester,
    ) async {
      final decision = order_domain.DecisionContract(
        state: 'pending',
        primaryAction: _payAction('action.payment_check_status'),
      );

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: DynamicActionButtons(
              decision: decision,
              callbacks: ActionCallbacks(onAction: (_) {}),
            ),
          ),
        ),
      );

      expect(find.text('Cek Status Pembayaran'), findsOneWidget);
    });

    testWidgets('action.pay_again renders "Bayar Ulang"', (tester) async {
      final decision = order_domain.DecisionContract(
        state: 'pending',
        primaryAction: _payAction('action.pay_again'),
      );

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: DynamicActionButtons(
              decision: decision,
              callbacks: ActionCallbacks(onAction: (_) {}),
            ),
          ),
        ),
      );

      expect(find.text('Bayar Ulang'), findsOneWidget);
    });

    testWidgets(
      'tapping the pay action routes to the callback regardless of label_key',
      (tester) async {
        for (final labelKey in [
          'action.pay_now',
          'action.payment_continue',
          'action.payment_check_status',
          'action.pay_again',
        ]) {
          order_domain.Action? tappedAction;
          final decision = order_domain.DecisionContract(
            state: 'pending',
            primaryAction: _payAction(labelKey),
          );

          await tester.pumpWidget(
            MaterialApp(
              home: Scaffold(
                body: DynamicActionButtons(
                  decision: decision,
                  callbacks: ActionCallbacks(
                    onAction: (action) => tappedAction = action,
                  ),
                ),
              ),
            ),
          );
          await tester.pump();

          await tester.tap(find.byType(ElevatedButton));
          await tester.pump();

          expect(
            tappedAction,
            isNotNull,
            reason: 'label_key=$labelKey must still route via type "pay"',
          );
          expect(tappedAction!.type, 'pay');
        }
      },
    );

    testWidgets(
      'tapping the primary pay button invokes onAction with type "pay"',
      (tester) async {
        order_domain.Action? tappedAction;
        final decision = order_domain.DecisionContract(
          state: 'pending',
          primaryAction: _payAction('action.payment_continue'),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: DynamicActionButtons(
                decision: decision,
                callbacks: ActionCallbacks(
                  onAction: (action) => tappedAction = action,
                ),
              ),
            ),
          ),
        );

        await tester.tap(find.text('Lanjutkan Pembayaran'));
        await tester.pump();

        expect(tappedAction, isNotNull);
        expect(tappedAction!.type, 'pay');
        expect(tappedAction!.labelKey, 'action.payment_continue');
      },
    );
  });

  group('DynamicActionButtons - terminal state renders no pay action', () {
    testWidgets('no primary/secondary actions renders only support button', (
      tester,
    ) async {
      const decision = order_domain.DecisionContract(state: 'completed');

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: DynamicActionButtons(
              decision: decision,
              callbacks: ActionCallbacks(onAction: (_) {}),
            ),
          ),
        ),
      );

      expect(find.text('Bayar Sekarang'), findsNothing);
      expect(find.text('Lanjutkan Pembayaran'), findsNothing);
      expect(find.text('Cek Status Pembayaran'), findsNothing);
      expect(find.text('Bayar Ulang'), findsNothing);
      expect(find.text('Butuh Bantuan?'), findsOneWidget);
    });

    testWidgets('disabled pay action is not rendered as the primary CTA', (
      tester,
    ) async {
      final disabledPay = order_domain.Action(
        type: 'pay',
        labelKey: 'action.pay_now',
        enabled: false,
        endpoint: '/api/v1/payments',
        method: 'POST',
        requiresIdempotency: true,
        financial: false,
      );
      final decision = order_domain.DecisionContract(
        state: 'expired',
        primaryAction: disabledPay,
      );

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: DynamicActionButtons(
              decision: decision,
              callbacks: ActionCallbacks(onAction: (_) {}),
            ),
          ),
        ),
      );

      expect(find.text('Bayar Sekarang'), findsNothing);
      expect(find.text('Butuh Bantuan?'), findsOneWidget);
    });
  });
}
