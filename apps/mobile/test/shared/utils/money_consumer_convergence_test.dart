// MONEY INPUT CONVERGENCE — the consumer lock.
//
// Owner-locked truth:
//   - one canonical money-input formatter (`MoneyInputFormatter`), which
//     delegates grouping to `formatGroupedAmount`;
//   - every monetary data-entry field consumes it;
//   - non-monetary numeric fields (IDs, account numbers, OTP, stock, size,
//     duration, usage counts, percentages, postal codes) stay out of it.
//
// This is a source-contract lock (same style as the consumer-convergence
// contracts around the canonical validators): the widget-level behavior is
// proven in `money_input_formatter_test.dart`, and this file makes a future
// regression — a screen re-growing its own digit filter or its own masking —
// fail before it ships.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _read(String path) => File(path).readAsStringSync();

/// Monetary data-entry consumers whose numeric input surface is money only.
const _moneyConsumers = <String>[
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_action_modal.dart',
  'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
  'lib/domains/commerce/catalog/auction/presentation/screens/seller_auction_relist_screen.dart',
  'lib/domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart',
  'lib/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_offer_sheet.dart',
  'lib/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_create_screen.dart',
  'lib/domains/commerce/transaction/shipping/presentation/widgets/shipping_option_setup_screen.dart',
  'lib/domains/commerce/transaction/shipping/presentation/widgets/shipping_quote_form_sheet.dart',
  'lib/domains/user/preference/seller/presentation/widgets/withdraw_dialog.dart',
];

/// Consumers carrying BOTH a monetary field and a non-monetary numeric
/// sibling (discount percentage, usage count): these must use the canonical
/// money formatter for the money field while the sibling keeps its digit
/// filter, so the sibling check is only that the money half is canonical.
const _mixedMoneyConsumers = <String>[
  'lib/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/discount_type_section.dart',
  'lib/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/limits_section.dart',
];

/// Non-monetary numeric inputs that must keep their primitive digit filter.
const _nonMoneyDigitFields = <String>[
  'lib/domains/user/profile/presentation/widgets/add_edit_bank_account_dialog.dart',
  'lib/domains/user/profile/presentation/widgets/phone_verification/otp_input_field.dart',
];

void main() {
  group('one money-input formatter', () {
    test('exactly one TextInputFormatter subclass exists in lib', () {
      final offenders = <String>[];
      for (final entity in Directory('lib').listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        if (entity.readAsStringSync().contains('extends TextInputFormatter')) {
          offenders.add(entity.path.replaceAll(r'\', '/'));
        }
      }
      expect(
        offenders,
        ['lib/shared/utils/money_input_formatter.dart'],
        reason:
            'a second money/text formatter is a second authority: $offenders',
      );
    });

    test('the canonical formatter delegates grouping to the display engine', () {
      final source = _read('lib/shared/utils/money_input_formatter.dart');
      expect(source.contains('formatGroupedAmount('), isTrue);
      // The hand-rolled grouping regex is banned lib-wide by
      // resource_projection_price_policy_test; this is the local floor.
      expect(source.contains(r'(?=(\d{3})+(?!\d))'), isFalse);
    });
  });

  group('every monetary field consumes the canonical formatter', () {
    for (final path in _moneyConsumers) {
      test(path, () {
        final source = _read(path);
        expect(
          source.contains('MoneyInputFormatter'),
          isTrue,
          reason: '$path must consume the canonical money-input formatter',
        );
        expect(
          source.contains('FilteringTextInputFormatter.digitsOnly'),
          isFalse,
          reason:
              '$path is monetary — a raw digit filter drops the Rupiah grouping',
        );
        expect(
          source.contains('parseAmount('),
          isTrue,
          reason:
              '$path must parse through MoneyInputFormatter.parseAmount so '
              'the business value stays punctuation-free',
        );
      });
    }

    for (final path in _mixedMoneyConsumers) {
      test('$path (mixed money + count/percent sibling)', () {
        final source = _read(path);
        expect(
          source.contains('MoneyInputFormatter'),
          isTrue,
          reason: '$path must consume the canonical money-input formatter',
        );
        expect(
          source.contains('parseAmount('),
          isTrue,
          reason: '$path must parse its monetary half canonically',
        );
      });
    }
  });

  group('non-monetary numeric fields stay out', () {
    for (final path in _nonMoneyDigitFields) {
      test('$path keeps its primitive digit filter', () {
        final source = _read(path);
        expect(source.contains('FilteringTextInputFormatter.digitsOnly'), isTrue);
        expect(
          source.contains('MoneyInputFormatter'),
          isFalse,
          reason: '$path is not monetary — grouping would corrupt the value',
        );
      });
    }
  });

  group('money display grouping is not re-owned by consumers', () {
    // Money fields that prefill must seed through the canonical display form
    // (`MoneyInputFormatter.display`), so the prefilled text reads exactly
    // like what the mask produces. Files with empty money controllers are not
    // listed here.
    const prefilling = <String>[
      'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_action_modal.dart',
      'lib/domains/commerce/catalog/auction/presentation/screens/seller_auction_relist_screen.dart',
      'lib/domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart',
      'lib/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_create_screen.dart',
      'lib/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/discount_type_section.dart',
      'lib/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/limits_section.dart',
      'lib/domains/commerce/transaction/shipping/presentation/widgets/shipping_option_setup_screen.dart',
      'lib/domains/user/preference/seller/presentation/widgets/withdraw_dialog.dart',
    ];
    for (final path in prefilling) {
      test('$path seeds in the canonical grouped display form', () {
        expect(
          _read(path).contains('MoneyInputFormatter.display('),
          isTrue,
          reason:
              '$path prefills a money field — seed it grouped via '
              'MoneyInputFormatter.display',
        );
      });
    }
  });
}
