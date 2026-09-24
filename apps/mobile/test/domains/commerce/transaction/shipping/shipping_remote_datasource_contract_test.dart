import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/remote/shipping_remote_datasource.dart';

class _RecordingApiClient implements ApiClient {
  String? lastGetPath;
  String? lastPostPath;
  String? lastPutPath;
  Map<String, dynamic>? lastGetQuery;
  dynamic lastPostData;
  dynamic lastPutData;

  dynamic getPayload = const {'success': true, 'data': {}};
  dynamic postPayload = const {'success': true, 'data': {}};
  dynamic putPayload = const {'success': true, 'data': {}};
  dynamic deletePayload = const {'success': true, 'data': null};

  @override
  Dio get dio => throw UnimplementedError();

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
    lastPutData = data;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: putPayload as T,
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> patch<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: putPayload as T,
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> delete<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: deletePayload as T,
      statusCode: 200,
    );
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
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as T,
      statusCode: 200,
    );
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
}

Map<String, dynamic> _shippingSetupJson({
  required String id,
  required String name,
  required bool isActive,
}) {
  return {
    'id': id,
    'name': name,
    'transport_type': 'train',
    'is_active': isActive,
    'created_at': '2026-01-01T00:00:00Z',
    'updated_at': '2026-01-01T00:00:00Z',
  };
}

Map<String, dynamic> _coverageJson({
  required String id,
  required String shippingSetupId,
  required String provinceCode,
  required String provinceName,
  required int rate,
  required bool isAvailable,
}) {
  return {
    'id': id,
    'shipping_option_id': shippingSetupId,
    'province_code': provinceCode,
    'province_name': provinceName,
    'rate': rate,
    'is_available': isAvailable,
    'created_at': '2026-01-01T00:00:00Z',
  };
}

void main() {
  group('ShippingRemoteDatasource contract', () {
    test(
      'listMyShippingSetups parses the wrapped shipping_options envelope',
      () async {
        final client = _RecordingApiClient()
          ..getPayload = {
            'success': true,
            'data': {
              'shipping_options': [
                _shippingSetupJson(
                  id: 'so-1',
                  name: 'JNE Reguler',
                  isActive: true,
                ),
              ],
              'count': 1,
            },
            'timestamp': '2026-01-01T00:00:00Z',
          };
        final ds = ShippingRemoteDatasource(client);

        final options = await ds.listMyShippingSetups();

        expect(client.lastGetPath, '/seller/shipping/options');
        expect(client.lastGetQuery, {'include_inactive': true});
        expect(options, hasLength(1));
        expect(options.first.id, 'so-1');
        expect(options.first.name, 'JNE Reguler');
        expect(options.first.type, 'train');
        expect(options.first.isActive, isTrue);
      },
    );

    test(
      'listMyActiveShippingSetups flips include_inactive to false',
      () async {
        final client = _RecordingApiClient()
          ..getPayload = {
            'success': true,
            'data': {'shipping_options': const [], 'count': 0},
            'timestamp': '2026-01-01T00:00:00Z',
          };
        final ds = ShippingRemoteDatasource(client);

        final options = await ds.listMyActiveShippingSetups();

        expect(client.lastGetQuery, {'include_inactive': false});
        expect(options, isEmpty);
      },
    );

    test(
      'createShippingSetup parses the nested shipping_option object',
      () async {
        final client = _RecordingApiClient()
          ..postPayload = {
            'success': true,
            'data': {
              'shipping_option':              _shippingSetupJson(
                id: 'so-2',
                name: 'Bus Handoyo',
                isActive: true,
              ),
            },
            'timestamp': '2026-01-01T00:00:00Z',
          };
        final ds = ShippingRemoteDatasource(client);

        // ONE-PACKAGE contract: identity + destinations in one request.
        final option = await ds.createShippingSetup({
          'name': 'Bus Handoyo',
          'transport_type': 'train',
          'internal_purpose': '',
          'destinations': [
            {
              'province_code': '31',
              'province_name': 'DKI Jakarta',
              'rate': 150000,
              'is_available': true,
              'city_qualifications': <Map<String, dynamic>>[],
            },
          ],
        });

        expect(client.lastPostPath, '/seller/shipping/options');
        expect(option.id, 'so-2');
        expect(option.name, 'Bus Handoyo');
      },
    );

    test(
      'getShippingSetup parses seller-private note and coverages',
      () async {
        final client = _RecordingApiClient()
          ..getPayload = {
            'success': true,
            'data': {
              'shipping_option': {
                ..._shippingSetupJson(
                  id: 'so-1',
                  name: 'Bus Kencana',
                  isActive: true,
                ),
                'internal_purpose': 'kantong besar, untuk 10 ekor',
              },
              'coverages': [
                {
                  ..._coverageJson(
                    id: 'cov-1',
                    shippingSetupId: 'so-1',
                    provinceCode: '31',
                    provinceName: 'DKI Jakarta',
                    rate: 150000,
                    isAvailable: true,
                  ),
                  'city_qualifications': [
                    {
                      'id': 'cq-1',
                      'city_code': '3171',
                      'city_name': 'Jakarta Pusat',
                      'rate': 165000,
                    },
                  ],
                },
              ],
              'coverage_count': 1,
            },
            'timestamp': '2026-01-01T00:00:00Z',
          };
        final ds = ShippingRemoteDatasource(client);

        final option = await ds.getShippingSetup('so-1');

        expect(client.lastGetPath, '/seller/shipping/options/so-1');
        expect(option.internalPurpose, 'kantong besar, untuk 10 ekor');
        expect(option.coverages, hasLength(1));
        expect(option.coverages!.first.cityQualifications, hasLength(1));
        expect(
          option.coverages!.first.cityQualifications.first.cityCode,
          '3171',
        );
        expect(
          option.coverages!.first.cityQualifications.first.rate,
          165000,
        );
      },
    );

    test(
      'rejects a bare list response instead of silently casting it',
      () async {
        final client = _RecordingApiClient()
          ..getPayload = {
            'success': true,
            'data': [
              _shippingSetupJson(
                id: 'so-1',
                name: 'JNE Reguler',
                isActive: true,
              ),
            ],
            'timestamp': '2026-01-01T00:00:00Z',
          };
        final ds = ShippingRemoteDatasource(client);

        expect(
          () => ds.listMyShippingSetups(),
          throwsA(isA<FormatException>()),
        );
      },
    );
  });
}
