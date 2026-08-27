/// Seller State Entity
///
/// **OWNER:** Seller Domain
/// **REALIGNMENT:** State-based UI for honest seller capability display
///
/// **BUSINESS TRUTH:**
/// - Seller state is derived from backend response (AuthUser)
/// - No client-side guessing or derivation
/// - UI must reflect honest backend state
///
/// **3 STATES:**
/// 1. NOT_SELLER - No seller profile
/// 2. ACTIVE - Has seller profile + active subscription
/// 3. EXPIRED - Has seller profile + expired subscription
///
/// **UI MAPPING:**
/// - NOT_SELLER: "Mulai Jualan" CTA
/// - ACTIVE: "Buat Listing", "Buat Lelang" enabled
/// - EXPIRED: "Buat Listing", "Buat Lelang" disabled + "Perpanjang Langganan" CTA
library;

import 'package:equatable/equatable.dart';

/// Canonical seller identity axis.
///
/// This is intentionally separate from capability so that "unknown" can
/// remain distinct from "non-seller" during hydration and transient states.
enum SellerIdentityStatus { unknown, seller, nonSeller }

/// Canonical seller capability axis.
///
/// This is intentionally separate from identity so that "unknown" can remain
/// distinct from "inactive" when no backend snapshot is available yet.
enum SellerCapabilityStatus { unknown, active, inactive }

/// Seller state enum - represents the 3 honest states
enum SellerStateType {
  /// User has not created a seller profile
  notSeller,

  /// User has active seller subscription
  /// Can create listings and auctions
  active,

  /// User has seller profile but subscription has expired
  /// Cannot create listings/auctions until renewed
  expired,
}

/// Seller state entity - holds the state and related data
class SellerState extends Equatable {
  /// Current state type
  final SellerStateType type;

  /// Has created a seller profile (workspace identity)
  final bool hasSellerProfile;

  /// Subscription status from backend ('active', 'expired', 'none')
  final String? subscriptionStatus;

  /// Has market authority (has profile + active subscription)
  final bool? hasMarketAuthority;

  const SellerState({
    required this.type,
    required this.hasSellerProfile,
    this.subscriptionStatus,
    this.hasMarketAuthority,
  });

  /// Factory: Create NOT_SELLER state
  const factory SellerState.notSeller() = SellerState._notSeller;

  /// Factory: Create ACTIVE state
  const factory SellerState.active() = SellerState._active;

  /// Factory: Create EXPIRED state
  const factory SellerState.expired() = SellerState._expired;

  // Private constructors for factory
  const SellerState._notSeller()
    : type = SellerStateType.notSeller,
      hasSellerProfile = false,
      subscriptionStatus = null,
      hasMarketAuthority = false;

  const SellerState._active()
    : type = SellerStateType.active,
      hasSellerProfile = true,
      subscriptionStatus = 'active',
      hasMarketAuthority = true;

  const SellerState._expired()
    : type = SellerStateType.expired,
      hasSellerProfile = true,
      subscriptionStatus = 'expired',
      hasMarketAuthority = false;

  /// Create from AuthUser - canonical factory
  ///
  /// Derives seller state from backend truth:
  /// - hasSellerProfile = false → NOT_SELLER
  /// - hasMarketAuthority = true → ACTIVE
  /// - hasSellerProfile = true + hasMarketAuthority = false → EXPIRED
  factory SellerState.fromAuthUser(dynamic authUser) {
    // Handle null user
    if (authUser == null) {
      return const SellerState.notSeller();
    }

    // Check for seller profile
    final hasSellerProfile = authUser.hasSellerProfile ?? false;

    // If no seller profile, user is not a seller
    if (!hasSellerProfile) {
      return const SellerState.notSeller();
    }

    // Has seller profile - check market authority
    final hasMarketAuthority = authUser.hasMarketAuthority ?? false;

    if (hasMarketAuthority) {
      return const SellerState.active();
    }

    // Has profile but no market authority = expired
    return const SellerState.expired();
  }

  /// Check if user can create listings/auctions
  bool get canCreateContent => type == SellerStateType.active;

  /// Check if user is a seller (active or expired)
  bool get isSeller =>
      type == SellerStateType.active || type == SellerStateType.expired;

  /// Check if seller is active (not expired)
  bool get isActive => type == SellerStateType.active;

  /// Check if seller is expired
  bool get isExpired => type == SellerStateType.expired;

  /// Get display label for UI
  String get displayLabel {
    switch (type) {
      case SellerStateType.notSeller:
        return 'Bukan Penjual';
      case SellerStateType.active:
        return 'Aktif';
      case SellerStateType.expired:
        return 'Berakhir';
    }
  }

  /// Get CTA label for UI
  String? get ctaLabel {
    switch (type) {
      case SellerStateType.notSeller:
        return 'Mulai Jualan';
      case SellerStateType.active:
        return null; // No CTA needed, buttons are enabled
      case SellerStateType.expired:
        return 'Perpanjang Langganan';
    }
  }

  /// Get banner message for expired sellers
  String? get bannerMessage {
    switch (type) {
      case SellerStateType.notSeller:
      case SellerStateType.active:
        return null;
      case SellerStateType.expired:
        return 'Langganan Anda telah berakhir';
    }
  }

  @override
  List<Object?> get props => [
    type,
    hasSellerProfile,
    subscriptionStatus,
    hasMarketAuthority,
  ];
}

/// Extension for easy seller state checking
extension SellerStateExtension on SellerState {
  /// True if NOT_SELLER state
  bool get isNotSeller => type == SellerStateType.notSeller;

  /// True if ACTIVE state
  bool get canSell => type == SellerStateType.active;

  /// True if EXPIRED state
  bool get needsRenewal => type == SellerStateType.expired;

  /// Get color for badge display
  int get badgeColorValue {
    switch (type) {
      case SellerStateType.notSeller:
        return 0xFF9E9E9E; // Grey
      case SellerStateType.active:
        return 0xFF4CAF50; // Green
      case SellerStateType.expired:
        return 0xFFF44336; // Red
    }
  }
}
