import 'dart:collection';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
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

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    getPaths.add(path);
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
    test('list + get + create use canonical /promotions/contracts paths',
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
        'data': {
          'contract': _contractPayload(id: 'ctr-2'),
        },
      };
      final getResult = await repo.getContract('ctr-2');
      expect(client.getPaths.last, '/promotions/contracts/ctr-2');
      expect(getResult.isSuccess, true);
      expect(getResult.data!.id, 'ctr-2');

      client.postPayload = {
        'data': {
          'contract': _contractPayload(id: 'ctr-3'),
        },
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
    });

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
      client.getPayload = {
        'data': _analyticsPayload(),
      };
      final repo = CanonicalPromotionAnalyticsRepositoryImpl(client);

      final result = await repo.getDeliveryAnalytics('ctr-1');
      expect(client.getPaths.last, '/promotions/contracts/ctr-1/analytics');
      expect(result.isSuccess, true);
      expect(result.data!.contractId, 'ctr-1');
    });
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
        'data': {
          'contract': _contractPayload(),
        },
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

      final allPaths = [...client.getPaths, ...client.postPaths];
      for (final legacy in _legacyPaths) {
        expect(
          allPaths.where((p) => p == legacy || p.startsWith(legacy)),
          isEmpty,
          reason: 'legacy path $legacy must never be called',
        );
      }
      // Only canonical contract paths were hit.
      expect(allPaths, isNotEmpty);
    });
  });
}