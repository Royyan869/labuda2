import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';

/// THE canonical Snackbar authority.
///
/// Every transient semantic toast in Labuda is rendered here and nowhere else.
/// There is exactly ONE visual model (floating, status-colored, icon + body
/// ink) and exactly FOUR semantic types ([AppSnackBarType]). Callers choose a
/// type; they never choose a colour, radius, icon, or foreground.
///
/// Foundation consumed:
/// - background/foreground: [AppStatusColors] (`context.statusColors`)
/// - geometry: [AppMetrics] / [AppShape] / [AppElevation] / [AppIconSize]
/// - typography: `context.typeRoles.bodyDense`
///
/// Actions are intentional: there is NO automatic "Close" action. A caller may
/// pass an [AppSnackBarAction] only when the interaction genuinely needs one.
class AppSnackBar {
  AppSnackBar._();

  /// Canonical type → duration defaults.
  static const Duration _shortDuration = Duration(seconds: 4);
  static const Duration _errorDuration = Duration(seconds: 6);

  /// Success (4s).
  static void showSuccess(
    BuildContext context,
    String message, {
    Duration duration = _shortDuration,
    AppSnackBarAction? action,
  }) => _show(
    context,
    message: message,
    type: AppSnackBarType.success,
    duration: duration,
    action: action,
  );

  /// Error (6s — the longest, because errors are the most important and the
  /// most likely to be the only channel for a failure).
  static void showError(
    BuildContext context,
    String message, {
    Duration duration = _errorDuration,
    AppSnackBarAction? action,
  }) => _show(
    context,
    message: message,
    type: AppSnackBarType.error,
    duration: duration,
    action: action,
  );

  /// Warning (4s).
  static void showWarning(
    BuildContext context,
    String message, {
    Duration duration = _shortDuration,
    AppSnackBarAction? action,
  }) => _show(
    context,
    message: message,
    type: AppSnackBarType.warning,
    duration: duration,
    action: action,
  );

  /// Informational (4s).
  static void showInfo(
    BuildContext context,
    String message, {
    Duration duration = _shortDuration,
    AppSnackBarAction? action,
  }) => _show(
    context,
    message: message,
    type: AppSnackBarType.info,
    duration: duration,
    action: action,
  );

  /// The ONLY renderer. One visible toast at a time: the previous toast is
  /// cleared before a new one is shown.
  static void _show(
    BuildContext context, {
    required String message,
    required AppSnackBarType type,
    required Duration duration,
    AppSnackBarAction? action,
  }) {
    ScaffoldMessenger.of(context).clearSnackBars();

    final config = _getTypeConfig(context, type);

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Row(
          children: [
            Icon(
              config.icon,
              color: config.foreground,
              size: AppIconSize.action,
            ),
            const SizedBox(width: AppMetrics.p12),
            Expanded(
              child: Text(
                message,
                style: context.typeRoles.bodyDense.copyWith(
                  color: config.foreground,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ),
          ],
        ),
        backgroundColor: config.background,
        behavior: SnackBarBehavior.floating,
        // The Scaffold owns system/bottom-UI positioning; this margin is
        // visual spacing only — never an inset calculation.
        margin: const EdgeInsets.only(
          bottom: AppMetrics.p16,
          left: AppMetrics.p16,
          right: AppMetrics.p16,
        ),
        padding: AppMetrics.inputPadding,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppShape.r12),
        ),
        duration: duration,
        elevation: AppElevation.snackBar,
        action: action == null
            ? null
            : SnackBarAction(
                label: action.label,
                textColor: config.foreground,
                onPressed: action.onPressed,
              ),
      ),
    );
  }

  /// Type → canonical background/foreground/icon. The foreground is the
  /// contrast-safe on-status role, never a blanket white.
  static _SnackBarConfig _getTypeConfig(
    BuildContext context,
    AppSnackBarType type,
  ) {
    final status = context.statusColors;
    switch (type) {
      case AppSnackBarType.success:
        return _SnackBarConfig(
          background: status.success,
          foreground: status.onSuccess,
          icon: Icons.check_circle_outline,
        );
      case AppSnackBarType.error:
        return _SnackBarConfig(
          background: status.error,
          foreground: status.onError,
          icon: Icons.error_outline,
        );
      case AppSnackBarType.warning:
        return _SnackBarConfig(
          background: status.warning,
          foreground: status.onWarning,
          icon: Icons.warning_amber_outlined,
        );
      case AppSnackBarType.info:
        return _SnackBarConfig(
          background: status.info,
          foreground: status.onInfo,
          icon: Icons.info_outline,
        );
    }
  }
}

/// The FOUR canonical Snackbar semantic types. No fifth type may be added —
/// "action required" is an action on a type, not a type; "coming soon" is not
/// a semantic at all.
enum AppSnackBarType { success, error, warning, info }

/// An intentional Snackbar action. Only provide one when the interaction
/// genuinely needs it.
class AppSnackBarAction {
  const AppSnackBarAction({required this.label, required this.onPressed});

  final String label;
  final VoidCallback onPressed;
}

class _SnackBarConfig {
  const _SnackBarConfig({
    required this.background,
    required this.foreground,
    required this.icon,
  });

  final Color background;
  final Color foreground;
  final IconData icon;
}
