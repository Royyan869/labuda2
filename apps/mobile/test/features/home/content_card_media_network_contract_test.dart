/// CONTENT MEDIA RENDER CONTRACT
///
/// Pins the canonical Content media rendering path on the feed card:
///   backend-resolved CloudFront URL
///     → `MediaEntity.originalUrl`
///     → `FeedCard`
///     → `AppImage` (URL as-is)
///
/// A video media item must never be handed to the image widget; it renders
/// through `CarouselVideoPlayer` (the shared video primitive).
///
/// Proof is at the widget boundary: the backend URL must reach the canonical
/// widget unchanged. HTTP fetching is owned by the cache package.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/shared/widgets/carousel_video_player.dart';

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
      authControllerProvider.overrideWith(
        _FakeUnauthenticatedAuthController.new,
      ),
    ],
    child: MaterialApp(
      home: Scaffold(body: SingleChildScrollView(child: child)),
    ),
  );
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
  testWidgets(
    'content card renders the backend URL through the canonical widget',
    (tester) async {
      const backendUrl =
          'https://d358tu61i1wrtt.cloudfront.net/images/1749600000000_author.jpg';

      await tester.pumpWidget(
        _wrap(
          FeedCard(
            item: _itemWithMedia(
              media: [
                _media(
                  url: backendUrl,
                  type: MediaType.image,
                  position: 0,
                ),
              ],
            ),
          ),
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
    },
  );

  testWidgets('content card renders the thumbnail variant when present', (
    tester,
  ) async {
    const originalUrl =
        'https://d358tu61i1wrtt.cloudfront.net/images/1749600000000_author.jpg';
    const thumbnailUrl =
        'https://d358tu61i1wrtt.cloudfront.net/images/thumbnail/1749600000000_author.jpg';

    await tester.pumpWidget(
      _wrap(
        FeedCard(
          item: _itemWithMedia(
            media: [
              _media(
                url: originalUrl,
                type: MediaType.image,
                position: 0,
              ).copyWith(
                variants: const {'thumbnail': thumbnailUrl},
              ),
            ],
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(AppImage), findsOneWidget);
    final appImage = tester.widget<AppImage>(find.byType(AppImage));
    expect(
      appImage.imageUrl,
      thumbnailUrl,
      reason: 'list surfaces render the Lambda thumbnail, never the original',
    );
  });

  testWidgets('content card video media is never handed to the image widget', (
    tester,
  ) async {
    const videoUrl =
        'https://d358tu61i1wrtt.cloudfront.net/videos/clip.mp4';

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

    expect(find.byType(CarouselVideoPlayer), findsOneWidget);
    expect(
      find.byType(AppImage),
      findsNothing,
      reason: 'a video reference must not reach the image widget',
    );
  });
}
