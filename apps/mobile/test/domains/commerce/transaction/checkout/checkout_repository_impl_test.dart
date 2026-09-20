import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/domains/commerce/transaction/checkout/data/repositories/checkout_repository_impl.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_request.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_response.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/usecases/create_order_usecase.dart';

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
  group('CheckoutRepositoryImpl', () {
    test('sends distinct product and sale surface ids', () async {
      const productId = '11111111-1111-1111-1111-111111111111';
      const forSaleId = '22222222-2222-2222-2222-222222222222';

      final apiClient = _RecordingApiClient();
      final repository = CheckoutRepositoryImpl(apiClient);
      final request = CheckoutRequest(
        productId: productId,
        forSaleId: forSaleId,
        addressId: '33333333-3333-3333-3333-333333333333',
        pricingToken: '44444444-4444-4444-4444-444444444444',
      );

      await repository.createOrder(request);

      expect(apiClient.lastPostPath, '/orders');
      final payload = apiClient.lastPostData!;
      expect(payload['product_id'], productId);
      expect(payload['source_type'], 'for_sale');
      expect(payload['source_id'], forSaleId);
      expect(payload.containsKey('for_sale_id'), isFalse);
      expect(payload['source_id'], isNot(equals(productId)));
      expect(payload['product_id'], isNot(equals(forSaleId)));
    });

    test(
      'sends auction source identity when auction checkout is used',
      () async {
        const productId = '11111111-1111-1111-1111-111111111111';
        const forSaleId = '22222222-2222-2222-2222-222222222222';
        const auctionId = '33333333-3333-3333-3333-333333333333';

        final apiClient = _RecordingApiClient();
        final repository = CheckoutRepositoryImpl(apiClient);
        final request = CheckoutRequest(
          productId: productId,
          forSaleId: forSaleId,
          addressId: '44444444-4444-4444-4444-444444444444',
          pricingToken: '55555555-5555-5555-5555-555555555555',
          auctionId: auctionId,
        );

        await repository.createOrder(request);

        expect(apiClient.lastPostPath, '/orders');
        final payload = apiClient.lastPostData!;
        expect(payload['product_id'], productId);
        expect(payload['source_type'], 'auction');
        expect(payload['source_id'], auctionId);
        expect(payload['source_id'], isNot(equals(forSaleId)));
      },
    );

    test('rejects missing product id instead of reusing sale id', () async {
      final apiClient = _RecordingApiClient();
      final repository = CheckoutRepositoryImpl(apiClient);
      final request = CheckoutRequest(
        forSaleId: '22222222-2222-2222-2222-222222222222',
        addressId: '33333333-3333-3333-3333-333333333333',
        pricingToken: '44444444-4444-4444-4444-444444444444',
      );

      await expectLater(
        () => repository.createOrder(request),
        throwsA(isA<CheckoutException>()),
      );
      expect(apiClient.lastPostPath, isNull);
      expect(apiClient.lastPostData, isNull);
    });
  });

  // ========================================================================
  // STAGE 14 — Source type contract proof
  // ========================================================================
  group('POST /orders source_type contract', () {
    test('for_sale: source_type is "for_sale", not "fixed_price_sale"', () async {
      final apiClient = _RecordingApiClient();
      final repository = CheckoutRepositoryImpl(apiClient);
      final request = CheckoutRequest(
        productId: '11111111-1111-1111-1111-111111111111',
        forSaleId: '22222222-2222-2222-2222-222222222222',
        addressId: '33333333-3333-3333-3333-333333333333',
        pricingToken: '44444444-4444-4444-4444-444444444444',
      );

      await repository.createOrder(request);

      final payload = apiClient.lastPostData!;
      expect(payload['source_type'], 'for_sale');
      expect(payload.containsKey('fixed_price_sale'), isFalse,
        reason: 'fixed_price_sale is obsolete; backend rejects it',
      );
    });

    test('auction: source_type is "auction"', () async {
      final apiClient = _RecordingApiClient();
      final repository = CheckoutRepositoryImpl(apiClient);
      final request = CheckoutRequest(
        productId: '11111111-1111-1111-1111-111111111111',
        forSaleId: '22222222-2222-2222-2222-222222222222',
        addressId: '33333333-3333-3333-3333-333333333333',
        pricingToken: '55555555-5555-5555-5555-555555555555',
        auctionId: '66666666-6666-6666-6666-666666666666',
      );

      await repository.createOrder(request);

      final payload = apiClient.lastPostData!;
      expect(payload['source_type'], 'auction');
    });

    test('no legacy fixed_price_sale on any live order path', () async {
      final apiClient = _RecordingApiClient();
      final repository = CheckoutRepositoryImpl(apiClient);

      // For Sale path
      final forSaleRequest = CheckoutRequest(
        productId: '11111111-1111-1111-1111-111111111111',
        forSaleId: '22222222-2222-2222-2222-222222222222',
        addressId: '33333333-3333-3333-3333-333333333333',
        pricingToken: '44444444-4444-4444-4444-444444444444',
      );
      await repository.createOrder(forSaleRequest);
      expect(apiClient.lastPostData!['source_type'], isNot('fixed_price_sale'));

      // Auction path
      final auctionRequest = CheckoutRequest(
        productId: '11111111-1111-1111-1111-111111111111',
        forSaleId: '22222222-2222-2222-2222-222222222222',
        addressId: '33333333-3333-3333-3333-333333333333',
        pricingToken: '55555555-5555-5555-5555-555555555555',
        auctionId: '66666666-6666-6666-6666-666666666666',
      );
      await repository.createOrder(auctionRequest);
      expect(apiClient.lastPostData!['source_type'], isNot('fixed_price_sale'));
    });
  });

  group('CreateOrderUseCase', () {
    test('rejects missing product id without calling repository', () async {
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
