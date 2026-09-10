// ============================================================================
// GUEST WELCOME → HOME — PRODUCTION ROUTER RUNTIME PROOF
//
// Proves the Owner canonical navigation flow with the REAL production
// GoRouter (goRouterProvider) and REAL redirect logic:
//
//   Guest (unauthenticated)
//   → Welcome Screen (/welcome)
//   → tap Home icon
//   → navigateToHome → go('/home')
//   → guest redirect allows /home
//   → MainScreen (Home tab = HomeScreen) renders
//   → ForSaleListScreen must NOT render
//
// Home intent = /home = canonical Home. Home intent ≠ For Sale destination.
//
// Uses actual goRouterProvider with only the HTTP transport layer faked.
// MainScreen, HomeScreen, WelcomeScreen, feedProvider, FeedNotifier and the
// router redirect are ALL real.
//
// Scope: GUEST_HOME_NAVIGATION_CANONICAL_ROUTER_WIRING
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
import 'package:labuda/domains/commerce/catalog/for_sale/for_sale.dart'
    show ForSaleListScreen;
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
import 'package:labuda/domains/user/preference/onboarding/presentation/screens/welcome_screen.dart';
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
  _HarnessAuthController({required this.authState, this.firebaseUser});

  final AuthState authState;
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

// ============================================================================
// Container + Harness builder
// ============================================================================

Future<ProviderContainer> _buildContainer({
  required _FakeFeedHttpAdapter adapter,
  required AuthState authState,
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
  // GUEST HOME NAVIGATION (production GoRouter + production redirect logic)
  //
  // Guest → Welcome → tap Home icon → /home → MainScreen (Home tab).
  // ForSaleListScreen must NOT open.
  // ==========================================================================
  group('GUEST HOME NAVIGATION (production router)', () {
    testWidgets(
      'guest on Welcome taps Home → canonical Home opens, not For Sale',
      (tester) async {
        // Feed transport succeeds with an empty feed so HomeScreen renders
        // its shell; we assert the SCREEN that opened, not feed content.
        final adapter = _FakeFeedHttpAdapter([
          const _CannedResponse(
            statusCode: 200,
            body: {
              'data': {
                'data': <Map<String, dynamic>>[],
                'next_cursor': null,
                'has_more': false,
              },
            },
          ),
        ]);

        final guestUser = _buildTestUser();
        final container = await _buildContainer(
          adapter: adapter,
          authState: const AuthStateUnauthenticated(),
          syncUser: guestUser,
        );
        addTearDown(container.dispose);

        await _pumpHarness(tester, container);

        // Ensure the guest is parked on the Welcome Screen (guest entry).
        if (find.byType(WelcomeScreen).evaluate().isEmpty) {
          container.read(goRouterProvider).go(RoutePaths.welcome);
          await tester.pumpAndSettle();
        }
        expect(find.byType(WelcomeScreen), findsOneWidget);

        // Guest taps the Welcome Screen Home icon (canonical Home action).
        final homeIcon = find.byIcon(Icons.home_outlined);
        expect(homeIcon, findsOneWidget);
        await tester.tap(homeIcon);
        await tester.pumpAndSettle();

        // ROUTE PROOF: the resolved location is /home.
        final routePath = container
            .read(goRouterProvider)
            .routerDelegate
            .currentConfiguration
            .uri
            .path;
        expect(routePath, equals(RoutePaths.home));

        // SCREEN PROOF: canonical Home shell rendered (MainScreen + Home tab).
        expect(find.byType(MainScreen), findsOneWidget);
        expect(find.byType(HomeScreen), findsOneWidget);
        expect(find.text('Komunitas & Marketplace Koi'), findsOneWidget);

        // NEGATIVE PROOF: the For Sale catalog screen did NOT open.
        expect(find.byType(ForSaleListScreen), findsNothing);
      },
    );
  });
}
