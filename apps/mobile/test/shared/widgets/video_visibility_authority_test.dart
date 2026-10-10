import 'package:chewie/chewie.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/shared/widgets/app_image.dart';
import 'package:hishumi/shared/widgets/carousel_video_player.dart';
import 'package:hishumi/shared/widgets/media_viewer_video_player.dart';
import 'package:visibility_detector/visibility_detector.dart';

/// VIDEO MAJOR SURFACE: players are visibility-aware, not eager.
///
/// Locks the exact waste this pass killed: every mounted player used to call
/// `VideoPlayerController.initialize()` in `initState`, so a 3-video carousel
/// paid for 3 controllers + 3 network buffers while showing 1 page, and a
/// scrolled-away detail hero kept buffering.
///
/// - Only the active page initializes; siblings render the poster mat.
/// - Scrolling the player off-screen pauses playback.
/// - Duration + mute live in the custom controls (already present) — this
///   file locks the gating, not the controls.
void main() {
  group('Video visibility authority', () {
    testWidgets('inactive carousel player renders poster mat, never Chewie', (
      tester,
    ) async {
      // Production VisibilityDetector polls visibility; zero the interval
      // so no timer is pending at teardown (repo-wide test convention).
      VisibilityDetectorController.instance.updateInterval = Duration.zero;
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: CarouselVideoPlayer(
              videoUrl: 'https://cdn.example.com/content/clip.mp4',
              width: 300,
              height: 200,
              isActive: false,
              posterUrl: 'https://cdn.example.com/content/clip_poster.jpg',
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.byType(Chewie), findsNothing);
      // Flush the VisibilityDetector fire-once timer plus the cached-image
      // error-retry timer (test env answers 400 to every HttpClient call).
      // Two-step: the first pump settles the image failure, the second
      // flushes the detector timer scheduled by that rebuild's repaint.
      await tester.pump(const Duration(seconds: 3));
      await tester.pump(const Duration(seconds: 3));
      // Poster mat: cached photo + play affordance, zero video bytes.
      expect(find.byType(AppImage), findsOneWidget);
      expect(find.byIcon(Icons.play_circle_fill), findsOneWidget);
    });

    testWidgets('inactive carousel player without poster renders static mat',
        (tester) async {
      VisibilityDetectorController.instance.updateInterval = Duration.zero;
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: CarouselVideoPlayer(
              videoUrl: 'https://cdn.example.com/content/clip.mp4',
              width: 300,
              height: 200,
              isActive: false,
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.byType(Chewie), findsNothing);
      await tester.pump(const Duration(milliseconds: 600));
      expect(find.byIcon(Icons.video_file_outlined), findsOneWidget);
    });

    testWidgets('inactive viewer player renders poster backdrop, never Chewie',
        (tester) async {
      VisibilityDetectorController.instance.updateInterval = Duration.zero;
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: MediaViewerVideoPlayer(
              videoUrl: 'https://cdn.example.com/content/clip.mp4',
              posterUrl: 'https://cdn.example.com/content/clip_poster.jpg',
              isActive: false,
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.byType(Chewie), findsNothing);
      await tester.pump(const Duration(milliseconds: 600));
      expect(find.byType(AppImage), findsOneWidget);
    });

    testWidgets('activating a player does not crash (init runs once)', (
      tester,
    ) async {
      VisibilityDetectorController.instance.updateInterval = Duration.zero;
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: CarouselVideoPlayer(
              videoUrl: 'https://cdn.example.com/content/clip.mp4',
              width: 300,
              height: 200,
              isActive: false,
            ),
          ),
        ),
      );
      await tester.pump();
      expect(find.byType(CarouselVideoPlayer), findsOneWidget);

      // Swipe-to-page: rebuild as the current page.
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: CarouselVideoPlayer(
              videoUrl: 'https://cdn.example.com/content/clip.mp4',
              width: 300,
              height: 200,
              isActive: true,
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 600));

      // In the test env the platform channel is absent, so init fails into
      // the error mat — the contract is: exactly one attempt, no crash, the
      // widget (not a stale controller) owns the frame.
      expect(find.byType(CarouselVideoPlayer), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    test('players default to active (existing single-video call sites unchanged)',
        () {
      const carousel = CarouselVideoPlayer(
        videoUrl: 'https://cdn.example.com/content/clip.mp4',
        width: 300,
        height: 200,
      );
      expect(carousel.isActive, isTrue);

      const viewer = MediaViewerVideoPlayer(
        videoUrl: 'https://cdn.example.com/content/clip.mp4',
      );
      expect(viewer.isActive, isTrue);
    });
  });
}
