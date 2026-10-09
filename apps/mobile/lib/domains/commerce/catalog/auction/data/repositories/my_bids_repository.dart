import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/bidding_item_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/auction_remote_datasource.dart';

class MyBidsRepository {
  final AuctionRemoteDatasource _datasource;
  final ILoggerService _logger;

  const MyBidsRepository(this._datasource, this._logger);

  /// My Bids: auctions with an open bidding process for the user
  /// (active + waiting_settlement). Visibility is decided by the canonical
  /// backend API — this repository is projection only and must not apply
  /// business filtering of its own.
  Future<Result<List<BiddingItemDto>>> getMyBids() async {
    try {
      final items = await _datasource.getMyBidding();
      return Result.success(items);
    } catch (e) {
      _logger.error('Failed to get my bidding: $e');
      return Result.error(e.toString());
    }
  }
}
