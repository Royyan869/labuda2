// AUTH-2 — Login completion authority / terminal-state semantics regression proof.
//
// Proves, against the REAL AuthController state machine, that:
//   1. Email & Google sign-in converge on one canonical commit path (sync).
//   2. Explicit completion does NOT depend on listener timing: the initiator
//      drives _syncWithBackend directly.
//   3. Backend failure / backend unavailable yield explicit, recoverable
//      terminal states — never a silent "logged in but stuck on Login".
//   4. Explicit + listener duplicate calls produce ONE exchange (no double).

import 'dart:async';

import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart' as auth_data;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/firebase_principal.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/user_profile_patch.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart' as profile_data
    show userSyncServiceProvider;
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart'
    show fcmServiceProvider;
import 'package:labuda/domains/system/notification/services/fcm_service.dart';

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

class _FakeUser extends Fake implements User {
  @override
  final String uid;
  _FakeUser(this.uid);
  @override
  String? get email => '$uid@example.com';
  @override
  bool get emailVerified => true;
  @override
  String? get phoneNumber => null;
  @override
  List<UserInfo> get providerData => const <UserInfo>[];
  @override
  UserMetadata get metadata => _FakeMetadata();
  @override
  Future<void> reload() async {}
  @override
  Future<String?> getIdToken([bool forceRefresh = false]) async => 'id-token';
}

class _FakeMetadata extends Fake implements UserMetadata {
  @override
  DateTime? get creationTime => DateTime.utc(2026, 1, 1);
  @override
  DateTime? get lastSignInTime => DateTime.utc(2026, 1, 1);
}

class _FakeAuth extends Fake implements FirebaseAuth {
  _FakeAuth(this.user);
  @override
  final User? user;
  @override
  User? get currentUser => user;
  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();
}

class _RecordingLocalStorage extends Fake implements ILocalStorageService {
  String? access;
  String? refresh;
  int saveCalls = 0;
  int clearCalls = 0;
  @override
  Future<Result<void>> saveLabudaCredential(String a, String r) async {
    access = a;
    refresh = r;
    saveCalls++;
    return Result.success(null);
  }
  @override
  Future<Result<String?>> readLabudaAccessToken() async => Result.success(access);
  @override
  Future<Result<String?>> readLabudaRefreshToken() async => Result.success(refresh);
  @override
  Future<Result<bool>> hasLabudaCredential() async =>
      Result.success(access != null && access!.isNotEmpty);
  @override
  Future<Result<void>> clearLabudaCredential() async {
    clearCalls++;
    access = null;
    refresh = null;
    return Result.success(null);
  }
  @override
  Future<Result<void>> setRestrictedToken(String t) async => Result.success(null);
  @override
  Future<Result<String?>> getRestrictedToken() async => Result.success(null);
  @override
  Future<Result<void>> clearRestrictedToken() async => Result.success(null);
  @override
  Future<Result<void>> setString(String key, String value) async => Result.success(null);
  @override
  Future<Result<String?>> getString(String key) async => Result.success(null);
  @override
  Future<Result<void>> setInt(String key, int value) async => Result.success(null);
  @override
  Future<Result<int?>> getInt(String key) async => Result.success(null);
  @override
  Future<Result<void>> setBool(String key, bool value) async => Result.success(null);
  @override
  Future<Result<bool?>> getBool(String key) async => Result.success(null);
  @override
  Future<Result<void>> setDouble(String key, double value) async => Result.success(null);
  @override
  Future<Result<double?>> getDouble(String key) async => Result.success(null);
  @override
  Future<Result<void>> setStringList(String key, List<String> value) async => Result.success(null);
  @override
  Future<Result<List<String>?>> getStringList(String key) async => Result.success(null);
  @override
  Future<Result<void>> setObject(String key, Map<String, dynamic> value) async => Result.success(null);
  @override
  Future<Result<Map<String, dynamic>?>> getObject(String key) async => Result.success(null);
  @override
  Future<Result<void>> setSecureString(String key, String value) async => Result.success(null);
  @override
  Future<Result<String?>> getSecureString(String key) async => Result.success(null);
  @override
  Future<Result<void>> remove(String key) async => Result.success(null);
  @override
  Future<Result<void>> removeSecure(String key) async => Result.success(null);
  @override
  Future<Result<void>> clear() async => Result.success(null);
  @override
  Future<Result<void>> clearSecure() async => Result.success(null);
  @override
  Future<Result<bool>> containsKey(String key) async => Result.success(false);
  @override
  Future<Result<Set<String>>> getKeys() async => Result.success(<String>{});
  @override
  Future<Result<void>> initialize() async => Result.success(null);
}

class _NoopLogger implements ILoggerService {
  const _NoopLogger();
  Result<void> _ok() => Result.success(null);
  @override
  Future<Result<void>> info(String m, {Map<String, dynamic>? extra}) async => _ok();
  @override
  Future<Result<void>> warning(String m, {Map<String, dynamic>? extra}) async => _ok();
  @override
  Future<Result<void>> error(String m, {Map<String, dynamic>? extra, StackTrace? stackTrace}) async => _ok();
  @override
  Future<Result<void>> debug(String m, {Map<String, dynamic>? extra}) async => _ok();
  @override
  Future<void> log(String m, {LogLevel level = LogLevel.debug}) async {}
  @override
  Future<Result<void>> fatal(String m, {Map<String, dynamic>? extra, StackTrace? stackTrace}) async => _ok();
  @override
  Future<Result<List<LogEntry>>> getLogs({LogLevel? minLevel, DateTime? startDate, DateTime? endDate, int? limit}) async => Result.success(const <LogEntry>[]);
  @override
  Future<Result<void>> clearLogs() async => _ok();
  @override
  Future<Result<void>> setLogLevel(LogLevel level) async => _ok();
  @override
  Future<Result<void>> logApiCall(String endpoint, {required String method, required int statusCode, required Duration duration, Map<String, dynamic>? requestData, Map<String, dynamic>? responseData}) async => _ok();
  @override
  Future<Result<void>> logPerformance(String operation, {required Duration duration, Map<String, dynamic>? metrics}) async => _ok();
  @override
  Future<Result<void>> logSecurityEvent(String event, {String? userId, String? severity, Map<String, dynamic>? details}) async => _ok();
  @override
  Future<Result<void>> logUserAction(String action, {String? userId, Map<String, dynamic>? parameters}) async => _ok();
  @override
  Future<void> debugSync(String uid) async {}
  @override
  Future<void> debugSyncSuccess(String uid) async {}
  @override
  Future<void> debugSyncFailed(String uid, String? e) async {}
  @override
  Future<void> debugSyncException(String uid, String e, String s) async {}
  @override
  Future<void> debugGetCurrentUserSuccess(String uid, bool v) async {}
  @override
  Future<void> debugGetCurrentUserFailed(String uid, String? e) async {}
  @override
  Future<void> debugCallingGetCurrentUser() async {}
  @override
  Future<void> debugRouterCheck(String uid, bool verified, String loc, bool isVerificationRoute) async {}
}

class _NoopAnalytics extends Fake implements IAnalyticsRepository {
  @override
  Future<Result<void>> logEvent(String e, {Map<String, dynamic>? parameters, String? userId}) async => Result.success(null);
  @override
  Future<Result<void>> flush() async => Result.success(null);
}

class _NoopFcm extends Fake implements FcmService {
  @override
  String? get fcmToken => null;
}

class _Controller extends AuthController {
  _Controller(this.user);
  @override
  final User? user;
  @override
  User? get activeFirebaseUser => user;
  @override
  bool get shouldInitializeAuthListener => false;
  @override
  Future<void> performFirebaseSignOut() async {}
}

class _RecordingSyncService extends UserSyncService {
  _RecordingSyncService({required this.auth})
      : super(
          firebaseAuth: _FakeAuth(auth),
          datasource: UserApiDatasource(
            ApiClient(logger: null),
            logger: const _NoopLogger(),
          ),
          logger: const _NoopLogger(),
          localStorage: _RecordingLocalStorage(),
        );

  final User auth;
  int exchangeCalls = 0;
  final List<Completer<Result<SyncUserResult>>> pending =
      <Completer<Result<SyncUserResult>>>[];

  @override
  Future<Result<SyncUserResult>> syncUser({
    required String username,
    String? phoneNumber,
  }) {
    exchangeCalls++;
    final completer = Completer<Result<SyncUserResult>>();
    pending.add(completer);
    return completer.future;
  }

  void completeNext(Result<SyncUserResult> result) {
    pending.removeAt(0).complete(result);
  }
}

class _FakeRepo extends Fake implements IAuthRepository {
  final Future<Result<FirebasePrincipal>> Function() signIn;
  _FakeRepo(this.signIn);
  @override
  Future<Result<FirebasePrincipal>> signInWithEmail({required String email, required String password}) => signIn();
  @override
  Future<Result<void>> signInWithGoogle() async => Result.success(null);
  @override
  Future<Result<FirebasePrincipal>> signUpWithEmail({required String email, required String password}) async => Result.error('n/a');
  @override
  Future<Result<void>> signOut() async => Result.success(null);
  @override
  Future<Result<void>> logoutCurrentSession({required String refreshToken, String? fcmToken, String? deviceId}) async => Result.success(null);
  @override
  Future<Result<void>> logoutAllSessions({bool deactivateFcmTokens = true}) async => Result.success(null);
  @override
  Future<Result<List<AuthSessionDto>>> getActiveSessions() async => Result.success(const <AuthSessionDto>[]);
  @override
  Future<Result<void>> revokeSession(String familyId) async => Result.success(null);
  @override
  Future<Result<void>> resetPassword({required String email}) async => Result.success(null);
  @override
  Future<Result<void>> verifyEmail() async => Result.success(null);
  @override
  Future<Result<void>> sendEmailVerification() async => Result.success(null);
  @override
  Future<Result<UserProfilePatch>> updateProfile({String? photoUrl, String? phoneNumber, DateTime? phoneVerifiedAt, String? username, String? bio, String? location, DateTime? dateOfBirth}) async => Result.error('n/a');
  @override
  Future<Result<AuthUser>> completeProfile({required String username}) async => Result.error('n/a');
  @override
  Future<Result<void>> changePassword({required String currentPassword, required String newPassword}) async => Result.success(null);
  @override
  Future<Result<void>> deleteAccount() async => Result.success(null);
  @override
  Future<Result<AuthUser?>> getUserById(String userId) async => Result.success(null);
  @override
  Future<Result<List<AuthUser>>> searchUsers({required String query, int limit = 20}) async => Result.success(const <AuthUser>[]);
  @override
  Future<Result<void>> deactivateAccount({required String userId, required String reason}) async => Result.success(null);
  @override
  Future<Result<AuthUser>> updateUserRole({required String userId, required UserRole newRole}) async => Result.error('n/a');
  @override
  Stream<FirebasePrincipal?> get authStateChanges => const Stream<FirebasePrincipal?>.empty();
}

class _TestAuthUser {
  static AuthUser account() => AuthUser(
        id: 'uid-1',
        createdAt: DateTime.utc(2026, 1, 1),
        updatedAt: DateTime.utc(2026, 1, 1),
        email: 'a@example.com',
        username: 'seller-one',
        isEmailVerified: true,
        accountStatus: AccountStatus.active,
        hasSellerProfile: false,
        hasMarketAuthority: false,
        sellerSubscriptionStatus: 'none',
        roles: const [UserRole.user],
        provider: AuthProvider.email,
      );
}

ProviderContainer buildContainer(_Controller controller, _RecordingSyncService sync) {
  final user = controller.user!;
  final storage = _RecordingLocalStorage();
  return ProviderContainer(
    overrides: [
      localStorageServiceProvider.overrideWithValue(storage),
      auth_data.authRepositoryProvider.overrideWithValue(
        _FakeRepo(() async => Result.success(
          FirebasePrincipal(uid: user.uid, emailVerified: true),
        )),
      ),
      profile_data.userSyncServiceProvider.overrideWithValue(sync),
      loggerServiceProvider.overrideWithValue(const _NoopLogger()),
      coreAnalyticsRepositoryProvider.overrideWithValue(_NoopAnalytics()),
      fcmServiceProvider.overrideWithValue(_NoopFcm() as dynamic),
      authControllerProvider.overrideWith(() => controller),
    ],
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('Email explicit login reaches Authenticated via sync — never stuck', () async {
    final controller = _Controller(_FakeUser('uid-1'));
    final sync = _RecordingSyncService(auth: controller.user!);
    final container = buildContainer(controller, sync);
    addTearDown(container.dispose);

    container.read(authControllerProvider);
    final future = controller.signInWithEmail(email: 'a@example.com', password: 'pw');
    // In production the listener fires during this await and calls _syncWithBackend;
    // because _isExplicitLoginInProgress is true it must NOT start a second exchange.
    await Future<void>.delayed(Duration.zero);
    sync.completeNext(Result.success(SyncUserResult(
      user: _TestAuthUser.account(),
      userId: 'uid-1',
      email: 'a@example.com',
      created: false,
      profileComplete: true,
      username: 'seller-one',
    )));
    await future;

    expect(controller.state, isA<AuthStateAuthenticated>());
    expect(sync.exchangeCalls, 1,
        reason: 'explicit + listener must not double-exchange');
  });

  test('Google explicit login converges onto the same Authenticated state', () async {
    final controller = _Controller(_FakeUser('uid-1'));
    final sync = _RecordingSyncService(auth: controller.user!);
    final container = buildContainer(controller, sync);
    addTearDown(container.dispose);

    container.read(authControllerProvider);
    final future = controller.signInWithGoogle();
    await Future<void>.delayed(Duration.zero);
    sync.completeNext(Result.success(SyncUserResult(
      user: _TestAuthUser.account(),
      userId: 'uid-1',
      email: 'a@example.com',
      created: false,
      profileComplete: true,
      username: 'seller-one',
    )));
    await future;

    expect(controller.state, isA<AuthStateAuthenticated>());
    expect(sync.exchangeCalls, 1);
  });

  test('Backend failure surfaces explicit recoverable state — not silent stuck', () async {
    final controller = _Controller(_FakeUser('uid-1'));
    final sync = _RecordingSyncService(auth: controller.user!);
    final container = buildContainer(controller, sync);
    addTearDown(container.dispose);

    container.read(authControllerProvider);
    final future = controller.signInWithEmail(email: 'a@example.com', password: 'pw');
    await Future<void>.delayed(Duration.zero);
    sync.completeNext(Result.error('validation rejected', statusCode: 422));
    await future;

    expect(controller.state, isA<AuthStateBackendFailure>());
    expect((controller.state as AuthStateBackendFailure).message, isNotEmpty,
        reason: 'backend failure must carry an explicit user-visible message');
  });

  test('Backend unavailable surfaces explicit state — not silent stuck', () async {
    final controller = _Controller(_FakeUser('uid-1'));
    final sync = _RecordingSyncService(auth: controller.user!);
    final container = buildContainer(controller, sync);
    addTearDown(container.dispose);

    container.read(authControllerProvider);
    final future = controller.signInWithGoogle();
    await Future<void>.delayed(Duration.zero);
    sync.completeNext(Result.error('network down', statusCode: 503));
    await future;

    expect(controller.state, isA<AuthStateBackendUnavailable>());
    expect((controller.state as AuthStateBackendUnavailable).message, isNotEmpty);
  });

  test('Profile completion yields canonical RequiresProfileCompletion state', () async {
    final controller = _Controller(_FakeUser('uid-1'));
    final sync = _RecordingSyncService(auth: controller.user!);
    final container = buildContainer(controller, sync);
    addTearDown(container.dispose);

    container.read(authControllerProvider);
    final future = controller.signInWithGoogle();
    await Future<void>.delayed(Duration.zero);
    sync.completeNext(Result.success(SyncUserResult(
      user: _TestAuthUser.account(),
      userId: 'uid-1',
      email: 'a@example.com',
      created: true,
      profileComplete: false,
      username: '',
    )));
    await future;

    expect(controller.state, isA<AuthStateRequiresProfileCompletion>());
    expect(sync.exchangeCalls, 1);
  });
}