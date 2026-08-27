/// Refund Repository Interface
library;

import '../domain.dart';

abstract class RefundRepository {
  Future<RepositoryResult<RefundRequest>> createRefund(
    CreateRefundParams params,
  );
  Future<RepositoryResult<RefundRequest>> getRefund(String refundId);
  Future<RepositoryResult<RefundRequest?>> getRefundByOrderId(String orderId);
  Future<RepositoryResult<List<RefundRequest>>> listBuyerRefunds(
    ListRefundsParams params,
  );
  Future<RepositoryResult<List<RefundRequest>>> listSellerRefunds(
    ListRefundsParams params,
  );
  Stream<RefundRequest?> watchRefundByOrderId(String orderId);

  // Refund decision actions (H2-D1)
  Future<RepositoryResult<RefundRequest>> approveRefund(
    String refundId, {
    String? notes,
  });
  Future<RepositoryResult<RefundRequest>> rejectRefund(
    String refundId, {
    String? notes,
  });
  Future<RepositoryResult<Map<String, dynamic>>> escalateRefund(
    String refundId,
  );

  // Order-scoped refund history listing (used by refund history pager)
  Future<RepositoryResult<RefundHistoryPageResult>> listOrderRefundHistory(
    ListOrderRefundHistoryParams params,
  );
}

class ListOrderRefundHistoryParams {
  final String? orderId;
  final String? cursor;
  final int? pageSize;

  const ListOrderRefundHistoryParams({
    this.orderId,
    this.cursor,
    this.pageSize,
  });
}
