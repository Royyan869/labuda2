// AUTH PENDING-VERIFICATION LISTENER STOMP — regression lock.
//
// Root-cause proof for the "email/password login stuck on splash" defect:
//
// `AuthController.debugHandleFirebaseAuthEvent` published
// `AuthStateFirebaseAuthenticated` BEFORE the explicit-flow deferral check.
// A late-processed `authStateChanges` emission (its `user.reload()` resolves
// after `signInWithEmail` already parked the session in
// `AuthStatePendingEmailVerification`) therefore:
//   1. stomped the parked state → `AuthStateFirebaseAuthenticated`;
//   2. deferred to the (already-returned) explicit initiator;
//   3. the router's `isSyncingWithBackend` block force-redirected
//      /auth/verify-email → /splash, disposing the verify screen — the ONLY
//      driver of `checkPendingEmailVerification`;
//   4. nothing remained in flight → the machine wedged on splash forever.
//
// Google Sign-In never parks (Google emails are always verified), which is
// why only the email/password path hit the wedge.
//
// These tests drive the REAL AuthController and lock the fixed contract:
// while an explicit flow owns the session, the listener must NOT publish any
// state — it defers silently and the parked/explicit state survives.

import 'dart:async';

import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart' hide NotificationEntity;
import 'package:hishumi/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data;
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/firebase_principal.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/user_profile_patch.dart';
import 'package:hishumi/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:hishumi/domains/user/profile/data/models/api/user_api_models.dart';
import 'package:hishumi/domains/user/profile/data/profile_providers.dart'
    as profile_data show userSyncServiceProvider;
import 'package:hishumi/domains/user/profile/data/services/user_sync_service.dart';
import 'package:hishumi/domains/system/notification/data/notification_providers.dart'
    show fcmServiceProvider;
import 'package:hishumi/domains/system/notification/services/fcm_service.dart';

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

class _FakeUser extends Fake implements User {
  _FakeUser(this.uid, {this.emailVerifiedValue = true});
  @override
  final String uid;
  bool emailVerifiedValue;

  /// When set, `reload()` parks until the completer resolves — models the
  /// real-device latency of the Firebase profile refresh inside the
  /// listener while the explicit flow has already moved on.
  Completer<void>? reloadGate;

  @override
  String get email => '$uid@example.com';
  @override
  bool get emailVerified => emailVerifiedValue;
  @override
  String? get phoneNumber => null;
  @override
  List<UserInfo> get providerData => const <UserInfo>[];
  @override
  UserMetadata get metadata => _FakeMetadata();
  @override
  Future<void> reload() async {
    final gate = reloadGate;
    if (gate != null) await gate.future;
  }
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
  final User? user;
  @override
  User? get currentUser => user;
  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();
}

class _RecordingLocalStorage extends Fake implements ILocalStorageService {
  String? access;
  String? refresh;
  @override
  Future<Result<void>> saveHiShumiCredential(String a, String r) async {
    access = a;
    refresh = r;
    return Result.success(null);
  }
  @override
  Future<Result<String?>> readHiShumiAccessToken() async =>
      Result.success(access);
  @override
  Future<Result<String?>> readHiShumiRefreshToken() async =>
      Result.success(refresh);
  @override
  Future<Result<bool>> hasHiShumiCredential() async =>
      Result.success(access != null && access!.isNotEmpty);
  @override
  Future<Result<void>> clearHiShumiCredential() async {
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
}

class _NoopLogger implements ILoggerService {
  const _NoopLogger();
  Result<void> _ok() => Result.success(null);
  @override
  Future<Result<void>> info(String m, {Map<String, dynamic>? extra}) async => _ok();
  @override
  Future<Result<void>> warning(String m, {Map<String, dynamic>? extra}) async => _ok();
  @override
  Future<Result<void>> error(
    String m, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _ok();
  @override
  Future<Result<void>> debug(String m, {Map<String, dynamic>? extra}) async => _ok();
  @override
  Future<void> log(String m, {LogLevel level = LogLevel.debug}) async {}
  @override
  Future<Result<void>> fatal(
    String m, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _ok();
  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async => Result.success(const <LogEntry>[]);
  @override
  Future<Result<void>> clearLogs() async => _ok();
  @override
  Future<Result<void>> setLogLevel(LogLevel level) async => _ok();
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
}

class _NoopAnalytics extends Fake implements IAnalyticsRepository {
  @override
  Future<Result<void>> logEvent(
    String e, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async => Result.success(null);
  @override
  Future<Result<void>> logScreenView({
    required String screenName,
    String? screenClass,
  }) async => Result.success(null);
}

class _NoopFcm extends Fake implements FcmService {
  @override
  String? get fcmToken => null;
}

class _TestController extends AuthController {
  _TestController(this.user);
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
  Future<Result<FirebasePrincipal>> signInWithEmail({
    required String email,
    required String password,
  }) => signIn();
  @override
  Future<Result<void>> signInWithGoogle({
    AuthCredential? pendingGoogleCredential,
  }) async => Result.success(null);
  @override
  Future<Result<FirebasePrincipal>> signUpWithEmail({
    required String email,
    required String password,
  }) async => Result.error('n/a');
  @override
  Future<Result<void>> signOut() async => Result.success(null);
  @override
  Future<Result<void>> logoutCurrentSession({
    required String refreshToken,
    String? fcmToken,
    String? deviceId,
  }) async => Result.success(null);
  @override
  Future<Result<void>> logoutAllSessions({bool deactivateFcmTokens = true}) async =>
      Result.success(null);
  @override
  Future<Result<List<AuthSessionDto>>> getActiveSessions() async =>
      Result.success(const <AuthSessionDto>[]);
  @override
  Future<Result<void>> revokeSession(String familyId) async => Result.success(null);
  @override
  Future<Result<void>> resetPassword({required String email}) async =>
      Result.success(null);
  @override
  Future<Result<void>> sendEmailVerification() async => Result.success(null);
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
    String? twitterHandle,
    String? tiktokHandle,
    DateTime? dateOfBirth,
  }) async => Result.error('n/a');
  @override
  Future<Result<AuthUser>> completeProfile({required String username}) async =>
      Result.error('n/a');
  @override
  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async => Result.success(null);
  @override
  Future<Result<void>> deleteAccount() async => Result.success(null);
  @override
  Future<Result<AuthUser?>> getUserById(String userId) async => Result.success(null);
  @override
  Future<Result<void>> deactivateAccount({
    required String userId,
    required String reason,
  }) async => Result.success(null);
}

AuthUser _account() => AuthUser(
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

SyncUserResult _syncSuccess() => SyncUserResult(
      user: _account(),
      userId: 'uid-1',
      email: 'a@example.com',
      created: false,
      profileComplete: true,
      username: 'seller-one',
    );

ProviderContainer _buildContainer(
  _TestController controller,
  _RecordingSyncService sync,
  IAuthRepository repo,
) {
  return ProviderContainer(
    overrides: [
      localStorageServiceProvider.overrideWithValue(_RecordingLocalStorage()),
      auth_data.authRepositoryProvider.overrideWithValue(repo),
      profile_data.userSyncServiceProvider.overrideWithValue(sync),
      loggerServiceProvider.overrideWithValue(const _NoopLogger()),
      coreAnalyticsRepositoryProvider.overrideWithValue(_NoopAnalytics()),
      fcmServiceProvider.overrideWithValue(_NoopFcm() as dynamic),
      authControllerProvider.overrideWith(() => controller),
    ],
  );
}

Future<void> _flush([int n = 4]) async {
  for (var i = 0; i < n; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test(
    'late listener emission while parked on pending verification must not '
    'wedge the machine (email/password stuck-on-splash root cause)',
    () async {
      final user = _FakeUser('uid-1', emailVerifiedValue: false);
      final controller = _TestController(user);
      final sync = _RecordingSyncService(auth: user);
      final container = _buildContainer(
        controller,
        sync,
        _FakeRepo(
          () async => Result.success(
            FirebasePrincipal(uid: 'uid-1', emailVerified: false),
          ),
        ),
      );
      addTearDown(container.dispose);
      container.read(authControllerProvider);

      // 1) Email/password login with an unverified identity parks the
      //    session on the verify-email surface (D2 hard gate).
      await controller.signInWithEmail(email: 'a@example.com', password: 'pw');
      expect(controller.state, isA<AuthStatePendingEmailVerification>());
      expect(sync.exchangeCalls, 0, reason: 'no exchange before verification');

      // 2) The authStateChanges emission from the sign-in is processed LATE:
      //    its reload resolves only now, after the park. Pre-fix this
      //    published AuthStateFirebaseAuthenticated — the router then
      //    force-redirected /auth/verify-email → /splash, disposing the
      //    verify screen (the only driver of the recovery bridge) and
      //    wedging the machine on splash forever.
      user.reloadGate = Completer<void>();
      final listenerDone = controller.debugHandleFirebaseAuthEvent(user);
      await _flush();
      user.reloadGate!.complete();
      await listenerDone;
      await _flush();

      expect(
        controller.state,
        isA<AuthStatePendingEmailVerification>(),
        reason:
            'the listener must defer to the explicit parked flow WITHOUT '
            'publishing a state — stomping the park with '
            'AuthStateFirebaseAuthenticated forces the router back to '
            '/splash and orphans the session (eternal splash).',
      );

      // 3) Recovery bridge still works: the verify screen poll finds the
      //    email verified and the canonical exchange completes the session.
      user.emailVerifiedValue = true;
      final poll = controller.checkPendingEmailVerification();
      await _flush();
      expect(sync.exchangeCalls, 1, reason: 'post-verification exchange runs');
      sync.completeNext(Result.success(_syncSuccess()));
      await poll;

      expect(controller.state, isA<AuthStateAuthenticated>());
    },
  );

  test(
    'late listener emission during an explicit verified email login defers '
    'without stomping and the explicit flow still completes',
    () async {
      final user = _FakeUser('uid-1');
      final controller = _TestController(user);
      final sync = _RecordingSyncService(auth: user);
      final container = _buildContainer(
        controller,
        sync,
        _FakeRepo(
          () async => Result.success(
            FirebasePrincipal(uid: 'uid-1', emailVerified: true),
          ),
        ),
      );
      addTearDown(container.dispose);
      container.read(authControllerProvider);

      final login = controller.signInWithEmail(
        email: 'a@example.com',
        password: 'pw',
      );
      await _flush();
      // Sync in flight; listener emission resolves mid-sync (gated reload).
      user.reloadGate = Completer<void>();
      final listenerDone = controller.debugHandleFirebaseAuthEvent(user);
      await _flush();
      user.reloadGate!.complete();
      sync.completeNext(Result.success(_syncSuccess()));
      await login;
      await listenerDone;
      await _flush();
      while (sync.pending.isNotEmpty) {
        sync.completeNext(Result.success(_syncSuccess()));
        await _flush();
      }

      expect(controller.state, isA<AuthStateAuthenticated>());
      expect(
        sync.exchangeCalls,
        1,
        reason: 'a deferred listener emission must never start a second exchange',
      );
    },
  );

  test(
    'Google sign-in is unaffected: listener defers and the explicit flow '
    'completes exactly one exchange',
    () async {
      final user = _FakeUser('uid-1');
      final controller = _TestController(user);
      final sync = _RecordingSyncService(auth: user);
      final container = _buildContainer(
        controller,
        sync,
        _FakeRepo(
          () async => Result.success(
            FirebasePrincipal(uid: 'uid-1', emailVerified: true),
          ),
        ),
      );
      addTearDown(container.dispose);
      container.read(authControllerProvider);

      final login = controller.signInWithGoogle();
      await _flush();
      sync.completeNext(Result.success(_syncSuccess()));
      await login;

      expect(controller.state, isA<AuthStateAuthenticated>());
      expect(sync.exchangeCalls, 1);
    },
  );
}
