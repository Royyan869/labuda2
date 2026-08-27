/// Dropdown State Builders
///
/// Helper widgets untuk menampilkan berbagai state dropdown
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Helper class untuk membuat state builders pada dropdown wilayah
class DropdownStateBuilders {
  const DropdownStateBuilders._();

  /// Build disabled dropdown state
  static Widget buildDisabled({
    required BuildContext context,
    required bool isDark,
    required String text,
    IconData? prefixIcon,
  }) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(
              prefixIcon,
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray400,
            ),
            const SizedBox(width: 12),
          ],
          Text(
            text,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray400,
              fontSize: 16,
            ),
          ),
        ],
      ),
    );
  }

  /// Build empty dropdown state
  static Widget buildEmpty({
    required BuildContext context,
    required bool isDark,
    required String text,
    IconData? prefixIcon,
  }) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(
              prefixIcon,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            const SizedBox(width: 12),
          ],
          Text(
            text,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
              fontSize: 16,
            ),
          ),
        ],
      ),
    );
  }

  /// Build loading dropdown state
  static Widget buildLoading({
    required BuildContext context,
    required bool isDark,
    required String text,
    IconData? prefixIcon,
  }) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(
              prefixIcon,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            const SizedBox(width: 12),
          ],
          const SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          const SizedBox(width: 12),
          Text(
            text,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }

  /// Build error dropdown state
  static Widget buildError({
    required BuildContext context,
    required bool isDark,
    required String text,
    IconData? prefixIcon,
  }) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: AppColors.statusError),
            const SizedBox(width: 12),
          ],
          Icon(Icons.error_outline, color: AppColors.statusError, size: 20),
          const SizedBox(width: 8),
          Text(text, style: TextStyle(color: AppColors.statusError)),
        ],
      ),
    );
  }
}
