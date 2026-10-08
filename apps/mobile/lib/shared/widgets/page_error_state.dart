import 'dart:async';

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// CANONICAL page-level load-error surface.
///
/// SEMANTIC OWNERSHIP (owner-locked):
/// - Error = a load/request FAILED. Never Empty (request succeeded with zero
///   data) and Never Loading. Empty stays in [EmptyState]; this widget must
///   never be used to render "no data".
///
/// SCOPE: full-page / page-body load failures only. Not for inline field
/// validation, form errors, snackbar, dialog, section-level error, commerce
/// not-found, unavailable-item badges, router errors, or auth/permission
/// full-page handling — those keep their own semantic owners.
///
/// SAFETY CONTRACT: the widget takes NO technical error input. Copy comes
/// only from localization ([AppLocalizations.pageErrorTitle],
/// [AppLocalizations.pageErrorMessage], [AppLocalizations.retryAction]), so a
/// raw exception, Dio error, backend message, or debug detail can never reach
/// the screen through this surface. Technical detail belongs in logging only.
///
/// RETRY CONTRACT: while the [onRetry] future is in flight the retry action
/// stays disabled, repeated taps cannot start a concurrent retry, and the
/// action re-enables when the callback settles. No artificial timers.
class PageErrorState extends StatefulWidget {
  const PageErrorState({super.key, this.onRetry});

  /// Called on retry tap. `null` renders the state without a retry action.
  /// A sync callback is allowed; the action is re-enabled once it returns.
  final FutureOr<void> Function()? onRetry;

  @override
  State<PageErrorState> createState() => _PageErrorStateState();
}

class _PageErrorStateState extends State<PageErrorState> {
  bool _retryInFlight = false;

  Future<void> _handleRetry() async {
    final onRetry = widget.onRetry;
    if (onRetry == null || _retryInFlight) return;
    setState(() => _retryInFlight = true);
    try {
      await onRetry();
    } finally {
      if (mounted) {
        setState(() => _retryInFlight = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = context.l10n;
    final scheme = Theme.of(context).colorScheme;

    return Center(
      // Basic accessibility: one live container so a screen reader announces
      // the state when it appears; title/message read as text, the retry
      // action reads as a labeled button.
      child: Semantics(
        container: true,
        liveRegion: true,
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.error_outline,
                size: AppIconSize.display,
                color: scheme.error,
              ),
              const SizedBox(height: 16),
              Text(
                l10n.pageErrorTitle,
                style: context.typeRoles.titleProminent.copyWith(
                  fontWeight: FontWeight.w600,
                  color: scheme.onSurface,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                l10n.pageErrorMessage,
                style: context.typeRoles.bodyDense.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
                textAlign: TextAlign.center,
              ),
              if (widget.onRetry != null) ...[
                const SizedBox(height: 24),
                ElevatedButton.icon(
                  onPressed: _retryInFlight ? null : _handleRetry,
                  icon: const Icon(Icons.refresh, size: AppIconSize.action),
                  label: Text(l10n.retryAction),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
