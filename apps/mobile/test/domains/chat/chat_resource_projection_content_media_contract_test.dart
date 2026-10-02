/// CHAT CONTENT SHARED-REFERENCE MEDIA RENDER CONTRACT
///
/// Pins the canonical Content media rendering path on the chat resource
/// projection card:
///   persisted `content_media.media_type`
///     → `mediaref.MediaRef.Kind`
///     → `ResourceMediaRef.kind`
///     → `ResourceMediaRef.mediaKind`
///     → image: `CommerceMarketplaceCardMedia` / `AppImage`
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
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_resource_projection_card.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/shared/widgets/carousel_video_player.dart';
import 'package:visibility_detector/visibility_detector.dart';

/// The backend URL handed to the canonical widget in the tree.
List<String?> _canonicalWidgetUrls(WidgetTester tester) {
  return tester
      .widgetList<AppImage>(find.byType(AppImage))
      .map((w) => w.imageUrl)
      .toList();
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
  // CarouselVideoPlayer is visibility-aware; zero the detector interval so
  // no timer is pending at teardown (repo-wide test convention).
  VisibilityDetectorController.instance.updateInterval = Duration.zero;
  final projection = ResourceProjection.fromJson(_contentLiveJson(media));
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
    const backendUrl =
        'https://d358tu61i1wrtt.cloudfront.net/images/1749600000005_chat.jpg';

    await _pumpProjection(tester, [
      {'url': backendUrl, 'kind': 'image'},
    ]);

    expect(find.byType(AppImage), findsWidgets);
    expect(find.byType(CarouselVideoPlayer), findsNothing);

    expect(_canonicalWidgetUrls(tester), contains(backendUrl));
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
    expect(
      find.byType(AppImage),
      findsNothing,
      reason: 'a video reference must not be handed to the image widget',
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
      find.byType(AppImage),
      findsNothing,
      reason: 'no image widget may target a video reference',
    );
  });

  testWidgets('kind=image with a video-looking suffix is still an image '
      '(kind, not extension, is the authority)', (tester) async {
    const imageUrl = 'https://cdn.example.com/content/poster.mp4';

    await _pumpProjection(tester, [
      {'url': imageUrl, 'kind': 'image'},
    ]);

    expect(find.byType(CarouselVideoPlayer), findsNothing);
    expect(_canonicalWidgetUrls(tester), contains(imageUrl));
  });

  testWidgets('content with no media renders neither image nor video '
      'player', (tester) async {
    await _pumpProjection(tester, const []);

    expect(find.byType(CarouselVideoPlayer), findsNothing);
    expect(find.byType(AppImage), findsNothing);
  });

  testWidgets('the first media entry is the card media (ordering authority)', (
    tester,
  ) async {
    const first =
        'https://d358tu61i1wrtt.cloudfront.net/images/1749600000006_first.jpg';
    const second =
        'https://d358tu61i1wrtt.cloudfront.net/images/1749600000007_second.jpg';

    await _pumpProjection(tester, [
      {'url': first, 'kind': 'image'},
      {'url': second, 'kind': 'image'},
    ]);

    final urls = _canonicalWidgetUrls(tester);
    expect(urls, contains(first));
    expect(urls, isNot(contains(second)));
  });
}
