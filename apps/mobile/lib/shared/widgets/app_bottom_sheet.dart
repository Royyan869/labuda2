/// The canonical modal Bottom Sheet family.
///
/// One authority, three presentation categories:
/// - [AppBottomSheetBase]: concise form/content sheets;
/// - [AppBottomSheetActions]: short action menus;
/// - [AppBottomSheetListSelection]: selection/search lists.
///
/// Surface, shape (top r20) and elevation come from `bottomSheetTheme`;
/// `AppDragHandle` is the single drag-handle authority. No raw
/// `showModalBottomSheet` call may re-implement these.
library;

export 'app_bottom_sheet_base.dart';
export 'app_bottom_sheet_actions.dart';
export 'app_bottom_sheet_list_selection.dart';

/// Compatibility facade — keeps the original `AppBottomSheet.*` entry point.
import 'package:flutter/material.dart';
import 'app_bottom_sheet_base.dart';
import 'app_bottom_sheet_actions.dart';
import 'app_bottom_sheet_list_selection.dart';

class AppBottomSheet {
  /// Show a standard (form/content) bottom sheet with custom content
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
  }) => AppBottomSheetBase.show<T>(
    context: context,
    content: content,
    title: title,
    height: height,
    isDismissible: isDismissible,
    enableDrag: enableDrag,
    useRootNavigator: useRootNavigator,
    showDragHandle: showDragHandle,
    padding: padding,
    onSave: onSave,
    saveButtonText: saveButtonText,
    showSaveButton: showSaveButton,
  );

  /// Show action-based bottom sheet
  static Future<T?> showActions<T>({
    required BuildContext context,
    String? title,
    String? subtitle,
    required List<BottomSheetAction<T>> actions,
    bool showCancel = true,
    String cancelLabel = 'Cancel',
    bool isDismissible = true,
  }) => AppBottomSheetActions.showActions<T>(
    context: context,
    title: title,
    subtitle: subtitle,
    actions: actions,
    showCancel: showCancel,
    cancelLabel: cancelLabel,
    isDismissible: isDismissible,
  );

  /// Show list selection bottom sheet
  static Future<T?> showListSelection<T>({
    required BuildContext context,
    required String title,
    required List<ListSelectionItem<T>> items,
    T? selectedValue,
    bool showSearch = false,
    String? searchHint,
  }) => AppBottomSheetListSelection.showListSelection<T>(
    context: context,
    title: title,
    items: items,
    selectedValue: selectedValue,
    showSearch: showSearch,
    searchHint: searchHint,
  );
}
