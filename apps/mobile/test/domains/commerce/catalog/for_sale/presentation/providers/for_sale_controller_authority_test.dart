import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_controller.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
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
  final List<bool> sellerForSalesIncludeWithdrawn = <bool>[];

  @override
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ForSale?>> getForSaleById(String forSaleId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
    bool includeWithdrawn = false,
  }) async {
    sellerForSalesIncludeWithdrawn.add(includeWithdrawn);
    return Result.success(const <ForSale>[]);
  }

  @override
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request) async {
    createCalls++;
    lastRequest = request;
    return Result.success(
      ForSale(
        forSaleId: 'forSale-1',
        productId: 'product-1',
        title: request.title,
        description: request.description,
        price: request.price,
        stock: request.quantity,
        sellerId: 'seller-1',
        status: ForSaleStatus.active,
        visibility: ForSaleVisibility.public,
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

}

/// The three seller axes are passed independently on purpose: workspace
/// (profile), capability (market authority) and expiry (subscription status).
AuthUser _seller({
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
  required String sellerSubscriptionStatus,
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
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: sellerSubscriptionStatus,
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

/// Mirrors the payload the create screen builds: create = publish — the
/// request carries its shipping selection; there is no visibility field.
CreateForSaleRequest _request() => const CreateForSaleRequest(
  title: 'Kohaku 50cm',
  description: 'Healthy koi',
  price: 500000,
  quantity: 2,
  mediaUrls: ['https://example.com/1.jpg'],
  variety: 'Kohaku',
  sizeCm: 50,
  shippingSetupIds: ['11111111-1111-1111-1111-111111111111'],
);

void main() {
  group('ForSaleController create-as-publish authority boundary', () {
    test(
      'seller with a profile but NO market authority is BLOCKED at create',
      () async {
        final repo = _FakeForSaleRepository();
        final controller = ForSaleController(
          repository: repo,
          logger: const _NoopLogger(),
        );

        final result = await controller.createForSaleIfAuthorized(
          _request(),
          AuthState.authenticated(
            _seller(
              hasSellerProfile: true,
              hasMarketAuthority: false,
              sellerSubscriptionStatus: 'none',
            ),
            emailVerified: true,
          ),
        );

        // CREATE = PUBLISH: creating without an active subscription is
        // rejected locally with the canonical code.
        expect(result.isError, isTrue);
        expect(result.errorCode, 'MARKET_AUTHORITY_REQUIRED');
        expect(repo.createCalls, 0);
      },
    );

    test(
      'expired-subscription seller is also blocked at create',
      () async {
        final repo = _FakeForSaleRepository();
        final controller = ForSaleController(
          repository: repo,
          logger: const _NoopLogger(),
        );

        final result = await controller.createForSaleIfAuthorized(
          _request(),
          AuthState.authenticated(
            _seller(
              hasSellerProfile: true,
              hasMarketAuthority: false,
              sellerSubscriptionStatus: 'expired',
            ),
            emailVerified: true,
          ),
        );

        expect(result.isError, isTrue);
        expect(result.errorCode, 'MARKET_AUTHORITY_REQUIRED');
        expect(repo.createCalls, 0);
      },
    );

    test('seller without a profile is still blocked (workspace gate)', () async {
      final repo = _FakeForSaleRepository();
      final controller = ForSaleController(
        repository: repo,
        logger: const _NoopLogger(),
      );

      final result = await controller.createForSaleIfAuthorized(
        _request(),
        AuthState.authenticated(
          _seller(
            hasSellerProfile: false,
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'none',
          ),
          emailVerified: true,
        ),
      );

      expect(result.isError, isTrue);
      expect(result.errorCode, 'SELLER_PROFILE_REQUIRED');
      expect(repo.createCalls, 0);
    });

    test('unhydrated session is still blocked (workspace gate)', () async {
      final repo = _FakeForSaleRepository();
      final controller = ForSaleController(
        repository: repo,
        logger: const _NoopLogger(),
      );

      final result = await controller.createForSaleIfAuthorized(
        _request(),
        const AuthState.loading(),
      );

      expect(result.isError, isTrue);
      expect(result.errorCode, 'AUTH_NOT_READY');
      expect(repo.createCalls, 0);
    });

    test('canCreateForSale requires profile AND capability', () {
      final controller = ForSaleController(
        repository: _FakeForSaleRepository(),
        logger: const _NoopLogger(),
      );

      final sellerWithoutCapability = AuthState.authenticated(
        _seller(
          hasSellerProfile: true,
          hasMarketAuthority: false,
          sellerSubscriptionStatus: 'none',
        ),
        emailVerified: true,
      );
      final nonSeller = AuthState.authenticated(
        _seller(
          hasSellerProfile: false,
          hasMarketAuthority: false,
          sellerSubscriptionStatus: 'none',
        ),
        emailVerified: true,
      );
      final activeSeller = AuthState.authenticated(
        _seller(
          hasSellerProfile: true,
          hasMarketAuthority: true,
          sellerSubscriptionStatus: 'active',
        ),
        emailVerified: true,
      );

      expect(controller.canCreateForSale(sellerWithoutCapability), isFalse);
      expect(controller.canCreateForSale(nonSeller), isFalse);
      expect(controller.canCreateForSale(activeSeller), isTrue);
      expect(controller.canCreateForSale(const AuthState.loading()), isFalse);
    });

  });

  group('getSellerForSales — owner inventory opt-in', () {
    test('forwards includeWithdrawn=true for the owner inventory surface',
        () async {
      final repo = _FakeForSaleRepository();
      final controller = ForSaleController(
        repository: repo,
        logger: const _NoopLogger(),
      );

      await controller.getSellerForSales('seller-1', includeWithdrawn: true);

      expect(repo.sellerForSalesIncludeWithdrawn, [true]);
    });

    test('defaults includeWithdrawn=false for public/contextual surfaces',
        () async {
      final repo = _FakeForSaleRepository();
      final controller = ForSaleController(
        repository: repo,
        logger: const _NoopLogger(),
      );

      await controller.getSellerForSales('seller-1');

      expect(repo.sellerForSalesIncludeWithdrawn, [false]);
    });

    test('SellerForSalesParams distinguishes includeWithdrawn in provider key',
        () {
      const base = SellerForSalesParams(sellerId: 'seller-1');
      const withWithdrawn = SellerForSalesParams(
        sellerId: 'seller-1',
        includeWithdrawn: true,
      );

      expect(base.includeWithdrawn, isFalse);
      expect(withWithdrawn.includeWithdrawn, isTrue);
      expect(withWithdrawn, isNot(equals(base)));
    });

    test('datasource and My For Sale screen lock the canonical opt-in', () {
      final datasource = File(
        'lib/domains/commerce/catalog/for_sale/data/remote/for_sale_remote_datasource.dart',
      ).readAsStringSync();
      expect(datasource.contains("'include_withdrawn': true"), isTrue);

      final screen = File(
        'lib/domains/commerce/catalog/for_sale/presentation/screens/my_for_sales_screen.dart',
      ).readAsStringSync();
      expect(screen.contains('includeWithdrawn: true'), isTrue);
    });
  });
}
