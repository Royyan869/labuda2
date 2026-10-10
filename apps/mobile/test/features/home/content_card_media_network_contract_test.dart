/// CONTENT MEDIA RENDER CONTRACT
///
/// Pins the canonical Content media rendering path on the feed card:
///   backend-resolved CloudFront URL
///     → `MediaEntity.originalUrl`
///     → `FeedCard`
///     → `FeedMediaMosaic` (1 full, 2 side-by-side, 3 big+stacked, 4+ grid+N)
///     → `AppImage` (thumbnail variant as-is, contain — never cropped)
///
/// A video tile must never hand its reference to the image widget; it shows
/// a play badge instead. Taps bubble to the card (detail owns routing).
///
/// Proof is at the widget boundary: the backend URL must reach the canonical
/// widget unchanged. HTTP fetching is owned by the cache package.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/features/home/domain/entities/feed_item.dart';
import 'package:hishumi/features/home/presentation/providers/feed_renderers.dart';
import 'package:hishumi/features/home/presentation/widgets/feed_media_mosaic.dart';
import 'package:hishumi/shared/widgets/app_image.dart';

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
  String? thumbnailUrl,
}) {
  return MediaEntity(
    id: 'media-$position',
    originalUrl: url,
    type: type,
    position: position,
    createdAt: DateTime.utc(2026, 9, 16),
    variants: {
      if (thumbnailUrl != null) 'thumbnail': thumbnailUrl,
    },
  );
}

void main() {
  testWidgets(
    'single image renders the thumbnail variant through the mosaic',
    (tester) async {
      const thumbnailUrl =
          'https://d358tu61i1wrtt.cloudfront.net/images/medium/1749600000000_author.jpg';

      await tester.pumpWidget(
        _wrap(
          FeedCard(
            item: _itemWithMedia(
              media: [
                _media(
                  url:
                      'https://d358tu61i1wrtt.cloudfront.net/images/1749600000000_author.jpg',
                  type: MediaType.image,
                  position: 0,
                  thumbnailUrl: thumbnailUrl,
                ),
              ],
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.byType(FeedMediaMosaic), findsOneWidget);
      expect(find.byType(AppImage), findsOneWidget);
      final appImage = tester.widget<AppImage>(find.byType(AppImage));
      expect(
        appImage.imageUrl,
        thumbnailUrl,
        reason: 'list surfaces render the Lambda variant, never the original',
      );
      expect(appImage.fit, BoxFit.contain);
    },
  );

  testWidgets('two images render side by side, both visible', (tester) async {
    await tester.pumpWidget(
      _wrap(
        FeedCard(
          item: _itemWithMedia(
            media: [
              _media(
                url: 'https://d358tu61i1wrtt.cloudfront.net/images/a.jpg',
                type: MediaType.image,
                position: 0,
              ),
              _media(
                url: 'https://d358tu61i1wrtt.cloudfront.net/images/b.jpg',
                type: MediaType.image,
                position: 1,
              ),
            ],
          ),
        ),
      ));
      await tester.pump();

      expect(find.byType(FeedMediaMosaic), findsOneWidget);
      expect(find.byType(AppImage), findsNWidgets(2));
    },
  );

  testWidgets('five media render a 2x2 grid with a +N overlay', (tester) async {
    await tester.pumpWidget(
      _wrap(
        FeedCard(
          item: _itemWithMedia(
            media: List.generate(
              5,
              (i) => _media(
                url: 'https://d358tu61i1wrtt.cloudfront.net/images/$i.jpg',
                type: MediaType.image,
                position: i,
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(FeedMediaMosaic), findsOneWidget);
    expect(find.byType(AppImage), findsNWidgets(4));
    expect(find.text('+1'), findsOneWidget);
    expect(find.text('5 media'), findsOneWidget);
  });

  testWidgets('video tile renders the poster frame, never the mp4 bytes', (
    tester,
  ) async {
    const videoUrl =
        'https://d358tu61i1wrtt.cloudfront.net/videos/clip.mp4';
    const posterUrl =
        'https://d358tu61i1wrtt.cloudfront.net/videos/clip_poster.jpg';

    await tester.pumpWidget(
      _wrap(
        FeedCard(
          item: _itemWithMedia(
            media: [
              _media(
                url: videoUrl,
                type: MediaType.video,
                position: 0,
                thumbnailUrl: posterUrl,
              ),
            ],
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(FeedMediaMosaic), findsOneWidget);
    // Poster through the image widget, play badge on top, mp4 never decoded.
    final appImage = tester.widget<AppImage>(find.byType(AppImage));
    expect(appImage.imageUrl, posterUrl);
    expect(find.byIcon(Icons.play_circle_fill), findsOneWidget);
  });
}
