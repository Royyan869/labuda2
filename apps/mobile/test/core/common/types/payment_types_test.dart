import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/types/payment_types.dart';

void main() {
  group('PaymentStatus.fromString', () {
    test('maps challenge to processing', () {
      expect(PaymentStatus.fromString('challenge'), PaymentStatus.processing);
    });

    test('maps deny and cancel to failed', () {
      expect(PaymentStatus.fromString('deny'), PaymentStatus.failed);
      expect(PaymentStatus.fromString('cancel'), PaymentStatus.failed);
    });

    test('maps expire to expired', () {
      expect(PaymentStatus.fromString('expire'), PaymentStatus.expired);
    });

    test('keeps direct enum names intact', () {
      expect(PaymentStatus.fromString('paid'), PaymentStatus.paid);
      expect(PaymentStatus.fromString('processing'), PaymentStatus.processing);
    });
  });
}
