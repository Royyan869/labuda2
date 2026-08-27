import 'dart:async';

import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data;
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show userSyncServiceProvider;
import 'package:labuda/domains/user/profile/data/services/user_sync_service.dart';

class _FakeAuthRepository extends Fake implements IAuthRepository {
  final List<Completer<Result<AuthUser?>>> pendingRequests = [];

  @override
  Future<Result<AuthUser?>> getCurrentUser() {
    final completer = Completer<Result<AuthUser?>>();
    pendingRequests.add(completer);
    return completer.future;
  }
}

class _MockUserApiDatasource extends Fake implements UserApiDatasource {}

class _FakeUserSyncService extends UserSyncService {
  _FakeUserSyncService()
    : super(
        firebaseAuth: _FakeFirebaseAuth(),
        datasource: _MockUserApiDatasource(),
      );

  final List<Completer<Result<AuthUser>>> pendingRequests = [];

  @override
  Future<Result<AuthUser>> getCurrentUser() {
    final completer = Completer<Result<AuthUser>>();
    pendingRequests.add(completer);
    return completer.future;
  }
}

class _FakeFirebaseAuth extends Fake implements FirebaseAuth {
  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();

  @override
  User? get currentUser => null;
}

class _FakeLoggerService extends Fake implements ILoggerService {
  Result<void> _okVoid() => Result.success(null);

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _okVoid();

  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) async => _okVoid();

  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) async => _okVoid();

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) async => _okVoid();

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) async => _okVoid();

  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) async => _okVoid();

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async => _okVoid();

  @override
  Future<Result<void>> clearLogs() async => _okVoid();

  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) async => Result.success(const <LogEntry>[]);

  @override
  Future<void> debugSync(String userId) async {}

  @override
  Future<void> debugSyncSuccess(String userId) async {}

  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}

  @override
  Future<void> debugCallingGetCurrentUser() async {}

  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}

  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}

  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}

  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}

  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}
}

class _FakeAnalyticsRepository extends Fake implements IAnalyticsRepository {
  Result<void> _okVoid() => Result.success(null);

  @override
  Future<Result<void>> logEvent(
    String eventName, {
    Map<String, dynamic>? parameters,
    String? userId,
  }) async => _okVoid();

  @override
  Future<Result<void>> logUserAction(
    String action,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> logCircumventionAttempt(
    String content,
    String userId, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> setUserProperties(
    Map<String, dynamic> properties,
  ) async => _okVoid();

  @override
  Future<Result<void>> trackEngagement({
    required String userId,
    required String contentId,
    required String contentType,
    required String engagementType,
    int? duration,
  }) async => _okVoid();

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
  Future<Result<void>> flush() async => _okVoid();
}

class _FakeLocalStorageService extends Fake implements ILocalStorageService {}

class _TestFirebaseUser extends Fake implements User {
  _TestFirebaseUser({required this.uidValue, required this.emailVerifiedValue});

  final String uidValue;
  final bool emailVerifiedValue;
  int reloadCalls = 0;

  @override
  String get uid => uidValue;

  @override
  bool get emailVerified => emailVerifiedValue;

  @override
  Future<void> reload() async {
    reloadCalls++;
  }
}

class _TestAuthController extends AuthController {
  _TestAuthController({required this.firebaseUser});

  final User firebaseUser;
  int signOutCalls = 0;

  @override
  User? get activeFirebaseUser => firebaseUser;

  @override
  bool get shouldInitializeAuthListener => false;

  @override
  Future<void> performFirebaseSignOut() async {
    signOutCalls++;
  }
}

AuthUser _seller({
  required String username,
  required bool hasMarketAuthority,
  required bool hasSellerProfile,
}) {
  return AuthUser(
    id: 'seller-user-id',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'seller@test.com',
    username: username,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    accountStatus: AccountStatus.active,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
  );
}

ProviderContainer _buildContainer(
  _TestAuthController controller,
  _FakeAuthRepository repository,
  _FakeUserSyncService userSyncService,
) {
  return ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(() => controller),
      auth_data.authRepositoryProvider.overrideWithValue(repository),
      userSyncServiceProvider.overrideWithValue(userSyncService),
      loggerServiceProvider.overrideWithValue(_FakeLoggerService()),
      coreAnalyticsRepositoryProvider.overrideWithValue(
        _FakeAnalyticsRepository(),
      ),
      localStorageServiceProvider.overrideWithValue(_FakeLocalStorageService()),
    ],
  );
}

Future<void> _flushMicrotasks() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
}

void main() {
  group('Auth hydration ordering', () {
    test('newer seller snapshot wins over stale non-seller snapshot', () async {
      final repository = _FakeAuthRepository();
      final userSyncService = _FakeUserSyncService();
      final controller = _TestAuthController(
        firebaseUser: _TestFirebaseUser(
          uidValue: 'firebase-uid',
          emailVerifiedValue: true,
        ),
      );
      final container = _buildContainer(
        controller,
        repository,
        userSyncService,
      );
      addTearDown(container.dispose);

      container.read(authControllerProvider.notifier);
      controller.state = AuthState.authenticated(
        _seller(
          username: 'seller-initial',
          hasMarketAuthority: true,
          hasSellerProfile: true,
        ),
        emailVerified: true,
      );

      final request1 = controller.refreshUserData();
      final request2 = controller.refreshUserData();

      expect(userSyncService.pendingRequests, hasLength(2));

      userSyncService.pendingRequests[1].complete(
        Result.success(
          _seller(
            username: 'seller-fresh',
            hasMarketAuthority: true,
            hasSellerProfile: true,
          ),
        ),
      );
      await _flushMicrotasks();

      userSyncService.pendingRequests[0].complete(
        Result.success(
          _seller(
            username: 'stale-buyer',
            hasMarketAuthority: false,
            hasSellerProfile: false,
          ),
        ),
      );
      await Future.wait([request1, request2]);

      final state = controller.state;
      expect(state, isA<AuthStateAuthenticated>());
      expect((state as AuthStateAuthenticated).user.username, 'seller-fresh');
      expect(state.user.hasSellerProfile, isTrue);
      expect(state.user.hasMarketAuthority, isTrue);
      expect(controller.signOutCalls, 0);
    });

    test(
      'stale identity failure cannot sign out a newer valid seller session',
      () async {
        final repository = _FakeAuthRepository();
        final userSyncService = _FakeUserSyncService();
        final controller = _TestAuthController(
          firebaseUser: _TestFirebaseUser(
            uidValue: 'firebase-uid',
            emailVerifiedValue: true,
          ),
        );
        final container = _buildContainer(
          controller,
          repository,
          userSyncService,
        );
        addTearDown(container.dispose);

        container.read(authControllerProvider.notifier);
        controller.state = AuthState.authenticated(
          _seller(
            username: 'seller-initial',
            hasMarketAuthority: true,
            hasSellerProfile: true,
          ),
          emailVerified: true,
        );

        final request1 = controller.refreshUserData();
        final request2 = controller.refreshUserData();

        expect(userSyncService.pendingRequests, hasLength(2));

        userSyncService.pendingRequests[1].complete(
          Result.success(
            _seller(
              username: 'seller-fresh',
              hasMarketAuthority: true,
              hasSellerProfile: true,
            ),
          ),
        );
        await _flushMicrotasks();

        userSyncService.pendingRequests[0].complete(
          Result.error(
            'Invalid or expired Firebase token',
            code: 'INVALID_TOKEN',
            statusCode: 401,
          ),
        );
        await Future.wait([request1, request2]);

        final state = controller.state;
        expect(state, isA<AuthStateAuthenticated>());
        expect((state as AuthStateAuthenticated).user.username, 'seller-fresh');
        expect(controller.signOutCalls, 0);
      },
    );
  });
}
