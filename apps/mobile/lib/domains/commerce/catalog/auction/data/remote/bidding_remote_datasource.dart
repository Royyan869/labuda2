/// Bidding Remote Datasource
/// API-based data source using ApiClient
library;

import 'package:labuda/core/api/api.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/bidding_dto.dart';

/// Bidding Remote Datasource
///
/// Handles HTTP calls to Go backend API:
/// - GET /api/v1/bidding - Get user's bidding activity
class BiddingRemoteDatasource extends BaseApiRepository {
  BiddingRemoteDatasource(super.apiClient, {super.logger});

  /// Get user's bidding activity
  ///
  /// Returns all auctions where the authenticated user has placed bids,
  /// aggregated with user's bid information and derived status.
  Future<BiddingResultDto> getMyBidding() async {
    final result = await executeRequest(
      () => apiClient.get('/bidding'),
      parser: (data) => BiddingResultDto.fromJson(data as Map<String, dynamic>),
    );

    return result.fold((error) => throw Exception(error), (data) => data);
  }
}
