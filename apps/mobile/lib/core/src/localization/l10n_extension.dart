import 'package:flutter/material.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Extension on BuildContext to provide easy access to AppLocalizations.
///
/// Usage:
/// ```dart
/// // Instead of:
/// final l10n = AppLocalizations.of(context)!;
/// Text(l10n.save);
///
/// // You can use:
/// Text(context.l10n.save);
/// ```
extension L10nExtension on BuildContext {
  /// Get the AppLocalizations instance from the current context.
  AppLocalizations get l10n => AppLocalizations.of(this)!;
}
