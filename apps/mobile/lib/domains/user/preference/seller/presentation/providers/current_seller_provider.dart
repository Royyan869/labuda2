/// Current Seller Provider
///
/// **OWNER:** Seller Domain
/// **REALIGNMENT:** Extracted from shared/providers/authenticated_account_provider.dart
///
/// These providers contain seller-specific business logic and should be owned
/// by the seller domain, not the shared authentication layer.
///
/// **SELLER AUTHORITY (S3 ALIGNMENT):**
/// - Identity axis uses sellerIdentityStatusProvider
/// - Capability axis uses sellerCapabilityStatusProvider
///
/// **BUSINESS TRUTH:**
/// - seller_profiles (hasSellerProfile) = workspace identity
/// - seller_subscriptions (hasMarketAuthority) = MARKET authority
/// - Workspace access: requires hasSellerProfile (expired sellers can view)
/// - Market features: require hasMarketAuthority (active subscription needed)
/// - Active + Grace = hasMarketAuthority
/// - Expired / no subscription = NOT hasMarketAuthority (even if role exists)
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart'
    show authenticatedUserProvider;
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_user.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';

SellerIdentityStatus _sellerIdentityStatusFromUser(AuthUser? user) {
  if (user == null) {
    return SellerIdentityStatus.unknown;
  }

  return user.hasSellerProfile == true
      ? SellerIdentityStatus.seller
      : SellerIdentityStatus.nonSeller;
}

SellerCapabilityStatus _sellerCapabilityStatusFromUser(AuthUser? user) {
  if (user == null) {
    return SellerCapabilityStatus.unknown;
  }

  return user.hasMarketAuthority == true
      ? SellerCapabilityStatus.active
      : SellerCapabilityStatus.inactive;
}

/// Provider to check the canonical seller identity axis.
///
/// Returns:
/// - `unknown` when no backend snapshot is available yet
/// - `seller` when the user has a seller profile
/// - `nonSeller` when the backend snapshot says the user has no seller profile
final sellerIdentityStatusProvider = Provider<SellerIdentityStatus>((ref) {
  return _sellerIdentityStatusFromUser(ref.watch(authenticatedUserProvider));
});

/// Provider to check the canonical seller capability axis.
///
/// Returns:
/// - `unknown` when no backend snapshot is available yet
/// - `active` when the user has market authority
/// - `inactive` when the user has a backend snapshot but no market authority
final sellerCapabilityStatusProvider = Provider<SellerCapabilityStatus>((ref) {
  return _sellerCapabilityStatusFromUser(ref.watch(authenticatedUserProvider));
});

/// Provider to check if current user has market authority.
///
/// Returns false when the user is not authenticated or the backend snapshot
/// does not grant seller market authority. This is the fail-closed boolean
/// companion to [sellerCapabilityStatusProvider].
final hasMarketAuthorityProvider = Provider<bool>((ref) {
  final user = ref.watch(authenticatedUserProvider);
  return user?.hasMarketAuthority ?? false;
});

/// Provider to get seller subscription status
///
/// **REALIGNED:** Previously in shared/providers/authenticated_account_provider.dart
/// **SOURCE:** Now canonical definition is in seller domain
///
/// Returns: 'active', 'expired', 'none', or null if not applicable
/// Use this for UI display of subscription state
final sellerSubscriptionStatusProvider = Provider<String?>((ref) {
  final user = ref.watch(authenticatedUserProvider);
  return user?.sellerSubscriptionStatus;
});

/// Provider to check if seller has an active subscription
///
/// **REALIGNED:** Previously in shared/providers/authenticated_account_provider.dart
/// **SOURCE:** Now canonical definition is in seller domain
///
/// This is the canonical check for seller feature access.
/// Returns true only when seller has active subscription.
final isSellerSubscriptionActiveProvider = Provider<bool>((ref) {
  return ref.watch(hasMarketAuthorityProvider);
});

/// Provider to check if seller subscription has expired
///
/// **REALIGNED:** Previously in shared/providers/authenticated_account_provider.dart
/// **SOURCE:** Now canonical definition is in seller domain
///
/// Returns true when seller has an expired subscription.
/// Use this to show renewal prompts or restrict features.
final isSellerSubscriptionExpiredProvider = Provider<bool>((ref) {
  final status = ref.watch(sellerSubscriptionStatusProvider);
  return status == 'expired';
});

/// Provider to check if user has a seller profile (identity)
///
/// **REALIGNED:** Previously in shared/providers/authenticated_account_provider.dart
/// **SOURCE:** Now canonical definition is in seller domain
///
/// Returns true when user has created a seller profile,
/// regardless of subscription status.
final hasSellerProfileProvider = Provider<bool>((ref) {
  final user = ref.watch(authenticatedUserProvider);
  return user?.hasSellerProfile ?? false;
});

/// Provider to get current seller ID if user is a seller
///
/// **REALIGNED:** Previously in shared/providers/authenticated_account_provider.dart
/// **SOURCE:** Now canonical definition is in seller domain
///
/// Returns the user ID if the user has a seller profile,
/// empty string otherwise. Useful for seller-specific API calls.
final currentSellerIdProvider = Provider<String>((ref) {
  final user = ref.watch(authenticatedUserProvider);
  final isSeller = ref.watch(hasSellerProfileProvider);
  if (isSeller && user != null) {
    return user.id;
  }
  return '';
});
