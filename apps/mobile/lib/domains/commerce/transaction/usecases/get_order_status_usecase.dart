import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/order_repository.dart';

/// Get Order Status Use Case
///
/// **DOMAIN:** Commerce → Transaction
/// **RESPONSIBILITY:** Business logic for fetching order status
/// **BOUNDARY:** Abstracts order repository for commerce domain
///
/// **RULES:**
/// - All order status queries go through this use case
/// - No direct repository access from UI/providers
/// - Centralized error handling
class GetOrderStatusUseCase {
  final OrderRepository _orderRepository;

  const GetOrderStatusUseCase(this._orderRepository);

  /// Execute the use case
  ///
  /// Returns order status for display in chat commerce context
  Future<Result<OrderStatus>> execute(String orderId) async {
    try {
      final result = await _orderRepository.getOrderById(orderId);

      return result.fold(
        (order) => Result.success(order.status),
        (error) => Result.error(error),
      );
    } catch (e) {
      return Result.error('Failed to get order status: $e');
    }
  }

  /// Get both order and payment status
  ///
  /// Returns a map with both statuses for complete order information
  Future<Result<Map<String, dynamic>>> executeWithPaymentStatus(
    String orderId,
  ) async {
    try {
      final result = await _orderRepository.getOrderById(orderId);

      return result.fold(
        (order) => Result.success({
          'orderStatus': order.status,
          'paymentStatus': order.paymentStatus,
          'orderId': order.id,
        }),
        (error) => Result.error(error),
      );
    } catch (e) {
      return Result.error('Failed to get order details: $e');
    }
  }
}
