import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/validate_discount_update_use_case.dart';

/// Discount UseCase Providers
///
/// NOTE: Most providers moved to presentation layer to avoid duplication.
/// This file only contains providers that don't belong in presentation layer.

/// Validate Discount Update UseCase Provider
final validateDiscountUpdateUseCaseProvider =
    Provider<ValidateDiscountUpdateUseCase>((ref) {
      // This usecase doesn't need repository, it's pure business logic
      return ValidateDiscountUpdateUseCase();
    });
