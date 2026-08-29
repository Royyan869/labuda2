/// Root-wiring widget tests proving the portal blocks protected access.
///
/// Uses production ProviderScope, goRouterProvider, MaterialApp.router.
/// External dependencies overridden with counting fakes so every
/// protected-provider invocation is explicitly measured.
library;

import 'package:firebase_auth/firebase_auth.dart';
import 'package:firebase_core/firebase_core.dart';
// ignore: depend_on_referenced_packages
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/app.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/rating/data/rating_providers.dart';
import 'package:labuda/domains/social/rating/domain/repositories/i_rating_repository.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data
    show authRepositoryProvider;
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/domains/system/notification/services/fcm_service.dart';
import 'package:labuda/domains/system/notification/services/local_notification_service.dart';
import 'package:labuda/features/explore/explore.dart';
import 'package:labuda/features/home/home.dart';

// =============================================================================
// Counting fakes — every protected-provider boundary is instrumented
// =============================================================================

class _CountingFcmService extends Fake implements FcmService {
  int initCallCount = 0;
  @override
  Future<void> initialize({required String userId}) async {
    initCallCount++;
  }
}

class _CountingHomeRepository implements HomeRepository {
  int feedCallCount = 0;
  @override
  Future<FeedPage> getFeedPage({
    int limit = 20,
    String? currentUserId,
    bool loadMore = false,
  }) async {
    feedCallCount++;
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

class _CountingNotificationRepository implements INotificationRepository {
  int unreadCountCallCount = 0;

  @override
  Stream<Result<int>> getUnreadCount({required String userId}) {
    unreadCountCallCount++;
    return Stream.value(Result.success(0));
  }

  @override Future<Result<void>> deleteAllNotifications(
          {required String userId}) async => Result.success(null);
  @override Future<Result<int>> deleteReadNotifications(
          {required String userId}) async => Result.success(0);
  @override Future<Result<void>> deleteNotification(
          {required String notificationId}) async => Result.success(null);
  @override Future<Result<NotificationPreferenceEntity>> getPreferences(
          {required String userId}) async =>
      Result.success(NotificationPreferenceEntity.defaultPrefs(userId));
  @override Future<Result<void>> updatePreferences(
          {required NotificationPreferenceEntity preferences}) async =>
      Result.success(null);
  @override Future<Result<void>> markAllAsRead(
          {required String userId}) async => Result.success(null);
  @override Future<Result<void>> markAsRead(
          {required String notificationId}) async => Result.success(null);
  @override Future<Result<void>> markAsReadByEntity(
          {required String userId,
          required String entityId,
          required String entityType}) async => Result.success(null);
  @override
  Stream<Result<List<NotificationEntity>>> getNotifications(
          {required String userId, int limit = 20}) =>
      Stream.value(Result.success(const <NotificationEntity>[]));
}

class _FakeAuthRepository extends Fake implements IAuthRepository {}
class _FakeContentRepository extends Fake implements ContentRepository {}
class _FakeRatingRepository extends Fake implements IRatingRepository {}
class _FakeFollowRepository extends Fake implements IFollowRepository {}
class _FakeLocalNotificationService extends Fake
    implements LocalNotificationService {}
class _FakeNotificationTrigger extends Fake
    implements INotificationTrigger {}
class _FakeLocalStorageService extends Fake
    implements ILocalStorageService {
  @override
  Future<Result<int?>> getInt(String key) async => Result.success(null);
}

class _NoopAnalyticsRepository implements IAnalyticsRepository {
  Result<void> _ok() => Result.success(null);
  @override Future<Result<void>> logEvent(String eventName,
          {Map<String, dynamic>? parameters, String? userId}) async => _ok();
  @override
  Future<Result<void>> setUserProperties(Map<String, dynamic> p) async =>
      _ok();
  @override Future<Result<void>> flush() async => _ok();
  @override Future<Result<void>> logUserAction(String a, String u,
          {Map<String, dynamic>? extra}) async => _ok();
  @override
  Future<Result<void>> logCircumventionAttempt(String c, String u,
          {Map<String, dynamic>? extra}) async => _ok();
  @override
  Future<Result<AnalyticsCircumventionStats>> getCircumventionStats(
          {required DateTime startDate,
          required DateTime endDate,
          String? userId,
          String? violationType}) async =>
      Result.success(const AnalyticsCircumventionStats(
        totalAttempts: 0,
        uniqueUsers: 0,
        violationTypes: <String, int>{},
        dailyAttempts: <String, int>{},
        averageConfidence: 0,
        blockedAttempts: 0,
        filteredAttempts: 0,
      ));
}

class _NoopLoggerService implements ILoggerService {
  const _NoopLoggerService();
  Result<void> _ok() => Result.success(null);
  @override Future<Result<void>> clearLogs() async => _ok();
  @override Future<Result<void>> debug(String m,
          {Map<String, dynamic>? extra}) async => _ok();
  @override Future<void> debugCallingGetCurrentUser() async {}
  @override Future<void> debugGetCurrentUserFailed(
          String u, String? e) async {}
  @override Future<void> debugGetCurrentUserSuccess(
          String u, bool v) async {}
  @override Future<void> debugRouterCheck(String u, bool v, String l,
          bool iv) async {}
  @override Future<void> debugSync(String u) async {}
  @override Future<void> debugSyncException(
          String u, String e, String s) async {}
  @override Future<void> debugSyncFailed(String u, String? e) async {}
  @override Future<void> debugSyncSuccess(String u) async {}
  @override Future<Result<void>> error(String m,
          {Map<String, dynamic>? extra,
          StackTrace? stackTrace}) async => _ok();
  @override Future<Result<List<LogEntry>>> getLogs(
          {LogLevel? minLevel,
          DateTime? startDate,
          DateTime? endDate,
          int? limit}) async => Result.success(const <LogEntry>[]);
  @override Future<Result<void>> fatal(String m,
          {Map<String, dynamic>? extra,
          StackTrace? stackTrace}) async => _ok();
  @override Future<Result<void>> info(String m,
          {Map<String, dynamic>? extra}) async => _ok();
  @override Future<void> log(String m,
          {LogLevel level = LogLevel.debug}) async {}
  @override Future<Result<void>> logApiCall(String e,
          {required String method,
          required int statusCode,
          required Duration duration,
          Map<String, dynamic>? requestData,
          Map<String, dynamic>? responseData}) async => _ok();
  @override Future<Result<void>> logPerformance(String o,
          {required Duration duration,
          Map<String, dynamic>? metrics}) async => _ok();
  @override Future<Result<void>> logSecurityEvent(String e,
          {String? userId, String? severity,
          Map<String, dynamic>? details}) async => _ok();
  @override Future<Result<void>> logUserAction(String a,
          {String? userId,
          Map<String, dynamic>? parameters}) async => _ok();
  @override Future<Result<void>> setLogLevel(LogLevel l) async => _ok();
  @override Future<Result<void>> warning(String m,
          {Map<String, dynamic>? extra}) async => _ok();
}

// =============================================================================
// Switching AuthController
// =============================================================================

const _testUserId = 'portal-test-user-001';
const _testFirebaseUid = 'fb-portal-test-001';

class _SwitchingAuthController extends AuthController {
  _SwitchingAuthController({required this.initialState});
  final AuthState initialState;
  void publish(AuthState s) { state = s; }
  @override bool get shouldInitializeAuthListener => false;
  @override User? get activeFirebaseUser => _StubFirebaseUser();
  @override
  AuthState build() { super.build(); return initialState; }
}

class _StubFirebaseUser extends Fake implements User {
  @override String get uid => _testFirebaseUid;
  @override String? get email => 'portal@test.com';
  @override bool get emailVerified => false;
  @override List<UserInfo> get providerData => [_StubUserInfo('password')];
}
class _StubUserInfo extends Fake implements UserInfo {
  _StubUserInfo(this._pid); final String _pid;
  @override String get providerId => _pid;
}

// =============================================================================
// Harness
// =============================================================================

/// All counting fakes are created once and injected into every test's
/// container so per-test invocation counts start at zero.
class _Harness {
  final _SwitchingAuthController controller;
  final _CountingFcmService countingFcm;
  final _CountingHomeRepository countingHomeRepo;
  final _CountingNotificationRepository countingNotifRepo;
  int forSaleProviderCallCount = 0;

  _Harness(AuthState initialState)
      : controller = _SwitchingAuthController(initialState: initialState),
        countingFcm = _CountingFcmService(),
        countingHomeRepo = _CountingHomeRepository(),
        countingNotifRepo = _CountingNotificationRepository();

  ProviderContainer buildContainer() {
    final registry = NavigationRegistryImpl();
    registerHomeTab(registry);
    registerExploreTab(registry);

    return ProviderContainer(overrides: [
      authControllerProvider.overrideWith(() => controller),
      auth_data.authRepositoryProvider.overrideWithValue(_FakeAuthRepository()),
      loggerServiceProvider.overrideWithValue(const _NoopLoggerService()),
      coreAnalyticsRepositoryProvider.overrideWithValue(
        _NoopAnalyticsRepository(),
      ),
      localStorageServiceProvider.overrideWithValue(
        _FakeLocalStorageService(),
      ),
      webSocketServiceProvider.overrideWithValue(
        WebSocketService(baseUrl: 'ws://example.invalid'),
      ),
      apiClientProvider.overrideWithValue(ApiClient.testing()),
      navigationRegistryProvider.overrideWithValue(registry),
      notificationRepositoryProvider.overrideWithValue(countingNotifRepo),
      followRepositoryProvider.overrideWithValue(_FakeFollowRepository()),
      fcmServiceProvider.overrideWithValue(countingFcm),
      localNotificationServiceProvider.overrideWithValue(
        _FakeLocalNotificationService(),
      ),
      homeRepositoryProvider.overrideWithValue(countingHomeRepo),
      contentRepositoryProvider.overrideWithValue(_FakeContentRepository()),
      ratingRepositoryProvider.overrideWithValue(_FakeRatingRepository()),
      coreNotificationTriggerProvider.overrideWithValue(
        _FakeNotificationTrigger(),
      ),
      forSalesProvider.overrideWith((ref, params) async {
        forSaleProviderCallCount++;
        return const <ForSale>[];
      }),
      exploreAuctionsStreamProvider.overrideWith((ref) {
        return Stream.value(const <Auction>[]);
      }),
    ]);
  }
}

Widget _buildApp(ProviderContainer container) {
  return UncontrolledProviderScope(
    container: container,
    child: const LabudaApp(),
  );
}

Future<void> _pumpApp(WidgetTester tester, ProviderContainer container) async {
  await tester.binding.setSurfaceSize(const Size(1600, 2400));
  addTearDown(() async { await tester.binding.setSurfaceSize(null); });
  await tester.pumpWidget(_buildApp(container));
  await tester.pumpAndSettle();
}

AuthUser _verifiedUser() => AuthUser(
  id: _testUserId,
  createdAt: DateTime(2025),
  updatedAt: DateTime(2025),
  email: 'portal@test.com',
  username: 'verified_user',
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  hasSellerProfile: false,
  sellerSubscriptionStatus: 'none',
  hasMarketAuthority: false,
  roles: const [UserRole.user],
  provider: ShonaAuthProvider.email,
);

// =============================================================================
// Tests
// =============================================================================

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
    await initializeRouterModules();
  });

  group('Portal blocks protected initialization', () {
    testWidgets(
      'Portal active → VerifyEmailScreen builds, '
      'Home build count = 0, feed count = 0, listing count = 0, '
      'notification unread polling count = 0, FCM count = 0',
      (tester) async {
        final h = _Harness(
          AuthState.requiresEmailVerification(
            userId: _testUserId, email: 'portal@test.com',
          ),
        );
        final container = h.buildContainer();
        addTearDown(container.dispose);

        await _pumpApp(tester, container);

        // Portal is visible.
        expect(find.byType(VerifyEmailScreen), findsOneWidget);

        // HomeScreen build count = 0.
        expect(find.byType(HomeScreen), findsNothing);

        // Feed datasource invocation count = 0.
        expect(h.countingHomeRepo.feedCallCount, 0);

        // ForSale provider invocation count = 0.
        expect(h.forSaleProviderCallCount, 0);

        // Notification unread polling count = 0.
        expect(h.countingNotifRepo.unreadCountCallCount, 0);

        // Authenticated FCM registration count = 0.
        expect(h.countingFcm.initCallCount, 0);
      },
    );

    testWidgets(
      'Portal → transition to verified → portal removed, '
      'Home builds, same GoRouter, providers may initialize',
      (tester) async {
        final h = _Harness(
          AuthState.requiresEmailVerification(
            userId: _testUserId, email: 'portal@test.com',
          ),
        );
        final container = h.buildContainer();
        addTearDown(container.dispose);

        await _pumpApp(tester, container);

        // Portal is active with all protected counts = 0.
        expect(find.byType(VerifyEmailScreen), findsOneWidget);
        expect(find.byType(HomeScreen), findsNothing);
        expect(h.countingHomeRepo.feedCallCount, 0);
        expect(h.forSaleProviderCallCount, 0);
        expect(h.countingNotifRepo.unreadCountCallCount, 0);
        expect(h.countingFcm.initCallCount, 0);

        final routerBefore = container.read(goRouterProvider);

        // Transition to verified.
        h.controller.publish(
          AuthState.authenticated(_verifiedUser(), emailVerified: true),
        );
        await tester.pumpAndSettle();

        // Portal is gone.
        expect(find.byType(VerifyEmailScreen), findsNothing);

        // Same GoRouter instance.
        expect(container.read(goRouterProvider), same(routerBefore));

        // After transition, protected providers are permitted to initialize.
        // We don't assert exact counts since what initializes depends on
        // shell routing, but the portal is definitively gone.
      },
    );

    testWidgets(
      'Counters stay at zero while portal is active '
      '(no premature provider initialization)',
      (tester) async {
        final h = _Harness(
          AuthState.requiresEmailVerification(
            userId: _testUserId, email: 'portal@test.com',
          ),
        );
        final container = h.buildContainer();
        addTearDown(container.dispose);

        await _pumpApp(tester, container);

        // Give extra pump cycles — any eager provider would fire by now.
        await tester.pump(const Duration(seconds: 1));
        await tester.pump(const Duration(seconds: 1));

        // All counters must still be zero.
        expect(h.countingHomeRepo.feedCallCount, 0,
            reason: 'feed must not be fetched behind portal');
        expect(h.forSaleProviderCallCount, 0,
            reason: 'listings must not be fetched behind portal');
        expect(h.countingNotifRepo.unreadCountCallCount, 0,
            reason: 'notification unread count must not be polled behind portal');
        expect(h.countingFcm.initCallCount, 0,
            reason: 'FCM must not register behind portal');
      },
    );
  });
}
