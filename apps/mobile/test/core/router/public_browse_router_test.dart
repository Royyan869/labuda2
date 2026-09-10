// Public browse + guest Home router guard tests.
//
// Verifies that unauthenticated users (guests) can reach public browse
// routes AND the canonical Home (/home) without being redirected to
// /welcome, and that private routes (orders, checkout, chat, etc.) are
// still gated.
//
// Canonical terminology: the commerce discovery surface is For Sale
// (routes /for-sale...). The legacy "listing" route vocabulary (/listing...)
// is NOT registered anywhere in the router and is NOT public — guests
// hitting it are redirected to /welcome like any unknown private location.
//
// Uses the [handleAuthRedirectForTest] seam instead of a full GoRouter stack
// to keep these tests fast and dependency-free.

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Use the concrete [AuthStateUnauthenticated] — a real sealed-class subtype
/// that is neither profile-completion nor account-restricted, so the redirect
/// logic falls through cleanly to the [AppAuthStatus] switch.
const _unauthState = AuthStateUnauthenticated();
const _unauthStatus = AppAuthStatus.unauthenticated;

String? _redirect(String location) =>
    handleAuthRedirectForTest(_unauthState, _unauthStatus, location);

String? _normalize(String location) => normalizeProfileIngressForTest(location);

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

void main() {
  group('Unauthenticated router guard — public browse routes', () {
    // ------------------------------------------------------------------
    // Public browse routes: no redirect (null)
    // ------------------------------------------------------------------
    test('1. /for-sale → no redirect (guest For Sale catalog)', () {
      expect(_redirect('/for-sale'), isNull);
    });

    test('2. /for-sale/:id → no redirect (guest For Sale detail)', () {
      expect(_redirect('/for-sale/some-for-sale-id'), isNull);
    });

    test('3. /auction/:id → no redirect (allowed for guest)', () {
      expect(_redirect('/auction/some-auction-id'), isNull);
    });

    test('4. /search → no redirect (allowed for guest)', () {
      expect(_redirect('/search'), isNull);
    });

    test('5. /search/results → no redirect (allowed for guest)', () {
      expect(_redirect('/search/results'), isNull);
    });

    test('6. /content/:id → no redirect (allowed for guest)', () {
      expect(_redirect('/content/some-content-id'), isNull);
    });

    test('7. /user/:id → no redirect (allowed for guest)', () {
      expect(_redirect('/user/some-user-id'), isNull);
    });

    // ------------------------------------------------------------------
    // Guest Home: canonical Home destination must be reachable by guests.
    // Home intent = /home = canonical Home. It is NOT a For Sale route and
    // must never resolve to /for-sale (or any legacy listing destination).
    // ------------------------------------------------------------------
    test('8. /home → no redirect (GUEST HOME — canonical Home reachable)', () {
      expect(_redirect('/home'), isNull);
    });

    test('9. /welcome → no redirect (landing page)', () {
      expect(_redirect('/welcome'), isNull);
    });

    test('10. /auth/sign-in → no redirect (auth flow)', () {
      expect(_redirect('/auth/sign-in'), isNull);
    });

    test('11. /auth/sign-up → no redirect (auth flow)', () {
      expect(_redirect('/auth/sign-up'), isNull);
    });

    // ------------------------------------------------------------------
    // Private routes: must redirect to /welcome
    // ------------------------------------------------------------------
    test('12. /orders → redirect to /welcome', () {
      expect(_redirect('/orders'), equals('/welcome'));
    });

    test('13. /orders/:id → redirect to /welcome', () {
      expect(_redirect('/orders/some-order-id'), equals('/welcome'));
    });

    test('14. /checkout/... → redirect to /welcome', () {
      expect(_redirect('/checkout/some-id'), equals('/welcome'));
    });

    test('15. /seller → redirect to /welcome', () {
      expect(_redirect('/seller'), equals('/welcome'));
    });

    test('16. /chat → redirect to /welcome', () {
      expect(_redirect('/chat'), equals('/welcome'));
    });

    test('17. /notifications → redirect to /welcome', () {
      expect(_redirect('/notifications'), equals('/welcome'));
    });

    test('18. /saved-items → redirect to /welcome', () {
      expect(_redirect('/saved-items'), equals('/welcome'));
    });

    test('19. /profile → redirect to /welcome', () {
      expect(_redirect('/profile'), equals('/welcome'));
    });

    test('profile ingress /profile/<id> normalizes to /user/<id>', () {
      expect(_normalize('/profile/alice-123'), equals('/user/alice-123'));
      expect(_redirect('/profile/alice-123'), equals('/user/alice-123'));
      expect(_redirect('/user/alice-123'), isNull);
    });

    test('reserved /profile/* routes are not treated as user IDs', () {
      expect(_normalize('/profile/edit'), isNull);
      expect(_normalize('/profile/personal-info'), isNull);
      expect(_normalize('/profile/addresses'), isNull);
      expect(_redirect('/profile/edit'), equals('/welcome'));
      expect(_redirect('/profile/personal-info'), equals('/welcome'));
      expect(_redirect('/profile/addresses'), equals('/welcome'));
    });

    // ------------------------------------------------------------------
    // Edge cases: exact-prefix collision guard
    // ------------------------------------------------------------------
    test('20. legacy /listing vocabulary is NOT public browse → /welcome', () {
      // Canonical commerce browse vocabulary is /for-sale. The legacy
      // "listing" route/prefix was removed; it must not be treated as a
      // guest destination.
      expect(_redirect('/listing'), equals('/welcome'));
      expect(_redirect('/listing/some-listing-id'), equals('/welcome'));
    });

    test('21. /userinfo (non-matching prefix) → redirect to /welcome', () {
      // "/userinfo".startsWith("/user/") is false; "/userinfo" != "/user"
      // So it should be gated.
      expect(_redirect('/userinfo'), equals('/welcome'));
    });

    test('22. /home-something does not match /home → redirect to /welcome', () {
      // The guest-home exemption is exact (/home or /home/*); lookalike
      // paths must not leak through.
      expect(_redirect('/homepage'), equals('/welcome'));
    });
  });

  group('Authenticated redirect still works', () {
    test('authenticated user on /welcome → redirects to /home', () {
      final result = handleAuthRedirectForTest(
        _unauthState,
        AppAuthStatus.authenticated,
        '/welcome',
      );
      expect(result, equals('/home'));
    });

    test('authenticated user on /home → no redirect', () {
      final result = handleAuthRedirectForTest(
        _unauthState,
        AppAuthStatus.authenticated,
        '/home',
      );
      expect(result, isNull);
    });
  });

  group('Initializing status', () {
    test('initializing on non-splash → redirects to /splash', () {
      final result = handleAuthRedirectForTest(
        _unauthState,
        AppAuthStatus.initializing,
        '/home',
      );
      expect(result, equals('/splash'));
    });

    test('initializing on /splash → no redirect', () {
      final result = handleAuthRedirectForTest(
        _unauthState,
        AppAuthStatus.initializing,
        '/splash',
      );
      expect(result, isNull);
    });
  });
}
