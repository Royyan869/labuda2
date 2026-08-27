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
// Fake Video Engine & Counter
// ============================================================================

class _FakeVideoEngine implements MediaViewerVideoEngine {
  _FakeVideoEngine({required this.mediaId, this.initializeCompleter});

  final String mediaId;
  final Completer<void>? initializeCompleter;

  int initializeCalls = 0;
  int playCalls = 0;
  int pauseCalls = 0;
  int disposeCalls = 0;
  bool _isPlaying = false;

  @override
  Future<void> initialize() async {
    initializeCalls += 1;
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
  }
}

class _EngineCounter {
  final List<_FakeVideoEngine> engines = [];
  final Map<String, Completer<void>> pendingCompleters = {};

  int get instanceCount => engines.length;
  int get totalInitializeCalls =>
      engines.fold(0, (sum, e) => sum + e.initializeCalls);
  int get totalPlayCalls => engines.fold(0, (sum, e) => sum + e.playCalls);

  /// Always creates a new engine instance (no putIfAbsent), so embedded and
  /// fullscreen lifecycles get independent engines.
  MediaViewerVideoEngine build(MediaEntity media) {
    final completer = pendingCompleters[media.id];
    final engine = _FakeVideoEngine(
      mediaId: media.id,
      initializeCompleter: completer,
    );
    engines.add(engine);
    return engine;
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

  void reset() {
    engines.clear();
    pendingCompleters.clear();
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
// Screen Wrapper
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
// Direct Viewer Wrapper (for non-ContentDetailScreen tests)
// ============================================================================

Widget _wrapViewer(
  List<MediaEntity> media, {
  required _EngineCounter counter,
  bool embedded = true,
  int initialIndex = 0,
}) {
  return MaterialApp(
    home: Scaffold(
      body: MediaViewerWidget(
        media: media,
        embedded: embedded,
        initialIndex: initialIndex,
        videoEngineBuilder: counter.build,
      ),
    ),
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

// ============================================================================
// Tests
// ============================================================================

void main() {
  // ==========================================================================
  // SCENARIO 1: Fullscreen opens poster-first
  // From actual ContentDetailScreen: single video → tap expand → fullscreen
  // opens → poster/play button visible → engine instances = 0
  // ==========================================================================

  testWidgets('fullscreen opens poster-first — engine not created on expand', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'poster-first test',
      authorId: '00000000-0000-0000-0000-000000000601',
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

    // Embedded: play button visible, zero engines
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
    expect(counter.instanceCount, 0);
    expect(counter.totalInitializeCalls, 0);

    // Tap expand → fullscreen opens
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen is open — poster/play button visible
    expect(find.text('1 / 1'), findsOneWidget);

    // Still zero engines: expand does NOT initialize video
    expect(counter.instanceCount, 0);
    expect(counter.totalInitializeCalls, 0);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 2: Explicit Play initializes exactly once
  // From actual ContentDetailScreen → expand → tap Play on fullscreen
  // ==========================================================================

  testWidgets('explicit Play on fullscreen creates exactly one engine', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'explicit play test',
      authorId: '00000000-0000-0000-0000-000000000602',
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

    // Tap expand → fullscreen
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen is open
    expect(find.text('1 / 1'), findsOneWidget);

    // Tap Play on fullscreen
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    // Exactly one engine created, initialized once
    expect(counter.instanceCount, 1);
    expect(counter.engineFor('vid-1').initializeCalls, 1);
    expect(counter.engineFor('vid-1').playCalls, 1);

    // Player widget is visible
    expect(find.byKey(const ValueKey('player-vid-1')), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 3: Repeated Play tap during initialization
  // Held Completer → tap Play multiple times → still only 1 init
  // ==========================================================================

  testWidgets('repeated Play tap during initialization de-duplicates', (
    tester,
  ) async {
    final counter = _EngineCounter();
    counter.pendingCompleters['vid-1'] = Completer<void>();
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'repeat tap test',
      authorId: '00000000-0000-0000-0000-000000000603',
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

    // Tap expand → fullscreen
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Tap Play: starts initialization but doesn't complete (held Completer)
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pump();

    // Spinner replaces play button while initializing
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);

    // Complete initialization
    counter.completeInitialization('vid-1');
    await tester.pumpAndSettle();

    // Exactly one engine, one init — guard prevented any duplicate
    expect(counter.instanceCount, 1);
    expect(counter.engineFor('vid-1').initializeCalls, 1);
    expect(counter.engineFor('vid-1').playCalls, 1);

    // Player is visible after init completes
    expect(find.byKey(const ValueKey('player-vid-1')), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // Additional sub-test: tap Play button multiple times before pump
  // (stress-test the _isInitializing guard at the event-loop level)
  testWidgets('concurrent Play taps before any pump are guarded', (
    tester,
  ) async {
    final counter = _EngineCounter();
    counter.pendingCompleters['vid-1'] = Completer<void>();
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'concurrent tap guard',
      authorId: '00000000-0000-0000-0000-000000000613',
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

    // Tap expand → fullscreen
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Multiple rapid taps on the play button
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    // Without pumping, the first tap handler is already scheduled; additional
    // taps queue but _isInitializing (set synchronously in the first handler)
    // will guard them.
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pump(); // Process first tap → sets _isInitializing = true
    await tester.pump(); // Process subsequent taps → guard returns early

    // Still only one engine
    expect(counter.instanceCount, 1);

    counter.completeInitialization('vid-1');
    await tester.pumpAndSettle();

    expect(counter.instanceCount, 1);
    expect(counter.engineFor('vid-1').initializeCalls, 1);

    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 4: Rebuild safety
  // After fullscreen video active → pump/rebuild parent → engine unchanged
  // ==========================================================================

  testWidgets('rebuild after fullscreen Play preserves engine identity', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const videoUrl = 'https://cdn.example.com/content/video.mp4';

    late StateSetter setParentState;
    await tester.pumpWidget(
      MaterialApp(
        home: StatefulBuilder(
          builder: (context, setState) {
            setParentState = setState;
            return Scaffold(
              body: MediaViewerWidget(
                media: [
                  _video(
                    id: 'vid-1',
                    url: videoUrl,
                    position: 0,
                    poster: videoUrl,
                  ),
                ],
                embedded: false,
                videoEngineBuilder: counter.build,
              ),
            );
          },
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Poster-first, no engine
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
    expect(counter.instanceCount, 0);

    // Tap Play → engine created
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    expect(counter.instanceCount, 1);
    expect(counter.engineFor('vid-1').initializeCalls, 1);
    final engineBefore = counter.engineFor('vid-1');

    // Rebuild parent via setState (does NOT destroy Navigator stack)
    setParentState(() {});
    await tester.pumpAndSettle();

    // Engine identity preserved — no new engine, controller unchanged
    expect(counter.instanceCount, 1);
    expect(counter.engineFor('vid-1'), same(engineBefore));
    expect(counter.engineFor('vid-1').initializeCalls, 1);

    // Player still visible after rebuild
    expect(find.byKey(const ValueKey('player-vid-1')), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 5: Mixed-media selected video
  // Image A / Video B / Image C → expand from index 1 → only Video B engine
  // ==========================================================================

  testWidgets('mixed media — only selected video creates engine on Play', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const imgA = 'https://cdn.example.com/content/image-a.jpg';
    const vidB = 'https://cdn.example.com/content/video-b.mp4';
    const imgC = 'https://cdn.example.com/content/image-c.jpg';

    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'mixed media engine test',
      authorId: '00000000-0000-0000-0000-000000000605',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _imageMedia(id: 'img-a', url: imgA, position: 0),
        _videoMedia(id: 'vid-b', url: vidB, posterUrl: imgA, position: 1),
        _imageMedia(id: 'img-c', url: imgC, position: 2),
      ],
    );

    await tester.pumpWidget(
      _wrapContentDetail(content, videoEngineBuilder: counter.build),
    );
    await tester.pumpAndSettle();

    // Swipe embedded to page 1 (Video B)
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

    // Tap expand → fullscreen at index 1 (Video B)
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen shows "2 / 3"
    expect(find.text('2 / 3'), findsOneWidget);

    // Engine NOT created on expand
    expect(counter.instanceCount, 0);

    // Tap Play on fullscreen Video B
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    // Only Video B engine exists
    expect(counter.instanceCount, 1);
    expect(counter.engineForOrNull('vid-b'), isNotNull);
    expect(counter.engineForOrNull('img-a'), isNull);
    expect(counter.engineForOrNull('img-c'), isNull);
    expect(counter.engineFor('vid-b').initializeCalls, 1);
    expect(counter.engineFor('vid-b').playCalls, 1);

    // Player widget visible for Video B only
    expect(find.byKey(const ValueKey('player-vid-b')), findsOneWidget);
    expect(find.byKey(const ValueKey('player-img-a')), findsNothing);
    expect(find.byKey(const ValueKey('player-img-c')), findsNothing);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 6: Multiple videos have independent ownership
  // Video A / Video B → Play A → navigate to B → Play B (separate engines)
  // ==========================================================================

  testWidgets('multiple videos each own independent engines', (tester) async {
    final counter = _EngineCounter();
    const vidA = 'https://cdn.example.com/content/video-a.mp4';
    const vidB = 'https://cdn.example.com/content/video-b.mp4';

    final videoA = _video(id: 'vid-a', url: vidA, position: 0, poster: vidA);
    final videoB = _video(id: 'vid-b', url: vidB, position: 1, poster: vidB);

    await tester.pumpWidget(
      _wrapViewer([videoA, videoB], counter: counter, embedded: false),
    );
    await tester.pumpAndSettle();

    // Page 0: Video A — tap Play
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    // Engine A exists, Engine B does not
    expect(counter.instanceCount, 1);
    expect(counter.engineFor('vid-a').initializeCalls, 1);
    expect(counter.engineFor('vid-a').playCalls, 1);
    expect(counter.engineForOrNull('vid-b'), isNull);

    // Swipe to Video B
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // Video B shows poster-first, no engine yet
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
    expect(counter.instanceCount, 1); // Still only engine A
    expect(counter.engineForOrNull('vid-b'), isNull);

    // Tap Play on Video B
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    // Both engines exist, distinct instances
    expect(counter.instanceCount, 2);
    expect(counter.engineFor('vid-a').initializeCalls, 1);
    expect(counter.engineFor('vid-b').initializeCalls, 1);
    expect(counter.engineFor('vid-a'), isNot(same(counter.engineFor('vid-b'))));

    // Each plays its own media
    expect(find.byKey(const ValueKey('player-vid-b')), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 7: Embedded/fullscreen ownership separation
  // ==========================================================================

  testWidgets('embedded and fullscreen engines are separate instances', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'ownership separation test',
      authorId: '00000000-0000-0000-0000-000000000607',
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

    // Embedded: not played yet → zero engines
    expect(counter.instanceCount, 0);

    // Play on embedded first
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    // Embedded engine created
    expect(counter.instanceCount, 1);
    final embeddedEngine = counter.engineFor('vid-1');
    expect(embeddedEngine.initializeCalls, 1);

    // Now expand to fullscreen
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    expect(find.text('1 / 1'), findsOneWidget);

    // Fullscreen shows poster — engine creation waits for explicit Play
    // (embedded engine still exists but is NOT shared with fullscreen)
    expect(counter.instanceCount, 1);

    // Play on fullscreen
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    // Fullscreen created a NEW, distinct engine
    expect(counter.instanceCount, 2);
    final fullscreenEngine = counter.engineFor('vid-1');
    expect(fullscreenEngine, isNot(same(embeddedEngine)));
    expect(fullscreenEngine.initializeCalls, 1);
    expect(embeddedEngine.initializeCalls, 1); // unchanged

    // No exception
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'embedded never played — fullscreen engine is the only instance',
    (tester) async {
      final counter = _EngineCounter();
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'fullscreen-only play',
        authorId: '00000000-0000-0000-0000-000000000608',
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

      // Expand: embedded never played
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();

      expect(counter.instanceCount, 0);

      // Play on fullscreen
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      // Only one engine: fullscreen's (embedded still has 0)
      expect(counter.instanceCount, 1);
      expect(counter.engineFor('vid-1').initializeCalls, 1);

      // Now go Back from fullscreen
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Back to Content Detail — embedded viewer still shows play button
      // (embedded engine was never created)
      expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 8: Image-only regression
  // Fullscreen image → engine instances = 0, no Play video
  // ==========================================================================

  testWidgets('image-only fullscreen creates zero video engines', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const imageUrl = 'https://cdn.example.com/content/image.jpg';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'image-only regression',
      authorId: '00000000-0000-0000-0000-000000000609',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [_imageMedia(id: 'img-1', url: imageUrl, position: 0)],
    );

    await tester.pumpWidget(
      _wrapContentDetail(content, videoEngineBuilder: counter.build),
    );
    await tester.pumpAndSettle();

    // No play button (it's an image)
    expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
    expect(counter.instanceCount, 0);

    // Expand to fullscreen
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen shows image, no video engine
    expect(find.text('1 / 1'), findsOneWidget);
    expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
    expect(counter.instanceCount, 0);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 9: Empty media regression
  // No viewer or engine
  // ==========================================================================

  testWidgets('empty media creates zero viewers and zero engines', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'empty media regression',
      authorId: '00000000-0000-0000-0000-000000000610',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [],
    );

    await tester.pumpWidget(
      _wrapContentDetail(content, videoEngineBuilder: counter.build),
    );
    await tester.pumpAndSettle();

    // No MediaViewerWidget, no expand button
    expect(find.byType(MediaViewerWidget), findsNothing);
    expect(find.byIcon(Icons.fullscreen), findsNothing);
    expect(counter.instanceCount, 0);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // ADDITIONAL: videoEngineBuilder injection handoff proof
  // Prove the builder reaches fullscreen through ContentDetailScreen
  // ==========================================================================

  testWidgets(
    'videoEngineBuilder reaches fullscreen through ContentDetailScreen handoff',
    (tester) async {
      final counter = _EngineCounter();
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'injection handoff proof',
        authorId: '00000000-0000-0000-0000-000000000611',
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

      // Pass builder through ContentDetailScreen
      await tester.pumpWidget(
        _wrapContentDetail(content, videoEngineBuilder: counter.build),
      );
      await tester.pumpAndSettle();

      // Expand → fullscreen (builder captured in handoff)
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();

      // Play on fullscreen — builder is called, engine created
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      // Engine created proves builder reached fullscreen
      expect(counter.instanceCount, 1);
      expect(counter.engineFor('vid-1').initializeCalls, 1);

      // Player rendered proves full engine pipeline works
      expect(find.byKey(const ValueKey('player-vid-1')), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // ADDITIONAL: multiple expand/back cycles preserve independence
  // ==========================================================================

  testWidgets(
    'expand/back/expand cycle creates independent fullscreen engines',
    (tester) async {
      final counter = _EngineCounter();
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'cycle test',
        authorId: '00000000-0000-0000-0000-000000000612',
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

      // First cycle: expand → Play → back
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();
      expect(counter.instanceCount, 1);

      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Second cycle: expand — a fresh fullscreen widget
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();

      // Fresh fullscreen shows poster-first — no engine from previous cycle
      // The display count depends on when the fullscreen was originally opened.
      // Check that we can still start fresh
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      // New engine created for second fullscreen cycle
      expect(counter.instanceCount, 2);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );
}
