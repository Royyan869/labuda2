import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/order/data/mappers/order_mapper.dart';
import 'package:labuda/domains/commerce/transaction/order/data/models/api/order_api_response_dtos.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart';

/// Canonical `GET /api/v1/orders/:id` payload for a seller + paid order, as
/// emitted by the backend decision engine (dto/decision.go): the primary action
/// IS the seller's canonical post-payment processing action.
Map<String, dynamic> _paidSellerOrderJson() {
  return {
    'id': 'order-1',
    'order_number': 'ORD-001',
    'buyer_id': 'buyer-1',
    'seller_id': 'seller-1',
    'quantity': 1,
    'subtotal': 100000,
    'shipping_total': 10000,
    'commission_amount': 0,
    'total_before_coins_amount': 110000,
    'total_payable_amount': 110000,
    'status': 'paid',
    'payment_status': 'paid',
    'escrow_status': 'holding',
    'created_at': 1750000000,
    'has_active_refund': false,
    'decision': {
      'state': 'paid',
      'version': '3.0.0',
      'decision_version': 1750000000,
      'primary_action': {
        'type': 'mark_shipped',
        'label_key': 'action.mark_shipped',
        'enabled': true,
        'endpoint': '/api/v1/orders/order-1/ship',
        'method': 'POST',
        'requires_idempotency': true,
        'financial': false,
      },
      'secondary_actions': <dynamic>[],
      'display': {
        'badge': 'Menunggu Pengiriman',
        'badge_variant': 'info',
      },
    },
  };
}

Map<String, dynamic> _pendingSellerOrderJson() {
  final json = _paidSellerOrderJson();
  json['status'] = 'pending_payment';
  json['payment_status'] = '';
  json['escrow_status'] = null;
  // Seller + pending_payment carries NO processing action.
  json.remove('decision');
  return json;
}

void main() {
  group('Order decision + escrow wiring — API → DTO → mapper → Order', () {
    test('parses backend decision and escrow_status onto the DTO', () {
      final dto = OrderApiResponse.fromJson(_paidSellerOrderJson());

      expect(dto.decision, isNotNull);
      expect(dto.decision!.state, 'paid');
      expect(dto.decision!.primaryAction, isNotNull);
      expect(dto.decision!.primaryAction!.type, 'mark_shipped');
      expect(dto.decision!.primaryAction!.labelKey, 'action.mark_shipped');
      expect(dto.escrowStatus, EscrowStatus.holding);
    });

    test('forwards decision and escrow_status through OrderMapper.toOrder', () {
      final dto = OrderApiResponse.fromJson(_paidSellerOrderJson());
      final order = OrderMapper.toOrder(dto);

      expect(order.decision, isNotNull);
      expect(order.decision!.primaryAction!.type, 'mark_shipped');
      expect(order.decision!.primaryAction!.enabled, isTrue);
      expect(order.escrowStatus, EscrowStatus.holding);
    });

    test('real consumer sees the seller mark_shipped action', () {
      final order = OrderMapper.toOrder(
        OrderApiResponse.fromJson(_paidSellerOrderJson()),
      );

      // This is exactly the predicate the Order Detail builder uses to decide
      // whether to render the action bar (order_detail_screen.dart).
      expect(order.decision, isNotNull);
      expect(order.decision!.hasActionType('mark_shipped'), isTrue);
    });

    test('absent decision stays null — no invented default', () {
      final dto = OrderApiResponse.fromJson(_pendingSellerOrderJson());
      expect(dto.decision, isNull);

      final order = OrderMapper.toOrder(dto);
      expect(order.decision, isNull);
      expect(order.escrowStatus, isNull);
    });

    test('unrecognised escrow_status maps to null, not a fake state', () {
      final json = _paidSellerOrderJson();
      json['escrow_status'] = 'frozen'; // not part of the canonical vocabulary
      final dto = OrderApiResponse.fromJson(json);
      expect(dto.escrowStatus, isNull);
    });

    test('list mapping preserves decision per order', () {
      final orders = OrderMapper.toOrderList([
        OrderApiResponse.fromJson(_paidSellerOrderJson()),
        OrderApiResponse.fromJson(_pendingSellerOrderJson()),
      ]);

      expect(orders.first.decision!.primaryAction!.type, 'mark_shipped');
      expect(orders.last.decision, isNull);
    });
  });
}