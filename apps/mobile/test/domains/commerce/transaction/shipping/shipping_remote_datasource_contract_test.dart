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
  bool isNetworkError(DioException e) => false;

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
    'shipping_setup_id': shippingSetupId,
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
                name: 'J&T Express',
                isActive: true,
              ),
            },
            'timestamp': '2026-01-01T00:00:00Z',
          };
        final ds = ShippingRemoteDatasource(client);

        final option = await ds.createShippingSetup({
          'name': 'J&T Express',
          'transport_type': 'train',
        });

        expect(client.lastPostPath, '/seller/shipping/options');
        expect(option.id, 'so-2');
        expect(option.name, 'J&T Express');
      },
    );

    test('addCoverage parses the nested coverage object', () async {
      final client = _RecordingApiClient()
        ..postPayload = {
          'success': true,
          'data': {
            'coverage': _coverageJson(
              id: 'cov-1',
              shippingSetupId: 'so-1',
              provinceCode: '31',
              provinceName: 'DKI Jakarta',
              rate: 150000,
              isAvailable: true,
            ),
          },
          'timestamp': '2026-01-01T00:00:00Z',
        };
      final ds = ShippingRemoteDatasource(client);

      final coverage = await ds.addCoverage('so-1', {
        'province_code': '31',
        'province_name': 'DKI Jakarta',
        'rate': 150000,
        'is_available': true,
      });

      expect(client.lastPostPath, '/seller/shipping/options/so-1/coverages');
      expect(coverage.id, 'cov-1');
      expect(coverage.shippingSetupId, 'so-1');
      expect(coverage.provinceCode, '31');
      expect(coverage.rate, 150000);
      expect(coverage.isAvailable, isTrue);
    });

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
