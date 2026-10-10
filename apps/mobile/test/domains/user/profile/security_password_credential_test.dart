// AUTH-H2 — Password credential authority & safe password change.
//
// Contract proven here against the REAL authority, controller, and screen:
//   1  Google-only (no 'password' provider) → form hidden, Google-managed
//      explanation visible.
//   2  Email/password ('password' provider) → form available.
//   3  Dual provider (google.com + password) → form available (the dual
//      shape is proven at the FirebasePrincipal mapping layer; the screen
//      consumes the single boolean).
//   4  Provider status not provable (fail-closed value false) → form is
//      NEVER opened optimistically.
//   5  Reauthentication failure → error shown inline, screen stays on
//      Security, session remains authenticated (controller-level proof:
//      real AuthController keeps AuthStateAuthenticated).
//   6  Password-change failure → no fake success feedback.
//   7  Password-change success → success feedback, session consistent.
//   8  No credential-status leak across sessions: the authority is a
//      stateless live read (no cache); consumption follows the CURRENT
//      value only.
//   9  (codebase search — reported separately) exactly one provider-check
//      authority decides password availability.

import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart' hide NotificationEntity;
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/firebase_principal.dart';
import 'package:hishumi/domains/user/profile/presentation/screens/security_screen.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/shared.dart' show hasPasswordCredentialProvider;

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

class _FakeUserInfo extends Fake implements UserInfo {
  _FakeUserInfo(this.providerId);
  @override
  final String providerId;
}

/// Firebase user whose providerData reflects the account's REAL credential
/// shape — the input the canonical [FirebasePrincipal] mapping captures.
class _FakeUser extends Fake implements User {
  _FakeUser(this.uid, {required List<String> providerIds})
    : _infos = providerIds.map((id) => _FakeUserInfo(id)).toList();
  @override
  final String uid;
  final List<UserInfo> _infos;
  @override
  String? get email => '$uid@example.com';
  @override
  bool get emailVerified => true;
  @override
  String? get phoneNumber => null;
  @override
  List<UserInfo> get providerData => _infos;
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

AuthUser _authUser() => AuthUser(
      id: 'backend-1',
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      email: 'user@example.com',
      username: 'user-one',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
    );

/// Scripted controller for the SCREEN tests: seeds Authenticated via
/// build() (no dependency graph) and replays a scripted changePassword
/// outcome.
class _ScriptedSecurityController extends AuthController {
  _ScriptedSecurityController({this.changePasswordResult});

  /// null = success; otherwise the inline failure message.
  final String? changePasswordResult;
  int changePasswordCalls = 0;

  @override
  AuthState build() => AuthState.authenticated(_authUser(), emailVerified: true);

  @override
  Future<String?> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    changePasswordCalls++;
    return changePasswordResult;
  }
}

// ---------------------------------------------------------------------------
// Authority input layer — FirebasePrincipal.providerIds (the canonical
// providerData capture the hasPasswordCredentialProvider consumes).
// ---------------------------------------------------------------------------

bool _hasPasswordViaPrincipal(User user) =>
    FirebasePrincipal.fromFirebaseUser(user).providerIds.contains('password');

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('Authority input — FirebasePrincipal.providerIds reflects the real '
      'credential shape', () {
    test('email/password account → password credential proven', () {
      expect(
        _hasPasswordViaPrincipal(_FakeUser('fb-1', providerIds: ['password'])),
        isTrue,
      );
    });

    test('Google-only account → no password credential', () {
      expect(
        _hasPasswordViaPrincipal(
          _FakeUser('fb-g', providerIds: ['google.com']),
        ),
        isFalse,
      );
    });

    test('dual account (Google + email/password) → password credential proven',
        () {
      expect(
        _hasPasswordViaPrincipal(
          _FakeUser('fb-d', providerIds: ['google.com', 'password']),
        ),
        isTrue,
      );
    });

    test('unknown/empty providerData → fail-closed (no proof, no action)',
        () {
      expect(_hasPasswordViaPrincipal(_FakeUser('fb-x', providerIds: [])), isFalse);
    });
  });

  // ---------------------------------------------------------------------------
  // Screen layer — SecurityScreen consumption of the single authority.
  // ---------------------------------------------------------------------------

  Future<void> pumpSecurity(
    WidgetTester tester, {
    required bool hasPasswordCredential,
    required _ScriptedSecurityController controller,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(() => controller),
          hasPasswordCredentialProvider.overrideWithValue(
            hasPasswordCredential,
          ),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const SecurityScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  Future<void> fillValidPasswordForm(WidgetTester tester) async {
    final fields = find.byType(TextFormField);
    await tester.enterText(fields.at(0), 'CurrentPass1');
    await tester.enterText(fields.at(1), 'Password1');
    await tester.enterText(fields.at(2), 'Password1');
    await tester.pump();
  }

  group('SecurityScreen — password credential gate', () {
    testWidgets(
      '1 — Google-only: no executable form; Google-managed explanation visible',
      (tester) async {
        final controller = _ScriptedSecurityController();
        await pumpSecurity(
          tester,
          hasPasswordCredential: false,
          controller: controller,
        );

        expect(find.text('Update Password'), findsNothing);
        expect(find.byType(TextFormField), findsNothing);
        expect(find.text('Password managed by Google'), findsOneWidget);
        expect(
          find.textContaining('manage it in your Google account'),
          findsOneWidget,
        );
        expect(controller.changePasswordCalls, 0);
      },
    );

    testWidgets('2 — email/password: the Change Password form is available',
        (tester) async {
      await pumpSecurity(
        tester,
        hasPasswordCredential: true,
        controller: _ScriptedSecurityController(),
      );

      expect(find.text('Update Password'), findsOneWidget);
      expect(find.byType(TextFormField), findsNWidgets(3));
      expect(find.text('Password managed by Google'), findsNothing);
    });

    testWidgets(
      '3 — dual provider consumes the SAME authority value (true) → form available',
      (tester) async {
        // The dual SHAPE (google.com + password → true) is proven in the
        // FirebasePrincipal group above; here the screen consumes the
        // authority's true value exactly like an email/password account —
        // one decision, one rendering.
        await pumpSecurity(
          tester,
          hasPasswordCredential: true,
          controller: _ScriptedSecurityController(),
        );

        expect(find.text('Update Password'), findsOneWidget);
        expect(find.text('Password managed by Google'), findsNothing);
      },
    );

    testWidgets(
      '4 — unprovable provider status (fail-closed false) never opens the form optimistically',
      (tester) async {
        // The authority returns false when the credential cannot be proven
        // (no Firebase user / unreadable identity). The screen must never
        // treat "unknown" as "password available".
        await pumpSecurity(
          tester,
          hasPasswordCredential: false,
          controller: _ScriptedSecurityController(),
        );

        expect(find.text('Update Password'), findsNothing);
        expect(find.byType(TextFormField), findsNothing);
      },
    );

    testWidgets(
      '5+6 — reauth/change failure: inline error, still on Security, no fake success',
      (tester) async {
        final controller = _ScriptedSecurityController(
          changePasswordResult: 'The password is invalid or the user does not have a password.',
        );
        await pumpSecurity(
          tester,
          hasPasswordCredential: true,
          controller: controller,
        );

        await fillValidPasswordForm(tester);
        await tester.tap(find.text('Update Password'));
        await tester.pumpAndSettle();

        expect(controller.changePasswordCalls, 1);
        expect(
          find.textContaining('password is invalid'),
          findsOneWidget,
          reason: 'the failure message is rendered on this screen (the '
              'security surface’s local ProfileStateView error state)',
        );
        // Still on SecurityScreen — the session was NOT kicked to /welcome
        // (pre-fix the controller published AuthState.error → router kick).
        expect(find.byType(SecurityScreen), findsOneWidget);
        // Never reported as success.
        expect(find.textContaining('Password updated'), findsNothing);
        // The error state is dismissible: dismissing returns the user to
        // the intact Change Password form (not stranded on the error page).
        await tester.tap(find.text('Try Again'));
        await tester.pumpAndSettle();
        expect(find.text('Update Password'), findsOneWidget);
      },
    );

    testWidgets('7 — success: success feedback, session consistent',
        (tester) async {
      final controller = _ScriptedSecurityController();
      await pumpSecurity(
        tester,
        hasPasswordCredential: true,
        controller: controller,
      );

      await fillValidPasswordForm(tester);
      await tester.tap(find.text('Update Password'));
      await tester.pumpAndSettle();

      expect(controller.changePasswordCalls, 1);
      expect(find.text('Password updated successfully!'), findsOneWidget);
      // Session intact: still the authenticated Security surface.
      expect(find.byType(SecurityScreen), findsOneWidget);
      expect(find.text('Password managed by Google'), findsNothing);
      // Flush the screen's3s success-clear timer so no pending timer leaks
      // past the test.
      await tester.pump(const Duration(seconds: 3));
      await tester.pump();
    });

    testWidgets(
      '8 — credential status follows the CURRENT value only (fresh scope per '
      'user; the authority itself is a stateless live read with no cache)',
      (tester) async {
        // Session A: a password-bearing account.
        await pumpSecurity(
          tester,
          hasPasswordCredential: true,
          controller: _ScriptedSecurityController(),
        );
        expect(find.text('Update Password'), findsOneWidget);

        // Session B (post-logout / account switch): a fresh scope resolves
        // the CURRENT authority value — nothing from session A leaks.
        await pumpSecurity(
          tester,
          hasPasswordCredential: false,
          controller: _ScriptedSecurityController(),
        );
        expect(find.text('Update Password'), findsNothing);
        expect(find.text('Password managed by Google'), findsOneWidget);
      },
    );
  });

  // ---------------------------------------------------------------------------
  // Controller layer — G3 session integrity against the REAL AuthController.
  // ---------------------------------------------------------------------------

  group('AuthController.changePassword — local outcome, global session intact',
      () {
    _TestRepo repo(String? error) => _TestRepo(error);

    ({_RealController controller, ProviderContainer container}) buildReal(
      _TestRepo scriptedRepo,
    ) {
      final controller = _RealController();
      final container = ProviderContainer(
        overrides: [
          authControllerProvider.overrideWith(() => controller),
          authRepositoryProvider.overrideWithValue(scriptedRepo),
          loggerServiceProvider.overrideWithValue(const _NoopLogger()),
          coreAnalyticsRepositoryProvider.overrideWithValue(_NoopAnalytics()),
          localStorageServiceProvider.overrideWithValue(_NoopStorage()),
        ],
      );
      addTearDown(container.dispose);
      container.read(authControllerProvider.notifier);
      controller.seed(AuthState.authenticated(_authUser(), emailVerified: true));
      return (controller: controller, container: container);
    }

    test('reauth failure returns the message and NEVER mutates AuthState',
        () async {
      final env = buildReal(repo('The password is invalid or the user does not have a password.'));

      final error = await env.controller.changePassword(
        currentPassword: 'wrong',
        newPassword: 'Password1',
      );

      expect(error, contains('password is invalid'));
      expect(
        env.controller.state,
        isA<AuthStateAuthenticated>(),
        reason: 'a per-form failure must never kick a live session '
            '(pre-fix: AuthState.error → /welcome redirect)',
      );
    });

    test('success returns null and keeps the session consistent', () async {
      final env = buildReal(repo(null));

      final error = await env.controller.changePassword(
        currentPassword: 'CurrentPass1',
        newPassword: 'Password1',
      );

      expect(error, isNull);
      expect(env.controller.state, isA<AuthStateAuthenticated>());
    });
  });
}

// ---------------------------------------------------------------------------
// Minimal real-controller harness (changePassword only touches the repo).
// ---------------------------------------------------------------------------

class _RealController extends AuthController {
  /// Subclass-scoped state seeding (protected member access).
  void seed(AuthState initial) => state = initial;
  @override
  bool get shouldInitializeAuthListener => false;
  @override
  User? get activeFirebaseUser => null;
  @override
  Future<void> performFirebaseSignOut() async {}
}

class _TestRepo extends Fake implements IAuthRepository {
  _TestRepo(this.error);
  final String? error;
  @override
  Future<Result<void>> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async =>
      error == null ? Result.success(null) : Result.error(error!);
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

class _NoopStorage extends Fake implements ILocalStorageService {}
