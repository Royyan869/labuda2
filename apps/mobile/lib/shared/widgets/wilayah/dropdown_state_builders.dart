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
    required String text,
    IconData? prefixIcon,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: scheme.onSurfaceVariant),
            const SizedBox(width: 12),
          ],
          Text(
            text,
            style: TextStyle(
              color: scheme.onSurfaceVariant,
              fontSize: AppType.s16,
            ),
          ),
        ],
      ),
    );
  }

  /// Build empty dropdown state
  static Widget buildEmpty({
    required BuildContext context,
    required String text,
    IconData? prefixIcon,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: scheme.onSurfaceVariant),
            const SizedBox(width: 12),
          ],
          Text(
            text,
            style: TextStyle(
              color: scheme.onSurfaceVariant,
              fontSize: AppType.s16,
            ),
          ),
        ],
      ),
    );
  }

  /// Build loading dropdown state
  static Widget buildLoading({
    required BuildContext context,
    required String text,
    IconData? prefixIcon,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: scheme.onSurfaceVariant),
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
            style: TextStyle(color: scheme.onSurfaceVariant),
          ),
        ],
      ),
    );
  }

  /// Build error dropdown state
  static Widget buildError({
    required BuildContext context,
    required String text,
    IconData? prefixIcon,
  }) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: context.statusColors.error),
            const SizedBox(width: 12),
          ],
          Icon(Icons.error_outline, color: context.statusColors.error, size: 20),
          const SizedBox(width: 8),
          Text(text, style: TextStyle(color: context.statusColors.error)),
        ],
      ),
    );
  }
}
