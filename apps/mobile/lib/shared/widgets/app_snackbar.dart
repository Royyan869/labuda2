import 'package:labuda/core/src/theme/app_colors.dart';
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

    final config = _getTypeConfig(type);

    // Calculate safe bottom margin that works with bottom navigation
    final mediaQuery = MediaQuery.of(context);
    final bottomInset = mediaQuery.viewInsets.bottom;
    final bottomPadding = mediaQuery.padding.bottom;
    // Use a fixed margin above bottom navigation (~90px for bottom nav + ~16px spacing)
    final bottomMargin = bottomInset > 0
        ? bottomInset + 16
        : 106 + bottomPadding;

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Row(
          children: [
            Icon(config.icon, color: AppColors.light, size: 20),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                message,
                style: const TextStyle(
                  color: AppColors.light,
                  fontWeight: FontWeight.w500,
                  fontSize: 14,
                ),
              ),
            ),
          ],
        ),
        backgroundColor: config.color,
        behavior: SnackBarBehavior.floating,
        margin: EdgeInsets.only(bottom: bottomMargin, left: 16, right: 16),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        duration: duration,
        elevation: 6,
        action: duration.inSeconds > 3
            ? SnackBarAction(
                label: 'Close',
                textColor: Colors.white70,
                onPressed: () {
                  ScaffoldMessenger.of(context).hideCurrentSnackBar();
                },
              )
            : null,
      ),
    );
  }

  /// Get configuration berdasarkan type
  static _SnackBarConfig _getTypeConfig(AppSnackBarType type) {
    switch (type) {
      case AppSnackBarType.success:
        return _SnackBarConfig(
          color: AppColors.statusSuccess,
          icon: Icons.check_circle_outline,
        );
      case AppSnackBarType.error:
        return _SnackBarConfig(
          color: AppColors.statusError,
          icon: Icons.error_outline,
        );
      case AppSnackBarType.info:
        return _SnackBarConfig(
          color: AppColors.primaryRed,
          icon: Icons.info_outline,
        );
      case AppSnackBarType.warning:
        return _SnackBarConfig(
          color: AppColors.statusWarning,
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
