import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Reusable Modal Dialog System
///
/// Provides consistent modal dialogs across the app instead of bottom sheets
/// Features:
/// - Professional dialog styling
/// - Dark/Light mode support
/// - Responsive sizing
/// - Consistent animations
class AppModal {
  /// Show a generic modal dialog
  static Future<T?> show<T>({
    required BuildContext context,
    required String title,
    required Widget content,
    List<Widget>? actions,
    bool barrierDismissible = true,
    double? width,
    double? height,
    EdgeInsetsGeometry? contentPadding,
  }) {
    final scheme = Theme.of(context).colorScheme;

    return showDialog<T>(
      context: context,
      barrierDismissible: barrierDismissible,
      builder: (BuildContext context) {
        return Dialog(
          backgroundColor: scheme.surfaceContainerHigh,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppShape.r16),
          ),
          child: Container(
            width: width,
            height: height,
            constraints: BoxConstraints(
              maxWidth: MediaQuery.of(context).size.width * 0.9,
              maxHeight: MediaQuery.of(context).size.height * 0.8,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                // Header
                Container(
                  padding: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p12, AppMetrics.p8, AppMetrics.p12),
                  decoration: BoxDecoration(
                    color: scheme.surfaceContainerHighest,
                    borderRadius: const BorderRadius.vertical(
                      top: Radius.circular(AppShape.r16),
                    ),
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          title,
                          style: TextStyle(
                            fontSize: AppType.s16,
                            fontWeight: FontWeight.w600,
                            color: scheme.onSurface,
                          ),
                        ),
                      ),
                      IconButton(
                        onPressed: () => Navigator.of(context).pop(),
                        icon: Icon(
                          Icons.close,
                          color: scheme.onSurfaceVariant,
                          size: 18,
                        ),
                        iconSize: 18,
                        padding: const EdgeInsets.all(AppMetrics.p8),
                        constraints: const BoxConstraints(),
                      ),
                    ],
                  ),
                ),

                // Content
                Flexible(
                  child: Container(
                    padding: contentPadding ?? const EdgeInsets.all(AppMetrics.p16),
                    child: content,
                  ),
                ),

                // Actions
                if (actions != null && actions.isNotEmpty) ...[
                  Container(
                    padding: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p8, AppMetrics.p16, AppMetrics.p16),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: actions
                          .map(
                            (action) => Padding(
                              padding: const EdgeInsets.only(left: AppMetrics.p12),
                              child: action,
                            ),
                          )
                          .toList(),
                    ),
                  ),
                ],
              ],
            ),
          ),
        );
      },
    );
  }

  /// Show confirmation modal
  static Future<bool?> showConfirmation({
    required BuildContext context,
    required String title,
    required String message,
    String confirmLabel = 'Confirm',
    String cancelLabel = 'Cancel',
    Color? confirmColor,
    IconData? icon,
  }) {
    final scheme = Theme.of(context).colorScheme;

    return show<bool>(
      context: context,
      title: title,
      width: 400,
      content: Row(
        children: [
          if (icon != null) ...[
            Icon(icon, color: confirmColor ?? scheme.primary, size: 24),
            const SizedBox(width: 16),
          ],
          Expanded(
            child: Text(
              message,
              style: TextStyle(
                fontSize: AppType.s16,
                color: scheme.onSurface,
                height: 1.4,
              ),
            ),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(
            cancelLabel,
            style: TextStyle(color: scheme.onSurfaceVariant),
          ),
        ),
        ElevatedButton(
          onPressed: () => Navigator.of(context).pop(true),
          style: ElevatedButton.styleFrom(
            backgroundColor: confirmColor ?? scheme.primary,
          ),
          child: Text(confirmLabel),
        ),
      ],
    );
  }

  /// Show loading modal
  static Future<T?> showLoading<T>({
    required BuildContext context,
    String title = 'Loading...',
    String message = 'Please wait...',
  }) {
    final scheme = Theme.of(context).colorScheme;

    return show<T>(
      context: context,
      title: title,
      width: 300,
      barrierDismissible: false,
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(color: scheme.primary),
          const SizedBox(height: 20),
          Text(
            message,
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: AppType.s14,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

}
