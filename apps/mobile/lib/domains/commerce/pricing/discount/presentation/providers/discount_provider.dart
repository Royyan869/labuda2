/// Discount Presentation Providers - Use case providers for discount feature
///
/// R4.1B DI MIGRATION: Migrated from generic service-locator access to proper provider composition
///
/// Previous implementation used service locator access to get use cases.
/// Now use cases are instantiated directly with repository from provider.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_validation_result.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/create_discount_use_case.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/delete_discount_use_case.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/get_seller_discounts_use_case.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/update_discount_use_case.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/validate_discount_use_case.dart';
import 'package:labuda/domains/commerce/pricing/discount/data/discount_providers.dart'
    show discountRepositoryProvider;

// =============================================================================
// USE CASE PROVIDERS (R4.1B: Migrated from service-locator access to provider composition)
// =============================================================================

/// Provider untuk Validate Discount Use Case
///
/// R4.1B: Previously used generic service-locator access
/// Now creates use case directly with repository from provider
final validateDiscountUseCaseProvider = Provider<ValidateDiscountUseCase>((
  ref,
) {
  final repository = ref.watch(discountRepositoryProvider);
  return ValidateDiscountUseCase(repository);
});

/// Provider untuk Create Discount Use Case
///
/// R4.1B: Previously used generic service-locator access
/// Now creates use case directly with repository from provider
final createDiscountUseCaseProvider = Provider<CreateDiscountUseCase>((ref) {
  final repository = ref.watch(discountRepositoryProvider);
  return CreateDiscountUseCase(repository);
});

/// Provider untuk Get Seller Discounts Use Case
///
/// R4.1B: Previously used generic service-locator access
/// Now creates use case directly with repository from provider
final getSellerDiscountsUseCaseProvider = Provider<GetSellerDiscountsUseCase>((
  ref,
) {
  final repository = ref.watch(discountRepositoryProvider);
  return GetSellerDiscountsUseCase(repository);
});

/// Provider untuk Delete Discount Use Case
///
/// R4.1B: Previously used generic service-locator access
/// Now creates use case directly with repository from provider
final deleteDiscountUseCaseProvider = Provider<DeleteDiscountUseCase>((ref) {
  final repository = ref.watch(discountRepositoryProvider);
  return DeleteDiscountUseCase(repository);
});

/// Provider untuk Update Discount Use Case
///
/// R4.1B: Previously used generic service-locator access
/// Now creates use case directly with repository from provider
final updateDiscountUseCaseProvider = Provider<UpdateDiscountUseCase>((ref) {
  final repository = ref.watch(discountRepositoryProvider);
  return UpdateDiscountUseCase(repository);
});

// =============================================================================
// DATA PROVIDERS (for UI consumption)
// =============================================================================

/// Provider untuk get seller's all discounts
final sellerDiscountsProvider = FutureProvider.family<List<Discount>, String>((
  ref,
  sellerId,
) async {
  final useCase = ref.read(getSellerDiscountsUseCaseProvider);
  final result = await useCase(sellerId);

  return result.fold(
    (error) => throw Exception(error),
    (discounts) => discounts,
  );
});

/// Provider untuk validate discount code
/// Params: {code, subtotal, userId, sellerId, collectionIds, varieties}
final discountValidationProvider =
    FutureProvider.family<DiscountValidationResult, ValidateDiscountParams>((
      ref,
      params,
    ) async {
      final useCase = ref.read(validateDiscountUseCaseProvider);
      final result = await useCase(params);

      return result.fold(
        (error) => DiscountValidationResult.error(error),
        (validation) => validation,
      );
    });
