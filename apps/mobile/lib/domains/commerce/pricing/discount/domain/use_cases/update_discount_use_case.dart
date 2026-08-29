import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/repositories/i_discount_repository.dart';

/// Use case untuk update discount
///
/// Delegates directly to PUT /api/v1/discounts/{id}.
/// The backend is the authority for business rules (e.g. cannot change
/// code/value/applies_to once the discount has usage). The caller
/// (SellerDiscountEditScreen) holds the original discount from the seller
/// list and shows read-only fields when currentUsageCount > 0, enforcing
/// constraints in the UI before this use case is called.
///
/// No pre-fetch by ID is performed here — GET /discounts/{id} does not exist.
class UpdateDiscountUseCase {
  final IDiscountRepository _repository;

  UpdateDiscountUseCase(this._repository);

  Future<Result<Discount>> call(Discount updatedDiscount) async {
    return await _repository.updateDiscount(updatedDiscount);
  }
}
