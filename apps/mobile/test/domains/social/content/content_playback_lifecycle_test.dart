import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/widgets/media_viewer_widget.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

class _FakeVideoEngine implements MediaViewerVideoEngine {
  _FakeVideoEngine({
    required this.mediaId,
    required this.initializeCompleter,
    this.failOnInitialize = false,
  });

  final String mediaId;
  final Completer<void> initializeCompleter;
  bool failOnInitialize;

  int initializeCalls = 0;
  int playCalls = 0;
  int pauseCalls = 0;
  int disposeCalls = 0;
  bool _isPlaying = false;

  @override
  Future<void> initialize() async {
    initializeCalls += 1;
    if (failOnInitialize) {
      throw StateError('initialize failed for $mediaId');
    }
    await initializeCompleter.future;
  }

  @override
  Future<void> play() async {
    playCalls += 1;
    _isPlaying = true;
  }

  @override
  Future<void> pause() async {
    pauseCalls += 1;
    _isPlaying = false;
  }

  @override
  bool get isPlaying => _isPlaying;

  @override
  Widget buildPlayer() {
    return Container(
      key: ValueKey('player-$mediaId'),
      color: Colors.black,
      child: Center(
        child: Text('player-$mediaId', key: ValueKey('label-$mediaId')),
      ),
    );
  }

  @override
  void dispose() {
    disposeCalls += 1;
    _isPlaying = false;
  }
}

class _EngineHarness {
  final engines = <String, _FakeVideoEngine>{};
  final completers = <String, Completer<void>>{};
  final failingIds = <String>{};

  MediaViewerVideoEngine build(MediaEntity media) {
    return engines.putIfAbsent(media.id, () {
      final completer = completers.putIfAbsent(media.id, Completer<void>.new);
      return _FakeVideoEngine(
        mediaId: media.id,
        initializeCompleter: completer,
        failOnInitialize: failingIds.contains(media.id),
      );
    });
  }

  void complete(String mediaId) {
    final completer = completers[mediaId];
    if (completer != null && !completer.isCompleted) {
      completer.complete();
    }
  }

  _FakeVideoEngine engine(String mediaId) => engines[mediaId]!;
}

MediaEntity _image({
  required String id,
  required String url,
  required int position,
}) {
  return MediaEntity(
    id: id,
    originalUrl: url,
    type: MediaType.image,
    position: position,
    createdAt: DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
  );
}

MediaEntity _video({
  required String id,
  required String url,
  required int position,
  String? poster,
}) {
  return MediaEntity(
    id: id,
    originalUrl: url,
    type: MediaType.video,
    position: position,
    createdAt: DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
    variants: poster != null ? {'thumbnail': poster} : const {},
  );
}

Widget _wrapViewer(
  List<MediaEntity> media, {
  required _EngineHarness harness,
  Brightness brightness = Brightness.light,
  RouteObserver<PageRoute<dynamic>>? routeObserver,
}) {
  return MaterialApp(
    theme: ThemeData(brightness: brightness),
    navigatorObservers: routeObserver != null ? [routeObserver] : const [],
    home: Scaffold(
      body: MediaViewerWidget(
        media: media,
        embedded: true,
        videoEngineBuilder: harness.build,
        routeObserver: routeObserver,
      ),
    ),
  );
}

void main() {
  testWidgets('image-only content renders without creating a video engine', (
    tester,
  ) async {
    final harness = _EngineHarness();
    final image = _image(
      id: 'image-a',
      url: 'https://cdn.example.com/content/image-a.jpg',
      position: 0,
    );

    await tester.pumpWidget(
      _wrapViewer([image], harness: harness, brightness: Brightness.light),
    );
    await tester.pumpAndSettle();

    expect(harness.engines, isEmpty);
    expect(find.byType(StableNetworkImage), findsNWidgets(2));
    expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
  });

  testWidgets(
    'video-only content waits for explicit tap and initializes exactly once',
    (tester) async {
      final harness = _EngineHarness();
      final video = _video(
        id: 'video-a',
        url: 'https://cdn.example.com/content/video-a.mp4',
        position: 0,
        poster: 'https://cdn.example.com/content/video-a-poster.jpg',
      );

      await tester.pumpWidget(
        _wrapViewer([video], harness: harness, brightness: Brightness.light),
      );
      await tester.pumpAndSettle();

      expect(harness.engines, isEmpty);
      expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
      expect(find.byType(CircularProgressIndicator), findsNothing);

      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pump();

      expect(harness.engines, hasLength(1));
      expect(harness.engine('video-a').initializeCalls, 1);
      expect(harness.engine('video-a').playCalls, 0);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      harness.complete('video-a');
      await tester.pumpAndSettle();

      expect(harness.engine('video-a').initializeCalls, 1);
      expect(harness.engine('video-a').playCalls, 1);
      expect(harness.engine('video-a').pauseCalls, 0);
      expect(find.byKey(const ValueKey('player-video-a')), findsOneWidget);
    },
  );

  testWidgets(
    'mixed media pauses the active video on swipe and reuses the same controller on return',
    (tester) async {
      final harness = _EngineHarness();
      final image = _image(
        id: 'image-a',
        url: 'https://cdn.example.com/content/image-a.jpg',
        position: 0,
      );
      final video = _video(
        id: 'video-b',
        url: 'https://cdn.example.com/content/video-b.mp4',
        position: 1,
      );

      await tester.pumpWidget(
        _wrapViewer([image, video], harness: harness),
      );
      await tester.pumpAndSettle();

      final pageView = find.byType(PageView);
      await tester.fling(pageView, const Offset(-1000, 0), 1000);
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pump();
      expect(harness.engines, hasLength(1));
      expect(harness.engine('video-b').initializeCalls, 1);

      harness.complete('video-b');
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey('player-video-b')), findsOneWidget);
      expect(harness.engine('video-b').playCalls, 1);

      await tester.fling(pageView, const Offset(1000, 0), 1000);
      await tester.pumpAndSettle();
      expect(harness.engine('video-b').pauseCalls, 1);

      await tester.fling(pageView, const Offset(-1000, 0), 1000);
      await tester.pumpAndSettle();

      expect(harness.engines, hasLength(1));
      expect(harness.engine('video-b').initializeCalls, 1);
      expect(harness.engine('video-b').playCalls, 1);
    },
  );

  testWidgets('app lifecycle and route focus pause the active video', (
    tester,
  ) async {
    final harness = _EngineHarness();
    final routeObserver = RouteObserver<PageRoute<dynamic>>();
    final video = _video(
      id: 'video-c',
      url: 'https://cdn.example.com/content/video-c.mp4',
      position: 0,
      poster: 'https://cdn.example.com/content/video-c-poster.jpg',
    );

    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData(brightness: Brightness.dark),
        navigatorObservers: [routeObserver],
        home: Builder(
          builder: (context) {
            return Scaffold(
              body: Column(
                children: [
                  Expanded(
                    child: MediaViewerWidget(
                      media: [video],
                      embedded: true,
                      videoEngineBuilder: harness.build,
                      routeObserver: routeObserver,
                    ),
                  ),
                  ElevatedButton(
                    onPressed: () {
                      Navigator.of(context).push(
                        MaterialPageRoute<void>(
                          builder: (_) => const Scaffold(
                            body: Text('next route'),
                          ),
                        ),
                      );
                    },
                    child: const Text('push'),
                  ),
                ],
              ),
            );
          },
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pump();
    harness.complete('video-c');
    await tester.pumpAndSettle();

    expect(harness.engine('video-c').playCalls, 1);

    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
    await tester.pump();

    expect(harness.engine('video-c').pauseCalls, 2);

    await tester.tap(find.text('push'));
    await tester.pumpAndSettle();

    expect(harness.engine('video-c').pauseCalls, greaterThanOrEqualTo(3));
  });

  testWidgets(
    'dispose stops stale completion and rebuild preserves the current page',
    (tester) async {
      final harness = _EngineHarness();
      final image = _image(
        id: 'image-a',
        url: 'https://cdn.example.com/content/image-a.jpg',
        position: 0,
      );
      final video = _video(
        id: 'video-d',
        url: 'https://cdn.example.com/content/video-d.mp4',
        position: 1,
      );

      late StateSetter setParentState;
      await tester.pumpWidget(
        MaterialApp(
          home: StatefulBuilder(
            builder: (context, setState) {
              setParentState = setState;
              return Scaffold(
                body: MediaViewerWidget(
                  media: [image, video],
                  embedded: true,
                  videoEngineBuilder: harness.build,
                ),
              );
            },
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.fling(find.byType(PageView), const Offset(-1000, 0), 1000);
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pump();
      expect(harness.engine('video-d').initializeCalls, 1);

      harness.complete('video-d');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('player-video-d')), findsOneWidget);

      setParentState(() {});
      await tester.pumpAndSettle();

      expect(harness.engines, hasLength(1));
      expect(find.byKey(const ValueKey('player-video-d')), findsOneWidget);

      await tester.pumpWidget(const MaterialApp(home: SizedBox.shrink()));
      await tester.pump();
      expect(harness.engine('video-d').disposeCalls, 1);

      await tester.pump();
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'stale initialization completion after dispose stays inert',
    (tester) async {
      final harness = _EngineHarness();
      final video = _video(
        id: 'video-stale',
        url: 'https://cdn.example.com/content/video-stale.mp4',
        position: 0,
      );

      await tester.pumpWidget(_wrapViewer([video], harness: harness));
      await tester.pumpAndSettle();

      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pump();
      expect(harness.engine('video-stale').initializeCalls, 1);

      await tester.pumpWidget(const MaterialApp(home: SizedBox.shrink()));
      await tester.pump();
      expect(harness.engine('video-stale').disposeCalls, 0);

      harness.complete('video-stale');
      await tester.pump();
      expect(tester.takeException(), isNull);
      expect(harness.engine('video-stale').disposeCalls, 1);
      expect(harness.engine('video-stale').playCalls, 0);
      expect(harness.engine('video-stale').pauseCalls, 0);
    },
  );

  testWidgets(
    'failed video only shows error on that item and retry leaves other media intact',
    (tester) async {
      final harness = _EngineHarness()..failingIds.add('video-e');
      final image = _image(
        id: 'image-a',
        url: 'https://cdn.example.com/content/image-a.jpg',
        position: 0,
      );
      final video = _video(
        id: 'video-e',
        url: 'https://cdn.example.com/content/video-e.mp4',
        position: 1,
        poster: 'https://cdn.example.com/content/video-e-poster.jpg',
      );

      await tester.pumpWidget(_wrapViewer([image, video], harness: harness));
      await tester.pumpAndSettle();

      await tester.fling(find.byType(PageView), const Offset(-1000, 0), 1000);
      await tester.pumpAndSettle();

      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(find.text('Video failed to load'), findsOneWidget);
      expect(harness.engine('video-e').initializeCalls, 1);
      expect(harness.engine('video-e').playCalls, 0);

      await tester.fling(find.byType(PageView), const Offset(1000, 0), 1000);
      await tester.pumpAndSettle();
      expect(find.byType(StableNetworkImage), findsNWidgets(2));

      harness.failingIds.clear();
      harness.engine('video-e').failOnInitialize = false;
      await tester.fling(find.byType(PageView), const Offset(-1000, 0), 1000);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Retry'));
      await tester.pump();

      harness.complete('video-e');
      await tester.pumpAndSettle();

      expect(harness.engine('video-e').initializeCalls, 2);
      expect(harness.engine('video-e').playCalls, 1);
      expect(find.byKey(const ValueKey('player-video-e')), findsOneWidget);
    },
  );

  testWidgets('light and dark themes both render the mixed-media viewer', (
    tester,
  ) async {
    final harness = _EngineHarness();
    final image = _image(
      id: 'image-a',
      url: 'https://cdn.example.com/content/image-a.jpg',
      position: 0,
    );
    final video = _video(
      id: 'video-f',
      url: 'https://cdn.example.com/content/video-f.mp4',
      position: 1,
      poster: 'https://cdn.example.com/content/video-f-poster.jpg',
    );

    for (final brightness in [Brightness.light, Brightness.dark]) {
      await tester.pumpWidget(
        _wrapViewer(
          [image, video],
          harness: harness,
          brightness: brightness,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(MediaViewerWidget), findsOneWidget);
      expect(find.byType(PageView), findsOneWidget);
      expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
    }
  });
}
