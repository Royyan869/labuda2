import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Reusable drawer item component
///
/// ListTile dengan icon, title, badge, dan destructive state support.
class MainDrawerItem extends StatelessWidget {
  final IconData icon;
  final String title;
  final VoidCallback onTap;
  final bool isDestructive;
  final int? badge;
  final Color? iconColor;

  const MainDrawerItem({
    super.key,
    required this.icon,
    required this.title,
    required this.onTap,
    this.isDestructive = false,
    this.badge,
    this.iconColor,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return ListTile(
      leading: Icon(
        icon,
        size: 24,
        color:
            iconColor ??
            (isDestructive
                ? scheme.error
                : scheme.onSurfaceVariant),
      ),
      title: Text(
        title,
        style: TextStyle(
          color: isDestructive
              ? scheme.error
              : scheme.onSurface,
          fontSize: AppType.s16,
          fontWeight: FontWeight.w500,
        ),
      ),
      trailing: badge != null && badge! > 0
          ? Container(
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
              decoration: BoxDecoration(
                color: scheme.error,
                borderRadius: BorderRadius.circular(AppShape.r12),
              ),
              constraints: const BoxConstraints(minWidth: 24, minHeight: 24),
              child: Text(
                badge! > 99 ? '99+' : badge.toString(),
                style: TextStyle(
                  color: scheme.onError,
                  fontSize: AppType.s12,
                  fontWeight: FontWeight.bold,
                ),
                textAlign: TextAlign.center,
              ),
            )
          : null,
      onTap: onTap,
    );
  }
}
