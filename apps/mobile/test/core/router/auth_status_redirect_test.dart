// PASS 2B: router redirect coverage for degraded/restricted/profile-
// completion states, plus the regression lock proving the previously
// silent /splash -> /welcome bounce on backend-unavailable/backend-failure
// no longer happens.
//
// Uses the [handleAuthRedirectForTest] pure seam — as of PASS 2B this seam
// calls the SAME function the live GoRouter `redirect:` callback delegates
// to (see app_router.dart's _handleAuthenticationRedirect), so these tests
// describe real router behavior, not just the test copy.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/core/core.dart';

AuthUser _testUser({AccountStatus? status}) {
  return AuthUser(
    id: 'test-user-id',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'test@test.com',
    username: 'testuser',
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    accountStatus: status,
  );
}

void main() {
  group('AuthStateBackendUnavailable / AuthStateBackendFailure (degraded)', () {
    const unavailable = AuthState.backendUnavailable('Backend down');
    const failure = AuthState.backendFailure('Validation error');

    test('backendUnavailable on /splash does NOT bounce to /welcome '
        '(regression lock for the PASS 2B fix)', () {
      final result = handleAuthRedirectForTest(
        unavailable,
        AppAuthStatus.degraded,
        '/splash',
      );
      expect(
        result,
        isNull,
        reason:
            'the router must stay on /splash so SplashScreen can render '
            'the dedicated backend-unavailable UI, not silently swap in '
            'an unexplained /welcome screen',
      );
    });

    test('backendFailure on /splash also stays (no redirect)', () {
      final result = handleAuthRedirectForTest(
        failure,
        AppAuthStatus.degraded,
        '/splash',
      );
      expect(result, isNull);
    });

    test(
      'degraded on a content route (/home) stays — offline browsing preserved',
      () {
        final result = handleAuthRedirectForTest(
          unavailable,
          AppAuthStatus.degraded,
          '/home',
        );
        expect(result, isNull);
      },
    );

    test('degraded on /orders stays', () {
      final result = handleAuthRedirectForTest(
        failure,
        AppAuthStatus.degraded,
        '/orders',
      );
      expect(result, isNull);
    });
  });

  group('AuthStateRequiresProfileCompletion', () {
    const profileState = AuthState.requiresProfileCompletion(
      userId: 'user-1',
      email: 'new@test.com',
    );

    test('from /splash redirects to /auth/complete-profile', () {
      final result = handleAuthRedirectForTest(
        profileState,
        AppAuthStatus.initializing,
        '/splash',
      );
      expect(result, equals('/auth/complete-profile'));
    });

    test('from /home redirects to /auth/complete-profile', () {
      final result = handleAuthRedirectForTest(
        profileState,
        AppAuthStatus.initializing,
        '/home',
      );
      expect(result, equals('/auth/complete-profile'));
    });

    test('already on /auth/complete-profile → no redirect', () {
      final result = handleAuthRedirectForTest(
        profileState,
        AppAuthStatus.initializing,
        '/auth/complete-profile',
      );
      expect(result, isNull);
    });

    test('takes priority over AppAuthStatus even if status says degraded', () {
      // Defensive: profile-completion is checked before the AppAuthStatus
      // switch in _authRedirectForLocation, so it must win regardless of
      // what appAuthStatus would otherwise compute.
      final result = handleAuthRedirectForTest(
        profileState,
        AppAuthStatus.degraded,
        '/splash',
      );
      expect(result, equals('/auth/complete-profile'));
    });
  });

  group('AuthStateAccountRestricted', () {
    test('redirects to /account-restricted from any route', () {
      final user = _testUser(status: AccountStatus.suspended);
      final state = AuthState.accountRestricted(
        user,
        restrictionType: AccountStatus.suspended,
      );

      expect(
        handleAuthRedirectForTest(
          state,
          AppAuthStatus.accountRestricted,
          '/home',
        ),
        equals('/account-restricted'),
      );
      expect(
        handleAuthRedirectForTest(
          state,
          AppAuthStatus.accountRestricted,
          '/splash',
        ),
        equals('/account-restricted'),
      );
    });

    test('already on /account-restricted → no redirect', () {
      final user = _testUser(status: AccountStatus.banned);
      final state = AuthState.accountRestricted(
        user,
        restrictionType: AccountStatus.banned,
      );

      expect(
        handleAuthRedirectForTest(
          state,
          AppAuthStatus.accountRestricted,
          '/account-restricted',
        ),
        isNull,
      );
    });
  });

  group('Authenticated user opening an auth route', () {
    const authedPlaceholder = AuthStateUnauthenticated();

    test('/auth/sign-in → redirects to /home', () {
      final result = handleAuthRedirectForTest(
        authedPlaceholder,
        AppAuthStatus.authenticated,
        '/auth/sign-in',
      );
      expect(result, equals('/home'));
    });

    test('/auth/sign-up → redirects to /home', () {
      final result = handleAuthRedirectForTest(
        authedPlaceholder,
        AppAuthStatus.authenticated,
        '/auth/sign-up',
      );
      expect(result, equals('/home'));
    });

    test('/auth/complete-profile is exempt (no redirect)', () {
      final result = handleAuthRedirectForTest(
        authedPlaceholder,
        AppAuthStatus.authenticated,
        '/auth/complete-profile',
      );
      expect(result, isNull);
    });
  });

  group('Unauthenticated user opening a protected route', () {
    const unauthPlaceholder = AuthStateUnauthenticated();

    test('/orders → redirects to /welcome', () {
      final result = handleAuthRedirectForTest(
        unauthPlaceholder,
        AppAuthStatus.unauthenticated,
        '/orders',
      );
      expect(result, equals('/welcome'));
    });

    test('/home → redirects to /welcome', () {
      final result = handleAuthRedirectForTest(
        unauthPlaceholder,
        AppAuthStatus.unauthenticated,
        '/home',
      );
      expect(result, equals('/welcome'));
    });
  });

  group('No-stuck-splash guard (state-level)', () {
    const unauthPlaceholder = AuthStateUnauthenticated();

    test(
      'initializing on /splash never redirects away (expected transient state)',
      () {
        expect(
          handleAuthRedirectForTest(
            unauthPlaceholder,
            AppAuthStatus.initializing,
            '/splash',
          ),
          isNull,
        );
      },
    );

    test('every AppAuthStatus value resolves to a route decision from /splash '
        '(no case silently falls through to an unhandled state)', () {
      // This is a coverage guard: if a new AppAuthStatus value is ever
      // added without updating the switch in _authRedirectForLocation,
      // Dart's exhaustiveness check on the sealed enum switch will fail
      // to compile rather than silently returning null forever. Asserting
      // each existing value returns *something well-defined* documents
      // the current, complete mapping.
      for (final status in AppAuthStatus.values) {
        // Every status must resolve without throwing — if a new
        // AppAuthStatus value were ever added without a matching switch
        // case, the exhaustive switch in _authRedirectForLocation would
        // fail to compile, not silently hang here at runtime.
        expect(
          () => handleAuthRedirectForTest(unauthPlaceholder, status, '/splash'),
          returnsNormally,
          reason: 'AppAuthStatus.$status must resolve to a defined redirect',
        );
      }
    });
  });
}
