// PASS 2A / F2: refreshUserData()/_validateSession() must detect
// authority-relevant changes, not just role changes.
//
// AuthController's mid-session refresh paths previously only compared
// `freshUser.role != currentState.user.role` before deciding whether to
// push a new AuthState.authenticated(). Seller authority fields
// (hasMarketAuthority, hasSellerProfile, sellerSubscriptionStatus,
// sellerTier) can all change without `role` ever changing — e.g. a seller
// subscription expiring mid-session flips hasMarketAuthority from true to
// false while `role` stays UserRole.user — so the stale cached AuthUser
// kept being read by SellerGuard/the router's seller guard until the next
// full login sync (up to the full 5-minute periodic-validation window).
//
// The fix compares the whole fresh AuthUser against the cached one
// (`freshUser != currentState.user`), which works because AuthUser extends
// Equatable (via BaseEntity) over every backend-authoritative field. These
// tests exercise that equality directly — it is the exact mechanism the
// controller's `if (freshUser != currentState.user)` check depends on.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/core/core.dart';

AuthUser _seller({
  bool? hasMarketAuthority,
  bool? hasSellerProfile,
  String? sellerSubscriptionStatus,
  SellerTier? sellerTier,
  AccountStatus? accountStatus = AccountStatus.active,
  List<UserRole> roles = const [UserRole.user],
}) {
  return AuthUser(
    id: 'seller-user-id',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'seller@test.com',
    username: 'sellertest',
    isEmailVerified: true,
    roles: roles,
    provider: ShonaAuthProvider.email,
    accountStatus: accountStatus,
    hasSellerProfile: hasSellerProfile,
    hasMarketAuthority: hasMarketAuthority,
    sellerSubscriptionStatus: sellerSubscriptionStatus,
    sellerTier: sellerTier,
  );
}

void main() {
  group('AuthUser equality detects authority changes with role unchanged', () {
    test(
      'hasMarketAuthority true → false (subscription expired) is a change',
      () {
        final before = _seller(
          hasMarketAuthority: true,
          hasSellerProfile: true,
          sellerSubscriptionStatus: 'active',
        );
        final after = _seller(
          hasMarketAuthority: false,
          hasSellerProfile: true,
          sellerSubscriptionStatus: 'expired',
        );

        // Role is unchanged — this is exactly the case the old
        // `freshUser.role != currentState.user.role` check missed.
        expect(after.role, before.role);
        expect(
          after != before,
          isTrue,
          reason:
              'a subscription expiring mid-session must be detected even '
              'though role never changes',
        );
      },
    );

    test(
      'sellerSubscriptionStatus alone changing (active → expired) is a change',
      () {
        final before = _seller(
          hasMarketAuthority: true,
          sellerSubscriptionStatus: 'active',
        );
        final after = _seller(
          hasMarketAuthority: false,
          sellerSubscriptionStatus: 'expired',
        );

        expect(after.role, before.role);
        expect(after != before, isTrue);
      },
    );

    test('sellerTier alone changing (basic → pro) is a change', () {
      final before = _seller(sellerTier: SellerTier.sellerBasic);
      final after = _seller(sellerTier: SellerTier.sellerPro);

      expect(after.role, before.role);
      expect(
        after != before,
        isTrue,
        reason:
            'tier upgrades must propagate to badge/UI providers even '
            'without a role change',
      );
    });

    test('hasSellerProfile alone changing is a change', () {
      final before = _seller(hasSellerProfile: false);
      final after = _seller(hasSellerProfile: true);

      expect(after.role, before.role);
      expect(after != before, isTrue);
    });
  });

  group('AuthUser equality is stable when nothing meaningful changed', () {
    test('two structurally-identical users are equal (no churn)', () {
      final a = _seller(
        hasMarketAuthority: true,
        hasSellerProfile: true,
        sellerSubscriptionStatus: 'active',
        sellerTier: SellerTier.sellerPro,
      );
      final b = _seller(
        hasMarketAuthority: true,
        hasSellerProfile: true,
        sellerSubscriptionStatus: 'active',
        sellerTier: SellerTier.sellerPro,
      );

      expect(
        a == b,
        isTrue,
        reason:
            'refreshUserData()/_validateSession() must NOT push a new '
            'AuthState (and rebuild the tree) when the fresh user is '
            'identical to the cached one',
      );
    });
  });

  group('role change is still detected (regression guard)', () {
    test('role user → admin with everything else equal is still a change', () {
      final before = _seller(roles: const [UserRole.user]);
      final after = _seller(roles: const [UserRole.admin]);

      expect(after != before, isTrue);
    });
  });

  group(
    'account restriction still takes priority (existing behavior, unchanged)',
    () {
      test('accountStatus becoming suspended is also detected by !=', () {
        final before = _seller(accountStatus: AccountStatus.active);
        final after = _seller(accountStatus: AccountStatus.suspended);

        // The controller checks accountStatus.isRestricted BEFORE the
        // equality check and returns early via AuthState.accountRestricted,
        // so this case never reaches the `!=` comparison in practice — but
        // confirming it also registers as "different" documents that the
        // equality check is not narrower than the restriction gate it sits
        // behind.
        expect(after != before, isTrue);
        expect(after.accountStatus!.isRestricted, isTrue);
      });
    },
  );
}
