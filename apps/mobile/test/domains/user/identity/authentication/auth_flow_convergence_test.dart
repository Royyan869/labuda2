// AUTH FLOW CONVERGENCE — automated proof for the email + Google
// registration/login convergence (canonical business rules 1-8).
//
// Drives the REAL AuthController (and the pure router redirect seam) through
// both canonical flows plus their races:
//   P1+P2  email registration → park survives late Firebase event →
//          verify → backend sync → authenticated
//   P3     unverified identity can never reach the exchange (D2 hard gate)
//   P4+P6  Google new user → RequiresProfileCompletion → completeProfile →
//          authenticated (single canonical end state)
//   P5     stored username is never re-prompted on re-login (email & Google)
//   P7     existing email/password and Google logins still converge
//   P8     backend exchange failure surfaces a visible terminal state —
//          never an eternal SyncingWithBackend/splash
//   P9     logout + late auth events never resurrect a stale session;
//          explicit intents are sync locks and die with the session
//   I1     post-verification bridge force-refreshes the Firebase ID token so
//          the exchange carries fresh `email_verified` claims
//   I4     a no-op forceRefreshAuthState cannot swallow an in-flight sync's
//          terminal publish (hydration-generation authority)

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

  /// Records every getIdToken call's forceRefresh argument.
  final List<bool> idTokenForceFlags = [];

  /// When set, `reload()` parks until the completer resolves (models the
  /// real-device latency of the listener's Firebase profile refresh).
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
  Future<String?> getIdToken([bool forceRefresh = false]) async {
    idTokenForceFlags.add(forceRefresh);
    return 'id-token';
  }
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
  Future<Result<void>> setRestrictedToken(String t) async =>
      Result.success(null);
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
  final List<String> exchangeUsernames = [];
  final List<Completer<Result<SyncUserResult>>> pendingExchange =
      <Completer<Result<SyncUserResult>>>[];
  final List<Completer<Result<AuthUser>>> pendingCurrentUser =
      <Completer<Result<AuthUser>>>[];

  @override
  Future<Result<SyncUserResult>> syncUser({
    required String username,
    String? phoneNumber,
  }) {
    exchangeCalls++;
    exchangeUsernames.add(username);
    final completer = Completer<Result<SyncUserResult>>();
    pendingExchange.add(completer);
    return completer.future;
  }

  @override
  Future<Result<AuthUser>> getCurrentUser() {
    final completer = Completer<Result<AuthUser>>();
    pendingCurrentUser.add(completer);
    return completer.future;
  }

  void completeExchange(Result<SyncUserResult> result) {
    pendingExchange.removeAt(0).complete(result);
  }

  void completeCurrentUser(Result<AuthUser> result) {
    pendingCurrentUser.removeAt(0).complete(result);
  }
}

/// Scriptable repository: sign-in/sign-up futures are controlled per test.
class _ScriptedRepo extends Fake implements IAuthRepository {
  _ScriptedRepo({
    Future<Result<FirebasePrincipal>> Function()? signIn,
    Future<Result<FirebasePrincipal>> Function()? signUp,
    Future<Result<AuthUser>> Function(String username)? completeProfile,
  })  : _signIn = signIn,
        _signUp = signUp,
        _completeProfile = completeProfile;

  final Future<Result<FirebasePrincipal>> Function()? _signIn;
  final Future<Result<FirebasePrincipal>> Function()? _signUp;
  final Future<Result<AuthUser>> Function(String username)? _completeProfile;

  int completeProfileCalls = 0;

  @override
  Future<Result<FirebasePrincipal>> signInWithEmail({
    required String email,
    required String password,
  }) => _signIn!();
  @override
  Future<Result<FirebasePrincipal>> signUpWithEmail({
    required String email,
    required String password,
  }) => _signUp!();
  @override
  Future<Result<void>> signInWithGoogle({
    AuthCredential? pendingGoogleCredential,
  }) async => Result.success(null);
  @override
  Future<Result<AuthUser>> completeProfile({required String username}) async {
    completeProfileCalls++;
    return _completeProfile!(username);
  }
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

AuthUser _account({String username = 'seller-one'}) => AuthUser(
      id: 'backend-1',
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      email: 'a@example.com',
      username: username,
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      hasMarketAuthority: false,
      sellerSubscriptionStatus: 'none',
      roles: const [UserRole.user],
      provider: AuthProvider.email,
    );

SyncUserResult _syncComplete() => SyncUserResult(
      user: _account(),
      userId: 'backend-1',
      email: 'a@example.com',
      created: false,
      profileComplete: true,
      username: 'seller-one',
    );

SyncUserResult _syncIncomplete() => const SyncUserResult(
      user: null,
      userId: 'backend-1',
      email: 'a@example.com',
      created: true,
      profileComplete: false,
      username: '',
    );

({_TestController controller, _RecordingSyncService sync, ProviderContainer container})
    _build({
  required _FakeUser user,
  required IAuthRepository repo,
}) {
  final controller = _TestController(user);
  final sync = _RecordingSyncService(auth: user);
  final container = ProviderContainer(
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
  container.read(authControllerProvider);
  return (controller: controller, sync: sync, container: container);
}

Future<void> _flush([int n = 4]) async {
  for (var i = 0; i < n; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('P1+P2+I1 — email registration: park survives late events, fresh '
      'claims at the verify bridge, sync converges to authenticated', () {
    test(
      'signUpWithEmail parks; late listener emission keeps the park AND the '
      'signup intent; verification then exchanges exactly once with the '
      'registration username and a force-refreshed ID token',
      () async {
        final user = _FakeUser('fb-1', emailVerifiedValue: false);
        final env = _build(
          user: user,
          repo: _ScriptedRepo(
            signUp: () async => Result.success(
              FirebasePrincipal(uid: 'fb-1', emailVerified: false),
            ),
          ),
        );
        addTearDown(env.container.dispose);
        final transitions = <String>[];
        env.container.listen(authControllerProvider, (_, next) {
          transitions.add(next.runtimeType.toString());
        });

        // Registration: Firebase account created, verification email sent,
        // session PARKED before any backend exchange (rule 1).
        await env.controller.signUpWithEmail(
          email: 'a@example.com',
          password: 'Password123!',
          username: 'royyan89',
        );
        expect(env.controller.state, isA<AuthStatePendingEmailVerification>());
        expect(env.sync.exchangeCalls, 0,
            reason: 'D2 hard gate: no exchange before verification');

        // Late Firebase emission (reload resolves AFTER the park) must not
        // stomp the park (listener deferral contract).
        user.reloadGate = Completer<void>();
        final listenerDone = env.controller.debugHandleFirebaseAuthEvent(user);
        await _flush();
        user.reloadGate!.complete();
        await listenerDone;
        await _flush();
        expect(env.controller.state, isA<AuthStatePendingEmailVerification>(),
            reason: 'late events must never steal the parked flow');
        expect(env.sync.exchangeCalls, 0);

        // User verifies out-of-band; the verify-screen poll bridges back.
        user.emailVerifiedValue = true;
        final poll = env.controller.checkPendingEmailVerification();
        await _flush();
        expect(env.sync.exchangeCalls, 1);
        env.sync.completeExchange(Result.success(_syncComplete()));
        await poll;
        await _flush();

        expect(env.controller.state, isA<AuthStateAuthenticated>());
        expect(env.sync.exchangeUsernames, ['royyan89'],
            reason: 'the registration username must ride the single '
                'post-verification exchange');
        expect(
          user.idTokenForceFlags.where((f) => f).length,
          1,
          reason: 'the post-verification bridge must force-refresh the '
              'Firebase ID token so the backend sees fresh email_verified '
              'claims (canonical profile requirement)',
        );
        expect(
          transitions.firstWhere((t) => t.contains('Authenticated')),
          contains('Authenticated'),
        );
      },
    );
  });

  group('P3 — unverified identity can never pass the gate to Home', () {
    test(
      'retryRegistrationUsername with an unverified identity parks on '
      'verification and performs zero exchanges',
      () async {
        final user = _FakeUser('fb-1', emailVerifiedValue: false);
        final env = _build(
          user: user,
          repo: _ScriptedRepo(
            signUp: () async => Result.success(
              FirebasePrincipal(uid: 'fb-1', emailVerified: false),
            ),
          ),
        );
        addTearDown(env.container.dispose);

        await env.controller.retryRegistrationUsername('correcteduser');
        expect(env.controller.state, isA<AuthStatePendingEmailVerification>());
        expect(env.sync.exchangeCalls, 0);
      },
    );

    test(
      'direct verified email login never force-refreshes the ID token '
      '(the bridge-only freshness contract keeps ordinary logins fast)',
      () async {
        final user = _FakeUser('fb-1');
        final env = _build(
          user: user,
          repo: _ScriptedRepo(
            signIn: () async => Result.success(
              FirebasePrincipal(uid: 'fb-1', emailVerified: true),
            ),
          ),
        );
        addTearDown(env.container.dispose);

        final login = env.controller.signInWithEmail(
          email: 'a@example.com',
          password: 'pw',
        );
        await _flush();
        env.sync.completeExchange(Result.success(_syncComplete()));
        await login;

        expect(env.controller.state, isA<AuthStateAuthenticated>());
        expect(user.idTokenForceFlags.where((f) => f), isEmpty);
      },
    );
  });

  group('P4+P5+P6 — Google flow: complete-profile once, then one canonical '
      'authenticated state', () {
    test(
      'new Google user → RequiresProfileCompletion → completeProfile → '
      'Authenticated, with the router sending each state to its exclusive '
      'surface',
      () async {
        final user = _FakeUser('fb-g1');
        final env = _build(
          user: user,
          repo: _ScriptedRepo(
            signIn: () async => Result.success(
              FirebasePrincipal(uid: 'fb-g1', emailVerified: true),
            ),
            completeProfile: (username) async => Result.success(
              _account(username: username),
            ),
          ),
        );
        addTearDown(env.container.dispose);

        final login = env.controller.signInWithGoogle();
        await _flush();
        env.sync.completeExchange(Result.success(_syncIncomplete()));
        await login;
        expect(env.controller.state, isA<AuthStateRequiresProfileCompletion>());
        expect(
          handleAuthRedirectForTest(
            env.controller.state,
            env.controller.appAuthStatus,
            '/home',
          ),
          '/auth/complete-profile',
          reason: 'rule 2: incomplete profile is routed to the exclusive '
              'complete-profile surface, never Home',
        );

        final outcome =
            await env.controller.completeProfile(username: 'googlenew');
        await _flush();
        expect(outcome.success, isTrue);
        expect(env.controller.state, isA<AuthStateAuthenticated>());
        expect(
          handleAuthRedirectForTest(
            env.controller.state,
            env.controller.appAuthStatus,
            '/auth/complete-profile',
          ),
          '/home',
          reason: 'rule 4/6: completion converges to the single '
              'authenticated state and Home',
        );
      },
    );

    test(
      'existing Google account with a stored username is NEVER sent back '
      'through Complete Profile (rule 3)',
      () async {
        final user = _FakeUser('fb-g2');
        final env = _build(
          user: user,
          repo: _ScriptedRepo(
            signIn: () async => Result.success(
              FirebasePrincipal(uid: 'fb-g2', emailVerified: true),
            ),
          ),
        );
        addTearDown(env.container.dispose);
        final transitions = <String>[];
        env.container.listen(authControllerProvider, (_, next) {
          transitions.add(next.runtimeType.toString());
        });

        final login = env.controller.signInWithGoogle();
        await _flush();
        env.sync.completeExchange(Result.success(_syncComplete()));
        await login;

        expect(env.controller.state, isA<AuthStateAuthenticated>());
        expect(
          transitions.where((t) => t.contains('RequiresProfileCompletion')),
          isEmpty,
          reason: 'a stored username must never re-prompt Complete Profile',
        );
      },
    );
  });

  group('P7 — existing logins converge (email & Google)', () {
    test('existing email/password login reaches Authenticated in one exchange',
        () async {
      final user = _FakeUser('fb-1');
      final env = _build(
        user: user,
        repo: _ScriptedRepo(
          signIn: () async => Result.success(
            FirebasePrincipal(uid: 'fb-1', emailVerified: true),
          ),
        ),
      );
      addTearDown(env.container.dispose);

      final login = env.controller.signInWithEmail(
        email: 'a@example.com',
        password: 'pw',
      );
      await _flush();
      env.sync.completeExchange(Result.success(_syncComplete()));
      await login;

      expect(env.controller.state, isA<AuthStateAuthenticated>());
      expect(env.sync.exchangeCalls, 1);
    });
  });

  group('P8 — backend failures leave a visible terminal state, never an '
      'eternal splash', () {
    test('exchange failure at the verify bridge → BackendUnavailable', () async {
      final user = _FakeUser('fb-1', emailVerifiedValue: false);
      final env = _build(
        user: user,
        repo: _ScriptedRepo(
          signUp: () async => Result.success(
            FirebasePrincipal(uid: 'fb-1', emailVerified: false),
          ),
        ),
      );
      addTearDown(env.container.dispose);

      user.emailVerifiedValue = true;
      final poll = env.controller.checkPendingEmailVerification();
      await _flush();
      env.sync.completeExchange(
        Result.error('network down', statusCode: 503),
      );
      await poll;
      await _flush();

      expect(env.controller.state, isA<AuthStateBackendUnavailable>());
      expect(
        (env.controller.state as AuthStateBackendUnavailable).message,
        isNotEmpty,
        reason: 'degraded states must carry a user-visible message (splash '
            'renders the retry UI)',
      );
    });

    test('exchange validation failure at the verify bridge → BackendFailure',
        () async {
      final user = _FakeUser('fb-1', emailVerifiedValue: false);
      final env = _build(
        user: user,
        repo: _ScriptedRepo(
          signUp: () async => Result.success(
            FirebasePrincipal(uid: 'fb-1', emailVerified: false),
          ),
        ),
      );
      addTearDown(env.container.dispose);

      user.emailVerifiedValue = true;
      final poll = env.controller.checkPendingEmailVerification();
      await _flush();
      env.sync.completeExchange(
        Result.error('validation rejected', statusCode: 422),
      );
      await poll;
      await _flush();

      expect(env.controller.state, isA<AuthStateBackendFailure>());
    });
  });

  group('P9 — logout: no stale resurrection, intents die with the session',
      () {
    test(
      'signOut then a late Firebase emission stays Unauthenticated with '
      'zero exchanges',
      () async {
        final user = _FakeUser('fb-1');
        final env = _build(
          user: user,
          repo: _ScriptedRepo(
            signIn: () async => Result.success(
              FirebasePrincipal(uid: 'fb-1', emailVerified: true),
            ),
          ),
        );
        addTearDown(env.container.dispose);

        final login = env.controller.signInWithEmail(
          email: 'a@example.com',
          password: 'pw',
        );
        await _flush();
        env.sync.completeExchange(Result.success(_syncComplete()));
        await login;
        expect(env.controller.state, isA<AuthStateAuthenticated>());

        await env.controller.signOut();
        expect(env.controller.state, isA<AuthStateUnauthenticated>());

        final exchangesBefore = env.sync.exchangeCalls;
        await env.controller.debugHandleFirebaseAuthEvent(user);
        await _flush();

        expect(env.controller.state, isA<AuthStateUnauthenticated>(),
            reason: 'a late auth event must never resurrect a signed-out '
                'session');
        expect(env.sync.exchangeCalls, exchangesBefore);
      },
    );

    test(
      'signOut clears a kept registration intent — the intent is a sync '
      'lock, not session state',
      () async {
        final user = _FakeUser('fb-1');
        final env = _build(
          user: user,
          repo: _ScriptedRepo(
            signUp: () async => Result.success(
              FirebasePrincipal(uid: 'fb-1', emailVerified: true),
            ),
          ),
        );
        addTearDown(env.container.dispose);

        // Already-verified signup whose exchange rejects the username:
        // the controller returns to Unauthenticated but KEEPS the
        // EmailSignupIntent for the Stage 1C retry — a live explicit intent.
        final signup = env.controller.signUpWithEmail(
          email: 'a@example.com',
          password: 'Password123!',
          username: 'takenname',
        );
        await _flush();
        env.sync.completeExchange(
          Result.error('taken', code: 'USERNAME_TAKEN', statusCode: 409),
        );
        await signup;
        expect(env.controller.state, isA<AuthStateUnauthenticated>());
        expect(env.controller.hasPendingRegistration, isTrue,
            reason: 'Stage 1C: the registration intent survives the '
                'rejection for the username retry');

        // Logout ends the SESSION — and every explicit intent with it.
        await env.controller.signOut();
        expect(
          env.controller.hasPendingRegistration,
          isFalse,
          reason: 'a registration intent must never outlive signOut '
                  '(it would make the next session\'s listener emissions '
                  'defer to a flow that no longer exists)',
        );

        final exchangesBefore = env.sync.exchangeCalls;
        await env.controller.debugHandleFirebaseAuthEvent(user);
        await _flush();
        expect(env.controller.state, isA<AuthStateUnauthenticated>());
        expect(env.sync.exchangeCalls, exchangesBefore);
      },
    );
  });

  group('I4 — hydration-generation authority', () {
    test(
      'forceRefreshAuthState from an invalid state is a no-op that cannot '
      'swallow an in-flight sync terminal publish',
      () async {
        final user = _FakeUser('fb-1');
        final env = _build(
          user: user,
          repo: _ScriptedRepo(
            signIn: () async => Result.success(
              FirebasePrincipal(uid: 'fb-1', emailVerified: true),
            ),
          ),
        );
        addTearDown(env.container.dispose);

        final login = env.controller.signInWithEmail(
          email: 'a@example.com',
          password: 'pw',
        );
        await _flush();
        expect(env.controller.state, isA<AuthStateSyncingWithBackend>());

        // Invalid-state refresh: must not bump the hydration generation.
        await env.controller.forceRefreshAuthState();

        env.sync.completeExchange(Result.success(_syncComplete()));
        await login;
        await _flush();

        expect(
          env.controller.state,
          isA<AuthStateAuthenticated>(),
          reason: 'a no-op forceRefreshAuthState must never invalidate the '
              'in-flight sync publish (pre-fix: wedged on '
              'SyncingWithBackend → eternal splash)',
        );
      },
    );
  });
}
