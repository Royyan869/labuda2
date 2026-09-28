/// Bidding Repository Implementation
/// API-based implementation using BiddingRemoteDatasource
library;

import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/bidding_mapper.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/remote/bidding_remote_datasource.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';

/// Bidding Repository Implementation
///
/// API implementation of BiddingRepository interface.
/// Uses BiddingRemoteDatasource for HTTP operations and BiddingMapper for conversions.
class BiddingRepositoryImpl implements BiddingRepository {
  final BiddingRemoteDatasource _datasource;
  final ILoggerService _logger;

  BiddingRepositoryImpl({
    required BiddingRemoteDatasource datasource,
    required ILoggerService logger,
  }) : _datasource = datasource,
       _logger = logger;

  @override
  Future<Result<BiddingResult>> getMyBidding() async {
    try {
      final dto = await _datasource.getMyBidding();
      final entity = BiddingMapper.toResultEntity(dto);
      return Result.success(entity);
    } catch (e) {
      _logger.error('Failed to get my bidding: $e');
      return Result.error(e.toString());
    }
  }
}
