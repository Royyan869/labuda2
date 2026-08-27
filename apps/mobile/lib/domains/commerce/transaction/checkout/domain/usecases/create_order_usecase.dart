import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/transaction/checkout/data/repositories/checkout_repository_impl.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_request.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_response.dart';
import 'package:uuid/uuid.dart';

/// Create Order Use Case
///
/// **DOMAIN:** Commerce → Transaction → Checkout
/// **RESPONSIBILITY:** Handle order creation with idempotency
/// **BOUNDARY:** Encapsulates order creation business logic
class CreateOrderUseCase {
  final CheckoutRepository _repository;

  CreateOrderUseCase(this._repository);

  /// UUID v4 generator for idempotency keys
  static const _uuid = Uuid();

  /// Execute the use case
  ///
  /// Creates an order with the given checkout request.
  /// Generates a UUID v4 idempotency key for safe retries.
  Future<Result<CheckoutResponse>> call(
    CheckoutRequest request, {
    String? idempotencyKey,
  }) async {
    try {
      final validation = validateRequest(request);
      if (validation.isError) {
        return Result.error(validation.error ?? 'Invalid checkout request');
      }

      // Generate idempotency key if not provided
      final key = idempotencyKey ?? _uuid.v4();

      final response = await _repository.createOrder(
        request,
        idempotencyKey: key,
      );

      return Result.success(response);
    } on CheckoutException catch (e) {
      // Preserve the API reason code so the call site can react to gating
      // signals like EMAIL_VERIFICATION_REQUIRED.
      return Result.error(e.userFriendlyMessage, code: e.code);
    } catch (e) {
      return Result.error('Failed to create order: $e');
    }
  }

  /// Validate checkout request
  ///
  /// Business Rules:
  /// - productId must not be empty
  /// - fixedPriceSaleId must not be empty
  /// - pricingToken must not be empty
  /// - shippingAddress must be valid
  Result<void> validateRequest(CheckoutRequest request) {
    if (request.productId == null || request.productId!.isEmpty) {
      return Result.error(
        'ID produk tidak valid. Silakan pilih produk kembali.',
      );
    }

    if (request.fixedPriceSaleId.isEmpty) {
      return Result.error(
        'ID produk tidak valid. Silakan pilih produk kembali.',
      );
    }

    if (request.pricingToken.isEmpty) {
      return Result.error(
        'Token harga tidak valid. Silakan refresh harga terbaru.',
      );
    }

    // Additional validation can be added here
    return Result.success(null);
  }
}
