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
