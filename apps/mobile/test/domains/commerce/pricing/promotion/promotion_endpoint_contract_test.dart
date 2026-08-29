import 'dart:collection';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/promotion_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/promotion_discovery_service.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/promotion_repository_impl.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/instance_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/ownership_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/target_type.dart';

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
  String? lastGetPath;
  String? lastPostPath;
  Map<String, dynamic>? lastGetQuery;
  dynamic lastPostData;

  dynamic getPayload = <String, dynamic>{};
  dynamic postPayload = <String, dynamic>{};

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastGetPath = path;
    lastGetQuery = queryParameters;
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
    lastPostPath = path;
    lastPostData = data;
    return _MapResponse<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as Map<String, dynamic>,
      statusCode: 200,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> _packagePayload({String id = 'pkg-1'}) => {
  'id': id,
  'name': 'Starter',
  'total_duration_hours': 24,
  'validity_window_hours': 72,
  'price_amount': 1500,
  'allowed_target_types': ['fixed_price_sale', 'auction', 'external_product'],
  'is_active': true,
  'created_at': '2026-06-01T00:00:00Z',
};

Map<String, dynamic> _ownershipPayload({String id = 'own-1'}) => {
  'id': id,
  'user_id': 'user-1',
  'package_id': 'pkg-1',
  'status': OwnershipStatus.available.value,
  'purchased_at': '2026-06-01T00:00:00Z',
  'expires_at': '2026-06-03T00:00:00Z',
  'total_duration_hours': 24,
  'consumed_duration_hours': 0,
  'created_at': '2026-06-01T00:00:00Z',
  'updated_at': '2026-06-01T00:00:00Z',
};

Map<String, dynamic> _instancePayload({String id = 'inst-1'}) => {
  'id': id,
  'ownership_id': 'own-1',
  'user_id': 'user-1',
  'target_type': TargetType.forSale.value,
  'target_id': 'fixed-price-sale-1',
  'status': InstanceStatus.active.value,
  'activated_at': '2026-06-01T00:00:00Z',
  'stopped_at': null,
  'stop_reason': null,
  'created_at': '2026-06-01T00:00:00Z',
  'updated_at': '2026-06-01T00:00:00Z',
};

Map<String, dynamic> _promotedItemsPayload() => {
  'promoted_items': [
    {
      'instance_id': 'inst-1',
      'target_type': 'for_sale',
      'target_id': 'for-sale-1',
    },
  ],
  'count': 1,
};

void main() {
  group('Promotion repository endpoint contract', () {
    test('uses canonical /promotions and /payments paths', () async {
      final client = _RecordingApiClient();
      final repo = PromotionRepositoryImpl(client);

      client.getPayload = {
        'packages': [_packagePayload()],
      };
      await repo.listPackages();
      expect(client.lastGetPath, '/promotions/packages');

      client.getPayload = _packagePayload(id: 'pkg-2');
      await repo.getPackageById('pkg-2');
      expect(client.lastGetPath, '/promotions/packages/pkg-2');

      client.getPayload = {
        'ownerships': [_ownershipPayload()],
      };
      await repo.listMyOwnerships(
        status: OwnershipStatus.available,
        limit: 10,
        offset: 20,
      );
      expect(client.lastGetPath, '/promotions/my/ownerships');
      expect(client.lastGetQuery, {
        'status': OwnershipStatus.available.value,
        'page_size': '10',
        'offset': '20',
      });

      client.getPayload = _ownershipPayload(id: 'own-2');
      await repo.getOwnershipById('own-2');
      expect(client.lastGetPath, '/promotions/ownerships/own-2');

      client.getPayload = {
        'instances': [_instancePayload()],
      };
      await repo.listMyInstances(status: InstanceStatus.active);
      expect(client.lastGetPath, '/promotions/my/instances');
      expect(client.lastGetQuery, {'status': InstanceStatus.active.value});

      client.getPayload = _instancePayload(id: 'inst-2');
      await repo.getInstanceById('inst-2');
      expect(client.lastGetPath, '/promotions/instances/inst-2');

      client.postPayload = {'instance': _instancePayload(id: 'inst-3')};
      await repo.activatePromotion(
        ownershipId: 'own-1',
        targetType: TargetType.forSale,
        targetId: 'fixed-price-sale-1',
      );
      expect(client.lastPostPath, '/promotions/activate');

      client.postPayload = {};
      await repo.deactivatePromotion(
        instanceId: 'inst-3',
        reason: 'user_paused',
      );
      expect(client.lastPostPath, '/promotions/instances/inst-3/deactivate');

      client.postPayload = {'instance': _instancePayload(id: 'inst-4')};
      await repo.reassignPromotion(
        instanceId: 'inst-3',
        newTargetType: TargetType.auction,
        newTargetId: 'auction-1',
      );
      expect(client.lastPostPath, '/promotions/instances/inst-3/reassign');

      client.postPayload = {'instance': _instancePayload(id: 'inst-5')};
      await repo.resumePromotion(instanceId: 'inst-3');
      expect(client.lastPostPath, '/promotions/instances/inst-3/resume');

      client.postPayload = {'billing_id': 'bill-1', 'amount': 1500};
      await repo.purchasePackage(packageId: 'pkg-1');
      expect(client.lastPostPath, '/promotions/packages/purchase');
      expect(client.lastPostData, {'package_id': 'pkg-1'});

      client.postPayload = {
        'payment_id': 'pay-1',
        'payment_url': 'https://pay.example.com',
        'gross_amount': 1500,
        'expired_at': '2026-06-01T01:00:00Z',
      };
      await repo.initiateBillingPayment(billingId: 'bill-1');
      expect(client.lastPostPath, '/payments/billing');
      expect(client.lastPostData, {'billing_id': 'bill-1'});
    });
  });

  group('PromotedItemDto title mapping (P2 contract patch)', () {
    test('maps legacy external_title key into externalTitle', () {
      final dto = PromotedItemDto.fromJson({
        'instance_id': 'inst-1',
        'target_type': 'external_product',
        'external_url': 'https://example.com',
        'external_title': 'Legacy Title',
      });
      expect(dto.externalTitle, 'Legacy Title');
    });

    test(
      'maps public title key into externalTitle when external_title absent',
      () {
        final dto = PromotedItemDto.fromJson({
          'instance_id': 'inst-2',
          'target_type': 'external_product',
          'external_url': 'https://example.com',
          'title': 'Public Entity Title',
        });
        expect(dto.externalTitle, 'Public Entity Title');
      },
    );

    test('prefers external_title over title when both keys are present', () {
      final dto = PromotedItemDto.fromJson({
        'instance_id': 'inst-3',
        'target_type': 'external_product',
        'external_url': 'https://example.com',
        'external_title': 'Legacy',
        'title': 'Public',
      });
      expect(dto.externalTitle, 'Legacy');
    });

    test(
      'fixed-price-sale/auction items without title keys have null externalTitle',
      () {
        final fixedPriceSale = PromotedItemDto.fromJson({
          'instance_id': 'inst-4',
          'target_type': 'fixed_price_sale',
          'target_id': 'fixed-price-sale-1',
        });
        expect(fixedPriceSale.externalTitle, isNull);

        final auction = PromotedItemDto.fromJson({
          'instance_id': 'inst-5',
          'target_type': 'auction',
          'target_id': 'auction-1',
        });
        expect(auction.externalTitle, isNull);
      },
    );
  });

  group('Promotion discovery endpoint contract', () {
    test('keeps canonical discovery paths unchanged', () async {
      final client = _RecordingApiClient();
      final service = PromotionDiscoveryService(client);

      client.getPayload = _promotedItemsPayload();
      final all = await service.getPromotedItems(limit: 5);
      expect(client.lastGetPath, '/promotions/discover');
      expect(client.lastGetQuery, {'limit': '5'});
      expect(all.count, 1);

      client.getPayload = _promotedItemsPayload();
      final byType = await service.getPromotedForSales(limit: 7);
      expect(client.lastGetPath, '/promotions/discover/for_sale');
      expect(client.lastGetQuery, {'limit': '7'});
      expect(byType.promotedItems.single.targetType, 'for_sale');

      client.getPayload = {
        'packages': [_packagePayload()],
      };
      final packages = await service.getPackages(includeInactive: true);
      expect(client.lastGetPath, '/promotions/packages');
      expect(client.lastGetQuery, {'include_inactive': 'true'});
      expect(packages.single.id, 'pkg-1');

      client.postPayload = {'billing_id': 'bill-1', 'amount': 1500};
      final purchase = await service.purchasePackage(packageId: 'pkg-1');
      expect(client.lastPostPath, '/promotions/packages/purchase');
      expect(client.lastPostData, {'package_id': 'pkg-1'});
      expect(purchase.billingId, 'bill-1');
    });
  });
}
