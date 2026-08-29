import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_validation_result.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/repositories/i_discount_repository.dart';

/// Use case untuk validasi discount code.
///
/// Backend authority: delegates validation to POST /api/v1/discounts/validate.
/// The backend validates code existence, active status, time window, usage
/// limits, minimum purchase, seller ownership, and target applicability.
///
/// Mobile no longer calculates discount amounts locally. It only surfaces
/// backend validation state.
class ValidateDiscountUseCase {
  final IDiscountRepository _repository;

  ValidateDiscountUseCase(this._repository);

  Future<Result<DiscountValidationResult>> call(
    ValidateDiscountParams params,
  ) async {
    try {
      final result = await _repository.validateDiscountCode(
        params.code,
        params.subtotal.round(),
        contextType: params.contextType,
        sellerId: params.sellerId,
      );

      return result.fold(
        (error) => Result.success(DiscountValidationResult.error(error)),
        (discount) => Result.success(
          DiscountValidationResult.success(
            discount: discount,
            discountAmount: 0,
          ),
        ),
      );
    } catch (e) {
      return Result.error('Terjadi kesalahan. Coba lagi.');
    }
  }
}

class ValidateDiscountParams {
  final String code;
  final double subtotal;
  final String contextType;
  final String sellerId;

  const ValidateDiscountParams({
    required this.code,
    required this.subtotal,
    required this.contextType,
    required this.sellerId,
  });
}
