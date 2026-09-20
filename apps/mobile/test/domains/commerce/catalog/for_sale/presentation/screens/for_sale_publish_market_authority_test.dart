import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_error_codes.dart' as codes;
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_controller.dart';

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

class _FakeForSaleRepository implements ForSaleRepository {
  Result<ForSale>? updateStatusResult;

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
  }) async {
    throw UnimplementedError();
  }

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
    final result = updateStatusResult;
    if (result == null) {
      throw UnimplementedError('updateStatusResult not configured');
    }
    return result;
  }
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

void main() {
  group('create_for_sale_screen error branch — canonical presenter consumption', () {
    test('publish (draft → active) rejection surfaces MARKET_AUTHORITY_REQUIRED code', () async {
      final repo = _FakeForSaleRepository()
        ..updateStatusResult = Result.error(
          'Active seller subscription required to publish for_sales',
          code: codes.marketAuthorityRequired,
          statusCode: 403,
        );
      final controller = ForSaleController(
        repository: repo,
        logger: const _NoopLogger(),
      );

      final result = await controller.updateForSaleStatus(
        'forSale-1',
        ForSaleStatus.active,
      );

      expect(result.isError, isTrue);
      expect(result.errorCode, codes.marketAuthorityRequired);
      expect(result.statusCode, 403);
    });

    testWidgets(
      'MARKET_AUTHORITY_REQUIRED → CommerceRestrictionPresenter.handle → seller renewal',
      (tester) async {
        final navigation = _RecordingNavigationHandler();

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              navigationHandlerProvider.overrideWithValue(navigation),
            ],
            child: const MaterialApp(
              home: Scaffold(body: SizedBox.shrink()),
            ),
          ),
        );
        await tester.pump();

        final consumed = CommerceRestrictionPresenter.handle(
          tester.element(find.byType(Scaffold)),
          errorCode: codes.marketAuthorityRequired,
          actionDescription: 'membuat forSale',
        );
        await tester.pump();

        expect(consumed, isTrue);
        expect(navigation.renewalCalls, 1);
        expect(navigation.upgradeCalls, 0);
        // Renewal navigation — NOT the restriction snackbar.
        expect(find.textContaining('dibatasi'), findsNothing);
      },
    );

    testWidgets(
      'COMMERCE_RESTRICTED → existing restriction snackbar behavior preserved',
      (tester) async {
        final navigation = _RecordingNavigationHandler();

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              navigationHandlerProvider.overrideWithValue(navigation),
            ],
            child: const MaterialApp(
              home: Scaffold(body: SizedBox.shrink()),
            ),
          ),
        );
        await tester.pump();

        final consumed = CommerceRestrictionPresenter.handle(
          tester.element(find.byType(Scaffold)),
          errorCode: codes.commerceRestricted,
          actionDescription: 'membuat forSale',
        );
        await tester.pump();

        expect(consumed, isTrue);
        expect(find.textContaining('dibatasi'), findsOneWidget);
        expect(find.textContaining('Hubungi dukungan'), findsOneWidget);
        expect(navigation.renewalCalls, 0);
        expect(navigation.upgradeCalls, 0);
      },
    );

    testWidgets(
      'generic FORBIDDEN → not consumed, caller keeps its generic error path',
      (tester) async {
        final navigation = _RecordingNavigationHandler();

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              navigationHandlerProvider.overrideWithValue(navigation),
            ],
            child: const MaterialApp(
              home: Scaffold(body: SizedBox.shrink()),
            ),
          ),
        );
        await tester.pump();

        final consumed = CommerceRestrictionPresenter.handle(
          tester.element(find.byType(Scaffold)),
          errorCode: 'FORBIDDEN',
          actionDescription: 'membuat forSale',
        );
        await tester.pump();

        expect(consumed, isFalse);
        expect(navigation.renewalCalls, 0);
        expect(navigation.upgradeCalls, 0);
        expect(find.textContaining('dibatasi'), findsNothing);
      },
    );
  });
}
