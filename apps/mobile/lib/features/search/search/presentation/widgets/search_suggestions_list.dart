import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

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
    if (isLoading) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.all(AppMetrics.p32),
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
            Icons.search,
          ),
        if (popularItems.isNotEmpty) ...[
          const SizedBox(height: 16),
          _buildSection(
            context,
            popularItemsTitle,
            popularItems,
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
    IconData icon,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p8),
          child: Row(
            children: [
              Icon(icon, size: AppIconSize.action, color: scheme.primary),
              const SizedBox(width: 8),
              Text(
                title,
                style: context.typeRoles.titleSection.copyWith(
                  fontWeight: FontWeight.w600,
                  color: scheme.onSurface,
                ),
              ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
          child: Wrap(
            spacing: 8,
            runSpacing: 8,
            children: items
                .map((item) => _buildSuggestionChip(context, item))
                .toList(),
          ),
        ),
      ],
    );
  }

  Widget _buildSuggestionChip(
    BuildContext context,
    String suggestion,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return FilterChip(
      label: Text(suggestion),
      onSelected: (_) => onSuggestionTap(suggestion),
      backgroundColor: scheme.surfaceContainerHigh,
      selectedColor: scheme.primary.withValues(alpha: 0.2),
      labelStyle: Theme.of(context).textTheme.labelLarge?.copyWith(
        color: scheme.onSurfaceVariant,
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r20),
        side: BorderSide(
          color: scheme.outlineVariant,
        ),
      ),
    );
  }
}
