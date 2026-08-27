import '../domain/domain.dart';
import 'mappers/refund_mapper.dart';
import 'models/api/order_api_response_dtos.dart';
import 'remote/order_remote_datasource.dart';

/// Refund Repository Implementation
class RefundRepositoryImpl implements RefundRepository {
  final OrderRemoteDatasource remoteDatasource;

  RefundRepositoryImpl({required this.remoteDatasource});

  @override
  Future<RepositoryResult<RefundRequest>> createRefund(
    CreateRefundParams params,
  ) async {
    try {
      final dto = RefundMapper.toCreateRefundDto(
        orderId: params.orderId,
        reason: params.reason.apiValue, // Convert enum to API string
        description: params.description,
        evidence: params.evidenceUrls,
      );
      final result = await remoteDatasource.requestRefund(params.orderId, dto);
      final refund = RefundMapper.toRefundRequest(result);
      return RepositoryResult.success(refund);
    } catch (e) {
      return RepositoryResult.failure(e.toString());
    }
  }

  @override
  Future<RepositoryResult<RefundRequest>> getRefund(String refundId) async {
    try {
      final dto = await remoteDatasource.getRefund(refundId);
      final refund = RefundMapper.toRefundRequest(dto);
      return RepositoryResult.success(refund);
    } catch (e) {
      return RepositoryResult.failure(e.toString());
    }
  }

  @override
  Future<RepositoryResult<RefundRequest?>> getRefundByOrderId(
    String orderId,
  ) async {
    try {
      final dto = await remoteDatasource.getRefundByOrderId(orderId);
      if (dto == null) {
        // No refund found for this order - return null (not a failure)
        return RepositoryResult.success(null);
      }
      final refund = RefundMapper.toRefundRequest(dto);
      return RepositoryResult.success(refund);
    } catch (e) {
      return RepositoryResult.failure(e.toString());
    }
  }

  @override
  Future<RepositoryResult<List<RefundRequest>>> listBuyerRefunds(
    ListRefundsParams params,
  ) async {
    try {
      final filterParams = RefundFilterParams(
        status: params.status?.apiValue, // Convert enum to API string
        page: params.page,
        pageSize: params.limit,
      );
      final dto = await remoteDatasource.listMyRefunds(params: filterParams);
      final refunds = RefundMapper.toRefundList(dto.data);
      return RepositoryResult.success(refunds);
    } catch (e) {
      return RepositoryResult.failure(e.toString());
    }
  }

  @override
  Future<RepositoryResult<List<RefundRequest>>> listSellerRefunds(
    ListRefundsParams params,
  ) async {
    try {
      final filterParams = RefundFilterParams(
        status: params.status?.apiValue, // Convert enum to API string
        page: params.page,
        pageSize: params.limit,
      );
      final dto = await remoteDatasource.listSellerRefunds(
        params: filterParams,
      );
      final refunds = RefundMapper.toRefundList(dto.data);
      return RepositoryResult.success(refunds);
    } catch (e) {
      return RepositoryResult.failure(e.toString());
    }
  }

  @override
  Stream<RefundRequest?> watchRefundByOrderId(String orderId) {
    return Stream.periodic(
      const Duration(seconds: 15),
      (_) => orderId,
    ).asyncMap((id) async {
      final result = await getRefundByOrderId(id);
      return result.fold((refund) => refund, (error) => null);
    });
  }

  // Refund decision actions (H2-D1)

  @override
  Future<RepositoryResult<RefundRequest>> approveRefund(
    String refundId, {
    String? notes,
  }) async {
    try {
      final dto = await remoteDatasource.approveRefund(refundId, notes: notes);
      final refund = RefundMapper.toRefundRequest(dto);
      return RepositoryResult.success(refund);
    } catch (e) {
      return RepositoryResult.failure(e.toString());
    }
  }

  @override
  Future<RepositoryResult<RefundRequest>> rejectRefund(
    String refundId, {
    String? notes,
  }) async {
    try {
      final dto = await remoteDatasource.rejectRefund(refundId, notes: notes);
      final refund = RefundMapper.toRefundRequest(dto);
      return RepositoryResult.success(refund);
    } catch (e) {
      return RepositoryResult.failure(e.toString());
    }
  }

  @override
  Future<RepositoryResult<Map<String, dynamic>>> escalateRefund(
    String refundId,
  ) async {
    try {
      final result = await remoteDatasource.escalateRefund(refundId);
      return RepositoryResult.success(result);
    } catch (e) {
      return RepositoryResult.failure(e.toString());
    }
  }

  @override
  Future<RepositoryResult<RefundHistoryPageResult>> listOrderRefundHistory(
    ListOrderRefundHistoryParams params,
  ) async {
    // Fallback aman sementara untuk menambal kontrak yang putus
    return RepositoryResult.success(
      const RefundHistoryPageResult(
        refunds: [],
        nextCursor: null,
        hasMore: false,
        pageSize: 0,
      ),
    );
  }
}
