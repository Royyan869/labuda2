import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/types/payment_types.dart';

void main() {
  group('PaymentStatus.fromString', () {
    test('parses every canonical enum name, case-insensitively', () {
      for (final status in PaymentStatus.values) {
        expect(PaymentStatus.fromString(status.name), status);
        expect(PaymentStatus.fromString(status.name.toUpperCase()), status);
        expect(PaymentStatus.fromString('  ${status.name}  '), status);
      }
    });

    test('rejects gateway vocabulary instead of coercing it', () {
      // Backend adalah authority status payment dan tidak pernah mengirim
      // kosakata Midtrans/gateway ke wire ini (`midtrans_status` termasuk
      // forbidden response key pada kontrak payment). Nilai-nilai itu dulu
      // dipetakan diam-diam; sekarang ditolak, karena nilai gateway yang
      // terbaca sebagai status lain adalah kebohongan di jalur uang.
      for (final legacy in [
        'settlement',
        'capture',
        'completed',
        'challenge',
        'deny',
        'cancel',
        'cancelled',
        'expire',
        'process',
      ]) {
        expect(
          () => PaymentStatus.fromString(legacy),
          throwsA(isA<FormatException>()),
          reason: 'nilai legacy/gateway tidak boleh dikoersi: $legacy',
        );
      }
    });
  });
}
