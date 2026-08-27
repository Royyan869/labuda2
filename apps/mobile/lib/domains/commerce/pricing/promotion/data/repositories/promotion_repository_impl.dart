/// HTTP-based implementation of PromotionRepository
library;

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/external_product_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/promotion_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_media.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/instance_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/ownership_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_instance.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_ownership.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_package.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/target_type.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/billing_payment_intent.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/repositories/promotion_repository.dart';

/// HTTP-based implementation of PromotionRepository
class PromotionRepositoryImpl implements PromotionRepository {
  final ApiClient _apiClient;

  PromotionRepositoryImpl(this._apiClient);

  @override
  Future<Result<List<PromotionPackage>>> listPackages({
    bool includeInactive = false,
  }) async {
    try {
      final queryParams = <String, dynamic>{
        if (includeInactive) 'include_inactive': 'true',
      };

      final response = await _apiClient.get(
        '/promotions/packages',
        queryParameters: queryParams,
      );

      final packagesJson = response.data['packages'] as List<dynamic>? ?? [];
      final packages = packagesJson
          .map(
            (json) =>
                PromotionPackageDto.fromJson(json as Map<String, dynamic>),
          )
          .map((dto) => dto.toEntity())
          .toList();

      return Result.success(packages);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionPackage>> getPackageById(String packageId) async {
    try {
      final response = await _apiClient.get('/promotions/packages/$packageId');
      final dto = PromotionPackageDto.fromJson(response.data);
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<PromotionOwnership>>> listMyOwnerships({
    OwnershipStatus? status,
    int limit = 20,
    int offset = 0,
  }) async {
    try {
      final queryParams = <String, dynamic>{
        if (status != null) 'status': status.value,
        'page_size': limit.toString(),
        'offset': offset.toString(),
      };

      final response = await _apiClient.get(
        '/promotions/my/ownerships',
        queryParameters: queryParams,
      );

      final ownershipsJson =
          response.data['ownerships'] as List<dynamic>? ?? [];
      final ownerships = ownershipsJson
          .map(
            (json) =>
                PromotionOwnershipDto.fromJson(json as Map<String, dynamic>),
          )
          .map((dto) => dto.toEntity())
          .toList();

      return Result.success(ownerships);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionOwnership>> getOwnershipById(
    String ownershipId,
  ) async {
    try {
      final response = await _apiClient.get(
        '/promotions/ownerships/$ownershipId',
      );
      final dto = PromotionOwnershipDto.fromJson(response.data);
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<PromotionInstance>>> listMyInstances({
    InstanceStatus? status,
  }) async {
    try {
      final queryParams = <String, dynamic>{
        if (status != null) 'status': status.value,
      };

      final response = await _apiClient.get(
        '/promotions/my/instances',
        queryParameters: queryParams,
      );

      final instancesJson = response.data['instances'] as List<dynamic>? ?? [];
      final instances = instancesJson
          .map(
            (json) =>
                PromotionInstanceDto.fromJson(json as Map<String, dynamic>),
          )
          .map((dto) => dto.toEntity())
          .toList();

      return Result.success(instances);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionInstance>> getInstanceById(String instanceId) async {
    try {
      final response = await _apiClient.get(
        '/promotions/instances/$instanceId',
      );
      final dto = PromotionInstanceDto.fromJson(response.data);
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionInstance>> activatePromotion({
    required String ownershipId,
    required TargetType targetType,
    String? targetId,
  }) async {
    try {
      final request = ActivatePromotionRequestDto(
        ownershipId: ownershipId,
        targetType: targetType.value,
        targetId: targetId,
      );

      final response = await _apiClient.post(
        '/promotions/activate',
        data: request.toJson(),
      );

      final instanceJson = response.data['instance'] as Map<String, dynamic>;
      final dto = PromotionInstanceDto.fromJson(instanceJson);
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> deactivatePromotion({
    required String instanceId,
    required String reason,
  }) async {
    try {
      final request = DeactivatePromotionRequestDto(reason: reason);

      await _apiClient.post(
        '/promotions/instances/$instanceId/deactivate',
        data: request.toJson(),
      );

      return Result.success(null);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionInstance>> reassignPromotion({
    required String instanceId,
    required TargetType newTargetType,
    String? newTargetId,
  }) async {
    try {
      final request = ReassignPromotionRequestDto(
        newTargetType: newTargetType.value,
        newTargetId: newTargetId,
      );

      final response = await _apiClient.post(
        '/promotions/instances/$instanceId/reassign',
        data: request.toJson(),
      );

      final instanceJson = response.data['instance'] as Map<String, dynamic>;
      final dto = PromotionInstanceDto.fromJson(instanceJson);
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PromotionInstance>> resumePromotion({
    required String instanceId,
  }) async {
    try {
      final response = await _apiClient.post(
        '/promotions/instances/$instanceId/resume',
      );

      final instanceJson = response.data['instance'] as Map<String, dynamic>;
      final dto = PromotionInstanceDto.fromJson(instanceJson);
      return Result.success(dto.toEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  /// Purchases a promotion package
  Future<Result<PurchasePromotionPackageResponseDto>> purchasePackage({
    required String packageId,
  }) async {
    try {
      final request = PurchasePromotionPackageRequestDto(packageId: packageId);

      final response = await _apiClient.post(
        '/promotions/packages/purchase',
        data: request.toJson(),
      );

      final dto = PurchasePromotionPackageResponseDto.fromJson(response.data);
      return Result.success(dto);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<BillingPaymentIntent>> initiateBillingPayment({
    required String billingId,
  }) async {
    try {
      final response = await _apiClient.post(
        '/payments/billing',
        data: {'billing_id': billingId},
      );

      final dto = InitiateBillingPaymentResponseDto.fromJson(
        response.data as Map<String, dynamic>,
      );
      return Result.success(
        BillingPaymentIntent(
          paymentId: dto.paymentId,
          paymentUrl: dto.paymentUrl,
          grossAmount: dto.grossAmount,
          expiredAt: dto.expiredAt,
        ),
      );
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  // ============================================================
  // EXTERNAL PRODUCTS
  // ============================================================

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
