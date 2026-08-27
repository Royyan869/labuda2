import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart' hide authRepositoryProvider;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/rating/data/rating_providers.dart';
import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';
import 'package:labuda/domains/social/rating/domain/repositories/i_rating_repository.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_profile_repository.dart';
import 'package:labuda/domains/user/profile/presentation/providers/user_data_provider.dart';
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';
import 'package:labuda/shared/widgets/follow_button.dart';
import 'package:labuda/shared/services/logger_service.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeAvatarCacheService implements AvatarCacheService {
  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;

  @override
  Future<Map<String, String?>> getUserAvatarUrls(List<String> userIds) async =>
      {for (final userId in userIds) userId: null};

  @override
  Future<void> preloadAvatars(List<String> userIds) async {}

  @override
  void clearUserCache(String userId) {}

  @override
  void clearAllCache() {}

  @override
  bool hasCachedAvatar(String userId) => false;

  @override
  Map<String, dynamic> getCacheStats() => const <String, dynamic>{};
}

class _FakeAuthRepository implements IAuthRepository {
  _FakeAuthRepository(this.lookup);

  final Future<Result<AuthUser?>> Function(String userId) lookup;

  @override
  Future<Result<AuthUser?>> getUserById(String userId) => lookup(userId);

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeProfileRepository implements IProfileRepository {
  _FakeProfileRepository(this.profileForUser);

  final ProfileEntity? Function(String userId) profileForUser;

  @override
  Future<Result<ProfileEntity?>> getProfile(String userId) async =>
      Result.success(profileForUser(userId));

  @override
  Future<Result<bool>> profileExists(String userId) async =>
      Result.success(profileForUser(userId) != null);

  @override
  Stream<ProfileEntity?> watchProfile(String userId) =>
      Stream.value(profileForUser(userId));

  @override
  Future<Result<ProfileStats>> getProfileStats(String userId) async =>
      Result.success(const ProfileStats(followersCount: 0, followingCount: 0));

  @override
  Future<Result<List<ProfileEntity>>> getMultipleProfiles(
    List<String> userIds,
  ) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> getProfilesByType(
    UserRole userRole, {
    int limit = 20,
    String? lastDocumentId,
  }) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> getTrendingProfiles({
    int limit = 10,
  }) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<List<ProfileEntity>>> searchProfiles(
    String query, {
    int limit = 20,
    String? lastDocumentId,
  }) async => Result.success(const <ProfileEntity>[]);

  @override
  Future<Result<ProfileEntity>> createProfile(ProfileEntity profile) =>
      throw UnimplementedError('Not used');

  @override
  Future<Result<ProfileEntity>> updateProfile(ProfileEntity profile) =>
      throw UnimplementedError('Not used');

  @override
  Future<Result<ProfileEntity>> updateFarmInfo(
    String userId,
    FarmInfo farmInfo,
  ) => throw UnimplementedError('Not used');

  @override
  Future<Result<List<ProfileEntity>>> getVerifiedSellers({
    int limit = 20,
    String? lastDocumentId,
  }) => throw UnimplementedError('Not used');

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeContentRepository implements ContentRepository {
  @override
  Future<ContentRepositoryResult<Content>> createContent({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    ContentType type = ContentType.post,
    List<MediaEntity> media = const [],
    List<String> tags = const [],
    List<String> taggedUsers = const [],
    ContentSettings settings = const ContentSettings(),
    ContentLocation? location,
  }) async => ContentRepositoryResult.error('Not used');

  @override
  Future<ContentRepositoryResult<void>> deleteContent(String contentId) async =>
      ContentRepositoryResult.error('Not used');

  @override
  Future<ContentRepositoryResult<Content>> fulfillRequest(
    String contentId,
  ) async => ContentRepositoryResult.error('Not used');

  @override
  Future<ContentRepositoryResult<Content>> getContentById(
    String contentId,
  ) async => ContentRepositoryResult.error('Not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
    ContentType? type,
    String? location,
    ContentStatus? status,
  }) async => ContentRepositoryResult.success(const <Content>[]);

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByAuthor(
    String authorId, {
    int? limit,
    int? offset,
  }) async => ContentRepositoryResult.success(const <Content>[]);

  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {
    int limit = 20,
    String? cursor,
  }) async => ContentRepositoryResult.success(
    const ContentAuthorPage(
      items: <Content>[],
      hasMore: false,
      nextCursor: null,
    ),
  );

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByLocation({
    required String location,
    int? limit,
  }) async => ContentRepositoryResult.success(const <Content>[]);

  @override
  Future<ContentRepositoryResult<List<Content>>> getTrendingContents({
    int? limit,
  }) async => ContentRepositoryResult.success(const <Content>[]);

  @override
  Future<ContentRepositoryResult<ContentSearchResult>> searchContents({
    required String query,
    int? limit,
    int? offset,
    ContentType? type,
    String? location,
  }) async => ContentRepositoryResult.error('Not used');

  @override
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  ) async => ContentRepositoryResult.error('Not used');

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeFollowRepository implements IFollowRepository {
  @override
  Future<Result<bool>> blockUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> checkFollowStatus({
    required String followerId,
    required String followingId,
  }) async => Result.success(false);

  @override
  Future<Result<bool>> followUser({
    required String followerId,
    required String followingId,
  }) async => Result.success(true);

  @override
  Future<Result<List<FollowableUser>>> getFollowers({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async => Result.success(const <FollowableUser>[]);

  @override
  Future<Result<List<FollowableUser>>> getFollowing({
    required String userId,
    int limit = 20,
    String? lastFollowId,
  }) async => Result.success(const <FollowableUser>[]);

  @override
  Future<Result<FollowStats>> getFollowStats({
    required String userId,
    String? currentUserId,
  }) async => Result.success(
    FollowStats(
      userId: userId,
      followersCount: 0,
      followingCount: 0,
      lastUpdated: DateTime.utc(2026, 1, 1),
    ),
  );

  @override
  Future<Result<List<FollowableUser>>> searchUsers({
    required String query,
    String? currentUserId,
    UserType? filterByType,
    int limit = 20,
  }) async => Result.success(const <FollowableUser>[]);

  @override
  Future<Result<bool>> unfollowUser({
    required String followerId,
    required String followingId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> muteUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> unmuteUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> unblockUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Stream<List<FollowableUser>> watchFollowers(String userId) =>
      Stream.value(const <FollowableUser>[]);

  @override
  Stream<List<FollowableUser>> watchFollowing(String userId) =>
      Stream.value(const <FollowableUser>[]);

  @override
  Stream<FollowStats> watchFollowStats(String userId) => Stream.value(
    FollowStats(
      userId: userId,
      followersCount: 0,
      followingCount: 0,
      lastUpdated: DateTime.utc(2026, 1, 1),
    ),
  );

  @override
  Stream<List<FollowActivity>> watchFollowActivities(String userId) =>
      Stream.value(const <FollowActivity>[]);

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeRatingRepository implements IRatingRepository {
  @override
  Future<Result<Rating>> createRatingForOrder({
    required String orderId,
    required int ratingValue,
    String? comment,
  }) async => Result.error('Not used');

  @override
  Future<Result<Rating?>> getRatingForOrder({required String orderId}) async =>
      Result.success(null);

  @override
  Future<Result<List<Rating>>> getRatingsGiven({
    int limit = 20,
    int? cursor,
  }) async => Result.success(const <Rating>[]);

  @override
  Future<Result<List<Rating>>> getRatingsReceived({
    required String sellerId,
    int limit = 20,
    int? cursor,
  }) async => Result.success(const <Rating>[]);

  @override
  Future<Result<RatingSummary>> getRatingSummary({
    required String sellerId,
  }) async => Result.success(
    const RatingSummary(
      totalRatings: 0,
      averageRating: 0,
      oneStarCount: 0,
      twoStarCount: 0,
      threeStarCount: 0,
      fourStarCount: 0,
      fiveStarCount: 0,
    ),
  );
}

AuthUser _authUser({required String id, required String username}) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 6, 1),
    updatedAt: DateTime.utc(2026, 6, 1),
    email: '$username@example.com',
    username: username,
    avatarUrl: 'https://example.com/$username.png',
    bio: 'Bio for $username',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    lifecycle: ContentLifecycle.active,
    hasSellerProfile: false,
    sellerSubscriptionStatus: 'none',
    hasMarketAuthority: false,
  );
}

ProfileEntity _profileEntity({
  required String userId,
  required String coverPhotoUrl,
  required String location,
}) {
  return ProfileEntity(
    id: 'profile-$userId',
    userId: userId,
    location: location,
    coverPhotoUrl: coverPhotoUrl,
    joinedAt: DateTime.utc(2026, 1, 1),
    lastActiveAt: DateTime.utc(2026, 6, 1),
    stats: const ProfileStats(followersCount: 0, followingCount: 0),
    verification: const UserVerificationInfo(
      isPhoneVerified: false,
      isEmailVerified: true,
      isIdVerified: false,
      isFarmVerified: false,
      badges: <ProfileBadge>[],
    ),
    contactInfo: null,
    farmInfo: null,
  );
}

Widget _wrapApp({
  required AuthState authState,
  required IAuthRepository authRepository,
  required IProfileRepository profileRepository,
  required ContentRepository contentRepository,
  required IFollowRepository followRepository,
  required IRatingRepository ratingRepository,
  Stream<Set<String>>? blockedUserIds,
  required String profileUserId,
  FutureOr<AuthUser?> Function(String userId)? userDataResolver,
  AsyncValue<AuthUser?>? userDataValue,
}) {
  final overrides = [
    authControllerProvider.overrideWith(() => _FakeAuthController(authState)),
    avatarCacheServiceProvider.overrideWithValue(_FakeAvatarCacheService()),
    authRepositoryProvider.overrideWithValue(authRepository),
    profileRepositoryProvider.overrideWithValue(profileRepository),
    contentRepositoryProvider.overrideWithValue(contentRepository),
    followRepositoryProvider.overrideWithValue(followRepository),
    ratingRepositoryProvider.overrideWithValue(ratingRepository),
    blockedUserIdsProvider.overrideWith(
      (ref) => blockedUserIds ?? Stream.value(<String>{}),
    ),
    loggerServiceProvider.overrideWithValue(LoggerService.instance),
  ];

  if (userDataResolver != null) {
    overrides.add(
      userDataProvider(
        profileUserId,
      ).overrideWith((ref) => userDataResolver(profileUserId)),
    );
  }

  if (userDataValue != null) {
    overrides.add(
      userDataProvider(profileUserId).overrideWithValue(userDataValue),
    );
  }

  return ProviderScope(
    overrides: overrides,
    child: MaterialApp(home: ProfileScreen(userId: profileUserId)),
  );
}

Future<void> _prepareProfileTestViewport(WidgetTester tester) async {
  await tester.binding.setSurfaceSize(const Size(1080, 2400));
  addTearDown(() => tester.binding.setSurfaceSize(null));
}

Future<void> _pumpUntilVisible(
  WidgetTester tester,
  Finder finder, {
  Duration step = const Duration(milliseconds: 50),
  int maxIterations = 20,
}) async {
  for (var i = 0; i < maxIterations && finder.evaluate().isEmpty; i++) {
    await tester.pump(step);
  }
}

void main() {
  group('ProfileScreen identity state', () {
    testWidgets('viewed profile loading renders loading state only', (
      tester,
    ) async {
      await _prepareProfileTestViewport(tester);
      final pending = Completer<Result<AuthUser?>>();
      final owner = _authUser(id: 'owner-1', username: 'owner');

      await tester.pumpWidget(
        _wrapApp(
          authState: AuthState.authenticated(owner, emailVerified: true),
          authRepository: _FakeAuthRepository((_) => pending.future),
          profileRepository: _FakeProfileRepository((_) => null),
          contentRepository: _FakeContentRepository(),
          followRepository: _FakeFollowRepository(),
          ratingRepository: _FakeRatingRepository(),
          profileUserId: 'target-1',
        ),
      );

      await tester.pump();

      expect(find.text('Memuat profil'), findsOneWidget);
      expect(find.text('Mengambil identitas pengguna tujuan'), findsOneWidget);
      expect(find.text('User'), findsNothing);
      expect(find.text('@user'), findsNothing);
      expect(find.byType(FollowButton), findsNothing);
      expect(find.byTooltip('Edit Profile'), findsNothing);
      expect(find.byTooltip('Settings'), findsNothing);
    });

    testWidgets('viewed profile failure renders retryable error state', (
      tester,
    ) async {
      await _prepareProfileTestViewport(tester);
      final owner = _authUser(id: 'owner-1', username: 'owner');
      var calls = 0;

      await tester.pumpWidget(
        _wrapApp(
          authState: AuthState.authenticated(owner, emailVerified: true),
          authRepository: _FakeAuthRepository(
            (_) async => Result.success(null),
          ),
          profileRepository: _FakeProfileRepository((_) => null),
          contentRepository: _FakeContentRepository(),
          followRepository: _FakeFollowRepository(),
          ratingRepository: _FakeRatingRepository(),
          profileUserId: 'target-1',
          userDataValue: AsyncError<AuthUser?>(
            Exception('Connection timed out. Please try again.'),
            StackTrace.empty,
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(calls, 0);
      expect(find.text('Profil belum bisa dimuat'), findsOneWidget);
      expect(find.text('Try Again'), findsOneWidget);
      expect(find.text('User'), findsNothing);
      expect(find.text('@user'), findsNothing);
      expect(find.byType(FollowButton), findsNothing);
      expect(find.byTooltip('Edit Profile'), findsNothing);
      expect(find.byTooltip('Settings'), findsNothing);
    });

    testWidgets('retry after failure resolves to real identity', (
      tester,
    ) async {
      await _prepareProfileTestViewport(tester);
      final owner = _authUser(id: 'owner-1', username: 'owner');
      final target = _authUser(id: 'target-1', username: 'target');

      await tester.pumpWidget(
        _wrapApp(
          authState: AuthState.authenticated(owner, emailVerified: true),
          authRepository: _FakeAuthRepository(
            (_) async => Result.success(null),
          ),
          profileRepository: _FakeProfileRepository((userId) {
            if (userId == target.id) {
              return _profileEntity(
                userId: target.id,
                coverPhotoUrl: 'https://example.com/target-cover.jpg',
                location: 'Bogor, West Java',
              );
            }
            return _profileEntity(
              userId: owner.id,
              coverPhotoUrl: 'https://example.com/owner-cover.jpg',
              location: 'Depok, West Java',
            );
          }),
          contentRepository: _FakeContentRepository(),
          followRepository: _FakeFollowRepository(),
          ratingRepository: _FakeRatingRepository(),
          profileUserId: target.id,
          userDataValue: AsyncError<AuthUser?>(
            Exception('Connection timed out. Please try again.'),
            StackTrace.empty,
          ),
        ),
      );

      await _pumpUntilVisible(tester, find.text('Try Again'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Try Again'));
      await tester.pumpWidget(
        _wrapApp(
          authState: AuthState.authenticated(owner, emailVerified: true),
          authRepository: _FakeAuthRepository(
            (_) async => Result.success(null),
          ),
          profileRepository: _FakeProfileRepository((userId) {
            if (userId == target.id) {
              return _profileEntity(
                userId: target.id,
                coverPhotoUrl: 'https://example.com/target-cover.jpg',
                location: 'Bogor, West Java',
              );
            }
            return _profileEntity(
              userId: owner.id,
              coverPhotoUrl: 'https://example.com/owner-cover.jpg',
              location: 'Depok, West Java',
            );
          }),
          contentRepository: _FakeContentRepository(),
          followRepository: _FakeFollowRepository(),
          ratingRepository: _FakeRatingRepository(),
          profileUserId: target.id,
          userDataValue: AsyncData<AuthUser?>(target),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Profil belum bisa dimuat'), findsNothing);
      expect(find.text('Try Again'), findsNothing);
      expect(find.text('@target'), findsOneWidget);
      expect(find.text('User'), findsNothing);
      expect(find.text('@user'), findsNothing);
      expect(find.byType(FollowButton), findsOneWidget);
      expect(
        tester.widget<FollowButton>(find.byType(FollowButton)).userId,
        target.id,
      );
      expect(find.byTooltip('Edit Profile'), findsNothing);
      expect(find.byTooltip('Settings'), findsNothing);
    });

    testWidgets('viewed profile success renders real identity authority', (
      tester,
    ) async {
      await _prepareProfileTestViewport(tester);
      final owner = _authUser(id: 'owner-1', username: 'owner');
      final target = _authUser(id: 'target-1', username: 'target');

      await tester.pumpWidget(
        _wrapApp(
          authState: AuthState.authenticated(owner, emailVerified: true),
          authRepository: _FakeAuthRepository((userId) async {
            if (userId == target.id) {
              return Result.success(target);
            }
            return Result.success(owner);
          }),
          profileRepository: _FakeProfileRepository((userId) {
            if (userId == target.id) {
              return _profileEntity(
                userId: target.id,
                coverPhotoUrl: 'https://example.com/target-cover.jpg',
                location: 'Bogor, West Java',
              );
            }
            return _profileEntity(
              userId: owner.id,
              coverPhotoUrl: 'https://example.com/owner-cover.jpg',
              location: 'Depok, West Java',
            );
          }),
          contentRepository: _FakeContentRepository(),
          followRepository: _FakeFollowRepository(),
          ratingRepository: _FakeRatingRepository(),
          profileUserId: target.id,
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('@target'), findsOneWidget);
      expect(find.byType(FollowButton), findsOneWidget);
      expect(
        tester.widget<FollowButton>(find.byType(FollowButton)).userId,
        target.id,
      );
      expect(find.byTooltip('Edit Profile'), findsNothing);
      expect(find.byTooltip('Settings'), findsNothing);
      expect(find.text('User'), findsNothing);
      expect(find.text('@user'), findsNothing);
    });

    testWidgets('own profile stays auth-backed and keeps edit authority', (
      tester,
    ) async {
      await _prepareProfileTestViewport(tester);
      final owner = _authUser(id: 'owner-1', username: 'owner');

      await tester.pumpWidget(
        _wrapApp(
          authState: AuthState.authenticated(owner, emailVerified: true),
          authRepository: _FakeAuthRepository((userId) async {
            if (userId == owner.id) {
              return Result.success(owner);
            }
            return Result.success(null);
          }),
          profileRepository: _FakeProfileRepository((userId) {
            if (userId == owner.id) {
              return _profileEntity(
                userId: owner.id,
                coverPhotoUrl: 'https://example.com/owner-cover.jpg',
                location: 'Depok, West Java',
              );
            }
            return null;
          }),
          contentRepository: _FakeContentRepository(),
          followRepository: _FakeFollowRepository(),
          ratingRepository: _FakeRatingRepository(),
          profileUserId: owner.id,
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('@owner'), findsOneWidget);
      expect(find.byType(FollowButton), findsNothing);
      expect(find.byTooltip('Edit Profile'), findsOneWidget);
      expect(find.byTooltip('Settings'), findsOneWidget);
      expect(find.text('User'), findsNothing);
      expect(find.text('@user'), findsNothing);
    });

    testWidgets('unavailable user renders explicit unavailable state', (
      tester,
    ) async {
      await _prepareProfileTestViewport(tester);
      final owner = _authUser(id: 'owner-1', username: 'owner');

      await tester.pumpWidget(
        _wrapApp(
          authState: AuthState.authenticated(owner, emailVerified: true),
          authRepository: _FakeAuthRepository(
            (_) async => Result.success(null),
          ),
          profileRepository: _FakeProfileRepository((_) => null),
          contentRepository: _FakeContentRepository(),
          followRepository: _FakeFollowRepository(),
          ratingRepository: _FakeRatingRepository(),
          profileUserId: 'missing-user',
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
      expect(
        find.text('Akun ini tidak ditemukan atau sudah tidak tersedia.'),
        findsOneWidget,
      );
      expect(find.text('User'), findsNothing);
      expect(find.text('@user'), findsNothing);
      expect(find.byType(FollowButton), findsNothing);
      expect(find.byTooltip('Edit Profile'), findsNothing);
      expect(find.byTooltip('Settings'), findsNothing);
    });
  });
}
