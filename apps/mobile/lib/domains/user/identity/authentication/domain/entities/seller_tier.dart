/// Seller performance tier in LABUDA platform
///
/// **BACKEND AUTHORITY:** This enum MUST match the Go backend exactly.
/// Source: backend/internal/domain/seller/entity/seller_profile.go
///
/// **IMPORTANT:** Seller tiers are NOT roles. They are performance-based levels
/// calculated from sales volume, rating score, and performance metrics.
///
/// **Roles vs Tiers:**
/// - Roles (UserRole): Permission-based (buyer, seller, support_admin, super_admin)
/// - Tiers (SellerTier): Performance-based (basic, pro, elite)
///
/// Tier evaluation is backend-only (SellerReputationRecomputeWorker).
/// Mobile displays the tier badge but does not evaluate thresholds.
///
/// Current ladder: Basic → Pro → Elite.
/// Legend tier is intentionally deferred — when adding it:
///   1. Add sellerLegend value here
///   2. Update apiValue, displayName, icon switches
///   3. Update SellerTierBadge._tierConfig to render Legend badge
///   4. No architecture rewrite required.
enum SellerTier {
  /// Basic seller tier — API value: "basic"
  sellerBasic,

  /// Pro seller tier — API value: "pro"
  sellerPro,

  /// Elite seller tier — API value: "elite"
  sellerElite;
  // sellerLegend — intentionally deferred; add when ready.

  /// Convert from API value to SellerTier enum
  ///
  /// **CANONICAL ALIGNMENT:** Backend uses bare 'basic', 'pro', 'elite'
  /// Source: backend/internal/domain/seller/entity/seller_profile.go
  static SellerTier? fromApiValue(String? value) {
    if (value == null || value.isEmpty) return null;
    return SellerTier.values.firstWhere(
      (tier) => tier.apiValue == value,
      orElse: () => SellerTier.sellerBasic,
    );
  }

  /// Get API value for backend communication
  ///
  /// **CANONICAL ALIGNMENT:** Returns bare 'basic', 'pro', 'elite' to match backend Tier
  /// Source: backend/internal/domain/seller/entity/seller_profile.go
  String get apiValue {
    switch (this) {
      case SellerTier.sellerBasic:
        return 'basic';
      case SellerTier.sellerPro:
        return 'pro';
      case SellerTier.sellerElite:
        return 'elite';
    }
  }

  /// Check if seller is elite
  bool get isElite => this == SellerTier.sellerElite;

  /// Check if seller is pro or elite
  bool get isProOrHigher =>
      this == SellerTier.sellerPro || this == SellerTier.sellerElite;

  /// Check if seller is basic or higher
  bool get isBasicOrHigher => true; // All values are at least basic

  /// Display name for UI
  String get displayName {
    switch (this) {
      case SellerTier.sellerBasic:
        return 'Basic Seller';
      case SellerTier.sellerPro:
        return 'Pro Seller';
      case SellerTier.sellerElite:
        return 'Elite Seller';
    }
  }

  /// Get badge icon/emoji for UI
  String get icon {
    switch (this) {
      case SellerTier.sellerBasic:
        return '🌱'; // Sprout for new badge earners
      case SellerTier.sellerPro:
        return '⭐'; // Star for proven sellers
      case SellerTier.sellerElite:
        return '👑'; // Crown for top performers
    }
  }
}
