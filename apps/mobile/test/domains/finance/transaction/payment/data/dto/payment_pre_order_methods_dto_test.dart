// Canonical PRE-ORDER payment pricing wire contract (Phase 2).
//
// WIRE AUTHORITY: GET /api/v1/payments/pre-order-methods →
// CorePaymentHandler.ListPreOrderPaymentMethods emits exactly:
//   pricing_token, expires_at, escrow_amount, coins_to_use, cash_amount,
//   currency, methods[]: {method_code, display_name,
//                         buyer_payment_fee_amount, final_payable_amount}
//
// The pre-order surface emits `final_payable_amount` (post-fee) and NEVER the
// ambiguous pre-fee `total_payable_amount`.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/finance/transaction/payment/data/dto/payment_dto.dart';

void main() {
  group('PreOrderPaymentPricingDto', () {
    test('parses the canonical pre-order response shape', () {
      final dto = PreOrderPaymentPricingDto.fromJson({
        'pricing_token': 'b1d2c3e4-0000-4000-8000-9a8b7c6d5e4f',
        'expires_at': '2026-10-05T10:10:00Z',
        'escrow_amount': 110000,
        'coins_to_use': 0,
        'cash_amount': 110000,
        'currency': 'IDR',
        'methods': [
          {
            'method_code': 'bank_transfer',
            'display_name': 'Transfer Bank (Virtual Account)',
            'buyer_payment_fee_amount': 4000,
            'final_payable_amount': 114000,
          },
        ],
      });

      expect(dto.pricingToken, 'b1d2c3e4-0000-4000-8000-9a8b7c6d5e4f');
      expect(dto.escrowAmount, 110000);
      expect(dto.coinsToUse, 0);
      expect(dto.cashAmount, 110000);
      expect(dto.currency, 'IDR');
      expect(dto.methods, hasLength(1));

      final m = dto.methods.single;
      expect(m.methodCode, 'bank_transfer');
      expect(m.buyerPaymentFeeAmount, 4000);
      expect(m.finalPayableAmount, 114000);
      expect(dto.expiresAt, isNotNull);
    });

    test('entity exposes optionFor and never derives money client-side', () {
      final entity = PreOrderPaymentPricingDto.fromJson({
        'pricing_token': 'tok-1',
        'expires_at': '2026-10-05T10:10:00Z',
        'escrow_amount': 50000,
        'coins_to_use': 0,
        'cash_amount': 50000,
        'currency': 'IDR',
        'methods': [
          {
            'method_code': 'qris',
            'display_name': 'QRIS',
            'buyer_payment_fee_amount': 350,
            'final_payable_amount': 50350,
          },
        ],
      }).toEntity();

      expect(entity.optionFor('qris')!.finalPayableAmount, 50350);
      expect(entity.optionFor('missing'), isNull);
    });
  });
}
