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
// Fake Video Engine & Counter — with held-Completer support for stale-init tests
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

  /// When true, dispose() throws a synchronous error (Scenario 11).
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

  /// Always creates a new engine instance (no putIfAbsent), so embedded and
  /// fullscreen lifecycles get independent engines.
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

  /// Returns the most recently created engine.
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
// Direct Fullscreen Viewer Wrapper (no ContentDetailScreen)
// ============================================================================

Widget _wrapFullscreenViewer(
  List<MediaEntity> media, {
  required _EngineCounter counter,
}) {
  // MediaViewerWidget in non-embedded mode creates its own Scaffold.
  // Do NOT wrap in another Scaffold — it causes nested Scaffold issues.
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
  // SCENARIO 1: Close fullscreen disposes playing engine once
  // From actual ContentDetailScreen: expand → Play → CloseButton
  // ==========================================================================

  testWidgets('SCENARIO 1: Close fullscreen disposes playing engine once', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'close disposes engine',
      authorId: '00000000-0000-0000-0000-000000000801',
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

    // Tap Play → engine created and playing
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    expect(counter.instanceCount, 1);
    expect(counter.engineFor('vid-1').playCalls, 1);
    expect(counter.engineFor('vid-1').disposeCalls, 0);

    // Tap CloseButton → route pops
    await tester.tap(find.byType(CloseButton));
    await tester.pumpAndSettle();

    // Fullscreen engine disposed exactly once
    expect(counter.engineFor('vid-1').disposeCalls, 1);

    // Fullscreen route gone — embedded viewer (offstage) still exists
    expect(find.byType(ContentDetailScreen), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 2: System Back disposes playing engine once
  // From actual ContentDetailScreen: expand → Play → system Back
  // ==========================================================================

  testWidgets('SCENARIO 2: System Back disposes playing engine once', (
    tester,
  ) async {
    final counter = _EngineCounter();
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'system back disposes engine',
      authorId: '00000000-0000-0000-0000-000000000802',
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

    // Expand → fullscreen → Play
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    expect(counter.engineFor('vid-1').disposeCalls, 0);

    // System Back: use Navigator.pop to simulate hardware Back
    final navigator = tester.state<NavigatorState>(find.byType(Navigator));
    navigator.pop();
    await tester.pumpAndSettle();

    // Fullscreen engine disposed exactly once
    expect(counter.engineFor('vid-1').disposeCalls, 1);

    // Content Detail is back
    expect(find.byType(ContentDetailScreen), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 3: Embedded engine is NOT disposed when fullscreen closes
  // From actual ContentDetailScreen: Play embedded → expand → Play fullscreen → close
  // ==========================================================================

  testWidgets(
    'SCENARIO 3: Embedded engine is not disposed when fullscreen closes',
    (tester) async {
      final counter = _EngineCounter();
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'embedded isolation test',
        authorId: '00000000-0000-0000-0000-000000000803',
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

      // Play embedded
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(counter.instanceCount, 1);
      final embeddedEngine = counter.engineFor('vid-1');
      expect(embeddedEngine.playCalls, 1);
      expect(embeddedEngine.disposeCalls, 0);

      // Expand to fullscreen
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();

      // Play fullscreen
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      // Two engines: embedded + fullscreen (distinct instances)
      expect(counter.instanceCount, 2);
      final fullscreenEngine = counter.latestEngine;
      expect(fullscreenEngine, isNot(same(embeddedEngine)));

      // Close fullscreen
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Fullscreen engine disposed = 1
      expect(fullscreenEngine.disposeCalls, 1);

      // Embedded engine disposed = 0
      expect(embeddedEngine.disposeCalls, 0);

      // Embedded engine is a different instance
      expect(embeddedEngine, isNot(same(fullscreenEngine)));

      // Content Detail back, embedded still rendering
      expect(find.byType(ContentDetailScreen), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 4: Content Detail disposal releases embedded engine
  // Navigate away from Content Detail → embedded engine disposed
  // ==========================================================================

  testWidgets(
    'SCENARIO 4: Content Detail disposal releases embedded engine once',
    (tester) async {
      final counter = _EngineCounter();
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'content detail disposal',
        authorId: '00000000-0000-0000-0000-000000000804',
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

      // Play embedded
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(counter.instanceCount, 1);
      expect(counter.engineFor('vid-1').disposeCalls, 0);

      // Navigate away — dispose entire Content Detail screen
      await tester.pumpWidget(const MaterialApp(home: SizedBox.shrink()));
      await tester.pumpAndSettle();

      // Embedded engine disposed exactly once
      expect(counter.engineFor('vid-1').disposeCalls, 1);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 5: Pop while initialization pending
  // Expand → tap Play (hold init) → close → complete init afterward
  // ==========================================================================

  testWidgets(
    'SCENARIO 5: Pop while initialization pending — no setState, late engine disposed',
    (tester) async {
      final counter = _EngineCounter();
      counter.holdInitialization('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'pending init pop test',
        authorId: '00000000-0000-0000-0000-000000000805',
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

      // Tap Play — starts initialization but Completer holds it
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pump(); // process tap, enter _startPlayback, hit await

      // Spinner visible — initialization is pending
      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      // Engine created, initialize called but not completed
      expect(counter.instanceCount, 1);
      expect(counter.engineFor('vid-1').initializeCalls, 1);
      expect(counter.engineFor('vid-1').playCalls, 0);

      // Close fullscreen while initialization is still pending
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Route is gone
      expect(find.byType(ContentDetailScreen), findsOneWidget);

      // Now complete the held initialization
      counter.completeInitialization('vid-1');
      await tester.pumpAndSettle();

      // The late engine was disposed by the stale-completion guard
      expect(counter.engineFor('vid-1').disposeCalls, 1);

      // No play call (stale completion did not play)
      expect(counter.engineFor('vid-1').playCalls, 0);

      // No uncaught exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 6: Pending initialization + system Back
  // Same as Scenario 5 but using system route pop instead of CloseButton
  // ==========================================================================

  testWidgets(
    'SCENARIO 6: Pending initialization + system Back — termination does not depend on CloseButton',
    (tester) async {
      final counter = _EngineCounter();
      counter.holdInitialization('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'pending init system back',
        authorId: '00000000-0000-0000-0000-000000000806',
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

      // Expand → fullscreen → tap Play (held)
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pump();

      expect(counter.instanceCount, 1);
      expect(counter.engineFor('vid-1').initializeCalls, 1);
      expect(counter.engineFor('vid-1').disposeCalls, 0);

      // System Back (Navigator.pop) while initialization still pending
      final navigator = tester.state<NavigatorState>(find.byType(Navigator));
      navigator.pop();
      await tester.pumpAndSettle();

      // Route is gone
      expect(find.byType(ContentDetailScreen), findsOneWidget);

      // Complete the held initialization
      counter.completeInitialization('vid-1');
      await tester.pumpAndSettle();

      // Late engine disposed exactly once
      expect(counter.engineFor('vid-1').disposeCalls, 1);
      expect(counter.engineFor('vid-1').playCalls, 0);

      // No uncaught exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 7: Initialization failure after disposal is inert
  // Pending initialization completes with error after route pop
  // ==========================================================================

  testWidgets(
    'SCENARIO 7: Initialization failure after disposal is inert — no uncaught error',
    (tester) async {
      final counter = _EngineCounter();
      counter.failingInitIds.add('vid-1');
      counter.holdInitialization('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'init failure after disposal',
        authorId: '00000000-0000-0000-0000-000000000807',
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

      // Expand → fullscreen → tap Play (held + will fail)
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pump();

      expect(counter.instanceCount, 1);
      expect(counter.engineFor('vid-1').initializeCalls, 1);

      // Close fullscreen before initialization completes/fails
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Route is gone
      expect(find.byType(ContentDetailScreen), findsOneWidget);

      // Now complete the held initialization (which will throw due to failOnInitialize)
      counter.completeInitialization('vid-1');
      await tester.pumpAndSettle();

      // Late engine was disposed (stale guard in catch block)
      expect(counter.engineFor('vid-1').disposeCalls, 1);

      // No play — stale completion is inert
      expect(counter.engineFor('vid-1').playCalls, 0);

      // No uncaught async error
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 8: Repeated pop/dispose safety
  // After fullscreen already closed, pump more, parent rebuild
  // ==========================================================================

  testWidgets(
    'SCENARIO 8: Repeated pop/dispose safety — engine disposeCalls stays 1',
    (tester) async {
      final counter = _EngineCounter();
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'repeated pop safety',
        authorId: '00000000-0000-0000-0000-000000000808',
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

      // Expand → Play → Close
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      expect(counter.engineFor('vid-1').disposeCalls, 1);

      // Pump additional frames — no duplicate dispose
      await tester.pump();
      await tester.pump(const Duration(seconds: 1));
      await tester.pumpAndSettle();

      expect(counter.engineFor('vid-1').disposeCalls, 1);

      // Navigate away from Content Detail (parent disposal)
      await tester.pumpWidget(const MaterialApp(home: SizedBox.shrink()));
      await tester.pumpAndSettle();

      // Fullscreen engine already disposed at 1 — no additional dispose from
      // Content Detail disposal because the fullscreen engine was already
      // disposed when the fullscreen route popped.
      expect(counter.engineFor('vid-1').disposeCalls, 1);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 9: Multiple videos dispose independently
  // Video A and Video B both played on fullscreen, then close
  // ==========================================================================

  testWidgets(
    'SCENARIO 9: Multiple videos dispose independently — no cross-dispose',
    (tester) async {
      final counter = _EngineCounter();
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

      // Page 0: Play Video A
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(counter.engineFor('vid-a').playCalls, 1);
      expect(counter.engineFor('vid-a').disposeCalls, 0);
      expect(counter.engineForOrNull('vid-b'), isNull);

      // Swipe to Video B (page 1)
      await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
      await tester.pumpAndSettle();

      // Play Video B
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(counter.instanceCount, 2);
      expect(counter.engineFor('vid-b').playCalls, 1);
      expect(counter.engineFor('vid-a').disposeCalls, 0);
      expect(counter.engineFor('vid-b').disposeCalls, 0);

      // Close fullscreen (pop the route via Navigator)
      final navigator = tester.state<NavigatorState>(find.byType(Navigator));
      navigator.pop();
      await tester.pumpAndSettle();

      // Both engines disposed exactly once — no cross-dispose
      expect(counter.engineFor('vid-a').disposeCalls, 1);
      expect(counter.engineFor('vid-b').disposeCalls, 1);
      // Exactly 2 dispose calls total
      expect(counter.totalDisposeCalls, 2);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 10: Poster-only fullscreen
  // Open then close without Play — zero engines, zero disposes
  // ==========================================================================

  testWidgets(
    'SCENARIO 10: Poster-only fullscreen — open and close without Play',
    (tester) async {
      final counter = _EngineCounter();
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'poster-only fullscreen',
        authorId: '00000000-0000-0000-0000-000000000810',
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

      // Expand to fullscreen (poster-first, no Play)
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();

      // Fullscreen open, poster visible
      expect(find.text('1 / 1'), findsOneWidget);

      // Zero engines created
      expect(counter.instanceCount, 0);

      // Close fullscreen without ever playing
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Still zero engines, zero disposes
      expect(counter.instanceCount, 0);
      expect(counter.totalDisposeCalls, 0);

      // Content Detail is back
      expect(find.byType(ContentDetailScreen), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 11: Dispose Future failure isolation
  // Fake engine.dispose() throws synchronous error
  // ==========================================================================

  testWidgets(
    'SCENARIO 11a: Dispose failure isolation — route still closes, no uncaught error',
    (tester) async {
      final counter = _EngineCounter();
      counter.failingDisposeIds.add('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'dispose failure isolation',
        authorId: '00000000-0000-0000-0000-000000000811',
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

      // Expand → Play → engine created
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      expect(counter.engineFor('vid-1').playCalls, 1);
      expect(counter.engineFor('vid-1').disposeCalls, 0);

      // Close — engine.dispose() will throw
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Dispose was attempted (exactly once, no retry despite failure)
      expect(counter.engineFor('vid-1').disposeCalls, 1);

      // Route still closed — Content Detail is visible
      expect(find.byType(ContentDetailScreen), findsOneWidget);

      // No uncaught exception surfaced
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 12: Dispose failure on stale completion engine
  // Pending init → pop → complete → engine.dispose() in stale guard throws
  // ==========================================================================

  testWidgets('SCENARIO 11b: Stale completion dispose failure is isolated', (
    tester,
  ) async {
    final counter = _EngineCounter();
    counter.holdInitialization('vid-1');
    // Mark for dispose failure: the engine's dispose() will throw
    counter.failingDisposeIds.add('vid-1');
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'stale dispose failure',
      authorId: '00000000-0000-0000-0000-000000000812',
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

    // Expand → tap Play (held)
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pump();

    // Close while init pending
    await tester.tap(find.byType(CloseButton));
    await tester.pumpAndSettle();

    // Complete held init → stale guard calls engine.dispose() which throws
    counter.completeInitialization('vid-1');
    await tester.pumpAndSettle();

    // Dispose was attempted once by stale guard
    expect(counter.engineFor('vid-1').disposeCalls, 1);

    // Route still closed
    expect(find.byType(ContentDetailScreen), findsOneWidget);

    // No uncaught exception surfaced
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 13: Direct viewer disposal (no fullscreen route)
  // Prove embedded engine disposal works without route pop
  // ==========================================================================

  testWidgets('SCENARIO 13: Direct viewer disposal releases engine once', (
    tester,
  ) async {
    final counter = _EngineCounter();
    final video = _video(
      id: 'vid-1',
      url: 'https://cdn.example.com/video.mp4',
      position: 0,
    );

    await tester.pumpWidget(_wrapFullscreenViewer([video], counter: counter));
    await tester.pumpAndSettle();

    // Play video
    await tester.tap(find.byIcon(Icons.play_arrow_rounded));
    await tester.pumpAndSettle();

    expect(counter.engineFor('vid-1').playCalls, 1);
    expect(counter.engineFor('vid-1').disposeCalls, 0);

    // Dispose entire widget tree
    await tester.pumpWidget(const MaterialApp(home: SizedBox.shrink()));
    await tester.pumpAndSettle();

    // Engine disposed exactly once
    expect(counter.engineFor('vid-1').disposeCalls, 1);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // SCENARIO 14: Token invalidation on second Play during pending init
  // First Play pending → quick double-tap guard proof → only latest init completes
  // ==========================================================================

  testWidgets(
    'SCENARIO 14: Token prevents stale init — _isInitializing guard de-duplicates taps',
    (tester) async {
      final counter = _EngineCounter();
      counter.holdInitialization('vid-1');
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'token guard test',
        authorId: '00000000-0000-0000-0000-000000000814',
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

      // Expand to fullscreen
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();

      // First tap: starts init (held)
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pump();

      // Engine created and initializing — spinner visible
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(counter.instanceCount, 1);
      expect(counter.latestEngine.initializeCalls, 1);

      // The first init is held, so _isInitializing=true.
      // The play button is gone (replaced by spinner), so no second tap is possible.
      // But the guard at the top of _startPlayback() would reject any re-entry.
      expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);

      // Still only one engine
      expect(counter.instanceCount, 1);
      expect(counter.latestEngine.initializeCalls, 1);

      // Complete first init
      counter.completeInitialization('vid-1');
      await tester.pumpAndSettle();

      // Still one engine, played once
      expect(counter.instanceCount, 1);
      expect(counter.latestEngine.playCalls, 1);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 15: Expand/close/expand cycle — each fullscreen is independent
  // ==========================================================================

  testWidgets(
    'SCENARIO 15: Expand/close/expand cycle creates independent engines each time',
    (tester) async {
      final counter = _EngineCounter();
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'cycle test',
        authorId: '00000000-0000-0000-0000-000000000815',
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

      // Cycle 1: expand → Play → close
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Cycle 1 engine disposed
      expect(counter.instanceCount, 1);
      expect(counter.engineFor('vid-1').disposeCalls, 1);

      // Cycle 2: expand → Play → close
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Two independent engines created and disposed
      expect(counter.instanceCount, 2);
      expect(counter.totalDisposeCalls, 2);

      // Cycle 3: expand → close without Play
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Still 2 engines, 2 disposes (no new engine for poster-only cycle)
      expect(counter.instanceCount, 2);
      expect(counter.totalDisposeCalls, 2);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // SCENARIO 16: Embedded engine survives fullscreen expand/close
  // ==========================================================================

  testWidgets(
    'SCENARIO 16: Embedded engine alive after multiple fullscreen open/close cycles',
    (tester) async {
      final counter = _EngineCounter();
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'embedded survives cycles',
        authorId: '00000000-0000-0000-0000-000000000816',
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

      // Play embedded first
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();

      final embeddedEngine = counter.engineFor('vid-1');
      expect(embeddedEngine.playCalls, 1);
      expect(embeddedEngine.disposeCalls, 0);

      // Cycle 1: expand → close
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Embedded engine still alive and undisposed
      expect(embeddedEngine.disposeCalls, 0);

      // Cycle 2: expand → Play → close
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.play_arrow_rounded));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(CloseButton));
      await tester.pumpAndSettle();

      // Embedded engine STILL undisposed
      expect(embeddedEngine.disposeCalls, 0);

      // Fullscreen engine (latest) was disposed
      expect(counter.latestEngine.disposeCalls, 1);

      // Content Detail still shows embedded
      expect(find.byType(ContentDetailScreen), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );
}
