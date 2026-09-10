import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/presentation/utils/all_tab_sections.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_result_item.dart';

/// All Tab content — SECTION-BASED multi-domain overview.
///
/// Renders each domain that has results as its own section:
///
/// ```text
/// User       ── [Lihat Semua]
///   ≤ AllTabPreviewLimits.users items (canonical User order)
/// For Sale   ── [Lihat Semua]
///   ≤ AllTabPreviewLimits.forSale items (canonical For Sale order)
/// Auctions   ── [Lihat Semua]
///   ≤ AllTabPreviewLimits.auctions items (canonical Auction order)
/// Content    ── [Lihat Semua]
///   ≤ AllTabPreviewLimits.contents items (canonical Content order)
/// ```
///
/// Empty domains are not rendered. "Lihat Semua" only switches the tab —
/// it never issues a new query or search.
class AllTabResultsView extends StatelessWidget {
  final UnifiedSearchResults results;
  final ValueChanged<SearchResultType> onSeeAll;
  final ValueChanged<SearchResult> onItemTap;

  const AllTabResultsView({
    super.key,
    required this.results,
    required this.onSeeAll,
    required this.onItemTap,
  });

  @override
  Widget build(BuildContext context) {
    final sections = buildAllTabSections(results);
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final dividerColor = isDark
        ? AppColors.darkGray600
        : AppColors.neutralGray200;

    // Sections are projected, not lazy: total preview size is bounded by
    // the All caps (≤ 18 items) so a plain ListView is appropriate.
    final children = <Widget>[];
    for (final section in sections) {
      children.add(_SectionHeader(
        title: section.title,
        onSeeAll: () => onSeeAll(section.tabType),
      ));
      for (final item in section.items) {
        children.add(SearchResultItem(
          result: item,
          onTap: () => onItemTap(item),
        ));
        children.add(Divider(
          height: 1,
          color: dividerColor,
        ));
      }
    }
    // No trailing divider after the very last row.
    if (children.isNotEmpty) children.removeLast();

    return ListView(
      padding: const EdgeInsets.only(bottom: 16),
      children: children,
    );
  }
}

class _SectionHeader extends StatelessWidget {
  final String title;
  final VoidCallback onSeeAll;

  const _SectionHeader({required this.title, required this.onSeeAll});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 8, 4),
      child: Row(
        children: [
          Expanded(
            child: Text(
              title,
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w700,
                color: isDark
                    ? AppColors.neutralGray100
                    : AppColors.neutralGray900,
              ),
            ),
          ),
          TextButton(
            key: ValueKey('seeAll-$title'),
            onPressed: onSeeAll,
            style: TextButton.styleFrom(
              foregroundColor: AppColors.primary,
              visualDensity: VisualDensity.compact,
              padding: const EdgeInsets.symmetric(horizontal: 12),
            ),
            child: const Text('Lihat Semua'),
          ),
        ],
      ),
    );
  }
}
