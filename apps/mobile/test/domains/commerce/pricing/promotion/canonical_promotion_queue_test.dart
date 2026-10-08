import 'dart:collection';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/canonical_promotion_providers.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_queue_screen.dart';

// ============================================================================
// CANONICAL PROMOTION REFILL QUEUE — MOBILE CONTRACT
//
// Proves the mobile refill surface talks ONLY to the canonical backend queue
// authority (promotion_contract_targets via /promotions/contracts/:id/targets):
//   - the queue rendered comes from GET .../targets (server truth);
//   - "Tambah Produk" appends through POST .../targets and then re-reads the
//     server queue (no optimistic final state);
//   - duplicate selection is not posted;
//   - a full queue (10) disables the add action;
//   - remove goes through DELETE .../targets/:targetId;
//   - backend queue errors are surfaced, never converted to success;
//   - refill NEVER calls create/funding/payment.
// ============================================================================

class _MapResponse<T> extends Response<T> with MapMixin<String, dynamic> {
  final Map<String, dynamic> _map;

  _MapResponse({
    required super.requestOptions,
    required Map<String, dynamic> data,
    super.statusCode,
  }) : _map = data,
       super(data: data as T);

  @override
  dynamic operator [](Object? key) => _map[key];

  @override
  void operator []=(String key, dynamic value) => _map[key] = value;

  @override
  void clear() => _map.clear();

  @override
  Iterable<String> get keys => _map.keys;

  @override
  dynamic remove(Object? key) => _map.remove(key);
}

/// Stateful fake backend for the canonical queue endpoints.
class _QueueApiClient implements ApiClient {
  final List<Map<String, dynamic>> targets;
  final List<String> calls = [];
  final List<dynamic> postPayloads = [];

  Object? postError;

  _QueueApiClient(this.targets);

  Map<String, dynamic> _row(String type, String id, int position) => {
    'ID': 't$position',
    'ContractID': 'ctr-1',
    'TargetType': type,
    'TargetID': id,
    'Position': position,
    'AddedAt': '2026-06-01T00:00:00Z',
  };

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    calls.add('GET $path');
    return _MapResponse<T>(
      requestOptions: RequestOptions(path: path),
      data: {
        'data': {
          'targets': List<Map<String, dynamic>>.from(targets),
          'count': targets.length,
        },
      },
      statusCode: 200,
    );
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    calls.add('POST $path');
    postPayloads.add(data);
    if (postError != null) throw postError!;
    final body = data as Map;
    final row = _row(
      body['target_type'] as String,
      body['target_id'] as String,
      targets.length,
    );
    targets.add(row);
    return _MapResponse<T>(
      requestOptions: RequestOptions(path: path),
      data: {
        'data': {'target': row},
      },
      statusCode: 201,
    );
  }

  @override
  Future<Response<T>> delete<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    calls.add('DELETE $path');
    final id = path.split('/').last;
    targets.removeWhere((t) => t['TargetID'] == id);
    return _MapResponse<T>(
      requestOptions: RequestOptions(path: path),
      data: {'data': <String, dynamic>{}},
      statusCode: 200,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

PromotionProductPicker _picker(String targetType, String targetId) =>
    (BuildContext context, WidgetRef ref, String kind) async =>
        PromotionProductSelection(
          targetType: targetType,
          targetId: targetId,
          title: 'Produk Uji',
        );

Widget _harness(
  _QueueApiClient client, {
  String kind = 'internal',
  PromotionProductPicker? picker,
}) {
  return ProviderScope(
    overrides: [
      apiClientProvider.overrideWithValue(client),
      promotionProductPickerProvider.overrideWithValue(
        picker ?? _picker('auction', 'au-1'),
      ),
    ],
    child: MaterialApp(
      home: CanonicalPromotionQueueScreen(contractId: 'ctr-1', kind: kind),
    ),
  );
}

Future<void> _flush(WidgetTester tester) async {
  for (var i = 0; i < 5; i++) {
    await tester.pump(const Duration(milliseconds: 100));
  }
  await tester.pump();
  await tester.pump();
}

Map<String, dynamic> _row(String type, String id, int position) => {
  'ID': 't$position',
  'ContractID': 'ctr-1',
  'TargetType': type,
  'TargetID': id,
  'Position': position,
  'AddedAt': '2026-06-01T00:00:00Z',
};

void main() {
  testWidgets('renders the canonical queue from the server', (tester) async {
    final client = _QueueApiClient([
      _row('for_sale', 'fs-1', 0),
      _row('auction', 'au-1', 1),
    ]);
    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    expect(client.calls, contains('GET /promotions/contracts/ctr-1/targets'));
    expect(find.text('For Sale'), findsOneWidget);
    expect(find.text('Lelang'), findsOneWidget);
  });

  testWidgets('add appends through AddTarget then re-reads server truth', (
    tester,
  ) async {
    final client = _QueueApiClient([_row('for_sale', 'fs-1', 0)]);
    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    await tester.tap(find.text('Tambah Produk'));
    await _flush(tester);

    expect(
      client.calls,
      contains('POST /promotions/contracts/ctr-1/targets'),
      reason: 'refill must use the canonical AddTarget endpoint',
    );
    expect(client.postPayloads.last, {
      'target_type': 'auction',
      'target_id': 'au-1',
    });
    // Server truth re-read: the new target is rendered from GET, not appended.
    expect(find.text('Lelang'), findsOneWidget);
    expect(
      client.calls.where((c) => c == 'GET /promotions/contracts/ctr-1/targets'),
      hasLength(2),
    );
  });

  testWidgets('a duplicate selection is never posted', (tester) async {
    final client = _QueueApiClient([_row('for_sale', 'fs-1', 0)]);
    await tester.pumpWidget(
      _harness(client, picker: _picker('for_sale', 'fs-1')),
    );
    await _flush(tester);

    await tester.tap(find.text('Tambah Produk'));
    await _flush(tester);

    expect(
      client.calls.where((c) => c.startsWith('POST')),
      isEmpty,
      reason: 'duplicate target must not reach the backend',
    );
    expect(find.text('Produk sudah ada di antrian'), findsOneWidget);
  });

  testWidgets('a full queue (10) disables the add action', (tester) async {
    final client = _QueueApiClient([
      for (var i = 0; i < 10; i++) _row('for_sale', 'fs-$i', i),
    ]);
    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    expect(find.text('Antrian penuh'), findsOneWidget);
    final fab = tester.widget<FloatingActionButton>(
      find.byType(FloatingActionButton),
    );
    expect(fab.onPressed, isNull);
    expect(client.calls.where((c) => c.startsWith('POST')), isEmpty);
  });

  testWidgets('remove goes through DeleteTarget and re-reads server truth', (
    tester,
  ) async {
    final client = _QueueApiClient([
      _row('for_sale', 'fs-1', 0),
      _row('auction', 'au-1', 1),
    ]);
    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    await tester.tap(find.byIcon(Icons.delete_outline).first);
    await _flush(tester);
    // Confirm the canonical dialog.
    await tester.tap(find.text('Hapus'));
    await _flush(tester);

    expect(
      client.calls,
      contains('DELETE /promotions/contracts/ctr-1/targets/fs-1'),
    );
    expect(find.text('For Sale'), findsNothing);
    expect(find.text('Lelang'), findsOneWidget);
  });

  testWidgets('backend queue rejection is surfaced, not converted to success', (
    tester,
  ) async {
    final client = _QueueApiClient([_row('for_sale', 'fs-1', 0)])
      ..postError = const ConflictException(
        message: 'queue full',
        code: 'QUEUE_FULL',
        statusCode: 409,
      );
    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    await tester.tap(find.text('Tambah Produk'));
    await _flush(tester);

    expect(
      find.text('Antrian penuh (maksimal 10 produk).'),
      findsOneWidget,
    );
  });

  testWidgets('refill never touches create / funding / payment paths', (
    tester,
  ) async {
    final client = _QueueApiClient([_row('for_sale', 'fs-1', 0)]);
    await tester.pumpWidget(_harness(client));
    await _flush(tester);

    await tester.tap(find.text('Tambah Produk'));
    await _flush(tester);

    final nonTargetCalls = client.calls
        .where((c) => !c.contains('/promotions/contracts/ctr-1/targets'))
        .toList();
    expect(
      nonTargetCalls,
      isEmpty,
      reason: 'refill must not create a promotion, funding intent, or payment',
    );
  });
}
