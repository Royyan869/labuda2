/// Auction Remote Datasource
/// API-based data source using ApiClient
library;

import 'package:labuda/core/api/api.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';

/// Auction Remote Datasource
///
/// Handles HTTP calls to Go backend API:
/// - GET/POST /api/v1/auctions - List/Create auctions
/// - GET/PUT /api/v1/auctions/:id - Read/Update operations
/// - POST /api/v1/auctions/:id/bid - Place bid
/// - POST /api/v1/auctions/:id/cancel - Cancel auction
/// - POST /api/v1/auctions/:id/claim - Winner claim
class AuctionRemoteDatasource extends BaseApiRepository {
  AuctionRemoteDatasource(super.apiClient, {super.logger});

  // ========== Auction CRUD Operations ==========

  /// Get list of auctions with filters
  Future<List<AuctionDto>> getAuctions({
    String? status,
    String? sellerId,
    int limit = 20,
    String? cursor,
  }) async {
    final result = await executeListRequest(
      () => apiClient.get(
        '/auctions',
        queryParameters: {
          'status': ?status,
          'seller_id': ?sellerId,
          'limit': limit,
          'cursor': ?cursor,
        },
      ),
      itemParser: (json) => AuctionDto.fromJson(json),
    );

    return result.fold((error) => throw Exception(error), (data) => data);
  }

  /// Get auction by ID
  Future<AuctionDto> getAuctionById(String auctionId) async {
    final result = await executeRequest(
      () => apiClient.get('/auctions/$auctionId'),
      parser: (data) => AuctionDto.fromJson(data as Map<String, dynamic>),
    );

    return result.fold((error) => throw Exception(error), (data) => data);
  }

  /// Get multiple auctions by IDs
  ///
  /// Unsupported: backend does not expose /auctions/batch.
  Future<List<AuctionDto>> getAuctionsByIds(List<String> auctionIds) async {
    throw UnsupportedError(
      'POST /auctions/batch is not supported by backend contract.',
    );
  }

  /// Create new auction
  Future<AuctionDto> createAuction(CreateAuctionDto request) async {
    final result = await executeRequest(
      () => apiClient.post('/auctions', data: request.toJson()),
      parser: (data) => AuctionDto.fromJson(data as Map<String, dynamic>),
    );

    if (result.isError) {
      throw StructuredApiException(
        message: result.error ?? 'Failed to create auction',
        code: result.errorCode,
        details: result.errorDetails,
      );
    }
    return result.data!;
  }

  /// Update auction
  Future<AuctionDto> updateAuction(
    String auctionId,
    UpdateAuctionDto request,
  ) async {
    final result = await executeRequest(
      () => apiClient.put('/auctions/$auctionId', data: request.toJson()),
      parser: (data) => AuctionDto.fromJson(data as Map<String, dynamic>),
    );

    return result.fold((error) => throw Exception(error), (data) => data);
  }

  // ========== Auction Lifecycle Operations ==========

  /// Schedule auction
  Future<void> scheduleAuction(String auctionId) async {
    final result = await executeVoidRequest(
      () => apiClient.post('/auctions/$auctionId/schedule'),
    );

    if (result.isError) {
      throw StructuredApiException(
        message: result.error ?? 'Failed to schedule auction',
        code: result.errorCode,
        details: result.errorDetails,
      );
    }
  }

  /// Cancel auction
  Future<void> cancelAuction(String auctionId, CancelAuctionDto request) async {
    final result = await executeVoidRequest(
      () =>
          apiClient.post('/auctions/$auctionId/cancel', data: request.toJson()),
    );

    return result.fold((error) => throw Exception(error), (data) => data);
  }

  // ========== Bidding Operations ==========

  /// Place bid on auction
  ///
  /// Returns a [RepositoryResult] so the call site can read the API error
  /// code via `result.errorCode` (e.g. `EMAIL_VERIFICATION_REQUIRED`)
  /// instead of pattern-matching on the error string.
  Future<RepositoryResult<BidDto>> placeBid(
    String auctionId,
    PlaceBidDto request,
  ) async {
    final result = await executeRequest(
      () => apiClient.post('/auctions/$auctionId/bid', data: request.toJson()),
      parser: (data) => BidDto.fromJson(data as Map<String, dynamic>),
    );

    if (result.isError) {
      return RepositoryResult.error(
        result.error ?? 'Unknown error',
        code: result.errorCode,
        details: result.errorDetails,
      );
    }
    return RepositoryResult.success(result.data!);
  }

  /// Get bid history for auction
  Future<List<BidDto>> getBidHistory(
    String auctionId, {
    int page = 1,
    int pageSize = 50,
  }) async {
    final result = await executeListRequest(
      () => apiClient.get(
        '/auctions/$auctionId/bids',
        queryParameters: {'page': page, 'page_size': pageSize},
      ),
      itemParser: (json) => BidDto.fromJson(json),
    );

    return result.fold((error) => throw Exception(error), (data) => data);
  }

  /// Get current highest bid info
  Future<CurrentBidDto> getCurrentBid(String auctionId) async {
    final auction = await getAuctionById(auctionId);
    return CurrentBidDto(
      auctionId: auction.id,
      currentHighestBid: auction.currentHighestBid,
      highestBidderId: auction.highestBidderId,
      minimumBid: auction.minimumBid,
      totalBids: auction.totalBids,
      timeRemainingSeconds: auction.timeRemainingSeconds,
      endTime: auction.endTime,
      isExtended: false,
      status: auction.status,
    );
  }

  // ========== Claim Operations ==========

  /// Claim auction - creates order for auction winner
  ///
  /// POST /api/v1/auctions/:id/claim
  ///
  /// This is the SINGLE SOURCE OF TRUTH for creating orders from won auctions.
  /// The backend validates:
  /// - Caller is the winner
  /// - Auction is in waiting_settlement status
  /// - Claim deadline has not passed
  /// - Creates order atomically with order_id set on auction
  ///
  /// Returns order_id on success
  Future<String> claimAuction(
    String auctionId, {
    required String addressId,
    required String shippingSetupId,
    String? discountCode,
    bool useCoins = false,
  }) async {
    final result = await executeRequest(
      () => apiClient.post(
        '/auctions/$auctionId/claim',
        data: {
          'address_id': addressId,
          'shipping_setup_id': shippingSetupId,
          if (discountCode != null) 'discount_code': discountCode,
          if (useCoins) 'use_coins': true,
        },
      ),
      parser: (data) {
        // Response: {"message": "...", "data": {"order_id": "..."}}
        final responseData = data['data'] as Map<String, dynamic>?;
        return responseData?['order_id'] as String? ??
            (data['order_id'] as String?);
      },
    );

    if (result.isError) {
      throw StructuredApiException(
        message: result.error ?? 'Failed to claim auction',
        code: result.errorCode,
        details: result.errorDetails,
      );
    }
    return result.data ?? '';
  }
}
