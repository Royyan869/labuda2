/// Explore promotion providers
///
/// Mobile-side Explore promo source that reuses the canonical promotion
/// discovery service. Covers fixed-price-sale and auction promoted items for Explore tabs.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/promotion_discovery_service.dart';

/// Canonical promotion discovery service for Explore.
final explorePromotionDiscoveryServiceProvider =
    Provider<PromotionDiscoveryService>((ref) {
      final apiClient = ref.watch(apiClientProvider);
      return PromotionDiscoveryService(apiClient);
    });

/// Promoted fixed-price-sale IDs for Explore.
///
/// Returns only fixed-price-sale promotion target IDs; fetched from the discovery service.
final explorePromotedFixedPriceSaleIdsProvider =
    FutureProvider.autoDispose<List<String>>((ref) async {
      final service = ref.watch(explorePromotionDiscoveryServiceProvider);
      final response = await service.getPromotedFixedPriceSales(limit: 2);

      return response.promotedItems
          .where((item) => item.isFixedPriceSale && item.targetId != null)
          .map((item) => item.targetId!)
          .toList();
    });

/// Promoted auction IDs for Explore.
///
/// Returns only auction promotion target IDs; fetched from the discovery service.
final explorePromotedAuctionIdsProvider =
    FutureProvider.autoDispose<List<String>>((ref) async {
      final service = ref.watch(explorePromotionDiscoveryServiceProvider);
      final response = await service.getPromotedAuctions(limit: 2);

      return response.promotedItems
          .where((item) => item.isAuction && item.targetId != null)
          .map((item) => item.targetId!)
          .toList();
    });
