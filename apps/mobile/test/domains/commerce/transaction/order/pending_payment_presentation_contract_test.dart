// `pending_payment` presentation contract.
//
// CANONICAL: `pending_payment` = order created and persisted, awaiting BUYER
// payment. It is directly payable; there is NO seller-confirmation step.
// Canonical buyer label: "Menunggu Pembayaran"; canonical CTA: "Bayar Sekarang".
//
// Positive proof: the three live order surfaces render the canonical label.
// Negative proof: the obsolete seller-confirmation concept and its wording are
// gone from the order scope, and the dead timeline use case file stays deleted.
// The payment-polling label ("Menunggu Konfirmasi Pembayaran") is a DIFFERENT
// semantic and MUST remain.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

const _orderLib = 'lib/domains/commerce/transaction/order';
const _list = '$_orderLib/presentation/screens/order_list_screen.dart';
const _infoCard = '$_orderLib/presentation/widgets/order_info_card.dart';
const _timeline = '$_orderLib/presentation/widgets/order_status_timeline.dart';
const _orderEntity = '$_orderLib/domain/entities/order.dart';
const _detail = '$_orderLib/presentation/screens/order_detail_screen.dart';
const _deadFile =
    '$_orderLib/domain/usecases/build_order_timeline_usecase.dart';
const _paymentResult =
    'lib/domains/commerce/transaction/checkout/presentation/screens/'
    'payment_result_screen_impl.dart';

String _read(String path) => File(path).readAsStringSync();

void main() {
  group('pending_payment renders "Menunggu Pembayaran" on buyer surfaces', () {
    test('order list badge', () {
      final src = _read(_list);
      expect(src, contains("return 'Menunggu Pembayaran';"));
      expect(src, isNot(contains('Menunggu Konfirmasi')));
    });

    test('order info card', () {
      final src = _read(_infoCard);
      expect(src, contains("return 'Menunggu Pembayaran';"));
      expect(src, isNot(contains('Menunggu Konfirmasi')));
    });

    test('order timeline status label', () {
      final src = _read(_timeline);
      expect(src, contains("return 'Menunggu Pembayaran';"));
      expect(src, isNot(contains('Menunggu Konfirmasi')));
    });

    test('timeline step sublabel means payment, not seller confirmation', () {
      final src = _read(_timeline);
      expect(src, contains("'Segera selesaikan pembayaran'"));
      expect(src, isNot(contains('Menunggu konfirmasi penjual')));
    });
  });

  group('obsolete seller-confirmation concept is purged', () {
    test('order entity no longer derives a seller action for pending', () {
      expect(_read(_orderEntity), isNot(contains('isSellerActionRequired')));
    });

    test('order detail has no seller-action-required banner gate', () {
      final src = _read(_detail);
      expect(src, isNot(contains('SellerActionRequiredBanner')));
      expect(src, isNot(contains('isSellerActionRequired')));
    });

    test('dead timeline use case file stays deleted', () {
      expect(
        File(_deadFile).existsSync(),
        isFalse,
        reason:
            'build_order_timeline_usecase.dart had zero consumers and must '
            'not be resurrected',
      );
    });
  });

  group('payment polling label is preserved', () {
    test('payment-result keeps "Menunggu Konfirmasi Pembayaran"', () {
      expect(
        _read(_paymentResult),
        contains('Menunggu Konfirmasi Pembayaran'),
        reason:
            'this is payment confirmation/polling, NOT seller confirmation '
            'of an order',
      );
    });
  });

  group('negative residue sweep over the order scope', () {
    test('no order file names the obsolete labels or symbols', () {
      const needles = <String>[
        'Menunggu Konfirmasi',
        'Menunggu konfirmasi penjual',
        'Terima atau tolak',
        'isSellerActionRequired',
        'SellerActionRequiredBanner',
        'BuildOrderTimelineUseCase',
        'build_order_timeline_usecase',
      ];
      final offenders = <String>[];
      final files = Directory(_orderLib)
          .listSync(recursive: true)
          .whereType<File>()
          .where((f) => f.path.endsWith('.dart'));
      for (final file in files) {
        final source = file.readAsStringSync();
        for (final needle in needles) {
          if (source.contains(needle)) {
            offenders.add('${file.path} contains "$needle"');
          }
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });
  });
}
