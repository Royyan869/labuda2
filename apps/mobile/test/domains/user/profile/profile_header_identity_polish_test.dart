import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen/profile_header_builder.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';

class _FakeApiClient implements ApiClient {
  const _FakeApiClient();

  @override
  Dio get dio => throw UnimplementedError('Not used in profile header tests');

  @override
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError('Not used in profile header tests');

  @override
  ApiException extractException(DioException e) =>
      UnknownApiException(message: e.message ?? 'unknown', details: e.error);

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError('Not used in profile header tests');

  @override
  bool isNetworkError(DioException e) => false;

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
  }) async => throw UnimplementedError('Not used in profile header tests');

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError('Not used in profile header tests');

  @override
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async => throw UnimplementedError('Not used in profile header tests');

  @override
  Future<Response<T>> uploadFile<T>(
    String path, {
    required String filePath,
    required String fieldName,
    Map<String, dynamic>? additionalFields,
    Options? options,
    CancelToken? cancelToken,
    void Function(int, int)? onSendProgress,
  }) async => throw UnimplementedError('Not used in profile header tests');
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

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.parse('2026-06-03T00:00:00.000Z');
    final user = AuthUser(
      id: 'user-1',
      createdAt: now,
      updatedAt: now,
      email: 'yayan@example.com',
      username: 'yayan',
      avatarUrl: 'https://example.com/avatar.png',
      bio: null,
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      sellerSubscriptionStatus: 'none',
      hasMarketAuthority: false,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );

    return AuthState.authenticated(user, emailVerified: true);
  }
}

Widget _wrap(Map<String, dynamic> profileData) {
  return ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(const _FakeApiClient()),
      loggerServiceProvider.overrideWithValue(const _FakeLoggerService()),
      authControllerProvider.overrideWith(_FakeAuthController.new),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: ProfileExpandedHeaderBuilder(
          userId: 'user-1',
          isDark: false,
          isSeller: profileData['farmName'] != null,
          isOwnProfile: false,
          profileData: profileData,
          headerExpandedHeight: 360,
          collapseProgress: 0,
        ),
      ),
    ),
  );
}

Map<String, dynamic> _baseProfile({
  required String name,
  required String username,
  String? farmName,
  String? sellerTier,
  ContentLifecycle lifecycle = ContentLifecycle.active,
}) {
  return {
    'name': name,
    'username': username,
    'farmName': farmName,
    'avatar': null,
    'bio': null,
    'location': null,
    'farmPhotoUrl': null,
    'coverPhotoUrl': null,
    'isVerified': false,
    'lifecycle': lifecycle,
    'sellerTier': sellerTier,
  };
}

void main() {
  testWidgets('Non seller header renders @username only', (tester) async {
    await tester.pumpWidget(
      _wrap(_baseProfile(name: '@yayan', username: '', farmName: null)),
    );

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Seller header renders @username then farmName', (tester) async {
    await tester.pumpWidget(
      _wrap(
        _baseProfile(
          name: '@yayan',
          username: '',
          farmName: 'Farm Koi Nusantara',
          sellerTier: 'pro',
        ),
      ),
    );

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsOneWidget);
    expect(find.byType(SellerTierBadge), findsOneWidget);
  });

  testWidgets('Missing farm renders @username only', (tester) async {
    await tester.pumpWidget(
      _wrap(_baseProfile(name: '@yayan', username: '', farmName: null)),
    );

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Lifecycle degraded renders redaction label only', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        _baseProfile(
          name: 'Pengguna tidak tersedia',
          username: '',
          farmName: null,
          lifecycle: ContentLifecycle.unavailable,
        ),
      ),
    );

    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Lifecycle deleted preserves tombstone redaction', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        _baseProfile(
          name: 'Pengguna dihapus',
          username: '',
          farmName: null,
          lifecycle: ContentLifecycle.removed,
        ),
      ),
    );

    expect(find.text('Pengguna dihapus'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });
}
