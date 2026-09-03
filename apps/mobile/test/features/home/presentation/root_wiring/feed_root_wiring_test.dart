// ============================================================================
// FEED ACTUAL MAINSCREEN ROOT WIRING TEST
//
// Proves the production route/MainScreen wiring actually builds:
//   canonical app route → MainScreen → Home tab → actual HomeScreen
//   → actual feedProvider lifecycle
//
// Uses actual goRouterProvider (production GoRouter) with only the HTTP
// transport layer faked. MainScreen, HomeScreen, feedProvider, FeedNotifier,
// homeRepositoryProvider, and feedApiDatasourceProvider are ALL real.
//
// Scope: CONTENT_FEED_ACTUAL_MAINSCREEN_ROOT_WIRING
// ============================================================================

import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:firebase_auth/firebase_auth.dart' hide AuthProvider;
import 'package:firebase_core/firebase_core.dart';
// ignore: depend_on_referenced_packages
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/app.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/promotion_providers.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_instance.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/domain/repositories/like_repository.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/social/rating/data/rating_providers.dart';
import 'package:labuda/domains/social/rating/domain/entities/rating_entity.dart';
import 'package:labuda/domains/social/rating/domain/repositories/i_rating_repository.dart';
import 'package:labuda/domains/social/rating/presentation/providers/rating_provider.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/domains/system/notification/services/fcm_service.dart';
import 'package:labuda/domains/system/notification/services/local_notification_service.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data
    show authRepositoryProvider;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userSyncServiceProvider;
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_view_provider.dart';
import 'package:labuda/features/explore/explore.dart';
import 'package:labuda/features/home/home.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

// ============================================================================
// Canonical HTTP fixture builder (from cross-boundary pipeline test)
// ============================================================================

/// Builds a canonical platform-envelope response body for the Feed endpoint.
///
/// The full wire shape is:
///   { "data": { "data": [...items], "next_cursor": ..., "has_more": ... } }
Map<String, dynamic> _feedEnvelope({
  required List<Map<String, dynamic>> items,
  String? nextCursor,
  bool hasMore = false,
}) {
  return <String, dynamic>{
    'data': <String, dynamic>{
      'data': items,
      'next_cursor': nextCursor,
      'has_more': hasMore,
    },
  };
}

// ============================================================================
// Fake HTTP transport — tracks /feed request count
// ============================================================================

/// A canned response for the fake HTTP adapter.
class _CannedResponse {
  final int statusCode;
  final Map<String, dynamic>? body;
  const _CannedResponse({required this.statusCode, this.body});
}

/// Dio HttpClientAdapter that returns canned responses from a queue.
///
/// Each /feed request consumes the next [_CannedResponse]. All other requests
/// get a generic success envelope so unrelated API calls don't crash.
class _FakeFeedHttpAdapter implements HttpClientAdapter {
  final List<_CannedResponse> _responses;
  int _feedRequestCount = 0;

  /// Captured query parameters for /feed requests, in call order.
  final List<Map<String, dynamic>> capturedQueryParams = [];

  _FakeFeedHttpAdapter(List<_CannedResponse> responses)
    : _responses = responses;

  ResponseBody _genericSuccess() {
    return ResponseBody.fromString(
      jsonEncode(<String, dynamic>{
        'success': true,
        'data': <String, dynamic>{},
        'timestamp': '2026-08-05T00:00:00Z',
      }),
      200,
      headers: {'content-type': ['application/json']},
    );
  }

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    // Only /feed requests consume from the canned response queue.
    if (!options.path.contains('feed') && !options.path.contains('/feed')) {
      return _genericSuccess();
    }

    _feedRequestCount++;
    capturedQueryParams.add(Map<String, dynamic>.from(
      options.queryParameters,
    ));

    if (_feedRequestCount > _responses.length) {
      // Exhausted canned responses — return empty feed.
      final body = _feedEnvelope(
        items: <Map<String, dynamic>>[],
        hasMore: false,
      );
      return ResponseBody.fromString(jsonEncode(body), 200, headers: {
        'content-type': ['application/json'],
      });
    }

    final canned = _responses[_feedRequestCount - 1];

    if (canned.body == null) {
      throw DioException(
        requestOptions: options,
        type: DioExceptionType.connectionError,
        message: 'Simulated network error',
      );
    }

    return ResponseBody.fromString(
      jsonEncode(canned.body),
      canned.statusCode,
      headers: {'content-type': ['application/json']},
    );
  }

  @override
  void close({bool force = false}) {}

  int get feedRequestCount => _feedRequestCount;

  void resetCount() {
    _feedRequestCount = 0;
  }
}

/// Create an ApiClient that routes all requests through [adapter].
ApiClient _fakeApiClient(_FakeFeedHttpAdapter adapter) {
  final client = ApiClient(logger: null);
  client.dio.httpClientAdapter = adapter;
  return client;
}

// ============================================================================
// Fake providers for UNRELATED dependencies
// ============================================================================

// -- Auth ---------------------------------------------------------------

class _FakeAuthRepository extends Fake implements IAuthRepository {}

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

// -- User Sync ----------------------------------------------------------

class _HarnessUserSyncService extends UserSyncService {
  _HarnessUserSyncService({
    required AuthUser syncUser,
    required FirebaseAuth firebaseAuth,
  }) : _syncUser = syncUser,
       super(
         firebaseAuth: firebaseAuth,
         datasource: UserApiDatasource(
           ApiClient(logger: null),
           logger: const _NoopLogger(),
         ),
         logger: const _NoopLogger(),
         localStorage: _FakeLocalStorage(),
       );

  final AuthUser _syncUser;

  @override
  Future<Result<AuthUser>> getCurrentUser() async {
    return Result.success(_syncUser);
  }
}

// -- Logger -------------------------------------------------------------

class _NoopLogger implements ILoggerService {
  const _NoopLogger();

  Result<void> _ok() => Result.success(null);

  @override
  Future<Result<void>> clearLogs() async => _ok();

  @override
  Future<Result<void>> debug(
    String message, {Map<String, dynamic>? extra,}
  ) async => _ok();

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
    String message, {Map<String, dynamic>? extra, StackTrace? stackTrace,}
  ) async => _ok();

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async => Result.success(const <LogEntry>[]);

  @override
  Future<Result<void>> fatal(
    String message, {Map<String, dynamic>? extra, StackTrace? stackTrace,}
  ) async => _ok();

  @override
  Future<Result<void>> info(
    String message, {Map<String, dynamic>? extra,}
  ) async => _ok();

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
    String operation, {required Duration duration, Map<String, dynamic>? metrics,}
  ) async => _ok();

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {String? userId, String? severity, Map<String, dynamic>? details,}
  ) async => _ok();

  @override
  Future<Result<void>> logUserAction(
    String action, {String? userId, Map<String, dynamic>? parameters,}
  ) async => _ok();

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async => _ok();

  @override
  Future<Result<void>> warning(
    String message, {Map<String, dynamic>? extra,}
  ) async => _ok();
}

// -- Analytics ----------------------------------------------------------

class _NoopAnalytics implements IAnalyticsRepository {
  const _NoopAnalytics();

  Result<void> _ok() => Result.success(null);

  @override
  Future<Result<AnalyticsCircumventionStats>> getCircumventionStats({
    required DateTime startDate,
    required DateTime endDate,
    String? userId,
    String? violationType,
  }) async => Result.success(const AnalyticsCircumventionStats(
    totalAttempts: 0,
    uniqueUsers: 0,
    violationTypes: <String, int>{},
    dailyAttempts: <String, int>{},
    averageConfidence: 0,
    blockedAttempts: 0,
    filteredAttempts: 0,
  ));

  @override
  Future<Result<void>> flush() async => _ok();

  @override
  Future<Result<void>> logCircumventionAttempt(
    String content,
    String userId, {Map<String, dynamic>? extra,}
  ) async => _ok();

  @override
  Future<Result<void>> logEvent(
    String eventName, {Map<String, dynamic>? parameters, String? userId,}
  ) async => _ok();

  @override
  Future<Result<void>> logUserAction(
    String action,
    String userId, {Map<String, dynamic>? extra,}
  ) async => _ok();

  @override
  Future<Result<void>> setUserProperties(Map<String, dynamic> properties) async => _ok();

  @override
  Future<Result<void>> trackEngagement({
    required String userId,
    required String contentId,
    required String contentType,
    required String engagementType,
    int? duration,
  }) async => _ok();
}

// -- Local Storage ------------------------------------------------------

class _FakeLocalStorage extends Fake implements ILocalStorageService {}

// -- FCM / Notifications ------------------------------------------------

class _FakeFcmService extends Fake implements FcmService {}
class _FakeLocalNotificationService extends Fake implements LocalNotificationService {}

class _FakeNotificationRepository extends Fake implements INotificationRepository {
  @override
  Stream<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) => Stream.value(Result.success(const <NotificationEntity>[]));

  @override
  Future<Result<void>> markAsRead({required String notificationId}) async =>
      Result.success(null);

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

// -- Follow -------------------------------------------------------------

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
  Future<Result<FollowStats>> getFollowStats({
    required String userId,
    String? currentUserId,
  }) async => Result.success(FollowStats(
    userId: userId,
    followersCount: 0,
    followingCount: 0,
    lastUpdated: DateTime.utc(2026, 1, 1),
  ));

  @override
  Stream<FollowStats> watchFollowStats(String userId) => Stream.value(FollowStats(
    userId: userId,
    followersCount: 0,
    followingCount: 0,
    lastUpdated: DateTime.utc(2026, 1, 1),
  ));
}

// -- Content ------------------------------------------------------------

class _FakeContentRepository extends Fake implements ContentRepository {
  @override
  Future<ContentRepositoryResult<List<Content>>> getContents({
    int? limit,
    int? offset,
    String? location,
    ContentStatus? status,
  }) async => ContentRepositoryResult.success(const <Content>[]);

  @override
  Future<ContentRepositoryResult<List<Content>>> getContentsByAuthor(
    String authorId, {int? limit, int? offset,}
  ) async => ContentRepositoryResult.success(const <Content>[]);

  @override
  Future<ContentRepositoryResult<ContentAuthorPage>> getContentsByAuthorPaged(
    String authorId, {int limit = 20, String? cursor,}
  ) async => ContentRepositoryResult.success(const ContentAuthorPage(
    items: <Content>[],
    nextCursor: null,
    hasMore: false,
  ));
}

// -- Like ---------------------------------------------------------------

class _FakeLikeRepository extends Fake implements LikeRepository {
  @override
  Future<Result<bool>> toggleLike({
    required String targetId,
    required LikeTargetType targetType,
    required String userId,
  }) async => Result.success(false);

  @override
  Future<Result<LikeStats>> getLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) async => Result.success(LikeStats(
    targetId: targetId,
    targetType: targetType,
    totalLikes: 0,
    isLikedByCurrentUser: false,
  ));

  @override
  Stream<LikeStats> watchLikeStats({
    required String targetId,
    required LikeTargetType targetType,
    required String currentUserId,
  }) => Stream.value(LikeStats(
    targetId: targetId,
    targetType: targetType,
    totalLikes: 0,
    isLikedByCurrentUser: false,
  ));
}

// -- Auction (CommercePreviewSection / exploreAuctionsStreamProvider) ---

class _FakeAuctionRepository extends Fake implements AuctionRepository {
  @override
  Stream<List<Auction>> watchActiveAuctions({int limit = 50}) {
    return Stream.value(const <Auction>[]);
  }
}

// -- Rating -------------------------------------------------------------

class _FakeRatingRepository extends Fake implements IRatingRepository {
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
  }) async => Result.success(const RatingSummary(
    totalRatings: 0,
    averageRating: 0,
    oneStarCount: 0,
    twoStarCount: 0,
    threeStarCount: 0,
    fourStarCount: 0,
    fiveStarCount: 0,
  ));

  @override
  Future<Result<Rating>> createRatingForOrder({
    required String orderId,
    required int ratingValue,
    String? comment,
  }) async => Result.error('Not used');

  @override
  Future<Result<Rating?>> getRatingForOrder({
    required String orderId,
  }) async => Result.success(null);
}

// ============================================================================
// Auth Controller harness
// ============================================================================

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

// ============================================================================
// Test user builder
// ============================================================================

const _testUserId = '00000000-0000-0000-0000-000000000001';

AuthUser _buildTestUser() {
  final now = DateTime.utc(2026, 8, 5, 10);
  return AuthUser(
    id: _testUserId,
    createdAt: now,
    updatedAt: now,
    email: 'test@example.com',
    username: 'testuser',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    hasSellerProfile: false,
    hasMarketAuthority: false,
    sellerSubscriptionStatus: 'none',
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

AuthStateAuthenticated _buildAuthState(AuthUser user) {
  return AuthState.authenticated(user, emailVerified: true)
      as AuthStateAuthenticated;
}

// ============================================================================
// Container + Harness builder
// ============================================================================

Future<ProviderContainer> _buildContainer({
  required _FakeFeedHttpAdapter adapter,
  required AuthStateAuthenticated authState,
  required AuthUser syncUser,
}) async {
  final fakeFirebaseUser = _FakeFirebaseUser(syncUser.id);
  final registry = NavigationRegistryImpl();
  registerHomeTab(registry);
  registerExploreTab(registry);

  final overrides = [
    // == TRANSPORT (the ONLY Feed pipeline override) ===================
    apiClientProvider.overrideWithValue(_fakeApiClient(adapter)),

    // == AUTH ==========================================================
    authControllerProvider.overrideWith(
      () => _HarnessAuthController(
        authState: authState,
        firebaseUser: fakeFirebaseUser,
      ),
    ),
    auth_data.authRepositoryProvider.overrideWithValue(_FakeAuthRepository()),

    // == CORE SERVICES =================================================
    loggerServiceProvider.overrideWithValue(const _NoopLogger()),
    coreAnalyticsRepositoryProvider.overrideWithValue(const _NoopAnalytics()),
    localStorageServiceProvider.overrideWithValue(_FakeLocalStorage()),
    webSocketServiceProvider.overrideWithValue(
      WebSocketService(baseUrl: 'ws://test.invalid'),
    ),
    navigationRegistryProvider.overrideWithValue(registry),

    // == NOTIFICATIONS =================================================
    fcmServiceProvider.overrideWithValue(_FakeFcmService()),
    localNotificationServiceProvider.overrideWithValue(
      _FakeLocalNotificationService(),
    ),
    notificationRepositoryProvider.overrideWithValue(
      _FakeNotificationRepository(),
    ),

    // == SOCIAL ========================================================
    followRepositoryProvider.overrideWithValue(_FakeFollowRepository()),
    contentRepositoryProvider.overrideWithValue(_FakeContentRepository()),
    likeRepositoryProvider.overrideWithValue(_FakeLikeRepository()),
    ratingRepositoryProvider.overrideWithValue(_FakeRatingRepository()),

    // == COMMERCE (CommercePreviewSection uses forSalesProvider via
    // apiClientProvider, plus auctionRepositoryProvider) ===============
    auctionRepositoryProvider.overrideWithValue(_FakeAuctionRepository()),

    // == USER SYNC =====================================================
    userSyncServiceProvider.overrideWithValue(
      _HarnessUserSyncService(
        syncUser: syncUser,
        firebaseAuth: _FakeFirebaseAuth(fakeFirebaseUser),
      ),
    ),

    // == PROFILE (MainDrawer avatar) ===================================
    profileViewDataProvider.overrideWith((ref, userId) async {
      if (userId != _testUserId) return null;
      final now = DateTime.utc(2026, 8, 5, 10);
      return ProfileViewData(
        user: syncUser,
        profile: ProfileEntity(
          id: _testUserId,
          userId: _testUserId,
          location: 'Test City',
          joinedAt: now,
          stats: const ProfileStats(followersCount: 0, followingCount: 0),
          verification: const UserVerificationInfo(
            isPhoneVerified: false,
            isEmailVerified: true,
            isIdVerified: false,
            isFarmVerified: false,
            badges: <ProfileBadge>[],
          ),
        ),
      );
    }),

    // == COMMERCE AUCTION PREVIEW ======================================
    exploreAuctionsStreamProvider.overrideWith((ref) {
      return Stream.value(const <Auction>[]);
    }),
    fixedPriceSaleActivePromotionsProvider.overrideWith((
      ref,
      fixedPriceSaleId,
    ) async {
      return Result.success(const <PromotionInstance>[]);
    }),
    getUserRatingSummaryProvider.overrideWith((ref, userId) async {
      return Result.success(const RatingSummary(
        totalRatings: 0,
        averageRating: 0,
        oneStarCount: 0,
        twoStarCount: 0,
        threeStarCount: 0,
        fourStarCount: 0,
        fiveStarCount: 0,
      ));
    }),
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

/// Pump enough frames for the Feed async chain to resolve.
/// Uses a bounded loop rather than pumpAndSettle to avoid hanging when
/// stream-based providers (ExploreScreen tabs, notification badges) keep
/// scheduling frames.
Future<void> _settleFeed(WidgetTester tester) async {
  for (int i = 0; i < 30; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

// ============================================================================
// Tests
// ============================================================================

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
    await initializeRouterModules();
  });

  // ==========================================================================
  // SCENARIO 1: Canonical route builds MainScreen and HomeScreen
  // ==========================================================================
  group('SCENARIO 1: canonical /home route builds MainScreen + HomeScreen', () {
    testWidgets('production /home route reaches MainScreen with HomeScreen', (
      tester,
    ) async {
      final adapter = _FakeFeedHttpAdapter([
        const _CannedResponse(
          statusCode: 200,
          body: null, // Signals network error → initial error state
        ),
      ]);
      // NOTE: We use a network-error response here intentionally so the
      // Feed pipeline does not render content. This proves MainScreen +
      // HomeScreen exist via widget tree, not content matching.

      final user = _buildTestUser();
      final container = await _buildContainer(
        adapter: adapter,
        authState: _buildAuthState(user),
        syncUser: user,
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);

      // Navigate to /home via production GoRouter.
      container.read(goRouterProvider).go(RoutePaths.home);
      await tester.pumpAndSettle();

      // PROOF 1a: Production route selected — MainScreen exists.
      expect(find.byType(MainScreen), findsOneWidget);

      // PROOF 1b: HomeScreen exists as child of MainScreen.
      expect(find.byType(HomeScreen), findsOneWidget);

      // PROOF 1c: Home tab is active (Home tab is index 0).
      // Verified by HomeScreen being rendered (IndexedStack at index 0).
      // We also verify the Explore tab is NOT visible (different tab).
      // ExploreScreen may be in the IndexedStack but not visible.
      // The key proof is that HomeScreen renders (it's inside IndexedStack).

      // PROOF 1d: HomeScreen renders its header text.
      expect(find.text('Komunitas & Marketplace Koi'), findsOneWidget);
    });
  });

  // ==========================================================================
  // SCENARIO 2: Feed pipeline initializes through root wiring
  // ==========================================================================
  group('SCENARIO 2: Feed pipeline initializes through root wiring', () {
    testWidgets('GET /feed occurs and Content appears inside HomeScreen', (
      tester,
    ) async {
      final adapter = _FakeFeedHttpAdapter([
        const _CannedResponse(
          statusCode: 200,
          body: {
            'data': {
              'data': [
                {
                  'id': 'feed-item-1',
                  'type': 'content',
                  'author_id': 'author-alice',
                  'author_username': 'alice',
                  'author_avatar': 'https://example.com/alice.jpg',
                  'status': 'active',
                  'body': 'Root-wired feed content!',
                  'created_at': '2026-08-05T10:00:00Z',
                  'updated_at': '2026-08-05T10:00:00Z',
                  'media': [],
                },
              ],
              'next_cursor': null,
              'has_more': false,
            },
          },
        ),
      ]);

      final user = _buildTestUser();
      final container = await _buildContainer(
        adapter: adapter,
        authState: _buildAuthState(user),
        syncUser: user,
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);
      container.read(goRouterProvider).go(RoutePaths.home);
      await _settleFeed(tester);

      // PROOF 2a: GET /feed occurred — the adapter recorded at least 1 request.
      expect(adapter.feedRequestCount, greaterThanOrEqualTo(1));

      // PROOF 2b: Feed content text appears inside HomeScreen.
      expect(find.text('Root-wired feed content!'), findsOneWidget);

      // PROOF 2c: feedProvider is initialized and has items.
      final feedState = container.read(feedProvider);
      expect(feedState.items, hasLength(1));
      expect(feedState.items[0].type, FeedItemType.content);
      expect(feedState.items[0].content, 'Root-wired feed content!');
      expect(feedState.isLoading, isFalse);

      // PROOF 2d: Empty state is absent (items are present).
      expect(find.text('🎯 Kamu ingin apa hari ini?'), findsNothing);

      // PROOF 2e: Error state is absent.
      expect(feedState.errorMessage, isNull);
    });
  });

  // ==========================================================================
  // SCENARIO 3: Feed initializes exactly once on first Home activation
  // ==========================================================================
  group('SCENARIO 3: Feed initializes exactly once', () {
    testWidgets('initial feed request count = 1 after first Home activation', (
      tester,
    ) async {
      final adapter = _FakeFeedHttpAdapter([
        const _CannedResponse(
          statusCode: 200,
          body: {
            'data': {
              'data': [
                {
                  'id': 'feed-once',
                  'type': 'content',
                  'author_id': 'author-1',
                  'author_username': 'alice',
                  'author_avatar': 'https://example.com/alice.jpg',
                  'status': 'active',
                  'body': 'One request only',
                  'created_at': '2026-08-05T10:00:00Z',
                  'updated_at': '2026-08-05T10:00:00Z',
                  'media': [],
                },
              ],
              'next_cursor': null,
              'has_more': false,
            },
          },
        ),
      ]);

      final user = _buildTestUser();
      final container = await _buildContainer(
        adapter: adapter,
        authState: _buildAuthState(user),
        syncUser: user,
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);
      container.read(goRouterProvider).go(RoutePaths.home);
      await _settleFeed(tester);

      // PROOF 3a: Content rendered.
      expect(find.text('One request only'), findsOneWidget);

      // PROOF 3b: Exactly 1 feed request was made.
      expect(adapter.feedRequestCount, 1);

      // PROOF 3c: No duplicate initialization from extra pump rounds.
      await _settleFeed(tester);
      expect(adapter.feedRequestCount, 1);

      // PROOF 3d: The feed state is stable — no loading spinners.
      final feedState = container.read(feedProvider);
      expect(feedState.isLoading, isFalse);
      expect(feedState.isLoadingMore, isFalse);
    });
  });

  // ==========================================================================
  // SCENARIO 4: Tab switch preserves Feed lifecycle
  // ==========================================================================
  group('SCENARIO 4: Tab switch preserves Feed lifecycle', () {
    testWidgets('switching to Explore and back preserves feed state', (
      tester,
    ) async {
      final adapter = _FakeFeedHttpAdapter([
        const _CannedResponse(
          statusCode: 200,
          body: {
            'data': {
              'data': [
                {
                  'id': 'feed-tab-switch',
                  'type': 'content',
                  'author_id': 'author-1',
                  'author_username': 'alice',
                  'author_avatar': 'https://example.com/alice.jpg',
                  'status': 'active',
                  'body': 'Persists across tab switch',
                  'created_at': '2026-08-05T10:00:00Z',
                  'updated_at': '2026-08-05T10:00:00Z',
                  'media': [],
                },
              ],
              'next_cursor': null,
              'has_more': false,
            },
          },
        ),
      ]);

      final user = _buildTestUser();
      final container = await _buildContainer(
        adapter: adapter,
        authState: _buildAuthState(user),
        syncUser: user,
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);
      container.read(goRouterProvider).go(RoutePaths.home);
      await _settleFeed(tester);

      // PROOF 4a: Content is present.
      expect(find.text('Persists across tab switch'), findsOneWidget);
      expect(adapter.feedRequestCount, 1);

      // PROOF 4b: Switch to Explore tab.
      await tester.tap(find.text('Explore'));
      await _settleFeed(tester);

      // ExploreScreen should now be visible.
      expect(find.byType(ExploreScreen), findsOneWidget);

      // PROOF 4c: Feed request count did NOT increment during tab switch.
      expect(adapter.feedRequestCount, 1);

      // PROOF 4d: Switch back to Home tab.
      await tester.tap(find.text('Home'));
      await _settleFeed(tester);

      // HomeScreen content is still there.
      expect(find.text('Persists across tab switch'), findsOneWidget);

      // PROOF 4e: Feed request count STILL 1 — IndexedStack preserved state.
      expect(adapter.feedRequestCount, 1);

      // PROOF 4f: FeedNotifier state is preserved.
      final feedState = container.read(feedProvider);
      expect(feedState.items, hasLength(1));
      expect(feedState.items[0].content, 'Persists across tab switch');
    });
  });

  // ==========================================================================
  // SCENARIO 5: Explicit invalidation creates a new lifecycle
  // ==========================================================================
  group('SCENARIO 5: Explicit invalidation triggers new request', () {
    testWidgets('feed refresh triggers exactly one new request', (tester) async {
      final adapter = _FakeFeedHttpAdapter([
        // Initial page.
        const _CannedResponse(
          statusCode: 200,
          body: {
            'data': {
              'data': [
                {
                  'id': 'feed-before-refresh',
                  'type': 'content',
                  'author_id': 'author-1',
                  'author_username': 'alice',
                  'author_avatar': 'https://example.com/alice.jpg',
                  'status': 'active',
                  'body': 'Before refresh',
                  'created_at': '2026-08-05T10:00:00Z',
                  'updated_at': '2026-08-05T10:00:00Z',
                  'media': [],
                },
              ],
              'next_cursor': null,
              'has_more': false,
            },
          },
        ),
        // Refresh page.
        const _CannedResponse(
          statusCode: 200,
          body: {
            'data': {
              'data': [
                {
                  'id': 'feed-after-refresh',
                  'type': 'content',
                  'author_id': 'author-2',
                  'author_username': 'bob',
                  'author_avatar': 'https://example.com/bob.jpg',
                  'status': 'active',
                  'body': 'After refresh',
                  'created_at': '2026-08-05T10:01:00Z',
                  'updated_at': '2026-08-05T10:01:00Z',
                  'media': [],
                },
              ],
              'next_cursor': null,
              'has_more': false,
            },
          },
        ),
      ]);

      final user = _buildTestUser();
      final container = await _buildContainer(
        adapter: adapter,
        authState: _buildAuthState(user),
        syncUser: user,
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);
      container.read(goRouterProvider).go(RoutePaths.home);
      await _settleFeed(tester);

      // PROOF 5a: Initial content is visible, 1 request made.
      expect(find.text('Before refresh'), findsOneWidget);
      expect(adapter.feedRequestCount, 1);

      // PROOF 5b: Invalidate the feed provider to trigger a fresh lifecycle.
      // This is the canonical Riverpod invalidation mechanism — it disposes
      // the existing notifier (and its autoDispose repository/datasource
      // chain), then lazily creates a new one on next read, which schedules
      // a fresh loadFeed() via build()'s Future.microtask.
      container.invalidate(feedProvider);
      // Re-read to trigger rebuild + new loadFeed.
      container.read(feedProvider);
      await _settleFeed(tester);

      // PROOF 5c: Second request occurred.
      expect(adapter.feedRequestCount, 2);

      // PROOF 5d: New content merged (loadFeed replaces items).
      // Since loadFeed replaces items (not appends), the old content
      // is replaced and the new content appears.
      expect(find.text('After refresh'), findsOneWidget);
      expect(find.text('Before refresh'), findsNothing);

      // PROOF 5e: Old cursor/state was not incorrectly reused.
      // The second response had different content, proving the fetch
      // actually went through the pipeline from scratch.
      final feedState = container.read(feedProvider);
      expect(feedState.items, hasLength(1));
      expect(feedState.items[0].id, 'feed-after-refresh');
    });
  });

  // ==========================================================================
  // SCENARIO 6: MainScreen disposal releases Feed lifecycle
  // ==========================================================================
  group('SCENARIO 6: Shell disposal releases Feed lifecycle', () {
    testWidgets('navigating away from shell releases feed lifecycle', (
      tester,
    ) async {
      final adapter = _FakeFeedHttpAdapter([
        const _CannedResponse(
          statusCode: 200,
          body: {
            'data': {
              'data': [
                {
                  'id': 'feed-dispose',
                  'type': 'content',
                  'author_id': 'author-1',
                  'author_username': 'alice',
                  'author_avatar': 'https://example.com/alice.jpg',
                  'status': 'active',
                  'body': 'Will be disposed',
                  'created_at': '2026-08-05T10:00:00Z',
                  'updated_at': '2026-08-05T10:00:00Z',
                  'media': [],
                },
              ],
              'next_cursor': null,
              'has_more': false,
            },
          },
        ),
      ]);

      final user = _buildTestUser();
      final container = await _buildContainer(
        adapter: adapter,
        authState: _buildAuthState(user),
        syncUser: user,
      );
      addTearDown(container.dispose);

      await _pumpHarness(tester, container);
      container.read(goRouterProvider).go(RoutePaths.home);
      await _settleFeed(tester);

      expect(adapter.feedRequestCount, 1);
      expect(find.byType(HomeScreen), findsOneWidget);

      // PROOF 6a: Navigate away from MainScreen shell.
      container.read(goRouterProvider).go(RoutePaths.settings);
      await _settleFeed(tester);

      // MainScreen is no longer in the widget tree.
      expect(find.byType(MainScreen), findsNothing);

      // PROOF 6b: No exception was thrown during disposal of HomeScreen.
      // (The test itself not throwing is the proof.)

      // PROOF 6c: No unexpected additional Feed request during/after navigation.
      expect(adapter.feedRequestCount, 1);

      // PROOF 6d: refreshFeedGlobally after leaving the shell does not
      // issue a new /feed request. HomeScreen's callback still exists
      // (production has no clearGlobalFeedRefreshCallback) but is gated
      // by `mounted`, so invalidate is skipped.
      refreshFeedGlobally();
      await _settleFeed(tester);
      expect(adapter.feedRequestCount, 1);

      // Isolate later tests: overwrite the session callback with a no-op
      // using the existing production setter.
      setGlobalFeedRefreshCallback(() {});
    });
  });
}
