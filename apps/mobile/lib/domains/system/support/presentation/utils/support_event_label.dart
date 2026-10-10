import 'package:hishumi/domains/system/support/domain/domain.dart';
import 'package:hishumi/domains/system/support/presentation/utils/support_category_label.dart';
import 'package:hishumi/domains/system/support/presentation/utils/support_priority_label.dart';
import 'package:hishumi/domains/system/support/presentation/utils/support_status_label.dart';
import 'package:hishumi/generated/app_localizations.dart';

/// I18N-18 — the ONE canonical Support Event display authority.
///
/// This file holds no copy of its own: it maps each canonical
/// [SupportEventType] to its resource in the canonical `AppLocalizations`
/// authority, so a rendered activity label always follows the active app
/// locale.
///
/// Event *identity* stays the wire value parsed by
/// [SupportEvent.parseEventType] (the backend `event_type` contract); a
/// localized label is display-only and must never be used for persistence,
/// filtering identity, analytics, routing, or the API payload.
///
/// Transition detail is derived from the event's own canonical identities
/// (`old_status`/`new_status`, `old_priority`/`new_priority`,
/// `old_category`/`new_category`) through the already-converged Category,
/// Status and Priority label authorities — a raw wire value is never rendered.
extension SupportEventTypeLabel on SupportEventType {
  String label(AppLocalizations l10n) => switch (this) {
    SupportEventType.ticketCreated => l10n.supportEventTicketCreated,
    SupportEventType.ticketClaimed => l10n.supportEventTicketClaimed,
    SupportEventType.ticketWaitingUser => l10n.supportEventTicketWaitingUser,
    SupportEventType.statusChanged => l10n.supportEventStatusChanged,
    SupportEventType.priorityChanged => l10n.supportEventPriorityChanged,
    SupportEventType.categoryChanged => l10n.supportEventCategoryChanged,
    SupportEventType.ticketResolved => l10n.supportEventTicketResolved,
    SupportEventType.ticketClosed => l10n.supportEventTicketClosed,
    SupportEventType.ticketReopened => l10n.supportEventTicketReopened,
    SupportEventType.adminAssigned => l10n.supportEventAdminAssigned,
    SupportEventType.adminUnassigned => l10n.supportEventAdminUnassigned,
    SupportEventType.ticketEscalated => l10n.supportEventTicketEscalated,
    SupportEventType.unknown => l10n.supportEventUnknown,
  };
}

/// Localized secondary line for an activity entry.
///
/// Returns `null` when the event carries no canonical transition we can state
/// truthfully — a missing or unrecognised identity is simply omitted rather
/// than rendered raw. Free-text `notes` are NOT surfaced: they may carry
/// internal agent copy.
extension SupportEventDisplay on SupportEvent {
  String? detail(AppLocalizations l10n) {
    switch (eventType) {
      case SupportEventType.statusChanged:
      case SupportEventType.ticketWaitingUser:
      case SupportEventType.ticketReopened:
        return _statusTransition(l10n);
      case SupportEventType.priorityChanged:
        return _priorityTransition(l10n);
      case SupportEventType.categoryChanged:
        return _categoryTransition(l10n);
      case SupportEventType.ticketCreated:
      case SupportEventType.ticketClaimed:
      case SupportEventType.ticketResolved:
      case SupportEventType.ticketClosed:
      case SupportEventType.adminAssigned:
      case SupportEventType.adminUnassigned:
      case SupportEventType.ticketEscalated:
      case SupportEventType.unknown:
        return null;
    }
  }

  String? _statusTransition(AppLocalizations l10n) {
    final fromValue = oldStatus;
    final toValue = newStatus;
    final from = fromValue == null ? null : SupportStatus.fromWire(fromValue);
    final to = toValue == null ? null : SupportStatus.fromWire(toValue);
    if (from == null || to == null) return null;
    return l10n.supportEventTransition(from.label(l10n), to.label(l10n));
  }

  String? _priorityTransition(AppLocalizations l10n) {
    final from = _priorityFromWire(metadata?['old_priority']);
    final to = _priorityFromWire(metadata?['new_priority']);
    if (from == null || to == null) return null;
    return l10n.supportEventTransition(from.label(l10n), to.label(l10n));
  }

  String? _categoryTransition(AppLocalizations l10n) {
    final fromRaw = metadata?['old_category'];
    final toRaw = metadata?['new_category'];
    final from = fromRaw is String ? SupportCategory.fromWire(fromRaw) : null;
    final to = toRaw is String ? SupportCategory.fromWire(toRaw) : null;
    if (from == null || to == null) return null;
    return l10n.supportEventTransition(from.label(l10n), to.label(l10n));
  }
}

/// Priority identity is the enum itself (`SupportPriority.name`), so the raw
/// metadata token is resolved back to the canonical enum before labelling.
SupportPriority? _priorityFromWire(Object? value) {
  if (value is! String) return null;
  for (final priority in SupportPriority.values) {
    if (priority.name == value) return priority;
  }
  return null;
}
