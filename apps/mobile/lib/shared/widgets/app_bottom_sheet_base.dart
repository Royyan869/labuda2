import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// The drag handle, as ONE authority.
///
/// Every sheet that showed a grab affordance used to spell `width: 40`,
/// `height: 4`, `outlineVariant` ink and `AppShape.r2` for itself — ten
/// spellings across ten files. The size, ink and radius live here now and no
/// sheet may re-spell them.
///
/// INTERACTION CONTRACT: the handle is a *visual affordance* for the sheet's
/// dismiss-on-drag gesture (the framework's `enableDrag`). It is NOT a
/// `DraggableScrollableSheet` resize grip — Labuda modal sheets are not
/// interactively resized. A sheet that disables drag (`enableDrag: false`) must
/// not render this handle, so it never promises an interaction it does not have.
///
/// The SPACE around the handle stays a per-sheet decision, because what sits
/// below the handle decides it. The default below is the base sheet's canonical
/// spacing.
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

/// Base AppBottomSheet with standard content support.
///
/// SURFACE / SHAPE / ELEVATION are owned by `bottomSheetTheme` — this builder
/// paints NO local colour, radius or shadow, so the framework's modal Material
/// is the single visual authority. Callers may only supply CONTENT and the
/// small grammar flags below.
class AppBottomSheetBase {
  /// The height a modal sheet may occupy on this screen — ONE authority.
  ///
  /// Window height minus the KEYBOARD inset (`viewInsets.bottom`) minus the
  /// SYSTEM TOP inset: the space that actually exists once the system UI has
  /// taken its share. A fraction of the raw screen height would go stale the
  /// moment the keyboard opens (the sheet would then be taller than the space
  /// left above it) and would let a sheet cover the status bar.
  ///
  /// Both the sheet's own ceiling ([show]) and any body-height request read
  /// THIS function, so a ceiling and a body can never disagree about how much
  /// room there is.
  static double availableHeight(BuildContext context) {
    final media = MediaQuery.of(context);
    final available =
        media.size.height - media.viewInsets.bottom - media.padding.top;
    return math.max(0.0, available);
  }

  /// Show a standard bottom sheet with custom content
  static Future<T?> show<T>({
    required BuildContext context,
    required Widget content,
    String? title,
    double? height,
    bool isDismissible = true,
    bool enableDrag = true,
    bool useRootNavigator = false,
    bool showDragHandle = true,
    EdgeInsetsGeometry? padding,
    VoidCallback? onSave,
    String saveButtonText = 'Save',
    bool showSaveButton = false,
  }) {
    return showModalBottomSheet<T>(
      context: context,
      isScrollControlled: true,
      isDismissible: isDismissible,
      enableDrag: enableDrag,
      useRootNavigator: useRootNavigator,
      // Surface, shape (top r20) and flat elevation come from
      // `bottomSheetTheme`; no local override is allowed here.
      builder: (sheetContext) => Padding(
        // Keyboard contract: one place lifts the sheet above the keyboard.
        padding: EdgeInsets.only(
          bottom: MediaQuery.viewInsetsOf(sheetContext).bottom,
        ),
        child: ConstrainedBox(
          // Ceiling: 90% of the space the sheet ACTUALLY has (keyboard and
          // system top inset removed), never 90% of the raw screen height.
          constraints: BoxConstraints(
            maxHeight: availableHeight(sheetContext) * 0.9,
          ),
          child: SizedBox(
            height: height,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                // Drag handle — one implementation ([AppDragHandle]); anchored
                // to the real drag gesture.
                if (showDragHandle && enableDrag) const AppDragHandle(),

                // Title Section
                if (title != null) ...[
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.fromLTRB(
                      AppMetrics.p24,
                      AppMetrics.p8,
                      AppMetrics.p24,
                      AppMetrics.p16,
                    ),
                    child: Text(
                      title,
                      style: context.typeRoles.titleSection.copyWith(
                        fontWeight: FontWeight.w600,
                        color: Theme.of(sheetContext).colorScheme.onSurface,
                      ),
                      textAlign: TextAlign.center,
                    ),
                  ),
                  const Divider(height: 1),
                ],

                // Content — scrolls internally inside the constrained sheet.
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
                    padding: const EdgeInsets.fromLTRB(
                      AppMetrics.p24,
                      AppMetrics.p8,
                      AppMetrics.p24,
                      AppMetrics.p16,
                    ),
                    child: ElevatedButton(
                      onPressed: onSave,
                      style: ElevatedButton.styleFrom(
                        minimumSize: const Size(double.infinity, 48),
                      ),
                      child: Text(
                        saveButtonText,
                        style: Theme.of(sheetContext).textTheme.labelLarge
                            ?.copyWith(fontWeight: FontWeight.w600),
                      ),
                    ),
                  ),
                ],

                // Bottom system inset — consumed ONCE, at the sheet's own
                // layer, from the EFFECTIVE bottom padding
                // (`MediaQuery.paddingOf`), the same value
                // `SafeArea(top: false)` consumes: while the keyboard covers
                // the system navigation area the effective inset is zero, so
                // no artificial gap is left between the content and the
                // keyboard. When the system bar hides, the spacer goes to
                // zero with it.
                SizedBox(height: MediaQuery.paddingOf(sheetContext).bottom),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
