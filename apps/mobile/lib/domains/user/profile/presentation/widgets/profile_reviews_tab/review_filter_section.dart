import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

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
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(
          bottom: BorderSide(
            color: scheme.outlineVariant,
          ),
        ),
      ),
      child: Row(
        children: [
          Text(
            'Filter by:',
            style: TextStyle(
              fontSize: AppType.s14,
              fontWeight: FontWeight.w500,
              color: scheme.onSurface,
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
                    padding: const EdgeInsets.only(right: AppMetrics.p8),
                    child: FilterChip(
                      label: Text(filter),
                      selected: isSelected,
                      onSelected: (selected) => onFilterChanged(filter),
                      selectedColor: scheme.primary.withValues(
                        alpha: 0.2,
                      ),
                      checkmarkColor: scheme.primary,
                      labelStyle: TextStyle(
                        fontSize: AppType.s12,
                        fontWeight: FontWeight.w500,
                        color: isSelected
                            ? scheme.primary
                            : scheme.onSurfaceVariant,
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
