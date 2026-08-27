// PASS 2C — logout/auth-stack behavior and complete-profile back-navigation
// safety, both proven via the pure [handleAuthRedirectForTest] seam.
//
// AUDIT VERDICT (see report): no code fix was needed for either concern.
//
// C. Logout/auth stack: AuthController.signOut() never explicitly clears
// the Navigator/GoRouter stack — it only flips AuthState to
// unauthenticated. This is safe because `goRouterProvider` (app_router.dart)
// is a plain Provider that does `ref.watch(authControllerProvider)` and is
// NOT wired via GoRouter's usual `refreshListenable` pattern (see that
// file's own comment: "Tidak ada RouterRefreshNotifier. Tidak ada
// refreshListenable."). Every AuthState change therefore rebuilds
// goRouterProvider into a BRAND NEW GoRouter instance (fresh internal
// navigation history), which MaterialApp.router swaps in as a new
// RouterConfig — the old Navigator stack (with any pushed protected
// screens) is discarded along with the old GoRouter object, not merely
// "left behind" for the user to back-navigate into. Even independent of
// that mechanism, the tests below prove the redirect function itself
// unconditionally sends `unauthenticated` away from every protected route
// tested here, so there is no route a stale Navigator entry could expose.
//
// D. Complete-profile back-navigation: CompleteProfileScreen renders no
// AppBar (confirmed by reading the widget — Scaffold has no `appBar:`),
// so there is no explicit back button UI on that screen. Even if a
// hardware/gesture back event were to pop one level, AuthStateRequires
// ProfileCompletion is checked BEFORE the AppAuthStatus switch and
// unconditionally re-forces /auth/complete-profile on the next redirect
// evaluation — the tests below prove this holds from every plausible
// "back to" location and is stable/idempotent across repeated calls
// (no oscillation, i.e. no route loop).
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';

const _unauthenticated = AuthStateUnauthenticated();

void main() {
  group(
    'C. Logout: every protected route redirects to /welcome once unauthenticated',
    () {
      const protectedRoutes = [
        '/orders',
        '/orders/some-order-id',
        '/chat',
        '/seller/dashboard',
        '/profile',
        '/settings',
        '/notifications',
        '/saved-items',
        '/checkout/some-id',
      ];

      for (final route in protectedRoutes) {
        test('$route → /welcome', () {
          final result = handleAuthRedirectForTest(
            _unauthenticated,
            AppAuthStatus.unauthenticated,
            route,
          );
          expect(result, equals('/welcome'));
        });
      }

      test(
        'a route the user was authenticated-and-pushed-to remains gated even '
        'if visited again after logout (no stale-authenticated bypass)',
        () {
          // Simulates: user was on /seller/dashboard while authenticated
          // (no redirect there — see seller_route_guard_test.dart), logs
          // out, and the SAME location is evaluated again post-logout.
          final whileAuthenticated = handleAuthRedirectForTest(
            _unauthenticated,
            AppAuthStatus.authenticated,
            '/seller/dashboard',
          );
          expect(
            whileAuthenticated,
            isNull,
            reason: 'sanity: no redirect while authenticated',
          );

          final afterLogout = handleAuthRedirectForTest(
            _unauthenticated,
            AppAuthStatus.unauthenticated,
            '/seller/dashboard',
          );
          expect(afterLogout, equals('/welcome'));
        },
      );

      test('/home → /welcome (regression lock, already covered elsewhere)', () {
        final result = handleAuthRedirectForTest(
          _unauthenticated,
          AppAuthStatus.unauthenticated,
          '/home',
        );
        expect(result, equals('/welcome'));
      });
    },
  );

  group('C. Auth-route redirects still work around logout', () {
    test('authenticated user opening /auth/sign-in → /home', () {
      final result = handleAuthRedirectForTest(
        _unauthenticated,
        AppAuthStatus.authenticated,
        '/auth/sign-in',
      );
      expect(result, equals('/home'));
    });

    test('unauthenticated user opening a protected route → /welcome', () {
      final result = handleAuthRedirectForTest(
        _unauthenticated,
        AppAuthStatus.unauthenticated,
        '/orders',
      );
      expect(result, equals('/welcome'));
    });
  });

  group(
    'D. Complete-profile is force-redirected from every plausible "back to" location',
    () {
      const profileState = AuthState.requiresProfileCompletion(
        userId: 'user-1',
        email: 'new@test.com',
      );

      const backTargets = [
        '/home',
        '/welcome',
        '/auth/sign-in',
        '/auth/sign-up',
        '/splash',
        '/orders',
      ];

      for (final location in backTargets) {
        test(
          'back to $location while incomplete → forced to /auth/complete-profile',
          () {
            final result = handleAuthRedirectForTest(
              profileState,
              AppAuthStatus.initializing,
              location,
            );
            expect(result, equals('/auth/complete-profile'));
          },
        );
      }

      test(
        'repeated evaluation on /auth/complete-profile is stable (no oscillation / no route loop)',
        () {
          // A route loop would show up as alternating non-null results across
          // repeated calls with identical inputs. The redirect function is
          // pure, so calling it N times with the same arguments must always
          // return the same, single-step-settled answer.
          for (var i = 0; i < 5; i++) {
            final result = handleAuthRedirectForTest(
              profileState,
              AppAuthStatus.initializing,
              '/auth/complete-profile',
            );
            expect(
              result,
              isNull,
              reason: 'iteration $i must settle with no further redirect',
            );
          }
        },
      );
    },
  );
}
