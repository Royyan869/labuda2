import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/types/payment_types.dart';
import 'package:labuda/domains/commerce/transaction/order/data/mappers/order_mapper.dart';
import 'package:labuda/domains/commerce/transaction/order/data/models/api/order_api_response_dtos.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/shipping_types.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/providers/order_providers.dart';

class _FakeOrderRepository implements OrderRepository {
  _FakeOrderRepository(this._stream);

  final Stream<Order> _stream;

  @override
  Stream<Order> watchOrder(String orderId) => _stream;

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Order _baseOrder({
  required String orderId,
  required RefundRequest? activeRefund,
}) {
  return Order(
    id: orderId,
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    items: const [],
    status: OrderStatus.shipped,
    paymentMethod: PaymentMethodType.bankTransfer,
    paymentStatus: PaymentStatus.pending,
    shippingInfo: const ShippingInfo(
      recipientName: 'Buyer',
      phone: '08123',
      address: 'Address',
      method: ShippingMethod.courier,
      shippingCost: 10000,
    ),
    pricing: const OrderPricing(
      subtotal: 100000,
      shippingCost: 10000,
      discount: 0,
      total: 110000,
    ),
    createdAt: DateTime(2026, 6, 1),
    source: OrderSource.forSale,
    hasActiveRefund: activeRefund != null,
    activeRefund: activeRefund,
  );
}

void main() {
  group('Order active refund visibility P1', () {
    test('OrderApiResponse parses snake_case active_refund payload', () {
      final dto = OrderApiResponse.fromJson({
        'id': 'order-1',
        'buyer_id': 'buyer-1',
        'seller_id': 'seller-1',
        'product_id': 'product-1',
        'quantity': 1,
        'total_amount': 100000,
        'final_amount': 100000,
        'shipping_fee': 0,
        'discount_amount': 0,
        'coin_discount': 0,
        'status': 'shipped',
        'payment_status': 'paid',
        'created_at': 1717200000,
        'has_active_refund': true,
        'active_refund': {
          'id': 'refund-1',
          'order_id': 'order-1',
          'buyer_id': 'buyer-1',
          'seller_id': 'seller-1',
          'status': 'seller_rejected',
          'reason': 'item_not_received',
          'description': 'not arrived',
          'requested_amount': 75000,
          'seller_notes': 'rejected by seller',
          'evidence_urls': ['https://example.com/evidence.jpg'],
          'created_at': 1717200000,
          'updated_at': 1717200300,
        },
      });

      expect(dto.hasActiveRefund, isTrue);
      expect(dto.activeRefund, isNotNull);
      expect(dto.activeRefund!.orderId, 'order-1');
      expect(dto.activeRefund!.buyerId, 'buyer-1');
      expect(dto.activeRefund!.sellerId, 'seller-1');
      expect(dto.activeRefund!.requestedAmount, 75000);
      expect(dto.activeRefund!.sellerNotes, 'rejected by seller');
    });

    test('OrderMapper maps active_refund into domain Order.activeRefund', () {
      final dto = OrderApiResponse.fromJson({
        'id': 'order-1',
        'buyer_id': 'buyer-1',
        'seller_id': 'seller-1',
        'product_id': 'product-1',
        'quantity': 1,
        'total_amount': 100000,
        'final_amount': 100000,
        'shipping_fee': 0,
        'discount_amount': 0,
        'coin_discount': 0,
        'status': 'shipped',
        'payment_status': 'paid',
        'created_at': 1717200000,
        'has_active_refund': true,
        'active_refund': {
          'id': 'refund-1',
          'order_id': 'order-1',
          'buyer_id': 'buyer-1',
          'seller_id': 'seller-1',
          'status': 'seller_rejected',
          'reason': 'item_not_received',
          'requested_amount': 50000,
          'created_at': 1717200000,
          'updated_at': 1717200300,
        },
      });

      final order = OrderMapper.toOrder(dto);
      expect(order.hasActiveRefund, isTrue);
      expect(order.activeRefund, isNotNull);
      expect(order.activeRefund!.id, 'refund-1');
      expect(order.activeRefund!.status, RefundStatus.sellerRejected);
    });

    test(
      'refundsByOrderProvider emits [activeRefund] from watchOrderProvider',
      () async {
        final refund = RefundRequest(
          id: 'refund-1',
          orderId: 'order-1',
          buyerId: 'buyer-1',
          sellerId: 'seller-1',
          reason: RefundReason.itemNotReceived,
          status: RefundStatus.pendingSellerReview,
          refundAmount: 100000,
          createdAt: DateTime(2026, 6, 1),
        );
        final order = _baseOrder(orderId: 'order-1', activeRefund: refund);

        final container = ProviderContainer(
          overrides: [
            orderRepositoryProvider.overrideWithValue(
              _FakeOrderRepository(Stream.value(order)),
            ),
          ],
        );
        addTearDown(container.dispose);

        final completer = Completer<List<RefundRequest>>();
        final sub = container.listen(refundsByOrderProvider('order-1'), (
          prev,
          next,
        ) {
          next.whenData((value) {
            if (!completer.isCompleted) completer.complete(value);
          });
        }, fireImmediately: true);
        addTearDown(sub.close);

        final emitted = await completer.future;
        expect(emitted, hasLength(1));
        expect(emitted.first.id, 'refund-1');
      },
    );

    test('refundsByOrderProvider emits [] when no active refund', () async {
      final order = _baseOrder(orderId: 'order-2', activeRefund: null);
      final container = ProviderContainer(
        overrides: [
          orderRepositoryProvider.overrideWithValue(
            _FakeOrderRepository(Stream.value(order)),
          ),
        ],
      );
      addTearDown(container.dispose);

      final completer = Completer<List<RefundRequest>>();
      final sub = container.listen(refundsByOrderProvider('order-2'), (
        prev,
        next,
      ) {
        next.whenData((value) {
          if (!completer.isCompleted) completer.complete(value);
        });
      }, fireImmediately: true);
      addTearDown(sub.close);

      final emitted = await completer.future;
      expect(emitted, isEmpty);
    });

    test('derived refund id can drive canonical action paths', () {
      final refund = RefundRequest(
        id: 'refund-xyz',
        orderId: 'order-1',
        buyerId: 'buyer-1',
        sellerId: 'seller-1',
        reason: RefundReason.itemNotReceived,
        status: RefundStatus.sellerRejected,
        refundAmount: 100000,
        createdAt: DateTime(2026, 6, 1),
      );

      expect('/refunds/${refund.id}/approve', '/refunds/refund-xyz/approve');
      expect('/refunds/${refund.id}/reject', '/refunds/refund-xyz/reject');
      expect('/refunds/${refund.id}/escalate', '/refunds/refund-xyz/escalate');
    });
  });
}
