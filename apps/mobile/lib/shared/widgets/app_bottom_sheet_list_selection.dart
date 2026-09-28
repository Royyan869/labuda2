import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'app_bottom_sheet_base.dart';

/// List Selection Item Class
class ListSelectionItem<T> {
  final String title;
  final String? subtitle;
  final IconData? icon;
  final T value;
  final bool enabled;

  const ListSelectionItem({
    required this.title,
    required this.value,
    this.subtitle,
    this.icon,
    this.enabled = true,
  });
}

/// AppBottomSheet for list selection
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
      height: items.length > 6 ? 500 : null,
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
            decoration: InputDecoration(
              hintText: widget.searchHint ?? 'Search...',
              prefixIcon: const Icon(Icons.search),
              filled: true,
              fillColor: scheme.surfaceContainerHigh,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(AppShape.r12),
                borderSide: BorderSide.none,
              ),
              contentPadding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p16,
                vertical: AppMetrics.p12,
              ),
            ),
          ),
          const SizedBox(height: 16),
        ],

        // Items List
        ListView.separated(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          itemCount: filteredItems.length,
          separatorBuilder: (context, index) => Divider(
            height: 1,
            color: scheme.outlineVariant,
          ),
          itemBuilder: (context, index) {
            final item = filteredItems[index];
            final isSelected = item.value == widget.selectedValue;

            return Material(
              color: Colors.transparent,
              child: InkWell(
                onTap: item.enabled
                    ? () => Navigator.of(context).pop(item.value)
                    : null,
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p16,
                    vertical: AppMetrics.p16,
                  ),
                  child: Row(
                    children: [
                      // Icon
                      if (item.icon != null) ...[
                        Icon(
                          item.icon,
                          color: item.enabled
                              ? scheme.onSurfaceVariant
                              : scheme.outline,
                          size: 24,
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
                              style: TextStyle(
                                fontSize: AppType.s16,
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
                                style: TextStyle(
                                  fontSize: AppType.s14,
                                  color: scheme.onSurfaceVariant,
                                ),
                              ),
                            ],
                          ],
                        ),
                      ),

                      // Selection Indicator
                      if (isSelected) ...[
                        Icon(
                          Icons.check_circle,
                          color: scheme.secondary,
                          size: 24,
                        ),
                      ],
                    ],
                  ),
                ),
              ),
            );
          },
        ),

        // Empty State
        if (filteredItems.isEmpty) ...[
          const SizedBox(height: 32),
          Column(
            children: [
              Icon(
                Icons.search_off,
                size: 48,
                color: scheme.outline,
              ),
              const SizedBox(height: 16),
              Text(
                'No items found',
                style: TextStyle(
                  fontSize: AppType.s16,
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
}
