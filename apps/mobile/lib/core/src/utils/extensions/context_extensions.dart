import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

extension BuildContextExtensions on BuildContext {
  // Theme access
  ThemeData get theme => Theme.of(this);
  ColorScheme get colorScheme => theme.colorScheme;
  TextTheme get textTheme => theme.textTheme;

  // Screen dimensions
  Size get screenSize => MediaQuery.of(this).size;
  double get screenWidth => screenSize.width;
  double get screenHeight => screenSize.height;

  // Safe area
  EdgeInsets get padding => MediaQuery.of(this).padding;
  EdgeInsets get viewInsets => MediaQuery.of(this).viewInsets;
  EdgeInsets get viewPadding => MediaQuery.of(this).viewPadding;

  // Device info
  bool get isLandscape =>
      MediaQuery.of(this).orientation == Orientation.landscape;
  bool get isPortrait => !isLandscape;
  double get devicePixelRatio => MediaQuery.of(this).devicePixelRatio;

  // Responsive helpers
  bool get isSmallScreen => screenWidth < 600;
  bool get isMediumScreen => screenWidth >= 600 && screenWidth < 1200;
  bool get isLargeScreen => screenWidth >= 1200;

  // Navigation - Removed pop() method to avoid conflict with GoRouter extension
  // Use NavigationHandler from ServiceLocator for navigation instead

  void popUntil(bool Function(Route<dynamic>) predicate) {
    Navigator.of(this).popUntil(predicate);
  }

  void popToRoot() => Navigator.of(this).popUntil((route) => route.isFirst);

  // Snackbar
  void showSnackBar(
    String message, {
    Duration duration = const Duration(seconds: 3),
    SnackBarAction? action,
    Color? backgroundColor,
  }) {
    // Use AppSnackBar for consistency
    if (backgroundColor == colorScheme.error) {
      AppSnackBar.showError(this, message, duration: duration);
    } else if (backgroundColor == AppColors.success) {
      AppSnackBar.showSuccess(this, message, duration: duration);
    } else {
      AppSnackBar.showInfo(this, message, duration: duration);
    }
  }

  void showErrorSnackBar(String message) {
    AppSnackBar.showError(this, message);
  }

  void showSuccessSnackBar(String message) {
    AppSnackBar.showSuccess(this, message);
  }

  // Dialog
  Future<T?> showAlertDialog<T>({
    required String title,
    required String content,
    String confirmText = 'OK',
    String? cancelText,
    VoidCallback? onConfirm,
    VoidCallback? onCancel,
  }) {
    return showDialog<T>(
      context: this,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: Text(content),
        actions: [
          if (cancelText != null)
            TextButton(
              onPressed: () {
                Navigator.of(context).pop();
                onCancel?.call();
              },
              child: Text(cancelText),
            ),
          TextButton(
            onPressed: () {
              Navigator.of(context).pop();
              onConfirm?.call();
            },
            child: Text(confirmText),
          ),
        ],
      ),
    );
  }

  Future<bool?> showConfirmDialog({
    required String title,
    required String content,
    String confirmText = 'Ya',
    String cancelText = 'Tidak',
  }) {
    return showDialog<bool>(
      context: this,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: Text(content),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(cancelText),
          ),
          TextButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(confirmText),
          ),
        ],
      ),
    );
  }

  // Loading indicator
  void showLoadingDialog() {
    showDialog(
      context: this,
      barrierDismissible: false,
      builder: (context) => const Center(child: CircularProgressIndicator()),
    );
  }

  void hideLoadingDialog() {
    if (Navigator.of(this).canPop()) {
      Navigator.of(this).pop();
    }
  }

  // Focus
  void unfocus() => FocusScope.of(this).unfocus();

  // Keyboard
  bool get isKeyboardVisible => viewInsets.bottom > 0;
}
