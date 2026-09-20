library;

/// Repository Interface for Support Module
/// Domain layer - defines kontrak untuk data access
/// Pure Dart - bebas dari Firebase, Flutter, dan external dependencies

import 'package:labuda/domains/system/support/domain/entities/support_failure.dart';
import 'package:labuda/domains/system/support/domain/entities/support_ticket.dart';
import 'package:labuda/domains/system/support/domain/entities/support_event.dart';
import 'package:labuda/domains/system/support/domain/entities/support_message.dart';

// Export entities for convenience
export 'package:labuda/domains/system/support/domain/entities/support_ticket.dart';
export 'package:labuda/domains/system/support/domain/entities/support_event.dart';
export 'package:labuda/domains/system/support/domain/entities/support_failure.dart';
export 'package:labuda/domains/system/support/domain/entities/support_config.dart';
export 'package:labuda/domains/system/support/domain/entities/support_message.dart';

// ============================================
// REPOSITORY INTERFACE
// ============================================

/// Support Repository Interface
/// Mendefinisikan kontrak untuk operasi support ticket
/// Implementation ada di data layer (Firebase-based)
abstract class SupportRepository {
  // ============================================
  // CREATE OPERATIONS
  // ============================================

  /// Create support chat/ticket
  ///
  /// The Support API is the sole authority for ticket creation. `userId`,
  /// `userName` and `userAvatar` are local display/context hints only — the
  /// backend derives ownership from the authenticated session. `category` uses
  /// the canonical taxonomy and is serialized as its canonical wire value.
  /// Returns the id of the created ticket (not a chat room id).
  Future<SupportResult<String>> createTicket({
    required String userId,
    required String userName,
    String? userAvatar,
    required SupportCategory category,
    SupportPriority priority = SupportPriority.medium,
    String? subject,
    String? description,
    String? linkedOrderId,
  });

  // ============================================
  // READ OPERATIONS
  // ============================================

  /// Get ticket by ID
  Future<SupportResult<SupportTicket>> getTicket(String ticketId);

  /// List the authenticated user's own tickets.
  ///
  /// The Support API is the identity authority for the ticket list. The chat
  /// room list must not be used to discover Support tickets.
  Future<SupportResult<List<SupportTicket>>> getMyTickets({int limit = 50});

  // REMOVED: watchTickets() - Admin-only endpoint
  // REMOVED: watchUnclaimedTicketsCount() - Admin-only endpoint
  // REMOVED: watchOpenTicketsCount() - Admin-only endpoint
  // REMOVED: watchMyTicketsCount() - Admin-only endpoint
  // REMOVED: getStatistics() - Admin-only endpoint

  // ============================================
  // UPDATE OPERATIONS
  // ============================================

  // REMOVED: claimTicket() - Admin-only endpoint
  // REMOVED: resolveTicket() - Admin-only endpoint

  /// Reopen ticket (User-only)
  Future<SupportResult<void>> reopenTicket(ReopenTicketRequest request);

  // REMOVED: closeTicket() - Admin-only endpoint
  // REMOVED: updateTicketPriority() - Admin-only endpoint
  // REMOVED: updateTicketCategory() - Admin-only endpoint

  // ============================================
  // MESSAGE OPERATIONS
  // ============================================

  /// Get ticket messages (conversation thread)
  Future<SupportResult<List<SupportMessage>>> getMessages(
    String ticketId, {
    int limit = 100,
  });

  /// Send a reply into the authenticated user's own ticket conversation.
  ///
  /// Only the message text is supplied — the backend derives the sender from
  /// the authenticated session. The client never sends a sender identity.
  Future<SupportResult<void>> sendMessage({
    required String ticketId,
    required String message,
  });

  // REMOVED: sendGreetingMessage() - Admin-only endpoint
  // REMOVED: sendSystemMessage() - Admin-only endpoint

  // ============================================
  // EVENT OPERATIONS
  // ============================================

  /// Get ticket events (audit trail) - Read-only for users
  Future<SupportResult<List<SupportEvent>>> getEvents(
    String ticketId, {
    int limit = 100,
  });

  // REMOVED: All ADMIN OPERATIONS section
  // - getSupportAdminIds()
  // - notifyAdminsAboutNewTicket()
}
