import '../dto/dto_barrel.dart';
import '../models/api/order_api_models.dart'
    show
        OrderApiResponse,
        OrderListApiResponse,
        OrderFilterParams,
        RefundFilterParams,
        CheckDeliveryApiResponse,
        CheckDeliveryApiRequest;
import '../../domain/entities/order_params.dart';

/// Order Remote Datasource Interface
/// Abstract to allow different implementations (API, Mock, etc.)
abstract class OrderRemoteDatasource {
  Future<OrderApiResponse> getOrder(String orderId);
  Future<OrderListApiResponse> listMyOrders({OrderFilterParams? params});
  Future<OrderListApiResponse> listSellerOrders({OrderFilterParams? params});
  Future<OrderApiResponse> shipOrder(String orderId, MarkAsShippedParams params);
  Future<OrderApiResponse> completeOrder(String orderId);
  Future<OrderApiResponse> cancelOrder(String orderId);
  Future<RefundDto> requestRefund(String orderId, CreateRefundDto request);
  Future<RefundDto?> getRefundByOrderId(String orderId);
  Future<RefundDto> getRefund(String refundId);
  Future<RefundListDto> listMyRefunds({RefundFilterParams? params});
  Future<RefundListDto> listSellerRefunds({RefundFilterParams? params});
  Future<CheckDeliveryApiResponse> checkDelivery(
    CheckDeliveryApiRequest request,
  );

  // ========================================
  // Refund Decision Operations (H2-D1)
  // ========================================

  /// Seller approves a buyer's refund request
  /// POST /refunds/{id}/approve
  Future<RefundDto> approveRefund(String refundId, {String? notes});

  /// Seller rejects a buyer's refund request
  /// POST /refunds/{id}/reject
  Future<RefundDto> rejectRefund(String refundId, {String? notes});

  /// Buyer escalates a seller-rejected refund to admin dispute
  /// POST /refunds/{id}/escalate
  Future<Map<String, dynamic>> escalateRefund(String refundId);

  // ========================================
  // Dispute Operations
  // ========================================

  /// Create dispute for an order (buyer escalation after seller rejection)
  Future<DisputeDto> createDispute(String orderId, CreateDisputeDto request);

  // ========================================
  // Order Action Operations (Decision V2)
  // ========================================

  /// Extend order confirmation deadline (buyer action)
  /// POST /orders/{id}/extend-confirmation
  /// Requires Idempotency-Key header
  Future<void> extendOrderConfirmation(String orderId);

  // ========================================
  // Pricing Preview Operations
  // ========================================

  /// Generate pricing preview and token via POST /pricing/preview
  /// Returns raw response map: { token, expires_at, pricing_snapshot: {...} }
  Future<Map<String, dynamic>> fetchPricingPreview(Map<String, dynamic> body);
}
