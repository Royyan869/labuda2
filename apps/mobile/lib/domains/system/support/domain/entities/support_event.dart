/// Support Ticket Event Entity
/// Represents a canonical read-only audit trail entry for ticket state changes
library;

import 'package:equatable/equatable.dart';

/// Support Event Type - type of event that occurred on a ticket.
///
/// IDENTITY: every value maps 1:1 to the backend `support_ticket_events.
/// event_type` wire value through [SupportEvent.parseEventType]. Event display
/// copy is NOT owned here — it lives in the canonical `AppLocalizations`
/// authority behind the presentation-layer resolver.
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
  ticketEscalated, // Ticket was escalated to dispute
  unknown, // Unrecognised event type
}

/// Support Event - canonical read-only audit-trail entry for a ticket.
///
/// Pure data: event identity ([eventType]), canonical transition values
/// ([oldStatus]/[newStatus] and [metadata]) and the timestamp. It stores no
/// user-facing copy — the presentation layer derives every label from
/// `AppLocalizations` so the activity timeline follows the active locale and
/// never surfaces a raw wire value.
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

  /// Parse the canonical backend `event_type` wire value into an event type.
  ///
  /// This is the SINGLE event-type parser for the Support domain; the API DTO
  /// delegates to it rather than keeping a second, drift-prone switch.
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
      case 'ticket_escalated':
        return SupportEventType.ticketEscalated;
      default:
        return SupportEventType.unknown;
    }
  }
}
