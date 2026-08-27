/// Refactored AppBottomSheet - Export all components
/// Reduced from 682 lines to 5 separate component files
///
/// Components:
/// - AppBottomSheetBase: Standard bottom sheet with custom content
/// - AppBottomSheetActions: Action-based bottom sheets
/// - AppBottomSheetSettings: Settings bottom sheets
/// - AppBottomSheetMediaPicker: Media picker bottom sheets
/// - AppBottomSheetListSelection: List selection bottom sheets
library;

export 'app_bottom_sheet_base.dart';
export 'app_bottom_sheet_actions.dart';
export 'app_bottom_sheet_settings.dart';
export 'app_bottom_sheet_media_picker.dart';
export 'app_bottom_sheet_list_selection.dart';

/// Compatibility layer - maintains original API
import 'package:flutter/material.dart';
import 'app_bottom_sheet_base.dart';
import 'app_bottom_sheet_actions.dart';
import 'app_bottom_sheet_settings.dart';
import 'app_bottom_sheet_media_picker.dart';
import 'app_bottom_sheet_list_selection.dart';

class AppBottomSheet {
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
  }) => AppBottomSheetBase.show<T>(
    context: context,
    content: content,
    title: title,
    height: height,
    isDismissible: isDismissible,
    enableDrag: enableDrag,
    useRootNavigator: useRootNavigator,
    backgroundColor: backgroundColor,
    elevation: elevation,
    shape: shape,
    padding: padding,
    showDragHandle: showDragHandle,
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

  /// Show settings bottom sheet
  static Future<void> showSettings({
    required BuildContext context,
    String title = 'Settings',
    List<SettingsItem>? customSettings,
  }) => AppBottomSheetSettings.showSettings(
    context: context,
    title: title,
    customSettings: customSettings,
  );

  /// Show quick media picker
  static Future<String?> showQuickMediaPicker({
    required BuildContext context,
    String title = 'Select Media',
    bool allowVideo = true,
  }) => AppBottomSheetMediaPicker.showQuickMediaPicker(
    context: context,
    title: title,
    allowVideo: allowVideo,
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
