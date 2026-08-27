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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Padding(
      padding: const EdgeInsets.all(48),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          // Icon
          if (showIcon) ...[
            _buildIcon(context, isDark),
            const SizedBox(height: 24),
          ],
          // Title
          Text(
            title,
            style: AppTypography.h4.copyWith(
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
            textAlign: TextAlign.center,
          ),
          // Subtitle
          if (subtitle != null) ...[
            const SizedBox(height: 12),
            Text(
              subtitle!,
              style: AppTypography.bodyMedium.copyWith(
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
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

  Widget _buildIcon(BuildContext context, bool isDark) {
    if (customIcon != null) {
      return SizedBox(width: 80, height: 80, child: customIcon!);
    }

    IconData? iconData = icon;
    Color iconColor = isDark
        ? AppColors.neutralGray500
        : AppColors.neutralGray400;

    switch (type) {
      case EmptyStateType.noData:
        iconData = icon ?? Icons.inbox_outlined;
        iconColor = isDark
            ? AppColors.neutralGray500
            : AppColors.neutralGray400;
        break;
      case EmptyStateType.noResults:
        iconData = icon ?? Icons.search_off_outlined;
        iconColor = isDark
            ? AppColors.neutralGray500
            : AppColors.neutralGray400;
        break;
      case EmptyStateType.noItems:
        iconData = icon ?? Icons.inventory_2_outlined;
        iconColor = isDark
            ? AppColors.neutralGray500
            : AppColors.neutralGray400;
        break;
      case EmptyStateType.noNotifications:
        iconData = icon ?? Icons.notifications_none_outlined;
        iconColor = isDark
            ? AppColors.neutralGray500
            : AppColors.neutralGray400;
        break;
      case EmptyStateType.noMessages:
        iconData = icon ?? Icons.message_outlined;
        iconColor = isDark
            ? AppColors.neutralGray500
            : AppColors.neutralGray400;
        break;
      case EmptyStateType.noFavorites:
        iconData = icon ?? Icons.favorite_border;
        iconColor = isDark
            ? AppColors.neutralGray500
            : AppColors.neutralGray400;
        break;
      case EmptyStateType.error:
        iconData = icon ?? Icons.error_outline;
        iconColor = AppColors.statusError;
        break;
      case EmptyStateType.loading:
        iconData = icon ?? Icons.hourglass_empty_outlined;
        iconColor = isDark
            ? AppColors.neutralGray500
            : AppColors.neutralGray400;
        break;
      default:
        iconData = icon ?? Icons.inbox_outlined;
        iconColor = isDark
            ? AppColors.neutralGray500
            : AppColors.neutralGray400;
        break;
    }

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
