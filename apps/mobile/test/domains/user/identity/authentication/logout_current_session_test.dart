import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/mockito.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/user/identity/authentication/data/datasources/auth_api_datasource.dart';
import 'package:labuda/domains/user/identity/authentication/data/repositories/auth_repository_impl.dart';
import 'package:labuda/domains/user/identity/authentication/data/repositories/auth_profile_repository.dart';

class RecordingApiClient implements ApiClient {
  String? lastPath;
  dynamic lastData;
  Map<String, dynamic>? lastQueryParameters;

  @override
  Dio get dio => throw UnimplementedError();

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
    lastQueryParameters = queryParameters;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      statusCode: 200,
      data:
          {
                'success': true,
                'data': {'ok': true},
              }
              as T,
    );
  }

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
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int p1, int p2)? onSendProgress,
  }) {
    throw UnimplementedError();
  }

  @override
  ApiException extractException(DioException e) {
    return UnknownApiException(message: e.message ?? 'Unknown error');
  }

  @override
  bool isNotFound(DioException e) => false;

  @override
  bool isNetworkError(DioException e) => false;

  @override
  bool isUnauthorized(DioException e) => false;

  @override
  bool isValidationError(DioException e) => false;
}

class RecordingAuthApiDatasource extends AuthApiDatasource {
  RecordingAuthApiDatasource(super.apiClient);

  bool called = false;
  String? refreshToken;
  String? fcmToken;
  String? deviceId;
  Result<void> result = Result.success(null);
  bool logoutAllCalled = false;
  bool? deactivateFcmTokens;
  Result<void> logoutAllResult = Result.success(null);

  @override
  Future<Result<void>> logoutCurrentSession({
    required String refreshToken,
    String? fcmToken,
    String? deviceId,
  }) async {
    called = true;
    this.refreshToken = refreshToken;
    this.fcmToken = fcmToken;
    this.deviceId = deviceId;
    return result;
  }

  @override
  Future<Result<void>> logoutAllSessions({
    bool deactivateFcmTokens = true,
  }) async {
    logoutAllCalled = true;
    this.deactivateFcmTokens = deactivateFcmTokens;
    return logoutAllResult;
  }
}

class MockFirebaseAuth extends Mock implements FirebaseAuth {
  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();
}

void main() {
  test(
    'logoutCurrentSession posts refresh_token and optional device fields',
    () async {
      final client = RecordingApiClient();
      final datasource = AuthApiDatasource(client);

      final result = await datasource.logoutCurrentSession(
        refreshToken: 'refresh-token-123',
        fcmToken: 'fcm-token-abc',
        deviceId: 'device-xyz',
      );

      expect(result.isSuccess, isTrue);
      expect(client.lastPath, equals('/auth/logout'));
      expect(client.lastData, isA<Map<String, dynamic>>());

      final body = client.lastData as Map<String, dynamic>;
      expect(body['refresh_token'], equals('refresh-token-123'));
      expect(body['fcm_token'], equals('fcm-token-abc'));
      expect(body['device_id'], equals('device-xyz'));
    },
  );

  test(
    'logoutCurrentSession repository delegates to datasource and preserves failures',
    () async {
      final client = RecordingApiClient();
      final datasource = RecordingAuthApiDatasource(client);
      datasource.result = Result.error('backend down');

      final repo = AuthProfileRepository(
        firebaseAuth: MockFirebaseAuth(),
        apiDatasource: datasource,
      );

      final result = await repo.logoutCurrentSession(
        refreshToken: 'refresh-token-123',
        fcmToken: 'fcm-token-abc',
        deviceId: 'device-xyz',
      );

      expect(datasource.called, isTrue);
      expect(datasource.refreshToken, equals('refresh-token-123'));
      expect(datasource.fcmToken, equals('fcm-token-abc'));
      expect(datasource.deviceId, equals('device-xyz'));
      expect(result.isError, isTrue);
      expect(result.error, equals('backend down'));
    },
  );

  test(
    'logoutAllSessions posts logout-all with default deactivate_fcm_tokens true',
    () async {
      final client = RecordingApiClient();
      final datasource = AuthApiDatasource(client);

      final result = await datasource.logoutAllSessions();

      expect(result.isSuccess, isTrue);
      expect(client.lastPath, equals('/auth/logout-all'));
      expect(client.lastData, isA<Map<String, dynamic>>());

      final body = client.lastData as Map<String, dynamic>;
      expect(body['deactivate_fcm_tokens'], isTrue);
    },
  );

  test(
    'logoutAllSessions repository delegates to datasource and preserves failures',
    () async {
      final client = RecordingApiClient();
      final datasource = RecordingAuthApiDatasource(client);
      datasource.logoutAllResult = Result.error('backend down');

      final repo = AuthProfileRepository(
        firebaseAuth: MockFirebaseAuth(),
        apiDatasource: datasource,
      );

      final result = await repo.logoutAllSessions(deactivateFcmTokens: false);

      expect(datasource.logoutAllCalled, isTrue);
      expect(datasource.deactivateFcmTokens, isFalse);
      expect(result.isError, isTrue);
      expect(result.error, equals('backend down'));
    },
  );

  test(
    'AuthRepositoryImpl logoutAllSessions delegates to the backend contract',
    () async {
      final client = RecordingApiClient();
      final repo = AuthRepositoryImpl(
        firebaseAuth: MockFirebaseAuth(),
        apiDatasource: AuthApiDatasource(client),
      );

      final result = await repo.logoutAllSessions(deactivateFcmTokens: false);

      expect(result.isSuccess, isTrue);
      expect(client.lastPath, equals('/auth/logout-all'));
      final body = client.lastData as Map<String, dynamic>;
      expect(body['deactivate_fcm_tokens'], isFalse);
    },
  );
}
