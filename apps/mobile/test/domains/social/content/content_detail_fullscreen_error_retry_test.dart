import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
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
// Fake Video Engine & Counter — with held-Completer and fail-on-init support
// ============================================================================

class _FakeVideoEngine implements MediaViewerVideoEngine {
  _FakeVideoEngine({
    required this.mediaId,
    this.initializeCompleter,
    this.failOnInitialize = false,
    this.failOnDispose = false,
  });

  final String mediaId;
  final Completer<void>? initializeCompleter;
  bool failOnInitialize;
  final bool failOnDispose;

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
    if (initializeCompleter != null) {
      await initializeCompleter!.future;
    }
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
    if (failOnDispose) {
      throw StateError('dispose failed for $mediaId');
    }
  }
}

class _EngineCounter {
  final List<_FakeVideoEngine> engines = [];
  final Map<String, Completer<void>> pendingCompleters = {};
  final Set<String> failingInitIds = {};
  final Set<String> failingDisposeIds = {};

  int get instanceCount => engines.length;
  int get totalDisposeCalls =>
      engines.fold(0, (sum, e) => sum + e.disposeCalls);

  /// Always creates a new engine instance — embedded and fullscreen get
  /// independent engines.
  MediaViewerVideoEngine build(MediaEntity media) {
    final completer = pendingCompleters[media.id];
    final engine = _FakeVideoEngine(
      mediaId: media.id,
      initializeCompleter: completer,
      failOnInitialize: failingInitIds.contains(media.id),
      failOnDispose: failingDisposeIds.contains(media.id),
    );
    engines.add(engine);
    return engine;
  }

  void holdInitialization(String mediaId) {
    pendingCompleters[mediaId] = Completer<void>();
  }

  void completeInitialization(String mediaId) {
    final completer = pendingCompleters[mediaId];
    if (completer != null && !completer.isCompleted) {
      completer.complete();
    }
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

  _FakeVideoEngine get latestEngine => engines.last;

  void reset() {
    engines.clear();
    pendingCompleters.clear();
    failingInitIds.clear();
    failingDisposeIds.clear();
  }
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

// ============================================================================
// Fake Dependencies
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
// Screen Wrapper (with optional videoEngineBuilder)
// ============================================================================

Widget _wrapContentDetail(
  Content content, {
  AuthState authState = const AuthState.unauthenticated(),
  LikeStats? likeStats,
  MediaViewerVideoEngineBuilder? videoEngineBuilder,
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
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/',
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
// Direct Fullscreen Viewer Wrapper
// ============================================================================

Widget _wrapFullscreenViewer(
  List<MediaEntity> media, {
  required _EngineCounter counter,
}) {
  return MaterialApp(
    home: MediaViewerWidget(
      media: media,
      embedded: false,
      videoEngineBuilder: counter.build,
    ),
  );
}

// ============================================================================
// Tests
// ============================================================================

void main() {
  // ==========================================================================
  // SCENARIO 1: Active initialization failure
  // From ContentDetailScreen: expand → Play (failing init) → error UI + Retry
  // ==========================================================================

  testWidgets(
    'SCENARIO 1: Active initialization failure shows per-video error and Retry',
    (tester) async {
      final counter = _EngineCounter();
      counter.failingInitIds.add('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'init failure test',
        authorId: '00000000-0000-0000-0000-000000000901',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _videoMedia(
            id: 'vid-1',
            url: videoUrl,
            posterUrl: videoUrl,
            position: 0,
          ),
        ],
      );

      await tester.pumpWidget(
        _wrapContentDetail(content, videoEngineBuilder: counter.build),
      );
      await tester.pumpAndSettle();

      // Expand → fullscreen
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();

      // Tap Play → init will fail
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      // Error UI visible with Retry button
      expect(find.text('Video failed to load'), findsOneWidget);
      expect(find.text('Retry'), findsOneWidget);

      // Failed engine created, initialized once, disposed once
      expect(counter.instanceCount, 1);
      expect(counter.engineFor('vid-1').initializeCalls, 1);
      expect(counter.engineFor('vid-1').disposeCalls, 1);
      expect(counter.engineFor('vid-1').playCalls, 0);

      // Content Detail is NOT replaced with global error
      expect(
        find.byType(ContentDetailScreen),
        findsNothing,
      ); // behind fullscreen

      // No uncaught exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 2: Successful retry
  // Engine #1 fails → tap Retry → Engine #2 succeeds → player visible
  // ==========================================================================

  testWidgets(
    'SCENARIO 2: Successful retry — new engine, play, player visible',
    (tester) async {
      final counter = _EngineCounter();
      counter.failingInitIds.add('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'retry success test',
        authorId: '00000000-0000-0000-0000-000000000902',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _videoMedia(
            id: 'vid-1',
            url: videoUrl,
            posterUrl: videoUrl,
            position: 0,
          ),
        ],
      );

      await tester.pumpWidget(
        _wrapContentDetail(content, videoEngineBuilder: counter.build),
      );
      await tester.pumpAndSettle();

      // Expand → Play → Engine #1 fails
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(find.text('Video failed to load'), findsOneWidget);
      expect(counter.instanceCount, 1);
      final failedEngine = counter.engineFor('vid-1');
      expect(failedEngine.disposeCalls, 1);

      // Make retry succeed
      counter.failingInitIds.clear();

      // Tap Retry → Engine #2 succeeds
      await tester.tap(find.text('Retry'));
      await tester.pumpAndSettle();

      // Two engines total: #1 (failed, disposed), #2 (success)
      expect(counter.instanceCount, 2);
      final retryEngine = counter.latestEngine;
      expect(retryEngine, isNot(same(failedEngine)));
      expect(retryEngine.initializeCalls, 1);
      expect(retryEngine.playCalls, 1);
      expect(retryEngine.disposeCalls, 0);

      // Failed engine NOT disposed a second time
      expect(failedEngine.disposeCalls, 1);

      // Player visible, error UI gone
      expect(find.byKey(const ValueKey('player-vid-1')), findsOneWidget);
      expect(find.text('Video failed to load'), findsNothing);
      expect(find.text('Retry'), findsNothing);

      // No uncaught exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 3: Repeated Retry de-duplication
  // Hold retry init with Completer → tap Retry multiple times → only one engine
  // ==========================================================================

  testWidgets(
    'SCENARIO 3: Repeated Retry de-duplicates — only one init in-flight',
    (tester) async {
      final counter = _EngineCounter();
      counter.failingInitIds.add('vid-1');
      counter.holdInitialization('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'retry dedup test',
        authorId: '00000000-0000-0000-0000-000000000903',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _videoMedia(
            id: 'vid-1',
            url: videoUrl,
            posterUrl: videoUrl,
            position: 0,
          ),
        ],
      );

      await tester.pumpWidget(
        _wrapContentDetail(content, videoEngineBuilder: counter.build),
      );
      await tester.pumpAndSettle();

      // Expand → Play → Engine #1 fails
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(counter.instanceCount, 1);
      expect(find.text('Retry'), findsOneWidget);

      // Now allow retry to succeed BUT hold the initialization
      counter.failingInitIds.clear();

      // Tap Retry → starts init (held)
      await tester.tap(find.text('Retry'));
      await tester.pump();

      // Engine #2 created, initializing (held)
      expect(counter.instanceCount, 2);
      expect(counter.latestEngine.initializeCalls, 1);
      // Spinner visible while initializing
      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      // Tap Retry again — guard should reject (_isInitializing = true)
      // Retry button is gone (replaced by spinner), so find nothing
      expect(find.text('Retry'), findsNothing);

      // Still only 2 engines (no third engine)
      expect(counter.instanceCount, 2);

      // Complete held init
      counter.completeInitialization('vid-1');
      await tester.pumpAndSettle();

      // Engine #2 initialized, played once
      expect(counter.instanceCount, 2);
      expect(counter.latestEngine.playCalls, 1);

      // Player visible
      expect(find.byKey(const ValueKey('player-vid-1')), findsOneWidget);

      // No uncaught exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 4: Retry also fails
  // Engine #1 fails → retry Engine #2 also fails → error persists, stable
  // ==========================================================================

  testWidgets(
    'SCENARIO 4: Retry failure again — both disposed, error persists',
    (tester) async {
      final counter = _EngineCounter();
      counter.failingInitIds.add('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'retry fail again test',
        authorId: '00000000-0000-0000-0000-000000000904',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _videoMedia(
            id: 'vid-1',
            url: videoUrl,
            posterUrl: videoUrl,
            position: 0,
          ),
        ],
      );

      await tester.pumpWidget(
        _wrapContentDetail(content, videoEngineBuilder: counter.build),
      );
      await tester.pumpAndSettle();

      // Expand → Play → Engine #1 fails
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(counter.instanceCount, 1);
      final engine1 = counter.engineFor('vid-1');
      expect(engine1.disposeCalls, 1);

      // Engine #2 also set to fail
      // Tap Retry → Engine #2 also fails
      await tester.tap(find.text('Retry'));
      await tester.pumpAndSettle();

      // Two engines total
      expect(counter.instanceCount, 2);
      final engine2 = counter.latestEngine;
      expect(engine2, isNot(same(engine1)));
      expect(engine2.disposeCalls, 1);
      expect(engine2.playCalls, 0);

      // Engine #1 not disposed a second time
      expect(engine1.disposeCalls, 1);

      // Error UI still available — Retry still available
      expect(find.text('Video failed to load'), findsOneWidget);
      expect(find.text('Retry'), findsOneWidget);

      // No automatic retry loop — no additional engines beyond the explicit tap
      expect(counter.instanceCount, 2);

      // No uncaught exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 5: Per-video isolation
  // Video A fails, swipe to Video B, Play B succeeds
  // ==========================================================================

  testWidgets('SCENARIO 5: Per-video isolation — B succeeds when A failed', (
    tester,
  ) async {
    final counter = _EngineCounter();
    counter.failingInitIds.add('vid-a');
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
      poster: 'https://cdn.example.com/a.jpg',
    );
    final vidB = _video(
      id: 'vid-b',
      url: 'https://cdn.example.com/b.mp4',
      position: 1,
      poster: 'https://cdn.example.com/b.jpg',
    );

    await tester.pumpWidget(
      _wrapFullscreenViewer([vidA, vidB], counter: counter),
    );
    await tester.pumpAndSettle();

    // Page 0: Play Video A → fails
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    expect(find.text('Video failed to load'), findsOneWidget);
    final engineA = counter.engineFor('vid-a');
    expect(engineA.disposeCalls, 1);
    expect(counter.engineForOrNull('vid-b'), isNull);

    // Swipe to Video B (page 1)
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // Video B shows poster + play button — NOT errored
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
    expect(find.text('Video failed to load'), findsNothing);

    // Play Video B → succeeds
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    // Engine B created and playing — distinct from engine A
    expect(counter.instanceCount, 2);
    final engineB = counter.engineFor('vid-b');
    expect(engineB, isNot(same(engineA)));
    expect(engineB.initializeCalls, 1);
    expect(engineB.playCalls, 1);
    expect(engineB.disposeCalls, 0);

    // Engine A not disposed a second time
    expect(engineA.disposeCalls, 1);

    // Player B visible
    expect(find.byKey(const ValueKey('player-vid-b')), findsOneWidget);

    // No uncaught exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 6: Retry after focus loss
  // Retry init pending → swipe away → complete → respects active/stale authority
  // ==========================================================================

  testWidgets('SCENARIO 6: Retry pending init respects focus/stale authority', (
    tester,
  ) async {
    final counter = _EngineCounter();
    counter.failingInitIds.add('vid-a');
    counter.holdInitialization('vid-a');
    final vidA = _video(
      id: 'vid-a',
      url: 'https://cdn.example.com/a.mp4',
      position: 0,
      poster: 'https://cdn.example.com/a.jpg',
    );
    final imgB = MediaEntity(
      id: 'img-b',
      originalUrl: 'https://cdn.example.com/b.jpg',
      type: MediaType.image,
      position: 1,
      createdAt: DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
    );

    await tester.pumpWidget(
      _wrapFullscreenViewer([vidA, imgB], counter: counter),
    );
    await tester.pumpAndSettle();

    // Play Video A → Engine #1 fails
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    expect(counter.instanceCount, 1);
    final engine1 = counter.engineFor('vid-a');
    expect(engine1.disposeCalls, 1);

    // Allow retry to succeed
    counter.failingInitIds.clear();

    // Tap Retry → starts init (held)
    await tester.tap(find.text('Retry'));
    await tester.pump();

    expect(counter.instanceCount, 2);
    final engine2 = counter.latestEngine;
    expect(engine2.initializeCalls, 1);

    // Swipe away to page 1 — isActive becomes false, triggers _pausePlayback
    // but there's no active engine to pause. didUpdateWidget also runs.
    // Use pump with duration to avoid pumpAndSettle hanging on network images.
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    // Now complete the held retry init
    counter.completeInitialization('vid-a');
    await tester.pumpAndSettle();

    // Engine #2's init completes on stale page (isActive=false).
    // Token/mounted check should handle this. The engine was created while
    // page was active, completed when page is inactive.
    // Since the page is still mounted (keep-alive), the token matches.
    // widget.isActive=false so it should pause, not play.
    expect(engine2.playCalls, 0); // No auto-play on inactive page
    // pause may or may not have been called depending on isActive check
    // The key proof: no crash, no uncaught error
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 7: Retry then Close
  // Retry succeeds → Close → recovered engine disposed once
  // ==========================================================================

  testWidgets('SCENARIO 7: Retry then Close — recovered engine disposed once', (
    tester,
  ) async {
    final counter = _EngineCounter();
    counter.failingInitIds.add('vid-1');
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'retry then close test',
      authorId: '00000000-0000-0000-0000-000000000907',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _videoMedia(
          id: 'vid-1',
          url: videoUrl,
          posterUrl: videoUrl,
          position: 0,
        ),
      ],
    );

    await tester.pumpWidget(
      _wrapContentDetail(content, videoEngineBuilder: counter.build),
    );
    await tester.pumpAndSettle();

    // Expand → Play → fails
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    final engine1 = counter.engineFor('vid-1');
    expect(engine1.disposeCalls, 1);

    // Retry → succeeds
    counter.failingInitIds.clear();
    await tester.tap(find.text('Retry'));
    await tester.pumpAndSettle();

    expect(counter.instanceCount, 2);
    final engine2 = counter.latestEngine;
    expect(engine2.playCalls, 1);
    expect(engine2.disposeCalls, 0);

    // Player visible
    expect(find.byKey(const ValueKey('player-vid-1')), findsOneWidget);

    // Close fullscreen
    await tester.tap(find.byType(CloseButton));
    await tester.pumpAndSettle();

    // Recovered engine disposed exactly once
    expect(engine2.disposeCalls, 1);
    // Engine #1 not disposed a second time
    expect(engine1.disposeCalls, 1);

    // Content Detail visible
    expect(find.byType(ContentDetailScreen), findsOneWidget);

    // No uncaught exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 8: Poster-only and image regression
  // No engine/error/retry without explicit Play
  // ==========================================================================

  testWidgets('SCENARIO 8a: Poster-only — no engine, no error, no retry', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'poster-only regression',
      authorId: '00000000-0000-0000-0000-000000000908',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _videoMedia(
          id: 'vid-1',
          url: videoUrl,
          posterUrl: videoUrl,
          position: 0,
        ),
      ],
    );

    await tester.pumpWidget(
      _wrapContentDetail(content, videoEngineBuilder: counter.build),
    );
    await tester.pumpAndSettle();

    // Poster visible — play button present, no engine, no error, no retry
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
    expect(find.text('Video failed to load'), findsNothing);
    expect(find.text('Retry'), findsNothing);
    expect(counter.instanceCount, 0);

    // Expand → fullscreen → still poster-only
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
    expect(find.text('Video failed to load'), findsNothing);
    expect(counter.instanceCount, 0);

    // Close
    await tester.tap(find.byType(CloseButton));
    await tester.pumpAndSettle();

    expect(counter.instanceCount, 0);
    expect(tester.takeException(), isNull);
  });

  testWidgets('SCENARIO 8b: Image regression — no engine, no error, no retry', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const imageUrl = 'https://cdn.example.com/content/image.jpg';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'image regression',
      authorId: '00000000-0000-0000-0000-000000000909',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [_imageMedia(id: 'img-1', url: imageUrl, position: 0)],
    );

    await tester.pumpWidget(
      _wrapContentDetail(content, videoEngineBuilder: counter.build),
    );
    await tester.pumpAndSettle();

    // Image-only — no play button, no engine, no error, no retry
    expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
    expect(find.text('Video failed to load'), findsNothing);
    expect(counter.instanceCount, 0);

    // Expand → fullscreen image
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    expect(find.text('1 / 1'), findsOneWidget);
    expect(counter.instanceCount, 0);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 9: Failure DOES NOT replace Content Detail as global error
  // ==========================================================================

  testWidgets(
    'SCENARIO 9: Video failure does not replace Content Detail with global error',
    (tester) async {
      final counter = _EngineCounter();
      counter.failingInitIds.add('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'per-video error scope test',
        authorId: '00000000-0000-0000-0000-000000000910',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _videoMedia(
            id: 'vid-1',
            url: videoUrl,
            posterUrl: videoUrl,
            position: 0,
          ),
        ],
      );

      await tester.pumpWidget(
        _wrapContentDetail(content, videoEngineBuilder: counter.build),
      );
      await tester.pumpAndSettle();

      // Expand → Play → fails
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      // Error is on the video page, not a global "Failed to load content"
      expect(find.text('Video failed to load'), findsOneWidget);

      // Close fullscreen → Content Detail is still healthy
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Content Detail loads its data and shows content
      expect(find.byType(ContentDetailScreen), findsOneWidget);
      expect(find.text('per-video error scope test'), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 10: Retry concurrency guard — _isInitializing via rapid taps
  // ==========================================================================

  testWidgets(
    'SCENARIO 10: Rapid retry taps are de-duplicated by _isInitializing guard',
    (tester) async {
      final counter = _EngineCounter();
      counter.failingInitIds.add('vid-1');
      counter.holdInitialization('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'rapid retry guard test',
        authorId: '00000000-0000-0000-0000-000000000911',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _videoMedia(
            id: 'vid-1',
            url: videoUrl,
            posterUrl: videoUrl,
            position: 0,
          ),
        ],
      );

      await tester.pumpWidget(
        _wrapContentDetail(content, videoEngineBuilder: counter.build),
      );
      await tester.pumpAndSettle();

      // Expand → Play → Engine #1 fails
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(counter.instanceCount, 1);

      // Allow retry to succeed but hold the init
      counter.failingInitIds.clear();

      // Rapid taps on Retry before pump
      await tester.tap(find.text('Retry'));
      await tester.tap(find.text('Retry'));
      await tester.tap(find.text('Retry'));
      await tester.pump(); // Process first tap → _isInitializing = true
      await tester.pump(); // Subsequent taps → guard returns early

      // Only one additional engine (total = 2), not 4
      expect(counter.instanceCount, 2);

      counter.completeInitialization('vid-1');
      await tester.pumpAndSettle();

      expect(counter.instanceCount, 2);
      expect(counter.latestEngine.playCalls, 1);

      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 11: Retry lifecycle continuity — focus loss after retry success
  // ==========================================================================

  testWidgets(
    'SCENARIO 11: Retry then focus loss — pause authority still applies',
    (tester) async {
      final counter = _EngineCounter();
      counter.failingInitIds.add('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'retry lifecycle test',
        authorId: '00000000-0000-0000-0000-000000000912',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _videoMedia(
            id: 'vid-1',
            url: videoUrl,
            posterUrl: videoUrl,
            position: 0,
          ),
        ],
      );

      await tester.pumpWidget(
        _wrapContentDetail(content, videoEngineBuilder: counter.build),
      );
      await tester.pumpAndSettle();

      // Expand → Play → fails → Retry → succeeds
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();
      counter.failingInitIds.clear();
      await tester.tap(find.text('Retry'));
      await tester.pumpAndSettle();

      expect(counter.latestEngine.playCalls, 1);
      expect(counter.latestEngine.pauseCalls, 0);

      // App lifecycle pause → still pauses recovered engine
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
      await tester.pump();
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      await tester.pump();

      expect(counter.latestEngine.pauseCalls, greaterThanOrEqualTo(1));
      expect(counter.latestEngine.disposeCalls, 0);

      // Resume lifecycle so route transition animations can proceed.
      // (In tests, paused lifecycle stops Tickers which blocks route pop.)
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pump();

      // Close via system Back → dispose once
      final navigator = tester.state<NavigatorState>(find.byType(Navigator));
      navigator.pop();
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 500));
      await tester.pump(const Duration(milliseconds: 500));
      expect(counter.latestEngine.disposeCalls, 1);

      expect(tester.takeException(), isNull);
    },
  );
}
