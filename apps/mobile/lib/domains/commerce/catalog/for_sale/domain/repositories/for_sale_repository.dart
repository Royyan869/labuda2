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

  /// Get multiple forSales by IDs
  Future<Result<List<ForSale>>> getForSalesByIds(List<String> forSaleIds);

  /// Get forSales by seller ID
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page,
    int pageSize,
  });

  /// Create a new forSale
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request);

  /// Update a fixed-price sale
  Future<Result<ForSale>> updateForSale(
    String forSaleId,
    UpdateForSaleRequest request,
  );

  /// Delete a fixed-price sale
  Future<Result<void>> deleteForSale(String forSaleId);

  /// Update fixed-price sale status
  Future<Result<ForSale>> updateForSaleStatus(
    String forSaleId,
    ForSaleStatus status,
  );
}
