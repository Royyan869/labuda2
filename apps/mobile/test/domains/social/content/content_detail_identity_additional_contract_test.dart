import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/data/content_providers.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/data/mappers/content_mapper.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';
import 'package:labuda/domains/social/content/presentation/providers/content_notifier.dart';
import 'package:labuda/domains/social/content/presentation/providers/content_state.dart';
import 'package:labuda/domains/social/content/presentation/screens/content_detail_screen.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_author_identity.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_resource_projection_card.dart';
import 'package:labuda/domains/social/share/presentation/widgets/share_bottom_sheet.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/shared/widgets/media_viewer_widget.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';
import 'package:video_player/video_player.dart';

Map<String, dynamic> _contentJson({
  required String id,
  required String caption,
  required String authorId,
  String? authorUsername,
  String? authorAvatar,
  required String authorLifecycle,
  Map<String, dynamic>? shareReference,
  List<Map<String, dynamic>> media = const [],
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

Map<String, dynamic> _contentJsonWithoutEmbeddedAuthor({
  required String id,
  required String caption,
  required String authorId,
  required String authorUsername,
}) {
  return <String, dynamic>{
    'id': id,
    'caption': caption,
    'author_id': authorId,
    'author_username': authorUsername,
    'author_avatar': null,
    'author_city': null,
    'author_province': null,
    'lifecycle': 'active',
    'visibility': 'public',
    'media': <Map<String, dynamic>>[],
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
    'share_reference': null,
  };
}

Map<String, dynamic> _contentJsonWithoutEmbeddedAuthorCard({
  required String id,
  required String caption,
  required String authorId,
  required String authorUsername,
}) {
  return <String, dynamic>{
    'id': id,
    'caption': caption,
    'author_id': authorId,
    'author_username': authorUsername,
    'author_avatar': null,
    'author_city': null,
    'author_province': null,
    'lifecycle': 'active',
    'visibility': 'public',
    'media': <Map<String, dynamic>>[],
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
    'share_reference': null,
    'card': <String, dynamic>{'id': id},
  };
}

Map<String, dynamic> _shareReferenceJson({
  required String targetType,
  required String targetId,
  required String title,
}) {
  return <String, dynamic>{
    'targetType': targetType,
    'targetId': targetId,
    'preview': <String, dynamic>{
      'title': title,
      'imageUrl': 'https://example.com/share.jpg',
      'isAvailable': true,
      'isSold': false,
      'isClosed': false,
      'isDeleted': false,
    },
  };
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
  Map<String, dynamic>? shareReference,
  List<Map<String, dynamic>> media = const [],
}) {
  return _contentFromJson(
    _contentJson(
      id: id,
      caption: caption,
      authorId: authorId,
      authorUsername: authorUsername,
      authorAvatar: authorAvatar,
      authorLifecycle: authorLifecycle,
      shareReference: shareReference,
      media: media,
    ),
  );
}

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

Widget _wrapCanonicalIdentity(Widget child, {required String userId}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(const AuthState.unauthenticated()),
      ),
      avatarCacheServiceProvider.overrideWithValue(_NoOpAvatarCacheService()),
      presenceSubscriptionRegistryProvider.overrideWithValue(
        _NoOpPresenceRegistry(),
      ),
      userOnlineStatusProvider(userId).overrideWithValue(false),
    ],
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

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
    child: const MaterialApp(
      home: Scaffold(body: ContentDetailScreen(contentId: 'content-1')),
    ),
  );
}

Widget _wrapContentDetailWithRouter(
  Content content, {
  AuthState authState = const AuthState.unauthenticated(),
  LikeStats? likeStats,
  void Function(String path)? onRouteBuilt,
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
        _FakeContentRepository([ContentRepositoryResult.success(content)]),
      ),
      likeRepositoryProvider.overrideWithValue(_FakeLikeRepository(stats)),
      authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
      authRepositoryProvider.overrideWithValue(
        _FakeAuthRepository(
          AuthUser(
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
          ),
        ),
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
            builder: (_, state) {
              onRouteBuilt?.call(state.uri.path);
              return Scaffold(
                body: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('profile:${state.pathParameters['id']}'),
                    Text('canonical:${state.path}'),
                  ],
                ),
              );
            },
          ),
          GoRoute(
            path: '/content/:id',
            builder: (_, state) {
              onRouteBuilt?.call(state.uri.path);
              return Scaffold(body: Text('canonical:${state.path}'));
            },
          ),
          GoRoute(
            path: '/listing/:id',
            builder: (_, state) {
              onRouteBuilt?.call(state.uri.path);
              return Scaffold(body: Text('canonical:${state.path}'));
            },
          ),
          GoRoute(
            path: '/auction/:id',
            builder: (_, state) {
              onRouteBuilt?.call(state.uri.path);
              return Scaffold(body: Text('canonical:${state.path}'));
            },
          ),
        ],
      ),
    ),
  );
}

void main() {
  test('missing card and flat keys fail closed', () {
    final dto = ContentDto.fromJson(
      _contentJsonWithoutEmbeddedAuthor(
        id: 'content-1',
        caption: 'hello',
        authorId: 'author-1',
        authorUsername: 'author',
      ),
    );

    expect(dto.author, isNull);
    expect(() => ContentMapper.toEntity(dto), throwsFormatException);
  });

  test('missing card.author fails closed', () {
    final dto = ContentDto.fromJson(
      _contentJsonWithoutEmbeddedAuthorCard(
        id: 'content-1',
        caption: 'hello',
        authorId: 'author-1',
        authorUsername: 'author',
      ),
    );

    expect(dto.author, isNull);
    expect(() => ContentMapper.toEntity(dto), throwsFormatException);
  });

  testWidgets('canonical UUID author still renders canonical handle', (
    tester,
  ) async {
    var tapped = false;
    const rawUuid = '00000000-0000-0000-0000-000000000123';

    await tester.pumpWidget(
      _wrapCanonicalIdentity(
        ContentAuthorIdentity(
          author: ContentAuthor(id: rawUuid, username: 'alice'),
          onTap: () => tapped = true,
        ),
        userId: rawUuid,
      ),
    );

    expect(find.text('@alice'), findsOneWidget);
    expect(find.text(rawUuid), findsNothing);

    await tester.tap(find.byType(ContentAuthorIdentity));
    await tester.pump();

    expect(tapped, isTrue);
  });

  testWidgets(
    'ContentDetailScreen uses shared author identity and navigates to profile',
    (tester) async {
      String? navigatedPath;
      final content = _contentWithAuthor(
        id: 'content-3',
        caption: 'hello detail',
        authorId: '00000000-0000-0000-0000-000000000125',
        authorUsername: 'alice',
        authorAvatar: 'https://example.com/alice.png',
        authorLifecycle: 'active',
      );

      await tester.pumpWidget(
        _wrapContentDetailWithRouter(
          content,
          onRouteBuilt: (path) => navigatedPath = path,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(ContentAuthorIdentity), findsOneWidget);
      expect(find.text('@alice'), findsOneWidget);

      await tester.tap(find.text('@alice'));
      await tester.pumpAndSettle();

      expect(navigatedPath, '/user/${content.author.id}');
    },
  );

  testWidgets(
    'ContentDetailScreen redacts unavailable authors and blocks navigation',
    (tester) async {
      final content = _contentWithAuthor(
        id: 'content-4',
        caption: 'hello detail',
        authorId: '00000000-0000-0000-0000-000000000126',
        authorUsername: 'alice',
        authorAvatar: 'https://example.com/alice.png',
        authorLifecycle: 'unavailable',
      );

      await tester.pumpWidget(_wrapContentDetailWithRouter(content));
      await tester.pumpAndSettle();

      expect(find.byType(ContentAuthorIdentity), findsOneWidget);
      expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
      expect(find.text('@alice'), findsNothing);

      await tester.tap(find.text('Pengguna tidak tersedia'));
      await tester.pumpAndSettle();

      expect(find.text('profile:${content.author.id}'), findsNothing);
    },
  );

  testWidgets(
    'ContentDetailScreen redacts malformed authors and blocks navigation',
    (tester) async {
      final content = _contentWithAuthor(
        id: 'content-5',
        caption: 'hello detail',
        authorId: '00000000-0000-0000-0000-000000000127',
        authorUsername: 'user_1234abcd',
        authorAvatar: 'https://example.com/user.png',
        authorLifecycle: 'active',
      );

      await tester.pumpWidget(_wrapContentDetailWithRouter(content));
      await tester.pumpAndSettle();

      expect(find.byType(ContentAuthorIdentity), findsOneWidget);
      expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
      expect(find.text('@user_1234abcd'), findsNothing);
      expect(find.textContaining('uuid', findRichText: true), findsNothing);

      await tester.tap(find.text('Pengguna tidak tersedia'));
      await tester.pumpAndSettle();

      expect(find.text('profile:${content.author.id}'), findsNothing);
    },
  );

  testWidgets('active author share sheet title derives from Content.author', (
    tester,
  ) async {
    final content = _contentWithAuthor(
      id: 'content-1',
      caption: 'hello share',
      authorId: '00000000-0000-0000-0000-000000000123',
      authorUsername: 'alice',
      authorAvatar: null,
      authorLifecycle: 'active',
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.share_outlined));
    await tester.pumpAndSettle();

    expect(find.byType(ShareBottomSheet), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(ShareBottomSheet),
        matching: find.text('@alice'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(ShareBottomSheet),
        matching: find.text('Produk Dijual'),
      ),
      findsNothing,
    );
  });

  testWidgets('unavailable author share sheet does not fabricate handle', (
    tester,
  ) async {
    final content = _contentWithAuthor(
      id: 'content-2',
      caption: 'hello share',
      authorId: '00000000-0000-0000-0000-000000000124',
      authorUsername: 'alice',
      authorAvatar: 'https://example.com/alice.png',
      authorLifecycle: 'unavailable',
    );

    await tester.pumpWidget(_wrapContentDetail(content));
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.share_outlined));
    await tester.pumpAndSettle();

    expect(find.byType(ShareBottomSheet), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(ShareBottomSheet),
        matching: find.text('@alice'),
      ),
      findsNothing,
    );
    expect(
      find.descendant(
        of: find.byType(ShareBottomSheet),
        matching: find.text('Produk Dijual'),
      ),
      findsNothing,
    );
  });

  testWidgets(
    'ContentDetailScreen prefers canonical LIVE projection and navigates by canonical identity',
    (tester) async {
      String? navigatedPath;
      final content =
          _contentWithAuthor(
            id: 'content-projection-1',
            caption: 'hello projection',
            authorId: '00000000-0000-0000-0000-000000000129',
            authorUsername: 'alice',
            authorAvatar: 'https://example.com/alice.png',
            authorLifecycle: 'active',
          ).copyWith(
            resourceProjection: ContentResourceProjection.fromJson(
              _profileProjectionJson(
                resourceId: 'profile-99',
                username: 'canonical-alice',
              ),
            ),
          );

      await tester.pumpWidget(
        _wrapContentDetailWithRouter(
          content,
          onRouteBuilt: (path) => navigatedPath = path,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(ContentResourceProjectionCard), findsOneWidget);
      expect(find.byType(ObjectPreviewCard), findsNothing);
      expect(find.text('legacy sale'), findsNothing);
      expect(find.text('@canonical-alice'), findsOneWidget);
      expect(find.text('LIVE'), findsOneWidget);

      await tester.dragUntilVisible(
        find.byType(ContentResourceProjectionCard),
        find.byType(CustomScrollView),
        const Offset(0, -300),
      );
      await tester.pumpAndSettle();
      final projectionInkWell = tester.widget<InkWell>(
        find.descendant(
          of: find.byType(ContentResourceProjectionCard),
          matching: find.byType(InkWell),
        ),
      );
      projectionInkWell.onTap?.call();
      await tester.pumpAndSettle();

      expect(navigatedPath, '/user/profile-99');
    },
  );

  testWidgets(
    'ContentDetailScreen keeps TOMBSTONE projection and blocks legacy leakage',
    (tester) async {
      final content =
          _contentWithAuthor(
            id: 'content-projection-2',
            caption: 'hello tombstone',
            authorId: '00000000-0000-0000-0000-000000000130',
            authorUsername: 'alice',
            authorAvatar: 'https://example.com/alice.png',
            authorLifecycle: 'active',
          ).copyWith(
            resourceProjection: ContentResourceProjection.fromJson(
              _profileProjectionJson(
                state: 'TOMBSTONE',
                resourceId: 'profile-100',
                username: 'canonical-alice',
              ),
            ),
          );

      await tester.pumpWidget(_wrapContentDetailWithRouter(content));
      await tester.pumpAndSettle();

      expect(find.byType(ContentResourceProjectionCard), findsOneWidget);
      expect(find.byType(ObjectPreviewCard), findsNothing);
      expect(find.text('legacy tombstone sale'), findsNothing);
      expect(find.textContaining('tidak tersedia'), findsOneWidget);
      expect(find.textContaining('TOMBSTONE'), findsWidgets);

      await tester.dragUntilVisible(
        find.byType(ContentResourceProjectionCard),
        find.byType(CustomScrollView),
        const Offset(0, -300),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byType(ContentResourceProjectionCard));
      await tester.pumpAndSettle();

      expect(find.text('canonical:/user/profile-100'), findsNothing);
    },
  );

  testWidgets('ContentDetailScreen renders canonical like state and count', (
    tester,
  ) async {
    final content =
        _contentWithAuthor(
          id: 'content-like-1',
          caption: 'hello like',
          authorId: '00000000-0000-0000-0000-000000000128',
          authorUsername: 'alice',
          authorAvatar: 'https://example.com/alice.png',
          authorLifecycle: 'active',
        ).copyWith(
          engagement: const ContentEngagement(likeCount: 9, commentCount: 2),
        );

    await tester.pumpWidget(
      _wrapContentDetail(
        content,
        authState: AuthStateAuthenticated(
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
        ),
        likeStats: LikeStats(
          targetId: content.id,
          targetType: LikeTargetType.content,
          totalLikes: 9,
          isLikedByCurrentUser: true,
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.favorite), findsOneWidget);
    expect(find.text('9'), findsWidgets);
  });

  test(
    'ContentDetail provider refresh replaces stale author identity',
    () async {
      final initial = _contentWithAuthor(
        id: 'content-refresh',
        caption: 'hello',
        authorId: '00000000-0000-0000-0000-000000000200',
        authorUsername: 'alice',
        authorAvatar: 'https://example.com/alice.png',
        authorLifecycle: 'active',
      );
      final refreshed = _contentWithAuthor(
        id: 'content-refresh',
        caption: 'hello',
        authorId: '00000000-0000-0000-0000-000000000200',
        authorUsername: null,
        authorAvatar: null,
        authorLifecycle: 'unavailable',
      );

      final container = ProviderContainer(
        overrides: [
          contentRepositoryProvider.overrideWithValue(
            _FakeContentRepository([
              ContentRepositoryResult.success(initial),
              ContentRepositoryResult.success(refreshed),
            ]),
          ),
        ],
      );

      try {
        final notifier = container.read(contentDetailProvider.notifier);

        await notifier.fetchContent('content-refresh');
        final first = container.read(contentDetailProvider);
        final firstContent = first.maybeMap(
          loaded: (state) => state.content,
          orElse: () => null,
        );
        expect(firstContent, isNotNull);
        expect(firstContent!.author.username, 'alice');
        expect(firstContent.author.avatarUrl, 'https://example.com/alice.png');

        await notifier.fetchContent('content-refresh');
        final second = container.read(contentDetailProvider);
        final secondContent = second.maybeMap(
          loaded: (state) => state.content,
          orElse: () => null,
        );
        expect(secondContent, isNotNull);
        expect(secondContent!.author.username, isNull);
        expect(secondContent.author.avatarUrl, isNull);
        expect(secondContent.author.lifecycle, ContentLifecycle.unavailable);
        expect(secondContent.author, isNot(equals(firstContent.author)));
      } finally {
        container.dispose();
      }
    },
  );

  test('detail screen share title source uses content.author', () {
    final source = File(
      'lib/domains/social/content/presentation/screens/content_detail_screen.dart',
    ).readAsStringSync();

    expect(
      source,
      contains('UserIdentityFormatter.formatHandle(content.author.username)'),
    );
    expect(source, isNot(contains(r"'@${content")));
    expect(source, contains('ContentAuthorIdentity('));
    expect(source, isNot(contains(r"'@${author.username}")));
  });

  testWidgets('ContentDetailScreen uses the shared mixed-media viewer', (
    tester,
  ) async {
    const rawUrl =
        'https://cdn.example.com/content/content-1.jpg?X-Amz-Signature=content';
    const secondUrl =
        'https://cdn.example.com/content/content-2.mp4?X-Amz-Signature=content';

    final content = _contentWithAuthor(
      id: 'content-6',
      caption: 'hello media',
      authorId: '00000000-0000-0000-0000-000000000128',
      authorUsername: 'alice',
      authorAvatar: 'https://example.com/alice.png',
      authorLifecycle: 'active',
      media: [
        <String, dynamic>{
          'url': rawUrl,
          'type': 'image',
          'thumbnailUrl': rawUrl,
          'position': 0,
        },
        <String, dynamic>{
          'url': secondUrl,
          'type': 'video',
          'thumbnailUrl': rawUrl,
          'position': 1,
        },
      ],
    );

    expect(content.media, hasLength(2));
    expect(content.media.first.originalUrl, rawUrl);
    expect(content.media.last.originalUrl, secondUrl);

    await tester.pumpWidget(_wrapContentDetailWithRouter(content));
    await tester.pumpAndSettle();

    expect(find.byType(MediaViewerWidget), findsOneWidget);
    expect(find.byType(PageView), findsOneWidget);
    expect(find.byIcon(Icons.play_arrow_rounded), findsNothing);
    expect(find.byType(VideoPlayer), findsNothing);

    final pageView = find.byType(PageView);
    await tester.fling(pageView, const Offset(-1000, 0), 1000);
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.play_arrow_rounded), findsOneWidget);
    expect(find.byType(VideoPlayer), findsNothing);
    expect(find.byType(MediaViewerWidget), findsOneWidget);
  });

  // ==========================================================================
  // GAP-FILLING: Removed author at screen level
  // ==========================================================================

  testWidgets(
    'ContentDetailScreen redacts removed authors and blocks navigation',
    (tester) async {
      final content = _contentWithAuthor(
        id: 'content-removed-1',
        caption: 'hello removed author',
        authorId: '00000000-0000-0000-0000-000000000201',
        authorUsername: 'removed_user',
        authorAvatar: 'https://example.com/removed.png',
        authorLifecycle: 'removed',
      );

      await tester.pumpWidget(_wrapContentDetailWithRouter(content));
      await tester.pumpAndSettle();

      expect(find.byType(ContentAuthorIdentity), findsOneWidget);
      expect(find.text('Pengguna dihapus'), findsOneWidget);
      expect(find.text('@removed_user'), findsNothing);

      await tester.tap(find.text('Pengguna dihapus'));
      await tester.pumpAndSettle();

      expect(find.text('profile:${content.author.id}'), findsNothing);
    },
  );

  // ==========================================================================
  // GAP-FILLING: Empty / malformed identity fields
  // ==========================================================================

  testWidgets('empty author ID degrades to unavailable label', (tester) async {
    final author = ContentAuthor(
      id: '',
      username: 'someone',
      lifecycle: ContentLifecycle.active,
    );

    await tester.pumpWidget(
      _wrapCanonicalIdentity(
        ContentAuthorIdentity(author: author),
        userId: '00000000-0000-0000-0000-000000000999',
      ),
    );

    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(find.text('@someone'), findsNothing);
    expect(author.canOpenProfile, isFalse);
  });

  testWidgets('empty username with valid UUID degrades to unavailable label', (
    tester,
  ) async {
    final author = ContentAuthor(
      id: '00000000-0000-0000-0000-000000000301',
      username: '',
      lifecycle: ContentLifecycle.active,
    );

    await tester.pumpWidget(
      _wrapCanonicalIdentity(
        ContentAuthorIdentity(author: author),
        userId: author.id,
      ),
    );

    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(author.canOpenProfile, isFalse);
  });

  testWidgets('null avatar URL with valid identity still renders handle', (
    tester,
  ) async {
    var tapped = false;
    const id = '00000000-0000-0000-0000-000000000302';

    await tester.pumpWidget(
      _wrapCanonicalIdentity(
        ContentAuthorIdentity(
          author: ContentAuthor(id: id, username: 'avatarless'),
          onTap: () => tapped = true,
        ),
        userId: id,
      ),
    );

    expect(find.text('@avatarless'), findsOneWidget);
    expect(find.byType(ProfileAvatar), findsOneWidget);

    await tester.tap(find.byType(ContentAuthorIdentity));
    await tester.pump();

    expect(tapped, isTrue);
  });

  testWidgets('null username with valid UUID degrades to unavailable label', (
    tester,
  ) async {
    final author = ContentAuthor(
      id: '00000000-0000-0000-0000-000000000303',
      username: null,
      lifecycle: ContentLifecycle.active,
    );

    await tester.pumpWidget(
      _wrapCanonicalIdentity(
        ContentAuthorIdentity(author: author),
        userId: author.id,
      ),
    );

    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(author.canOpenProfile, isFalse);
  });

  // ==========================================================================
  // GAP-FILLING: No duplicate identity rendering
  // ==========================================================================

  testWidgets(
    'ContentDetailScreen has exactly one ContentAuthorIdentity in widget tree',
    (tester) async {
      final content = _contentWithAuthor(
        id: 'content-no-dup',
        caption: 'single identity row',
        authorId: '00000000-0000-0000-0000-000000000304',
        authorUsername: 'alice',
        authorAvatar: 'https://example.com/alice.png',
        authorLifecycle: 'active',
      );

      await tester.pumpWidget(_wrapContentDetailWithRouter(content));
      await tester.pumpAndSettle();

      expect(find.byType(ContentAuthorIdentity), findsOneWidget);
    },
  );

  // ==========================================================================
  // GAP-FILLING: Regression — caption / mount-dispose
  // ==========================================================================

  testWidgets('ContentDetailScreen renders content caption body', (
    tester,
  ) async {
    final content = _contentWithAuthor(
      id: 'content-caption-1',
      caption: 'this is the caption body text',
      authorId: '00000000-0000-0000-0000-000000000305',
      authorUsername: 'alice',
      authorLifecycle: 'active',
    );

    await tester.pumpWidget(_wrapContentDetailWithRouter(content));
    await tester.pumpAndSettle();

    expect(find.text('this is the caption body text'), findsOneWidget);
  });

  testWidgets('ContentDetailScreen mounts and disposes without exception', (
    tester,
  ) async {
    final content = _contentWithAuthor(
      id: 'content-mount-1',
      caption: 'mount test',
      authorId: '00000000-0000-0000-0000-000000000306',
      authorUsername: 'alice',
      authorLifecycle: 'active',
    );

    await tester.pumpWidget(_wrapContentDetailWithRouter(content));
    await tester.pumpAndSettle();

    // Prove the screen rendered
    expect(find.byType(ContentDetailScreen), findsOneWidget);

    // Dispose by pumping a different widget
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpAndSettle();

    // No exception thrown — test completes
    expect(tester.takeException(), isNull);
  });

  // ==========================================================================
  // GAP-FILLING: Source-level guard — no bare @username interpolation
  // ==========================================================================

  test('ContentDetailScreen source has no bare @-interpolation', () {
    final source = File(
      'lib/domains/social/content/presentation/screens/content_detail_screen.dart',
    ).readAsStringSync();

    // No manual @username string interpolation anywhere in the screen
    expect(source, isNot(contains("'@\${")));
    expect(source, isNot(contains('"@\${')));
    // Only the canonical ContentAuthorIdentity is used for author identity
    expect(source, contains('ContentAuthorIdentity('));
    // Only the canonical formatter is used for handle formatting
    expect(
      source,
      contains('UserIdentityFormatter.formatHandle(content.author.username)'),
    );
  });
}
