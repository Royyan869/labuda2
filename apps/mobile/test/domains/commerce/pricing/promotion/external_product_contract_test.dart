import 'dart:collection';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/external_product_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/promotion_repository_impl.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_review_status.dart';

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
  String? lastPatchPath;
  String? lastDeletePath;
  Map<String, dynamic>? lastGetQuery;
  dynamic lastPostData;
  dynamic lastPatchData;

  dynamic getPayload = <String, dynamic>{};
  dynamic postPayload = <String, dynamic>{};
  dynamic patchPayload = <String, dynamic>{};
  dynamic deletePayload = <String, dynamic>{};

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
  Future<Response<T>> patch<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPatchPath = path;
    lastPatchData = data;
    return _MapResponse<T>(
      requestOptions: RequestOptions(path: path),
      data: patchPayload as Map<String, dynamic>,
      statusCode: 200,
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
    lastDeletePath = path;
    return _MapResponse<T>(
      requestOptions: RequestOptions(path: path),
      data: deletePayload as Map<String, dynamic>,
      statusCode: 200,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> _externalProductPayload({String id = 'ep-1'}) => {
  'id': id,
  'owner_user_id': 'user-1',
  'title': 'My Product',
  'description': 'A great product',
  'external_url': 'https://example.com/product',
  'normalized_external_url': 'https://example.com/product',
  'review_status': 'draft',
  'rejection_reason': null,
  'unsafe_url_flag': false,
  'submitted_at': null,
  'approved_at': null,
  'rejected_at': null,
  'hidden_at': null,
  'created_at': '2026-06-01T00:00:00Z',
  'updated_at': '2026-06-01T00:00:00Z',
  'media': <Map<String, dynamic>>[],
  'can_edit': true,
  'can_submit': true,
  'can_resubmit': false,
  'public_visible': false,
};

Map<String, dynamic> _externalProductMediaPayload({String id = 'epm-1'}) => {
  'id': id,
  'external_product_id': 'ep-1',
  'media_type': 'image',
  'storage_key': 'uploads/ep/image1.jpg',
  'url': 'https://cdn.example.com/image1.jpg',
  'thumbnail_url': 'https://cdn.example.com/image1_thumb.jpg',
  'sort_order': 0,
  'created_at': '2026-06-01T00:00:00Z',
};

void main() {
  group('ExternalProductDto mapping', () {
    test('fromJson maps all fields correctly', () {
      final json = _externalProductPayload();
      json['review_status'] = 'approved';
      json['can_edit'] = true;
      json['can_submit'] = false;
      json['can_resubmit'] = false;
      json['public_visible'] = true;
      json['approved_at'] = '2026-06-02T00:00:00Z';
      json['media'] = [_externalProductMediaPayload()];

      final dto = ExternalProductDto.fromJson(json);
      final entity = dto.toEntity();

      expect(entity.id, 'ep-1');
      expect(entity.ownerUserId, 'user-1');
      expect(entity.title, 'My Product');
      expect(entity.description, 'A great product');
      expect(entity.externalUrl, 'https://example.com/product');
      expect(entity.normalizedExternalUrl, 'https://example.com/product');
      expect(entity.reviewStatus, ExternalProductReviewStatus.approved);
      expect(entity.rejectionReason, isNull);
      expect(entity.unsafeUrlFlag, false);
      expect(entity.approvedAt, isNotNull);
      expect(entity.canEdit, true);
      expect(entity.canSubmit, false);
      expect(entity.canResubmit, false);
      expect(entity.publicVisible, true);
      expect(entity.media.length, 1);
      expect(entity.media.first.id, 'epm-1');
      expect(entity.media.first.mediaType, 'image');
      expect(entity.media.first.url, 'https://cdn.example.com/image1.jpg');
      expect(
        entity.media.first.thumbnailUrl,
        'https://cdn.example.com/image1_thumb.jpg',
      );
    });

    test('fromJson handles rejected status with reason', () {
      final json = _externalProductPayload();
      json['review_status'] = 'rejected';
      json['rejection_reason'] = 'URL does not match product';
      json['rejected_at'] = '2026-06-03T00:00:00Z';

      final entity = ExternalProductDto.fromJson(json).toEntity();

      expect(entity.reviewStatus, ExternalProductReviewStatus.rejected);
      expect(entity.rejectionReason, 'URL does not match product');
      expect(entity.hasRejectionReason, true);
      expect(entity.rejectedAt, isNotNull);
      expect(entity.isApproved, false);
    });

    test('fromJson handles hidden status', () {
      final json = _externalProductPayload();
      json['review_status'] = 'hidden';
      json['rejection_reason'] = 'Policy violation';

      final entity = ExternalProductDto.fromJson(json).toEntity();

      expect(entity.reviewStatus, ExternalProductReviewStatus.hidden);
      expect(entity.hasRejectionReason, true);
    });
  });

  group('ExternalProductMediaDto mapping', () {
    test('fromJson maps all fields correctly', () {
      final json = _externalProductMediaPayload();
      final dto = ExternalProductMediaDto.fromJson(json);
      final entity = dto.toEntity();

      expect(entity.id, 'epm-1');
      expect(entity.externalProductId, 'ep-1');
      expect(entity.mediaType, 'image');
      expect(entity.storageKey, 'uploads/ep/image1.jpg');
      expect(entity.url, 'https://cdn.example.com/image1.jpg');
      expect(entity.thumbnailUrl, 'https://cdn.example.com/image1_thumb.jpg');
      expect(entity.sortOrder, 0);
    });
  });

  group('ExternalProductReviewStatus', () {
    test('fromString parses all 6 values', () {
      expect(
        ExternalProductReviewStatus.fromString('draft'),
        ExternalProductReviewStatus.draft,
      );
      expect(
        ExternalProductReviewStatus.fromString('pending_review'),
        ExternalProductReviewStatus.pendingReview,
      );
      expect(
        ExternalProductReviewStatus.fromString('approved'),
        ExternalProductReviewStatus.approved,
      );
      expect(
        ExternalProductReviewStatus.fromString('rejected'),
        ExternalProductReviewStatus.rejected,
      );
      expect(
        ExternalProductReviewStatus.fromString('request_changes'),
        ExternalProductReviewStatus.requestChanges,
      );
      expect(
        ExternalProductReviewStatus.fromString('hidden'),
        ExternalProductReviewStatus.hidden,
      );
    });

    test('fromString defaults to draft for unknown', () {
      expect(
        ExternalProductReviewStatus.fromString('unknown_value'),
        ExternalProductReviewStatus.draft,
      );
    });
  });

  group('External product repository endpoint contract', () {
    test('all 9 endpoints use canonical paths', () async {
      final client = _RecordingApiClient();
      final repo = PromotionRepositoryImpl(client);

      // 1. POST /promotions/external-products (create draft)
      client.postPayload = _externalProductPayload(id: 'ep-new');
      await repo.createExternalProductDraft(
        title: 'New Product',
        externalUrl: 'https://example.com/new',
        description: 'Desc',
      );
      expect(client.lastPostPath, '/promotions/external-products');
      expect(client.lastPostData, {
        'title': 'New Product',
        'external_url': 'https://example.com/new',
        'description': 'Desc',
      });

      // 2. PATCH /promotions/external-products/:id (update)
      client.patchPayload = _externalProductPayload(id: 'ep-1');
      await repo.updateExternalProduct(id: 'ep-1', title: 'Updated Title');
      expect(client.lastPatchPath, '/promotions/external-products/ep-1');
      expect(client.lastPatchData, {'title': 'Updated Title'});

      // 3. POST /promotions/external-products/:id/submit
      client.postPayload = _externalProductPayload(id: 'ep-1');
      await repo.submitExternalProduct(id: 'ep-1');
      expect(client.lastPostPath, '/promotions/external-products/ep-1/submit');

      // 4. POST /promotions/external-products/:id/resubmit
      client.postPayload = _externalProductPayload(id: 'ep-1');
      await repo.resubmitExternalProduct(id: 'ep-1', note: 'Fixed URL');
      expect(
        client.lastPostPath,
        '/promotions/external-products/ep-1/resubmit',
      );
      expect(client.lastPostData, {'note': 'Fixed URL'});

      // 5. GET /promotions/external-products/:id (detail)
      client.getPayload = _externalProductPayload(id: 'ep-2');
      await repo.getExternalProduct('ep-2');
      expect(client.lastGetPath, '/promotions/external-products/ep-2');

      // 6. GET /promotions/my/external-products (list mine)
      client.getPayload = {
        'items': [_externalProductPayload()],
        'count': 1,
      };
      final listResult = await repo.listMyExternalProducts();
      expect(client.lastGetPath, '/promotions/my/external-products');
      expect(listResult.isSuccess, true);
      expect(listResult.data!.length, 1);

      // 7. POST /promotions/external-products/:id/media (attach)
      client.postPayload = _externalProductMediaPayload(id: 'epm-new');
      await repo.attachExternalProductMedia(
        externalProductId: 'ep-1',
        mediaType: 'image',
        storageKey: 'uploads/ep/new.jpg',
        url: 'https://cdn.example.com/new.jpg',
      );
      expect(client.lastPostPath, '/promotions/external-products/ep-1/media');
      expect(client.lastPostData, {
        'media_type': 'image',
        'storage_key': 'uploads/ep/new.jpg',
        'url': 'https://cdn.example.com/new.jpg',
      });

      // 8. GET /promotions/external-products/:id/media (list)
      client.getPayload = {
        'items': [_externalProductMediaPayload()],
        'count': 1,
      };
      final mediaResult = await repo.listExternalProductMedia('ep-1');
      expect(client.lastGetPath, '/promotions/external-products/ep-1/media');
      expect(mediaResult.isSuccess, true);
      expect(mediaResult.data!.length, 1);

      // 9. DELETE /promotions/external-products/:id/media/:mediaId
      client.deletePayload = <String, dynamic>{};
      await repo.deleteExternalProductMedia(
        externalProductId: 'ep-1',
        mediaId: 'epm-1',
      );
      expect(
        client.lastDeletePath,
        '/promotions/external-products/ep-1/media/epm-1',
      );
    });
  });

  group('Request DTO serialization', () {
    test('CreateExternalProductRequestDto omits null description', () {
      final dto = CreateExternalProductRequestDto(
        title: 'Test',
        externalUrl: 'https://example.com',
      );
      expect(dto.toJson(), {
        'title': 'Test',
        'external_url': 'https://example.com',
      });
    });

    test('UpdateExternalProductRequestDto omits null fields', () {
      final dto = UpdateExternalProductRequestDto(title: 'New Title');
      expect(dto.toJson(), {'title': 'New Title'});
    });

    test('AttachExternalProductMediaRequestDto includes required fields', () {
      final dto = AttachExternalProductMediaRequestDto(
        mediaType: 'video',
        storageKey: 'uploads/ep/vid.mp4',
        url: 'https://cdn.example.com/vid.mp4',
        thumbnailUrl: 'https://cdn.example.com/vid_thumb.jpg',
        sortOrder: 1,
      );
      expect(dto.toJson(), {
        'media_type': 'video',
        'storage_key': 'uploads/ep/vid.mp4',
        'url': 'https://cdn.example.com/vid.mp4',
        'thumbnail_url': 'https://cdn.example.com/vid_thumb.jpg',
        'sort_order': 1,
      });
    });
  });
}
