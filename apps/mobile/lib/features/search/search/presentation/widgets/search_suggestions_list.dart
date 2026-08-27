import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Widget to display search suggestions and popular search items
///
/// SEARCH SURFACE PURGE V1:
/// - "Trending Searches" renamed to "Popular Items" (honest label)
/// - Title is now configurable via popularItemsTitle
/// - No longer claims to show real trending data
class SearchSuggestionsList extends StatelessWidget {
  final List<String> suggestions;
  final List<String> popularItems;
  final String popularItemsTitle;
  final ValueChanged<String> onSuggestionTap;
  final bool isLoading;

  const SearchSuggestionsList({
    super.key,
    required this.suggestions,
    required List<String> trendingSearches,
    required this.onSuggestionTap,
    this.isLoading = false,
    String? popularItemsTitle,
  }) : popularItems = trendingSearches,
       popularItemsTitle = popularItemsTitle ?? 'Popular';

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (isLoading) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.all(32),
          child: CircularProgressIndicator(),
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (suggestions.isNotEmpty)
          _buildSection(
            context,
            'Suggestions',
            suggestions,
            isDark,
            Icons.search,
          ),
        if (popularItems.isNotEmpty) ...[
          const SizedBox(height: 16),
          _buildSection(
            context,
            popularItemsTitle,
            popularItems,
            isDark,
            Icons
                .local_fire_department, // Changed from trending_up to avoid fake "trending" implication
          ),
        ],
      ],
    );
  }

  Widget _buildSection(
    BuildContext context,
    String title,
    List<String> items,
    bool isDark,
    IconData icon,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Row(
            children: [
              Icon(icon, size: 18, color: AppColors.primary),
              const SizedBox(width: 8),
              Text(
                title,
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralGray100
                      : AppColors.neutralGray900,
                ),
              ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Wrap(
            spacing: 8,
            runSpacing: 8,
            children: items
                .map((item) => _buildSuggestionChip(context, item, isDark))
                .toList(),
          ),
        ),
      ],
    );
  }

  Widget _buildSuggestionChip(
    BuildContext context,
    String suggestion,
    bool isDark,
  ) {
    return FilterChip(
      label: Text(suggestion),
      onSelected: (_) => onSuggestionTap(suggestion),
      backgroundColor: isDark
          ? AppColors.darkGray700
          : AppColors.neutralGray100,
      selectedColor: AppColors.primary.withValues(alpha: 0.2),
      labelStyle: TextStyle(
        color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray600,
        fontSize: 14,
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(20),
        side: BorderSide(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
    );
  }
}
