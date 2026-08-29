/// Discount API Repository Implementation
///
/// Repository implementation that uses Go API datasource.
library;

import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/pricing/discount/data/datasources/discount_api_datasource.dart';
import 'package:labuda/domains/commerce/pricing/discount/data/models/discount_model.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/repositories/i_discount_repository.dart';

/// Repository implementation using Go API datasource
class DiscountApiRepositoryImpl implements IDiscountRepository {
  final DiscountApiDatasource _datasource;

  DiscountApiRepositoryImpl(this._datasource);

  @override
  Future<Result<Discount>> getDiscountByCode(String code) async {
    final result = await _datasource.getDiscountByCode(code);
    return result.fold(
      (error) => Result.error(error),
      (model) => model != null
          ? Result.success(model)
          : Result.error('Discount not found'),
    );
  }

  @override
  Future<Result<List<Discount>>> getSellerDiscounts(String sellerId) async {
    final result = await _datasource.getSellerDiscounts(sellerId);
    return result.fold(
      (error) => Result.error(error),
      (models) => Result.success(models),
    );
  }

  @override
  Future<Result<Discount>> validateDiscountCode(
    String code,
    int subtotalCents, {
    required String contextType,
    required String sellerId,
  }) async {
    final result = await _datasource.validateDiscountCode(
      code: code,
      subtotalCents: subtotalCents,
      contextType: contextType,
      sellerId: sellerId,
    );
    return result.fold(
      (error) => Result.error(error),
      (model) => Result.success(model),
    );
  }

  @override
  Future<Result<Discount>> createDiscount(Discount discount) async {
    final model = DiscountModel.fromEntity(discount);
    final result = await _datasource.createDiscount(model);
    return result.fold(
      (error) => Result.error(error),
      (createdModel) => Result.success(createdModel),
    );
  }

  @override
  Future<Result<Discount>> updateDiscount(Discount discount) async {
    final model = DiscountModel.fromEntity(discount);
    final result = await _datasource.updateDiscount(model);
    return result.fold(
      (error) => Result.error(error),
      (_) => Result.success(discount),
    );
  }

  @override
  Future<Result<void>> deleteDiscount(String discountId) async {
    return await _datasource.deleteDiscount(discountId);
  }
}
