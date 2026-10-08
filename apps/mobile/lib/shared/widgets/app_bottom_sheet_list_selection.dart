import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'app_bottom_sheet_base.dart';

/// List Selection Item Class
class ListSelectionItem<T> {
  final String title;
  final String? subtitle;

  /// Optional plain leading icon. Prefer [leading] when a richer visual is
  /// needed (e.g. the canonical payment-method logo).
  final IconData? icon;

  /// Optional leading widget. When set, it replaces [icon].
  final Widget? leading;
  final T value;
  final bool enabled;

  /// Optional trailing text (e.g. a price/total). The selection check icon
  /// still renders after it when this row is the current value.
  final String? trailingText;

  const ListSelectionItem({
    required this.title,
    required this.value,
    this.subtitle,
    this.icon,
    this.leading,
    this.enabled = true,
    this.trailingText,
  });
}

/// AppBottomSheet for list selection.
///
/// The canonical selection builder: item rows are laid out in a plain [Column]
/// and the WHOLE sheet content scrolls (via the base's scroll area) — no
/// `shrinkWrap` + `NeverScrollableScrollPhysics` nested list, and no local
/// surface/radius. Keyboard and safe area come from the base.
class AppBottomSheetListSelection {
  /// Show list selection bottom sheet
  static Future<T?> showListSelection<T>({
    required BuildContext context,
    required String title,
    required List<ListSelectionItem<T>> items,
    T? selectedValue,
    bool showSearch = false,
    String? searchHint,
  }) {
    return AppBottomSheetBase.show<T>(
      context: context,
      title: title,
      content: _ListSelectionContent<T>(
        items: items,
        selectedValue: selectedValue,
        showSearch: showSearch,
        searchHint: searchHint,
      ),
    );
  }
}

/// List Selection Content Widget
class _ListSelectionContent<T> extends StatefulWidget {
  final List<ListSelectionItem<T>> items;
  final T? selectedValue;
  final bool showSearch;
  final String? searchHint;

  const _ListSelectionContent({
    required this.items,
    this.selectedValue,
    this.showSearch = false,
    this.searchHint,
  });

  @override
  State<_ListSelectionContent<T>> createState() =>
      _ListSelectionContentState<T>();
}

class _ListSelectionContentState<T> extends State<_ListSelectionContent<T>> {
  late List<ListSelectionItem<T>> filteredItems;
  final TextEditingController _searchController = TextEditingController();

  @override
  void initState() {
    super.initState();
    filteredItems = widget.items;
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _filterItems(String query) {
    setState(() {
      if (query.isEmpty) {
        filteredItems = widget.items;
      } else {
        filteredItems = widget.items.where((item) {
          return item.title.toLowerCase().contains(query.toLowerCase()) ||
              (item.subtitle?.toLowerCase().contains(query.toLowerCase()) ??
                  false);
        }).toList();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Search Bar
        if (widget.showSearch) ...[
          TextField(
            controller: _searchController,
            onChanged: _filterItems,
            decoration: AppTheme.searchDecoration(
              scheme,
              hintText: widget.searchHint ?? 'Search...',
            ).copyWith(prefixIcon: const Icon(Icons.search)),
          ),
          const SizedBox(height: 16),
        ],

        // Items — plain column inside the base's scroll area.
        for (var i = 0; i < filteredItems.length; i++) ...[
          if (i > 0) const Divider(height: 1),
          _buildItem(context, scheme, filteredItems[i]),
        ],

        // Empty State
        if (filteredItems.isEmpty) ...[
          const SizedBox(height: 32),
          Column(
            children: [
              Icon(
                Icons.search_off,
                size: AppIconSize.display,
                color: scheme.outline,
              ),
              const SizedBox(height: 16),
              Text(
                'No items found',
                style: context.typeRoles.titleProminent.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
          const SizedBox(height: 32),
        ],
      ],
    );
  }

  Widget _buildItem(
    BuildContext context,
    ColorScheme scheme,
    ListSelectionItem<T> item,
  ) {
    final isSelected = item.value == widget.selectedValue;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: item.enabled
            ? () => Navigator.of(context).pop(item.value)
            : null,
        child: Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p16,
            vertical: AppMetrics.p16,
          ),
          child: Row(
            children: [
              // Leading visual (preferred) or a plain icon.
              if (item.leading != null) ...[
                item.leading!,
                const SizedBox(width: 16),
              ] else if (item.icon != null) ...[
                Icon(
                  item.icon,
                  color: item.enabled
                      ? scheme.onSurfaceVariant
                      : scheme.outline,
                  size: AppIconSize.header,
                ),
                const SizedBox(width: 16),
              ],

              // Text Content
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      item.title,
                      style: context.typeRoles.titleCompact.copyWith(
                        fontWeight: FontWeight.w500,
                        color: item.enabled
                            ? scheme.onSurface
                            : scheme.onSurfaceVariant,
                      ),
                    ),
                    if (item.subtitle != null) ...[
                      const SizedBox(height: 2),
                      Text(
                        item.subtitle!,
                        style: context.typeRoles.bodyDense.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ],
                ),
              ),

              // Optional trailing text (price/total)
              if (item.trailingText != null) ...[
                const SizedBox(width: AppMetrics.p8),
                Text(
                  item.trailingText!,
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurface,
                  ),
                ),
              ],

              // Selection Indicator
              if (isSelected) ...[
                const SizedBox(width: AppMetrics.p8),
                Icon(
                  Icons.check_circle,
                  color: scheme.secondary,
                  size: AppIconSize.header,
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
