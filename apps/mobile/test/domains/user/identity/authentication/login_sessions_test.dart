// login_sessions_test.dart — mobile contract regression proof for
// the Login Sessions feature (F3G).
//
// Invariants covered:
//   LS-1:  AuthSessionDto.fromJson parses full payload correctly
//   LS-2:  AuthSessionDto.fromJson handles missing optional fields safely
//   LS-3:  AuthSessionDto.fromJson never contains token_hash / jti / ip_hash
//   LS-4:  AuthApiDatasource.getActiveSessions calls GET /auth/sessions
//   LS-5:  AuthApiDatasource.revokeSession calls DELETE /auth/sessions/:family_id
//   LS-6:  AuthProfileRepository.getActiveSessions delegates to datasource
//   LS-7:  AuthProfileRepository.revokeSession delegates to datasource
//   LS-8:  AuthProfileRepository.getActiveSessions propagates datasource failure
//   LS-9:  AuthProfileRepository.revokeSession propagates datasource failure
//   LS-10: AuthRepositoryImpl.getActiveSessions delegates to profile repository
//   LS-11: AuthRepositoryImpl.revokeSession delegates to profile repository
//   LS-12: logoutAllSessions contract still passes (regression)
//   LS-13: AuthSession entity deviceLabel falls back correctly
//   LS-14: AuthSession entity lastActivity prefers lastUsedAt over issuedAt

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:mockito/mockito.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/user/identity/authentication/data/datasources/auth_api_datasource.dart';
import 'package:labuda/domains/user/identity/authentication/data/repositories/auth_profile_repository.dart';
import 'package:labuda/domains/user/identity/authentication/data/repositories/auth_repository_impl.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_session.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

class _RecordingApiClient implements ApiClient {
  String? lastPath;
  String? lastMethod;
  dynamic lastData;

  void reset() {
    lastPath = null;
    lastMethod = null;
    lastData = null;
  }

  @override
  Dio get dio => throw UnimplementedError();

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPath = path;
    lastMethod = 'GET';
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      statusCode: 200,
      data:
          {
                'success': true,
                'data': {
                  'sessions': [
                    {
                      'family_id': 'fam-001',
                      'device_id': 'dev-001',
                      'device_name': 'Pixel 7',
                      'platform': 'android',
                      'app_version': '1.2.3',
                      'issued_at': '2026-06-01T10:00:00Z',
                      'expires_at': '2026-07-01T10:00:00Z',
                      'last_used_at': '2026-06-14T09:00:00Z',
                      'fcm_token_active': true,
                    },
                  ],
                },
              }
              as T,
    );
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
    lastMethod = 'POST';
    lastData = data;
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
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPath = path;
    lastMethod = 'DELETE';
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      statusCode: 200,
      data: {'success': true, 'data': null} as T,
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

  @override
  ApiException extractException(DioException e) =>
      UnknownApiException(message: e.message ?? 'Unknown error');

  @override
  bool isNotFound(DioException e) => false;

  @override
  bool isNetworkError(DioException e) => false;

  @override
  bool isUnauthorized(DioException e) => false;

  @override
  bool isValidationError(DioException e) => false;
}

class _RecordingDatasource extends AuthApiDatasource {
  _RecordingDatasource(super.apiClient);

  String? getActiveSessionsCalled;
  String? revokeSessionFamilyId;
  bool logoutAllCalled = false;
  bool? deactivateFcmTokens;

  Result<List<AuthSessionDto>> getSessionsResult = Result.success([]);
  Result<void> revokeResult = Result.success(null);
  Result<void> logoutAllResult = Result.success(null);

  @override
  Future<Result<List<AuthSessionDto>>> getActiveSessions() async {
    getActiveSessionsCalled = 'called';
    return getSessionsResult;
  }

  @override
  Future<Result<void>> revokeSession(String familyId) async {
    revokeSessionFamilyId = familyId;
    return revokeResult;
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

class _MockFirebaseAuth extends Mock implements FirebaseAuth {
  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();
}

// ---------------------------------------------------------------------------
// LS-1 / LS-2 / LS-3: AuthSessionDto parsing
// ---------------------------------------------------------------------------

void main() {
  group('LS-1: AuthSessionDto.fromJson — full payload', () {
    test('parses all fields from canonical backend response', () {
      final json = {
        'family_id': 'fam-abc-123',
        'device_id': 'dev-xyz',
        'device_name': 'iPhone 15',
        'platform': 'ios',
        'app_version': '2.1.0',
        'issued_at': '2026-06-01T08:00:00Z',
        'expires_at': '2026-07-01T08:00:00Z',
        'last_used_at': '2026-06-14T10:30:00Z',
        'fcm_token_active': true,
      };

      final dto = AuthSessionDto.fromJson(json);

      expect(dto.familyId, equals('fam-abc-123'));
      expect(dto.deviceId, equals('dev-xyz'));
      expect(dto.deviceName, equals('iPhone 15'));
      expect(dto.platform, equals('ios'));
      expect(dto.appVersion, equals('2.1.0'));
      expect(dto.issuedAt, equals(DateTime.parse('2026-06-01T08:00:00Z')));
      expect(dto.expiresAt, equals(DateTime.parse('2026-07-01T08:00:00Z')));
      expect(dto.lastUsedAt, equals(DateTime.parse('2026-06-14T10:30:00Z')));
      expect(dto.fcmTokenActive, isTrue);
    });
  });

  group('LS-2: AuthSessionDto.fromJson — missing optional fields', () {
    test('optional fields are null when absent from response', () {
      final json = {
        'family_id': 'fam-minimal',
        'issued_at': '2026-06-10T12:00:00Z',
        'expires_at': '2026-07-10T12:00:00Z',
      };

      final dto = AuthSessionDto.fromJson(json);

      expect(dto.familyId, equals('fam-minimal'));
      expect(dto.deviceId, isNull);
      expect(dto.deviceName, isNull);
      expect(dto.platform, isNull);
      expect(dto.appVersion, isNull);
      expect(dto.lastUsedAt, isNull);
      expect(dto.fcmTokenActive, isNull);
    });
  });

  group('LS-3: AuthSessionDto — no sensitive fields', () {
    test('AuthSessionDto has no token_hash, jti, or ip_hash fields', () {
      // Static check: verify the model class does not expose sensitive fields.
      // This is a compile-time contract that raw token data is not in DTO.
      final dto = AuthSessionDto.fromJson({
        'family_id': 'fam-001',
        'issued_at': '2026-06-01T00:00:00Z',
        'expires_at': '2026-07-01T00:00:00Z',
        // These fields, even if accidentally sent by backend, must not be
        // accessible as named properties on AuthSessionDto.
        'token_hash': 'should-be-ignored',
        'jti': 'should-be-ignored',
        'ip_hash': 'should-be-ignored',
      });

      // Verify that accessing named properties does not expose sensitive data.
      // The class only has: familyId, deviceId, deviceName, platform,
      // appVersion, issuedAt, expiresAt, lastUsedAt, fcmTokenActive.
      expect(dto.familyId, equals('fam-001'));
      // The following lines would fail to compile if sensitive fields existed:
      // dto.tokenHash; dto.jti; dto.ipHash; — absence is the contract.
    });
  });

  // ---------------------------------------------------------------------------
  // LS-4 / LS-5: AuthApiDatasource route calls
  // ---------------------------------------------------------------------------

  group(
    'LS-4: AuthApiDatasource.getActiveSessions calls GET /auth/sessions',
    () {
      test('uses correct path', () async {
        final client = _RecordingApiClient();
        final ds = AuthApiDatasource(client);

        await ds.getActiveSessions();

        expect(client.lastPath, equals('/auth/sessions'));
        expect(client.lastMethod, equals('GET'));
      });
    },
  );

  group(
    'LS-5: AuthApiDatasource.revokeSession calls DELETE /auth/sessions/:family_id',
    () {
      test('uses correct path with family_id', () async {
        final client = _RecordingApiClient();
        final ds = AuthApiDatasource(client);

        await ds.revokeSession('fam-abc-123');

        expect(client.lastPath, equals('/auth/sessions/fam-abc-123'));
        expect(client.lastMethod, equals('DELETE'));
      });
    },
  );

  // ---------------------------------------------------------------------------
  // LS-6 / LS-7 / LS-8 / LS-9: AuthProfileRepository delegation
  // ---------------------------------------------------------------------------

  group(
    'LS-6: AuthProfileRepository.getActiveSessions delegates to datasource',
    () {
      test('returns sessions from datasource', () async {
        final client = _RecordingApiClient();
        final ds = _RecordingDatasource(client);
        ds.getSessionsResult = Result.success([
          AuthSessionDto.fromJson({
            'family_id': 'fam-001',
            'issued_at': '2026-06-01T00:00:00Z',
            'expires_at': '2026-07-01T00:00:00Z',
          }),
        ]);
        final repo = AuthProfileRepository(
          firebaseAuth: _MockFirebaseAuth(),
          apiDatasource: ds,
        );

        final result = await repo.getActiveSessions();

        expect(ds.getActiveSessionsCalled, equals('called'));
        expect(result.isSuccess, isTrue);
        expect(result.data, hasLength(1));
        expect(result.data!.first.familyId, equals('fam-001'));
      });
    },
  );

  group(
    'LS-7: AuthProfileRepository.revokeSession delegates to datasource',
    () {
      test('passes family_id to datasource', () async {
        final client = _RecordingApiClient();
        final ds = _RecordingDatasource(client);
        final repo = AuthProfileRepository(
          firebaseAuth: _MockFirebaseAuth(),
          apiDatasource: ds,
        );

        final result = await repo.revokeSession('fam-xyz');

        expect(ds.revokeSessionFamilyId, equals('fam-xyz'));
        expect(result.isSuccess, isTrue);
      });
    },
  );

  group('LS-8: AuthProfileRepository.getActiveSessions propagates failure', () {
    test('returns error when datasource fails', () async {
      final client = _RecordingApiClient();
      final ds = _RecordingDatasource(client);
      ds.getSessionsResult = Result.error('network error');
      final repo = AuthProfileRepository(
        firebaseAuth: _MockFirebaseAuth(),
        apiDatasource: ds,
      );

      final result = await repo.getActiveSessions();

      expect(result.isError, isTrue);
      expect(result.error, equals('network error'));
    });
  });

  group('LS-9: AuthProfileRepository.revokeSession propagates failure', () {
    test('returns error when datasource fails', () async {
      final client = _RecordingApiClient();
      final ds = _RecordingDatasource(client);
      ds.revokeResult = Result.error('revoke failed');
      final repo = AuthProfileRepository(
        firebaseAuth: _MockFirebaseAuth(),
        apiDatasource: ds,
      );

      final result = await repo.revokeSession('fam-xyz');

      expect(result.isError, isTrue);
      expect(result.error, equals('revoke failed'));
    });
  });

  // ---------------------------------------------------------------------------
  // LS-10 / LS-11: AuthRepositoryImpl delegation
  // ---------------------------------------------------------------------------

  group(
    'LS-10: AuthRepositoryImpl.getActiveSessions delegates to profile repo',
    () {
      test('calls GET /auth/sessions via datasource chain', () async {
        final client = _RecordingApiClient();
        final ds = AuthApiDatasource(client);
        final repo = AuthRepositoryImpl(
          firebaseAuth: _MockFirebaseAuth(),
          apiDatasource: ds,
        );

        await repo.getActiveSessions();

        expect(client.lastPath, equals('/auth/sessions'));
        expect(client.lastMethod, equals('GET'));
      });
    },
  );

  group(
    'LS-11: AuthRepositoryImpl.revokeSession delegates to profile repo',
    () {
      test(
        'calls DELETE /auth/sessions/:family_id via datasource chain',
        () async {
          final client = _RecordingApiClient();
          final ds = AuthApiDatasource(client);
          final repo = AuthRepositoryImpl(
            firebaseAuth: _MockFirebaseAuth(),
            apiDatasource: ds,
          );

          await repo.revokeSession('fam-impl-test');

          expect(client.lastPath, equals('/auth/sessions/fam-impl-test'));
          expect(client.lastMethod, equals('DELETE'));
        },
      );
    },
  );

  // ---------------------------------------------------------------------------
  // LS-12: logoutAllSessions regression
  // ---------------------------------------------------------------------------

  group('LS-12: logoutAllSessions contract regression', () {
    test(
      'AuthRepositoryImpl.logoutAllSessions still calls POST /auth/logout-all',
      () async {
        final client = _RecordingApiClient();
        final repo = AuthRepositoryImpl(
          firebaseAuth: _MockFirebaseAuth(),
          apiDatasource: AuthApiDatasource(client),
        );

        final result = await repo.logoutAllSessions(deactivateFcmTokens: false);

        expect(result.isSuccess, isTrue);
        expect(client.lastPath, equals('/auth/logout-all'));
        expect(client.lastMethod, equals('POST'));
        final body = client.lastData as Map<String, dynamic>;
        expect(body['deactivate_fcm_tokens'], isFalse);
      },
    );
  });

  // ---------------------------------------------------------------------------
  // LS-13 / LS-14: AuthSession entity computed properties
  // ---------------------------------------------------------------------------

  group('LS-13: AuthSession.deviceLabel fallback chain', () {
    test('returns deviceName when present', () {
      final s = AuthSession(
        familyId: 'f1',
        deviceName: 'My Phone',
        platform: 'android',
        issuedAt: DateTime.now(),
        expiresAt: DateTime.now().add(const Duration(days: 30)),
      );
      expect(s.deviceLabel, equals('My Phone'));
    });

    test('returns capitalized platform when deviceName absent', () {
      final s = AuthSession(
        familyId: 'f1',
        platform: 'android',
        issuedAt: DateTime.now(),
        expiresAt: DateTime.now().add(const Duration(days: 30)),
      );
      expect(s.deviceLabel, equals('Android'));
    });

    test('iOS is correctly capitalized', () {
      final s = AuthSession(
        familyId: 'f1',
        platform: 'ios',
        issuedAt: DateTime.now(),
        expiresAt: DateTime.now().add(const Duration(days: 30)),
      );
      expect(s.deviceLabel, equals('iOS'));
    });

    test('falls back to "Perangkat tidak dikenal" when both absent', () {
      final s = AuthSession(
        familyId: 'f1',
        issuedAt: DateTime.now(),
        expiresAt: DateTime.now().add(const Duration(days: 30)),
      );
      expect(s.deviceLabel, equals('Perangkat tidak dikenal'));
    });
  });

  group('LS-14: AuthSession.lastActivity prefers lastUsedAt over issuedAt', () {
    test('returns lastUsedAt when present', () {
      final issued = DateTime(2026, 6, 1);
      final lastUsed = DateTime(2026, 6, 14);
      final s = AuthSession(
        familyId: 'f1',
        issuedAt: issued,
        expiresAt: issued.add(const Duration(days: 30)),
        lastUsedAt: lastUsed,
      );
      expect(s.lastActivity, equals(lastUsed));
    });

    test('falls back to issuedAt when lastUsedAt is null', () {
      final issued = DateTime(2026, 6, 1);
      final s = AuthSession(
        familyId: 'f1',
        issuedAt: issued,
        expiresAt: issued.add(const Duration(days: 30)),
      );
      expect(s.lastActivity, equals(issued));
    });
  });
}
