/// Auction Status Enum
/// Backend-aligned with Go backend auction state machine
///
/// BACKEND AUTHORITY: Status values are determined by backend
/// Client MUST NOT compute or derive status from client-side logic
///
/// TRUTH: Backend has 6 canonical states (auction.go transitionAllowed):
/// - draft, scheduled, active, waiting_settlement, ended, cancelled
///
/// PRESENTATION-ONLY STATES (NOT canonical):
/// - "sold" = ended + hasWinner (derived, not a backend state)
/// - "expired" = ended + !hasWinner (derived, not a backend state)
///
/// PURGED: expired_bnr — never a backend state. Settlement failure
/// (winner did not pay) returns the auction to DRAFT via
/// TransitionToDraftOnSettlementFailure; the winner-violation record is
/// backend bookkeeping, not an auction status.
library;

/// Auction lifecycle statuses (backend-aligned canonical states only)
enum AuctionStatus {
  /// Seller is drafting the auction
  /// API: `draft`
  draft,

  /// Auction is scheduled to start at a future time
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
  ///
  /// Winner must complete purchase; on settlement failure the auction
  /// returns to DRAFT (backend TransitionToDraftOnSettlementFailure).
  waitingSettlement,

  /// Auction was cancelled by the seller
  /// API: `cancelled`
  cancelled,
}

/// Extension for AuctionStatus API conversion
extension AuctionStatusApi on AuctionStatus {
  /// Convert to API value (snake_case)
  String get apiValue {
    switch (this) {
      case AuctionStatus.draft:
        return 'draft';
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

  /// Check if auction is in pre-active state
  bool get isPreActive =>
      this == AuctionStatus.draft || this == AuctionStatus.scheduled;

  /// Display name for AuctionStatus (Indonesian)
  String get displayName {
    switch (this) {
      case AuctionStatus.draft:
        return 'Draft';
      case AuctionStatus.scheduled:
        return 'Terjadwal';
      case AuctionStatus.active:
        return 'Aktif';
      case AuctionStatus.ended:
        return 'Berakhir';
      case AuctionStatus.waitingSettlement:
        return 'Menunggu Pembayaran';
      case AuctionStatus.cancelled:
        return 'Dibatalkan';
    }
  }
}

/// Parse AuctionStatus from API value
///
/// Handles backend canonical states (draft, scheduled, active,
/// waiting_settlement, ended, cancelled) and normalizes legacy/presentation
/// states (sold, expired, expired_bnr) to their canonical mapping.
///
/// IMPORTANT: 'sold'/'expired'/'expired_bnr' are NOT backend states.
/// 'sold' and 'expired' map to 'ended' — use Auction.winnerId to determine
/// the actual outcome:
/// - winnerId != null → auction was sold
/// - winnerId == null → auction expired without bids
/// 'expired_bnr' maps to 'draft' — that is where the backend actually puts
/// the auction after settlement failure (TransitionToDraftOnSettlementFailure).
AuctionStatus parseAuctionStatus(String? value) {
  if (value == null) return AuctionStatus.draft;

  // Normalize to lowercase for case-insensitive matching
  final normalized = value.toLowerCase().trim();

  // Direct matches (backend-aligned canonical states)
  switch (normalized) {
    case 'draft':
      return AuctionStatus.draft;
    case 'scheduled':
      return AuctionStatus.scheduled;
    case 'active':
      return AuctionStatus.active;
    case 'ended':
      return AuctionStatus.ended;
    case 'waiting_settlement':
      return AuctionStatus.waitingSettlement;
    case 'waitingsettlement':
      // Handle alternate formatting (no underscore)
      return AuctionStatus.waitingSettlement;
    case 'expired_bnr':
    case 'expiredbnr':
      // PURGED status: never a backend state. Settlement failure returns the
      // auction to draft — map it there, never to a phantom enum value.
      return AuctionStatus.draft;
    case 'cancelled':
    case 'canceled':
      return AuctionStatus.cancelled; // Handle alternate spelling (US English)
    case 'sold':
    case 'expired':
      // These are NOT backend states - map to 'ended'
      // Use Auction.winnerId to determine actual outcome
      return AuctionStatus.ended;
    default:
      // Fallback for unknown values - treat as draft
      return AuctionStatus.draft;
  }
}
