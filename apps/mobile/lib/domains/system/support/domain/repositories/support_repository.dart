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
  /// Returns chatId of created ticket
  Future<SupportResult<String>> createSupportChat({
    required String userId,
    required String userName,
    String? userAvatar,
    required SupportCategory category,
    SupportPriority priority = SupportPriority.medium,
    String? description,
    String? linkedOrderId,
  });

  // ============================================
  // READ OPERATIONS
  // ============================================

  /// Get ticket by ID
  Future<SupportResult<SupportTicket>> getTicket(String ticketId);

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

  /// Get ticket messages (conversation thread) - Read-only for users
  Future<SupportResult<List<SupportMessage>>> getMessages(
    String ticketId, {
    int limit = 100,
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
