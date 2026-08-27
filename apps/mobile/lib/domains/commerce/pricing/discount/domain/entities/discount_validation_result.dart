import 'package:equatable/equatable.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';

/// Result of discount validation
class DiscountValidationResult extends Equatable {
  final bool isValid;
  final Discount? discount;
  final double discountAmount; // Amount to be discounted
  final String? errorMessage;

  const DiscountValidationResult({
    required this.isValid,
    this.discount,
    required this.discountAmount,
    this.errorMessage,
  });

  /// Factory for success result
  factory DiscountValidationResult.success({
    required Discount discount,
    required double discountAmount,
  }) {
    return DiscountValidationResult(
      isValid: true,
      discount: discount,
      discountAmount: discountAmount,
      errorMessage: null,
    );
  }

  /// Factory for error result
  factory DiscountValidationResult.error(String message) {
    return DiscountValidationResult(
      isValid: false,
      discount: null,
      discountAmount: 0,
      errorMessage: message,
    );
  }

  @override
  List<Object?> get props => [isValid, discount, discountAmount, errorMessage];
}
