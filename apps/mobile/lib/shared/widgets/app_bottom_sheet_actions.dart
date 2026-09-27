import 'package:flutter/material.dart';
import 'app_bottom_sheet_base.dart';

/// Bottom Sheet Action Styles
enum BottomSheetActionStyle { normal, destructive, cancel }

/// Bottom Sheet Action Class
class BottomSheetAction<T> {
  final String title;
  final String? subtitle;
  final IconData? icon;
  final Color? iconColor;
  final VoidCallback? onPressed;
  final BottomSheetActionStyle style;
  final String? badge;

  const BottomSheetAction({
    required this.title,
    this.subtitle,
    this.icon,
    this.iconColor,
    this.onPressed,
    this.style = BottomSheetActionStyle.normal,
    this.badge,
  });
}

/// AppBottomSheet for action-based bottom sheets
class AppBottomSheetActions {
  /// Show action-based bottom sheet
  static Future<T?> showActions<T>({
    required BuildContext context,
    String? title,
    String? subtitle,
    required List<BottomSheetAction<T>> actions,
    bool showCancel = true,
    String cancelLabel = 'Cancel',
    bool isDismissible = true,
  }) {
    final scheme = Theme.of(context).colorScheme;

    return AppBottomSheetBase.show<T>(
      context: context,
      title: title,
      isDismissible: isDismissible,
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Subtitle
          if (subtitle != null) ...[
            Padding(
              padding: const EdgeInsets.only(bottom: 20),
              child: Text(
                subtitle,
                style: TextStyle(
                  fontSize: 14,
color: scheme.onSurfaceVariant,
                  height: 1.4,
                ),
                textAlign: TextAlign.center,
              ),
            ),
          ],

          // Actions
          ...actions.map(
            (action) => _buildActionItem(
              context: context,
              action: action,
              scheme: scheme,
            ),
          ),

          // Cancel Button
          if (showCancel) ...[
            const SizedBox(height: 8),
            Container(
              width: double.infinity,
              height: 1,
              color: scheme.outlineVariant,
            ),
            const SizedBox(height: 8),
            _buildActionItem(
              context: context,
              action: BottomSheetAction<T>(
                title: cancelLabel,
                onPressed: () => Navigator.of(context).pop(),
                style: BottomSheetActionStyle.cancel,
              ),
              scheme: scheme,
            ),
          ],
        ],
      ),
    );
  }

  /// Build action item widget
  static Widget _buildActionItem<T>({
    required BuildContext context,
    required BottomSheetAction<T> action,
    required ColorScheme scheme,
  }) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: action.onPressed,
        borderRadius: BorderRadius.circular(12),
        child: Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
          child: Row(
            children: [
              // Icon
              if (action.icon != null) ...[
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: (action.iconColor ?? scheme.primary)
                        .withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Icon(
                    action.icon,
                    color: action.iconColor ?? scheme.primary,
                    size: 20,
                  ),
                ),
                const SizedBox(width: 16),
              ],

              // Text Content
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      action.title,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w500,
                        color: _getTextColor(action.style, scheme),
                      ),
                    ),
                    if (action.subtitle != null) ...[
                      const SizedBox(height: 2),
                      Text(
                        action.subtitle!,
                        style: TextStyle(
                          fontSize: 14,
color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ],
                ),
              ),

              // Badge
              if (action.badge != null) ...[
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: scheme.primary,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Text(
                    action.badge!,
style: TextStyle(
                       color: scheme.onPrimary,
                       fontSize: 12,
                       fontWeight: FontWeight.w600,
                     ),
                  ),
                ),
              ],

              // Arrow
              const SizedBox(width: 8),
              Icon(
                Icons.chevron_right,
                color: scheme.onSurfaceVariant,
                size: 20,
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// Get text color based on action style
  static Color _getTextColor(BottomSheetActionStyle style, ColorScheme scheme) {
    switch (style) {
      case BottomSheetActionStyle.destructive:
        return scheme.error;
      case BottomSheetActionStyle.cancel:
        return scheme.onSurfaceVariant;
      case BottomSheetActionStyle.normal:
        return scheme.onSurface;
    }
  }
}
