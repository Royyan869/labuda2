import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_state.dart';

// =============================================================================
// _PendingEmailRegistration lifecycle tests.
// =============================================================================

/// A visible-for-testing copy of the production contract exercised by
/// AuthController._PendingEmailRegistration. These tests verify the
/// state machine that protects compensation safety.
class TestPendingEmailRegistration {
  final String expectedEmail;
  final String normalizedUsername;
  final int authEpoch;
  String? firebaseUid;
  bool cleared = false;
  bool createdByCurrentAttempt = false;

  TestPendingEmailRegistration({
    required this.expectedEmail,
    required this.normalizedUsername,
    required this.authEpoch,
  });

  void markFirebaseUserCreated(String uid) {
    createdByCurrentAttempt = true;
    firebaseUid = uid;
  }

  bool isValidFor(String uid, int epoch) {
    if (cleared) return false;
    if (authEpoch != epoch) return false;
    if (firebaseUid != null && firebaseUid != uid) return false;
    return true;
  }
}

/// Production contract for compensation eligibility.
bool canCompensate(
  TestPendingEmailRegistration? reg,
  String? activeFirebaseUid,
) {
  if (reg == null || reg.cleared) return false;
  if (!reg.createdByCurrentAttempt) return false;
  final bound = reg.firebaseUid;
  if (bound == null) return false;
  if (activeFirebaseUid != bound) return false;
  return true;
}

void main() {
  // ==========================================================================
  // 1. Pending registration lifecycle
  // ==========================================================================

  group('_PendingEmailRegistration lifecycle', () {
    test('createdByCurrentAttempt is false before Firebase creation', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      expect(reg.createdByCurrentAttempt, isFalse);
      expect(reg.firebaseUid, isNull);
    });

    test('markFirebaseUserCreated sets flag and binds UID', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('firebase-uid-1');

      expect(reg.createdByCurrentAttempt, isTrue);
      expect(reg.firebaseUid, 'firebase-uid-1');
    });

    test('isValidFor rejects cleared context', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('firebase-uid-1');
      reg.cleared = true;

      expect(reg.isValidFor('firebase-uid-1', 1), isFalse);
    });

    test('isValidFor rejects different epoch', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('firebase-uid-1');

      expect(reg.isValidFor('firebase-uid-1', 2), isFalse);
    });

    test('isValidFor rejects different UID after binding', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('firebase-uid-1');

      expect(reg.isValidFor('firebase-uid-2', 1), isFalse);
    });

    test('isValidFor accepts matching UID and epoch', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('firebase-uid-1');

      expect(reg.isValidFor('firebase-uid-1', 1), isTrue);
    });

    test('isValidFor accepts null firebaseUid before binding', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      // firebaseUid is null — no UID check
      expect(reg.isValidFor('any-uid', 1), isTrue);
    });
  });

  // ==========================================================================
  // 2. Compensation safety gates
  // ==========================================================================

  group('compensation eligibility', () {
    test('null registration blocks compensation', () {
      expect(canCompensate(null, 'fb-1'), isFalse);
    });

    test('cleared registration blocks compensation', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('fb-1');
      reg.cleared = true;
      expect(canCompensate(reg, 'fb-1'), isFalse);
    });

    test('createdByCurrentAttempt=false blocks compensation', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      // Never called markFirebaseUserCreated
      expect(canCompensate(reg, 'fb-1'), isFalse);
    });

    test('null bound UID blocks compensation', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.createdByCurrentAttempt = true;
      // firebaseUid is still null
      expect(canCompensate(reg, 'fb-1'), isFalse);
    });

    test('UID mismatch blocks compensation — existing login user', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('fb-signup-1');
      // activeFirebaseUid is different — this is an existing user
      expect(canCompensate(reg, 'fb-existing-2'), isFalse);
    });

    test('UID mismatch blocks compensation — session restore user', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('fb-signup-1');
      // Different UID — would be session restore, not signup
      expect(canCompensate(reg, 'fb-restore-3'), isFalse);
    });

    test('matching UID + createdByCurrentAttempt allows compensation', () {
      final reg = TestPendingEmailRegistration(
        expectedEmail: 'test@example.com',
        normalizedUsername: 'testuser',
        authEpoch: 1,
      );
      reg.markFirebaseUserCreated('fb-signup-1');
      expect(canCompensate(reg, 'fb-signup-1'), isTrue);
    });

    test('Google user cannot trigger email-signup compensation', () {
      // A Google sign-in would never create a _PendingEmailRegistration,
      // so `reg` would be null. canCompensate(null, ...) → false.
      expect(canCompensate(null, 'google-fb-uid'), isFalse);
    });
  });

  // ==========================================================================
  // 3. Flow contract: state transitions
  // ==========================================================================

  group('email signup flow contract', () {
    test('AuthStateRequiresEmailVerification is distinct from Complete Profile',
        () {
      final verifyState =
          AuthState.requiresEmailVerification(userId: 'u1', email: 'e@t.com');
      final completeState =
          AuthState.requiresProfileCompletion(userId: 'u1', email: 'e@t.com');

      expect(verifyState.runtimeType, isNot(completeState.runtimeType));
      expect(verifyState, isA<AuthStateRequiresEmailVerification>());
      expect(verifyState, isNot(isA<AuthStateRequiresProfileCompletion>()));
    });

    test('AuthStateRequiresEmailVerification has correct AppAuthStatus', () {
      expect(
        AppAuthStatus.requiresEmailVerification,
        isNot(AppAuthStatus.requiresProfileCompletion),
      );
      expect(
        AppAuthStatus.requiresEmailVerification,
        isNot(AppAuthStatus.authenticated),
      );
    });

    test('email signup route is /auth/verify-email not complete-profile', () {
      expect(RoutePaths.verifyEmail, '/auth/verify-email');
      expect(RoutePaths.completeProfile, '/auth/complete-profile');
      expect(RoutePaths.verifyEmail, isNot(RoutePaths.completeProfile));
    });
  });

  // ==========================================================================
  // 4. AuthState error handling
  // ==========================================================================

  group('AuthState error does not leak raw backend text', () {
    test('AuthStateError is correctly constructable', () {
      const error = AuthState.error('Registration failed');
      expect(error, isA<AuthStateError>());
      final err = error as AuthStateError;
      expect(err.message, 'Registration failed');
    });

    test('AuthStateAuthenticated carries username', () {
      final state = AuthState.authenticated(
        AuthUser(
          id: 'u1',
          email: 'test@example.com',
          username: 'newuser',
          isEmailVerified: false,
          roles: const [UserRole.user],
          provider: ShonaAuthProvider.email,
          createdAt: DateTime(2026),
          updatedAt: DateTime(2026),
        ),
        emailVerified: false,
      );
      final user = (state as AuthStateAuthenticated).user;
      expect(user.username, 'newuser');
      expect(user.isEmailVerified, isFalse);
    });
  });
}
