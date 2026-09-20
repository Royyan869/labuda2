// SECTION-BASED ALL authority tests (pure projection).
//
// Verifies the canonical All Tab contract:
// - Each domain keeps its own canonical ordering and is projected into its
//   own section — NEVER flattened into one cross-domain list.
// - Users preview ≤ 3; For Sale / Auctions / Content preview ≤ 5.
// - Empty domains produce NO section.
// - Every section carries the domain tab it must open via "Lihat Semua".

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/presentation/utils/all_tab_sections.dart';
import 'package:labuda/features/search/search/presentation/utils/search_result_type_helper.dart';

SearchResult _item(SearchResultType type, String id) {
  return SearchResult(
    id: id,
    type: type,
    title: id,
    createdAt: DateTime.utc(2026, 1, 1),
  );
}

List<SearchResult> _domain(SearchResultType type, int count, String prefix) {
  return List.generate(count, (i) => _item(type, '$prefix-${i + 1}'));
}

UnifiedSearchResults _results({
  List<SearchResult> users = const [],
  List<SearchResult> forSales = const [],
  List<SearchResult> auctions = const [],
  List<SearchResult> contents = const [],
}) {
  return UnifiedSearchResults(
    users: users,
    forSales: forSales,
    auctions: auctions,
    contents: contents,
    totalCount:
        users.length + forSales.length + auctions.length + contents.length,
    query: 'koi',
  );
}

void main() {
  group('buildAllTabSections — per-domain preview caps and ordering', () {
    test(
      'Users >3 available → All section keeps only the first 3 in canonical order',
      () {
        final users = _domain(SearchResultType.user, 6, 'u');
        final sections = buildAllTabSections(
          _results(users: users, contents: _domain(SearchResultType.content, 1, 'c')),
        );

        final userSection = sections.firstWhere(
          (s) => s.tabType == SearchResultType.user,
        );
        expect(userSection.items, hasLength(AllTabPreviewLimits.users));
        expect(
          userSection.items.map((e) => e.id).toList(),
          ['u-1', 'u-2', 'u-3'],
          reason: 'canonical domain order preserved inside the section',
        );
      },
    );

    test(
      'For Sale / Auctions / Content >5 available → All sections keep only 5 each',
      () {
        final sections = buildAllTabSections(
          _results(
            forSales: _domain(SearchResultType.forSale, 9, 'fs'),
            auctions: _domain(SearchResultType.auction, 7, 'a'),
            contents: _domain(SearchResultType.content, 6, 'c'),
          ),
        );

        final forSale = sections.firstWhere(
          (s) => s.tabType == SearchResultType.forSale,
        );
        final auctions = sections.firstWhere(
          (s) => s.tabType == SearchResultType.auction,
        );
        final contents = sections.firstWhere(
          (s) => s.tabType == SearchResultType.content,
        );

        expect(forSale.items, hasLength(AllTabPreviewLimits.forSale));
        expect(forSale.items.map((e) => e.id).toList(),
            ['fs-1', 'fs-2', 'fs-3', 'fs-4', 'fs-5']);
        expect(auctions.items, hasLength(AllTabPreviewLimits.auctions));
        expect(auctions.items.map((e) => e.id).toList(),
            ['a-1', 'a-2', 'a-3', 'a-4', 'a-5']);
        expect(contents.items, hasLength(AllTabPreviewLimits.contents));
        expect(contents.items.map((e) => e.id).toList(),
            ['c-1', 'c-2', 'c-3', 'c-4', 'c-5']);
      },
    );

    test('section items mirror the domain collection (no reordering across domains)', () {
      // Backend "relevance" order can be arbitrary per domain — All must
      // present exactly that order, never a re-sorted union.
      final sections = buildAllTabSections(
        _results(
          users: [_item(SearchResultType.user, 'z-user'), _item(SearchResultType.user, 'a-user')],
          forSales: [_item(SearchResultType.forSale, 'm-forSale')],
        ),
      );
      final userSection = sections.firstWhere(
        (s) => s.tabType == SearchResultType.user,
      );
      expect(userSection.items.map((e) => e.id).toList(), ['z-user', 'a-user']);
      expect(sections.map((s) => s.title).toList(), ['User', 'For Sale']);
    });

    test('domain collections below the cap render in full', () {
      final sections = buildAllTabSections(
        _results(
          users: _domain(SearchResultType.user, 2, 'u'),
          auctions: _domain(SearchResultType.auction, 3, 'a'),
        ),
      );
      final userSection = sections.firstWhere(
        (s) => s.tabType == SearchResultType.user,
      );
      expect(userSection.items, hasLength(2));
    });
  });

  group('buildAllTabSections — empty sections', () {
    test('empty domains produce NO section; other sections stay', () {
      final sections = buildAllTabSections(
        _results(
          users: _domain(SearchResultType.user, 3, 'u'),
          forSales: const [],
          auctions: const [],
          contents: _domain(SearchResultType.content, 2, 'c'),
        ),
      );
      expect(sections.map((s) => s.title).toList(), ['User', 'Content']);
    });

    test('all domains empty → no sections (screen falls back to global empty state)', () {
      final sections = buildAllTabSections(_results());
      expect(sections, isEmpty);
      expect(_results().isEmpty, isTrue);
    });
  });

  group('Lihat Semua — section → domain tab mapping', () {
    test('each section carries the domain tab its "Lihat Semua" must open', () {
      final sections = buildAllTabSections(
        _results(
          users: _domain(SearchResultType.user, 3, 'u'),
          forSales: _domain(SearchResultType.forSale, 5, 'fs'),
          auctions: _domain(SearchResultType.auction, 5, 'a'),
          contents: _domain(SearchResultType.content, 5, 'c'),
        ),
      );

      // Same tab ordering as the SearchResultsScreen tab bar:
      // All(0) For Sale(1) Auctions(2) User(3) Content(4).
      expect(sections.map((s) => s.title).toList(),
          ['User', 'For Sale', 'Auctions', 'Content']);

      final expectations = <SearchResultType, int>{
        SearchResultType.user: 3,
        SearchResultType.forSale: 1,
        SearchResultType.auction: 2,
        SearchResultType.content: 4,
      };
      for (final section in sections) {
        expect(
          SearchResultTypeHelper.getTabIndex(section.tabType),
          expectations[section.tabType],
          reason: '${section.title} Lihat Semua must land on its domain tab',
        );
      }
    });
  });
}
