/// Auction Repository Interface
/// Pure Dart interface - no implementation details
library;

import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';

/// Auction Repository Interface
///
/// Defines all auction-related operations without implementation details.
/// Implementations can use API, Firestore, or any other data source.
abstract class AuctionRepository {
  // ========== Auction CRUD Operations ==========

  /// Create new auction. A Product is created inline by the backend from
  /// the item fields below — there is no productId/listingId parameter.
  Future<RepositoryResult<Auction>> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required double openingBid,
    required double bidIncrement,
    double? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    String? farmAddressId,
    AuctionLocation? location,

    /// Required — backend rejects creation without at least one option
    /// (auction is still a physical fish that must ship).
    required List<String> shippingSetupIds,
    String? preparationNote,
  });

  /// Get auction by ID
  Future<RepositoryResult<Auction>> getAuctionById(String auctionId);

  /// Get multiple auctions by IDs
  Future<RepositoryResult<List<Auction>>> getAuctionsByIds(
    List<String> auctionIds,
  );

  /// Get active auctions with filters
  Future<RepositoryResult<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
    String? lastAuctionId,
  });

  /// Get user's auctions (seller dashboard)
  Future<RepositoryResult<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
    String? lastAuctionId,
  });

  /// Update auction
  Future<RepositoryResult<Auction>> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  );

  /// Update auction status
  Future<RepositoryResult<Auction>> updateAuctionStatus({
    required String auctionId,
    required AuctionStatus status,
  });

  /// Cancel auction (seller only)
  Future<RepositoryResult<void>> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  });

  // Note: View tracking is handled by backend automatically
  // No explicit incrementViewCount needed

  // ========== Bidding Operations ==========

  /// Place bid on auction
  Future<RepositoryResult<AuctionBid>> placeBid({
    required String auctionId,
    required String bidderId,
    required double amount,
  });

  /// Get auction bids
  Future<RepositoryResult<List<AuctionBid>>> getAuctionBids({
    required String auctionId,
    int limit = 50,
  });

  // PASS_21C: a dedicated buyNow() RPC method was removed here — it never
  // had a working backend endpoint (POST /auctions/:id/buy-now does not
  // exist) and was confirmed unreachable from any UI. The live buy-now flow
  // (auction_detail_screen.dart._handleBuyNow) routes through the generic
  // checkout screen instead, same as any other order-creation path.

  // ========== Claim Operations ==========

  /// Claim auction - creates order for auction winner
  ///
  /// This is the SINGLE SOURCE OF TRUTH for creating orders from won auctions.
  /// The backend validates:
  /// - Caller is the winner
  /// - Auction is in waiting_settlement status
  /// - Claim deadline has not passed
  /// - Creates order atomically with order_id set on auction
  ///
  /// Returns order_id on success
  Future<RepositoryResult<String>> claimAuction({
    required String auctionId,
    required String addressId,
    required String shippingSetupId,
    String? discountCode,
    bool useCoins = false,
  });

  // ========== Real-time Streams ==========

  /// Watch user's auctions stream (realtime updates for seller dashboard)
  Stream<List<Auction>> watchUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 100,
  });

  /// Watch active auctions (for explore tab)
  Stream<List<Auction>> watchActiveAuctions({int limit = 50});

  /// Watch single auction (for detail screen real-time updates)
  Stream<Auction?> watchAuction(String auctionId);

  /// Watch auction bids (for real-time bid updates)
  Stream<List<AuctionBid>> watchAuctionBids(String auctionId, {int limit = 50});
}

/// Create auction request params. A Product is created inline by the
/// backend from the item fields below — there is no productId/listingId.
class CreateAuctionParams {
  final String sellerId;
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatar;
  final String title;
  final String description;
  final List<String> mediaUrls;
  final List<AuctionMediaType> mediaTypes;
  final KoiDetails koiDetails;
  final double openingBid;
  final double bidIncrement;
  final double? buyNowPrice;
  final String startMode;
  final DateTime? scheduledStartAt;
  final int durationHours;
  final String? farmAddressId;
  final AuctionLocation? location;

  /// Required — backend rejects creation without at least one option.
  final List<String> shippingSetupIds;
  final String? preparationNote;

  const CreateAuctionParams({
    required this.sellerId,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatar,
    required this.title,
    required this.description,
    required this.mediaUrls,
    required this.mediaTypes,
    required this.koiDetails,
    required this.openingBid,
    required this.bidIncrement,
    this.buyNowPrice,
    required this.startMode,
    this.scheduledStartAt,
    required this.durationHours,
    this.farmAddressId,
    this.location,
    required this.shippingSetupIds,
    this.preparationNote,
  });

  Map<String, dynamic> toMap() => {
    'sellerId': sellerId,
    if (sellerUsername != null) 'sellerUsername': sellerUsername,
    if (sellerFarmName != null) 'sellerFarmName': sellerFarmName,
    if (sellerAvatar != null) 'sellerAvatar': sellerAvatar,
    'title': title,
    'description': description,
    'mediaUrls': mediaUrls,
    'mediaTypes': mediaTypes.map((t) => t.name).toList(),
    'variety': koiDetails.variety,
    'sizeInCm': koiDetails.sizeInCm,
    'ageInMonths': koiDetails.ageInMonths,
    'gender': koiDetails.gender,
    'certificates': koiDetails.certificates,
    if (koiDetails.breeder != null) 'breeder': koiDetails.breeder,
    if (koiDetails.bloodline != null) 'bloodline': koiDetails.bloodline,
    'openingBid': openingBid,
    'bidIncrement': bidIncrement,
    if (buyNowPrice != null) 'buyNowPrice': buyNowPrice,
    'startMode': startMode,
    if (scheduledStartAt != null)
      'scheduledStartAt': scheduledStartAt!.toIso8601String(),
    'durationHours': durationHours,
    if (farmAddressId != null) 'farmAddressId': farmAddressId,
    if (location != null)
      'location': {
        'cityId': location!.cityId,
        'cityName': location!.cityName,
        'provinceId': location!.provinceId,
        'provinceName': location!.provinceName,
      },
    'shippingSetupIds': shippingSetupIds,
    if (preparationNote != null) 'preparationNote': preparationNote,
  };
}
