/// Promotion Filter Helper (Phase 4)
///
/// Utility for anti-duplicate filtering of promoted items.
/// Per Rule 3: NO ORGANIC/PROMOTED DUPLICATE LYING
library;

import 'package:labuda/features/search/search/domain/entities/search_result.dart';

/// Helper for filtering out promoted items from organic results
///
/// This ensures that items that appear as promoted don't also appear
/// in the organic results of the same batch, preventing duplicate
/// and misleading presentation.
class PromotionFilterHelper {
  /// Filter out promoted items from organic results
  ///
  /// [organicResults] - The organic search results
  /// [promotedIds] - Set of IDs that are already promoted
  ///
  /// Returns a filtered list without the promoted items
  static List<SearchResult> filterPromotedFromOrganic(
    List<SearchResult> organicResults,
    Set<String> promotedIds,
  ) {
    if (promotedIds.isEmpty) {
      return organicResults;
    }

    return organicResults.where((result) {
      // Filter out if the result's ID is in the promoted set
      return !promotedIds.contains(result.id);
    }).toList();
  }

  /// Extract promoted item IDs from a list of search results
  ///
  /// [results] - List of search results (may include promoted items)
  ///
  /// Returns a set of IDs for all items marked as promoted
  static Set<String> extractPromotedIds(List<SearchResult> results) {
    return results.where((r) => r.isPromoted).map((r) => r.id).toSet();
  }

  /// Merge promoted items with organic results, removing duplicates
  ///
  /// [promotedItems] - Items that are promoted
  /// [organicResults] - Organic search results
  ///
  /// Returns a merged list with promoted items first, followed by
  /// organic items that are NOT in the promoted list
  static List<SearchResult> mergeWithDeduplication(
    List<SearchResult> promotedItems,
    List<SearchResult> organicResults,
  ) {
    final promotedIds = extractPromotedIds(promotedItems);
    final filteredOrganic = filterPromotedFromOrganic(
      organicResults,
      promotedIds,
    );

    return [...promotedItems, ...filteredOrganic];
  }

  /// Mark search results as promoted based on promoted item IDs
  ///
  /// [results] - The search results to mark
  /// [promotedIds] - Map of promoted item IDs to their promotion instance IDs
  ///
  /// Returns a new list with results marked as promoted
  static List<SearchResult> markPromotedItems(
    List<SearchResult> results,
    Map<String, String> promotedIds, // id -> instanceId
  ) {
    return results.map((result) {
      final instanceId = promotedIds[result.id];
      if (instanceId != null) {
        return result.copyWith(
          isPromoted: true,
          promotionInstanceId: instanceId,
        );
      }
      return result;
    }).toList();
  }
}
