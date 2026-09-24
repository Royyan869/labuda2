/// MyForSalesScreen `_changeStatus()` — canonical restriction dispatch.
///
/// Proves that the status-change error path routes the restriction family
/// through `CommerceRestrictionPresenter.handle(...)`:
///
/// - `MARKET_AUTHORITY_REQUIRED` → canonical seller renewal navigation
///   (via NavigationHandler, NOT a message-string heuristic).
/// - `COMMERCE_RESTRICTED` → existing restriction snackbar behavior.
/// - `SHIPPING_NOT_CONFIGURED` → existing shipping-setup dialog behavior.
/// - any other code (including generic `FORBIDDEN` whose message happens to
///   mention a subscription) → existing generic error snackbar.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_error_codes.dart' as codes;
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/screens/my_for_sales_screen.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

// =============================================================================
// Test doubles
// =============================================================================

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _RecordingNavigationHandler implements NavigationHandler {
  int renewalCalls = 0;
  int upgradeCalls = 0;

  @override
  void navigateToSellerRenewal() => renewalCalls++;

  @override
  void navigateToSellerUpgrade() => upgradeCalls++;

  @override
  void showSnackBar(String message, {bool isError = false}) {}

  @override
  void noSuchMethod(Invocation invocation) {}
}

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
  Future<Result<void>> setLogLevel(LogLevel level) async => Result.success(null);

  @override
  Future<Result<void>> warning(
    String message, {
    Map<String, dynamic>? extra,
  }) async => Result.success(null);
}

class _FakeForSaleRepository implements ForSaleRepository {
  Result<ForSale>? updateStatusResult;
  final List<(String, ForSaleStatus)> updateStatusCalls = [];

  @override
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ForSale?>> getForSaleById(String forSaleId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<ForSale>>> getForSalesByIds(List<String> forSaleIds) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
  }) async => Result.success([_listing()]);

  @override
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request) async {
    throw UnimplementedError();
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
    updateStatusCalls.add((forSaleId, status));
    final result = updateStatusResult;
    if (result == null) {
      throw UnimplementedError('updateStatusResult not configured');
    }
    return result;
  }
}

// =============================================================================
// Fixtures + harness
// =============================================================================

class _Harness {
  const _Harness(this.navigation, this.repository);

  final _RecordingNavigationHandler navigation;
  final _FakeForSaleRepository repository;
}

AuthUser _seller() {
  return AuthUser(
    id: 'seller-1',
    createdAt: DateTime.utc(2026, 7, 1),
    updatedAt: DateTime.utc(2026, 7, 1),
    email: 'seller-1@example.com',
    username: 'seller-1',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    hasSellerProfile: true,
    sellerSubscriptionStatus: 'active',
    hasMarketAuthority: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    lifecycle: ContentLifecycle.active,
  );
}

ForSale _listing() {
  final now = DateTime.utc(2026, 7, 1, 7);
  return ForSale(
    forSaleId: 'forSale-1',
    productId: 'product-1',
    title: 'Kohaku 50cm',
    description: 'desc',
    price: 1500000,
    stock: 1,
    sellerId: 'seller-1',
    status: ForSaleStatus.active,
    createdAt: now,
    updatedAt: now,
  );
}

Future<_Harness> _pumpScreen(
  WidgetTester tester, {
  required Result<ForSale> statusResult,
}) async {
  final navigation = _RecordingNavigationHandler();
  final repository = _FakeForSaleRepository()
    ..updateStatusResult = statusResult;

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(
            AuthState.authenticated(_seller(), emailVerified: true),
          ),
        ),
        forSaleRepositoryProvider.overrideWithValue(repository),
        loggerServiceProvider.overrideWithValue(const _NoopLogger()),
        navigationHandlerProvider.overrideWithValue(navigation),
      ],
      child: const MaterialApp(home: MyForSalesScreen()),
    ),
  );

  await tester.pump();
  await tester.pumpAndSettle();

  // The seller's active forSale must be on screen for the action menu to exist.
  expect(find.text('Kohaku 50cm'), findsOneWidget);

  return _Harness(navigation, repository);
}

/// Drives the real UI path: action menu → "Nonaktifkan" → confirm dialog.
Future<void> _submitStatusChange(WidgetTester tester) async {
  await tester.tap(find.byType(PopupMenuButton<String>));
  await tester.pumpAndSettle();

  await tester.tap(find.text('Nonaktifkan'));
  await tester.pumpAndSettle();

  expect(find.text('Ubah Status For Sale'), findsOneWidget);
  await tester.tap(find.text('Ya, Ubah'));
  await tester.pumpAndSettle();
}

void main() {
  group('MyForSalesScreen._changeStatus — canonical restriction dispatch', () {
    testWidgets(
      'MARKET_AUTHORITY_REQUIRED → canonical seller renewal, no generic error',
      (tester) async {
        final harness = await _pumpScreen(
          tester,
          statusResult: Result.error(
            'Active seller subscription required to publish for_sales',
            code: codes.marketAuthorityRequired,
            statusCode: 403,
          ),
        );

        await _submitStatusChange(tester);

        expect(harness.repository.updateStatusCalls, [
          ('forSale-1', ForSaleStatus.withdrawn),
        ]);
        expect(harness.navigation.renewalCalls, 1);
        expect(harness.navigation.upgradeCalls, 0);
        // No generic error snackbar, no restriction snackbar, no shipping dialog.
        expect(find.textContaining('Gagal mengubah status'), findsNothing);
        expect(find.textContaining('dibatasi'), findsNothing);
        expect(find.text('Pengiriman Belum Dipilih'), findsNothing);
      },
    );

    testWidgets(
      'COMMERCE_RESTRICTED → existing restriction snackbar preserved',
      (tester) async {
        final harness = await _pumpScreen(
          tester,
          statusResult: Result.error(
            'Commerce activity restricted',
            code: codes.commerceRestricted,
            statusCode: 403,
          ),
        );

        await _submitStatusChange(tester);

        expect(find.textContaining('dibatasi'), findsOneWidget);
        expect(find.textContaining('mengubah status For Sale'), findsOneWidget);
        expect(find.textContaining('Hubungi dukungan'), findsOneWidget);
        expect(harness.navigation.renewalCalls, 0);
        expect(harness.navigation.upgradeCalls, 0);
        expect(find.textContaining('Gagal mengubah status'), findsNothing);
      },
    );

    testWidgets(
      'SHIPPING_NOT_CONFIGURED → existing shipping setup dialog preserved',
      (tester) async {
        final harness = await _pumpScreen(
          tester,
          statusResult: Result.error(
            'Shipping not configured',
            code: 'SHIPPING_NOT_CONFIGURED',
            statusCode: 422,
          ),
        );

        await _submitStatusChange(tester);

        expect(find.text('Pengiriman Belum Dipilih'), findsOneWidget);
        expect(find.text('Tutup'), findsOneWidget);
        expect(find.text('Atur Opsi'), findsOneWidget);
        expect(find.text('Edit For Sale'), findsOneWidget);
        expect(harness.navigation.renewalCalls, 0);
        expect(find.textContaining('dibatasi'), findsNothing);
        expect(find.textContaining('Gagal mengubah status'), findsNothing);
      },
    );

    testWidgets(
      'generic error → generic snackbar preserved (code decides, not message)',
      (tester) async {
        final harness = await _pumpScreen(
          tester,
          statusResult: Result.error(
            // Message mentions the market-authority wording on purpose: the
            // dispatch MUST stay code-based and never match on message text.
            'Active seller subscription required to publish for_sales',
            code: 'FORBIDDEN',
            statusCode: 403,
          ),
        );

        await _submitStatusChange(tester);

        expect(harness.navigation.renewalCalls, 0);
        expect(harness.navigation.upgradeCalls, 0);
        expect(find.textContaining('Gagal mengubah status'), findsOneWidget);
        expect(find.textContaining('dibatasi'), findsNothing);
        expect(find.text('Pengiriman Belum Dipilih'), findsNothing);
      },
    );
  });
}
