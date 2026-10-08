import 'package:flutter/material.dart';

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

  // Navigation helpers are NOT hosted here: GoRouter's own `context.pop()`
  // extension is the ONE pop authority. The former `popUntil` / `popToRoot`
  // wrappers had zero consumers and are purged.

  // Snackbar presentation lives in its ONE canonical widget authority
  // (`shared/widgets/app_snackbar.dart`). The former convenience methods here
  // had no colour decision of their own and were purged — callers pick a
  // semantic type on that authority directly.

  // Dialog presentation lives in the canonical `AppDialog` authority
  // (`shared/widgets/app_dialog.dart`). The former competing extension helpers
  // here had ZERO consumers and are purged — a second dialog authority is
  // exactly the split this file is not allowed to host.

  // Focus
  void unfocus() => FocusScope.of(this).unfocus();

  // Keyboard
  bool get isKeyboardVisible => viewInsets.bottom > 0;
}
