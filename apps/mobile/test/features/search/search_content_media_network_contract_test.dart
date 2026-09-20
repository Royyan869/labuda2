/// SEARCH CONTENT MEDIA RENDER CONTRACT
///
/// Pins the canonical media rendering path on search result rows:
///   persisted reference (`content_media.media_url`, projected by /search/content)
///     → `SearchResult.imageUrl`
///     → `SearchResultItem`
///     → `StableNetworkImage` / `resolveNetworkImageUrl`
///     → `NetworkImage` request with the *resolved* readable URL.
///
/// NEGATIVE PROOF: the raw persisted storage reference is never handed to the
/// image decoder, and there is no search-local URL builder — the row renders
/// through the same shared widget the converged Content media surfaces use.
library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_result_item.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

/// The canonical CDN/S3 base the resolver prefixes a storage reference with.
String get _mediaBaseUrl => AppConstants.useCloudFront
    ? AppConstants.cdnBaseUrl
    : AppConstants.awsS3BaseUrl;

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

/// Every network URL actually handed to the image decoder in the tree.
List<String> _decodedNetworkUrls(WidgetTester tester) {
  final urls = <String>[];
  for (final image in tester.widgetList<Image>(find.byType(Image))) {
    final provider = image.image;
    final resolved = provider is ResizeImage
        ? provider.imageProvider
        : provider;
    if (resolved is NetworkImage) {
      urls.add(resolved.url);
    }
  }
  return urls;
}

void main() {
  testWidgets('content search row resolves a storage reference through the '
      'canonical image path', (tester) async {
    const storageReference = 'images/1749600000003_search.jpg';

    await tester.pumpWidget(
      _wrap(
        SearchResultItem(result: _contentResult(imageUrl: storageReference)),
      ),
    );
    await tester.pump();

    expect(find.byType(StableNetworkImage), findsOneWidget);

    final urls = _decodedNetworkUrls(tester);
    final expectedResolved = '$_mediaBaseUrl/$storageReference';
    expect(
      urls,
      contains(expectedResolved),
      reason: 'the storage reference must be resolved before decoding',
    );
    expect(
      urls,
      isNot(contains(storageReference)),
      reason: 'the raw persisted reference must never reach the image decoder',
    );
  });

  testWidgets('content search row passes an absolute readable URL through '
      'unchanged', (tester) async {
    const absoluteUrl = 'https://cdn.example.com/content/search-image.jpg';

    await tester.pumpWidget(
      _wrap(SearchResultItem(result: _contentResult(imageUrl: absoluteUrl))),
    );
    await tester.pump();

    expect(_decodedNetworkUrls(tester), contains(absoluteUrl));
  });

  testWidgets('content search row without media renders the placeholder and '
      'decodes nothing', (tester) async {
    await tester.pumpWidget(_wrap(SearchResultItem(result: _contentResult())));
    await tester.pump();

    expect(find.byType(StableNetworkImage), findsNothing);
    expect(_decodedNetworkUrls(tester), isEmpty);
  });

  testWidgets('the thumbnail field is shared by every row type and renders '
      'through the canonical media path', (tester) async {
    // The row thumbnail is ONE field rendered by ONE widget for every row
    // type, so the canonical widget must be its only renderer. (User rows are
    // excluded here only because they mount FollowButton, which requires the
    // auth/api providers — unrelated to media rendering.)
    for (final type in [SearchResultType.forSale, SearchResultType.auction]) {
      await tester.pumpWidget(
        _wrap(
          SearchResultItem(
            result: _result(
              type: type,
              imageUrl: 'images/1749600000004_row.jpg',
              metadata: const {'lifecycle': 'active'},
            ),
          ),
        ),
      );
      await tester.pump();

      expect(
        find.byType(StableNetworkImage),
        findsOneWidget,
        reason: '$type row must render through the canonical widget',
      );
      expect(
        _decodedNetworkUrls(tester),
        contains('$_mediaBaseUrl/images/1749600000004_row.jpg'),
        reason: '$type row must resolve through the canonical resolver',
      );
    }
  });

  test('search result media has no raw decoder, no local URL builder, and no '
      'extension-based media type inference', () {
    final widgetSource = File(
      'lib/features/search/search/presentation/widgets/search_result_item.dart',
    ).readAsStringSync();

    expect(widgetSource, contains('StableNetworkImage('));
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
      widgetSource.contains('.mp4'),
      isFalse,
      reason: 'search must not infer media type from a file extension',
    );

    // FACTUAL CONTRACT: /search/content transports media as URL-only (no
    // media_type), so a search row is thumbnail-only and has no video branch.
    // This is recorded, not bridged: no media-type detector may be invented.
    final dtoSource = File(
      'lib/features/search/search/data/dto/search_dto.dart',
    ).readAsStringSync();
    expect(dtoSource.contains('media_type'), isFalse);
  });
}
