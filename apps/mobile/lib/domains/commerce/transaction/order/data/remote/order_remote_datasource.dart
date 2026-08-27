import '../dto/dto_barrel.dart';
import '../models/api/order_api_response_dtos.dart';
import '../../domain/entities/order_params.dart';

/// Order Remote Datasource Interface
/// Abstract to allow different implementations (API, Mock, etc.)
abstract class OrderRemoteDatasource {
  Future<PreviewOrderResponseDto> previewOrder(PreviewOrderRequestDto request);
  Future<OrderDto> createOrder(CreateOrderDto request);
  Future<OrderDto> getOrder(String orderId);
  Future<OrderDto> getOrderByNumber(String orderNumber);
  Future<OrderListDto> listMyOrders({OrderFilterParams? params});
  Future<OrderListDto> listSellerOrders({OrderFilterParams? params});
  Future<OrderStatsDto> getOrderStats({bool asSeller = false});
  Future<OrderDto> updateOrderStatus(
    String orderId,
    UpdateOrderStatusDto request,
  );
  Future<OrderDto> confirmOrder(String orderId);
  Future<OrderDto> shipOrder(String orderId, MarkAsShippedParams params);
  Future<OrderDto> completeOrder(String orderId);
  Future<OrderDto> cancelOrder(String orderId);
  Future<RefundDto> requestRefund(String orderId, CreateRefundDto request);
  Future<RefundDto?> getRefundByOrderId(String orderId);
  Future<RefundDto> getRefund(String refundId);
  Future<RefundListDto> listMyRefunds({RefundFilterParams? params});
  Future<RefundListDto> listSellerRefunds({RefundFilterParams? params});
  Future<CheckDeliveryDto> checkDelivery(CheckDeliveryRequestDto request);
  Future<ShippingProofDto> uploadShippingProof(
    String orderId,
    CreateShippingProofDto request,
  );
  Future<ShippingProofDto> getShippingProof(String orderId);
  Future<ShippingProofDto> updateShippingProof(
    String orderId,
    CreateShippingProofDto request,
  );

  // Order Confirmation endpoints
  Future<OrderConfirmationDto> getConfirmation(String orderId);
  Future<OrderConfirmationDto> extendConfirmation(
    String orderId,
    DateTime newEndDate,
  );
  Future<OrderConfirmationDto> completeConfirmation(
    String orderId,
    String status,
    String completionReason,
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

  /// Get dispute by ID
  Future<DisputeDto> getDispute(String disputeId);

  /// List disputes for admin review
  Future<DisputeListDto> listAdminDisputes({DisputeFilterParams? params});

  /// Admin approve dispute (refund to buyer)
  Future<DisputeDto> adminApproveDispute(
    String disputeId,
    AdminDisputeResolutionDto request,
  );

  /// Admin reject dispute (release to seller)
  Future<DisputeDto> adminRejectDispute(
    String disputeId,
    AdminDisputeResolutionDto request,
  );

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
