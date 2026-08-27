import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_controller.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _NoopLogger implements ILoggerService {
  const _NoopLogger();

  @override
  Future<Result<void>> clearLogs() async => Result.success(null);

  @override
  Future<Result<void>> debug(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

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
  }) async => Result.success(null);

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
  }) async => Result.success(null);

  @override
  Future<Result<void>> info(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);

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
  }) async => Result.success(null);

  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) async => Result.success(null);

  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) async => Result.success(null);

  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) async => Result.success(null);

  @override
  Future<Result<void>> setLogLevel(LogLevel level) async =>
      Result.success(null);

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);
}

class _FakeForSaleRepository implements ForSaleRepository {
  int createCalls = 0;
  CreateForSaleRequest? lastRequest;

  @override
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ForSale?>> getForSaleById(String forSaleId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<ForSale>>> getForSalesByIds(
    List<String> forSaleIds,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request) async {
    createCalls++;
    lastRequest = request;
    return Result.success(
      ForSale(
        forSaleId: 'listing-1',
        productId: 'product-1',
        title: request.title,
        description: request.description,
        price: request.price,
        stock: request.quantity,
        sellerId: 'seller-1',
        status: ForSaleStatus.draft,
        visibility: ForSaleVisibility.private,
        isNegotiable: request.negotiationEnabled,
        createdAt: DateTime.utc(2026, 1, 1),
        updatedAt: DateTime.utc(2026, 1, 1),
        variety: request.variety,
        sizeCm: request.sizeCm,
        ageMonths: request.ageMonths,
        gender: request.gender,
        breeder: request.breeder,
        bloodline: request.bloodline,
      ),
    );
  }

  @override
  Future<Result<ForSale>> updateForSale(
    String forSaleId,
    UpdateForSaleRequest request,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> deleteForSale(String forSaleId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ForSale>> updateForSaleStatus(
    String forSaleId,
    ForSaleStatus status,
  ) async {
    throw UnimplementedError();
  }
}

AuthUser _seller({
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
  bool isEmailVerified = true,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: 'seller-1',
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller',
    isEmailVerified: isEmailVerified,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

CreateForSaleRequest _request() => const CreateForSaleRequest(
  title: 'Kohaku 50cm',
  description: 'Healthy koi',
  price: 500000,
  quantity: 2,
  mediaUrls: ['https://example.com/1.jpg'],
  variety: 'Kohaku',
  sizeCm: 50,
);

void main() {
  group('ForSaleController create-listing authority boundary', () {
    test('blocked states never call the repository', () async {
      final repo = _FakeForSaleRepository();
      final controller = ForSaleController(
        repository: repo,
        logger: const _NoopLogger(),
      );

      final blockedStates = [
        const AuthState.loading(),
        AuthState.authenticated(
          _seller(hasSellerProfile: false, hasMarketAuthority: false),
          emailVerified: true,
        ),
        AuthState.authenticated(
          _seller(hasSellerProfile: true, hasMarketAuthority: false),
          emailVerified: true,
        ),
      ];

      for (final state in blockedStates) {
        final result = await controller.createForSaleIfAuthorized(
          _request(),
          state,
        );

        expect(result.isError, isTrue);
        expect(repo.createCalls, 0);
      }
    });

    test(
      'active seller can reach submission and call the repository',
      () async {
        final repo = _FakeForSaleRepository();
        final controller = ForSaleController(
          repository: repo,
          logger: const _NoopLogger(),
        );

        final result = await controller.createForSaleIfAuthorized(
          _request(),
          AuthState.authenticated(
            _seller(hasSellerProfile: true, hasMarketAuthority: true),
            emailVerified: true,
          ),
        );

        expect(result.isSuccess, isTrue);
        expect(repo.createCalls, 1);
        expect(repo.lastRequest?.title, 'Kohaku 50cm');
      },
    );

    test('current principal switch flips authorization immediately', () {
      final controller = ForSaleController(
        repository: _FakeForSaleRepository(),
        logger: const _NoopLogger(),
      );

      final active = AuthState.authenticated(
        _seller(hasSellerProfile: true, hasMarketAuthority: true),
        emailVerified: true,
      );
      final expired = AuthState.authenticated(
        _seller(hasSellerProfile: true, hasMarketAuthority: false),
        emailVerified: true,
      );

      expect(controller.canCreateForSale(active), isTrue);
      expect(controller.canCreateForSale(expired), isFalse);
    });
  });
}
