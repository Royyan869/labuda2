import 'package:labuda/domains/system/support/domain/domain.dart';
import 'package:labuda/generated/app_localizations.dart';

/// I18N-15 — the ONE canonical Support Status display-label authority.
///
/// This extension holds no copy of its own: it maps each canonical
/// [SupportStatus] to its resource in the canonical `AppLocalizations`
/// authority, so the rendered label always follows the active app locale.
///
/// Status *identity* stays `SupportStatus.wireValue` (the lifecycle / API
/// contract); a localized label is display-only and must never be used for
/// persistence, filtering identity, sorting, analytics, routing, state
/// transition, or the API payload.
extension SupportStatusLabel on SupportStatus {
  String label(AppLocalizations l10n) => switch (this) {
    SupportStatus.open => l10n.supportStatusOpen,
    SupportStatus.inProgress => l10n.supportStatusInProgress,
    SupportStatus.waitingUser => l10n.supportStatusWaitingUser,
    SupportStatus.resolved => l10n.supportStatusResolved,
    SupportStatus.closed => l10n.supportStatusClosed,
  };
}
