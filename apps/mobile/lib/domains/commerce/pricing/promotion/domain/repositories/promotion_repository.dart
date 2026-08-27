import 'package:labuda/core/common/result.dart';

import '../entities/external_product.dart';
import '../entities/external_product_media.dart';
import '../entities/promotion_instance.dart';
import '../entities/promotion_ownership.dart';
import '../entities/promotion_package.dart';
import '../entities/instance_status.dart';
import '../entities/ownership_status.dart';
import '../entities/target_type.dart';
import '../entities/billing_payment_intent.dart';

/// Repository interface for Promotion operations.
///
/// This repository manages three entities: PromotionPackage, PromotionOwnership, and PromotionInstance.
///
/// Business truth: Duration is ONLY stored at ownership level.
/// Instances are pointers that consume duration from their ownership.
abstract class PromotionRepository {
  // ============================================================
  // PROMOTION PACKAGES
  // ============================================================

  /// Lists all active packages.
  Future<Result<List<PromotionPackage>>> listPackages({
    bool includeInactive = false,
  });

  /// Gets a package by ID.
  Future<Result<PromotionPackage>> getPackageById(String packageId);

  // ============================================================
  // PROMOTION OWNERSHIP
  // ============================================================

  /// Lists ownerships for the current user, optionally filtered by status.
  Future<Result<List<PromotionOwnership>>> listMyOwnerships({
    OwnershipStatus? status,
    int limit = 20,
    int offset = 0,
  });

  /// Gets a single ownership by ID.
  Future<Result<PromotionOwnership>> getOwnershipById(String ownershipId);

  // ============================================================
  // PROMOTION INSTANCES
  // ============================================================

  /// Lists instances for the current user, optionally filtered by status.
  Future<Result<List<PromotionInstance>>> listMyInstances({
    InstanceStatus? status,
  });

  /// Gets a single instance by ID.
  Future<Result<PromotionInstance>> getInstanceById(String instanceId);

  /// Activates a promotion on a target.
  Future<Result<PromotionInstance>> activatePromotion({
    required String ownershipId,
    required TargetType targetType,
    String? targetId,
  });

  /// Deactivates an active promotion.
  Future<Result<void>> deactivatePromotion({
    required String instanceId,
    required String reason, // 'user_paused' or 'user_cancelled'
  });

  /// Reassigns a promotion to a new target.
  Future<Result<PromotionInstance>> reassignPromotion({
    required String instanceId,
    required TargetType newTargetType,
    String? newTargetId,
  });

  /// Resumes a paused promotion instance.
  Future<Result<PromotionInstance>> resumePromotion({
    required String instanceId,
  });

  /// Initiates a billing payment for promotion package purchase.
  Future<Result<BillingPaymentIntent>> initiateBillingPayment({
    required String billingId,
  });

  // ============================================================
  // EXTERNAL PRODUCTS
  // ============================================================

  /// Creates a draft external product.
  Future<Result<ExternalProduct>> createExternalProductDraft({
    required String title,
    required String externalUrl,
    String? description,
  });

  /// Updates an owned external product.
  Future<Result<ExternalProduct>> updateExternalProduct({
    required String id,
    String? title,
    String? description,
    String? externalUrl,
  });

  /// Submits an external product for admin review.
  Future<Result<ExternalProduct>> submitExternalProduct({
    required String id,
    String? note,
  });

  /// Resubmits a rejected external product for review.
  Future<Result<ExternalProduct>> resubmitExternalProduct({
    required String id,
    String? note,
  });

  /// Gets a single external product by ID.
  Future<Result<ExternalProduct>> getExternalProduct(String id);

  /// Lists the current user's external products.
  Future<Result<List<ExternalProduct>>> listMyExternalProducts();

  /// Attaches media metadata to an external product.
  Future<Result<ExternalProductMedia>> attachExternalProductMedia({
    required String externalProductId,
    required String mediaType,
    required String storageKey,
    required String url,
    String? thumbnailUrl,
    int? sortOrder,
  });

  /// Lists media for an external product.
  Future<Result<List<ExternalProductMedia>>> listExternalProductMedia(
    String externalProductId,
  );

  /// Deletes a media item from an external product.
  Future<Result<void>> deleteExternalProductMedia({
    required String externalProductId,
    required String mediaId,
  });
}
