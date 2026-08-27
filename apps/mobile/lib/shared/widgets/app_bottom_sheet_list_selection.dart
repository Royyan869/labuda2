import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

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
              fillColor: isDark
                  ? AppColors.darkGray700
                  : AppColors.neutralGray50,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
                borderSide: BorderSide.none,
              ),
              contentPadding: const EdgeInsets.symmetric(
                horizontal: 16,
                vertical: 12,
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
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
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
                    horizontal: 16,
                    vertical: 16,
                  ),
                  child: Row(
                    children: [
                      // Icon
                      if (item.icon != null) ...[
                        Icon(
                          item.icon,
                          color: item.enabled
                              ? (isDark
                                    ? AppColors.neutralGray300
                                    : AppColors.neutralGray600)
                              : AppColors.neutralGray400,
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
                                fontSize: 16,
                                fontWeight: FontWeight.w500,
                                color: item.enabled
                                    ? (isDark
                                          ? AppColors.neutralWhite
                                          : AppColors.neutralGray900)
                                    : AppColors.neutralGray400,
                              ),
                            ),
                            if (item.subtitle != null) ...[
                              const SizedBox(height: 2),
                              Text(
                                item.subtitle!,
                                style: TextStyle(
                                  fontSize: 14,
                                  color: isDark
                                      ? AppColors.neutralGray400
                                      : AppColors.neutralGray600,
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
                          color: AppColors.primaryBlue,
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
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray400,
              ),
              const SizedBox(height: 16),
              Text(
                'No items found',
                style: TextStyle(
                  fontSize: 16,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
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
