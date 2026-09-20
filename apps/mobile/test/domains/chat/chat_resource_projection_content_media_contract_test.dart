/// CHAT CONTENT SHARED-REFERENCE MEDIA RENDER CONTRACT
///
/// Pins the canonical Content media rendering path on the chat resource
/// projection card:
///   persisted `content_media.media_type`
///     → `mediaref.MediaRef.Kind`
///     → `ChatContentMediaRef.kind`
///     → `ChatContentMediaRef.mediaType`
///     → image: `CommerceMarketplaceCardMedia` / `StableNetworkImage`
///     → video: `CarouselVideoPlayer` (the shared video primitive).
///
/// NEGATIVE PROOF: the render decision must come from the transported media
/// kind, never from a URL file extension. A `kind: video` reference with a
/// signature-only URL (no suffix) is therefore still a video, and an `image`
/// reference whose URL happens to end in `.mp4` is still an image. A video
/// reference is never handed to the image decoder.
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_resource_projection.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_resource_projection_card.dart';
import 'package:labuda/shared/widgets/carousel_video_player.dart';
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

Map<String, dynamic> _contentLiveJson(List<Map<String, dynamic>> media) {
  return {
    'state': 'LIVE',
    'resource_type': 'content',
    'resource_id': 'content-resource-1',
    'canonical_url': '/content/content-resource-1',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': false,
      'blocked_by_tombstone': false,
    },
    'content': {
      'caption': 'Konten utama',
      'media': media,
      'lifecycle': 'active',
      'created_at': '2026-09-16T10:11:12Z',
      'author': {
        'id': 'author-1',
        'username': 'author_user',
        'avatar_url': 'https://cdn.example.test/author.png',
        'lifecycle': 'active',
      },
    },
  };
}

Future<void> _pumpProjection(
  WidgetTester tester,
  List<Map<String, dynamic>> media,
) async {
  final projection = ChatResourceProjection.fromJson(_contentLiveJson(media));
  await tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(
          child: ChatResourceProjectionCard(resourceProjection: projection),
        ),
      ),
    ),
  );
  await tester.pump();
}

void main() {
  testWidgets('kind=image renders the canonical image media path', (
    tester,
  ) async {
    const storageReference = 'images/1749600000005_chat.jpg';

    await _pumpProjection(tester, [
      {'url': storageReference, 'kind': 'image'},
    ]);

    expect(find.byType(StableNetworkImage), findsOneWidget);
    expect(find.byType(CarouselVideoPlayer), findsNothing);

    final urls = _decodedNetworkUrls(tester);
    expect(urls, contains('$_mediaBaseUrl/$storageReference'));
    expect(
      urls,
      isNot(contains(storageReference)),
      reason: 'the raw storage reference must never reach the image decoder',
    );
  });

  testWidgets('kind=video renders the canonical video renderer, not the image '
      'decoder', (tester) async {
    // A canonical readable video reference (presigned/CDN read URL) carries no
    // file extension, so an extension sniffer would hand it to the image
    // decoder. The render decision must come from the transported kind.
    const videoUrl =
        'https://cdn.example.com/content/media?X-Amz-Signature=deadbeef';

    await _pumpProjection(tester, [
      {'url': videoUrl, 'kind': 'video'},
    ]);

    expect(find.byType(CarouselVideoPlayer), findsOneWidget);
    expect(find.byType(StableNetworkImage), findsNothing);

    final urls = _decodedNetworkUrls(tester);
    expect(
      urls,
      isNot(contains(videoUrl)),
      reason: 'a video reference must not be handed to the image decoder',
    );
  });

  testWidgets('kind=video with a video-looking suffix still uses the video '
      'renderer', (tester) async {
    const videoUrl = 'https://cdn.example.com/content/clip.mp4';

    await _pumpProjection(tester, [
      {'url': videoUrl, 'kind': 'video'},
    ]);

    expect(find.byType(CarouselVideoPlayer), findsOneWidget);
    expect(
      _decodedNetworkUrls(tester),
      isNot(contains(videoUrl)),
      reason: 'no image request may target a video reference',
    );
  });

  testWidgets('kind=image with a video-looking suffix is still an image '
      '(kind, not extension, is the authority)', (tester) async {
    const imageUrl = 'https://cdn.example.com/content/poster.mp4';

    await _pumpProjection(tester, [
      {'url': imageUrl, 'kind': 'image'},
    ]);

    expect(find.byType(CarouselVideoPlayer), findsNothing);
    expect(_decodedNetworkUrls(tester), contains(imageUrl));
  });

  testWidgets('content with no media renders neither decoder nor video '
      'player', (tester) async {
    await _pumpProjection(tester, const []);

    // The card media is the canonical placeholder: the shared image widget is
    // present but is handed no URL, so no image request is ever issued.
    final images = tester.widgetList<StableNetworkImage>(
      find.byType(StableNetworkImage),
    );
    expect(images.every((image) => image.imageUrl == null), isTrue);
    expect(find.byType(CarouselVideoPlayer), findsNothing);
    expect(_decodedNetworkUrls(tester), isEmpty);
  });

  testWidgets('the first media entry is the card media (ordering authority)', (
    tester,
  ) async {
    const first = 'images/1749600000006_first.jpg';
    const second = 'images/1749600000007_second.jpg';

    await _pumpProjection(tester, [
      {'url': first, 'kind': 'image'},
      {'url': second, 'kind': 'image'},
    ]);

    final urls = _decodedNetworkUrls(tester);
    expect(urls, contains('$_mediaBaseUrl/$first'));
    expect(urls, isNot(contains('$_mediaBaseUrl/$second')));
  });
}
