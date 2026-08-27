// Public browse router guard tests.
//
// Verifies that unauthenticated users can reach public browse routes without
// being redirected to /welcome, and that private routes (orders, checkout,
// home, etc.) are still gated.
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
    test('11. /listing/:id → no redirect (allowed for guest)', () {
      expect(_redirect('/listing/some-listing-id'), isNull);
    });

    test('12. /auction/:id → no redirect (allowed for guest)', () {
      expect(_redirect('/auction/some-auction-id'), isNull);
    });

    test('13. /search → no redirect (allowed for guest)', () {
      expect(_redirect('/search'), isNull);
    });

    test('14. /search/results → no redirect (allowed for guest)', () {
      expect(_redirect('/search/results'), isNull);
    });

    test('15. /content/:id → no redirect (allowed for guest)', () {
      expect(_redirect('/content/some-content-id'), isNull);
    });

    test('16. /user/:id → no redirect (allowed for guest)', () {
      expect(_redirect('/user/some-user-id'), isNull);
    });

    test('17. /welcome → no redirect (landing page)', () {
      expect(_redirect('/welcome'), isNull);
    });

    test('18. /auth/sign-in → no redirect (auth flow)', () {
      expect(_redirect('/auth/sign-in'), isNull);
    });

    test('19. /auth/sign-up → no redirect (auth flow)', () {
      expect(_redirect('/auth/sign-up'), isNull);
    });

    // ------------------------------------------------------------------
    // Private routes: must redirect to /welcome
    // ------------------------------------------------------------------
    test('20. /orders → redirect to /welcome', () {
      expect(_redirect('/orders'), equals('/welcome'));
    });

    test('21. /orders/:id → redirect to /welcome', () {
      expect(_redirect('/orders/some-order-id'), equals('/welcome'));
    });

    test('22. /checkout/... → redirect to /welcome', () {
      expect(_redirect('/checkout/some-id'), equals('/welcome'));
    });

    test('23. /home → redirect to /welcome', () {
      expect(_redirect('/home'), equals('/welcome'));
    });

    test('24. /seller → redirect to /welcome', () {
      expect(_redirect('/seller'), equals('/welcome'));
    });

    test('25. /chat → redirect to /welcome', () {
      expect(_redirect('/chat'), equals('/welcome'));
    });

    test('26. /notifications → redirect to /welcome', () {
      expect(_redirect('/notifications'), equals('/welcome'));
    });

    test('27. /saved-items → redirect to /welcome', () {
      expect(_redirect('/saved-items'), equals('/welcome'));
    });

    test('28. /profile → redirect to /welcome', () {
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
    // Edge cases: prefix collision guard
    // ------------------------------------------------------------------
    test('29. /listing (bare, no id) → no redirect', () {
      // /listing == /listing → publicBrowsePrefixes contains '/listing'
      expect(_redirect('/listing'), isNull);
    });

    test('30. /userinfo (non-matching prefix) → redirect to /welcome', () {
      // "/userinfo".startsWith("/user/") is false; "/userinfo" != "/user"
      // So it should be gated.
      expect(_redirect('/userinfo'), equals('/welcome'));
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
