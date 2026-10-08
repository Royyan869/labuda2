// Post-order payment method convergence — wire parse proof.
//
// The canonical backend key `payment_method_code` (OrderDetailResponse) must map
// onto Order.paymentMethodCode. Null = unbound (auction-claim) order.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/data/mappers/order_mapper.dart';
import 'package:labuda/domains/commerce/transaction/order/data/models/api/order_api_response_dtos.dart';

void main() {
  test('OrderApiResponse maps payment_method_code onto Order.paymentMethodCode',
      () {
    final dto = OrderApiResponse.fromJson({
      'id': 'order-1',
      'buyer_id': 'buyer-1',
      'seller_id': 'seller-1',
      'status': 'pending_payment',
      'payment_status': 'pending',
      'payment_method_code': 'bank_transfer',
      'created_at': '2026-10-05T10:00:00Z',
    });

    final order = OrderMapper.toOrder(dto);
    expect(order.paymentMethodCode, 'bank_transfer');
  });

  test('unbound order (no payment_method_code) maps to null', () {
    final dto = OrderApiResponse.fromJson({
      'id': 'order-2',
      'buyer_id': 'buyer-1',
      'seller_id': 'seller-1',
      'status': 'pending_payment',
      'payment_status': 'pending',
      'created_at': '2026-10-05T10:00:00Z',
    });

    final order = OrderMapper.toOrder(dto);
    expect(order.paymentMethodCode, isNull);
  });
}
