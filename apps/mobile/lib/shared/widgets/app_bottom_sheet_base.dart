import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// The drag handle, as ONE authority.
///
/// Every sheet that showed a grab affordance used to spell `width: 40`,
/// `height: 4`, `outlineVariant` ink and `AppShape.r2` for itself — ten
/// spellings across ten files, several of them carrying a comment swearing they
/// matched the link picker sheet. A comment claiming two hand-copies agree is
/// exactly how they stop agreeing; the size, ink and radius live here now, and
/// [AppBottomSheetBase]'s `showDragHandle` flag remains the one switch that
/// decides whether a base sheet shows it at all.
///
/// The SPACE around the handle stays a per-sheet decision, because what sits
/// below the handle decides it: the copies used four different margins
/// (`top p12`, `top p12 + bottom p8`, `bottom p8`, `bottom p24`). The default
/// below is the base sheet's canonical spacing, so the sites that had it pass
/// nothing and the sites that differ say so.
class AppDragHandle extends StatelessWidget {
  const AppDragHandle({super.key, this.padding = _defaultPadding});

  /// How much room the handle wants around itself.
  final EdgeInsetsGeometry padding;

  static const EdgeInsetsGeometry _defaultPadding = EdgeInsets.only(
    top: AppMetrics.p12,
    bottom: AppMetrics.p8,
  );

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: padding,
      child: Container(
        width: 40,
        height: 4,
        decoration: BoxDecoration(
          color: scheme.outlineVariant,
          borderRadius: BorderRadius.circular(AppShape.r2),
        ),
      ),
    );
  }
}

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
              // Drag Handle — one implementation ([AppDragHandle]); this flag is
              // the switch.
              if (showDragHandle) const AppDragHandle(),

              // Title Section
              if (title != null) ...[
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.fromLTRB(AppMetrics.p24, AppMetrics.p8, AppMetrics.p24, AppMetrics.p16),
                  child: Text(
                    title,
                    style: TextStyle(
                      fontSize: AppType.s20,
                      fontWeight: FontWeight.w600,
color: scheme.onSurface,
                    ),
                    textAlign: TextAlign.center,
                  ),
                ),
                Container(
                  height: 1,
                  margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p24),
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
                    padding: padding ?? const EdgeInsets.all(AppMetrics.p24),
                    child: content,
                  ),
                ),
              ),

              // Save Button
              if (showSaveButton && onSave != null) ...[
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.fromLTRB(AppMetrics.p24, AppMetrics.p8, AppMetrics.p24, AppMetrics.p16),
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
