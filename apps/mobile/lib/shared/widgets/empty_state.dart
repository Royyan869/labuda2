import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Empty State Widget
///
/// Reusable empty state component with consistent styling.
/// Supports different types of empty states with icons and messages.
enum EmptyStateType {
  noData,
  noResults,
  noItems,
  noNotifications,
  noMessages,
  noFavorites,
  error,
  loading,
  custom,
}

class EmptyState extends StatelessWidget {
  final String title;
  final String? subtitle;
  final IconData? icon;
  final EmptyStateType type;
  final VoidCallback? onRetry;
  final Widget? customIcon;
  final String? actionLabel;
  final VoidCallback? onAction;
  final bool showIcon;

  const EmptyState({
    super.key,
    required this.title,
    this.subtitle,
    this.icon,
    this.type = EmptyStateType.noData,
    this.onRetry,
    this.customIcon,
    this.actionLabel,
    this.onAction,
    this.showIcon = true,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Padding(
      padding: const EdgeInsets.all(48),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          // Icon
          if (showIcon) ...[
            _buildIcon(context),
            const SizedBox(height: 24),
          ],
          // Title
          Text(
            title,
            style: AppTypography.h4.copyWith(color: scheme.onSurface),
            textAlign: TextAlign.center,
          ),
          // Subtitle
          if (subtitle != null) ...[
            const SizedBox(height: 12),
            Text(
              subtitle!,
              style: AppTypography.bodyMedium.copyWith(
                color: scheme.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
          ],
          // Action button (retry or custom action)
          if (onRetry != null || onAction != null) ...[
            const SizedBox(height: 32),
            if (onRetry != null)
              ElevatedButton.icon(
                icon: const Icon(Icons.refresh),
                label: const Text('Try Again'),
                onPressed: onRetry,
              )
            else if (onAction != null && actionLabel != null)
              FilledButton(onPressed: onAction, child: Text(actionLabel!)),
          ],
        ],
      ),
    );
  }

  Widget _buildIcon(BuildContext context) {
    if (customIcon != null) {
      return SizedBox(width: 80, height: 80, child: customIcon!);
    }

    // One rule: error type uses the error role, everything else the
    // neutral variant ink. The old per-type switch produced the same gray
    // in 8 branches — duplicate authority, killed.
    final scheme = Theme.of(context).colorScheme;
    final IconData iconData =
        icon ??
        const {
          EmptyStateType.noData: Icons.inbox_outlined,
          EmptyStateType.noResults: Icons.search_off_outlined,
          EmptyStateType.noItems: Icons.inventory_2_outlined,
          EmptyStateType.noNotifications: Icons.notifications_none_outlined,
          EmptyStateType.noMessages: Icons.message_outlined,
          EmptyStateType.noFavorites: Icons.favorite_border,
          EmptyStateType.error: Icons.error_outline,
          EmptyStateType.loading: Icons.hourglass_empty_outlined,
          EmptyStateType.custom: Icons.inbox_outlined,
        }[type]!;
    final Color iconColor = type == EmptyStateType.error
        ? scheme.error
        : scheme.onSurfaceVariant;

    return Container(
      width: 80,
      height: 80,
      decoration: BoxDecoration(
        color: iconColor.withValues(alpha: 0.1),
        shape: BoxShape.circle,
      ),
      child: Icon(iconData, size: 40, color: iconColor),
    );
  }

  // Named constructors for common empty states
  factory EmptyState.noData({
    required String title,
    String? subtitle,
    VoidCallback? onRetry,
  }) {
    return EmptyState(
      title: title,
      subtitle: subtitle,
      type: EmptyStateType.noData,
      onRetry: onRetry,
    );
  }

  factory EmptyState.noResults({
    required String title,
    String? subtitle,
    VoidCallback? onRetry,
  }) {
    return EmptyState(
      title: title,
      subtitle: subtitle,
      type: EmptyStateType.noResults,
      onRetry: onRetry,
    );
  }

  factory EmptyState.noNotifications({
    required String title,
    String? subtitle,
    VoidCallback? onRetry,
  }) {
    return EmptyState(
      title: title,
      subtitle: subtitle,
      type: EmptyStateType.noNotifications,
      onRetry: onRetry,
    );
  }

  factory EmptyState.noMessages({
    required String title,
    String? subtitle,
    VoidCallback? onRetry,
  }) {
    return EmptyState(
      title: title,
      subtitle: subtitle,
      type: EmptyStateType.noMessages,
      onRetry: onRetry,
    );
  }

  factory EmptyState.error({
    required String title,
    String? subtitle,
    VoidCallback? onRetry,
  }) {
    return EmptyState(
      title: title,
      subtitle: subtitle,
      type: EmptyStateType.error,
      onRetry: onRetry,
    );
  }

  factory EmptyState.loading({required String title, String? subtitle}) {
    return EmptyState(
      title: title,
      subtitle: subtitle,
      type: EmptyStateType.loading,
      showIcon: false,
    );
  }
}
