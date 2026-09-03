import 'dart:async';

import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/email_verification_controller.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/email_verification_state.dart';

class _MockFirebaseUser extends Fake implements User {
  _MockFirebaseUser({
    required this.emailVerifiedValue,
    this.reloadThrows = false,
    this.reloadCompleter,
  });

  final bool emailVerifiedValue;
  final bool reloadThrows;
  final Completer<void>? reloadCompleter;
  int reloadCalls = 0;

  @override
  String get uid => 'user-1';

  @override
  String? get email => 'verified@test.com';

  @override
  bool get emailVerified => emailVerifiedValue;

  @override
  Future<void> reload() async {
    reloadCalls++;
    if (reloadCompleter != null) {
      await reloadCompleter!.future;
    }
    if (reloadThrows) {
      throw Exception('reload failed');
    }
  }
}

class _MockFirebaseAuth extends Fake implements FirebaseAuth {
  _MockFirebaseAuth(this.currentUserValue);

  final User? currentUserValue;

  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();

  @override
  User? get currentUser => currentUserValue;
}

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._initialState, {required this.syncResult});

  final AuthState _initialState;
  final bool syncResult;
  int refreshVerifiedEmailAccountCalls = 0;

  @override
  AuthState build() => _initialState;

  @override
  Future<bool> refreshVerifiedEmailAccount() async {
    refreshVerifiedEmailAccountCalls++;
    return syncResult;
  }
}

AuthUser _testUser({required bool isEmailVerified}) {
  return AuthUser(
    id: 'user-1',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'verified@test.com',
    username: 'verifieduser',
    isEmailVerified: isEmailVerified,
    accountStatus: AccountStatus.active,
    hasSellerProfile: false,
    sellerSubscriptionStatus: 'none',
    hasMarketAuthority: false,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

ProviderContainer _container({
  required EmailVerificationController verificationController,
  required AuthController authController,
}) {
  return ProviderContainer(
    overrides: [
      emailVerificationControllerProvider.overrideWith(
        () => verificationController,
      ),
      authControllerProvider.overrideWith(() => authController),
    ],
  );
}

void main() {
  group('EmailVerificationController.refreshEmailVerificationStatus', () {
    test(
      'verified Firebase user syncs canonical account and becomes verified',
      () async {
        final firebaseUser = _MockFirebaseUser(emailVerifiedValue: true);
        final verificationController = EmailVerificationController(
          firebaseAuth: _MockFirebaseAuth(firebaseUser),
        );
        final authController = _FakeAuthController(
          AuthState.authenticated(
            _testUser(isEmailVerified: false),
            emailVerified: false,
          ),
          syncResult: true,
        );
        final container = _container(
          verificationController: verificationController,
          authController: authController,
        );
        addTearDown(container.dispose);

        container.read(emailVerificationControllerProvider.notifier);

        await verificationController.refreshEmailVerificationStatus();

        expect(firebaseUser.reloadCalls, 1);
        expect(authController.refreshVerifiedEmailAccountCalls, 1);
        expect(verificationController.state, isA<EmailVerificationVerified>());
        expect(authController.state, isA<AuthStateAuthenticated>());
      },
    );

    test(
      'verified Firebase user but backend sync fails exposes retryable error',
      () async {
        final firebaseUser = _MockFirebaseUser(emailVerifiedValue: true);
        final verificationController = EmailVerificationController(
          firebaseAuth: _MockFirebaseAuth(firebaseUser),
        );
        final authController = _FakeAuthController(
          AuthState.authenticated(
            _testUser(isEmailVerified: false),
            emailVerified: false,
          ),
          syncResult: false,
        );
        final container = _container(
          verificationController: verificationController,
          authController: authController,
        );
        addTearDown(container.dispose);

        container.read(emailVerificationControllerProvider.notifier);

        await verificationController.refreshEmailVerificationStatus();

        expect(firebaseUser.reloadCalls, 1);
        expect(authController.refreshVerifiedEmailAccountCalls, 1);
        expect(verificationController.state, isA<EmailVerificationError>());
        expect(authController.state, isA<AuthStateAuthenticated>());
      },
    );

    test(
      'still-unverified Firebase user stays unverified and skips auth sync',
      () async {
        final firebaseUser = _MockFirebaseUser(emailVerifiedValue: false);
        final verificationController = EmailVerificationController(
          firebaseAuth: _MockFirebaseAuth(firebaseUser),
        );
        final authController = _FakeAuthController(
          AuthState.authenticated(
            _testUser(isEmailVerified: false),
            emailVerified: false,
          ),
          syncResult: true,
        );
        final container = _container(
          verificationController: verificationController,
          authController: authController,
        );
        addTearDown(container.dispose);

        container.read(emailVerificationControllerProvider.notifier);

        await verificationController.refreshEmailVerificationStatus();

        expect(firebaseUser.reloadCalls, 1);
        expect(authController.refreshVerifiedEmailAccountCalls, 0);
        expect(
          verificationController.state,
          isA<EmailVerificationUnverified>(),
        );
      },
    );

    test('Firebase reload failure exposes error and skips auth sync', () async {
      final firebaseUser = _MockFirebaseUser(
        emailVerifiedValue: false,
        reloadThrows: true,
      );
      final verificationController = EmailVerificationController(
        firebaseAuth: _MockFirebaseAuth(firebaseUser),
      );
      final authController = _FakeAuthController(
        AuthState.authenticated(
          _testUser(isEmailVerified: false),
          emailVerified: false,
        ),
        syncResult: true,
      );
      final container = _container(
        verificationController: verificationController,
        authController: authController,
      );
      addTearDown(container.dispose);

      container.read(emailVerificationControllerProvider.notifier);

      await verificationController.refreshEmailVerificationStatus();

      expect(firebaseUser.reloadCalls, 1);
      expect(authController.refreshVerifiedEmailAccountCalls, 0);
      expect(verificationController.state, isA<EmailVerificationError>());
    });

    test(
      'a second refresh while the first is still checking does not start another verification sync',
      () async {
        final reloadCompleter = Completer<void>();
        final firebaseUser = _MockFirebaseUser(
          emailVerifiedValue: true,
          reloadCompleter: reloadCompleter,
        );
        final verificationController = EmailVerificationController(
          firebaseAuth: _MockFirebaseAuth(firebaseUser),
        );
        final authController = _FakeAuthController(
          AuthState.authenticated(
            _testUser(isEmailVerified: false),
            emailVerified: false,
          ),
          syncResult: true,
        );
        final container = _container(
          verificationController: verificationController,
          authController: authController,
        );
        addTearDown(container.dispose);

        container.read(emailVerificationControllerProvider.notifier);

        final firstRefresh = verificationController
            .refreshEmailVerificationStatus();
        expect(verificationController.state, isA<EmailVerificationChecking>());

        final secondRefresh = verificationController
            .refreshEmailVerificationStatus();
        await Future<void>.delayed(Duration.zero);
        expect(firebaseUser.reloadCalls, 1);
        expect(authController.refreshVerifiedEmailAccountCalls, 0);

        reloadCompleter.complete();
        await Future.wait([firstRefresh, secondRefresh]);

        expect(firebaseUser.reloadCalls, 1);
        expect(authController.refreshVerifiedEmailAccountCalls, 1);
        expect(verificationController.state, isA<EmailVerificationVerified>());
      },
    );
  });
}
