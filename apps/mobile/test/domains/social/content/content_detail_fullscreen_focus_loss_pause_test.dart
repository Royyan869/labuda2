import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/observability/screen_view_route_observer.dart';

import 'package:labuda/domains/social/content/data/content_providers.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/data/mappers/content_mapper.dart';

import 'package:labuda/domains/social/content/presentation/providers/content_notifier.dart';
import 'package:labuda/domains/social/content/presentation/screens/content_detail_screen.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/widgets/media_viewer_widget.dart';

// ============================================================================
// Fake Video Engine & Counter
// ============================================================================

class _FakeVideoEngine implements MediaViewerVideoEngine {
  _FakeVideoEngine({required this.mediaId, this.failOnPause = false});

  final String mediaId;
  bool failOnPause;

  int initializeCalls = 0;
  int playCalls = 0;
  int pauseCalls = 0;
  int disposeCalls = 0;
  bool _isPlaying = false;

  @override
  Future<void> initialize() async {
    initializeCalls += 1;
  }

  @override
  Future<void> play() async {
    playCalls += 1;
    _isPlaying = true;
  }

  @override
  Future<void> pause() async {
    pauseCalls += 1;
    if (failOnPause) {
      throw StateError('pause failed for $mediaId');
    }
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

class _EngineCounter {
  final List<_FakeVideoEngine> engines = [];
  final Set<String> failingPauseIds = {};

  int get instanceCount => engines.length;

  MediaViewerVideoEngine build(MediaEntity media) {
    final engine = _FakeVideoEngine(
      mediaId: media.id,
      failOnPause: failingPauseIds.contains(media.id),
    );
    engines.add(engine);
    return engine;
  }

  _FakeVideoEngine engineFor(String mediaId) {
    return engines.lastWhere((e) => e.mediaId == mediaId);
  }

  _FakeVideoEngine? engineForOrNull(String mediaId) {
    try {
      return engineFor(mediaId);
    } catch (_) {
      return null;
    }
  }

  void reset() {
    engines.clear();
  }
}

// ============================================================================
// Fake Analytics (minimal — ScreenViewRouteObserver only calls logEvent)
// ============================================================================

class _FakeAnalyticsRepository implements IAnalyticsRepository {
  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async => Result.success(null);

  @override
  Future<Result<void>> logUserAction(
    String action,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

  @override
  Future<Result<void>> logCircumventionAttempt(
    String content,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

  @override
  Future<Result<void>> setUserProperties(
    Map<String, dynamic> properties,
  ) async => Result.success(null);

  @override
  Future<Result<AnalyticsCircumventionStats>> getCircumventionStats({
    required DateTime startDate,
    required DateTime endDate,
    String? userId,
    String? violationType,
  }) async => Result.success(
    AnalyticsCircumventionStats(
      totalAttempts: 0,
      uniqueUsers: 0,
      violationTypes: {},
      dailyAttempts: {},
      averageConfidence: 0,
      blockedAttempts: 0,
      filteredAttempts: 0,
    ),
  );

  @override
  Future<Result<void>> flush() async => Result.success(null);
}

// ============================================================================
// Test Fixture Helpers
// ============================================================================

Map<String, dynamic> _contentJson({
  required String id,
  required String caption,
  required String authorId,
  String? authorUsername,
  String? authorAvatar,
  required String authorLifecycle,
  List<Map<String, dynamic>> media = const [],
  Map<String, dynamic>? shareReference,
}) {
  final author = <String, dynamic>{
    'id': authorId,
    'lifecycle': authorLifecycle,
  };
  if (authorUsername != null) {
    author['username'] = authorUsername;
  }
  if (authorAvatar != null) {
    author['avatar_url'] = authorAvatar;
  }

  return <String, dynamic>{
    'id': id,
    'caption': caption,
    'author_city': null,
    'author_province': null,
    'lifecycle': 'active',
    'visibility': 'public',
    'media': media,
    'tags': <String>[],
    'location': null,
    'engagement': <String, dynamic>{
      'viewCount': 0,
      'likeCount': 0,
      'commentCount': 0,
      'shareCount': 0,
      'saveCount': 0,
      'reportCount': 0,
    },
    'moderation_info': null,
    'published_at': null,
    'created_at': '2026-07-23T00:00:00.000Z',
    'updated_at': '2026-07-23T00:00:00.000Z',
    'is_liked': null,
    'is_saved': null,
    'original_author_id': null,
    'share_reference': shareReference,
    'card': <String, dynamic>{'id': id, 'author': author},
  };
}

Content _contentFromJson(Map<String, dynamic> json) {
  return ContentMapper.toEntity(ContentDto.fromJson(json));
}

Content _contentWithAuthor({
  required String id,
  required String caption,
  required String authorId,
  String? authorUsername,
  String? authorAvatar,
  required String authorLifecycle,
  List<Map<String, dynamic>> media = const [],
  Map<String, dynamic>? shareReference,
}) {
  return _contentFromJson(
    _contentJson(
      id: id,
      caption: caption,
      authorId: authorId,
      authorUsername: authorUsername,
      authorAvatar: authorAvatar,
      authorLifecycle: authorLifecycle,
      media: media,
      shareReference: shareReference,
    ),
  );
}

Map<String, dynamic> _imageMedia({
  required String id,
  required String url,
  int position = 0,
}) {
  return <String, dynamic>{
    'id': id,
    'url': url,
    'type': 'image',
    'position': position,
    'thumbnailUrl': url,
  };
}

Map<String, dynamic> _videoMedia({
  required String id,
  required String url,
  String? posterUrl,
  int position = 0,
}) {
  return <String, dynamic>{
    'id': id,
    'url': url,
    'type': 'video',
    'position': position,
    'thumbnailUrl': posterUrl ?? url,
  };
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

// ============================================================================
// Fake Content Dependencies
// ============================================================================

class _FakeContentRepository implements ContentRepository {
  _FakeContentRepository(this._responses);

  final List<ContentRepositoryResult<Content>> _responses;
  int getByIdCalls = 0;

  @override
  Future<ContentRepositoryResult<Content>> getContentById(
    String contentId,
  ) async {
    getByIdCalls += 1;
    final index = getByIdCalls - 1;
    return _responses[index < _responses.length
        ? index
        : _responses.length - 1];
  }

  @override
  Future<ContentRepositoryResult<Content>> createContent({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    List<MediaEntity> media = const [],
    List<String> tags = const [],
    List<String> taggedUsers = const [],
    ContentSettings settings = const ContentSettings(),
    ContentLocation? location,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<void>> deleteContent(String contentId) async =>
      ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
    String? location,
    ContentStatus? status,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByAuthor(
    String authorId, {
    int? limit,
    int? offset,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByLocation({
    required String location,
    int? limit,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getTrendingContents({
    int? limit,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  ) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<ContentSearchResult>> searchContents({
    required String query,
    int? limit,
    int? offset,
    String? location,
  }) async => ContentRepositoryResult.error('not used');

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAuthRepository implements IAuthRepository {
  _FakeAuthRepository(this._user);
  final AuthUser _user;

  @override
  Future<Result<AuthUser?>> getUserById(String userId) async {
    return Result.success(_user);
  }

  @override
  Future<Result<List<AuthUser>>> searchUsers({
    required String query,
    int limit = 20,
  }) async => Result.success(const <AuthUser>[]);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeLikeRepository implements LikeRepository {
  _FakeLikeRepository(this.stats);
  final LikeStats stats;

  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async => Result.success(!stats.isLikedByCurrentUser);

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async => Result.success(stats);

  @override
  Future<Result<bool>> hasUserLiked({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async => Result.success(stats.isLikedByCurrentUser);

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) => Stream<LikeStats>.value(stats);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);
  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _NoOpUserApiDatasource extends Fake implements UserApiDatasource {}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpUserApiDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _NoOpPresenceRegistry extends PresenceSubscriptionRegistry {
  @override
  PresenceSubscriptionHandle acquire(Set<String> userIds) {
    return PresenceSubscriptionHandle(() async {});
  }

  @override
  Future<void> prepareForLogout() async {}

  @override
  PresenceState? lookup(String userId) => null;

  @override
  Map<String, PresenceState?> lookupMany(Iterable<String> userIds) => {};

  @override
  Future<void> publishSelfPresence({required bool isOnline}) async {}

  @override
  Future<void> setForeground(bool isForeground) async {}
}

// ============================================================================
// Screen Wrapper (with optional RouteObserver for ContentDetailScreen tests)
// ============================================================================

Widget _wrapContentDetail(
  Content content, {
  AuthState authState = const AuthState.unauthenticated(),
  LikeStats? likeStats,
  MediaViewerVideoEngineBuilder? videoEngineBuilder,
  ScreenViewRouteObserver? routeObserver,
}) {
  final fakeAuthUser = AuthUser(
    id: content.author.id,
    createdAt: DateTime.utc(2026, 7, 23),
    updatedAt: DateTime.utc(2026, 7, 23),
    email: 'author@example.com',
    username: content.author.username ?? 'author',
    avatarUrl: content.author.avatarUrl,
    isEmailVerified: true,
    roles: const [],
    provider: ShonaAuthProvider.email,
    lifecycle: content.author.lifecycle,
  );
  final stats =
      likeStats ??
      LikeStats(
        targetId: content.id,
        targetType: LikeTargetType.content,
        totalLikes: content.engagement.likeCount,
        isLikedByCurrentUser: false,
      );

  return ProviderScope(
    overrides: [
      contentRepositoryProvider.overrideWithValue(
        _FakeContentRepository([ContentRepositoryResult.success(content)]),
      ),
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository(stats)),
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      authRepositoryProvider.overrideWithValue(
        _FakeAuthRepository(fakeAuthUser),
      ),
      avatarCacheServiceProvider.overrideWithValue(_NoOpAvatarCacheService()),
      presenceSubscriptionRegistryProvider.overrideWithValue(
        _NoOpPresenceRegistry(),
      ),
      userOnlineStatusProvider(content.author.id).overrideWithValue(false),
      objectPreviewProvider.overrideWith((ref, reference) async => null),
      if (routeObserver != null)
        screenViewRouteObserverProvider.overrideWithValue(routeObserver),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/',
        observers: routeObserver != null ? [routeObserver] : const [],
        routes: [
          GoRoute(
            path: '/',
            builder: (_, _) => Scaffold(
              body: ContentDetailScreen(
                contentId: 'content-1',
                videoEngineBuilder: videoEngineBuilder,
              ),
            ),
          ),
          GoRoute(
            path: '/user/:id',
            builder: (_, state) =>
                Scaffold(body: Text('profile:${state.pathParameters['id']}')),
          ),
        ],
      ),
    ),
  );
}

// ============================================================================
// Direct Viewer Wrappers
// ============================================================================

/// Fullscreen viewer with route observer for route-push tests.
Widget _wrapFullscreenWithRouteObserver({
  required List<MediaEntity> media,
  required _EngineCounter counter,
  required RouteObserver<PageRoute<dynamic>> routeObserver,
  required WidgetBuilder pushButtonBuilder,
}) {
  return MaterialApp(
    navigatorObservers: [routeObserver],
    home: Builder(
      builder: (context) => Scaffold(
        body: Column(
          children: [
            Expanded(
              child: MediaViewerWidget(
                media: media,
                embedded: false,
                videoEngineBuilder: counter.build,
                routeObserver: routeObserver,
              ),
            ),
            pushButtonBuilder(context),
          ],
        ),
      ),
    ),
  );
}

/// Fullscreen viewer for swipe/app-lifecycle tests (no route observer needed).
Widget _wrapFullscreen({
  required List<MediaEntity> media,
  required _EngineCounter counter,
}) {
  return MaterialApp(
    home: Scaffold(
      body: MediaViewerWidget(
        media: media,
        embedded: false,
        videoEngineBuilder: counter.build,
      ),
    ),
  );
}

// ============================================================================
// Tests
// ============================================================================

void main() {
  // ==========================================================================
  // SCENARIO 1: Swipe video → image pauses
  // From actual ContentDetailScreen: open fullscreen, Play, swipe to image
  // ==========================================================================

  testWidgets('swipe from playing video to image pauses the video', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final routeObserver = ScreenViewRouteObserver(_FakeAnalyticsRepository());
    const vidA = 'https://cdn.example.com/content/video-a.mp4';
    const imgB = 'https://cdn.example.com/content/image-b.jpg';

    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'swipe pause test',
      authorId: '00000000-0000-0000-0000-000000000701',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _videoMedia(id: 'vid-a', url: vidA, posterUrl: vidA, position: 0),
        _imageMedia(id: 'img-b', url: imgB, position: 1),
      ],
    );

    await tester.pumpWidget(
      _wrapContentDetail(
        content,
        videoEngineBuilder: counter.build,
        routeObserver: routeObserver,
      ),
    );
    await tester.pumpAndSettle();

    // Expand → fullscreen at Video A
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();
    expect(find.text('1 / 2'), findsOneWidget);

    // Play Video A
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    expect(counter.engineFor('vid-a').playCalls, 1);
    expect(counter.engineFor('vid-a').pauseCalls, 0);

    // Swipe to Image B (page 1)
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // Video A paused on swipe-away
    expect(counter.engineFor('vid-a').pauseCalls, 1);
    // Play count unchanged
    expect(counter.engineFor('vid-a').playCalls, 1);
    // Image B did not create any engine
    expect(counter.engineForOrNull('img-b'), isNull);
    // Video A engine not replaced
    expect(counter.instanceCount, 1);

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 2: Swipe video → video pauses previous
  // Direct fullscreen: two videos, Play A, swipe to B
  // ==========================================================================

  testWidgets('swipe video-to-video pauses previous and B waits for Play', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
    );
    final vidB = _video(
      id: 'vid-b',
      url: 'https://cdn.example.com/b.mp4',
      position: 1,
    );

    await tester.pumpWidget(
      _wrapFullscreen(media: [vidA, vidB], counter: counter),
    );
    await tester.pumpAndSettle();

    // Play Video A
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    expect(counter.engineFor('vid-a').playCalls, 1);
    expect(counter.engineFor('vid-a').isPlaying, isTrue);

    // Swipe to Video B
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // A paused, B not yet created
    expect(counter.engineFor('vid-a').pauseCalls, 1);
    expect(counter.engineForOrNull('vid-b'), isNull);

    // B shows poster-first — play button visible
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 3: Return to previous page does not create new engine
  // Direct fullscreen: Play A, swipe away, swipe back
  // ==========================================================================

  testWidgets('return to video page reuses same engine without re-init', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
    );
    final imgB = _image(
      id: 'img-b',
      url: 'https://cdn.example.com/b.jpg',
      position: 1,
    );

    await tester.pumpWidget(
      _wrapFullscreen(media: [vidA, imgB], counter: counter),
    );
    await tester.pumpAndSettle();

    // Play Video A
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    final engineA = counter.engineFor('vid-a');
    expect(engineA.initializeCalls, 1);
    expect(engineA.playCalls, 1);
    expect(counter.instanceCount, 1);

    // Swipe to Image B
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();
    expect(engineA.pauseCalls, 1);

    // Swipe back to Video A
    await tester.fling(find.byType(PageView), const Offset(400, 0), 1000);
    await tester.pumpAndSettle();

    // Same engine instance, no re-init, no auto-play
    expect(counter.instanceCount, 1);
    expect(counter.engineFor('vid-a'), same(engineA));
    expect(engineA.initializeCalls, 1);
    // No automatic play on return (explicit-play contract)
    expect(engineA.playCalls, 1);

    // Shows player (engine is alive, not poster)
    expect(find.byKey(const ValueKey('player-vid-a')), findsOneWidget);

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 4: Route push pauses fullscreen video
  // Actual route observer — push transient route on top of fullscreen viewer
  // ==========================================================================

  testWidgets('route push on top of fullscreen pauses active video', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final routeObserver = RouteObserver<PageRoute<dynamic>>();
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
    );

    await tester.pumpWidget(
      _wrapFullscreenWithRouteObserver(
        media: [vidA],
        counter: counter,
        routeObserver: routeObserver,
        pushButtonBuilder: (context) => ElevatedButton(
          onPressed: () {
            Navigator.of(context).push(
              MaterialPageRoute<void>(
                builder: (_) => Scaffold(
                  appBar: AppBar(title: const Text('transient')),
                  body: const Text('transient route'),
                ),
              ),
            );
          },
          child: const Text('push'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Play Video A
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();
    expect(counter.engineFor('vid-a').playCalls, 1);
    final pauseBefore = counter.engineFor('vid-a').pauseCalls;

    // Push transient route
    await tester.tap(find.text('push'));
    await tester.pumpAndSettle();

    // Transient route is visible — video paused via didPushNext
    expect(find.text('transient route'), findsOneWidget);
    expect(counter.engineFor('vid-a').pauseCalls, pauseBefore + 1);
    expect(counter.engineFor('vid-a').disposeCalls, 0);

    // Back from transient route
    await tester.tap(find.byType(BackButton));
    await tester.pumpAndSettle();

    // Player still exists (not disposed), no auto-play
    expect(counter.instanceCount, 1);
    expect(counter.engineFor('vid-a').playCalls, 1);
    expect(counter.engineFor('vid-a').disposeCalls, 0);

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 5: Embedded video pauses when fullscreen opens
  // From actual ContentDetailScreen: Play embedded, tap expand
  // ==========================================================================

  testWidgets('embedded video pauses when fullscreen route opens on top', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final routeObserver = ScreenViewRouteObserver(_FakeAnalyticsRepository());
    const vidUrl = 'https://cdn.example.com/content/video.mp4';

    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'embedded pause test',
      authorId: '00000000-0000-0000-0000-000000000705',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _videoMedia(id: 'vid-1', url: vidUrl, posterUrl: vidUrl, position: 0),
      ],
    );

    await tester.pumpWidget(
      _wrapContentDetail(
        content,
        videoEngineBuilder: counter.build,
        routeObserver: routeObserver,
      ),
    );
    await tester.pumpAndSettle();

    // Play on embedded
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    expect(counter.engineFor('vid-1').playCalls, 1);
    expect(counter.engineFor('vid-1').pauseCalls, 0);
    expect(counter.instanceCount, 1);

    // Tap expand → fullscreen opens on top
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen is open
    expect(find.text('1 / 1'), findsOneWidget);

    // Embedded engine paused via didPushNext (fullscreen route pushed on top)
    expect(counter.engineFor('vid-1').pauseCalls, 1);
    // Fullscreen has NOT created its own engine yet (poster-first)
    // Counter is still 1 (embedded engine only)
    // Embedded engine preserved (not disposed)
    expect(counter.engineFor('vid-1').disposeCalls, 0);

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 6: App inactive pauses
  // Direct fullscreen: Play video, simulate inactive
  // ==========================================================================

  testWidgets('app inactive lifecycle pauses active video', (tester) async {
    final counter = _EngineCounter();
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
    );

    await tester.pumpWidget(_wrapFullscreen(media: [vidA], counter: counter));
    await tester.pumpAndSettle();

    // Play Video A
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();
    expect(counter.engineFor('vid-a').playCalls, 1);
    expect(counter.engineFor('vid-a').pauseCalls, 0);

    // Simulate app inactive
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();

    expect(counter.engineFor('vid-a').pauseCalls, 1);
    expect(counter.engineFor('vid-a').disposeCalls, 0);
    expect(counter.instanceCount, 1);

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 7: App paused pauses
  // Direct fullscreen: Play video, simulate paused
  // ==========================================================================

  testWidgets('app paused lifecycle pauses active video without dispose', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
    );

    await tester.pumpWidget(_wrapFullscreen(media: [vidA], counter: counter));
    await tester.pumpAndSettle();

    // Play Video A
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();
    expect(counter.engineFor('vid-a').playCalls, 1);
    final pauseBefore = counter.engineFor('vid-a').pauseCalls;

    // Simulate app paused
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
    await tester.pump();

    expect(counter.engineFor('vid-a').pauseCalls, pauseBefore + 1);
    // Engine NOT disposed — only pause
    expect(counter.engineFor('vid-a').disposeCalls, 0);
    expect(counter.instanceCount, 1);

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 8: Resume does not auto-play
  // Direct fullscreen: inactive → resume, verify no new engine/play
  // ==========================================================================

  testWidgets('resume from background does not auto-play or create engine', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
    );

    await tester.pumpWidget(_wrapFullscreen(media: [vidA], counter: counter));
    await tester.pumpAndSettle();

    // Play Video A
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();
    expect(counter.engineFor('vid-a').playCalls, 1);

    // Simulate inactive → paused
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
    await tester.pump();

    final pauseCountAfterBackground = counter.engineFor('vid-a').pauseCalls;
    final playCountAfterBackground = counter.engineFor('vid-a').playCalls;
    final instanceCountAfterBackground = counter.instanceCount;

    // Simulate resume
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    await tester.pumpAndSettle();

    // No auto-play
    expect(counter.engineFor('vid-a').playCalls, playCountAfterBackground);
    // No new engine
    expect(counter.instanceCount, instanceCountAfterBackground);
    // No additional pauses
    expect(counter.engineFor('vid-a').pauseCalls, pauseCountAfterBackground);
    // No dispose
    expect(counter.engineFor('vid-a').disposeCalls, 0);

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 9: No engine means no synthetic pause failure
  // Poster-only video: swipe away, push route, background — no crash
  // ==========================================================================

  testWidgets('poster-only video survives all focus-loss paths without crash', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final routeObserver = RouteObserver<PageRoute<dynamic>>();
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
    );
    final imgB = _image(
      id: 'img-b',
      url: 'https://cdn.example.com/b.jpg',
      position: 1,
    );

    await tester.pumpWidget(
      _wrapFullscreenWithRouteObserver(
        media: [vidA, imgB],
        counter: counter,
        routeObserver: routeObserver,
        pushButtonBuilder: (context) => ElevatedButton(
          onPressed: () {
            Navigator.of(context).push(
              MaterialPageRoute<void>(
                builder: (_) => Scaffold(
                  appBar: AppBar(title: const Text('overlay')),
                  body: const Text('overlay body'),
                ),
              ),
            );
          },
          child: const Text('push'),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Video A is poster-only — not played
    expect(counter.instanceCount, 0);

    // Swipe away from Video A to Image B
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();
    expect(counter.instanceCount, 0);

    // Swipe back to Video A
    await tester.fling(find.byType(PageView), const Offset(400, 0), 1000);
    await tester.pumpAndSettle();

    // Push route on top
    await tester.tap(find.text('push'));
    await tester.pumpAndSettle();
    expect(find.text('overlay'), findsOneWidget);

    // Background the app
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
    await tester.pump();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();

    // Still zero engines, zero crashes
    expect(counter.instanceCount, 0);
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 10: Multiple video isolation
  // A and B both played, focus loss isolates pause to correct engine
  // ==========================================================================

  testWidgets('focus loss only pauses the active video, not all videos', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
    );
    final vidB = _video(
      id: 'vid-b',
      url: 'https://cdn.example.com/b.mp4',
      position: 1,
    );

    await tester.pumpWidget(
      _wrapFullscreen(media: [vidA, vidB], counter: counter),
    );
    await tester.pumpAndSettle();

    // Play Video A
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();
    expect(counter.engineFor('vid-a').playCalls, 1);

    // Swipe to Video B
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // A paused, B not yet created
    expect(counter.engineFor('vid-a').pauseCalls, 1);

    // Play Video B
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();
    expect(counter.engineFor('vid-b').playCalls, 1);
    expect(counter.engineFor('vid-b').pauseCalls, 0);

    // Swipe back to Video A
    await tester.fling(find.byType(PageView), const Offset(400, 0), 1000);
    await tester.pumpAndSettle();

    // B paused, A NOT auto-paused (it was already paused)
    expect(counter.engineFor('vid-b').pauseCalls, 1);
    // A still has same pause count (didUpdateWidget with isActive=true does nothing)
    expect(counter.engineFor('vid-a').pauseCalls, 1);

    // Both engines preserved, distinct instances
    expect(counter.instanceCount, 2);
    expect(counter.engineFor('vid-a'), isNot(same(counter.engineFor('vid-b'))));

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // PAUSE ERROR ISOLATION TESTS
  // ==========================================================================

  testWidgets(
    'swipe-away pause failure is isolated — no crash, engine preserved',
    (tester) async {
      final counter = _EngineCounter()..failingPauseIds.add('vid-a');
      final vidA = _video(
        id: 'vid-a',
        url: 'https://cdn.example.com/a.mp4',
        position: 0,
      );
      final imgB = _image(
        id: 'img-b',
        url: 'https://cdn.example.com/b.jpg',
        position: 1,
      );

      await tester.pumpWidget(
        _wrapFullscreen(media: [vidA, imgB], counter: counter),
      );
      await tester.pumpAndSettle();

      // Play Video A
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();
      expect(counter.engineFor('vid-a').playCalls, 1);

      // Swipe to Image B — triggers pause on A, which throws
      await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
      await tester.pumpAndSettle();

      // Pause was called exactly once
      expect(counter.engineFor('vid-a').pauseCalls, 1);
      // Engine NOT disposed despite pause failure
      expect(counter.engineFor('vid-a').disposeCalls, 0);
      // Engine NOT replaced
      expect(counter.instanceCount, 1);
      // No uncaught exception surfaced to test framework
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'route-push pause failure is isolated — route visible, no crash',
    (tester) async {
      final counter = _EngineCounter()..failingPauseIds.add('vid-a');
      final routeObserver = RouteObserver<PageRoute<dynamic>>();
      final vidA = _video(
        id: 'vid-a',
        url: 'https://cdn.example.com/a.mp4',
        position: 0,
      );

      await tester.pumpWidget(
        _wrapFullscreenWithRouteObserver(
          media: [vidA],
          counter: counter,
          routeObserver: routeObserver,
          pushButtonBuilder: (context) => ElevatedButton(
            onPressed: () {
              Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => Scaffold(
                    appBar: AppBar(title: const Text('transient')),
                    body: const Text('transient route'),
                  ),
                ),
              );
            },
            child: const Text('push'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Play Video A
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();
      expect(counter.engineFor('vid-a').playCalls, 1);

      // Push route → triggers pause via didPushNext, which throws
      await tester.tap(find.text('push'));
      await tester.pumpAndSettle();

      // Route is visible — UI not disrupted by pause failure
      expect(find.text('transient route'), findsOneWidget);
      // Pause was attempted
      expect(counter.engineFor('vid-a').pauseCalls, 1);
      // Engine NOT disposed
      expect(counter.engineFor('vid-a').disposeCalls, 0);
      // No uncaught exception
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('app-lifecycle pause failure is isolated — no crash, no dispose', (
    tester,
  ) async {
    final counter = _EngineCounter()..failingPauseIds.add('vid-a');
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
    );

    await tester.pumpWidget(_wrapFullscreen(media: [vidA], counter: counter));
    await tester.pumpAndSettle();

    // Play Video A
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();
    expect(counter.engineFor('vid-a').playCalls, 1);
    expect(counter.engineFor('vid-a').disposeCalls, 0);

    // Simulate app paused — triggers pause via WidgetsBindingObserver, which throws
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    await tester.pump();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
    await tester.pump();

    // Pause was attempted (count reflects calls, not successes)
    expect(counter.engineFor('vid-a').pauseCalls, greaterThanOrEqualTo(1));
    // Engine NOT disposed despite pause failure
    expect(counter.engineFor('vid-a').disposeCalls, 0);
    // Engine NOT replaced
    expect(counter.instanceCount, 1);
    // No uncaught exception
    expect(tester.takeException(), isNull);
  });
}
