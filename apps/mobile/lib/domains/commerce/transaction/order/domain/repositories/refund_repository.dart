/// Refund Repository Interface
library;

import 'package:labuda/core/common/result.dart';
import '../domain.dart';

abstract class RefundRepository {
  Future<Result<RefundRequest>> createRefund(
    CreateRefundParams params,
  );
  Future<Result<RefundRequest>> getRefund(String refundId);
  Future<Result<RefundRequest?>> getRefundByOrderId(String orderId);
  Future<Result<List<RefundRequest>>> listBuyerRefunds(
    ListRefundsParams params,
  );
  Future<Result<List<RefundRequest>>> listSellerRefunds(
    ListRefundsParams params,
  );
  Stream<RefundRequest?> watchRefundByOrderId(String orderId);

  // Refund decision actions (H2-D1)
  Future<Result<RefundRequest>> approveRefund(
    String refundId, {
    String? notes,
  });
  Future<Result<RefundRequest>> rejectRefund(
    String refundId, {
    String? notes,
  });
  Future<Result<Map<String, dynamic>>> escalateRefund(
    String refundId,
  );

  // Order-scoped refund history forSale (used by refund history pager)
  Future<Result<RefundHistoryPageResult>> listOrderRefundHistory(
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
