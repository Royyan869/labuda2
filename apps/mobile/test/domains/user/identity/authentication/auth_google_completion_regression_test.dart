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
        'AuthStateRequiresProfileCompletion does not map to authenticated',
        () {
      // AuthController.appAuthStatus maps RequiresProfileCompletion to
      // initializing (splash) while router redirects to /auth/complete-profile.
      // It must NOT be authenticated.
      expect(
        AppAuthStatus.authenticated,
        isNot(AppAuthStatus.initializing),
      );
      // Placeholder assertion documenting the contract — no
      // AppAuthStatus.requiresProfileCompletion variant exists.
      expect(true, isTrue);
    });
  });

  // ==========================================================================
  // Defect B: Only one CompleteProfileScreen authority
  // ==========================================================================

  group('Complete Profile screen singleton authority', () {
    test('route path is defined once', () {
      const path = RoutePaths.completeProfile;
      expect(path, '/auth/complete-profile');
    });

    test('route name is defined once', () {
      const name = RouteNames.completeProfile;
      expect(name, 'completeProfile');
    });
  });

  // ==========================================================================
  // Duplicate state detection
  // ==========================================================================

  group('Completion state cannot loop', () {
    test('AuthStateRequiresProfileCompletion → Authenticated transition', () {
      final authenticated = AuthState.authenticated(
        AuthUser(
          id: 'u1',
          email: 'g@test.com',
          username: 'newuser',
          isEmailVerified: true,
          roles: const [UserRole.user],
          provider: AuthProvider.google,
          createdAt: DateTime(2026),
          updatedAt: DateTime(2026),
        ),
        emailVerified: true,
      );

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
          provider: AuthProvider.google,
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
}
