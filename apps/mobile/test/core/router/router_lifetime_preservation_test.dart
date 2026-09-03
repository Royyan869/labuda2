import 'package:firebase_core/firebase_core.dart';
// ignore: depend_on_referenced_packages
import 'package:firebase_core_platform_interface/test.dart';
import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/app.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/screens/create_for_sale_screen.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart';
import 'package:labuda/domains/commerce/pricing/promotion/promotion.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/promotion_providers.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/rating/data/rating_providers.dart';
import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';
import 'package:labuda/domains/social/rating/domain/repositories/i_rating_repository.dart';
import 'package:labuda/domains/social/rating/presentation/providers/rating_provider.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data
    show authRepositoryProvider;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userSyncServiceProvider;
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_view_provider.dart';
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen.dart';
import 'package:labuda/domains/user/profile/presentation/screens/settings_screen.dart';
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/domains/system/notification/services/fcm_service.dart';
import 'package:labuda/domains/system/notification/services/local_notification_service.dart';
import 'package:labuda/features/explore/explore.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

const _profileUserId = '00000000-0000-0000-0000-00000000a001';
const _sellerUserId = '00000000-0000-0000-0000-00000000a002';
const _forSaleId = 'for-sale-001';

AuthUser _buildUser({
  required String id,
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
  DateTime? updatedAt,
}) {
  final now = DateTime.utc(2026, 7, 31, 6);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: updatedAt ?? now,
    email: '$id@example.com',
    username: hasSellerProfile ? 'seller_user' : 'profile_user',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    hasSellerProfile: hasSellerProfile,
    hasMarketAuthority: hasMarketAuthority,
    sellerSubscriptionStatus: hasMarketAuthority == true ? 'active' : 'none',
    roles: hasSellerProfile == true
        ? const [UserRole.user]
        : const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

AuthStateAuthenticated _buildAuthenticatedState(AuthUser user) {
  return AuthState.authenticated(user, emailVerified: true)
      as AuthStateAuthenticated;
}

ProfileEntity _buildProfileEntity(String userId) {
  final now = DateTime.utc(2026, 7, 31, 6);
  return ProfileEntity(
    id: userId,
    userId: userId,
    location: 'Jakarta, Indonesia',
    joinedAt: now,
    stats: const ProfileStats(followersCount: 0, followingCount: 0),
    verification: const UserVerificationInfo(
      isPhoneVerified: false,
      isEmailVerified: false,
      isIdVerified: false,
      isFarmVerified: false,
      badges: <ProfileBadge>[],
    ),
  );
}

ForSale _buildForSale() {
  final now = DateTime.utc(2026, 7, 31, 6);
  return ForSale(
    forSaleId: _forSaleId,
    productId: 'product-001',
    title: 'Showa Koi 30cm',
    description: 'Premium showa for lifecycle preservation proof.',
    price: 1500000,
    stock: 1,
    sellerId: _sellerUserId,
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerAvatar: null,    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    sellerTier: null,
    status: ForSaleStatus.active,
    visibility: ForSaleVisibility.public,
    isNegotiable: true,
    location: null,
    viewCount: 0,
    preparationTime: PreparationTime.immediate,
    createdAt: now,
    updatedAt: now,
    variety: 'Showa',
    sizeCm: 30,
    ageMonths: 14,
    gender: 'female',
    breeder: 'Kohaku Lab',
    bloodline: 'Matsunosuke',
  );
}

Auction _buildAuction() {
  final now = DateTime.utc(2026, 7, 31, 6);
  return Auction(
    id: 'auction-001',
    sellerId: _sellerUserId,
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerAvatar: null,    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    title: 'Auction Showa 28cm',
    description: 'Auction preview item.',
    media: const [],
    koiDetails: const KoiDetails(
      variety: 'Showa',
      sizeInCm: 28,
      ageInMonths: 12,
      gender: 'female',
    ),
    openingBid: 1000000,
    currentBid: 0,
    bidIncrement: 100000,
    buyNowPrice: null,
    condition: AuctionCondition.good,
    startTime: now,
    endTime: now.add(const Duration(hours: 24)),
    status: AuctionStatus.active,
    createdAt: now,
    decision: const DecisionContract(state: 'active'),
  );
}

class _NoopLoggerService implements ILoggerService {
  const _NoopLoggerService();

  Result<void> _ok() => Result.success(null);

  @override
  Future<Result<void>> clearLogs() async => _ok();

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<void> debugCallingGetCurrentUser() async {}

  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}

  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}

  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}

  @override
  Future<void> debugSync(String userId) async {}

  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}

  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}

  @override
  Future<void> debugSyncSuccess(String userId) async {}

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _ok();

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async => Result.success(const <LogEntry>[]);

  @override
  Future<Result<void>> fatal(
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
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}

  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) async => _ok();

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) async => _ok();

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) async => _ok();

  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) async => _ok();

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async => _ok();

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _ok();
}

class _NoopAnalyticsRepository implements IAnalyticsRepository {
  const _NoopAnalyticsRepository();

  Result<void> _ok() => Result.success(null);

  @override
  Future<Result<AnalyticsCircumventionStats>> getCircumventionStats({
    required DateTime startDate,
    required DateTime endDate,
    String? userId,
    String? violationType,
  }) async => Result.success(
    const AnalyticsCircumventionStats(
      totalAttempts: 0,
      uniqueUsers: 0,
      violationTypes: <String, int>{},
      dailyAttempts: <String, int>{},
      averageConfidence: 0,
      blockedAttempts: 0,
      filteredAttempts: 0,
    ),
  );

  @override
  Future<Result<void>> flush() async => _ok();

  @override
  Future<Result<void>> logCircumventionAttempt(
    String content,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async => _ok();

  @override
  Future<Result<void>> logUserAction(
    String action,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => _ok();

  @override
  Future<Result<void>> setUserProperties(
    Map<String, dynamic> properties,
  ) async => _ok();

  @override
  Future<Result<void>> trackEngagement({
    required String userId,
    required String contentId,
    required String contentType,
    required String engagementType,
    int? duration,
  }) async => _ok();

}

class _FakeAuthRepository extends Fake implements IAuthRepository {}

class _FakeLocalStorageService extends Fake implements ILocalStorageService {}

class _FakeFcmService extends Fake implements FcmService {}

class _FakeLocalNotificationService extends Fake
    implements LocalNotificationService {}

class _FakeFirebaseUser extends Fake implements User {
  _FakeFirebaseUser(this.uid);

  @override
  final String uid;

  @override
  Future<String> getIdToken([bool forceRefresh = false]) async => 'token';

  @override
  Future<void> reload() async {}
}

class _FakeFirebaseAuth extends Fake implements FirebaseAuth {
  _FakeFirebaseAuth(this._currentUser);

  final User? _currentUser;

  @override
  User? get currentUser => _currentUser;
}

class _HarnessUserSyncService extends UserSyncService {
  _HarnessUserSyncService({
    required AuthUser syncUser,
    required FirebaseAuth firebaseAuth,
  }) : _syncUser = syncUser,
       super(
         firebaseAuth: firebaseAuth,
         datasource: UserApiDatasource(
           ApiClient(logger: const _NoopLoggerService()),
           logger: const _NoopLoggerService(),
         ),
         logger: const _NoopLoggerService(),
         localStorage: _FakeLocalStorageService(),
       );

  final AuthUser _syncUser;

  @override
  Future<Result<AuthUser>> getCurrentUser() async {
    return Result.success(_syncUser);
  }
}

class _HarnessAuthController extends AuthController {
  _HarnessAuthController({required this.authState, required this.firebaseUser});

  final AuthStateAuthenticated authState;
  final User? firebaseUser;

  @override
  bool get shouldInitializeAuthListener => false;

  @override
  User? get activeFirebaseUser => firebaseUser;

  @override
  AuthState build() {
    super.build();
    return authState;
  }
}

class _FakeHomeRepository implements HomeRepository {
  @override
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  }) async {
    return const FeedPage(items: <FeedItem>[], hasMore: false);
  }

  @override
  Stream<List<FeedItem>> watchFeedItems({
    int limit = 20,
    String? currentUserId,
  }) {
    return const Stream<List<FeedItem>>.empty();
  }

  @override
  Future<void> refreshFeedItems() async {}
}

class _FakeContentRepository implements ContentRepository {
  @override
  Future<ContentRepositoryResult<Content>> createContent({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    List<MediaEntity> media = const [],
    List<String> tags = const [],
    List<String> mentionedUserIds = const [],
    ContentSettings settings = const ContentSettings(),
    ContentLocation? location,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<void>> deleteContent(String contentId) async =>
      ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<Content>> getContentById(
    String contentId,
  ) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
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
      nextCursor: null,
      hasMore: false,
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
    String? location,
  }) async => ContentRepositoryResult.error('not used');

  @override
  Future<ContentRepositoryResult<Content>> updateContent(
    String contentId,
    Content content,
  ) async => ContentRepositoryResult.error('not used');

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _FakeNotificationRepository extends Fake
    implements INotificationRepository {
  @override
  Stream<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) => Stream.value(Result.success(const <NotificationEntity>[]));

  @override
  Future<Result<void>> markAsRead({required String notificationId}) async =>
      Result.success(null);

  @override
  Future<Result<void>> markAsReadByEntity({
    required String userId,
    required String entityType,
    required String entityId,
  }) async => Result.success(null);

  @override
  Future<Result<void>> markAllAsRead({required String userId}) async =>
      Result.success(null);

  @override
  Stream<Result<int>> getUnreadCount({required String userId}) =>
      Stream.value(Result.success(0));

  @override
  Future<Result<NotificationPreferenceEntity>> getPreferences({
    required String userId,
  }) async => Result.success(NotificationPreferenceEntity.defaultPrefs(userId));

  @override
  Future<Result<void>> updatePreferences({
    required NotificationPreferenceEntity preferences,
  }) async => Result.success(null);

  @override
  Future<Result<void>> deleteNotification({required String notificationId}) =>
      Future.value(Result.success(null));

  @override
  Future<Result<void>> deleteAllNotifications({required String userId}) =>
      Future.value(Result.success(null));

  @override
  Future<Result<int>> deleteReadNotifications({required String userId}) async =>
      Result.success(0);
}

class _FakeFollowRepository extends Fake implements IFollowRepository {
  @override
  Future<Result<bool>> followUser({
    required String followerId,
    required String followingId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> unfollowUser({
    required String followerId,
    required String followingId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> blockUser({
    required String userId,
    required String targetUserId,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> unblockUser({
    required String userId,
    required String targetUserId,
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
  Future<Result<bool>> checkFollowStatus({
    required String followerId,
    required String followingId,
  }) async => Result.success(false);

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
}

class _FakeRatingRepository implements IRatingRepository {
  @override
  Future<Result<Rating>> createRatingForOrder({
    required String orderId,
    required int ratingValue,
    String? comment,
  }) async => Result.error('not used');

  @override
  Future<Result<List<Rating>>> getRatingsReceived({
    required String sellerId,
    int limit = 20,
    int? cursor,
  }) async => Result.success(const <Rating>[]);

  @override
  Future<Result<List<Rating>>> getRatingsGiven({
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

  @override
  Future<Result<Rating?>> getRatingForOrder({
    required String orderId,
  }) async => Result.success(null);
}

Future<ProviderContainer> _buildContainer({
  required AuthStateAuthenticated authState,
  required AuthUser syncUser,
  required ForSale forSale,
  required Auction auction,
}) async {
  final fakeFirebaseUser = _FakeFirebaseUser(syncUser.id);
  final registry = NavigationRegistryImpl();
  registerHomeTab(registry);
  registerExploreTab(registry);

  final overrides = [
    authControllerProvider.overrideWith(
      () => _HarnessAuthController(
        authState: authState,
        firebaseUser: fakeFirebaseUser,
      ),
    ),
    auth_data.authRepositoryProvider.overrideWithValue(_FakeAuthRepository()),
    loggerServiceProvider.overrideWithValue(const _NoopLoggerService()),
    coreAnalyticsRepositoryProvider.overrideWithValue(
      const _NoopAnalyticsRepository(),
    ),
    localStorageServiceProvider.overrideWithValue(_FakeLocalStorageService()),
    webSocketServiceProvider.overrideWithValue(
      WebSocketService(baseUrl: 'ws://example.invalid'),
    ),
    notificationRepositoryProvider.overrideWithValue(
      _FakeNotificationRepository(),
    ),
    followRepositoryProvider.overrideWithValue(_FakeFollowRepository()),
    fcmServiceProvider.overrideWithValue(_FakeFcmService()),
    localNotificationServiceProvider.overrideWithValue(
      _FakeLocalNotificationService(),
    ),
    apiClientProvider.overrideWithValue(
      ApiClient(logger: const _NoopLoggerService()),
    ),
    navigationRegistryProvider.overrideWithValue(registry),
    homeRepositoryProvider.overrideWithValue(_FakeHomeRepository()),
    contentRepositoryProvider.overrideWithValue(_FakeContentRepository()),
    ratingRepositoryProvider.overrideWithValue(_FakeRatingRepository()),
    profileViewDataProvider.overrideWith((ref, userId) async {
      if (userId != _profileUserId && userId != _sellerUserId) {
        return null;
      }
      final user = userId == _profileUserId
          ? _buildUser(
              id: _profileUserId,
              hasSellerProfile: false,
              hasMarketAuthority: false,
            )
          : _buildUser(
              id: _sellerUserId,
              hasSellerProfile: true,
              hasMarketAuthority: true,
            );
      return ProfileViewData(user: user, profile: _buildProfileEntity(userId));
    }),
    forSalesProvider.overrideWith((ref, params) async {
      return [forSale];
    }),
    exploreAuctionsStreamProvider.overrideWith((ref) {
      return Stream.value([auction]);
    }),
    forSaleDetailProvider.overrideWith((ref, forSaleId) async {
      if (forSaleId == forSale.forSaleId) {
        return forSale;
      }
      return null;
    }),
    fixedPriceSaleActivePromotionsProvider.overrideWith((
      ref,
      fixedPriceSaleId,
    ) async {
      return Result.success(const <PromotionInstance>[]);
    }),
    getUserRatingSummaryProvider.overrideWith((ref, userId) async {
      return Result.success(
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
    }),
    userSyncServiceProvider.overrideWithValue(
      _HarnessUserSyncService(
        syncUser: syncUser,
        firebaseAuth: _FakeFirebaseAuth(fakeFirebaseUser),
      ),
    ),
  ];

  return ProviderContainer(overrides: overrides);
}

Widget _buildApp(ProviderContainer container) {
  return UncontrolledProviderScope(
    container: container,
    child: const LabudaApp(),
  );
}

Future<void> _pumpHarness(
  WidgetTester tester,
  ProviderContainer container,
) async {
  await tester.binding.setSurfaceSize(const Size(1600, 2400));
  addTearDown(() async {
    await tester.binding.setSurfaceSize(null);
  });
  await tester.pumpWidget(_buildApp(container));
  await tester.pumpAndSettle();
}

Future<void> _refreshSamePrincipal(
  WidgetTester tester,
  ProviderContainer container,
) async {
  await container.read(authControllerProvider.notifier).refreshUserData();
  await tester.pumpAndSettle();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
    await initializeRouterModules();
  });

  testWidgets(
    '/profile keeps the same screen and router identity after same-principal refresh',
    (tester) async {
      final user = _buildUser(
        id: _profileUserId,
        hasSellerProfile: false,
        hasMarketAuthority: false,
      );
      final refreshedUser = user;
      final container = await _buildContainer(
        authState: _buildAuthenticatedState(user),
        syncUser: refreshedUser,
        forSale: _buildForSale(),
        auction: _buildAuction(),
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);
      final router = container.read(goRouterProvider);
      router.go(RoutePaths.profile);
      await tester.pumpAndSettle();

      expect(find.byType(ProfileScreen), findsOneWidget);
      final profileState = tester.state(find.byType(ProfileScreen));
      final routerBefore = container.read(goRouterProvider);

      await _refreshSamePrincipal(tester, container);

      expect(container.read(goRouterProvider), same(routerBefore));
      expect(tester.state(find.byType(ProfileScreen)), same(profileState));
    },
  );

  testWidgets(
    '/settings preserves local toggles after same-principal refresh',
    (tester) async {
      final user = _buildUser(
        id: _profileUserId,
        hasSellerProfile: false,
        hasMarketAuthority: false,
      );
      final refreshedUser = user;
      final container = await _buildContainer(
        authState: _buildAuthenticatedState(user),
        syncUser: refreshedUser,
        forSale: _buildForSale(),
        auction: _buildAuction(),
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);
      container.read(goRouterProvider).go(RoutePaths.settings);
      await tester.pumpAndSettle();

      expect(find.byType(SettingsScreen), findsOneWidget);
      final settingsState = tester.state(find.byType(SettingsScreen));
      final onlineStatusTile = find.widgetWithText(
        SwitchListTile,
        'Show Online Status',
      );

      expect(onlineStatusTile, findsOneWidget);
      final valueBeforeTap =
          tester.widget<SwitchListTile>(onlineStatusTile).value;
      await tester.tap(onlineStatusTile);
      await tester.pump();

      final valueAfterTap =
          tester.widget<SwitchListTile>(onlineStatusTile).value;
      expect(valueAfterTap, isNot(valueBeforeTap));
      final routerBefore = container.read(goRouterProvider);

      await _refreshSamePrincipal(tester, container);

      expect(container.read(goRouterProvider), same(routerBefore));
      expect(tester.state(find.byType(SettingsScreen)), same(settingsState));
      expect(
        tester.widget<SwitchListTile>(onlineStatusTile).value,
        valueAfterTap,
      );
    },
  );

  testWidgets('for-sale detail stays mounted and router identity stays stable', (
    tester,
  ) async {
    final user = _buildUser(
      id: _profileUserId,
      hasSellerProfile: false,
      hasMarketAuthority: false,
    );
    final refreshedUser = user;
    final forSale = _buildForSale();
    final container = await _buildContainer(
      authState: _buildAuthenticatedState(user),
      syncUser: refreshedUser,
      forSale: forSale,
      auction: _buildAuction(),
    );
    addTearDown(container.dispose);

    await _pumpHarness(tester, container);
    container.read(goRouterProvider).go('/for-sale/${forSale.forSaleId}');
    await tester.pumpAndSettle();

    expect(find.byType(ForSaleDetailScreen), findsOneWidget);
    final detailState = tester.state(find.byType(ForSaleDetailScreen));
    final routerBefore = container.read(goRouterProvider);

    await _refreshSamePrincipal(tester, container);

    expect(container.read(goRouterProvider), same(routerBefore));
    expect(tester.state(find.byType(ForSaleDetailScreen)), same(detailState));
  });

  testWidgets(
    'create listing form keeps text state after same-principal refresh',
    (tester) async {
      final sellerUser = _buildUser(
        id: _sellerUserId,
        hasSellerProfile: true,
        hasMarketAuthority: true,
      );
      final refreshedSeller = sellerUser;
      final container = await _buildContainer(
        authState: _buildAuthenticatedState(sellerUser),
        syncUser: refreshedSeller,
        forSale: _buildForSale(),
        auction: _buildAuction(),
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);
      container.read(goRouterProvider).go(RoutePaths.createForSale);
      await tester.pumpAndSettle();

      expect(find.byType(CreateForSaleScreen), findsOneWidget);
      final createState = tester.state(find.byType(CreateForSaleScreen));
      final titleField = find.byType(TextFormField).first;

      await tester.enterText(titleField, 'Lifecycle Proof ForSale');
      await tester.pump();

      expect(find.text('Lifecycle Proof ForSale'), findsWidgets);
      final routerBefore = container.read(goRouterProvider);

      await _refreshSamePrincipal(tester, container);

      expect(container.read(goRouterProvider), same(routerBefore));
      expect(tester.state(find.byType(CreateForSaleScreen)), same(createState));
      expect(find.text('Lifecycle Proof ForSale'), findsWidgets);
    },
  );

  testWidgets(
    'shell route keeps branch selection and shell identity after same-principal refresh',
    (tester) async {
      final user = _buildUser(
        id: _profileUserId,
        hasSellerProfile: false,
        hasMarketAuthority: false,
      );
      final refreshedUser = user;
      final container = await _buildContainer(
        authState: _buildAuthenticatedState(user),
        syncUser: refreshedUser,
        forSale: _buildForSale(),
        auction: _buildAuction(),
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);
      container.read(goRouterProvider).go(RoutePaths.home);
      await tester.pumpAndSettle();

      expect(find.byType(MainScreen), findsOneWidget);
      final mainState = tester.state(find.byType(MainScreen));

      await tester.tap(find.text('Explore'));
      await tester.pumpAndSettle();
      expect(find.byType(ExploreScreen), findsOneWidget);

      final routerBefore = container.read(goRouterProvider);
      await _refreshSamePrincipal(tester, container);

      expect(container.read(goRouterProvider), same(routerBefore));
      expect(tester.state(find.byType(MainScreen)), same(mainState));
      expect(find.byType(ExploreScreen), findsOneWidget);
    },
  );
}
