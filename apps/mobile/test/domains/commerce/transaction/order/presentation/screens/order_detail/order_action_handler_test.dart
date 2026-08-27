import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart'
    as order_domain;
import 'package:labuda/domains/commerce/transaction/order/presentation/screens/order_detail/order_action_handler.dart';

order_domain.Order _order() {
  return order_domain.Order(
    id: 'order-1',
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    items: const [],
    status: order_domain.OrderStatus.pending,
    paymentMethod: order_domain.PaymentMethodType.bankTransfer,
    paymentStatus: order_domain.PaymentStatus.pending,
    shippingInfo: const order_domain.ShippingInfo(
      recipientName: 'Buyer',
      phone: '08123456789',
      address: 'Some address',
      method: order_domain.ShippingMethod.courier,
      shippingCost: 10000,
    ),
    pricing: const order_domain.OrderPricing(
      subtotal: 100000,
      shippingCost: 10000,
      discount: 0,
      total: 110000,
    ),
    createdAt: DateTime.utc(2026, 7, 1),
    source: order_domain.OrderSource.forSale,
  );
}

order_domain.Action _payAction() {
  return const order_domain.Action(
    type: 'pay',
    labelKey: 'action.pay_now',
    enabled: true,
    endpoint: '/api/v1/payments',
    method: 'POST',
    requiresIdempotency: true,
    financial: true,
  );
}

void main() {
  testWidgets('pay action routes to onPayNow', (tester) async {
    late BuildContext capturedContext;
    var payNowCalled = false;

    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) {
            capturedContext = context;
            return const SizedBox.shrink();
          },
        ),
      ),
    );

    final handler = OrderActionHandler(
      order: _order(),
      context: capturedContext,
      onAcceptOrder: (orderId, sellerId) {},
      onRejectOrder: (orderId, sellerId, reason) {},
      onShipOrder: (orderId, sellerId, proofData) {},
      onConfirmDelivery: (orderId, buyerId) {},
      onExtendConfirmation: (orderId) {},
      onRefundRequestRequest:
          ({
            required String orderId,
            required double orderSubtotal,
            required String buyerId,
            required String sellerId,
          }) {},
      onRate: (orderId, fromUserId, toUserId, rating, review) {},
      onPayNow: (order) {
        payNowCalled = true;
      },
      onChangePaymentMethod: (order) {},
      onCancelOrder: (orderId, reason) {},
      onOpenDispute: ({required String orderId}) {},
      onRequestSupport: () {},
    );

    await handler.handleAction(_payAction());

    expect(payNowCalled, isTrue);
  });
}
