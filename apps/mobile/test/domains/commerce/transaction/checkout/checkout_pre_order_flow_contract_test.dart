// Checkout pre-order flow contract (Phase 2) — NEGATIVE / RESIDUE PROOF.
//
// Proves the OLD checkout sequence is dead and cannot resurrect:
//   create order → get order-scoped methods → picker
// and that the technical wording / ready-state manual refresh are gone.
//
// The canonical flow is: preview → pre-order methods → select method → final
// total → create order (method bound) → payment (same method).
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _read(String path) => File(path).readAsStringSync();

const _logic =
    'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart';
const _impl =
    'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart';
const _summary =
    'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart';
const _actionBar =
    'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_action_bar.dart';

void main() {
  group('Checkout pre-order flow — canonical sequence', () {
    test('checkout loads PRE-ORDER methods and binds the method to the order',
        () {
      final logic = _read(_logic);
      expect(logic, contains('getPreOrderPaymentPricing('));
      expect(logic, contains('paymentMethodCode:'));
      // Order creation is the FIRST durable write and carries the selection.
      expect(logic, contains('notifier.createOrder(request)'));
    });

    test('the old order-scoped sequence is purged', () {
      final logic = _read(_logic);
      expect(
        logic,
        isNot(contains('getPaymentMethodOptions(')),
        reason: 'checkout must not fetch methods AFTER creating the order',
      );
      expect(
        logic,
        isNot(contains('PaymentMethodPickerSheet')),
        reason: 'checkout owns the selection; no post-order picker',
      );
    });

    test('server expires_at is the only expiry authority', () {
      final impl = _read(_impl);
      expect(impl, contains('expiresAt'));
      expect(
        impl,
        isNot(contains('_tokenValidityDuration')),
        reason: 'client created_at + fixed duration must not be an authority',
      );
      expect(impl, isNot(contains('_previewTokenCreatedAt')));
    });
  });

  group('Checkout pre-order flow — wording + ready refresh purged', () {
    test('technical implementation copy is gone', () {
      final summary = _read(_summary);
      expect(summary, isNot(contains('Akan dihitung oleh server')));
      expect(summary, isNot(contains('Harga lokal sementara')));
    });

    test('ready state has no manual refresh affordance', () {
      final summary = _read(_summary);
      expect(
        summary,
        isNot(contains('Harga Terkunci')),
        reason: 'the ready banner (with a manual Refresh) must be purged',
      );
      expect(summary, contains('return const SizedBox.shrink();'));
    });

    test('the CTA shows the backend final payable for the selected method', () {
      final actionBar = _read(_actionBar);
      expect(actionBar, contains('finalPayableAmount'));
      expect(actionBar, contains('Total Pembayaran'));
    });
  });
}
