import 'package:labuda/features/search/search/domain/entities/search_result.dart';

/// SECTION-BASED ALL (canonical): All Tab is an overview multi-domain
/// projection, NOT a global cross-domain ranking engine.
///
/// Preview limits per domain. These caps only ever apply to the All
/// overview rendering — they never truncate the domain collections that
/// back the per-type tabs.
abstract final class AllTabPreviewLimits {
  static const int users = 3;
  static const int forSale = 5;
  static const int auctions = 5;
  static const int contents = 5;
}

/// One section of the All overview.
///
/// [tabType] is the domain tab that "Lihat Semua" must open for this
/// section; [items] are the canonical domain results truncated to the
/// section's preview cap, in canonical domain order.
class AllTabSection {
  /// Title shown above the section (matches the corresponding tab label).
  final String title;

  /// Domain tab that "Lihat Semua" navigates to.
  final SearchResultType tabType;

  /// Canonical domain items, capped at the section preview limit.
  final List<SearchResult> items;

  const AllTabSection({
    required this.title,
    required this.tabType,
    required this.items,
  });

  bool get isEmpty => items.isEmpty;
}

List<SearchResult> _preview(List<SearchResult> domainResults, int cap) {
  if (domainResults.length <= cap) return domainResults;
  return domainResults.sublist(0, cap);
}

/// Project the canonical domain collections into All overview sections.
///
/// Order follows the section order: User, For Sale, Auctions, Content.
/// A domain with no results produces NO section (empty domains are simply
/// not shown in the All overview).
List<AllTabSection> buildAllTabSections(UnifiedSearchResults results) {
  final sections = <AllTabSection>[
    AllTabSection(
      title: 'User',
      tabType: SearchResultType.user,
      items: _preview(results.users, AllTabPreviewLimits.users),
    ),
    AllTabSection(
      title: 'For Sale',
      tabType: SearchResultType.forSale,
      items: _preview(results.forSales, AllTabPreviewLimits.forSale),
    ),
    AllTabSection(
      title: 'Auctions',
      tabType: SearchResultType.auction,
      items: _preview(results.auctions, AllTabPreviewLimits.auctions),
    ),
    AllTabSection(
      title: 'Content',
      tabType: SearchResultType.content,
      items: _preview(results.contents, AllTabPreviewLimits.contents),
    ),
  ];
  return sections.where((section) => section.items.isNotEmpty).toList();
}
