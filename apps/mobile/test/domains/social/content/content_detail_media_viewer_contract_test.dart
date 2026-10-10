/// CONTENT MEDIA FULLSCREEN RENDER CONTRACT
///
/// Pins the canonical Content media rendering path on the detail fullscreen
/// viewer (`MediaViewerWidget`):
///   backend-resolved CloudFront URL
///     → `MediaEntity.originalUrl`
///     → `MediaViewerWidget`
///     → image: `AppImage` (URL as-is)
///     → video: `MediaViewerVideoPlayer` (never the image widget).
///
/// NEGATIVE PROOF: the render decision comes from `MediaEntity.type`, never
/// from a URL file extension.
library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/shared/widgets/app_image.dart';
import 'package:hishumi/shared/widgets/media_viewer_video_player.dart';
import 'package:hishumi/shared/widgets/media_viewer_widget.dart';

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

void main() {
  testWidgets('fullscreen viewer renders the backend URL as-is', (tester) async {
    const backendUrl =
        'https://d358tu61i1wrtt.cloudfront.net/images/1749600000002_detail.jpg';

    await tester.pumpWidget(
      _wrap(
        MediaViewerWidget(
          media: [
            _media(url: backendUrl, type: MediaType.image, position: 0),
          ],
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(AppImage), findsWidgets);
    final appImage = tester.widget<AppImage>(find.byType(AppImage).first);
    expect(appImage.imageUrl, backendUrl);
  });

  testWidgets('a video entity renders through the video primitive and never '
      'reaches the image widget', (tester) async {
    const videoUrl =
        'https://d358tu61i1wrtt.cloudfront.net/videos/media';

    await tester.pumpWidget(
      _wrap(
        MediaViewerWidget(
          media: [_media(url: videoUrl, type: MediaType.video, position: 0)],
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(MediaViewerVideoPlayer), findsOneWidget);
    expect(find.byType(AppImage), findsNothing);
  });

  testWidgets('an image entity whose URL has a video-looking suffix is still '
      'an image (type, not extension, is the authority)', (
    tester,
  ) async {
    const imageUrl =
        'https://d358tu61i1wrtt.cloudfront.net/images/poster.mp4';

    await tester.pumpWidget(
      _wrap(
        MediaViewerWidget(
          media: [_media(url: imageUrl, type: MediaType.image, position: 0)],
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(MediaViewerVideoPlayer), findsNothing);
    final appImage = tester.widget<AppImage>(find.byType(AppImage).first);
    expect(appImage.imageUrl, imageUrl);
  });

  group('detail hero frame contract (4:5 contain, same as card)', () {
    final source = File(
      'lib/domains/social/content/presentation/screens/content_detail_screen.dart',
    ).readAsStringSync();

    test('hero uses 4:5 aspect, never a fixed crop box', () {
      expect(source.contains('aspectRatio: 4 / 5'), isTrue);
      expect(source.contains('height: 300'), isFalse);
    });

    test('hero never crops (cover is forbidden on koi media)', () {
      expect(
        RegExp(r'fit:\s*BoxFit\.cover').hasMatch(source),
        isFalse,
        reason: 'cover crops fins — hero renders contain like every surface',
      );
    });
  });
}

