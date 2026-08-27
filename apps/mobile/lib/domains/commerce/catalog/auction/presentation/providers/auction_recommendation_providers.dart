/// Auction Recommendation Providers
///
/// Providers for related auctions (same seller, similar items)
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/repositories/auction_repository.dart';

/// Repository provider
final auctionRepositoryProvider = Provider<AuctionRepository>((ref) {
  // This should be overridden from the main auction module
  throw UnimplementedError('auctionRepositoryProvider must be overridden');
});

/// Provider for owner's other auctions
final ownerOtherAuctionsProvider = FutureProvider.family<List<Auction>, String>(
  (ref, auctionId) async {
    final repository = ref.watch(auctionRepositoryProvider);

    // First get the auction to find the seller
    final auctionResult = await repository.getAuctionById(auctionId);
    if (auctionResult.isError || auctionResult.data == null) {
      return [];
    }

    final auction = auctionResult.data!;
    final result = await repository.getUserAuctions(
      sellerId: auction.sellerId,
      limit: 5,
    );

    if (result.isSuccess && result.data != null) {
      // Filter out the current auction
      return result.data!.where((a) => a.id != auctionId).take(4).toList();
    }
    return [];
  },
);

/// Provider for similar auctions (by variety)
final similarAuctionsProvider = FutureProvider.family<List<Auction>, String>((
  ref,
  auctionId,
) async {
  final repository = ref.watch(auctionRepositoryProvider);

  // First get the auction to find the variety
  final auctionResult = await repository.getAuctionById(auctionId);
  if (auctionResult.isError || auctionResult.data == null) {
    return [];
  }

  final auction = auctionResult.data!;
  final variety = auction.koiDetails.variety;

  // Get active auctions with same variety
  final result = await repository.getActiveAuctions(
    variety: variety,
    limit: 10,
  );

  if (result.isSuccess && result.data != null) {
    // Filter out the current auction
    return result.data!.where((a) => a.id != auctionId).take(4).toList();
  }
  return [];
});
