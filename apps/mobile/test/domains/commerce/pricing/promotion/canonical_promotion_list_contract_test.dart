// ============================================================================
// CANONICAL PROMOTION CONTRACT LIST — MOBILE CONSUMER CONTRACT
//
// Proves the canonical seller contract list consumer boundary:
//   M1: PromotionContractDto maps the canonical wire keys (id, kind, status,
//       budget_rupiah, cpm_rupiah, planned_start, planned_finish, city_ids)
//       — the promotion_contracts authority. No package/ownership vocabulary.
//   M2: PromotionContractRepositoryImpl calls exactly GET /promotions/contracts
//       — never /promotions/my, never /promotions/my/instances.
//   M3: Provider states: loading -> data, loading -> error, empty success.
//   M4: Canonical contract list rendering (Internal/Eksternal, status,
//       budget, CPM, geography, created date).
//   M5: Truthful empty state (not an error).
//   M6: Error state with a WORKING retry (real provider refetch).
//   M7: 'Lihat Analitik' navigates with the exact canonical contract id to
//       /seller/promotions/:contractId/analytics.
//   M8: Real seller entry point (Seller Dashboard 'Kelola Promosi') reaches
//       CanonicalPromotionListScreen — no second orphaned route.
//   M9: Legacy isolation — the new surface never touches the legacy
//       /promotions/my or instance model/provider/endpoint.
// ============================================================================

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/router/modules/seller_module.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/promotion_contract_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/promotion_contract_repository.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/canonical_promotion_providers.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_list_screen.dart';
import 'package:labuda/shared/services/logger_service.dart';

// ============================================================================
// Fake ApiClient — records every path; canned responses; optional pending gate
// (loading proof) and fail-first-N (retry proof).
// ============================================================================

class _ListApiClient implements ApiClient {
  final List<String> pathsCalled = [];
  Response<dynamic>? response;
  Exception? error;
  int failFirstCalls = 0;
  Completer<void>? _gate;

  _ListApiClient({this.response, this.error, this.failFirstCalls = 0});

  factory _ListApiClient.pending() => _ListApiClient().._gate = Completer<void>();

  void complete(Response<dynamic> value) {
    _gate?.complete();
    response = value;
  }

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    pathsCalled.add(path);
    if (failFirstCalls > 0) {
      failFirstCalls--;
      throw const ServerException(message: 'boom');
    }
    final err = error;
    if (err != null) {
      throw err;
    }
    final gate = _gate;
    if (gate != null) {
      await gate.future;
    }
    return (response ??
            Response<dynamic>(
              requestOptions: RequestOptions(path: path),
              data: <String, dynamic>{},
              statusCode: 200,
            ))
        as Response<T>;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Response<dynamic> _listEnvelope({
  required List<Map<String, dynamic>> contracts,
  int? count,
}) {
  return Response<dynamic>(
    requestOptions: RequestOptions(path: '/promotions/contracts'),
    data: <String, dynamic>{
      'success': true,
      'data': <String, dynamic>{
        'contracts': contracts,
        'count': count ?? contracts.length,
      },
    },
    statusCode: 200,
  );
}

Map<String, dynamic> _contractPayload({
  required String id,
  String kind = 'internal',
  String status = 'active',
  int budgetRupiah = 500000,
  int cpmRupiah = 25000,
  List<String> cityIds = const [],
}) {
  return <String, dynamic>{
    'id': id,
    'seller_id': 'seller-1',
    'kind': kind,
    'status': status,
    'budget_rupiah': budgetRupiah,
    'cpm_rupiah': cpmRupiah,
    'planned_start': '2026-09-01T00:00:00Z',
    'planned_finish': '2026-10-01T00:00:00Z',
    'allocation_account_id': 'alloc-1',
    'paused_at': null,
    'finalized_at': null,
    'created_at': '2026-09-01T00:00:00Z',
    'updated_at': '2026-09-02T00:00:00Z',
    'city_ids': cityIds,
  };
}

Widget _screenHarness(_ListApiClient client) {
  return ProviderScope(
    overrides: [apiClientProvider.overrideWithValue(client)],
    child: const MaterialApp(
      home: CanonicalPromotionListScreen(),
    ),
  );
}

Future<void> _flush(WidgetTester tester) async {
  await tester.pump();
  await tester.pump();
}

// ============================================================================
// M1 — DTO canonical identity
// ============================================================================

void main() {
  setUpAll(() async {
    // AppFormatters uses DateFormat('dd MMM yyyy', 'id_ID'); widget tests
    // render formatted dates, which requires the intl locale data.
    await initializeDateFormatting('id_ID');
  });

  group('M1: DTO canonical identity', () {
    test('maps contract wire keys — never package/ownership vocabulary', () {
      final dto = PromotionContractDto.fromJson(
        _contractPayload(
          id: 'CONTRACT-UUID',
          kind: 'internal',
          status: 'active',
          budgetRupiah: 500000,
          cpmRupiah: 25000,
        ),
      );

      expect(dto.id, 'CONTRACT-UUID');
      expect(dto.sellerId, 'seller-1');
      expect(dto.kind, 'internal');
      expect(dto.status, 'active');
      expect(dto.budgetRupiah, 500000);
      expect(dto.cpmRupiah, 25000);
      expect(dto.plannedStart, '2026-09-01T00:00:00Z');
      expect(dto.plannedFinish, '2026-10-01T00:00:00Z');
      expect(dto.allocationAccountId, 'alloc-1');
      expect(dto.pausedAt, isNull);
      expect(dto.finalizedAt, isNull);
      expect(dto.cityIds, isEmpty);
    });

    test('city_ids parse truthfully; empty = nationwide', () {
      final nationwide = PromotionContractDto.fromJson(_contractPayload(id: 'C1'));
      expect(nationwide.cityIds, isEmpty);

      final scoped = PromotionContractDto.fromJson(
        _contractPayload(id: 'C2', cityIds: ['city-a', 'city-b']),
      );
      expect(scoped.cityIds, ['city-a', 'city-b']);
    });

    test('list DTO maps contracts + count; empty is a truthful state', () {
      final list = PromotionContractListDto.fromJson(<String, dynamic>{
        'contracts': [
          _contractPayload(id: 'CONTRACT-1'),
          _contractPayload(id: 'CONTRACT-2', status: 'paused'),
        ],
        'count': 2,
      });

      expect(list.contracts, hasLength(2));
      expect(list.count, 2);
      expect(list.contracts[0].id, 'CONTRACT-1');
      expect(list.contracts[0].status, 'active');

      final empty = PromotionContractListDto.fromJson(<String, dynamic>{
        'contracts': <dynamic>[],
        'count': 0,
      });
      expect(empty.contracts, isEmpty);
      expect(empty.count, 0);
    });
  });

  // ==========================================================================
  // M2 — Repository exact endpoint
  // ==========================================================================

  group('M2: repository calls exactly GET /promotions/contracts', () {
    test('success maps the envelope into the contract list DTO', () async {
      final client = _ListApiClient(
        response: _listEnvelope(contracts: [_contractPayload(id: 'CONTRACT-1')]),
      );
      final repo = PromotionContractRepositoryImpl(client);

      final result = await repo.listMyContracts();

      expect(client.pathsCalled, <String>['/promotions/contracts']);
      expect(result.isSuccess, isTrue);
      expect(result.data!.contracts.single.id, 'CONTRACT-1');
      expect(result.data!.count, 1);
    });

    test('never calls legacy /promotions/my, instances, or analytics', () async {
      final client = _ListApiClient(
        response: _listEnvelope(contracts: []),
      );
      final repo = PromotionContractRepositoryImpl(client);

      await repo.listMyContracts();

      final forbidden = client.pathsCalled.where(
        (p) =>
            p.contains('/promotions/my') ||
            p.contains('/promotions/instances') ||
            p.contains('/promotions/ownerships') ||
            p.contains('/analytics'),
      );
      expect(forbidden, isEmpty,
          reason: 'the canonical list must never read legacy surfaces');
      expect(client.pathsCalled, hasLength(1));
    });

    test('failure surfaces Result.error, never a fabricated list', () async {
      final client = _ListApiClient(
        error: const ServerException(message: 'boom'),
      );
      final repo = PromotionContractRepositoryImpl(client);

      final result = await repo.listMyContracts();

      expect(result.isError, isTrue);
      expect(result.error, 'boom');
      expect(result.data, isNull);
    });
  });

  // ==========================================================================
  // M3 — Provider states
  // ==========================================================================

  group('M3: provider states', () {
    test('loading -> data', () async {
      final client = _ListApiClient.pending();
      final container = ProviderContainer(
        overrides: [apiClientProvider.overrideWithValue(client)],
      );
      addTearDown(container.dispose);

      final async = container.read(myPromotionContractsProvider);
      expect(async.isLoading, isTrue);

      client.complete(_listEnvelope(contracts: [_contractPayload(id: 'C1')]));
      final result = await container.read(myPromotionContractsProvider.future);

      expect(result.isSuccess, isTrue);
      expect(result.data!.contracts.single.id, 'C1');
    });

    test('loading -> error', () async {
      final client = _ListApiClient(
        error: const ServerException(message: 'boom'),
      );
      final container = ProviderContainer(
        overrides: [apiClientProvider.overrideWithValue(client)],
      );
      addTearDown(container.dispose);

      final async = container.read(myPromotionContractsProvider);
      expect(async.isLoading, isTrue);

      final result = await container.read(myPromotionContractsProvider.future);

      expect(result.isError, isTrue);
      expect(result.data, isNull);
    });

    test('empty list is a successful state, not an error', () async {
      final client = _ListApiClient(response: _listEnvelope(contracts: []));
      final container = ProviderContainer(
        overrides: [apiClientProvider.overrideWithValue(client)],
      );
      addTearDown(container.dispose);

      final result = await container.read(myPromotionContractsProvider.future);

      expect(result.isSuccess, isTrue);
      expect(result.data!.contracts, isEmpty);
    });
  });

  // ==========================================================================
  // M4 — Canonical contract list rendering
  // ==========================================================================

  group('M4: canonical contract list rendering', () {
    testWidgets('renders truthful contract facts for each item', (
      tester,
    ) async {
      final client = _ListApiClient(
        response: _listEnvelope(contracts: [
          _contractPayload(
            id: 'CONTRACT-1',
            kind: 'internal',
            status: 'active',
            budgetRupiah: 500000,
            cpmRupiah: 25000,
          ),
          _contractPayload(
            id: 'CONTRACT-2',
            kind: 'external',
            status: 'paused',
            budgetRupiah: 1000000,
            cpmRupiah: 50000,
            cityIds: ['city-a'],
          ),
        ]),
      );

      await tester.pumpWidget(_screenHarness(client));
      await _flush(tester);

      expect(find.text('Internal'), findsOneWidget);
      expect(find.text('Eksternal'), findsOneWidget);
      expect(find.text('Aktif'), findsOneWidget);
      expect(find.text('Dijeda'), findsOneWidget);
      expect(find.text('Lihat Analitik'), findsNWidgets(2));
      expect(find.textContaining('Budget:'), findsNWidgets(2));
      expect(find.textContaining('CPM:'), findsNWidgets(2));
      // No error or empty state is shown.
      expect(find.text('Gagal Memuat Promosi'), findsNothing);
      expect(find.text('Belum ada promosi'), findsNothing);
    });
  });

  // ==========================================================================
  // M5 — Truthful empty state
  // ==========================================================================

  group('M5: empty state', () {
    testWidgets('successful empty list shows empty state, not error', (
      tester,
    ) async {
      final client = _ListApiClient(response: _listEnvelope(contracts: []));

      await tester.pumpWidget(_screenHarness(client));
      await _flush(tester);

      expect(find.text('Belum ada promosi'), findsOneWidget);
      expect(find.text('Gagal Memuat Promosi'), findsNothing);
      expect(find.byType(CircularProgressIndicator), findsNothing);
    });
  });

  // ==========================================================================
  // M6 — Error state + working retry
  // ==========================================================================

  group('M6: error state + retry', () {
    testWidgets('first failure shows error; retry refetches and renders data', (
      tester,
    ) async {
      final client = _ListApiClient(
        failFirstCalls: 1,
        response: _listEnvelope(contracts: [
          _contractPayload(id: 'CONTRACT-AFTER-RETRY'),
        ]),
      );

      await tester.pumpWidget(_screenHarness(client));
      await _flush(tester);

      expect(find.text('Gagal Memuat Promosi'), findsOneWidget);
      expect(find.text('boom'), findsOneWidget);

      // Tap the retry button — must trigger a REAL provider refetch.
      await tester.tap(find.text('Coba Lagi'));
      await _flush(tester);

      expect(client.pathsCalled, hasLength(2),
          reason: 'retry must actually re-request GET /promotions/contracts');
      expect(find.text('Internal'), findsOneWidget);
      expect(find.text('Gagal Memuat Promosi'), findsNothing);
    });
  });

  // ==========================================================================
  // M7 — Analytics ID traversal (mandatory)
  // ==========================================================================

  group('M7: analytics ID traversal', () {
    testWidgets('Lihat Analitik navigates with the exact canonical contract id', (
      tester,
    ) async {
      final client = _ListApiClient(
        response: _listEnvelope(contracts: [
          _contractPayload(id: 'CONTRACT-UUID'),
        ]),
      );
      final capturedContractIds = <String>[];
      final router = GoRouter(
        initialLocation: '/seller/canonical-promotions',
        routes: [
          GoRoute(
            path: '/seller/canonical-promotions',
            builder: (context, state) => const CanonicalPromotionListScreen(),
          ),
          GoRoute(
            path: '/seller/promotions/:contractId/analytics',
            builder: (context, state) {
              capturedContractIds.add(state.pathParameters['contractId']!);
              return const Scaffold(body: Text('analytics-screen'));
            },
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [apiClientProvider.overrideWithValue(client)],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await _flush(tester);

      await tester.tap(find.text('Lihat Analitik'));
      await _flush(tester);

      expect(capturedContractIds, <String>['CONTRACT-UUID'],
          reason: 'navigation must carry promotion_contracts.id');
      expect(find.text('analytics-screen'), findsOneWidget);
    });
  });

  // ==========================================================================
  // M8 — Entry point reachability (real seller dashboard action)
  // ==========================================================================

  group('M8: entry point reachability', () {
    testWidgets(
      'Seller Dashboard Kelola Promosi reaches CanonicalPromotionListScreen',
      (tester) async {
        _setTallViewport(tester);

        final client = _ListApiClient(
          response: _listEnvelope(contracts: [
            _contractPayload(id: 'CONTRACT-FROM-DASHBOARD'),
          ]),
        );
        final router = GoRouter(
          initialLocation: '/seller/dashboard',
          routes: [...SellerModule().routes],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              apiClientProvider.overrideWithValue(client),
              authControllerProvider.overrideWith(
                _FakeSellerAuthController.new,
              ),
              loggerServiceProvider.overrideWithValue(LoggerService.instance),
              // The dashboard's order sections stream via watchSellerOrders,
              // which schedules 30s polling timers in the real repository.
              // This fake returns timer-free empty streams so the entry-point
              // proof is not blocked by the unrelated order subsystem.
              orderRepositoryProvider.overrideWithValue(
                _TimerFreeOrderRepository(),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await _flush(tester);

        // The dashboard's 'Kelola Promosi' quick action is the real entry.
        await tester.ensureVisible(find.text('Kelola Promosi'));
        await tester.pump();
        await tester.tap(find.text('Kelola Promosi'));
        await _flush(tester);

        expect(find.byType(CanonicalPromotionListScreen), findsOneWidget,
            reason: 'tapping the dashboard action must open the canonical list');
        expect(find.text('Internal'), findsOneWidget,
            reason: 'the list loaded through the real route');
      },
    );
  });

  // ==========================================================================
  // M9 — Legacy isolation
  // ==========================================================================

  group('M9: legacy isolation', () {
    test('new surface never requests legacy /promotions/my or instances', () async {
      final client = _ListApiClient(response: _listEnvelope(contracts: []));
      final container = ProviderContainer(
        overrides: [apiClientProvider.overrideWithValue(client)],
      );
      addTearDown(container.dispose);

      await container.read(myPromotionContractsProvider.future);

      expect(client.pathsCalled, <String>['/promotions/contracts']);
      expect(client.pathsCalled.where((p) => p.contains('my')), isEmpty);
      expect(client.pathsCalled.where((p) => p.contains('instances')), isEmpty);
      expect(client.pathsCalled.where((p) => p.contains('ownerships')), isEmpty);
      expect(client.pathsCalled.where((p) => p.contains('packages')), isEmpty);
    });

    testWidgets('list screen renders contract DTOs, not legacy instances', (
      tester,
    ) async {
      final client = _ListApiClient(
        response: _listEnvelope(contracts: [
          _contractPayload(id: 'CONTRACT-1'),
        ]),
      );

      await tester.pumpWidget(_screenHarness(client));
      await _flush(tester);

      // The canonical contract vocabulary is used, and no legacy instance
      // wording (e.g. "Remaining:", package durations) appears.
      expect(find.text('Internal'), findsOneWidget);
      expect(find.textContaining('Remaining:'), findsNothing);
      expect(find.textContaining('Expires:'), findsNothing);
      expect(find.textContaining('Package'), findsNothing);
    });
  });
}

// ============================================================================
// Harness helpers
// ============================================================================

/// Order repository stub with timer-free watch streams (the real
/// OrderRepositoryImpl schedules 30s polling timers, which are unrelated to
/// this slice and would trip flutter_test's pending-timer invariant).
class _TimerFreeOrderRepository implements OrderRepository {
  @override
  Stream<List<Order>> watchSellerOrders(WatchOrdersParams params) =>
      Stream.value(const <Order>[]);

  @override
  Stream<List<Order>> watchBuyerOrders(WatchOrdersParams params) =>
      Stream.value(const <Order>[]);

  @override
  Stream<Order> watchOrder(String orderId) => Stream.empty();

  @override
  Stream<List<Order>> watchSellerNewOrders(String sellerId) =>
      Stream.value(const <Order>[]);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeSellerAuthController extends AuthController {
  @override
  AuthState build() => AuthState.authenticated(
    AuthUser(
      id: 'seller-1',
      email: 'seller@labuda.test',
      username: 'seller',
      isEmailVerified: true,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      hasSellerProfile: true,
      hasMarketAuthority: true,
    ),
    emailVerified: true,
  );
}

void _setTallViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 3000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}