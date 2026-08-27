/// Withdraw dialog display test
///
/// PASS_18H: locks the owner-mandated withdrawal money model in the UI.
/// Requesting Rp100,000 with a Rp5,000 fee must display:
///   "Jumlah withdrawal"        = Rp100,000 (requested amount)
///   "Biaya penarikan"          = Rp5,000   (flat fee)
///   "Jumlah diterima"          = Rp95,000  (net payout = requested - fee)
///   "Total dipotong dari saldo" = Rp100,000 (requested amount only, never +fee)
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/identity/verification/presentation/providers/seller_verification_v2_provider.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/withdraw_dialog.dart';

class _VerifiedSellerNotifier extends SellerVerificationV2Notifier {
  @override
  SellerVerificationV2State build() =>
      const SellerVerificationV2State(isVerified: true);
}

Future<void> _pumpDialog(
  WidgetTester tester, {
  required double availableBalance,
  required double withdrawalFeeAmount,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        sellerVerificationV2NotifierProvider.overrideWith(
          _VerifiedSellerNotifier.new,
        ),
      ],
      child: MaterialApp(
        home: Scaffold(
          body: Builder(
            builder: (context) => Center(
              child: ElevatedButton(
                onPressed: () => showWithdrawDialog(
                  context,
                  availableBalance,
                  withdrawalFeeAmount: withdrawalFeeAmount,
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
}

void main() {
  group('WithdrawDialog - PASS_18H money model display', () {
    testWidgets(
      'requesting Rp100,000 with Rp5,000 fee shows correct breakdown',
      (tester) async {
        await _pumpDialog(
          tester,
          availableBalance: 100000,
          withdrawalFeeAmount: 5000,
        );

        // Amount field is prefilled with the full available balance
        // (the requested amount), not balance-minus-fee.
        expect(find.text('Jumlah withdrawal'), findsOneWidget);
        expect(find.text('Biaya penarikan'), findsOneWidget);
        expect(find.text('Jumlah diterima'), findsOneWidget);
        expect(find.text('Total dipotong dari saldo'), findsOneWidget);

        expect(
          find.text('Rp 100.000'),
          findsWidgets,
        ); // requested + total deducted
        expect(
          find.text('Rp 5.000'),
          findsWidgets,
        ); // fee (breakdown + footnote)
        expect(find.text('Rp 95.000'), findsOneWidget); // net received
      },
    );

    testWidgets('zero fee omits fee and net-received rows', (tester) async {
      await _pumpDialog(
        tester,
        availableBalance: 50000,
        withdrawalFeeAmount: 0,
      );

      expect(find.text('Jumlah withdrawal'), findsOneWidget);
      expect(find.text('Biaya penarikan'), findsNothing);
      expect(find.text('Jumlah diterima'), findsNothing);
      expect(find.text('Total dipotong dari saldo'), findsOneWidget);
    });
  });
}
