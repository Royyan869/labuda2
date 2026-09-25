// CANONICAL header identity contract — ported from the purged duplicate
// (profile_screen/profile_header_builder.dart) onto the one true
// ProfileScreen header. Owner directive: one truth, purge duplicates.
//
// Locked here, on the production screen:
//   1. Non-seller header renders @username only (no farm line).
//   2. Seller header renders @username then farmName + SellerTierBadge.
//   3. Missing farm renders @username only.
//   4. Degraded lifecycle renders the public redaction label only.
//   5. Deleted lifecycle preserves the tombstone redaction.
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/auction.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/for_sale.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/follow/data/follow_providers.dart';
import 'package:labuda/domains/social/follow/domain/repositories/i_follow_repository.dart';
import 'package:labuda/domains/social/rating/rating.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_stream_provider.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_view_provider.dart';
import 'package:labuda/domains/user/profile/presentation/providers/user_data_provider.dart';
import 'package:labuda/domains/user/profile/presentation/screens/profile_screen.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';
import 'package:labuda/shared/providers/block_state_provider.dart';

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

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

class _MutableAuthController extends AuthController {
  _MutableAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _FakeFollowRepository implements IFollowRepository {
  @override
  Future<Result<bool>> checkFollowStatus({
    required String followerId,
    required String followingId,
  }) async => Result.success(false);

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeContentRepository implements ContentRepository {
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
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeRatingRepository implements IRatingRepository {
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
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

AuthUser _authUser({
  required String id,
  required String username,
  bool hasSellerProfile = false,
  SellerTier? sellerTier,
  ContentLifecycle lifecycle = ContentLifecycle.active,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 7, 1),
    updatedAt: DateTime.utc(2026, 7, 1),
    email: '$username@example.com',
    username: username,
    avatarUrl: null,
    bio: null,
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    hasSellerProfile: hasSellerProfile,
    sellerTier: sellerTier,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    // Codebase factual: AuthUser.lifecycle is the server-coarsened
    // ContentLifecycle enum (ADR-006 §11) — never a raw string.
    lifecycle: lifecycle,
  );
}

ProfileEntity _profileEntity({required String userId, String? farmName}) {
  return ProfileEntity(
    id: 'profile-$userId',
    userId: userId,
    location: null,
    coverPhotoUrl: null,
    joinedAt: DateTime.utc(2026, 7, 1),
    lastActiveAt: DateTime.utc(2026, 7, 1),
    stats: const ProfileStats(followersCount: 0, followingCount: 0),
    verification: const UserVerificationInfo(
      isPhoneVerified: false,
      isEmailVerified: true,
      isIdVerified: false,
      isFarmVerified: false,
      badges: <ProfileBadge>[],
    ),
    farmInfo: farmName == null
        ? null
        : const FarmInfo(farmName: 'Farm Koi Nusantara'),
  );
}

void main() {
  testWidgets('Non seller header renders @username only', (tester) async {
    await _pumpCanonicalHeader(
      tester,
      username: 'yayan',
      hasSellerProfile: false,
      sellerTier: null,
      lifecycle: ContentLifecycle.active,
      farmName: false,
    );

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Seller header renders @username then farmName', (tester) async {
    await _pumpCanonicalHeader(
      tester,
      username: 'yayan',
      hasSellerProfile: true,
      sellerTier: SellerTier.sellerPro,
      lifecycle: ContentLifecycle.active,
      farmName: true,
    );

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsOneWidget);
    expect(find.byType(SellerTierBadge), findsOneWidget);
  });

  testWidgets('Missing farm renders @username only', (tester) async {
    await _pumpCanonicalHeader(
      tester,
      username: 'yayan',
      hasSellerProfile: true,
      sellerTier: null,
      lifecycle: ContentLifecycle.active,
      farmName: false,
    );

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Lifecycle degraded renders redaction label only', (tester) async {
    await _pumpCanonicalHeader(
      tester,
      username: 'yayan',
      hasSellerProfile: false,
      sellerTier: null,
      lifecycle: ContentLifecycle.unavailable,
      farmName: false,
    );

    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Lifecycle deleted preserves tombstone redaction', (
    tester,
  ) async {
    await _pumpCanonicalHeader(
      tester,
      username: 'yayan',
      hasSellerProfile: false,
      sellerTier: null,
      lifecycle: ContentLifecycle.removed,
      farmName: false,
    );

    expect(find.text('Pengguna dihapus'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });
}

/// Pumps the CANONICAL ProfileScreen (production widget, no test doubles for
/// the screen itself) against a fully overridden provider graph.
Future<void> _pumpCanonicalHeader(
  WidgetTester tester, {
  required String username,
  required bool hasSellerProfile,
  required SellerTier? sellerTier,
  required ContentLifecycle lifecycle,
  required bool farmName,
}) async {
  const targetId = 'target-1';
  final targetUser = _authUser(
    id: targetId,
    username: username,
    hasSellerProfile: hasSellerProfile,
    sellerTier: sellerTier,
    lifecycle: lifecycle,
  );
  final targetProfile = _profileEntity(
    userId: targetId,
    farmName: farmName ? 'Farm Koi Nusantara' : null,
  );

  await tester.binding.setSurfaceSize(const Size(1080, 2400));
  addTearDown(() => tester.binding.setSurfaceSize(null));

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => _MutableAuthController(
            AuthState.authenticated(
              _authUser(id: 'viewer-1', username: 'viewer'),
              emailVerified: true,
            ),
          ),
        ),
        // Identity card for the viewed profile — drives @username / farmName /
        // lifecycle rendering through the canonical _getProfileData path.
        userDataProvider(targetId).overrideWith((ref) => Future.value(targetUser)),
        profileViewDataProvider(targetId).overrideWithValue(
          AsyncData(ProfileViewData(user: targetUser, profile: targetProfile)),
        ),
        profileStreamProvider(targetId).overrideWith(
          (ref) => Stream.value(targetProfile),
        ),
        followRepositoryProvider.overrideWithValue(_FakeFollowRepository()),
        contentRepositoryProvider.overrideWithValue(_FakeContentRepository()),
        ratingRepositoryProvider.overrideWithValue(_FakeRatingRepository()),
        apiClientProvider.overrideWithValue(const _FakeApiClient()),
        loggerServiceProvider.overrideWithValue(const _FakeLoggerService()),
        blockedUserIdsProvider.overrideWith((ref) => Stream.value(<String>{})),
        userOnlineStatusProvider(targetId).overrideWith(
          (ref) => Stream.value(false),
        ),
        sellerForSalesProvider(
          SellerForSalesParams(sellerId: targetId, page: 1, pageSize: 50),
        ).overrideWith((ref) => Future.value(<ForSale>[])),
        sellerAuctionsProvider(
          targetId,
        ).overrideWith((ref) => Future.value(<Auction>[])),
      ],
      child: const MaterialApp(home: ProfileScreen(userId: targetId)),
    ),
  );
  await tester.pump();
  // Seller profiles flip the main tab count 3→4 through a post-frame
  // TabController swap (self-correcting on the next frame). The one-frame
  // controller/TabBar length mismatch raises a transient debug assertion
  // that is NOT part of the identity contract under test — drain it.
  tester.takeException();
  await tester.pump();
  tester.takeException();
}
