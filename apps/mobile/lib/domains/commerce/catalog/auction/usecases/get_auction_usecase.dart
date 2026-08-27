import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/repositories/auction_repository.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

/// Get Auction Use Case
///
/// **DOMAIN:** Commerce → Catalog → Auction
/// **RESPONSIBILITY:** Business logic for fetching auction details
/// **BOUNDARY:** Provides single point of access for auction data
class GetAuctionUseCase {
  final AuctionRepository _auctionRepository;

  const GetAuctionUseCase(this._auctionRepository);

  /// Execute the use case
  ///
  /// Returns [Result.success] with the auction
  /// Returns [Result.error] if auction not found
  Future<Result<Auction>> execute(String auctionId) async {
    try {
      final result = await _auctionRepository.getAuctionById(auctionId);

      return result.fold((auction) {
        // **BUSINESS LOGIC HERE - NOT IN UI**
        // Additional business rules can be added here
        // For example: Validate auction is visible, not deleted, etc.
        return Result.success(auction);
      }, (error) => Result.error(error));
    } catch (e) {
      return Result.error('Failed to get auction: $e');
    }
  }

  /// Get multiple auctions
  Future<Result<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
  }) async {
    try {
      final result = await _auctionRepository.getActiveAuctions(
        variety: variety,
        minSize: minSize,
        maxSize: maxSize,
        maxBid: maxBid,
        limit: limit,
      );

      return result.fold(
        (auctions) => Result.success(auctions),
        (error) => Result.error(error),
      );
    } catch (e) {
      return Result.error('Failed to get active auctions: $e');
    }
  }

  /// Get user's auctions (seller dashboard)
  Future<Result<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
  }) async {
    try {
      final result = await _auctionRepository.getUserAuctions(
        sellerId: sellerId,
        status: status,
        limit: limit,
      );

      return result.fold(
        (auctions) => Result.success(auctions),
        (error) => Result.error(error),
      );
    } catch (e) {
      return Result.error('Failed to get user auctions: $e');
    }
  }
}
