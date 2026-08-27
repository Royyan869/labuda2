import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/checkout/presentation/screens/payment_result_screen_impl.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_result_notifier.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_result_state.dart';
import 'package:labuda/core/common/types/payment_types.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';

/// Fake notifier that renders a fixed [PaymentResultState] and records which
/// handler methods the screen calls, without touching real repositories or
/// network. This is what lets us assert the F2/F4 behavioral split (status
/// check vs. reopen payment vs. resume) at the widget level instead of only
/// via source-text inspection.
class _FakePaymentResultNotifier extends PaymentResultNotifier {
  _FakePaymentResultNotifier(this._initial);

  final PaymentResultState _initial;

  int retryCalls = 0;
  int recheckOnResumeCalls = 0;

  @override
  PaymentResultState build() => _initial;

  @override
  Future<void> startChecking(String orderId) async {
    // No-op: widget tests drive state via the fixed initial state only.
  }

  @override
  Future<void> retry(String orderId) async {
    retryCalls++;
  }

  @override
  Future<void> recheckOnResume(String orderId) async {
    recheckOnResumeCalls++;
  }

  @override
  void stopChecking() {
    // No-op.
  }
}

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();
}

Payment _payment({
  required PaymentStatus status,
  String? paymentUrl,
  DateTime? expiredAt,
}) {
  return Payment(
    id: 'pay-1',
    paymentNumber: 'PAY-1',
    userId: 'buyer-1',
    grossAmount: 110000,
    coinDiscount: 0,
    coinDiscountAmount: 0,
    netAmount: 110000,
    status: status,
    referenceType: 'order',
    referenceId: 'order-1',
    createdAt: DateTime.utc(2026, 6, 1),
    expiredAt: expiredAt ?? DateTime.utc(2026, 8, 2),
    paymentUrl: paymentUrl,
  );
}

Future<_FakePaymentResultNotifier> _pumpScreen(
  WidgetTester tester,
  PaymentResultState state, {
  String? returnToChat,
}) async {
  final notifier = _FakePaymentResultNotifier(state);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        paymentResultProvider.overrideWith(() => notifier),
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ],
      child: MaterialApp(
        home: PaymentResultScreen(
          orderId: 'order-1',
          orderNumber: 'ORD-1',
          returnToChat: returnToChat,
        ),
      ),
    ),
  );
  await tester.pump();
  return notifier;
}

void main() {
  group('PaymentResultScreen - checking state (F2/F4)', () {
    testWidgets(
      'Lanjutkan Pembayaran is hidden when no reusable payment URL exists',
      (tester) async {
        await _pumpScreen(tester, PaymentResultState.checking(pollAttempts: 0));

        expect(find.text('Coba Lagi'), findsOneWidget);
        expect(find.text('Lanjutkan Pembayaran'), findsNothing);
      },
    );

    testWidgets(
      'Lanjutkan Pembayaran appears when a reusable, non-terminal payment URL exists',
      (tester) async {
        await _pumpScreen(
          tester,
          PaymentResultState.checking(
            pollAttempts: 5,
            payment: _payment(
              status: PaymentStatus.pending,
              paymentUrl: 'https://pay.example.com/snap/1',
            ),
          ),
        );

        expect(find.text('Coba Lagi'), findsOneWidget);
        expect(find.text('Lanjutkan Pembayaran'), findsOneWidget);
      },
    );

    testWidgets(
      'Coba Lagi is disabled while isChecking is true (overlap guard preserved)',
      (tester) async {
        final notifier = await _pumpScreen(
          tester,
          PaymentResultState.checking(pollAttempts: 0),
        );

        await tester.tap(find.text('Coba Lagi'));
        await tester.pump();
        expect(notifier.retryCalls, 0);
      },
    );
  });

  group('PaymentResultScreen - timeout state (F2)', () {
    testWidgets(
      'Cek Status Lagi calls retry only, no continue button when no URL',
      (tester) async {
        final notifier = await _pumpScreen(
          tester,
          PaymentResultState.timeout(pollAttempts: 20),
        );

        expect(find.text('Cek Status Lagi'), findsOneWidget);
        expect(find.text('Lanjutkan Pembayaran'), findsNothing);

        await tester.tap(find.text('Cek Status Lagi'));
        await tester.pump();

        expect(notifier.retryCalls, 1);
      },
    );

    testWidgets(
      'Lanjutkan Pembayaran appears alongside Cek Status Lagi when reusable URL exists',
      (tester) async {
        final notifier = await _pumpScreen(
          tester,
          PaymentResultState.timeout(
            pollAttempts: 20,
            payment: _payment(
              status: PaymentStatus.processing,
              paymentUrl: 'https://pay.example.com/snap/1',
            ),
          ),
        );

        expect(find.text('Cek Status Lagi'), findsOneWidget);
        expect(find.text('Lanjutkan Pembayaran'), findsOneWidget);

        // Tapping the status-check button must not also count as a
        // continue-payment action - retry is the only thing it does.
        await tester.tap(find.text('Cek Status Lagi'));
        await tester.pump();
        expect(notifier.retryCalls, 1);
      },
    );

    testWidgets('Lanjutkan Pembayaran does not trigger retry when tapped', (
      tester,
    ) async {
      final notifier = await _pumpScreen(
        tester,
        PaymentResultState.timeout(
          pollAttempts: 20,
          payment: _payment(
            status: PaymentStatus.pending,
            paymentUrl: 'https://pay.example.com/snap/1',
          ),
        ),
      );

      await tester.tap(find.text('Lanjutkan Pembayaran'));
      await tester.pump();

      // Reopening the URL is a separate action from the status check - it
      // must not also invoke retry().
      expect(notifier.retryCalls, 0);
    });

    testWidgets(
      'Lanjutkan Pembayaran is absent when the payment resource is already settled',
      (tester) async {
        await _pumpScreen(
          tester,
          PaymentResultState.timeout(
            pollAttempts: 20,
            payment: _payment(
              status: PaymentStatus.paid,
              paymentUrl: 'https://pay.example.com/snap/1',
            ),
          ),
        );

        expect(find.text('Lanjutkan Pembayaran'), findsNothing);
      },
    );
  });

  group('PaymentResultScreen - network error state (F2)', () {
    testWidgets('Coba Lagi calls retry, no invented support/chat route', (
      tester,
    ) async {
      final notifier = await _pumpScreen(
        tester,
        PaymentResultState.networkError(
          errorMessage: 'Terjadi kesalahan koneksi.',
          pollAttempts: 3,
        ),
      );

      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.text('Lanjutkan Pembayaran'), findsNothing);
      expect(find.textContaining('Chat'), findsNothing);
      expect(find.textContaining('Support'), findsNothing);

      await tester.tap(find.text('Coba Lagi'));
      await tester.pump();
      expect(notifier.retryCalls, 1);
    });
  });

  group('PaymentResultScreen - success state', () {
    testWidgets('Kembali ke Chat appears only when returnToChat is provided', (
      tester,
    ) async {
      await _pumpScreen(
        tester,
        const PaymentResultState(status: PaymentResultScreenStatus.success),
        returnToChat: 'chat-1',
      );

      expect(find.text('Lihat Pesanan'), findsOneWidget);
      expect(find.text('Kembali ke Chat'), findsOneWidget);
    });

    testWidgets('Kembali ke Chat is absent without a chat context', (
      tester,
    ) async {
      await _pumpScreen(
        tester,
        const PaymentResultState(status: PaymentResultScreenStatus.success),
      );

      expect(find.text('Lihat Pesanan'), findsOneWidget);
      expect(find.text('Kembali ke Chat'), findsNothing);
    });
  });

  group('PaymentResultScreen - failed state', () {
    testWidgets('Support button is disabled without an authenticated user', (
      tester,
    ) async {
      await _pumpScreen(
        tester,
        const PaymentResultState(
          status: PaymentResultScreenStatus.failed,
          errorMessage: 'Pembayaran gagal.',
        ),
      );

      final supportButtonFinder = find.widgetWithText(
        ElevatedButton,
        'Support',
      );
      expect(supportButtonFinder, findsOneWidget);
      final button = tester.widget<ElevatedButton>(supportButtonFinder);
      expect(button.onPressed, isNull);
    });
  });
}
