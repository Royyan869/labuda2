import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';

/// Filter section for reviews
class ReviewFilterSection extends StatelessWidget {
  final String selectedFilter;
  final List<String> filterOptions;
  final ValueChanged<String> onFilterChanged;

  const ReviewFilterSection({
    super.key,
    required this.selectedFilter,
    required this.filterOptions,
    required this.onFilterChanged,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        border: Border(
          bottom: BorderSide(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
          ),
        ),
      ),
      child: Row(
        children: [
          Text(
            'Filter by:',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w500,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: filterOptions.map((filter) {
                  final isSelected = selectedFilter == filter;
                  return Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: FilterChip(
                      label: Text(filter),
                      selected: isSelected,
                      onSelected: (selected) => onFilterChanged(filter),
                      selectedColor: AppColors.primaryRed.withValues(
                        alpha: 0.2,
                      ),
                      checkmarkColor: AppColors.primaryRed,
                      labelStyle: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w500,
                        color: isSelected
                            ? AppColors.primaryRed
                            : (isDark
                                  ? AppColors.neutralGray300
                                  : AppColors.neutralGray600),
                      ),
                    ),
                  );
                }).toList(),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
