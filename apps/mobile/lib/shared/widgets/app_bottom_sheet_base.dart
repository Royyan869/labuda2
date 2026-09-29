import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Base AppBottomSheet with standard content support
class AppBottomSheetBase {
  /// Show a standard bottom sheet with custom content
  static Future<T?> show<T>({
    required BuildContext context,
    required Widget content,
    String? title,
    double? height,
    bool isDismissible = true,
    bool enableDrag = true,
    bool useRootNavigator = false,
    Color? backgroundColor,
    double? elevation,
    ShapeBorder? shape,
    EdgeInsetsGeometry? padding,
    bool showDragHandle = true,
    VoidCallback? onSave,
    String saveButtonText = 'Save',
    bool showSaveButton = false,
  }) {
    final scheme = Theme.of(context).colorScheme;

    return showModalBottomSheet<T>(
      context: context,
      isDismissible: isDismissible,
      enableDrag: enableDrag,
      useRootNavigator: useRootNavigator,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      elevation: AppElevation.none,
      builder: (context) => Padding(
        padding: MediaQuery.of(context).viewInsets,
        child: Container(
          height: height,
          constraints: BoxConstraints(
            maxHeight: MediaQuery.of(context).size.height * 0.9,
            minHeight: 200,
          ),
          decoration: BoxDecoration(
            color:
                backgroundColor ??
                scheme.surface,
            borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
            boxShadow: [
              BoxShadow(
                color: scheme.shadow.withValues(alpha: 0.2),
                blurRadius: 20,
                offset: const Offset(0, -5),
              ),
            ],
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              // Drag Handle
              if (showDragHandle)
                Container(
                  margin: const EdgeInsets.only(top: AppMetrics.p12, bottom: AppMetrics.p8),
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
color: scheme.outlineVariant,
                    borderRadius: BorderRadius.circular(AppShape.r2),
                  ),
                ),

              // Title Section
              if (title != null) ...[
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.fromLTRB(AppMetrics.p20, AppMetrics.p8, AppMetrics.p20, AppMetrics.p16),
                  child: Text(
                    title,
                    style: TextStyle(
                      fontSize: AppType.s18,
                      fontWeight: FontWeight.w600,
color: scheme.onSurface,
                    ),
                    textAlign: TextAlign.center,
                  ),
                ),
                Container(
                  height: 1,
                  margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p20),
                  decoration: BoxDecoration(
color: scheme.outlineVariant,
                  ),
                ),
              ],

              // Content
              Flexible(
                child: SingleChildScrollView(
                  child: Container(
                    width: double.infinity,
                    padding: padding ?? const EdgeInsets.all(AppMetrics.p20),
                    child: content,
                  ),
                ),
              ),

              // Save Button
              if (showSaveButton && onSave != null) ...[
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.fromLTRB(AppMetrics.p20, AppMetrics.p8, AppMetrics.p20, AppMetrics.p16),
                  child: ElevatedButton(
                    onPressed: onSave,
                    style: ElevatedButton.styleFrom(
                      minimumSize: const Size(double.infinity, 48),
                    ),
                    child: Text(
                      saveButtonText,
style: TextStyle(
                         fontSize: AppType.s16,
                         fontWeight: FontWeight.w600,
                         color: scheme.onPrimary,
                       ),
                    ),
                  ),
                ),
              ],

              // Safe Area Bottom
              SizedBox(height: MediaQuery.of(context).padding.bottom),
            ],
          ),
        ),
      ),
    );
  }
}
