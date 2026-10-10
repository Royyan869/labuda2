/// Order Repository Interface
library;

import 'package:hishumi/core/common/result.dart';
import '../domain.dart';

abstract class OrderRepository {
  // Order Preview Operations
  Future<Result<PreviewOrderResult>> previewOrder(PreviewOrderParams params);

  // Order CRUD Operations
  Future<Result<Order>> getOrderById(String orderId);
  Future<Result<List<Order>>> getBuyerOrders(GetOrdersParams params);
  Future<Result<List<Order>>> getSellerOrders(GetOrdersParams params);

  // Order Status Operations
  Future<Result<Order>> cancelOrder(String orderId, CancelOrderParams params);

  // Ship + complete actions.
  //
  // These are the canonical repository entry points for the backend
  // POST /orders/:id/ship and POST /orders/:id/complete contracts.
  // Buyer "Terima Barang" and seller "Kirim" flow through these.
  Future<Result<Order>> markAsShipped(MarkAsShippedParams params);
  Future<Result<Order>> markAsDelivered(String orderId);

  // ========================================
  // Order Action Operations (Decision V2)
  // ========================================

  /// Extend order confirmation deadline (buyer action from Decision V2 contract)
  /// POST /orders/{id}/extend-confirmation
  Future<Result<void>> extendOrderConfirmation(String orderId);

  // Real-time Streams
  Stream<Order> watchOrder(String orderId);
  Stream<List<Order>> watchBuyerOrders(WatchOrdersParams params);
  Stream<List<Order>> watchSellerOrders(WatchOrdersParams params);
  Stream<List<Order>> watchSellerNewOrders(String sellerId);
}
