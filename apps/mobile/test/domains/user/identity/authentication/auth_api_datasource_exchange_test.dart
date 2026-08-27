import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/domains/user/identity/authentication/data/datasources/auth_api_datasource.dart';

class _RecordingApiClient implements ApiClient {
  String? lastPath;
  dynamic lastData;
  Options? lastOptions;

  @override
  Dio get dio => throw UnimplementedError();

  @override
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) {
    throw UnimplementedError();
  }

  @override
  ApiException extractException(DioException e) =>
      UnknownApiException(message: e.message ?? 'unknown');

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) {
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
  Future<Response<T>> patch<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPath = path;
    lastData = data;
    lastOptions = options;
    final responseData = path == '/auth/complete-profile'
        ? {
            'success': true,
            'data': {
              'user_id': 'user-1',
              'access_token': 'platform-access-token',
              'refresh_token': 'platform-refresh-token',
              'expires_at': '2026-07-19T00:00:00Z',
              'refresh_expires_at': '2026-07-20T00:00:00Z',
              'requires_profile_completion': false,
              'created': true,
            },
          }
        : {
            'success': true,
            'data': {
              'user_id': 'user-1',
              'access_token': 'restricted-token',
              'expires_at': '2026-07-19T00:00:00Z',
              'requires_profile_completion': true,
              'created': false,
            },
          };
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      statusCode: 200,
      data: responseData as T,
    );
  }

  @override
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) {
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
  }) {
    throw UnimplementedError();
  }
}

void main() {
  test(
    'exchangeFirebaseSession posts only the Firebase ID token with skipAuth',
    () async {
      final client = _RecordingApiClient();
      final datasource = AuthApiDatasource(client);

      final result = await datasource.exchangeFirebaseSession(
        firebaseIdToken: 'firebase-id-token-123',
      );

      expect(result.isSuccess, isTrue);
      expect(client.lastPath, equals('/auth/firebase/exchange'));
      expect(
        client.lastData,
        equals({'firebase_id_token': 'firebase-id-token-123'}),
      );
      expect(client.lastOptions, isNotNull);
      final extra = client.lastOptions!.extra;
      expect(extra, isNotNull);
      expect(extra!['skipAuth'], isTrue);
    },
  );

  test(
    'completeProfile posts username to /auth/complete-profile with restricted bearer and skipAuth',
    () async {
      final client = _RecordingApiClient();
      final datasource = AuthApiDatasource(client);

      final result = await datasource.completeProfile(
        username: 'seeded_username',
        restrictedToken: 'restricted-token-abc',
      );

      expect(result.isSuccess, isTrue);
      expect(client.lastPath, equals('/auth/complete-profile'));
      expect(client.lastData, equals({'username': 'seeded_username'}));
      expect(client.lastOptions, isNotNull);
      final extra = client.lastOptions!.extra;
      expect(extra, isNotNull);
      expect(extra!['skipAuth'], isTrue);
      expect(
        client.lastOptions!.headers?['Authorization'],
        equals('Bearer restricted-token-abc'),
      );
    },
  );
}
