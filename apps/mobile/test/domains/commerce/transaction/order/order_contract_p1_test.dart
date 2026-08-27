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
  'product_id': 'p1',
  'status': 'paid',
  'source_type': 'fixed_price_sale',
  'source_id': 'p1',
  'subtotal': 10000,
  'shipping_total': 1000,
  'commission_amount': 500,
  'escrow_amount': 11500,
  'created_at': '2026-06-01T00:00:00Z',
  'updated_at': '2026-06-01T00:00:00Z',
};

class _FakeDatasource implements OrderRemoteDatasource {
  final List<String> calls = [];
  String? lastOrderId;
  OrderApiException? failWith;

  void _maybeThrow() {
    if (failWith != null) throw failWith!;
  }

  @override
  Future<OrderDto> cancelOrder(String orderId) async {
    calls.add('cancel');
    lastOrderId = orderId;
    _maybeThrow();
    return OrderDto.fromJson({'id': orderId});
  }

  @override
  Future<CheckDeliveryDto> checkDelivery(
    CheckDeliveryRequestDto request,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderDto> completeOrder(String orderId) async {
    calls.add('complete');
    lastOrderId = orderId;
    _maybeThrow();
    return OrderDto.fromJson({'id': orderId});
  }

  @override
  Future<OrderConfirmationDto> completeConfirmation(
    String orderId,
    String status,
    String completionReason,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderDto> confirmOrder(String orderId) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderDto> createOrder(CreateOrderDto request) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<DisputeDto> createDispute(
    String orderId,
    CreateDisputeDto request,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<DisputeDto> adminApproveDispute(
    String disputeId,
    AdminDisputeResolutionDto request,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<DisputeDto> adminRejectDispute(
    String disputeId,
    AdminDisputeResolutionDto request,
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
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderConfirmationDto> extendConfirmation(
    String orderId,
    DateTime newEndDate,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderConfirmationDto> getConfirmation(String orderId) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<DisputeDto> getDispute(String disputeId) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderDto> getOrder(String orderId) async {
    calls.add('get-order');
    return OrderDto.fromJson(_orderPayload(orderId));
  }

  @override
  Future<OrderDto> getOrderByNumber(String orderNumber) async {
    throw UnsupportedError('not used');
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
  Future<ShippingProofDto> getShippingProof(String orderId) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<DisputeListDto> listAdminDisputes({
    DisputeFilterParams? params,
  }) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<RefundListDto> listMyRefunds({RefundFilterParams? params}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderListDto> listMyOrders({OrderFilterParams? params}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<RefundListDto> listSellerRefunds({RefundFilterParams? params}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderListDto> listSellerOrders({OrderFilterParams? params}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderDto> shipOrder(String orderId, MarkAsShippedParams params) async {
    calls.add('ship');
    lastOrderId = orderId;
    _maybeThrow();
    return OrderDto.fromJson({'id': orderId});
  }

  @override
  Future<OrderStatsDto> getOrderStats({bool asSeller = false}) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<PreviewOrderResponseDto> previewOrder(
    PreviewOrderRequestDto request,
  ) async {
    throw UnsupportedError('not used');
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

  @override
  Future<ShippingProofDto> updateShippingProof(
    String orderId,
    CreateShippingProofDto request,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<ShippingProofDto> uploadShippingProof(
    String orderId,
    CreateShippingProofDto request,
  ) async {
    throw UnsupportedError('not used');
  }

  @override
  Future<OrderDto> updateOrderStatus(
    String orderId,
    UpdateOrderStatusDto request,
  ) async {
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

    test('unsupported endpoints throw UnsupportedError', () async {
      final client = _RecordingApiClient();
      final ds = OrderApiDatasourceImpl(client);

      expect(
        () => ds.previewOrder(
          const PreviewOrderRequestDto(
            productId: 'p1',
            quantity: 1,
            shippingAddress: ShippingAddressRequestDto(
              recipientName: 'r',
              phoneNumber: 'p',
              addressLine1: 'a',
            ),
          ),
        ),
        throwsA(isA<UnsupportedError>()),
      );
      expect(() => ds.getOrderByNumber('n1'), throwsA(isA<UnsupportedError>()));
      expect(() => ds.getOrderStats(), throwsA(isA<UnsupportedError>()));
      expect(() => ds.confirmOrder('o1'), throwsA(isA<UnsupportedError>()));
      expect(
        () => ds.getRefundByOrderId('o1'),
        throwsA(isA<UnsupportedError>()),
      );
      expect(() => ds.getRefund('r1'), throwsA(isA<UnsupportedError>()));
      expect(() => ds.listMyRefunds(), throwsA(isA<UnsupportedError>()));
      expect(() => ds.listSellerRefunds(), throwsA(isA<UnsupportedError>()));
      expect(
        () => ds.uploadShippingProof(
          'o1',
          CreateShippingProofDto(trackingNumber: 't'),
        ),
        throwsA(isA<UnsupportedError>()),
      );
      expect(() => ds.getShippingProof('o1'), throwsA(isA<UnsupportedError>()));
      expect(
        () => ds.updateShippingProof(
          'o1',
          CreateShippingProofDto(trackingNumber: 't'),
        ),
        throwsA(isA<UnsupportedError>()),
      );
      expect(() => ds.getConfirmation('o1'), throwsA(isA<UnsupportedError>()));
      expect(
        () => ds.extendConfirmation('o1', DateTime.now()),
        throwsA(isA<UnsupportedError>()),
      );
      expect(
        () => ds.completeConfirmation('o1', 'x', 'x'),
        throwsA(isA<UnsupportedError>()),
      );
      expect(() => ds.getDispute('d1'), throwsA(isA<UnsupportedError>()));
      expect(() => ds.listAdminDisputes(), throwsA(isA<UnsupportedError>()));
      expect(
        () => ds.adminApproveDispute('d1', AdminDisputeResolutionDto()),
        throwsA(isA<UnsupportedError>()),
      );
      expect(
        () => ds.adminRejectDispute('d1', AdminDisputeResolutionDto()),
        throwsA(isA<UnsupportedError>()),
      );
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

        await repo.completeOrder('o1');
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

      final result = await repo.completeOrder('o1');
      expect(result.isError, isTrue);
      expect(result.errorCode, 'ACCOUNT_SUSPENDED');
      expect(result.errorDetails?['reason'], 'suspended');
    });
  });
}
