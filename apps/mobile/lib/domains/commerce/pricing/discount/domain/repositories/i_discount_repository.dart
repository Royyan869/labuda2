import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';

/// Repository interface untuk Discount.
abstract class IDiscountRepository {
  Future<Result<Discount>> getDiscountByCode(String code);

  Future<Result<List<Discount>>> getSellerDiscounts(String sellerId);

  /// Validate a discount code against the backend authority.
  Future<Result<Discount>> validateDiscountCode(
    String code,
    int subtotalCents, {
    required String contextType,
    required String sellerId,
    String? listingId,
    String? auctionId,
  });

  Future<Result<Discount>> createDiscount(Discount discount);

  Future<Result<Discount>> updateDiscount(Discount discount);

  Future<Result<void>> deleteDiscount(String discountId);
}
