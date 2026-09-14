// AUTH CLEANUP SLICE 22 — completeProfile STRUCTURED ERROR METADATA CARRY-THROUGH
//
// The defect this covers (proven in Slice 21):
//
//   _apiDatasource.completeProfile(...) → isError
//     → Result.error(result.error ?? 'Failed to complete profile')
//
// ...stripped the backend's machine-readable `code`/`statusCode`/`details`.
// AuthController.completeProfile branches on those structured fields
// (PROFILE_ALREADY_COMPLETED / INVALID_SCOPE / 5xx → backendUnavailable) and was
// left relying on free-text matching such as `error.contains('already completed')`.
//
// This test crosses the REAL seam — real AuthApiDatasource (envelope/exception →
// Result) into the REAL AuthProfileRepository.completeProfile — instead of
// injecting a pre-built Result into a fake IAuthRepository, which would not
// exercise the metadata-loss point at all.
//
// Slice 24 update: BaseApiRepository.executeRequest now forwards `statusCode`
// on its DioException branch (previously the only execute* method that dropped
// it). Case C therefore asserts statusCode == 500 — the value that makes
// AuthController.classifyAuthSyncError resolve a 5xx to `backendUnavailable`.

import 'package:dio/dio.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/datasources/auth_api_datasource.dart';
import 'package:labuda/domains/user/identity/authentication/data/repositories/auth_profile_repository.dart';

/// ApiClient fake that returns whatever HTTP outcome a test scripts, mirroring
/// production `ApiClient.extractException` semantics (api_client.dart:195).
class _ScriptedApiClient implements ApiClient {
  _ScriptedApiClient({required this.onRequest});

  /// Scripted HTTP outcome per path — serves both the `post` leg
  /// (/auth/complete-profile) and the `get` leg (/users/me).
  final Future<Response<dynamic>> Function(String path) onRequest;

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    final response = await onRequest(path);
    return Response<T>(
      requestOptions: response.requestOptions,
      statusCode: response.statusCode,
      data: response.data as T,
    );
  }

  @override
  ApiException extractException(DioException e) {
    final error = e.error;
    return error is ApiException
        ? error
        : UnknownApiException(message: e.message ?? 'unknown');
  }

  // ----- unused ApiClient surface (never reached by completeProfile) -----

  @override
  Dio get dio => throw UnimplementedError();

  @override
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) => throw UnimplementedError();

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    final response = await onRequest(path);
    return Response<T>(
      requestOptions: response.requestOptions,
      statusCode: response.statusCode,
      data: response.data as T,
    );
  }

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
  }) => throw UnimplementedError();

  @override
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) => throw UnimplementedError();

  @override
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int, int)? onSendProgress,
  }) => throw UnimplementedError();
}

class _NoopFirebaseAuth extends Fake implements FirebaseAuth {}

class _StubLocalStorage extends Fake implements ILocalStorageService {
  @override
  Future<Result<String?>> getRestrictedToken() async =>
      Result.success('restricted-token');

  @override
  Future<Result<void>> saveLabudaCredential(
    String accessToken,
    String refreshToken,
  ) async => Result.success(null);

  @override
  Future<Result<void>> clearRestrictedToken() async => Result.success(null);

  @override
  Future<Result<void>> clearLabudaCredential() async => Result.success(null);
}

Response<dynamic> _response({
  required int statusCode,
  required Map<String, dynamic> body,
  required String path,
}) => Response<dynamic>(
  requestOptions: RequestOptions(path: path),
  statusCode: statusCode,
  data: body,
);

Map<String, dynamic> _backendError(int statusCode, String code, String message) =>
    <String, dynamic>{
      'success': false,
      'error': <String, dynamic>{'code': code, 'message': message},
      'timestamp': '2026-09-14T00:00:00Z',
    };

AuthProfileRepository _repository(_ScriptedApiClient client) =>
    AuthProfileRepository(
      firebaseAuth: _NoopFirebaseAuth(),
      apiDatasource: AuthApiDatasource(client),
      localStorage: _StubLocalStorage(),
    );

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'Case A — 409 PROFILE_ALREADY_COMPLETED reaches the caller with code and statusCode',
    () async {
      final client = _ScriptedApiClient(
        onRequest: (path) async => _response(
          statusCode: 409,
          path: path,
          body: _backendError(
            409,
            'PROFILE_ALREADY_COMPLETED',
            'Profile is already completed',
          ),
        ),
      );

      final result = await _repository(client).completeProfile(
        username: 'seller-two',
      );

      expect(result.isError, isTrue);
      expect(result.error, equals('Profile is already completed'));
      expect(result.errorCode, equals('PROFILE_ALREADY_COMPLETED'));
      expect(result.statusCode, equals(409));
    },
  );

  test(
    'Case B — 403 INVALID_SCOPE reaches the caller with code and statusCode',
    () async {
      final client = _ScriptedApiClient(
        onRequest: (path) async => _response(
          statusCode: 403,
          path: path,
          body: _backendError(
            403,
            'INVALID_SCOPE',
            'Restricted token scope is invalid',
          ),
        ),
      );

      final result = await _repository(client).completeProfile(
        username: 'seller-two',
      );

      expect(result.isError, isTrue);
      expect(result.error, equals('Restricted token scope is invalid'));
      expect(result.errorCode, equals('INVALID_SCOPE'));
      expect(result.statusCode, equals(403));
    },
  );

  test(
    'Case C — 500 keeps structured code, statusCode and details, and classifies '
    'as backendUnavailable for the controller retry path',
    () async {
      final client = _ScriptedApiClient(
        onRequest: (path) async => throw DioException(
          requestOptions: RequestOptions(path: path),
          type: DioExceptionType.badResponse,
          response: _response(
            statusCode: 500,
            path: path,
            body: _backendError(500, 'INTERNAL_SERVER_ERROR', 'Database error'),
          ),
          error: const ServerException(
            message: 'Database error',
            code: 'INTERNAL_SERVER_ERROR',
            statusCode: 500,
            details: <String, dynamic>{'op': 'complete_profile'},
          ),
        ),
      );

      final result = await _repository(client).completeProfile(
        username: 'seller-two',
      );

      expect(result.isError, isTrue);
      expect(result.error, equals('Database error'));
      // Datasource metadata must survive the repository carry-through.
      expect(result.errorCode, equals('INTERNAL_SERVER_ERROR'));
      expect(result.errorDetails, equals(<String, dynamic>{'op': 'complete_profile'}));
      expect(result.statusCode, equals(500));

      // AUTH consequence (Slice 23/24 contract): a 5xx with the HTTP status
      // preserved classifies as backendUnavailable — the state that keeps the
      // Firebase session and schedules the bounded auto-retry — instead of
      // degrading to a terminal backendFailure through free-text matching.
      expect(
        classifyAuthSyncError(
          result.error,
          errorCode: result.errorCode,
          statusCode: result.statusCode,
        ),
        AuthSyncErrorKind.backendUnavailable,
      );
    },
  );

  test(
    'SESSION_USER_MISMATCH branch stays repository-owned and unchanged',
    () async {
      final client = _ScriptedApiClient(
        onRequest: (path) async => path == '/auth/complete-profile'
            ? _response(
                statusCode: 200,
                path: path,
                body: <String, dynamic>{
                  'success': true,
                  'data': <String, dynamic>{
                    'user_id': 'user-1',
                    'access_token': 'platform-access-token',
                    'refresh_token': 'platform-refresh-token',
                    'expires_at': '2026-09-15T00:00:00Z',
                    'refresh_expires_at': '2026-09-16T00:00:00Z',
                    'requires_profile_completion': false,
                    'created': true,
                  },
                },
              )
            : _response(
                statusCode: 200,
                path: path,
                body: <String, dynamic>{
                  'success': true,
                  'data': <String, dynamic>{
                    'user': <String, dynamic>{
                      'id': 'user-2',
                      'email': 'seller@example.com',
                      'username': 'seller-one',
                      'account_status': 'active',
                      'roles': <String>['seller'],
                      'has_seller_profile': true,
                      'seller_subscription_status': 'active',
                      'has_market_authority': true,
                      'is_email_verified': false,
                      'created_at': '2026-09-01T00:00:00Z',
                      'updated_at': '2026-09-02T00:00:00Z',
                    },
                    'profile': <String, dynamic>{
                      'id': 'user-2',
                      'username': 'seller-one',
                      'bio': 'bio',
                      'avatar_url': 'https://example.com/avatar.png',
                      'followers_count': 1,
                      'following_count': 2,
                      'preferred_lang': 'en',
                    },
                  },
                },
              ),
      );

      final result = await _repository(client).completeProfile(
        username: 'seller-two',
      );

      expect(result.isError, isTrue);
      expect(result.error, equals('Backend session user mismatch'));
      expect(result.errorCode, equals('SESSION_USER_MISMATCH'));
      expect(result.statusCode, equals(409));
      expect(
        result.errorDetails,
        equals(<String, dynamic>{
          'complete_profile_user_id': 'user-1',
          'current_user_id': 'user-2',
        }),
      );
    },
  );
}
