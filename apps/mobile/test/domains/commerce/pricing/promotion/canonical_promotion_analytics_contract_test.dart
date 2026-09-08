// ============================================================================
// CANONICAL PROMOTION DELIVERY ANALYTICS — MOBILE BOUNDARY CONTRACT
//
// Proves the analytics consumer boundary is TRUTHFUL:
//   P1: CanonicalPromotionAnalyticsDto maps exactly the canonical wire keys
//       contract_id / included_count / impression_count — no aliases.
//   P2: CanonicalPromotionAnalyticsRepositoryImpl calls exactly
//       GET /promotions/contracts/:id/analytics and never a legacy analytics
//       path (campaigns/instances/events or /promotions/:id/analytics).
//   P3: The analytics UI renders the truthful zero state (0 / 0) when the
//       backend reports no events.
//   P4: The loading state follows the project convention.
//   P5: An API failure surfaces an error state and NEVER fabricates metrics.
//
// P7 (real seller entry point) is proven by the contract list test (M7):
// the canonical list screen navigates with the exact contract id.
// ============================================================================

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/canonical_promotion_analytics_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/canonical_promotion_analytics_repository.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/screens/canonical_promotion_analytics_screen.dart';

// ============================================================================
// Fake ApiClient — records every path, returns canned responses, and can
// hold the GET open (loading proof) or throw (failure proof).
// ============================================================================

class _AnalyticsApiClient implements ApiClient {
  final List<String> pathsCalled = [];
  Response<dynamic>? response;
  Exception? error;
  Completer<void>? _gate;

  _AnalyticsApiClient({this.response, this.error});

  /// Client whose GET stays pending until [complete] is called.
  factory _AnalyticsApiClient.pending() => _AnalyticsApiClient().._gate = Completer<void>();

  void complete(Response<dynamic> value) {
    _gate!.complete();
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

Response<dynamic> _analyticsEnvelope({
  required String contractId,
  required int included,
  required int impressions,
  int clicks = 0,
}) {
  return Response<dynamic>(
    requestOptions: RequestOptions(
      path: '/promotions/contracts/$contractId/analytics',
    ),
    data: <String, dynamic>{
      'success': true,
      'data': <String, dynamic>{
        'contract_id': contractId,
        'included_count': included,
        'impression_count': impressions,
        'click_count': clicks,
      },
    },
    statusCode: 200,
  );
}

Widget _screenHarness(_AnalyticsApiClient client) {
  return ProviderScope(
    overrides: [apiClientProvider.overrideWithValue(client)],
    child: const MaterialApp(
      home: CanonicalPromotionAnalyticsScreen(contractId: 'contract-1'),
    ),
  );
}

Future<void> _flush(WidgetTester tester) async {
  await tester.pump();
  await tester.pump();
}

// ============================================================================
// P1 — DTO maps the exact canonical wire keys
// ============================================================================

void main() {
  group('P1: CanonicalPromotionAnalyticsDto wire contract', () {
    test('fromJson maps contract_id / included_count / impression_count / click_count', () {
      final dto = CanonicalPromotionAnalyticsDto.fromJson(<String, dynamic>{
        'contract_id': 'contract-abc',
        'included_count': 12,
        'impression_count': 7,
        'click_count': 4,
      });

      expect(dto.contractId, 'contract-abc');
      expect(dto.includedCount, 12);
      expect(dto.impressionCount, 7);
      expect(dto.clickCount, 4);
    });

    test('toJson emits the exact canonical keys', () {
      const dto = CanonicalPromotionAnalyticsDto(
        contractId: 'contract-abc',
        includedCount: 12,
        impressionCount: 7,
        clickCount: 2,
      );

      expect(dto.toJson(), <String, dynamic>{
        'contract_id': 'contract-abc',
        'included_count': 12,
        'impression_count': 7,
        'click_count': 2,
      });
    });

    test('empty factory is a truthful zero state', () {
      final dto = CanonicalPromotionAnalyticsDto.empty('contract-abc');

      expect(dto.contractId, 'contract-abc');
      expect(dto.includedCount, 0);
      expect(dto.impressionCount, 0);
      expect(dto.clickCount, 0);
      expect(dto.isEmpty, isTrue);
    });
  });

  // ==========================================================================
  // P2 — Repository endpoint contract
  // ==========================================================================

  group('P2: repository calls exactly /promotions/contracts/:id/analytics', () {
    test('success maps the envelope into the DTO', () async {
      final client = _AnalyticsApiClient(
        response: _analyticsEnvelope(
          contractId: 'contract-1',
          included: 3,
          impressions: 1,
          clicks: 2,
        ),
      );
      final repo = CanonicalPromotionAnalyticsRepositoryImpl(client);

      final result = await repo.getDeliveryAnalytics('contract-1');

      expect(
        client.pathsCalled,
        <String>['/promotions/contracts/contract-1/analytics'],
      );
      expect(result.isSuccess, isTrue);
      expect(result.data!.contractId, 'contract-1');
      expect(result.data!.includedCount, 3);
      expect(result.data!.impressionCount, 1);
      expect(result.data!.clickCount, 2);
    });

    test('never calls a legacy promotion analytics endpoint', () async {
      final client = _AnalyticsApiClient(
        response: _analyticsEnvelope(
          contractId: 'contract-1',
          included: 0,
          impressions: 0,
        ),
      );
      final repo = CanonicalPromotionAnalyticsRepositoryImpl(client);

      await repo.getDeliveryAnalytics('contract-1');

      final legacyPaths = client.pathsCalled.where(
        (p) =>
            p.contains('/promotions/campaigns/') ||
            p.contains('/promotions/instances/') ||
            p.contains('/promotions/events') ||
            p.contains('/promotions/my') ||
            p == '/promotions/contract-1/analytics',
      );
      expect(legacyPaths, isEmpty,
          reason: 'canonical analytics must hit the contracts route only');
      expect(client.pathsCalled, hasLength(1));
    });

    test('404 returns an error, not fabricated zero metrics', () async {
      final client = _AnalyticsApiClient(
        error: const NotFoundException(message: 'not found'),
      );
      final repo = CanonicalPromotionAnalyticsRepositoryImpl(client);

      final result = await repo.getDeliveryAnalytics('contract-1');

      expect(result.isError, isTrue);
      expect(result.error, 'Promotion not found');
      expect(result.data, isNull);
    });

    test('server failure surfaces the error without fabricating data', () async {
      final client = _AnalyticsApiClient(
        error: const ServerException(message: 'boom'),
      );
      final repo = CanonicalPromotionAnalyticsRepositoryImpl(client);

      final result = await repo.getDeliveryAnalytics('contract-1');

      expect(result.isError, isTrue);
      expect(result.error, 'boom');
      expect(result.data, isNull);
    });
  });

  // ==========================================================================
  // P4 — Loading state follows project convention (spinner + label)
  // ==========================================================================

  group('P4: loading state convention', () {
    testWidgets('shows CircularProgressIndicator while analytics load', (
      tester,
    ) async {
      final client = _AnalyticsApiClient.pending();

      await tester.pumpWidget(_screenHarness(client));
      await tester.pump();

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(find.text('Loading analytics...'), findsOneWidget);

      client.complete(
        _analyticsEnvelope(
          contractId: 'contract-1',
          included: 1,
          impressions: 0,
        ),
      );
      await _flush(tester);

      expect(find.byType(CircularProgressIndicator), findsNothing);
      expect(find.text('Included'), findsOneWidget);
      expect(find.text('Clicks'), findsOneWidget);
    });
  });

  // ==========================================================================
  // P3 — Truthful zero state (+ C15: Clicks rendered from click_count)
  // ==========================================================================

  group('P3: zero state is truthful', () {
    testWidgets('backend-reported zeros render as 0 included / 0 impressions / 0 clicks',
        (tester) async {
      final client = _AnalyticsApiClient(
        response: _analyticsEnvelope(
          contractId: 'contract-1',
          included: 0,
          impressions: 0,
        ),
      );

      await tester.pumpWidget(_screenHarness(client));
      await _flush(tester);

      expect(find.text('Included'), findsOneWidget);
      expect(find.text('Impressions'), findsOneWidget);
      expect(find.text('Clicks'), findsOneWidget);
      // The only numeric values rendered are the three metric values.
      expect(find.text('0'), findsNWidgets(3));
    });

    testWidgets('nonzero backend counts render exactly', (tester) async {
      final client = _AnalyticsApiClient(
        response: _analyticsEnvelope(
          contractId: 'contract-1',
          included: 42,
          impressions: 9,
          clicks: 5,
        ),
      );

      await tester.pumpWidget(_screenHarness(client));
      await _flush(tester);

      expect(find.text('42'), findsOneWidget);
      expect(find.text('9'), findsOneWidget);
      expect(find.text('5'), findsOneWidget);
      expect(find.text('0'), findsNothing);
    });
  });

  // ==========================================================================
  // P5 — API failure never fabricates metrics
  // ==========================================================================

  group('P5: failure does not fabricate metrics', () {
    testWidgets('error state shows message and no metric values', (
      tester,
    ) async {
      final client = _AnalyticsApiClient(
        error: const ServerException(message: 'boom'),
      );

      await tester.pumpWidget(_screenHarness(client));
      await _flush(tester);

      expect(find.text('Failed to Load Analytics'), findsOneWidget);
      expect(find.text('boom'), findsOneWidget);
      // No metric cards and no zero-value placeholders are rendered.
      expect(find.text('Included'), findsNothing);
      expect(find.text('Impressions'), findsNothing);
      expect(find.text('Clicks'), findsNothing);
      expect(find.text('0'), findsNothing);
    });

    testWidgets('not-found error state does not show fake zeros', (tester) async {
      final client = _AnalyticsApiClient(
        error: const NotFoundException(message: 'not found'),
      );

      await tester.pumpWidget(_screenHarness(client));
      await _flush(tester);

      expect(find.text('Failed to Load Analytics'), findsOneWidget);
      expect(find.text('Promotion not found'), findsOneWidget);
      expect(find.text('Included'), findsNothing);
      expect(find.text('Clicks'), findsNothing);
      expect(find.text('0'), findsNothing);
    });
  });
}