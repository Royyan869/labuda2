/// SEARCH CONTENT MEDIA RENDER CONTRACT
///
/// Pins the canonical media rendering path on search result rows:
///   backend-resolved CloudFront URL (`/search/content`)
///     → `SearchResult.imageUrl`
///     → `SearchResultItem`
///     → `AppImage` (URL as-is).
///
/// NEGATIVE PROOF: mobile builds no readable URL — the row renders the
/// backend URL unchanged through the single cached widget.
library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_result_item.dart';
import 'package:labuda/shared/widgets/app_image.dart';

Widget _wrap(Widget child) {
  return ProviderScope(
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

SearchResult _result({
  required SearchResultType type,
  String? imageUrl,
  Map<String, dynamic> metadata = const {},
}) {
  return SearchResult(
    id: 'search-row-1',
    type: type,
    title: 'Search row',
    subtitle: 'author',
    imageUrl: imageUrl,
    metadata: metadata,
    createdAt: DateTime.utc(2026, 9, 16, 10, 0),
  );
}

SearchResult _contentResult({String? imageUrl}) {
  return _result(
    type: SearchResultType.content,
    imageUrl: imageUrl,
    metadata: const {'lifecycle': 'active', 'authorLifecycle': 'active'},
  );
}

void main() {
  testWidgets('content search row renders the backend URL as-is',
      (tester) async {
    const backendUrl =
        'https://d358tu61i1wrtt.cloudfront.net/images/1749600000003_search.jpg';

    await tester.pumpWidget(
      _wrap(
        SearchResultItem(result: _contentResult(imageUrl: backendUrl)),
      ),
    );
    await tester.pump();

    expect(find.byType(AppImage), findsOneWidget);
    final appImage = tester.widget<AppImage>(find.byType(AppImage));
    expect(
      appImage.imageUrl,
      backendUrl,
      reason: 'the backend URL must reach the canonical widget unchanged',
    );
  });

  testWidgets('content search row without media renders the placeholder and '
      'no image widget', (tester) async {
    await tester.pumpWidget(_wrap(SearchResultItem(result: _contentResult())));
    await tester.pump();

    expect(find.byType(AppImage), findsNothing);
  });

  testWidgets('the thumbnail field is shared by every row type and renders '
      'through the canonical media path', (tester) async {
    for (final type in [SearchResultType.forSale, SearchResultType.auction]) {
      await tester.pumpWidget(
        _wrap(
          SearchResultItem(
            result: _result(
              type: type,
              imageUrl:
                  'https://d358tu61i1wrtt.cloudfront.net/images/1749600000004_row.jpg',
              metadata: const {'lifecycle': 'active'},
            ),
          ),
        ),
      );
      await tester.pump();

      expect(
        find.byType(AppImage),
        findsOneWidget,
        reason: '$type row must render through the canonical widget',
      );
      final appImage = tester.widget<AppImage>(find.byType(AppImage));
      expect(
        appImage.imageUrl,
        'https://d358tu61i1wrtt.cloudfront.net/images/1749600000004_row.jpg',
        reason: '$type row must render the backend URL unchanged',
      );
    }
  });

  test('search result media has no raw decoder, no local URL builder, and no '
      'extension-based media type inference', () {
    final widgetSource = File(
      'lib/features/search/search/presentation/widgets/search_result_item.dart',
    ).readAsStringSync();

    expect(widgetSource, contains('AppImage('));
    expect(
      widgetSource.contains('Image.network'),
      isFalse,
      reason: 'the raw decoder must not bypass the canonical media path',
    );
    expect(
      widgetSource.contains('cdnBaseUrl'),
      isFalse,
      reason: 'search must not build readable URLs locally',
    );
    expect(
      widgetSource.contains('awsS3BaseUrl'),
      isFalse,
      reason: 'search must not build readable URLs locally',
    );
    expect(
      widgetSource.contains('StableNetworkImage'),
      isFalse,
      reason: 'StableNetworkImage was purged; use AppImage',
    );
    expect(
      widgetSource.contains('.mp4'),
      isFalse,
      reason: 'search must not infer media type from a file extension',
    );

    final dtoSource = File(
      'lib/features/search/search/data/dto/search_dto.dart',
    ).readAsStringSync();
    expect(dtoSource.contains('media_type'), isFalse);
  });
}
