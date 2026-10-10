/// Dropdown Decoration Helper
///
/// Helper untuk membuat decoration dan style dropdown yang konsisten
library;

import 'package:flutter/material.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Helper class untuk decoration dropdown
class DropdownDecorationHelper {
  const DropdownDecorationHelper._();

  /// Create InputDecoration untuk dropdown
  static InputDecoration createInputDecoration({
    required ColorScheme scheme,
    required String hintText,
    IconData? prefixIcon,
  }) {
    return InputDecoration(
      hintText: hintText,
      prefixIcon: prefixIcon != null
          ? Icon(prefixIcon, color: scheme.onSurfaceVariant)
          : null,
      border: InputBorder.none,
      contentPadding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p16,
      ),
      hintStyle: TextStyle(color: scheme.onSurfaceVariant),
    );
  }

  /// Get dropdown menu background from the scheme
  static Color getDropdownColor(ColorScheme scheme) {
    return scheme.surfaceContainerHigh;
  }

  /// Get text style untuk dropdown
  static TextStyle getTextStyle(ColorScheme scheme) {
    return TextStyle(color: scheme.onSurface);
  }

  /// Create selected item builder untuk dropdown
  static List<Widget> buildSelectedItems<T>(
    List<T> items,
    String Function(T) nameGetter,
  ) {
    return items.map((item) {
      return Text(
        nameGetter(item),
        overflow: TextOverflow.ellipsis,
        maxLines: 1,
      );
    }).toList();
  }

  /// Create dropdown menu items
  static List<DropdownMenuItem<T>> buildDropdownItems<T>(
    List<T> items,
    String Function(T) nameGetter,
  ) {
    return items.map((item) {
      return DropdownMenuItem<T>(
        value: item,
        child: Text(
          nameGetter(item),
          overflow: TextOverflow.ellipsis,
          maxLines: 1,
        ),
      );
    }).toList();
  }
}
