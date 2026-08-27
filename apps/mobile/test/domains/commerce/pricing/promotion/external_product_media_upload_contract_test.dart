/// Contract test: external product media upload path.
///
/// Verifies that after a successful S3 upload the repository serializes
/// storageKey and url as independent fields in the request body.
/// After the P12 storage-key patch, storageKey holds the raw S3 object key
/// (e.g. `images/1234_photo.jpg`) and url holds the CDN display URL.
///
/// S3 upload itself is an external service and is NOT tested here —
/// these are pure repository-layer contract tests.
library;

import 'dart:collection';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/external_product_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/promotion_repository_impl.dart';

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
  String? lastPostPath;
  String? lastGetPath;
  String? lastPatchPath;
  String? lastDeletePath;
  dynamic lastPostData;
  dynamic lastPatchData;
  Map<String, dynamic>? lastGetQuery;

  dynamic postPayload = <String, dynamic>{};
  dynamic getPayload = <String, dynamic>{};
  dynamic patchPayload = <String, dynamic>{};
  dynamic deletePayload = <String, dynamic>{};

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

Map<String, dynamic> _mediaPayload({String id = 'epm-1'}) => {
  'id': id,
  'external_product_id': 'ep-1',
  'media_type': 'image',
  'storage_key': 'https://cdn.example.com/image.jpg',
  'url': 'https://cdn.example.com/image.jpg',
  'thumbnail_url': null,
  'sort_order': 0,
  'created_at': '2026-06-01T00:00:00Z',
};

void main() {
  group('External product media upload contract', () {
    test(
      'attachExternalProductMedia sends storageKey and url to correct endpoint',
      () async {
        final client = _RecordingApiClient();
        final repo = PromotionRepositoryImpl(client);
        client.postPayload = _mediaPayload();

        const s3Key = 'images/1749600000000_photo.jpg';
        const cdnUrl = 'https://cdn.example.com/images/1749600000000_photo.jpg';

        final result = await repo.attachExternalProductMedia(
          externalProductId: 'ep-1',
          mediaType: 'image',
          storageKey: s3Key,
          url: cdnUrl,
        );

        expect(client.lastPostPath, '/promotions/external-products/ep-1/media');
        expect(client.lastPostData, {
          'media_type': 'image',
          'storage_key': s3Key,
          'url': cdnUrl,
        });
        expect(result.isSuccess, true);
      },
    );

    test(
      'attachExternalProductMedia serializes storageKey and url independently (key != url)',
      () async {
        final client = _RecordingApiClient();
        final repo = PromotionRepositoryImpl(client);
        client.postPayload = _mediaPayload();

        const s3Key = 'images/1749600000000_photo.jpg'; // raw S3 object key
        const cdnUrl =
            'https://cdn.example.com/images/1749600000000_photo.jpg'; // CDN URL

        await repo.attachExternalProductMedia(
          externalProductId: 'ep-1',
          mediaType: 'image',
          storageKey: s3Key,
          url: cdnUrl,
        );

        final body = client.lastPostData as Map<String, dynamic>;
        expect(body['storage_key'], s3Key);
        expect(body['url'], cdnUrl);
        expect(
          body['storage_key'],
          isNot(equals(body['url'])),
          reason: 'storage_key must be raw S3 key, not CDN URL',
        );
        // S3 key must not contain scheme — it is a path segment only
        expect(
          (body['storage_key'] as String).contains('://'),
          false,
          reason: 'storage_key should be object key without https:// scheme',
        );
      },
    );

    test(
      'attachExternalProductMedia omits null thumbnailUrl from body',
      () async {
        final client = _RecordingApiClient();
        final repo = PromotionRepositoryImpl(client);
        client.postPayload = _mediaPayload();

        const s3Url = 'https://cdn.example.com/images/1234_photo.jpg';

        await repo.attachExternalProductMedia(
          externalProductId: 'ep-1',
          mediaType: 'image',
          storageKey: s3Url,
          url: s3Url,
          // thumbnailUrl omitted (null)
        );

        final body = client.lastPostData as Map<String, dynamic>;
        expect(body.containsKey('thumbnail_url'), false);
      },
    );

    test(
      'attachExternalProductMedia video type sends correct mediaType',
      () async {
        final client = _RecordingApiClient();
        final repo = PromotionRepositoryImpl(client);
        client.postPayload = {
          ..._mediaPayload(),
          'media_type': 'video',
          'storage_key': 'videos/1749600000000_clip.mp4',
          'url': 'https://cdn.example.com/videos/1749600000000_clip.mp4',
        };

        const videoKey = 'videos/1749600000000_clip.mp4';
        const videoUrl =
            'https://cdn.example.com/videos/1749600000000_clip.mp4';

        await repo.attachExternalProductMedia(
          externalProductId: 'ep-2',
          mediaType: 'video',
          storageKey: videoKey,
          url: videoUrl,
        );

        expect(client.lastPostPath, '/promotions/external-products/ep-2/media');
        final body = client.lastPostData as Map<String, dynamic>;
        expect(body['media_type'], 'video');
        expect(body['storage_key'], videoKey);
        expect(body['url'], videoUrl);
        expect(body['storage_key'], isNot(equals(body['url'])));
      },
    );

    test(
      'AttachExternalProductMediaRequestDto omits null thumbnail and sortOrder',
      () {
        final dto = AttachExternalProductMediaRequestDto(
          mediaType: 'image',
          storageKey: 'https://cdn.example.com/image.jpg',
          url: 'https://cdn.example.com/image.jpg',
        );
        final json = dto.toJson();

        expect(json['media_type'], 'image');
        expect(json['storage_key'], 'https://cdn.example.com/image.jpg');
        expect(json['url'], 'https://cdn.example.com/image.jpg');
        expect(json.containsKey('thumbnail_url'), false);
        expect(json.containsKey('sort_order'), false);
      },
    );

    test(
      'AttachExternalProductMediaRequestDto includes thumbnail when provided',
      () {
        final dto = AttachExternalProductMediaRequestDto(
          mediaType: 'video',
          storageKey: 'https://cdn.example.com/video.mp4',
          url: 'https://cdn.example.com/video.mp4',
          thumbnailUrl: 'https://cdn.example.com/video_thumb.jpg',
          sortOrder: 2,
        );
        final json = dto.toJson();

        expect(
          json['thumbnail_url'],
          'https://cdn.example.com/video_thumb.jpg',
        );
        expect(json['sort_order'], 2);
      },
    );

    test('ExternalProductMediaDto fromJson maps storage_key correctly', () {
      final json = _mediaPayload();
      final dto = ExternalProductMediaDto.fromJson(json);
      final entity = dto.toEntity();

      expect(entity.storageKey, 'https://cdn.example.com/image.jpg');
      expect(entity.url, 'https://cdn.example.com/image.jpg');
      expect(entity.thumbnailUrl, isNull);
    });
  });
}
