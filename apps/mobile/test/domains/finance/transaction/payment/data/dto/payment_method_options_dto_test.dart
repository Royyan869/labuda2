import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/finance/transaction/payment/data/dto/payment_dto.dart';

/// FIN-R01E-C / FIN-R01E-D — payment-method presentation contract.
///
/// GET /payments/methods exposes exactly ONE order amount key on the wire:
///
///   backend/internal/serverboot/dependencies.go
///     baseAmount := order.TotalBeforeCoinsAmount // PD+S canonical buyer base
///     gin.H{"base_amount": baseAmount.Int64(), ...}
///
/// `total_before_coins_amount` is the persisted ORDER column name (the canonical
/// persisted financial authority), NOT a payment-method wire key.
///
/// FIN-R01E-D: the mobile client consumes only the per-method options. The
/// order-level `base_amount` was parsed but never read downstream, so it was
/// purged from `PaymentMethodOptionsDto` rather than kept "just in case". This
/// test now guards the contract the client actually uses.
void main() {
  group('PaymentMethodOptionsDto (GET /payments/methods)', () {
    test('maps the per-method options the client consumes', () {
      final dto = PaymentMethodOptionsDto.fromJson(<String, dynamic>{
        'order_id': 'order-1',
        'base_amount': 110000,
        'coins_to_use': 0,
        'methods': <dynamic>[
          <String, dynamic>{
            'method_code': 'bank_transfer',
            'display_name': 'Bank Transfer',
            'coins_to_use': 0,
            'cash_amount': 110000,
            'buyer_payment_fee_amount': 4000,
            'total_payable_amount': 114000,
          },
        ],
      });

      expect(dto.orderId, 'order-1');
      expect(dto.methods.single.methodCode, 'bank_transfer');
      expect(dto.methods.single.displayName, 'Bank Transfer');
      expect(dto.methods.single.buyerPaymentFeeAmount, 4000);
      expect(dto.methods.single.totalPayableAmount, 114000);
    });

    test('a stray persisted-column key is ignored, not honored', () {
      final dto = PaymentMethodOptionsDto.fromJson(<String, dynamic>{
        'order_id': 'order-2',
        'base_amount': 110000,
        'total_before_coins_amount': 999000,
        'methods': <dynamic>[],
      });

      expect(dto.methods, isEmpty);
    });

    test('missing base_amount does not break method parsing', () {
      final dto = PaymentMethodOptionsDto.fromJson(<String, dynamic>{
        'order_id': 'order-3',
        'total_before_coins_amount': 110000,
        'methods': <dynamic>[
          <String, dynamic>{
            'method_code': 'qris',
            'display_name': 'QRIS',
            'buyer_payment_fee_amount': 1000,
            'total_payable_amount': 111000,
          },
        ],
      });

      expect(dto.methods.single.methodCode, 'qris');
      expect(dto.methods.single.totalPayableAmount, 111000);
    });
  });
}
