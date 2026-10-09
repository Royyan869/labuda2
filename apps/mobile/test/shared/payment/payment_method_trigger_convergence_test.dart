import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// The payment-method surfaces, all converged onto the one canonical
/// `PaymentMethodTrigger`.
const _consumers = <String>[
  'lib/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart',
  'lib/domains/user/preference/seller/presentation/screens/seller_renewal_screen.dart',
  'lib/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_create_screen.dart',
  'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
];

/// The local trigger renderers purged by this convergence.
const _obsoleteRenderers = <String>[
  '_buildSubscriptionMethodSelector(',
  '_buildMethodSelector(',
  '_methodSelector(',
];

void main() {
  group('payment-method trigger convergence (residue proof)', () {
    test('every consumer uses the one canonical trigger', () {
      for (final path in _consumers) {
        final src = File(path).readAsStringSync();
        expect(
          src.contains('PaymentMethodTrigger'),
          isTrue,
          reason: '$path must render through the canonical trigger',
        );
        expect(
          src.contains('PaymentMethodPickerSheet.show'),
          isTrue,
          reason:
              '$path must keep the canonical picker as its selection surface',
        );
        expect(
          src.contains('selectedMethodCode:'),
          isTrue,
          reason: '$path must forward the selected method code to the picker',
        );
      }
    });

    test('the obsolete local trigger renderers are purged', () {
      for (final path in _consumers) {
        final src = File(path).readAsStringSync();
        for (final banned in _obsoleteRenderers) {
          expect(
            src.contains(banned),
            isFalse,
            reason: '$path must not keep the local renderer "$banned"',
          );
        }
      }
    });

    test('no consumer owns a payment-method visual/asset mapping', () {
      for (final path in _consumers) {
        final src = File(path).readAsStringSync();
        expect(
          src.contains('assets/icons/payment/'),
          isFalse,
          reason: '$path must not map method_code -> asset',
        );
      }
    });

    test('the canonical authorities survive', () {
      for (final path in <String>[
        'lib/shared/payment/payment_method_trigger.dart',
        'lib/shared/payment/payment_method_visuals.dart',
        'lib/shared/payment/payment_method_logo.dart',
        'lib/domains/finance/transaction/payment/presentation/widgets/payment_method_picker_sheet.dart',
      ]) {
        expect(
          File(path).existsSync(),
          isTrue,
          reason: '$path must remain the canonical component',
        );
      }
    });
  });
}
