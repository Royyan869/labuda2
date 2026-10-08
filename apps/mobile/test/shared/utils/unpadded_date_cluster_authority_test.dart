// UNPADDED d/M/yyyy CLUSTER — ABSOLUTE SHORT-DATE AUTHORITY GATE.
//
// Owner doctrine (audit-then-converge):
//   Absolute short-date UI formatting → AppFormatters (formatShortDate /
//   formatDate). Visible change: d/M/yyyy → dd/MM/yyyy (padding only) or
//   dd MMM yyyy (readable absolute date) where the consumer is a validity /
//   readable label.
//
// Closed family (must stay unpadded by contract — do NOT converge):
//   Chat day-pill Today/Yesterday calendar fallback remains presentation-local
//   unpadded d/M/yyyy (chat_detail_date_header_presentation_test).
//
// Out of scope (untouched): domain order timeline, notification grouping,
// countdown/duration family.
//
// This gate proves:
//   * AppFormatters.formatShortDate is the padded canonical short date
//   * OBSOLETE presentation sites no longer interpolate unpadded d/M/yyyy
//   * chat day-pill contract remains unpadded and green
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:labuda/shared/utils/app_formatters.dart';

const List<String> _obsoleteSites = <String>[
  'lib/shared/widgets/attachment_widget.dart',
  'lib/domains/commerce/transaction/order/presentation/widgets/order_overdue_cards.dart',
  'lib/domains/commerce/transaction/order/presentation/screens/order_detail_screen.dart',
  'lib/domains/user/profile/presentation/widgets/personal_information_section.dart',
  'lib/domains/user/profile/presentation/widgets/personal_info/phone_verification_field.dart',
  'lib/domains/user/profile/presentation/widgets/personal_info/date_of_birth_picker.dart',
  'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
  'lib/domains/commerce/catalog/auction/presentation/screens/seller_auction_relist_screen.dart',
];

String _source(String path) =>
    File(path).readAsStringSync().replaceAll('\r\n', '\n');

bool _hasUnpaddedDateInterpolation(String source) {
  // Matches ${x.day}/${x.month}/${x.year} style unpadded absolute dates.
  return RegExp(r'\$\{[^}]+\.day\}/\$\{[^}]+\.month\}/\$\{[^}]+\.year\}')
      .hasMatch(source);
}

void main() {
  setUpAll(() async {
    await initializeDateFormatting('id_ID');
  });

  group('AppFormatters absolute short-date authority', () {
    test('formatShortDate is the padded canonical short date', () {
      expect(
        AppFormatters.formatShortDate(DateTime(2024, 1, 15)),
        '15/01/2024',
      );
      expect(
        AppFormatters.formatShortDate(DateTime(2026, 5, 1)),
        '01/05/2026',
      );
    });

    test('formatDate is the readable absolute month-name date', () {
      expect(AppFormatters.formatDate(DateTime(2024, 1, 15)), '15 Jan 2024');
    });
  });

  group('OBSOLETE unpadded presentation sites are converged', () {
    test('no OBSOLETE site still interpolates unpadded d/M/yyyy', () {
      for (final String path in _obsoleteSites) {
        final String source = _source(path);
        expect(
          _hasUnpaddedDateInterpolation(source),
          isFalse,
          reason: '$path must not keep unpadded d/M/yyyy interpolation',
        );
      }
    });

    test('order readiness surfaces use AppFormatters.formatShortDate', () {
      expect(
        _source(_obsoleteSites[1]),
        contains('AppFormatters.formatShortDate(order.readyToShipBy!)'),
      );
      expect(
        _source(_obsoleteSites[2]),
        contains('AppFormatters.formatShortDate(readyToShipBy)'),
      );
    });

    test('profile identity dates use AppFormatters.formatShortDate', () {
      expect(
        _source(_obsoleteSites[3]),
        contains('AppFormatters.formatShortDate(phoneVerifiedAt!)'),
      );
      expect(
        _source(_obsoleteSites[3]),
        contains('AppFormatters.formatShortDate(dateOfBirth!)'),
      );
      expect(
        _source(_obsoleteSites[4]),
        contains('AppFormatters.formatShortDate(phoneVerifiedAt!)'),
      );
      expect(
        _source(_obsoleteSites[5]),
        contains('AppFormatters.formatShortDate(dateOfBirth!)'),
      );
    });

    test('shipping validity uses AppFormatters.formatDate', () {
      expect(
        _source(_obsoleteSites[0]),
        contains('AppFormatters.formatDate(shipping.validUntil)'),
      );
      expect(_source(_obsoleteSites[0]), isNot(contains('_formatDate')));
    });
  });

  group('Closed chat day-pill family remains presentation-local unpadded', () {
    test('chat_detail_screen day-pill fallback is still unpadded d/M/yyyy', () {
      final String source = _source(
        'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
      );
      expect(
        _hasUnpaddedDateInterpolation(source),
        isTrue,
        reason:
            'chat day-pill calendar fallback remains unpadded by CLOSED contract',
      );
    });
  });
}
