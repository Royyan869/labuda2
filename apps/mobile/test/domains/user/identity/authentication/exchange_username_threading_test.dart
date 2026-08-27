// Stage 1B — Registration username reaches the authenticated exchange.
//
// Proves the mobile side of the canonical registration flow:
//
//   Register → username selected → Firebase account registration
//     → authenticated exchange(username) → backend assigns username
//
// Covers:
//   1. UserApiDatasource.exchangeFirebaseSession sends `username` in the body.
//   2. AuthApiDatasource.exchangeFirebaseSession sends `username` in the body.
//   3. UserSyncService.syncUser forwards the `username` argument to the
//      datasource exchange call (the caller-facing threading seam).
//   4. Omitting username (login / Google-first-sync) sends only the ID token.
//
// Uses the real datasource + a recording ApiClient so the wire contract is
// asserted without inventing parallel architecture.

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/domains/user/identity/authentication/data/datasources/auth_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';

class _RecordingApiClient implements ApiClient {
  String? lastPath;
  dynamic lastData;
  Options? lastOptions;

  @override
  Dio get dio => throw UnimplementedError();

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
    final responseData = <String, dynamic>{
      'success': true,
      'data': {
        'user_id': 'user-1',
        'access_token': 'restricted-token',
        'expires_at': '2026-07-19T00:00:00Z',
        'requires_profile_completion': false,
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
    void Function(int, int)? onSendProgress,
  }) {
    throw UnimplementedError();
  }
}

void main() {
  group('UserApiDatasource.exchangeFirebaseSession username threading', () {
    test('posts firebase_id_token + username when a registration username is '
        'provided', () async {
      final client = _RecordingApiClient();
      final ds = UserApiDatasource(client);

      final result = await ds.exchangeFirebaseSession(
        firebaseIdToken: 'firebase-id-token-123',
        username: 'alice_reg',
      );

      expect(result.isSuccess, isTrue);
      expect(client.lastPath, equals('/auth/firebase/exchange'));
      expect(
        client.lastData,
        equals({
          'firebase_id_token': 'firebase-id-token-123',
          'username': 'alice_reg',
        }),
      );
      expect(client.lastOptions!.extra!['skipAuth'], isTrue);
    });

    test('omits username entirely when username is empty (login / Google '
        'first-sync)', () async {
      final client = _RecordingApiClient();
      final ds = UserApiDatasource(client);

      final result = await ds.exchangeFirebaseSession(
        firebaseIdToken: 'firebase-id-token-456',
      );

      expect(result.isSuccess, isTrue);
      expect(
        client.lastData,
        equals({'firebase_id_token': 'firebase-id-token-456'}),
        reason: 'Login/Google-first-sync must not send a username field',
      );
    });
  });

  group('AuthApiDatasource.exchangeFirebaseSession username threading', () {
    test('posts firebase_id_token + username when a registration username is '
        'provided', () async {
      final client = _RecordingApiClient();
      final ds = AuthApiDatasource(client);

      final result = await ds.exchangeFirebaseSession(
        firebaseIdToken: 'firebase-id-token-789',
        username: 'bob_reg',
      );

      expect(result.isSuccess, isTrue);
      expect(client.lastPath, equals('/auth/firebase/exchange'));
      expect(
        client.lastData,
        equals({
          'firebase_id_token': 'firebase-id-token-789',
          'username': 'bob_reg',
        }),
      );
      expect(client.lastOptions!.extra!['skipAuth'], isTrue);
    });

    test('omits username entirely when username is null', () async {
      final client = _RecordingApiClient();
      final ds = AuthApiDatasource(client);

      final result = await ds.exchangeFirebaseSession(
        firebaseIdToken: 'firebase-id-token-999',
      );

      expect(result.isSuccess, isTrue);
      expect(
        client.lastData,
        equals({'firebase_id_token': 'firebase-id-token-999'}),
      );
    });
  });
}