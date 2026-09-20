/// CONTENT MEDIA RENDER CONTRACT
///
/// Pins the canonical Content media rendering path on the feed card:
///   persisted reference (`content_media.media_url`)
///     → `MediaEntity.originalUrl`
///     → `FeedCard`
///     → `StableNetworkImage` / `resolveNetworkImageUrl`
///     → `NetworkImage` request with the *resolved* readable URL.
///
/// The proof reaches the actual image request boundary: it inspects the
/// `NetworkImage.url` handed to the image decoder — and, for the executed
/// request case, the real HTTP fetch of the resolved URL — rather than merely
/// asserting that a widget type exists. A video media item must never be handed
/// to that decoder; it renders through `CarouselVideoPlayer` (the shared video
/// primitive).
library;

import 'dart:collection';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/widgets/carousel_video_player.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

import '../../support/queued_image_http_client.dart';

/// Only the session state is faked — every downstream widget stays production
/// code. The card footer watches the auth state; unauthenticated keeps the
/// harness free of like/repository dependencies unrelated to media rendering.
class _FakeUnauthenticatedAuthController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();
}

Widget _wrap(Widget child) {
  return ProviderScope(
    overrides: [
      objectPreviewProvider.overrideWith((ref, reference) async => null),
      authControllerProvider.overrideWith(
        _FakeUnauthenticatedAuthController.new,
      ),
    ],
    child: MaterialApp(
      home: Scaffold(body: SingleChildScrollView(child: child)),
    ),
  );
}

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

FeedItem _itemWithMedia({required List<MediaEntity> media}) {
  return FeedItem(
    id: 'content-media-1',
    content: 'content with media',
    authorId: 'author-1',
    authorUsername: 'author',
    type: FeedItemType.content,
    createdAt: DateTime.utc(2026, 9, 16, 10, 0),
    media: media,
    additionalData: const {'status': 'active'},
  );
}

MediaEntity _media({
  required String url,
  required MediaType type,
  required int position,
}) {
  return MediaEntity(
    id: 'media-$position',
    originalUrl: url,
    type: type,
    position: position,
    createdAt: DateTime.utc(2026, 9, 16),
  );
}

void main() {
  // EXECUTED REQUEST PROOF — deliberately first so no earlier case can leave
  // image-cache or in-flight state that would mask the fetch this proves.
  //
  // Scope note: this proves the request actually leaves the widget at the
  // resolved readable URL. Rendering a *decoded* frame cannot be observed in
  // this harness (the image stream does not complete inside the widget-test
  // fake-async zone — the same limit the existing commerce media contract test
  // works around by asserting only the failure state).
  testWidgets(
    'content card performs the HTTP fetch for the resolved read URL',
    (tester) async {
      const storageReference = 'images/1749600000001_request_proof.jpg';
      final resolvedUrl = '$_mediaBaseUrl/$storageReference';
      final httpClient = QueuedImageHttpClient(
        <String, Queue<QueuedImageResponseSpec>>{
          resolvedUrl: Queue<QueuedImageResponseSpec>.of([
            QueuedImageResponseSpec.success(onePxPngBytes),
          ]),
        },
      );

      await HttpOverrides.runZoned(() async {
        await tester.pumpWidget(
          _wrap(
            FeedCard(
              item: _itemWithMedia(
                media: [
                  _media(
                    url: storageReference,
                    type: MediaType.image,
                    position: 0,
                  ),
                ],
              ),
            ),
          ),
        );
        await tester.pump();

        // Let the mocked fetch and the image codec run on the real event loop,
        // then paint the result.
        await tester.runAsync(
          () => Future<void>.delayed(const Duration(milliseconds: 200)),
        );
        await tester.pumpAndSettle();

        // The readable URL was actually requested over HTTP, exactly once, and
        // the queued response was consumed by the card's image load.
        expect(
          httpClient.requestCounts[resolvedUrl],
          1,
          reason: 'the card must perform the request for the resolved URL',
        );
        expect(
          httpClient.requestCounts[storageReference],
          isNull,
          reason: 'the raw storage reference must never be requested',
        );
        expect(_decodedNetworkUrls(tester), contains(resolvedUrl));
      }, createHttpClient: (_) => httpClient);
    },
  );

  testWidgets(
    'content card requests the resolved read URL, never the raw storage reference',
    (tester) async {
      // A storage reference as persisted in content_media.media_url — no
      // scheme, no authority, exactly what the upload path can produce.
      const storageReference = 'images/1749600000000_author.jpg';

      await tester.pumpWidget(
        _wrap(
          FeedCard(
            item: _itemWithMedia(
              media: [
                _media(
                  url: storageReference,
                  type: MediaType.image,
                  position: 0,
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      // The card renders through the canonical widget, not a bespoke image.
      expect(find.byType(StableNetworkImage), findsOneWidget);

      final urls = _decodedNetworkUrls(tester);
      expect(urls, isNotEmpty, reason: 'the image decoder must be reached');

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

  testWidgets(
    'content card passes an absolute readable URL through unchanged',
    (tester) async {
      const absoluteUrl = 'https://cdn.example.com/content/absolute-image.jpg';

      await tester.pumpWidget(
        _wrap(
          FeedCard(
            item: _itemWithMedia(
              media: [
                _media(url: absoluteUrl, type: MediaType.image, position: 0),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      final urls = _decodedNetworkUrls(tester);
      expect(urls, contains(absoluteUrl));
    },
  );

  testWidgets('content card video media is never handed to the image decoder', (
    tester,
  ) async {
    const videoUrl = 'https://cdn.example.com/content/clip.mp4';

    await tester.pumpWidget(
      _wrap(
        FeedCard(
          item: _itemWithMedia(
            media: [_media(url: videoUrl, type: MediaType.video, position: 0)],
          ),
        ),
      ),
    );
    await tester.pump();

    // Video renders through the canonical video primitive.
    expect(find.byType(CarouselVideoPlayer), findsOneWidget);

    // No image request for the video reference — a .mp4 must never be
    // decoded as an image (that is the placeholder defect this pins).
    final urls = _decodedNetworkUrls(tester);
    expect(
      urls,
      isNot(contains(videoUrl)),
      reason: 'a video reference must not be handed to the image decoder',
    );
    expect(
      urls.where((url) => url.endsWith('.mp4')),
      isEmpty,
      reason: 'no image request may target a video file',
    );
  });
}
