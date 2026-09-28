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
    quantity: 1,
    // Canonical pricing contract (PD + S and PD + S + F); no legacy
    // total_amount / final_amount / shipping_fee / discount aliases.
    subtotal: 100000,
    shippingTotal: 0,
    commissionAmount: 0,
    totalBeforeCoinsAmount: 100000,
    totalPayableAmount: 100000,
    status: 'pending_payment',
    paymentStatus: paymentStatus,
    createdAt: DateTime.now(),
    hasActiveRefund: false,
    paymentId: paymentId,
  );
}

void main() {
  group('OrderMapper payment status — canonical vocabulary', () {
    test(
      'absent payment_status means NO VERDICT — not pending',
      () {
        final dto = _orderWith(paymentStatus: '');
        expect(() => OrderMapper.toOrder(dto), returnsNormally);
        final order = OrderMapper.toOrder(dto);
        expect(order.paymentStatus, isNull);
      },
    );

    test('toOrderList with absent payment_status does not throw', () {
      final dtos = [_orderWith(), _orderWith(), _orderWith()];
      expect(() => OrderMapper.toOrderList(dtos), returnsNormally);
    });

    test(
      'unknown payment_status is rejected loudly instead of reading as pending',
      () {
      final dto = _orderWith(paymentStatus: 'future_gateway_status');
      expect(() => OrderMapper.toOrder(dto), throwsFormatException,
          reason: 'a raw gateway status must not parse');
      // No client-side translation table: the backend owns the vocabulary,
      // so an unparseable value surfaces instead of silently reading pending.
    });

    test('gateway vocabulary is rejected — the backend normalises before the wire', () {
      // The client holds NO translation table. 'settlement' / 'capture' /
      // 'challenge' are payments-table vocabulary owned by finance; a leak of
      // that vocabulary onto the order wire is a contract violation, not
      // something this layer silently repairs.
      for (final raw in ['settlement', 'capture', 'challenge']) {
        expect(
          () => OrderMapper.toOrder(_orderWith(paymentStatus: raw)),
          throwsFormatException,
          reason: 'the wire speaks the canonical vocabulary only (raw: $raw)',
        );
      }
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
      final dto = _orderWith(paymentStatus: 'paid');
      // Override status to 'paid'
      final paidDto = OrderApiResponse(
        id: dto.id,
        orderNumber: dto.orderNumber,
        buyerId: dto.buyerId,
        sellerId: dto.sellerId,
        quantity: dto.quantity,
        subtotal: dto.subtotal,
        shippingTotal: dto.shippingTotal,
        commissionAmount: dto.commissionAmount,
        totalBeforeCoinsAmount: dto.totalBeforeCoinsAmount,
        totalPayableAmount: dto.totalPayableAmount,
        status: 'paid',
        paymentStatus: 'paid',
        createdAt: dto.createdAt,
        hasActiveRefund: false,
      );
      final order = OrderMapper.toOrder(paidDto);
      expect(order.status, isNot(OrderStatus.pending));
      expect(order.paymentStatus, PaymentStatus.paid);
    });
  });
}
