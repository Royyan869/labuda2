// PASS 2C: EmailVerificationBanner resume behavior.
//
// EmailVerificationBanner.didChangeAppLifecycleState(resumed) calls the
// email-verification refresh path only when the user is authenticated AND
// their email is not yet verified — this lets a user who verified their
// email in a browser/email client see the banner disappear the moment they
// return to the app, without polling or a full auth sync.
import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/email_verification_controller.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/email_verification_state.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/email_verification_banner.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._initialState);

  final AuthState _initialState;

  @override
  AuthState build() => _initialState;

  @override
  Future<void> refreshAuthState() async {
    fail('refreshAuthState() must not be called by EmailVerificationBanner');
  }
}

class _FakeFirebaseAuth extends Fake implements FirebaseAuth {
  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();

  @override
  User? get currentUser => null;
}

class _FakeEmailVerificationController extends EmailVerificationController {
  _FakeEmailVerificationController() : super(firebaseAuth: _FakeFirebaseAuth());

  int refreshCallCount = 0;

  @override
  EmailVerificationState build() => const EmailVerificationState.initial();

  @override
  Future<void> refreshEmailVerificationStatus() async {
    refreshCallCount++;
  }
}

AuthUser _testUser() {
  return AuthUser(
    id: 'user-1',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'test@test.com',
    username: 'testuser',
    isEmailVerified: false,
    accountStatus: AccountStatus.active,
    hasSellerProfile: false,
    sellerSubscriptionStatus: 'none',
    hasMarketAuthority: false,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

Widget _wrap(
  AuthController authController,
  EmailVerificationController verificationController,
) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => authController),
      emailVerificationControllerProvider.overrideWith(
        () => verificationController,
      ),
    ],
    child: const MaterialApp(home: Scaffold(body: EmailVerificationBanner())),
  );
}

/// Dispatches a resumed lifecycle event through the SAME WidgetsBinding the
/// widget under test registered its observer with, exercising the real
/// didChangeAppLifecycleState callback rather than calling it directly.
void _resumeApp(WidgetTester tester) {
  tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
}

void main() {
  group('EmailVerificationBanner resume refresh', () {
    testWidgets(
      'authenticated + emailVerified=false: resume calls refreshEmailVerificationStatus',
      (tester) async {
        final authController = _FakeAuthController(
          AuthState.authenticated(_testUser(), emailVerified: false),
        );
        final verificationController = _FakeEmailVerificationController();

        await tester.pumpWidget(_wrap(authController, verificationController));
        await tester.pump();

        expect(verificationController.refreshCallCount, 0);

        _resumeApp(tester);
        await tester.pump();

        expect(verificationController.refreshCallCount, 1);
      },
    );

    testWidgets(
      'authenticated + emailVerified=true: resume does NOT call refreshEmailVerificationStatus',
      (tester) async {
        final authController = _FakeAuthController(
          AuthState.authenticated(_testUser(), emailVerified: true),
        );
        final verificationController = _FakeEmailVerificationController();

        await tester.pumpWidget(_wrap(authController, verificationController));
        await tester.pump();

        _resumeApp(tester);
        await tester.pump();

        expect(verificationController.refreshCallCount, 0);
      },
    );

    testWidgets(
      'unauthenticated: resume does NOT call refreshEmailVerificationStatus (no user to check)',
      (tester) async {
        final authController = _FakeAuthController(
          const AuthState.unauthenticated(),
        );
        final verificationController = _FakeEmailVerificationController();

        await tester.pumpWidget(_wrap(authController, verificationController));
        await tester.pump();

        _resumeApp(tester);
        await tester.pump();

        expect(verificationController.refreshCallCount, 0);
      },
    );

    testWidgets(
      'multiple resume events while still unverified call refresh each time '
      '(no debounce assumed — the verification refresh path is expected to '
      'handle repeated checks safely)',
      (tester) async {
        final authController = _FakeAuthController(
          AuthState.authenticated(_testUser(), emailVerified: false),
        );
        final verificationController = _FakeEmailVerificationController();

        await tester.pumpWidget(_wrap(authController, verificationController));
        await tester.pump();

        _resumeApp(tester);
        await tester.pump();
        _resumeApp(tester);
        await tester.pump();

        expect(verificationController.refreshCallCount, 2);
      },
    );

    testWidgets(
      'disposes observer safely: resume after the banner is removed from '
      'the tree does not call refreshEmailVerificationStatus and does not throw (no '
      'disposed-notifier/ref-after-dispose issue)',
      (tester) async {
        final authController = _FakeAuthController(
          AuthState.authenticated(_testUser(), emailVerified: false),
        );
        final verificationController = _FakeEmailVerificationController();

        await tester.pumpWidget(_wrap(authController, verificationController));
        await tester.pump();

        // Replace the tree so EmailVerificationBanner's State.dispose() runs
        // (and, per its implementation, removes itself from
        // WidgetsBinding's observer list).
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authControllerProvider.overrideWith(() => authController),
              emailVerificationControllerProvider.overrideWith(
                () => verificationController,
              ),
            ],
            child: const MaterialApp(home: Scaffold(body: SizedBox.shrink())),
          ),
        );
        await tester.pump();

        expect(
          () => _resumeApp(tester),
          returnsNormally,
          reason:
              'a disposed observer must not still be registered with '
              'WidgetsBinding, and must not touch ref/ConsumerState after '
              'dispose',
        );
        await tester.pump();

        // No new call after disposal — the removed widget's observer must
        // not have fired.
        expect(verificationController.refreshCallCount, 0);
      },
    );
  });

  group(
    'EmailVerificationBanner build (regression guard, not resume-specific)',
    () {
      testWidgets('renders nothing when email is verified', (tester) async {
        final authController = _FakeAuthController(
          AuthState.authenticated(_testUser(), emailVerified: true),
        );
        final verificationController = _FakeEmailVerificationController();

        await tester.pumpWidget(_wrap(authController, verificationController));
        await tester.pump();

        expect(find.byType(SizedBox), findsOneWidget);
        expect(find.textContaining('Verifikasi'), findsNothing);
      });

      testWidgets('renders the banner when email is not verified', (
        tester,
      ) async {
        final authController = _FakeAuthController(
          AuthState.authenticated(_testUser(), emailVerified: false),
        );
        final verificationController = _FakeEmailVerificationController();

        await tester.pumpWidget(_wrap(authController, verificationController));
        await tester.pump();

        expect(find.textContaining('Verifikasi'), findsWidgets);
      });
    },
  );
}
