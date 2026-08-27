import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';

/// Validate Discount Update Use Case
///
/// **DOMAIN:** Commerce - Pricing - Discount domain
/// **RESPONSIBILITY:** Validate discount update operations
/// **BOUNDARY:** Ensures business rules for discount modifications are enforced
class ValidateDiscountUpdateUseCase {
  /// Execute the use case
  ///
  /// Validates discount update according to business rules:
  /// - Description cannot be empty
  /// - Used discounts cannot have validity period shortened
  /// - Used discounts cannot have usage limit decreased below current usage
  Result<void> call({
    required String description,
    required DateTime validUntil,
    required int? totalUsageLimit,
    required Discount originalDiscount,
  }) {
    // Business Rule: Description cannot be empty
    if (description.trim().isEmpty) {
      return Result.error('Description cannot be empty');
    }

    // Business Rule: For used discounts, restricted fields validation
    if (originalDiscount.currentUsageCount > 0) {
      // Cannot shorten validUntil for used discounts
      if (validUntil.isBefore(originalDiscount.validUntil)) {
        return Result.error(
          'Cannot shorten validity period for discount that has been used',
        );
      }

      // Cannot decrease totalUsageLimit below current usage
      if (totalUsageLimit != null &&
          totalUsageLimit < originalDiscount.currentUsageCount) {
        return Result.error(
          'Usage limit cannot be less than current usage (${originalDiscount.currentUsageCount})',
        );
      }
    }

    return Result.success(null);
  }
}
