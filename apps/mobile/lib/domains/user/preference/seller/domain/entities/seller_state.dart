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
/// **TWO CANONICAL AXES (never collapse them):**
/// 1. IDENTITY (`hasSellerProfile`) — workspace identity. Presence of a
///    seller_profiles row. Says nothing about money.
/// 2. EXPIRY (`sellerSubscriptionStatus`) — the latest seller_subscriptions
///    row's status: 'active' | 'expired' | 'none'.
///
/// `hasMarketAuthority` is the backend's CAPABILITY verdict (profile AND an
/// active, in-window subscription interval). Capability `false` means "cannot
/// sell right now" — it does NOT mean "expired". A freshly onboarded seller
/// whose payment has not settled yet is capability-false with expiry 'none'.
///
/// **RULE (anti-regression):** only `subscriptionStatus == 'expired'` may be
/// rendered as "Berakhir" / "Perpanjang Langganan". Deriving expiry from
/// `!hasMarketAuthority` is the RF-02 defect that told fresh sellers their
/// subscription had expired.
///
/// **4 STATES:**
/// 1. NOT_SELLER - No seller profile
/// 2. PENDING_ACTIVATION - Has seller profile, no active interval yet ('none')
/// 3. ACTIVE - Has seller profile + active subscription
/// 4. EXPIRED - Has seller profile + subscription that has ENDED
///
/// **UI MAPPING:**
/// - NOT_SELLER: "Mulai Jualan" CTA
/// - PENDING_ACTIVATION: "Buat ForSale", "Buat Lelang" disabled, no expiry claim
/// - ACTIVE: "Buat ForSale", "Buat Lelang" enabled
/// - EXPIRED: "Buat ForSale", "Buat Lelang" disabled + "Perpanjang Langganan" CTA
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

/// Seller state enum - represents the honest states
enum SellerStateType {
  /// User has not created a seller profile
  notSeller,

  /// User has a seller profile but no ACTIVE subscription interval yet
  /// (backend `seller_subscription_status == 'none'`). This is a normal,
  /// non-expired state: onboarding is done, the subscription is not activated.
  /// Cannot create forSales/auctions, but MUST NOT be told they expired.
  pendingActivation,

  /// User has active seller subscription
  /// Can create forSales and auctions
  active,

  /// User has seller profile and their subscription period has ENDED
  /// (backend `seller_subscription_status == 'expired'`).
  /// Cannot create forSales/auctions until renewed
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

  /// Factory: Create PENDING_ACTIVATION state
  const factory SellerState.pendingActivation() = SellerState._pendingActivation;

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

  const SellerState._pendingActivation()
    : type = SellerStateType.pendingActivation,
      hasSellerProfile = true,
      subscriptionStatus = 'none',
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
  /// Derives seller state from backend truth, in this order:
  /// - no seller profile → NOT_SELLER
  /// - market authority (backend capability verdict) OR 'active' → ACTIVE
  /// - 'expired' → EXPIRED
  /// - otherwise (profile + 'none'/absent) → PENDING_ACTIVATION
  ///
  /// Capability is checked before expiry so a wire surface that omits
  /// `seller_subscription_status` can never downgrade an active seller.
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

    final hasMarketAuthority = authUser.hasMarketAuthority ?? false;
    final subscriptionStatus = authUser.sellerSubscriptionStatus as String?;

    if (hasMarketAuthority || subscriptionStatus == 'active') {
      return const SellerState.active();
    }

    // Only an ENDED subscription period is "expired".
    if (subscriptionStatus == 'expired') {
      return const SellerState.expired();
    }

    // Seller profile without an active interval yet (payment not settled).
    return const SellerState.pendingActivation();
  }

  /// Check if user can create forSales/auctions
  bool get canCreateContent => type == SellerStateType.active;

  /// Check if user is a seller (active, expired, or awaiting activation)
  bool get isSeller =>
      type == SellerStateType.active ||
      type == SellerStateType.expired ||
      type == SellerStateType.pendingActivation;

  /// Check if seller is active (not expired)
  bool get isActive => type == SellerStateType.active;

  /// Check if seller is expired
  bool get isExpired => type == SellerStateType.expired;

  /// True when the seller has a profile but no active interval yet.
  bool get isPendingActivation => type == SellerStateType.pendingActivation;

  /// Get display label for UI
  String get displayLabel {
    switch (type) {
      case SellerStateType.notSeller:
        return 'Bukan Penjual';
      case SellerStateType.pendingActivation:
        return 'Belum Aktif';
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
      case SellerStateType.pendingActivation:
        return 'Aktifkan Langganan';
      case SellerStateType.active:
        return null; // No CTA needed, buttons are enabled
      case SellerStateType.expired:
        return 'Perpanjang Langganan';
    }
  }

  /// Get banner message — ONLY for an ENDED subscription. A seller who has not
  /// activated yet gets no "has ended" claim (RF-02).
  String? get bannerMessage {
    switch (type) {
      case SellerStateType.notSeller:
      case SellerStateType.pendingActivation:
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
      case SellerStateType.pendingActivation:
        return 0xFFFF9800; // Amber — not an error state
      case SellerStateType.active:
        return 0xFF4CAF50; // Green
      case SellerStateType.expired:
        return 0xFFF44336; // Red
    }
  }
}
