import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/data/mappers/order_mapper.dart';
import 'package:labuda/domains/commerce/transaction/order/data/models/api/order_api_response_dtos.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart'
    show PaymentStatus;

// Minimal OrderApiResponse factory for mapper tests.
OrderApiResponse _orderWith({String paymentStatus = '', String? paymentId}) {
  return OrderApiResponse(
    id: 'test-order-id',
    orderNumber: 'ORD-001',
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    productId: 'product-1',
    quantity: 1,
    totalAmount: 100000,
    finalAmount: 100000,
    shippingFee: 0,
    discountAmount: 0,
    coinDiscount: 0,
    status: 'pending_payment',
    paymentStatus: paymentStatus,
    createdAt: DateTime.now(),
    hasActiveRefund: false,
    paymentId: paymentId,
  );
}

void main() {
  group('OrderMapper payment status — crash prevention', () {
    test(
      'absent payment_status (empty string) maps to pending without throwing',
      () {
        final dto = _orderWith(paymentStatus: '');
        expect(() => OrderMapper.toOrder(dto), returnsNormally);
        final order = OrderMapper.toOrder(dto);
        expect(order.paymentStatus, PaymentStatus.pending);
      },
    );

    test('toOrderList with absent payment_status does not throw', () {
      final dtos = [_orderWith(), _orderWith(), _orderWith()];
      expect(() => OrderMapper.toOrderList(dtos), returnsNormally);
    });

    test('unknown payment_status degrades gracefully to pending', () {
      final dto = _orderWith(paymentStatus: 'future_gateway_status');
      expect(() => OrderMapper.toOrder(dto), returnsNormally);
      final order = OrderMapper.toOrder(dto);
      expect(order.paymentStatus, PaymentStatus.pending);
    });

    test('settlement maps to paid', () {
      final order = OrderMapper.toOrder(
        _orderWith(paymentStatus: 'settlement'),
      );
      expect(order.paymentStatus, PaymentStatus.paid);
    });

    test('capture maps to paid', () {
      final order = OrderMapper.toOrder(_orderWith(paymentStatus: 'capture'));
      expect(order.paymentStatus, PaymentStatus.paid);
    });

    test('challenge maps to processing', () {
      final order = OrderMapper.toOrder(_orderWith(paymentStatus: 'challenge'));
      expect(order.paymentStatus, PaymentStatus.processing);
    });

    test('paid maps to paid', () {
      final order = OrderMapper.toOrder(_orderWith(paymentStatus: 'paid'));
      expect(order.paymentStatus, PaymentStatus.paid);
    });

    test('pending maps to pending', () {
      final order = OrderMapper.toOrder(_orderWith(paymentStatus: 'pending'));
      expect(order.paymentStatus, PaymentStatus.pending);
    });

    test('failed maps to failed', () {
      final order = OrderMapper.toOrder(_orderWith(paymentStatus: 'failed'));
      expect(order.paymentStatus, PaymentStatus.failed);
    });

    test('expired maps to expired', () {
      final order = OrderMapper.toOrder(_orderWith(paymentStatus: 'expired'));
      expect(order.paymentStatus, PaymentStatus.expired);
    });

    test('refunded maps to refunded', () {
      final order = OrderMapper.toOrder(_orderWith(paymentStatus: 'refunded'));
      expect(order.paymentStatus, PaymentStatus.refunded);
    });
  });

  group('OrderMapper payment identity — hydration', () {
    test('payment_id maps to Order.paymentId for detail DTO', () {
      final order = OrderMapper.toOrder(_orderWith(paymentId: 'pay-123'));
      expect(order.paymentId, 'pay-123');
    });

    test('payment_id maps to Order.paymentId for list hydration', () {
      final orders = OrderMapper.toOrderList([
        _orderWith(paymentId: 'pay-abc'),
        _orderWith(),
      ]);

      expect(orders.first.paymentId, 'pay-abc');
      expect(orders.last.paymentId, isNull);
    });
  });

  group('handlePayNow guard — order status based', () {
    // The guard in order_detail_handlers.dart uses order.status == OrderStatus.pending.
    // Verify that OrderStatus.pending is the correct value for payable orders.
    test('pending_payment order status parses to OrderStatus.pending', () {
      final dto = _orderWith(paymentStatus: 'pending');
      final order = OrderMapper.toOrder(dto);
      // Backend sends 'pending_payment'; mobile maps to OrderStatus.pending
      expect(order.status, OrderStatus.pending);
    });

    test('paid order status must not be OrderStatus.pending', () {
      final dto = _orderWith(paymentStatus: 'settlement');
      // Override status to 'paid'
      final paidDto = OrderApiResponse(
        id: dto.id,
        orderNumber: dto.orderNumber,
        buyerId: dto.buyerId,
        sellerId: dto.sellerId,
        productId: dto.productId,
        quantity: dto.quantity,
        totalAmount: dto.totalAmount,
        finalAmount: dto.finalAmount,
        shippingFee: dto.shippingFee,
        discountAmount: dto.discountAmount,
        coinDiscount: dto.coinDiscount,
        status: 'paid',
        paymentStatus: 'settlement',
        createdAt: dto.createdAt,
        hasActiveRefund: false,
      );
      final order = OrderMapper.toOrder(paidDto);
      expect(order.status, isNot(OrderStatus.pending));
      expect(order.paymentStatus, PaymentStatus.paid);
    });
  });
}
