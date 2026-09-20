/// CONTENT MEDIA FULLSCREEN RENDER CONTRACT
///
/// Pins the canonical Content media rendering path on the detail fullscreen
/// viewer (`MediaViewerWidget`):
///   persisted reference (`content_media.media_url`)
///     → `MediaEntity.originalUrl`
///     → `MediaViewerWidget`
///     → image: `StableNetworkImage` / `resolveNetworkImageUrl`
///     → `NetworkImage` request with the *resolved* readable URL
///     → video: `MediaViewerVideoPlayer` (never the image decoder).
///
/// NEGATIVE PROOF: the render decision must come from `MediaEntity.type`, never
/// from a URL file extension. A `.mp4` reference is therefore never handed to
/// the image decoder even though the old implementation would have sniffed it
/// as a video by suffix, and the raw storage reference is never decoded.
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/widgets/media_viewer_video_player.dart';
import 'package:labuda/shared/widgets/media_viewer_widget.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

/// The canonical CDN/S3 base the resolver prefixes a storage reference with.
String get _mediaBaseUrl => AppConstants.useCloudFront
    ? AppConstants.cdnBaseUrl
    : AppConstants.awsS3BaseUrl;

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

Widget _wrap(Widget child) {
  return MaterialApp(home: child);
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
  testWidgets('fullscreen viewer resolves a storage reference through the '
      'canonical image path', (tester) async {
    const storageReference = 'images/1749600000002_detail.jpg';

    await tester.pumpWidget(
      _wrap(
        MediaViewerWidget(
          media: [
            _media(url: storageReference, type: MediaType.image, position: 0),
          ],
        ),
      ),
    );
    await tester.pump();

    // The image renders through the canonical widget, not a bespoke image.
    expect(find.byType(StableNetworkImage), findsWidgets);

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
  });

  testWidgets('fullscreen viewer passes an absolute readable URL through '
      'unchanged', (tester) async {
    const absoluteUrl = 'https://cdn.example.com/content/detail-image.jpg';

    await tester.pumpWidget(
      _wrap(
        MediaViewerWidget(
          media: [_media(url: absoluteUrl, type: MediaType.image, position: 0)],
        ),
      ),
    );
    await tester.pump();

    expect(_decodedNetworkUrls(tester), contains(absoluteUrl));
  });

  testWidgets('a video entity renders through the video primitive and never '
      'reaches the image decoder', (tester) async {
    // A canonical readable video reference (presigned/CDN read URL) carries no
    // file extension at all, so an extension sniffer would hand this reference
    // straight to the image decoder. The render decision must come from
    // MediaEntity.type.
    const videoUrl =
        'https://cdn.example.com/content/media?X-Amz-Signature=deadbeef';

    await tester.pumpWidget(
      _wrap(
        MediaViewerWidget(
          media: [_media(url: videoUrl, type: MediaType.video, position: 0)],
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(MediaViewerVideoPlayer), findsOneWidget);
    expect(find.byType(StableNetworkImage), findsNothing);

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

  testWidgets('an image entity whose URL has a video-looking suffix is still '
      'decoded as an image (type, not extension, is the authority)', (
    tester,
  ) async {
    const imageUrl = 'https://cdn.example.com/content/poster.mp4';

    await tester.pumpWidget(
      _wrap(
        MediaViewerWidget(
          media: [_media(url: imageUrl, type: MediaType.image, position: 0)],
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(MediaViewerVideoPlayer), findsNothing);
    expect(_decodedNetworkUrls(tester), contains(imageUrl));
  });
}
