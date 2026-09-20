/// Order Repository Interface
library;

import '../domain.dart';

abstract class OrderRepository {
  // Order Preview Operations
  Future<RepositoryResult<PreviewOrderResult>> previewOrder(
    PreviewOrderParams params,
  );

  // Order CRUD Operations
  Future<RepositoryResult<Order>> getOrderById(String orderId);
  Future<RepositoryResult<List<Order>>> getBuyerOrders(GetOrdersParams params);
  Future<RepositoryResult<List<Order>>> getSellerOrders(GetOrdersParams params);

  // Order Page-based forSale (used by order list pager controllers)
  Future<RepositoryResult<OrderPageResult>> getBuyerOrdersPage(
    GetOrdersParams params,
  );
  Future<RepositoryResult<OrderPageResult>> getSellerOrdersPage(
    GetOrdersParams params,
  );

  // Order Status Operations
  Future<RepositoryResult<Order>> cancelOrder(
    String orderId,
    CancelOrderParams params,
  );

  // Ship + complete actions.
  //
  // These are the canonical repository entry points for the backend
  // POST /orders/:id/ship and POST /orders/:id/complete contracts.
  // Buyer "Terima Barang" and seller "Kirim" flow through these.
  Future<RepositoryResult<Order>> markAsShipped(MarkAsShippedParams params);
  Future<RepositoryResult<Order>> markAsDelivered(String orderId);

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
