// RF-02 contract: seller state must separate the IDENTITY axis
// (`hasSellerProfile`) and the EXPIRY axis (`sellerSubscriptionStatus`) from the
// CAPABILITY verdict (`hasMarketAuthority`).
//
// A freshly registered seller has a seller profile but no active subscription
// interval yet ('none'). That is a legitimate, non-expired state. Deriving
// "expired" from `hasMarketAuthority == false` is the RF-02 defect that told
// fresh sellers their subscription had ended and pushed them to renewal.
//
// These tests are the anti-regression lock: only `sellerSubscriptionStatus ==
// 'expired'` may render expiry copy / a renewal CTA.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';

AuthUser _testUser({
  List<UserRole> roles = const [UserRole.user],
  bool? hasSellerProfile,
  String? sellerSubscriptionStatus,
  bool? hasMarketAuthority,
  SellerTier? sellerTier,
}) {
  return AuthUser(
    id: 'test-user-id',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'test@test.com',
    username: 'testuser',
    isEmailVerified: true,
    roles: roles,
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: sellerSubscriptionStatus,
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: sellerTier,
  );
}

void main() {
  group('SellerState.fromAuthUser — RF-02 lifecycle separation', () {
    test('fresh seller (profile + no active interval) is NOT expired', () {
      // Exactly the state of a seller who finished onboarding but whose
      // subscription payment has not settled.
      final state = SellerState.fromAuthUser(
        _testUser(
          hasSellerProfile: true,
          sellerSubscriptionStatus: 'none',
          hasMarketAuthority: false,
        ),
      );

      expect(state.type, SellerStateType.pendingActivation);
      expect(state.isSeller, isTrue, reason: 'workspace identity exists');
      expect(state.canCreateContent, isFalse, reason: 'no market authority');
      expect(state.isExpired, isFalse);
      expect(state.needsRenewal, isFalse);
    });

    test('fresh seller is never told the subscription has ended', () {
      final state = SellerState.fromAuthUser(
        _testUser(
          hasSellerProfile: true,
          sellerSubscriptionStatus: 'none',
          hasMarketAuthority: false,
        ),
      );

      expect(state.bannerMessage, isNull);
      expect(state.displayLabel, isNot('Berakhir'));
      expect(state.ctaLabel, isNot('Perpanjang Langganan'));
      expect(state.displayLabel, 'Belum Aktif');
      expect(state.ctaLabel, 'Aktifkan Langganan');
    });

    test('absent subscription status with capability is still ACTIVE', () {
      // A wire surface that omits seller_subscription_status must never
      // downgrade a seller the backend already granted market authority.
      final state = SellerState.fromAuthUser(
        _testUser(hasSellerProfile: true, hasMarketAuthority: true),
      );

      expect(state.type, SellerStateType.active);
      expect(state.canCreateContent, isTrue);
      expect(state.isExpired, isFalse);
    });

    test('active subscription is ACTIVE', () {
      final state = SellerState.fromAuthUser(
        _testUser(
          hasSellerProfile: true,
          sellerSubscriptionStatus: 'active',
          hasMarketAuthority: true,
        ),
      );

      expect(state.type, SellerStateType.active);
      expect(state.isExpired, isFalse);
    });

    test('ENDED subscription is EXPIRED and carries renewal copy', () {
      final state = SellerState.fromAuthUser(
        _testUser(
          hasSellerProfile: true,
          sellerSubscriptionStatus: 'expired',
          hasMarketAuthority: false,
        ),
      );

      expect(state.type, SellerStateType.expired);
      expect(state.isExpired, isTrue);
      expect(state.needsRenewal, isTrue);
      expect(state.bannerMessage, 'Langganan Anda telah berakhir');
      expect(state.ctaLabel, 'Perpanjang Langganan');
      expect(state.isSeller, isTrue);
    });

    test('no seller profile is NOT_SELLER regardless of other fields', () {
      final state = SellerState.fromAuthUser(
        _testUser(
          hasSellerProfile: false,
          sellerSubscriptionStatus: 'none',
          hasMarketAuthority: false,
        ),
      );

      expect(state.type, SellerStateType.notSeller);
      expect(state.isSeller, isFalse);
      expect(state.isExpired, isFalse);
    });

    test('null user is NOT_SELLER', () {
      expect(SellerState.fromAuthUser(null).type, SellerStateType.notSeller);
    });
  });
}
