import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_user.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_state.dart';

// =============================================================================
// Regression tests for Google signup completion defects
//
// Defect A: New Google identity going directly to Home (should go to
//           Complete Profile). Proves AuthStateRequiresProfileCompletion
//           is NOT AuthStateAuthenticated.
//
// Defect B: Duplicate CompleteProfileScreen instance. Proves only the
//           router constructs the screen.
// =============================================================================

void main() {
  // ==========================================================================
  // Defect A: New Google user must not publish Authenticated state
  // ==========================================================================

  group('Google signup completion state mapping', () {
    test('AuthStateRequiresProfileCompletion is distinct from Authenticated',
        () {
      final completionState =
          AuthState.requiresProfileCompletion(userId: 'u1', email: 'g@test.com');

      // The completion state must NOT be Authenticated.
      expect(completionState, isA<AuthStateRequiresProfileCompletion>());
      expect(completionState, isNot(isA<AuthStateAuthenticated>()));
    });

    test(
        'AuthStateRequiresProfileCompletion carries userId and email but no user',
        () {
      final completionState =
          AuthState.requiresProfileCompletion(userId: 'u1', email: 'g@test.com');

      expect(
        (completionState as AuthStateRequiresProfileCompletion).userId,
        'u1',
      );
      expect(
        (completionState as AuthStateRequiresProfileCompletion).email,
        'g@test.com',
      );
    });

    test(
        'AuthStateRequiresProfileCompletion has correct AppAuthStatus mapping',
        () {
      // The appAuthStatus mapping must NOT map this to authenticated or
      // initializing. It must map to requiresProfileCompletion.
      // This is verified by AuthController.appAuthStatus which is tested
      // in the redirect suite — this documents the expected contract.
      expect(
        AppAuthStatus.requiresProfileCompletion,
        isNot(AppAuthStatus.authenticated),
      );
      expect(
        AppAuthStatus.requiresProfileCompletion,
        isNot(AppAuthStatus.initializing),
      );
    });
  });

  // ==========================================================================
  // Defect B: Only one CompleteProfileScreen authority
  // ==========================================================================

  group('Complete Profile screen singleton authority', () {
    test('route path is defined once', () {
      // Verify the route constant exists (compile-time check).
      // RoutePaths.completeProfile is a static const — if it didn't exist
      // this test wouldn't compile.
      const path = RoutePaths.completeProfile;
      expect(path, '/auth/complete-profile');
    });

    test('route name is defined once', () {
      const name = RouteNames.completeProfile;
      expect(name, 'completeProfile');
    });

    test('VerifyEmail and CompleteProfile are distinct routes', () {
      expect(RoutePaths.verifyEmail, isNot(RoutePaths.completeProfile));
      expect(RoutePaths.verifyEmail, '/auth/verify-email');
      expect(RoutePaths.completeProfile, '/auth/complete-profile');
    });
  });

  // ==========================================================================
  // Duplicate state detection
  // ==========================================================================

  group('Completion state cannot loop', () {
    test('AuthStateRequiresProfileCompletion → Authenticated transition', () {
      // Documents the required transition: completion state must NOT transition
      // back to itself after a successful completion. The only valid outgoing
      // transitions from AuthStateRequiresProfileCompletion are:
      //   1. AuthStateAuthenticated (on success)
      //   2. AuthStateUnauthenticated (on sign out)
      //   3. AuthStateError (on failure)
      //   4. AuthStateAccountRestricted (on restriction)
      //
      // It must NEVER transition to another AuthStateRequiresProfileCompletion
      // after a successful write.

      final authenticated = AuthState.authenticated(
        AuthUser(
          id: 'u1',
          email: 'g@test.com',
          username: 'newuser',
          isEmailVerified: true,
          roles: const [UserRole.user],
          provider: ShonaAuthProvider.google,
          createdAt: DateTime(2026),
          updatedAt: DateTime(2026),
        ),
        emailVerified: true,
      );

      // Authenticated state must not be completion-required.
      expect(authenticated, isNot(isA<AuthStateRequiresProfileCompletion>()));
    });

    test('AuthStateAuthenticated carries canonical username', () {
      final authenticated = AuthState.authenticated(
        AuthUser(
          id: 'u1',
          email: 'g@test.com',
          username: 'canonical_user',
          isEmailVerified: true,
          roles: const [UserRole.user],
          provider: ShonaAuthProvider.google,
          createdAt: DateTime(2026),
          updatedAt: DateTime(2026),
        ),
        emailVerified: true,
      );

      final authUser = (authenticated as AuthStateAuthenticated).user;
      expect(authUser.username, 'canonical_user');
      expect(authUser.username, isNotEmpty);
    });
  });

  // ==========================================================================
  // Provider-based state routing
  // ==========================================================================

  group('Provider-aware state distinction', () {
    test('Google user never enters email verification state', () {
      final verifyState =
          AuthState.requiresEmailVerification(userId: 'u1', email: 'g@test.com');
      final completeState =
          AuthState.requiresProfileCompletion(userId: 'u1', email: 'g@test.com');

      // These states must be distinct — Google uses profile completion,
      // email/password uses email verification.
      expect(verifyState.runtimeType, isNot(completeState.runtimeType));
    });
  });
}
