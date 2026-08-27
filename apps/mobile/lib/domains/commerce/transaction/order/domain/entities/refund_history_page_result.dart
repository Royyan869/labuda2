library;

import 'refund_request.dart';

/// Canonical paged refund history result from the backend.
///
/// The backend owns pagination truth. The UI can only derive `hasMore` from the
/// returned page metadata, never from local heuristics alone.
class RefundHistoryPageResult {
  final List<RefundRequest> refunds;
  final String? nextCursor;
  final bool hasMore;
  final int pageSize;

  const RefundHistoryPageResult({
    required this.refunds,
    required this.nextCursor,
    required this.hasMore,
    required this.pageSize,
  });
}
