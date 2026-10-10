import 'package:dio/dio.dart';
import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data;
import 'package:hishumi/domains/user/identity/authentication/data/datasources/auth_api_datasource.dart';
import 'package:hishumi/domains/user/identity/authentication/data/repositories/auth_profile_repository.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/firebase_principal.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/user_profile_patch.dart';
import 'package:hishumi/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:hishumi/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    show userSyncServiceProvider;
import 'package:hishumi/domains/user/profile/data/services/user_sync_service.dart';

class _MockApiClient implements ApiClient {
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

class _MockFirebaseUser extends Fake implements User {
  _MockFirebaseUser({required this.idToken});

  final String idToken;

  @override
  String get uid => 'firebase-user-1';

  @override
  String? get email => 'firebase@example.com';

  @override
  bool get emailVerified => true;

  @override
  String? get phoneNumber => null;

  @override
  List<UserInfo> get providerData => const <UserInfo>[];

  @override
  UserMetadata get metadata => _MockUserMetadata();

  @override
  Future<String?> getIdToken([bool forceRefresh = false]) async => idToken;
}

class _MockUserMetadata extends Fake implements UserMetadata {
  @override
  DateTime? get creationTime => DateTime.parse('2026-06-01T00:00:00Z');

  @override
  DateTime? get lastSignInTime => DateTime.parse('2026-06-02T00:00:00Z');
}

class _MockFirebaseAuth extends Fake implements FirebaseAuth {
  _MockFirebaseAuth({required this.currentUserValue});

  final User? currentUserValue;

  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();

  @override
  User? get currentUser => currentUserValue;
}

class _RecordingLocalStorageService extends Fake
    implements ILocalStorageService {
  String? authToken;
  String? refreshToken;
  String? restrictedToken;
  int setAuthTokenCalls = 0;
  int setRefreshTokenCalls = 0;
  int clearAuthTokenCalls = 0;
  int clearRefreshTokenCalls = 0;
  int setRestrictedTokenCalls = 0;
  int clearRestrictedTokenCalls = 0;

  @override
  Future<Result<void>> initialize() async => Result.success(null);

  @override
  Future<Result<void>> clear() async => Result.success(null);

  @override
  Future<Result<void>> clearSecure() async => Result.success(null);

  Future<Result<void>> clearAuthToken() async {
    clearAuthTokenCalls++;
    authToken = null;
    return Result.success(null);
  }

  Future<Result<void>> clearRefreshToken() async {
    clearRefreshTokenCalls++;
    refreshToken = null;
    return Result.success(null);
  }

  // AUTH-2 (CREDENTIAL AUTHORITY): back the recording fields via the canonical
  // credential boundary; the counters keep meaning so the login/session tests
  // observe the same writes as production.
  @override
  Future<Result<void>> saveHiShumiCredential(String access, String refresh) async {
    setAuthTokenCalls++;
    setRefreshTokenCalls++;
    authToken = access;
    refreshToken = refresh;
    return Result.success(null);
  }

  @override
  Future<Result<String?>> readHiShumiAccessToken() async =>
      Result.success(authToken);

  @override
  Future<Result<String?>> readHiShumiRefreshToken() async =>
      Result.success(refreshToken);

  @override
  Future<Result<bool>> hasHiShumiCredential() async => Result.success(
        authToken != null &&
            authToken!.isNotEmpty &&
            refreshToken != null &&
            refreshToken!.isNotEmpty,
      );

  @override
  Future<Result<void>> clearHiShumiCredential() async {
    clearAuthTokenCalls++;
    clearRefreshTokenCalls++;
    authToken = null;
    refreshToken = null;
    return Result.success(null);
  }

  // AUTH-2 (CREDENTIAL AUTHORITY): the restricted-completion credential lives
  // on its own isolated key (canonical contract). completeProfile reads it,
  // consumes it via clearRestrictedToken, and persists the full credential via
  // saveHiShumiCredential. Back these with the dedicated field so the
  // session-hydration tests observe the real production path.
  @override
  Future<Result<void>> setRestrictedToken(String token) async {
    setRestrictedTokenCalls++;
    restrictedToken = token;
    return Result.success(null);
  }

  @override
  Future<Result<String?>> getRestrictedToken() async =>
      Result.success(restrictedToken);

  @override
  Future<Result<void>> clearRestrictedToken() async {
    clearRestrictedTokenCalls++;
    restrictedToken = null;
    return Result.success(null);
  }

  Future<Result<void>> clearUserSession() async {
    authToken = null;
    refreshToken = null;
    return Result.success(null);
  }

  @override
  Future<Result<void>> setString(String key, String value) async =>
      Result.success(null);

  @override
  Future<Result<String?>> getString(String key) async => Result.success(null);

  @override
  Future<Result<void>> setInt(String key, int value) async =>
      Result.success(null);

  @override
  Future<Result<int?>> getInt(String key) async => Result.success(null);

  @override
  Future<Result<void>> setBool(String key, bool value) async =>
      Result.success(null);

  @override
  Future<Result<bool?>> getBool(String key) async => Result.success(null);

  @override
  Future<Result<void>> setDouble(String key, double value) async =>
      Result.success(null);

  @override
  Future<Result<double?>> getDouble(String key) async => Result.success(null);

  @override
  Future<Result<void>> setStringList(String key, List<String> value) async =>
      Result.success(null);

  @override
  Future<Result<List<String>?>> getStringList(String key) async =>
      Result.success(null);

  @override
  Future<Result<void>> setObject(
    String key,
    Map<String, dynamic> value,
  ) async => Result.success(null);

  @override
  Future<Result<Map<String, dynamic>?>> getObject(String key) async =>
      Result.success(null);

  @override
  Future<Result<void>> setSecureString(String key, String value) async =>
      Result.success(null);

  @override
  Future<Result<String?>> getSecureString(String key) async =>
      Result.success(null);

  @override
  Future<Result<void>> remove(String key) async => Result.success(null);

  @override
  Future<Result<void>> removeSecure(String key) async => Result.success(null);

  @override
  Future<Result<bool>> containsKey(String key) async => Result.success(false);

  @override
  Future<Result<Set<String>>> getKeys() async => Result.success(<String>{});
}

class _NoopLoggerService extends Fake implements ILoggerService {
  Result<void> _okVoid() => Result.success(null);

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _okVoid();

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _okVoid();

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async => _okVoid();

  @override
  Future<Result<void>> clearLogs() async => _okVoid();

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async => Result.success(const <LogEntry>[]);

  @override
  Future<void> debugSync(String userId) async {}

  @override
  Future<void> debugSyncSuccess(String userId) async {}

  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}

  @override
  Future<void> debugCallingGetCurrentUser() async {}

  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}

  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}

  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}

  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}

  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}
}

class _NoopAnalyticsRepository extends Fake implements IAnalyticsRepository {
  Result<void> _okVoid() => Result.success(null);

  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async => _okVoid();

  @override
  Future<Result<void>> logScreenView({
    required String screenName,
    String? screenClass,
  }) async => _okVoid();
}

class _MockUserApiDatasource extends UserApiDatasource {
  _MockUserApiDatasource() : super(_MockApiClient());

  Result<FirebaseExchangeResponse> exchangeResult = Result.success(
    const FirebaseExchangeResponse(
      userId: 'user-1',
      accessToken: 'access-token',
      expiresAt: '2026-06-14T00:00:00Z',
      requiresProfileCompletion: false,
      created: true,
      refreshToken: 'refresh-token',
      refreshExpiresAt: '2026-07-14T00:00:00Z',
      email: 'seller@example.com',
    ),
  );

  Result<UserApiResponse> currentUserResult = Result.success(
    UserApiResponse.fromJson({
      'id': 'user-1',
      'email': 'seller@example.com',
      'username': 'seller-one',
      'account_status': 'active',
      'roles': ['seller'],
      'has_seller_profile': true,
      'seller_subscription_status': 'expired',
      'has_market_authority': false,
      'is_email_verified': true,
      'created_at': '2026-06-01T00:00:00Z',
      'updated_at': '2026-06-02T00:00:00Z',
      'profile': {
        'id': 'user-1',
        'username': 'seller-one',
        'bio': 'bio',
        'avatar_url': 'https://example.com/avatar.png',
        'followers_count': 1,
        'following_count': 2,
        'preferred_lang': 'en',
      },
    }),
  );

  @override
  Future<Result<FirebaseExchangeResponse>> exchangeFirebaseSession({
    required String firebaseIdToken,
    String? username,
  }) async {
    return exchangeResult;
  }

  @override
  Future<Result<UserApiResponse>> getCurrentUser() async => currentUserResult;
}

class _MockAuthApiDatasource extends AuthApiDatasource {
  _MockAuthApiDatasource() : super(_MockApiClient());

  Result<FirebaseExchangeCompleteResponse> completeResult = Result.success(
    const FirebaseExchangeCompleteResponse(
      userId: 'user-1',
      accessToken: 'access-token',
      refreshToken: 'refresh-token',
      expiresAt: '2026-06-14T00:00:00Z',
      refreshExpiresAt: '2026-07-14T00:00:00Z',
      requiresProfileCompletion: false,
      created: true,
    ),
  );

  @override
  Future<Result<FirebaseExchangeCompleteResponse>> completeProfile({
    required String username,
    required String restrictedToken,
  }) async {
    return completeResult;
  }
}

class _MockUserSyncService extends UserSyncService {
  _MockUserSyncService({FirebaseAuth? firebaseAuth})
    : super(
        firebaseAuth:
            firebaseAuth ??
            _MockFirebaseAuth(
              currentUserValue: _MockFirebaseUser(idToken: 'firebase-token'),
            ),
        datasource: _MockUserApiDatasource(),
      );

  Result<SyncUserResult> syncResult = Result.error('backend failure');

  @override
  Future<Result<SyncUserResult>> syncUser({
    required String username,
    String? phoneNumber,
  }) async {
    return syncResult;
  }
}

class _FailingAuthRepository extends Fake implements IAuthRepository {
  Result<AuthUser> completeProfileResult = Result.error('backend failure');

  @override
  Future<Result<FirebasePrincipal>> signInWithEmail({
    required String email,
    required String password,
  }) async => Result.error('not used');

  @override
  Future<Result<void>> signInWithGoogle({AuthCredential? pendingGoogleCredential}) async => Result.success(null);

  @override
  Future<Result<FirebasePrincipal>> signUpWithEmail({
    required String email,
    required String password,
  }) async => Result.error('not used');

  @override
  Future<Result<void>> signOut() async => Result.success(null);

  @override
  Future<Result<void>> logoutCurrentSession({
    required String refreshToken,
    String? fcmToken,
    String? deviceId,
  }) async => Result.success(null);

  @override
  Future<Result<void>> logoutAllSessions({
    bool deactivateFcmTokens = true,
  }) async => Result.success(null);

  @override
  Future<Result<List<AuthSessionDto>>> getActiveSessions() async =>
      Result.success(const <AuthSessionDto>[]);

  @override
  Future<Result<void>> revokeSession(String familyId) async =>
      Result.success(null);

  @override
  Future<Result<void>> resetPassword({required String email}) async =>
      Result.success(null);

  @override
  Future<Result<UserProfilePatch>> updateProfile({
    String? photoUrl,
    String? phoneNumber,
    DateTime? phoneVerifiedAt,
    String? username,
    String? bio,
    String? location,
    String? coverPhotoUrl,
    String? instagramHandle,
    String? facebookHandle,
    String? tiktokHandle,
    String? twitterHandle,
    DateTime? dateOfBirth,
  }) async => Result.error('not used');

  @override
  Future<Result<AuthUser>> completeProfile({required String username}) async {
    return completeProfileResult;
  }

  @override
  Future<Result<AuthUser?>> getUserById(String userId) async =>
      Result.success(null);

  @override
  Future<Result<void>> deactivateAccount({
    required String userId,
    required String reason,
  }) async => Result.success(null);

  @override
  Future<Result<void>> deleteAccount() async => Result.success(null);

  @override
  Future<Result<void>> sendEmailVerification() async => Result.success(null);

  @override
  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async => Result.success(null);
}

class _TestAuthController extends AuthController {
  _TestAuthController({required this.firebaseUser});

  final User? firebaseUser;
  int signOutCalls = 0;

  @override
  User? get activeFirebaseUser => firebaseUser;

  @override
  bool get shouldInitializeAuthListener => false;

  @override
  Future<void> performFirebaseSignOut() async {
    signOutCalls++;
  }
}

ProviderContainer _container({
  required AuthController controller,
  required IAuthRepository authRepository,
  required UserSyncService userSyncService,
  required ILoggerService logger,
  required IAnalyticsRepository analyticsRepository,
  required ILocalStorageService localStorageService,
}) {
  return ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(() => controller),
      auth_data.authRepositoryProvider.overrideWithValue(authRepository),
      userSyncServiceProvider.overrideWithValue(userSyncService),
      loggerServiceProvider.overrideWithValue(logger),
      coreAnalyticsRepositoryProvider.overrideWithValue(analyticsRepository),
      localStorageServiceProvider.overrideWithValue(localStorageService),
    ],
  );
}

void main() {
  test(
    'exchange mismatch clears stored tokens and returns a session mismatch error',
    () async {
      final authAuth = _MockFirebaseAuth(
        currentUserValue: _MockFirebaseUser(idToken: 'firebase-token'),
      );
      final datasource = _MockUserApiDatasource()
        ..exchangeResult = Result.success(
          const FirebaseExchangeResponse(
            userId: 'session-user-1',
            accessToken: 'access-token',
            expiresAt: '2026-06-14T00:00:00Z',
            requiresProfileCompletion: false,
            created: true,
            refreshToken: 'refresh-token',
            refreshExpiresAt: '2026-07-14T00:00:00Z',
            email: 'seller@example.com',
          ),
        )
        ..currentUserResult = Result.success(
          UserApiResponse.fromJson({
            'id': 'current-user-2',
            'email': 'seller@example.com',
            'username': 'seller-two',
            'account_status': 'active',
            'roles': ['seller'],
            'has_seller_profile': true,
            'seller_subscription_status': 'expired',
            'has_market_authority': false,
            'is_email_verified': true,
            'created_at': '2026-06-01T00:00:00Z',
            'updated_at': '2026-06-02T00:00:00Z',
            'profile': {
              'id': 'current-user-2',
              'username': 'seller-two',
              'bio': 'bio',
              'avatar_url': 'https://example.com/avatar.png',
              'followers_count': 1,
              'following_count': 2,
              'preferred_lang': 'en',
            },
          }),
        );
      final localStorage = _RecordingLocalStorageService();
      final service = UserSyncService(
        firebaseAuth: authAuth,
        datasource: datasource,
        localStorage: localStorage,
        logger: _NoopLoggerService(),
      );

      final result = await service.syncUser(username: 'seller-two');

      expect(result.isError, isTrue);
      expect(result.errorCode, 'SESSION_USER_MISMATCH');
      expect(localStorage.authToken, isNull);
      expect(localStorage.refreshToken, isNull);
      expect(localStorage.clearAuthTokenCalls, 1);
      expect(localStorage.clearRefreshTokenCalls, 1);
    },
  );

  test(
    'exchange transient /users/me failure keeps stored tokens for retry',
    () async {
      final authAuth = _MockFirebaseAuth(
        currentUserValue: _MockFirebaseUser(idToken: 'firebase-token'),
      );
      final datasource = _MockUserApiDatasource()
        ..currentUserResult = Result.error(
          'Backend unavailable',
          statusCode: 503,
        );
      final localStorage = _RecordingLocalStorageService();
      final service = UserSyncService(
        firebaseAuth: authAuth,
        datasource: datasource,
        localStorage: localStorage,
        logger: _NoopLoggerService(),
      );

      final result = await service.syncUser(username: 'seller-two');

      expect(result.isError, isTrue);
      expect(result.statusCode, 503);
      expect(localStorage.authToken, 'access-token');
      expect(localStorage.refreshToken, 'refresh-token');
      expect(localStorage.clearAuthTokenCalls, 0);
      expect(localStorage.clearRefreshTokenCalls, 0);
    },
  );

  test(
    'complete-profile mismatch clears stored tokens and returns a session mismatch error',
    () async {
      final datasource = _MockAuthApiDatasource()
        ..completeResult = Result.success(
          const FirebaseExchangeCompleteResponse(
            userId: 'session-user-1',
            accessToken: 'access-token',
            refreshToken: 'refresh-token',
            expiresAt: '2026-06-14T00:00:00Z',
            refreshExpiresAt: '2026-07-14T00:00:00Z',
            requiresProfileCompletion: false,
            created: true,
          ),
        );
      // Canonical /users/me authority: UserApiDatasource (the former
      // AuthApiDatasource duplicate was purged).
      final userDatasource = _MockUserApiDatasource()
        ..currentUserResult = Result.success(
          UserApiResponse.fromJson({
            'id': 'current-user-2',
            'email': 'seller@example.com',
            'username': 'seller-two',
            'account_status': 'active',
            'roles': ['seller'],
            'has_seller_profile': true,
            'seller_subscription_status': 'expired',
            'has_market_authority': false,
            'is_email_verified': true,
            'created_at': '2026-06-01T00:00:00Z',
            'updated_at': '2026-06-02T00:00:00Z',
            'profile': {
              'id': 'current-user-2',
              'username': 'seller-two',
              'bio': 'bio',
              'avatar_url': 'https://example.com/avatar.png',
              'followers_count': 1,
              'following_count': 2,
              'preferred_lang': 'en',
            },
          }),
        );
      final localStorage = _RecordingLocalStorageService()
        ..restrictedToken = 'restricted-token';
      final repository = AuthProfileRepository(
        firebaseAuth: _MockFirebaseAuth(
          currentUserValue: _MockFirebaseUser(idToken: 'firebase-token'),
        ),
        apiDatasource: datasource,
        userDatasource: userDatasource,
        localStorage: localStorage,
      );

      final result = await repository.completeProfile(username: 'seller-two');

      expect(result.isError, isTrue);
      expect(result.errorCode, 'SESSION_USER_MISMATCH');
      expect(localStorage.authToken, isNull);
      expect(localStorage.refreshToken, isNull);
      expect(localStorage.clearAuthTokenCalls, 1);
      expect(localStorage.clearRefreshTokenCalls, 1);
    },
  );

  test(
    'complete-profile success persists the full session pair, consumes the '
    'restricted token, and hydrates via the canonical /users/me authority',
    () async {
      final datasource = _MockAuthApiDatasource()
        ..completeResult = Result.success(
          const FirebaseExchangeCompleteResponse(
            userId: 'session-user-1',
            accessToken: 'platform-access-token',
            refreshToken: 'platform-refresh-token',
            expiresAt: '2026-06-14T00:00:00Z',
            refreshExpiresAt: '2026-07-14T00:00:00Z',
            requiresProfileCompletion: false,
            created: false,
          ),
        );
      final userDatasource = _MockUserApiDatasource()
        ..currentUserResult = Result.success(
          UserApiResponse.fromJson({
            'id': 'session-user-1',
            'email': 'seller@example.com',
            'username': 'completed-user',
            'account_status': 'active',
            'roles': ['seller'],
            'has_seller_profile': false,
            'seller_subscription_status': 'none',
            'has_market_authority': false,
            'is_email_verified': true,
            'created_at': '2026-06-01T00:00:00Z',
            'updated_at': '2026-06-02T00:00:00Z',
            'profile': {
              'id': 'session-user-1',
              'username': 'completed-user',
              'bio': 'bio',
              'avatar_url': 'https://example.com/avatar.png',
              'followers_count': 0,
              'following_count': 0,
              'preferred_lang': 'en',
            },
          }),
        );
      final localStorage = _RecordingLocalStorageService()
        ..restrictedToken = 'restricted-token';
      final repository = AuthProfileRepository(
        firebaseAuth: _MockFirebaseAuth(
          currentUserValue: _MockFirebaseUser(idToken: 'firebase-token'),
        ),
        apiDatasource: datasource,
        userDatasource: userDatasource,
        localStorage: localStorage,
      );

      final result = await repository.completeProfile(username: 'completed-user');

      expect(result.isSuccess, isTrue);
      // ONE identity, ONE session: the returned profile is the SAME backend
      // user the restricted token was issued for (consistency guard passed).
      expect(result.data?.id, 'session-user-1');
      expect(result.data?.username, 'completed-user');
      // The full access+refresh pair from the completion response replaces
      // the restricted credential via the canonical boundary.
      expect(localStorage.authToken, 'platform-access-token');
      expect(localStorage.refreshToken, 'platform-refresh-token');
      expect(localStorage.setAuthTokenCalls, 1);
      expect(localStorage.setRefreshTokenCalls, 1);
      // The restricted completion credential is CONSUMED â€” no stale second
      // credential may survive the session upgrade.
      expect(localStorage.clearRestrictedTokenCalls, 1);
      expect(localStorage.restrictedToken, isNull);
      // Nothing clobbered the freshly persisted pair.
      expect(localStorage.clearAuthTokenCalls, 0);
      expect(localStorage.clearRefreshTokenCalls, 0);
    },
  );

  test(
    'complete-profile transient /users/me failure preserves stored tokens',
    () async {
      final datasource = _MockAuthApiDatasource()
        ..completeResult = Result.success(
          const FirebaseExchangeCompleteResponse(
            userId: 'session-user-1',
            accessToken: 'access-token',
            refreshToken: 'refresh-token',
            expiresAt: '2026-06-14T00:00:00Z',
            refreshExpiresAt: '2026-07-14T00:00:00Z',
            requiresProfileCompletion: false,
            created: true,
          ),
        );
      final userDatasource = _MockUserApiDatasource()
        ..currentUserResult = Result.error(
          'Backend unavailable',
          statusCode: 503,
        );
      final localStorage = _RecordingLocalStorageService()
        ..restrictedToken = 'restricted-token';
      final repository = AuthProfileRepository(
        firebaseAuth: _MockFirebaseAuth(
          currentUserValue: _MockFirebaseUser(idToken: 'firebase-token'),
        ),
        apiDatasource: datasource,
        userDatasource: userDatasource,
        localStorage: localStorage,
      );

      final result = await repository.completeProfile(username: 'seller-two');

      expect(result.isError, isTrue);
      expect(result.statusCode, 503);
      expect(localStorage.authToken, 'access-token');
      expect(localStorage.refreshToken, 'refresh-token');
      expect(localStorage.clearAuthTokenCalls, 0);
      expect(localStorage.clearRefreshTokenCalls, 0);
    },
  );

  test(
    'refreshAuthState turns exchange mismatch into a deterministic error state',
    () async {
      final userSyncService = _MockUserSyncService()
        ..syncResult = Result.error(
          'Backend session user mismatch',
          code: 'SESSION_USER_MISMATCH',
          statusCode: 409,
        );
      final controller = _TestAuthController(
        firebaseUser: _MockFirebaseUser(idToken: 'firebase-token'),
      );
      final container = _container(
        controller: controller,
        authRepository: _FailingAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: _RecordingLocalStorageService(),
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      controller.state = const AuthState.loading();

      await controller.refreshAuthState();
      await Future<void>.delayed(const Duration(milliseconds: 1));

      expect(controller.state, isA<AuthStateError>());
      expect(controller.signOutCalls, 0);
    },
  );

  test(
    'completeProfile preserves requires-profile-completion on transient /users/me failure',
    () async {
      final authRepository = _FailingAuthRepository()
        ..completeProfileResult = Result.error(
          'Backend unavailable',
          code: 'BACKEND_UNREACHABLE',
          statusCode: 503,
        );
      final controller = _TestAuthController(
        firebaseUser: _MockFirebaseUser(idToken: 'firebase-token'),
      );
      final container = _container(
        controller: controller,
        authRepository: authRepository,
        userSyncService: _MockUserSyncService(),
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: _RecordingLocalStorageService(),
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      controller.state = const AuthState.requiresProfileCompletion(
        userId: 'session-user-1',
        email: 'seller@example.com',
      );

      final result = await controller.completeProfile(username: 'seller-two');

      expect(result.success, isFalse);
      expect(controller.state, isA<AuthStateRequiresProfileCompletion>());
    },
  );

  test(
    'completeProfile mismatch becomes a deterministic error state',
    () async {
      final authRepository = _FailingAuthRepository()
        ..completeProfileResult = Result.error(
          'Backend session user mismatch',
          code: 'SESSION_USER_MISMATCH',
          statusCode: 409,
        );
      final controller = _TestAuthController(
        firebaseUser: _MockFirebaseUser(idToken: 'firebase-token'),
      );
      final container = _container(
        controller: controller,
        authRepository: authRepository,
        userSyncService: _MockUserSyncService(),
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: _RecordingLocalStorageService(),
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      controller.state = const AuthState.requiresProfileCompletion(
        userId: 'session-user-1',
        email: 'seller@example.com',
      );

      final result = await controller.completeProfile(username: 'seller-two');

      expect(result.success, isFalse);
      expect(controller.state, isA<AuthStateError>());
    },
  );
}
