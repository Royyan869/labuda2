/// Stop reason constants for promotion instances.
///
/// Canonical stop reason constants - these match backend values.
class StopReason {
  StopReason._();

  // User-initiated stops
  static const String userPaused = 'user_paused';
  static const String userCancelled = 'user_cancelled';

  // Admin-initiated stops
  static const String adminCancelled = 'admin_cancelled';

  // Duration-based stops
  static const String durationExhausted = 'duration_exhausted';
  static const String validityExpired = 'validity_expired';

  // Fixed-price sale-specific stops
  static const String fixedPriceSaleSold = 'fixed_price_sale_sold';
  static const String fixedPriceSaleHidden = 'fixed_price_sale_hidden';
  static const String fixedPriceSaleDeleted = 'fixed_price_sale_deleted';
  static const String fixedPriceSaleModerated = 'fixed_price_sale_moderated';
  static const String fixedPriceSaleExpired = 'fixed_price_sale_expired';

  // Auction-specific stops
  static const String auctionEnded = 'auction_ended';
  static const String auctionCancelled = 'auction_cancelled';
  static const String auctionDeleted = 'auction_deleted';
  static const String auctionModerated = 'auction_moderated';

  // External product stops
  static const String externalInvalid = 'external_invalid';
}
