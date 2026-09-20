/// CONTENT SHARED-REFERENCE MEDIA RENDER CONTRACT
///
/// Pins the canonical media path for a Content share reference rendered by
/// `ObjectPreviewCard` — the legacy reference branch a chat message falls back
/// to when the server has not projected a canonical resource projection:
///   Content reference preview image
///     → `ShareTargetType.content`
///     → `StableNetworkImage` / `resolveNetworkImageUrl`
///     → `NetworkImage` request with the *resolved* readable URL.
///
/// NEGATIVE PROOF: the raw persisted storage reference must never reach the
/// image decoder. Before convergence this branch used `Image.network` directly,
/// which handed the persisted reference straight to the decoder.
///
/// Commerce (`for_sale` / `auction`) and profile references keep their own
/// rendering path and are asserted here only to pin that they were not changed.
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

/// The canonical CDN/S3 base the resolver prefixes a storage reference with.
String get _mediaBaseUrl => AppConstants.useCloudFront
    ? AppConstants.cdnBaseUrl
    : AppConstants.awsS3BaseUrl;

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

ShareReference _contentReference({String? imageUrl}) {
  return ShareReference(
    targetType: ShareTargetType.content,
    targetId: 'content-1',
    preview: ObjectPreview(
      id: 'content-1',
      type: 'content',
      title: 'Konten utama',
      imageUrl: imageUrl,
      status: 'available',
    ),
  );
}

Widget _card(ShareReference reference) {
  return MaterialApp(
    home: Scaffold(
      body: SingleChildScrollView(child: ObjectPreviewCard(reference: reference)),
    ),
  );
}

void main() {
  testWidgets(
    'content reference resolves a persisted storage reference before decoding',
    (tester) async {
      // A reference as persisted in `content_media.media_url`: no scheme, no
      // authority — exactly what the upload path can produce.
      const storageReference = 'images/1749600000003_shared.jpg';

      await tester.pumpWidget(
        ProviderScope(child: _card(_contentReference(imageUrl: storageReference))),
      );
      await tester.pump();

      // The content branch renders through the shared network-media widget.
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
        reason: 'the raw storage reference must never reach the image decoder',
      );
    },
  );

  testWidgets('content reference passes an absolute readable URL through', (
    tester,
  ) async {
    const absoluteUrl = 'https://cdn.example.com/content/shared-image.jpg';

    await tester.pumpWidget(
      ProviderScope(child: _card(_contentReference(imageUrl: absoluteUrl))),
    );
    await tester.pump();

    expect(_decodedNetworkUrls(tester), contains(absoluteUrl));
  });

  testWidgets(
    'content reference snapshot path (no live preview yet) uses the canonical '
    'media path too',
    (tester) async {
      const storageReference = 'images/1749600000004_snapshot.jpg';

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            // A provider that never resolves keeps the card on its snapshot
            // branch (the state before a live preview has ever been resolved).
            objectPreviewProvider.overrideWith(
              (ref, reference) => Completer<ObjectPreview?>().future,
            ),
          ],
          child: _card(_contentReference(imageUrl: storageReference)),
        ),
      );
      await tester.pump();

      expect(find.byType(StableNetworkImage), findsOneWidget);
      final urls = _decodedNetworkUrls(tester);
      expect(urls, contains('$_mediaBaseUrl/$storageReference'));
      expect(urls, isNot(contains(storageReference)));
    },
  );

  testWidgets('content reference without a preview image decodes nothing', (
    tester,
  ) async {
    await tester.pumpWidget(ProviderScope(child: _card(_contentReference())));
    await tester.pump();

    expect(find.byType(StableNetworkImage), findsNothing);
    expect(_decodedNetworkUrls(tester), isEmpty);
  });

  testWidgets(
    'commerce reference keeps its unchanged direct image decoder path',
    (tester) async {
      final reference = ShareReference.forSale(
        forSaleId: 'sale-1',
        title: 'Koi Premium',
        imageUrl: 'https://cdn.example.com/commerce/sale.jpg',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            // Keep the commerce card on its cached-preview branch so this case
            // is only about the media path, not about live commerce data.
            objectPreviewProvider.overrideWith(
              (ref, objectReference) => Completer<ObjectPreview?>().future,
            ),
          ],
          child: _card(reference),
        ),
      );
      await tester.pump();

      // Commerce references are NOT converged in this scope: they must render
      // exactly as before.
      expect(find.byType(StableNetworkImage), findsNothing);
      expect(
        _decodedNetworkUrls(tester),
        contains('https://cdn.example.com/commerce/sale.jpg'),
      );
    },
  );
}
