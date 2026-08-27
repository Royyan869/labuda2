import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Reusable empty state widget untuk konsistensi UI
/// Digunakan di comment sections, lists, dan area kosong lainnya
class EmptyStateWidget extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final double iconSize;
  final EdgeInsets padding;
  final Widget? action;

  const EmptyStateWidget({
    super.key,
    required this.icon,
    required this.title,
    required this.subtitle,
    this.iconSize = 48,
    this.padding = const EdgeInsets.all(24),
    this.action,
  });

  /// Factory constructor untuk comment empty state
  factory EmptyStateWidget.comments({
    Key? key,
    EdgeInsets padding = const EdgeInsets.all(24),
    Widget? action,
    double iconSize = 48,
  }) {
    return EmptyStateWidget(
      key: key,
      icon: Icons.forum_outlined,
      title: 'No comments yet',
      subtitle: 'Be the first to share your thoughts!',
      padding: padding,
      action: action,
      iconSize: iconSize,
    );
  }

  /// Factory constructor untuk generic empty list
  factory EmptyStateWidget.list({
    Key? key,
    required String title,
    required String subtitle,
    IconData icon = Icons.inbox_outlined,
    EdgeInsets padding = const EdgeInsets.all(24),
    Widget? action,
  }) {
    return EmptyStateWidget(
      key: key,
      icon: icon,
      title: title,
      subtitle: subtitle,
      padding: padding,
      action: action,
    );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Center(
      child: SingleChildScrollView(
        child: Container(
          padding: padding,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Icon(
                icon,
                size: iconSize,
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray400,
              ),
              const SizedBox(height: 16),
              Text(
                title,
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                subtitle,
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray500,
                ),
                textAlign: TextAlign.center,
              ),
              if (action != null) ...[const SizedBox(height: 16), action!],
            ],
          ),
        ),
      ),
    );
  }
}
