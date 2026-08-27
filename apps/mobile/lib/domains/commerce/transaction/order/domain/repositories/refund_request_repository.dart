import '../entities/refund_request.dart';
import 'repository_result.dart';

/// Refund Request Repository Interface
///
/// Defines contract for refund request operations.
/// Implementation in data layer will handle API/Firestore calls.
abstract class RefundRequestRepository {
  // ========================================
  // Refund CRUD Operations
  // ========================================

  /// Get refund by ID
  Future<RepositoryResult<RefundRequest>> getRefundById(String refundId);

  /// Get refund by order ID
  Future<RepositoryResult<RefundRequest?>> getRefundByOrderId(String orderId);

  /// List buyer's refunds
  Future<RepositoryResult<List<RefundRequest>>> getBuyerRefunds(
    ListRefundsParams params,
  );

  /// List seller's refunds
  Future<RepositoryResult<List<RefundRequest>>> getSellerRefunds(
    ListRefundsParams params,
  );

  // ========================================
  // Refund Actions (Buyer)
  // ========================================

  /// Create refund request
  Future<RepositoryResult<RefundRequest>> createRefundRequest(
    CreateRefundParams params,
  );

  // ========================================
  // Real-time Streams
  // ========================================

  /// Watch refund by ID
  Stream<RefundRequest?> watchRefund(String refundId);

  /// Watch refund by order ID
  Stream<RefundRequest?> watchRefundByOrderId(String orderId);
}

/// Parameters for listing refunds
class ListRefundsParams {
  final String userId;
  final RefundStatus? status;
  final int page;
  final int limit;

  const ListRefundsParams({
    required this.userId,
    this.status,
    this.page = 1,
    this.limit = 20,
  });
}

/// Parameters for creating refund request
class CreateRefundParams {
  final String orderId;
  final RefundReason reason;
  final String description;
  final List<String>? evidenceUrls;

  const CreateRefundParams({
    required this.orderId,
    required this.reason,
    required this.description,
    this.evidenceUrls,
  });
}
