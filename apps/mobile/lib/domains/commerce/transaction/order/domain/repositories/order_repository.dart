/// Order Repository Interface
library;

import '../domain.dart';

abstract class OrderRepository {
  // Order Preview Operations
  Future<RepositoryResult<PreviewOrderResult>> previewOrder(
    PreviewOrderParams params,
  );

  // Order CRUD Operations
  Future<RepositoryResult<Order>> createOrder(CreateOrderParams params);
  Future<RepositoryResult<Order>> getOrderById(String orderId);
  Future<RepositoryResult<Order>> getOrderByNumber(String orderNumber);
  Future<RepositoryResult<List<Order>>> getBuyerOrders(GetOrdersParams params);
  Future<RepositoryResult<List<Order>>> getSellerOrders(GetOrdersParams params);

  // Order Page-based listing (used by order list pager controllers)
  Future<RepositoryResult<OrderPageResult>> getBuyerOrdersPage(
    GetOrdersParams params,
  );
  Future<RepositoryResult<OrderPageResult>> getSellerOrdersPage(
    GetOrdersParams params,
  );

  Future<RepositoryResult<OrderStats>> getOrderStats(
    GetOrderStatsParams params,
  );

  // Order Status Operations
  Future<RepositoryResult<Order>> updateOrderStatus(
    UpdateOrderStatusParams params,
  );
  Future<RepositoryResult<Order>> confirmOrder(String orderId);
  Future<RepositoryResult<Order>> completeOrder(String orderId);
  Future<RepositoryResult<Order>> cancelOrder(
    String orderId,
    CancelOrderParams params,
  );

  // Additional shipping operations (aliases for notifier compatibility)
  Future<RepositoryResult<Order>> markAsShipped(MarkAsShippedParams params);
  Future<RepositoryResult<Order>> markAsDelivered(String orderId);

  // Payment Operations
  Future<RepositoryResult<Order>> processPayment(
    String orderId,
    ProcessPaymentParams params,
  );
  Future<RepositoryResult<PaymentStatus>> checkPaymentStatus(String orderId);
  Future<RepositoryResult<Order>> updatePaymentToken(
    UpdatePaymentTokenParams params,
  );

  // Shipping Operations
  Future<RepositoryResult<Order>> updateShippingInfo(
    String orderId,
    UpdateShippingInfoParams params,
  );
  Future<RepositoryResult<Order>> addTrackingNumber(
    String orderId,
    String trackingNumber,
  );

  // Validation
  Future<RepositoryResult<bool>> validateShippingAddress(ShippingInfo info);

  // Order Confirmation Operations
  Future<RepositoryResult<OrderConfirmation?>> getConfirmation(String orderId);
  Future<RepositoryResult<OrderConfirmation>> extendConfirmation({
    required String orderId,
    required String buyerId,
  });
  Future<RepositoryResult<OrderConfirmation>> completeConfirmation({
    required String orderId,
    required String completionReason,
  });

  // ========================================
  // Order Action Operations (Decision V2)
  // ========================================

  /// Extend order confirmation deadline (buyer action from Decision V2 contract)
  /// POST /orders/{id}/extend-confirmation
  Future<RepositoryResult<void>> extendOrderConfirmation(String orderId);

  // Real-time Streams
  Stream<Order> watchOrder(String orderId);
  Stream<List<Order>> watchBuyerOrders(WatchOrdersParams params);
  Stream<List<Order>> watchSellerOrders(WatchOrdersParams params);
  Stream<List<Order>> watchSellerNewOrders(String sellerId);
}
