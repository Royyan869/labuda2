import 'dart:async';

import 'package:dio/dio.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';

class _MutableFirebaseUser extends Fake implements User {
  _MutableFirebaseUser({
    required this.uidValue,
    required this.idTokenValue,
    Completer<String?>? tokenCompleter,
  }) : _tokenCompleter = tokenCompleter;

  final String uidValue;
  final String? idTokenValue;
  final Completer<String?>? _tokenCompleter;

  @override
  String get uid => uidValue;

  @override
  bool get emailVerified => true;

  @override
  Future<String?> getIdToken([bool forceRefresh = false]) async {
    if (_tokenCompleter != null) {
      return _tokenCompleter.future;
    }
    return idTokenValue;
  }
}

class _MutableFirebaseAuth extends Fake implements FirebaseAuth {
  _MutableFirebaseAuth({required User? currentUserValue})
    : _currentUserValue = currentUserValue;

  User? _currentUserValue;

  set currentUserValue(User? value) {
    _currentUserValue = value;
  }

  @override
  User? get currentUser => _currentUserValue;

  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();
}

class _RecordingLocalStorageService extends Fake
    implements ILocalStorageService {
  String? authToken;
  String? refreshToken;
  int clearAuthTokenCalls = 0;
  int clearRefreshTokenCalls = 0;

  @override
  Future<Result<void>> setAuthToken(String token) async {
    authToken = token;
    return Result.success(null);
  }

  @override
  Future<Result<void>> setRefreshToken(String token) async {
    refreshToken = token;
    return Result.success(null);
  }

  @override
  Future<Result<void>> clearAuthToken() async {
    clearAuthTokenCalls++;
    authToken = null;
    return Result.success(null);
  }

  @override
  Future<Result<void>> clearRefreshToken() async {
    clearRefreshTokenCalls++;
    refreshToken = null;
    return Result.success(null);
  }
}

/// Simple mock exchange function usable with [FirebaseExchangeFn] typedef.
class _MockExchangeFn {
  int calls = 0;
  String? lastRegistrationUsername;
  Result<FirebaseExchangeResponse> result = Result.success(
    const FirebaseExchangeResponse(
      userId: 'backend-user-1',
      accessToken: 'access-token',
      expiresAt: '2026-06-14T00:00:00Z',
      requiresProfileCompletion: false,
      created: true,
      email: 'seller@example.com',
    ),
  );

  Future<Result<FirebaseExchangeResponse>> call({
    required String firebaseIdToken,
    String? registrationUsername,
  }) async {
    calls++;
    lastRegistrationUsername = registrationUsername;
    return result;
  }
}

class _RecordingUserApiDatasource extends UserApiDatasource {
  _RecordingUserApiDatasource() : super(_NoopApiClient());

  int currentUserCalls = 0;
  Result<UserApiResponse> currentUserResult = Result.success(
    UserApiResponse.fromJson({
      'id': 'backend-user-1',
      'email': 'seller@example.com',
      'username': 'seller-one',
      'account_status': 'active',
      'roles': ['seller'],
      'has_seller_profile': true,
      'seller_subscription_status': 'active',
      'has_market_authority': true,
      'is_email_verified': true,
      'created_at': '2026-06-01T00:00:00Z',
      'updated_at': '2026-06-02T00:00:00Z',
      'profile': {
        'id': 'backend-user-1',
        'username': 'seller-one',
        'bio': 'bio',
        'avatar_url': 'https://example.com/avatar.png',
        'followers_count': 1,
        'following_count': 2,
        'preferred_lang': 'en',
      },
    }),
  );
  Completer<Result<UserApiResponse>>? currentUserCompleter;

  @override
  Future<Result<UserApiResponse>> getCurrentUser() async {
    currentUserCalls++;
    if (currentUserCompleter != null) {
      return currentUserCompleter!.future;
    }
    return currentUserResult;
  }
}

class _NoopApiClient implements ApiClient {
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

Future<void> _drain() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
}

void main() {
  group('UserSyncService principal safety', () {
    test('getCurrentUser rejects a mismatched Firebase uid', () async {
      final auth = _MutableFirebaseAuth(
        currentUserValue: _MutableFirebaseUser(
          uidValue: 'uid-b',
          idTokenValue: 'token-b',
        ),
      );
      final userDatasource = _RecordingUserApiDatasource();
      final exchangeMock = _MockExchangeFn();
      final service = UserSyncService(
        firebaseAuth: auth,
        userDatasource: userDatasource,
        exchange: exchangeMock.call,
        localStorage: _RecordingLocalStorageService(),
      );

      final result = await service.getCurrentUser(
        expectedUid: 'uid-a',
        expectedEpoch: 1,
        isCurrentPrincipalOperation: (expectedUid, expectedEpoch) => true,
      );

      expect(result.isError, isTrue);
      expect(result.errorCode, 'STALE_PRINCIPAL');
      expect(userDatasource.currentUserCalls, 0);
    });

    test(
      'syncUser rejects a uid swap before token retrieval completes',
      () async {
        final tokenCompleter = Completer<String?>();
        final auth = _MutableFirebaseAuth(
          currentUserValue: _MutableFirebaseUser(
            uidValue: 'uid-a',
            idTokenValue: null,
            tokenCompleter: tokenCompleter,
          ),
        );
        final userDatasource = _RecordingUserApiDatasource();
        final exchangeMock = _MockExchangeFn();
        final service = UserSyncService(
          firebaseAuth: auth,
          userDatasource: userDatasource,
          exchange: exchangeMock.call,
          localStorage: _RecordingLocalStorageService(),
        );

        final future = service.syncUser(
          expectedUid: 'uid-a',
          expectedEpoch: 1,
          isCurrentPrincipalOperation: (expectedUid, expectedEpoch) => true,
          username: 'seller-one',
        );
        await _drain();
        auth.currentUserValue = _MutableFirebaseUser(
          uidValue: 'uid-b',
          idTokenValue: 'token-b',
        );
        tokenCompleter.complete('token-a');

        final result = await future;

        expect(result.isError, isTrue);
        expect(result.errorCode, 'STALE_PRINCIPAL');
        expect(exchangeMock.calls, 0);
      },
    );

    test(
      'syncUser leaves stored tokens untouched when the uid changes before /users/me returns',
      () async {
        final auth = _MutableFirebaseAuth(
          currentUserValue: _MutableFirebaseUser(
            uidValue: 'uid-a',
            idTokenValue: 'token-a',
          ),
        );
        final userDatasource = _RecordingUserApiDatasource();
        final exchangeMock = _MockExchangeFn();
        final currentUserCompleter = Completer<Result<UserApiResponse>>();
        userDatasource.currentUserCompleter = currentUserCompleter;
        final localStorage = _RecordingLocalStorageService();
        final service = UserSyncService(
          firebaseAuth: auth,
          userDatasource: userDatasource,
          exchange: exchangeMock.call,
          localStorage: localStorage,
        );

        final future = service.syncUser(
          expectedUid: 'uid-a',
          expectedEpoch: 1,
          isCurrentPrincipalOperation: (expectedUid, expectedEpoch) => true,
          username: 'seller-one',
        );
        await _drain();
        auth.currentUserValue = _MutableFirebaseUser(
          uidValue: 'uid-b',
          idTokenValue: 'token-b',
        );
        currentUserCompleter.complete(
          Result.success(
            UserApiResponse.fromJson({
              'id': 'backend-user-1',
              'email': 'seller@example.com',
              'username': 'seller-one',
              'account_status': 'active',
              'roles': ['seller'],
              'has_seller_profile': true,
              'seller_subscription_status': 'active',
              'has_market_authority': true,
              'is_email_verified': true,
              'created_at': '2026-06-01T00:00:00Z',
              'updated_at': '2026-06-02T00:00:00Z',
              'profile': {
                'id': 'backend-user-1',
                'username': 'seller-one',
                'bio': 'bio',
                'avatar_url': 'https://example.com/avatar.png',
                'followers_count': 1,
                'following_count': 2,
                'preferred_lang': 'en',
              },
            }),
          ),
        );

        final result = await future;

        expect(result.isError, isTrue);
        expect(result.errorCode, 'STALE_PRINCIPAL');
        expect(exchangeMock.calls, 1);
        expect(localStorage.authToken, isNull);
        expect(localStorage.refreshToken, isNull);
        expect(localStorage.clearAuthTokenCalls, 0);
        expect(localStorage.clearRefreshTokenCalls, 0);
      },
    );

    test(
      'syncUser leaves stored tokens untouched when the epoch changes before /users/me returns',
      () async {
        final auth = _MutableFirebaseAuth(
          currentUserValue: _MutableFirebaseUser(
            uidValue: 'uid-a',
            idTokenValue: 'token-a',
          ),
        );
        final userDatasource = _RecordingUserApiDatasource();
        final exchangeMock = _MockExchangeFn();
        final currentUserCompleter = Completer<Result<UserApiResponse>>();
        userDatasource.currentUserCompleter = currentUserCompleter;
        final localStorage = _RecordingLocalStorageService();
        var isCurrent = true;
        final service = UserSyncService(
          firebaseAuth: auth,
          userDatasource: userDatasource,
          exchange: exchangeMock.call,
          localStorage: localStorage,
        );

        final future = service.syncUser(
          expectedUid: 'uid-a',
          expectedEpoch: 1,
          isCurrentPrincipalOperation: (expectedUid, expectedEpoch) =>
              isCurrent,
          username: 'seller-one',
        );
        await _drain();
        isCurrent = false;
        currentUserCompleter.complete(
          Result.success(
            UserApiResponse.fromJson({
              'id': 'backend-user-1',
              'email': 'seller@example.com',
              'username': 'seller-one',
              'account_status': 'active',
              'roles': ['seller'],
              'has_seller_profile': true,
              'seller_subscription_status': 'active',
              'has_market_authority': true,
              'is_email_verified': true,
              'created_at': '2026-06-01T00:00:00Z',
              'updated_at': '2026-06-02T00:00:00Z',
              'profile': {
                'id': 'backend-user-1',
                'username': 'seller-one',
                'bio': 'bio',
                'avatar_url': 'https://example.com/avatar.png',
                'followers_count': 1,
                'following_count': 2,
                'preferred_lang': 'en',
              },
            }),
          ),
        );

        final result = await future;

        expect(result.isError, isTrue);
        expect(result.errorCode, 'STALE_PRINCIPAL');
        expect(exchangeMock.calls, 1);
        expect(localStorage.authToken, isNull);
        expect(localStorage.refreshToken, isNull);
        expect(localStorage.clearAuthTokenCalls, 0);
        expect(localStorage.clearRefreshTokenCalls, 0);
      },
    );
  });
}
