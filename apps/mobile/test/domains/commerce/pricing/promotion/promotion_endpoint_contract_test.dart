import 'dart:collection';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart'
    show ConflictException, ForbiddenException, NotFoundException;
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/canonical_promotion_analytics_repository.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/promotion_contract_repository.dart';

/// Canonical promotion endpoint contract.
///
/// The single mobile promotion authority is promotion_contracts:
///   GET    /promotions/contracts
///   POST   /promotions/contracts
///   GET    /promotions/contracts/:id
///   POST   /promotions/contracts/:id/{pause,resume,finalize}
///   GET    /promotions/contracts/:id/analytics
///
/// NEGATIVE PROOF: the legacy package/ownership/instance/discovery/events
/// endpoints are purged. The canonical repository must never call:
///   /promotions/packages, /promotions/packages/purchase,
///   /promotions/my/ownerships, /promotions/my/instances,
///   /promotions/activate, /promotions/instances/*,
///   /promotions/discover, /promotions/events.
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

class _RecordingApiClient implements ApiClient {
  final List<String> getPaths = [];
  final List<String> postPaths = [];
  final List<dynamic> postPayloads = [];

  dynamic getPayload = <String, dynamic>{};
  dynamic postPayload = <String, dynamic>{};

  Object? getError;
  Object? postError;

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    getPaths.add(path);
    if (getError != null) throw getError!;
    return _MapResponse<T>(
      requestOptions: RequestOptions(path: path),
      data: getPayload as Map<String, dynamic>,
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
    postPaths.add(path);
    postPayloads.add(data);
    if (postError != null) throw postError!;
    return _MapResponse<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as Map<String, dynamic>,
      statusCode: 200,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> _contractPayload({String id = 'ctr-1'}) => {
  'id': id,
  'seller_id': 'user-1',
  'kind': 'internal',
  'status': 'active',
  'budget_rupiah': 30000,
  'cpm_rupiah': 1200,
  'planned_start': '2026-06-01T00:00:00Z',
  'planned_finish': '2026-06-04T00:00:00Z',
  'allocation_account_id': 'acc-1',
  'paused_at': null,
  'finalized_at': null,
  'created_at': '2026-06-01T00:00:00Z',
  'updated_at': '2026-06-01T00:00:00Z',
  'city_ids': <String>[],
};

Map<String, dynamic> _analyticsPayload() => {
  'contract_id': 'ctr-1',
  'included_count': 100,
  'impression_count': 60,
  'click_count': 5,
};

/// Sibling disclosure surfaces that MUST NOT be reused for promotion funding:
/// the order-scoped `/payments/methods` (fee base = order cash amount) and the
/// subscription-scoped `/seller/subscription/payment-methods` (fee base = the
/// yearly subscription principal) have different scopes and principals.
const _foreignFundingPaths = <String>[
  '/payments/methods',
  '/seller/subscription/payment-methods',
  '/seller/subscription/initiate',
  '/payments/billing',
];

const _legacyPaths = <String>[
  '/promotions/packages',
  '/promotions/packages/purchase',
  '/promotions/my/ownerships',
  '/promotions/my/instances',
  '/promotions/activate',
  '/promotions/instances/inst-1',
  '/promotions/instances/inst-1/deactivate',
  '/promotions/instances/inst-1/resume',
  '/promotions/discover',
  '/promotions/discover/for_sale',
  '/promotions/events',
];

void main() {
  group('Canonical promotion contract endpoint contract', () {
    test(
      'list + get + create use canonical /promotions/contracts paths',
      () async {
        final client = _RecordingApiClient();

        client.getPayload = {
          'data': {
            'contracts': [_contractPayload()],
            'count': 1,
          },
        };
        final repo = PromotionContractRepositoryImpl(client);
        final listResult = await repo.listMyContracts();
        expect(client.getPaths.last, '/promotions/contracts');
        expect(listResult.isSuccess, true);
        expect(listResult.data!.contracts.single.id, 'ctr-1');

        client.getPayload = {
          'data': {'contract': _contractPayload(id: 'ctr-2')},
        };
        final getResult = await repo.getContract('ctr-2');
        expect(client.getPaths.last, '/promotions/contracts/ctr-2');
        expect(getResult.isSuccess, true);
        expect(getResult.data!.id, 'ctr-2');

        client.postPayload = {
          'data': {'contract': _contractPayload(id: 'ctr-3')},
        };
        final createResult = await repo.createContract(
          kind: 'internal',
          budgetRupiah: 30000,
          durationDays: 3,
          cityIds: const [],
        );
        expect(client.postPaths.last, '/promotions/contracts');
        expect((client.postPayloads.last as Map)['kind'], 'internal');
        expect((client.postPayloads.last as Map)['budget_rupiah'], 30000);
        expect((client.postPayloads.last as Map)['duration_days'], 3);
        expect((client.postPayloads.last as Map)['city_ids'], isEmpty);
        expect(createResult.isSuccess, true);
        expect(createResult.data!.id, 'ctr-3');
      },
    );

    test('pause / resume / finalize use canonical contract paths', () async {
      final client = _RecordingApiClient();
      final repo = PromotionContractRepositoryImpl(client);

      await repo.pauseContract('ctr-1');
      expect(client.postPaths.last, '/promotions/contracts/ctr-1/pause');

      await repo.resumeContract('ctr-1');
      expect(client.postPaths.last, '/promotions/contracts/ctr-1/resume');

      await repo.finalizeContract('ctr-1');
      expect(client.postPaths.last, '/promotions/contracts/ctr-1/finalize');
    });

    test('analytics uses canonical contract analytics endpoint', () async {
      final client = _RecordingApiClient();
      client.getPayload = {'data': _analyticsPayload()};
      final repo = CanonicalPromotionAnalyticsRepositoryImpl(client);

      final result = await repo.getDeliveryAnalytics('ctr-1');
      expect(client.getPaths.last, '/promotions/contracts/ctr-1/analytics');
      expect(result.isSuccess, true);
      expect(result.data!.contractId, 'ctr-1');
    });

    test(
      'promote balance uses canonical read-only GET /promote-balance',
      () async {
        final client = _RecordingApiClient();
        client.getPayload = {
          'data': {'balance': 50000},
        };
        final repo = PromotionContractRepositoryImpl(client);

        final result = await repo.getPromoteBalance();

        expect(client.getPaths.last, '/promote-balance');
        expect(result.isSuccess, true);
        expect(result.data!.balance, 50000);
      },
    );

    test(
      'exact-shortage funding gate uses POST /promotions/contracts/payment-intent',
      () async {
        final client = _RecordingApiClient();
        client.postPayload = {
          'data': {
            'intent': {
              'payment_required': true,
              'shortage': 20000,
              'intent_id': 'intent-1',
              'billing_id': 'bill-1',
              'required_cost': 30000,
              'available_funding': 10000,
            },
          },
        };
        final repo = PromotionContractRepositoryImpl(client);

        final result = await repo.createFundingIntent(
          kind: 'internal',
          budgetRupiah: 30000,
          durationDays: 3,
          cityIds: const [],
        );

        expect(client.postPaths.last, '/promotions/contracts/payment-intent');
        expect((client.postPayloads.last as Map)['kind'], 'internal');
        expect((client.postPayloads.last as Map)['budget_rupiah'], 30000);
        expect((client.postPayloads.last as Map)['duration_days'], 3);
        expect(result.isSuccess, true);
        // The client renders the backend numbers verbatim — it never recomputes
        // the shortage or the available balance.
        expect(result.data!.paymentRequired, true);
        expect(result.data!.shortage, 20000);
        expect(result.data!.requiredCost, 30000);
        expect(result.data!.availableFunding, 10000);
        expect(result.data!.intentId, 'intent-1');
        expect(result.data!.billingId, 'bill-1');
      },
    );

    test(
      'sufficient reusable balance reports payment_required = false and no intent',
      () async {
        final client = _RecordingApiClient();
        client.postPayload = {
          'data': {
            'intent': {
              'payment_required': false,
              'shortage': 0,
              'required_cost': 30000,
              'available_funding': 30000,
            },
          },
        };
        final repo = PromotionContractRepositoryImpl(client);

        final result = await repo.createFundingIntent(
          kind: 'internal',
          budgetRupiah: 30000,
          durationDays: 3,
          cityIds: const [],
        );

        expect(result.data!.shortage, 0);
        expect(result.data!.paymentRequired, false);
        expect(result.data!.intentId, isNull);
      },
    );

    test(
      'payment-method disclosure uses GET payment-intent/:id/payment-methods',
      () async {
        final client = _RecordingApiClient();
        client.getPayload = {
          'data': {
            'shortage_amount': 20000,
            'currency': 'IDR',
            'methods': [
              {
                'method_code': 'gopay',
                'display_name': 'GoPay',
                'service_fee_amount': 300,
                'gross_amount': 20300,
              },
            ],
          },
        };
        final repo = PromotionContractRepositoryImpl(client);

        final result = await repo.getFundingPaymentMethods('intent-1');

        expect(
          client.getPaths.last,
          '/promotions/contracts/payment-intent/intent-1/payment-methods',
        );
        expect(result.isSuccess, true);
        // Fee and gross are backend-computed values rendered verbatim.
        expect(result.data!.shortageAmount, 20000);
        expect(result.data!.currency, 'IDR');
        expect(result.data!.methods.single.methodCode, 'gopay');
        expect(result.data!.methods.single.serviceFeeAmount, 300);
        expect(result.data!.methods.single.grossAmount, 20300);
      },
    );

    test('payment initiation uses POST payment-intent/:id/pay', () async {
      final client = _RecordingApiClient();
      client.postPayload = {
        'data': {
          'payment_id': 'pay-1',
          'payment_url': 'https://snap.example/redirect',
          'gross_amount': 20300,
          'shortage': 20000,
          'intent_id': 'intent-1',
        },
      };
      final repo = PromotionContractRepositoryImpl(client);

      final result = await repo.initiateFundingPayment(
        intentId: 'intent-1',
        paymentMethodCode: 'gopay',
      );

      expect(
        client.postPaths.last,
        '/promotions/contracts/payment-intent/intent-1/pay',
      );
      // Only the canonical method code is sent — never an amount or a fee.
      expect((client.postPayloads.last as Map)['payment_method_code'], 'gopay');
      expect((client.postPayloads.last as Map).length, 1);
      expect(result.isSuccess, true);
      expect(result.data!.paymentUrl, 'https://snap.example/redirect');
      expect(result.data!.grossAmount, 20300);
    });

    test(
      'funding obligation errors preserve the backend status code',
      () async {
        final client = _RecordingApiClient();
        final repo = PromotionContractRepositoryImpl(client);

        client.getError = const NotFoundException(message: 'not found');
        final notFound = await repo.getFundingPaymentMethods('intent-404');
        expect(notFound.isError, true);
        expect(notFound.statusCode, 404);

        client.getError = const ForbiddenException(message: 'not yours');
        final forbidden = await repo.getFundingPaymentMethods('intent-403');
        expect(forbidden.isError, true);
        expect(forbidden.statusCode, 403);

        client.postError = const ConflictException(message: 'not pending');
        final conflict = await repo.initiateFundingPayment(
          intentId: 'intent-409',
          paymentMethodCode: 'gopay',
        );
        expect(conflict.isError, true);
        expect(conflict.statusCode, 409);
        expect(client.postPaths.last, contains('/intent-409/pay'));
      },
    );
  });

  group('Negative proof — legacy promotion endpoints are purged', () {
    test('canonical repository never touches legacy promotion paths', () async {
      final client = _RecordingApiClient();
      final repo = PromotionContractRepositoryImpl(client);

      client.getPayload = {
        'data': {
          'contracts': [_contractPayload()],
          'count': 1,
        },
      };
      client.postPayload = {
        'data': {'contract': _contractPayload()},
      };

      await repo.listMyContracts();
      await repo.getContract('ctr-1');
      await repo.createContract(
        kind: 'internal',
        budgetRupiah: 30000,
        durationDays: 3,
        cityIds: const ['3204'],
      );
      await repo.pauseContract('ctr-1');
      await repo.resumeContract('ctr-1');
      await repo.finalizeContract('ctr-1');

      // Canonical promotion funding path: gate → disclosure → initiation.
      client.postPayload = {
        'data': {
          'intent': {
            'payment_required': true,
            'shortage': 20000,
            'intent_id': 'intent-1',
            'billing_id': 'bill-1',
            'required_cost': 30000,
            'available_funding': 10000,
          },
        },
      };
      await repo.createFundingIntent(
        kind: 'internal',
        budgetRupiah: 30000,
        durationDays: 3,
        cityIds: const ['3204'],
      );
      client.getPayload = {
        'data': {
          'shortage_amount': 20000,
          'currency': 'IDR',
          'methods': const <Map<String, dynamic>>[],
        },
      };
      await repo.getFundingPaymentMethods('intent-1');
      await repo.initiateFundingPayment(
        intentId: 'intent-1',
        paymentMethodCode: 'gopay',
      );

      final allPaths = [...client.getPaths, ...client.postPaths];
      for (final legacy in _legacyPaths) {
        expect(
          allPaths.where((p) => p == legacy || p.startsWith(legacy)),
          isEmpty,
          reason: 'legacy path $legacy must never be called',
        );
      }
      // Promotion funding never borrows another scope's disclosure/initiation.
      for (final foreign in _foreignFundingPaths) {
        expect(
          allPaths.where((p) => p == foreign || p.startsWith(foreign)),
          isEmpty,
          reason: 'foreign funding path $foreign must never be called',
        );
      }
      // Only canonical contract paths were hit.
      expect(allPaths, isNotEmpty);
      expect(
        allPaths.where((p) => p.contains('payment-intent')).length,
        3,
        reason: 'funding uses exactly the three canonical funding paths',
      );
    });
  });
}
