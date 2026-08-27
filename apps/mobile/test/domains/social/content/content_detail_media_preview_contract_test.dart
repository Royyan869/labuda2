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

/// Canonical media fixture helpers for distinguishable test media.
///
/// Uses clearly distinguishable URLs so that the pipeline from DTO through
/// entity to visible widget can be traced end-to-end.
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
  // 1. EMPTY MEDIA — no phantom container, caption renders, no exception
  // ==========================================================================

  testWidgets('empty media renders caption without media section', (
    tester,
  ) async {
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'hello empty media',
      authorId: '00000000-0000-0000-0000-000000000401',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [],
    );

    expect(content.media, isEmpty);

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Caption must render
    expect(find.text('hello empty media'), findsOneWidget);

    // Author identity must render
    expect(find.byType(ContentAuthorIdentity), findsOneWidget);

    // No media viewer widget
    expect(find.byType(MediaViewerWidget), findsNothing);

    // No phantom page view
    expect(find.byType(PageView), findsNothing);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 2. SINGLE IMAGE — actual image preview exists, no video elements
  // ==========================================================================

  testWidgets('single image renders image preview without video artifacts', (
    tester,
  ) async {
    const imageUrl = 'https://cdn.example.com/content/single-image.jpg';
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'single image content',
      authorId: '00000000-0000-0000-0000-000000000402',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [_imageMedia(id: 'img-1', url: imageUrl, position: 0)],
    );

    expect(content.media, hasLength(1));
    expect(content.media.first.type, MediaType.image);
    expect(content.media.first.originalUrl, imageUrl);

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Media viewer exists exactly once
    expect(find.byType(MediaViewerWidget), findsOneWidget);

    // PageView exists
    expect(find.byType(PageView), findsOneWidget);

    // Image fallback: network load fails in test → shows image_not_supported_outlined
    expect(find.byIcon(Icons.image_not_supported_outlined), findsWidgets);

    // No video artifacts
    expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
    expect(find.byIcon(Icons.video_file_outlined), findsNothing);
    expect(find.byType(VideoPlayer), findsNothing);

    // Caption and author still render
    expect(find.text('single image content'), findsOneWidget);
    expect(find.byType(ContentAuthorIdentity), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 3. SINGLE VIDEO — video preview/poster exists, image renderer not used
  // ==========================================================================

  testWidgets(
    'single video renders poster with play button, not image fallback',
    (tester) async {
      const videoUrl = 'https://cdn.example.com/content/single-video.mp4';
      const posterUrl = 'https://cdn.example.com/content/video-poster.jpg';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'single video content',
        authorId: '00000000-0000-0000-0000-000000000403',
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

      expect(content.media, hasLength(1));
      expect(content.media.first.type, MediaType.video);

      await tester.pumpWidget(_wrapContentDetail(content));
      await tester.pumpAndSettle();

      // Media viewer exists
      expect(find.byType(MediaViewerWidget), findsOneWidget);
      expect(find.byType(PageView), findsOneWidget);

      // Video poster-first: play button is visible (tap-to-play)
      expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

      // Video is NOT auto-initialized — no VideoPlayer instantiated
      expect(find.byType(VideoPlayer), findsNothing);

      // Caption and author still render
      expect(find.text('single video content'), findsOneWidget);
      expect(find.byType(ContentAuthorIdentity), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // 4. MIXED ORDERING — Image A, Video B, Image C
  //    Prove all items appear exactly once, types correct, order preserved
  // ==========================================================================

  testWidgets(
    'mixed image-video-image preserves canonical order through PageView',
    (tester) async {
      const imgA = 'https://cdn.example.com/content/image-a.jpg';
      const vidB = 'https://cdn.example.com/content/video-b.mp4';
      const imgC = 'https://cdn.example.com/content/image-c.jpg';

      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'mixed media content',
        authorId: '00000000-0000-0000-0000-000000000404',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _imageMedia(id: 'img-a', url: imgA, position: 0),
          _videoMedia(id: 'vid-b', url: vidB, posterUrl: imgA, position: 1),
          _imageMedia(id: 'img-c', url: imgC, position: 2),
        ],
      );

      // Entity-level assertions: order, type, count
      expect(content.media, hasLength(3));
      expect(content.media[0].id, 'img-a');
      expect(content.media[0].type, MediaType.image);
      expect(content.media[1].id, 'vid-b');
      expect(content.media[1].type, MediaType.video);
      expect(content.media[2].id, 'img-c');
      expect(content.media[2].type, MediaType.image);

      await tester.pumpWidget(_wrapContentDetail(content));
      await tester.pumpAndSettle();

      // Media viewer and PageView exist
      expect(find.byType(MediaViewerWidget), findsOneWidget);
      expect(find.byType(PageView), findsOneWidget);

      // Page 0: Image A — image fallback visible, no video play button
      expect(find.byIcon(Icons.image_not_supported_outlined), findsWidgets);
      expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
      expect(find.byIcon(Icons.video_file_outlined), findsNothing);

      // Fling to page 1: Video B
      await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
      await tester.pumpAndSettle();

      // Video page: play button appears, video_file_outlined may appear as poster fallback
      expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
      // Video is NOT auto-initialized
      expect(find.byType(VideoPlayer), findsNothing);

      // Fling to page 2: Image C
      await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
      await tester.pumpAndSettle();

      // Image page again: image fallback visible, no video elements
      expect(find.byIcon(Icons.image_not_supported_outlined), findsWidgets);
      expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
      expect(find.byIcon(Icons.video_file_outlined), findsNothing);

      // Caption and author persist across navigation
      expect(find.text('mixed media content'), findsOneWidget);
      expect(find.byType(ContentAuthorIdentity), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // 5. REBUILD / DUPLICATE PREVENTION — same media, same count, same order
  // ==========================================================================

  testWidgets('rebuild preserves media count and order without duplication', (
    tester,
  ) async {
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'rebuild test',
      authorId: '00000000-0000-0000-0000-000000000405',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _imageMedia(
          id: 'img-a',
          url: 'https://cdn.example.com/content/a.jpg',
          position: 0,
        ),
        _videoMedia(
          id: 'vid-b',
          url: 'https://cdn.example.com/content/b.mp4',
          posterUrl: 'https://cdn.example.com/content/a.jpg',
          position: 1,
        ),
        _imageMedia(
          id: 'img-c',
          url: 'https://cdn.example.com/content/c.jpg',
          position: 2,
        ),
      ],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Initial state
    expect(find.byType(MediaViewerWidget), findsOneWidget);
    expect(find.byType(PageView), findsOneWidget);

    // Pump the SAME widget again (simulates parent rebuild / tab switch)
    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Media viewer still exactly one
    expect(find.byType(MediaViewerWidget), findsOneWidget);
    expect(find.byType(PageView), findsOneWidget);

    // Verify all 3 pages still accessible
    // Page 0: image
    expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);

    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // Page 1: video — play button visible
    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

    await tester.fling(find.byType(PageView), const Offset(-400, 0), 1000);
    await tester.pumpAndSettle();

    // Page 2: image — no play button
    expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);

    // No duplicate: exactly one MediaViewerWidget
    expect(find.byType(MediaViewerWidget), findsOneWidget);

    // Caption and author persist after rebuild
    expect(find.text('rebuild test'), findsOneWidget);
    expect(find.byType(ContentAuthorIdentity), findsOneWidget);

    // No exception
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 6. INVALID IMAGE URL — fallback non-destructive, screen stays mounted
  // ==========================================================================

  testWidgets(
    'malformed image URL shows fallback without crash or type change',
    (tester) async {
      // URL that will fail to load in test (any network URL)
      const badUrl = 'https://invalid.cdn.example.com/broken-image.jpg';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'bad image url',
        authorId: '00000000-0000-0000-0000-000000000406',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [_imageMedia(id: 'bad-img', url: badUrl, position: 0)],
      );

      expect(content.media.first.type, MediaType.image);

      await tester.pumpWidget(_wrapContentDetail(content));
      await tester.pumpAndSettle();

      // Screen remains mounted
      expect(find.byType(ContentDetailScreen), findsOneWidget);

      // Media viewer still exists
      expect(find.byType(MediaViewerWidget), findsOneWidget);

      // Fallback icon appears instead of crash
      expect(find.byIcon(Icons.image_not_supported_outlined), findsWidgets);

      // Item did NOT change type — no video elements for an image media
      expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
      expect(find.byType(VideoPlayer), findsNothing);

      // Caption still renders
      expect(find.text('bad image url'), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // 7. INVALID VIDEO POSTER URL — fallback canonical, no uncaught exception
  // ==========================================================================

  testWidgets(
    'video with unresolvable poster shows canonical fallback without crash',
    (tester) async {
      const videoUrl = 'https://cdn.example.com/content/video.mp4';
      // Poster URL that will fail to load
      const badPoster = 'https://invalid.cdn.example.com/broken-poster.jpg';
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'bad video poster',
        authorId: '00000000-0000-0000-0000-000000000407',
        authorUsername: 'alice',
        authorLifecycle: 'active',
        media: [
          _videoMedia(
            id: 'bad-vid',
            url: videoUrl,
            posterUrl: badPoster,
            position: 0,
          ),
        ],
      );

      expect(content.media.first.type, MediaType.video);

      await tester.pumpWidget(_wrapContentDetail(content));
      await tester.pumpAndSettle();

      // Screen remains mounted
      expect(find.byType(ContentDetailScreen), findsOneWidget);

      // Media viewer exists
      expect(find.byType(MediaViewerWidget), findsOneWidget);

      // Video page is still a video page: play button visible
      // (poster network load fails → _buildPlaceholder fallback →
      //  play button overlay on top)
      expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);

      // Video is NOT auto-initialized on failed poster
      expect(find.byType(VideoPlayer), findsNothing);

      // Item did NOT become an image — image_not_supported icon not from video page
      // (video poster fallback is video_file_outlined, not image_not_supported)

      // Caption still renders
      expect(find.text('bad video poster'), findsOneWidget);

      // No uncaught exception
      expect(tester.takeException(), isNull);
    },
  );

  // ==========================================================================
  // 8. REGRESSION — author identity, caption, mount/dispose
  // ==========================================================================

  testWidgets(
    'media section does not regress author identity or caption rendering',
    (tester) async {
      final content = _contentWithAuthor(
        id: 'content-1',
        caption: 'regression caption check',
        authorId: '00000000-0000-0000-0000-000000000408',
        authorUsername: 'regression_user',
        authorAvatar: 'https://example.com/regression-avatar.png',
        authorLifecycle: 'active',
        media: [
          _imageMedia(
            id: 'img-1',
            url: 'https://cdn.example.com/content/regression.jpg',
            position: 0,
          ),
        ],
      );

      await tester.pumpWidget(_wrapContentDetail(content));
      await tester.pumpAndSettle();

      // Author identity: exactly one ContentAuthorIdentity in tree
      expect(find.byType(ContentAuthorIdentity), findsOneWidget);
      expect(find.text('@regression_user'), findsOneWidget);

      // Profile avatar renders
      expect(find.byType(ProfileAvatar), findsOneWidget);

      // Caption text renders
      expect(find.text('regression caption check'), findsOneWidget);

      // Timestamp renders
      expect(find.text('Posted on 23/7/2026 at 00:00'), findsOneWidget);

      // Media section renders alongside author and caption
      expect(find.byType(MediaViewerWidget), findsOneWidget);

      // No exception
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('media detail screen mounts and disposes without exception', (
    tester,
  ) async {
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'mount dispose test',
      authorId: '00000000-0000-0000-0000-000000000409',
      authorUsername: 'alice',
      authorLifecycle: 'active',
      media: [
        _imageMedia(
          id: 'img-1',
          url: 'https://cdn.example.com/content/mount.jpg',
          position: 0,
        ),
        _videoMedia(
          id: 'vid-2',
          url: 'https://cdn.example.com/content/mount.mp4',
          posterUrl: 'https://cdn.example.com/content/mount-poster.jpg',
          position: 1,
        ),
      ],
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    // Prove the screen rendered with media
    expect(find.byType(ContentDetailScreen), findsOneWidget);
    expect(find.byType(MediaViewerWidget), findsOneWidget);

    // Dispose by replacing with empty widget
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpAndSettle();

    // No exception thrown — test completes
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // 9. SOURCE-LEVEL GUARD — no silent media filtering or legacy reordering
  // ==========================================================================

  test('media pipeline has no silent filtering or legacy reordering', () {
    final screenSource = File(
      'lib/domains/social/content/presentation/screens/content_detail_screen.dart',
    ).readAsStringSync();

    // No whereType filtering that silently drops items
    expect(screenSource, isNot(contains('whereType')));
    // No try-catch that skips media items
    expect(
      screenSource,
      isNot(contains(RegExp(r'try\s*\{[^}]*media', multiLine: true))),
    );
    // No reordering — media is passed directly
    expect(screenSource, contains('media: content.media'));

    // Mapper source: no legacy sorting by position
    final mapperSource = File(
      'lib/domains/social/content/data/mappers/content_mapper.dart',
    ).readAsStringSync();

    // Media list is mapped in order, not sorted
    expect(mapperSource, contains('dto.media.map(_mapMediaEntity).toList()'));
    // No sort call on media
    expect(mapperSource, isNot(contains('media.sort')));
    expect(mapperSource, isNot(contains('media..sort')));

    // DTO source: position field is read but not used for ordering
    final dtoSource = File(
      'lib/domains/social/content/data/dto/content_dto.dart',
    ).readAsStringSync();

    // position is parsed (available for downstream use) but DTO list order
    // is the canonical order
    expect(dtoSource, contains("'position'"));
  });

  // ==========================================================================
  // 10. MEDIA TYPE FIDELITY — image stays image, video stays video
  // ==========================================================================

  test(
    'MediaEntity type fidelity is preserved through DTO → Entity round-trip',
    () {
      // Image round-trip
      final imageDto = MediaDto.fromJson(<String, dynamic>{
        'url': 'https://cdn.example.com/photo.jpg',
        'type': 'image',
      });
      expect(imageDto.type, 'image');

      final imageEntity = ContentMapper.toEntity(
        ContentDto.fromJson(
          _contentJson(
            id: 'c-1',
            caption: 'test',
            authorId: 'author-1',
            authorUsername: 'alice',
            authorLifecycle: 'active',
            media: [
              <String, dynamic>{
                'id': 'img-1',
                'url': 'https://cdn.example.com/photo.jpg',
                'type': 'image',
                'position': 0,
              },
            ],
          ),
        ),
      );
      expect(imageEntity.media.first.type, MediaType.image);

      // Video round-trip
      final videoDto = MediaDto.fromJson(<String, dynamic>{
        'url': 'https://cdn.example.com/clip.mp4',
        'type': 'video',
      });
      expect(videoDto.type, 'video');

      final videoEntity = ContentMapper.toEntity(
        ContentDto.fromJson(
          _contentJson(
            id: 'c-2',
            caption: 'test',
            authorId: 'author-1',
            authorUsername: 'alice',
            authorLifecycle: 'active',
            media: [
              <String, dynamic>{
                'id': 'vid-1',
                'url': 'https://cdn.example.com/clip.mp4',
                'type': 'video',
                'position': 0,
              },
            ],
          ),
        ),
      );
      expect(videoEntity.media.first.type, MediaType.video);

      // Unknown type defaults to image (fail-closed, not crash)
      final unknownDto = MediaDto.fromJson(<String, dynamic>{
        'url': 'https://cdn.example.com/unknown.bin',
      });
      expect(unknownDto.type, 'image');
    },
  );

  // ==========================================================================
  // 11. MEDIA POSITION IS NOT CLIENT-SIDE ORDERING AUTHORITY
  // ==========================================================================

  test(
    'media list order from DTO is preserved regardless of position values',
    () {
      // If backend sends position in non-sorted order, client preserves wire order
      final entity = ContentMapper.toEntity(
        ContentDto.fromJson(
          _contentJson(
            id: 'c-order',
            caption: 'order test',
            authorId: 'author-1',
            authorUsername: 'alice',
            authorLifecycle: 'active',
            media: [
              <String, dynamic>{
                'id': 'third-in-wire',
                'url': 'https://cdn.example.com/third.jpg',
                'type': 'image',
                'position': 99,
              },
              <String, dynamic>{
                'id': 'first-in-wire',
                'url': 'https://cdn.example.com/first.mp4',
                'type': 'video',
                'position': 1,
              },
              <String, dynamic>{
                'id': 'second-in-wire',
                'url': 'https://cdn.example.com/second.jpg',
                'type': 'image',
                'position': 50,
              },
            ],
          ),
        ),
      );

      // Wire order is preserved — position is metadata, not ordering authority
      expect(entity.media, hasLength(3));
      expect(entity.media[0].id, 'third-in-wire');
      expect(entity.media[0].position, 99);
      expect(entity.media[1].id, 'first-in-wire');
      expect(entity.media[1].position, 1);
      expect(entity.media[2].id, 'second-in-wire');
      expect(entity.media[2].position, 50);
    },
  );
}
