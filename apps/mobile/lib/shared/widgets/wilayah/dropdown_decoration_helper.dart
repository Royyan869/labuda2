/// Dropdown Decoration Helper
///
/// Helper untuk membuat decoration dan style dropdown yang konsisten
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Helper class untuk decoration dropdown
class DropdownDecorationHelper {
  const DropdownDecorationHelper._();

  /// Create InputDecoration untuk dropdown
  static InputDecoration createInputDecoration({
    required bool isDark,
    required String hintText,
    IconData? prefixIcon,
  }) {
    return InputDecoration(
      hintText: hintText,
      prefixIcon: prefixIcon != null
          ? Icon(
              prefixIcon,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            )
          : null,
      border: InputBorder.none,
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      hintStyle: TextStyle(
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
      ),
    );
  }

  /// Get dropdown color based on theme
  static Color getDropdownColor(bool isDark) {
    return isDark ? AppColors.darkGray700 : AppColors.neutralWhite;
  }

  /// Get text style untuk dropdown
  static TextStyle getTextStyle(bool isDark) {
    return TextStyle(
      color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900,
      fontSize: 16,
    );
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
