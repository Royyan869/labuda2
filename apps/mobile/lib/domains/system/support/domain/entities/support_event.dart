/// Support Ticket Event Entity
/// Represents an audit trail event for ticket state transitions
library;

import 'package:equatable/equatable.dart';

/// Support Event Type - type of event that occurred on a ticket
enum SupportEventType {
  ticketCreated, // Ticket was created
  ticketClaimed, // Admin claimed ticket
  ticketWaitingUser, // Status changed to waiting for user
  statusChanged, // General status change
  priorityChanged, // Priority was changed
  categoryChanged, // Category was changed
  ticketResolved, // Ticket was resolved
  ticketClosed, // Ticket was closed
  ticketReopened, // Ticket was reopened
  adminAssigned, // Admin was assigned
  adminUnassigned, // Admin was unassigned
  unknown, // Unknown event type
}

/// Support Event - audit trail entry for ticket state changes
class SupportEvent extends Equatable {
  final String id;
  final String ticketId;
  final SupportEventType eventType;
  final String? actorId; // User/admin who performed the action
  final String? actorName;
  final String? oldStatus;
  final String? newStatus;
  final String? notes;
  final Map<String, dynamic>? metadata;
  final DateTime createdAt;

  const SupportEvent({
    required this.id,
    required this.ticketId,
    required this.eventType,
    this.actorId,
    this.actorName,
    this.oldStatus,
    this.newStatus,
    this.notes,
    this.metadata,
    required this.createdAt,
  });

  /// Helper: get display label for event type
  String get eventTypeLabel {
    switch (eventType) {
      case SupportEventType.ticketCreated:
        return 'Ticket Created';
      case SupportEventType.ticketClaimed:
        return 'Ticket Claimed';
      case SupportEventType.ticketWaitingUser:
        return 'Waiting for User';
      case SupportEventType.statusChanged:
        return 'Status Changed';
      case SupportEventType.priorityChanged:
        return 'Priority Changed';
      case SupportEventType.categoryChanged:
        return 'Category Changed';
      case SupportEventType.ticketResolved:
        return 'Ticket Resolved';
      case SupportEventType.ticketClosed:
        return 'Ticket Closed';
      case SupportEventType.ticketReopened:
        return 'Ticket Reopened';
      case SupportEventType.adminAssigned:
        return 'Admin Assigned';
      case SupportEventType.adminUnassigned:
        return 'Admin Unassigned';
      case SupportEventType.unknown:
        return 'Unknown Event';
    }
  }

  /// Helper: get description for the event
  String get description {
    switch (eventType) {
      case SupportEventType.ticketCreated:
        return 'Ticket was created';
      case SupportEventType.ticketClaimed:
        return actorName != null
            ? '$actorName claimed this ticket'
            : 'Ticket was claimed';
      case SupportEventType.ticketWaitingUser:
        return 'Waiting for user response';
      case SupportEventType.statusChanged:
        if (oldStatus != null && newStatus != null) {
          return 'Status changed from $oldStatus to $newStatus';
        }
        return 'Status was updated';
      case SupportEventType.priorityChanged:
        final oldPriority = metadata?['old_priority'] as String?;
        final newPriority = metadata?['new_priority'] as String?;
        if (oldPriority != null && newPriority != null) {
          return 'Priority changed from $oldPriority to $newPriority';
        }
        return 'Priority was updated';
      case SupportEventType.categoryChanged:
        final oldCategory = metadata?['old_category'] as String?;
        final newCategory = metadata?['new_category'] as String?;
        if (oldCategory != null && newCategory != null) {
          return 'Category changed from $oldCategory to $newCategory';
        }
        return 'Category was updated';
      case SupportEventType.ticketResolved:
        final notesText = notes != null && notes!.isNotEmpty
            ? '\nNotes: $notes'
            : '';
        return 'Ticket was resolved$notesText';
      case SupportEventType.ticketClosed:
        final reasonText = notes != null && notes!.isNotEmpty
            ? '\nReason: $notes'
            : '';
        return 'Ticket was closed$reasonText';
      case SupportEventType.ticketReopened:
        return 'Ticket was reopened';
      case SupportEventType.adminAssigned:
        return actorName != null
            ? '$actorName was assigned'
            : 'Admin was assigned';
      case SupportEventType.adminUnassigned:
        return 'Admin was unassigned';
      case SupportEventType.unknown:
        return notes ?? 'Event occurred';
    }
  }

  /// Helper: get icon for the event type
  String get iconName {
    switch (eventType) {
      case SupportEventType.ticketCreated:
        return 'add_circle_outline';
      case SupportEventType.ticketClaimed:
        return 'person_add';
      case SupportEventType.ticketWaitingUser:
        return 'hourglass_empty';
      case SupportEventType.statusChanged:
        return 'sync';
      case SupportEventType.priorityChanged:
        return 'flag';
      case SupportEventType.categoryChanged:
        return 'category';
      case SupportEventType.ticketResolved:
        return 'check_circle';
      case SupportEventType.ticketClosed:
        return 'close';
      case SupportEventType.ticketReopened:
        return 'restore';
      case SupportEventType.adminAssigned:
        return 'assignment_ind';
      case SupportEventType.adminUnassigned:
        return 'person_remove';
      case SupportEventType.unknown:
        return 'info';
    }
  }

  @override
  List<Object?> get props => [
    id,
    ticketId,
    eventType,
    actorId,
    actorName,
    oldStatus,
    newStatus,
    notes,
    metadata,
    createdAt,
  ];

  /// Parse event type from string
  static SupportEventType parseEventType(String value) {
    switch (value) {
      case 'ticket_created':
        return SupportEventType.ticketCreated;
      case 'ticket_claimed':
        return SupportEventType.ticketClaimed;
      case 'ticket_waiting_user':
        return SupportEventType.ticketWaitingUser;
      case 'status_changed':
        return SupportEventType.statusChanged;
      case 'priority_changed':
        return SupportEventType.priorityChanged;
      case 'category_changed':
        return SupportEventType.categoryChanged;
      case 'ticket_resolved':
        return SupportEventType.ticketResolved;
      case 'ticket_closed':
        return SupportEventType.ticketClosed;
      case 'ticket_reopened':
        return SupportEventType.ticketReopened;
      case 'admin_assigned':
        return SupportEventType.adminAssigned;
      case 'admin_unassigned':
        return SupportEventType.adminUnassigned;
      default:
        return SupportEventType.unknown;
    }
  }
}
