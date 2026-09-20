import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/dto/shipping_dto.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/mappers/shipping_mapper.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/remote/shipping_remote_datasource.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';

// Canonical distinct UUIDs for ID-confusion proof.
const _productId = '11111111-1111-1111-1111-111111111111';
const _auctionId = '22222222-2222-2222-2222-222222222222';
const _fixedPriceSaleId = '33333333-3333-3333-3333-333333333333';
const _provinceCode = '31';
const _cityCode = '3171';

/// Canonical backend payload for POST /api/v1/shipping/check.
///
/// Copied from the current producer:
/// backend/internal/commerce/shipping/delivery/http/shipping_handler.go
/// (`response.Success(c, gin.H{...})`), including the empty-options case.
Map<String, dynamic> _canonicalCheckDeliveryEnvelope({
  List<Map<String, dynamic>>? options,
  bool productConfigured = true,
  String city = _cityCode,
}) {
  final resolvedOptions =
      options ??
      [
        {
          'shipping_option_id': 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
          'name': 'JNE Reguler',
          'transport_type': 'train',
          'rate': 15000,
          'is_available': true,
        },
        {
          'shipping_option_id': 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
          'name': 'SiCepat',
          'transport_type': 'bus',
          'rate': 18000,
          'is_available': true,
        },
      ];

  return {
    'success': true,
    'data': {
      'product_id': _productId,
      'province': _provinceCode,
      'city': city,
      'options': resolvedOptions,
      'count': resolvedOptions.length,
      'product_configured': productConfigured,
    },
    'timestamp': '2026-09-16T00:00:00Z',
  };
}

class _RecordingApiClient implements ApiClient {
  String? lastPostPath;
  Map<String, dynamic>? lastPostData;
  dynamic postPayload = <String, dynamic>{'success': true, 'data': {}};

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
  bool isNotFound(DioException e) => false;

  @override
  bool isUnauthorized(DioException e) => false;

  @override
  bool isValidationError(DioException e) => false;

  @override
  Dio get dio => throw UnimplementedError();
}

CheckDeliveryRequest _request({String productId = _productId}) {
  return CheckDeliveryRequest(
    productId: productId,
    provinceId: _provinceCode,
    cityId: _cityCode,
  );
}

void main() {
  group('POST /shipping/check request wire contract', () {
    test('serializes the canonical product_id, province_code and city_code', () {
      final json = DeliveryOptionMapper.checkDeliveryToJson(_request());

      expect(json['product_id'], _productId);
      expect(json['province_code'], _provinceCode);
      expect(json['city_code'], _cityCode);
      // Exactly the canonical triple — no extra keys travel on the wire.
      expect(json.keys.toSet(), {'product_id', 'province_code', 'city_code'});
    });

    test('never emits the stale province_id / city_id / city_name keys', () {
      final json = DeliveryOptionMapper.checkDeliveryToJson(_request());

      expect(json.containsKey('province_id'), isFalse);
      expect(json.containsKey('city_id'), isFalse);
      expect(json.containsKey('city_name'), isFalse);
    });

    test('auction delivery check uses productId — never auctionId', () {
      final json = DeliveryOptionMapper.checkDeliveryToJson(
        _request(productId: _productId),
      );

      expect(json['product_id'], _productId);
      expect(json['product_id'], isNot(equals(_auctionId)));
      expect(json.containsKey('auction_id'), isFalse);
      expect(json.containsKey('source_id'), isFalse);
      expect(json.containsKey('for_sale_id'), isFalse);
    });

    test('FPS delivery check uses productId — never fixedPriceSaleId', () {
      final json = DeliveryOptionMapper.checkDeliveryToJson(
        _request(productId: _productId),
      );

      expect(json['product_id'], _productId);
      expect(json['product_id'], isNot(equals(_fixedPriceSaleId)));
      expect(json.containsKey('fixed_price_sale_id'), isFalse);
    });

    test('productId, auctionId and fixedPriceSaleId are distinct values', () {
      expect(_productId, isNot(equals(_auctionId)));
      expect(_productId, isNot(equals(_fixedPriceSaleId)));
      expect(_auctionId, isNot(equals(_fixedPriceSaleId)));
    });

    test('datasource posts the canonical body to /shipping/check', () async {
      final client = _RecordingApiClient()
        ..postPayload = _canonicalCheckDeliveryEnvelope();
      final datasource = ShippingRemoteDatasource(client);

      await datasource.checkDeliveryAvailability(
        DeliveryOptionMapper.checkDeliveryToJson(_request()),
      );

      expect(client.lastPostPath, '/shipping/check');
      final payload = client.lastPostData!;
      expect(payload['product_id'], _productId);
      expect(payload['province_code'], _provinceCode);
      expect(payload['city_code'], _cityCode);
      expect(payload.containsKey('city_name'), isFalse);
    });
  });

  group('POST /shipping/check response wire contract', () {
    test('parses the canonical backend envelope', () async {
      final client = _RecordingApiClient()
        ..postPayload = _canonicalCheckDeliveryEnvelope();
      final datasource = ShippingRemoteDatasource(client);

      final dto = await datasource.checkDeliveryAvailability(
        DeliveryOptionMapper.checkDeliveryToJson(_request()),
      );

      expect(dto.productId, _productId);
      expect(dto.province, _provinceCode);
      expect(dto.city, _cityCode);
      expect(dto.count, 2);
      expect(dto.productConfigured, isTrue);
      expect(dto.options, hasLength(2));

      final first = dto.options.first;
      expect(first.shippingOptionId, 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa');
      expect(first.name, 'JNE Reguler');
      expect(first.transportType, 'train');
      expect(first.rate, 15000);
      expect(first.isAvailable, isTrue);

      final second = dto.options.last;
      expect(second.shippingOptionId, 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb');
      expect(second.name, 'SiCepat');
      expect(second.transportType, 'bus');
      expect(second.rate, 18000);
      expect(second.isAvailable, isTrue);
    });

    test('maps wire option keys onto the shipping domain entity', () async {
      final client = _RecordingApiClient()
        ..postPayload = _canonicalCheckDeliveryEnvelope();
      final datasource = ShippingRemoteDatasource(client);

      final dto = await datasource.checkDeliveryAvailability(
        DeliveryOptionMapper.checkDeliveryToJson(_request()),
      );
      final entities = DeliveryOptionMapper.toEntityList(dto.options);

      expect(entities, hasLength(2));
      expect(
        entities.first.shippingSetupId,
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
      );
      expect(entities.first.displayName, 'JNE Reguler');
      expect(entities.first.type, 'train');
      expect(entities.first.rate, 15000);
    });

    test(
      'parses an uncovered address: empty options with product_configured true',
      () async {
        final client = _RecordingApiClient()
          ..postPayload = _canonicalCheckDeliveryEnvelope(
            options: const [],
            productConfigured: true,
            city: '',
          );
        final datasource = ShippingRemoteDatasource(client);

        final dto = await datasource.checkDeliveryAvailability(
          DeliveryOptionMapper.checkDeliveryToJson(_request()),
        );

        expect(dto.options, isEmpty);
        expect(dto.count, 0);
        expect(dto.productConfigured, isTrue);
        expect(dto.city, '');
        expect(DeliveryOptionMapper.toEntityList(dto.options), isEmpty);
      },
    );

    test('parses an unconfigured seller: no links, empty options', () async {
      final client = _RecordingApiClient()
        ..postPayload = _canonicalCheckDeliveryEnvelope(
          options: const [],
          productConfigured: false,
        );
      final datasource = ShippingRemoteDatasource(client);

      final dto = await datasource.checkDeliveryAvailability(
        DeliveryOptionMapper.checkDeliveryToJson(_request()),
      );

      expect(dto.options, isEmpty);
      expect(dto.count, 0);
      expect(dto.productConfigured, isFalse);
    });
  });

  group('stale wire vocabulary guard', () {
    test('a stale-shaped option payload is rejected instead of tolerated', () {
      final staleData =
          _canonicalCheckDeliveryEnvelope()['data'] as Map<String, dynamic>;
      final withStaleOption = {
        ...staleData,
        'available': true,
        'options': [
          {
            'shipping_setup_id': 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
            'display_name': 'JNE Reguler',
            'type': 'train',
            'rate': 15000,
            'source': 'backend',
          },
        ],
      };

      expect(
        () => CheckDeliveryResponseDto.fromJson(withStaleOption),
        throwsA(isA<TypeError>()),
      );
    });

    test('the removed `available` flag is never used as a fallback', () {
      final canonical =
          _canonicalCheckDeliveryEnvelope()['data'] as Map<String, dynamic>;
      final withoutConfigured = Map<String, dynamic>.from(canonical)
        ..remove('product_configured');

      expect(
        () => CheckDeliveryResponseDto.fromJson(withoutConfigured),
        throwsA(isA<TypeError>()),
      );

      final availableOnly = <String, dynamic>{
        'available': true,
        'options': const [],
      };

      expect(
        () => CheckDeliveryResponseDto.fromJson(availableOnly),
        throwsA(isA<TypeError>()),
      );
    });
  });
}
