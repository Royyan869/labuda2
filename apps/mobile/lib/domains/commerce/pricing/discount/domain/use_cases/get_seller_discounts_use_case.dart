import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/repositories/i_discount_repository.dart';

/// Use case untuk get semua discount milik seller
class GetSellerDiscountsUseCase {
  final IDiscountRepository _repository;

  GetSellerDiscountsUseCase(this._repository);

  Future<Result<List<Discount>>> call(String sellerId) async {
    return await _repository.getSellerDiscounts(sellerId);
  }
}

// GetActiveSellerDiscountsUseCase removed: GET /discounts/seller/{id}/active
// does not exist in backend. Filter active discounts locally from
// GetSellerDiscountsUseCase result: discount.isActive && !discount.isExpired.
