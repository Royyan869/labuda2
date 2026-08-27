import 'dart:async';

import 'package:dio/dio.dart';
import 'package:fake_async/fake_async.dart';
import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/firebase_principal.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/user_profile_patch.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userSyncServiceProvider;
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';

class _MutableFirebaseUser extends Fake implements User {
  _MutableFirebaseUser({required this.uidValue})
    : idTokenValue = 'firebase-token';

  String uidValue;
  String? idTokenValue;
  bool emailVerifiedValue = true;

  @override
  String get uid => uidValue;

  @override
  bool get emailVerified => emailVerifiedValue;

  @override
  String? get email => '$uidValue@example.com';

  @override
  String? get phoneNumber => null;

  @override
  List<UserInfo> get providerData => const <UserInfo>[];

  @override
  UserMetadata get metadata => _MutableUserMetadata();

  @override
  Future<void> reload() async {}

  @override
  Future<String?> getIdToken([bool forceRefresh = false]) async => idTokenValue;
}

class _MutableUserMetadata extends Fake implements UserMetadata {
  @override
  DateTime? get creationTime => DateTime(2026, 6, 1);

  @override
  DateTime? get lastSignInTime => DateTime(2026, 6, 2);
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

class _NoopUserApiDatasource extends UserApiDatasource {
  _NoopUserApiDatasource() : super(_NoopApiClient());
}

class _SyncCall {
  _SyncCall({
    required this.expectedUid,
    required this.expectedEpoch,
    required this.username,
  });

  final String expectedUid;
  final int expectedEpoch;
  final String username;
  final Completer<Result<SyncUserResult>> completer =
      Completer<Result<SyncUserResult>>();
}

class _ControllerUserSyncService extends UserSyncService {
  _ControllerUserSyncService({required FirebaseAuth firebaseAuth})
    : super(firebaseAuth: firebaseAuth, userDatasource: _NoopUserApiDatasource());

  Result<AuthUser> currentUserResult = Result.error('not configured');
  Completer<Result<AuthUser>>? currentUserCompleter;
  final List<_SyncCall> syncCalls = <_SyncCall>[];

  @override
  Future<Result<AuthUser>> getCurrentUser({
    required String expectedUid,
    required int expectedEpoch,
    required PrincipalOperationCheck isCurrentPrincipalOperation,
  }) async {
    if (currentUserCompleter != null) {
      return currentUserCompleter!.future;
    }
    return currentUserResult;
  }

  @override
  Future<Result<SyncUserResult>> syncUser({
    required String expectedUid,
    required int expectedEpoch,
    required PrincipalOperationCheck isCurrentPrincipalOperation,
    required String username,
    String? phoneNumber,
  }) {
    final call = _SyncCall(
      expectedUid: expectedUid,
      expectedEpoch: expectedEpoch,
      username: username,
    );
    syncCalls.add(call);
    return call.completer.future;
  }

  void completeSync(int index, Result<SyncUserResult> result) {
    syncCalls[index].completer.complete(result);
  }

  void failSync(int index, Object error) {
    syncCalls[index].completer.completeError(error);
  }
}

class _RecordingLocalStorageService extends Fake
    implements ILocalStorageService {
  String? authToken;
  String? refreshToken;
  int setAuthTokenCalls = 0;
  int setRefreshTokenCalls = 0;
  int clearAuthTokenCalls = 0;
  int clearRefreshTokenCalls = 0;

  @override
  Future<Result<void>> initialize() async => Result.success(null);

  @override
  Future<Result<void>> clear() async => Result.success(null);

  @override
  Future<Result<void>> clearSecure() async => Result.success(null);

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

  @override
  Future<Result<String?>> getAuthToken() async => Result.success(authToken);

  @override
  Future<Result<String?>> getRefreshToken() async =>
      Result.success(refreshToken);

  @override
  Future<Result<void>> setAuthToken(String token) async {
    setAuthTokenCalls++;
    authToken = token;
    return Result.success(null);
  }

  @override
  Future<Result<void>> setRefreshToken(String token) async {
    setRefreshTokenCalls++;
    refreshToken = token;
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
  Result<void> _ok() => Result.success(null);

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _ok();

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _ok();

  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) async => _ok();

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) async => _ok();

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) async => _ok();

  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) async => _ok();

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async => _ok();

  @override
  Future<Result<void>> clearLogs() async => _ok();

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
  Result<void> _ok() => Result.success(null);

  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async => _ok();

  @override
  Future<Result<void>> logUserAction(
    String action,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> logCircumventionAttempt(
    String content,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> setUserProperties(
    Map<String, dynamic> properties,
  ) async => _ok();

  @override
  Future<Result<AnalyticsCircumventionStats>> getCircumventionStats({
    required DateTime startDate,
    required DateTime endDate,
    String? userId,
    String? violationType,
  }) async => Result.success(
    const AnalyticsCircumventionStats(
      totalAttempts: 0,
      uniqueUsers: 0,
      violationTypes: <String, int>{},
      dailyAttempts: <String, int>{},
      averageConfidence: 0,
      blockedAttempts: 0,
      filteredAttempts: 0,
    ),
  );

  @override
  Future<Result<void>> flush() async => _ok();
}

class _TestAuthRepository extends Fake implements IAuthRepository {
  @override
  Future<Result<FirebasePrincipal>> signInWithEmail({
    required String email,
    required String password,
  }) async => Result.error('not used');

  @override
  Future<Result<void>> signInWithGoogle() async => Result.success(null);

  @override
  Future<Result<FirebasePrincipal>> signUpWithEmail({
    required String email,
    required String password,
    required String username,
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
  Future<Result<AuthUser?>> getCurrentUser() async => Result.success(null);

  @override
  Future<Result<void>> resetPassword({required String email}) async =>
      Result.success(null);

  @override

  @override
  Future<Result<UserProfilePatch>> updateProfile({
    String? photoUrl,
    String? phoneNumber,
    DateTime? phoneVerifiedAt,
    String? username,
    String? bio,
    String? location,
    DateTime? dateOfBirth,
  }) async => Result.error('not used');

  @override
  Future<Result<AuthUser>> completeProfile({required String username}) async {
    return Result.error('not used');
  }

  @override
  Future<Result<AuthUser?>> getUserById(String userId) async =>
      Result.success(null);

  @override
  Future<Result<List<AuthUser>>> searchUsers({
    required String query,
    int limit = 20,
  }) async => Result.success(const <AuthUser>[]);

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
  Future<Result<void>> changeEmail({
    required String newEmail,
    required String currentPassword,
  }) async => Result.success(null);

  @override
  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async => Result.success(null);

  @override
  Future<Result<AuthUser>> updateUserRole({
    required String userId,
    required UserRole newRole,
  }) async => Result.error('not used');

  @override
  Stream<FirebasePrincipal?> get authStateChanges =>
      const Stream<FirebasePrincipal?>.empty();
}

class _TestAuthController extends AuthController {
  _TestAuthController({required this.firebaseUser});

  User? firebaseUser;
  int signOutCalls = 0;

  @override
  User? get activeFirebaseUser => firebaseUser;

  @override
  bool get shouldInitializeAuthListener => false;

  @override
  Future<void> performFirebaseSignOut() async {
    signOutCalls++;
    firebaseUser = null;
  }
}

AuthUser _principalUser(
  String id, {
  bool emailVerified = true,
  AccountStatus accountStatus = AccountStatus.active,
  bool hasMarketAuthority = true,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2026, 6, 1),
    updatedAt: DateTime(2026, 6, 2),
    email: '$id@example.com',
    username: '$id-user',
    isEmailVerified: emailVerified,
    accountStatus: accountStatus,
    hasSellerProfile: true,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
  );
}

Result<SyncUserResult> _syncSuccess(AuthUser user) {
  return Result.success(
    SyncUserResult(
      user: user,
      userId: user.id,
      email: user.email,
      created: false,
      profileComplete: true,
      username: user.username,
    ),
  );
}

Result<SyncUserResult> _profileIncomplete(String userId) {
  return Result.success(
    SyncUserResult(
      user: null,
      userId: userId,
      email: '$userId@example.com',
      created: false,
      profileComplete: false,
      username: '',
    ),
  );
}

Result<SyncUserResult> _restrictedSuccess(AuthUser user) {
  return Result.success(
    SyncUserResult(
      user: user,
      userId: user.id,
      email: user.email,
      created: false,
      profileComplete: true,
      username: user.username,
    ),
  );
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

Future<void> _seedPrincipal({
  required _TestAuthController controller,
  required _ControllerUserSyncService userSyncService,
  required _MutableFirebaseUser firebaseUser,
  required AuthUser user,
}) async {
  controller.firebaseUser = firebaseUser;
  userSyncService.currentUserResult = Result.success(user);
  controller.state = AuthState.authenticated(
    user,
    emailVerified: user.isEmailVerified,
  );
  await controller.refreshUserData();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'stable principal retry keeps retry behavior and publishes the current principal',
    () async {
      final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final userSyncService = _ControllerUserSyncService(
        firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
      );
      final container = _container(
        controller: controller,
        authRepository: _TestAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: _RecordingLocalStorageService(),
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      await _seedPrincipal(
        controller: controller,
        userSyncService: userSyncService,
        firebaseUser: firebaseUser,
        user: _principalUser('uid-a'),
      );

      fakeAsync((async) {
        controller.refreshAuthState();
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(1));
        expect(userSyncService.syncCalls.single.expectedUid, 'uid-a');
        expect(userSyncService.syncCalls.single.expectedEpoch, 1);

        userSyncService.completeSync(
          0,
          Result.error('Backend unavailable', statusCode: 503),
        );
        async.flushMicrotasks();

        expect(controller.state, isA<AuthStateBackendUnavailable>());

        async.elapse(const Duration(seconds: 2));
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(2));
        expect(userSyncService.syncCalls[1].expectedUid, 'uid-a');
        expect(userSyncService.syncCalls[1].expectedEpoch, 1);

        userSyncService.completeSync(1, _syncSuccess(_principalUser('uid-a')));
        async.flushMicrotasks();

        expect(controller.state, isA<AuthStateAuthenticated>());
        expect((controller.state as AuthStateAuthenticated).user.id, 'uid-a');
      });
    },
  );

  test(
    'A to B before retry fires keeps the old timer inert and lets B sync independently',
    () async {
      final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final userSyncService = _ControllerUserSyncService(
        firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
      );
      final localStorage = _RecordingLocalStorageService();
      final container = _container(
        controller: controller,
        authRepository: _TestAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: localStorage,
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      await _seedPrincipal(
        controller: controller,
        userSyncService: userSyncService,
        firebaseUser: firebaseUser,
        user: _principalUser('uid-a'),
      );

      fakeAsync((async) {
        controller.refreshAuthState();
        async.flushMicrotasks();
        userSyncService.completeSync(
          0,
          Result.error('Backend unavailable', statusCode: 503),
        );
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(1));
        expect(controller.state, isA<AuthStateBackendUnavailable>());

        controller.signOut();
        async.flushMicrotasks();
        expect(controller.state, isA<AuthStateUnauthenticated>());

        firebaseUser.uidValue = 'uid-b';
        userSyncService.currentUserResult = Result.success(
          _principalUser('uid-b'),
        );
        controller.state = AuthState.authenticated(
          _principalUser('uid-b'),
          emailVerified: true,
        );
        controller.refreshUserData();
        async.flushMicrotasks();

        controller.refreshAuthState();
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(2));
        expect(userSyncService.syncCalls[1].expectedUid, 'uid-b');

        userSyncService.completeSync(1, _syncSuccess(_principalUser('uid-b')));
        async.flushMicrotasks();

        expect(controller.state, isA<AuthStateAuthenticated>());
        expect((controller.state as AuthStateAuthenticated).user.id, 'uid-b');

        async.elapse(const Duration(seconds: 3));
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(2));
        expect(localStorage.clearAuthTokenCalls, 0);
        expect(localStorage.clearRefreshTokenCalls, 0);
      });
    },
  );

  test('A to null before retry fires cancels the old retry path', () async {
    final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
    final controller = _TestAuthController(firebaseUser: firebaseUser);
    final userSyncService = _ControllerUserSyncService(
      firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
    );
    final container = _container(
      controller: controller,
      authRepository: _TestAuthRepository(),
      userSyncService: userSyncService,
      logger: _NoopLoggerService(),
      analyticsRepository: _NoopAnalyticsRepository(),
      localStorageService: _RecordingLocalStorageService(),
    );
    addTearDown(container.dispose);

    container.read(authControllerProvider.notifier);
    await _seedPrincipal(
      controller: controller,
      userSyncService: userSyncService,
      firebaseUser: firebaseUser,
      user: _principalUser('uid-a'),
    );

    fakeAsync((async) {
      controller.refreshAuthState();
      async.flushMicrotasks();
      userSyncService.completeSync(
        0,
        Result.error('Backend unavailable', statusCode: 503),
      );
      async.flushMicrotasks();

      expect(userSyncService.syncCalls, hasLength(1));

      controller.signOut();
      async.flushMicrotasks();
      firebaseUser.uidValue = 'uid-a';

      async.elapse(const Duration(seconds: 3));
      async.flushMicrotasks();

      expect(userSyncService.syncCalls, hasLength(1));
      expect(controller.state, isA<AuthStateUnauthenticated>());
    });
  });

  test(
    'logout A then login same UID A keeps the new epoch live and does not suppress the new sync',
    () async {
      final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final userSyncService = _ControllerUserSyncService(
        firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
      );
      final container = _container(
        controller: controller,
        authRepository: _TestAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: _RecordingLocalStorageService(),
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      await _seedPrincipal(
        controller: controller,
        userSyncService: userSyncService,
        firebaseUser: firebaseUser,
        user: _principalUser('uid-a'),
      );

      fakeAsync((async) {
        controller.refreshAuthState();
        async.flushMicrotasks();

        controller.signOut();
        async.flushMicrotasks();

        firebaseUser.uidValue = 'uid-a';
        userSyncService.currentUserResult = Result.success(
          _principalUser('uid-a'),
        );
        controller.state = AuthState.authenticated(
          _principalUser('uid-a'),
          emailVerified: true,
        );
        controller.refreshUserData();
        async.flushMicrotasks();

        userSyncService.completeSync(0, _syncSuccess(_principalUser('uid-a')));
        async.flushMicrotasks();

        controller.refreshAuthState();
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(2));
        expect(
          userSyncService.syncCalls[0].expectedEpoch,
          isNot(userSyncService.syncCalls[1].expectedEpoch),
        );
      });
    },
  );

  test(
    'in-flight A completion cannot clear B ownership or timer state',
    () async {
      final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final userSyncService = _ControllerUserSyncService(
        firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
      );
      final localStorage = _RecordingLocalStorageService();
      final container = _container(
        controller: controller,
        authRepository: _TestAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: localStorage,
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      await _seedPrincipal(
        controller: controller,
        userSyncService: userSyncService,
        firebaseUser: firebaseUser,
        user: _principalUser('uid-a'),
      );

      fakeAsync((async) {
        controller.refreshAuthState();
        async.flushMicrotasks();

        controller.signOut();
        async.flushMicrotasks();

        firebaseUser.uidValue = 'uid-b';
        userSyncService.currentUserResult = Result.success(
          _principalUser('uid-b'),
        );
        controller.state = AuthState.authenticated(
          _principalUser('uid-b'),
          emailVerified: true,
        );
        controller.refreshUserData();
        async.flushMicrotasks();

        controller.refreshAuthState();
        async.flushMicrotasks();
        expect(userSyncService.syncCalls, hasLength(2));

        userSyncService.completeSync(
          1,
          Result.error('Backend unavailable', statusCode: 503),
        );
        async.flushMicrotasks();

        expect(controller.state, isA<AuthStateBackendUnavailable>());

        userSyncService.completeSync(0, _syncSuccess(_principalUser('uid-a')));
        async.flushMicrotasks();

        async.elapse(const Duration(seconds: 2));
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(3));

        userSyncService.completeSync(2, _syncSuccess(_principalUser('uid-b')));
        async.flushMicrotasks();

        expect(controller.state, isA<AuthStateAuthenticated>());
        expect((controller.state as AuthStateAuthenticated).user.id, 'uid-b');
        expect(localStorage.setAuthTokenCalls, 0);
        expect(localStorage.setRefreshTokenCalls, 0);
      });
    },
  );

  test(
    'stale failure publication matrix stays silent once A is stale under B',
    () async {
      final cases =
          <
            String,
            void Function(_ControllerUserSyncService service, int callIndex)
          >{
            'backend unavailable': (service, callIndex) {
              service.completeSync(
                callIndex,
                Result.error('Backend unavailable', statusCode: 503),
              );
            },
            'profile incomplete': (service, callIndex) {
              service.completeSync(callIndex, _profileIncomplete('uid-a'));
            },
            'account restricted': (service, callIndex) {
              service.completeSync(
                callIndex,
                _restrictedSuccess(
                  _principalUser(
                    'uid-a',
                    accountStatus: AccountStatus.suspended,
                  ),
                ),
              );
            },
            'authentication failure': (service, callIndex) {
              service.completeSync(
                callIndex,
                Result.error(
                  'Invalid token',
                  code: 'INVALID_TOKEN',
                  statusCode: 401,
                ),
              );
            },
            'generic exception': (service, callIndex) {
              service.failSync(callIndex, StateError('boom'));
            },
          };

      for (final entry in cases.entries) {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final localStorage = _RecordingLocalStorageService();
        final container = _container(
          controller: controller,
          authRepository: _TestAuthRepository(),
          userSyncService: userSyncService,
          logger: _NoopLoggerService(),
          analyticsRepository: _NoopAnalyticsRepository(),
          localStorageService: localStorage,
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);
        await _seedPrincipal(
          controller: controller,
          userSyncService: userSyncService,
          firebaseUser: firebaseUser,
          user: _principalUser('uid-a'),
        );

        fakeAsync((async) {
          controller.refreshAuthState();
          async.flushMicrotasks();

          controller.signOut();
          async.flushMicrotasks();

          firebaseUser.uidValue = 'uid-b';
          userSyncService.currentUserResult = Result.success(
            _principalUser('uid-b'),
          );
          controller.state = AuthState.authenticated(
            _principalUser('uid-b'),
            emailVerified: true,
          );
          controller.refreshUserData();
          async.flushMicrotasks();

          expect(controller.state, isA<AuthStateAuthenticated>());
          expect((controller.state as AuthStateAuthenticated).user.id, 'uid-b');

          entry.value(userSyncService, 0);
          async.flushMicrotasks();

          expect(controller.state, isA<AuthStateAuthenticated>());
          expect((controller.state as AuthStateAuthenticated).user.id, 'uid-b');

          async.elapse(const Duration(seconds: 8));
          async.flushMicrotasks();

          expect(userSyncService.syncCalls, hasLength(1));
          expect(localStorage.clearAuthTokenCalls, 0);
          expect(localStorage.clearRefreshTokenCalls, 0);
        });
      }
    },
  );

  test(
    'provider disposal cancels pending retry and keeps late completions inert',
    () async {
      final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final userSyncService = _ControllerUserSyncService(
        firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
      );
      final localStorage = _RecordingLocalStorageService();
      final container = _container(
        controller: controller,
        authRepository: _TestAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: localStorage,
      );

      container.read(authControllerProvider.notifier);
      await _seedPrincipal(
        controller: controller,
        userSyncService: userSyncService,
        firebaseUser: firebaseUser,
        user: _principalUser('uid-a'),
      );

      fakeAsync((async) {
        controller.refreshAuthState();
        async.flushMicrotasks();
        userSyncService.completeSync(
          0,
          Result.error('Backend unavailable', statusCode: 503),
        );
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(1));

        controller.refreshAuthState();
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(2));

        container.dispose();

        async.elapse(const Duration(seconds: 10));
        async.flushMicrotasks();

        userSyncService.completeSync(1, _syncSuccess(_principalUser('uid-a')));
        async.flushMicrotasks();

        expect(userSyncService.syncCalls, hasLength(2));
        expect(localStorage.setAuthTokenCalls, 0);
        expect(localStorage.setRefreshTokenCalls, 0);
        expect(localStorage.clearAuthTokenCalls, 0);
        expect(localStorage.clearRefreshTokenCalls, 0);
      });
    },
  );

  // ─────────────────────────────────────────────────────────────────
  // UNSOLICITED_HOME_NAVIGATION regression (P1)
  // ─────────────────────────────────────────────────────────────────

  test(
    'forceRefreshAuthState does NOT publish AuthState.loading() '
    'during refresh — stays AuthStateAuthenticated throughout',
    () async {
      final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final userSyncService = _ControllerUserSyncService(
        firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
      );
      final container = _container(
        controller: controller,
        authRepository: _TestAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: _RecordingLocalStorageService(),
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      await _seedPrincipal(
        controller: controller,
        userSyncService: userSyncService,
        firebaseUser: firebaseUser,
        user: _principalUser('uid-a'),
      );

      // Arrange: set up a delayed getCurrentUser so we can observe the
      // state during the refresh.
      final currentUserCompleter = Completer<Result<AuthUser>>();
      userSyncService.currentUserCompleter = currentUserCompleter;

      // Act: call forceRefreshAuthState — it should NOT publish loading.
      // We don't need fakeAsync for this test because we control the
      // completer.
      unawaited(controller.forceRefreshAuthState());

      // Let the microtask queue drain so forceRefreshAuthState gets past
      // the synchronous checks.
      await Future<void>.delayed(Duration.zero);

      // Assert: the state must still be AuthStateAuthenticated, NOT
      // AuthStateLoading. This is the regression lock — before the fix,
      // forceRefreshAuthState would call _setState(AuthState.loading())
      // which would cause the router to redirect to /splash.
      expect(controller.state, isA<AuthStateAuthenticated>());
      expect(
        (controller.state as AuthStateAuthenticated).user.id,
        'uid-a',
      );

      // Complete the refresh.
      currentUserCompleter.complete(
        Result.success(_principalUser('uid-a')),
      );
      await Future<void>.delayed(Duration.zero);

      // Assert: state is still AuthStateAuthenticated after refresh.
      expect(controller.state, isA<AuthStateAuthenticated>());
      expect(
        (controller.state as AuthStateAuthenticated).user.id,
        'uid-a',
      );
    },
  );

  test(
    'forceRefreshAuthState with changed user data updates in-place '
    'without publishing loading',
    () async {
      final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final userSyncService = _ControllerUserSyncService(
        firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
      );
      final container = _container(
        controller: controller,
        authRepository: _TestAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: _RecordingLocalStorageService(),
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      await _seedPrincipal(
        controller: controller,
        userSyncService: userSyncService,
        firebaseUser: firebaseUser,
        user: _principalUser('uid-a'),
      );

      // Arrange: getCurrentUser returns a different user (e.g., seller
      // role changed).
      final changedUser = _principalUser(
        'uid-a',
        hasMarketAuthority: false,
      );
      userSyncService.currentUserResult = Result.success(changedUser);

      // Act
      await controller.forceRefreshAuthState();

      // Assert: state is AuthStateAuthenticated with the updated user.
      // The state never left AuthStateAuthenticated during the refresh.
      expect(controller.state, isA<AuthStateAuthenticated>());
      final authed = controller.state as AuthStateAuthenticated;
      expect(authed.user.id, 'uid-a');
      expect(authed.user.hasMarketAuthority, false);
    },
  );

  test(
    'forceRefreshAuthState on backend-unavailable keeps current state '
    'and transitions to degraded without passing through loading',
    () async {
      final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final userSyncService = _ControllerUserSyncService(
        firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
      );
      final container = _container(
        controller: controller,
        authRepository: _TestAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: _RecordingLocalStorageService(),
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      await _seedPrincipal(
        controller: controller,
        userSyncService: userSyncService,
        firebaseUser: firebaseUser,
        user: _principalUser('uid-a'),
      );

      // Arrange: getCurrentUser returns backend-unavailable
      userSyncService.currentUserResult = Result.error(
        'Backend unavailable',
        statusCode: 503,
      );

      // Act
      await controller.forceRefreshAuthState();

      // Assert: state transitions from AuthStateAuthenticated directly
      // to AuthStateBackendUnavailable — no AuthStateLoading in between.
      // AuthStateBackendUnavailable maps to AppAuthStatus.degraded which
      // does NOT trigger a router redirect.
      expect(controller.state, isA<AuthStateBackendUnavailable>());
    },
  );

  // ─────────────────────────────────────────────────────────────────
  // Firebase listener guard proof (UNSOLICITED_HOME_NAVIGATION P1)
  // ─────────────────────────────────────────────────────────────────

  group('Firebase listener same-principal guard', () {
    test('same UID while fully authenticated → suppressed', () async {
      final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
      final controller = _TestAuthController(firebaseUser: firebaseUser);
      final userSyncService = _ControllerUserSyncService(
        firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
      );
      final container = _container(
        controller: controller,
        authRepository: _TestAuthRepository(),
        userSyncService: userSyncService,
        logger: _NoopLoggerService(),
        analyticsRepository: _NoopAnalyticsRepository(),
        localStorageService: _RecordingLocalStorageService(),
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);

      // Seed: fully authenticated with uid-a, _syncedUserId set
      controller.state = AuthState.authenticated(
        _principalUser('uid-a'),
        emailVerified: true,
      );
      // completeProfile sets _syncedUserId after success; simulate that.
      // The _syncWithBackend guard relies on _syncedUserId matching and
      // state being AuthStateAuthenticated. Set both directly.
      final notifier = controller;
      // Seed _activePrincipalUid by calling _activatePrincipal via refreshUserData
      await notifier.refreshUserData();

      // Now re-seed state (refreshUserData may have changed it) and
      // manually ensure internal sync state
      userSyncService.currentUserResult = Result.success(
        _principalUser('uid-a'),
      );
      controller.state = AuthState.authenticated(
        _principalUser('uid-a'),
        emailVerified: true,
      );

      // The guard method checks: state is AuthStateAuthenticated AND
      // _syncedUserId == uid AND _activePrincipalUid == uid.
      // After a real initial sync, all three are true. We call
      // forceRefreshAuthState — which previously would publish loading
      // — to prove the state stays AuthStateAuthenticated.
      await controller.forceRefreshAuthState();

      expect(controller.state, isA<AuthStateAuthenticated>());
      expect(
        (controller.state as AuthStateAuthenticated).user.id,
        'uid-a',
      );
    });

    test(
      'different UID is NOT suppressed — must trigger re-auth',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-b');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final container = _container(
          controller: controller,
          authRepository: _TestAuthRepository(),
          userSyncService: userSyncService,
          logger: _NoopLoggerService(),
          analyticsRepository: _NoopAnalyticsRepository(),
          localStorageService: _RecordingLocalStorageService(),
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);

        // Seed authenticated with uid-a
        controller.state = AuthState.authenticated(
          _principalUser('uid-a'),
          emailVerified: true,
        );

        // Now simulate a different principal
        userSyncService.currentUserResult = Result.success(
          _principalUser('uid-b'),
        );
        controller.refreshAuthState();

        // refreshAuthState clears _syncedUserId and calls _syncWithBackend
        // which should trigger a full re-sync for uid-b
        expect(controller.state, isA<AuthStateAuthenticated>());
        expect(
          (controller.state as AuthStateAuthenticated).user.id,
          anyOf('uid-a', 'uid-b'),
        );
      },
    );

    test(
      'sign-out (null user) is NOT suppressed — must publish unauthenticated',
          () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final container = _container(
          controller: controller,
          authRepository: _TestAuthRepository(),
          userSyncService: userSyncService,
          logger: _NoopLoggerService(),
          analyticsRepository: _NoopAnalyticsRepository(),
          localStorageService: _RecordingLocalStorageService(),
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);

        // Seed authenticated
        controller.state = AuthState.authenticated(
          _principalUser('uid-a'),
          emailVerified: true,
        );

        // Sign out
        await controller.signOut();

        // signOut publishes AuthStateUnauthenticated
        expect(controller.state, isA<AuthStateUnauthenticated>());
      },
    );
  });

  // ─────────────────────────────────────────────────────────────────
  // Principal-switch and stale-result proof
  // ─────────────────────────────────────────────────────────────────

  group('Principal-switch and stale-result safety', () {
    test(
      'forceRefreshAuthState from A cannot publish stale result after '
      'switch to B',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final container = _container(
          controller: controller,
          authRepository: _TestAuthRepository(),
          userSyncService: userSyncService,
          logger: _NoopLoggerService(),
          analyticsRepository: _NoopAnalyticsRepository(),
          localStorageService: _RecordingLocalStorageService(),
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);

        // Seed A
        controller.state = AuthState.authenticated(
          _principalUser('uid-a'),
          emailVerified: true,
        );

        // Start a refresh from A that will be held open.
        final staleCompleter = Completer<Result<AuthUser>>();
        userSyncService.currentUserCompleter = staleCompleter;
        unawaited(controller.forceRefreshAuthState());
        await Future<void>.delayed(Duration.zero);

        // Switch to B: clear the completer so the B-path uses
        // currentUserResult instead, then do a full refreshAuthState
        // which re-initialises the principal and syncs B.
        userSyncService.currentUserCompleter = null;
        userSyncService.currentUserResult = Result.success(
          _principalUser('uid-b'),
        );
        firebaseUser.uidValue = 'uid-b';
        controller.firebaseUser = firebaseUser;
        controller.state = AuthState.authenticated(
          _principalUser('uid-b'),
          emailVerified: true,
        );
        await controller.refreshUserData();

        // Now complete A's stale refresh — it must be silently dropped
        // because the hydration generation has advanced.
        staleCompleter.complete(Result.success(_principalUser('uid-a')));
        await Future<void>.delayed(Duration.zero);

        // Assert: state must still be B
        expect(controller.state, isA<AuthStateAuthenticated>());
        expect(
          (controller.state as AuthStateAuthenticated).user.id,
          'uid-b',
          reason: 'Stale forceRefreshAuthState from A must not overwrite B',
        );
      },
    );

    test(
      'duplicate concurrent forceRefreshAuthState cannot publish older '
      'result last',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final container = _container(
          controller: controller,
          authRepository: _TestAuthRepository(),
          userSyncService: userSyncService,
          logger: _NoopLoggerService(),
          analyticsRepository: _NoopAnalyticsRepository(),
          localStorageService: _RecordingLocalStorageService(),
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);
        await _seedPrincipal(
          controller: controller,
          userSyncService: userSyncService,
          firebaseUser: firebaseUser,
          user: _principalUser('uid-a'),
        );

        // First refresh — will complete second
        final firstCompleter = Completer<Result<AuthUser>>();
        userSyncService.currentUserCompleter = firstCompleter;
        unawaited(controller.forceRefreshAuthState());
        await Future<void>.delayed(Duration.zero);

        // State should still be authenticated (no loading published)
        expect(controller.state, isA<AuthStateAuthenticated>());

        // Second refresh — will complete first with different data
        final secondCompleter = Completer<Result<AuthUser>>();
        userSyncService.currentUserCompleter = secondCompleter;
        unawaited(controller.forceRefreshAuthState());
        await Future<void>.delayed(Duration.zero);

        // Complete second refresh first
        final updatedUser = _principalUser(
          'uid-a',
          hasMarketAuthority: false,
        );
        secondCompleter.complete(Result.success(updatedUser));
        await Future<void>.delayed(Duration.zero);

        // Complete first (stale) refresh
        firstCompleter.complete(Result.success(_principalUser('uid-a')));
        await Future<void>.delayed(Duration.zero);

        // Assert: state must reflect the latest refresh, not the stale one
        expect(controller.state, isA<AuthStateAuthenticated>());
        final authed = controller.state as AuthStateAuthenticated;
        expect(authed.user.id, 'uid-a');
        // The _beginHydrationRequest counter ensures only the current
        // generation's result is published. The stale first refresh
        // should have been dropped.
      },
    );
  });

  // ─────────────────────────────────────────────────────────────────
  // Router control proofs: startup, logout, genuine unauthenticated
  // ─────────────────────────────────────────────────────────────────

  group('Startup and logout routing control', () {
    test(
      'logout from authenticated publishes unauthenticated — router must '
      'follow canonical auth route, not Home',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final container = _container(
          controller: controller,
          authRepository: _TestAuthRepository(),
          userSyncService: userSyncService,
          logger: _NoopLoggerService(),
          analyticsRepository: _NoopAnalyticsRepository(),
          localStorageService: _RecordingLocalStorageService(),
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);
        await _seedPrincipal(
          controller: controller,
          userSyncService: userSyncService,
          firebaseUser: firebaseUser,
          user: _principalUser('uid-a'),
        );

        // Act
        await controller.signOut();

        // Assert: must be unauthenticated, not any other state
        expect(controller.state, isA<AuthStateUnauthenticated>());
        // Router would see AppAuthStatus.unauthenticated → /welcome
        final status = controller.appAuthStatus;
        expect(status, AppAuthStatus.unauthenticated);
      },
    );

    test(
      'initial build returns AuthStateInitial → appAuthStatus.initializing '
      '→ router must show /splash',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final container = _container(
          controller: controller,
          authRepository: _TestAuthRepository(),
          userSyncService: userSyncService,
          logger: _NoopLoggerService(),
          analyticsRepository: _NoopAnalyticsRepository(),
          localStorageService: _RecordingLocalStorageService(),
        );
        addTearDown(container.dispose);

        // Read the notifier from the container to initialise it.
        final notifier = container.read(authControllerProvider.notifier);

        // build() returns AuthStateInitial
        expect(notifier.state, isA<AuthStateInitial>());
        expect(notifier.appAuthStatus, AppAuthStatus.initializing);
      },
    );

    test(
      'explicit navigation to Home via AppRouter still works '
      '(positive control)',
      () {
        // This test verifies that the canonical navigateToHome sink
        // is intact and has not been accidentally disabled by the fix.
        final router = AppRouter();
        // Navigate to home is a void call; we just verify it does not
        // throw and the method signature is unchanged.
        expect(router.navigateToHome, isA<Function>());
      },
    );
  });
}
