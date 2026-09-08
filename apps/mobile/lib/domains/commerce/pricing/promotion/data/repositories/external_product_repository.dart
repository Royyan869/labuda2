/// Canonical External Product repository.
///
/// External products are a canonical promotion asset: sellers submit
/// URL-based products for admin review before they can be used as
/// promotion targets. This repository talks ONLY to the canonical
/// /promotions/external-products surface. The legacy package/ownership/
/// instance/discovery authority is purged and never referenced here.
library;

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/external_product_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_media.dart';

abstract class ExternalProductRepository {
  Future<Result<ExternalProduct>> createExternalProductDraft({
    required String title,
    required String externalUrl,
    String? description,
  });

  Future<Result<ExternalProduct>> updateExternalProduct({
    required String id,
    String? title,
    String? description,
    String? externalUrl,
  });

  Future<Result<ExternalProduct>> submitExternalProduct({
    required String id,
    String? note,
  });

  Future<Result<ExternalProduct>> resubmitExternalProduct({
    required String id,
    String? note,
  });

  Future<Result<ExternalProduct>> getExternalProduct(String id);

  Future<Result<List<ExternalProduct>>> listMyExternalProducts();

  Future<Result<ExternalProductMedia>> attachExternalProductMedia({
    required String externalProductId,
    required String mediaType,
    required String storageKey,
    required String url,
    String? thumbnailUrl,
    int? sortOrder,
  });

  Future<Result<List<ExternalProductMedia>>> listExternalProductMedia(
    String externalProductId,
  );

  Future<Result<void>> deleteExternalProductMedia({
    required String externalProductId,
    required String mediaId,
  });
}

/// HTTP implementation of [ExternalProductRepository] over the canonical
/// /promotions/external-products surface.
class ExternalProductRepositoryImpl implements ExternalProductRepository {
  final ApiClient _apiClient;

  ExternalProductRepositoryImpl(this._apiClient);

  @override
  Future<Result<ExternalProduct>> createExternalProductDraft({
    required String title,
    required String externalUrl,
    String? description,
  }) async {
    try {
      final request = CreateExternalProductRequestDto(
        title: title,
        externalUrl: externalUrl,
        description: description,
      );

      final response = await _apiClient.post(
        '/promotions/external-products',
        data: request.toJson(),
      );

      final dto = ExternalProductDto.fromJson(
        response.data as Map<String, dynamic>,
      );
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<ExternalProduct>> updateExternalProduct({
    required String id,
    String? title,
    String? description,
    String? externalUrl,
  }) async {
    try {
      final request = UpdateExternalProductRequestDto(
        title: title,
        description: description,
        externalUrl: externalUrl,
      );

      final response = await _apiClient.patch(
        '/promotions/external-products/$id',
        data: request.toJson(),
      );

      final dto = ExternalProductDto.fromJson(
        response.data as Map<String, dynamic>,
      );
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<ExternalProduct>> submitExternalProduct({
    required String id,
    String? note,
  }) async {
    try {
      final request = SubmitExternalProductRequestDto(note: note);

      final response = await _apiClient.post(
        '/promotions/external-products/$id/submit',
        data: request.toJson(),
      );

      final dto = ExternalProductDto.fromJson(
        response.data as Map<String, dynamic>,
      );
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<ExternalProduct>> resubmitExternalProduct({
    required String id,
    String? note,
  }) async {
    try {
      final request = SubmitExternalProductRequestDto(note: note);

      final response = await _apiClient.post(
        '/promotions/external-products/$id/resubmit',
        data: request.toJson(),
      );

      final dto = ExternalProductDto.fromJson(
        response.data as Map<String, dynamic>,
      );
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<ExternalProduct>> getExternalProduct(String id) async {
    try {
      final response = await _apiClient.get(
        '/promotions/external-products/$id',
      );

      final dto = ExternalProductDto.fromJson(
        response.data as Map<String, dynamic>,
      );
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<ExternalProduct>>> listMyExternalProducts() async {
    try {
      final response = await _apiClient.get('/promotions/my/external-products');

      final itemsJson = response.data['items'] as List<dynamic>? ?? [];
      final products = itemsJson
          .map(
            (json) => ExternalProductDto.fromJson(json as Map<String, dynamic>),
          )
          .map((dto) => dto.toEntity())
          .toList();

      return Result.success(products);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<ExternalProductMedia>> attachExternalProductMedia({
    required String externalProductId,
    required String mediaType,
    required String storageKey,
    required String url,
    String? thumbnailUrl,
    int? sortOrder,
  }) async {
    try {
      final request = AttachExternalProductMediaRequestDto(
        mediaType: mediaType,
        storageKey: storageKey,
        url: url,
        thumbnailUrl: thumbnailUrl,
        sortOrder: sortOrder,
      );

      final response = await _apiClient.post(
        '/promotions/external-products/$externalProductId/media',
        data: request.toJson(),
      );

      final dto = ExternalProductMediaDto.fromJson(
        response.data as Map<String, dynamic>,
      );
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<ExternalProductMedia>>> listExternalProductMedia(
    String externalProductId,
  ) async {
    try {
      final response = await _apiClient.get(
        '/promotions/external-products/$externalProductId/media',
      );

      final itemsJson = response.data['items'] as List<dynamic>? ?? [];
      final media = itemsJson
          .map(
            (json) =>
                ExternalProductMediaDto.fromJson(json as Map<String, dynamic>),
          )
          .map((dto) => dto.toEntity())
          .toList();

      return Result.success(media);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> deleteExternalProductMedia({
    required String externalProductId,
    required String mediaId,
  }) async {
    try {
      await _apiClient.delete(
        '/promotions/external-products/$externalProductId/media/$mediaId',
      );

      return Result.success(null);
    } catch (e) {
      return Result.error(e.toString());
    }
  }
}