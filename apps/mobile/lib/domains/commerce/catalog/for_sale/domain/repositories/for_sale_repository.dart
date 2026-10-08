/// For Sale Repository Interface
///
/// Repository for forSale - the fixed-price selling surface over Product.
library;

import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';

/// ForSale repository interface
abstract class ForSaleRepository {
  /// Get list of forSales with optional filters
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params);

  /// Get fixed-price sale by ID
  Future<Result<ForSale?>> getForSaleById(String forSaleId);

  /// Get forSales by seller ID.
  ///
  /// [includeWithdrawn] opts the owning seller into their full inventory
  /// history (active + sold + withdrawn). The backend excludes withdrawn by
  /// default and only honours this for the owner branch.
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page,
    int pageSize,
    bool includeWithdrawn,
  });

  /// Create a new forSale
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request);

  /// Update a fixed-price sale
  Future<Result<ForSale>> updateForSale(
    String forSaleId,
    UpdateForSaleRequest request,
  );

  /// Delete (withdraw) a fixed-price sale — there is no status-update path:
  /// create = publish, and live surfaces are immutable.
  Future<Result<void>> deleteForSale(String forSaleId);
}
