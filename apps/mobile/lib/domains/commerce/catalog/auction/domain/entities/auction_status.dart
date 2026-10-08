/// Auction Status Enum
/// Backend-aligned with Go backend auction state machine
///
/// BACKEND AUTHORITY: Status values are determined by backend
/// Client MUST NOT compute or derive status from client-side logic
///
/// TRUTH: Backend canonical states (auction.go transitionAllowed):
/// - scheduled, active, waiting_settlement, ended, cancelled, lapsed
///
/// THERE IS NO DRAFT STATE (owner decision, Oct 2026): create = publish.
/// A created auction is born `scheduled` (start_mode=scheduled) or `active`
/// (start_mode=now); the seller never sees a draft workspace.
///
/// `lapsed` = the auction never went live because the seller's market
/// authority expired before activation. It is hidden from every public
/// surface and the seller can relist (republish) it after renewal.
///
/// PRESENTATION-ONLY STATES (NOT canonical):
/// - "sold" = ended + hasWinner (derived, not a backend state)
/// - "expired" = ended + !hasWinner (derived, not a backend state)
library;

/// Auction lifecycle statuses (backend-aligned canonical states only)
enum AuctionStatus {
  /// Auction is scheduled to start at a future time
  /// (born state — create = publish)
  /// API: `scheduled`
  scheduled,

  /// Auction is currently active and accepting bids
  /// API: `active`
  active,

  /// Auction has ended (may or may not have winner)
  /// API: `ended`
  ///
  /// NOTE: Use winnerId field to determine if sold:
  /// - winnerId != null → auction was sold
  /// - winnerId == null → auction expired with no bids
  ended,

  /// Waiting for winner to complete settlement (checkout/payment)
  /// API: `waiting_settlement`
  waitingSettlement,

  /// Auction was cancelled by the seller or by moderation/admin
  /// API: `cancelled`
  cancelled,

  /// Never went live: the seller's market authority expired before
  /// activation. Hidden from public surfaces; relistable (republish)
  /// after renewal.
  /// API: `lapsed`
  lapsed,
}

/// Extension for AuctionStatus API conversion
extension AuctionStatusApi on AuctionStatus {
  /// Convert to API value (snake_case)
  String get apiValue {
    switch (this) {
      case AuctionStatus.scheduled:
        return 'scheduled';
      case AuctionStatus.active:
        return 'active';
      case AuctionStatus.ended:
        return 'ended';
      case AuctionStatus.waitingSettlement:
        return 'waiting_settlement';
      case AuctionStatus.cancelled:
        return 'cancelled';
      case AuctionStatus.lapsed:
        return 'lapsed';
    }
  }

  /// Check if auction is currently active
  bool get isActive => this == AuctionStatus.active;

  /// Check if auction is in a terminal state
  bool get isTerminal =>
      this == AuctionStatus.ended ||
      this == AuctionStatus.waitingSettlement ||
      this == AuctionStatus.cancelled;

  /// Check if auction was cancelled
  bool get isCancelled => this == AuctionStatus.cancelled;

  /// Check if auction never went live (authority lapsed before activation)
  bool get isLapsed => this == AuctionStatus.lapsed;

  /// Check if auction is in pre-active state (scheduled — lapsed never went live)
  bool get isPreActive => this == AuctionStatus.scheduled;

  /// Display name for AuctionStatus (Indonesian)
  String get displayName {
    switch (this) {
      case AuctionStatus.scheduled:
        return 'Terjadwal';
      case AuctionStatus.active:
        return 'Aktif';
      case AuctionStatus.ended:
        return 'Berakhir';
      case AuctionStatus.waitingSettlement:
        return 'Menunggu Penyelesaian';
      case AuctionStatus.cancelled:
        return 'Dibatalkan';
      case AuctionStatus.lapsed:
        return 'Kadaluarsa';
    }
  }
}

/// Parse AuctionStatus from API value
///
/// Handles backend canonical states (scheduled, active, waiting_settlement,
/// ended, cancelled, lapsed) and normalizes legacy/presentation states
/// (draft, sold, expired, expired_bnr) to their canonical mapping.
///
/// IMPORTANT: 'sold'/'expired' are NOT backend states.
/// 'sold' and 'expired' map to 'ended' — use Auction.winnerId to determine
/// the actual outcome:
/// - winnerId != null → auction was sold
/// - winnerId == null → auction expired without bids
/// Legacy 'draft'/'expired_bnr' rows map to 'lapsed' — the canonical
/// never-live/relistable state (draft no longer exists anywhere).
///
/// Unknown values map to 'cancelled' — the backend PublicPhase default for
/// statuses that are not part of the public vocabulary.
AuctionStatus parseAuctionStatus(String? value) {
  if (value == null) return AuctionStatus.cancelled;

  // Normalize to lowercase for case-insensitive matching
  final normalized = value.toLowerCase().trim();

  // Direct matches (backend-aligned canonical states)
  switch (normalized) {
    case 'scheduled':
      return AuctionStatus.scheduled;
    case 'active':
      return AuctionStatus.active;
    case 'ended':
      return AuctionStatus.ended;
    case 'waiting_settlement':
    case 'waitingsettlement':
      return AuctionStatus.waitingSettlement;
    case 'lapsed':
      return AuctionStatus.lapsed;
    case 'draft':
    case 'expired_bnr':
    case 'expiredbnr':
      // PURGED legacy vocabulary: draft never crosses the wire anymore.
      // A draft was never live — that is exactly 'lapsed'.
      return AuctionStatus.lapsed;
    case 'cancelled':
    case 'canceled':
      return AuctionStatus.cancelled; // Handle alternate spelling (US English)
    case 'sold':
    case 'expired':
      // These are NOT backend states - map to 'ended'
      // Use Auction.winnerId to determine actual outcome
      return AuctionStatus.ended;
    default:
      // Unknown values: the backend PublicPhase default (cancelled).
      return AuctionStatus.cancelled;
  }
}
