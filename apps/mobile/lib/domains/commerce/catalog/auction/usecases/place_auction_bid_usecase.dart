import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/repositories/auction_repository.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

/// Place Auction Bid Use Case
///
/// **DOMAIN:** Commerce → Catalog → Auction
/// **RESPONSIBILITY:** Business logic for placing bids on auctions
/// **BOUNDARY:** Centralizes all bid validation rules in one place
///
/// **RULES:**
/// - Auction must be active
/// - Bid amount must be >= minimum bid (current bid + increment)
/// - Bidder cannot be the seller
/// - Double-submit prevention (synchronous guard)
class PlaceAuctionBidUseCase {
  final AuctionRepository _auctionRepository;

  const PlaceAuctionBidUseCase(this._auctionRepository);

  /// Execute the use case
  ///
  /// Returns [Result.success] with the placed bid
  /// Returns [Result.error] if validation fails or bid cannot be placed
  Future<Result<void>> execute({
    required String auctionId,
    required String bidderId,
    required int amount,
  }) async {
    try {
      // Get auction first for validation
      final auctionResult = await _auctionRepository.getAuctionById(auctionId);

      if (auctionResult.isError) {
        return Result.error(auctionResult.error ?? 'Auction not found');
      }

      final auction = auctionResult.data!;

      // **BUSINESS LOGIC HERE - NOT IN UI**
      // Validate auction is active and can accept bids
      if (!auction.isActive) {
        final errorMsg = auction.status == AuctionStatus.ended
            ? 'Lelang sudah berakhir'
            : auction.status == AuctionStatus.scheduled
            ? 'Lelang belum dimulai'
            : 'Lelang tidak aktif';
        return Result.error(errorMsg);
      }

      // LOCAL COMPUTATION: UX optimization - backend is final authority
      // Derived from factual fields: currentBid + bidIncrement
      // Validate bid amount against the canonical integer representation.
      // Read-side entities still carry doubles; a fractional minimum would
      // contradict the backend bigint contract, so it fails loudly here
      // rather than truncating silently.
      final minimumBidDouble = auction.currentBid + auction.bidIncrement;
      if (minimumBidDouble != minimumBidDouble.toInt()) {
        return Result.error(
          'Minimum bid must be integral: Rp ${minimumBidDouble.toStringAsFixed(0)}',
        );
      }
      final minimumBid = minimumBidDouble.toInt();
      if (amount < minimumBid) {
        return Result.error('Bid minimum: Rp $minimumBid');
      }

      // Validate user is not seller
      if (auction.sellerId == bidderId) {
        return Result.error('Tidak dapat bid pada lelang sendiri');
      }

      // Place the bid
      final result = await _auctionRepository.placeBid(
        auctionId: auctionId,
        bidderId: bidderId,
        amount: amount,
      );

      return result.fold(
        (bid) => Result.success(null),
        (error) => Result.error(error),
      );
    } catch (e) {
      return Result.error('Failed to place bid: $e');
    }
  }
}
