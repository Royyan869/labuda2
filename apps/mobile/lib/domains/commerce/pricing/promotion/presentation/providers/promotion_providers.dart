/// Promotion Providers
///
/// Riverpod providers for promotion feature
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/repositories/promotion_repository_impl.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_media.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/target_type.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/instance_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/billing_payment_intent.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_instance.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_ownership.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_package.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/repositories/promotion_repository.dart';

/// Promotion repository provider
final promotionRepositoryProvider = Provider<PromotionRepository>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return PromotionRepositoryImpl(apiClient);
});

/// Available packages provider
final availablePackagesProvider =
    FutureProvider.autoDispose<Result<List<PromotionPackage>>>((ref) async {
      final repository = ref.watch(promotionRepositoryProvider);
      return repository.listPackages(includeInactive: false);
    });

/// My ownerships provider
final myOwnershipsProvider =
    FutureProvider.autoDispose<Result<List<PromotionOwnership>>>((ref) async {
      final repository = ref.watch(promotionRepositoryProvider);
      return repository.listMyOwnerships();
    });

/// My instances provider
final myInstancesProvider =
    FutureProvider.autoDispose<Result<List<PromotionInstance>>>((ref) async {
      final repository = ref.watch(promotionRepositoryProvider);
      return repository.listMyInstances();
    });

/// Active instances for a specific fixed-price sale target
final fixedPriceSaleActivePromotionsProvider = FutureProvider.autoDispose
    .family<Result<List<PromotionInstance>>, String>((ref, targetId) async {
      final repository = ref.watch(promotionRepositoryProvider);
      final result = await repository.listMyInstances(
        status: InstanceStatus.active,
      );

      if (result.isSuccess) {
        final instances = result.data ?? [];
        final fixedPriceSaleInstances = instances
            .where((i) => i.targetId == targetId)
            .toList();
        return Result.success(fixedPriceSaleInstances);
      } else {
        return Result.error(result.error ?? 'Unknown error');
      }
    });

/// Promotion controller - for write operations
final promotionControllerProvider = Provider<PromotionController>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final repository = PromotionRepositoryImpl(apiClient);
  return PromotionController(repository);
});

/// Controller for promotion operations
class PromotionController {
  final PromotionRepositoryImpl _repository;

  PromotionController(this._repository);

  /// Purchase a promotion package
  Future<Result<String>> purchasePackage(String packageId) async {
    final result = await _repository.purchasePackage(packageId: packageId);
    if (result.isSuccess) {
      return Result.success(result.data?.billingId ?? '');
    } else {
      return Result.error(result.error ?? 'Purchase failed');
    }
  }

  /// Initiate payment for a promotion billing transaction.
  Future<Result<BillingPaymentIntent>> initiateBillingPayment(
    String billingId,
  ) async {
    return _repository.initiateBillingPayment(billingId: billingId);
  }

  /// Activate promotion on a target
  Future<Result<PromotionInstance>> activatePromotion({
    required String ownershipId,
    required TargetType targetType,
    String? targetId,
  }) async {
    return _repository.activatePromotion(
      ownershipId: ownershipId,
      targetType: targetType,
      targetId: targetId,
    );
  }

  /// Reassign a promotion instance to a new target
  Future<Result<PromotionInstance>> reassignPromotion({
    required String instanceId,
    required TargetType newTargetType,
    String? newTargetId,
  }) async {
    return _repository.reassignPromotion(
      instanceId: instanceId,
      newTargetType: newTargetType,
      newTargetId: newTargetId,
    );
  }

  /// Pause a promotion
  Future<Result<void>> pausePromotion(String instanceId) async {
    return _repository.deactivatePromotion(
      instanceId: instanceId,
      reason: 'user_paused',
    );
  }

  /// Cancel a promotion
  Future<Result<void>> cancelPromotion(String instanceId) async {
    return _repository.deactivatePromotion(
      instanceId: instanceId,
      reason: 'user_cancelled',
    );
  }

  /// Resume a paused promotion
  Future<Result<PromotionInstance>> resumePromotion(String instanceId) async {
    return _repository.resumePromotion(instanceId: instanceId);
  }

  // ============================================================
  // EXTERNAL PRODUCTS
  // ============================================================

  /// Create a draft external product
  Future<Result<ExternalProduct>> createExternalProductDraft({
    required String title,
    required String externalUrl,
    String? description,
  }) async {
    return _repository.createExternalProductDraft(
      title: title,
      externalUrl: externalUrl,
      description: description,
    );
  }

  /// Update an external product
  Future<Result<ExternalProduct>> updateExternalProduct({
    required String id,
    String? title,
    String? description,
    String? externalUrl,
  }) async {
    return _repository.updateExternalProduct(
      id: id,
      title: title,
      description: description,
      externalUrl: externalUrl,
    );
  }

  /// Submit an external product for review
  Future<Result<ExternalProduct>> submitExternalProduct({
    required String id,
    String? note,
  }) async {
    return _repository.submitExternalProduct(id: id, note: note);
  }

  /// Resubmit a rejected external product
  Future<Result<ExternalProduct>> resubmitExternalProduct({
    required String id,
    String? note,
  }) async {
    return _repository.resubmitExternalProduct(id: id, note: note);
  }

  /// Attach media to an external product
  Future<Result<ExternalProductMedia>> attachExternalProductMedia({
    required String externalProductId,
    required String mediaType,
    required String storageKey,
    required String url,
    String? thumbnailUrl,
    int? sortOrder,
  }) async {
    return _repository.attachExternalProductMedia(
      externalProductId: externalProductId,
      mediaType: mediaType,
      storageKey: storageKey,
      url: url,
      thumbnailUrl: thumbnailUrl,
      sortOrder: sortOrder,
    );
  }

  /// Delete media from an external product
  Future<Result<void>> deleteExternalProductMedia({
    required String externalProductId,
    required String mediaId,
  }) async {
    return _repository.deleteExternalProductMedia(
      externalProductId: externalProductId,
      mediaId: mediaId,
    );
  }
}

// =============================================================================
// EXTERNAL PRODUCT PROVIDERS
// =============================================================================

/// My external products provider
final myExternalProductsProvider =
    FutureProvider.autoDispose<Result<List<ExternalProduct>>>((ref) async {
      final repository = ref.watch(promotionRepositoryProvider);
      return repository.listMyExternalProducts();
    });

/// External product detail provider
final externalProductDetailProvider = FutureProvider.autoDispose
    .family<Result<ExternalProduct>, String>((ref, productId) async {
      final repository = ref.watch(promotionRepositoryProvider);
      return repository.getExternalProduct(productId);
    });

/// External product media list provider
final externalProductMediaProvider = FutureProvider.autoDispose
    .family<Result<List<ExternalProductMedia>>, String>((ref, productId) async {
      final repository = ref.watch(promotionRepositoryProvider);
      return repository.listExternalProductMedia(productId);
    });
