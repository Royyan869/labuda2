import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Reusable SnackBar component dengan styling konsisten
///
/// Features:
/// - Success (hijau), Error (merah), Info (biru), Warning (orange)
/// - Floating style dengan rounded corners
/// - Icon otomatis sesuai type
/// - Duration customizable
/// - Consistent spacing dan typography
class AppSnackBar {
  /// Show success snackbar (hijau)
  static void showSuccess(
    BuildContext context,
    String message, {
    Duration duration = const Duration(seconds: 3),
  }) {
    _show(
      context,
      message: message,
      type: AppSnackBarType.success,
      duration: duration,
    );
  }

  /// Show error snackbar (merah)
  static void showError(
    BuildContext context,
    String message, {
    Duration duration = const Duration(seconds: 4),
  }) {
    _show(
      context,
      message: message,
      type: AppSnackBarType.error,
      duration: duration,
    );
  }

  /// Show info snackbar (biru)
  static void showInfo(
    BuildContext context,
    String message, {
    Duration duration = const Duration(seconds: 3),
  }) {
    _show(
      context,
      message: message,
      type: AppSnackBarType.info,
      duration: duration,
    );
  }

  /// Show warning snackbar (orange)
  static void showWarning(
    BuildContext context,
    String message, {
    Duration duration = const Duration(seconds: 3),
  }) {
    _show(
      context,
      message: message,
      type: AppSnackBarType.warning,
      duration: duration,
    );
  }

  /// Internal method untuk show snackbar - DISABLED TEMPORARILY
  static void _show(
    BuildContext context, {
    required String message,
    required AppSnackBarType type,
    Duration duration = const Duration(seconds: 3),
  }) {
    // Clear existing snackbar
    ScaffoldMessenger.of(context).clearSnackBars();

    final config = _getTypeConfig(context, 
      type,
      Theme.of(context).colorScheme,
    );

    // Calculate safe bottom margin that works with bottom navigation
    final mediaQuery = MediaQuery.of(context);
    final bottomInset = mediaQuery.viewInsets.bottom;
    final bottomPadding = mediaQuery.padding.bottom;
    // Use a fixed margin above bottom navigation (~90px for bottom nav + ~16px spacing)
    final bottomMargin = bottomInset > 0
        ? bottomInset + 16
        : 106 + bottomPadding;

    final scheme = Theme.of(context).colorScheme;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Row(
          children: [
            Icon(config.icon, color: scheme.onPrimary, size: 20),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                message,
                style: TextStyle(
                  color: scheme.onPrimary,
                  fontWeight: FontWeight.w500,
                  fontSize: AppType.s14,
                ),
              ),
            ),
          ],
        ),
        backgroundColor: config.color,
        behavior: SnackBarBehavior.floating,
        margin: EdgeInsets.only(bottom: bottomMargin, left: AppMetrics.p16, right: AppMetrics.p16),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppShape.r12)),
        duration: duration,
        elevation: AppElevation.snackBar,
        action: duration.inSeconds > 3
            ? SnackBarAction(
                label: 'Close',
                textColor: scheme.onPrimary.withValues(alpha: 0.7),
                onPressed: () {
                  ScaffoldMessenger.of(context).hideCurrentSnackBar();
                },
              )
            : null,
      ),
    );
  }

  /// Get configuration berdasarkan type
  static _SnackBarConfig _getTypeConfig(BuildContext context,
    AppSnackBarType type,
    ColorScheme scheme,
  ) {
    switch (type) {
      case AppSnackBarType.success:
        return _SnackBarConfig(
          color: context.statusColors.success,
          icon: Icons.check_circle_outline,
        );
      case AppSnackBarType.error:
        return _SnackBarConfig(
          color: context.statusColors.error,
          icon: Icons.error_outline,
        );
      case AppSnackBarType.info:
        return _SnackBarConfig(
          // Same canonical red value via the scheme role (info uses the
          // brand red by product convention — hue unchanged).
          color: scheme.primary,
          icon: Icons.info_outline,
        );
      case AppSnackBarType.warning:
        return _SnackBarConfig(
          color: context.statusColors.warning,
          icon: Icons.warning_amber_outlined,
        );
    }
  }
}

/// Types untuk snackbar
enum AppSnackBarType { success, error, info, warning }

/// Internal config untuk snackbar
class _SnackBarConfig {
  final Color color;
  final IconData icon;

  const _SnackBarConfig({required this.color, required this.icon});
}
