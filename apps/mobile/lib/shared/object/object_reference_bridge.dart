/// Object Reference Bridge
///
/// Safe conversion layer between ShareReference and ObjectReference.
///
/// PURPOSE:
/// - Eliminate dual path logic while maintaining dual structure temporarily
/// - Enable gradual migration from ShareReference to ObjectReference
/// - Provide pure mapping without business logic
///
/// USAGE:
/// - ShareReference → ObjectReference: Direct conversion (no API call)
/// - ObjectReference → ShareReference: Requires live data fetch (uses ObjectPreview)
///
/// CONTRACT:
/// - Pure mapping functions - no business logic
/// - Type-safe conversions with proper null handling
/// - Bridge layer isolates migration complexity

library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview.dart';
import 'package:labuda/shared/object/object_preview_provider.dart'
    as individual;
import 'package:labuda/shared/object/object_reference.dart';
import 'package:labuda/shared/object/object_preview_batch_provider.dart'
    as batch;

// ============================================================================
// PURE MAPPING HELPERS (No Business Logic)
// ============================================================================

/// Convert ShareReference to ObjectReference (pure mapping)
///
/// This is a direct conversion with no API call required.
/// ShareReference contains the canonical targetType + targetId.
ObjectReference shareToObjective(ShareReference shareRef) {
  return ObjectReference(type: shareRef.objectType, id: shareRef.targetId);
}

/// Convert ObjectReference to ShareReference (requires live data)
///
/// **⚠️ WARNING:**
/// This method should ONLY be used for:
/// - UI fallback scenarios
/// - Legacy compatibility layers
///
/// **DO NOT use for:**
/// - Main data flow
/// - Rendering logic
/// - Business logic
///
/// **Reason:** ShareReference is transport-layer ONLY (snapshot data).
/// This function exists to bridge ObjectReference → ShareReference for
/// components that haven't been migrated to ObjectReference yet.
///
/// **Preferred approach:** Use ObjectReference directly with ObjectPreview
/// for rendering. The ObjectReference → ObjectPreview → UI flow ensures
/// live data and avoids snapshot-driven architecture.
///
/// **Technical note:** This function fetches live data from the backend.
/// Use only when you need the full ShareReference with preview data.
Future<ShareReference?> buildFallbackShareReference(
  Ref ref,
  ObjectReference objRef,
) async {
  // Fetch live preview data
  final preview = await ref.read(
    individual.objectPreviewProvider(objRef).future,
  );

  if (preview == null) return null;

  // Convert to ShareReference based on type
  switch (objRef.type) {
    case 'fixed_price_sale':
      return ShareReference.forSale(
        forSaleId: objRef.id,
        title: preview.title,
        imageUrl: preview.imageUrl,
        isAvailable: _isFixedPriceSaleAvailable(preview.status),
        isSold: _isFixedPriceSaleSold(preview.status),
        isDeleted: _isDeleted(preview.status),
      );

    case 'auction':
      return ShareReference.auction(
        auctionId: objRef.id,
        title: preview.title,
        imageUrl: preview.imageUrl,
        isAvailable: _isAuctionAvailable(preview.status),
        isClosed: _isAuctionClosed(preview.status),
        isDeleted: _isDeleted(preview.status),
      );

    case 'content':
      return ShareReference.content(
        contentId: objRef.id,
        title: preview.title,
        imageUrl: preview.imageUrl,
        isAvailable: true, // Content doesn't have availability
        isDeleted: _isDeleted(preview.status),
      );

    case 'profile':
      return ShareReference.profile(
        profileId: objRef.id,
        name: preview.title,
        avatarUrl: preview.imageUrl,
        isAvailable: true, // Profile doesn't have availability
        isDeleted: _isDeleted(preview.status),
      );

    default:
      return null;
  }
}

/// Convert multiple ObjectReferences to ShareReferences (batch)
///
/// More efficient than calling buildFallbackShareReference multiple times.
/// Uses the batch provider for fetching.
Future<List<ShareReference?>> objectsToShareRefs(
  Ref ref,
  List<ObjectReference> objRefs,
) async {
  // Import batch provider locally to avoid circular dependency
  final batchProvider = objectPreviewBatchProviderProvider;
  final previews = await ref.read(batchProvider(objRefs).future);

  return objRefs.map((objRef) {
    final key = '${objRef.type}:${objRef.id}';
    final preview = previews[key];

    if (preview == null) return null;

    return _objectPreviewToShareRef(objRef, preview);
  }).toList();
}

// ============================================================================
// INTERNAL HELPERS
// ============================================================================

/// Convert ObjectPreview to ShareReference (internal helper)
ShareReference? _objectPreviewToShareRef(
  ObjectReference objRef,
  ObjectPreview preview,
) {
  switch (objRef.type) {
    case 'fixed_price_sale':
      return ShareReference.forSale(
        forSaleId: objRef.id,
        title: preview.title,
        imageUrl: preview.imageUrl,
        isAvailable: _isFixedPriceSaleAvailable(preview.status),
        isSold: _isFixedPriceSaleSold(preview.status),
        isDeleted: _isDeleted(preview.status),
      );

    case 'auction':
      return ShareReference.auction(
        auctionId: objRef.id,
        title: preview.title,
        imageUrl: preview.imageUrl,
        isAvailable: _isAuctionAvailable(preview.status),
        isClosed: _isAuctionClosed(preview.status),
        isDeleted: _isDeleted(preview.status),
      );

    case 'content':
      return ShareReference.content(
        contentId: objRef.id,
        title: preview.title,
        imageUrl: preview.imageUrl,
        isAvailable: true,
        isDeleted: _isDeleted(preview.status),
      );

    case 'profile':
      return ShareReference.profile(
        profileId: objRef.id,
        name: preview.title,
        avatarUrl: preview.imageUrl,
        isAvailable: true,
        isDeleted: _isDeleted(preview.status),
      );

    default:
      return null;
  }
}

// Status mapping helpers

bool _isFixedPriceSaleAvailable(String status) {
  return status == 'available';
}

bool _isFixedPriceSaleSold(String status) {
  return status == 'sold';
}

bool _isAuctionAvailable(String status) {
  return status == 'active' || status == 'scheduled';
}

bool _isAuctionClosed(String status) {
  return status == 'ended' ||
      status == 'cancelled' ||
      status == 'waiting_settlement';
}

bool _isDeleted(String status) {
  return status == 'deleted';
}

// ============================================================================
// BATCH PROVIDER ACCESS
// ============================================================================

/// Access the batch provider for batch conversion
final objectPreviewBatchProviderProvider = batch.objectPreviewBatchProvider;
