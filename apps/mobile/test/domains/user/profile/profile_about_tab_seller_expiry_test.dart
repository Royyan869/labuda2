// RF-02 follow-up — profile About tab expiry consumer contract.
//
// The About tab's own-profile seller status used to derive EXPIRY from the
// CAPABILITY axis (`capability != active → SellerState.expired()`), which told
// fresh sellers ('none') their subscription had ended. The fix delegates to the
// canonical seller-domain authority (`SellerState.fromAuthUser`) over the
// hydrated account snapshot, so ONLY `sellerSubscriptionStatus == 'expired'`
// may render the expired badge / renewal banner.
//
// These tests lock the three canonical subscription states at the UI seam:
//   'none'    → pending-activation badge, NO expiry copy
//   'expired' → expired badge + renewal banner
//   'active'  → active badge, NO expiry copy
//
// Harness follows the repo's lightweight profile widget-test pattern
// (profile_header_identity_polish_test.dart): fake ApiClient/Logger +
// overridden authControllerProvider, so the real profileAboutDataProvider
// pipeline (userDataProvider → profileStreamProvider → ProfileAboutTab) runs
// against fakes only.
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/address_list_provider.dart'
    show addressesStreamProvider;
import 'package:labuda/domains/user/profile/presentation/providers/profile_stream_provider.dart'
    show profileStreamProvider;
import 'package:labuda/domains/user/profile/presentation/providers/user_data_provider.dart'
    show userDataProvider;
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen/profile_about_tab.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeApiClient implements ApiClient {
  const _FakeApiClient();

  @override
  Dio get dio => throw UnimplementedError('Not used in about-tab tests');

  @override
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError('Not used in about-tab tests');

  @override
  ApiException extractException(DioException e) =>
      UnknownApiException(message: e.message ?? 'unknown', details: e.error);

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError('Not used in about-tab tests');

  @override
  bool isNotFound(DioException e) => false;

  @override
  bool isUnauthorized(DioException e) => false;

  @override
  bool isValidationError(DioException e) => false;

  @override
  Future<Response<T>> patch<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError('Not used in about-tab tests');

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError('Not used in about-tab tests');

  @override
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError('Not used in about-tab tests');

  @override
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int, int)? onSendProgress,
  }) async => throw UnimplementedError('Not used in about-tab tests');
}

class _FakeLoggerService implements ILoggerService {
  const _FakeLoggerService();

  Result<void> _okVoid() => Result.success(null);

  @override
  Future<Result<void>> clearLogs() async => _okVoid();

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> debugCallingGetCurrentUser() async => _okVoid();

  @override
  Future<Result<void>> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async => _okVoid();

  @override
  Future<Result<void>> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async => _okVoid();

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
  }) async => _okVoid();

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
  }) async => _okVoid();

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();

  @override
  Future<Result<void>> log(
    String message, {
    LogLevel level = LogLevel.debug,
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
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) async => _okVoid();

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async => _okVoid();

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => _okVoid();
}

/// Auth controller stub that hydrates the authenticated account with a seller
/// whose subscription status is the parameter under test.
class _HarnessAuthController extends AuthController {
  _HarnessAuthController(this._user);

  final AuthUser _user;

  @override
  bool get shouldInitializeAuthListener => false;

  @override
  AuthState build() {
    return AuthState.authenticated(_user, emailVerified: true);
  }
}

AuthUser _sellerUser({required String sellerSubscriptionStatus}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: 'seller-1',
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'sellerone',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    hasSellerProfile: true,
    sellerSubscriptionStatus: sellerSubscriptionStatus,
    // Capability is FALSE in every scenario: proving the About tab no longer
    // derives expiry from capability. Only the subscription axis decides.
    hasMarketAuthority: false,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

Widget _wrap(AuthUser user) {
  return ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(const _FakeApiClient()),
      loggerServiceProvider.overrideWithValue(const _FakeLoggerService()),
      authControllerProvider.overrideWith(() => _HarnessAuthController(user)),
      // About-tab data pipeline: user comes from the snapshot under test,
      // extended profile / addresses are empty (non-expiry behavior only).
      userDataProvider.overrideWith((ref, userId) async => user),
      profileStreamProvider.overrideWith(
        (ref, userId) => Stream<ProfileEntity?>.value(null),
      ),
      addressesStreamProvider.overrideWith(
        (ref, userId) =>
            Stream<Result<List<AddressEntity>>>.value(Result.success(const [])),
      ),
    ],
    child: const MaterialApp(
      home: Scaffold(body: ProfileAboutTab(userId: 'seller-1')),
    ),
  );
}

void main() {
  testWidgets('subscription none: seller is NOT shown as expired', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(_sellerUser(sellerSubscriptionStatus: 'none')));
    await tester.pumpAndSettle();

    // Pending-activation badge renders; no expired state anywhere.
    expect(find.text('Belum Aktif'), findsOneWidget);
    expect(find.text('Berakhir'), findsNothing);
    expect(find.text('Langganan Anda telah berakhir'), findsNothing);
    expect(find.text('Perpanjang Langganan'), findsNothing);
  });

  testWidgets('subscription expired: seller IS shown as expired', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_sellerUser(sellerSubscriptionStatus: 'expired')),
    );
    await tester.pumpAndSettle();

    expect(find.text('Berakhir'), findsOneWidget);
    expect(find.text('Langganan Anda telah berakhir'), findsOneWidget);
  });

  testWidgets('subscription active: seller is NOT shown as expired', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_sellerUser(sellerSubscriptionStatus: 'active')),
    );
    await tester.pumpAndSettle();

    expect(find.text('Aktif'), findsOneWidget);
    expect(find.text('Berakhir'), findsNothing);
    expect(find.text('Langganan Anda telah berakhir'), findsNothing);
    expect(find.text('Perpanjang Langganan'), findsNothing);
  });
}
