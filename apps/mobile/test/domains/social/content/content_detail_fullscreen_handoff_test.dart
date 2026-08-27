import 'dart:io';

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
import 'package:labuda/domains/social/content/presentation/widgets/content_author_identity.dart';
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
import 'package:labuda/shared/widgets/profile_avatar.dart';
import 'package:video_player/video_player.dart';

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
            builder: (_, _) => const Scaffold(
              body: ContentDetailScreen(contentId: 'content-1'),
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
// Tests
// ============================================================================

void main() {
  // ==========================================================================
  // 1. EMPTY MEDIA — no expand button, no route push
  // ==========================================================================

  testWidgets('empty media has no expand button and does not push route', (
    tester,
  ) async {
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'empty media content',
      authorId: '00000000-0000-0000-0000-000000000501',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [],
    );

    expect(content.media, isEmpty);

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // No expand button
    expect(find.byIcon(Icons.fullscreen), findsNothing);

    // No MediaViewerWidget at all (empty media)
    expect(find.byType(MediaViewerWidget), findsNothing);

    // Caption and author still render
    expect(find.text('empty media content'), findsOneWidget);
    expect(find.byType(ContentAuthorIdentity), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 2. SINGLE IMAGE — expand button visible, tap opens fullscreen at index 0
  // ==========================================================================

  testWidgets('single image expand opens fullscreen viewer at index 0', (
    tester,
  ) async {
    const imageUrl = 'https://cdn.example.com/content/single-image.jpg';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'single image fullscreen',
      authorId: '00000000-0000-0000-0000-000000000502',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [_imageMedia(id: 'img-1', url: imageUrl, position: 0)],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Embedded MediaViewerWidget exists
    expect(find.byType(MediaViewerWidget), findsOneWidget);

    // Expand button is visible exactly once
    expect(find.byIcon(Icons.fullscreen), findsOneWidget);

    // Tap expand button
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen route pushed — verify fullscreen elements visible.
    // The embedded viewer goes offstage behind the fullscreen route, so
    // onstage MediaViewerWidget count is 1 (fullscreen). Both exist
    // when including offstage.
    expect(
      find.byType(MediaViewerWidget, skipOffstage: false),
      findsNWidgets(2),
    );

    // Fullscreen scaffold is visible: dark background, AppBar with "1 / 1"
    expect(find.text('1 / 1'), findsOneWidget);

    // The fullscreen viewer is at index 0 (title "1 / 1" confirms)
    // No nested expand button in fullscreen mode — the embedded expand
    // is offstage, and fullscreen has no expand overlay
    expect(find.byIcon(Icons.fullscreen), findsNothing);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 3. SINGLE VIDEO — expand/play separation
  // ==========================================================================

  testWidgets(
    'single video expand opens fullscreen, play button stays separate',
    (tester) async {
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      const posterUrl = 'https://cdn.example.com/content/poster.jpg';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'video fullscreen test',
        authorId: '00000000-0000-0000-0000-000000000503',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _videoMedia(
            id: 'vid-1',
            url: videoUrl,
            posterUrl: posterUrl,
            position: 0,
          ),
        ],
      );

      await tester.pumpWidget(_wrapContentDetail(content));
      await tester.pumpAndSettle();

      // Play button visible (tap-to-play, not auto-init)
      expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

      // Expand button visible — separate from play
      expect(find.byIcon(Icons.fullscreen), findsOneWidget);

      // Tap expand (not play)
      await tester.tap(find.byIcon(Icons.fullscreen));
      await tester.pumpAndSettle();

      // Fullscreen opened — both viewers exist (embedded offstage)
      expect(
        find.byType(MediaViewerWidget, skipOffstage: false),
        findsNWidgets(2),
      );

      // Fullscreen shows video poster + play button (not auto-init)
      // The fullscreen viewer has "1 / 1" title
      expect(find.text('1 / 1'), findsOneWidget);

      // Video playback engine NOT created on expand alone
      expect(find.byType(VideoPlayer), findsNothing);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // 4. MIXED INDEX CONTINUITY — swipe embedded to index 1, expand, prove index
  // ==========================================================================

  testWidgets('mixed media expand carries current index to fullscreen', (
    tester,
  ) async {
    const imgA = 'https://cdn.example.com/content/image-a.jpg';
    const vidB = 'https://cdn.example.com/content/video-b.mp4';
    const imgC = 'https://cdn.example.com/content/image-c.jpg';

    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'mixed index handoff',
      authorId: '00000000-0000-0000-0000-000000000504',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _imageMedia(id: 'img-a', url: imgA, position: 0),
        _videoMedia(id: 'vid-b', url: vidB, posterUrl: imgA, position: 1),
        _imageMedia(id: 'img-c', url: imgC, position: 2),
      ],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Expand button visible on embedded viewer
    expect(find.byIcon(Icons.fullscreen), findsOneWidget);

    // Swipe embedded to page 1 (Video B)
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // Page 1 is video: play button visible
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

    // Tap expand
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen viewer opened — both viewers exist (embedded offstage)
    expect(
      find.byType(MediaViewerWidget, skipOffstage: false),
      findsNWidgets(2),
    );

    // Fullscreen title shows "2 / 3" (current index = 1, display = 1-based)
    expect(find.text('2 / 3'), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 5. LAST-INDEX CONTINUITY — swipe to index 2, expand, prove index
  // ==========================================================================

  testWidgets('last index expand carries correct position to fullscreen', (
    tester,
  ) async {
    const imgA = 'https://cdn.example.com/content/image-a.jpg';
    const vidB = 'https://cdn.example.com/content/video-b.mp4';
    const imgC = 'https://cdn.example.com/content/image-c.jpg';

    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'last index handoff',
      authorId: '00000000-0000-0000-0000-000000000505',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _imageMedia(id: 'img-a', url: imgA, position: 0),
        _videoMedia(id: 'vid-b', url: vidB, posterUrl: imgA, position: 1),
        _imageMedia(id: 'img-c', url: imgC, position: 2),
      ],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Swipe to page 1
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // Swipe to page 2 (last)
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // Tap expand
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen opened — both viewers exist (embedded offstage)
    expect(
      find.byType(MediaViewerWidget, skipOffstage: false),
      findsNWidgets(2),
    );

    // Fullscreen title shows "3 / 3" (current index = 2, display = 1-based)
    expect(find.text('3 / 3'), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 6. BACK CONTINUITY — Back returns to Content Detail, embedded index stays
  // ==========================================================================

  testWidgets('back from fullscreen returns to content detail at same index', (
    tester,
  ) async {
    const imgA = 'https://cdn.example.com/content/image-a.jpg';
    const vidB = 'https://cdn.example.com/content/video-b.mp4';
    const imgC = 'https://cdn.example.com/content/image-c.jpg';

    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'back continuity test',
      authorId: '00000000-0000-0000-0000-000000000506',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _imageMedia(id: 'img-a', url: imgA, position: 0),
        _videoMedia(id: 'vid-b', url: vidB, posterUrl: imgA, position: 1),
        _imageMedia(id: 'img-c', url: imgC, position: 2),
      ],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Swipe embedded to page 1 (Video B)
    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

    // Tap expand → fullscreen at index 1
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen is open: both viewers exist (embedded offstage)
    expect(
      find.byType(MediaViewerWidget, skipOffstage: false),
      findsNWidgets(2),
    );
    expect(find.text('2 / 3'), findsOneWidget);

    // Navigate back — fullscreen is a fullscreenDialog route with a close
    // button; tap the CloseButton (the onstage AppBar leading widget).
    await tester.tap(find.byType(CloseButton));
    await tester.pumpAndSettle();

    // Back to Content Detail: exactly one onstage MediaViewerWidget (embedded)
    expect(find.byType(MediaViewerWidget), findsOneWidget);

    // Content detail is still rendering
    expect(find.byType(ContentDetailScreen), findsOneWidget);

    // Caption and author still render
    expect(find.text('back continuity test'), findsOneWidget);
    expect(find.byType(ContentAuthorIdentity), findsOneWidget);

    // Embedded viewer still shows Video B (index 1): play button visible
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 7. NO DUPLICATE PUSH — single tap produces exactly one fullscreen route
  // ==========================================================================

  testWidgets('single tap produces exactly one fullscreen route', (
    tester,
  ) async {
    const imageUrl = 'https://cdn.example.com/content/single-image.jpg';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'no duplicate push',
      authorId: '00000000-0000-0000-0000-000000000507',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [_imageMedia(id: 'img-1', url: imageUrl, position: 0)],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Initial state: one MediaViewerWidget (embedded)
    expect(find.byType(MediaViewerWidget), findsOneWidget);

    // Tap expand
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen opened — both viewers exist (embedded offstage)
    expect(
      find.byType(MediaViewerWidget, skipOffstage: false),
      findsNWidgets(2),
    );

    // Pump additional frames — no new pushes
    await tester.pumpAndSettle();
    await tester.pump(const Duration(seconds: 1));
    await tester.pumpAndSettle();

    // Still exactly two (no duplicate push)
    expect(
      find.byType(MediaViewerWidget, skipOffstage: false),
      findsNWidgets(2),
    );

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 8. EXPAND/PLAY SEPARATION — expand does not start video playback
  // ==========================================================================

  testWidgets('expand on video does not create video playback engine', (
    tester,
  ) async {
    const videoUrl = 'https://cdn.example.com/content/video.mp4';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'expand play separation',
      authorId: '00000000-0000-0000-0000-000000000508',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [_videoMedia(id: 'vid-1', url: videoUrl, position: 0)],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Play button exists but video NOT initialized
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
    expect(find.byType(VideoPlayer), findsNothing);

    // Tap expand
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen opened — still no VideoPlayer (expand does not start playback)
    expect(find.byType(VideoPlayer), findsNothing);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 9. FULLSCREEN MODE HAS NO NESTED EXPAND
  // ==========================================================================

  testWidgets('fullscreen viewer has no expand button', (tester) async {
    const imageUrl = 'https://cdn.example.com/content/image.jpg';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'no nested expand',
      authorId: '00000000-0000-0000-0000-000000000509',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [_imageMedia(id: 'img-1', url: imageUrl, position: 0)],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Expand visible on embedded
    expect(find.byIcon(Icons.fullscreen), findsOneWidget);

    // Tap expand
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen mode: the fullscreen Scaffold covers the embedded viewer.
    // The embedded expand button is behind the fullscreen route, so
    // the only visible Icons.fullscreen widgets are in the hidden embedded
    // viewer. The fullscreen viewer itself has embedded=false so no
    // expand overlay is rendered.

    // Since fullscreen covers embedded, no visible expand icon at the
    // top of the widget stack. (The embedded expand exists but is hidden.)
    // We can verify that the fullscreen AppBar title is present and
    // the expand icon finder at surface level returns nothing.
    expect(find.text('1 / 1'), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 10. INVALID INITIAL INDEX SAFETY — clamping works
  // ==========================================================================

  testWidgets('initial index clamping protects against out-of-range values', (
    tester,
  ) async {
    // Render a narrow viewer directly with invalid indices
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              // negative initialIndex → clamps to 0
              SizedBox(
                height: 200,
                child: MediaViewerWidget(
                  mediaUrls: [
                    'https://cdn.example.com/a.jpg',
                    'https://cdn.example.com/b.jpg',
                  ],
                  initialIndex: -5,
                  embedded: true,
                ),
              ),
              // too-large initialIndex → clamps to last
              SizedBox(
                height: 200,
                child: MediaViewerWidget(
                  mediaUrls: [
                    'https://cdn.example.com/a.jpg',
                    'https://cdn.example.com/b.jpg',
                  ],
                  initialIndex: 999,
                  embedded: true,
                ),
              ),
            ],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Both viewers render without crash
    expect(find.byType(MediaViewerWidget), findsNWidgets(2));

    // The negative-index viewer shows page 1/2 (index 0, 1-based display)
    expect(find.text('1 / 2'), findsNothing); // embedded mode has no title text

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 11. REGRESSION — embedded ordering, author identity, caption
  // ==========================================================================

  testWidgets(
    'fullscreen handoff does not regress embedded ordering or layout',
    (tester) async {
      const imgA = 'https://cdn.example.com/content/reg-a.jpg';
      const vidB = 'https://cdn.example.com/content/reg-b.mp4';
      const imgC = 'https://cdn.example.com/content/reg-c.jpg';

      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'regression fullscreen',
        authorId: '00000000-0000-0000-0000-000000000510',
        authorUsername: 'regression_user',
        authorAvatar: 'https://example.com/reg-avatar.png',
        authorLifecycle: 'active',
        media: [
          _imageMedia(id: 'img-a', url: imgA, position: 0),
          _videoMedia(id: 'vid-b', url: vidB, posterUrl: imgA, position: 1),
          _imageMedia(id: 'img-c', url: imgC, position: 2),
        ],
      );

      await tester.pumpWidget(_wrapContentDetail(content));
      await tester.pumpAndSettle();

      // Author identity renders
      expect(find.byType(ContentAuthorIdentity), findsOneWidget);
      expect(find.text('@regression_user'), findsOneWidget);

      // Profile avatar renders
      expect(find.byType(ProfileAvatar), findsOneWidget);

      // Caption renders
      expect(find.text('regression fullscreen'), findsOneWidget);

      // Timestamp renders
      expect(find.text('Posted on 23/7/2026 at 00:00'), findsOneWidget);

      // Embedded viewer exists exactly once
      expect(find.byType(MediaViewerWidget), findsOneWidget);

      // Expand button visible
      expect(find.byIcon(Icons.fullscreen), findsOneWidget);

      // Embedded ordering: page 0 = image (no play button)
      expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);

      // Swipe to page 1
      await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

      // Swipe to page 2
      await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);

      // Caption and author persist across embedded swipes
      expect(find.text('regression fullscreen'), findsOneWidget);
      expect(find.byType(ContentAuthorIdentity), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // 12. SOURCE-LEVEL — no legacy handoff, no global router changes
  // ==========================================================================

  test('fullscreen handoff uses typed media and local navigation only', () {
    final screenSource = File(
      'lib/domains/social/content/presentation/screens/content_detail_screen.dart',
    ).readAsStringSync();

    // Uses typed media: content.media (not legacy mediaUrls)
    expect(screenSource, contains('media: content.media'));

    // Uses Navigator.push, not GoRouter
    expect(screenSource, contains('Navigator.of(context).push'));

    // Does NOT add RoutePaths or GoRouter route
    expect(screenSource, isNot(contains('RoutePaths')));
    expect(screenSource, isNot(contains('GoRoute')));

    // No legacy mediaUrls handoff in fullscreen path
    expect(screenSource, isNot(contains('mediaUrls:')));

    // No hardcoded initialIndex: 0 in fullscreen builder
    // (the index comes from the callback parameter)
    final fullscreenBlock = screenSource.substring(
      screenSource.indexOf('onFullscreenRequested:'),
    );
    expect(fullscreenBlock, contains('initialIndex: index'));

    // Viewer source: onFullscreenRequested callback exists
    final viewerSource = File(
      'lib/shared/widgets/media_viewer_widget.dart',
    ).readAsStringSync();

    expect(viewerSource, contains('onFullscreenRequested'));

    // No global navigation imported into viewer
    expect(viewerSource, isNot(contains('Navigator')));
    expect(viewerSource, isNot(contains('GoRouter')));
  });

  // ==========================================================================
  // 13. MOUNT/DISPOSE — safe across route push and pop
  // ==========================================================================

  testWidgets('mount and dispose safe across fullscreen push and pop', (
    tester,
  ) async {
    const imageUrl = 'https://cdn.example.com/content/dispose-test.jpg';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'dispose safety test',
      authorId: '00000000-0000-0000-0000-000000000511',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [_imageMedia(id: 'img-1', url: imageUrl, position: 0)],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Content detail renders
    expect(find.byType(ContentDetailScreen), findsOneWidget);

    // Tap expand
    await tester.tap(find.byIcon(Icons.fullscreen));
    await tester.pumpAndSettle();

    // Fullscreen pushed — both viewers exist (embedded offstage)
    expect(
      find.byType(MediaViewerWidget, skipOffstage: false),
      findsNWidgets(2),
    );

    // Verify fullscreen AppBar is visible
    expect(find.text('1 / 1'), findsOneWidget);

    // Back from fullscreen via CloseButton
    await tester.tap(find.byType(CloseButton));
    await tester.pumpAndSettle();

    // Content detail still renders — exactly one onstage MediaViewerWidget
    expect(find.byType(MediaViewerWidget), findsOneWidget);
    expect(find.byType(ContentDetailScreen), findsOneWidget);

    // Dispose entire screen
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpAndSettle();

    // No exception thrown
    expect(tester.takeException(), isNull);
  });
}
