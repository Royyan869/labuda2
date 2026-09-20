import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/domains/commerce/transaction/checkout/data/repositories/checkout_repository_impl.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_request.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_response.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/usecases/create_order_usecase.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';

class _RecordingApiClient implements ApiClient {
  String? lastPostPath;
  Map<String, dynamic>? lastPostData;
  dynamic postPayload = <String, dynamic>{
    'success': true,
    'data': <String, dynamic>{'id': 'order-123'},
  };

  @override
  Future<Response<T>> post<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPostPath = path;
    lastPostData = data as Map<String, dynamic>?;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as T,
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Response<T>> put<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Response<T>> patch<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Response<T>> delete<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int, int)? onSendProgress,
  }) async {
    throw UnimplementedError();
  }

  @override
  ApiException extractException(DioException e) {
    throw UnimplementedError();
  }

  @override
  bool isNetworkError(DioException e) => false;

  @override
  bool isNotFound(DioException e) => false;

  @override
  bool isUnauthorized(DioException e) => false;

  @override
  bool isValidationError(DioException e) => false;

  @override
  Dio get dio => throw UnimplementedError();
}

class _FailingCheckoutRepository implements CheckoutRepository {
  bool called = false;

  @override
  Future<CheckoutResponse> createOrder(
    CheckoutRequest request, {
    String? idempotencyKey,
  }) async {
    called = true;
    throw StateError('createOrder should not be called for invalid input');
  }
}

void main() {
  group('Checkout shipping fallback contract', () {
    test('forSale checkout preserves product, source, and shipping option ids',
        () async {
      const productId = '11111111-1111-1111-1111-111111111111';
      const forSaleId = '22222222-2222-2222-2222-222222222222';
      const shippingSetupId = 'ship-1';

      final apiClient = _RecordingApiClient();
      final repository = CheckoutRepositoryImpl(apiClient);
      final request = CheckoutRequest(
        productId: productId,
        forSaleId: forSaleId,
        addressId: '33333333-3333-3333-3333-333333333333',
        pricingToken: '44444444-4444-4444-4444-444444444444',
        shippingOptionId: shippingSetupId,
      );

      await repository.createOrder(request);

      expect(apiClient.lastPostPath, '/orders');
      final payload = apiClient.lastPostData!;
      expect(payload['product_id'], productId);
      expect(payload['source_type'], 'for_sale');
      expect(payload['source_id'], forSaleId);
      // Canonical backend key for the selected shipping option.
      expect(payload['shipping_option_id'], shippingSetupId);
      expect(payload.containsKey('shipping_setup_id'), isFalse);
      expect(payload['shipping_quote_id'], isNull);
    });

    test(
      'selected delivery option id travels as shipping_option_id unchanged',
      () async {
        // Value-trace proof: the id the buyer picked in the shipping picker is the
        // id that reaches POST /orders (DeliveryOption -> CheckoutRequest -> wire).
        const selectedOption = DeliveryOption(
          shippingSetupId: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
          displayName: 'Kurir Kereta',
          type: 'train',
          rate: 15000,
        );

        final apiClient = _RecordingApiClient();
        final repository = CheckoutRepositoryImpl(apiClient);
        final request = CheckoutRequest(
          productId: '11111111-1111-1111-1111-111111111111',
          forSaleId: '22222222-2222-2222-2222-222222222222',
          addressId: '33333333-3333-3333-3333-333333333333',
          pricingToken: '44444444-4444-4444-4444-444444444444',
          shippingOptionId: selectedOption.shippingSetupId,
        );

        await repository.createOrder(request);

        final payload = apiClient.lastPostData!;
        expect(
          payload['shipping_option_id'],
          selectedOption.shippingSetupId,
        );
        expect(payload.containsKey('shipping_setup_id'), isFalse);
        expect(payload['shipping_option_id'], isNot(equals(request.productId)));
      },
    );

    test('auction checkout preserves product, source, and shipping quote ids',
        () async {
      const productId = '11111111-1111-1111-1111-111111111111';
      const forSaleId = '22222222-2222-2222-2222-222222222222';
      const auctionId = '33333333-3333-3333-3333-333333333333';
      const shippingQuoteId = 'quote-1';

      final apiClient = _RecordingApiClient();
      final repository = CheckoutRepositoryImpl(apiClient);
      final request = CheckoutRequest(
        productId: productId,
        forSaleId: forSaleId,
        addressId: '44444444-4444-4444-4444-444444444444',
        pricingToken: '55555555-5555-5555-5555-555555555555',
        auctionId: auctionId,
        shippingQuoteId: shippingQuoteId,
      );

      await repository.createOrder(request);

      expect(apiClient.lastPostPath, '/orders');
      final payload = apiClient.lastPostData!;
      expect(payload['product_id'], productId);
      expect(payload['source_type'], 'auction');
      expect(payload['source_id'], auctionId);
      expect(payload['shipping_quote_id'], shippingQuoteId);
      // Quote mode: no shipping option key is sent at all.
      expect(payload.containsKey('shipping_option_id'), isFalse);
      expect(payload.containsKey('shipping_setup_id'), isFalse);
    });

    test('missing product id is rejected before order creation', () async {
      final repository = _FailingCheckoutRepository();
      final useCase = CreateOrderUseCase(repository);
      final request = CheckoutRequest(
        forSaleId: '22222222-2222-2222-2222-222222222222',
        addressId: '33333333-3333-3333-3333-333333333333',
        pricingToken: '44444444-4444-4444-4444-444444444444',
      );

      final result = await useCase(request);

      expect(result.isError, isTrue);
      expect(result.error, contains('ID produk tidak valid'));
      expect(repository.called, isFalse);
    });
  });
}
