import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/commerce/transaction/order/data/dto/dto_barrel.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_repository_impl.dart';
import 'package:labuda/domains/commerce/transaction/order/data/remote/order_api_datasource_impl.dart';
import 'package:labuda/domains/commerce/transaction/order/data/remote/order_remote_datasource.dart';
import 'package:labuda/domains/commerce/transaction/order/data/models/api/order_api_models.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_params.dart';

class _RecordingApiClient implements ApiClient {
  String? lastGetPath;
  String? lastPostPath;
  String? lastPutPath;
  Map<String, dynamic>? lastGetQuery;
  dynamic lastPostData;

  dynamic getPayload = <String, dynamic>{
    'success': true,
    'data': <String, dynamic>{},
  };
  dynamic postPayload = <String, dynamic>{
    'success': true,
    'data': <String, dynamic>{},
  };
  dynamic putPayload = <String, dynamic>{
    'success': true,
    'data': <String, dynamic>{},
  };

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastGetPath = path;
    lastGetQuery = queryParameters;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: getPayload as T,
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPostPath = path;
    lastPostData = data;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as T,
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> put<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPutPath = path;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: putPayload as T,
      statusCode: 200,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> _orderPayload(String id) => {
  'id': id,
  'buyer_id': 'b1',
  'seller_id': 's1',
  'status': 'paid',
  'source_type': 'for_sale',
  'source_id': 'p1',
  'subtotal': 10000,
  'shipping_total': 1000,
  'commission_amount': 500,
  'total_before_coins_amount': 11000,
  'created_at': '2026-06-01T00:00:00Z',
  'updated_at': '2026-06-01T00:00:00Z',
};

class _FakeDatasource implements OrderRemoteDatasource {
  final List<String> calls = [];
  String? lastOrderId;
  OrderApiException? failWith;
  Map<String, dynamic> Function(Map<String, dynamic> body)? pricingPreviewCallback;

  /// Overrides the canonical GET /orders/:id payload when set.
  Map<String, dynamic>? getOrderPayload;

  void _maybeThrow() {
    if (failWith != null) throw failWith!;
  }

  @override
  Future<OrderApiResponse> cancelOrder(String orderId) async {
    calls.add('cancel');
    lastOrderId = orderId;
    _maybeThrow();
    return OrderApiResponse.fromJson({'id': orderId});
  }

  @override
  Future<CheckDeliveryApiResponse> checkDelivery(
    CheckDeliveryApiRequest request,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderApiResponse> completeOrder(String orderId) async {
    calls.add('complete');
    lastOrderId = orderId;
    _maybeThrow();
    return OrderApiResponse.fromJson({'id': orderId});
  }

  @override
  Future<DisputeDto> createDispute(
    String orderId,
    CreateDisputeDto request,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<void> extendOrderConfirmation(String orderId) async {
    calls.add('extend-confirmation');
    lastOrderId = orderId;
    _maybeThrow();
  }

  @override
  Future<Map<String, dynamic>> fetchPricingPreview(
    Map<String, dynamic> body,
  ) async {
    if (pricingPreviewCallback != null) {
      return pricingPreviewCallback!(body);
    }
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderApiResponse> getOrder(String orderId) async {
    calls.add('get-order');
    return OrderApiResponse.fromJson(getOrderPayload ?? _orderPayload(orderId));
  }

  @override
  Future<RefundDto> getRefund(String refundId) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<RefundDto?> getRefundByOrderId(String orderId) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<RefundListDto> listMyRefunds({RefundFilterParams? params}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderListApiResponse> listMyOrders({OrderFilterParams? params}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<RefundListDto> listSellerRefunds({RefundFilterParams? params}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderListApiResponse> listSellerOrders({OrderFilterParams? params}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderApiResponse> shipOrder(String orderId, MarkAsShippedParams params) async {
    calls.add('ship');
    lastOrderId = orderId;
    _maybeThrow();
    return OrderApiResponse.fromJson({'id': orderId});
  }

  @override
  Future<RefundDto> requestRefund(
    String orderId,
    CreateRefundDto request,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<Map<String, dynamic>> escalateRefund(String refundId) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<RefundDto> approveRefund(String refundId, {String? notes}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<RefundDto> rejectRefund(String refundId, {String? notes}) async {
    throw UnsupportedError('not used');
  }
}

void main() {
  group('Order contract P1 datasource', () {
    test('GET /orders uses role=buyer and limit, not page/page_size', () async {
      final client = _RecordingApiClient();
      final ds = OrderApiDatasourceImpl(client);
      client.getPayload = {
        'success': true,
        'data': {'data': <dynamic>[]},
      };

      await ds.listMyOrders(params: OrderFilterParams(page: 4, pageSize: 25));
      expect(client.lastGetPath, '/orders');
      expect(client.lastGetQuery?['role'], 'buyer');
      expect(client.lastGetQuery?['limit'], 25);
      expect(client.lastGetQuery?.containsKey('page'), isFalse);
      expect(client.lastGetQuery?.containsKey('page_size'), isFalse);
    });

    test('GET /orders uses role=seller and limit', () async {
      final client = _RecordingApiClient();
      final ds = OrderApiDatasourceImpl(client);
      client.getPayload = {
        'success': true,
        'data': {'data': <dynamic>[]},
      };

      await ds.listSellerOrders(params: OrderFilterParams(pageSize: 15));
      expect(client.lastGetPath, '/orders');
      expect(client.lastGetQuery?['role'], 'seller');
      expect(client.lastGetQuery?['limit'], 15);
    });

    test('GET /orders parses seller list envelope with empty orders', () async {
      final client = _RecordingApiClient();
      final ds = OrderApiDatasourceImpl(client);
      client.getPayload = {
        'success': true,
        'data': {'orders': <dynamic>[], 'limit': 3},
      };

      final result = await ds.listSellerOrders(
        params: OrderFilterParams(pageSize: 3),
      );
      expect(result.data, isEmpty);
    });

    test('GET /orders tolerates legacy raw list payloads', () async {
      final client = _RecordingApiClient();
      final ds = OrderApiDatasourceImpl(client);
      client.getPayload = {'success': true, 'data': <dynamic>[]};

      final result = await ds.listSellerOrders(
        params: OrderFilterParams(pageSize: 3),
      );
      expect(result.data, isEmpty);
    });

    // The obsolete order-lifecycle endpoints (preview, stats, confirm, order
    // confirmation, order-by-number, update-status, shipping-proof, admin
    // dispute) were purged from OrderRemoteDatasource. Their absence is now
    // enforced by the type system: referencing them fails compilation.
    // Only the surviving backend-unsupported refund read endpoints are
    // asserted at runtime below.
    test('refund read endpoints unsupported by backend contract throw', () async {
      final client = _RecordingApiClient();
      final ds = OrderApiDatasourceImpl(client);

      expect(
        () => ds.getRefundByOrderId('o1'),
        throwsA(isA<UnsupportedError>()),
      );
      expect(() => ds.getRefund('r1'), throwsA(isA<UnsupportedError>()));
      expect(() => ds.listMyRefunds(), throwsA(isA<UnsupportedError>()));
      expect(() => ds.listSellerRefunds(), throwsA(isA<UnsupportedError>()));
    });
  });

  group('Order contract P1 repository', () {
    test(
      'ship/complete/cancel/extend-confirmation perform action then getOrder',
      () async {
        final ds = _FakeDatasource();
        final repo = OrderRepositoryImpl(ds);

        await repo.markAsShipped(
          const MarkAsShippedParams(orderId: 'o1', shippingReference: 'R123'),
        );
        expect(ds.calls, ['ship', 'get-order']);
        ds.calls.clear();

        await repo.markAsDelivered('o1');
        expect(ds.calls, ['complete', 'get-order']);
        ds.calls.clear();

        await repo.cancelOrder('o1', const CancelOrderParams(reason: 'x'));
        expect(ds.calls, ['cancel', 'get-order']);
        ds.calls.clear();

        await repo.extendOrderConfirmation('o1');
        expect(ds.calls, ['extend-confirmation', 'get-order']);
      },
    );

    test('preserves backend error code into RepositoryResult', () async {
      final ds = _FakeDatasource()
        ..failWith = const OrderApiException(
          'blocked',
          code: 'ACCOUNT_SUSPENDED',
          details: {'reason': 'suspended'},
        );
      final repo = OrderRepositoryImpl(ds);

      final result = await repo.markAsDelivered('o1');
      expect(result.isError, isTrue);
      expect(result.errorCode, 'ACCOUNT_SUSPENDED');
      expect(result.errorDetails?['reason'], 'suspended');
    });
  });

  // ========================================================================
  // STAGE 13 — OrderRepositoryImpl.previewOrder wire contract proof
  // ========================================================================
  group('OrderRepositoryImpl.previewOrder shipping option contract', () {
    test('standard shipping: wire payload contains shipping_option_id, not shipping_setup_id', () async {
      Map<String, dynamic>? capturedBody;
      final ds = _FakeDatasource()
        ..pricingPreviewCallback = (body) {
          capturedBody = body;
          return {
            'token': 'tok-1',
            'expires_at': '2026-12-31T23:59:59Z',
            'pricing_snapshot': {
              'subtotal': 10000,
              'shipping_total': 5000,
              'total_payable_amount': 15000,
            },
          };
        };
      final repo = OrderRepositoryImpl(ds);

      final result = await repo.previewOrder(
        const PreviewOrderParams(
          productId: '11111111-1111-1111-1111-111111111111',
          sourceType: 'for_sale',
          sourceId: '22222222-2222-2222-2222-222222222222',
          quantity: 1,
          addressId: '44444444-4444-4444-4444-444444444444',
          shippingSetupId: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        ),
      );

      expect(result.isSuccess, isTrue);
      expect(capturedBody, isNotNull);
      expect(capturedBody!['shipping_option_id'], 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa');
      expect(capturedBody!.containsKey('shipping_setup_id'), isFalse,
        reason: 'stale wire key must never appear in live pricing preview',
      );
    });

    test('value trace: selected shipping option ID travels unchanged as shipping_option_id', () async {
      const selectedOptionId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';
      Map<String, dynamic>? capturedBody;
      final ds = _FakeDatasource()
        ..pricingPreviewCallback = (body) {
          capturedBody = body;
          return {
            'token': 'tok-2',
            'expires_at': '2026-12-31T23:59:59Z',
            'pricing_snapshot': {
              'subtotal': 10000,
              'shipping_total': 5000,
              'total_payable_amount': 15000,
            },
          };
        };
      final repo = OrderRepositoryImpl(ds);

      await repo.previewOrder(
        const PreviewOrderParams(
          productId: '11111111-1111-1111-1111-111111111111',
          sourceType: 'for_sale',
          sourceId: '22222222-2222-2222-2222-222222222222',
          quantity: 1,
          addressId: '44444444-4444-4444-4444-444444444444',
          shippingSetupId: selectedOptionId,
        ),
      );

      expect(capturedBody!['shipping_option_id'], equals(selectedOptionId));
    });

    test('quote mode: no shipping_option_id when shippingQuoteId is set', () async {
      Map<String, dynamic>? capturedBody;
      final ds = _FakeDatasource()
        ..pricingPreviewCallback = (body) {
          capturedBody = body;
          return {
            'token': 'tok-3',
            'expires_at': '2026-12-31T23:59:59Z',
            'pricing_snapshot': {
              'subtotal': 10000,
              'shipping_total': 5000,
              'total_payable_amount': 15000,
            },
          };
        };
      final repo = OrderRepositoryImpl(ds);

      await repo.previewOrder(
        const PreviewOrderParams(
          productId: '11111111-1111-1111-1111-111111111111',
          sourceType: 'for_sale',
          sourceId: '22222222-2222-2222-2222-222222222222',
          quantity: 1,
          addressId: '44444444-4444-4444-4444-444444444444',
          shippingQuoteId: 'quote-abc',
        ),
      );

      expect(capturedBody!['shipping_quote_id'], 'quote-abc');
      expect(capturedBody!.containsKey('shipping_option_id'), isFalse);
      expect(capturedBody!.containsKey('shipping_setup_id'), isFalse);
    });

    test('negative stale-key guard: no live preview request emits shipping_setup_id', () async {
      Map<String, dynamic>? capturedBody;
      final ds = _FakeDatasource()
        ..pricingPreviewCallback = (body) {
          capturedBody = body;
          return {
            'token': 'tok-4',
            'expires_at': '2026-12-31T23:59:59Z',
            'pricing_snapshot': {
              'subtotal': 10000,
              'shipping_total': 0,
              'total_payable_amount': 10000,
            },
          };
        };
      final repo = OrderRepositoryImpl(ds);

      // With standard shipping option
      await repo.previewOrder(
        const PreviewOrderParams(
          productId: '11111111-1111-1111-1111-111111111111',
          sourceType: 'for_sale',
          sourceId: '22222222-2222-2222-2222-222222222222',
          quantity: 1,
          addressId: '44444444-4444-4444-4444-444444444444',
          shippingSetupId: 'opt-111',
        ),
      );
      expect(capturedBody!.containsKey('shipping_setup_id'), isFalse);

      // With no shipping
      await repo.previewOrder(
        const PreviewOrderParams(
          productId: '11111111-1111-1111-1111-111111111111',
          sourceType: 'for_sale',
          sourceId: '22222222-2222-2222-2222-222222222222',
          quantity: 1,
          addressId: '44444444-4444-4444-4444-444444444444',
        ),
      );
      expect(capturedBody!.containsKey('shipping_setup_id'), isFalse);
    });

    test(
      'FIN-R01E-D: preview payable comes from the single canonical total_payable_amount key',
      () async {
        final ds = _FakeDatasource()
          ..pricingPreviewCallback = (_) => {
            'token': 'tok-canonical',
            'expires_at': '2026-12-31T23:59:59Z',
            'pricing_snapshot': {
              'subtotal': 100000,
              'shipping_total': 10000,
              'discount_amount': 15000,
              // canonical pricing-preview payable key (escrow + service fee)
              'total_payable_amount': 95000,
              // persisted ORDER column name — MUST NOT be honored as a fallback
              'total_before_coins_amount': 999999,
            },
          };
        final repo = OrderRepositoryImpl(ds);

        final result = await repo.previewOrder(
          const PreviewOrderParams(
            productId: '11111111-1111-1111-1111-111111111111',
            sourceType: 'for_sale',
            sourceId: '22222222-2222-2222-2222-222222222222',
            quantity: 1,
            addressId: '44444444-4444-4444-4444-444444444444',
          ),
        );

        expect(result.isSuccess, isTrue);
        // Canonical payable: no `total` compatibility alias may exist.
        expect(result.data!.pricing.totalPayableAmount, 95000);
      },
    );
  });

  group('Canonical GET /orders/:id payload mapping', () {
    test('maps shipping_address snapshot, items[] and shipping proof', () async {
      final ds = _FakeDatasource()
        ..getOrderPayload = {
          'id': 'o-1',
          'order_number': 'ORD-1',
          'buyer_id': 'b1',
          'seller_id': 's1',
          'quantity': 1,
          'status': 'shipped',
          'subtotal': 100000,
          'shipping_total': 10000,
          'commission_amount': 5000,
          'service_fee_amount': 2000,
          'total_before_coins_amount': 110000,
          'total_payable_amount': 112000,
          'payment_status': 'settlement',
          'tracking_number': 'RESI-123',
          'proof_type': 'phone',
          'shipping_note': 'diantar malam ini',
          'completed_at': 1767225600,
          'items': [
            {
              'id': 'i-1',
              'order_id': 'o-1',
              'product_id': 'p-1',
              'name': 'Koi Kohaku 30cm',
              'unit_price_snapshot': 100000,
              'quantity': 1,
              'subtotal': 100000,
            },
          ],
          'shipping_address': {
            'recipient_name': 'Budi',
            'phone': '0812345678',
            'street_address': 'Jl. Mawar No. 1',
            'province_name': 'Jawa Barat',
            'city_name': 'Bandung',
            'district_name': 'Coblong',
            'postal_code': '40132',
            'latitude': -6.9,
            'longitude': 107.6,
          },
        };
      final repo = OrderRepositoryImpl(ds);

      final result = await repo.getOrderById('o-1');
      expect(result.isSuccess, isTrue);
      final order = result.data!;

      // Canonical money — read directly from the persisted fields.
      expect(order.pricing.subtotal, 100000);
      expect(order.pricing.shippingCost, 10000);
      expect(order.pricing.commissionAmount, 5000);
      expect(order.pricing.serviceFeeAmount, 2000);
      expect(order.pricing.totalBeforeCoinsAmount, 110000);
      expect(order.pricing.totalPayableAmount, 112000);

      // Canonical address snapshot: orders.address_snapshot → shipping_address.
      expect(order.shippingInfo.recipientName, 'Budi');
      expect(order.shippingInfo.phone, '0812345678');
      expect(order.shippingInfo.address, 'Jl. Mawar No. 1');
      expect(order.shippingInfo.cityName, 'Bandung');
      expect(order.shippingInfo.provinceName, 'Jawa Barat');
      expect(order.shippingInfo.districtName, 'Coblong');
      expect(order.shippingInfo.postalCode, '40132');
      expect(order.shippingInfo.latitude, -6.9);

      // Canonical line items — backend `items[]`.
      expect(order.items.single.forSaleName, 'Koi Kohaku 30cm');
      expect(order.items.single.productId, 'p-1');
      expect(order.items.single.price, 100000);
      expect(order.items.single.quantity, 1);

      // Shipping proof: tracking_number + proof_type → UI reference vocabulary.
      expect(order.shippingInfo.trackingNumber, 'RESI-123');
      expect(order.shippingInfo.referenceType, 'phone');
      expect(order.shippingInfo.shippingNote, 'diantar malam ini');

      // Canonical completion timestamp.
      expect(order.completedAt, isNotNull);
    });
  });
}
