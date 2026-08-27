import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/core/src/providers/presence_provider.dart'
    show
        PresenceAuthorityState,
        PresenceManager,
        PresenceState,
        PresenceSubscriptionHandle,
        PresenceSubscriptionRegistry,
        presenceManagerProvider,
        presenceSubscriptionRegistryProvider;
import 'package:labuda/domains/social/content/data/content_providers.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';
import 'package:labuda/domains/social/content/domain/repositories/content_repository.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_controller.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_state.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/auth_user.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/profile_feed_tab.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';
import 'package:video_player/video_player.dart';

class _NoopLogger implements ILoggerService {
  Result<void> _ok() => Result.success(null);

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _ok();

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoOpDatasource extends Fake implements UserApiDatasource {}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _FakePresenceManager extends PresenceManager {
  @override
  PresenceAuthorityState build() => const PresenceAuthorityState.empty();
}

class _FakePresenceRegistry extends PresenceSubscriptionRegistry {
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

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeLikeRepository implements LikeRepository {
  _FakeLikeRepository(this.stats);

  final LikeStats stats;

  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    return Result.success(!stats.isLikedByCurrentUser);
  }

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async {
    return Result.success(stats);
  }

  @override
  Future<Result<bool>> hasUserLiked({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async {
    return Result.success(stats.isLikedByCurrentUser);
  }

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) {
    return Stream<LikeStats>.value(stats);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeContentRepository implements ContentRepository {
  _FakeContentRepository(this.page);

  final ContentAuthorPage page;

  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  }) async => ContentRepositoryResult.success(page);

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
  }) async => ContentRepositoryResult.success(page.items);

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByLocation({
    required String location,
    int? limit,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<Content>> getContentById(
    String contentId,
  ) async => ContentRepositoryResult.error('not used');

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

Content _content({required String id, required List<MediaEntity> media}) {
  return Content(
    id: id,
    content: 'content-$id',
    author: const ContentAuthor(id: 'author-1', username: 'alice'),
    media: media,
    tags: const [],
    settings: const ContentSettings(),
    engagement: const ContentEngagement(),
    moderationInfo: const ContentModerationInfo(),
    createdAt: DateTime.utc(2026, 7, 28),
    updatedAt: DateTime.utc(2026, 7, 28),
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

FeedItem _feedItem(Content content) {
  return FeedItem(
    id: content.id,
    content: content.content,
    author: content.author,
    type: FeedItemType.content,
    createdAt: content.createdAt,
    media: content.media,
    likes: content.engagement.likeCount,
    comments: content.engagement.commentCount,
    additionalData: const {'status': 'active'},
  );
}

Map<String, dynamic> _profileProjectionJson({
  String state = 'LIVE',
  String resourceId = 'profile-1',
  String username = 'alice',
}) {
  final json = <String, dynamic>{
    'state': state,
    'resource_type': 'profile',
    'resource_id': resourceId,
  };
  if (state == 'LIVE') {
    json['profile'] = <String, dynamic>{
      'username': username,
      'avatar_url': 'https://example.com/profile.jpg',
      'lifecycle': 'active',
    };
  }
  return json;
}

Widget _parityApp({
  required FeedItem feedItem,
  required Content content,
  AuthState authState = const AuthState.unauthenticated(),
  LikeStats? likeStats,
}) {
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
        _FakeContentRepository(
          ContentAuthorPage(items: [content], nextCursor: null, hasMore: false),
        ),
      ),
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository(stats)),
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      loggerServiceProvider.overrideWithValue(_NoopLogger()),
      objectPreviewProvider.overrideWith((ref, reference) async => null),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: Column(
          children: [
            SizedBox(height: 360, child: FeedCard(item: feedItem)),
            const Divider(height: 1),
            const Expanded(child: ProfileFeedTab(userId: 'author-1')),
          ],
        ),
      ),
    ),
  );
}

Widget _profileOnlyApp(Content content) {
  return ProviderScope(
    overrides: [
      contentRepositoryProvider.overrideWithValue(
        _FakeContentRepository(
          ContentAuthorPage(items: [content], nextCursor: null, hasMore: false),
        ),
      ),
      likeRepositoryProvider.overrideWithValue(
        _FakeLikeRepository(
          LikeStats(
            targetId: content.id,
            targetType: LikeTargetType.content,
            totalLikes: content.engagement.likeCount,
            isLikedByCurrentUser: false,
          ),
        ),
      ),
      authControllerProvider.overrideWith(
        () => _FakeAuthController(const AuthState.unauthenticated()),
      ),
      avatarCacheServiceProvider.overrideWith((_) => _NoOpAvatarCacheService()),
      presenceManagerProvider.overrideWith(_FakePresenceManager.new),
      presenceSubscriptionRegistryProvider.overrideWithValue(
        _FakePresenceRegistry(),
      ),
      loggerServiceProvider.overrideWithValue(_NoopLogger()),
      objectPreviewProvider.overrideWith((ref, reference) async => null),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: SizedBox(height: 1200, child: ProfileFeedTab(userId: 'author-1')),
      ),
    ),
  );
}

Widget _homeOnlyApp(FeedItem feedItem) {
  return ProviderScope(
    overrides: [
      objectPreviewProvider.overrideWith((ref, reference) async => null),
      authControllerProvider.overrideWith(
        () => _FakeAuthController(const AuthState.unauthenticated()),
      ),
      contentRepositoryProvider.overrideWithValue(
        _FakeContentRepository(
          ContentAuthorPage(items: const [], nextCursor: null, hasMore: false),
        ),
      ),
      likeRepositoryProvider.overrideWithValue(
        _FakeLikeRepository(
          LikeStats(
            targetId: 'placeholder',
            targetType: LikeTargetType.content,
            totalLikes: 0,
            isLikedByCurrentUser: false,
          ),
        ),
      ),
      avatarCacheServiceProvider.overrideWith((_) => _NoOpAvatarCacheService()),
      presenceManagerProvider.overrideWith(_FakePresenceManager.new),
      presenceSubscriptionRegistryProvider.overrideWithValue(
        _FakePresenceRegistry(),
      ),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(child: FeedCard(item: feedItem)),
      ),
    ),
  );
}

void main() {
  testWidgets(
    'Home and Profile choose the same preview for canonical mixed media',
    (tester) async {
      final mixedContent = _content(
        id: 'mixed-1',
        media: [
          _image(
            id: 'image-a',
            url: 'https://cdn.example.com/content/image-a.jpg',
            position: 0,
          ),
          _image(
            id: 'image-b',
            url: 'https://cdn.example.com/content/image-b.jpg',
            position: 1,
          ),
          _video(
            id: 'video-b',
            url: 'https://cdn.example.com/content/video-b.mp4',
            position: 2,
          ),
          _video(
            id: 'video-a',
            url: 'https://cdn.example.com/content/video-a.mp4',
            position: 3,
          ),
        ],
      );
      final feedItem = _feedItem(mixedContent);

      await tester.pumpWidget(
        _parityApp(feedItem: feedItem, content: mixedContent),
      );
      await tester.pump();

      final homePreview = tester.widget<StableNetworkImage>(
        find
            .descendant(
              of: find.byType(FeedCard),
              matching: find.byType(StableNetworkImage),
            )
            .first,
      );
      expect(
        homePreview.imageUrl,
        'https://cdn.example.com/content/image-a.jpg',
      );
      expect(find.byType(VideoPlayer), findsNothing);
      expect(find.byIcon(Icons.play_circle_fill), findsNothing);

      final profilePreview = tester.widget<StableNetworkImage>(
        find
            .descendant(
              of: find.byType(ProfileFeedTab),
              matching: find.byType(StableNetworkImage),
            )
            .first,
      );
      expect(
        profilePreview.imageUrl,
        'https://cdn.example.com/content/image-a.jpg',
      );
      expect(find.byType(VideoPlayer), findsNothing);
      expect(find.byIcon(Icons.play_circle_fill), findsNothing);
    },
  );

  testWidgets(
    'Home and Profile use the same poster or fallback for video-only content',
    (tester) async {
      final videoOnly = _content(
        id: 'video-only-1',
        media: [
          _video(
            id: 'video-only-media',
            url: 'https://cdn.example.com/content/video-only.mp4',
            position: 0,
            poster: 'https://cdn.example.com/content/video-only-poster.jpg',
          ),
        ],
      );
      final feedItem = _feedItem(videoOnly);

      await tester.pumpWidget(
        _parityApp(feedItem: feedItem, content: videoOnly),
      );
      await tester.pump();

      final homePreview = tester.widget<StableNetworkImage>(
        find
            .descendant(
              of: find.byType(FeedCard),
              matching: find.byType(StableNetworkImage),
            )
            .first,
      );
      expect(
        homePreview.imageUrl,
        'https://cdn.example.com/content/video-only-poster.jpg',
      );
      expect(find.byType(VideoPlayer), findsNothing);

      final profilePreview = tester.widget<StableNetworkImage>(
        find
            .descendant(
              of: find.byType(ProfileFeedTab),
              matching: find.byType(StableNetworkImage),
            )
            .first,
      );
      expect(
        profilePreview.imageUrl,
        'https://cdn.example.com/content/video-only-poster.jpg',
      );
      expect(find.byType(VideoPlayer), findsNothing);
    },
  );

  testWidgets(
    'Home and Profile render canonical like state on the shared card',
    (tester) async {
      final content = _content(id: 'liked-1', media: const []).copyWith(
        engagement: const ContentEngagement(likeCount: 12, commentCount: 3),
      );
      final feedItem = _feedItem(content);
      final authState = AuthStateAuthenticated(
        AuthUser(
          id: 'viewer-1',
          createdAt: DateTime.utc(2026, 7, 23),
          updatedAt: DateTime.utc(2026, 7, 23),
          email: 'viewer@example.com',
          username: 'viewer',
          avatarUrl: null,
          isEmailVerified: true,
          roles: const [],
          provider: ShonaAuthProvider.email,
          lifecycle: ContentLifecycle.active,
        ),
        emailVerified: true,
      );

      await tester.pumpWidget(
        _parityApp(
          feedItem: feedItem,
          content: content,
          authState: authState,
          likeStats: LikeStats(
            targetId: content.id,
            targetType: LikeTargetType.content,
            totalLikes: 12,
            isLikedByCurrentUser: true,
          ),
        ),
      );
      await tester.pump();

      expect(
        find.descendant(
          of: find.byType(FeedCard).first,
          matching: find.byIcon(Icons.favorite),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: find.byType(FeedCard).last,
          matching: find.byIcon(Icons.favorite),
        ),
        findsOneWidget,
      );
      expect(find.text('12'), findsWidgets);
    },
  );

  testWidgets(
    'Home and Profile prefer canonical resource projection over legacy share_reference',
    (tester) async {
      final resourceProjection = ContentResourceProjection.fromJson(
        _profileProjectionJson(),
      );

      final content = _content(
        id: 'projection-home-profile',
        media: const [],
      ).copyWith(resourceProjection: resourceProjection);
      final feedItem = _feedItem(content).copyWith(
        additionalData: {
          'status': 'active',
          'resourceProjection': resourceProjection,
        },
      );

      await tester.pumpWidget(_homeOnlyApp(feedItem));
      await tester.pump();

      expect(find.byType(ObjectPreviewCard), findsNothing);
      expect(find.text('legacy sale'), findsNothing);

      await tester.pumpWidget(_profileOnlyApp(content));
      await tester.pump();

      expect(find.byType(ObjectPreviewCard), findsNothing);
      expect(find.text('@alice'), findsOneWidget);
      expect(find.text('LIVE'), findsOneWidget);
    },
  );

  testWidgets(
    'TOMBSTONE resource projection suppresses legacy share previews on Home and Profile',
    (tester) async {
      final resourceProjection = ContentResourceProjection.fromJson(
        _profileProjectionJson(state: 'TOMBSTONE', resourceId: 'profile-2'),
      );

      final content = _content(
        id: 'tombstone-home-profile',
        media: const [],
      ).copyWith(resourceProjection: resourceProjection);
      final feedItem = _feedItem(content).copyWith(
        additionalData: {
          'status': 'active',
          'resourceProjection': resourceProjection,
        },
      );

      await tester.pumpWidget(_homeOnlyApp(feedItem));
      await tester.pump();

      expect(find.byType(ObjectPreviewCard), findsNothing);
      expect(find.textContaining('tidak tersedia'), findsWidgets);
      expect(find.textContaining('TOMBSTONE'), findsWidgets);

      await tester.pumpWidget(_profileOnlyApp(content));
      await tester.pump();

      expect(find.byType(ObjectPreviewCard), findsNothing);
      expect(find.text('legacy tombstone sale'), findsNothing);
      expect(find.textContaining('tidak tersedia'), findsWidgets);
      expect(find.textContaining('TOMBSTONE'), findsWidgets);
    },
  );
}
