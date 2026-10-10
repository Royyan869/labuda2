// AUTH-H1 — Google entry-point convergence (Login vs Sign-up) +
// Complete Profile outcome proof.
//
// Owner contract proven here, against the REAL AuthController and the pure
// router redirect seam:
//   G1/G2  Google from Login AND from Sign-up drive the SAME canonical flow
//          (`signUpWithGoogle` aliases `signInWithGoogle`): a new Google
//          account without a username lands on RequiresProfileCompletion →
//          the exclusive /auth/complete-profile surface.
//   G3/G4  An existing Google account with a complete profile goes straight
//          to the single Authenticated state → /home, from BOTH entries,
//          and never sees RequiresProfileCompletion.
//   G5     Complete Profile success → Authenticated → /home (one identity,
//          one session; the repository-level credential proof lives in
//          auth_session_hydration_fail_closed_test).
//   G6     Complete Profile failure → visible outcome + the session stays on
//          the completion surface (username rejection, transient backend
//          failure) or surfaces a terminal error (session mismatch) — never
//          a fake redirect to /home.
//   G7     Logout + late Firebase events never resurrect the Google session.
//   G8     Same email never yields a second profile or an auto-merge: one
//          exchange, zero linking calls, backend remains the binding
//          authority (unbound-row bind / IDENTITY_CONFLICT 409).

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
// Fakes (same harness shape as auth_flow_convergence_test)
// ---------------------------------------------------------------------------

class _FakeUser extends Fake implements User {
  _FakeUser(this.uid);
  @override
  final String uid;
  @override
  String get email => '$uid@example.com';
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
  Future<Result<void>> info(String m, {Map<String, dynamic>? extra}) async =>
      _ok();
  @override
  Future<Result<void>> warning(String m, {Map<String, dynamic>? extra}) async =>
      _ok();
  @override
  Future<Result<void>> error(
    String m, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _ok();
  @override
  Future<Result<void>> debug(String m, {Map<String, dynamic>? extra}) async =>
      _ok();
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

/// Google-aware scripted repository: records every Google call (count +
/// whether a pending/link credential was supplied — the auto-merge probe).
class _GoogleScriptedRepo extends Fake implements IAuthRepository {
  _GoogleScriptedRepo({
    Future<Result<AuthUser>> Function(String username)? completeProfile,
  }) : _completeProfile = completeProfile;

  final Future<Result<AuthUser>> Function(String username)? _completeProfile;

  int googleCallCount = 0;
  final List<AuthCredential?> googlePendingCredentials = [];
  int completeProfileCalls = 0;

  @override
  Future<Result<void>> signInWithGoogle({
    AuthCredential? pendingGoogleCredential,
  }) async {
    googleCallCount++;
    googlePendingCredentials.add(pendingGoogleCredential);
    return Result.success(null);
  }

  @override
  Future<Result<AuthUser>> completeProfile({required String username}) async {
    completeProfileCalls++;
    return _completeProfile!(username);
  }

  @override
  Future<Result<FirebasePrincipal>> signInWithEmail({
    required String email,
    required String password,
  }) async => Result.error('n/a');
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
  Future<Result<void>> revokeSession(String familyId) async =>
      Result.success(null);
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
  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async => Result.success(null);
  @override
  Future<Result<void>> deleteAccount() async => Result.success(null);
  @override
  Future<Result<AuthUser?>> getUserById(String userId) async =>
      Result.success(null);
  @override
  Future<Result<void>> deactivateAccount({
    required String userId,
    required String reason,
  }) async => Result.success(null);
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

AuthUser _account({String username = 'google-user'}) => AuthUser(
      id: 'backend-1',
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      email: 'g@example.com',
      username: username,
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      hasMarketAuthority: false,
      sellerSubscriptionStatus: 'none',
      roles: const [UserRole.user],
      provider: AuthProvider.google,
    );

SyncUserResult _syncComplete() => SyncUserResult(
      user: _account(),
      userId: 'backend-1',
      email: 'g@example.com',
      created: false,
      profileComplete: true,
      username: 'google-user',
    );

SyncUserResult _syncIncomplete() => const SyncUserResult(
      user: null,
      userId: 'backend-1',
      email: 'g@example.com',
      created: true,
      profileComplete: false,
      username: '',
    );

({
  _TestController controller,
  _RecordingSyncService sync,
  _GoogleScriptedRepo repo,
  ProviderContainer container,
})
    _build({required _FakeUser user, _GoogleScriptedRepo? repo}) {
  final controller = _TestController(user);
  final sync = _RecordingSyncService(auth: user);
  final scripted = repo ?? _GoogleScriptedRepo();
  final container = ProviderContainer(
    overrides: [
      localStorageServiceProvider.overrideWithValue(_RecordingLocalStorage()),
      auth_data.authRepositoryProvider.overrideWithValue(scripted),
      profile_data.userSyncServiceProvider.overrideWithValue(sync),
      loggerServiceProvider.overrideWithValue(const _NoopLogger()),
      coreAnalyticsRepositoryProvider.overrideWithValue(_NoopAnalytics()),
      fcmServiceProvider.overrideWithValue(_NoopFcm() as dynamic),
      authControllerProvider.overrideWith(() => controller),
    ],
  );
  container.read(authControllerProvider);
  return (
    controller: controller,
    sync: sync,
    repo: scripted,
    container: container,
  );
}

Future<void> _flush([int n = 4]) async {
  for (var i = 0; i < n; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('G1+G2 — new Google account without username → Complete Profile, '
      'from BOTH entry points with identical outcomes', () {
    test('Login entry (signInWithGoogle) and Sign-up entry (signUpWithGoogle) '
        'converge on the same RequiresProfileCompletion surface', () async {
      // Login entry.
      final login = _build(user: _FakeUser('fb-g1'));
      addTearDown(login.container.dispose);
      final loginFlow = login.controller.signInWithGoogle();
      await _flush();
      login.sync.completeNext(Result.success(_syncIncomplete()));
      await loginFlow;
      expect(login.controller.state, isA<AuthStateRequiresProfileCompletion>());
      expect(
        handleAuthRedirectForTest(
          login.controller.state,
          login.controller.appAuthStatus,
          '/home',
        ),
        '/auth/complete-profile',
        reason: 'rule 2: incomplete profile is exclusively routed to the '
            'completion surface, never Home',
      );
      expect(login.sync.exchangeCalls, 1);

      // Sign-up entry — the SAME account must produce the SAME outcome.
      final signup = _build(user: _FakeUser('fb-g1'));
      addTearDown(signup.container.dispose);
      final signupFlow = signup.controller.signUpWithGoogle();
      await _flush();
      signup.sync.completeNext(Result.success(_syncIncomplete()));
      await signupFlow;
      expect(signup.controller.state, isA<AuthStateRequiresProfileCompletion>());
      expect(
        handleAuthRedirectForTest(
          signup.controller.state,
          signup.controller.appAuthStatus,
          '/home',
        ),
        '/auth/complete-profile',
      );
      expect(signup.sync.exchangeCalls, 1);

      // Parity: same terminal state type, same exchange count, and NEITHER
      // entry passed any link credential into the Google call (no auto-merge
      // input).
      expect(
        signup.controller.state.runtimeType,
        login.controller.state.runtimeType,
      );
      expect(login.repo.googlePendingCredentials, [null]);
      expect(signup.repo.googlePendingCredentials, [null]);
    });
  });

  group('G3+G4 — existing Google account with a complete profile → Home, '
      'from BOTH entry points', () {
    test('Login entry reaches Authenticated directly', () async {
      final env = _build(user: _FakeUser('fb-g2'));
      addTearDown(env.container.dispose);
      final flow = env.controller.signInWithGoogle();
      await _flush();
      env.sync.completeNext(Result.success(_syncComplete()));
      await flow;

      expect(env.controller.state, isA<AuthStateAuthenticated>());
      expect(
        handleAuthRedirectForTest(
          env.controller.state,
          env.controller.appAuthStatus,
          '/auth/complete-profile',
        ),
        '/home',
        reason: 'rule 3: a complete profile goes Home from any auth surface',
      );
    });

    test('Sign-up entry reaches the SAME Authenticated state and never sees '
        'RequiresProfileCompletion', () async {
      final env = _build(user: _FakeUser('fb-g2'));
      addTearDown(env.container.dispose);
      final transitions = <String>[];
      env.container.listen(authControllerProvider, (_, next) {
        transitions.add(next.runtimeType.toString());
      });

      final flow = env.controller.signUpWithGoogle();
      await _flush();
      env.sync.completeNext(Result.success(_syncComplete()));
      await flow;

      expect(env.controller.state, isA<AuthStateAuthenticated>());
      expect(
        transitions.where((t) => t.contains('RequiresProfileCompletion')),
        isEmpty,
        reason: 'rule 3/5: the stored username is backend truth — the '
            'sign-up page must never re-prompt it',
      );
      expect(env.sync.exchangeCalls, 1);
    });
  });

  group('G5 — Complete Profile success from the Google-seeded state', () {
    test('success → the single Authenticated state → /home', () async {
      final env = _build(
        user: _FakeUser('fb-g1'),
        repo: _GoogleScriptedRepo(
          completeProfile: (username) async => Result.success(
            _account(username: username),
          ),
        ),
      );
      addTearDown(env.container.dispose);

      final flow = env.controller.signInWithGoogle();
      await _flush();
      env.sync.completeNext(Result.success(_syncIncomplete()));
      await flow;
      expect(env.controller.state, isA<AuthStateRequiresProfileCompletion>());

      final outcome = await env.controller.completeProfile(username: 'newguy');
      await _flush();

      expect(outcome.success, isTrue);
      expect(env.repo.completeProfileCalls, 1);
      expect(env.controller.state, isA<AuthStateAuthenticated>());
      expect(
        handleAuthRedirectForTest(
          env.controller.state,
          env.controller.appAuthStatus,
          '/auth/complete-profile',
        ),
        '/home',
      );
    });
  });

  group('G6 — Complete Profile failure: visible outcome, never a fake Home',
      () {
    test('username rejection → inline outcome, state preserved, still on '
        '/auth/complete-profile', () async {
      final env = _build(
        user: _FakeUser('fb-g1'),
        repo: _GoogleScriptedRepo(
          completeProfile: (username) async => Result.error(
            'taken',
            code: 'USERNAME_TAKEN',
            statusCode: 409,
          ),
        ),
      );
      addTearDown(env.container.dispose);

      final flow = env.controller.signInWithGoogle();
      await _flush();
      env.sync.completeNext(Result.success(_syncIncomplete()));
      await flow;

      final outcome = await env.controller.completeProfile(username: 'taken');
      await _flush();

      expect(outcome.success, isFalse);
      expect(outcome.usernameError, isNotNull);
      expect(env.controller.state, isA<AuthStateRequiresProfileCompletion>());
      expect(
        handleAuthRedirectForTest(
          env.controller.state,
          env.controller.appAuthStatus,
          '/auth/complete-profile',
        ),
        isNull,
        reason: 'the user stays on the completion surface to correct the '
              'username — no fake redirect',
      );
    });

    test('transient backend failure → failure outcome, state preserved', () async {
      final env = _build(
        user: _FakeUser('fb-g1'),
        repo: _GoogleScriptedRepo(
          completeProfile: (username) async =>
              Result.error('down', statusCode: 503),
        ),
      );
      addTearDown(env.container.dispose);

      final flow = env.controller.signInWithGoogle();
      await _flush();
      env.sync.completeNext(Result.success(_syncIncomplete()));
      await flow;

      final outcome = await env.controller.completeProfile(username: 'guy');
      await _flush();

      expect(outcome.success, isFalse);
      expect(outcome.failureError, isNotNull);
      expect(env.controller.state, isA<AuthStateRequiresProfileCompletion>());
      expect(
        handleAuthRedirectForTest(
          env.controller.state,
          env.controller.appAuthStatus,
          '/home',
        ),
        '/auth/complete-profile',
      );
    });

    test('session mismatch → terminal error state, never /home', () async {
      final env = _build(
        user: _FakeUser('fb-g1'),
        repo: _GoogleScriptedRepo(
          completeProfile: (username) async => Result.error(
            'Backend session user mismatch',
            code: 'SESSION_USER_MISMATCH',
            statusCode: 409,
          ),
        ),
      );
      addTearDown(env.container.dispose);

      final flow = env.controller.signInWithGoogle();
      await _flush();
      env.sync.completeNext(Result.success(_syncIncomplete()));
      await flow;

      final outcome = await env.controller.completeProfile(username: 'guy');
      await _flush();

      expect(outcome.success, isFalse);
      expect(env.controller.state, isA<AuthStateError>());
      expect(
        handleAuthRedirectForTest(
          env.controller.state,
          env.controller.appAuthStatus,
          '/home',
        ),
        isNot('/home'),
        reason: 'a mismatched session surfaces a terminal error — it must '
            'never land on Home',
      );
    });
  });

  group('G7 — Google session + logout + late Firebase event', () {
    test('a late auth event after signOut never resurrects the session',
        () async {
      final user = _FakeUser('fb-g2');
      final env = _build(user: user);
      addTearDown(env.container.dispose);

      final flow = env.controller.signInWithGoogle();
      await _flush();
      env.sync.completeNext(Result.success(_syncComplete()));
      await flow;
      expect(env.controller.state, isA<AuthStateAuthenticated>());

      await env.controller.signOut();
      expect(env.controller.state, isA<AuthStateUnauthenticated>());

      final exchangesBefore = env.sync.exchangeCalls;
      final googleCallsBefore = env.repo.googleCallCount;
      await env.controller.debugHandleFirebaseAuthEvent(user);
      await _flush();

      expect(env.controller.state, isA<AuthStateUnauthenticated>());
      expect(env.sync.exchangeCalls, exchangesBefore,
          reason: 'no session may be resurrected from a stale auth event');
      expect(env.repo.googleCallCount, googleCallsBefore);
    });
  });

  group('G8 — same email: one identity, one profile, no auto-merge', () {
    test('Google login of an existing account runs exactly one exchange and '
        'zero linking calls', () async {
      final env = _build(user: _FakeUser('fb-g2'));
      addTearDown(env.container.dispose);

      final flow = env.controller.signInWithGoogle();
      await _flush();
      env.sync.completeNext(Result.success(_syncComplete()));
      await flow;

      expect(env.controller.state, isA<AuthStateAuthenticated>());
      expect(env.sync.exchangeCalls, 1,
          reason: 'one exchange mints one backend session for one identity');
      expect(env.repo.googleCallCount, 1);
      expect(env.repo.googlePendingCredentials, [null],
          reason: 'no pending/link credential is ever synthesized from an '
              'email match — merging is UID-scoped, explicit, and backend-'
              'gated (unbound bind / IDENTITY_CONFLICT 409)');
      expect(env.repo.completeProfileCalls, 0,
          reason: 'a complete profile must never re-enter completion');
    });
  });
}
