/// Bidding Repository Interface
/// Pure Dart interface - no implementation details
library;

import 'package:labuda/domains/commerce/catalog/auction/domain/entities/bidding_item.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';

/// Bidding Repository Interface
///
/// Defines all bidding-related operations without implementation details.
/// Implementations can use API, Firestore, or any other data source.
abstract class BiddingRepository {
  /// Get user's bidding activity
  ///
  /// Returns all auctions where the user has placed bids,
  /// aggregated with user's bid information and derived status.
  Future<RepositoryResult<BiddingResult>> getMyBidding();
}
