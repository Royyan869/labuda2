import 'package:hishumi/domains/system/support/domain/domain.dart';
import 'package:hishumi/generated/app_localizations.dart';

/// I18N-16 — the ONE canonical Support Priority display-label authority.
///
/// This extension holds no copy of its own: it maps each canonical
/// [SupportPriority] to its resource in the canonical `AppLocalizations`
/// authority, so the rendered label always follows the active app locale.
///
/// Priority *identity* stays the enum itself (`SupportPriority.name` — the
/// backend / API contract, validated by `oneof=low medium high urgent`); a
/// localized label is display-only and must never be used for persistence,
/// filtering identity, sorting, analytics, routing, or the API payload.
extension SupportPriorityLabel on SupportPriority {
  String label(AppLocalizations l10n) => switch (this) {
    SupportPriority.low => l10n.supportPriorityLow,
    SupportPriority.medium => l10n.supportPriorityMedium,
    SupportPriority.high => l10n.supportPriorityHigh,
    SupportPriority.urgent => l10n.supportPriorityUrgent,
  };
}
