/// Object Preview Provider
///
/// Resolves ObjectReference to live ObjectPreview data.
/// Uses existing providers (fixedPriceSaleDetailProvider, auctionDetailProvider)
/// to fetch the most up-to-date information.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'object_preview.dart';
import 'object_reference.dart';

/// Provider that resolves ObjectReference to ObjectPreview
///
/// Input: ObjectReference (type + id)
/// Output: ObjectPreview (LIVE data from entity)
///
/// This provider:
/// - Uses fixedPriceSaleDetailProvider for fixed-price sales
/// - Uses auctionDetailProvider for auctions
/// - Returns ObjectPreview with LIVE data
/// - NEVER uses preview from API
final objectPreviewProvider =
    FutureProvider.family<ObjectPreview?, ObjectReference>((
      ref,
      reference,
    ) async {
      switch (reference.type) {
        case 'fixed_price_sale':
          return _resolveFixedPriceSale(ref, reference.id);

        case 'auction':
          return _resolveAuction(ref, reference.id);

        case 'content':
        case 'profile':
        default:
          // For content/profile, return null (not supported in this phase)
          return null;
      }
    });

/// Resolve fixed-price sale to live preview
Future<ObjectPreview?> _resolveFixedPriceSale(
  Ref ref,
  String fixedPriceSaleId,
) async {
  try {
    final listingAsync = await ref.read(
      forSaleDetailProvider(fixedPriceSaleId).future,
    );

    if (listingAsync == null) return null;

    return ObjectPreview(
      id: listingAsync.forSaleId,
      type: 'fixed_price_sale',
      title: listingAsync.title,
      imageUrl: listingAsync.media.isNotEmpty
          ? listingAsync.media.first.originalUrl
          : null,
      price: listingAsync.price.toInt(),
      status: listingAsync.status.name,
    );
  } catch (_) {
    return null;
  }
}

/// Resolve auction to live preview
Future<ObjectPreview?> _resolveAuction(Ref ref, String auctionId) async {
  try {
    final auctionAsync = await ref.read(
      auctionDetailProvider(auctionId).future,
    );

    if (auctionAsync == null) return null;

    return ObjectPreview(
      id: auctionAsync.id,
      type: 'auction',
      title: auctionAsync.title,
      imageUrl: auctionAsync.media.isNotEmpty
          ? auctionAsync.media.first.originalUrl
          : null,
      price: (auctionAsync.currentBid as dynamic)?.toInt(),
      status: auctionAsync.status.name,
    );
  } catch (_) {
    return null;
  }
}
