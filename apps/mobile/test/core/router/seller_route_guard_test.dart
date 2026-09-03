// Seller route guard unit tests.
//
// Verifies TWO-TIER seller guard doctrine:
// 1. Non-seller routes are never blocked
// 2. /seller/upgrade is always accessible (onboarding / renewal entry point)
// 3. TIER 1 — WORKSPACE/OBLIGATION routes require hasSellerProfile only:
//    expired sellers CAN access dashboard/orders/earnings/bank-accounts/verification
// 4. TIER 2 - MARKET ACTION routes require hasMarketAuthority (active subscription):
//    expired sellers are redirected from shipping-setup, promotions, unlisted routes
// 5. Users without seller profile are redirected from all seller routes
// 6. Null user is redirected from all seller routes
// 7. Role field is NOT used for gating (regression lock)
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';

/// Minimal AuthUser for testing — only seller state fields matter.
AuthUser _testUser({
  List<UserRole> roles = const [UserRole.user],
  bool? hasSellerProfile,
  String? sellerSubscriptionStatus,
  bool? hasMarketAuthority,
  bool isEmailVerified = true,
  bool? isIdVerified,
  bool? isFarmVerified,
  SellerTier? sellerTier,
}) {
  return AuthUser(
    id: 'test-user-id',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'test@test.com',
    username: 'testuser',
    isEmailVerified: isEmailVerified,
    isIdVerified: isIdVerified,
    isFarmVerified: isFarmVerified,
    roles: roles,
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: sellerSubscriptionStatus,
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: sellerTier,
  );
}

AuthUser _expiredSeller() => _testUser(
  hasSellerProfile: true,
  sellerSubscriptionStatus: 'expired',
  hasMarketAuthority: false,
);

AuthUser _activeSeller() => _testUser(
  hasSellerProfile: true,
  sellerSubscriptionStatus: 'active',
  hasMarketAuthority: true,
);

void main() {
  group('Seller route guard', () {
    // -------------------------------------------------------------------------
    // Non-seller routes are never blocked
    // -------------------------------------------------------------------------
    group('non-seller routes are never blocked', () {
      test('home route passes through', () {
        final result = handleSellerRouteGuardForTest(null, '/home');
        expect(result, isNull);
      });

      test('for-sale route passes through', () {
        final result = handleSellerRouteGuardForTest(null, '/for-sale/123');
        expect(result, isNull);
      });

      test('create for-sale route is gated', () {
        final result = handleSellerRouteGuardForTest(null, '/create/for-sale');
        expect(result, RoutePaths.sellerUpgrade);
      });
    });

    // -------------------------------------------------------------------------
    // TIER 0: /seller/upgrade — always accessible
    // -------------------------------------------------------------------------
    group('/seller/upgrade is always accessible', () {
      test('non-seller can access /seller/upgrade', () {
        final user = _testUser(hasSellerProfile: false);
        final result = handleSellerRouteGuardForTest(user, '/seller/upgrade');
        expect(result, isNull);
      });

      test('null user can access /seller/upgrade', () {
        final result = handleSellerRouteGuardForTest(null, '/seller/upgrade');
        expect(result, isNull);
      });

      test('user with role=user can access /seller/upgrade', () {
        final user = _testUser(roles: [UserRole.user]);
        final result = handleSellerRouteGuardForTest(user, '/seller/upgrade');
        expect(result, isNull);
      });

      test('expired seller can access /seller/upgrade (renewal path)', () {
        final result = handleSellerRouteGuardForTest(
          _expiredSeller(),
          '/seller/upgrade',
        );
        expect(result, isNull);
      });
    });

    // -------------------------------------------------------------------------
    // TIER 1: WORKSPACE / OBLIGATION routes — require hasSellerProfile only
    // P1 FIX: expired sellers MUST be able to access these routes
    // -------------------------------------------------------------------------
    group('TIER 1 — expired seller can access workspace/obligation routes', () {
      test('expired seller can access /seller/orders', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/seller/orders'),
          isNull,
          reason:
              'Expired seller has active order obligations — must be able to view',
        );
      });

      test('expired seller can access /seller/orders/:id sub-path', () {
        expect(
          handleSellerRouteGuardForTest(
            _expiredSeller(),
            '/seller/orders/some-order-uuid',
          ),
          isNull,
          reason: 'Expired seller must be able to view specific order detail',
        );
      });

      test('expired seller can access /seller/earnings', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/seller/earnings'),
          isNull,
          reason: 'Earned balance visibility survives subscription expiry',
        );
      });

      test('expired seller can access /seller/dashboard', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/seller/dashboard'),
          isNull,
          reason:
              'Dashboard workspace access requires only seller profile existence',
        );
      });

      test('expired seller can access /seller/bank-accounts', () {
        expect(
          handleSellerRouteGuardForTest(
            _expiredSeller(),
            '/seller/bank-accounts',
          ),
          isNull,
          reason:
              'Bank account setup needed for payout — survives expiry per PAYOUT_AUTHORITY_DOCTRINE',
        );
      });

      test('expired seller can access /verification/seller', () {
        expect(
          handleSellerRouteGuardForTest(
            _expiredSeller(),
            '/verification/seller',
          ),
          isNull,
          reason:
              'Verification status/submission must be accessible to expired sellers',
        );
      });

      test('expired seller can access /verification', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/verification'),
          isNull,
          reason:
              'Verification entry route must be accessible to expired sellers',
        );
      });
    });

    // -------------------------------------------------------------------------
    // TIER 2: MARKET ACTION routes — expired sellers are blocked
    // -------------------------------------------------------------------------
    group('TIER 2 — expired seller is blocked from market action routes', () {
      test('expired seller is redirected from /seller/shipping', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/seller/shipping'),
          equals('/seller/upgrade'),
          reason:
              'Shipping setup is market-config — requires active subscription',
        );
      });

      test('expired seller is redirected from /seller/shipping/new', () {
        expect(
          handleSellerRouteGuardForTest(
            _expiredSeller(),
            '/seller/shipping/new',
          ),
          equals('/seller/upgrade'),
        );
      });

      test('expired seller is redirected from /seller/promotions', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/seller/promotions'),
          equals('/seller/upgrade'),
          reason: 'Promotions are market actions — require active subscription',
        );
      });

      test('expired seller is redirected from unlisted /seller/* route', () {
        expect(
          handleSellerRouteGuardForTest(
            _expiredSeller(),
            '/seller/some-new-market-feature',
          ),
          equals('/seller/upgrade'),
          reason: 'Unlisted /seller/* routes default to market-action tier',
        );
      });
    });

    // -------------------------------------------------------------------------
    // Active sellers can access all routes
    // -------------------------------------------------------------------------
    group('active seller can access all seller routes', () {
      test('active seller can access /seller/dashboard', () {
        expect(
          handleSellerRouteGuardForTest(_activeSeller(), '/seller/dashboard'),
          isNull,
        );
      });

      test('active seller can access /seller/earnings', () {
        expect(
          handleSellerRouteGuardForTest(_activeSeller(), '/seller/earnings'),
          isNull,
        );
      });

      test('active seller can access /seller/orders', () {
        expect(
          handleSellerRouteGuardForTest(_activeSeller(), '/seller/orders'),
          isNull,
        );
      });

      test('active seller can access /seller/shipping', () {
        expect(
          handleSellerRouteGuardForTest(_activeSeller(), '/seller/shipping'),
          isNull,
        );
      });

      test('active seller can access /seller/bank-accounts', () {
        expect(
          handleSellerRouteGuardForTest(
            _activeSeller(),
            '/seller/bank-accounts',
          ),
          isNull,
        );
      });

      test('active seller can access /verification/seller', () {
        expect(
          handleSellerRouteGuardForTest(
            _activeSeller(),
            '/verification/seller',
          ),
          isNull,
        );
      });

      test('active seller can access /verification', () {
        expect(
          handleSellerRouteGuardForTest(_activeSeller(), '/verification'),
          isNull,
        );
      });

      test('active seller can access /create/for-sale', () {
        expect(
          handleSellerRouteGuardForTest(_activeSeller(), '/create/for-sale'),
          isNull,
        );
      });

      test(
        'active seller with no KYC and elite tier can still access /create/for-sale',
        () {
          final user = _testUser(
            roles: [UserRole.user],
            hasSellerProfile: true,
            sellerSubscriptionStatus: 'active',
            hasMarketAuthority: true,
            isEmailVerified: false,
            isIdVerified: false,
            isFarmVerified: false,
            sellerTier: SellerTier.sellerElite,
          );
          expect(
            handleSellerRouteGuardForTest(user, '/create/for-sale'),
            isNull,
            reason:
                'create for-sale must use only seller profile + market authority',
          );
        },
      );
    });

    // -------------------------------------------------------------------------
    // Users without seller profile — blocked from ALL seller routes
    // -------------------------------------------------------------------------
    group(
      'user without seller profile is redirected from all seller routes',
      () {
        test('user without profile redirected from /seller/dashboard', () {
          final user = _testUser(hasSellerProfile: false);
          expect(
            handleSellerRouteGuardForTest(user, '/seller/dashboard'),
            equals('/seller/upgrade'),
            reason: 'hasSellerProfile required even for workspace routes',
          );
        });

        test('user without profile redirected from /seller/orders', () {
          final user = _testUser(hasSellerProfile: false);
          expect(
            handleSellerRouteGuardForTest(user, '/seller/orders'),
            equals('/seller/upgrade'),
          );
        });

        test('user without profile redirected from /seller/earnings', () {
          final user = _testUser(hasSellerProfile: false);
          expect(
            handleSellerRouteGuardForTest(user, '/seller/earnings'),
            equals('/seller/upgrade'),
          );
        });

        test('user with null profile redirected from /seller/dashboard', () {
          final user = _testUser(); // hasSellerProfile defaults to null → false
          expect(
            handleSellerRouteGuardForTest(user, '/seller/dashboard'),
            equals('/seller/upgrade'),
          );
        });

        test('user without profile redirected from /verification/seller', () {
          final user = _testUser(hasSellerProfile: false);
          expect(
            handleSellerRouteGuardForTest(user, '/verification/seller'),
            equals('/seller/upgrade'),
            reason: 'hasSellerProfile required for verification routes too',
          );
        });

        test('user without profile redirected from /verification', () {
          final user = _testUser(hasSellerProfile: false);
          expect(
            handleSellerRouteGuardForTest(user, '/verification'),
            equals('/seller/upgrade'),
          );
        });

        test('user without profile redirected from /create/for-sale', () {
          final user = _testUser(hasSellerProfile: false);
          expect(
            handleSellerRouteGuardForTest(user, '/create/for-sale'),
            equals('/seller/upgrade'),
          );
        });
      },
    );

    // -------------------------------------------------------------------------
    // Null user — redirected from all seller routes
    // -------------------------------------------------------------------------
    group('null user is redirected from all seller routes', () {
      test('null user redirected from /seller/dashboard', () {
        expect(
          handleSellerRouteGuardForTest(null, '/seller/dashboard'),
          equals('/seller/upgrade'),
        );
      });

      test('null user redirected from /seller/earnings', () {
        expect(
          handleSellerRouteGuardForTest(null, '/seller/earnings'),
          equals('/seller/upgrade'),
        );
      });

      test('null user redirected from /seller/orders', () {
        expect(
          handleSellerRouteGuardForTest(null, '/seller/orders'),
          equals('/seller/upgrade'),
        );
      });

      test('null user redirected from /verification/seller', () {
        expect(
          handleSellerRouteGuardForTest(null, '/verification/seller'),
          equals('/seller/upgrade'),
        );
      });

      test('null user redirected from /create/for-sale', () {
        expect(
          handleSellerRouteGuardForTest(null, '/create/for-sale'),
          equals('/seller/upgrade'),
        );
      });
    });

    // -------------------------------------------------------------------------
    // Role is NOT used for gating (regression lock)
    // -------------------------------------------------------------------------
    group('role is NOT used for gating (regression lock)', () {
      test('user with no seller profile is redirected', () {
        final user = _testUser(roles: [UserRole.user], hasSellerProfile: false);
        expect(
          handleSellerRouteGuardForTest(user, '/seller/dashboard'),
          equals('/seller/upgrade'),
          reason:
              'Role alone does not grant access — backend-derived fields only',
        );
      });

      test('user with user role but WITH profile+authority is allowed', () {
        final user = _testUser(
          roles: [UserRole.user],
          hasSellerProfile: true,
          sellerSubscriptionStatus: 'active',
          hasMarketAuthority: true,
        );
        expect(
          handleSellerRouteGuardForTest(user, '/seller/dashboard'),
          isNull,
        );
      });

      test(
        'user with user role, profile only (expired) can access workspace',
        () {
          final user = _testUser(
            roles: [UserRole.user],
            hasSellerProfile: true,
            sellerSubscriptionStatus: 'expired',
            hasMarketAuthority: false,
          );
          expect(
            handleSellerRouteGuardForTest(user, '/seller/orders'),
            isNull,
            reason: 'Role irrelevant — workspace access is by hasSellerProfile',
          );
        },
      );
    });

    // -------------------------------------------------------------------------
    // PASS 2A / F2 regression guard: the guard must react to a fresh
    // hasMarketAuthority value, not a role field. This directly backs the
    // AuthController fix (auth_authority_refresh_test.dart) — once
    // refreshUserData()/_validateSession() push the fresh AuthUser into
    // AuthState.authenticated, authenticatedUserProvider (and therefore this
    // guard) sees the update immediately.
    group('guard reacts to a mid-session authority flip (F2)', () {
      test('same seller, market route allowed before subscription expiry and '
          'blocked after — with role unchanged throughout', () {
        final before = _activeSeller();
        final after = _expiredSeller();

        // Role never changes between the two snapshots — this is exactly
        // the case the old `role != role` comparison in AuthController
        // would have missed.
        expect(after.role, before.role);

        expect(
          handleSellerRouteGuardForTest(before, '/seller/promotions'),
          isNull,
          reason: 'active subscription grants market-action access',
        );
        expect(
          handleSellerRouteGuardForTest(after, '/seller/promotions'),
          '/seller/upgrade',
          reason:
              'once hasMarketAuthority flips to false, the SAME route '
              'must now redirect to /seller/upgrade — proving the guard '
              'is driven by fresh authority data, not a stale role flag',
        );
      });

      test(
        'expired seller loses market access while workspace access stays intact',
        () {
          final active = _activeSeller();
          final expired = _expiredSeller();

          expect(
            handleSellerRouteGuardForTest(active, '/seller/shipping-setup'),
            isNull,
          );
          expect(
            handleSellerRouteGuardForTest(expired, '/seller/shipping-setup'),
            '/seller/upgrade',
          );
        },
      );

      test('workspace routes remain accessible across the same authority flip '
          '(Tier 1 is keyed on hasSellerProfile, not hasMarketAuthority)', () {
        final before = _activeSeller();
        final after = _expiredSeller();

        expect(
          handleSellerRouteGuardForTest(before, '/seller/dashboard'),
          isNull,
        );
        expect(
          handleSellerRouteGuardForTest(after, '/seller/dashboard'),
          isNull,
          reason:
              'workspace access must survive subscription expiry — only '
              'market-action routes should react to the authority flip',
        );
      });
    });
  });
}
