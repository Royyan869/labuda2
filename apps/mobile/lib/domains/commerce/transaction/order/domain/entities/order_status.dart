// =============================================================================
// ORDER STATUS - GUARDRAILS
// =============================================================================
//
// STATUS AUTHORITY: Backend Go enum is SINGLE SOURCE OF TRUTH
// Backend source: backend/internal/domain/order/entity/order_status.go
//
// STATUS LIFECYCLE:
// - Normal: pending -> paid -> shipped -> delivered -> completed
// - Terminal: cancelled, cancelled_timeout, refunded, partially_refunded, dispute_open, expired
//
// DO NOT:
// - Add or remove statuses without backend alignment
// - Create "convenience" status combinations in Flutter
// - Add status variants that don't exist in backend
//
// WHEN PARSING:
// - Use OrderStatusExtension.parse() for string → OrderStatus
//
// ═══════════════════════════════════════════════════════════════════════════════
// LEGACY STATUS MAPPINGS (COMPATIBILITY-ONLY, NON-CANONICAL)
// ═══════════════════════════════════════════════════════════════════════════════
// The following legacy status strings are mapped for backward compatibility:
// - waiting_payment / waitingpayment → pending (pre-P11 backend status)
// - confirmed → paid (pre-P11 backend status, renamed to 'paid')
// - processing → paid (was never a real backend status, safety mapping)
//
// These mappings exist because:
// 1. Existing orders in the database may have old status values
// 2. Backend API may still send these for backward compatibility
// 3. Removing them would break display of historical orders
//
// These can be REMOVED when:
// - All historical orders have been migrated to new status values
// - Backend no longer sends legacy status strings
// - A coordinated migration is completed
// ═══════════════════════════════════════════════════════════════════════════════
// =============================================================================
//

/// Order Status (Primary)
///
/// Backend authority - statuses match backend Go enums.
/// Do not add or remove statuses without backend alignment.
///
/// Backend source: backend/internal/commerce/order/entity/order_status.go
/// B4A Status lifecycle: pending -> paid -> shipped -> completed (buyer-facing)
/// StatusDelivered exists in backend but is unreachable — no code path sets it.
/// Terminal states: cancelled, cancelledTimeout, refunded, partially_refunded, dispute_open, expired
enum OrderStatus {
  pending, // Backend: StatusPending ("pending_payment") - Initial state when order is created
  paid, // Backend: StatusPaid - Buyer payment confirmed (renamed from 'confirmed' in P11 migration)
  shipped, // Backend: StatusShipped - Seller ships the item
  delivered, // Backend: StatusDelivered - Buyer confirms receipt
  completed, // Backend: StatusCompleted - Order fully settled, escrow released
  cancelled, // Backend: StatusCancelled - Order cancelled before payment
  cancelledTimeout, // Backend: StatusCancelledTimeout ("cancelled_timeout") - Auto-cancelled due to shipment timeout
  refunded, // Backend: StatusRefunded - Order refunded
  disputeOpen, // Backend: StatusDisputeOpen - Dispute active, escrow frozen (dispute_open)
  partiallyRefunded, // Backend: StatusPartiallyRefunded - Partial refund processed
  expired, // Backend: StatusExpired - Payment expired, order terminated
}

/// Extension for OrderStatus parsing
extension OrderStatusExtension on OrderStatus {
  String get value {
    switch (this) {
      case OrderStatus.pending:
        return 'pending';
      case OrderStatus.paid:
        return 'paid';
      case OrderStatus.shipped:
        return 'shipped';
      case OrderStatus.delivered:
        return 'delivered';
      case OrderStatus.completed:
        return 'completed';
      case OrderStatus.cancelled:
        return 'cancelled';
      case OrderStatus.cancelledTimeout:
        return 'cancelled_timeout';
      case OrderStatus.refunded:
        return 'refunded';
      case OrderStatus.disputeOpen:
        return 'dispute_open';
      case OrderStatus.partiallyRefunded:
        return 'partially_refunded';
      case OrderStatus.expired:
        return 'expired';
    }
  }

  static OrderStatus? parse(String? value) {
    if (value == null) return null;
    switch (value.toLowerCase()) {
      case 'pending':
      case 'pending_payment': // Backend canonical wire value (StatusPending = "pending_payment")
        return OrderStatus.pending;
      case 'paid':
        return OrderStatus
            .paid; // Backend sends 'paid' → frontend OrderStatus.paid (P11 aligned)
      // Legacy: waiting_payment → pending
      case 'waiting_payment':
      case 'waitingpayment':
        return OrderStatus.pending;
      // Legacy: confirmed → paid (for backward compatibility during P11 migration)
      case 'confirmed':
        return OrderStatus.paid;
      // Legacy: processing was removed in O1 - map to paid for safety
      // 'processing' was never a real backend status
      case 'processing':
        return OrderStatus.paid;
      case 'shipped':
        return OrderStatus.shipped;
      case 'delivered':
        return OrderStatus.delivered;
      case 'completed':
        return OrderStatus.completed;
      case 'cancelled':
        return OrderStatus.cancelled;
      case 'cancelled_timeout':
      case 'cancelledtimeout':
        return OrderStatus.cancelledTimeout;
      case 'refunded':
        return OrderStatus.refunded;
      case 'dispute_open':
      case 'disputeopen':
        return OrderStatus.disputeOpen;
      case 'partially_refunded':
      case 'partiallyrefunded':
        return OrderStatus.partiallyRefunded;
      case 'expired':
        return OrderStatus.expired;
      default:
        return null;
    }
  }
}

// =============================================================================
// ESCROW STATUS - FINANCIAL STATE
// =============================================================================
//
// STATUS AUTHORITY: Backend Go enum is SINGLE SOURCE OF TRUTH
// Backend source: backend/internal/commerce/order/entity/escrow_status.go
//
// CRITICAL: This is a READ-ONLY projection of Wallet.Escrow.Status.
// MUST match wallet/entity/escrow.go exactly.
//
// Valid states (mirrored from Wallet):
// - "holding": Funds held in escrow awaiting completion
// - "released": Funds released to seller
// - "refunded": Funds refunded to buyer
//
// STATES REMOVED (no longer derivable from Wallet):
// - "none": Use Order.Status = "pending" instead
// - "frozen": Use Order.HasDispute = true instead
// - "partially_refunded": Use Order.Status + separate tracking
// - "partially_released": Use Order.Status + separate tracking
//
// RULE: EscrowStatus can ONLY be set by deriving from Wallet.Escrow.Status.
// NEVER set independently based on business logic.
//
// ═══════════════════════════════════════════════════════════════════════════════
// BACKEND ALIGNMENT:
// ═══════════════════════════════════════════════════════════════════════════════
// This enum MUST match the backend Go enum exactly:
// backend/internal/commerce/order/entity/escrow_status.go
//
// Backend Values (ONLY 3 STATES):
// - "holding" → EscrowStatus.holding
// - "released" → EscrowStatus.released
// - "refunded" → EscrowStatus.refunded
//
// NO OTHER STATES EXIST IN BACKEND.
// ═══════════════════════════════════════════════════════════════════════════════

/// Escrow Status (Financial State)
///
/// Backend authority - statuses match backend Go enums exactly.
/// DO NOT add or remove statuses without backend alignment.
///
/// Backend source: backend/internal/commerce/order/entity/escrow_status.go
/// Escrow lifecycle: holding → released OR holding → refunded
enum EscrowStatus {
  /// Funds are held in escrow awaiting completion
  /// This is the ACTIVE escrow state during normal order flow
  holding, // Backend: EscrowStatusHolding
  /// Funds have been released to seller (terminal state)
  released, // Backend: EscrowStatusReleased
  /// Funds have been refunded to buyer (terminal state)
  refunded, // Backend: EscrowStatusRefunded
}

/// Extension for EscrowStatus parsing
extension EscrowStatusExtension on EscrowStatus {
  String get value {
    switch (this) {
      case EscrowStatus.holding:
        return 'holding';
      case EscrowStatus.released:
        return 'released';
      case EscrowStatus.refunded:
        return 'refunded';
    }
  }

  static EscrowStatus? parse(String? value) {
    if (value == null) return null;
    switch (value.toLowerCase()) {
      case 'holding':
        return EscrowStatus.holding;
      case 'released':
        return EscrowStatus.released;
      case 'refunded':
        return EscrowStatus.refunded;
      default:
        // Unknown value - log warning and return null
        // Caller should handle null appropriately
        return null;
    }
  }

  /// Check if escrow is active (holding funds)
  bool get isActive {
    return this == EscrowStatus.holding;
  }

  /// Check if escrow is in terminal state (no further changes possible)
  bool get isTerminal {
    return this == EscrowStatus.released || this == EscrowStatus.refunded;
  }

  /// Check if funds are currently held (not released/refunded)
  bool get isHolding {
    return this == EscrowStatus.holding;
  }
}

// =============================================================================
// ESCROW STATUS NULL-SAFE HELPERS
// =============================================================================
//
// Helper extensions for handling nullable EscrowStatus in UI.
// Used when backend sends unknown values (should never happen in production).
// =============================================================================

/// Extension for nullable EscrowStatus (null-safe UI helpers)
extension EscrowStatusNullable on EscrowStatus? {
  /// Get display label for escrow status (null-safe)
  String get displayLabel {
    if (this == null) return 'Status Escrow Tidak Diketahui';
    switch (this!) {
      case EscrowStatus.holding:
        return 'Dana Ditahan';
      case EscrowStatus.released:
        return 'Dana Dilepas ke Penjual';
      case EscrowStatus.refunded:
        return 'Dana Dikembalikan ke Pembeli';
    }
  }

  /// Check if escrow is active (null-safe)
  bool get isActive {
    return this?.isActive ?? false;
  }

  /// Check if escrow is in terminal state (null-safe)
  bool get isTerminal {
    return this?.isTerminal ?? false;
  }

  /// Check if funds are held (null-safe)
  bool get isHolding {
    return this?.isHolding ?? false;
  }

  /// Get emoji for escrow status (null-safe)
  String get emoji {
    if (this == null) return '❓';
    switch (this!) {
      case EscrowStatus.holding:
        return '🔒';
      case EscrowStatus.released:
        return '💰';
      case EscrowStatus.refunded:
        return '↩️';
    }
  }
}

// =============================================================================
// REMOVED: OrderIssue enum and OrderIssueExtension
// =============================================================================
// These were never populated or used in the codebase.
// Status authority comes from backend decision.state, not local issue tracking.
// If issues need to be tracked in the future, they should come from backend
// decision display hints or a dedicated issues endpoint.
// =============================================================================
