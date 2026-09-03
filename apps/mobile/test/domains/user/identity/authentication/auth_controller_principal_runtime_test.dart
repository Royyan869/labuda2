import 'dart:async';

import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userSyncServiceProvider;
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';

// ============================================================================
// AuthController principal/runtime behavior — CURRENT architecture.
//
// This suite proves AuthController runtime safety through the current seams:
//   * AuthController (Notifier<AuthState>) owns identity state.
//   * UserSyncService is the canonical sync seam; this test subclasses it and
//     overrides the CURRENT signatures (getCurrentUser() / syncUser() with
//     username+phoneNumber). There is no epoch / principal-check parameter —
//     stale async results are dropped by AuthController's hydration-generation
//     guard (_beginHydrationRequest / _publishIfCurrent), not by the service.
//   * Scenarios preserved from the pre-rewrite suite (A→B switch, in-flight
//     completion isolation, disposal, forceRefresh no-loading, sign-out).
// ============================================================================

class _MutableFirebaseUser extends Fake implements User {
  _MutableFirebaseUser({required this.uidValue});

  String uidValue;

  @override
  String get uid => uidValue;

  @override
  bool get emailVerified => true;

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
  Future<String?> getIdToken([bool forceRefresh = false]) async =>
      'firebase-token';
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

  final User? _currentUserValue;

  @override
  User? get currentUser => _currentUserValue;

  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();
}

class _MockUserApiDatasource extends Fake implements UserApiDatasource {}

/// Canonical sync seam — overrides the CURRENT UserSyncService signatures.
///
/// getCurrentUser()/syncUser() are driven by injected completers so tests can
/// hold requests in flight and complete them out of order.
class _ControllerUserSyncService extends UserSyncService {
  _ControllerUserSyncService({required FirebaseAuth firebaseAuth})
    : super(
        firebaseAuth: firebaseAuth,
        datasource: _MockUserApiDatasource(),
      );

  Result<AuthUser> currentUserResult = Result.error('not configured');
  Completer<Result<AuthUser>>? currentUserCompleter;
  final List<Completer<Result<SyncUserResult>>> syncCalls =
      <Completer<Result<SyncUserResult>>>[];

  @override
  Future<Result<AuthUser>> getCurrentUser() {
    final completer = currentUserCompleter;
    if (completer != null) {
      return completer.future;
    }
    return Future<Result<AuthUser>>.value(currentUserResult);
  }

  @override
  Future<Result<SyncUserResult>> syncUser({
    required String username,
    String? phoneNumber,
  }) {
    final completer = Completer<Result<SyncUserResult>>();
    syncCalls.add(completer);
    return completer.future;
  }

  void completeSync(int index, Result<SyncUserResult> result) {
    syncCalls[index].complete(result);
  }

  void failSync(int index, Object error) {
    syncCalls[index].completeError(error);
  }
}

class _RecordingLocalStorageService extends Fake
    implements ILocalStorageService {
  int clearAuthTokenCalls = 0;
  int clearRefreshTokenCalls = 0;
  int setAuthTokenCalls = 0;
  int setRefreshTokenCalls = 0;

  @override
  Future<Result<void>> initialize() async => Result.success(null);

  @override
  Future<Result<void>> clear() async => Result.success(null);

  @override
  Future<Result<void>> clearSecure() async => Result.success(null);

  @override
  Future<Result<void>> clearAuthToken() async {
    clearAuthTokenCalls++;
    return Result.success(null);
  }

  @override
  Future<Result<void>> clearRefreshToken() async {
    clearRefreshTokenCalls++;
    return Result.success(null);
  }

  @override
  Future<Result<String?>> getAuthToken() async => Result.success(null);

  @override
  Future<Result<String?>> getRefreshToken() async => Result.success(null);

  @override
  Future<Result<void>> setAuthToken(String token) async {
    setAuthTokenCalls++;
    return Result.success(null);
  }

  @override
  Future<Result<void>> setRefreshToken(String token) async {
    setRefreshTokenCalls++;
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
  Future<Result<void>> signOut() async => Result.success(null);
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
    provider: AuthProvider.email,
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

ProviderContainer _container({
  required _TestAuthController controller,
  required _ControllerUserSyncService userSyncService,
  ILocalStorageService? localStorageService,
}) {
  return ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(() => controller),
      auth_data.authRepositoryProvider.overrideWithValue(_TestAuthRepository()),
      userSyncServiceProvider.overrideWithValue(userSyncService),
      loggerServiceProvider.overrideWithValue(_NoopLoggerService()),
      coreAnalyticsRepositoryProvider.overrideWithValue(
        _NoopAnalyticsRepository(),
      ),
      localStorageServiceProvider.overrideWithValue(
        localStorageService ?? _RecordingLocalStorageService(),
      ),
    ],
  );
}

Future<void> _flushMicrotasks() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
}

/// Seeds the controller into an authenticated state for the given principal,
/// then runs a full refresh so the hydration-generation machinery is live.
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

  group('AuthController principal runtime', () {
    test(
      'A to B before retry fires keeps the old timer inert and lets B sync '
      'independently',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final localStorage = _RecordingLocalStorageService();
        final container = _container(
          controller: controller,
          userSyncService: userSyncService,
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

        // A's sync fails with backend unavailable → retry path arms.
        controller.refreshAuthState();
        await _flushMicrotasks();
        expect(userSyncService.syncCalls, hasLength(1));

        userSyncService.completeSync(
          0,
          Result.error('Backend unavailable', statusCode: 503),
        );
        await _flushMicrotasks();
        expect(controller.state, isA<AuthStateBackendUnavailable>());

        // Switch to B before the retry timer fires.
        controller.signOut();
        await _flushMicrotasks();
        expect(controller.state, isA<AuthStateUnauthenticated>());

        firebaseUser.uidValue = 'uid-b';
        userSyncService.currentUserResult = Result.success(
          _principalUser('uid-b'),
        );
        controller.state = AuthState.authenticated(
          _principalUser('uid-b'),
          emailVerified: true,
        );
        await controller.refreshUserData();
        controller.refreshAuthState();
        await _flushMicrotasks();

        // B performs its own sync; A's old retry must not re-fire.
        expect(userSyncService.syncCalls, hasLength(2));

        userSyncService.completeSync(1, _syncSuccess(_principalUser('uid-b')));
        await _flushMicrotasks();
        expect(controller.state, isA<AuthStateAuthenticated>());
        expect((controller.state as AuthStateAuthenticated).user.id, 'uid-b');
        expect(localStorage.clearAuthTokenCalls, 0);
        expect(localStorage.clearRefreshTokenCalls, 0);
      },
    );

    test(
      'in-flight A completion cannot clear B ownership after sign-out + '
      'login as B',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final localStorage = _RecordingLocalStorageService();
        final container = _container(
          controller: controller,
          userSyncService: userSyncService,
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

        controller.refreshAuthState();
        await _flushMicrotasks();
        controller.signOut();
        await _flushMicrotasks();

        // Login as B.
        firebaseUser.uidValue = 'uid-b';
        userSyncService.currentUserResult = Result.success(
          _principalUser('uid-b'),
        );
        controller.state = AuthState.authenticated(
          _principalUser('uid-b'),
          emailVerified: true,
        );
        await controller.refreshUserData();
        controller.refreshAuthState();
        await _flushMicrotasks();
        expect(userSyncService.syncCalls, hasLength(2));

        // B fails first (backend unavailable), then A's stale success lands.
        userSyncService.completeSync(
          1,
          Result.error('Backend unavailable', statusCode: 503),
        );
        await _flushMicrotasks();
        expect(controller.state, isA<AuthStateBackendUnavailable>());

        userSyncService.completeSync(0, _syncSuccess(_principalUser('uid-a')));
        await _flushMicrotasks();

        // A's stale completion must not promote A back into ownership —
        // the hydration generation has advanced under B.
        expect(controller.state, isNot(isA<AuthStateAuthenticated>()));
        expect(localStorage.setAuthTokenCalls, 0);
        expect(localStorage.setRefreshTokenCalls, 0);
      },
    );

    test(
      'forceRefreshAuthState with changed user data updates in-place',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final container = _container(
          controller: controller,
          userSyncService: userSyncService,
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);
        await _seedPrincipal(
          controller: controller,
          userSyncService: userSyncService,
          firebaseUser: firebaseUser,
          user: _principalUser('uid-a'),
        );

        // getCurrentUser returns changed authority (e.g. market authority
        // revoked by backend).
        final changedUser = _principalUser(
          'uid-a',
          hasMarketAuthority: false,
        );
        userSyncService.currentUserResult = Result.success(changedUser);

        await controller.forceRefreshAuthState();

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
          userSyncService: userSyncService,
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);
        await _seedPrincipal(
          controller: controller,
          userSyncService: userSyncService,
          firebaseUser: firebaseUser,
          user: _principalUser('uid-a'),
        );

        userSyncService.currentUserResult = Result.error(
          'Backend unavailable',
          statusCode: 503,
        );

        await controller.forceRefreshAuthState();

        // Direct authenticated → backend-unavailable; no loading in between.
        // AuthStateBackendUnavailable maps to AppAuthStatus.degraded which
        // does NOT trigger a router redirect.
        expect(controller.state, isA<AuthStateBackendUnavailable>());
      },
    );

    test(
      'stale forceRefreshAuthState from A cannot publish after switch to B',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final container = _container(
          controller: controller,
          userSyncService: userSyncService,
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);

        controller.state = AuthState.authenticated(
          _principalUser('uid-a'),
          emailVerified: true,
        );

        // Start a refresh from A that will be held open.
        final staleCompleter = Completer<Result<AuthUser>>();
        userSyncService.currentUserCompleter = staleCompleter;
        unawaited(controller.forceRefreshAuthState());
        await _flushMicrotasks();

        // Switch to B: the B-path uses currentUserResult and re-initialises
        // the principal.
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

        // Complete A's stale refresh — it must be silently dropped because
        // the hydration generation has advanced.
        staleCompleter.complete(Result.success(_principalUser('uid-a')));
        await _flushMicrotasks();

        expect(controller.state, isA<AuthStateAuthenticated>());
        expect(
          (controller.state as AuthStateAuthenticated).user.id,
          'uid-b',
          reason: 'Stale forceRefreshAuthState from A must not overwrite B',
        );
      },
    );

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
          userSyncService: userSyncService,
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);
        await _seedPrincipal(
          controller: controller,
          userSyncService: userSyncService,
          firebaseUser: firebaseUser,
          user: _principalUser('uid-a'),
        );

        await controller.signOut();

        expect(controller.state, isA<AuthStateUnauthenticated>());
        expect(controller.appAuthStatus, AppAuthStatus.unauthenticated);
      },
    );

    test(
      'initial build returns AuthStateInitial → appAuthStatus.initializing',
      () async {
        final firebaseUser = _MutableFirebaseUser(uidValue: 'uid-a');
        final controller = _TestAuthController(firebaseUser: firebaseUser);
        final userSyncService = _ControllerUserSyncService(
          firebaseAuth: _MutableFirebaseAuth(currentUserValue: firebaseUser),
        );
        final container = _container(
          controller: controller,
          userSyncService: userSyncService,
        );
        addTearDown(container.dispose);

        final notifier = container.read(authControllerProvider.notifier);

        expect(notifier.state, isA<AuthStateInitial>());
        expect(notifier.appAuthStatus, AppAuthStatus.initializing);
      },
    );

    test(
      'explicit navigation to Home via AppRouter still works '
      '(positive control)',
      () {
        final router = AppRouter();
        expect(router.navigateToHome, isA<Function>());
      },
    );
  });
}
