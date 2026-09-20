// Seller route guard unit tests.
//
// Verifies the seller guard doctrine:
// 1. Non-seller routes are never blocked
// 2. /seller/upgrade is always accessible (REGISTRATION entry point for
//    non-sellers; existing sellers are gated inside the wizard itself, so the
//    router never ejects them from an in-flight registration)
// 3. TIER 1 — WORKSPACE/OBLIGATION routes require hasSellerProfile only:
//    expired sellers CAN access dashboard/orders/earnings/bank-accounts/verification
// 4. TIER 2 - MARKET ACTION routes require hasMarketAuthority (active subscription):
//    expired sellers are redirected to /seller/renewal — the payment-only
//    renewal lifecycle — and never back into the registration wizard
// 5. /seller/renewal requires hasSellerProfile and deliberately does NOT require
//    market authority, because its canonical caller is the expired seller
// 6. Users without seller profile are redirected from all seller routes
// 7. Null user is redirected from all seller routes
// 8. Role field is NOT used for gating (regression lock)
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

      test('expired seller can still open /seller/upgrade (wizard gates them)', () {
        final result = handleSellerRouteGuardForTest(
          _expiredSeller(),
          '/seller/upgrade',
        );
        expect(result, isNull);
      });
    });

    // -------------------------------------------------------------------------
    // RENEWAL — /seller/renewal is the payment-only renewal lifecycle entry
    // Requires hasSellerProfile; market authority is NOT required, because the
    // canonical caller is the expired seller renewing.
    // -------------------------------------------------------------------------
    group('/seller/renewal is the renewal entry point', () {
      test('expired seller can access /seller/renewal', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/seller/renewal'),
          isNull,
          reason: 'The expired seller is the canonical renewal caller',
        );
      });

      test('active seller can access /seller/renewal (early renewal)', () {
        expect(
          handleSellerRouteGuardForTest(_activeSeller(), '/seller/renewal'),
          isNull,
        );
      });

      test('non-seller is redirected from /seller/renewal to registration', () {
        final user = _testUser(hasSellerProfile: false);
        expect(
          handleSellerRouteGuardForTest(user, '/seller/renewal'),
          equals(RoutePaths.sellerUpgrade),
          reason: 'Renewal only exists for users who are already sellers',
        );
      });

      test('null user is redirected from /seller/renewal to registration', () {
        expect(
          handleSellerRouteGuardForTest(null, '/seller/renewal'),
          equals(RoutePaths.sellerUpgrade),
        );
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

      test('seller without market authority can access /create/for-sale', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/create/for-sale'),
          isNull,
          reason:
              'POST /for-sale creates a PRIVATE DRAFT (workspace state). Market '
              'authority is enforced transactionally at publish (draft → active), '
              'never at draft creation — so this route is not a market-action tier.',
        );
      });
    });

    // -------------------------------------------------------------------------
    // TIER 2: MARKET ACTION routes — expired sellers are blocked
    // -------------------------------------------------------------------------
    group('TIER 2 — expired seller is blocked from market action routes', () {
      test('expired seller is redirected to /seller/renewal from /seller/shipping', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/seller/shipping'),
          equals(RoutePaths.sellerRenewal),
          reason:
              'Shipping setup is market-config — requires active subscription; '
              'the fix is payment-only renewal, not registration',
        );
      });

      test('expired seller is redirected to /seller/renewal from /seller/shipping/new', () {
        expect(
          handleSellerRouteGuardForTest(
            _expiredSeller(),
            '/seller/shipping/new',
          ),
          equals(RoutePaths.sellerRenewal),
        );
      });

      test('expired seller is redirected to /seller/renewal from /seller/promotions', () {
        expect(
          handleSellerRouteGuardForTest(_expiredSeller(), '/seller/promotions'),
          equals(RoutePaths.sellerRenewal),
          reason: 'Promotions are market actions — require active subscription',
        );
      });

      test('expired seller is redirected to /seller/renewal from unlisted /seller/* route', () {
        expect(
          handleSellerRouteGuardForTest(
            _expiredSeller(),
            '/seller/some-new-market-feature',
          ),
          equals(RoutePaths.sellerRenewal),
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
                'create for-sale is gated on seller profile only (workspace draft)',
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
          RoutePaths.sellerRenewal,
          reason:
              'once hasMarketAuthority flips to false, the SAME route '
              'must now redirect to the payment-only renewal lifecycle — '
              'proving the guard is driven by fresh authority data, not a '
              'stale role flag',
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
            RoutePaths.sellerRenewal,
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

    // -------------------------------------------------------------------------
    // ANTI-RESURRECTION LOCK (SCOPE 3)
    // An existing seller is NEVER routed back into the registration wizard:
    // registration is for non-sellers, renewal is payment-only.
    // -------------------------------------------------------------------------
    group('existing sellers are never routed into registration', () {
      test('every gated market route sends an expired seller to renewal', () {
        // /create/for-sale is deliberately NOT in this list: it creates a
        // PRIVATE DRAFT (workspace state), so it must never demand market
        // authority or bounce a profile holder into the renewal lifecycle.
        for (final route in const [
          '/seller/shipping',
          '/seller/promotions',
          '/seller/shipping-setup',
          '/seller/some-new-market-feature',
        ]) {
          expect(
            handleSellerRouteGuardForTest(_expiredSeller(), route),
            equals(RoutePaths.sellerRenewal),
            reason:
                '$route must send an existing seller to payment-only renewal, '
                'never into the registration wizard',
          );
        }
      });

      test('only users without a seller profile are sent to registration', () {
        final nonSeller = _testUser(hasSellerProfile: false);
        expect(
          handleSellerRouteGuardForTest(nonSeller, '/seller/promotions'),
          equals(RoutePaths.sellerUpgrade),
        );
        expect(
          handleSellerRouteGuardForTest(null, '/seller/dashboard'),
          equals(RoutePaths.sellerUpgrade),
        );
      });
    });
  });
}
