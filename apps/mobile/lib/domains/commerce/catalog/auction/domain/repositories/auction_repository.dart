/// Auction Repository Interface
/// Pure Dart interface - no implementation details
library;

import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_bid.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';

import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/common/types/preparation_time.dart';

/// Auction Repository Interface
///
/// Defines all auction-related operations without implementation details.
/// Implementations can use API, Firestore, or any other data source.
abstract class AuctionRepository {
  // ========== Auction CRUD Operations ==========

  /// Create new auction. A Product is created inline by the backend from
  /// the item fields below — there is no productId/forSaleId parameter.
  Future<Result<Auction>> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,

    /// Preparation-time range the seller needs after checkout (default 1–3 days).
    required PreparationTime preparationTime,

    /// Required — backend rejects creation without at least one option
    /// (auction is still a physical fish that must ship).
    required List<String> shippingSetupIds,
  });

  /// Get auction by ID
  Future<Result<Auction>> getAuctionById(String auctionId);

  /// Get multiple auctions by IDs
  Future<Result<List<Auction>>> getAuctionsByIds(List<String> auctionIds);

  /// Get active auctions with filters
  Future<Result<List<Auction>>> getActiveAuctions({
    String? variety,
    double? minSize,
    double? maxSize,
    double? maxBid,
    int limit = 20,
    String? lastAuctionId,
  });

  /// Get user's auctions (seller dashboard)
  ///
  /// `lastAuctionId` is forwarded to the API as its `cursor` query parameter,
  /// which the backend parses as an RFC3339 timestamp — the created_at of the
  /// last row the caller already holds. Despite the parameter name it is NOT
  /// an auction id. See SellerAuctionsPagerController.loadMore().
  Future<Result<List<Auction>>> getUserAuctions({
    required String sellerId,
    AuctionStatus? status,
    int limit = 20,
    String? lastAuctionId,
  });

  /// Update auction
  Future<Result<Auction>> updateAuction(
    String auctionId,
    Map<String, dynamic> updates,
  );

  /// Cancel auction (seller only)
  Future<Result<void>> cancelAuction({
    required String auctionId,
    required String sellerId,
    required String reason,
  });

  /// Relist (republish) an auction that ended with no bids, or one that
  /// lapsed before activation (seller only).
  ///
  /// The backend requires the full create-form payload (fresh timing/pricing)
  /// and rejects any auction carrying a bid, winner or bound order.
  Future<Result<void>> relistAuction({
    required String auctionId,
    required String title,
    required String description,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
  });

  // ========== Bidding Operations ==========

  /// Place bid on auction
  Future<Result<AuctionBid>> placeBid({
    required String auctionId,
    required String bidderId,
    required int amount,
  });

  /// Get auction bids
  Future<Result<List<AuctionBid>>> getAuctionBids({
    required String auctionId,
    int limit = 50,
  });

  // PASS_21C: a dedicated buyNow() RPC method was removed here — it never
  // had a working backend endpoint (POST /auctions/:id/buy-now does not
  // exist) and was confirmed unreachable from any UI. The live buy-now flow
  // (auction_detail_screen.dart._handleBuyNow) routes through the generic
  // checkout screen instead, same as any other order-creation path.

  // ========== Bid-Win Settlement ==========
  // NO claim RPC exists (purged with the claim flow). The winner completes
  // the shared Checkout: POST /pricing/preview → POST /orders (bid-win
  // branch binds the auction settlement + creates the order in one tx).

  // ========== Real-time Streams ==========
  // LIST discovery is NOT a stream — one engine (Future) with ForSale lives
  // in the presentation providers (marketplaceAuctionsProvider etc).
  // Streams are reserved for live detail surfaces only.

  /// Watch single auction (for detail screen real-time updates)
  Stream<Auction?> watchAuction(String auctionId);

  /// Watch auction bids (for real-time bid updates)
  Stream<List<AuctionBid>> watchAuctionBids(String auctionId, {int limit = 50});
}

/// Create auction request params. A Product is created inline by the
/// backend from the item fields below — there is no productId/forSaleId.
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
  final int openingBid;
  final int bidIncrement;
  final int? buyNowPrice;
  final String startMode;
  final DateTime? scheduledStartAt;
  final int durationHours;

  /// Preparation-time range the seller needs after checkout (1–3 / 4–7 / 8–15
  /// days, default 1–3).
  final PreparationTime preparationTime;

  /// Required — backend rejects creation without at least one option.
  final List<String> shippingSetupIds;

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
    required this.preparationTime,
    required this.shippingSetupIds,
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
    'preparationTime': preparationTime.toJson(),
    'shippingSetupIds': shippingSetupIds,
  };
}
