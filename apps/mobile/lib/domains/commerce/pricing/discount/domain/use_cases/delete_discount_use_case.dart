import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/repositories/i_discount_repository.dart';

/// Use case untuk delete (soft-deactivate) discount
///
/// Business Rule: Can only delete if currentUsageCount == 0.
/// The caller (SellerDiscountListScreen) guards this in the UI by hiding
/// the Delete button when discount.currentUsageCount > 0. No pre-fetch
/// by ID is needed here — the discount object is already in the seller list.
class DeleteDiscountUseCase {
  final IDiscountRepository _repository;

  DeleteDiscountUseCase(this._repository);

  Future<Result<void>> call(String discountId) async {
    return await _repository.deleteDiscount(discountId);
  }
}
