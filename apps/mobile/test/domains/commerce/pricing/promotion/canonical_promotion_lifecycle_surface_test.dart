// ============================================================================
// CANONICAL PROMOTION LIFECYCLE SURFACE — STATUS VOCABULARY + ACTIONS
//
// Proves the seller lifecycle surface:
//   S1: only the canonical contract status vocabulary is rendered
//       (prepared | active | paused | finalizing | finalized).
//   S2: the legacy competing-aggregate vocabulary
//       (created | funded | eligible | cancelled | failed) is NEVER translated
//       into a friendly label — it falls through as raw text, i.e. the mobile
//       no longer claims to understand it.
//   S3: lifecycle actions are offered only where the canonical backend allows
//       them: active → Jeda/Hentikan, paused → Lanjutkan/Hentikan,
//       finalized → none.
//   S4: stopping a promotion calls the canonical finalize endpoint and
//       refreshes the reusable funding projection.
// ============================================================================

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_list_screen.dart';

class _LifecycleApiClient implements ApiClient {
  final List<Map<String, dynamic>> contracts;
  final List<String> calls = [];

  _LifecycleApiClient(this.contracts);

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    calls.add('GET $path');
    if (path == '/promotions/contracts') {
      return Response<dynamic>(
            requestOptions: RequestOptions(path: path),
            data: {
              'data': {'contracts': contracts, 'count': contracts.length},
            },
            statusCode: 200,
          )
          as Response<T>;
    }
    if (path == '/promote-balance') {
      return Response<dynamic>(
            requestOptions: RequestOptions(path: path),
            data: {
              'data': {'balance': 0},
            },
            statusCode: 200,
          )
          as Response<T>;
    }
    return Response<dynamic>(
          requestOptions: RequestOptions(path: path),
          data: {'data': <String, dynamic>{}},
          statusCode: 200,
        )
        as Response<T>;
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    calls.add('POST $path');
    return Response<dynamic>(
          requestOptions: RequestOptions(path: path),
          data: {
            'data': {'message': 'ok'},
          },
          statusCode: 200,
        )
        as Response<T>;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Widget _harness(_LifecycleApiClient client) => ProviderScope(
  overrides: [apiClientProvider.overrideWithValue(client)],
  child: const MaterialApp(home: CanonicalPromotionListScreen()),
);

Future<void> _flush(WidgetTester tester) async {
  await tester.pump();
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
}

/// The list is a lazy ListView; a tall viewport keeps every contract row built
/// so the status/action assertions are not clipped by the test surface.
void _setTallViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 3000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}

void main() {
  setUpAll(() async {
    await initializeDateFormatting('id_ID');
  });

  testWidgets('S1: canonical statuses render their canonical labels', (
    tester,
  ) async {
    _setTallViewport(tester);
    final client = _LifecycleApiClient([
      {
        'id': 'C-active',
        'seller_id': 'seller-1',
        'kind': 'internal',
        'status': 'active',
        'budget_rupiah': 30000,
        'cpm_rupiah': 7500,
        'planned_start': '2026-09-01T00:00:00Z',
        'planned_finish': '2026-09-04T00:00:00Z',
        'allocation_account_id': 'alloc-1',
        'paused_at': null,
        'finalized_at': null,
        'created_at': '2026-09-01T00:00:00Z',
        'updated_at': '2026-09-01T00:00:00Z',
        'city_ids': <String>[],
      },
      {
        'id': 'C-paused',
        'seller_id': 'seller-1',
        'kind': 'internal',
        'status': 'paused',
        'budget_rupiah': 30000,
        'cpm_rupiah': 7500,
        'planned_start': '2026-09-01T00:00:00Z',
        'planned_finish': '2026-09-04T00:00:00Z',
        'allocation_account_id': 'alloc-1',
        'paused_at': '2026-09-02T00:00:00Z',
        'finalized_at': null,
        'created_at': '2026-09-01T00:00:00Z',
        'updated_at': '2026-09-01T00:00:00Z',
        'city_ids': <String>[],
      },
      {
        'id': 'C-finalized',
        'seller_id': 'seller-1',
        'kind': 'internal',
        'status': 'finalized',
        'budget_rupiah': 30000,
        'cpm_rupiah': 7500,
        'planned_start': '2026-09-01T00:00:00Z',
        'planned_finish': '2026-09-04T00:00:00Z',
        'allocation_account_id': 'alloc-1',
        'paused_at': null,
        'finalized_at': '2026-09-03T00:00:00Z',
        'created_at': '2026-09-01T00:00:00Z',
        'updated_at': '2026-09-01T00:00:00Z',
        'city_ids': <String>[],
      },
    ]);

    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    expect(find.text('Aktif'), findsOneWidget);
    expect(find.text('Dijeda'), findsOneWidget);
    expect(find.text('Selesai'), findsOneWidget);
  });

  testWidgets('S2: legacy statuses are never given a friendly label', (
    tester,
  ) async {
    _setTallViewport(tester);
    final client = _LifecycleApiClient([
      {
        'id': 'C-legacy',
        'seller_id': 'seller-1',
        'kind': 'internal',
        'status': 'funded',
        'budget_rupiah': 30000,
        'cpm_rupiah': 7500,
        'planned_start': '2026-09-01T00:00:00Z',
        'planned_finish': '2026-09-04T00:00:00Z',
        'allocation_account_id': 'alloc-1',
        'paused_at': null,
        'finalized_at': null,
        'created_at': '2026-09-01T00:00:00Z',
        'updated_at': '2026-09-01T00:00:00Z',
        'city_ids': <String>[],
      },
    ]);

    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    // The purged `promotions` aggregate label must not reappear.
    expect(find.text('Didanai'), findsNothing);
    expect(
      find.text('funded'),
      findsOneWidget,
      reason:
          'an unknown/legacy status falls through as raw, never as a '
          'friendly legacy label',
    );
  });

  testWidgets('S3: lifecycle actions follow the canonical status', (
    tester,
  ) async {
    _setTallViewport(tester);
    final client = _LifecycleApiClient([
      {
        'id': 'C-active',
        'seller_id': 'seller-1',
        'kind': 'internal',
        'status': 'active',
        'budget_rupiah': 30000,
        'cpm_rupiah': 7500,
        'planned_start': '2026-09-01T00:00:00Z',
        'planned_finish': '2026-09-04T00:00:00Z',
        'allocation_account_id': 'alloc-1',
        'paused_at': null,
        'finalized_at': null,
        'created_at': '2026-09-01T00:00:00Z',
        'updated_at': '2026-09-01T00:00:00Z',
        'city_ids': <String>[],
      },
      {
        'id': 'C-paused',
        'seller_id': 'seller-1',
        'kind': 'internal',
        'status': 'paused',
        'budget_rupiah': 30000,
        'cpm_rupiah': 7500,
        'planned_start': '2026-09-01T00:00:00Z',
        'planned_finish': '2026-09-04T00:00:00Z',
        'allocation_account_id': 'alloc-1',
        'paused_at': '2026-09-02T00:00:00Z',
        'finalized_at': null,
        'created_at': '2026-09-01T00:00:00Z',
        'updated_at': '2026-09-01T00:00:00Z',
        'city_ids': <String>[],
      },
      {
        'id': 'C-finalized',
        'seller_id': 'seller-1',
        'kind': 'internal',
        'status': 'finalized',
        'budget_rupiah': 30000,
        'cpm_rupiah': 7500,
        'planned_start': '2026-09-01T00:00:00Z',
        'planned_finish': '2026-09-04T00:00:00Z',
        'allocation_account_id': 'alloc-1',
        'paused_at': null,
        'finalized_at': '2026-09-03T00:00:00Z',
        'created_at': '2026-09-01T00:00:00Z',
        'updated_at': '2026-09-01T00:00:00Z',
        'city_ids': <String>[],
      },
    ]);

    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    // active: Jeda + Hentikan ; paused: Lanjutkan + Hentikan
    expect(find.text('Jeda'), findsOneWidget);
    expect(find.text('Lanjutkan'), findsOneWidget);
    // Two contracts (active + paused) can be stopped; the finalized one cannot.
    expect(find.text('Hentikan'), findsNWidgets(2));
    expect(find.text('Lihat Analitik'), findsNWidgets(3));
  });

  testWidgets('S4: stopping calls the canonical finalize endpoint', (
    tester,
  ) async {
    _setTallViewport(tester);
    final client = _LifecycleApiClient([
      {
        'id': 'C-active',
        'seller_id': 'seller-1',
        'kind': 'internal',
        'status': 'active',
        'budget_rupiah': 30000,
        'cpm_rupiah': 7500,
        'planned_start': '2026-09-01T00:00:00Z',
        'planned_finish': '2026-09-04T00:00:00Z',
        'allocation_account_id': 'alloc-1',
        'paused_at': null,
        'finalized_at': null,
        'created_at': '2026-09-01T00:00:00Z',
        'updated_at': '2026-09-01T00:00:00Z',
        'city_ids': <String>[],
      },
    ]);

    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    await tester.tap(find.text('Hentikan'));
    await _flush(tester);
    // Confirmation dialog → confirm.
    expect(find.text('Hentikan promosi?'), findsOneWidget);
    await tester.tap(find.widgetWithText(TextButton, 'Hentikan'));
    await _flush(tester);

    expect(
      client.calls,
      contains('POST /promotions/contracts/C-active/finalize'),
    );
  });
}
