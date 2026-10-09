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
        return 'pending_payment';
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
// =============================================================================
// ESCROW STATUS — PURGED
// =============================================================================
// The order-level escrow status projection was removed from the backend
// (orders.escrow_status dropped; escrows table is the sole authority) and
// from this client. Order UI decisions come exclusively from the backend
// Decision contract (primary_action / secondary_actions). If an escrow
// badge is ever needed, it must be fed by a dedicated live escrow read —
// never by a persisted order projection.
// =============================================================================

// =============================================================================
// REMOVED: OrderIssue enum and OrderIssueExtension
// =============================================================================
// These were never populated or used in the codebase.
// Status authority comes from backend decision.state, not local issue tracking.
// If issues need to be tracked in the future, they should come from backend
// decision display hints or a dedicated issues endpoint.
// =============================================================================
