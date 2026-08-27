/// Object Preview Batch Provider
///
/// Resolves multiple ObjectReferences to live ObjectPreview data in a single batch.
/// Eliminates N+1 query problem by grouping references by type and fetching in batches.
///
/// HARDENED VERSION:
/// - Stable cache key using ObjectReferenceListKey
/// - Automatic deduplication of references
/// - Partial failure handling with fallback to individual providers
/// - Selective keepAlive for screen lifecycle
/// - Comprehensive logging
library;

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/shared/object/object_preview.dart';
import 'package:labuda/shared/object/object_preview_provider.dart'
    as individual;
import 'package:labuda/shared/object/object_reference.dart';

/// Batch object preview provider
///
/// Input: `List<ObjectReference>` (multiple references to resolve)
/// Output: `Map<String, ObjectPreview>` (key: "type:id", value: live preview)
///
/// This provider:
/// - Creates stable cache key using ObjectReferenceListKey
/// - Groups references by type (fixed-price sales, auctions)
/// - Fetches all fixed-price sales in one batch call
/// - Fetches all auctions in one batch call
/// - Handles partial failures with fallback to individual providers
/// - Returns a map for O(1) lookup by reference
/// - Reduces N API calls to 2-3 calls (listings + auctions)
final objectPreviewBatchProvider = FutureProvider.family
    .autoDispose<Map<String, ObjectPreview>, List<ObjectReference>>((
      ref,
      references,
    ) async {
      if (references.isEmpty) {
        return {};
      }

      // STEP 1: Create stable cache key with deduplication
      final listKey = ObjectReferenceListKey.from(references);

      // STEP 2: Log batch operation
      _logBatchStart(listKey);

      // STEP 3: Group references by type
      final grouped = _groupByType(listKey.refs);

      // STEP 4: Fetch each type in batch with partial failure handling
      final results = <String, ObjectPreview>{};
      final fallbackRefs = <ObjectReference>[];

      // Fetch all fixed-price sales in one batch
      if (grouped['fixed_price_sale'] != null &&
          grouped['fixed_price_sale']!.isNotEmpty) {
        final fixedPriceSaleResult = await _fetchFixedPriceSaleBatch(
          ref,
          grouped['fixed_price_sale']!,
        );
        results.addAll(fixedPriceSaleResult.success);

        // Track failures for fallback
        fallbackRefs.addAll(fixedPriceSaleResult.failed);
      }

      // Fetch all auctions in one batch
      if (grouped['auction'] != null && grouped['auction']!.isNotEmpty) {
        final auctionResult = await _fetchAuctionsBatch(
          ref,
          grouped['auction']!,
        );
        results.addAll(auctionResult.success);

        // Track failures for fallback
        fallbackRefs.addAll(auctionResult.failed);
      }

      // STEP 5: Fallback to individual providers for failed items
      if (fallbackRefs.isNotEmpty) {
        _logFallback(fallbackRefs);
        await _fetchIndividual(ref, fallbackRefs, results);
      }

      // STEP 6: Log completion
      _logBatchComplete(listKey, results.length, fallbackRefs.length);

      // STEP 7: Keep alive for screen lifecycle
      ref.keepAlive();

      return results;
    }, name: 'objectPreviewBatchProvider');

/// Result of batch fetch with partial failure handling
class _BatchFetchResult {
  final Map<String, ObjectPreview> success;
  final List<ObjectReference> failed;

  _BatchFetchResult({required this.success, required this.failed});
}

/// Group references by type
Map<String, List<ObjectReference>> _groupByType(
  List<ObjectReference> references,
) {
  final grouped = <String, List<ObjectReference>>{};

  for (final ref in references) {
    if (!grouped.containsKey(ref.type)) {
      grouped[ref.type] = [];
    }
    grouped[ref.type]!.add(ref);
  }

  return grouped;
}

/// Fetch multiple fixed-price sales in batch and convert to ObjectPreview
/// Returns partial result with success/failure tracking
Future<_BatchFetchResult> _fetchFixedPriceSaleBatch(
  Ref ref,
  List<ObjectReference> refs,
) async {
  final fixedPriceSaleIds = refs.map((ref) => ref.id).toSet().toList();
  final success = <String, ObjectPreview>{};
  final failed = <ObjectReference>[];

  try {
    // Call the repository batch method
    final listingRepository = ref.read(forSaleRepositoryProvider);
    final result = await listingRepository.getForSalesByIds(fixedPriceSaleIds);

    return result.fold(
      (error) {
        // Batch failed completely - mark all for fallback
        _logBatchFailure(
          'fixed_price_sale',
          fixedPriceSaleIds.length,
          error.toString(),
        );
        return _BatchFetchResult(success: success, failed: refs);
      },
      (listings) {
        // Track which IDs we got successfully
        final foundIds = <String>{};

        for (final listing in listings) {
          final key = 'fixed_price_sale:${listing.forSaleId}';
          foundIds.add(listing.forSaleId);
          success[key] = ObjectPreview(
            id: listing.forSaleId,
            type: 'fixed_price_sale',
            title: listing.title,
            imageUrl: listing.media.isNotEmpty
                ? listing.media.first.originalUrl
                : null,
            price: listing.price.toInt(),
            status: listing.status.name,
          );
        }

        // Find missing listings for fallback
        for (final ref in refs) {
          if (!foundIds.contains(ref.id)) {
            failed.add(ref);
          }
        }

        _logBatchSuccess('fixed_price_sale', listings.length, failed.length);

        return _BatchFetchResult(success: success, failed: failed);
      },
    );
  } catch (e) {
    // Unexpected error - mark all for fallback
    _logBatchError('fixed_price_sale', e.toString());
    return _BatchFetchResult(success: success, failed: refs);
  }
}

/// Fetch multiple auctions in batch and convert to ObjectPreview
/// Returns partial result with success/failure tracking
///
/// PASS_21C: the backend has no /auctions/batch endpoint —
/// AuctionRepository.getAuctionsByIds always throws UnsupportedError (see
/// auction_contract_p1_test.dart). Calling it here would always fail and
/// fall through to the try/catch below, wasting a guaranteed-failing call
/// (plus its stack-trace logging) on every batch preview resolution. Skip
/// straight to the individual-provider fallback instead.
Future<_BatchFetchResult> _fetchAuctionsBatch(
  Ref ref,
  List<ObjectReference> refs,
) async {
  return _BatchFetchResult(success: const {}, failed: refs);
}

/// Fetch failed items individually using individual providers
Future<void> _fetchIndividual(
  Ref ref,
  List<ObjectReference> refs,
  Map<String, ObjectPreview> results,
) async {
  for (final refItem in refs) {
    try {
      final preview = await ref.read(
        individual.objectPreviewProvider(refItem).future,
      );
      if (preview != null) {
        final key = '${refItem.type}:${refItem.id}';
        results[key] = preview;
      }
    } catch (e) {
      // Individual fetch also failed - skip this item
      _logIndividualError(refItem, e.toString());
    }
  }
}

/// Logging helpers

void _logBatchStart(ObjectReferenceListKey listKey) {
  if (kDebugMode) {
    print(
      '[BatchResolver] Starting batch fetch for ${listKey.refs.length} unique refs',
    );
    print('[BatchResolver] Cache key: ${listKey.cacheKey}');
  }
}

void _logBatchSuccess(String type, int successCount, int failureCount) {
  if (kDebugMode) {
    print(
      '[BatchResolver] $type batch: $successCount success, $failureCount fallback',
    );
  }
}

void _logBatchFailure(String type, int count, String error) {
  if (kDebugMode) {
    print(
      '[BatchResolver] $type batch FAILED completely ($count items): $error',
    );
  }
}

void _logBatchError(String type, String error) {
  if (kDebugMode) {
    print('[BatchResolver] $type batch ERROR: $error');
  }
}

void _logFallback(List<ObjectReference> refs) {
  if (kDebugMode) {
    print(
      '[BatchResolver] Falling back to individual providers for ${refs.length} items',
    );
  }
}

void _logIndividualError(ObjectReference ref, String error) {
  if (kDebugMode) {
    print(
      '[BatchResolver] Individual fetch failed for ${ref.type}:${ref.id}: $error',
    );
  }
}

void _logBatchComplete(
  ObjectReferenceListKey listKey,
  int successCount,
  int fallbackCount,
) {
  if (kDebugMode) {
    print(
      '[BatchResolver] Batch complete: $successCount total, $fallbackCount from fallback',
    );
    print(
      '[BatchResolver] API calls saved: ${listKey.refs.length - fallbackCount - 2}',
    );
  }
}

/// Helper to get cache key from ObjectReference
String getCacheKey(ObjectReference reference) {
  return '${reference.type}:${reference.id}';
}
