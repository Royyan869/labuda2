/// Auction Condition Enum
/// Backend-aligned with Go backend auction_enums.go
///
/// BACKEND AUTHORITY: Condition values are determined by backend
///
/// Backend Source: backend/internal/domain/auction/domain/entity/auction_enums.go:17-25
library;

/// Condition of auction item
enum AuctionCondition {
  /// Brand new, unused item
  /// API: `new`
  newItem,

  /// Like new - minimal signs of use
  /// API: `like_new`
  likeNew,

  /// Good condition - normal signs of use
  /// API: `good`
  good,

  /// Fair condition - noticeable wear
  /// API: `fair`
  fair,

  /// For parts or not working
  /// API: `for_parts`
  forParts,
}

/// Extension for AuctionCondition API conversion
extension AuctionConditionApi on AuctionCondition {
  /// Convert to API value (snake_case)
  String get apiValue {
    switch (this) {
      case AuctionCondition.newItem:
        return 'new';
      case AuctionCondition.likeNew:
        return 'like_new';
      case AuctionCondition.good:
        return 'good';
      case AuctionCondition.fair:
        return 'fair';
      case AuctionCondition.forParts:
        return 'for_parts';
    }
  }

  /// Display label for UI
  String get displayName {
    switch (this) {
      case AuctionCondition.newItem:
        return 'New';
      case AuctionCondition.likeNew:
        return 'Like New';
      case AuctionCondition.good:
        return 'Good';
      case AuctionCondition.fair:
        return 'Fair';
      case AuctionCondition.forParts:
        return 'For Parts';
    }
  }
}

/// Parse AuctionCondition from API value with legacy fallback support
AuctionCondition? parseAuctionCondition(String? value) {
  if (value == null) return null;

  // Normalize to lowercase for case-insensitive matching
  final normalized = value.toLowerCase().trim();

  // Direct matches (backend-aligned)
  switch (normalized) {
    case 'new':
      return AuctionCondition.newItem;
    case 'like_new':
      return AuctionCondition.likeNew;
    case 'good':
      return AuctionCondition.good;
    case 'fair':
      return AuctionCondition.fair;
    case 'for_parts':
      return AuctionCondition.forParts;
    default:
      // Return null for unknown values
      return null;
  }
}
